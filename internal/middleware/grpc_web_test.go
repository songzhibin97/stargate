package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestGRPCWebMiddleware_IsGRPCWebRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "gRPC-Web binary",
			contentType: "application/grpc-web",
			expected:    true,
		},
		{
			name:        "gRPC-Web proto",
			contentType: "application/grpc-web+proto",
			expected:    true,
		},
		{
			name:        "gRPC-Web text",
			contentType: "application/grpc-web-text",
			expected:    true,
		},
		{
			name:        "gRPC-Web text proto",
			contentType: "application/grpc-web-text+proto",
			expected:    true,
		},
		{
			name:        "Regular JSON",
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "Regular gRPC",
			contentType: "application/grpc",
			expected:    false,
		},
		{
			name:        "Empty content type",
			contentType: "",
			expected:    false,
		},
	}

	config := &config.GRPCWebConfig{
		Enabled: true,
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test.Service/Method", nil)
			req.Header.Set("Content-Type", tt.contentType)

			result := middleware.isGRPCWebRequest(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGRPCWebMiddleware_CORSPreflight(t *testing.T) {
	config := &config.GRPCWebConfig{
		Enabled: true,
		CORS: config.GRPCWebCORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"https://example.com", "https://test.com"},
			AllowedMethods: []string{"POST", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "X-Grpc-Web"},
			MaxAge:         3600,
		},
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "Allowed origin",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "Disallowed origin",
			origin:         "https://malicious.com",
			expectedStatus: http.StatusForbidden,
			checkHeaders:   false,
		},
		{
			name:           "No origin",
			origin:         "",
			expectedStatus: http.StatusForbidden,
			checkHeaders:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("OPTIONS", "/test.Service/Method", nil)
			req.Header.Set("Content-Type", "application/grpc-web")
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rr := httptest.NewRecorder()
			middleware.handleCORSPreflight(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.checkHeaders {
				if rr.Header().Get("Access-Control-Allow-Origin") != tt.origin {
					t.Errorf("Expected Access-Control-Allow-Origin to be %s, got %s", 
						tt.origin, rr.Header().Get("Access-Control-Allow-Origin"))
				}
				if rr.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Error("Expected Access-Control-Allow-Methods to be set")
				}
				if rr.Header().Get("Access-Control-Allow-Headers") == "" {
					t.Error("Expected Access-Control-Allow-Headers to be set")
				}
			}
		})
	}
}

func TestGRPCWebMiddleware_ParseGRPCWebRequest(t *testing.T) {
	config := &config.GRPCWebConfig{
		Enabled:  true,
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	tests := []struct {
		name        string
		path        string
		contentType string
		body        string
		expectError bool
		expectedSvc string
		expectedMtd string
		expectedTxt bool
	}{
		{
			name:        "Valid binary request",
			path:        "/test.Service/GetUser",
			contentType: "application/grpc-web+proto",
			body:        "test message",
			expectError: false,
			expectedSvc: "test.Service",
			expectedMtd: "GetUser",
			expectedTxt: false,
		},
		{
			name:        "Valid text request",
			path:        "/user.UserService/CreateUser",
			contentType: "application/grpc-web-text+proto",
			body:        "dGVzdCBtZXNzYWdl", // base64 encoded "test message"
			expectError: false,
			expectedSvc: "user.UserService",
			expectedMtd: "CreateUser",
			expectedTxt: true,
		},
		{
			name:        "Invalid path format",
			path:        "/invalid",
			contentType: "application/grpc-web",
			body:        "test",
			expectError: true,
		},
		{
			name:        "Empty path",
			path:        "/",
			contentType: "application/grpc-web",
			body:        "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("X-User-Agent", "grpc-web-javascript/0.1")

			result, err := middleware.parseGRPCWebRequest(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Service != tt.expectedSvc {
				t.Errorf("Expected service %s, got %s", tt.expectedSvc, result.Service)
			}

			if result.Method != tt.expectedMtd {
				t.Errorf("Expected method %s, got %s", tt.expectedMtd, result.Method)
			}

			if result.IsText != tt.expectedTxt {
				t.Errorf("Expected IsText %v, got %v", tt.expectedTxt, result.IsText)
			}

			if result.ContentType != tt.contentType {
				t.Errorf("Expected content type %s, got %s", tt.contentType, result.ContentType)
			}
		})
	}
}

func TestGRPCWebMiddleware_Disabled(t *testing.T) {
	config := &config.GRPCWebConfig{
		Enabled:  false, // Disabled
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	handler := middleware.Handler()(testHandler)

	req := httptest.NewRequest("POST", "/test.Service/Method", strings.NewReader("test"))
	req.Header.Set("Content-Type", "application/grpc-web")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should have been called when middleware is disabled")
	}

	if rr.Body.String() != "original response" {
		t.Error("Expected original response when middleware is disabled")
	}
}

func TestGRPCWebMiddleware_NonGRPCWebRequest(t *testing.T) {
	config := &config.GRPCWebConfig{
		Enabled:  true,
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	handler := middleware.Handler()(testHandler)

	req := httptest.NewRequest("POST", "/api/users", strings.NewReader(`{"name": "test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should have been called for non-gRPC-Web requests")
	}

	if rr.Body.String() != "original response" {
		t.Error("Expected original response for non-gRPC-Web requests")
	}
}

func TestGRPCWebMiddleware_Stats(t *testing.T) {
	config := &config.GRPCWebConfig{
		Enabled:  true,
		Services: make(map[string]config.GRPCServiceConfig),
	}

	middleware, err := NewGRPCWebMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	// Update some statistics
	middleware.updateProcessedStats()
	middleware.updateSuccessStats("test.Service")
	middleware.updateFailedStats("test.Service")
	middleware.updateBytesSent(1024)

	stats := middleware.GetStats()

	if stats.RequestsProcessed != 1 {
		t.Errorf("Expected RequestsProcessed to be 1, got %d", stats.RequestsProcessed)
	}

	if stats.GRPCCallsSucceeded != 1 {
		t.Errorf("Expected GRPCCallsSucceeded to be 1, got %d", stats.GRPCCallsSucceeded)
	}

	if stats.GRPCCallsFailed != 1 {
		t.Errorf("Expected GRPCCallsFailed to be 1, got %d", stats.GRPCCallsFailed)
	}

	if stats.BytesSent != 1024 {
		t.Errorf("Expected BytesSent to be 1024, got %d", stats.BytesSent)
	}

	if stats.ServiceCalls["test.Service"] != 1 {
		t.Errorf("Expected ServiceCalls for test.Service to be 1, got %d", stats.ServiceCalls["test.Service"])
	}
}
