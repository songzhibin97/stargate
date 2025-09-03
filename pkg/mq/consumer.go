package mq

import (
	"context"
	"time"
)

// MessageHandler is called to process received messages
type MessageHandler func(ctx context.Context, message *Message) error

// Consumer defines the interface for message consumers
type Consumer interface {
	// Subscribe subscribes to a topic with a message handler
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	
	// SubscribeWithOptions subscribes to a topic with custom options
	SubscribeWithOptions(ctx context.Context, topic string, handler MessageHandler, opts *SubscribeOptions) error
	
	// SubscribeMultiple subscribes to multiple topics with the same handler
	SubscribeMultiple(ctx context.Context, topics []string, handler MessageHandler) error
	
	// Unsubscribe unsubscribes from a topic
	Unsubscribe(topic string) error
	
	// UnsubscribeAll unsubscribes from all topics
	UnsubscribeAll() error
	
	// Commit manually commits message offsets (for manual commit mode)
	Commit(ctx context.Context) error
	
	// CommitMessage commits a specific message offset
	CommitMessage(ctx context.Context, message *Message) error
	
	// Seek seeks to a specific offset for a topic partition
	Seek(ctx context.Context, topic string, partition int32, offset int64) error
	
	// Pause pauses consumption from specified topics
	Pause(topics []string) error
	
	// Resume resumes consumption from specified topics
	Resume(topics []string) error
	
	// Close closes the consumer and releases resources
	Close() error
	
	// Health returns the health status of the consumer
	Health(ctx context.Context) HealthStatus
	
	// GetMetrics returns consumer metrics
	GetMetrics() ConsumerMetrics
}

// SubscribeOptions contains options for subscribing to topics
type SubscribeOptions struct {
	// StartFromBeginning determines if consumption should start from the beginning
	StartFromBeginning bool
	
	// StartFromEnd determines if consumption should start from the end
	StartFromEnd bool
	
	// StartFromTimestamp starts consumption from a specific timestamp
	StartFromTimestamp *time.Time
	
	// StartFromOffset starts consumption from a specific offset
	StartFromOffset *int64
	
	// MaxRetries for message processing failures
	MaxRetries int
	
	// RetryDelay between retry attempts
	RetryDelay time.Duration
	
	// DeadLetterTopic for failed messages
	DeadLetterTopic string
	
	// FilterExpression for message filtering
	FilterExpression string
	
	// BatchSize for batch processing
	BatchSize int
	
	// BatchTimeout for batch processing
	BatchTimeout time.Duration
}

// ConsumerMetrics contains metrics for a consumer
type ConsumerMetrics struct {
	// Total messages consumed
	MessagesConsumed int64 `json:"messages_consumed"`
	
	// Total bytes consumed
	BytesConsumed int64 `json:"bytes_consumed"`
	
	// Total processing errors
	ProcessingErrors int64 `json:"processing_errors"`
	
	// Average processing latency in milliseconds
	AvgProcessingLatency float64 `json:"avg_processing_latency"`
	
	// Current lag (messages behind)
	Lag int64 `json:"lag"`
	
	// Connection status
	Connected bool `json:"connected"`
	
	// Subscribed topics
	SubscribedTopics []string `json:"subscribed_topics"`
	
	// Last error
	LastError string `json:"last_error,omitempty"`
	
	// Timestamp of last update
	LastUpdated time.Time `json:"last_updated"`
}

// ConsumerFactory defines the interface for creating consumers
type ConsumerFactory interface {
	// CreateConsumer creates a new consumer with the given configuration
	CreateConsumer(config *ConsumerConfig) (Consumer, error)
	
	// ValidateConfig validates the consumer configuration
	ValidateConfig(config *ConsumerConfig) error
	
	// GetSupportedFeatures returns the features supported by this factory
	GetSupportedFeatures() []string
}

// MessageProcessor provides higher-level message processing capabilities
type MessageProcessor interface {
	// ProcessMessage processes a single message
	ProcessMessage(ctx context.Context, message *Message) error
	
	// ProcessBatch processes a batch of messages
	ProcessBatch(ctx context.Context, messages []*Message) error
	
	// HandleError handles processing errors
	HandleError(ctx context.Context, message *Message, err error) error
}

// RetryableMessageProcessor extends MessageProcessor with retry capabilities
type RetryableMessageProcessor interface {
	MessageProcessor
	
	// ShouldRetry determines if a message should be retried
	ShouldRetry(ctx context.Context, message *Message, err error, attempt int) bool
	
	// GetRetryDelay returns the delay before the next retry attempt
	GetRetryDelay(ctx context.Context, message *Message, err error, attempt int) time.Duration
}

// DeadLetterHandler handles messages that cannot be processed
type DeadLetterHandler interface {
	// HandleDeadLetter handles a message that has exceeded retry limits
	HandleDeadLetter(ctx context.Context, message *Message, originalError error) error
}

// MessageFilter filters messages before processing
type MessageFilter interface {
	// ShouldProcess determines if a message should be processed
	ShouldProcess(ctx context.Context, message *Message) bool
}

// BatchProcessor processes messages in batches
type BatchProcessor interface {
	// ProcessBatch processes a batch of messages
	ProcessBatch(ctx context.Context, messages []*Message) error
	
	// GetBatchSize returns the preferred batch size
	GetBatchSize() int
	
	// GetBatchTimeout returns the maximum time to wait for a batch
	GetBatchTimeout() time.Duration
}

// ConsumerGroup represents a consumer group for coordinated consumption
type ConsumerGroup interface {
	// Join joins the consumer group
	Join(ctx context.Context) error
	
	// Leave leaves the consumer group
	Leave(ctx context.Context) error
	
	// GetMembers returns the current group members
	GetMembers(ctx context.Context) ([]string, error)
	
	// GetAssignments returns the current partition assignments
	GetAssignments(ctx context.Context) (map[string][]int32, error)
	
	// Rebalance triggers a group rebalance
	Rebalance(ctx context.Context) error
}

// OffsetManager manages message offsets
type OffsetManager interface {
	// GetOffset returns the current offset for a topic partition
	GetOffset(ctx context.Context, topic string, partition int32) (int64, error)
	
	// SetOffset sets the offset for a topic partition
	SetOffset(ctx context.Context, topic string, partition int32, offset int64) error
	
	// CommitOffset commits the offset for a topic partition
	CommitOffset(ctx context.Context, topic string, partition int32, offset int64) error
	
	// ResetOffset resets the offset for a topic partition
	ResetOffset(ctx context.Context, topic string, partition int32, resetType OffsetResetType) error
}

// OffsetResetType defines how to reset offsets
type OffsetResetType string

const (
	OffsetResetEarliest OffsetResetType = "earliest"
	OffsetResetLatest   OffsetResetType = "latest"
	OffsetResetNone     OffsetResetType = "none"
)

// APIUsageEventProcessor processes API usage events
type APIUsageEventProcessor struct {
	serializer Serializer
	handler    func(ctx context.Context, event *APIUsageEvent) error
}

// NewAPIUsageEventProcessor creates a new API usage event processor
func NewAPIUsageEventProcessor(serializer Serializer, handler func(ctx context.Context, event *APIUsageEvent) error) *APIUsageEventProcessor {
	return &APIUsageEventProcessor{
		serializer: serializer,
		handler:    handler,
	}
}

// ProcessMessage processes an API usage event message
func (p *APIUsageEventProcessor) ProcessMessage(ctx context.Context, message *Message) error {
	var event APIUsageEvent
	if err := p.serializer.Deserialize(message.Payload, &event); err != nil {
		return NewSerializationError("DESERIALIZE_FAILED", "Failed to deserialize API usage event", err)
	}
	
	return p.handler(ctx, &event)
}

// ProcessBatch processes a batch of API usage event messages
func (p *APIUsageEventProcessor) ProcessBatch(ctx context.Context, messages []*Message) error {
	for _, message := range messages {
		if err := p.ProcessMessage(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

// HandleError handles processing errors
func (p *APIUsageEventProcessor) HandleError(ctx context.Context, message *Message, err error) error {
	// Log the error and potentially send to dead letter queue
	return err
}

// MetricsMessageProcessor processes metrics messages
type MetricsMessageProcessor struct {
	serializer Serializer
	handler    func(ctx context.Context, metrics *MetricsMessage) error
}

// NewMetricsMessageProcessor creates a new metrics message processor
func NewMetricsMessageProcessor(serializer Serializer, handler func(ctx context.Context, metrics *MetricsMessage) error) *MetricsMessageProcessor {
	return &MetricsMessageProcessor{
		serializer: serializer,
		handler:    handler,
	}
}

// ProcessMessage processes a metrics message
func (p *MetricsMessageProcessor) ProcessMessage(ctx context.Context, message *Message) error {
	var metrics MetricsMessage
	if err := p.serializer.Deserialize(message.Payload, &metrics); err != nil {
		return NewSerializationError("DESERIALIZE_FAILED", "Failed to deserialize metrics message", err)
	}
	
	return p.handler(ctx, &metrics)
}

// ProcessBatch processes a batch of metrics messages
func (p *MetricsMessageProcessor) ProcessBatch(ctx context.Context, messages []*Message) error {
	for _, message := range messages {
		if err := p.ProcessMessage(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

// HandleError handles processing errors
func (p *MetricsMessageProcessor) HandleError(ctx context.Context, message *Message, err error) error {
	// Log the error and potentially send to dead letter queue
	return err
}
