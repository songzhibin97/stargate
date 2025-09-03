package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// AggregatorMiddleware represents the API aggregator middleware
type AggregatorMiddleware struct {
	config *config.AggregatorConfig
	client *http.Client
	mutex  sync.RWMutex

	// Statistics
	totalRequests     int64
	aggregatedRequests int64
	failedRequests    int64
}

// UpstreamRequest represents a single upstream request configuration
type UpstreamRequest struct {
	Name     string            `yaml:"name" json:"name"`
	URL      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Timeout  time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Required bool              `yaml:"required,omitempty" json:"required,omitempty"`
}

// AggregateRoute represents a single aggregate route configuration
type AggregateRoute struct {
	ID               string            `yaml:"id" json:"id"`
	Path             string            `yaml:"path" json:"path"`
	Method           string            `yaml:"method" json:"method"`
	UpstreamRequests []UpstreamRequest `yaml:"upstream_requests" json:"upstream_requests"`
	ResponseTemplate string            `yaml:"response_template,omitempty" json:"response_template,omitempty"`
	Timeout          time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// UpstreamResult represents the result of an upstream request
type UpstreamResult struct {
	Name       string
	StatusCode int
	Body       []byte
	Headers    http.Header
	Error      error
	Duration   time.Duration
}

// AggregatedResponse represents the final aggregated response
type AggregatedResponse struct {
	StatusCode int
	Body       interface{}
	Headers    http.Header
}

// NewAggregatorMiddleware creates a new aggregator middleware
func NewAggregatorMiddleware(cfg *config.AggregatorConfig) *AggregatorMiddleware {
	client := &http.Client{
		Timeout: cfg.DefaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &AggregatorMiddleware{
		config: cfg,
		client: client,
	}
}

// Handler returns the HTTP middleware handler
func (m *AggregatorMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Update total requests statistics
			m.updateTotalRequests()

			// Check if this request matches any aggregate route
			route := m.matchAggregateRoute(r)
			if route == nil {
				// No match, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Handle aggregate request
			m.handleAggregateRequest(w, r, route)
		})
	}
}

// matchAggregateRoute finds a matching aggregate route for the request
func (m *AggregatorMiddleware) matchAggregateRoute(r *http.Request) *AggregateRoute {
	path := r.URL.Path
	method := r.Method

	for _, route := range m.config.Routes {
		// Match path and method
		if m.matchPath(path, route.Path) && m.matchMethod(method, route.Method) {
			// Convert config.AggregateRoute to AggregateRoute
			aggregateRoute := &AggregateRoute{
				ID:               route.ID,
				Path:             route.Path,
				Method:           route.Method,
				UpstreamRequests: make([]UpstreamRequest, len(route.UpstreamRequests)),
				ResponseTemplate: route.ResponseTemplate,
				Timeout:          route.Timeout,
			}

			// Convert upstream requests
			for i, upstreamReq := range route.UpstreamRequests {
				aggregateRoute.UpstreamRequests[i] = UpstreamRequest{
					Name:     upstreamReq.Name,
					URL:      upstreamReq.URL,
					Method:   upstreamReq.Method,
					Headers:  upstreamReq.Headers,
					Timeout:  upstreamReq.Timeout,
					Required: upstreamReq.Required,
				}
			}

			return aggregateRoute
		}
	}

	return nil
}

// matchPath checks if the request path matches the route path
func (m *AggregatorMiddleware) matchPath(requestPath, routePath string) bool {
	// Simple exact match for now, can be extended to support patterns
	return requestPath == routePath
}

// matchMethod checks if the request method matches the route method
func (m *AggregatorMiddleware) matchMethod(requestMethod, routeMethod string) bool {
	// Empty route method matches all methods
	if routeMethod == "" {
		return true
	}
	return strings.EqualFold(requestMethod, routeMethod)
}

// handleAggregateRequest processes an aggregate request
func (m *AggregatorMiddleware) handleAggregateRequest(w http.ResponseWriter, r *http.Request, route *AggregateRoute) {
	// Update aggregated requests statistics
	m.updateAggregatedRequests()

	// Set timeout for the entire aggregation process
	timeout := route.Timeout
	if timeout == 0 {
		timeout = m.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Execute upstream requests in parallel
	results := m.executeUpstreamRequests(ctx, route.UpstreamRequests)

	// Check if any required requests failed
	if m.hasRequiredFailures(results, route.UpstreamRequests) {
		m.updateFailedRequests()
		m.handleError(w, r, http.StatusBadGateway, "Required upstream request failed")
		return
	}

	// Merge responses
	aggregatedResponse := m.mergeResponses(results, route.ResponseTemplate)

	// Write response
	m.writeResponse(w, aggregatedResponse)
}

// executeUpstreamRequests executes all upstream requests in parallel
func (m *AggregatorMiddleware) executeUpstreamRequests(ctx context.Context, upstreamRequests []UpstreamRequest) map[string]*UpstreamResult {
	results := make(chan *UpstreamResult, len(upstreamRequests))
	
	// Start all requests in parallel
	for _, upstreamReq := range upstreamRequests {
		go m.executeUpstreamRequest(ctx, upstreamReq, results)
	}

	// Collect results
	responseMap := make(map[string]*UpstreamResult)
	for i := 0; i < len(upstreamRequests); i++ {
		select {
		case result := <-results:
			responseMap[result.Name] = result
		case <-ctx.Done():
			// Context cancelled or timed out
			log.Printf("Upstream request collection timed out")
			break
		}
	}

	return responseMap
}

// executeUpstreamRequest executes a single upstream request
func (m *AggregatorMiddleware) executeUpstreamRequest(ctx context.Context, upstreamReq UpstreamRequest, results chan<- *UpstreamResult) {
	start := time.Now()
	result := &UpstreamResult{
		Name: upstreamReq.Name,
	}

	defer func() {
		result.Duration = time.Since(start)
		results <- result
	}()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, upstreamReq.Method, upstreamReq.URL, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		return
	}

	// Set headers
	for key, value := range upstreamReq.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := m.client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("request failed: %w", err)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to read response body: %w", err)
		return
	}

	result.StatusCode = resp.StatusCode
	result.Body = body
	result.Headers = resp.Header
}

// hasRequiredFailures checks if any required upstream requests failed
func (m *AggregatorMiddleware) hasRequiredFailures(results map[string]*UpstreamResult, upstreamRequests []UpstreamRequest) bool {
	for _, upstreamReq := range upstreamRequests {
		if upstreamReq.Required {
			result, exists := results[upstreamReq.Name]
			if !exists || result.Error != nil || result.StatusCode >= 400 {
				return true
			}
		}
	}
	return false
}

// mergeResponses merges upstream responses into a single response
func (m *AggregatorMiddleware) mergeResponses(results map[string]*UpstreamResult, template string) *AggregatedResponse {
	responseData := make(map[string]interface{})

	// Process each upstream result
	for name, result := range results {
		if result.Error != nil {
			responseData[name] = map[string]interface{}{
				"error":  result.Error.Error(),
				"status": "failed",
			}
		} else {
			// Try to parse JSON response
			var data interface{}
			if err := json.Unmarshal(result.Body, &data); err != nil {
				// If not JSON, store as string
				responseData[name] = string(result.Body)
			} else {
				responseData[name] = data
			}
		}
	}

	// Apply template if provided (simplified implementation)
	if template != "" {
		// For now, just return the template with data substitution
		// This can be extended to support more complex templating
		return &AggregatedResponse{
			StatusCode: http.StatusOK,
			Body:       responseData,
			Headers:    make(http.Header),
		}
	}

	// Default merge strategy
	return &AggregatedResponse{
		StatusCode: http.StatusOK,
		Body:       responseData,
		Headers:    make(http.Header),
	}
}

// writeResponse writes the aggregated response
func (m *AggregatorMiddleware) writeResponse(w http.ResponseWriter, response *AggregatedResponse) {
	// Set headers
	for key, values := range response.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Write status code
	w.WriteHeader(response.StatusCode)

	// Write body
	if response.Body != nil {
		if err := json.NewEncoder(w).Encode(response.Body); err != nil {
			log.Printf("Failed to encode aggregated response: %v", err)
		}
	}
}

// handleError handles error responses
func (m *AggregatorMiddleware) handleError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"path":    r.URL.Path,
		"method":  r.Method,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// Statistics methods
func (m *AggregatorMiddleware) updateTotalRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalRequests++
}

func (m *AggregatorMiddleware) updateAggregatedRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.aggregatedRequests++
}

func (m *AggregatorMiddleware) updateFailedRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failedRequests++
}

// GetStats returns middleware statistics
func (m *AggregatorMiddleware) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_requests":      m.totalRequests,
		"aggregated_requests": m.aggregatedRequests,
		"failed_requests":     m.failedRequests,
		"success_rate":        float64(m.aggregatedRequests-m.failedRequests) / float64(m.aggregatedRequests) * 100,
	}
}
