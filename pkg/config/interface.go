package config

import (
	"context"
	"time"
)

// Source defines the interface for configuration sources.
// This interface abstracts the common behavior of configuration centers,
// allowing the system to work with different configuration backends
// (file, etcd, consul, etc.) through a unified interface.
type Source interface {
	// Get retrieves the complete configuration data from the source.
	// It returns the raw configuration data as bytes, which can be
	// in any format (YAML, JSON, etc.) depending on the source implementation.
	//
	// Returns:
	//   - []byte: The complete configuration data
	//   - error: Any error that occurred during retrieval
	Get() ([]byte, error)

	// Watch monitors configuration changes and returns a channel that
	// delivers the latest complete configuration data whenever changes occur.
	// The channel will receive the full configuration data, not just the changes.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//
	// Returns:
	//   - <-chan []byte: A receive-only channel that delivers complete config data
	//   - error: Any error that occurred during watch setup
	//
	// The implementation should:
	//   - Send the current configuration immediately upon successful watch setup
	//   - Send updated configuration whenever changes are detected
	//   - Close the channel when the context is cancelled
	//   - Handle reconnection and error recovery transparently
	Watch(ctx context.Context) (<-chan []byte, error)

	// Close closes the source and cleans up any resources
	Close() error
}



// Loader defines the interface for configuration loading
type Loader interface {
	// Load loads configuration from multiple sources
	Load(sources ...Source) (*Config, error)

	// Reload reloads configuration
	Reload() error

	// Get gets a configuration value by key
	Get(key string) interface{}

	// Set sets a configuration value
	Set(key string, value interface{}) error

	// Subscribe subscribes to configuration changes
	Subscribe(callback ConfigChangeCallback) error
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(key string, oldValue, newValue interface{})

// Validator defines the interface for configuration validation
type Validator interface {
	// Validate validates configuration
	Validate(config *Config) error
	
	// ValidateField validates a specific field
	ValidateField(key string, value interface{}) error
}

// Config represents the configuration structure
type Config struct {
	// Raw configuration data
	Data map[string]interface{} `json:"data"`
	
	// Metadata
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// Provider defines the interface for configuration providers
type Provider interface {
	// Get gets configuration by key
	Get(ctx context.Context, key string) (*Config, error)

	// Set sets configuration
	Set(ctx context.Context, key string, config *Config) error

	// Delete deletes configuration
	Delete(ctx context.Context, key string) error

	// List lists configurations with prefix
	List(ctx context.Context, prefix string) (map[string]*Config, error)

	// Watch watches for configuration changes
	Watch(ctx context.Context, key string) (<-chan *Config, error)

	// Health returns provider health status
	Health() HealthStatus
}

// HealthStatus represents the health status of a provider
type HealthStatus struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Manager defines the interface for configuration management
type Manager interface {
	// RegisterSource registers a configuration source
	RegisterSource(name string, source Source) error
	
	// UnregisterSource unregisters a configuration source
	UnregisterSource(name string) error
	
	// GetSource gets a configuration source by name
	GetSource(name string) (Source, error)
	
	// ListSources lists all registered sources
	ListSources() []string
	
	// Start starts the configuration manager
	Start(ctx context.Context) error
	
	// Stop stops the configuration manager
	Stop() error
}
