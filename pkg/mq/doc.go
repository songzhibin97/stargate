// Package mq provides interfaces and types for message queue operations.
//
// This package defines a comprehensive message queue abstraction layer that supports
// various messaging patterns including publish-subscribe, point-to-point, and
// request-response. It's designed to work with multiple message queue backends
// such as Kafka, RabbitMQ, Redis, NATS, and others.
//
// # Architecture
//
// The mq package follows a clean architecture pattern with the following components:
//
//   - Producer: Interface for publishing messages to topics/queues
//   - Consumer: Interface for consuming messages from topics/queues
//   - Message: Core message structure with headers, payload, and metadata
//   - Serializer: Interface for message serialization/deserialization
//   - Compressor: Interface for message compression/decompression
//
// # Key Features
//
//   - Multiple backend support (Kafka, RabbitMQ, Redis, NATS, etc.)
//   - Synchronous and asynchronous message publishing
//   - Batch operations for high throughput
//   - Message compression and serialization
//   - Dead letter queue support
//   - Consumer groups and load balancing
//   - Offset management and seeking
//   - Health monitoring and metrics
//   - Retry mechanisms with exponential backoff
//   - Message filtering and routing
//
// # Usage Examples
//
// ## Basic Producer Usage
//
//	// Create producer configuration
//	config := &mq.ProducerConfig{
//		Brokers:   []string{"localhost:9092"},
//		ClientID:  "api-gateway-producer",
//		Timeout:   30 * time.Second,
//		Compression: mq.CompressionGzip,
//		Serialization: mq.SerializationJSON,
//	}
//
//	// Create producer
//	producer, err := factory.CreateProducer(config)
//	if err != nil {
//		log.Fatal("Failed to create producer:", err)
//	}
//	defer producer.Close()
//
//	// Build and publish a message
//	message := mq.NewMessageBuilder().
//		WithTopic("api.usage").
//		WithPayload([]byte(`{"user_id": "123", "action": "api_call"}`)).
//		WithKey("user-123").
//		WithHeader("content-type", "application/json").
//		Build()
//
//	err = producer.Publish(ctx, "api.usage", message)
//	if err != nil {
//		log.Error("Failed to publish message:", err)
//	}
//
// ## API Usage Event Publishing
//
//	// Create API usage event
//	event := mq.NewAPIUsageEventBuilder().
//		WithRequestID("req-123").
//		WithApplicationID("app-456").
//		WithUserID("user-789").
//		WithMethod("GET").
//		WithPath("/api/v1/users").
//		WithStatusCode(200).
//		WithResponseTime(150).
//		WithRequestSize(1024).
//		WithResponseSize(2048).
//		WithClientIP("192.168.1.100").
//		WithUserAgent("MyApp/1.0").
//		Build()
//
//	// Convert to message and publish
//	message, err := mq.NewAPIUsageEventBuilder().
//		WithRequestID(event.RequestID).
//		WithApplicationID(event.ApplicationID).
//		// ... other fields
//		ToMessage(jsonSerializer)
//	if err != nil {
//		return err
//	}
//
//	err = producer.Publish(ctx, "api.usage", message)
//	if err != nil {
//		return err
//	}
//
// ## Batch Publishing
//
//	var messages []*mq.Message
//	for i := 0; i < 100; i++ {
//		message := mq.NewMessageBuilder().
//			WithTopic("metrics").
//			WithPayload([]byte(fmt.Sprintf(`{"metric": "counter", "value": %d}`, i))).
//			WithKey(fmt.Sprintf("metric-%d", i)).
//			Build()
//		messages = append(messages, message)
//	}
//
//	err = producer.PublishBatch(ctx, "metrics", messages)
//	if err != nil {
//		log.Error("Failed to publish batch:", err)
//	}
//
// ## Async Publishing with Callback
//
//	callback := func(message *mq.Message, err error) {
//		if err != nil {
//			log.Error("Async publish failed:", err)
//		} else {
//			log.Info("Message published successfully:", message.ID)
//		}
//	}
//
//	err = producer.PublishAsync(ctx, "notifications", message, callback)
//	if err != nil {
//		log.Error("Failed to start async publish:", err)
//	}
//
// ## Basic Consumer Usage
//
//	// Create consumer configuration
//	config := &mq.ConsumerConfig{
//		Brokers:        []string{"localhost:9092"},
//		GroupID:        "api-usage-processor",
//		ClientID:       "consumer-1",
//		Topics:         []string{"api.usage"},
//		AutoCommit:     false,
//		CommitInterval: 5 * time.Second,
//	}
//
//	// Create consumer
//	consumer, err := factory.CreateConsumer(config)
//	if err != nil {
//		log.Fatal("Failed to create consumer:", err)
//	}
//	defer consumer.Close()
//
//	// Define message handler
//	handler := func(ctx context.Context, message *mq.Message) error {
//		log.Info("Received message:", string(message.Payload))
//		
//		// Process the message
//		if err := processMessage(message); err != nil {
//			return err
//		}
//		
//		// Manually commit the message
//		return consumer.CommitMessage(ctx, message)
//	}
//
//	// Subscribe to topic
//	err = consumer.Subscribe(ctx, "api.usage", handler)
//	if err != nil {
//		log.Fatal("Failed to subscribe:", err)
//	}
//
// ## API Usage Event Processing
//
//	// Create API usage event processor
//	processor := mq.NewAPIUsageEventProcessor(jsonSerializer, func(ctx context.Context, event *mq.APIUsageEvent) error {
//		// Process the API usage event
//		log.Info("Processing API usage:", event.RequestID, event.ApplicationID, event.Path)
//		
//		// Store in database, update metrics, etc.
//		return storeAPIUsage(event)
//	})
//
//	// Use processor as message handler
//	handler := func(ctx context.Context, message *mq.Message) error {
//		return processor.ProcessMessage(ctx, message)
//	}
//
//	err = consumer.Subscribe(ctx, "api.usage", handler)
//	if err != nil {
//		return err
//	}
//
// ## Consumer with Options
//
//	opts := &mq.SubscribeOptions{
//		StartFromBeginning: true,
//		MaxRetries:         3,
//		RetryDelay:         1 * time.Second,
//		DeadLetterTopic:    "api.usage.dlq",
//		BatchSize:          10,
//		BatchTimeout:       5 * time.Second,
//	}
//
//	err = consumer.SubscribeWithOptions(ctx, "api.usage", handler, opts)
//	if err != nil {
//		return err
//	}
//
// ## Error Handling
//
//	err = producer.Publish(ctx, "topic", message)
//	if err != nil {
//		switch {
//		case mq.IsConnectionError(err):
//			// Handle connection issues - maybe retry
//			log.Warn("Connection error, retrying:", err)
//			time.Sleep(1 * time.Second)
//			// retry logic
//		case mq.IsTimeoutError(err):
//			// Handle timeout - maybe increase timeout or retry
//			log.Warn("Timeout error:", err)
//		case mq.IsSerializationError(err):
//			// Handle serialization issues - fix data format
//			log.Error("Serialization error:", err)
//			return err
//		case mq.IsConfigurationError(err):
//			// Handle config issues - fix configuration
//			log.Error("Configuration error:", err)
//			return err
//		default:
//			log.Error("Unexpected error:", err)
//			return err
//		}
//	}
//
// ## Health Monitoring
//
//	// Check producer health
//	health := producer.Health(ctx)
//	if health.Status != "healthy" {
//		log.Warn("Producer unhealthy:", health.Message)
//	}
//
//	// Get producer metrics
//	metrics := producer.GetMetrics()
//	log.Info("Producer metrics:",
//		"messages_published", metrics.MessagesPublished,
//		"avg_latency", metrics.AvgPublishLatency,
//		"pending", metrics.PendingMessages)
//
//	// Check consumer health
//	health = consumer.Health(ctx)
//	if health.Status != "healthy" {
//		log.Warn("Consumer unhealthy:", health.Message)
//	}
//
//	// Get consumer metrics
//	consumerMetrics := consumer.GetMetrics()
//	log.Info("Consumer metrics:",
//		"messages_consumed", consumerMetrics.MessagesConsumed,
//		"lag", consumerMetrics.Lag,
//		"processing_errors", consumerMetrics.ProcessingErrors)
//
// # Implementation Guidelines
//
// When implementing these interfaces:
//
//   - Ensure thread-safety for concurrent access
//   - Support context cancellation and timeouts
//   - Implement proper error handling with structured error types
//   - Provide health check and metrics functionality
//   - Support graceful shutdown and resource cleanup
//   - Handle connection failures and automatic reconnection
//   - Implement proper backpressure mechanisms
//   - Support message ordering guarantees where needed
//   - Provide configuration validation
//   - Support both sync and async operations
//
// # Performance Considerations
//
//   - Use batch operations for high throughput scenarios
//   - Implement connection pooling for multiple producers/consumers
//   - Consider message compression for large payloads
//   - Use appropriate serialization formats (protobuf for performance, JSON for debugging)
//   - Implement proper buffering and batching strategies
//   - Monitor and tune consumer lag and processing rates
//   - Use consumer groups for horizontal scaling
//   - Implement proper offset management strategies
//
package mq
