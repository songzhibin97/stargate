package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store/driver/memory"
	pkgstore "github.com/songzhibin97/stargate/pkg/store"
)

// MemoryStore implements the Store interface using in-memory storage
type MemoryStore struct {
	atomicStore pkgstore.AtomicStore
	mu          sync.RWMutex
	watchers    map[string][]WatchCallback
	stopCh      chan struct{}
	started     bool
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(cfg *config.Config) (*MemoryStore, error) {
	// Create store config
	storeConfig := &pkgstore.Config{
		Type:      "memory",
		KeyPrefix: cfg.Store.KeyPrefix,
	}

	// Create atomic store
	atomicStore, err := memory.New(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create atomic store: %w", err)
	}

	return &MemoryStore{
		atomicStore: atomicStore,
		watchers:    make(map[string][]WatchCallback),
		stopCh:      make(chan struct{}),
	}, nil
}

// Get retrieves a value by key
func (ms *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	return ms.atomicStore.Get(ctx, key)
}

// Put stores a value by key
func (ms *MemoryStore) Put(ctx context.Context, key string, value []byte) error {
	return ms.atomicStore.Set(ctx, key, value, 0)
}

// Delete deletes a value by key
func (ms *MemoryStore) Delete(ctx context.Context, key string) error {
	return ms.atomicStore.Delete(ctx, key)
}

// List lists all keys with the given prefix
func (ms *MemoryStore) List(ctx context.Context, prefix string) (map[string][]byte, error) {
	// Memory store doesn't have a native List operation, so we'll return empty for now
	// In a real implementation, you might want to iterate through all keys
	return make(map[string][]byte), nil
}

// Exists checks if a key exists
func (ms *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	return ms.atomicStore.Exists(ctx, key)
}

// Watch watches for changes on a key or prefix
func (ms *MemoryStore) Watch(key string, callback WatchCallback) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.watchers[key] == nil {
		ms.watchers[key] = make([]WatchCallback, 0)
	}
	ms.watchers[key] = append(ms.watchers[key], callback)

	return nil
}

// Unwatch stops watching a key
func (ms *MemoryStore) Unwatch(key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.watchers, key)
	return nil
}

// Close closes the store connection
func (ms *MemoryStore) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.started {
		close(ms.stopCh)
		ms.started = false
	}

	if ms.atomicStore != nil {
		return ms.atomicStore.Close()
	}

	return nil
}

// Health returns the health status of the store
func (ms *MemoryStore) Health() map[string]interface{} {
	health := make(map[string]interface{})
	health["status"] = "healthy"
	health["type"] = "memory"
	health["timestamp"] = time.Now()

	if ms.atomicStore != nil {
		atomicHealth := ms.atomicStore.Health(context.Background())
		health["status"] = atomicHealth.Status
		health["message"] = atomicHealth.Message
		health["details"] = atomicHealth.Details
	} else {
		health["status"] = "unhealthy"
		health["message"] = "Memory store not initialized"
	}

	return health
}

// notifyWatchers notifies all watchers for a key
func (ms *MemoryStore) notifyWatchers(key string, value []byte, eventType EventType) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if callbacks, exists := ms.watchers[key]; exists {
		for _, callback := range callbacks {
			go callback(key, value, eventType)
		}
	}
}
