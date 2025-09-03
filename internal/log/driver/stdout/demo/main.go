package main

import (
	"context"
	"errors"
	"time"

	"github.com/songzhibin97/stargate/internal/log/driver/stdout"
	"github.com/songzhibin97/stargate/pkg/log"
)

func main() {
	// Test 1: Basic JSON output
	println("=== Test 1: Basic JSON Output ===")
	testBasicOutput()

	// Test 2: Different log levels
	println("\n=== Test 2: Different Log Levels ===")
	testLogLevels()

	// Test 3: Structured fields
	println("\n=== Test 3: Structured Fields ===")
	testStructuredFields()

	// Test 4: Child logger with context
	println("\n=== Test 4: Child Logger with Context ===")
	testChildLogger()

	// Test 5: Context-aware logging
	println("\n=== Test 5: Context-Aware Logging ===")
	testContextLogging()

	// Test 6: Performance optimized logger
	println("\n=== Test 6: Performance Optimized Logger ===")
	testPerformanceOptimized()

	// Test 7: Custom configuration
	println("\n=== Test 7: Custom Configuration ===")
	testCustomConfiguration()

	// Test 8: Error handling
	println("\n=== Test 8: Error Handling ===")
	testErrorHandling()
}

func testBasicOutput() {
	logger, err := stdout.New(nil)
	if err != nil {
		panic(err)
	}

	logger.Info("Basic JSON output test", log.String("test", "basic"))
}

func testLogLevels() {
	config := stdout.DefaultConfig()
	config.Level = log.DebugLevel
	logger, err := stdout.New(config)
	if err != nil {
		panic(err)
	}

	logger.Debug("Debug level message", log.String("level", "debug"))
	logger.Info("Info level message", log.String("level", "info"))
	logger.Warn("Warning level message", log.String("level", "warn"))
	logger.Error("Error level message", log.String("level", "error"))
}

func testStructuredFields() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	now := time.Now()
	duration := 150 * time.Millisecond

	logger.Info("Structured fields test",
		log.String("string_field", "hello world"),
		log.Int("int_field", 42),
		log.Int64("int64_field", 9223372036854775807),
		log.Float64("float64_field", 3.14159),
		log.Bool("bool_field", true),
		log.Time("time_field", now),
		log.Duration("duration_field", duration),
		log.Any("complex_field", map[string]interface{}{
			"nested": map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			"array": []int{1, 2, 3, 4, 5},
		}),
	)
}

func testChildLogger() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Create service logger
	serviceLogger := logger.With(
		log.String("service", "user-service"),
		log.String("version", "2.1.0"),
		log.String("environment", "production"),
	)

	serviceLogger.Info("Service started")

	// Create request logger
	requestLogger := serviceLogger.With(
		log.String("request_id", "req-123-456"),
		log.String("user_id", "user-789"),
		log.String("method", "POST"),
		log.String("path", "/api/users"),
	)

	requestLogger.Info("Processing request")
	requestLogger.Warn("Request took longer than expected", 
		log.Duration("duration", 2*time.Second),
		log.Int("timeout_threshold_ms", 1000),
	)
}

func testContextLogging() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Create context with trace and request IDs
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	ctx = context.WithValue(ctx, "request_id", "req-def-456")

	// Create logger with context
	contextLogger := logger.WithContext(ctx)

	contextLogger.Info("Request started with context")
	contextLogger.Error("Request failed with context", 
		log.Error(errors.New("validation error")),
		log.String("validation_field", "email"),
	)
}

func testPerformanceOptimized() {
	config := stdout.DefaultConfig()
	config.Level = log.InfoLevel

	logger, err := stdout.NewPerformanceOptimized(config)
	if err != nil {
		panic(err)
	}

	// Log multiple messages to demonstrate performance
	for i := 0; i < 5; i++ {
		logger.Info("Performance optimized logging",
			log.Int("iteration", i),
			log.String("status", "processing"),
			log.Bool("success", i%2 == 0),
			log.Float64("cpu_usage", 23.5+float64(i)),
			log.Duration("response_time", time.Duration(i*10)*time.Millisecond),
		)
	}
}

func testCustomConfiguration() {
	// Test with custom field names and configuration
	config := stdout.NewConfigBuilder().
		WithLevel(log.DebugLevel).
		WithTimeFormat(time.RFC3339Nano).
		WithCaller(true).
		WithStacktrace(true).
		WithDevelopment(true).
		WithTimeFieldName("ts").
		WithLevelFieldName("lvl").
		WithMessageFieldName("msg").
		WithCallerFieldName("source").
		Build()

	logger, err := stdout.New(config)
	if err != nil {
		panic(err)
	}

	logger.Info("Custom configuration test",
		log.String("config_type", "custom"),
		log.Bool("caller_enabled", true),
		log.String("time_format", "RFC3339Nano"),
	)
}

func testErrorHandling() {
	logger, err := stdout.New(stdout.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Test various error scenarios
	sampleErrors := []error{
		errors.New("simple error"),
		errors.New("database connection failed"),
		errors.New("validation failed: email is required"),
	}

	for i, err := range sampleErrors {
		logger.Error("Error handling test",
			log.Int("error_index", i),
			log.Error(err),
			log.String("operation", "test_operation"),
			log.String("component", "error_handler"),
			log.Bool("recoverable", i%2 == 0),
		)
	}

	// Test with additional context
	logger.Error("Complex error scenario",
		log.Error(errors.New("upstream service unavailable")),
		log.String("upstream_service", "payment-service"),
		log.String("endpoint", "https://api.payment.com/charge"),
		log.Int("retry_count", 3),
		log.Duration("timeout", 30*time.Second),
		log.Bool("circuit_breaker_open", true),
		log.Any("request_payload", map[string]interface{}{
			"amount": 99.99,
			"currency": "USD",
			"customer_id": "cust_123",
		}),
	)
}
