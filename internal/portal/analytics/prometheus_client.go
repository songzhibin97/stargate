package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PrometheusClient handles communication with Prometheus server
type PrometheusClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(baseURL string, timeout time.Duration) *PrometheusClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &PrometheusClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// QueryInstant executes an instant query against Prometheus
func (pc *PrometheusClient) QueryInstant(ctx context.Context, query string, timestamp time.Time) (*PrometheusQueryResult, error) {
	params := url.Values{}
	params.Set("query", query)
	if !timestamp.IsZero() {
		params.Set("time", strconv.FormatInt(timestamp.Unix(), 10))
	}

	return pc.executeQuery(ctx, "/api/v1/query", params)
}

// QueryRange executes a range query against Prometheus
func (pc *PrometheusClient) QueryRange(ctx context.Context, query string, start, end time.Time, step string) (*PrometheusQueryResult, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(start.Unix(), 10))
	params.Set("end", strconv.FormatInt(end.Unix(), 10))
	params.Set("step", step)

	return pc.executeQuery(ctx, "/api/v1/query_range", params)
}

// executeQuery executes a query against Prometheus
func (pc *PrometheusClient) executeQuery(ctx context.Context, endpoint string, params url.Values) (*PrometheusQueryResult, error) {
	reqURL := pc.baseURL + endpoint + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result PrometheusQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", result.Error)
	}

	return &result, nil
}

// Health checks if Prometheus is healthy
func (pc *PrometheusClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", pc.baseURL+"/-/healthy", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// QueryBuilder helps build PromQL queries with consumer_id filtering
type QueryBuilder struct {
	namespace string
	subsystem string
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(namespace, subsystem string) *QueryBuilder {
	return &QueryBuilder{
		namespace: namespace,
		subsystem: subsystem,
	}
}

// BuildRequestsQuery builds a query for total requests by consumer_id
func (qb *QueryBuilder) BuildRequestsQuery(consumerIDs []string, timeRange string) string {
	metricName := qb.buildMetricName("http_requests_total")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		return fmt.Sprintf("sum(increase(%s{%s}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
	}
	return fmt.Sprintf("sum(%s{%s}) by (consumer_id)", metricName, consumerFilter)
}

// BuildSuccessRateQuery builds a query for success rate by consumer_id
func (qb *QueryBuilder) BuildSuccessRateQuery(consumerIDs []string, timeRange string) string {
	metricName := qb.buildMetricName("http_requests_total")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		successQuery := fmt.Sprintf("sum(increase(%s{%s,status_code=~\"2..\"}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
		totalQuery := fmt.Sprintf("sum(increase(%s{%s}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
		return fmt.Sprintf("(%s / %s) * 100", successQuery, totalQuery)
	}
	
	successQuery := fmt.Sprintf("sum(%s{%s,status_code=~\"2..\"}) by (consumer_id)", metricName, consumerFilter)
	totalQuery := fmt.Sprintf("sum(%s{%s}) by (consumer_id)", metricName, consumerFilter)
	return fmt.Sprintf("(%s / %s) * 100", successQuery, totalQuery)
}

// BuildResponseTimeQuery builds a query for average response time by consumer_id
func (qb *QueryBuilder) BuildResponseTimeQuery(consumerIDs []string, timeRange string) string {
	metricName := qb.buildMetricName("http_request_duration_seconds")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		return fmt.Sprintf("avg(rate(%s_sum{%s}[%s]) / rate(%s_count{%s}[%s])) by (consumer_id)", 
			metricName, consumerFilter, timeRange, metricName, consumerFilter, timeRange)
	}
	return fmt.Sprintf("avg(%s{%s}) by (consumer_id)", metricName, consumerFilter)
}

// BuildErrorRateQuery builds a query for error rate by consumer_id
func (qb *QueryBuilder) BuildErrorRateQuery(consumerIDs []string, timeRange string) string {
	metricName := qb.buildMetricName("http_requests_total")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		errorQuery := fmt.Sprintf("sum(increase(%s{%s,status_code=~\"[45]..\"}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
		totalQuery := fmt.Sprintf("sum(increase(%s{%s}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
		return fmt.Sprintf("(%s / %s) * 100", errorQuery, totalQuery)
	}
	
	errorQuery := fmt.Sprintf("sum(%s{%s,status_code=~\"[45]..\"}) by (consumer_id)", metricName, consumerFilter)
	totalQuery := fmt.Sprintf("sum(%s{%s}) by (consumer_id)", metricName, consumerFilter)
	return fmt.Sprintf("(%s / %s) * 100", errorQuery, totalQuery)
}

// BuildDataTransferQuery builds a query for data transfer by consumer_id
func (qb *QueryBuilder) BuildDataTransferQuery(consumerIDs []string, timeRange string) string {
	metricName := qb.buildMetricName("http_response_size_bytes")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		return fmt.Sprintf("sum(increase(%s_sum{%s}[%s])) by (consumer_id)", metricName, consumerFilter, timeRange)
	}
	return fmt.Sprintf("sum(%s{%s}) by (consumer_id)", metricName, consumerFilter)
}

// BuildTopEndpointsQuery builds a query for top endpoints by consumer_id
func (qb *QueryBuilder) BuildTopEndpointsQuery(consumerIDs []string, timeRange string, limit int) string {
	metricName := qb.buildMetricName("http_requests_total")
	consumerFilter := qb.buildConsumerFilter(consumerIDs)
	
	if timeRange != "" {
		return fmt.Sprintf("topk(%d, sum(increase(%s{%s}[%s])) by (consumer_id, method, route))", 
			limit, metricName, consumerFilter, timeRange)
	}
	return fmt.Sprintf("topk(%d, sum(%s{%s}) by (consumer_id, method, route))", 
		limit, metricName, consumerFilter)
}

// buildMetricName constructs the full metric name with namespace and subsystem
func (qb *QueryBuilder) buildMetricName(metricName string) string {
	if qb.namespace != "" && qb.subsystem != "" {
		return fmt.Sprintf("%s_%s_%s", qb.namespace, qb.subsystem, metricName)
	} else if qb.namespace != "" {
		return fmt.Sprintf("%s_%s", qb.namespace, metricName)
	}
	return metricName
}

// buildConsumerFilter constructs the consumer_id filter for PromQL queries
func (qb *QueryBuilder) buildConsumerFilter(consumerIDs []string) string {
	if len(consumerIDs) == 0 {
		return "consumer_id!=\"\""
	}
	
	if len(consumerIDs) == 1 {
		return fmt.Sprintf("consumer_id=\"%s\"", consumerIDs[0])
	}
	
	// Multiple consumer IDs - use regex
	filter := "consumer_id=~\""
	for i, id := range consumerIDs {
		if i > 0 {
			filter += "|"
		}
		filter += id
	}
	filter += "\""
	
	return filter
}

// ParseTimeRange converts time range string to duration
func ParseTimeRange(timeRange string) (time.Duration, error) {
	switch timeRange {
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "12h":
		return 12 * time.Hour, nil
	case "24h", "1d":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	default:
		return time.ParseDuration(timeRange)
	}
}

// DetermineGranularity determines appropriate granularity based on time range
func DetermineGranularity(duration time.Duration) string {
	if duration <= time.Hour {
		return "1m"
	} else if duration <= 6*time.Hour {
		return "5m"
	} else if duration <= 24*time.Hour {
		return "15m"
	} else if duration <= 7*24*time.Hour {
		return "1h"
	} else {
		return "6h"
	}
}
