package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestFixedWindowRateLimiter_NewFixedWindowRateLimiter(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     10,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	if limiter == nil {
		t.Fatal("Expected non-nil limiter")
	}

	if limiter.windowSize != config.WindowSize {
		t.Errorf("Expected window size %v, got %v", config.WindowSize, limiter.windowSize)
	}

	if limiter.maxRequests != config.MaxRequests {
		t.Errorf("Expected max requests %d, got %d", config.MaxRequests, limiter.maxRequests)
	}
}

func TestFixedWindowRateLimiter_IsAllowed_FirstRequest(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     5,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	// First request should be allowed
	allowed := limiter.IsAllowed("test-client")
	if !allowed {
		t.Error("First request should be allowed")
	}

	// Check quota
	quota := limiter.GetQuota("test-client")
	if quota.Remaining != 4 {
		t.Errorf("Expected 4 remaining requests, got %d", quota.Remaining)
	}
}

func TestFixedWindowRateLimiter_IsAllowed_WithinLimit(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     3,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// Make requests within limit
	for i := 0; i < 3; i++ {
		allowed := limiter.IsAllowed(identifier)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Check quota after all requests
	quota := limiter.GetQuota(identifier)
	if quota.Remaining != 0 {
		t.Errorf("Expected 0 remaining requests, got %d", quota.Remaining)
	}
}

func TestFixedWindowRateLimiter_IsAllowed_ExceedsLimit(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     2,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// Make requests up to limit
	for i := 0; i < 2; i++ {
		allowed := limiter.IsAllowed(identifier)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Next request should be denied
	allowed := limiter.IsAllowed(identifier)
	if allowed {
		t.Error("Request exceeding limit should be denied")
	}

	// Check quota
	quota := limiter.GetQuota(identifier)
	if quota.Remaining != 0 {
		t.Errorf("Expected 0 remaining requests, got %d", quota.Remaining)
	}
}

func TestFixedWindowRateLimiter_WindowReset(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      100 * time.Millisecond, // Very short window for testing
		MaxRequests:     2,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// Use up the quota
	for i := 0; i < 2; i++ {
		allowed := limiter.IsAllowed(identifier)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Check quota after using up all requests
	quota := limiter.GetQuota(identifier)
	t.Logf("After consuming all requests: remaining=%d, limit=%d", quota.Remaining, quota.Limit)

	// Next request should be denied
	allowed := limiter.IsAllowed(identifier)
	if allowed {
		t.Error("Request exceeding limit should be denied")
	}

	// Wait for window to reset - use longer time to ensure we're in a new window
	time.Sleep(200 * time.Millisecond)

	// Check quota before making new request
	quota = limiter.GetQuota(identifier)
	t.Logf("After window reset, before new request: remaining=%d, limit=%d", quota.Remaining, quota.Limit)

	// Request should be allowed again
	allowed = limiter.IsAllowed(identifier)
	if !allowed {
		t.Error("Request should be allowed after window reset")
	}

	// Check quota after making one request in new window
	quota = limiter.GetQuota(identifier)
	t.Logf("After making 1 request in new window: remaining=%d, limit=%d", quota.Remaining, quota.Limit)
	if quota.Remaining != 1 {
		t.Errorf("Expected 1 remaining request after consuming 1 in new window, got %d", quota.Remaining)
	}
}

func TestFixedWindowRateLimiter_MultipleClients(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     2,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	client1 := "client-1"
	client2 := "client-2"

	// Each client should have independent quota
	for i := 0; i < 2; i++ {
		allowed1 := limiter.IsAllowed(client1)
		allowed2 := limiter.IsAllowed(client2)

		if !allowed1 {
			t.Errorf("Client 1 request %d should be allowed", i+1)
		}
		if !allowed2 {
			t.Errorf("Client 2 request %d should be allowed", i+1)
		}
	}

	// Both clients should be at limit
	allowed1 := limiter.IsAllowed(client1)
	allowed2 := limiter.IsAllowed(client2)

	if allowed1 {
		t.Error("Client 1 should be rate limited")
	}
	if allowed2 {
		t.Error("Client 2 should be rate limited")
	}
}

func TestFixedWindowRateLimiter_GetStats(t *testing.T) {
	config := &FixedWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     5,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewFixedWindowRateLimiter(config)
	defer limiter.Stop()

	// Make some requests
	limiter.IsAllowed("client-1")
	limiter.IsAllowed("client-1")
	limiter.IsAllowed("client-2")

	stats := limiter.GetStats()

	if stats.Algorithm != "fixed_window" {
		t.Errorf("Expected algorithm 'fixed_window', got %s", stats.Algorithm)
	}

	if stats.MaxRequests != 5 {
		t.Errorf("Expected max requests 5, got %d", stats.MaxRequests)
	}

	if stats.WindowSize != time.Minute {
		t.Errorf("Expected window size %v, got %v", time.Minute, stats.WindowSize)
	}

	if stats.TotalIdentifiers != 2 {
		t.Errorf("Expected 2 total identifiers, got %d", stats.TotalIdentifiers)
	}
}

func TestExtractIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		headers  map[string]string
		remoteAddr string
		expected string
	}{
		{
			name:     "IP strategy with RemoteAddr",
			strategy: "ip",
			headers:  map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			expected: "192.168.1.1",
		},
		{
			name:     "IP strategy with X-Forwarded-For",
			strategy: "ip",
			headers:  map[string]string{"X-Forwarded-For": "203.0.113.1, 192.168.1.1"},
			remoteAddr: "192.168.1.1:12345",
			expected: "203.0.113.1",
		},
		{
			name:     "User strategy with user ID",
			strategy: "user",
			headers:  map[string]string{"X-User-ID": "user123"},
			remoteAddr: "192.168.1.1:12345",
			expected: "user:user123",
		},
		{
			name:     "User strategy fallback to IP",
			strategy: "user",
			headers:  map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			expected: "ip:192.168.1.1",
		},
		{
			name:     "API key strategy",
			strategy: "api_key",
			headers:  map[string]string{"X-API-Key": "abc123"},
			remoteAddr: "192.168.1.1:12345",
			expected: "api_key:abc123",
		},
		{
			name:     "Combined strategy",
			strategy: "combined",
			headers:  map[string]string{"X-User-ID": "user123"},
			remoteAddr: "192.168.1.1:12345",
			expected: "user:user123:ip:192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := ExtractIdentifier(req, tt.strategy)
			if result != tt.expected {
				t.Errorf("Expected identifier %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSetRateLimitHeaders(t *testing.T) {
	quota := &QuotaInfo{
		Limit:       100,
		Remaining:   75,
		ResetTime:   time.Unix(1640995200, 0), // 2022-01-01 00:00:00 UTC
		WindowStart: time.Unix(1640995140, 0), // 2022-01-01 00:00:00 UTC - 1 minute
	}

	// Create a mock response writer
	headers := make(http.Header)
	mockWriter := &mockResponseWriter{headers: headers}

	SetRateLimitHeaders(mockWriter, quota)

	if headers.Get("X-RateLimit-Limit") != "100" {
		t.Errorf("Expected X-RateLimit-Limit header to be '100', got %s", headers.Get("X-RateLimit-Limit"))
	}

	if headers.Get("X-RateLimit-Remaining") != "75" {
		t.Errorf("Expected X-RateLimit-Remaining header to be '75', got %s", headers.Get("X-RateLimit-Remaining"))
	}

	if headers.Get("X-RateLimit-Reset") != "1640995200" {
		t.Errorf("Expected X-RateLimit-Reset header to be '1640995200', got %s", headers.Get("X-RateLimit-Reset"))
	}
}

// Mock response writer for testing
type mockResponseWriter struct {
	headers http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	// Do nothing
}
