package log

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Factory provides a centralized way to create and manage loggers.
type Factory struct {
	mu            sync.RWMutex
	defaultLogger Logger
	loggers       map[string]Logger
	config        *FactoryConfig
}

// FactoryConfig represents the configuration for the logger factory.
type FactoryConfig struct {
	// DefaultDriver specifies the default logging driver to use
	DefaultDriver string `json:"default_driver" yaml:"default_driver"`
	
	// Level sets the global minimum logging level
	Level Level `json:"level" yaml:"level"`
	
	// Development enables development mode features
	Development bool `json:"development" yaml:"development"`
	
	// EnableCaller adds caller information to log entries
	EnableCaller bool `json:"enable_caller" yaml:"enable_caller"`
	
	// EnableStacktrace adds stack traces for error and fatal levels
	EnableStacktrace bool `json:"enable_stacktrace" yaml:"enable_stacktrace"`
	
	// TimeFormat specifies the time format for timestamps
	TimeFormat string `json:"time_format" yaml:"time_format"`
	
	// FieldNames allows customization of field names
	FieldNames map[string]string `json:"field_names" yaml:"field_names"`
}

// DefaultFactoryConfig returns a default factory configuration.
func DefaultFactoryConfig() *FactoryConfig {
	return &FactoryConfig{
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
}

// NewFactory creates a new logger factory with the given configuration.
func NewFactory(config *FactoryConfig) (*Factory, error) {
	if config == nil {
		config = DefaultFactoryConfig()
	}

	factory := &Factory{
		loggers: make(map[string]Logger),
		config:  config,
	}

	// Create default logger
	defaultLogger, err := factory.createLogger(config.DefaultDriver, "default")
	if err != nil {
		return nil, fmt.Errorf("failed to create default logger: %w", err)
	}

	factory.defaultLogger = defaultLogger
	factory.loggers["default"] = defaultLogger

	return factory, nil
}

// GetDefault returns the default logger instance.
func (f *Factory) GetDefault() Logger {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.defaultLogger
}

// GetLogger returns a named logger instance, creating it if it doesn't exist.
func (f *Factory) GetLogger(name string) Logger {
	f.mu.RLock()
	if logger, exists := f.loggers[name]; exists {
		f.mu.RUnlock()
		return logger
	}
	f.mu.RUnlock()

	// Create new logger
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := f.loggers[name]; exists {
		return logger
	}

	logger, err := f.createLogger(f.config.DefaultDriver, name)
	if err != nil {
		// Return default logger if creation fails
		return f.defaultLogger
	}

	f.loggers[name] = logger
	return logger
}

// GetComponentLogger returns a logger for a specific component with pre-configured fields.
func (f *Factory) GetComponentLogger(component string) Logger {
	baseLogger := f.GetLogger(component)
	return baseLogger.With(String("component", component))
}

// GetServiceLogger returns a logger for a specific service with pre-configured fields.
func (f *Factory) GetServiceLogger(service, version string) Logger {
	baseLogger := f.GetLogger(service)
	return baseLogger.With(
		String("service", service),
		String("version", version),
	)
}

// GetRequestLogger returns a logger for a specific request with pre-configured fields.
func (f *Factory) GetRequestLogger(requestID, userID string) Logger {
	baseLogger := f.GetDefault()
	return baseLogger.With(
		String("request_id", requestID),
		String("user_id", userID),
	)
}

// createLogger creates a new logger instance with the specified driver.
func (f *Factory) createLogger(driver, name string) (Logger, error) {
	switch driver {
	case "stdout":
		return f.createStdoutLogger(name)
	default:
		return nil, fmt.Errorf("unsupported logger driver: %s", driver)
	}
}

// createStdoutLogger creates a stdout logger with factory configuration.
func (f *Factory) createStdoutLogger(name string) (Logger, error) {
	// For now, return a simple fallback logger to avoid circular imports
	// This will be properly implemented when we refactor the driver system
	return &fallbackLogger{name: name}, nil
}

// Shutdown gracefully shuts down all loggers.
func (f *Factory) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// For now, we don't need to do anything special for shutdown
	// In the future, this could flush async loggers, close files, etc.
	return nil
}

// Global factory instance
var (
	globalFactory     *Factory
	globalFactoryOnce sync.Once
	globalFactoryMu   sync.RWMutex
)

// InitGlobalFactory initializes the global logger factory.
func InitGlobalFactory(config *FactoryConfig) error {
	var err error
	globalFactoryOnce.Do(func() {
		globalFactoryMu.Lock()
		defer globalFactoryMu.Unlock()
		globalFactory, err = NewFactory(config)
	})
	return err
}

// GetGlobalFactory returns the global logger factory instance.
func GetGlobalFactory() *Factory {
	globalFactoryMu.RLock()
	defer globalFactoryMu.RUnlock()
	return globalFactory
}

// Default returns the default logger from the global factory.
func Default() Logger {
	factory := GetGlobalFactory()
	if factory == nil {
		// Fallback: create a simple fallback logger
		return &fallbackLogger{name: "default"}
	}
	return factory.GetDefault()
}

// Component returns a component logger from the global factory.
func Component(component string) Logger {
	factory := GetGlobalFactory()
	if factory == nil {
		// Fallback: create a simple fallback logger with component field
		return &fallbackLogger{name: component}
	}
	return factory.GetComponentLogger(component)
}

// Service returns a service logger from the global factory.
func Service(service, version string) Logger {
	factory := GetGlobalFactory()
	if factory == nil {
		// Fallback: create a simple fallback logger with service fields
		return &fallbackLogger{name: service + "-" + version}
	}
	return factory.GetServiceLogger(service, version)
}

// Request returns a request logger from the global factory.
func Request(requestID, userID string) Logger {
	factory := GetGlobalFactory()
	if factory == nil {
		// Fallback: create a simple fallback logger with request fields
		return &fallbackLogger{name: "request-" + requestID}
	}
	return factory.GetRequestLogger(requestID, userID)
}

// FromContext extracts a logger from the context, or returns the default logger.
func FromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerContextKey).(Logger); ok {
		return logger
	}
	return Default()
}

// ToContext adds a logger to the context.
func ToContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// loggerContextKey is the key used to store loggers in context.
type contextKey string

const loggerContextKey contextKey = "logger"

// fallbackLogger is a simple logger implementation to avoid circular imports
type fallbackLogger struct {
	name   string
	fields []Field
}

func (l *fallbackLogger) Debug(msg string, fields ...Field) {
	// Simple fallback implementation - just print to stdout for now
	fmt.Printf("[DEBUG] %s: %s\n", l.name, msg)
}

func (l *fallbackLogger) Info(msg string, fields ...Field) {
	fmt.Printf("[INFO] %s: %s\n", l.name, msg)
}

func (l *fallbackLogger) Warn(msg string, fields ...Field) {
	fmt.Printf("[WARN] %s: %s\n", l.name, msg)
}

func (l *fallbackLogger) Error(msg string, fields ...Field) {
	fmt.Printf("[ERROR] %s: %s\n", l.name, msg)
}

func (l *fallbackLogger) Fatal(msg string, fields ...Field) {
	fmt.Printf("[FATAL] %s: %s\n", l.name, msg)
	os.Exit(1)
}

func (l *fallbackLogger) With(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)
	return &fallbackLogger{
		name:   l.name,
		fields: newFields,
	}
}

func (l *fallbackLogger) WithContext(ctx context.Context) Logger {
	return l
}
