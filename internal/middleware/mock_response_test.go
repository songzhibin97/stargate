package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestMockResponseMiddleware_BasicMocking(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.MockResponseConfig
		requestMethod  string
		requestPath    string
		requestHeaders map[string]string
		expectedStatus int
		expectedBody   string
		expectedHeaders map[string]string
		shouldMock     bool
	}{
		{
			name: "Simple path match",
			config: &config.MockResponseConfig{
				Enabled: true,
				Rules: []config.MockRule{
					{
						ID:      "test-rule-1",
						Name:    "Test Rule 1",
						Enabled: true,
						Priority: 100,
						Conditions: config.MockConditions{
							Paths: []config.MockPathMatcher{
								{Type: "exact", Value: "/api/test"},
							},
						},
						Response: config.MockResponse{
							StatusCode: 200,
							Headers: map[string]string{
								"Content-Type": "application/json",
								"X-Mock":       "true",
							},
							Body: `{"message": "mocked response"}`,
						},
					},
				},
			},
			requestMethod:  "GET",
			requestPath:    "/api/test",
			expectedStatus: 200,
			expectedBody:   `{"message": "mocked response"}`,
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"X-Mock":       "true",
			},
			shouldMock: true,
		},
		{
			name: "Method and path match",
			config: &config.MockResponseConfig{
				Enabled: true,
				Rules: []config.MockRule{
					{
						ID:      "test-rule-2",
						Name:    "Test Rule 2",
						Enabled: true,
						Priority: 100,
						Conditions: config.MockConditions{
							Methods: []string{"POST"},
							Paths: []config.MockPathMatcher{
								{Type: "prefix", Value: "/api/"},
							},
						},
						Response: config.MockResponse{
							StatusCode: 201,
							Body:       `{"created": true}`,
						},
					},
				},
			},
			requestMethod:  "POST",
			requestPath:    "/api/users",
			expectedStatus: 201,
			expectedBody:   `{"created": true}`,
			shouldMock:     true,
		},
		{
			name: "No match - different method",
			config: &config.MockResponseConfig{
				Enabled: true,
				Rules: []config.MockRule{
					{
						ID:      "test-rule-3",
						Name:    "Test Rule 3",
						Enabled: true,
						Priority: 100,
						Conditions: config.MockConditions{
							Methods: []string{"POST"},
							Paths: []config.MockPathMatcher{
								{Type: "exact", Value: "/api/test"},
							},
						},
						Response: config.MockResponse{
							StatusCode: 200,
							Body:       `{"mocked": true}`,
						},
					},
				},
			},
			requestMethod: "GET",
			requestPath:   "/api/test",
			shouldMock:    false,
		},
		{
			name: "Header matching",
			config: &config.MockResponseConfig{
				Enabled: true,
				Rules: []config.MockRule{
					{
						ID:      "test-rule-4",
						Name:    "Test Rule 4",
						Enabled: true,
						Priority: 100,
						Conditions: config.MockConditions{
							Paths: []config.MockPathMatcher{
								{Type: "exact", Value: "/api/test"},
							},
							Headers: map[string]string{
								"X-Test-Header": "test-value",
							},
						},
						Response: config.MockResponse{
							StatusCode: 200,
							Body:       `{"header_matched": true}`,
						},
					},
				},
			},
			requestMethod: "GET",
			requestPath:   "/api/test",
			requestHeaders: map[string]string{
				"X-Test-Header": "test-value",
			},
			expectedStatus: 200,
			expectedBody:   `{"header_matched": true}`,
			shouldMock:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware, err := NewMockResponseMiddleware(tt.config)
			if err != nil {
				t.Fatalf("Failed to create middleware: %v", err)
			}

			// Create test handler that should not be called if mock is served
			var handlerCalled bool
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("original response"))
			})

			// Create middleware chain
			handler := middleware.Handler()(testHandler)

			// Create request
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute middleware
			handler.ServeHTTP(rr, req)

			if tt.shouldMock {
				// Verify mock response was served
				if handlerCalled {
					t.Error("Handler should not have been called when mock is served")
				}

				if rr.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
				}

				if body := rr.Body.String(); body != tt.expectedBody {
					t.Errorf("Expected body %s, got %s", tt.expectedBody, body)
				}

				for key, expectedValue := range tt.expectedHeaders {
					if actualValue := rr.Header().Get(key); actualValue != expectedValue {
						t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, actualValue)
					}
				}
			} else {
				// Verify original handler was called
				if !handlerCalled {
					t.Error("Handler should have been called when no mock matches")
				}

				if body := rr.Body.String(); body != "original response" {
					t.Errorf("Expected original response, got %s", body)
				}
			}
		})
	}
}

func TestMockResponseMiddleware_PathMatching(t *testing.T) {
	tests := []struct {
		name        string
		pathMatcher config.MockPathMatcher
		requestPath string
		shouldMatch bool
	}{
		{
			name:        "Exact match - success",
			pathMatcher: config.MockPathMatcher{Type: "exact", Value: "/api/users"},
			requestPath: "/api/users",
			shouldMatch: true,
		},
		{
			name:        "Exact match - failure",
			pathMatcher: config.MockPathMatcher{Type: "exact", Value: "/api/users"},
			requestPath: "/api/users/123",
			shouldMatch: false,
		},
		{
			name:        "Prefix match - success",
			pathMatcher: config.MockPathMatcher{Type: "prefix", Value: "/api/"},
			requestPath: "/api/users/123",
			shouldMatch: true,
		},
		{
			name:        "Prefix match - failure",
			pathMatcher: config.MockPathMatcher{Type: "prefix", Value: "/api/"},
			requestPath: "/v1/users",
			shouldMatch: false,
		},
		{
			name:        "Regex match - success",
			pathMatcher: config.MockPathMatcher{Type: "regex", Value: "^/api/users/\\d+$"},
			requestPath: "/api/users/123",
			shouldMatch: true,
		},
		{
			name:        "Regex match - failure",
			pathMatcher: config.MockPathMatcher{Type: "regex", Value: "^/api/users/\\d+$"},
			requestPath: "/api/users/abc",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.MockResponseConfig{
				Enabled: true,
				Rules: []config.MockRule{
					{
						ID:      "path-test",
						Name:    "Path Test",
						Enabled: true,
						Priority: 100,
						Conditions: config.MockConditions{
							Paths: []config.MockPathMatcher{tt.pathMatcher},
						},
						Response: config.MockResponse{
							StatusCode: 200,
							Body:       `{"matched": true}`,
						},
					},
				},
			}

			middleware, err := NewMockResponseMiddleware(config)
			if err != nil {
				t.Fatalf("Failed to create middleware: %v", err)
			}

			var handlerCalled bool
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.Handler()(testHandler)
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tt.shouldMatch {
				if handlerCalled {
					t.Error("Handler should not have been called when path matches")
				}
				if rr.Body.String() != `{"matched": true}` {
					t.Error("Expected mock response body")
				}
			} else {
				if !handlerCalled {
					t.Error("Handler should have been called when path doesn't match")
				}
			}
		})
	}
}

func TestMockResponseMiddleware_QueryParamMatching(t *testing.T) {
	config := &config.MockResponseConfig{
		Enabled: true,
		Rules: []config.MockRule{
			{
				ID:      "query-test",
				Name:    "Query Test",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/api/test"},
					},
					QueryParams: map[string]string{
						"version": "v1",
						"format":  "json",
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"query_matched": true}`,
				},
			},
		},
	}

	middleware, err := NewMockResponseMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	tests := []struct {
		name        string
		requestURL  string
		shouldMatch bool
	}{
		{
			name:        "All query params match",
			requestURL:  "/api/test?version=v1&format=json",
			shouldMatch: true,
		},
		{
			name:        "Missing query param",
			requestURL:  "/api/test?version=v1",
			shouldMatch: false,
		},
		{
			name:        "Wrong query param value",
			requestURL:  "/api/test?version=v2&format=json",
			shouldMatch: false,
		},
		{
			name:        "Extra query params (should still match)",
			requestURL:  "/api/test?version=v1&format=json&extra=value",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handlerCalled bool
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.Handler()(testHandler)
			req := httptest.NewRequest("GET", tt.requestURL, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tt.shouldMatch {
				if handlerCalled {
					t.Error("Handler should not have been called when query params match")
				}
				if rr.Body.String() != `{"query_matched": true}` {
					t.Error("Expected mock response body")
				}
			} else {
				if !handlerCalled {
					t.Error("Handler should have been called when query params don't match")
				}
			}
		})
	}
}

func TestMockResponseMiddleware_DynamicValues(t *testing.T) {
	config := &config.MockResponseConfig{
		Enabled: true,
		Rules: []config.MockRule{
			{
				ID:      "dynamic-test",
				Name:    "Dynamic Test",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/api/dynamic"},
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"method": "${method}", "path": "${path}", "host": "${host}", "user_agent": "${header:User-Agent}", "version": "${query:version}"}`,
				},
			},
		},
	}

	middleware, err := NewMockResponseMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not have been called")
	})

	handler := middleware.Handler()(testHandler)
	req := httptest.NewRequest("POST", "/api/dynamic?version=v2", nil)
	req.Host = "example.com"
	req.Header.Set("User-Agent", "test-agent/1.0")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	expectedBody := `{"method": "POST", "path": "/api/dynamic", "host": "example.com", "user_agent": "test-agent/1.0", "version": "v2"}`
	if rr.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, rr.Body.String())
	}
}

func TestMockResponseMiddleware_Disabled(t *testing.T) {
	config := &config.MockResponseConfig{
		Enabled: false, // Disabled
		Rules: []config.MockRule{
			{
				ID:      "disabled-rule",
				Name:    "Disabled Rule",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/api/test"},
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"should": "not_appear"}`,
				},
			},
		},
	}

	middleware, err := NewMockResponseMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	handler := middleware.Handler()(testHandler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should have been called when middleware is disabled")
	}

	if rr.Body.String() != "original response" {
		t.Error("Expected original response when middleware is disabled")
	}
}
