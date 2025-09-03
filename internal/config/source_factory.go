package config

import (
	"fmt"
	"time"

	"github.com/songzhibin97/stargate/internal/config/source/etcd"
	"github.com/songzhibin97/stargate/internal/config/source/file"
	pkgConfig "github.com/songzhibin97/stargate/pkg/config"
)

// CreateConfigSource creates a configuration source based on the provided configuration.
// It acts as a factory function that returns the appropriate config.Source implementation
// based on the driver specified in the configuration.
//
// Parameters:
//   - cfg: The configuration containing source driver and settings
//
// Returns:
//   - pkgConfig.Source: The configuration source implementation
//   - error: Any error that occurred during source creation
func CreateConfigSource(cfg *Config) (pkgConfig.Source, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	sourceConfig := cfg.ConfigSource.Source

	switch sourceConfig.Driver {
	case "file":
		return createFileSource(sourceConfig)
	case "etcd":
		return createEtcdSource(sourceConfig)
	default:
		return nil, fmt.Errorf("unsupported configuration source driver: %s", sourceConfig.Driver)
	}
}

// createFileSource creates a file-based configuration source
func createFileSource(sourceConfig SourceConfig) (pkgConfig.Source, error) {
	filePath := sourceConfig.File.Path
	if filePath == "" {
		return nil, fmt.Errorf("file path is required for file source driver")
	}

	// Use poll interval from file config, fallback to source config, then default
	pollInterval := sourceConfig.File.PollInterval
	if pollInterval == 0 {
		pollInterval = sourceConfig.PollInterval
	}
	if pollInterval == 0 {
		pollInterval = 1 * time.Second // Default poll interval
	}

	source, err := file.NewFileSource(filePath, pollInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create file source: %w", err)
	}

	return source, nil
}

// createEtcdSource creates an etcd-based configuration source
func createEtcdSource(sourceConfig SourceConfig) (pkgConfig.Source, error) {
	etcdConfig := sourceConfig.Etcd

	if len(etcdConfig.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints are required for etcd source driver")
	}

	if etcdConfig.Key == "" {
		return nil, fmt.Errorf("etcd key is required for etcd source driver")
	}

	// Convert internal config to etcd source config
	etcdSourceConfig := &etcd.EtcdConfig{
		Endpoints: etcdConfig.Endpoints,
		Timeout:   etcdConfig.Timeout,
		Username:  etcdConfig.Username,
		Password:  etcdConfig.Password,
	}

	// Set default timeout if not specified
	if etcdSourceConfig.Timeout == 0 {
		etcdSourceConfig.Timeout = 5 * time.Second
	}

	// Convert TLS config if enabled
	if etcdConfig.TLS.Enabled {
		etcdSourceConfig.TLS = &etcd.TLSConfig{
			Enabled:  etcdConfig.TLS.Enabled,
			CertFile: etcdConfig.TLS.CertFile,
			KeyFile:  etcdConfig.TLS.KeyFile,
			CAFile:   etcdConfig.TLS.CAFile,
		}
	}

	source, err := etcd.NewEtcdSource(etcdSourceConfig, etcdConfig.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd source: %w", err)
	}

	return source, nil
}

// ValidateSourceConfig validates the configuration source settings
func ValidateSourceConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	sourceConfig := cfg.ConfigSource.Source

	// Validate driver
	validDrivers := map[string]bool{
		"file": true,
		"etcd": true,
	}

	if !validDrivers[sourceConfig.Driver] {
		return fmt.Errorf("invalid configuration source driver: %s (valid options: file, etcd)", sourceConfig.Driver)
	}

	// Validate driver-specific configuration
	switch sourceConfig.Driver {
	case "file":
		return validateFileSourceConfig(sourceConfig.File)
	case "etcd":
		return validateEtcdSourceConfig(sourceConfig.Etcd)
	}

	return nil
}

// validateFileSourceConfig validates file source configuration
func validateFileSourceConfig(fileConfig FileSourceConfig) error {
	if fileConfig.Path == "" {
		return fmt.Errorf("file path is required for file source driver")
	}

	if fileConfig.PollInterval < 0 {
		return fmt.Errorf("file poll interval cannot be negative")
	}

	return nil
}

// validateEtcdSourceConfig validates etcd source configuration
func validateEtcdSourceConfig(etcdConfig EtcdSourceConfig) error {
	if len(etcdConfig.Endpoints) == 0 {
		return fmt.Errorf("etcd endpoints are required for etcd source driver")
	}

	if etcdConfig.Key == "" {
		return fmt.Errorf("etcd key is required for etcd source driver")
	}

	if etcdConfig.Timeout < 0 {
		return fmt.Errorf("etcd timeout cannot be negative")
	}

	// Validate TLS configuration if enabled
	if etcdConfig.TLS.Enabled {
		if etcdConfig.TLS.CertFile != "" && etcdConfig.TLS.KeyFile == "" {
			return fmt.Errorf("etcd TLS key file is required when cert file is specified")
		}
		if etcdConfig.TLS.KeyFile != "" && etcdConfig.TLS.CertFile == "" {
			return fmt.Errorf("etcd TLS cert file is required when key file is specified")
		}
	}

	return nil
}

// GetSupportedDrivers returns a list of supported configuration source drivers
func GetSupportedDrivers() []string {
	return []string{"file", "etcd"}
}

// GetDefaultSourceConfig returns a default source configuration for the specified driver
func GetDefaultSourceConfig(driver string) (*SourceConfig, error) {
	switch driver {
	case "file":
		return &SourceConfig{
			Driver:       "file",
			PollInterval: 1 * time.Second,
			File: FileSourceConfig{
				Path:         "routes.yaml",
				PollInterval: 1 * time.Second,
			},
		}, nil
	case "etcd":
		return &SourceConfig{
			Driver: "etcd",
			Etcd: EtcdSourceConfig{
				Endpoints: []string{"localhost:2379"},
				Key:       "/stargate/routes",
				Timeout:   5 * time.Second,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}
