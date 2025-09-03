package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestAggregatorMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.AggregatorConfig
		requestPath    string
		requestMethod  string
		expectedStatus int
		expectAggregated bool
	}{
		{
			name: "disabled middleware should pass through",
			config: &config.AggregatorConfig{
				Enabled: false,
			},
			requestPath:      "/test",
			requestMethod:    "GET",
			expectedStatus:   http.StatusOK,
			expectAggregated: false,
		},
		{
			name: "no matching route should pass through",
			config: &config.AggregatorConfig{
				Enabled: true,
				Routes: []config.AggregateRoute{
					{
						ID:     "test-route",
						Path:   "/aggregated/test",
						Method: "GET",
					},
				},
			},
			requestPath:      "/other/path",
			requestMethod:    "GET",
			expectedStatus:   http.StatusOK,
			expectAggregated: false,
		},
		{
			name: "matching route should be aggregated",
			config: &config.AggregatorConfig{
				Enabled:        true,
				DefaultTimeout: 10 * time.Second,
				Routes: []config.AggregateRoute{
					{
						ID:     "test-route",
						Path:   "/aggregated/test",
						Method: "GET",
						UpstreamRequests: []config.UpstreamRequest{
							{
								Name:     "service1",
								URL:      "http://example.com/api1",
								Method:   "GET",
								Required: false,
							},
						},
					},
				},
			},
			requestPath:      "/aggregated/test",
			requestMethod:    "GET",
			expectedStatus:   http.StatusOK,
			expectAggregated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewAggregatorMiddleware(tt.config)
			
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
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify if next handler was called
			if tt.expectAggregated && nextCalled {
				t.Error("expected aggregation, but next handler was called")
			}
			if !tt.expectAggregated && !nextCalled {
				t.Error("expected pass-through, but next handler was not called")
			}
		})
	}
}

func TestAggregatorMiddleware_matchAggregateRoute(t *testing.T) {
	config := &config.AggregatorConfig{
		Enabled: true,
		Routes: []config.AggregateRoute{
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
				ID:     "post-only",
				Path:   "/api/create",
				Method: "POST",
			},
		},
	}

	middleware := NewAggregatorMiddleware(config)

	tests := []struct {
		name           string
		requestPath    string
		requestMethod  string
		expectedRouteID string
	}{
		{
			name:           "exact path and method match",
			requestPath:    "/api/users",
			requestMethod:  "GET",
			expectedRouteID: "exact-match",
		},
		{
			name:           "path match with any method",
			requestPath:    "/api/posts",
			requestMethod:  "PUT",
			expectedRouteID: "any-method",
		},
		{
			name:           "method mismatch",
			requestPath:    "/api/create",
			requestMethod:  "GET",
			expectedRouteID: "",
		},
		{
			name:           "path mismatch",
			requestPath:    "/api/unknown",
			requestMethod:  "GET",
			expectedRouteID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			route := middleware.matchAggregateRoute(req)

			if tt.expectedRouteID == "" {
				if route != nil {
					t.Errorf("expected no match, but got route %s", route.ID)
				}
			} else {
				if route == nil {
					t.Errorf("expected route %s, but got no match", tt.expectedRouteID)
				} else if route.ID != tt.expectedRouteID {
					t.Errorf("expected route %s, got %s", tt.expectedRouteID, route.ID)
				}
			}
		})
	}
}

func TestAggregatorMiddleware_executeUpstreamRequest(t *testing.T) {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message": "success",
			"path":    r.URL.Path,
			"method":  r.Method,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	config := &config.AggregatorConfig{
		Enabled:        true,
		DefaultTimeout: 5 * time.Second,
	}
	middleware := NewAggregatorMiddleware(config)

	tests := []struct {
		name            string
		upstreamRequest UpstreamRequest
		expectError     bool
	}{
		{
			name: "successful request",
			upstreamRequest: UpstreamRequest{
				Name:   "test-service",
				URL:    testServer.URL + "/api/test",
				Method: "GET",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
			expectError: false,
		},
		{
			name: "invalid URL",
			upstreamRequest: UpstreamRequest{
				Name:   "invalid-service",
				URL:    "invalid-url",
				Method: "GET",
			},
			expectError: true,
		},
		{
			name: "timeout request",
			upstreamRequest: UpstreamRequest{
				Name:    "timeout-service",
				URL:     "http://10.255.255.1:12345/timeout", // Non-routable IP
				Method:  "GET",
				Timeout: 100 * time.Millisecond,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			results := make(chan *UpstreamResult, 1)
			go middleware.executeUpstreamRequest(ctx, tt.upstreamRequest, results)

			select {
			case result := <-results:
				if tt.expectError && result.Error == nil {
					t.Error("expected error, but got none")
				}
				if !tt.expectError && result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				if result.Name != tt.upstreamRequest.Name {
					t.Errorf("expected name %s, got %s", tt.upstreamRequest.Name, result.Name)
				}
			case <-ctx.Done():
				t.Error("request timed out")
			}
		})
	}
}

func TestAggregatorMiddleware_mergeResponses(t *testing.T) {
	config := &config.AggregatorConfig{
		Enabled: true,
	}
	middleware := NewAggregatorMiddleware(config)

	tests := []struct {
		name     string
		results  map[string]*UpstreamResult
		template string
		expected map[string]interface{}
	}{
		{
			name: "successful responses",
			results: map[string]*UpstreamResult{
				"service1": {
					Name:       "service1",
					StatusCode: 200,
					Body:       []byte(`{"id": 1, "name": "test"}`),
					Error:      nil,
				},
				"service2": {
					Name:       "service2",
					StatusCode: 200,
					Body:       []byte(`{"count": 5}`),
					Error:      nil,
				},
			},
			template: "",
			expected: map[string]interface{}{
				"service1": map[string]interface{}{
					"id":   float64(1),
					"name": "test",
				},
				"service2": map[string]interface{}{
					"count": float64(5),
				},
			},
		},
		{
			name: "mixed success and error",
			results: map[string]*UpstreamResult{
				"success": {
					Name:       "success",
					StatusCode: 200,
					Body:       []byte(`{"status": "ok"}`),
					Error:      nil,
				},
				"failure": {
					Name:  "failure",
					Error: fmt.Errorf("connection failed"),
				},
			},
			template: "",
			expected: map[string]interface{}{
				"success": map[string]interface{}{
					"status": "ok",
				},
				"failure": map[string]interface{}{
					"error":  "connection failed",
					"status": "failed",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := middleware.mergeResponses(tt.results, tt.template)

			if response.StatusCode != http.StatusOK {
				t.Errorf("expected status 200, got %d", response.StatusCode)
			}

			responseData, ok := response.Body.(map[string]interface{})
			if !ok {
				t.Fatal("response body is not a map")
			}

			// Verify each expected key
			for key, expectedValue := range tt.expected {
				actualValue, exists := responseData[key]
				if !exists {
					t.Errorf("expected key %s not found in response", key)
					continue
				}

				// For complex comparison, convert to JSON and compare
				expectedJSON, _ := json.Marshal(expectedValue)
				actualJSON, _ := json.Marshal(actualValue)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("for key %s, expected %s, got %s", key, expectedJSON, actualJSON)
				}
			}
		})
	}
}

func TestAggregatorMiddleware_hasRequiredFailures(t *testing.T) {
	config := &config.AggregatorConfig{
		Enabled: true,
	}
	middleware := NewAggregatorMiddleware(config)

	tests := []struct {
		name             string
		results          map[string]*UpstreamResult
		upstreamRequests []UpstreamRequest
		expectFailure    bool
	}{
		{
			name: "all required services successful",
			results: map[string]*UpstreamResult{
				"required1": {Name: "required1", StatusCode: 200, Error: nil},
				"optional1": {Name: "optional1", StatusCode: 500, Error: nil},
			},
			upstreamRequests: []UpstreamRequest{
				{Name: "required1", Required: true},
				{Name: "optional1", Required: false},
			},
			expectFailure: false,
		},
		{
			name: "required service failed",
			results: map[string]*UpstreamResult{
				"required1": {Name: "required1", StatusCode: 500, Error: nil},
				"optional1": {Name: "optional1", StatusCode: 200, Error: nil},
			},
			upstreamRequests: []UpstreamRequest{
				{Name: "required1", Required: true},
				{Name: "optional1", Required: false},
			},
			expectFailure: true,
		},
		{
			name: "required service missing",
			results: map[string]*UpstreamResult{
				"optional1": {Name: "optional1", StatusCode: 200, Error: nil},
			},
			upstreamRequests: []UpstreamRequest{
				{Name: "required1", Required: true},
				{Name: "optional1", Required: false},
			},
			expectFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasFailure := middleware.hasRequiredFailures(tt.results, tt.upstreamRequests)
			if hasFailure != tt.expectFailure {
				t.Errorf("expected failure %v, got %v", tt.expectFailure, hasFailure)
			}
		})
	}
}

func TestAggregatorMiddleware_GetStats(t *testing.T) {
	config := &config.AggregatorConfig{
		Enabled: true,
	}
	middleware := NewAggregatorMiddleware(config)

	// Update some statistics
	middleware.updateTotalRequests()
	middleware.updateTotalRequests()
	middleware.updateAggregatedRequests()
	middleware.updateFailedRequests()

	stats := middleware.GetStats()

	expectedStats := map[string]interface{}{
		"total_requests":      int64(2),
		"aggregated_requests": int64(1),
		"failed_requests":     int64(1),
		"success_rate":        float64(0), // (1-1)/1 * 100 = 0
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
