package mq

import (
	"context"
	"time"
)

// PublishCallback is called when an async publish operation completes
type PublishCallback func(message *Message, err error)

// Producer defines the interface for message producers
type Producer interface {
	// Publish publishes a single message to the specified topic
	Publish(ctx context.Context, topic string, message *Message) error
	
	// PublishWithOptions publishes a message with custom options
	PublishWithOptions(ctx context.Context, topic string, message *Message, opts *PublishOptions) error
	
	// PublishBatch publishes multiple messages to the specified topic in a single operation
	PublishBatch(ctx context.Context, topic string, messages []*Message) error
	
	// PublishBatchWithOptions publishes multiple messages with custom options
	PublishBatchWithOptions(ctx context.Context, topic string, messages []*Message, opts *PublishOptions) error
	
	// PublishAsync publishes a message asynchronously and calls the callback when complete
	PublishAsync(ctx context.Context, topic string, message *Message, callback PublishCallback) error
	
	// PublishAsyncWithOptions publishes a message asynchronously with custom options
	PublishAsyncWithOptions(ctx context.Context, topic string, message *Message, opts *PublishOptions, callback PublishCallback) error
	
	// Flush flushes any pending messages
	Flush(ctx context.Context) error
	
	// Close closes the producer and releases resources
	Close() error
	
	// Health returns the health status of the producer
	Health(ctx context.Context) HealthStatus
	
	// GetMetrics returns producer metrics
	GetMetrics() ProducerMetrics
}

// ProducerMetrics contains metrics for a producer
type ProducerMetrics struct {
	// Total messages published
	MessagesPublished int64 `json:"messages_published"`
	
	// Total bytes published
	BytesPublished int64 `json:"bytes_published"`
	
	// Total publish errors
	PublishErrors int64 `json:"publish_errors"`
	
	// Average publish latency in milliseconds
	AvgPublishLatency float64 `json:"avg_publish_latency"`
	
	// Current pending messages
	PendingMessages int64 `json:"pending_messages"`
	
	// Connection status
	Connected bool `json:"connected"`
	
	// Last error
	LastError string `json:"last_error,omitempty"`
	
	// Timestamp of last update
	LastUpdated time.Time `json:"last_updated"`
}

// ProducerFactory defines the interface for creating producers
type ProducerFactory interface {
	// CreateProducer creates a new producer with the given configuration
	CreateProducer(config *ProducerConfig) (Producer, error)
	
	// ValidateConfig validates the producer configuration
	ValidateConfig(config *ProducerConfig) error
	
	// GetSupportedFeatures returns the features supported by this factory
	GetSupportedFeatures() []string
}

// Serializer defines the interface for message serialization
type Serializer interface {
	// Serialize converts data to bytes
	Serialize(data interface{}) ([]byte, error)
	
	// Deserialize converts bytes to data
	Deserialize(data []byte, target interface{}) error
	
	// ContentType returns the content type for this serializer
	ContentType() string
}

// Compressor defines the interface for message compression
type Compressor interface {
	// Compress compresses the input data
	Compress(data []byte) ([]byte, error)
	
	// Decompress decompresses the input data
	Decompress(data []byte) ([]byte, error)
	
	// Type returns the compression type
	Type() CompressionType
}

// MessageBuilder provides a fluent interface for building messages
type MessageBuilder struct {
	message *Message
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		message: &Message{
			Headers:   make(map[string]string),
			Timestamp: time.Now(),
		},
	}
}

// WithID sets the message ID
func (b *MessageBuilder) WithID(id string) *MessageBuilder {
	b.message.ID = id
	return b
}

// WithTopic sets the message topic
func (b *MessageBuilder) WithTopic(topic string) *MessageBuilder {
	b.message.Topic = topic
	return b
}

// WithPayload sets the message payload
func (b *MessageBuilder) WithPayload(payload []byte) *MessageBuilder {
	b.message.Payload = payload
	return b
}

// WithKey sets the message key
func (b *MessageBuilder) WithKey(key string) *MessageBuilder {
	b.message.Key = key
	return b
}

// WithHeader adds a header to the message
func (b *MessageBuilder) WithHeader(key, value string) *MessageBuilder {
	b.message.Headers[key] = value
	return b
}

// WithHeaders sets multiple headers
func (b *MessageBuilder) WithHeaders(headers map[string]string) *MessageBuilder {
	for k, v := range headers {
		b.message.Headers[k] = v
	}
	return b
}

// WithPriority sets the message priority
func (b *MessageBuilder) WithPriority(priority int) *MessageBuilder {
	b.message.Priority = priority
	return b
}

// WithTTL sets the message TTL
func (b *MessageBuilder) WithTTL(ttl time.Duration) *MessageBuilder {
	b.message.TTL = ttl
	return b
}

// WithTimestamp sets the message timestamp
func (b *MessageBuilder) WithTimestamp(timestamp time.Time) *MessageBuilder {
	b.message.Timestamp = timestamp
	return b
}

// Build returns the constructed message
func (b *MessageBuilder) Build() *Message {
	return b.message
}

// APIUsageEventBuilder provides a fluent interface for building API usage events
type APIUsageEventBuilder struct {
	event *APIUsageEvent
}

// NewAPIUsageEventBuilder creates a new API usage event builder
func NewAPIUsageEventBuilder() *APIUsageEventBuilder {
	return &APIUsageEventBuilder{
		event: &APIUsageEvent{
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithRequestID sets the request ID
func (b *APIUsageEventBuilder) WithRequestID(requestID string) *APIUsageEventBuilder {
	b.event.RequestID = requestID
	return b
}

// WithApplicationID sets the application ID
func (b *APIUsageEventBuilder) WithApplicationID(appID string) *APIUsageEventBuilder {
	b.event.ApplicationID = appID
	return b
}

// WithUserID sets the user ID
func (b *APIUsageEventBuilder) WithUserID(userID string) *APIUsageEventBuilder {
	b.event.UserID = userID
	return b
}

// WithMethod sets the HTTP method
func (b *APIUsageEventBuilder) WithMethod(method string) *APIUsageEventBuilder {
	b.event.Method = method
	return b
}

// WithPath sets the API path
func (b *APIUsageEventBuilder) WithPath(path string) *APIUsageEventBuilder {
	b.event.Path = path
	return b
}

// WithStatusCode sets the HTTP status code
func (b *APIUsageEventBuilder) WithStatusCode(statusCode int) *APIUsageEventBuilder {
	b.event.StatusCode = statusCode
	return b
}

// WithResponseTime sets the response time in milliseconds
func (b *APIUsageEventBuilder) WithResponseTime(responseTime int64) *APIUsageEventBuilder {
	b.event.ResponseTime = responseTime
	return b
}

// WithRequestSize sets the request size in bytes
func (b *APIUsageEventBuilder) WithRequestSize(size int64) *APIUsageEventBuilder {
	b.event.RequestSize = size
	return b
}

// WithResponseSize sets the response size in bytes
func (b *APIUsageEventBuilder) WithResponseSize(size int64) *APIUsageEventBuilder {
	b.event.ResponseSize = size
	return b
}

// WithClientIP sets the client IP address
func (b *APIUsageEventBuilder) WithClientIP(ip string) *APIUsageEventBuilder {
	b.event.ClientIP = ip
	return b
}

// WithUserAgent sets the user agent
func (b *APIUsageEventBuilder) WithUserAgent(userAgent string) *APIUsageEventBuilder {
	b.event.UserAgent = userAgent
	return b
}

// WithMetadata adds metadata to the event
func (b *APIUsageEventBuilder) WithMetadata(key string, value interface{}) *APIUsageEventBuilder {
	b.event.Metadata[key] = value
	return b
}

// WithTimestamp sets the event timestamp
func (b *APIUsageEventBuilder) WithTimestamp(timestamp time.Time) *APIUsageEventBuilder {
	b.event.Timestamp = timestamp
	return b
}

// Build returns the constructed API usage event
func (b *APIUsageEventBuilder) Build() *APIUsageEvent {
	return b.event
}

// ToMessage converts the API usage event to a message
func (b *APIUsageEventBuilder) ToMessage(serializer Serializer) (*Message, error) {
	event := b.Build()
	payload, err := serializer.Serialize(event)
	if err != nil {
		return nil, err
	}
	
	return NewMessageBuilder().
		WithTopic("api.usage").
		WithPayload(payload).
		WithKey(event.ApplicationID).
		WithHeader("event_type", "api_usage").
		WithHeader("application_id", event.ApplicationID).
		WithHeader("user_id", event.UserID).
		WithTimestamp(event.Timestamp).
		Build(), nil
}
