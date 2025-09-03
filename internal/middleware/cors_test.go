package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestCORSMiddleware_Handler(t *testing.T) {
	// Create test handler that just returns OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		config         *config.CORSConfig
		setupRequest   func() *http.Request
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "Disabled CORS middleware",
			config: &config.CORSConfig{
				Enabled: false,
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "Non-CORS request (no Origin header)",
			config: &config.CORSConfig{
				Enabled: true,
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "Simple CORS request with allow all origins",
			config: &config.CORSConfig{
				Enabled:         true,
				AllowAllOrigins: true,
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Error("Expected Access-Control-Allow-Origin to be '*'")
				}
			},
		},
		{
			name: "Simple CORS request with specific allowed origin",
			config: &config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com", "https://test.com"},
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
					t.Error("Expected Access-Control-Allow-Origin to be 'https://example.com'")
				}
			},
		},
		{
			name: "CORS request with disallowed origin",
			config: &config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://allowed.com"},
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://forbidden.com")
				return req
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "Wildcard origin matching",
			config: &config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*.example.com"},
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://api.example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Access-Control-Allow-Origin") != "https://api.example.com" {
					t.Error("Expected Access-Control-Allow-Origin to be 'https://api.example.com'")
				}
			},
		},
		{
			name: "Preflight request - allowed",
			config: &config.CORSConfig{
				Enabled:         true,
				AllowAllOrigins: true,
				AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE"},
				AllowedHeaders:  []string{"Content-Type", "Authorization"},
				MaxAge:          time.Hour,
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("OPTIONS", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
				return req
			},
			expectedStatus: http.StatusNoContent,
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Error("Expected Access-Control-Allow-Origin to be '*'")
				}
				if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
					t.Error("Expected Access-Control-Allow-Methods to contain 'POST'")
				}
				if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Content-Type") {
					t.Error("Expected Access-Control-Allow-Headers to contain 'Content-Type'")
				}
				if w.Header().Get("Access-Control-Max-Age") != "3600" {
					t.Error("Expected Access-Control-Max-Age to be '3600'")
				}
			},
		},
		{
			name: "Preflight request - disallowed method",
			config: &config.CORSConfig{
				Enabled:         true,
				AllowAllOrigins: true,
				AllowedMethods:  []string{"GET", "POST"},
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("OPTIONS", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("Access-Control-Request-Method", "DELETE")
				return req
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "CORS with credentials",
			config: &config.CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"https://example.com"},
				AllowCredentials: true,
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Error("Expected Access-Control-Allow-Credentials to be 'true'")
				}
			},
		},
		{
			name: "CORS with exposed headers",
			config: &config.CORSConfig{
				Enabled:         true,
				AllowAllOrigins: true,
				ExposedHeaders:  []string{"X-Total-Count", "X-Page-Count"},
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Origin", "https://example.com")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				exposedHeaders := w.Header().Get("Access-Control-Expose-Headers")
				if !strings.Contains(exposedHeaders, "X-Total-Count") {
					t.Error("Expected Access-Control-Expose-Headers to contain 'X-Total-Count'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware := NewCORSMiddleware(tt.config)

			// Create handler with middleware
			handler := middleware.Handler()(testHandler)

			// Create request
			req := tt.setupRequest()

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check body for successful requests
			if tt.expectedStatus == http.StatusOK && tt.expectedBody != "" {
				if rr.Body.String() != tt.expectedBody {
					t.Errorf("Expected body %q, got %q", tt.expectedBody, rr.Body.String())
				}
			}

			// Check custom headers
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, rr)
			}
		})
	}
}

func TestCORSMiddleware_OriginMatching(t *testing.T) {
	middleware := NewCORSMiddleware(&config.CORSConfig{Enabled: true})

	tests := []struct {
		origin   string
		pattern  string
		expected bool
	}{
		{"https://example.com", "https://example.com", true},
		{"https://example.com", "https://different.com", false},
		{"https://api.example.com", "*.example.com", true},
		{"https://example.com", "*.example.com", true},
		{"https://sub.api.example.com", "*.example.com", true},
		{"https://example.org", "*.example.com", false},
		{"https://notexample.com", "*.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.origin+"_vs_"+tt.pattern, func(t *testing.T) {
			result := middleware.matchOrigin(tt.origin, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchOrigin(%q, %q) = %v, expected %v", tt.origin, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestCORSMiddleware_HeadersParsing(t *testing.T) {
	middleware := NewCORSMiddleware(&config.CORSConfig{Enabled: true})

	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"Content-Type", []string{"Content-Type"}},
		{"Content-Type, Authorization", []string{"Content-Type", "Authorization"}},
		{"Content-Type,Authorization,X-Custom", []string{"Content-Type", "Authorization", "X-Custom"}},
		{" Content-Type , Authorization ", []string{"Content-Type", "Authorization"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := middleware.parseHeadersList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(result))
				return
			}
			for i, header := range result {
				if header != tt.expected[i] {
					t.Errorf("Expected header %q at index %d, got %q", tt.expected[i], i, header)
				}
			}
		})
	}
}

func TestCORSMiddleware_UpdateConfig(t *testing.T) {
	initialConfig := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://old.com"},
	}

	middleware := NewCORSMiddleware(initialConfig)

	// Test initial config
	cfg := middleware.GetConfig()
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://old.com" {
		t.Error("Initial config not set correctly")
	}

	// Update config
	newConfig := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://new.com", "https://another.com"},
	}

	middleware.UpdateConfig(newConfig)

	// Test updated config
	cfg = middleware.GetConfig()
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("Expected 2 allowed origins, got %d", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "https://new.com" {
		t.Errorf("Expected first origin to be 'https://new.com', got %q", cfg.AllowedOrigins[0])
	}
}
