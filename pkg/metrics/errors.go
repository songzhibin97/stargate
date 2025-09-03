package metrics

import (
	"errors"
	"fmt"
)

// Common errors for metrics operations
var (
	// ErrInvalidName indicates an invalid metric name
	ErrInvalidName = errors.New("invalid metric name")
	
	// ErrInvalidLabel indicates an invalid label name or value
	ErrInvalidLabel = errors.New("invalid label name or value")
	
	// ErrInvalidValue indicates an invalid metric value
	ErrInvalidValue = errors.New("invalid metric value")
	
	// ErrMetricNotFound indicates a metric was not found
	ErrMetricNotFound = errors.New("metric not found")
	
	// ErrAlreadyRegistered indicates a metric is already registered
	ErrAlreadyRegistered = errors.New("metric already registered")
	
	// ErrNotRegistered indicates a metric is not registered
	ErrNotRegistered = errors.New("metric not registered")
	
	// ErrCollectorNotFound indicates a collector was not found
	ErrCollectorNotFound = errors.New("collector not found")
	
	// ErrInvalidBuckets indicates invalid histogram buckets
	ErrInvalidBuckets = errors.New("invalid histogram buckets")
	
	// ErrInvalidObjectives indicates invalid summary objectives
	ErrInvalidObjectives = errors.New("invalid summary objectives")
	
	// ErrProviderClosed indicates the provider is closed
	ErrProviderClosed = errors.New("metrics provider is closed")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrUnsupportedFormat indicates an unsupported format
	ErrUnsupportedFormat = errors.New("unsupported format")
)

// MetricError represents a metric-specific error
type MetricError struct {
	Op     string // operation that failed
	Name   string // metric name
	Labels map[string]string // metric labels
	Err    error  // underlying error
}

// Error implements the error interface
func (e *MetricError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("metrics: %s %s: %v", e.Op, e.Name, e.Err)
	}
	return fmt.Sprintf("metrics: %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *MetricError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target
func (e *MetricError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewMetricError creates a new MetricError
func NewMetricError(op, name string, labels map[string]string, err error) *MetricError {
	return &MetricError{
		Op:     op,
		Name:   name,
		Labels: labels,
		Err:    err,
	}
}

// RegistrationError represents a registration error
type RegistrationError struct {
	Name string
	Err  error
}

// Error implements the error interface
func (e *RegistrationError) Error() string {
	return fmt.Sprintf("registration error for %s: %v", e.Name, e.Err)
}

// Unwrap returns the underlying error
func (e *RegistrationError) Unwrap() error {
	return e.Err
}

// ValidationError represents a validation error
type ValidationError struct {
	Field string
	Value interface{}
	Err   error
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s (value: %v): %v", e.Field, e.Value, e.Err)
}

// Unwrap returns the underlying error
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// IsMetricError checks if an error is a MetricError
func IsMetricError(err error) bool {
	var me *MetricError
	return errors.As(err, &me)
}

// IsRegistrationError checks if an error is a RegistrationError
func IsRegistrationError(err error) bool {
	var re *RegistrationError
	return errors.As(err, &re)
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
