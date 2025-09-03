package store

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStore implements the Store interface using etcd
type EtcdStore struct {
	config   *config.Config
	client   *clientv3.Client
	mu       sync.RWMutex
	watchers map[string]*watcher
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// watcher represents an etcd watcher
type watcher struct {
	key      string
	callback WatchCallback
	watchCh  clientv3.WatchChan
	cancel   context.CancelFunc
}

// WatchCallback is called when a key changes
type WatchCallback func(key string, value []byte, eventType EventType)

// EventType represents the type of watch event
type EventType int

const (
	EventTypePut EventType = iota
	EventTypeDelete
)

// Store interface defines the configuration store operations
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) (map[string][]byte, error)
	Watch(key string, callback WatchCallback) error
	Unwatch(key string) error
	Close() error
}

// NewEtcdStore creates a new etcd store
func NewEtcdStore(cfg *config.Config) (*EtcdStore, error) {
	// Create etcd client configuration
	clientConfig := clientv3.Config{
		Endpoints:   cfg.Store.Etcd.Endpoints,
		DialTimeout: cfg.Store.Etcd.Timeout,
	}

	// Add authentication if configured
	if cfg.Store.Etcd.Username != "" {
		clientConfig.Username = cfg.Store.Etcd.Username
		clientConfig.Password = cfg.Store.Etcd.Password
	}

	// Add TLS if configured
	if cfg.Store.Etcd.TLS.Enabled {
		tlsConfig, err := createTLSConfig(&cfg.Store.Etcd.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		clientConfig.TLS = tlsConfig
	}

	// Create etcd client
	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Store.Etcd.Timeout)
	defer cancel()

	_, err = client.Status(ctx, cfg.Store.Etcd.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &EtcdStore{
		config:   cfg,
		client:   client,
		watchers: make(map[string]*watcher),
		stopCh:   make(chan struct{}),
	}, nil
}

// Get retrieves a value by key
func (es *EtcdStore) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := es.getFullKey(key)
	
	resp, err := es.client.Get(ctx, fullKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key %s not found", key)
	}

	return resp.Kvs[0].Value, nil
}

// Put stores a value by key
func (es *EtcdStore) Put(ctx context.Context, key string, value []byte) error {
	fullKey := es.getFullKey(key)
	
	_, err := es.client.Put(ctx, fullKey, string(value))
	if err != nil {
		return fmt.Errorf("failed to put key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key
func (es *EtcdStore) Delete(ctx context.Context, key string) error {
	fullKey := es.getFullKey(key)
	
	_, err := es.client.Delete(ctx, fullKey)
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// List retrieves all keys with the given prefix
func (es *EtcdStore) List(ctx context.Context, prefix string) (map[string][]byte, error) {
	fullPrefix := es.getFullKey(prefix)
	
	resp, err := es.client.Get(ctx, fullPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list keys with prefix %s: %w", prefix, err)
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		// Remove the full prefix to get the relative key
		relativeKey := strings.TrimPrefix(string(kv.Key), es.config.Store.KeyPrefix+"/")
		result[relativeKey] = kv.Value
	}

	return result, nil
}

// Watch starts watching a key for changes
func (es *EtcdStore) Watch(key string, callback WatchCallback) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Check if already watching this key
	if _, exists := es.watchers[key]; exists {
		return fmt.Errorf("already watching key %s", key)
	}

	fullKey := es.getFullKey(key)
	
	// Create watch context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start watching
	watchCh := es.client.Watch(ctx, fullKey, clientv3.WithPrefix())
	
	// Create watcher
	w := &watcher{
		key:      key,
		callback: callback,
		watchCh:  watchCh,
		cancel:   cancel,
	}

	es.watchers[key] = w

	// Start watch goroutine
	es.wg.Add(1)
	go es.runWatcher(w)

	return nil
}

// Unwatch stops watching a key
func (es *EtcdStore) Unwatch(key string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	w, exists := es.watchers[key]
	if !exists {
		return fmt.Errorf("not watching key %s", key)
	}

	// Cancel the watcher
	w.cancel()
	delete(es.watchers, key)

	return nil
}

// Close closes the etcd store
func (es *EtcdStore) Close() error {
	// Stop all watchers
	close(es.stopCh)
	
	es.mu.Lock()
	for _, w := range es.watchers {
		w.cancel()
	}
	es.mu.Unlock()

	// Wait for all watchers to stop
	es.wg.Wait()

	// Close etcd client
	return es.client.Close()
}

// runWatcher runs a single watcher
func (es *EtcdStore) runWatcher(w *watcher) {
	defer es.wg.Done()

	for {
		select {
		case <-es.stopCh:
			return
		case watchResp, ok := <-w.watchCh:
			if !ok {
				return
			}

			if watchResp.Err() != nil {
				// Log error and continue
				continue
			}

			// Process events
			for _, event := range watchResp.Events {
				// Convert etcd key back to relative key
				relativeKey := strings.TrimPrefix(string(event.Kv.Key), es.config.Store.KeyPrefix+"/")
				
				var eventType EventType
				switch event.Type {
				case clientv3.EventTypePut:
					eventType = EventTypePut
				case clientv3.EventTypeDelete:
					eventType = EventTypeDelete
				}

				// Call callback
				go w.callback(relativeKey, event.Kv.Value, eventType)
			}
		}
	}
}

// getFullKey returns the full key with prefix
func (es *EtcdStore) getFullKey(key string) string {
	if es.config.Store.KeyPrefix == "" {
		return key
	}
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(es.config.Store.KeyPrefix, "/"), key)
}

// Health returns the health status of the etcd store
func (es *EtcdStore) Health() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status": "healthy",
		"type":   "etcd",
	}

	// Check each endpoint
	endpoints := make(map[string]interface{})
	for _, endpoint := range es.config.Store.Etcd.Endpoints {
		status, err := es.client.Status(ctx, endpoint)
		if err != nil {
			endpoints[endpoint] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			health["status"] = "unhealthy"
		} else {
			endpoints[endpoint] = map[string]interface{}{
				"status":   "healthy",
				"version":  status.Version,
				"leader":   status.Leader == status.Header.MemberId,
				"db_size":  status.DbSize,
			}
		}
	}
	health["endpoints"] = endpoints

	es.mu.RLock()
	health["watchers"] = len(es.watchers)
	es.mu.RUnlock()

	return health
}

// Metrics returns etcd store metrics
func (es *EtcdStore) Metrics() map[string]interface{} {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]interface{}{
		"watchers":  len(es.watchers),
		"endpoints": len(es.config.Store.Etcd.Endpoints),
	}
}

// createTLSConfig creates TLS configuration from config
func createTLSConfig(tlsConfig *config.TLSConfig) (*tls.Config, error) {
	// This is a placeholder - implement actual TLS config creation
	return nil, fmt.Errorf("TLS configuration not implemented")
}
