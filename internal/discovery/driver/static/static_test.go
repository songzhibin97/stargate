package static

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/discovery"
	"gopkg.in/yaml.v3"
)

func TestStaticRegistry_GetService(t *testing.T) {
	// Create temporary config file
	configFile := createTempConfigFile(t, testConfig)
	defer os.Remove(configFile)

	// Create registry
	config := &discovery.Config{
		Type:    "static",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"config_path": configFile,
		},
	}

	registry, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	// Test getting existing service
	instances, err := registry.GetService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("Expected 2 instances, got %d", len(instances))
	}

	// Verify first instance
	instance := instances[0]
	if instance.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", instance.ServiceName)
	}
	if instance.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", instance.Host)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}

	// Test getting non-existing service
	instances, err = registry.GetService(ctx, "non-existing")
	if err != nil {
		t.Fatalf("Failed to get non-existing service: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("Expected 0 instances for non-existing service, got %d", len(instances))
	}
}

func TestStaticRegistry_ListServices(t *testing.T) {
	// Create temporary config file
	configFile := createTempConfigFile(t, testConfig)
	defer os.Remove(configFile)

	// Create registry
	config := &discovery.Config{
		Type:    "static",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"config_path": configFile,
		},
	}

	registry, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()
	services, err := registry.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}

	expectedServices := []string{"test-service", "another-service"}
	if len(services) != len(expectedServices) {
		t.Fatalf("Expected %d services, got %d", len(expectedServices), len(services))
	}

	// Check if all expected services are present
	serviceMap := make(map[string]bool)
	for _, service := range services {
		serviceMap[service] = true
	}

	for _, expected := range expectedServices {
		if !serviceMap[expected] {
			t.Errorf("Expected service '%s' not found", expected)
		}
	}
}

func TestStaticRegistry_Watch(t *testing.T) {
	// Create temporary config file
	configFile := createTempConfigFile(t, testConfig)
	defer os.Remove(configFile)

	// Create registry with refresh interval
	config := &discovery.Config{
		Type:            "static",
		Timeout:         5 * time.Second,
		RefreshInterval: 100 * time.Millisecond,
		Options: map[string]interface{}{
			"config_path": configFile,
		},
	}

	registry, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()
	eventCh := make(chan *discovery.WatchEvent, 10)

	// Start watching
	err = registry.Watch(ctx, "test-service", func(event *discovery.WatchEvent) {
		eventCh <- event
	})
	if err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait for initial event
	select {
	case event := <-eventCh:
		if event.Type != discovery.EventTypeServiceUpdated {
			t.Errorf("Expected ServiceUpdated event, got %s", event.Type)
		}
		if event.ServiceName != "test-service" {
			t.Errorf("Expected service name 'test-service', got '%s'", event.ServiceName)
		}
		if len(event.Instances) != 2 {
			t.Errorf("Expected 2 instances, got %d", len(event.Instances))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial watch event")
	}

	// Update config file
	updatedConfig := testConfigUpdated
	updateTempConfigFile(t, configFile, updatedConfig)

	// Wait for update event
	select {
	case event := <-eventCh:
		if event.Type != discovery.EventTypeServiceUpdated {
			t.Errorf("Expected ServiceUpdated event, got %s", event.Type)
		}
		if len(event.Instances) != 1 {
			t.Errorf("Expected 1 instance after update, got %d", len(event.Instances))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for config update event")
	}
}

func TestStaticRegistry_Health(t *testing.T) {
	// Create temporary config file
	configFile := createTempConfigFile(t, testConfig)
	defer os.Remove(configFile)

	// Create registry
	config := &discovery.Config{
		Type:    "static",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"config_path": configFile,
		},
	}

	registry, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()
	health := registry.Health(ctx)

	if health.Status != "healthy" {
		t.Errorf("Expected healthy status, got '%s'", health.Status)
	}

	// Test with non-existing config file
	os.Remove(configFile)
	health = registry.Health(ctx)

	if health.Status != "unhealthy" {
		t.Errorf("Expected unhealthy status when config file is missing, got '%s'", health.Status)
	}
}

func TestDriver_Ping(t *testing.T) {
	// Create temporary config file
	configFile := createTempConfigFile(t, testConfig)
	defer os.Remove(configFile)

	driver := NewDriver()

	// Test with valid config
	config := &discovery.Config{
		Type:    "static",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"config_path": configFile,
		},
	}

	ctx := context.Background()
	err := driver.Ping(ctx, config)
	if err != nil {
		t.Fatalf("Ping failed with valid config: %v", err)
	}

	// Test with non-existing config file
	config.Options["config_path"] = "/non/existing/file.yaml"
	err = driver.Ping(ctx, config)
	if err == nil {
		t.Fatal("Expected ping to fail with non-existing config file")
	}
}

// Test configuration data
var testConfig = StaticConfig{
	Services: map[string]*ServiceConfig{
		"test-service": {
			Instances: []*InstanceConfig{
				{
					ID:       "test-1",
					Host:     "localhost",
					Port:     8080,
					Weight:   1,
					Priority: 0,
					Healthy:  true,
					Tags:     map[string]string{"env": "test"},
					Metadata: map[string]string{"version": "1.0.0"},
					Status:   "up",
					Version:  "1.0.0",
					Zone:     "us-east-1a",
					Region:   "us-east-1",
				},
				{
					ID:       "test-2",
					Host:     "localhost",
					Port:     8081,
					Weight:   2,
					Priority: 1,
					Healthy:  true,
					Tags:     map[string]string{"env": "test"},
					Metadata: map[string]string{"version": "1.0.0"},
					Status:   "up",
					Version:  "1.0.0",
					Zone:     "us-east-1b",
					Region:   "us-east-1",
				},
			},
		},
		"another-service": {
			Instances: []*InstanceConfig{
				{
					ID:      "another-1",
					Host:    "localhost",
					Port:    9090,
					Weight:  1,
					Healthy: true,
					Status:  "up",
				},
			},
		},
	},
}

var testConfigUpdated = StaticConfig{
	Services: map[string]*ServiceConfig{
		"test-service": {
			Instances: []*InstanceConfig{
				{
					ID:       "test-1",
					Host:     "localhost",
					Port:     8080,
					Weight:   1,
					Priority: 0,
					Healthy:  true,
					Tags:     map[string]string{"env": "test"},
					Metadata: map[string]string{"version": "1.1.0"},
					Status:   "up",
					Version:  "1.1.0",
					Zone:     "us-east-1a",
					Region:   "us-east-1",
				},
			},
		},
	},
	}

// Helper functions for testing

// createTempConfigFile creates a temporary YAML config file for testing
func createTempConfigFile(t *testing.T, config StaticConfig) string {
	t.Helper()

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "static_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Write config to file
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	return tmpFile.Name()
}

// updateTempConfigFile updates an existing config file
func updateTempConfigFile(t *testing.T, filename string, config StaticConfig) {
	t.Helper()

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Sleep a bit to ensure file modification time changes
	time.Sleep(10 * time.Millisecond)
}
