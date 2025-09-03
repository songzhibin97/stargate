package kubernetes

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/songzhibin97/stargate/pkg/discovery"
)

// Driver implements the discovery.Driver interface for Kubernetes service discovery
type Driver struct{}

// NewDriver creates a new Kubernetes service discovery driver
func NewDriver() discovery.Driver {
	return &Driver{}
}

// Name returns the driver name
func (d *Driver) Name() string {
	return "kubernetes"
}

// Open creates a new service discovery registry instance
func (d *Driver) Open(config *discovery.Config) (discovery.Registry, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate required configuration
	if err := d.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create and return the Kubernetes registry
	return New(config)
}

// Ping tests the connection to the service discovery backend
func (d *Driver) Ping(ctx context.Context, config *discovery.Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate configuration
	if err := d.validateConfig(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Parse Kubernetes-specific configuration
	k8sConfig, err := parseKubernetesConfig(config)
	if err != nil {
		return fmt.Errorf("failed to parse kubernetes config: %w", err)
	}

	// Create Kubernetes client to test connectivity
	clientset, err := createKubernetesClient(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Test connection by listing namespaces
	if _, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	return nil
}

// validateConfig validates the Kubernetes service discovery configuration
func (d *Driver) validateConfig(config *discovery.Config) error {
	if config.Type != "kubernetes" {
		return fmt.Errorf("invalid driver type: expected 'kubernetes', got '%s'", config.Type)
	}

	// Validate timeout
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	// Validate retry settings
	if config.RetryCount < 0 {
		return fmt.Errorf("retry_count cannot be negative")
	}

	if config.RetryInterval < 0 {
		return fmt.Errorf("retry_interval cannot be negative")
	}

	// Validate refresh interval
	if config.RefreshInterval < 0 {
		return fmt.Errorf("refresh_interval cannot be negative")
	}

	// Validate Kubernetes-specific options
	if config.Options != nil {
		// Validate kubeconfig path if provided
		if kubeconfig, ok := config.Options["kubeconfig"].(string); ok && kubeconfig != "" {
			// Could add file existence check here
		}

		// Validate namespace if provided
		if namespace, ok := config.Options["namespace"].(string); ok && namespace != "" {
			// Could add namespace validation here
		}

		// Validate use_endpoints option
		if _, ok := config.Options["use_endpoints"]; ok {
			if _, isBool := config.Options["use_endpoints"].(bool); !isBool {
				return fmt.Errorf("use_endpoints must be a boolean")
			}
		}

		// Validate label_selector if provided
		if labelSelector, ok := config.Options["label_selector"].(string); ok && labelSelector != "" {
			// Could add label selector syntax validation here
		}

		// Validate field_selector if provided
		if fieldSelector, ok := config.Options["field_selector"].(string); ok && fieldSelector != "" {
			// Could add field selector syntax validation here
		}
	}

	return nil
}

// GetDefaultConfig returns a default Kubernetes service discovery configuration
func GetDefaultConfig() *discovery.Config {
	return &discovery.Config{
		Type:            "kubernetes",
		Timeout:         30 * time.Second,
		RefreshInterval: 0, // Real-time watching, no periodic refresh needed
		RetryCount:      3,
		RetryInterval:   5 * time.Second,
		Options: map[string]interface{}{
			"use_endpoints": true,  // Use Endpoints by default for backward compatibility
			"namespace":     "",    // Watch all namespaces by default
		},
	}
}

// init registers the Kubernetes driver
func init() {
	// This will be called when the package is imported
	// The actual registration should be done by the discovery manager
}
