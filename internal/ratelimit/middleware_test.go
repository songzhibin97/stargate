package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware_NewMiddleware(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        10,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	if middleware == nil {
		t.Fatal("Expected non-nil middleware")
	}

	if middleware.config != config {
		t.Error("Config not set correctly")
	}
}

func TestMiddleware_Handler_Disabled(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        10,
		CleanupInterval:    5 * time.Minute,
		Enabled:            false,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Should pass through without rate limiting
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", w.Body.String())
	}
}

func TestMiddleware_Handler_AllowedRequest(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        5,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Should be allowed
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check rate limit headers
	if w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("Expected X-RateLimit-Limit header to be '5', got %s", w.Header().Get("X-RateLimit-Limit"))
	}

	if w.Header().Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("Expected X-RateLimit-Remaining header to be '4', got %s", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestMiddleware_Handler_RateLimited(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        2,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	clientIP := "192.168.1.1:12345"

	// Make requests up to limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i+1, w.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should be rate limited
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Check response body
	var errorResponse RateLimitErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errorResponse.Error != "Too Many Requests" {
		t.Errorf("Expected error 'Too Many Requests', got %s", errorResponse.Error)
	}

	if errorResponse.Code != http.StatusTooManyRequests {
		t.Errorf("Expected code 429, got %d", errorResponse.Code)
	}

	// Check rate limit headers
	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining header to be '0', got %s", w.Header().Get("X-RateLimit-Remaining"))
	}

	// Check Retry-After header
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header to be set")
	}
}

func TestMiddleware_Handler_CustomHeaders(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        1,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
		CustomHeaders: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	clientIP := "192.168.1.1:12345"

	// Use up the quota
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Next request should be rate limited
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check custom header
	if w.Header().Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected X-Custom-Header to be 'custom-value', got %s", w.Header().Get("X-Custom-Header"))
	}
}

func TestConditionalMiddleware_ConditionByPath(t *testing.T) {
	config := &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        1,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
	}

	// Only apply rate limiting to /api paths
	condition := ConditionByPath([]string{"/api/test"})
	middleware, err := NewConditionalMiddleware(config, condition)
	if err != nil {
		t.Fatalf("Failed to create conditional middleware: %v", err)
	}
	defer middleware.Stop()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	clientIP := "192.168.1.1:12345"

	// Request to non-API path should not be rate limited
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request to /health should be allowed, got status %d", w.Code)
	}

	// Request to API path should be rate limited after quota is used
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = clientIP
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request to /api/test should be allowed, got status %d", w.Code)
	}

	// Second request to API path should be rate limited
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = clientIP
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second request to /api/test should be rate limited, got status %d", w.Code)
	}
}

func TestConditionByMethod(t *testing.T) {
	condition := ConditionByMethod([]string{"POST", "PUT"})

	tests := []struct {
		method   string
		expected bool
	}{
		{"GET", false},
		{"POST", true},
		{"PUT", true},
		{"DELETE", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			result := condition(req)
			if result != tt.expected {
				t.Errorf("Expected %v for method %s, got %v", tt.expected, tt.method, result)
			}
		})
	}
}

func TestConditionByHeader(t *testing.T) {
	condition := ConditionByHeader("X-API-Key", "secret")

	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "Matching header",
			headers:  map[string]string{"X-API-Key": "secret"},
			expected: true,
		},
		{
			name:     "Non-matching header",
			headers:  map[string]string{"X-API-Key": "wrong"},
			expected: false,
		},
		{
			name:     "Missing header",
			headers:  map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := condition(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCombineConditions(t *testing.T) {
	condition1 := ConditionByMethod([]string{"POST"})
	condition2 := ConditionByPath([]string{"/api/test"})
	combined := CombineConditions(condition1, condition2)

	tests := []struct {
		name     string
		method   string
		path     string
		expected bool
	}{
		{
			name:     "Both conditions match",
			method:   "POST",
			path:     "/api/test",
			expected: true,
		},
		{
			name:     "Only method matches",
			method:   "POST",
			path:     "/other",
			expected: false,
		},
		{
			name:     "Only path matches",
			method:   "GET",
			path:     "/api/test",
			expected: false,
		},
		{
			name:     "Neither matches",
			method:   "GET",
			path:     "/other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			result := combined(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
