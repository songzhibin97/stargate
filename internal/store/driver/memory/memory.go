package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
)

// entry represents a single entry in the memory store
type entry struct {
	value     []byte
	expiresAt time.Time
	hasExpiry bool
}

// isExpired checks if the entry has expired
func (e *entry) isExpired() bool {
	return e.hasExpiry && time.Now().After(e.expiresAt)
}

// MemoryStore implements the store.Store interface using in-memory storage
type MemoryStore struct {
	data      map[string]*entry
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	keyPrefix string
	started   bool
}

// New creates a new in-memory store instance
func New(config *store.Config) (store.AtomicStore, error) {
	if config == nil {
		config = store.DefaultConfig()
	}

	ms := &MemoryStore{
		data:      make(map[string]*entry),
		stopCh:    make(chan struct{}),
		keyPrefix: config.KeyPrefix,
	}

	// Start cleanup goroutine
	ms.start()

	return ms, nil
}

// start starts the cleanup goroutine
func (ms *MemoryStore) start() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.started {
		return
	}

	ms.started = true
	ms.wg.Add(1)
	go ms.cleanupExpired()
}

// getKey returns the full key with prefix
func (ms *MemoryStore) getKey(key string) string {
	if ms.keyPrefix == "" {
		return key
	}
	return ms.keyPrefix + ":" + key
}

// IncrBy atomically increments the value of a key by the given amount
func (ms *MemoryStore) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	fullKey := ms.getKey(key)
	existingEntry, exists := ms.data[fullKey]

	// Check if entry exists and is not expired
	if !exists || existingEntry.isExpired() {
		// Create new entry with the increment value
		newValue := value
		ms.data[fullKey] = &entry{
			value:     []byte(fmt.Sprintf("%d", newValue)),
			hasExpiry: false,
		}
		return newValue, nil
	}

	// Parse existing value
	var currentValue int64
	if err := json.Unmarshal(existingEntry.value, &currentValue); err != nil {
		// If parsing fails, treat as string and try to parse as int64
		if val, parseErr := parseIntFromBytes(existingEntry.value); parseErr == nil {
			currentValue = val
		} else {
			return 0, fmt.Errorf("cannot increment non-numeric value: %w", err)
		}
	}

	// Increment the value
	newValue := currentValue + value
	existingEntry.value = []byte(fmt.Sprintf("%d", newValue))

	return newValue, nil
}

// Set stores a value by key with optional TTL
func (ms *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	fullKey := ms.getKey(key)
	entry := &entry{
		value:     make([]byte, len(value)),
		hasExpiry: ttl > 0,
	}

	copy(entry.value, value)

	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	ms.data[fullKey] = entry
	return nil
}

// Get retrieves a value by key
func (ms *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	fullKey := ms.getKey(key)
	existingEntry, exists := ms.data[fullKey]

	if !exists || existingEntry.isExpired() {
		return nil, nil
	}

	// Return a copy of the value to prevent external modification
	result := make([]byte, len(existingEntry.value))
	copy(result, existingEntry.value)
	return result, nil
}

// Delete removes a key from storage
func (ms *MemoryStore) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	fullKey := ms.getKey(key)
	delete(ms.data, fullKey)
	return nil
}

// Exists checks if a key exists in storage
func (ms *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	fullKey := ms.getKey(key)
	existingEntry, exists := ms.data[fullKey]

	if !exists || existingEntry.isExpired() {
		return false, nil
	}

	return true, nil
}

// TTL returns the remaining time to live for a key
func (ms *MemoryStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	fullKey := ms.getKey(key)
	existingEntry, exists := ms.data[fullKey]

	if !exists {
		return -2 * time.Second, nil // Key doesn't exist
	}

	if !existingEntry.hasExpiry {
		return -1 * time.Second, nil // Key has no expiration
	}

	if existingEntry.isExpired() {
		return -2 * time.Second, nil // Key doesn't exist (expired)
	}

	return time.Until(existingEntry.expiresAt), nil
}

// Close closes the store connection and releases resources
func (ms *MemoryStore) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.started {
		return nil
	}

	close(ms.stopCh)
	ms.started = false

	// Wait for cleanup goroutine to finish
	ms.wg.Wait()

	// Clear all data
	ms.data = make(map[string]*entry)

	return nil
}

// Health returns the health status of the store
func (ms *MemoryStore) Health(ctx context.Context) store.HealthStatus {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return store.HealthStatus{
		Status:    "healthy",
		Message:   "Memory store is operational",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"type":       "memory",
			"keys_count": len(ms.data),
			"started":    ms.started,
		},
	}
}

// cleanupExpired removes expired entries periodically
func (ms *MemoryStore) cleanupExpired() {
	defer ms.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.performCleanup()
		case <-ms.stopCh:
			return
		}
	}
}

// performCleanup removes expired entries
func (ms *MemoryStore) performCleanup() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	for key, entry := range ms.data {
		if entry.hasExpiry && now.After(entry.expiresAt) {
			delete(ms.data, key)
		}
	}
}

// parseIntFromBytes tries to parse an integer from byte slice
func parseIntFromBytes(data []byte) (int64, error) {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return 0, err
	}
	return value, nil
}
