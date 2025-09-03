package ratelimit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestTokenBucketMiddleware_Integration(t *testing.T) {
	config := &Config{
		Strategy:           StrategyTokenBucket,
		IdentifierStrategy: IdentifierIP,
		Rate:               2.0, // 2 requests per second
		BurstSize:          5,   // 5 request burst
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	// Create a simple handler that returns 200 OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with rate limiting middleware
	wrappedHandler := middleware.Handler()(handler)

	// Test burst capacity - first 5 requests should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}

		// Check rate limit headers
		if w.Header().Get("X-RateLimit-Limit") != "5" {
			t.Errorf("Expected X-RateLimit-Limit: 5, got %s", w.Header().Get("X-RateLimit-Limit"))
		}

		expectedRemaining := strconv.Itoa(5 - i - 1)
		if w.Header().Get("X-RateLimit-Remaining") != expectedRemaining {
			t.Errorf("Expected X-RateLimit-Remaining: %s, got %s", 
				expectedRemaining, w.Header().Get("X-RateLimit-Remaining"))
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th request should be rate limited, got status %d", w.Code)
	}

	// Check error response
	var errorResp RateLimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
	}

	if errorResp.Code != http.StatusTooManyRequests {
		t.Errorf("Expected error code %d, got %d", http.StatusTooManyRequests, errorResp.Code)
	}

	if errorResp.Remaining != 0 {
		t.Errorf("Expected remaining 0, got %d", errorResp.Remaining)
	}
}

func TestTokenBucketMiddleware_MultipleClients(t *testing.T) {
	config := &Config{
		Strategy:           StrategyTokenBucket,
		IdentifierStrategy: IdentifierIP,
		Rate:               5.0,
		BurstSize:          3,
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.Handler()(handler)

	// Test multiple clients - each should have independent rate limits
	clients := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}

	for _, clientIP := range clients {
		// Each client should be able to make 3 requests (burst capacity)
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = clientIP
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Client %s request %d should succeed, got status %d", 
					clientIP, i+1, w.Code)
			}
		}

		// 4th request should be rate limited
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Client %s 4th request should be rate limited, got status %d", 
				clientIP, w.Code)
		}
	}
}

func TestTokenBucketMiddleware_TokenRefill(t *testing.T) {
	config := &Config{
		Strategy:           StrategyTokenBucket,
		IdentifierStrategy: IdentifierIP,
		Rate:               10.0, // 10 requests per second
		BurstSize:          2,    // 2 request burst
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.Handler()(handler)

	clientIP := "192.168.1.1:12345"

	// Consume all tokens
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request should be rate limited, got status %d", w.Code)
	}

	// Wait for tokens to refill (200ms = 2 tokens at 10 tokens/sec)
	time.Sleep(200 * time.Millisecond)

	// Should be allowed again
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	w = httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request should succeed after token refill, got status %d", w.Code)
	}
}

func TestTokenBucketMiddleware_Disabled(t *testing.T) {
	config := &Config{
		Strategy:           StrategyTokenBucket,
		IdentifierStrategy: IdentifierIP,
		Rate:               1.0,
		BurstSize:          1,
		Enabled:            false, // Disabled
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.Handler()(handler)

	// All requests should succeed when rate limiting is disabled
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed when rate limiting disabled, got status %d", 
				i+1, w.Code)
		}
	}
}

func TestTokenBucketMiddleware_ConcurrentRequests(t *testing.T) {
	config := &Config{
		Strategy:           StrategyTokenBucket,
		IdentifierStrategy: IdentifierIP,
		Rate:               20.0,
		BurstSize:          10,
		Enabled:            true,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.Handler()(handler)

	numGoroutines := 20
	requestsPerGoroutine := 2
	var successCount, rateLimitedCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			localSuccess := 0
			localRateLimited := 0
			
			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", clientID)
				w := httptest.NewRecorder()

				wrappedHandler.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					localSuccess++
				} else if w.Code == http.StatusTooManyRequests {
					localRateLimited++
				}
			}
			
			mu.Lock()
			successCount += localSuccess
			rateLimitedCount += localRateLimited
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	totalRequests := numGoroutines * requestsPerGoroutine
	if successCount+rateLimitedCount != totalRequests {
		t.Errorf("Expected %d total responses, got %d", 
			totalRequests, successCount+rateLimitedCount)
	}

	// Should have some successful requests
	if successCount == 0 {
		t.Error("Expected some successful requests")
	}

	t.Logf("Concurrent test: %d successful, %d rate limited out of %d total", 
		successCount, rateLimitedCount, totalRequests)
}
