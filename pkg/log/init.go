package log

import (
	"fmt"
	"os"

	"github.com/songzhibin97/stargate/internal/config"
)

// InitializeLogging initializes the global logging system based on configuration.
func InitializeLogging(cfg *config.Config) error {
	// Create factory configuration from main config
	factoryConfig := &FactoryConfig{
		DefaultDriver:    "stdout",
		Level:            InfoLevel,
		Development:      false,
		EnableCaller:     false,
		EnableStacktrace: true,
		TimeFormat:       "2006-01-02T15:04:05Z07:00", // RFC3339
		FieldNames: map[string]string{
			"time":    "timestamp",
			"level":   "level",
			"message": "message",
			"caller":  "caller",
		},
	}

	// Override with config values if available
	// For now, we'll use default values since the config structure doesn't have Log field yet
	// This will be properly implemented when the config structure is updated
	if cfg != nil {
		// TODO: Add log configuration to config.Config structure
		// For now, use defaults
	}

	// Initialize the global factory
	if err := InitGlobalFactory(factoryConfig); err != nil {
		return fmt.Errorf("failed to initialize global logger factory: %w", err)
	}

	// Log initialization success
	logger := Default()
	logger.Info("Logging system initialized",
		String("driver", factoryConfig.DefaultDriver),
		String("level", factoryConfig.Level.String()),
		Bool("development", factoryConfig.Development),
		Bool("caller_enabled", factoryConfig.EnableCaller),
		Bool("stacktrace_enabled", factoryConfig.EnableStacktrace),
		String("time_format", factoryConfig.TimeFormat),
	)

	return nil
}

// InitializeDefaultLogging initializes logging with default settings.
func InitializeDefaultLogging() error {
	return InitializeLogging(nil)
}

// InitializeProductionLogging initializes logging with production-ready settings.
func InitializeProductionLogging() error {
	factoryConfig := &FactoryConfig{
		DefaultDriver:    "stdout",
		Level:            InfoLevel,
		Development:      false,
		EnableCaller:     false,
		EnableStacktrace: true,
		TimeFormat:       "2006-01-02T15:04:05Z07:00", // RFC3339
		FieldNames: map[string]string{
			"time":    "timestamp",
			"level":   "level",
			"message": "message",
			"caller":  "caller",
		},
	}

	if err := InitGlobalFactory(factoryConfig); err != nil {
		return fmt.Errorf("failed to initialize production logger factory: %w", err)
	}

	logger := Default()
	logger.Info("Production logging system initialized")
	return nil
}

// InitializeDevelopmentLogging initializes logging with development-friendly settings.
func InitializeDevelopmentLogging() error {
	factoryConfig := &FactoryConfig{
		DefaultDriver:    "stdout",
		Level:            DebugLevel,
		Development:      true,
		EnableCaller:     true,
		EnableStacktrace: true,
		TimeFormat:       "2006-01-02T15:04:05Z07:00", // RFC3339
		FieldNames: map[string]string{
			"time":    "time",
			"level":   "level",
			"message": "msg",
			"caller":  "caller",
		},
	}

	if err := InitGlobalFactory(factoryConfig); err != nil {
		return fmt.Errorf("failed to initialize development logger factory: %w", err)
	}

	logger := Default()
	logger.Info("Development logging system initialized")
	return nil
}

// MustInitializeLogging initializes logging and panics on error.
func MustInitializeLogging(cfg *config.Config) {
	if err := InitializeLogging(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
}

// MustInitializeDefaultLogging initializes default logging and panics on error.
func MustInitializeDefaultLogging() {
	if err := InitializeDefaultLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize default logging: %v\n", err)
		os.Exit(1)
	}
}

// MustInitializeProductionLogging initializes production logging and panics on error.
func MustInitializeProductionLogging() {
	if err := InitializeProductionLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize production logging: %v\n", err)
		os.Exit(1)
	}
}

// MustInitializeDevelopmentLogging initializes development logging and panics on error.
func MustInitializeDevelopmentLogging() {
	if err := InitializeDevelopmentLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize development logging: %v\n", err)
		os.Exit(1)
	}
}

// GetApplicationLogger returns a logger for the main application.
func GetApplicationLogger() Logger {
	return Service("stargate", "1.0.0")
}

// GetComponentLogger returns a logger for a specific component.
func GetComponentLogger(component string) Logger {
	return Component(component)
}

// GetServiceLogger returns a logger for a specific service.
func GetServiceLogger(service, version string) Logger {
	return Service(service, version)
}

// GetRequestLogger returns a logger for a specific request.
func GetRequestLogger(requestID, userID string) Logger {
	return Request(requestID, userID)
}

// Shutdown gracefully shuts down the logging system.
func Shutdown() error {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil
	}
	return factory.Shutdown(nil)
}
