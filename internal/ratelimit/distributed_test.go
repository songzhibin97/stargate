package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
)

// MockAtomicStore implements store.AtomicStore interface for testing
type MockAtomicStore struct {
	data   map[string]*mockEntry
	mu     sync.RWMutex
	closed bool
}

type mockEntry struct {
	value     []byte
	expiresAt time.Time
	hasExpiry bool
}

func NewMockAtomicStore() *MockAtomicStore {
	return &MockAtomicStore{
		data: make(map[string]*mockEntry),
	}
}

// IncrBy atomically increments the value of a key by the given amount
func (mas *MockAtomicStore) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	if mas.closed {
		return 0, fmt.Errorf("storage is closed")
	}

	entry, exists := mas.data[key]
	var currentValue int64

	if !exists || (entry.hasExpiry && time.Now().After(entry.expiresAt)) {
		// Key doesn't exist or expired, create new with increment value
		newValue := value
		mas.data[key] = &mockEntry{
			value:     []byte(fmt.Sprintf("%d", newValue)),
			hasExpiry: false,
		}
		return newValue, nil
	}

	// Parse existing value
	if val, err := strconv.ParseInt(string(entry.value), 10, 64); err == nil {
		currentValue = val
	}

	// Increment the value
	newValue := currentValue + value
	entry.value = []byte(fmt.Sprintf("%d", newValue))

	return newValue, nil
}

// Set stores a value by key with optional TTL
func (mas *MockAtomicStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	if mas.closed {
		return fmt.Errorf("storage is closed")
	}

	entry := &mockEntry{
		value:     make([]byte, len(value)),
		hasExpiry: ttl > 0,
	}

	copy(entry.value, value)

	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	mas.data[key] = entry
	return nil
}

// Get retrieves a value by key
func (mas *MockAtomicStore) Get(ctx context.Context, key string) ([]byte, error) {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	if mas.closed {
		return nil, fmt.Errorf("storage is closed")
	}

	entry, exists := mas.data[key]
	if !exists {
		return nil, nil
	}

	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	// Return a copy of the value
	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, nil
}

// TTL returns the remaining time to live for a key
func (mas *MockAtomicStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	if mas.closed {
		return 0, fmt.Errorf("storage is closed")
	}

	entry, exists := mas.data[key]
	if !exists {
		return -2 * time.Second, nil // Key doesn't exist
	}

	if !entry.hasExpiry {
		return -1 * time.Second, nil // Key has no expiration
	}

	if time.Now().After(entry.expiresAt) {
		return -2 * time.Second, nil // Key doesn't exist (expired)
	}

	return time.Until(entry.expiresAt), nil
}

// Delete removes a key from storage
func (mas *MockAtomicStore) Delete(ctx context.Context, key string) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	if mas.closed {
		return fmt.Errorf("storage is closed")
	}

	delete(mas.data, key)
	return nil
}

// Exists checks if a key exists in storage
func (mas *MockAtomicStore) Exists(ctx context.Context, key string) (bool, error) {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	if mas.closed {
		return false, fmt.Errorf("storage is closed")
	}

	entry, exists := mas.data[key]
	if !exists {
		return false, nil
	}
	
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		return false, nil
	}
	
	return true, nil
}

// Close closes the store connection and releases resources
func (mas *MockAtomicStore) Close() error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	mas.closed = true
	mas.data = make(map[string]*mockEntry)
	return nil
}

// Health returns the health status of the store
func (mas *MockAtomicStore) Health(ctx context.Context) store.HealthStatus {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	status := "healthy"
	if mas.closed {
		status = "unhealthy"
	}

	return store.HealthStatus{
		Status:    status,
		Message:   "Mock atomic store is operational",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"type":       "mock",
			"keys_count": len(mas.data),
			"closed":     mas.closed,
		},
	}
}

func TestDistributedRateLimiter_FixedWindow(t *testing.T) {
	// Create shared storage (simulates Redis)
	storage := NewMockAtomicStore()
	defer storage.Close()

	config := &DistributedConfig{
		Strategy:    StrategyFixedWindow,
		WindowSize:  100 * time.Millisecond,
		MaxRequests: 3,
		KeyPrefix:   "test:",
	}

	// Create two limiter instances (simulates multiple service instances)
	limiter1 := NewDistributedRateLimiter(storage, config)
	defer limiter1.Stop()

	limiter2 := NewDistributedRateLimiter(storage, config)
	defer limiter2.Stop()
	
	identifier := "test-client"
	
	// Both limiters should share the same counter
	// First limiter makes 2 requests
	if !limiter1.IsAllowed(identifier) {
		t.Error("First request from limiter1 should be allowed")
	}
	if !limiter1.IsAllowed(identifier) {
		t.Error("Second request from limiter1 should be allowed")
	}
	
	// Second limiter makes 1 request (should be the 3rd total)
	if !limiter2.IsAllowed(identifier) {
		t.Error("First request from limiter2 should be allowed")
	}
	
	// Next request from either limiter should be denied
	if limiter1.IsAllowed(identifier) {
		t.Error("Fourth request should be denied (limit reached)")
	}
	if limiter2.IsAllowed(identifier) {
		t.Error("Fifth request should be denied (limit reached)")
	}
}

func TestDistributedRateLimiter_TokenBucket(t *testing.T) {
	storage := NewMockAtomicStore()
	defer storage.Close()

	config := &DistributedConfig{
		Strategy:  StrategyTokenBucket,
		Rate:      10.0, // 10 tokens per second
		BurstSize: 5,    // 5 token burst
		KeyPrefix: "test:",
	}

	limiter1 := NewDistributedRateLimiter(storage, config)
	defer limiter1.Stop()

	limiter2 := NewDistributedRateLimiter(storage, config)
	defer limiter2.Stop()
	
	identifier := "test-client"
	
	// Both limiters should share the same token bucket
	// Consume all tokens across both limiters
	allowedCount := 0
	for i := 0; i < 10; i++ {
		var allowed bool
		if i%2 == 0 {
			allowed = limiter1.IsAllowed(identifier)
		} else {
			allowed = limiter2.IsAllowed(identifier)
		}
		
		if allowed {
			allowedCount++
		}
	}
	
	// Should allow exactly 5 requests (burst capacity)
	if allowedCount != 5 {
		t.Errorf("Expected 5 allowed requests, got %d", allowedCount)
	}
}

func TestDistributedRateLimiter_ConcurrentAccess(t *testing.T) {
	storage := NewMockAtomicStore()
	defer storage.Close()

	config := &DistributedConfig{
		Strategy:    StrategyFixedWindow,
		WindowSize:  1 * time.Second,
		MaxRequests: 100,
		KeyPrefix:   "test:",
	}

	// Create multiple limiter instances
	numLimiters := 5
	limiters := make([]*DistributedRateLimiter, numLimiters)

	for i := 0; i < numLimiters; i++ {
		limiter := NewDistributedRateLimiter(storage, config)
		limiters[i] = limiter
		defer limiter.Stop()
	}
	
	identifier := "concurrent-client"
	requestsPerLimiter := 50
	
	var wg sync.WaitGroup
	var totalAllowed int64
	var mu sync.Mutex
	
	// Make concurrent requests from all limiters
	for i, limiter := range limiters {
		wg.Add(1)
		go func(limiterIndex int, l *DistributedRateLimiter) {
			defer wg.Done()
			
			localAllowed := 0
			for j := 0; j < requestsPerLimiter; j++ {
				if l.IsAllowed(identifier) {
					localAllowed++
				}
			}
			
			mu.Lock()
			totalAllowed += int64(localAllowed)
			mu.Unlock()
		}(i, limiter)
	}
	
	wg.Wait()
	
	// Total allowed should not exceed the limit
	if totalAllowed > 100 {
		t.Errorf("Total allowed requests %d exceeds limit 100", totalAllowed)
	}
	
	// Should allow some requests
	if totalAllowed == 0 {
		t.Error("No requests were allowed")
	}
	
	t.Logf("Concurrent test: %d requests allowed out of %d total", 
		totalAllowed, numLimiters*requestsPerLimiter)
}
