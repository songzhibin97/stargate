package static

import (
	"context"
	"fmt"
	"os"

	"github.com/songzhibin97/stargate/pkg/discovery"
)

// Driver implements the discovery.Driver interface for static service discovery
type Driver struct{}

// NewDriver creates a new static service discovery driver
func NewDriver() discovery.Driver {
	return &Driver{}
}

// Name returns the driver name
func (d *Driver) Name() string {
	return "static"
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

	// Create and return the static registry
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

	// Check if config file exists and is readable
	configPath, _ := config.Options["config_path"].(string)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not accessible: %w", err)
	}

	// Try to read the file to ensure it's readable
	if _, err := os.ReadFile(configPath); err != nil {
		return fmt.Errorf("config file not readable: %w", err)
	}

	return nil
}

// validateConfig validates the static service discovery configuration
func (d *Driver) validateConfig(config *discovery.Config) error {
	if config.Type != "static" {
		return fmt.Errorf("invalid driver type: expected 'static', got '%s'", config.Type)
	}

	// Check if config_path is provided in options
	configPath, ok := config.Options["config_path"].(string)
	if !ok || configPath == "" {
		return fmt.Errorf("config_path is required in options")
	}

	// Validate refresh interval
	if config.RefreshInterval < 0 {
		return fmt.Errorf("refresh_interval cannot be negative")
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

	return nil
}

// init registers the static driver
func init() {
	// This will be called when the package is imported
	// The actual registration should be done by the discovery manager
}
