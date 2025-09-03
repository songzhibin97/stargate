package analytics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// Service provides analytics functionality
type Service struct {
	prometheusClient *PrometheusClient
	queryBuilder     *QueryBuilder
	appRepo          portal.ApplicationRepository
	config           *Config
}

// Config represents analytics service configuration
type Config struct {
	PrometheusURL string        `yaml:"prometheus_url" json:"prometheus_url"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	Namespace     string        `yaml:"namespace" json:"namespace"`
	Subsystem     string        `yaml:"subsystem" json:"subsystem"`
	DefaultRange  string        `yaml:"default_range" json:"default_range"`
}

// NewService creates a new analytics service
func NewService(config *Config, appRepo portal.ApplicationRepository) *Service {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.DefaultRange == "" {
		config.DefaultRange = "24h"
	}
	if config.Namespace == "" {
		config.Namespace = "stargate"
	}
	if config.Subsystem == "" {
		config.Subsystem = "gateway"
	}

	prometheusClient := NewPrometheusClient(config.PrometheusURL, config.Timeout)
	queryBuilder := NewQueryBuilder(config.Namespace, config.Subsystem)

	return &Service{
		prometheusClient: prometheusClient,
		queryBuilder:     queryBuilder,
		appRepo:          appRepo,
		config:           config,
	}
}

// GetAnalyticsSummary retrieves analytics summary for a user
func (s *Service) GetAnalyticsSummary(ctx context.Context, userID string, req *AnalyticsSummaryRequest) (*AnalyticsSummaryResponse, error) {
	startTime := time.Now()

	// Get user's applications
	apps, err := s.appRepo.GetApplicationsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user applications: %w", err)
	}

	if len(apps) == 0 {
		return s.createEmptyResponse(req), nil
	}

	// Extract consumer IDs (API keys) from applications
	consumerIDs := make([]string, 0, len(apps))
	appMap := make(map[string]*portal.Application)
	
	for _, app := range apps {
		if app.Status == portal.ApplicationStatusActive {
			consumerIDs = append(consumerIDs, app.APIKey)
			appMap[app.APIKey] = app
		}
	}

	if len(consumerIDs) == 0 {
		return s.createEmptyResponse(req), nil
	}

	// Filter by specific application IDs if requested
	if len(req.ApplicationIDs) > 0 {
		filteredConsumerIDs := make([]string, 0)
		for _, appID := range req.ApplicationIDs {
			for _, app := range apps {
				if app.ID == appID && app.Status == portal.ApplicationStatusActive {
					filteredConsumerIDs = append(filteredConsumerIDs, app.APIKey)
					break
				}
			}
		}
		consumerIDs = filteredConsumerIDs
	}

	// Determine time range
	timeRange, start, end, err := s.parseTimeRange(req)
	if err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	// Determine granularity
	granularity := req.Granularity
	if granularity == "" {
		granularity = DetermineGranularity(end.Sub(start))
	}

	// Query Prometheus for summary data
	summary, err := s.getSummaryData(ctx, consumerIDs, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary data: %w", err)
	}

	// Query Prometheus for time series data
	timeSeries, err := s.getTimeSeriesData(ctx, consumerIDs, start, end, granularity)
	if err != nil {
		return nil, fmt.Errorf("failed to get time series data: %w", err)
	}

	// Query Prometheus for application breakdown
	applicationAnalytics, err := s.getApplicationAnalytics(ctx, consumerIDs, appMap, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get application analytics: %w", err)
	}

	// Build response
	response := &AnalyticsSummaryResponse{
		Summary:      *summary,
		TimeSeries:   timeSeries,
		Applications: applicationAnalytics,
		Metadata: ResponseMetadata{
			StartTime:        start,
			EndTime:          end,
			Granularity:      granularity,
			ApplicationCount: len(apps),
			DataFreshness:    time.Now(),
			QueryDuration:    time.Since(startTime).Milliseconds(),
		},
	}

	return response, nil
}

// getSummaryData retrieves summary statistics from Prometheus
func (s *Service) getSummaryData(ctx context.Context, consumerIDs []string, timeRange string) (*AnalyticsSummary, error) {
	summary := &AnalyticsSummary{}

	// Query total requests
	requestsQuery := s.queryBuilder.BuildRequestsQuery(consumerIDs, timeRange)
	requestsResult, err := s.prometheusClient.QueryInstant(ctx, requestsQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query total requests: %w", err)
	}

	totalRequests := s.sumMetricResults(requestsResult)
	summary.TotalRequests = int64(totalRequests)

	// Query success rate
	successRateQuery := s.queryBuilder.BuildSuccessRateQuery(consumerIDs, timeRange)
	successRateResult, err := s.prometheusClient.QueryInstant(ctx, successRateQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query success rate: %w", err)
	}

	successRate := s.avgMetricResults(successRateResult)
	summary.SuccessRate = successRate
	summary.SuccessfulRequests = int64(float64(totalRequests) * successRate / 100)
	summary.FailedRequests = summary.TotalRequests - summary.SuccessfulRequests

	// Query average response time
	responseTimeQuery := s.queryBuilder.BuildResponseTimeQuery(consumerIDs, timeRange)
	responseTimeResult, err := s.prometheusClient.QueryInstant(ctx, responseTimeQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query response time: %w", err)
	}

	avgResponseTime := s.avgMetricResults(responseTimeResult)
	summary.AvgResponseTime = avgResponseTime * 1000 // Convert to milliseconds

	// Query data transfer
	dataTransferQuery := s.queryBuilder.BuildDataTransferQuery(consumerIDs, timeRange)
	dataTransferResult, err := s.prometheusClient.QueryInstant(ctx, dataTransferQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query data transfer: %w", err)
	}

	totalDataTransfer := s.sumMetricResults(dataTransferResult)
	summary.TotalDataTransferred = int64(totalDataTransfer)

	// TODO: Add queries for P95, P99 response times and unique endpoints
	// These would require histogram metrics and more complex queries

	return summary, nil
}

// getTimeSeriesData retrieves time series data from Prometheus
func (s *Service) getTimeSeriesData(ctx context.Context, consumerIDs []string, start, end time.Time, step string) ([]TimeSeriesData, error) {
	// Query requests over time
	requestsQuery := s.queryBuilder.BuildRequestsQuery(consumerIDs, step)
	requestsResult, err := s.prometheusClient.QueryRange(ctx, requestsQuery, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests time series: %w", err)
	}

	// Query success rate over time
	successRateQuery := s.queryBuilder.BuildSuccessRateQuery(consumerIDs, step)
	successRateResult, err := s.prometheusClient.QueryRange(ctx, successRateQuery, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query success rate time series: %w", err)
	}

	// Query response time over time
	responseTimeQuery := s.queryBuilder.BuildResponseTimeQuery(consumerIDs, step)
	responseTimeResult, err := s.prometheusClient.QueryRange(ctx, responseTimeQuery, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query response time time series: %w", err)
	}

	// Combine results into time series data
	timeSeries := s.combineTimeSeriesResults(requestsResult, successRateResult, responseTimeResult)

	return timeSeries, nil
}

// getApplicationAnalytics retrieves per-application analytics
func (s *Service) getApplicationAnalytics(ctx context.Context, consumerIDs []string, appMap map[string]*portal.Application, timeRange string) ([]ApplicationAnalytics, error) {
	analytics := make([]ApplicationAnalytics, 0, len(consumerIDs))

	for _, consumerID := range consumerIDs {
		app, exists := appMap[consumerID]
		if !exists {
			continue
		}

		appAnalytics, err := s.getApplicationAnalyticsForConsumer(ctx, consumerID, app, timeRange)
		if err != nil {
			return nil, fmt.Errorf("failed to get analytics for application %s: %w", app.ID, err)
		}

		analytics = append(analytics, *appAnalytics)
	}

	return analytics, nil
}

// getApplicationAnalyticsForConsumer retrieves analytics for a specific consumer
func (s *Service) getApplicationAnalyticsForConsumer(ctx context.Context, consumerID string, app *portal.Application, timeRange string) (*ApplicationAnalytics, error) {
	consumerIDs := []string{consumerID}

	// Query requests for this consumer
	requestsQuery := s.queryBuilder.BuildRequestsQuery(consumerIDs, timeRange)
	requestsResult, err := s.prometheusClient.QueryInstant(ctx, requestsQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}

	totalRequests := int64(s.sumMetricResults(requestsResult))

	// Query success rate for this consumer
	successRateQuery := s.queryBuilder.BuildSuccessRateQuery(consumerIDs, timeRange)
	successRateResult, err := s.prometheusClient.QueryInstant(ctx, successRateQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query success rate: %w", err)
	}

	successRate := s.avgMetricResults(successRateResult)
	successfulRequests := int64(float64(totalRequests) * successRate / 100)
	failedRequests := totalRequests - successfulRequests

	// Query response time for this consumer
	responseTimeQuery := s.queryBuilder.BuildResponseTimeQuery(consumerIDs, timeRange)
	responseTimeResult, err := s.prometheusClient.QueryInstant(ctx, responseTimeQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query response time: %w", err)
	}

	avgResponseTime := s.avgMetricResults(responseTimeResult) * 1000 // Convert to milliseconds

	// Query data transfer for this consumer
	dataTransferQuery := s.queryBuilder.BuildDataTransferQuery(consumerIDs, timeRange)
	dataTransferResult, err := s.prometheusClient.QueryInstant(ctx, dataTransferQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query data transfer: %w", err)
	}

	dataTransferred := int64(s.sumMetricResults(dataTransferResult))

	// Query top endpoints for this consumer
	topEndpointsQuery := s.queryBuilder.BuildTopEndpointsQuery(consumerIDs, timeRange, 10)
	topEndpointsResult, err := s.prometheusClient.QueryInstant(ctx, topEndpointsQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query top endpoints: %w", err)
	}

	topEndpoints := s.parseTopEndpoints(topEndpointsResult)

	return &ApplicationAnalytics{
		ApplicationID:      app.ID,
		ApplicationName:    app.Name,
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		SuccessRate:        successRate,
		AvgResponseTime:    avgResponseTime,
		P95ResponseTime:    0, // TODO: Implement with histogram metrics
		DataTransferred:    dataTransferred,
		TopEndpoints:       topEndpoints,
		ErrorBreakdown:     []ErrorStats{}, // TODO: Implement error breakdown
	}, nil
}

// parseTimeRange parses the time range from request
func (s *Service) parseTimeRange(req *AnalyticsSummaryRequest) (string, time.Time, time.Time, error) {
	var start, end time.Time
	var timeRange string

	if req.StartTime != nil && req.EndTime != nil {
		start = *req.StartTime
		end = *req.EndTime
		duration := end.Sub(start)
		timeRange = DetermineGranularity(duration)
	} else if req.TimeRange != "" {
		duration, err := ParseTimeRange(req.TimeRange)
		if err != nil {
			return "", time.Time{}, time.Time{}, err
		}
		end = time.Now()
		start = end.Add(-duration)
		timeRange = req.TimeRange
	} else {
		// Use default range
		duration, _ := ParseTimeRange(s.config.DefaultRange)
		end = time.Now()
		start = end.Add(-duration)
		timeRange = s.config.DefaultRange
	}

	return timeRange, start, end, nil
}

// createEmptyResponse creates an empty response when no data is available
func (s *Service) createEmptyResponse(req *AnalyticsSummaryRequest) *AnalyticsSummaryResponse {
	_, start, end, _ := s.parseTimeRange(req)
	granularity := req.Granularity
	if granularity == "" {
		granularity = DetermineGranularity(end.Sub(start))
	}

	return &AnalyticsSummaryResponse{
		Summary: AnalyticsSummary{
			TotalRequests:      0,
			SuccessfulRequests: 0,
			FailedRequests:     0,
			SuccessRate:        0,
			AvgResponseTime:    0,
			P95ResponseTime:    0,
			P99ResponseTime:    0,
			TotalDataTransferred: 0,
			UniqueEndpoints:    0,
		},
		TimeSeries:   []TimeSeriesData{},
		Applications: []ApplicationAnalytics{},
		Metadata: ResponseMetadata{
			StartTime:        start,
			EndTime:          end,
			Granularity:      granularity,
			ApplicationCount: 0,
			DataFreshness:    time.Now(),
			QueryDuration:    0,
		},
	}
}

// sumMetricResults sums all metric values from Prometheus result
func (s *Service) sumMetricResults(result *PrometheusQueryResult) float64 {
	if result == nil || result.Data.ResultType != "vector" {
		return 0
	}

	var sum float64
	for _, metric := range result.Data.Result {
		if len(metric.Value) >= 2 {
			if value, err := strconv.ParseFloat(fmt.Sprintf("%v", metric.Value[1]), 64); err == nil {
				sum += value
			}
		}
	}
	return sum
}

// avgMetricResults calculates average of all metric values from Prometheus result
func (s *Service) avgMetricResults(result *PrometheusQueryResult) float64 {
	if result == nil || result.Data.ResultType != "vector" || len(result.Data.Result) == 0 {
		return 0
	}

	var sum float64
	var count int
	for _, metric := range result.Data.Result {
		if len(metric.Value) >= 2 {
			if value, err := strconv.ParseFloat(fmt.Sprintf("%v", metric.Value[1]), 64); err == nil {
				sum += value
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// combineTimeSeriesResults combines multiple Prometheus range query results into time series data
func (s *Service) combineTimeSeriesResults(requestsResult, successRateResult, responseTimeResult *PrometheusQueryResult) []TimeSeriesData {
	timeSeriesMap := make(map[int64]*TimeSeriesData)

	// Process requests data
	if requestsResult != nil && requestsResult.Data.ResultType == "matrix" {
		for _, metric := range requestsResult.Data.Result {
			for _, value := range metric.Values {
				if len(value) >= 2 {
					timestamp := int64(value[0].(float64))
					if requestCount, err := strconv.ParseFloat(fmt.Sprintf("%v", value[1]), 64); err == nil {
						if _, exists := timeSeriesMap[timestamp]; !exists {
							timeSeriesMap[timestamp] = &TimeSeriesData{
								Timestamp: time.Unix(timestamp, 0),
							}
						}
						timeSeriesMap[timestamp].RequestCount += int64(requestCount)
					}
				}
			}
		}
	}

	// Process success rate data
	if successRateResult != nil && successRateResult.Data.ResultType == "matrix" {
		for _, metric := range successRateResult.Data.Result {
			for _, value := range metric.Values {
				if len(value) >= 2 {
					timestamp := int64(value[0].(float64))
					if successRate, err := strconv.ParseFloat(fmt.Sprintf("%v", value[1]), 64); err == nil {
						if ts, exists := timeSeriesMap[timestamp]; exists {
							ts.SuccessCount = int64(float64(ts.RequestCount) * successRate / 100)
							ts.ErrorCount = ts.RequestCount - ts.SuccessCount
						}
					}
				}
			}
		}
	}

	// Process response time data
	if responseTimeResult != nil && responseTimeResult.Data.ResultType == "matrix" {
		for _, metric := range responseTimeResult.Data.Result {
			for _, value := range metric.Values {
				if len(value) >= 2 {
					timestamp := int64(value[0].(float64))
					if responseTime, err := strconv.ParseFloat(fmt.Sprintf("%v", value[1]), 64); err == nil {
						if ts, exists := timeSeriesMap[timestamp]; exists {
							ts.AvgResponseTime = responseTime * 1000 // Convert to milliseconds
						}
					}
				}
			}
		}
	}

	// Convert map to slice and sort by timestamp
	timeSeries := make([]TimeSeriesData, 0, len(timeSeriesMap))
	for _, ts := range timeSeriesMap {
		timeSeries = append(timeSeries, *ts)
	}

	// Sort by timestamp (simple bubble sort for small datasets)
	for i := 0; i < len(timeSeries)-1; i++ {
		for j := 0; j < len(timeSeries)-i-1; j++ {
			if timeSeries[j].Timestamp.After(timeSeries[j+1].Timestamp) {
				timeSeries[j], timeSeries[j+1] = timeSeries[j+1], timeSeries[j]
			}
		}
	}

	return timeSeries
}

// parseTopEndpoints parses top endpoints from Prometheus result
func (s *Service) parseTopEndpoints(result *PrometheusQueryResult) []EndpointStats {
	if result == nil || result.Data.ResultType != "vector" {
		return []EndpointStats{}
	}

	endpoints := make([]EndpointStats, 0, len(result.Data.Result))
	for _, metric := range result.Data.Result {
		if len(metric.Value) >= 2 {
			if requestCount, err := strconv.ParseFloat(fmt.Sprintf("%v", metric.Value[1]), 64); err == nil {
				endpoint := EndpointStats{
					Path:         metric.Metric["route"],
					Method:       metric.Metric["method"],
					RequestCount: int64(requestCount),
					// TODO: Add response time and success rate for each endpoint
					AvgResponseTime: 0,
					SuccessRate:     0,
				}
				endpoints = append(endpoints, endpoint)
			}
		}
	}

	return endpoints
}

// Health checks if the analytics service is healthy
func (s *Service) Health(ctx context.Context) error {
	return s.prometheusClient.Health(ctx)
}
