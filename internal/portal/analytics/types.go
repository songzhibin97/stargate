package analytics

import (
	"time"
)

// AnalyticsSummaryRequest represents the request parameters for analytics summary
type AnalyticsSummaryRequest struct {
	// Time range for the analytics data
	StartTime *time.Time `json:"start_time,omitempty" form:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" form:"end_time"`
	
	// Time range shortcuts (e.g., "1h", "24h", "7d", "30d")
	TimeRange string `json:"time_range,omitempty" form:"time_range"`
	
	// Specific application IDs to filter (optional, defaults to all user's apps)
	ApplicationIDs []string `json:"application_ids,omitempty" form:"application_ids"`
	
	// Granularity for time series data (e.g., "5m", "1h", "1d")
	Granularity string `json:"granularity,omitempty" form:"granularity"`
	
	// Metrics to include in the response
	Metrics []string `json:"metrics,omitempty" form:"metrics"`
}

// AnalyticsSummaryResponse represents the response for analytics summary
type AnalyticsSummaryResponse struct {
	// Summary statistics
	Summary AnalyticsSummary `json:"summary"`
	
	// Time series data
	TimeSeries []TimeSeriesData `json:"time_series"`
	
	// Application breakdown
	Applications []ApplicationAnalytics `json:"applications"`
	
	// Request metadata
	Metadata ResponseMetadata `json:"metadata"`
}

// AnalyticsSummary represents overall summary statistics
type AnalyticsSummary struct {
	// Total number of requests
	TotalRequests int64 `json:"total_requests"`
	
	// Total number of successful requests (2xx status codes)
	SuccessfulRequests int64 `json:"successful_requests"`
	
	// Total number of failed requests (4xx and 5xx status codes)
	FailedRequests int64 `json:"failed_requests"`
	
	// Success rate as percentage
	SuccessRate float64 `json:"success_rate"`
	
	// Average response time in milliseconds
	AvgResponseTime float64 `json:"avg_response_time"`
	
	// 95th percentile response time in milliseconds
	P95ResponseTime float64 `json:"p95_response_time"`
	
	// 99th percentile response time in milliseconds
	P99ResponseTime float64 `json:"p99_response_time"`
	
	// Total data transferred in bytes
	TotalDataTransferred int64 `json:"total_data_transferred"`
	
	// Number of unique endpoints accessed
	UniqueEndpoints int64 `json:"unique_endpoints"`
	
	// Most active application
	MostActiveApplication *ApplicationSummary `json:"most_active_application,omitempty"`
}

// TimeSeriesData represents time series metrics data
type TimeSeriesData struct {
	// Timestamp for this data point
	Timestamp time.Time `json:"timestamp"`
	
	// Request count for this time period
	RequestCount int64 `json:"request_count"`
	
	// Success count for this time period
	SuccessCount int64 `json:"success_count"`
	
	// Error count for this time period
	ErrorCount int64 `json:"error_count"`
	
	// Average response time for this time period
	AvgResponseTime float64 `json:"avg_response_time"`
	
	// Data transferred for this time period
	DataTransferred int64 `json:"data_transferred"`
}

// ApplicationAnalytics represents analytics data for a specific application
type ApplicationAnalytics struct {
	// Application information
	ApplicationID   string `json:"application_id"`
	ApplicationName string `json:"application_name"`
	
	// Request statistics
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"`
	
	// Performance metrics
	AvgResponseTime float64 `json:"avg_response_time"`
	P95ResponseTime float64 `json:"p95_response_time"`
	
	// Data transfer
	DataTransferred int64 `json:"data_transferred"`
	
	// Top endpoints for this application
	TopEndpoints []EndpointStats `json:"top_endpoints"`
	
	// Error breakdown
	ErrorBreakdown []ErrorStats `json:"error_breakdown"`
}

// ApplicationSummary represents a brief summary of an application
type ApplicationSummary struct {
	ApplicationID   string `json:"application_id"`
	ApplicationName string `json:"application_name"`
	RequestCount    int64  `json:"request_count"`
}

// EndpointStats represents statistics for a specific endpoint
type EndpointStats struct {
	// Endpoint path
	Path string `json:"path"`
	
	// HTTP method
	Method string `json:"method"`
	
	// Request count
	RequestCount int64 `json:"request_count"`
	
	// Average response time
	AvgResponseTime float64 `json:"avg_response_time"`
	
	// Success rate
	SuccessRate float64 `json:"success_rate"`
}

// ErrorStats represents error statistics
type ErrorStats struct {
	// HTTP status code
	StatusCode int `json:"status_code"`
	
	// Error count
	Count int64 `json:"count"`
	
	// Percentage of total requests
	Percentage float64 `json:"percentage"`
	
	// Most common error message (if available)
	CommonMessage string `json:"common_message,omitempty"`
}

// ResponseMetadata represents metadata about the response
type ResponseMetadata struct {
	// Time range of the data
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	
	// Granularity used for time series data
	Granularity string `json:"granularity"`
	
	// Number of applications included
	ApplicationCount int `json:"application_count"`
	
	// Data freshness (when the data was last updated)
	DataFreshness time.Time `json:"data_freshness"`
	
	// Query execution time in milliseconds
	QueryDuration int64 `json:"query_duration"`
}

// PrometheusQueryResult represents the result from a Prometheus query
type PrometheusQueryResult struct {
	Status string                 `json:"status"`
	Data   PrometheusQueryData    `json:"data"`
	Error  string                 `json:"error,omitempty"`
}

// PrometheusQueryData represents the data portion of a Prometheus query result
type PrometheusQueryData struct {
	ResultType string                   `json:"resultType"`
	Result     []PrometheusMetricResult `json:"result"`
}

// PrometheusMetricResult represents a single metric result from Prometheus
type PrometheusMetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value,omitempty"`
	Values [][]interface{}   `json:"values,omitempty"`
}

// PromQLQuery represents a PromQL query configuration
type PromQLQuery struct {
	// Query name for identification
	Name string `json:"name"`
	
	// PromQL query string
	Query string `json:"query"`
	
	// Query type (instant or range)
	Type string `json:"type"`
	
	// Step for range queries
	Step string `json:"step,omitempty"`
}
