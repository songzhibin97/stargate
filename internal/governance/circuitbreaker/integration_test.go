package circuitbreaker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// TestCircuitBreakerIntegrationWithPipeline tests circuit breaker integration with the full pipeline
func TestCircuitBreakerIntegrationWithPipeline(t *testing.T) {
	// Create circuit breaker config
	cbConfig := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         5, // Higher threshold for integration test
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   8,
		ErrorPercentageThreshold: 50,
	}

	// Create middleware
	middleware, err := NewMiddleware(cbConfig)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker middleware: %v", err)
	}

	// Create a test backend server that can simulate failures
	var backendFailure bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if backendFailure {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Backend Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Backend Success"))
		}
	}))
	defer backend.Close()

	// Create a handler that simulates the pipeline behavior
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate calling backend service
		resp, err := http.Get(backend.URL)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		if resp.StatusCode >= 500 {
			w.Write([]byte("Backend Error"))
		} else {
			w.Write([]byte("Success"))
		}
	}))

	// Test 1: Normal operation - circuit should be closed
	t.Run("NormalOperation", func(t *testing.T) {
		backendFailure = false
		
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "route_id", "test-route")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if w.Header().Get("X-Circuit-Breaker-State") != "CLOSED" {
			t.Errorf("Expected circuit breaker state CLOSED, got %s", w.Header().Get("X-Circuit-Breaker-State"))
		}
	})

	// Test 2: Backend failures - circuit should trip
	t.Run("BackendFailures", func(t *testing.T) {
		backendFailure = true

		// Make enough failed requests to trip the circuit breaker
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), "route_id", "test-route")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// First 5 requests should reach the backend (before consecutive failure threshold)
			if i < 5 && w.Code != http.StatusInternalServerError {
				t.Errorf("Request %d: Expected status 500, got %d", i, w.Code)
			}
		}

		// Next request should be rejected by circuit breaker
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "route_id", "test-route")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected circuit breaker to reject request with 503, got %d", w.Code)
		}

		if w.Header().Get("X-Circuit-Breaker-State") != "OPEN" {
			t.Errorf("Expected circuit breaker state OPEN, got %s", w.Header().Get("X-Circuit-Breaker-State"))
		}
	})

	// Test 3: Recovery - circuit should transition to half-open then closed
	t.Run("Recovery", func(t *testing.T) {
		// Wait for recovery timeout
		time.Sleep(120 * time.Millisecond)

		// Fix the backend
		backendFailure = false

		// First request after recovery timeout should be allowed (half-open)
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "route_id", "test-route")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected first request after recovery to succeed, got %d", w.Code)
		}

		// Make a few more successful requests to close the circuit
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), "route_id", "test-route")
			req = req.WithContext(ctx)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Recovery request %d: Expected status 200, got %d", i, w.Code)
			}
		}

		// Circuit should be closed now
		req = httptest.NewRequest("GET", "/test", nil)
		ctx = context.WithValue(req.Context(), "route_id", "test-route")
		req = req.WithContext(ctx)
		
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Header().Get("X-Circuit-Breaker-State") != "CLOSED" {
			t.Errorf("Expected circuit breaker state CLOSED after recovery, got %s", w.Header().Get("X-Circuit-Breaker-State"))
		}
	})
}

// TestCircuitBreakerMultipleRoutes tests that different routes have independent circuit breakers
func TestCircuitBreakerMultipleRoutes(t *testing.T) {
	cbConfig := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(cbConfig)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker middleware: %v", err)
	}

	// Handler that fails for route1 but succeeds for route2
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeID := r.Context().Value("route_id").(string)
		if routeID == "route1" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Route1 Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Route2 Success"))
		}
	}))

	// Trip circuit breaker for route1
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest("GET", "/route1", nil)
		ctx := context.WithValue(req.Context(), "route_id", "route1")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Verify route1 circuit is open
	req1 := httptest.NewRequest("GET", "/route1", nil)
	ctx1 := context.WithValue(req1.Context(), "route_id", "route1")
	req1 = req1.WithContext(ctx1)
	
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected route1 to be rejected by circuit breaker, got %d", w1.Code)
	}

	// Verify route2 is still working
	req2 := httptest.NewRequest("GET", "/route2", nil)
	ctx2 := context.WithValue(req2.Context(), "route_id", "route2")
	req2 = req2.WithContext(ctx2)
	
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected route2 to work normally, got %d", w2.Code)
	}

	if w2.Header().Get("X-Circuit-Breaker-State") != "CLOSED" {
		t.Errorf("Expected route2 circuit breaker to be CLOSED, got %s", w2.Header().Get("X-Circuit-Breaker-State"))
	}
}

// TestCircuitBreakerConcurrentRequests tests circuit breaker behavior under concurrent load
func TestCircuitBreakerConcurrentRequests(t *testing.T) {
	cbConfig := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         5,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   10,
		ErrorPercentageThreshold: 50,
	}

	middleware, err := NewMiddleware(cbConfig)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker middleware: %v", err)
	}

	// Handler that always fails
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
	}))

	// Make concurrent requests to trip the circuit breaker
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), "route_id", "concurrent-test")
			req = req.WithContext(ctx)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify circuit breaker is open
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), "route_id", "concurrent-test")
	req = req.WithContext(ctx)
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected circuit breaker to be open after concurrent failures, got %d", w.Code)
	}
}

// TestCircuitBreakerErrorPercentage tests circuit breaker tripping based on error percentage
func TestCircuitBreakerErrorPercentage(t *testing.T) {
	cbConfig := &config.CircuitBreakerConfig{
		Enabled:                  true,
		FailureThreshold:         100, // High threshold to test percentage
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   10,
		ErrorPercentageThreshold: 60, // 60% error rate
	}

	middleware, err := NewMiddleware(cbConfig)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker middleware: %v", err)
	}

	// Handler that fails 70% of the time
	requestCount := 0
	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount%10 < 7 { // 70% failure rate
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}
	}))

	// Make requests to trigger error percentage threshold
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "route_id", "percentage-test")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should be rejected by circuit breaker
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), "route_id", "percentage-test")
	req = req.WithContext(ctx)
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected circuit breaker to trip based on error percentage, got %d", w.Code)
	}

	if w.Header().Get("X-Circuit-Breaker-State") != "OPEN" {
		t.Errorf("Expected circuit breaker state OPEN, got %s", w.Header().Get("X-Circuit-Breaker-State"))
	}
}
