package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestHeaderTransformMiddleware_RequestHeaders(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.HeaderTransformConfig
		requestHeaders map[string]string
		expectedHeaders map[string]string
		expectedRemoved []string
	}{
		{
			name: "Add request headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Add: map[string]string{
						"X-Request-ID": "test-123",
						"X-Custom":     "custom-value",
					},
				},
			},
			requestHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Request-ID": "test-123",
				"X-Custom":     "custom-value",
			},
		},
		{
			name: "Remove request headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Remove: []string{"X-Remove-Me", "Authorization"},
				},
			},
			requestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"X-Remove-Me":   "should-be-removed",
				"Authorization": "Bearer token",
				"X-Keep-Me":     "should-remain",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Keep-Me":    "should-remain",
			},
			expectedRemoved: []string{"X-Remove-Me", "Authorization"},
		},
		{
			name: "Rename request headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Rename: map[string]string{
						"X-Old-Name": "X-New-Name",
						"User-Agent": "X-User-Agent",
					},
				},
			},
			requestHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Old-Name":   "old-value",
				"User-Agent":   "test-agent",
			},
			expectedHeaders: map[string]string{
				"Content-Type":   "application/json",
				"X-New-Name":     "old-value",
				"X-User-Agent":   "test-agent",
			},
			expectedRemoved: []string{"X-Old-Name", "User-Agent"},
		},
		{
			name: "Replace request headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Replace: map[string]string{
						"Content-Type": "application/xml",
						"X-Version":    "v2.0",
					},
				},
			},
			requestHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Version":    "v1.0",
				"X-Keep":       "unchanged",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/xml",
				"X-Version":    "v2.0",
				"X-Keep":       "unchanged",
			},
		},
		{
			name: "Combined operations",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Add: map[string]string{
						"X-Added": "added-value",
					},
					Remove: []string{"X-Remove"},
					Rename: map[string]string{
						"X-Old": "X-New",
					},
					Replace: map[string]string{
						"Content-Type": "application/xml",
					},
				},
			},
			requestHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Old":        "old-value",
				"X-Remove":     "remove-me",
				"X-Keep":       "keep-me",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/xml",
				"X-New":        "old-value",
				"X-Keep":       "keep-me",
				"X-Added":      "added-value",
			},
			expectedRemoved: []string{"X-Remove", "X-Old"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewHeaderTransformMiddleware(tt.config)

			// Create test handler that captures the request
			var capturedRequest *http.Request
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(http.StatusOK)
			})

			// Create request with test headers
			req := httptest.NewRequest("GET", "/test", nil)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute middleware
			middlewareHandler := middleware.Handler()(handler)
			middlewareHandler.ServeHTTP(rr, req)

			// Verify expected headers are present
			for key, expectedValue := range tt.expectedHeaders {
				actualValue := capturedRequest.Header.Get(key)
				if actualValue != expectedValue {
					t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Verify removed headers are not present
			for _, removedHeader := range tt.expectedRemoved {
				if capturedRequest.Header.Get(removedHeader) != "" {
					t.Errorf("Expected header %s to be removed, but it's still present", removedHeader)
				}
			}
		})
	}
}

func TestHeaderTransformMiddleware_ResponseHeaders(t *testing.T) {
	tests := []struct {
		name            string
		config          *config.HeaderTransformConfig
		responseHeaders map[string]string
		expectedHeaders map[string]string
		expectedRemoved []string
	}{
		{
			name: "Add response headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				ResponseHeaders: config.HeaderTransformRules{
					Add: map[string]string{
						"X-Response-ID": "resp-123",
						"X-Server":      "stargate",
					},
				},
			},
			responseHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedHeaders: map[string]string{
				"Content-Type":   "application/json",
				"X-Response-ID":  "resp-123",
				"X-Server":       "stargate",
			},
		},
		{
			name: "Remove response headers",
			config: &config.HeaderTransformConfig{
				Enabled: true,
				ResponseHeaders: config.HeaderTransformRules{
					Remove: []string{"Server", "X-Powered-By"},
				},
			},
			responseHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Server":        "nginx",
				"X-Powered-By":  "PHP",
				"X-Keep":        "keep-me",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Keep":       "keep-me",
			},
			expectedRemoved: []string{"Server", "X-Powered-By"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewHeaderTransformMiddleware(tt.config)

			// Create test handler that sets response headers
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.responseHeaders {
					w.Header().Set(key, value)
				}
				w.WriteHeader(http.StatusOK)
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute middleware
			middlewareHandler := middleware.Handler()(handler)
			middlewareHandler.ServeHTTP(rr, req)

			// Verify expected headers are present
			for key, expectedValue := range tt.expectedHeaders {
				actualValue := rr.Header().Get(key)
				if actualValue != expectedValue {
					t.Errorf("Expected response header %s to be %s, got %s", key, expectedValue, actualValue)
				}
			}

			// Verify removed headers are not present
			for _, removedHeader := range tt.expectedRemoved {
				if rr.Header().Get(removedHeader) != "" {
					t.Errorf("Expected response header %s to be removed, but it's still present", removedHeader)
				}
			}
		})
	}
}

func TestHeaderTransformMiddleware_Disabled(t *testing.T) {
	config := &config.HeaderTransformConfig{
		Enabled: false,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Should-Not-Add": "should-not-be-added",
			},
		},
	}

	middleware := NewHeaderTransformMiddleware(config)

	// Create test handler
	var capturedRequest *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Original-Header", "original-value")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware
	middlewareHandler := middleware.Handler()(handler)
	middlewareHandler.ServeHTTP(rr, req)

	// Verify original header is preserved
	if capturedRequest.Header.Get("Original-Header") != "original-value" {
		t.Error("Original header should be preserved when middleware is disabled")
	}

	// Verify no headers were added
	if capturedRequest.Header.Get("X-Should-Not-Add") != "" {
		t.Error("No headers should be added when middleware is disabled")
	}
}

func TestHeaderTransformMiddleware_DynamicValues(t *testing.T) {
	config := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Method": "${method}",
				"X-Path":   "${path}",
				"X-Host":   "${host}",
			},
		},
	}

	middleware := NewHeaderTransformMiddleware(config)

	// Create test handler
	var capturedRequest *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	// Create request
	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Host = "example.com"

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware
	middlewareHandler := middleware.Handler()(handler)
	middlewareHandler.ServeHTTP(rr, req)

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

func TestHeaderTransformMiddleware_PerRouteConfig(t *testing.T) {
	config := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Global": "global-value",
			},
		},
		PerRoute: map[string]config.HeaderTransformRule{
			"api-route": {
				Enabled: true,
				RequestHeaders: config.HeaderTransformRules{
					Add: map[string]string{
						"X-Route-Specific": "route-value",
					},
				},
			},
		},
	}

	middleware := NewHeaderTransformMiddleware(config)

	// Create test handler
	var capturedRequest *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	// Create request with route context
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx := req.Context()
	ctx = context.WithValue(ctx, "route_id", "api-route")
	req = req.WithContext(ctx)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware
	middlewareHandler := middleware.Handler()(handler)
	middlewareHandler.ServeHTTP(rr, req)

	// Verify route-specific header was added
	if capturedRequest.Header.Get("X-Route-Specific") != "route-value" {
		t.Errorf("Expected X-Route-Specific to be route-value, got %s", capturedRequest.Header.Get("X-Route-Specific"))
	}

	// Global config should not apply when per-route config exists
	if capturedRequest.Header.Get("X-Global") != "" {
		t.Error("Global config should not apply when per-route config exists")
	}
}

func TestHeaderTransformMiddleware_Stats(t *testing.T) {
	config := &config.HeaderTransformConfig{
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

	middleware := NewHeaderTransformMiddleware(config)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Remove", "should-be-removed")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware
	middlewareHandler := middleware.Handler()(handler)
	middlewareHandler.ServeHTTP(rr, req)

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

func TestHeaderTransformMiddleware_HeaderExpansion(t *testing.T) {
	config := &config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Original-User-Agent": "${header:User-Agent}",
				"X-Timestamp":           "${timestamp}",
			},
		},
	}

	middleware := NewHeaderTransformMiddleware(config)

	// Create test handler
	var capturedRequest *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	})

	// Create request with User-Agent header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware
	middlewareHandler := middleware.Handler()(handler)
	middlewareHandler.ServeHTTP(rr, req)

	// Verify header expansion worked
	if capturedRequest.Header.Get("X-Original-User-Agent") != "test-agent/1.0" {
		t.Errorf("Expected X-Original-User-Agent to be test-agent/1.0, got %s", capturedRequest.Header.Get("X-Original-User-Agent"))
	}

	// Verify timestamp was added (just check it's not empty)
	if capturedRequest.Header.Get("X-Timestamp") == "" {
		t.Error("Expected X-Timestamp to be set")
	}
}
