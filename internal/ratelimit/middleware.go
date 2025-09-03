package ratelimit

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// Middleware represents the rate limiting middleware
type Middleware struct {
	manager    *Manager
	config     *Config
	limiterName string
}

// NewMiddleware creates a new rate limiting middleware
func NewMiddleware(config *Config) (*Middleware, error) {
	if config == nil {
		config = DefaultConfig()
	}

	manager := NewManager(config)
	
	// Create default rate limiter
	limiterName := "default"
	_, err := manager.CreateLimiter(limiterName, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}

	return &Middleware{
		manager:     manager,
		config:      config,
		limiterName: limiterName,
	}, nil
}

// Handler returns an HTTP middleware handler function
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting if disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			result := m.manager.CheckRequest(m.limiterName, r)
			
			// Set rate limit headers
			if result.Quota != nil {
				SetRateLimitHeaders(w, result.Quota)
			}

			// If rate limited, return 429
			if !result.Allowed {
				m.handleRateLimited(w, r, result)
				return
			}

			// Request is allowed, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// handleRateLimited handles rate limited requests
func (m *Middleware) handleRateLimited(w http.ResponseWriter, r *http.Request, result *RateLimitResult) {
	// Set Retry-After header
	if result.RetryAfter > 0 {
		retryAfterSeconds := int(result.RetryAfter.Seconds())
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}

	// Set custom headers if configured
	for key, value := range m.config.CustomHeaders {
		w.Header().Set(key, value)
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")
	
	// Set status code
	w.WriteHeader(http.StatusTooManyRequests)

	// Create error response
	errorResponse := RateLimitErrorResponse{
		Error:   "Too Many Requests",
		Message: "Rate limit exceeded. Please try again later.",
		Code:    http.StatusTooManyRequests,
	}

	if result.Quota != nil {
		errorResponse.Limit = result.Quota.Limit
		errorResponse.Remaining = result.Quota.Remaining
		errorResponse.ResetTime = result.Quota.ResetTime.Unix()
	}

	if result.RetryAfter > 0 {
		errorResponse.RetryAfter = int(result.RetryAfter.Seconds())
	}

	// Write JSON response
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Failed to encode rate limit error response: %v", err)
		// Fallback to plain text
		w.Write([]byte(`{"error":"Too Many Requests","message":"Rate limit exceeded"}`))
	}
}

// RateLimitErrorResponse represents the error response for rate limited requests
type RateLimitErrorResponse struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Code       int    `json:"code"`
	Limit      int    `json:"limit,omitempty"`
	Remaining  int    `json:"remaining,omitempty"`
	ResetTime  int64  `json:"reset_time,omitempty"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

// UpdateConfig updates the middleware configuration
func (m *Middleware) UpdateConfig(config *Config) error {
	if config == nil {
		return ErrInvalidConfig
	}

	// Stop existing manager
	m.manager.Stop()

	// Create new manager with updated config
	m.manager = NewManager(config)
	m.config = config

	// Recreate rate limiter with new config
	_, err := m.manager.CreateLimiter(m.limiterName, config)
	if err != nil {
		return fmt.Errorf("failed to recreate rate limiter: %w", err)
	}

	return nil
}

// GetStats returns statistics about the rate limiter
func (m *Middleware) GetStats() map[string]*RateLimiterStats {
	return m.manager.GetAllStats()
}

// Health returns the health status of the middleware
func (m *Middleware) Health() map[string]interface{} {
	health := m.manager.Health()
	health["middleware_enabled"] = m.config.Enabled
	return health
}

// Stop stops the middleware and cleans up resources
func (m *Middleware) Stop() {
	if m.manager != nil {
		m.manager.Stop()
	}
}

// ConditionalMiddleware creates a middleware that applies rate limiting conditionally
type ConditionalMiddleware struct {
	middleware *Middleware
	condition  func(*http.Request) bool
}

// NewConditionalMiddleware creates a new conditional rate limiting middleware
func NewConditionalMiddleware(config *Config, condition func(*http.Request) bool) (*ConditionalMiddleware, error) {
	middleware, err := NewMiddleware(config)
	if err != nil {
		return nil, err
	}

	return &ConditionalMiddleware{
		middleware: middleware,
		condition:  condition,
	}, nil
}

// Handler returns an HTTP middleware handler function that applies rate limiting conditionally
func (cm *ConditionalMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check condition
			if cm.condition != nil && !cm.condition(r) {
				// Condition not met, skip rate limiting
				next.ServeHTTP(w, r)
				return
			}

			// Apply rate limiting
			cm.middleware.Handler()(next).ServeHTTP(w, r)
		})
	}
}

// Stop stops the conditional middleware
func (cm *ConditionalMiddleware) Stop() {
	if cm.middleware != nil {
		cm.middleware.Stop()
	}
}

// Common condition functions

// ConditionByPath returns a condition function that applies rate limiting to specific paths
func ConditionByPath(paths []string) func(*http.Request) bool {
	pathMap := make(map[string]bool)
	for _, path := range paths {
		pathMap[path] = true
	}

	return func(r *http.Request) bool {
		return pathMap[r.URL.Path]
	}
}

// ConditionByMethod returns a condition function that applies rate limiting to specific HTTP methods
func ConditionByMethod(methods []string) func(*http.Request) bool {
	methodMap := make(map[string]bool)
	for _, method := range methods {
		methodMap[method] = true
	}

	return func(r *http.Request) bool {
		return methodMap[r.Method]
	}
}

// ConditionByHeader returns a condition function that applies rate limiting based on header presence
func ConditionByHeader(headerName, headerValue string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if headerValue == "" {
			// Just check if header exists
			return r.Header.Get(headerName) != ""
		}
		// Check if header has specific value
		return r.Header.Get(headerName) == headerValue
	}
}

// ConditionByUserAgent returns a condition function that applies rate limiting to specific user agents
func ConditionByUserAgent(userAgents []string) func(*http.Request) bool {
	uaMap := make(map[string]bool)
	for _, ua := range userAgents {
		uaMap[ua] = true
	}

	return func(r *http.Request) bool {
		return uaMap[r.Header.Get("User-Agent")]
	}
}

// ConditionExcludeIPs returns a condition function that excludes specific IP addresses from rate limiting
func ConditionExcludeIPs(excludedIPs []string) func(*http.Request) bool {
	ipMap := make(map[string]bool)
	for _, ip := range excludedIPs {
		ipMap[ip] = true
	}

	return func(r *http.Request) bool {
		clientIP := extractClientIP(r)
		return !ipMap[clientIP] // Apply rate limiting if IP is NOT in excluded list
	}
}

// CombineConditions combines multiple conditions with AND logic
func CombineConditions(conditions ...func(*http.Request) bool) func(*http.Request) bool {
	return func(r *http.Request) bool {
		for _, condition := range conditions {
			if !condition(r) {
				return false
			}
		}
		return true
	}
}

// CombineConditionsOr combines multiple conditions with OR logic
func CombineConditionsOr(conditions ...func(*http.Request) bool) func(*http.Request) bool {
	return func(r *http.Request) bool {
		for _, condition := range conditions {
			if condition(r) {
				return true
			}
		}
		return false
	}
}
