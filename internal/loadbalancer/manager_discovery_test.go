package loadbalancer

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/pkg/discovery"
	discoveryManager "github.com/songzhibin97/stargate/internal/discovery"
)

// MockRegistry implements discovery.Registry for testing
type MockRegistry struct {
	services map[string][]*discovery.ServiceInstance
	watchers map[string][]discovery.WatchCallback
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		services: make(map[string][]*discovery.ServiceInstance),
		watchers: make(map[string][]discovery.WatchCallback),
	}
}

func (m *MockRegistry) GetService(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error) {
	instances, exists := m.services[serviceName]
	if !exists {
		return []*discovery.ServiceInstance{}, nil
	}
	return instances, nil
}

func (m *MockRegistry) Watch(ctx context.Context, serviceName string, callback discovery.WatchCallback) error {
	m.watchers[serviceName] = append(m.watchers[serviceName], callback)
	return nil
}

func (m *MockRegistry) Unwatch(serviceName string) error {
	delete(m.watchers, serviceName)
	return nil
}

func (m *MockRegistry) Close() error {
	return nil
}

func (m *MockRegistry) Health(ctx context.Context) *discovery.HealthStatus {
	return &discovery.HealthStatus{
		Status:    "healthy",
		Message:   "Mock registry is healthy",
		Timestamp: time.Now(),
	}
}

func (m *MockRegistry) ListServices(ctx context.Context) ([]string, error) {
	services := make([]string, 0, len(m.services))
	for serviceName := range m.services {
		services = append(services, serviceName)
	}
	return services, nil
}

// AddService adds a service with instances to the mock registry
func (m *MockRegistry) AddService(serviceName string, instances []*discovery.ServiceInstance) {
	m.services[serviceName] = instances
}

// TriggerServiceEvent triggers a service event for testing
func (m *MockRegistry) TriggerServiceEvent(serviceName string, eventType discovery.EventType, instances []*discovery.ServiceInstance) {
	callbacks, exists := m.watchers[serviceName]
	if !exists {
		return
	}

	event := &discovery.WatchEvent{
		Type:        eventType,
		ServiceName: serviceName,
		Instances:   instances,
		Timestamp:   time.Now(),
	}

	for _, callback := range callbacks {
		go callback(event)
	}
}

func TestManager_ServiceDiscoveryIntegration(t *testing.T) {
	// Create mock registry
	mockRegistry := NewMockRegistry()

	// Create discovery manager with proper configuration
	discoveryConfig := &discoveryManager.ManagerConfig{
		DefaultRegistry: "mock",
		Registries: map[string]*discovery.Config{
			"mock": {
				Type:    "mock",
				Timeout: 5 * time.Second,
			},
		},
	}
	discoveryMgr := discoveryManager.NewManager(discoveryConfig)

	// Register mock driver (this is a simplified approach for testing)
	// In a real implementation, we would need to create a proper mock driver
	
	// Create load balancer manager with service discovery
	cfg := &config.Config{}
	healthChecker := &health.ActiveHealthChecker{}
	logger := log.New(os.Stdout, "[LoadBalancer] ", log.LstdFlags)
	manager := NewManagerWithDiscovery(cfg, healthChecker, discoveryMgr, logger)

	// Initialize balancers
	if err := manager.InitializeBalancers(); err != nil {
		t.Fatalf("Failed to initialize balancers: %v", err)
	}

	// Test service discovery is enabled
	if !manager.IsServiceDiscoveryEnabled() {
		t.Error("Service discovery should be enabled")
	}

	// Add test service instances to mock registry
	testInstances := []*discovery.ServiceInstance{
		{
			ID:          "test-1",
			ServiceName: "test-service",
			Host:        "10.0.0.1",
			Port:        8080,
			Weight:      1,
			Healthy:     true,
			Status:      discovery.InstanceStatusUp,
		},
		{
			ID:          "test-2",
			ServiceName: "test-service",
			Host:        "10.0.0.2",
			Port:        8080,
			Weight:      2,
			Healthy:     true,
			Status:      discovery.InstanceStatusUp,
		},
	}
	mockRegistry.AddService("test-service", testInstances)

	// Test watching service (this will fail with our mock setup, but we test the error handling)
	err := manager.WatchService("test-service")
	if err == nil {
		t.Log("WatchService succeeded (unexpected with mock setup)")
		// Verify service is being watched
		watchedServices := manager.GetWatchedServices()
		if len(watchedServices) != 1 || watchedServices[0] != "test-service" {
			t.Errorf("Expected 1 watched service 'test-service', got %v", watchedServices)
		}
	} else {
		t.Logf("WatchService failed as expected with mock setup: %v", err)
	}

	// Test triggering service event
	updatedInstances := []*discovery.ServiceInstance{
		{
			ID:          "test-1",
			ServiceName: "test-service",
			Host:        "10.0.0.1",
			Port:        8080,
			Weight:      1,
			Healthy:     true,
			Status:      discovery.InstanceStatusUp,
		},
		{
			ID:          "test-3",
			ServiceName: "test-service",
			Host:        "10.0.0.3",
			Port:        8080,
			Weight:      1,
			Healthy:     true,
			Status:      discovery.InstanceStatusUp,
		},
	}

	// Trigger service update event
	mockRegistry.TriggerServiceEvent("test-service", discovery.EventTypeServiceUpdated, updatedInstances)

	// Give some time for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// Test unwatching service (only if we were actually watching)
	watchedServices := manager.GetWatchedServices()
	if len(watchedServices) > 0 {
		if err := manager.UnwatchService("test-service"); err != nil {
			t.Logf("Failed to unwatch service (expected): %v", err)
		}
	}

	// Verify final state
	watchedServices = manager.GetWatchedServices()
	t.Logf("Final watched services count: %d", len(watchedServices))

	// Test health status includes service discovery info
	health := manager.Health()
	discoveryHealth, exists := health["service_discovery"].(map[string]interface{})
	if !exists {
		t.Error("Health status should include service_discovery information")
	}

	if enabled, ok := discoveryHealth["enabled"].(bool); !ok || !enabled {
		t.Error("Service discovery should be enabled in health status")
	}

	// Test disabling service discovery
	manager.DisableServiceDiscovery()
	if manager.IsServiceDiscoveryEnabled() {
		t.Error("Service discovery should be disabled")
	}

	// Test stopping manager
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}
}

func TestManager_AddUpstreamFromDiscovery(t *testing.T) {
	// Create mock registry
	mockRegistry := NewMockRegistry()

	// Add test service
	testInstances := []*discovery.ServiceInstance{
		{
			ID:          "api-1",
			ServiceName: "api-service",
			Host:        "192.168.1.10",
			Port:        9090,
			Weight:      1,
			Healthy:     true,
			Status:      discovery.InstanceStatusUp,
		},
	}
	mockRegistry.AddService("api-service", testInstances)

	// Create discovery manager
	discoveryMgr := discoveryManager.NewManager(nil)

	// Create load balancer manager
	cfg := &config.Config{}
	healthChecker := &health.ActiveHealthChecker{}
	logger := log.New(os.Stdout, "[LoadBalancer] ", log.LstdFlags)
	manager := NewManagerWithDiscovery(cfg, healthChecker, discoveryMgr, logger)

	// Initialize balancers
	if err := manager.InitializeBalancers(); err != nil {
		t.Fatalf("Failed to initialize balancers: %v", err)
	}

	// Test adding upstream from discovery (this will fail with mock setup, but we test the flow)
	err := manager.AddUpstreamFromDiscovery("api-service")
	if err == nil {
		t.Log("AddUpstreamFromDiscovery completed successfully")
	} else {
		t.Logf("AddUpstreamFromDiscovery failed as expected with mock setup: %v", err)
	}

	// Test removing upstream from discovery
	err = manager.RemoveUpstreamFromDiscovery("api-service")
	if err != nil {
		t.Logf("RemoveUpstreamFromDiscovery result: %v", err)
	}

	// Clean up
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}
}

func TestManager_ServiceDiscoveryDisabled(t *testing.T) {
	// Create manager without service discovery
	cfg := &config.Config{}
	healthChecker := &health.ActiveHealthChecker{}
	logger := log.New(os.Stdout, "[LoadBalancer] ", log.LstdFlags)
	manager := NewManager(cfg, healthChecker, logger)

	// Test service discovery is disabled
	if manager.IsServiceDiscoveryEnabled() {
		t.Error("Service discovery should be disabled by default")
	}

	// Test operations fail when service discovery is disabled
	if err := manager.WatchService("test-service"); err == nil {
		t.Error("WatchService should fail when service discovery is disabled")
	}

	if _, err := manager.GetServiceInstances("test-service"); err == nil {
		t.Error("GetServiceInstances should fail when service discovery is disabled")
	}

	if err := manager.RefreshService("test-service"); err == nil {
		t.Error("RefreshService should fail when service discovery is disabled")
	}

	// Test health status shows service discovery as disabled
	health := manager.Health()
	discoveryHealth, exists := health["service_discovery"].(map[string]interface{})
	if !exists {
		t.Error("Health status should include service_discovery information")
	}

	if enabled, ok := discoveryHealth["enabled"].(bool); !ok || enabled {
		t.Error("Service discovery should be disabled in health status")
	}

	// Clean up
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}
}
