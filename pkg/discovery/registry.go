package discovery

import (
	"context"
	"time"
)

// Registry defines the interface for service discovery
type Registry interface {
	// GetService retrieves service instances by service name
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	
	// Watch watches for service changes and calls the callback when changes occur
	Watch(ctx context.Context, serviceName string, callback WatchCallback) error
	
	// Unwatch stops watching a service
	Unwatch(serviceName string) error
	
	// Close closes the service discovery client and releases resources
	Close() error
	
	// Health returns the health status of the service discovery client
	Health(ctx context.Context) *HealthStatus
	
	// ListServices lists all available services
	ListServices(ctx context.Context) ([]string, error)
}

// ServiceInstance represents a service instance
type ServiceInstance struct {
	// ID is the unique identifier of the service instance
	ID string `json:"id"`
	
	// ServiceName is the name of the service
	ServiceName string `json:"service_name"`
	
	// Host is the hostname or IP address of the service instance
	Host string `json:"host"`
	
	// Port is the port number of the service instance
	Port int `json:"port"`
	
	// Weight is the load balancing weight of the service instance
	Weight int `json:"weight"`
	
	// Priority is the priority of the service instance (lower values have higher priority)
	Priority int `json:"priority"`
	
	// Healthy indicates whether the service instance is healthy
	Healthy bool `json:"healthy"`
	
	// Tags are key-value pairs for service instance classification
	Tags map[string]string `json:"tags,omitempty"`
	
	// Metadata contains additional metadata for the service instance
	Metadata map[string]string `json:"metadata,omitempty"`
	
	// Status represents the current status of the service instance
	Status InstanceStatus `json:"status"`
	
	// RegisterTime is when the service instance was registered
	RegisterTime time.Time `json:"register_time"`
	
	// LastHeartbeat is the last heartbeat time from the service instance
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
	
	// Version is the version of the service instance
	Version string `json:"version,omitempty"`
	
	// Zone is the availability zone of the service instance
	Zone string `json:"zone,omitempty"`
	
	// Region is the region of the service instance
	Region string `json:"region,omitempty"`
}

// InstanceStatus represents the status of a service instance
type InstanceStatus string

const (
	InstanceStatusUp      InstanceStatus = "up"
	InstanceStatusDown    InstanceStatus = "down"
	InstanceStatusStarting InstanceStatus = "starting"
	InstanceStatusStopping InstanceStatus = "stopping"
	InstanceStatusUnknown InstanceStatus = "unknown"
)

// WatchCallback is called when service instances change
type WatchCallback func(event *WatchEvent)

// WatchEvent represents a service discovery event
type WatchEvent struct {
	// Type is the type of the event
	Type EventType `json:"type"`
	
	// ServiceName is the name of the service that changed
	ServiceName string `json:"service_name"`
	
	// Instance is the service instance that changed (for instance-level events)
	Instance *ServiceInstance `json:"instance,omitempty"`
	
	// Instances is the current list of service instances (for service-level events)
	Instances []*ServiceInstance `json:"instances,omitempty"`
	
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`
}

// EventType represents the type of service discovery event
type EventType string

const (
	EventTypeInstanceAdded   EventType = "instance_added"
	EventTypeInstanceRemoved EventType = "instance_removed"
	EventTypeInstanceUpdated EventType = "instance_updated"
	EventTypeServiceAdded    EventType = "service_added"
	EventTypeServiceRemoved  EventType = "service_removed"
	EventTypeServiceUpdated  EventType = "service_updated"
)

// HealthStatus represents the health status of the service discovery client
type HealthStatus struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Filter defines criteria for filtering service instances
type Filter struct {
	// Tags filters instances by tags (all specified tags must match)
	Tags map[string]string `json:"tags,omitempty"`
	
	// HealthyOnly filters to only healthy instances
	HealthyOnly bool `json:"healthy_only,omitempty"`
	
	// Status filters instances by status
	Status []InstanceStatus `json:"status,omitempty"`
	
	// Zone filters instances by availability zone
	Zone string `json:"zone,omitempty"`
	
	// Region filters instances by region
	Region string `json:"region,omitempty"`
	
	// Version filters instances by version
	Version string `json:"version,omitempty"`
}

// Config represents the configuration for service discovery
type Config struct {
	// Type specifies the service discovery type (static, kubernetes, consul, etc.)
	Type string `yaml:"type" json:"type"`
	
	// Endpoints are the service discovery endpoints
	Endpoints []string `yaml:"endpoints" json:"endpoints"`
	
	// Namespace is the namespace for service discovery (if applicable)
	Namespace string `yaml:"namespace" json:"namespace"`
	
	// Timeout for service discovery operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// RefreshInterval is the interval for refreshing service instances
	RefreshInterval time.Duration `yaml:"refresh_interval" json:"refresh_interval"`
	
	// RetryCount is the number of retries for failed operations
	RetryCount int `yaml:"retry_count" json:"retry_count"`
	
	// RetryInterval is the interval between retries
	RetryInterval time.Duration `yaml:"retry_interval" json:"retry_interval"`
	
	// Authentication configuration
	Auth *AuthConfig `yaml:"auth" json:"auth,omitempty"`
	
	// TLS configuration
	TLS *TLSConfig `yaml:"tls" json:"tls,omitempty"`
	
	// Additional options specific to the service discovery implementation
	Options map[string]interface{} `yaml:"options" json:"options,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// Type of authentication (basic, token, certificate, etc.)
	Type string `yaml:"type" json:"type"`
	
	// Username for basic authentication
	Username string `yaml:"username" json:"username"`
	
	// Password for basic authentication
	Password string `yaml:"password" json:"password"`
	
	// Token for token-based authentication
	Token string `yaml:"token" json:"token"`
	
	// TokenFile path to token file
	TokenFile string `yaml:"token_file" json:"token_file"`
	
	// Additional authentication options
	Options map[string]interface{} `yaml:"options" json:"options,omitempty"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	// Enabled indicates whether TLS is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// InsecureSkipVerify skips certificate verification
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
	
	// CertFile path to certificate file
	CertFile string `yaml:"cert_file" json:"cert_file"`
	
	// KeyFile path to private key file
	KeyFile string `yaml:"key_file" json:"key_file"`
	
	// CAFile path to CA certificate file
	CAFile string `yaml:"ca_file" json:"ca_file"`
	
	// ServerName for certificate verification
	ServerName string `yaml:"server_name" json:"server_name"`
}

// Driver defines the interface for service discovery drivers
type Driver interface {
	// Name returns the driver name
	Name() string
	
	// Open creates a new service discovery registry instance
	Open(config *Config) (Registry, error)
	
	// Ping tests the connection to the service discovery backend
	Ping(ctx context.Context, config *Config) error
}

// Manager defines the interface for service discovery management
type Manager interface {
	// CreateRegistry creates a new service discovery registry
	CreateRegistry(name string, config *Config) (Registry, error)
	
	// GetRegistry gets a service discovery registry by name
	GetRegistry(name string) (Registry, error)
	
	// RemoveRegistry removes a service discovery registry
	RemoveRegistry(name string) error
	
	// ListRegistries lists all service discovery registries
	ListRegistries() []string
	
	// RegisterDriver registers a service discovery driver
	RegisterDriver(name string, driver Driver) error
	
	// GetDriver gets a driver by name
	GetDriver(name string) (Driver, error)
	
	// ListDrivers lists all registered drivers
	ListDrivers() []string
	
	// HealthCheck performs health check on all registries
	HealthCheck(ctx context.Context) map[string]*HealthStatus
}

// DefaultConfig returns a default service discovery configuration
func DefaultConfig() *Config {
	return &Config{
		Type:            "static",
		Endpoints:       []string{},
		Namespace:       "",
		Timeout:         5 * time.Second,
		RefreshInterval: 30 * time.Second,
		RetryCount:      3,
		RetryInterval:   1 * time.Second,
		Options:         make(map[string]interface{}),
	}
}
