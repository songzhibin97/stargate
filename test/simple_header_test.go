package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/middleware"
)

// TestHeaderTransformSimple tests the header transformation middleware with a simple setup
func TestHeaderTransformSimple(t *testing.T) {
	// Create configuration for header transformation
	cfg := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Request-Id": "test-request-123",
				"X-Gateway":    "stargate",
			},
			Remove: []string{"X-Internal-Token"},
			Rename: map[string]string{
				"User-Agent": "X-Original-User-Agent",
			},
			Replace: map[string]string{
				"Accept": "application/json",
			},
		},
		ResponseHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Processed-By": "stargate-gateway",
				"X-Version":      "1.0.0",
			},
			Remove: []string{"Server"},
			Rename: map[string]string{
				"Content-Length": "X-Content-Size",
			},
			Replace: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
			},
		},
	}

	// Create middleware
	middleware := middleware.NewHeaderTransformMiddleware(cfg)

	// Create a test handler that captures the request and sets response headers
	var capturedRequest *http.Request
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		
		// Set some response headers that will be transformed
		w.Header().Set("Server", "nginx/1.20")
		w.Header().Set("Content-Length", "100")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("Content-Type", "application/json")
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	})

	// Create the middleware chain
	handler := middleware.Handler()(testHandler)

	// Test case 1: Verify request header transformations
	t.Run("Request Header Transformations", func(t *testing.T) {
		// Create request with headers that should be transformed
		req := httptest.NewRequest("POST", "/api/test", nil)
		req.Header.Set("User-Agent", "test-client/1.0")
		req.Header.Set("Accept", "text/html") // This should be replaced
		req.Header.Set("X-Internal-Token", "secret") // This should be removed
		req.Header.Set("Content-Type", "application/json")

		// Create response recorder
		rr := httptest.NewRecorder()

		// Execute middleware
		handler.ServeHTTP(rr, req)

		// Verify added headers
		if capturedRequest.Header.Get("X-Request-Id") != "test-request-123" {
			t.Errorf("Expected X-Request-Id to be 'test-request-123', got %s", capturedRequest.Header.Get("X-Request-Id"))
		}
		if capturedRequest.Header.Get("X-Gateway") != "stargate" {
			t.Errorf("Expected X-Gateway to be 'stargate', got %s", capturedRequest.Header.Get("X-Gateway"))
		}

		// Verify removed headers
		if capturedRequest.Header.Get("X-Internal-Token") != "" {
			t.Error("X-Internal-Token should have been removed")
		}

		// Verify renamed headers
		if capturedRequest.Header.Get("User-Agent") != "" {
			t.Error("User-Agent should have been renamed")
		}
		if capturedRequest.Header.Get("X-Original-User-Agent") != "test-client/1.0" {
			t.Errorf("Expected X-Original-User-Agent to be 'test-client/1.0', got %s", capturedRequest.Header.Get("X-Original-User-Agent"))
		}

		// Verify replaced headers
		if capturedRequest.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept to be replaced with 'application/json', got %s", capturedRequest.Header.Get("Accept"))
		}
	})

	// Test case 2: Verify response header transformations
	t.Run("Response Header Transformations", func(t *testing.T) {
		// Create simple request
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Execute middleware
		handler.ServeHTTP(rr, req)

		// Verify added response headers
		if rr.Header().Get("X-Processed-By") != "stargate-gateway" {
			t.Errorf("Expected X-Processed-By to be 'stargate-gateway', got %s", rr.Header().Get("X-Processed-By"))
		}
		if rr.Header().Get("X-Version") != "1.0.0" {
			t.Errorf("Expected X-Version to be '1.0.0', got %s", rr.Header().Get("X-Version"))
		}

		// Verify removed response headers
		if rr.Header().Get("Server") != "" {
			t.Error("Server header should have been removed")
		}

		// Verify renamed response headers
		if rr.Header().Get("Content-Length") != "" {
			t.Error("Content-Length should have been renamed")
		}
		if rr.Header().Get("X-Content-Size") == "" {
			t.Error("X-Content-Size should be present (renamed from Content-Length)")
		}

		// Verify replaced response headers
		if rr.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
			t.Errorf("Expected Cache-Control to be replaced, got %s", rr.Header().Get("Cache-Control"))
		}
	})
}

// TestHeaderTransformWithDynamicValues tests dynamic value expansion
func TestHeaderTransformWithDynamicValues(t *testing.T) {
	cfg := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Method": "${method}",
				"X-Path":   "${path}",
				"X-Host":   "${host}",
			},
		},
	}

	middleware := middleware.NewHeaderTransformMiddleware(cfg)

	var capturedRequest *http.Request
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Create request with specific method, path, and host
	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()

	// Execute middleware
	handler.ServeHTTP(rr, req)

	// Verify dynamic values were expanded
	if capturedRequest.Header.Get("X-Method") != "POST" {
		t.Errorf("Expected X-Method to be POST, got %s", capturedRequest.Header.Get("X-Method"))
	}
	if capturedRequest.Header.Get("X-Path") != "/api/users" {
		t.Errorf("Expected X-Path to be /api/users, got %s", capturedRequest.Header.Get("X-Path"))
	}
	if capturedRequest.Header.Get("X-Host") != "example.com" {
		t.Errorf("Expected X-Host to be example.com, got %s", capturedRequest.Header.Get("X-Host"))
	}
}

// TestHeaderTransformDisabled tests that middleware is bypassed when disabled
func TestHeaderTransformDisabled(t *testing.T) {
	cfg := &config.HeaderTransformConfig{
		Enabled: false, // Disabled
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Should-Not-Add": "should-not-be-added",
			},
		},
	}

	middleware := middleware.NewHeaderTransformMiddleware(cfg)

	var capturedRequest *http.Request
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Original-Header", "original-value")
	rr := httptest.NewRecorder()

	// Execute middleware
	handler.ServeHTTP(rr, req)

	// Verify original header is preserved
	if capturedRequest.Header.Get("Original-Header") != "original-value" {
		t.Error("Original header should be preserved when middleware is disabled")
	}

	// Verify no headers were added
	if capturedRequest.Header.Get("X-Should-Not-Add") != "" {
		t.Error("No headers should be added when middleware is disabled")
	}
}

// TestHeaderTransformStats tests statistics collection
func TestHeaderTransformStats(t *testing.T) {
	cfg := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Added": "value",
			},
			Remove: []string{"X-Remove"},
		},
		ResponseHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Response-Added": "response-value",
			},
		},
	}

	middleware := middleware.NewHeaderTransformMiddleware(cfg)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Remove", "should-be-removed")
	rr := httptest.NewRecorder()

	// Execute middleware
	handler.ServeHTTP(rr, req)

	// Check statistics
	stats := middleware.GetStats()
	if stats.RequestsProcessed != 1 {
		t.Errorf("Expected RequestsProcessed to be 1, got %d", stats.RequestsProcessed)
	}
	if stats.RequestHeadersAdded != 1 {
		t.Errorf("Expected RequestHeadersAdded to be 1, got %d", stats.RequestHeadersAdded)
	}
	if stats.RequestHeadersRemoved != 1 {
		t.Errorf("Expected RequestHeadersRemoved to be 1, got %d", stats.RequestHeadersRemoved)
	}
	if stats.ResponseHeadersAdded != 1 {
		t.Errorf("Expected ResponseHeadersAdded to be 1, got %d", stats.ResponseHeadersAdded)
	}
}
