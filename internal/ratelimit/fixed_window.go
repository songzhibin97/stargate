package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// FixedWindowRateLimiter implements a fixed window rate limiting algorithm
// It uses a map to store request counts for each identifier within fixed time windows
type FixedWindowRateLimiter struct {
	mu           sync.RWMutex
	windows      map[string]*windowData // key: identifier, value: window data
	windowSize   time.Duration          // size of each time window
	maxRequests  int                    // maximum requests allowed per window
	cleanupTicker *time.Ticker          // ticker for cleanup expired windows
	stopCh       chan struct{}          // channel to stop cleanup goroutine
}

// windowData represents the data for a single time window
type windowData struct {
	count     int       // current request count in this window
	windowStart time.Time // start time of the current window
}

// FixedWindowConfig represents configuration for fixed window rate limiter
type FixedWindowConfig struct {
	WindowSize   time.Duration // duration of each window (e.g., 1 minute)
	MaxRequests  int           // maximum requests allowed per window
	CleanupInterval time.Duration // how often to clean up expired windows
}

// NewFixedWindowRateLimiter creates a new fixed window rate limiter
func NewFixedWindowRateLimiter(config *FixedWindowConfig) *FixedWindowRateLimiter {
	if config == nil {
		config = &FixedWindowConfig{
			WindowSize:      time.Minute,
			MaxRequests:     100,
			CleanupInterval: 5 * time.Minute,
		}
	}

	limiter := &FixedWindowRateLimiter{
		windows:     make(map[string]*windowData),
		windowSize:  config.WindowSize,
		maxRequests: config.MaxRequests,
		stopCh:      make(chan struct{}),
	}

	// Start cleanup goroutine
	limiter.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go limiter.cleanupExpiredWindows()

	return limiter
}

// IsAllowed checks if a request from the given identifier is allowed
// Returns true if allowed, false if rate limited
func (fw *FixedWindowRateLimiter) IsAllowed(identifier string) bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()
	
	// Get or create window data for this identifier
	window, exists := fw.windows[identifier]
	if !exists {
		// First request from this identifier
		fw.windows[identifier] = &windowData{
			count:       1,
			windowStart: fw.getWindowStart(now),
		}
		return true
	}

	// Check if we're still in the same window
	currentWindowStart := fw.getWindowStart(now)
	if window.windowStart.Equal(currentWindowStart) {
		// Same window, check if we can allow more requests
		if window.count >= fw.maxRequests {
			return false // Rate limited
		}
		window.count++
		return true
	} else {
		// New window, reset counter
		window.count = 1
		window.windowStart = currentWindowStart
		return true
	}
}

// GetQuota returns the current quota information for an identifier
func (fw *FixedWindowRateLimiter) GetQuota(identifier string) *QuotaInfo {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	now := time.Now()
	currentWindowStart := fw.getWindowStart(now)
	
	window, exists := fw.windows[identifier]
	if !exists {
		// No requests yet from this identifier
		return &QuotaInfo{
			Limit:     fw.maxRequests,
			Remaining: fw.maxRequests,
			ResetTime: currentWindowStart.Add(fw.windowSize),
			WindowStart: currentWindowStart,
		}
	}

	// Check if we're in the same window
	if window.windowStart.Equal(currentWindowStart) {
		remaining := fw.maxRequests - window.count
		if remaining < 0 {
			remaining = 0
		}
		return &QuotaInfo{
			Limit:     fw.maxRequests,
			Remaining: remaining,
			ResetTime: window.windowStart.Add(fw.windowSize),
			WindowStart: window.windowStart,
		}
	} else {
		// New window, full quota available
		return &QuotaInfo{
			Limit:     fw.maxRequests,
			Remaining: fw.maxRequests,
			ResetTime: currentWindowStart.Add(fw.windowSize),
			WindowStart: currentWindowStart,
		}
	}
}

// getWindowStart calculates the start time of the window for a given time
func (fw *FixedWindowRateLimiter) getWindowStart(t time.Time) time.Time {
	// Calculate window start by truncating to window size
	// Use nanoseconds for better precision with small window sizes
	windowSizeNanos := int64(fw.windowSize.Nanoseconds())
	if windowSizeNanos == 0 {
		windowSizeNanos = int64(time.Second.Nanoseconds()) // Prevent division by zero
	}
	windowStartNanos := t.UnixNano() / windowSizeNanos * windowSizeNanos
	return time.Unix(0, windowStartNanos)
}

// cleanupExpiredWindows removes expired window data to prevent memory leaks
func (fw *FixedWindowRateLimiter) cleanupExpiredWindows() {
	for {
		select {
		case <-fw.cleanupTicker.C:
			fw.performCleanup()
		case <-fw.stopCh:
			return
		}
	}
}

// performCleanup removes expired windows
func (fw *FixedWindowRateLimiter) performCleanup() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()
	currentWindowStart := fw.getWindowStart(now)
	
	// Remove windows that are older than the current window
	for identifier, window := range fw.windows {
		if window.windowStart.Before(currentWindowStart) {
			delete(fw.windows, identifier)
		}
	}
}

// Stop stops the rate limiter and cleans up resources
func (fw *FixedWindowRateLimiter) Stop() {
	if fw.cleanupTicker != nil {
		fw.cleanupTicker.Stop()
	}
	close(fw.stopCh)
}

// GetStats returns statistics about the rate limiter
func (fw *FixedWindowRateLimiter) GetStats() *RateLimiterStats {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	activeWindows := 0
	totalRequests := 0
	
	now := time.Now()
	currentWindowStart := fw.getWindowStart(now)
	
	for _, window := range fw.windows {
		if window.windowStart.Equal(currentWindowStart) {
			activeWindows++
			totalRequests += window.count
		}
	}

	return &RateLimiterStats{
		Algorithm:      "fixed_window",
		ActiveWindows:  activeWindows,
		TotalIdentifiers: len(fw.windows),
		TotalRequests:  totalRequests,
		WindowSize:     fw.windowSize,
		MaxRequests:    fw.maxRequests,
	}
}

// QuotaInfo represents quota information for an identifier
type QuotaInfo struct {
	Limit       int           // maximum requests allowed in the window
	Remaining   int           // remaining requests in the current window
	ResetTime   time.Time     // when the window resets
	WindowStart time.Time     // start of the current window
}

// RateLimiterStats represents statistics about the rate limiter
type RateLimiterStats struct {
	Algorithm        string        // rate limiting algorithm name
	ActiveWindows    int           // number of active windows
	TotalIdentifiers int           // total number of tracked identifiers
	TotalRequests    int           // total requests in current windows
	WindowSize       time.Duration // size of each window
	MaxRequests      int           // maximum requests per window
	StorageHealth    string        // health status of storage backend (for distributed limiters)
}

// ExtractIdentifier extracts an identifier from an HTTP request
// This function determines how to identify clients (IP, user ID, API key, etc.)
func ExtractIdentifier(r *http.Request, strategy string) string {
	switch strategy {
	case "ip":
		return extractClientIP(r)
	case "user":
		// Extract user ID from authentication context
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			return "user:" + userID
		}
		// Fallback to IP if no user ID
		return "ip:" + extractClientIP(r)
	case "api_key":
		// Extract API key
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			return "api_key:" + apiKey
		}
		// Fallback to IP if no API key
		return "ip:" + extractClientIP(r)
	case "combined":
		// Combine multiple identifiers
		ip := extractClientIP(r)
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			return fmt.Sprintf("user:%s:ip:%s", userID, ip)
		}
		return "ip:" + ip
	default:
		// Default to IP-based identification
		return extractClientIP(r)
	}
}

// extractClientIP extracts the client IP address from the request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := len(xff); idx > 0 {
			if commaIdx := 0; commaIdx < idx {
				for i, c := range xff {
					if c == ',' {
						commaIdx = i
						break
					}
				}
				if commaIdx > 0 {
					return xff[:commaIdx]
				}
			}
			return xff
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Check X-Forwarded header
	if xf := r.Header.Get("X-Forwarded"); xf != "" {
		return xf
	}

	// Fall back to RemoteAddr
	if r.RemoteAddr != "" {
		// RemoteAddr is in format "IP:port", extract just the IP
		if colonIdx := len(r.RemoteAddr) - 1; colonIdx >= 0 {
			for i := colonIdx; i >= 0; i-- {
				if r.RemoteAddr[i] == ':' {
					return r.RemoteAddr[:i]
				}
			}
		}
		return r.RemoteAddr
	}

	return "unknown"
}

// IsRateLimited checks if the response indicates rate limiting
func IsRateLimited(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests
}

// SetRateLimitHeaders sets standard rate limit headers on the response
func SetRateLimitHeaders(w http.ResponseWriter, quota *QuotaInfo) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", quota.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", quota.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", quota.ResetTime.Unix()))
	w.Header().Set("X-RateLimit-Window", quota.WindowStart.Format(time.RFC3339))
}
