package proxy

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/ratelimit"
)

func TestPipeline_RateLimitIntegration(t *testing.T) {
	// Create configuration with rate limiting enabled
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:            32768,
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
		RateLimit: config.RateLimitConfig{
			Enabled:            true,
			DefaultRate:        2, // Very low limit for testing
			Burst:              2,
			Storage:            "memory",
			Strategy:           "fixed_window",
			IdentifierStrategy: "ip",
			WindowSize:         time.Minute,
			CleanupInterval:    5 * time.Minute,
			SkipSuccessful:     false,
			SkipFailed:         false,
			CustomHeaders:      make(map[string]string),
			ExcludedPaths:      []string{},
			ExcludedIPs:        []string{},
			PerRoute:           make(map[string]config.RouteRateLimit),
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	// Create pipeline
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Stop()

	// Create test server
	server := httptest.NewServer(pipeline)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	clientIP := "192.168.1.100"

	// Test allowed requests
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("GET", server.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Forwarded-For", clientIP)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound { // Expected since no routes are configured
			t.Logf("Request %d: Status %d (expected 404 due to no routes)", i+1, resp.StatusCode)
		}

		// Check rate limit headers
		if resp.Header.Get("X-RateLimit-Limit") != "2" {
			t.Errorf("Request %d: Expected X-RateLimit-Limit '2', got '%s'", i+1, resp.Header.Get("X-RateLimit-Limit"))
		}

		expectedRemaining := 2 - (i + 1)
		if resp.Header.Get("X-RateLimit-Remaining") != string(rune('0'+expectedRemaining)) {
			t.Errorf("Request %d: Expected X-RateLimit-Remaining '%d', got '%s'", i+1, expectedRemaining, resp.Header.Get("X-RateLimit-Remaining"))
		}
	}

	// Test rate limited request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Rate limited request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should be rate limited
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", resp.StatusCode)
	}

	// Check rate limit headers
	if resp.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining '0', got '%s'", resp.Header.Get("X-RateLimit-Remaining"))
	}

	if resp.Header.Get("Retry-After") == "" {
		t.Error("Expected Retry-After header to be set")
	}

	// Check response body
	var errorResponse ratelimit.RateLimitErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	if err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorResponse.Error != "Too Many Requests" {
		t.Errorf("Expected error 'Too Many Requests', got '%s'", errorResponse.Error)
	}

	if errorResponse.Code != http.StatusTooManyRequests {
		t.Errorf("Expected code 429, got %d", errorResponse.Code)
	}
}

func TestPipeline_RateLimitDisabled(t *testing.T) {
	// Create configuration with rate limiting disabled
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:            32768,
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
		RateLimit: config.RateLimitConfig{
			Enabled: false,
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	// Create pipeline
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Stop()

	// Create test server
	server := httptest.NewServer(pipeline)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Make multiple requests - should not be rate limited
	for i := 0; i < 10; i++ {
		req, err := http.NewRequest("GET", server.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		// Should not be rate limited (will get 404 due to no routes)
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("Request %d should not be rate limited when disabled", i+1)
		}

		// Should not have rate limit headers
		if resp.Header.Get("X-RateLimit-Limit") != "" {
			t.Errorf("Request %d should not have rate limit headers when disabled", i+1)
		}
	}
}

func TestPipeline_RateLimitMultipleClients(t *testing.T) {
	// Create configuration with rate limiting enabled
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:            32768,
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
		RateLimit: config.RateLimitConfig{
			Enabled:            true,
			DefaultRate:        1, // Very low limit for testing
			Burst:              1,
			Storage:            "memory",
			Strategy:           "fixed_window",
			IdentifierStrategy: "ip",
			WindowSize:         time.Minute,
			CleanupInterval:    5 * time.Minute,
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	// Create pipeline
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Stop()

	// Create test server
	server := httptest.NewServer(pipeline)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Test different client IPs
	clientIPs := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	for _, clientIP := range clientIPs {
		// First request should be allowed
		req, err := http.NewRequest("GET", server.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Forwarded-For", clientIP)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request from %s failed: %v", clientIP, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("First request from %s should not be rate limited", clientIP)
		}

		// Second request should be rate limited
		req, err = http.NewRequest("GET", server.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Forwarded-For", clientIP)

		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("Second request from %s failed: %v", clientIP, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Second request from %s should be rate limited, got status %d", clientIP, resp.StatusCode)
		}
	}
}

func TestPipeline_RateLimitWindowReset(t *testing.T) {
	// Create configuration with very short window for testing
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:            32768,
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
		RateLimit: config.RateLimitConfig{
			Enabled:            true,
			DefaultRate:        1,
			Burst:              1,
			Storage:            "memory",
			Strategy:           "fixed_window",
			IdentifierStrategy: "ip",
			WindowSize:         200 * time.Millisecond, // Very short window
			CleanupInterval:    5 * time.Minute,
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	// Create pipeline
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Stop()

	// Create test server
	server := httptest.NewServer(pipeline)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	clientIP := "192.168.1.100"

	// First request should be allowed
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Error("First request should not be rate limited")
	}

	// Second request should be rate limited
	req, err = http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", resp.StatusCode)
	}

	// Wait for window to reset
	time.Sleep(300 * time.Millisecond)

	// Request after window reset should be allowed
	req, err = http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Request after reset failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Error("Request after window reset should not be rate limited")
	}

	// Check that remaining count is reset
	if resp.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining '0' after reset, got '%s'", resp.Header.Get("X-RateLimit-Remaining"))
	}
}
