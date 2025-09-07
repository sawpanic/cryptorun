package stream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// KafkaBus implements EventBus using Kafka
type KafkaBus struct {
	config      BusConfig
	started     bool
	mu          sync.RWMutex
	
	// Mock/stub components for now (would be replaced with actual Kafka client)
	topics      map[string]*TopicInfo
	subscribers map[string][]MessageHandler
	messages    map[string][]*Message
	metrics     HealthMetrics
}

// NewKafkaBus creates a new Kafka event bus
func NewKafkaBus(config BusConfig) (EventBus, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers must be specified")
	}
	
	bus := &KafkaBus{
		config:      config,
		topics:      make(map[string]*TopicInfo),
		subscribers: make(map[string][]MessageHandler),
		messages:    make(map[string][]*Message),
		metrics: HealthMetrics{
			ConnectedBrokers:  len(config.Brokers),
			ActiveTopics:      0,
			ActiveConsumers:   0,
			ProducerLatencyMS: 0,
			ConsumerLatencyMS: 0,
		},
	}
	
	return bus, nil
}

// Start initializes the Kafka bus
func (k *KafkaBus) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if k.started {
		return nil
	}
	
	// In a real implementation, this would:
	// 1. Connect to Kafka brokers
	// 2. Initialize producer and consumer clients  
	// 3. Set up error handlers and metrics collection
	// 4. Start health check routines
	
	k.started = true
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_bus_started", 1, map[string]string{
			"type": "kafka",
			"client_id": k.config.ClientID,
		})
	}
	
	return nil
}

// Stop shuts down the Kafka bus
func (k *KafkaBus) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return nil
	}
	
	// In a real implementation, this would:
	// 1. Flush pending messages
	// 2. Close producer and consumer connections
	// 3. Clean up resources
	
	k.started = false
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_bus_stopped", 1, map[string]string{
			"type": "kafka",
		})
	}
	
	return nil
}

// Publish sends a message to a topic
func (k *KafkaBus) Publish(ctx context.Context, topic, key string, payload []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return ErrBusNotStarted
	}
	
	// Create message
	message := &Message{
		ID:        fmt.Sprintf("%s-%d", topic, time.Now().UnixNano()),
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		Timestamp: time.Now(),
		Partition: 0, // Simplified - would use partitioner in real impl
		Offset:    int64(len(k.messages[topic])),
	}
	
	// Store message (in real impl, this would send to Kafka)
	k.messages[topic] = append(k.messages[topic], message)
	
	// Update metrics
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_publish_total", 1, map[string]string{
			"topic": topic,
			"type":  "kafka",
		})
		k.config.MetricsCallback("stream_publish_bytes", len(payload), map[string]string{
			"topic": topic,
		})
	}
	
	// Simulate processing with small delay for realism
	select {
	case <-time.After(time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	
	return nil
}

// PublishBatch sends multiple messages atomically  
func (k *KafkaBus) PublishBatch(ctx context.Context, messages []Message) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return ErrBusNotStarted
	}
	
	// In a real implementation, this would use Kafka transactions
	// or batched producer sends for atomicity
	
	totalBytes := 0
	for i := range messages {
		message := &messages[i]
		message.ID = fmt.Sprintf("batch-%d-%d", time.Now().UnixNano(), i)
		message.Timestamp = time.Now()
		message.Offset = int64(len(k.messages[message.Topic]))
		
		k.messages[message.Topic] = append(k.messages[message.Topic], message)
		totalBytes += len(message.Payload)
	}
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_publish_batch_total", len(messages), map[string]string{
			"type": "kafka",
		})
		k.config.MetricsCallback("stream_publish_batch_bytes", totalBytes, map[string]string{
			"type": "kafka",
		})
	}
	
	return nil
}

// Subscribe registers a handler for messages on a topic
func (k *KafkaBus) Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return ErrBusNotStarted
	}
	
	// Register subscriber
	consumerKey := fmt.Sprintf("%s:%s", topic, group)
	k.subscribers[consumerKey] = append(k.subscribers[consumerKey], handler)
	k.metrics.ActiveConsumers++
	
	// Start message delivery goroutine (simplified)
	go k.consumeMessages(ctx, topic, group, handler)
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_subscribe_total", 1, map[string]string{
			"topic": topic,
			"group": group,
			"type":  "kafka",
		})
	}
	
	return nil
}

// SubscribeWithFilter subscribes with message filtering
func (k *KafkaBus) SubscribeWithFilter(ctx context.Context, topic, group string, filter MessageFilter, handler MessageHandler) error {
	// Wrap handler with filter
	filteredHandler := func(ctx context.Context, message *Message) error {
		if filter(message) {
			return handler(ctx, message)
		}
		return nil // Skip filtered messages
	}
	
	return k.Subscribe(ctx, topic, group, filteredHandler)
}

// consumeMessages simulates message consumption (in real impl, would use Kafka consumer)
func (k *KafkaBus) consumeMessages(ctx context.Context, topic, group string, handler MessageHandler) {
	ticker := time.NewTicker(100 * time.Millisecond) // Poll every 100ms
	defer ticker.Stop()
	
	offset := int64(0)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			k.mu.RLock()
			messages := k.messages[topic]
			
			if int64(len(messages)) > offset {
				// Process new messages
				for i := offset; i < int64(len(messages)); i++ {
					message := messages[i]
					
					// Call handler with retry logic
					err := k.callHandlerWithRetry(ctx, handler, message)
					if err != nil {
						// In real impl, would send to DLQ after max retries
						if k.config.MetricsCallback != nil {
							k.config.MetricsCallback("stream_handler_error_total", 1, map[string]string{
								"topic": topic,
								"group": group,
								"error": err.Error(),
							})
						}
					} else {
						if k.config.MetricsCallback != nil {
							k.config.MetricsCallback("stream_consume_total", 1, map[string]string{
								"topic": topic,
								"group": group,
							})
						}
					}
				}
				offset = int64(len(messages))
			}
			k.mu.RUnlock()
		}
	}
}

// callHandlerWithRetry implements retry logic with exponential backoff
func (k *KafkaBus) callHandlerWithRetry(ctx context.Context, handler MessageHandler, message *Message) error {
	var lastErr error
	
	for attempt := 0; attempt <= k.config.RetryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := time.Duration(float64(k.config.RetryConfig.InitialDelay) * 
				float64(k.config.RetryConfig.BackoffFactor) * float64(attempt))
			
			if delay > k.config.RetryConfig.MaxDelay {
				delay = k.config.RetryConfig.MaxDelay
			}
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		err := handler(ctx, message)
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		if k.config.MetricsCallback != nil {
			k.config.MetricsCallback("stream_retries_total", 1, map[string]string{
				"topic":   message.Topic,
				"attempt": fmt.Sprintf("%d", attempt+1),
			})
		}
	}
	
	// Send to DLQ if enabled and max retries exceeded
	if k.config.DeadLetterConfig.Enabled {
		dlqMessage := *message
		dlqMessage.Topic = k.config.DeadLetterConfig.Topic
		dlqMessage.Headers = map[string]string{
			"original_topic": message.Topic,
			"error":          lastErr.Error(),
			"retry_count":    fmt.Sprintf("%d", k.config.RetryConfig.MaxRetries),
		}
		
		// In real implementation, would publish to DLQ topic
		if k.config.MetricsCallback != nil {
			k.config.MetricsCallback("stream_dlq_total", 1, map[string]string{
				"original_topic": message.Topic,
			})
		}
	}
	
	return lastErr
}

// CreateTopic creates a new topic
func (k *KafkaBus) CreateTopic(ctx context.Context, config TopicConfig) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return ErrBusNotStarted
	}
	
	// Check if topic already exists
	if _, exists := k.topics[config.Name]; exists {
		return fmt.Errorf("topic %s already exists", config.Name)
	}
	
	// Create topic info
	partitions := make([]PartitionInfo, config.Partitions)
	for i := int32(0); i < config.Partitions; i++ {
		partitions[i] = PartitionInfo{
			ID:       i,
			Leader:   0, // Simplified
			Replicas: []int32{0},
			ISR:      []int32{0},
		}
	}
	
	topicInfo := &TopicInfo{
		Name:       config.Name,
		Partitions: partitions,
		Config: map[string]string{
			"retention.ms":        fmt.Sprintf("%d", config.RetentionTime.Milliseconds()),
			"max.message.bytes":   fmt.Sprintf("%d", config.MaxMessageSize),
			"cleanup.policy":      func() string { if config.CompactionEnabled { return "compact" } else { return "delete" } }(),
		},
		CreatedAt: time.Now(),
		Stats: TopicStats{
			MessageCount: 0,
			ByteSize:     0,
			ConsumerLag:  0,
		},
	}
	
	k.topics[config.Name] = topicInfo
	k.messages[config.Name] = make([]*Message, 0)
	k.metrics.ActiveTopics++
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_topic_created", 1, map[string]string{
			"topic": config.Name,
		})
	}
	
	return nil
}

// DeleteTopic deletes a topic
func (k *KafkaBus) DeleteTopic(ctx context.Context, topic string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return ErrBusNotStarted
	}
	
	if _, exists := k.topics[topic]; !exists {
		return ErrTopicNotFound
	}
	
	delete(k.topics, topic)
	delete(k.messages, topic)
	k.metrics.ActiveTopics--
	
	if k.config.MetricsCallback != nil {
		k.config.MetricsCallback("stream_topic_deleted", 1, map[string]string{
			"topic": topic,
		})
	}
	
	return nil
}

// GetTopicInfo returns information about a topic
func (k *KafkaBus) GetTopicInfo(ctx context.Context, topic string) (*TopicInfo, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	
	if !k.started {
		return nil, ErrBusNotStarted
	}
	
	topicInfo, exists := k.topics[topic]
	if !exists {
		return nil, ErrTopicNotFound
	}
	
	// Update stats
	messages := k.messages[topic]
	totalBytes := int64(0)
	for _, msg := range messages {
		totalBytes += int64(len(msg.Payload))
	}
	
	topicInfo.Stats = TopicStats{
		MessageCount: int64(len(messages)),
		ByteSize:     totalBytes,
		ConsumerLag:  0, // Simplified
		ProducerRate: 0, // Would calculate based on recent activity
		ConsumerRate: 0, // Would calculate based on recent activity
	}
	
	// Return a copy to prevent external modification
	infoCopy := *topicInfo
	return &infoCopy, nil
}

// Health returns the current health status
func (k *KafkaBus) Health() HealthStatus {
	k.mu.RLock()
	defer k.mu.RUnlock()
	
	status := HealthStatus{
		Healthy:   k.started,
		Status:    func() string { if k.started { return "running" } else { return "stopped" } }(),
		Metrics:   k.metrics,
		LastCheck: time.Now(),
	}
	
	// In a real implementation, would check broker connectivity,
	// consumer group health, etc.
	if !k.started {
		status.Errors = append(status.Errors, "bus not started")
	}
	
	return status
}

// DefaultKafkaConfig returns sensible defaults for Kafka configuration
func DefaultKafkaConfig() BusConfig {
	return BusConfig{
		Brokers:          []string{"localhost:9092"},
		ClientID:         "cryptorun-kafka-client",
		SecurityProtocol: "PLAINTEXT",
		ConnectTimeout:   30 * time.Second,
		ProducerConfig: ProducerConfig{
			RequiredAcks:     1, // Wait for leader
			CompressionType:  "gzip",
			MaxMessageBytes:  1048576, // 1MB
			BatchSize:        16384,   // 16KB
			LingerMS:         5,
			EnableIdempotent: true,
		},
		ConsumerConfig: ConsumerConfig{
			GroupID:                "cryptorun-consumers",
			AutoOffsetReset:        "latest",
			EnableAutoCommit:       true,
			AutoCommitIntervalMS:   5000,
			SessionTimeoutMS:       30000,
			HeartbeatIntervalMS:    3000,
			MaxPollRecords:         500,
			FetchMinBytes:          1,
			FetchMaxWaitMS:         500,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    3,
			InitialDelay:  100 * time.Millisecond,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			JitterEnabled: true,
		},
		DeadLetterConfig: DeadLetterConfig{
			Enabled:         true,
			Topic:           "cryptorun-dlq",
			MaxRetries:      3,
			RetentionTime:   24 * time.Hour,
			QuarantineAfter: 5,
		},
		MetricsEnabled: true,
	}
}