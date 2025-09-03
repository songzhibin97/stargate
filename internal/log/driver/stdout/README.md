# Stdout JSON Logger Driver

A high-performance, structured JSON logger implementation for the Stargate project that outputs to stdout. This driver implements the `log.Logger` interface and provides comprehensive logging capabilities with excellent performance characteristics.

## Features

- **Structured Logging**: Full support for structured fields with type safety
- **JSON Output**: Clean, parseable JSON format suitable for log aggregation systems
- **High Performance**: Optimized with object pools and efficient field conversion
- **Flexible Configuration**: Extensive configuration options with preset configurations
- **Context Support**: Extract trace IDs, request IDs, and other contextual information
- **Async Logging**: Optional asynchronous logging for high-throughput scenarios
- **Level Filtering**: Efficient log level filtering to reduce overhead
- **Child Loggers**: Create child loggers with additional context fields
- **Thread Safe**: Safe for concurrent use across multiple goroutines

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/songzhibin97/stargate/internal/log/driver/stdout"
    "github.com/songzhibin97/stargate/pkg/log"
)

func main() {
    // Create logger with default configuration
    logger, err := stdout.New(nil)
    if err != nil {
        panic(err)
    }

    // Log messages with structured fields
    logger.Info("Application started", 
        log.String("version", "1.0.0"),
        log.Int("port", 8080),
        log.Bool("debug", true),
    )
}
```

### Using Configuration Builder

```go
config := stdout.NewConfigBuilder().
    WithLevel(log.DebugLevel).
    WithTimeFormat(time.RFC3339).
    WithCaller(true).
    WithStacktrace(true).
    WithDevelopment(true).
    Build()

logger, err := stdout.New(config)
```

### Using Preset Configurations

```go
// Development configuration
devLogger, _ := stdout.New(stdout.GetPresetConfig("development"))

// Production configuration  
prodLogger, _ := stdout.New(stdout.GetPresetConfig("production"))

// Debug configuration
debugLogger, _ := stdout.New(stdout.GetPresetConfig("debug"))
```

### Using Functional Options

```go
logger, err := stdout.NewWithOptions(
    stdout.WithLevelOption(log.InfoLevel),
    stdout.WithTimeFormatOption(time.RFC3339Nano),
    stdout.WithCallerOption(true),
    stdout.WithDevelopmentOption(false),
)
```

## Configuration Options

### Config Structure

```go
type Config struct {
    Level            log.Level   // Minimum logging level
    TimeFormat       string      // Time format for timestamps
    EnableCaller     bool        // Include caller information
    EnableStacktrace bool        // Include stack traces for errors
    DisableColors    bool        // Disable colored output
    Development      bool        // Enable development mode
    FieldNames       FieldNames  // Custom field names
}
```

### Field Names Customization

```go
config := stdout.DefaultConfig()
config.FieldNames = stdout.FieldNames{
    Time:    "ts",
    Level:   "lvl", 
    Message: "msg",
    Caller:  "caller",
}
```

## Advanced Features

### Child Loggers with Context

```go
// Create base logger
logger, _ := stdout.New(stdout.DefaultConfig())

// Create service logger
serviceLogger := logger.With(
    log.String("service", "user-service"),
    log.String("version", "2.1.0"),
)

// Create request logger
requestLogger := serviceLogger.With(
    log.String("request_id", "req-123"),
    log.String("user_id", "user-456"),
)

requestLogger.Info("Processing request") // Includes all context fields
```

### Context-Aware Logging

```go
ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
ctx = context.WithValue(ctx, "request_id", "req-456")

contextLogger := logger.WithContext(ctx)
contextLogger.Info("Request processed") // Includes trace_id and request_id
```

### Performance-Optimized Logger

```go
// Uses object pools to reduce memory allocations
perfLogger, err := stdout.NewPerformanceOptimized(config)
```

### Asynchronous Logging

```go
// Create async logger with buffer size of 1000
asyncLogger, err := stdout.NewAsync(config, 1000)
defer asyncLogger.Close() // Important: flush remaining logs

asyncLogger.Info("Async message") // Non-blocking
```

## Field Types

The logger supports various field types with optimized conversion:

```go
logger.Info("Field types example",
    log.String("string", "value"),
    log.Int("int", 42),
    log.Int64("int64", 123456789),
    log.Float64("float", 3.14),
    log.Bool("bool", true),
    log.Time("time", time.Now()),
    log.Duration("duration", 5*time.Second),
    log.Error(errors.New("example error")),
    log.Any("complex", map[string]interface{}{
        "nested": "data",
    }),
)
```

## JSON Output Format

The logger produces clean, structured JSON output:

```json
{
  "timestamp": "2023-12-07T10:30:45Z",
  "level": "info",
  "message": "User logged in",
  "service": "auth-service",
  "user_id": "user-123",
  "request_id": "req-456",
  "duration": 0.045,
  "success": true
}
```

## Performance Considerations

### Object Pools

The performance-optimized logger uses object pools to reduce garbage collection pressure:

- Field slice pooling
- Zap field slice pooling  
- Buffer pooling for JSON serialization

### Level Filtering

Log level filtering happens early to avoid unnecessary work:

```go
config := stdout.DefaultConfig()
config.Level = log.WarnLevel // Only warn and above will be processed
```

### Async Logging

For high-throughput scenarios, use async logging:

```go
asyncLogger, _ := stdout.NewAsync(config, 10000) // Large buffer
defer asyncLogger.Close()
```

## Best Practices

### 1. Use Structured Fields

```go
// Good
logger.Info("User login successful", 
    log.String("user_id", userID),
    log.Duration("login_time", duration),
)

// Avoid
logger.Info(fmt.Sprintf("User %s login took %v", userID, duration))
```

### 2. Create Context Loggers

```go
// Create loggers with context for related operations
requestLogger := logger.With(
    log.String("request_id", requestID),
    log.String("user_id", userID),
)

requestLogger.Info("Request started")
requestLogger.Info("Validation passed") 
requestLogger.Info("Request completed")
```

### 3. Use Appropriate Log Levels

- `Debug`: Detailed diagnostic information
- `Info`: General application flow
- `Warn`: Potentially harmful situations
- `Error`: Error events that don't stop the application
- `Fatal`: Severe errors that cause application termination

### 4. Handle Errors Properly

```go
if err := someOperation(); err != nil {
    logger.Error("Operation failed",
        log.Error(err),
        log.String("operation", "some_operation"),
        log.String("context", "additional_context"),
    )
}
```

## Testing

Run the test suite:

```bash
go test ./internal/log/driver/stdout/...
```

Run benchmarks:

```bash
go test -bench=. ./internal/log/driver/stdout/
```

## Integration with Stargate

This logger is designed to integrate seamlessly with the Stargate project's logging infrastructure:

```go
// In your service initialization
logger, err := stdout.New(stdout.GetPresetConfig("production"))
if err != nil {
    return fmt.Errorf("failed to create logger: %w", err)
}

// Use throughout your application
service := &MyService{
    logger: logger.With(log.String("component", "my-service")),
}
```

## Thread Safety

All logger instances are thread-safe and can be used concurrently across multiple goroutines without additional synchronization.

## Memory Management

The logger is designed to minimize memory allocations:

- Reuses field slices through object pools
- Efficient field type conversion
- Minimal string allocations
- Optional async processing to reduce blocking

## Error Handling

The logger handles errors gracefully:

- Invalid configurations fall back to defaults
- Field conversion errors are handled transparently
- Async logger falls back to sync on buffer overflow
