package circuitbreaker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestMiddlewareDisabled(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled: false,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("Expected body 'success', got '%s'", w.Body.String())
	}
}

func TestMiddlewareEnabled(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Test successful request
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check circuit breaker headers
	if w.Header().Get("X-Circuit-Breaker-State") != "CLOSED" {
		t.Errorf("Expected circuit breaker state CLOSED, got %s", w.Header().Get("X-Circuit-Breaker-State"))
	}
}

func TestMiddlewareCircuitOpen(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Handler that always returns 500
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))

	// Make enough failed requests to trip the circuit breaker
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should be rejected by circuit breaker
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	if w.Header().Get("X-Circuit-Breaker-State") != "OPEN" {
		t.Errorf("Expected circuit breaker state OPEN, got %s", w.Header().Get("X-Circuit-Breaker-State"))
	}

	// Check response body contains circuit breaker information
	body := w.Body.String()
	if !strings.Contains(body, "Circuit breaker is open") {
		t.Errorf("Expected response body to contain circuit breaker message, got: %s", body)
	}
}

func TestMiddlewareRouteSpecificCircuitBreaker(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Handler that always returns 500
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))

	// Make requests to route1 to trip its circuit breaker
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "route_id", "route1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Request to route1 should be rejected
	req1 := httptest.NewRequest("GET", "/test", nil)
	ctx1 := context.WithValue(req1.Context(), "route_id", "route1")
	req1 = req1.WithContext(ctx1)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected route1 request to be rejected with 503, got %d", w1.Code)
	}

	// Request to route2 should still be allowed
	req2 := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req2.Context(), "route_id", "route2")
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusInternalServerError {
		t.Errorf("Expected route2 request to be processed normally, got %d", w2.Code)
	}
}

func TestMiddlewareRecovery(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          50 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Handler that can switch between success and failure
	var shouldFail bool
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))

	// Trip the circuit breaker
	shouldFail = true
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Verify circuit is open
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected circuit to be open, got status %d", w.Code)
	}

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Switch to success mode
	shouldFail = false

	// Make successful requests to close the circuit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		// First request should be allowed (half-open state)
		if i == 0 && w.Code == http.StatusServiceUnavailable {
			t.Errorf("Expected first request after recovery timeout to be allowed, got %d", w.Code)
		}
	}

	// Circuit should be closed now
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected circuit to be closed, got status %d", w.Code)
	}

	if w.Header().Get("X-Circuit-Breaker-State") != "CLOSED" {
		t.Errorf("Expected circuit breaker state CLOSED, got %s", w.Header().Get("X-Circuit-Breaker-State"))
	}
}

func TestMiddlewareGetCircuitBreaker(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Get default circuit breaker
	defaultCB := middleware.GetCircuitBreaker("default")
	if defaultCB == nil {
		t.Error("Expected to get default circuit breaker")
	}

	if defaultCB.GetName() != "default" {
		t.Errorf("Expected circuit breaker name 'default', got '%s'", defaultCB.GetName())
	}

	// Get non-existent circuit breaker
	nonExistentCB := middleware.GetCircuitBreaker("non-existent")
	if nonExistentCB != nil {
		t.Error("Expected nil for non-existent circuit breaker")
	}
}

func TestMiddlewareResetCircuitBreaker(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Trip the default circuit breaker
	defaultCB := middleware.GetCircuitBreaker("default")
	for i := 0; i < 10; i++ {
		defaultCB.RecordFailure()
	}

	if defaultCB.GetState() != StateOpen {
		t.Errorf("Expected circuit breaker to be OPEN, got %s", defaultCB.GetState())
	}

	// Reset the circuit breaker
	err = middleware.ResetCircuitBreaker("default")
	if err != nil {
		t.Errorf("Failed to reset circuit breaker: %v", err)
	}

	if defaultCB.GetState() != StateClosed {
		t.Errorf("Expected circuit breaker to be CLOSED after reset, got %s", defaultCB.GetState())
	}

	// Try to reset non-existent circuit breaker
	err = middleware.ResetCircuitBreaker("non-existent")
	if err == nil {
		t.Error("Expected error when resetting non-existent circuit breaker")
	}
}

func TestMiddlewareHealthCheck(t *testing.T) {
	config := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Get health check information
	health := middleware.HealthCheck()
	if health == nil {
		t.Error("Expected health check to return data")
	}

	// Should contain default circuit breaker
	if _, exists := health["default"]; !exists {
		t.Error("Expected health check to contain default circuit breaker")
	}

	defaultHealth := health["default"].(map[string]interface{})
	if defaultHealth["state"] != "CLOSED" {
		t.Errorf("Expected default circuit breaker state to be CLOSED, got %s", defaultHealth["state"])
	}
}
