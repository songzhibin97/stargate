package store

import (
	"context"
	"time"
)

// Store defines the interface for key-value storage operations
type Store interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) ([]byte, error)
	
	// Put stores a value by key
	Put(ctx context.Context, key string, value []byte) error
	
	// Delete deletes a value by key
	Delete(ctx context.Context, key string) error
	
	// List lists all keys with the given prefix
	List(ctx context.Context, prefix string) (map[string][]byte, error)
	
	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)
	
	// Watch watches for changes on a key or prefix
	Watch(key string, callback WatchCallback) error
	
	// Unwatch stops watching a key
	Unwatch(key string) error
	
	// Close closes the store connection
	Close() error
	
	// Health returns the health status of the store
	Health() HealthStatus
}

// WatchCallback is called when a watched key changes
type WatchCallback func(key string, value []byte, eventType EventType)

// EventType represents the type of store event
type EventType int

const (
	EventTypePut EventType = iota
	EventTypeDelete
)

// HealthStatus represents the health status of a store
type HealthStatus struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// TransactionalStore defines the interface for transactional operations
type TransactionalStore interface {
	Store
	
	// BeginTx begins a transaction
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction defines the interface for store transactions
type Transaction interface {
	// Get retrieves a value by key within the transaction
	Get(ctx context.Context, key string) ([]byte, error)
	
	// Put stores a value by key within the transaction
	Put(ctx context.Context, key string, value []byte) error
	
	// Delete deletes a value by key within the transaction
	Delete(ctx context.Context, key string) error
	
	// Commit commits the transaction
	Commit(ctx context.Context) error
	
	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error
}

// CacheStore defines the interface for cache operations
type CacheStore interface {
	// Get retrieves a cached value
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete deletes a cached value
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in cache
	Exists(ctx context.Context, key string) (bool, error)

	// TTL returns the remaining TTL for a key
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Clear clears all cached values
	Clear(ctx context.Context) error

	// Size returns the number of cached items
	Size(ctx context.Context) (int64, error)
}

// AtomicStore defines the interface for atomic operations
// This interface is specifically designed for rate limiting and similar use cases
// that require atomic increment operations and TTL support.
type AtomicStore interface {
	// IncrBy atomically increments the value of a key by the given amount
	// If the key doesn't exist, it will be created with the increment value
	// Returns the new value after increment
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// Set stores a value by key with optional TTL
	// If ttl is 0, the key will not expire
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Get retrieves a value by key
	// Returns nil if key doesn't exist
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key from storage
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in storage
	Exists(ctx context.Context, key string) (bool, error)

	// TTL returns the remaining time to live for a key
	// Returns -1 if key has no expiration, -2 if key doesn't exist
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Close closes the store connection and releases resources
	Close() error

	// Health returns the health status of the store
	Health(ctx context.Context) HealthStatus
}

// DistributedStore defines the interface for distributed storage operations
type DistributedStore interface {
	Store
	
	// Lock acquires a distributed lock
	Lock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
	
	// TryLock tries to acquire a distributed lock without blocking
	TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

// Lock defines the interface for distributed locks
type Lock interface {
	// Key returns the lock key
	Key() string
	
	// Extend extends the lock TTL
	Extend(ctx context.Context, ttl time.Duration) error
	
	// Release releases the lock
	Release(ctx context.Context) error
	
	// IsHeld checks if the lock is still held
	IsHeld(ctx context.Context) (bool, error)
}

// Driver defines the interface for store drivers
type Driver interface {
	// Name returns the driver name
	Name() string
	
	// Open opens a connection to the store
	Open(config map[string]interface{}) (Store, error)
	
	// Ping tests the connection to the store
	Ping(ctx context.Context, config map[string]interface{}) error
}

// Registry defines the interface for driver registry
type Registry interface {
	// Register registers a store driver
	Register(name string, driver Driver) error
	
	// Unregister unregisters a store driver
	Unregister(name string) error
	
	// Get gets a store driver by name
	Get(name string) (Driver, error)
	
	// List lists all registered drivers
	List() []string
}

// Manager defines the interface for store management
type Manager interface {
	// CreateStore creates a new store instance
	CreateStore(name string, driverName string, config map[string]interface{}) (Store, error)

	// GetStore gets an existing store instance
	GetStore(name string) (Store, error)

	// CloseStore closes a store instance
	CloseStore(name string) error

	// ListStores lists all store instances
	ListStores() []string

	// HealthCheck performs health check on all stores
	HealthCheck(ctx context.Context) map[string]HealthStatus
}

// Config represents the configuration for a store
type Config struct {
	// Type specifies the store type (memory, redis, etc.)
	Type string `yaml:"type" json:"type"`

	// Address is the connection address for remote stores
	Address string `yaml:"address" json:"address"`

	// Database number for stores that support multiple databases
	Database int `yaml:"database" json:"database"`

	// Username for authentication
	Username string `yaml:"username" json:"username"`

	// Password for authentication
	Password string `yaml:"password" json:"password"`

	// Timeout for operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// MaxRetries for failed operations
	MaxRetries int `yaml:"max_retries" json:"max_retries"`

	// KeyPrefix for all keys stored by this instance
	KeyPrefix string `yaml:"key_prefix" json:"key_prefix"`

	// Additional configuration options
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// DefaultConfig returns a default store configuration
func DefaultConfig() *Config {
	return &Config{
		Type:       "memory",
		Address:    "",
		Database:   0,
		Username:   "",
		Password:   "",
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		KeyPrefix:  "",
		Options:    make(map[string]interface{}),
	}
}
