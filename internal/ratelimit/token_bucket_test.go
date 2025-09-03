package ratelimit

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTokenBucketRateLimiter(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:            10.0,
		BurstSize:       20,
		CleanupInterval: 5 * time.Minute,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	if limiter == nil {
		t.Fatal("Expected non-nil limiter")
	}

	if limiter.rate != 10.0 {
		t.Errorf("Expected rate 10.0, got %f", limiter.rate)
	}

	if limiter.burstSize != 20 {
		t.Errorf("Expected burst size 20, got %d", limiter.burstSize)
	}
}

func TestNewTokenBucketRateLimiter_DefaultConfig(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(nil)
	defer limiter.Stop()

	if limiter == nil {
		t.Fatal("Expected non-nil limiter")
	}

	if limiter.rate != 10.0 {
		t.Errorf("Expected default rate 10.0, got %f", limiter.rate)
	}

	if limiter.burstSize != 20 {
		t.Errorf("Expected default burst size 20, got %d", limiter.burstSize)
	}
}

func TestNewTokenBucketRateLimiter_InvalidConfig(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      -1.0, // invalid
		BurstSize: -5,   // invalid
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	if limiter.rate != 1.0 {
		t.Errorf("Expected corrected rate 1.0, got %f", limiter.rate)
	}

	if limiter.burstSize != 1 {
		t.Errorf("Expected corrected burst size 1, got %d", limiter.burstSize)
	}
}

func TestTokenBucketRateLimiter_BasicFunctionality(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      2.0, // 2 tokens per second
		BurstSize: 5,   // max 5 tokens
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// First 5 requests should be allowed (burst capacity)
	for i := 0; i < 5; i++ {
		if !limiter.IsAllowed(identifier) {
			t.Errorf("Request %d should be allowed (burst capacity)", i+1)
		}
	}

	// 6th request should be denied (no tokens left)
	if limiter.IsAllowed(identifier) {
		t.Error("6th request should be denied (no tokens left)")
	}
}

func TestTokenBucketRateLimiter_TokenRefill(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      10.0, // 10 tokens per second
		BurstSize: 5,    // max 5 tokens
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// Consume all tokens
	for i := 0; i < 5; i++ {
		if !limiter.IsAllowed(identifier) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Should be denied now
	if limiter.IsAllowed(identifier) {
		t.Error("Request should be denied (no tokens)")
	}

	// Wait for tokens to refill (0.5 seconds = 5 tokens at 10 tokens/sec)
	time.Sleep(500 * time.Millisecond)

	// Should be allowed again
	if !limiter.IsAllowed(identifier) {
		t.Error("Request should be allowed after token refill")
	}
}

func TestTokenBucketRateLimiter_SteadyRate(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      5.0, // 5 tokens per second
		BurstSize: 1,   // only 1 token capacity (no burst)
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// First request should be allowed
	if !limiter.IsAllowed(identifier) {
		t.Error("First request should be allowed")
	}

	// Second request should be denied (no burst capacity)
	if limiter.IsAllowed(identifier) {
		t.Error("Second request should be denied (no burst)")
	}

	// Wait for one token to refill (200ms = 1 token at 5 tokens/sec)
	time.Sleep(200 * time.Millisecond)

	// Should be allowed again
	if !limiter.IsAllowed(identifier) {
		t.Error("Request should be allowed after steady rate refill")
	}
}

func TestTokenBucketRateLimiter_MultipleIdentifiers(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      2.0,
		BurstSize: 3,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	// Each identifier should have its own bucket
	for i := 0; i < 3; i++ {
		identifier := fmt.Sprintf("client-%d", i)
		
		// Each client should be able to make 3 requests (burst capacity)
		for j := 0; j < 3; j++ {
			if !limiter.IsAllowed(identifier) {
				t.Errorf("Client %d request %d should be allowed", i, j+1)
			}
		}
		
		// 4th request should be denied
		if limiter.IsAllowed(identifier) {
			t.Errorf("Client %d 4th request should be denied", i)
		}
	}
}

func TestTokenBucketRateLimiter_GetQuota(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      5.0,
		BurstSize: 10,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "test-client"

	// Check initial quota
	quota := limiter.GetQuota(identifier)
	if quota.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", quota.Limit)
	}
	if quota.Remaining != 10 {
		t.Errorf("Expected remaining 10, got %d", quota.Remaining)
	}

	// Consume some tokens
	for i := 0; i < 3; i++ {
		limiter.IsAllowed(identifier)
	}

	// Check quota after consumption
	quota = limiter.GetQuota(identifier)
	if quota.Remaining != 7 {
		t.Errorf("Expected remaining 7, got %d", quota.Remaining)
	}
}

func TestTokenBucketRateLimiter_GetStats(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      5.0,
		BurstSize: 10,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	// Make requests from different identifiers
	limiter.IsAllowed("client1")
	limiter.IsAllowed("client2")
	limiter.IsAllowed("client3")

	stats := limiter.GetStats()
	if stats.Algorithm != "token_bucket" {
		t.Errorf("Expected algorithm 'token_bucket', got %s", stats.Algorithm)
	}
	if stats.TotalIdentifiers != 3 {
		t.Errorf("Expected 3 identifiers, got %d", stats.TotalIdentifiers)
	}
	if stats.MaxRequests != 10 {
		t.Errorf("Expected max requests 10, got %d", stats.MaxRequests)
	}
}

func TestTokenBucketRateLimiter_ConcurrentAccess(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      100.0,
		BurstSize: 50,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "concurrent-client"
	numGoroutines := 10
	requestsPerGoroutine := 10
	
	var wg sync.WaitGroup
	var allowedCount int32
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localAllowed := 0
			
			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.IsAllowed(identifier) {
					localAllowed++
				}
			}
			
			mu.Lock()
			allowedCount += int32(localAllowed)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Should not exceed burst capacity
	if allowedCount > 50 {
		t.Errorf("Allowed count %d exceeds burst capacity 50", allowedCount)
	}

	// Should allow at least some requests
	if allowedCount == 0 {
		t.Error("No requests were allowed")
	}
}

func TestTokenBucketRateLimiter_Stop(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(nil)

	// Make some requests to populate buckets
	limiter.IsAllowed("client1")
	limiter.IsAllowed("client2")

	// Stop the limiter
	limiter.Stop()

	// Verify cleanup
	limiter.mu.RLock()
	bucketCount := len(limiter.buckets)
	limiter.mu.RUnlock()

	if bucketCount != 0 {
		t.Errorf("Expected 0 buckets after stop, got %d", bucketCount)
	}
}

func TestTokenBucketRateLimiter_Cleanup(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:            5.0,
		BurstSize:       10,
		CleanupInterval: 100 * time.Millisecond, // Very short for testing
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	// Make requests from multiple clients
	for i := 0; i < 5; i++ {
		identifier := fmt.Sprintf("client-%d", i)
		limiter.IsAllowed(identifier)
	}

	// Verify buckets exist
	limiter.mu.RLock()
	initialCount := len(limiter.buckets)
	limiter.mu.RUnlock()

	if initialCount != 5 {
		t.Errorf("Expected 5 buckets, got %d", initialCount)
	}

	// Wait for cleanup to run (buckets should be cleaned after 10 minutes of inactivity)
	// Since we can't wait 10 minutes, we'll test the cleanup logic directly
	limiter.performCleanup() // This won't clean anything yet since buckets are recent

	limiter.mu.RLock()
	afterCleanupCount := len(limiter.buckets)
	limiter.mu.RUnlock()

	if afterCleanupCount != 5 {
		t.Errorf("Expected 5 buckets after cleanup (too recent), got %d", afterCleanupCount)
	}
}

func TestTokenBucketRateLimiter_BurstThenSteady(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      2.0, // 2 tokens per second
		BurstSize: 5,   // 5 token burst
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "burst-client"

	// Test burst capacity - should allow 5 requests immediately
	allowedInBurst := 0
	for i := 0; i < 10; i++ {
		if limiter.IsAllowed(identifier) {
			allowedInBurst++
		}
	}

	if allowedInBurst != 5 {
		t.Errorf("Expected 5 requests allowed in burst, got %d", allowedInBurst)
	}

	// Wait for 1 second (should get 2 more tokens)
	time.Sleep(1 * time.Second)

	// Should allow 2 more requests
	allowedAfterWait := 0
	for i := 0; i < 5; i++ {
		if limiter.IsAllowed(identifier) {
			allowedAfterWait++
		}
	}

	if allowedAfterWait != 2 {
		t.Errorf("Expected 2 requests allowed after 1 second, got %d", allowedAfterWait)
	}
}

func TestTokenBucketRateLimiter_ZeroRate(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      0.0, // Should be corrected to 1.0
		BurstSize: 3,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	if limiter.rate != 1.0 {
		t.Errorf("Expected rate to be corrected to 1.0, got %f", limiter.rate)
	}
}

func TestTokenBucketRateLimiter_HighConcurrency(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      50.0,
		BurstSize: 100,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	numGoroutines := 50
	requestsPerGoroutine := 5
	var allowedCount int64

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			identifier := fmt.Sprintf("client-%d", clientID)

			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.IsAllowed(identifier) {
					atomic.AddInt64(&allowedCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Each client should be able to make some requests (at least burst capacity per client)
	// With 50 clients and 100 burst each, we should see significant allowed requests
	if allowedCount == 0 {
		t.Error("No requests were allowed in high concurrency test")
	}

	t.Logf("High concurrency test: %d requests allowed out of %d total",
		allowedCount, numGoroutines*requestsPerGoroutine)
}

func TestTokenBucketRateLimiter_QuotaAccuracy(t *testing.T) {
	config := &TokenBucketConfig{
		Rate:      1.0, // 1 token per second
		BurstSize: 5,
	}

	limiter := NewTokenBucketRateLimiter(config)
	defer limiter.Stop()

	identifier := "quota-client"

	// Initial quota should be full
	quota := limiter.GetQuota(identifier)
	if quota.Remaining != 5 {
		t.Errorf("Expected initial remaining 5, got %d", quota.Remaining)
	}

	// Consume 3 tokens
	for i := 0; i < 3; i++ {
		if !limiter.IsAllowed(identifier) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Check quota after consumption
	quota = limiter.GetQuota(identifier)
	if quota.Remaining != 2 {
		t.Errorf("Expected remaining 2 after consuming 3, got %d", quota.Remaining)
	}

	// Wait for 2 seconds (should refill 2 tokens, but cap at burst size)
	time.Sleep(2 * time.Second)

	quota = limiter.GetQuota(identifier)
	if quota.Remaining != 4 {
		t.Errorf("Expected remaining 4 after 2 second refill, got %d", quota.Remaining)
	}
}
