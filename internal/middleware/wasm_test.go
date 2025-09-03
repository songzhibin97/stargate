package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestWASMMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.WASMConfig
		requestPath    string
		requestMethod  string
		requestBody    string
		expectedStatus int
		expectProcessed bool
	}{
		{
			name: "disabled middleware should pass through",
			config: &config.WASMConfig{
				Enabled: false,
			},
			requestPath:     "/test",
			requestMethod:   "GET",
			expectedStatus:  http.StatusOK,
			expectProcessed: false,
		},
		{
			name: "no matching rule should pass through",
			config: &config.WASMConfig{
				Enabled: true,
				Plugins: []config.WASMPlugin{
					{
						ID:   "test-plugin",
						Name: "Test Plugin",
						Path: "test.wasm",
					},
				},
				Rules: []config.WASMRule{
					{
						ID:      "test-rule",
						Path:    "/api/wasm-test",
						Method:  "POST",
						Plugins: []string{"test-plugin"},
					},
				},
			},
			requestPath:     "/other/path",
			requestMethod:   "GET",
			expectedStatus:  http.StatusOK,
			expectProcessed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require actual WASM files for now
			if tt.config.Enabled && len(tt.config.Plugins) > 0 {
				t.Skip("Skipping test that requires WASM plugin files")
			}

			middleware, err := NewWASMMiddleware(tt.config)
			if tt.config.Enabled && err != nil {
				t.Fatalf("Failed to create WASM middleware: %v", err)
			}
			if !tt.config.Enabled {
				// For disabled middleware, create a simple instance
				middleware = &WASMMiddleware{
					config:  tt.config,
					plugins: make(map[string]*WASMPlugin),
				}
			}
			
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
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
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

func TestWASMMiddleware_matchRule(t *testing.T) {
	config := &config.WASMConfig{
		Enabled: true,
		Rules: []config.WASMRule{
			{
				ID:      "exact-match",
				Path:    "/api/users",
				Method:  "GET",
				Plugins: []string{"plugin1"},
			},
			{
				ID:      "any-method",
				Path:    "/api/posts",
				Method:  "", // Empty method matches all
				Plugins: []string{"plugin2"},
			},
			{
				ID:     "with-headers",
				Path:   "/api/secure",
				Method: "POST",
				Headers: map[string]string{
					"X-API-Key": "secret",
				},
				Plugins: []string{"plugin3"},
			},
		},
	}

	middleware := &WASMMiddleware{
		config:  config,
		plugins: make(map[string]*WASMPlugin),
	}

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

			var matchedRuleID string
			for _, rule := range config.Rules {
				if middleware.matchRule(req, rule) {
					matchedRuleID = rule.ID
					break
				}
			}

			if tt.expectedRuleID == "" {
				if matchedRuleID != "" {
					t.Errorf("expected no match, but got rule %s", matchedRuleID)
				}
			} else {
				if matchedRuleID == "" {
					t.Errorf("expected rule %s, but got no match", tt.expectedRuleID)
				} else if matchedRuleID != tt.expectedRuleID {
					t.Errorf("expected rule %s, got %s", tt.expectedRuleID, matchedRuleID)
				}
			}
		})
	}
}

func TestWASMMiddleware_findMatchingPlugins(t *testing.T) {
	// Create test plugins
	plugin1 := &WASMPlugin{
		ID:       "plugin1",
		Name:     "Test Plugin 1",
		LoadedAt: time.Now(),
	}
	plugin2 := &WASMPlugin{
		ID:       "plugin2",
		Name:     "Test Plugin 2",
		LoadedAt: time.Now(),
	}

	config := &config.WASMConfig{
		Enabled: true,
		Rules: []config.WASMRule{
			{
				ID:      "rule1",
				Path:    "/api/test",
				Method:  "GET",
				Plugins: []string{"plugin1"},
			},
			{
				ID:      "rule2",
				Path:    "/api/test",
				Method:  "POST",
				Plugins: []string{"plugin1", "plugin2"},
			},
		},
	}

	middleware := &WASMMiddleware{
		config: config,
		plugins: map[string]*WASMPlugin{
			"plugin1": plugin1,
			"plugin2": plugin2,
		},
	}

	tests := []struct {
		name            string
		requestPath     string
		requestMethod   string
		expectedPlugins []string
	}{
		{
			name:            "GET request matches rule1",
			requestPath:     "/api/test",
			requestMethod:   "GET",
			expectedPlugins: []string{"plugin1"},
		},
		{
			name:            "POST request matches rule2",
			requestPath:     "/api/test",
			requestMethod:   "POST",
			expectedPlugins: []string{"plugin1", "plugin2"},
		},
		{
			name:            "no matching rule",
			requestPath:     "/api/other",
			requestMethod:   "GET",
			expectedPlugins: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			matchingPlugins := middleware.findMatchingPlugins(req)

			if len(matchingPlugins) != len(tt.expectedPlugins) {
				t.Errorf("expected %d plugins, got %d", len(tt.expectedPlugins), len(matchingPlugins))
				return
			}

			// Check if all expected plugins are present
			pluginIDs := make(map[string]bool)
			for _, plugin := range matchingPlugins {
				pluginIDs[plugin.ID] = true
			}

			for _, expectedID := range tt.expectedPlugins {
				if !pluginIDs[expectedID] {
					t.Errorf("expected plugin %s not found in matching plugins", expectedID)
				}
			}
		})
	}
}

func TestWASMMiddleware_GetStats(t *testing.T) {
	plugin1 := &WASMPlugin{
		ID:         "plugin1",
		Name:       "Test Plugin 1",
		LoadedAt:   time.Now(),
		LastUsed:   time.Now(),
		CallCount:  5,
		ErrorCount: 1,
	}

	middleware := &WASMMiddleware{
		config: &config.WASMConfig{Enabled: true},
		plugins: map[string]*WASMPlugin{
			"plugin1": plugin1,
		},
		totalRequests:   10,
		pluginRequests:  5,
		failedRequests:  1,
		pluginLoadTime:  100 * time.Millisecond,
	}

	stats := middleware.GetStats()

	// Check basic stats
	if stats["total_requests"] != int64(10) {
		t.Errorf("expected total_requests 10, got %v", stats["total_requests"])
	}
	if stats["plugin_requests"] != int64(5) {
		t.Errorf("expected plugin_requests 5, got %v", stats["plugin_requests"])
	}
	if stats["failed_requests"] != int64(1) {
		t.Errorf("expected failed_requests 1, got %v", stats["failed_requests"])
	}
	if stats["loaded_plugins"] != 1 {
		t.Errorf("expected loaded_plugins 1, got %v", stats["loaded_plugins"])
	}

	// Check plugin stats
	pluginStats, ok := stats["plugin_stats"].(map[string]interface{})
	if !ok {
		t.Fatal("plugin_stats should be a map")
	}

	plugin1Stats, ok := pluginStats["plugin1"].(map[string]interface{})
	if !ok {
		t.Fatal("plugin1 stats should be a map")
	}

	if plugin1Stats["name"] != "Test Plugin 1" {
		t.Errorf("expected plugin name 'Test Plugin 1', got %v", plugin1Stats["name"])
	}
	if plugin1Stats["call_count"] != int64(5) {
		t.Errorf("expected call_count 5, got %v", plugin1Stats["call_count"])
	}
	if plugin1Stats["error_count"] != int64(1) {
		t.Errorf("expected error_count 1, got %v", plugin1Stats["error_count"])
	}
}

func TestWASMMiddleware_matchPath(t *testing.T) {
	middleware := &WASMMiddleware{}

	tests := []struct {
		name        string
		requestPath string
		rulePath    string
		expected    bool
	}{
		{
			name:        "exact match",
			requestPath: "/api/users",
			rulePath:    "/api/users",
			expected:    true,
		},
		{
			name:        "no match",
			requestPath: "/api/users",
			rulePath:    "/api/posts",
			expected:    false,
		},
		{
			name:        "empty rule path",
			requestPath: "/api/users",
			rulePath:    "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.matchPath(tt.requestPath, tt.rulePath)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
