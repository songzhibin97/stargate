package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/songzhibin97/stargate/internal/config"
)

// PrometheusMiddleware provides Prometheus metrics collection
type PrometheusMiddleware struct {
	config *config.PrometheusConfig
	
	// HTTP request metrics
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	
	// Connection metrics
	activeConnections prometheus.Gauge
	
	// Error metrics
	errorsTotal *prometheus.CounterVec
}

// prometheusResponseWrapper wraps http.ResponseWriter to capture response details
type prometheusResponseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
}

func (rw *prometheusResponseWrapper) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *prometheusResponseWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(n)
	return n, err
}

// NewPrometheusMiddleware creates a new Prometheus metrics middleware
func NewPrometheusMiddleware(cfg *config.PrometheusConfig) (*PrometheusMiddleware, error) {
	if cfg == nil {
		cfg = &config.PrometheusConfig{
			Enabled:   true,
			Namespace: "stargate",
			Subsystem: "node",
		}
	}

	m := &PrometheusMiddleware{
		config: cfg,
	}

	// Initialize metrics
	m.initMetrics()

	// Register metrics with Prometheus
	if err := m.registerMetrics(); err != nil {
		return nil, err
	}

	return m, nil
}

// initMetrics initializes all Prometheus metrics
func (m *PrometheusMiddleware) initMetrics() {
	// HTTP request total counter
	m.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed",
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTP request duration histogram
	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTP request size histogram
	m.requestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100, 1K, 10K, 100K, 1M, 10M, 100M, 1G
		},
		[]string{"method", "route"},
	)

	// HTTP response size histogram
	m.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100, 1K, 10K, 100K, 1M, 10M, 100M, 1G
		},
		[]string{"method", "route", "status_code"},
	)

	// Active connections gauge
	m.activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_active_connections",
			Help:      "Number of active HTTP connections",
		},
	)

	// Error counter
	m.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_errors_total",
			Help:      "Total number of HTTP errors",
		},
		[]string{"method", "route", "status_code", "error_type"},
	)
}

// registerMetrics registers all metrics with Prometheus
func (m *PrometheusMiddleware) registerMetrics() error {
	collectors := []prometheus.Collector{
		m.requestsTotal,
		m.requestDuration,
		m.requestSize,
		m.responseSize,
		m.activeConnections,
		m.errorsTotal,
	}

	for _, collector := range collectors {
		if err := prometheus.Register(collector); err != nil {
			// If already registered, ignore the error
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return err
			}
		}
	}

	return nil
}

// Handler returns the HTTP middleware handler
func (m *PrometheusMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Increment active connections
			m.activeConnections.Inc()
			defer m.activeConnections.Dec()

			// Wrap response writer to capture response details
			wrapper := &prometheusResponseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapper, r)

			// Calculate duration
			duration := time.Since(start)

			// Extract labels
			method := r.Method
			route := m.getRouteID(r)
			statusCode := strconv.Itoa(wrapper.statusCode)

			// Record metrics
			m.requestsTotal.WithLabelValues(method, route, statusCode).Inc()
			m.requestDuration.WithLabelValues(method, route, statusCode).Observe(duration.Seconds())

			// Record request size if available
			if r.ContentLength > 0 {
				m.requestSize.WithLabelValues(method, route).Observe(float64(r.ContentLength))
			}

			// Record response size
			if wrapper.responseSize > 0 {
				m.responseSize.WithLabelValues(method, route, statusCode).Observe(float64(wrapper.responseSize))
			}

			// Record errors for 4xx and 5xx status codes
			if wrapper.statusCode >= 400 {
				errorType := m.getErrorType(wrapper.statusCode)
				m.errorsTotal.WithLabelValues(method, route, statusCode, errorType).Inc()
			}
		})
	}
}

// getRouteID extracts route ID from request context, fallback to path
func (m *PrometheusMiddleware) getRouteID(r *http.Request) string {
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
func (m *PrometheusMiddleware) getErrorType(statusCode int) string {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// GetMetrics returns current metric values (for debugging/monitoring)
func (m *PrometheusMiddleware) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":   m.config.Enabled,
		"namespace": m.config.Namespace,
		"subsystem": m.config.Subsystem,
	}
}
