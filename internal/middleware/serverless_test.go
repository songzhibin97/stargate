package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestServerlessMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.ServerlessConfig
		requestPath    string
		requestMethod  string
		requestBody    string
		expectedStatus int
		expectProcessed bool
	}{
		{
			name: "disabled middleware should pass through",
			config: &config.ServerlessConfig{
				Enabled: false,
			},
			requestPath:     "/test",
			requestMethod:   "GET",
			expectedStatus:  http.StatusOK,
			expectProcessed: false,
		},
		{
			name: "no matching rule should pass through",
			config: &config.ServerlessConfig{
				Enabled: true,
				Rules: []config.ServerlessRule{
					{
						ID:     "test-rule",
						Path:   "/api/transform",
						Method: "POST",
					},
				},
			},
			requestPath:     "/other/path",
			requestMethod:   "GET",
			expectedStatus:  http.StatusOK,
			expectProcessed: false,
		},
		{
			name: "matching rule should be processed",
			config: &config.ServerlessConfig{
				Enabled:        true,
				DefaultTimeout: 10 * time.Second,
				Rules: []config.ServerlessRule{
					{
						ID:     "test-rule",
						Path:   "/api/transform",
						Method: "POST",
						PreProcess: []config.ServerlessFunction{
							{
								ID:      "pre-func",
								Name:    "Pre Function",
								URL:     "http://example.com/pre",
								Method:  "POST",
								OnError: "continue",
							},
						},
					},
				},
			},
			requestPath:     "/api/transform",
			requestMethod:   "POST",
			requestBody:     `{"test": "data"}`,
			expectedStatus:  http.StatusOK,
			expectProcessed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewServerlessMiddleware(tt.config)
			
			// Create a mock next handler
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("next handler"))
			})

			// Create handler with middleware
			handler := middleware.Handler()(nextHandler)

			// Create test request
			var req *http.Request
			if tt.requestBody != "" {
				req = httptest.NewRequest(tt.requestMethod, tt.requestPath, strings.NewReader(tt.requestBody))
			} else {
				req = httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			}
			if tt.requestBody != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify if next handler was called
			if !nextCalled {
				t.Error("next handler was not called")
			}
		})
	}
}

func TestServerlessMiddleware_matchRule(t *testing.T) {
	config := &config.ServerlessConfig{
		Enabled: true,
		Rules: []config.ServerlessRule{
			{
				ID:     "exact-match",
				Path:   "/api/users",
				Method: "GET",
			},
			{
				ID:     "any-method",
				Path:   "/api/posts",
				Method: "", // Empty method matches all
			},
			{
				ID:     "with-headers",
				Path:   "/api/secure",
				Method: "POST",
				Headers: map[string]string{
					"X-API-Key": "secret",
				},
			},
		},
	}

	middleware := NewServerlessMiddleware(config)

	tests := []struct {
		name           string
		requestPath    string
		requestMethod  string
		requestHeaders map[string]string
		expectedRuleID string
	}{
		{
			name:           "exact path and method match",
			requestPath:    "/api/users",
			requestMethod:  "GET",
			expectedRuleID: "exact-match",
		},
		{
			name:           "path match with any method",
			requestPath:    "/api/posts",
			requestMethod:  "PUT",
			expectedRuleID: "any-method",
		},
		{
			name:          "method mismatch",
			requestPath:   "/api/users",
			requestMethod: "POST",
			expectedRuleID: "",
		},
		{
			name:          "path mismatch",
			requestPath:   "/api/unknown",
			requestMethod: "GET",
			expectedRuleID: "",
		},
		{
			name:          "header match",
			requestPath:   "/api/secure",
			requestMethod: "POST",
			requestHeaders: map[string]string{
				"X-API-Key": "secret",
			},
			expectedRuleID: "with-headers",
		},
		{
			name:          "header mismatch",
			requestPath:   "/api/secure",
			requestMethod: "POST",
			requestHeaders: map[string]string{
				"X-API-Key": "wrong",
			},
			expectedRuleID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}
			
			rule := middleware.matchRule(req)

			if tt.expectedRuleID == "" {
				if rule != nil {
					t.Errorf("expected no match, but got rule %s", rule.ID)
				}
			} else {
				if rule == nil {
					t.Errorf("expected rule %s, but got no match", tt.expectedRuleID)
				} else if rule.ID != tt.expectedRuleID {
					t.Errorf("expected rule %s, got %s", tt.expectedRuleID, rule.ID)
				}
			}
		})
	}
}

func TestServerlessMiddleware_callServerlessFunction(t *testing.T) {
	// Create a test server that returns different responses
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			response := FunctionResponse{
				Body:   `{"transformed": true}`,
				Status: 200,
				Headers: map[string]string{
					"X-Transformed": "true",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		case "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	config := &config.ServerlessConfig{
		Enabled:        true,
		DefaultTimeout: 1 * time.Second,
	}
	middleware := NewServerlessMiddleware(config)

	tests := []struct {
		name         string
		function     ServerlessFunction
		expectError  bool
		expectedBody string
	}{
		{
			name: "successful function call",
			function: ServerlessFunction{
				ID:      "success-func",
				Name:    "Success Function",
				URL:     testServer.URL + "/success",
				Method:  "POST",
				Timeout: 5 * time.Second,
			},
			expectError:  false,
			expectedBody: `{"transformed": true}`,
		},
		{
			name: "function returns error",
			function: ServerlessFunction{
				ID:      "error-func",
				Name:    "Error Function",
				URL:     testServer.URL + "/error",
				Method:  "POST",
				Timeout: 5 * time.Second,
			},
			expectError: true,
		},
		{
			name: "function timeout",
			function: ServerlessFunction{
				ID:      "timeout-func",
				Name:    "Timeout Function",
				URL:     testServer.URL + "/timeout",
				Method:  "POST",
				Timeout: 500 * time.Millisecond,
			},
			expectError: true,
		},
		{
			name: "invalid URL",
			function: ServerlessFunction{
				ID:     "invalid-func",
				Name:   "Invalid Function",
				URL:    "invalid-url",
				Method: "POST",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test": "data"}`))
			req.Header.Set("Content-Type", "application/json")

			response, err := middleware.callServerlessFunction(req, tt.function, `{"test": "data"}`)

			if tt.expectError && err == nil {
				t.Error("expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && response != nil && response.Body != tt.expectedBody {
				t.Errorf("expected body %s, got %s", tt.expectedBody, response.Body)
			}
		})
	}
}

func TestServerlessMiddleware_executePreProcessFunctions(t *testing.T) {
	// Create a test server that modifies request body
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := FunctionResponse{
			Body: `{"modified": true, "original": "preserved"}`,
			Headers: map[string]string{
				"X-Modified": "true",
			},
			Status: 200,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	config := &config.ServerlessConfig{
		Enabled:        true,
		DefaultTimeout: 5 * time.Second,
	}
	middleware := NewServerlessMiddleware(config)

	rule := &ServerlessRule{
		ID:     "test-rule",
		Path:   "/api/transform",
		Method: "POST",
		PreProcess: []ServerlessFunction{
			{
				ID:      "transform-func",
				Name:    "Transform Function",
				URL:     testServer.URL,
				Method:  "POST",
				OnError: "continue",
			},
		},
	}

	originalBody := `{"original": "data"}`
	req := httptest.NewRequest("POST", "/api/transform", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")

	modifiedReq, err := middleware.executePreProcessFunctions(req, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read modified request body
	buf := new(bytes.Buffer)
	buf.ReadFrom(modifiedReq.Body)
	modifiedBody := buf.String()

	expectedBody := `{"modified": true, "original": "preserved"}`
	if modifiedBody != expectedBody {
		t.Errorf("expected modified body %s, got %s", expectedBody, modifiedBody)
	}

	// Check modified headers
	if modifiedReq.Header.Get("X-Modified") != "true" {
		t.Error("expected X-Modified header to be set")
	}
}

func TestServerlessMiddleware_GetStats(t *testing.T) {
	config := &config.ServerlessConfig{
		Enabled: true,
	}
	middleware := NewServerlessMiddleware(config)

	// Update some statistics
	middleware.updateTotalRequests()
	middleware.updateTotalRequests()
	middleware.updatePreProcessRequests()
	middleware.updatePostProcessRequests()
	middleware.updateFailedRequests()

	stats := middleware.GetStats()

	expectedStats := map[string]interface{}{
		"total_requests":        int64(2),
		"pre_process_requests":  int64(1),
		"post_process_requests": int64(1),
		"failed_requests":       int64(1),
		"success_rate":          float64(50), // (2-1)/2 * 100 = 50
	}

	for key, expected := range expectedStats {
		actual, exists := stats[key]
		if !exists {
			t.Errorf("expected stat %s not found", key)
			continue
		}
		if actual != expected {
			t.Errorf("for stat %s, expected %v, got %v", key, expected, actual)
		}
	}
}

func TestServerlessResponseWrapper(t *testing.T) {
	// Create a response recorder
	recorder := httptest.NewRecorder()
	
	// Create wrapper
	wrapper := &serverlessResponseWrapper{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
		body:          &bytes.Buffer{},
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusCreated)
	if wrapper.statusCode != http.StatusCreated {
		t.Errorf("expected status code %d, got %d", http.StatusCreated, wrapper.statusCode)
	}

	// Test Write
	testData := []byte("test response data")
	n, err := wrapper.Write(testData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Check that data was written to both the original response and buffer
	if wrapper.body.String() != string(testData) {
		t.Errorf("expected buffer to contain %s, got %s", string(testData), wrapper.body.String())
	}
	if recorder.Body.String() != string(testData) {
		t.Errorf("expected recorder to contain %s, got %s", string(testData), recorder.Body.String())
	}
}
