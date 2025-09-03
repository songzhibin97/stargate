package circuitbreaker

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// Middleware represents the circuit breaker middleware
type Middleware struct {
	config          *config.CircuitBreakerConfig
	circuitBreakers map[string]*CircuitBreaker
	mutex           sync.RWMutex
	
	// Default circuit breaker for routes without specific configuration
	defaultCircuitBreaker *CircuitBreaker
}

// NewMiddleware creates a new circuit breaker middleware
func NewMiddleware(config *config.CircuitBreakerConfig) (*Middleware, error) {
	if config == nil {
		return nil, fmt.Errorf("circuit breaker config cannot be nil")
	}

	// Create default circuit breaker configuration
	defaultConfig := &Config{
		FailureThreshold:         config.FailureThreshold,
		RecoveryTimeout:          config.RecoveryTimeout,
		RequestVolumeThreshold:   config.RequestVolumeThreshold,
		ErrorPercentageThreshold: config.ErrorPercentageThreshold,
		MaxHalfOpenRequests:      3, // Default value
		SuccessThreshold:         2, // Default value
	}

	// Set defaults if not configured
	if defaultConfig.FailureThreshold <= 0 {
		defaultConfig.FailureThreshold = 5
	}
	if defaultConfig.RecoveryTimeout <= 0 {
		defaultConfig.RecoveryTimeout = 30 * time.Second
	}
	if defaultConfig.RequestVolumeThreshold <= 0 {
		defaultConfig.RequestVolumeThreshold = 10
	}
	if defaultConfig.ErrorPercentageThreshold <= 0 {
		defaultConfig.ErrorPercentageThreshold = 50
	}

	defaultCB := New("default", defaultConfig)
	defaultCB.SetStateChangeCallback(logStateChange)

	return &Middleware{
		config:                config,
		circuitBreakers:       make(map[string]*CircuitBreaker),
		defaultCircuitBreaker: defaultCB,
	}, nil
}

// Handler returns the HTTP middleware handler
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if circuit breaker is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get circuit breaker for this route
			cb := m.getCircuitBreaker(r)

			// Check if request can be executed
			if !cb.CanExecute() {
				m.handleCircuitOpen(w, r, cb)
				return
			}

			// Create response wrapper to capture status code
			wrapper := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Execute the request
			start := time.Now()
			next.ServeHTTP(wrapper, r)
			duration := time.Since(start)

			// Record the result
			if m.isSuccessfulResponse(wrapper.statusCode) {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure()
			}

			// Add circuit breaker headers
			m.addCircuitBreakerHeaders(wrapper, cb, duration)
		})
	}
}

// getCircuitBreaker returns the appropriate circuit breaker for the request
func (m *Middleware) getCircuitBreaker(r *http.Request) *CircuitBreaker {
	// Try to get route-specific circuit breaker
	routeID := m.getRouteID(r)
	if routeID != "" {
		m.mutex.RLock()
		cb, exists := m.circuitBreakers[routeID]
		m.mutex.RUnlock()
		
		if exists {
			return cb
		}

		// Create new circuit breaker for this route
		m.mutex.Lock()
		// Double-check after acquiring write lock
		if cb, exists := m.circuitBreakers[routeID]; exists {
			m.mutex.Unlock()
			return cb
		}

		// Create new circuit breaker with default config
		config := &Config{
			FailureThreshold:         m.config.FailureThreshold,
			RecoveryTimeout:          m.config.RecoveryTimeout,
			RequestVolumeThreshold:   m.config.RequestVolumeThreshold,
			ErrorPercentageThreshold: m.config.ErrorPercentageThreshold,
			MaxHalfOpenRequests:      3,
			SuccessThreshold:         2,
		}

		cb = New(routeID, config)
		cb.SetStateChangeCallback(logStateChange)
		m.circuitBreakers[routeID] = cb
		m.mutex.Unlock()

		return cb
	}

	// Return default circuit breaker
	return m.defaultCircuitBreaker
}

// getRouteID extracts route ID from request context or headers
func (m *Middleware) getRouteID(r *http.Request) string {
	// Try to get route ID from context (set by router)
	if routeID := r.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok {
			return id
		}
	}

	// Try to get route ID from header
	if routeID := r.Header.Get("X-Route-ID"); routeID != "" {
		return routeID
	}

	// Fallback to path-based identification
	return r.URL.Path
}

// isSuccessfulResponse determines if the response is considered successful
func (m *Middleware) isSuccessfulResponse(statusCode int) bool {
	// Consider 2xx and 3xx as successful, 4xx and 5xx as failures
	// 4xx are client errors, but for circuit breaker purposes, we might want to
	// treat them as successful since they don't indicate backend issues
	// However, for this implementation, we'll be conservative and only treat 2xx as success
	return statusCode >= 200 && statusCode < 300
}

// handleCircuitOpen handles requests when circuit breaker is open
func (m *Middleware) handleCircuitOpen(w http.ResponseWriter, r *http.Request, cb *CircuitBreaker) {
	stats := cb.GetStatistics()
	
	// Set circuit breaker headers
	w.Header().Set("X-Circuit-Breaker-State", cb.GetState().String())
	w.Header().Set("X-Circuit-Breaker-Name", cb.GetName())
	w.Header().Set("X-Circuit-Breaker-Error-Rate", fmt.Sprintf("%.2f", stats.ErrorRate()))
	w.Header().Set("X-Circuit-Breaker-Failed-Requests", strconv.FormatInt(stats.FailedRequests, 10))
	w.Header().Set("X-Circuit-Breaker-Total-Requests", strconv.FormatInt(stats.TotalRequests, 10))

	// Return 503 Service Unavailable
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{
		"error": "Service temporarily unavailable",
		"message": "Circuit breaker is open",
		"circuit_breaker": {
			"name": "` + cb.GetName() + `",
			"state": "` + cb.GetState().String() + `",
			"error_rate": ` + fmt.Sprintf("%.2f", stats.ErrorRate()) + `,
			"failed_requests": ` + strconv.FormatInt(stats.FailedRequests, 10) + `,
			"total_requests": ` + strconv.FormatInt(stats.TotalRequests, 10) + `
		}
	}`))
}

// addCircuitBreakerHeaders adds circuit breaker information to response headers
func (m *Middleware) addCircuitBreakerHeaders(w http.ResponseWriter, cb *CircuitBreaker, duration time.Duration) {
	stats := cb.GetStatistics()
	
	w.Header().Set("X-Circuit-Breaker-State", cb.GetState().String())
	w.Header().Set("X-Circuit-Breaker-Name", cb.GetName())
	w.Header().Set("X-Circuit-Breaker-Error-Rate", fmt.Sprintf("%.2f", stats.ErrorRate()))
	w.Header().Set("X-Circuit-Breaker-Response-Time", duration.String())
}

// GetCircuitBreaker returns a circuit breaker by name
func (m *Middleware) GetCircuitBreaker(name string) *CircuitBreaker {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if name == "default" {
		return m.defaultCircuitBreaker
	}
	
	return m.circuitBreakers[name]
}

// GetAllCircuitBreakers returns all circuit breakers
func (m *Middleware) GetAllCircuitBreakers() map[string]*CircuitBreaker {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	result := make(map[string]*CircuitBreaker)
	result["default"] = m.defaultCircuitBreaker
	
	for name, cb := range m.circuitBreakers {
		result[name] = cb
	}
	
	return result
}

// ResetCircuitBreaker resets a specific circuit breaker
func (m *Middleware) ResetCircuitBreaker(name string) error {
	cb := m.GetCircuitBreaker(name)
	if cb == nil {
		return fmt.Errorf("circuit breaker '%s' not found", name)
	}
	
	cb.Reset()
	return nil
}

// ResetAllCircuitBreakers resets all circuit breakers
func (m *Middleware) ResetAllCircuitBreakers() {
	m.defaultCircuitBreaker.Reset()
	
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	for _, cb := range m.circuitBreakers {
		cb.Reset()
	}
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write ensures WriteHeader is called with 200 if not already called
func (rw *responseWrapper) Write(data []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(data)
}

// logStateChange logs circuit breaker state changes
func logStateChange(name string, from, to State) {
	log.Printf("Circuit breaker '%s' state changed from %s to %s", name, from.String(), to.String())
}

// HealthCheck returns the health status of all circuit breakers
func (m *Middleware) HealthCheck() map[string]interface{} {
	result := make(map[string]interface{})
	
	allCBs := m.GetAllCircuitBreakers()
	for name, cb := range allCBs {
		stats := cb.GetStatistics()
		result[name] = map[string]interface{}{
			"state":              cb.GetState().String(),
			"error_rate":         stats.ErrorRate(),
			"total_requests":     stats.TotalRequests,
			"failed_requests":    stats.FailedRequests,
			"successful_requests": stats.SuccessfulRequests,
			"consecutive_failures": stats.ConsecutiveFailures,
			"last_failure_time":  stats.LastFailureTime,
			"state_changed_at":   stats.StateChangedAt,
		}
	}
	
	return result
}
