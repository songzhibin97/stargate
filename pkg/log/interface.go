package log

import (
	"context"
	"time"
)

// Logger defines the interface for structured logging operations.
// It provides methods for logging at different levels with structured fields,
// and supports creating child loggers with additional context.
//
// Example usage:
//   logger.Info("User logged in", String("user_id", "123"), Int("attempts", 3))
//   childLogger := logger.With(String("request_id", "abc-123"))
//   childLogger.Error("Database connection failed", Error(err))
type Logger interface {
	// Debug logs a debug message with optional structured fields.
	// Debug messages are typically used for detailed diagnostic information
	// that is only of interest when diagnosing problems.
	Debug(msg string, fields ...Field)

	// Info logs an informational message with optional structured fields.
	// Info messages are used to record general information about program execution.
	Info(msg string, fields ...Field)

	// Warn logs a warning message with optional structured fields.
	// Warning messages indicate that something unexpected happened, or
	// indicate some problem in the near future (e.g. 'disk space low').
	Warn(msg string, fields ...Field)

	// Error logs an error message with optional structured fields.
	// Error messages indicate that the software has not been able to perform
	// some function due to an error condition.
	Error(msg string, fields ...Field)

	// Fatal logs a fatal message with optional structured fields and exits the program.
	// Fatal messages indicate that the program cannot continue and must terminate.
	// This method should be used sparingly and only for truly unrecoverable errors.
	Fatal(msg string, fields ...Field)

	// With creates a new logger instance with additional structured fields.
	// The returned logger will include the provided fields in all subsequent log entries.
	// This is useful for adding context that applies to multiple log statements.
	//
	// Example:
	//   requestLogger := logger.With(String("request_id", requestID), String("user_id", userID))
	//   requestLogger.Info("Processing request")
	//   requestLogger.Error("Request failed", Error(err))
	With(fields ...Field) Logger

	// WithContext creates a new logger instance with context information.
	// The context can be used to extract additional fields like trace IDs,
	// request IDs, or other contextual information for distributed tracing.
	WithContext(ctx context.Context) Logger
}

// Level represents the logging level, determining which messages should be logged.
// Lower numeric values represent more verbose logging levels.
type Level int

const (
	// DebugLevel is the most verbose logging level, used for detailed diagnostic information.
	DebugLevel Level = iota
	// InfoLevel is used for general informational messages about program execution.
	InfoLevel
	// WarnLevel is used for warning messages that indicate potential issues.
	WarnLevel
	// ErrorLevel is used for error messages that indicate failures.
	ErrorLevel
	// FatalLevel is used for fatal errors that require program termination.
	FatalLevel
)

// String returns the string representation of the logging level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Field represents a structured logging field with a key-value pair.
// Fields are used to add structured data to log entries, making them
// more searchable and analyzable in log aggregation systems.
type Field struct {
	Key   string      `json:"key"`   // The field name/key
	Value interface{} `json:"value"` // The field value (can be any type)
}

// String creates a string field for structured logging.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field for structured logging.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field for structured logging.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field for structured logging.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field for structured logging.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Time creates a time field for structured logging.
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a duration field for structured logging.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field for structured logging.
// This is a convenience function that uses "error" as the key.
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value type for structured logging.
// This should be used when the value type is not covered by other field functions.
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}


