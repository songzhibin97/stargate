package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/pkg/discovery"
	"github.com/songzhibin97/stargate/internal/discovery/driver/static"
	"github.com/songzhibin97/stargate/internal/discovery/driver/kubernetes"
)

// Manager implements the discovery.Manager interface
type Manager struct {
	mu         sync.RWMutex
	drivers    map[string]discovery.Driver
	registries map[string]discovery.Registry
	config     *ManagerConfig
}

// ManagerConfig represents the configuration for the discovery manager
type ManagerConfig struct {
	// DefaultRegistry is the default registry to use when none is specified
	DefaultRegistry string `yaml:"default_registry" json:"default_registry"`
	
	// Registries contains the configuration for each registry
	Registries map[string]*discovery.Config `yaml:"registries" json:"registries"`
	
	// HealthCheckInterval is the interval for health checking registries
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	
	// EnableMetrics enables metrics collection
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics"`
}

// NewManager creates a new discovery manager
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = &ManagerConfig{
			DefaultRegistry:     "static",
			Registries:          make(map[string]*discovery.Config),
			HealthCheckInterval: 30 * time.Second,
			EnableMetrics:       true,
		}
	}

	manager := &Manager{
		drivers:    make(map[string]discovery.Driver),
		registries: make(map[string]discovery.Registry),
		config:     config,
	}

	// Register built-in drivers
	manager.registerBuiltinDrivers()

	return manager
}

// registerBuiltinDrivers registers the built-in service discovery drivers
func (m *Manager) registerBuiltinDrivers() {
	// Register static driver
	staticDriver := static.NewDriver()
	m.drivers[staticDriver.Name()] = staticDriver

	// Register kubernetes driver
	kubernetesDriver := kubernetes.NewDriver()
	m.drivers[kubernetesDriver.Name()] = kubernetesDriver

	log.Println("Registered built-in service discovery drivers: static, kubernetes")
}

// CreateRegistry creates a new service discovery registry
func (m *Manager) CreateRegistry(name string, config *discovery.Config) (discovery.Registry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if registry already exists
	if _, exists := m.registries[name]; exists {
		return nil, fmt.Errorf("registry %s already exists", name)
	}

	// Get driver
	driver, exists := m.drivers[config.Type]
	if !exists {
		return nil, fmt.Errorf("driver %s not found", config.Type)
	}

	// Create registry
	registry, err := driver.Open(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry %s: %w", name, err)
	}

	// Store registry
	m.registries[name] = registry

	log.Printf("Created service discovery registry: %s (type: %s)", name, config.Type)
	return registry, nil
}

// GetRegistry gets a service discovery registry by name
func (m *Manager) GetRegistry(name string) (discovery.Registry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registry, exists := m.registries[name]
	if !exists {
		return nil, fmt.Errorf("registry %s not found", name)
	}

	return registry, nil
}

// GetDefaultRegistry gets the default service discovery registry
func (m *Manager) GetDefaultRegistry() (discovery.Registry, error) {
	if m.config.DefaultRegistry == "" {
		return nil, fmt.Errorf("no default registry configured")
	}

	return m.GetRegistry(m.config.DefaultRegistry)
}

// RemoveRegistry removes a service discovery registry
func (m *Manager) RemoveRegistry(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	registry, exists := m.registries[name]
	if !exists {
		return fmt.Errorf("registry %s not found", name)
	}

	// Close registry
	if err := registry.Close(); err != nil {
		log.Printf("Error closing registry %s: %v", name, err)
	}

	// Remove from map
	delete(m.registries, name)

	log.Printf("Removed service discovery registry: %s", name)
	return nil
}

// ListRegistries lists all service discovery registries
func (m *Manager) ListRegistries() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registries := make([]string, 0, len(m.registries))
	for name := range m.registries {
		registries = append(registries, name)
	}

	return registries
}

// RegisterDriver registers a service discovery driver
func (m *Manager) RegisterDriver(name string, driver discovery.Driver) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.drivers[name]; exists {
		return fmt.Errorf("driver %s already registered", name)
	}

	m.drivers[name] = driver
	log.Printf("Registered service discovery driver: %s", name)
	return nil
}

// GetDriver gets a driver by name
func (m *Manager) GetDriver(name string) (discovery.Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	driver, exists := m.drivers[name]
	if !exists {
		return nil, fmt.Errorf("driver %s not found", name)
	}

	return driver, nil
}

// ListDrivers lists all registered drivers
func (m *Manager) ListDrivers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	drivers := make([]string, 0, len(m.drivers))
	for name := range m.drivers {
		drivers = append(drivers, name)
	}

	return drivers
}

// HealthCheck performs health check on all registries
func (m *Manager) HealthCheck(ctx context.Context) map[string]*discovery.HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]*discovery.HealthStatus)
	for name, registry := range m.registries {
		results[name] = registry.Health(ctx)
	}

	return results
}

// Initialize initializes the discovery manager with configured registries
func (m *Manager) Initialize() error {
	for name, config := range m.config.Registries {
		if _, err := m.CreateRegistry(name, config); err != nil {
			return fmt.Errorf("failed to initialize registry %s: %w", name, err)
		}
	}

	log.Printf("Initialized %d service discovery registries", len(m.config.Registries))
	return nil
}

// Start starts the discovery manager
func (m *Manager) Start(ctx context.Context) error {
	// Initialize registries
	if err := m.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize discovery manager: %w", err)
	}

	// Start health check routine if enabled
	if m.config.HealthCheckInterval > 0 {
		go m.healthCheckRoutine(ctx)
	}

	log.Println("Discovery manager started")
	return nil
}

// Stop stops the discovery manager
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastError error
	for name, registry := range m.registries {
		if err := registry.Close(); err != nil {
			log.Printf("Error closing registry %s: %v", name, err)
			lastError = err
		}
	}

	// Clear registries
	m.registries = make(map[string]discovery.Registry)

	log.Println("Discovery manager stopped")
	return lastError
}

// healthCheckRoutine performs periodic health checks on all registries
func (m *Manager) healthCheckRoutine(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			results := m.HealthCheck(ctx)
			for name, status := range results {
				if status.Status != "healthy" {
					log.Printf("Registry %s is unhealthy: %s", name, status.Message)
				}
			}
		}
	}
}

// GetServiceInstances gets service instances from a specific registry
func (m *Manager) GetServiceInstances(ctx context.Context, registryName, serviceName string) ([]*discovery.ServiceInstance, error) {
	registry, err := m.GetRegistry(registryName)
	if err != nil {
		return nil, err
	}

	return registry.GetService(ctx, serviceName)
}

// GetServiceInstancesFromDefault gets service instances from the default registry
func (m *Manager) GetServiceInstancesFromDefault(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error) {
	registry, err := m.GetDefaultRegistry()
	if err != nil {
		return nil, err
	}

	return registry.GetService(ctx, serviceName)
}

// WatchService watches for service changes in a specific registry
func (m *Manager) WatchService(ctx context.Context, registryName, serviceName string, callback discovery.WatchCallback) error {
	registry, err := m.GetRegistry(registryName)
	if err != nil {
		return err
	}

	return registry.Watch(ctx, serviceName, callback)
}

// WatchServiceFromDefault watches for service changes in the default registry
func (m *Manager) WatchServiceFromDefault(ctx context.Context, serviceName string, callback discovery.WatchCallback) error {
	registry, err := m.GetDefaultRegistry()
	if err != nil {
		return err
	}

	return registry.Watch(ctx, serviceName, callback)
}

// DefaultManagerConfig returns a default manager configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultRegistry: "static",
		Registries: map[string]*discovery.Config{
			"static": {
				Type:            "static",
				Timeout:         5 * time.Second,
				RefreshInterval: 30 * time.Second,
				RetryCount:      3,
				RetryInterval:   1 * time.Second,
				Options: map[string]interface{}{
					"config_path": "config/services.yaml",
				},
			},
		},
		HealthCheckInterval: 30 * time.Second,
		EnableMetrics:       true,
	}
}
