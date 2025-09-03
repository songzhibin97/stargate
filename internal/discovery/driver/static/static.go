package static

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/songzhibin97/stargate/pkg/discovery"
)

// StaticRegistry implements the discovery.Registry interface using static configuration
type StaticRegistry struct {
	config       *discovery.Config
	services     map[string][]*discovery.ServiceInstance
	watchers     map[string][]discovery.WatchCallback
	mu           sync.RWMutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	started      bool
	configPath   string
	lastModTime  time.Time
}

// StaticConfig represents the static service discovery configuration
type StaticConfig struct {
	Services map[string]*ServiceConfig `yaml:"services" json:"services"`
}

// ServiceConfig represents the configuration for a single service
type ServiceConfig struct {
	Instances []*InstanceConfig `yaml:"instances" json:"instances"`
}

// InstanceConfig represents the configuration for a service instance
type InstanceConfig struct {
	ID            string            `yaml:"id" json:"id"`
	Host          string            `yaml:"host" json:"host"`
	Port          int               `yaml:"port" json:"port"`
	Weight        int               `yaml:"weight,omitempty" json:"weight,omitempty"`
	Priority      int               `yaml:"priority,omitempty" json:"priority,omitempty"`
	Healthy       bool              `yaml:"healthy,omitempty" json:"healthy,omitempty"`
	Tags          map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Status        string            `yaml:"status,omitempty" json:"status,omitempty"`
	Version       string            `yaml:"version,omitempty" json:"version,omitempty"`
	Zone          string            `yaml:"zone,omitempty" json:"zone,omitempty"`
	Region        string            `yaml:"region,omitempty" json:"region,omitempty"`
}

// New creates a new static service discovery registry
func New(config *discovery.Config) (discovery.Registry, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Get config file path from options
	configPath, ok := config.Options["config_path"].(string)
	if !ok || configPath == "" {
		return nil, fmt.Errorf("config_path is required in options")
	}

	registry := &StaticRegistry{
		config:     config,
		services:   make(map[string][]*discovery.ServiceInstance),
		watchers:   make(map[string][]discovery.WatchCallback),
		stopCh:     make(chan struct{}),
		configPath: configPath,
	}

	// Load initial configuration
	if err := registry.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	// Start file watcher if refresh interval is set
	if config.RefreshInterval > 0 {
		registry.start()
	}

	return registry, nil
}

// GetService retrieves service instances by service name
func (r *StaticRegistry) GetService(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, exists := r.services[serviceName]
	if !exists {
		return []*discovery.ServiceInstance{}, nil
	}

	// Return a copy to prevent external modification
	result := make([]*discovery.ServiceInstance, len(instances))
	for i, instance := range instances {
		result[i] = r.copyInstance(instance)
	}

	return result, nil
}

// Watch watches for service changes and calls the callback when changes occur
func (r *StaticRegistry) Watch(ctx context.Context, serviceName string, callback discovery.WatchCallback) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Add callback to watchers
	r.watchers[serviceName] = append(r.watchers[serviceName], callback)

	// Send initial event with current instances
	instances, exists := r.services[serviceName]
	if exists && len(instances) > 0 {
		go func() {
			event := &discovery.WatchEvent{
				Type:        discovery.EventTypeServiceUpdated,
				ServiceName: serviceName,
				Instances:   r.copyInstances(instances),
				Timestamp:   time.Now(),
			}
			callback(event)
		}()
	}

	return nil
}

// Unwatch stops watching a service
func (r *StaticRegistry) Unwatch(serviceName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.watchers, serviceName)
	return nil
}

// Close closes the service discovery client and releases resources
func (r *StaticRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return nil
	}

	close(r.stopCh)
	r.started = false

	// Wait for file watcher to finish
	r.wg.Wait()

	// Clear all data
	r.services = make(map[string][]*discovery.ServiceInstance)
	r.watchers = make(map[string][]discovery.WatchCallback)

	return nil
}

// Health returns the health status of the service discovery client
func (r *StaticRegistry) Health(ctx context.Context) *discovery.HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := "healthy"
	message := "Static registry is operational"

	// Check if config file exists
	if _, err := os.Stat(r.configPath); os.IsNotExist(err) {
		status = "unhealthy"
		message = fmt.Sprintf("Config file not found: %s", r.configPath)
	}

	return &discovery.HealthStatus{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"type":           "static",
			"config_path":    r.configPath,
			"services_count": len(r.services),
			"started":        r.started,
			"last_modified":  r.lastModTime,
		},
	}
}

// ListServices lists all available services
func (r *StaticRegistry) ListServices(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.services))
	for serviceName := range r.services {
		services = append(services, serviceName)
	}

	return services, nil
}

// start starts the file watcher goroutine
func (r *StaticRegistry) start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return
	}

	r.started = true
	r.wg.Add(1)
	go r.watchConfigFile()
}

// watchConfigFile watches for configuration file changes
func (r *StaticRegistry) watchConfigFile() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkAndReloadConfig()
		case <-r.stopCh:
			return
		}
	}
}

// checkAndReloadConfig checks if the config file has changed and reloads it
func (r *StaticRegistry) checkAndReloadConfig() {
	stat, err := os.Stat(r.configPath)
	if err != nil {
		return // File might have been deleted or become inaccessible
	}

	r.mu.RLock()
	lastModTime := r.lastModTime
	r.mu.RUnlock()

	// Check if file has been modified
	if stat.ModTime().After(lastModTime) {
		if err := r.loadConfig(); err == nil {
			r.notifyWatchers()
		}
	}
}

// loadConfig loads the configuration from file
func (r *StaticRegistry) loadConfig() error {
	// Check if file exists
	stat, err := os.Stat(r.configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	// Read file content
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse configuration based on file extension
	var staticConfig StaticConfig
	ext := strings.ToLower(filepath.Ext(r.configPath))
	
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &staticConfig); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &staticConfig); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Convert to service instances
	services := make(map[string][]*discovery.ServiceInstance)
	for serviceName, serviceConfig := range staticConfig.Services {
		instances := make([]*discovery.ServiceInstance, 0, len(serviceConfig.Instances))
		
		for _, instanceConfig := range serviceConfig.Instances {
			instance := r.convertToServiceInstance(serviceName, instanceConfig)
			instances = append(instances, instance)
		}
		
		services[serviceName] = instances
	}

	// Update registry state
	r.mu.Lock()
	r.services = services
	r.lastModTime = stat.ModTime()
	r.mu.Unlock()

	return nil
}

// convertToServiceInstance converts InstanceConfig to ServiceInstance
func (r *StaticRegistry) convertToServiceInstance(serviceName string, config *InstanceConfig) *discovery.ServiceInstance {
	// Set defaults
	weight := config.Weight
	if weight <= 0 {
		weight = 1
	}

	priority := config.Priority
	if priority < 0 {
		priority = 0
	}

	healthy := config.Healthy
	if config.Status == "" {
		if healthy {
			config.Status = string(discovery.InstanceStatusUp)
		} else {
			config.Status = string(discovery.InstanceStatusDown)
		}
	}

	status := discovery.InstanceStatus(config.Status)
	if status == "" {
		status = discovery.InstanceStatusUp
	}

	// Generate ID if not provided
	id := config.ID
	if id == "" {
		id = fmt.Sprintf("%s:%d", config.Host, config.Port)
	}

	now := time.Now()
	return &discovery.ServiceInstance{
		ID:            id,
		ServiceName:   serviceName,
		Host:          config.Host,
		Port:          config.Port,
		Weight:        weight,
		Priority:      priority,
		Healthy:       healthy,
		Tags:          r.copyStringMap(config.Tags),
		Metadata:      r.copyStringMap(config.Metadata),
		Status:        status,
		RegisterTime:  now,
		LastHeartbeat: now,
		Version:       config.Version,
		Zone:          config.Zone,
		Region:        config.Region,
	}
}

// notifyWatchers notifies all watchers about service changes
func (r *StaticRegistry) notifyWatchers() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for serviceName, callbacks := range r.watchers {
		instances, exists := r.services[serviceName]
		if !exists {
			continue
		}

		event := &discovery.WatchEvent{
			Type:        discovery.EventTypeServiceUpdated,
			ServiceName: serviceName,
			Instances:   r.copyInstances(instances),
			Timestamp:   time.Now(),
		}

		// Notify all callbacks for this service
		for _, callback := range callbacks {
			go func(cb discovery.WatchCallback, evt *discovery.WatchEvent) {
				cb(evt)
			}(callback, event)
		}
	}
}

// copyInstance creates a deep copy of a service instance
func (r *StaticRegistry) copyInstance(instance *discovery.ServiceInstance) *discovery.ServiceInstance {
	if instance == nil {
		return nil
	}

	return &discovery.ServiceInstance{
		ID:            instance.ID,
		ServiceName:   instance.ServiceName,
		Host:          instance.Host,
		Port:          instance.Port,
		Weight:        instance.Weight,
		Priority:      instance.Priority,
		Healthy:       instance.Healthy,
		Tags:          r.copyStringMap(instance.Tags),
		Metadata:      r.copyStringMap(instance.Metadata),
		Status:        instance.Status,
		RegisterTime:  instance.RegisterTime,
		LastHeartbeat: instance.LastHeartbeat,
		Version:       instance.Version,
		Zone:          instance.Zone,
		Region:        instance.Region,
	}
}

// copyInstances creates a deep copy of service instances slice
func (r *StaticRegistry) copyInstances(instances []*discovery.ServiceInstance) []*discovery.ServiceInstance {
	if instances == nil {
		return nil
	}

	result := make([]*discovery.ServiceInstance, len(instances))
	for i, instance := range instances {
		result[i] = r.copyInstance(instance)
	}
	return result
}

// copyStringMap creates a deep copy of a string map
func (r *StaticRegistry) copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
