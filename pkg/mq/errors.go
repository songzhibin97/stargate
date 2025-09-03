package mq

import (
	"errors"
	"fmt"
	"time"
)

// Common error variables
var (
	// Connection errors
	ErrConnectionFailed    = errors.New("connection failed")
	ErrConnectionClosed    = errors.New("connection closed")
	ErrConnectionTimeout   = errors.New("connection timeout")
	ErrBrokerUnavailable   = errors.New("broker unavailable")
	
	// Producer errors
	ErrProducerClosed      = errors.New("producer closed")
	ErrPublishFailed       = errors.New("publish failed")
	ErrPublishTimeout      = errors.New("publish timeout")
	ErrMessageTooLarge     = errors.New("message too large")
	ErrTopicNotFound       = errors.New("topic not found")
	ErrInvalidTopic        = errors.New("invalid topic")
	
	// Consumer errors
	ErrConsumerClosed      = errors.New("consumer closed")
	ErrSubscriptionFailed  = errors.New("subscription failed")
	ErrCommitFailed        = errors.New("commit failed")
	ErrOffsetOutOfRange    = errors.New("offset out of range")
	ErrGroupCoordinator    = errors.New("group coordinator error")
	
	// Serialization errors
	ErrSerializationFailed   = errors.New("serialization failed")
	ErrDeserializationFailed = errors.New("deserialization failed")
	ErrCompressionFailed     = errors.New("compression failed")
	ErrDecompressionFailed   = errors.New("decompression failed")
	
	// Configuration errors
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrMissingConfig       = errors.New("missing configuration")
	ErrInvalidBroker       = errors.New("invalid broker address")
	
	// General errors
	ErrInvalidMessage      = errors.New("invalid message")
	ErrOperationNotSupported = errors.New("operation not supported")
	ErrInternalError       = errors.New("internal error")
)

// ErrorType represents the type of message queue error
type ErrorType string

const (
	ErrorTypeConnection     ErrorType = "connection"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeProducer       ErrorType = "producer"
	ErrorTypeConsumer       ErrorType = "consumer"
	ErrorTypeSerialization  ErrorType = "serialization"
	ErrorTypeConfiguration  ErrorType = "configuration"
	ErrorTypeInternal       ErrorType = "internal"
)

// MQError represents a structured message queue error
type MQError struct {
	Type    ErrorType `json:"type"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Cause   error     `json:"-"`
	Retryable bool    `json:"retryable"`
}

// Error implements the error interface
func (e *MQError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *MQError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *MQError) IsRetryable() bool {
	return e.Retryable
}

// NewConnectionError creates a new connection error
func NewConnectionError(code, message string, retryable bool) *MQError {
	return &MQError{
		Type:      ErrorTypeConnection,
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(code, message string) *MQError {
	return &MQError{
		Type:      ErrorTypeTimeout,
		Code:      code,
		Message:   message,
		Retryable: true,
	}
}

// NewProducerError creates a new producer error
func NewProducerError(code, message string, retryable bool) *MQError {
	return &MQError{
		Type:      ErrorTypeProducer,
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

// NewConsumerError creates a new consumer error
func NewConsumerError(code, message string, retryable bool) *MQError {
	return &MQError{
		Type:      ErrorTypeConsumer,
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

// NewSerializationError creates a new serialization error
func NewSerializationError(code, message string, cause error) *MQError {
	return &MQError{
		Type:      ErrorTypeSerialization,
		Code:      code,
		Message:   message,
		Cause:     cause,
		Retryable: false,
	}
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(code, message string) *MQError {
	return &MQError{
		Type:      ErrorTypeConfiguration,
		Code:      code,
		Message:   message,
		Retryable: false,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(code, message string, cause error) *MQError {
	return &MQError{
		Type:      ErrorTypeInternal,
		Code:      code,
		Message:   message,
		Cause:     cause,
		Retryable: true,
	}
}

// IsConnectionError checks if the error is a connection error
func IsConnectionError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeConnection
	}
	return errors.Is(err, ErrConnectionFailed) || 
		   errors.Is(err, ErrConnectionClosed) || 
		   errors.Is(err, ErrConnectionTimeout) ||
		   errors.Is(err, ErrBrokerUnavailable)
}

// IsTimeoutError checks if the error is a timeout error
func IsTimeoutError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeTimeout
	}
	return errors.Is(err, ErrConnectionTimeout) || 
		   errors.Is(err, ErrPublishTimeout)
}

// IsProducerError checks if the error is a producer error
func IsProducerError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeProducer
	}
	return errors.Is(err, ErrProducerClosed) || 
		   errors.Is(err, ErrPublishFailed) ||
		   errors.Is(err, ErrMessageTooLarge) ||
		   errors.Is(err, ErrTopicNotFound)
}

// IsConsumerError checks if the error is a consumer error
func IsConsumerError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeConsumer
	}
	return errors.Is(err, ErrConsumerClosed) || 
		   errors.Is(err, ErrSubscriptionFailed) ||
		   errors.Is(err, ErrCommitFailed) ||
		   errors.Is(err, ErrOffsetOutOfRange)
}

// IsSerializationError checks if the error is a serialization error
func IsSerializationError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeSerialization
	}
	return errors.Is(err, ErrSerializationFailed) || 
		   errors.Is(err, ErrDeserializationFailed) ||
		   errors.Is(err, ErrCompressionFailed) ||
		   errors.Is(err, ErrDecompressionFailed)
}

// IsConfigurationError checks if the error is a configuration error
func IsConfigurationError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.Type == ErrorTypeConfiguration
	}
	return errors.Is(err, ErrInvalidConfig) || 
		   errors.Is(err, ErrMissingConfig) ||
		   errors.Is(err, ErrInvalidBroker)
}

// IsRetryableError checks if the error is retryable
func IsRetryableError(err error) bool {
	var mqErr *MQError
	if errors.As(err, &mqErr) {
		return mqErr.IsRetryable()
	}
	
	// Default retry logic for standard errors
	return IsConnectionError(err) || IsTimeoutError(err)
}

// RetryStrategy defines retry behavior for different error types
type RetryStrategy struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	Jitter          bool
}

// DefaultRetryStrategy returns a default retry strategy
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:      3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
	}
}

// GetRetryStrategy returns appropriate retry strategy for error type
func GetRetryStrategy(err error) *RetryStrategy {
	if IsTimeoutError(err) {
		return &RetryStrategy{
			MaxRetries:      5,
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     10 * time.Second,
			Multiplier:      1.5,
			Jitter:          true,
		}
	}
	
	if IsConnectionError(err) {
		return &RetryStrategy{
			MaxRetries:      10,
			InitialInterval: 1 * time.Second,
			MaxInterval:     60 * time.Second,
			Multiplier:      2.0,
			Jitter:          true,
		}
	}
	
	return DefaultRetryStrategy()
}
