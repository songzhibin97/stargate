package mq

import (
	"time"
)

// Message represents a message in the message queue
type Message struct {
	// ID is the unique identifier for the message
	ID string `json:"id"`
	
	// Topic is the topic/queue name where the message is published
	Topic string `json:"topic"`
	
	// Payload is the actual message content
	Payload []byte `json:"payload"`
	
	// Headers contains additional metadata for the message
	Headers map[string]string `json:"headers,omitempty"`
	
	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
	
	// Key is used for partitioning in systems like Kafka
	Key string `json:"key,omitempty"`
	
	// Priority defines message priority (0-9, higher is more important)
	Priority int `json:"priority,omitempty"`
	
	// TTL defines time-to-live for the message
	TTL time.Duration `json:"ttl,omitempty"`
	
	// Retry count for failed message processing
	RetryCount int `json:"retry_count,omitempty"`
	
	// MaxRetries defines maximum retry attempts
	MaxRetries int `json:"max_retries,omitempty"`
}

// PublishOptions contains options for publishing messages
type PublishOptions struct {
	// Sync determines if the publish should be synchronous
	Sync bool
	
	// Timeout for the publish operation
	Timeout time.Duration
	
	// Compression type for the message payload
	Compression CompressionType
	
	// Serialization format for the message
	Serialization SerializationType
	
	// DeduplicationID for message deduplication
	DeduplicationID string
	
	// DelaySeconds delays message delivery
	DelaySeconds int
	
	// Headers to add to the message
	Headers map[string]string
}

// CompressionType represents message compression algorithms
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionLZ4  CompressionType = "lz4"
	CompressionZstd CompressionType = "zstd"
)

// SerializationType represents message serialization formats
type SerializationType string

const (
	SerializationJSON     SerializationType = "json"
	SerializationProtobuf SerializationType = "protobuf"
	SerializationAvro     SerializationType = "avro"
	SerializationRaw      SerializationType = "raw"
)

// ProducerConfig contains configuration for message producers
type ProducerConfig struct {
	// Brokers is the list of broker addresses
	Brokers []string `yaml:"brokers" json:"brokers"`
	
	// ClientID identifies the producer client
	ClientID string `yaml:"client_id" json:"client_id"`
	
	// Timeout for operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// RetryConfig for failed operations
	RetryConfig RetryConfig `yaml:"retry" json:"retry"`
	
	// BatchConfig for batch publishing
	BatchConfig BatchConfig `yaml:"batch" json:"batch"`
	
	// Compression settings
	Compression CompressionType `yaml:"compression" json:"compression"`
	
	// Serialization settings
	Serialization SerializationType `yaml:"serialization" json:"serialization"`
	
	// Security settings
	Security SecurityConfig `yaml:"security" json:"security"`
	
	// Additional driver-specific options
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// ConsumerConfig contains configuration for message consumers
type ConsumerConfig struct {
	// Brokers is the list of broker addresses
	Brokers []string `yaml:"brokers" json:"brokers"`
	
	// GroupID for consumer group
	GroupID string `yaml:"group_id" json:"group_id"`
	
	// ClientID identifies the consumer client
	ClientID string `yaml:"client_id" json:"client_id"`
	
	// Topics to subscribe to
	Topics []string `yaml:"topics" json:"topics"`
	
	// AutoCommit enables automatic offset commits
	AutoCommit bool `yaml:"auto_commit" json:"auto_commit"`
	
	// CommitInterval for automatic commits
	CommitInterval time.Duration `yaml:"commit_interval" json:"commit_interval"`
	
	// MaxPollRecords limits records per poll
	MaxPollRecords int `yaml:"max_poll_records" json:"max_poll_records"`
	
	// SessionTimeout for consumer sessions
	SessionTimeout time.Duration `yaml:"session_timeout" json:"session_timeout"`
	
	// Security settings
	Security SecurityConfig `yaml:"security" json:"security"`
	
	// Additional driver-specific options
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// RetryConfig contains retry configuration
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	
	// InitialInterval is the initial retry interval
	InitialInterval time.Duration `yaml:"initial_interval" json:"initial_interval"`
	
	// MaxInterval is the maximum retry interval
	MaxInterval time.Duration `yaml:"max_interval" json:"max_interval"`
	
	// Multiplier for exponential backoff
	Multiplier float64 `yaml:"multiplier" json:"multiplier"`
	
	// Jitter adds randomness to retry intervals
	Jitter bool `yaml:"jitter" json:"jitter"`
}

// BatchConfig contains batch processing configuration
type BatchConfig struct {
	// Size is the maximum batch size
	Size int `yaml:"size" json:"size"`
	
	// Timeout is the maximum time to wait for a batch
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// FlushInterval forces batch flush at intervals
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	// TLS configuration
	TLS TLSConfig `yaml:"tls" json:"tls"`
	
	// SASL configuration
	SASL SASLConfig `yaml:"sasl" json:"sasl"`
}

// TLSConfig contains TLS configuration
type TLSConfig struct {
	// Enabled determines if TLS is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// CertFile path to certificate file
	CertFile string `yaml:"cert_file" json:"cert_file"`
	
	// KeyFile path to private key file
	KeyFile string `yaml:"key_file" json:"key_file"`
	
	// CAFile path to CA certificate file
	CAFile string `yaml:"ca_file" json:"ca_file"`
	
	// InsecureSkipVerify skips certificate verification
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
}

// SASLConfig contains SASL authentication configuration
type SASLConfig struct {
	// Enabled determines if SASL is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Mechanism specifies the SASL mechanism
	Mechanism string `yaml:"mechanism" json:"mechanism"`
	
	// Username for authentication
	Username string `yaml:"username" json:"username"`
	
	// Password for authentication
	Password string `yaml:"password" json:"password"`
}

// HealthStatus represents the health status of a message queue component
type HealthStatus struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// APIUsageEvent represents an API usage event for metrics collection
type APIUsageEvent struct {
	// RequestID uniquely identifies the API request
	RequestID string `json:"request_id"`
	
	// ApplicationID identifies the application making the request
	ApplicationID string `json:"application_id"`
	
	// UserID identifies the user owning the application
	UserID string `json:"user_id"`
	
	// Method is the HTTP method used
	Method string `json:"method"`
	
	// Path is the API endpoint path
	Path string `json:"path"`
	
	// StatusCode is the HTTP response status code
	StatusCode int `json:"status_code"`
	
	// ResponseTime is the request processing time in milliseconds
	ResponseTime int64 `json:"response_time"`
	
	// RequestSize is the size of the request in bytes
	RequestSize int64 `json:"request_size"`
	
	// ResponseSize is the size of the response in bytes
	ResponseSize int64 `json:"response_size"`
	
	// Timestamp when the request was made
	Timestamp time.Time `json:"timestamp"`
	
	// IP address of the client
	ClientIP string `json:"client_ip"`
	
	// User agent of the client
	UserAgent string `json:"user_agent"`
	
	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MetricsMessage represents a metrics data message
type MetricsMessage struct {
	// Type of metrics (usage, performance, error, etc.)
	Type string `json:"type"`
	
	// Source component generating the metrics
	Source string `json:"source"`
	
	// Data contains the actual metrics data
	Data interface{} `json:"data"`
	
	// Tags for categorizing metrics
	Tags map[string]string `json:"tags,omitempty"`
	
	// Timestamp when the metrics were collected
	Timestamp time.Time `json:"timestamp"`
}
