package kubernetes

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/songzhibin97/stargate/pkg/discovery"
)

func TestKubernetesRegistry_GetService(t *testing.T) {
	// Create fake clientset with test data
	clientset := fake.NewSimpleClientset(
		createTestEndpoints(),
		createTestService(),
	)

	// Create registry
	config := &discovery.Config{
		Type:    "kubernetes",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"namespace":     "default",
			"use_endpoints": true,
		},
	}

	registry := &KubernetesRegistry{
		config:       config,
		clientset:    clientset,
		services:     make(map[string][]*discovery.ServiceInstance),
		watchers:     make(map[string][]discovery.WatchCallback),
		stopCh:       make(chan struct{}),
		namespace:    "default",
		useEndpoints: true,
	}

	// Manually populate services for testing
	instances := registry.convertEndpointsToInstances(createTestEndpoints())
	registry.services["test-service"] = instances

	ctx := context.Background()

	// Test getting existing service
	result, err := registry.GetService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 instances, got %d", len(result))
	}

	// Verify first instance
	instance := result[0]
	if instance.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", instance.ServiceName)
	}
	if instance.Host != "10.0.0.1" {
		t.Errorf("Expected host '10.0.0.1', got '%s'", instance.Host)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}
	if !instance.Healthy {
		t.Errorf("Expected instance to be healthy")
	}

	// Test getting non-existing service
	result, err = registry.GetService(ctx, "non-existing")
	if err != nil {
		t.Fatalf("Failed to get non-existing service: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("Expected 0 instances for non-existing service, got %d", len(result))
	}
}

func TestKubernetesRegistry_ListServices(t *testing.T) {
	registry := &KubernetesRegistry{
		services: map[string][]*discovery.ServiceInstance{
			"test-service":    {createTestServiceInstance("test-service", "10.0.0.1", 8080)},
			"another-service": {createTestServiceInstance("another-service", "10.0.0.2", 9090)},
		},
	}

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

func TestKubernetesRegistry_Watch(t *testing.T) {
	registry := &KubernetesRegistry{
		services: map[string][]*discovery.ServiceInstance{
			"test-service": {createTestServiceInstance("test-service", "10.0.0.1", 8080)},
		},
		watchers: make(map[string][]discovery.WatchCallback),
	}

	ctx := context.Background()
	eventCh := make(chan *discovery.WatchEvent, 10)

	// Start watching
	err := registry.Watch(ctx, "test-service", func(event *discovery.WatchEvent) {
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
		if len(event.Instances) != 1 {
			t.Errorf("Expected 1 instance, got %d", len(event.Instances))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial watch event")
	}
}

func TestKubernetesRegistry_Health(t *testing.T) {
	// Create fake clientset
	clientset := fake.NewSimpleClientset()

	registry := &KubernetesRegistry{
		clientset: clientset,
		namespace: "default",
		started:   true,
		services:  make(map[string][]*discovery.ServiceInstance),
	}

	ctx := context.Background()
	health := registry.Health(ctx)

	if health.Status != "healthy" {
		t.Errorf("Expected healthy status, got '%s'", health.Status)
	}

	// Verify details
	details := health.Details
	if details["type"] != "kubernetes" {
		t.Errorf("Expected type 'kubernetes', got '%v'", details["type"])
	}
	if details["namespace"] != "default" {
		t.Errorf("Expected namespace 'default', got '%v'", details["namespace"])
	}
}

func TestConvertEndpointsToInstances(t *testing.T) {
	registry := &KubernetesRegistry{}
	endpoints := createTestEndpoints()

	instances := registry.convertEndpointsToInstances(endpoints)

	if len(instances) != 2 {
		t.Fatalf("Expected 2 instances, got %d", len(instances))
	}

	// Check first instance (ready)
	instance := instances[0]
	if instance.Host != "10.0.0.1" {
		t.Errorf("Expected host '10.0.0.1', got '%s'", instance.Host)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}
	if !instance.Healthy {
		t.Errorf("Expected instance to be healthy")
	}
	if instance.Status != discovery.InstanceStatusUp {
		t.Errorf("Expected status 'up', got '%s'", instance.Status)
	}

	// Check second instance (not ready)
	instance = instances[1]
	if instance.Host != "10.0.0.2" {
		t.Errorf("Expected host '10.0.0.2', got '%s'", instance.Host)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}
	if instance.Healthy {
		t.Errorf("Expected instance to be unhealthy")
	}
	if instance.Status != discovery.InstanceStatusDown {
		t.Errorf("Expected status 'down', got '%s'", instance.Status)
	}
}

func TestConvertEndpointSlicesToInstances(t *testing.T) {
	registry := &KubernetesRegistry{}
	endpointSlice := createTestEndpointSlice()

	instances := registry.convertEndpointSlicesToInstances(endpointSlice)

	if len(instances) != 1 {
		t.Fatalf("Expected 1 instance, got %d", len(instances))
	}

	instance := instances[0]
	if instance.Host != "10.0.0.1" {
		t.Errorf("Expected host '10.0.0.1', got '%s'", instance.Host)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", instance.Port)
	}
	if !instance.Healthy {
		t.Errorf("Expected instance to be healthy")
	}
	if instance.Zone != "us-east-1a" {
		t.Errorf("Expected zone 'us-east-1a', got '%s'", instance.Zone)
	}
}

func TestDriver_Ping(t *testing.T) {
	driver := NewDriver()

	// Test with valid config (will fail without real Kubernetes cluster)
	config := &discovery.Config{
		Type:    "kubernetes",
		Timeout: 5 * time.Second,
		Options: map[string]interface{}{
			"namespace": "default",
		},
	}

	ctx := context.Background()
	err := driver.Ping(ctx, config)
	// This will fail in test environment without real Kubernetes cluster
	// but we can test that it doesn't panic and returns an error
	if err == nil {
		t.Log("Ping succeeded (running in Kubernetes environment)")
	} else {
		t.Logf("Ping failed as expected (not in Kubernetes environment): %v", err)
	}
}

// Helper functions for creating test data

func createTestEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"stargate.io/weight": "1",
			},
			CreationTimestamp: metav1.Now(),
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.1",
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "test-pod-1",
							Namespace: "default",
						},
					},
				},
				NotReadyAddresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.2",
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "test-pod-2",
							Namespace: "default",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port:     8080,
						Protocol: corev1.ProtocolTCP,
						Name:     "http",
					},
				},
			},
		},
	}
}

func createTestService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"service.beta.kubernetes.io/load-balancer-source-ranges": "0.0.0.0/0",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.0.1",
			Ports: []corev1.ServicePort{
				{
					Port:     8080,
					Protocol: corev1.ProtocolTCP,
					Name:     "http",
				},
			},
			Selector: map[string]string{
				"app": "test",
			},
		},
	}
}

func createTestEndpointSlice() *discoveryv1.EndpointSlice {
	ready := true
	port := int32(8080)
	zone := "us-east-1a"
	nodeName := "node-1"

	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-abc123",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
				"app":                        "test",
			},
			CreationTimestamp: metav1.Now(),
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: &ready,
				},
				Zone:     &zone,
				NodeName: &nodeName,
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod-1",
					Namespace: "default",
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     &port,
				Protocol: (*corev1.Protocol)(&[]corev1.Protocol{corev1.ProtocolTCP}[0]),
				Name:     stringPtr("http"),
			},
		},
	}
}

func createTestServiceInstance(serviceName, host string, port int) *discovery.ServiceInstance {
	return &discovery.ServiceInstance{
		ID:            fmt.Sprintf("%s:%d", host, port),
		ServiceName:   serviceName,
		Host:          host,
		Port:          port,
		Weight:        1,
		Priority:      0,
		Healthy:       true,
		Tags:          map[string]string{"app": "test"},
		Metadata:      map[string]string{"version": "1.0.0"},
		Status:        discovery.InstanceStatusUp,
		RegisterTime:  time.Now(),
		LastHeartbeat: time.Now(),
		Zone:          "us-east-1a",
		Region:        "us-east-1",
	}
}

func stringPtr(s string) *string {
	return &s
}
