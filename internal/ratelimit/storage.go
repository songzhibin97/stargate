package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Storage defines the interface for rate limiting storage backends
type Storage interface {
	// Increment increments the counter for the given key and returns the new value
	// If the key doesn't exist, it creates it with value 1
	Increment(ctx context.Context, key string) (int64, error)
	
	// IncrementWithExpiry increments the counter and sets expiry if it's a new key
	// Returns the new value and whether the key was newly created
	IncrementWithExpiry(ctx context.Context, key string, expiry time.Duration) (int64, bool, error)
	
	// Get retrieves the current value for the given key
	// Returns 0 if the key doesn't exist
	Get(ctx context.Context, key string) (int64, error)
	
	// Set sets the value for the given key with optional expiry
	Set(ctx context.Context, key string, value int64, expiry time.Duration) error
	
	// Delete removes the key from storage
	Delete(ctx context.Context, key string) error
	
	// Exists checks if a key exists in storage
	Exists(ctx context.Context, key string) (bool, error)
	
	// GetWithTTL retrieves the value and remaining TTL for a key
	// Returns value, TTL (0 if no expiry), and error
	GetWithTTL(ctx context.Context, key string) (int64, time.Duration, error)
	
	// SetExpiry sets or updates the expiry time for an existing key
	SetExpiry(ctx context.Context, key string, expiry time.Duration) error
	
	// Close closes the storage connection and cleans up resources
	Close() error
	
	// Health returns the health status of the storage backend
	Health() map[string]interface{}
}

// MemoryStorage implements Storage interface using in-memory storage
type MemoryStorage struct {
	data    map[string]*memoryEntry
	mu      sync.RWMutex
	stopCh  chan struct{}
	started bool
}

type memoryEntry struct {
	value     int64
	expiresAt time.Time
	hasExpiry bool
}

// NewMemoryStorage creates a new in-memory storage backend
func NewMemoryStorage() *MemoryStorage {
	ms := &MemoryStorage{
		data:   make(map[string]*memoryEntry),
		stopCh: make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go ms.cleanupExpired()
	ms.started = true
	
	return ms
}

// Increment increments the counter for the given key
func (ms *MemoryStorage) Increment(ctx context.Context, key string) (int64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	entry, exists := ms.data[key]
	if !exists {
		ms.data[key] = &memoryEntry{value: 1}
		return 1, nil
	}
	
	// Check if expired
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		ms.data[key] = &memoryEntry{value: 1}
		return 1, nil
	}
	
	entry.value++
	return entry.value, nil
}

// IncrementWithExpiry increments the counter and sets expiry if it's a new key
func (ms *MemoryStorage) IncrementWithExpiry(ctx context.Context, key string, expiry time.Duration) (int64, bool, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	entry, exists := ms.data[key]
	if !exists {
		ms.data[key] = &memoryEntry{
			value:     1,
			expiresAt: time.Now().Add(expiry),
			hasExpiry: expiry > 0,
		}
		return 1, true, nil
	}
	
	// Check if expired
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		ms.data[key] = &memoryEntry{
			value:     1,
			expiresAt: time.Now().Add(expiry),
			hasExpiry: expiry > 0,
		}
		return 1, true, nil
	}
	
	entry.value++
	return entry.value, false, nil
}

// Get retrieves the current value for the given key
func (ms *MemoryStorage) Get(ctx context.Context, key string) (int64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	entry, exists := ms.data[key]
	if !exists {
		return 0, nil
	}
	
	// Check if expired
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		return 0, nil
	}
	
	return entry.value, nil
}

// Set sets the value for the given key with optional expiry
func (ms *MemoryStorage) Set(ctx context.Context, key string, value int64, expiry time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	entry := &memoryEntry{
		value:     value,
		hasExpiry: expiry > 0,
	}
	
	if expiry > 0 {
		entry.expiresAt = time.Now().Add(expiry)
	}
	
	ms.data[key] = entry
	return nil
}

// Delete removes the key from storage
func (ms *MemoryStorage) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	delete(ms.data, key)
	return nil
}

// Exists checks if a key exists in storage
func (ms *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	entry, exists := ms.data[key]
	if !exists {
		return false, nil
	}
	
	// Check if expired
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		return false, nil
	}
	
	return true, nil
}

// GetWithTTL retrieves the value and remaining TTL for a key
func (ms *MemoryStorage) GetWithTTL(ctx context.Context, key string) (int64, time.Duration, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	entry, exists := ms.data[key]
	if !exists {
		return 0, 0, nil
	}
	
	now := time.Now()
	if entry.hasExpiry && now.After(entry.expiresAt) {
		return 0, 0, nil
	}
	
	var ttl time.Duration
	if entry.hasExpiry {
		ttl = entry.expiresAt.Sub(now)
		if ttl < 0 {
			ttl = 0
		}
	}
	
	return entry.value, ttl, nil
}

// SetExpiry sets or updates the expiry time for an existing key
func (ms *MemoryStorage) SetExpiry(ctx context.Context, key string, expiry time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	entry, exists := ms.data[key]
	if !exists {
		return nil // Key doesn't exist, nothing to do
	}
	
	if expiry > 0 {
		entry.expiresAt = time.Now().Add(expiry)
		entry.hasExpiry = true
	} else {
		entry.hasExpiry = false
	}
	
	return nil
}

// Close closes the storage and cleans up resources
func (ms *MemoryStorage) Close() error {
	if ms.started {
		close(ms.stopCh)
		ms.started = false
	}
	
	ms.mu.Lock()
	ms.data = make(map[string]*memoryEntry)
	ms.mu.Unlock()
	
	return nil
}

// Health returns the health status of the memory storage
func (ms *MemoryStorage) Health() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	return map[string]interface{}{
		"status":     "healthy",
		"type":       "memory",
		"keys_count": len(ms.data),
	}
}

// cleanupExpired removes expired entries periodically
func (ms *MemoryStorage) cleanupExpired() {
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
func (ms *MemoryStorage) performCleanup() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	now := time.Now()
	for key, entry := range ms.data {
		if entry.hasExpiry && now.After(entry.expiresAt) {
			delete(ms.data, key)
		}
	}
}
