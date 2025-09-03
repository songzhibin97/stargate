package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/auth"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// MetricsConfig represents configuration for metrics middleware
type MetricsConfig struct {
	Enabled           bool                    `yaml:"enabled" json:"enabled"`
	Provider          string                  `yaml:"provider" json:"provider"`                     // Provider type (prometheus, statsd, etc.)
	Namespace         string                  `yaml:"namespace" json:"namespace"`
	Subsystem         string                  `yaml:"subsystem" json:"subsystem"`
	ConstLabels       map[string]string       `yaml:"const_labels" json:"const_labels"`
	EnabledMetrics    map[string]bool         `yaml:"enabled_metrics" json:"enabled_metrics"`       // Enable/disable specific metrics
	CustomLabels      map[string]string       `yaml:"custom_labels" json:"custom_labels"`           // Custom labels to add
	SampleRate        float64                 `yaml:"sample_rate" json:"sample_rate"`               // Sampling rate for high traffic
	LabelExtractors   map[string]string       `yaml:"label_extractors" json:"label_extractors"`     // Custom label extraction rules
	SensitiveLabels   []string                `yaml:"sensitive_labels" json:"sensitive_labels"`     // Labels to filter out
	MaxLabelLength    int                     `yaml:"max_label_length" json:"max_label_length"`     // Maximum label value length
	AsyncUpdates      bool                    `yaml:"async_updates" json:"async_updates"`           // Enable async metric updates
	BufferSize        int                     `yaml:"buffer_size" json:"buffer_size"`               // Buffer size for async updates
}

// DefaultMetricsConfig returns default configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:   true,
		Provider:  "prometheus",
		Namespace: "stargate",
		Subsystem: "http",
		EnabledMetrics: map[string]bool{
			"requests_total":        true,
			"request_duration":      true,
			"request_size":          true,
			"response_size":         true,
			"active_connections":    true,
			"errors_total":          true,
		},
		SampleRate:     1.0,
		MaxLabelLength: 256,
		AsyncUpdates:   false,
		BufferSize:     1000,
	}
}

// MetricsMiddleware provides generic metrics collection using metrics.Provider
type MetricsMiddleware struct {
	config   *MetricsConfig
	provider metrics.Provider
	
	// HTTP request metrics
	requestsTotal    metrics.CounterVec
	requestDuration  metrics.HistogramVec
	requestSize      metrics.HistogramVec
	responseSize     metrics.HistogramVec
	
	// Connection metrics
	activeConnections metrics.Gauge
	
	// Error metrics
	errorsTotal metrics.CounterVec
	
	// Label cache for performance
	labelCache sync.Map
	
	// Async processing
	metricsChan chan *metricUpdate
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// metricUpdate represents an async metric update
type metricUpdate struct {
	metricType string
	labels     map[string]string
	value      float64
	timestamp  time.Time
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(config *MetricsConfig, provider metrics.Provider) (*MetricsMiddleware, error) {
	if config == nil {
		config = DefaultMetricsConfig()
	}
	
	if provider == nil {
		return nil, metrics.NewMetricError("create", "middleware", nil, 
			fmt.Errorf("metrics provider is required"))
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &MetricsMiddleware{
		config:   config,
		provider: provider,
		ctx:      ctx,
		cancel:   cancel,
	}
	
	// Initialize async processing if enabled
	if config.AsyncUpdates {
		m.metricsChan = make(chan *metricUpdate, config.BufferSize)
		m.startAsyncProcessor()
	}
	
	// Initialize metrics
	if err := m.initMetrics(); err != nil {
		cancel()
		return nil, err
	}
	
	return m, nil
}

// initMetrics initializes all metrics
func (m *MetricsMiddleware) initMetrics() error {
	var err error
	
	// HTTP request total counter
	if m.isMetricEnabled("requests_total") {
		m.requestsTotal, err = m.provider.NewCounterVec(metrics.MetricOptions{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests processed",
			Labels:      []string{"method", "route", "status_code", "consumer_id"},
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create requests counter: %w", err)
		}
	}

	// HTTP request duration histogram
	if m.isMetricEnabled("request_duration") {
		m.requestDuration, err = m.provider.NewHistogramVec(metrics.MetricOptions{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration in seconds",
			Labels:      []string{"method", "route", "status_code", "consumer_id"},
			Buckets:     metrics.GetDefaultBuckets("duration"),
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create request duration histogram: %w", err)
		}
	}
	
	// HTTP request size histogram
	if m.isMetricEnabled("request_size") {
		m.requestSize, err = m.provider.NewHistogramVec(metrics.MetricOptions{
			Name:        "http_request_size_bytes",
			Help:        "HTTP request size in bytes",
			Labels:      []string{"method", "route", "consumer_id"},
			Buckets:     metrics.GetDefaultBuckets("size"),
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create request size histogram: %w", err)
		}
	}

	// HTTP response size histogram
	if m.isMetricEnabled("response_size") {
		m.responseSize, err = m.provider.NewHistogramVec(metrics.MetricOptions{
			Name:        "http_response_size_bytes",
			Help:        "HTTP response size in bytes",
			Labels:      []string{"method", "route", "status_code", "consumer_id"},
			Buckets:     metrics.GetDefaultBuckets("size"),
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create response size histogram: %w", err)
		}
	}
	
	// Active connections gauge
	if m.isMetricEnabled("active_connections") {
		m.activeConnections, err = m.provider.NewGauge(metrics.MetricOptions{
			Name:        "http_active_connections",
			Help:        "Number of active HTTP connections",
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create active connections gauge: %w", err)
		}
	}
	
	// Error counter
	if m.isMetricEnabled("errors_total") {
		m.errorsTotal, err = m.provider.NewCounterVec(metrics.MetricOptions{
			Name:        "http_errors_total",
			Help:        "Total number of HTTP errors",
			Labels:      []string{"method", "route", "status_code", "error_type", "consumer_id"},
			ConstLabels: m.config.ConstLabels,
		})
		if err != nil {
			return fmt.Errorf("failed to create errors counter: %w", err)
		}
	}
	
	return nil
}

// isMetricEnabled checks if a specific metric is enabled
func (m *MetricsMiddleware) isMetricEnabled(metricName string) bool {
	if enabled, exists := m.config.EnabledMetrics[metricName]; exists {
		return enabled
	}
	return true // Default to enabled if not specified
}

// startAsyncProcessor starts the async metric processing goroutine
func (m *MetricsMiddleware) startAsyncProcessor() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case update := <-m.metricsChan:
				m.processMetricUpdate(update)
			case <-m.ctx.Done():
				// Process remaining updates
				for {
					select {
					case update := <-m.metricsChan:
						m.processMetricUpdate(update)
					default:
						return
					}
				}
			}
		}
	}()
}

// processMetricUpdate processes a single metric update
func (m *MetricsMiddleware) processMetricUpdate(update *metricUpdate) {
	// Implementation would process the metric update based on type
	// This is a simplified version
	switch update.metricType {
	case "counter":
		// Process counter update
	case "histogram":
		// Process histogram update
	case "gauge":
		// Process gauge update
	}
}

// Handler returns the HTTP middleware handler
func (m *MetricsMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Apply sampling if configured
			if m.config.SampleRate < 1.0 && !m.shouldSample() {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Increment active connections
			if m.activeConnections != nil {
				m.activeConnections.Inc()
				defer m.activeConnections.Dec()
			}

			// Wrap response writer to capture response details
			wrapper := &metricsResponseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapper, r)

			// Calculate duration
			duration := time.Since(start)

			// Extract and normalize labels
			labels := m.extractLabels(r, wrapper)

			// Record metrics
			m.recordMetrics(r, wrapper, duration, labels)
		})
	}
}

// shouldSample determines if this request should be sampled
func (m *MetricsMiddleware) shouldSample() bool {
	// Simple random sampling - in production, you might want more sophisticated sampling
	return true // For now, always sample
}

// extractLabels extracts and normalizes labels from request and response
func (m *MetricsMiddleware) extractLabels(r *http.Request, wrapper *metricsResponseWrapper) map[string]string {
	// Check cache first
	cacheKey := m.buildCacheKey(r, wrapper)
	if cached, ok := m.labelCache.Load(cacheKey); ok {
		return cached.(map[string]string)
	}

	labels := make(map[string]string)

	// Basic labels
	labels["method"] = r.Method
	labels["route"] = m.getRouteID(r)
	labels["status_code"] = strconv.Itoa(wrapper.statusCode)

	// Extract consumer_id from authentication context
	if consumer, ok := auth.GetConsumerFromContext(r.Context()); ok && consumer != nil {
		labels["consumer_id"] = consumer.ID
	} else {
		// Use "anonymous" for unauthenticated requests
		labels["consumer_id"] = "anonymous"
	}

	// Add custom labels from config
	for key, value := range m.config.CustomLabels {
		labels[key] = value
	}

	// Apply custom label extractors
	for labelName, extractor := range m.config.LabelExtractors {
		if value := m.applyLabelExtractor(extractor, r, wrapper); value != "" {
			labels[labelName] = value
		}
	}

	// Normalize and filter labels
	labels = m.normalizeLabels(labels)

	// Cache the result
	m.labelCache.Store(cacheKey, labels)

	return labels
}

// buildCacheKey builds a cache key for label extraction
func (m *MetricsMiddleware) buildCacheKey(r *http.Request, wrapper *metricsResponseWrapper) string {
	return r.Method + "|" + r.URL.Path + "|" + strconv.Itoa(wrapper.statusCode)
}

// applyLabelExtractor applies a custom label extractor
func (m *MetricsMiddleware) applyLabelExtractor(extractor string, r *http.Request, wrapper *metricsResponseWrapper) string {
	// Simple implementation - in production, you might want a more sophisticated system
	switch extractor {
	case "user_agent":
		return r.Header.Get("User-Agent")
	case "remote_addr":
		return r.RemoteAddr
	case "host":
		return r.Host
	default:
		return ""
	}
}

// normalizeLabels normalizes and filters label values
func (m *MetricsMiddleware) normalizeLabels(labels map[string]string) map[string]string {
	normalized := make(map[string]string)

	for key, value := range labels {
		// Skip sensitive labels
		if m.isSensitiveLabel(key) {
			continue
		}

		// Normalize value
		normalizedValue := m.normalizeLabelValue(value)
		if normalizedValue != "" {
			normalized[key] = normalizedValue
		}
	}

	return normalized
}

// isSensitiveLabel checks if a label is marked as sensitive
func (m *MetricsMiddleware) isSensitiveLabel(labelName string) bool {
	for _, sensitive := range m.config.SensitiveLabels {
		if labelName == sensitive {
			return true
		}
	}
	return false
}

// normalizeLabelValue normalizes a label value
func (m *MetricsMiddleware) normalizeLabelValue(value string) string {
	// Trim whitespace
	value = strings.TrimSpace(value)

	// Limit length
	if m.config.MaxLabelLength > 0 && len(value) > m.config.MaxLabelLength {
		value = value[:m.config.MaxLabelLength]
	}

	// Replace problematic characters
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\t", " ")

	return value
}

// recordMetrics records all enabled metrics
func (m *MetricsMiddleware) recordMetrics(r *http.Request, wrapper *metricsResponseWrapper, duration time.Duration, labels map[string]string) {
	if m.config.AsyncUpdates {
		m.recordMetricsAsync(r, wrapper, duration, labels)
	} else {
		m.recordMetricsSync(r, wrapper, duration, labels)
	}
}

// recordMetricsSync records metrics synchronously
func (m *MetricsMiddleware) recordMetricsSync(r *http.Request, wrapper *metricsResponseWrapper, duration time.Duration, labels map[string]string) {
	method := labels["method"]
	route := labels["route"]
	statusCode := labels["status_code"]
	consumerID := labels["consumer_id"]

	// Record request count
	if m.requestsTotal != nil {
		m.requestsTotal.WithLabelValues(method, route, statusCode, consumerID).Inc()
	}

	// Record request duration
	if m.requestDuration != nil {
		m.requestDuration.WithLabelValues(method, route, statusCode, consumerID).Observe(duration.Seconds())
	}

	// Record request size if available
	if m.requestSize != nil && r.ContentLength > 0 {
		m.requestSize.WithLabelValues(method, route, consumerID).Observe(float64(r.ContentLength))
	}

	// Record response size
	if m.responseSize != nil && wrapper.responseSize > 0 {
		m.responseSize.WithLabelValues(method, route, statusCode, consumerID).Observe(float64(wrapper.responseSize))
	}

	// Record errors for 4xx and 5xx status codes
	if m.errorsTotal != nil && wrapper.statusCode >= 400 {
		errorType := m.getErrorType(wrapper.statusCode)
		m.errorsTotal.WithLabelValues(method, route, statusCode, errorType, consumerID).Inc()
	}
}

// recordMetricsAsync records metrics asynchronously
func (m *MetricsMiddleware) recordMetricsAsync(r *http.Request, wrapper *metricsResponseWrapper, duration time.Duration, labels map[string]string) {
	timestamp := time.Now()

	// Send updates to async processor
	select {
	case m.metricsChan <- &metricUpdate{
		metricType: "requests_total",
		labels:     labels,
		value:      1,
		timestamp:  timestamp,
	}:
	default:
		// Channel full, record synchronously as fallback
		m.recordMetricsSync(r, wrapper, duration, labels)
		return
	}

	// Add more metric updates...
}

// getRouteID extracts route ID from request context, fallback to path
func (m *MetricsMiddleware) getRouteID(r *http.Request) string {
	// Try to get route ID from context
	if routeID := r.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok && id != "" {
			return id
		}
	}

	// Fallback to request path
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return path
}

// getErrorType categorizes HTTP status codes into error types
func (m *MetricsMiddleware) getErrorType(statusCode int) string {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// Close gracefully shuts down the middleware
func (m *MetricsMiddleware) Close() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.config.AsyncUpdates {
		// Close the channel and wait for processor to finish
		close(m.metricsChan)
		m.wg.Wait()
	}

	return nil
}

// GetMetrics returns current metric values (for debugging/monitoring)
func (m *MetricsMiddleware) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         m.config.Enabled,
		"provider":        m.config.Provider,
		"namespace":       m.config.Namespace,
		"subsystem":       m.config.Subsystem,
		"sample_rate":     m.config.SampleRate,
		"async_updates":   m.config.AsyncUpdates,
		"enabled_metrics": m.config.EnabledMetrics,
	}
}

// GetProvider returns the underlying metrics provider
func (m *MetricsMiddleware) GetProvider() metrics.Provider {
	return m.provider
}

// metricsResponseWrapper wraps http.ResponseWriter to capture response details
type metricsResponseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
}

func (rw *metricsResponseWrapper) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *metricsResponseWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(n)
	return n, err
}
