package stdout_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/songzhibin97/stargate/internal/log/driver/stdout"
	"github.com/songzhibin97/stargate/pkg/log"
)

// ExampleNew demonstrates basic usage of the stdout logger.
func ExampleNew() {
	// Create a logger with default configuration
	logger, err := stdout.New(nil)
	if err != nil {
		panic(err)
	}

	// Log some messages
	logger.Info("Application started", log.String("version", "1.0.0"))
	logger.Debug("Debug information", log.Int("debug_level", 2))
	logger.Warn("This is a warning", log.Bool("important", true))
	logger.Error("An error occurred", log.Error(errors.New("example error")))

	// Output will be JSON formatted to stdout
}

// ExampleNewConfigBuilder demonstrates using the config builder.
func ExampleNewConfigBuilder() {
	// Build a custom configuration
	config := stdout.NewConfigBuilder().
		WithLevel(log.DebugLevel).
		WithTimeFormat(time.RFC3339).
		WithCaller(true).
		WithStacktrace(true).
		WithDevelopment(true).
		WithTimeFieldName("ts").
		WithLevelFieldName("lvl").
		WithMessageFieldName("msg").
		Build()

	logger, err := stdout.New(config)
	if err != nil {
		panic(err)
	}

	logger.Info("Custom configured logger", log.String("config", "custom"))
}

// ExampleGetPresetConfig demonstrates using preset configurations.
func ExampleGetPresetConfig() {
	// Use development preset
	devConfig := stdout.GetPresetConfig("development")
	devLogger, err := stdout.New(devConfig)
	if err != nil {
		panic(err)
	}

	devLogger.Debug("Development mode enabled")

	// Use production preset
	prodConfig := stdout.GetPresetConfig("production")
	prodLogger, err := stdout.New(prodConfig)
	if err != nil {
		panic(err)
	}

	prodLogger.Info("Production mode enabled")
}

// ExampleNewWithOptions demonstrates using functional options.
func ExampleNewWithOptions() {
	logger, err := stdout.NewWithOptions(
		stdout.WithLevelOption(log.InfoLevel),
		stdout.WithTimeFormatOption(time.RFC3339Nano),
		stdout.WithCallerOption(true),
		stdout.WithDevelopmentOption(false),
	)
	if err != nil {
		panic(err)
	}

	logger.Info("Logger created with options", log.String("method", "functional"))
}

// ExampleStdoutLogger_With demonstrates creating child loggers with additional fields.
func ExampleStdoutLogger_With() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Create a child logger with service context
	serviceLogger := logger.With(
		log.String("service", "user-service"),
		log.String("version", "2.1.0"),
		log.String("environment", "production"),
	)

	// All logs from serviceLogger will include the above fields
	serviceLogger.Info("User service started")
	serviceLogger.Error("Database connection failed", log.Error(errors.New("connection timeout")))

	// Create another child logger with request context
	requestLogger := serviceLogger.With(
		log.String("request_id", "req-123-456"),
		log.String("user_id", "user-789"),
	)

	requestLogger.Info("Processing user request")
	requestLogger.Warn("Request took longer than expected", log.Duration("duration", 2*time.Second))
}

// ExampleStdoutLogger_WithContext demonstrates using context for logging.
func ExampleStdoutLogger_WithContext() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Create context with trace and request IDs
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	ctx = context.WithValue(ctx, "request_id", "req-def-456")

	// Create logger with context
	contextLogger := logger.WithContext(ctx)

	// All logs will include context information
	contextLogger.Info("Request started")
	contextLogger.Error("Request failed", log.Error(errors.New("validation error")))
}

// Example_fieldTypes demonstrates different field types.
func Example_fieldTypes() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	now := time.Now()
	duration := 150 * time.Millisecond

	logger.Info("Demonstrating field types",
		log.String("string_field", "hello world"),
		log.Int("int_field", 42),
		log.Int64("int64_field", 9223372036854775807),
		log.Float64("float64_field", 3.14159),
		log.Bool("bool_field", true),
		log.Time("time_field", now),
		log.Duration("duration_field", duration),
		log.Error(errors.New("example error")),
		log.Any("complex_field", map[string]interface{}{
			"nested": map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			"array": []int{1, 2, 3},
		}),
	)
}

// ExampleNewPerformanceOptimized demonstrates the performance-optimized logger.
func ExampleNewPerformanceOptimized() {
	config := stdout.DefaultConfig()
	config.Level = log.InfoLevel

	logger, err := stdout.NewPerformanceOptimized(config)
	if err != nil {
		panic(err)
	}

	// This logger uses object pools to reduce memory allocations
	for i := 0; i < 1000; i++ {
		logger.Info("High-performance logging",
			log.Int("iteration", i),
			log.String("status", "processing"),
			log.Bool("success", i%2 == 0),
		)
	}
}

// ExampleNewAsync demonstrates asynchronous logging.
func ExampleNewAsync() {
	config := stdout.DefaultConfig()
	logger, err := stdout.NewAsync(config, 1000) // Buffer size of 1000
	if err != nil {
		panic(err)
	}
	defer logger.Close() // Important: close to flush remaining logs

	// Async logging - logs are processed in background
	for i := 0; i < 100; i++ {
		logger.Info("Async log message",
			log.Int("message_id", i),
			log.String("type", "async"),
		)
	}

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)
}

// ExampleConfig_ToJSON demonstrates config serialization.
func ExampleConfig_ToJSON() {
	config := stdout.NewConfigBuilder().
		WithLevel(log.InfoLevel).
		WithTimeFormat(time.RFC3339).
		WithCaller(true).
		Build()

	jsonData, err := config.ToJSON()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Config JSON: %s\n", jsonData)
}

// ExampleConfig_FromJSON demonstrates config deserialization.
func ExampleConfig_FromJSON() {
	jsonConfig := `{
		"level": 1,
		"time_format": "2006-01-02T15:04:05Z07:00",
		"enable_caller": true,
		"enable_stacktrace": true,
		"disable_colors": false,
		"development": true,
		"field_names": {
			"time": "timestamp",
			"level": "level",
			"message": "message",
			"caller": "caller"
		}
	}`

	config := &stdout.Config{}
	err := config.FromJSON([]byte(jsonConfig))
	if err != nil {
		panic(err)
	}

	logger, err := stdout.New(config)
	if err != nil {
		panic(err)
	}

	logger.Info("Logger created from JSON config")
}

// Example_webServerLogging demonstrates logging in a web server context.
func Example_webServerLogging() {
	// Create base logger
	logger, err := stdout.New(stdout.GetPresetConfig("production"))
	if err != nil {
		panic(err)
	}

	// Create service logger
	serviceLogger := logger.With(
		log.String("service", "api-gateway"),
		log.String("version", "1.2.3"),
	)

	// Simulate request handling
	handleRequest := func(requestID, userID, method, path string) {
		requestLogger := serviceLogger.With(
			log.String("request_id", requestID),
			log.String("user_id", userID),
			log.String("method", method),
			log.String("path", path),
		)

		start := time.Now()
		requestLogger.Info("Request started")

		// Simulate processing
		time.Sleep(10 * time.Millisecond)

		// Log completion
		duration := time.Since(start)
		requestLogger.Info("Request completed",
			log.Duration("duration", duration),
			log.Int("status_code", 200),
			log.Int64("response_size", 1024),
		)
	}

	// Handle some requests
	handleRequest("req-001", "user-123", "GET", "/api/users")
	handleRequest("req-002", "user-456", "POST", "/api/orders")
	handleRequest("req-003", "user-789", "PUT", "/api/profile")
}

// Example_errorHandling demonstrates error logging patterns.
func Example_errorHandling() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Function that might fail
	processData := func(data string) error {
		if data == "" {
			return errors.New("data cannot be empty")
		}
		if len(data) > 100 {
			return errors.New("data too long")
		}
		return nil
	}

	// Process some data with error handling
	testData := []string{"", "valid data", "this is a very long string that exceeds the maximum allowed length for processing"}

	for i, data := range testData {
		logger.Info("Processing data", log.Int("index", i), log.String("data", data))

		if err := processData(data); err != nil {
			logger.Error("Failed to process data",
				log.Int("index", i),
				log.String("data", data),
				log.Error(err),
				log.String("operation", "process_data"),
			)
		} else {
			logger.Info("Data processed successfully",
				log.Int("index", i),
				log.String("status", "success"),
			)
		}
	}
}

// Example_structuredLogging demonstrates structured logging best practices.
func Example_structuredLogging() {
	logger, err := stdout.New(stdout.GetPresetConfig("development"))
	if err != nil {
		panic(err)
	}

	// Application startup
	logger.Info("Application starting",
		log.String("app_name", "stargate"),
		log.String("version", "1.0.0"),
		log.String("environment", "development"),
		log.String("config_file", "/etc/stargate/config.yaml"),
	)

	// Database connection
	logger.Info("Connecting to database",
		log.String("host", "localhost"),
		log.Int("port", 5432),
		log.String("database", "stargate_db"),
		log.String("user", "stargate_user"),
	)

	// Successful connection
	logger.Info("Database connected successfully",
		log.Duration("connection_time", 150*time.Millisecond),
		log.Int("max_connections", 100),
		log.String("driver", "postgres"),
	)

	// Business logic
	logger.Info("Processing user registration",
		log.String("user_email", "user@example.com"),
		log.String("user_role", "customer"),
		log.Bool("email_verified", false),
		log.Time("registration_time", time.Now()),
	)

	// Performance metrics
	logger.Info("Request metrics",
		log.Duration("response_time", 45*time.Millisecond),
		log.Int("requests_per_second", 150),
		log.Float64("cpu_usage", 23.5),
		log.Float64("memory_usage", 67.8),
	)
}
