package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/middleware"
)

// TestHeaderTransformIntegration tests the header transformation middleware end-to-end
func TestHeaderTransformIntegration(t *testing.T) {
	// Create a mock upstream server that echoes received headers
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo all received headers in the response
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}

		response := map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"headers": headers,
		}

		// Set some response headers that will be transformed
		w.Header().Set("Server", "nginx/1.20")
		w.Header().Set("X-Powered-By", "PHP/8.0")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("X-Custom-Response", "custom-response-value")
		w.Header().Set("Content-Type", "application/json")

		// Encode response to JSON
		responseData, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		// Set correct content length
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseData)))

		// Write response
		w.Write(responseData)
	}))
	defer upstreamServer.Close()

	// Create header transform middleware configuration
	headerConfig := config.HeaderTransformConfig{
		Enabled: true,
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Request-Id": "test-request-123",
				"X-Gateway":    "stargate",
				"X-Method":     "${method}",
				"X-Path":       "${path}",
			},
			Remove: []string{"X-Internal-Token", "Authorization"},
			Rename: map[string]string{
				"X-Custom-Header": "X-Renamed-Header",
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
			Remove: []string{"Server", "X-Powered-By"},
			Rename: map[string]string{
				"X-Custom-Response": "X-Renamed-Response",
			},
			Replace: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
			},
		},
	}

	// Create header transform middleware
	headerMiddleware := middleware.NewHeaderTransformMiddleware(&headerConfig)

	// Create a proxy handler that forwards requests to the upstream server
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to the upstream server
		upstreamURL := upstreamServer.URL + r.URL.Path
		if r.URL.RawQuery != "" {
			upstreamURL += "?" + r.URL.RawQuery
		}

		upstreamReq, err := http.NewRequest(r.Method, upstreamURL, r.Body)
		if err != nil {
			http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
			return
		}

		// Copy headers from original request
		for name, values := range r.Header {
			for _, value := range values {
				upstreamReq.Header.Add(name, value)
			}
		}

		// Make the upstream request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(upstreamReq)
		if err != nil {
			http.Error(w, "Failed to reach upstream", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// Copy status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		io.Copy(w, resp.Body)
	})

	// Wrap the proxy handler with header transform middleware
	wrappedHandler := headerMiddleware.Handler()(proxyHandler)

	// Create test server with the wrapped handler
	testServer := httptest.NewServer(wrappedHandler)
	defer testServer.Close()

	// First, test the upstream server directly
	t.Run("Upstream Server Test", func(t *testing.T) {
		req, err := http.NewRequest("GET", upstreamServer.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Test-Header", "test-value")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if len(body) == 0 {
			t.Fatal("Upstream server returned empty response")
		}
	})

	// Test case 1: Verify request headers are transformed
	t.Run("Request Header Transformation", func(t *testing.T) {
		// Create request with headers that should be transformed
		req, err := http.NewRequest("POST", testServer.URL+"/api/test", strings.NewReader("test body"))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Set headers that will be transformed
		req.Header.Set("X-Custom-Header", "custom-value") // This should be renamed
		req.Header.Set("Accept", "text/html") // This should be replaced
		req.Header.Set("X-Internal-Token", "secret") // This should be removed
		req.Header.Set("Authorization", "Bearer token") // This should be removed
		req.Header.Set("Content-Type", "application/json")

		// Make request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		headers, ok := response["headers"].(map[string]interface{})
		if !ok {
			t.Fatal("Response does not contain headers")
		}

		// Verify added headers
		if headers["X-Request-Id"] != "test-request-123" {
			t.Errorf("Expected X-Request-Id to be 'test-request-123', got %v", headers["X-Request-Id"])
		}
		if headers["X-Gateway"] != "stargate" {
			t.Errorf("Expected X-Gateway to be 'stargate', got %v", headers["X-Gateway"])
		}
		if headers["X-Method"] != "POST" {
			t.Errorf("Expected X-Method to be 'POST', got %v", headers["X-Method"])
		}
		if headers["X-Path"] != "/api/test" {
			t.Errorf("Expected X-Path to be '/api/test', got %v", headers["X-Path"])
		}

		// Verify removed headers
		if _, exists := headers["X-Internal-Token"]; exists {
			t.Error("X-Internal-Token should have been removed")
		}
		if _, exists := headers["Authorization"]; exists {
			t.Error("Authorization should have been removed")
		}

		// Verify renamed headers
		if _, exists := headers["X-Custom-Header"]; exists {
			t.Error("X-Custom-Header should have been renamed")
		}
		if headers["X-Renamed-Header"] != "custom-value" {
			t.Errorf("Expected X-Renamed-Header to be 'custom-value', got %v", headers["X-Renamed-Header"])
		}

		// Verify replaced headers
		if headers["Accept"] != "application/json" {
			t.Errorf("Expected Accept to be replaced with 'application/json', got %v", headers["Accept"])
		}
	})

	// Test case 2: Verify response headers are transformed
	t.Run("Response Header Transformation", func(t *testing.T) {
		// Create simple request
		req, err := http.NewRequest("GET", testServer.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Make request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Verify added response headers
		if resp.Header.Get("X-Processed-By") != "stargate-gateway" {
			t.Errorf("Expected X-Processed-By to be 'stargate-gateway', got %s", resp.Header.Get("X-Processed-By"))
		}
		if resp.Header.Get("X-Version") != "1.0.0" {
			t.Errorf("Expected X-Version to be '1.0.0', got %s", resp.Header.Get("X-Version"))
		}

		// Verify removed response headers
		if resp.Header.Get("Server") != "" {
			t.Error("Server header should have been removed")
		}
		if resp.Header.Get("X-Powered-By") != "" {
			t.Error("X-Powered-By header should have been removed")
		}

		// Verify renamed response headers
		if resp.Header.Get("X-Custom-Response") != "" {
			t.Error("X-Custom-Response should have been renamed")
		}
		if resp.Header.Get("X-Renamed-Response") != "custom-response-value" {
			t.Errorf("Expected X-Renamed-Response to be 'custom-response-value', got %s", resp.Header.Get("X-Renamed-Response"))
		}

		// Verify replaced response headers
		if resp.Header.Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
			t.Errorf("Expected Cache-Control to be replaced, got %s", resp.Header.Get("Cache-Control"))
		}
	})
}

// TestHeaderTransformDisabled tests that middleware is bypassed when disabled
func TestHeaderTransformDisabled(t *testing.T) {
	// Create a mock upstream server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo received headers
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "nginx/1.20") // This should NOT be removed when disabled
		json.NewEncoder(w).Encode(map[string]interface{}{
			"headers": headers,
		})
	}))
	defer upstreamServer.Close()

	// Create header transform middleware configuration (disabled)
	headerConfig := config.HeaderTransformConfig{
		Enabled: false, // Disabled
		RequestHeaders: config.HeaderTransformRules{
			Add: map[string]string{
				"X-Should-Not-Add": "should-not-be-added",
			},
		},
		ResponseHeaders: config.HeaderTransformRules{
			Remove: []string{"Server"}, // This should NOT be removed when disabled
		},
	}

	// Create header transform middleware
	headerMiddleware := middleware.NewHeaderTransformMiddleware(&headerConfig)

	// Create a proxy handler that forwards requests to the upstream server
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to the upstream server
		upstreamURL := upstreamServer.URL + r.URL.Path
		if r.URL.RawQuery != "" {
			upstreamURL += "?" + r.URL.RawQuery
		}

		upstreamReq, err := http.NewRequest(r.Method, upstreamURL, r.Body)
		if err != nil {
			http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
			return
		}

		// Copy headers from original request
		for name, values := range r.Header {
			for _, value := range values {
				upstreamReq.Header.Add(name, value)
			}
		}

		// Make the upstream request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(upstreamReq)
		if err != nil {
			http.Error(w, "Failed to reach upstream", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// Copy status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		io.Copy(w, resp.Body)
	})

	// Wrap the proxy handler with header transform middleware (should be bypassed)
	wrappedHandler := headerMiddleware.Handler()(proxyHandler)

	// Create test server with the wrapped handler
	testServer := httptest.NewServer(wrappedHandler)
	defer testServer.Close()

	// Make request
	req, err := http.NewRequest("GET", testServer.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Original-Header", "original-value")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	headers, ok := response["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("Response does not contain headers")
	}

	// Verify original header is preserved
	if headers["Original-Header"] != "original-value" {
		t.Error("Original header should be preserved when middleware is disabled")
	}

	// Verify no headers were added
	if _, exists := headers["X-Should-Not-Add"]; exists {
		t.Error("No headers should be added when middleware is disabled")
	}

	// Verify response headers are not transformed
	if resp.Header.Get("Server") == "" {
		t.Error("Server header should NOT be removed when middleware is disabled")
	}
}
