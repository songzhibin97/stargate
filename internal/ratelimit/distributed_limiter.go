package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
)

// DistributedRateLimiter implements rate limiting using store.AtomicStore interface
// It supports both fixed window and token bucket algorithms with distributed storage
type DistributedRateLimiter struct {
	store     store.AtomicStore
	strategy  RateLimitStrategy
	config    *DistributedConfig
	keyPrefix string
}

// DistributedConfig represents configuration for distributed rate limiter
type DistributedConfig struct {
	// Common settings
	Strategy           RateLimitStrategy
	IdentifierStrategy IdentifierStrategy

	// Fixed window settings
	WindowSize   time.Duration
	MaxRequests  int

	// Token bucket settings
	Rate      float64
	BurstSize int

	// Storage settings
	KeyPrefix string
}

// NewDistributedRateLimiter creates a new distributed rate limiter with the given store
func NewDistributedRateLimiter(store store.AtomicStore, config *DistributedConfig) *DistributedRateLimiter {
	if config == nil {
		config = &DistributedConfig{
			Strategy:    StrategyFixedWindow,
			WindowSize:  time.Minute,
			MaxRequests: 100,
			KeyPrefix:   "ratelimit:",
		}
	}

	return &DistributedRateLimiter{
		store:     store,
		strategy:  config.Strategy,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}
}



// IsAllowed checks if a request from the given identifier is allowed
func (drl *DistributedRateLimiter) IsAllowed(identifier string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	switch drl.strategy {
	case StrategyFixedWindow:
		return drl.isAllowedFixedWindow(ctx, identifier)
	case StrategyTokenBucket:
		return drl.isAllowedTokenBucket(ctx, identifier)
	default:
		return false
	}
}

// isAllowedFixedWindow implements fixed window algorithm using distributed storage
func (drl *DistributedRateLimiter) isAllowedFixedWindow(ctx context.Context, identifier string) bool {
	// Calculate window key based on current time
	now := time.Now()
	windowStart := drl.getWindowStart(now)
	windowKey := fmt.Sprintf("%s%s:fw:%d", drl.keyPrefix, identifier, windowStart.Unix())

	// Check if key exists, if not, set it with TTL and increment
	exists, err := drl.store.Exists(ctx, windowKey)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}

	if !exists {
		// Set initial value with TTL
		err = drl.store.Set(ctx, windowKey, []byte("0"), drl.config.WindowSize)
		if err != nil {
			return true
		}
	}

	// Increment counter
	count, err := drl.store.IncrBy(ctx, windowKey, 1)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}

	return count <= int64(drl.config.MaxRequests)
}

// isAllowedTokenBucket implements token bucket algorithm using distributed storage
func (drl *DistributedRateLimiter) isAllowedTokenBucket(ctx context.Context, identifier string) bool {
	tokenKey := fmt.Sprintf("%s%s:tb:tokens", drl.keyPrefix, identifier)
	lastRefillKey := fmt.Sprintf("%s%s:tb:last", drl.keyPrefix, identifier)

	now := time.Now()
	nowUnix := now.Unix()

	// Get current tokens and last refill time
	tokensData, err := drl.store.Get(ctx, tokenKey)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}

	lastRefillData, err := drl.store.Get(ctx, lastRefillKey)
	if err != nil {
		lastRefillData = nil
	}

	var tokens int64
	var lastRefill int64

	// Parse tokens
	if tokensData != nil {
		if val, parseErr := strconv.ParseInt(string(tokensData), 10, 64); parseErr == nil {
			tokens = val
		}
	}

	// Parse last refill time
	var isFirstRequest bool
	if lastRefillData != nil {
		if val, parseErr := strconv.ParseInt(string(lastRefillData), 10, 64); parseErr == nil {
			lastRefill = val
		} else {
			isFirstRequest = true
		}
	} else {
		isFirstRequest = true
	}

	// If this is the first request, initialize with full bucket
	if tokens == 0 && isFirstRequest {
		tokens = int64(drl.config.BurstSize)
		lastRefill = nowUnix
	} else if isFirstRequest {
		lastRefill = nowUnix
	}

	// Calculate tokens to add based on elapsed time
	elapsed := float64(nowUnix - lastRefill)
	tokensToAdd := elapsed * drl.config.Rate
	newTokens := float64(tokens) + tokensToAdd

	// Cap at burst size
	if newTokens > float64(drl.config.BurstSize) {
		newTokens = float64(drl.config.BurstSize)
	}

	// Check if we have at least one token
	if newTokens < 1.0 {
		return false
	}

	// Consume one token
	newTokens -= 1.0

	// Update storage
	expiry := time.Duration(float64(drl.config.BurstSize)/drl.config.Rate) * time.Second
	if expiry < time.Minute {
		expiry = time.Minute
	}

	drl.store.Set(ctx, tokenKey, []byte(fmt.Sprintf("%d", int64(newTokens))), expiry)
	drl.store.Set(ctx, lastRefillKey, []byte(fmt.Sprintf("%d", nowUnix)), expiry)

	return true
}

// GetQuota returns the current quota information for an identifier
func (drl *DistributedRateLimiter) GetQuota(identifier string) *QuotaInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	switch drl.strategy {
	case StrategyFixedWindow:
		return drl.getQuotaFixedWindow(ctx, identifier)
	case StrategyTokenBucket:
		return drl.getQuotaTokenBucket(ctx, identifier)
	default:
		return &QuotaInfo{
			Limit:     -1,
			Remaining: -1,
			ResetTime: time.Time{},
		}
	}
}

// getQuotaFixedWindow returns quota info for fixed window algorithm
func (drl *DistributedRateLimiter) getQuotaFixedWindow(ctx context.Context, identifier string) *QuotaInfo {
	now := time.Now()
	windowStart := drl.getWindowStart(now)
	windowKey := fmt.Sprintf("%s%s:fw:%d", drl.keyPrefix, identifier, windowStart.Unix())

	// Get current count
	countData, err := drl.store.Get(ctx, windowKey)
	var count int64
	if err == nil && countData != nil {
		if val, parseErr := strconv.ParseInt(string(countData), 10, 64); parseErr == nil {
			count = val
		}
	}

	// Get TTL
	ttl, err := drl.store.TTL(ctx, windowKey)
	if err != nil || ttl < 0 {
		ttl = drl.config.WindowSize
	}

	remaining := drl.config.MaxRequests - int(count)
	if remaining < 0 {
		remaining = 0
	}

	resetTime := windowStart.Add(drl.config.WindowSize)
	if ttl > 0 {
		resetTime = now.Add(ttl)
	}

	return &QuotaInfo{
		Limit:       drl.config.MaxRequests,
		Remaining:   remaining,
		ResetTime:   resetTime,
		WindowStart: windowStart,
	}
}

// getQuotaTokenBucket returns quota info for token bucket algorithm
func (drl *DistributedRateLimiter) getQuotaTokenBucket(ctx context.Context, identifier string) *QuotaInfo {
	tokenKey := fmt.Sprintf("%s%s:tb:tokens", drl.keyPrefix, identifier)
	lastRefillKey := fmt.Sprintf("%s%s:tb:last", drl.keyPrefix, identifier)

	now := time.Now()
	nowUnix := now.Unix()

	// Get current tokens
	tokensData, err := drl.store.Get(ctx, tokenKey)
	var tokens int64 = int64(drl.config.BurstSize)
	if err == nil && tokensData != nil {
		if val, parseErr := strconv.ParseInt(string(tokensData), 10, 64); parseErr == nil {
			tokens = val
		}
	}

	// Get last refill time
	lastRefillData, err := drl.store.Get(ctx, lastRefillKey)
	var lastRefill int64 = nowUnix
	if err == nil && lastRefillData != nil {
		if val, parseErr := strconv.ParseInt(string(lastRefillData), 10, 64); parseErr == nil {
			lastRefill = val
		}
	}

	// Calculate current tokens after refill
	elapsed := float64(nowUnix - lastRefill)
	tokensToAdd := elapsed * drl.config.Rate
	currentTokens := float64(tokens) + tokensToAdd

	if currentTokens > float64(drl.config.BurstSize) {
		currentTokens = float64(drl.config.BurstSize)
	}

	remaining := int(currentTokens)
	if remaining < 0 {
		remaining = 0
	}

	// Calculate when bucket will be full
	tokensNeeded := float64(drl.config.BurstSize) - currentTokens
	var resetTime time.Time
	if tokensNeeded > 0 {
		secondsToFull := tokensNeeded / drl.config.Rate
		resetTime = now.Add(time.Duration(secondsToFull) * time.Second)
	} else {
		resetTime = now
	}

	return &QuotaInfo{
		Limit:       drl.config.BurstSize,
		Remaining:   remaining,
		ResetTime:   resetTime,
		WindowStart: time.Unix(lastRefill, 0),
	}
}

// GetStats returns statistics about the rate limiter
func (drl *DistributedRateLimiter) GetStats() *RateLimiterStats {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := drl.store.Health(ctx)

	return &RateLimiterStats{
		Algorithm:        fmt.Sprintf("distributed_%s", string(drl.strategy)),
		ActiveWindows:    -1, // Not easily calculable for distributed storage
		TotalIdentifiers: -1, // Not easily calculable for distributed storage
		TotalRequests:    -1, // Not easily calculable for distributed storage
		WindowSize:       drl.config.WindowSize,
		MaxRequests:      drl.config.MaxRequests,
		StorageHealth:    health.Status,
	}
}

// Stop stops the rate limiter and cleans up resources
func (drl *DistributedRateLimiter) Stop() {
	if drl.store != nil {
		drl.store.Close()
	}
}

// getWindowStart calculates the start time of the window for a given time
func (drl *DistributedRateLimiter) getWindowStart(t time.Time) time.Time {
	windowSizeNanos := int64(drl.config.WindowSize.Nanoseconds())
	if windowSizeNanos == 0 {
		windowSizeNanos = int64(time.Second.Nanoseconds())
	}
	windowStartNanos := t.UnixNano() / windowSizeNanos * windowSizeNanos
	return time.Unix(0, windowStartNanos)
}
