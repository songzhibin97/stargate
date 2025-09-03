package ratelimit

import (
	"sync"
	"time"
)

// TokenBucketRateLimiter implements a token bucket rate limiting algorithm
// It allows for burst traffic up to the bucket capacity while maintaining a steady rate
type TokenBucketRateLimiter struct {
	mu           sync.RWMutex
	buckets      map[string]*bucketData // key: identifier, value: bucket data
	rate         float64                // tokens per second
	burstSize    int                    // maximum tokens in bucket
	cleanupTicker *time.Ticker          // ticker for cleanup expired buckets
	stopCh       chan struct{}          // channel to stop cleanup goroutine
}

// bucketData represents the data for a single token bucket
type bucketData struct {
	tokens     float64   // current number of tokens in the bucket
	lastRefill time.Time // last time tokens were added to the bucket
}

// TokenBucketConfig represents configuration for token bucket rate limiter
type TokenBucketConfig struct {
	Rate            float64       // tokens per second (requests per second)
	BurstSize       int           // maximum tokens in bucket (burst capacity)
	CleanupInterval time.Duration // how often to clean up expired buckets
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter
func NewTokenBucketRateLimiter(config *TokenBucketConfig) *TokenBucketRateLimiter {
	if config == nil {
		config = &TokenBucketConfig{
			Rate:            10.0,
			BurstSize:       20,
			CleanupInterval: 5 * time.Minute,
		}
	}

	// Ensure minimum values
	if config.Rate <= 0 {
		config.Rate = 1.0
	}
	if config.BurstSize <= 0 {
		config.BurstSize = int(config.Rate)
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	limiter := &TokenBucketRateLimiter{
		buckets:   make(map[string]*bucketData),
		rate:      config.Rate,
		burstSize: config.BurstSize,
		stopCh:    make(chan struct{}),
	}

	// Start cleanup goroutine
	limiter.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go limiter.cleanupExpiredBuckets()

	return limiter
}

// IsAllowed checks if a request from the given identifier is allowed
// Returns true if allowed (token consumed), false if rate limited
func (tb *TokenBucketRateLimiter) IsAllowed(identifier string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	
	// Get or create bucket data for this identifier
	bucket, exists := tb.buckets[identifier]
	if !exists {
		// First request from this identifier, create new bucket with full capacity
		tb.buckets[identifier] = &bucketData{
			tokens:     float64(tb.burstSize) - 1, // consume one token for this request
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	tb.refillTokens(bucket, now)

	// Check if we have at least one token
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0 // consume one token
		return true
	}

	// No tokens available, rate limited
	return false
}

// refillTokens adds tokens to the bucket based on elapsed time
// This method assumes the caller holds the write lock
func (tb *TokenBucketRateLimiter) refillTokens(bucket *bucketData, now time.Time) {
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}

	// Calculate tokens to add based on rate and elapsed time
	tokensToAdd := elapsed * tb.rate
	bucket.tokens += tokensToAdd

	// Cap at burst size
	if bucket.tokens > float64(tb.burstSize) {
		bucket.tokens = float64(tb.burstSize)
	}

	bucket.lastRefill = now
}

// GetQuota returns the current quota information for an identifier
func (tb *TokenBucketRateLimiter) GetQuota(identifier string) *QuotaInfo {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	now := time.Now()
	
	bucket, exists := tb.buckets[identifier]
	if !exists {
		// No requests yet from this identifier
		return &QuotaInfo{
			Limit:       tb.burstSize,
			Remaining:   tb.burstSize,
			ResetTime:   now.Add(time.Duration(float64(tb.burstSize)/tb.rate) * time.Second),
			WindowStart: now,
		}
	}

	// Create a copy to avoid modifying the original while holding read lock
	bucketCopy := &bucketData{
		tokens:     bucket.tokens,
		lastRefill: bucket.lastRefill,
	}

	// Simulate refill to get current token count
	tb.refillTokensReadOnly(bucketCopy, now)

	remaining := int(bucketCopy.tokens)
	if remaining < 0 {
		remaining = 0
	}

	// Calculate when the bucket will be full again
	tokensNeeded := float64(tb.burstSize) - bucketCopy.tokens
	var resetTime time.Time
	if tokensNeeded > 0 {
		secondsToFull := tokensNeeded / tb.rate
		resetTime = now.Add(time.Duration(secondsToFull) * time.Second)
	} else {
		resetTime = now
	}

	return &QuotaInfo{
		Limit:       tb.burstSize,
		Remaining:   remaining,
		ResetTime:   resetTime,
		WindowStart: bucket.lastRefill,
	}
}

// refillTokensReadOnly simulates token refill without modifying the original bucket
func (tb *TokenBucketRateLimiter) refillTokensReadOnly(bucket *bucketData, now time.Time) {
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}

	tokensToAdd := elapsed * tb.rate
	bucket.tokens += tokensToAdd

	if bucket.tokens > float64(tb.burstSize) {
		bucket.tokens = float64(tb.burstSize)
	}
}

// GetStats returns statistics about the rate limiter
func (tb *TokenBucketRateLimiter) GetStats() *RateLimiterStats {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	now := time.Now()
	activeBuckets := 0
	totalTokens := 0.0

	for _, bucket := range tb.buckets {
		// Create a copy for read-only refill simulation
		bucketCopy := &bucketData{
			tokens:     bucket.tokens,
			lastRefill: bucket.lastRefill,
		}
		tb.refillTokensReadOnly(bucketCopy, now)
		
		if bucketCopy.tokens > 0 {
			activeBuckets++
		}
		totalTokens += bucketCopy.tokens
	}

	return &RateLimiterStats{
		Algorithm:        "token_bucket",
		ActiveWindows:    activeBuckets,
		TotalIdentifiers: len(tb.buckets),
		TotalRequests:    int(totalTokens), // approximate
		WindowSize:       time.Duration(float64(tb.burstSize)/tb.rate) * time.Second,
		MaxRequests:      tb.burstSize,
	}
}

// Stop stops the rate limiter and cleans up resources
func (tb *TokenBucketRateLimiter) Stop() {
	if tb.cleanupTicker != nil {
		tb.cleanupTicker.Stop()
	}
	
	close(tb.stopCh)
	
	tb.mu.Lock()
	tb.buckets = make(map[string]*bucketData)
	tb.mu.Unlock()
}

// cleanupExpiredBuckets removes expired bucket data to prevent memory leaks
func (tb *TokenBucketRateLimiter) cleanupExpiredBuckets() {
	for {
		select {
		case <-tb.cleanupTicker.C:
			tb.performCleanup()
		case <-tb.stopCh:
			return
		}
	}
}

// performCleanup removes buckets that haven't been used for a long time
func (tb *TokenBucketRateLimiter) performCleanup() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// Remove buckets that haven't been accessed for more than 10 minutes
	expireThreshold := 10 * time.Minute

	for identifier, bucket := range tb.buckets {
		if now.Sub(bucket.lastRefill) > expireThreshold {
			delete(tb.buckets, identifier)
		}
	}
}
