package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// HeaderTransformMiddleware handles HTTP header transformations
type HeaderTransformMiddleware struct {
	config *config.HeaderTransformConfig
	mu     sync.RWMutex
	stats  *HeaderTransformStats
}

// HeaderTransformStats represents statistics for header transformations
type HeaderTransformStats struct {
	RequestsProcessed    int64     `json:"requests_processed"`
	RequestHeadersAdded  int64     `json:"request_headers_added"`
	RequestHeadersRemoved int64    `json:"request_headers_removed"`
	RequestHeadersRenamed int64    `json:"request_headers_renamed"`
	RequestHeadersReplaced int64   `json:"request_headers_replaced"`
	ResponseHeadersAdded  int64    `json:"response_headers_added"`
	ResponseHeadersRemoved int64   `json:"response_headers_removed"`
	ResponseHeadersRenamed int64   `json:"response_headers_renamed"`
	ResponseHeadersReplaced int64  `json:"response_headers_replaced"`
	LastProcessedAt      time.Time `json:"last_processed_at"`
}

// HeaderTransformResult represents the result of header transformation
type HeaderTransformResult struct {
	RouteID           string            `json:"route_id"`
	RequestTransforms map[string]string `json:"request_transforms"`
	ResponseTransforms map[string]string `json:"response_transforms"`
	ProcessedAt       time.Time         `json:"processed_at"`
}

// NewHeaderTransformMiddleware creates a new header transform middleware
func NewHeaderTransformMiddleware(config *config.HeaderTransformConfig) *HeaderTransformMiddleware {
	return &HeaderTransformMiddleware{
		config: config,
		stats: &HeaderTransformStats{
			LastProcessedAt: time.Now(),
		},
	}
}

// Handler returns the HTTP middleware handler
func (m *HeaderTransformMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get route ID from context
			routeID := m.getRouteID(r)

			// Transform request headers
			m.transformRequestHeaders(r, routeID)

			// Create response wrapper to capture and transform response headers
			wrapper := &responseWrapper{
				ResponseWriter: w,
				middleware:     m,
				routeID:       routeID,
				request:       r,
			}

			// Update statistics
			m.updateStats()

			// Continue to next handler
			next.ServeHTTP(wrapper, r)
		})
	}
}

// transformRequestHeaders applies header transformations to the request
func (m *HeaderTransformMiddleware) transformRequestHeaders(r *http.Request, routeID string) {
	// Get transformation rules for this route
	rules := m.getRequestTransformRules(routeID)

	// Apply ADD operations
	for key, value := range rules.Add {
		// Support dynamic values (e.g., ${request_id}, ${timestamp})
		expandedValue := m.expandValue(value, r)
		r.Header.Set(key, expandedValue)
		m.incrementStat("request_headers_added")
	}

	// Apply REMOVE operations
	for _, key := range rules.Remove {
		if r.Header.Get(key) != "" {
			r.Header.Del(key)
			m.incrementStat("request_headers_removed")
		}
	}

	// Apply RENAME operations
	for oldKey, newKey := range rules.Rename {
		if value := r.Header.Get(oldKey); value != "" {
			r.Header.Del(oldKey)
			r.Header.Set(newKey, value)
			m.incrementStat("request_headers_renamed")
		}
	}

	// Apply REPLACE operations
	for key, value := range rules.Replace {
		if r.Header.Get(key) != "" {
			expandedValue := m.expandValue(value, r)
			r.Header.Set(key, expandedValue)
			m.incrementStat("request_headers_replaced")
		}
	}
}

// transformResponseHeaders applies header transformations to the response
func (m *HeaderTransformMiddleware) transformResponseHeaders(w http.ResponseWriter, r *http.Request, routeID string) {
	// Get transformation rules for this route
	rules := m.getResponseTransformRules(routeID)

	// Apply ADD operations
	for key, value := range rules.Add {
		expandedValue := m.expandValue(value, r)
		w.Header().Set(key, expandedValue)
		m.incrementStat("response_headers_added")
	}

	// Apply REMOVE operations
	for _, key := range rules.Remove {
		if w.Header().Get(key) != "" {
			w.Header().Del(key)
			m.incrementStat("response_headers_removed")
		}
	}

	// Apply RENAME operations
	for oldKey, newKey := range rules.Rename {
		if value := w.Header().Get(oldKey); value != "" {
			w.Header().Del(oldKey)
			w.Header().Set(newKey, value)
			m.incrementStat("response_headers_renamed")
		}
	}

	// Apply REPLACE operations
	for key, value := range rules.Replace {
		if w.Header().Get(key) != "" {
			expandedValue := m.expandValue(value, r)
			w.Header().Set(key, expandedValue)
			m.incrementStat("response_headers_replaced")
		}
	}
}

// getRequestTransformRules gets the request header transformation rules for a route
func (m *HeaderTransformMiddleware) getRequestTransformRules(routeID string) config.HeaderTransformRules {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for per-route configuration first
	if routeConfig, exists := m.config.PerRoute[routeID]; exists && routeConfig.Enabled {
		return routeConfig.RequestHeaders
	}

	// Fall back to global configuration
	return m.config.RequestHeaders
}

// getResponseTransformRules gets the response header transformation rules for a route
func (m *HeaderTransformMiddleware) getResponseTransformRules(routeID string) config.HeaderTransformRules {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for per-route configuration first
	if routeConfig, exists := m.config.PerRoute[routeID]; exists && routeConfig.Enabled {
		return routeConfig.ResponseHeaders
	}

	// Fall back to global configuration
	return m.config.ResponseHeaders
}

// expandValue expands dynamic values in header values
func (m *HeaderTransformMiddleware) expandValue(value string, r *http.Request) string {
	// Replace common placeholders
	expanded := value
	expanded = strings.ReplaceAll(expanded, "${timestamp}", time.Now().Format(time.RFC3339))
	expanded = strings.ReplaceAll(expanded, "${request_id}", m.generateRequestID())
	expanded = strings.ReplaceAll(expanded, "${method}", r.Method)
	expanded = strings.ReplaceAll(expanded, "${path}", r.URL.Path)
	expanded = strings.ReplaceAll(expanded, "${host}", r.Host)
	
	// Replace request header values
	if strings.Contains(expanded, "${header:") {
		// Extract header name from ${header:X-Original-Header}
		start := strings.Index(expanded, "${header:")
		if start != -1 {
			end := strings.Index(expanded[start:], "}")
			if end != -1 {
				headerName := expanded[start+9 : start+end]
				headerValue := r.Header.Get(headerName)
				placeholder := expanded[start : start+end+1]
				expanded = strings.ReplaceAll(expanded, placeholder, headerValue)
			}
		}
	}

	return expanded
}

// getRouteID extracts route ID from request context
func (m *HeaderTransformMiddleware) getRouteID(r *http.Request) string {
	if routeID := r.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok {
			return id
		}
	}
	return "default"
}

// generateRequestID generates a unique request ID
func (m *HeaderTransformMiddleware) generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + strings.ToUpper(randomString(8))
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// incrementStat safely increments a statistic
func (m *HeaderTransformMiddleware) incrementStat(statName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch statName {
	case "request_headers_added":
		m.stats.RequestHeadersAdded++
	case "request_headers_removed":
		m.stats.RequestHeadersRemoved++
	case "request_headers_renamed":
		m.stats.RequestHeadersRenamed++
	case "request_headers_replaced":
		m.stats.RequestHeadersReplaced++
	case "response_headers_added":
		m.stats.ResponseHeadersAdded++
	case "response_headers_removed":
		m.stats.ResponseHeadersRemoved++
	case "response_headers_renamed":
		m.stats.ResponseHeadersRenamed++
	case "response_headers_replaced":
		m.stats.ResponseHeadersReplaced++
	}
}

// updateStats updates general statistics
func (m *HeaderTransformMiddleware) updateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.stats.RequestsProcessed++
	m.stats.LastProcessedAt = time.Now()
}

// GetStats returns current statistics
func (m *HeaderTransformMiddleware) GetStats() *HeaderTransformStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	statsCopy := *m.stats
	return &statsCopy
}

// responseWrapper wraps http.ResponseWriter to capture and transform response headers
type responseWrapper struct {
	http.ResponseWriter
	middleware   *HeaderTransformMiddleware
	routeID      string
	request      *http.Request
	headersSent  bool
	statusCode   int
}

// WriteHeader captures the status code and transforms response headers before writing
func (w *responseWrapper) WriteHeader(statusCode int) {
	if w.headersSent {
		return
	}

	w.statusCode = statusCode
	w.headersSent = true

	// Transform response headers before sending
	w.middleware.transformResponseHeaders(w.ResponseWriter, w.request, w.routeID)

	w.ResponseWriter.WriteHeader(statusCode)
}

// Write ensures headers are sent before writing body
func (w *responseWrapper) Write(data []byte) (int, error) {
	if !w.headersSent {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// UpdateConfig updates the middleware configuration
func (m *HeaderTransformMiddleware) UpdateConfig(config *config.HeaderTransformConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// ResetStats resets all statistics
func (m *HeaderTransformMiddleware) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = &HeaderTransformStats{
		LastProcessedAt: time.Now(),
	}
}
