package stream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PulsarBus implements EventBus using Apache Pulsar
type PulsarBus struct {
	config      BusConfig
	started     bool
	mu          sync.RWMutex
	
	// Mock/stub components for now (would be replaced with actual Pulsar client)
	topics      map[string]*TopicInfo
	subscribers map[string][]MessageHandler
	messages    map[string][]*Message
	metrics     HealthMetrics
}

// NewPulsarBus creates a new Pulsar event bus
func NewPulsarBus(config BusConfig) (EventBus, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("pulsar brokers must be specified")
	}
	
	bus := &PulsarBus{
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

// Start initializes the Pulsar bus
func (p *PulsarBus) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.started {
		return nil
	}
	
	// In a real implementation, this would:
	// 1. Connect to Pulsar brokers
	// 2. Initialize producer and consumer clients
	// 3. Set up schema registry if needed
	// 4. Start health check routines
	// 5. Configure multi-tenant namespaces
	
	p.started = true
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_bus_started", 1, map[string]string{
			"type": "pulsar",
			"client_id": p.config.ClientID,
		})
	}
	
	return nil
}

// Stop shuts down the Pulsar bus
func (p *PulsarBus) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return nil
	}
	
	// In a real implementation, this would:
	// 1. Flush pending messages
	// 2. Close producer and consumer connections
	// 3. Clean up resources and subscriptions
	
	p.started = false
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_bus_stopped", 1, map[string]string{
			"type": "pulsar",
		})
	}
	
	return nil
}

// Publish sends a message to a topic
func (p *PulsarBus) Publish(ctx context.Context, topic, key string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return ErrBusNotStarted
	}
	
	// Create message with Pulsar-specific features
	message := &Message{
		ID:        fmt.Sprintf("pulsar-%s-%d", topic, time.Now().UnixNano()),
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"producer": "cryptorun-pulsar",
			"version":  "1.0",
		},
		Partition: 0, // Pulsar handles partitioning automatically
		Offset:    int64(len(p.messages[topic])),
	}
	
	// Store message (in real impl, this would send to Pulsar)
	p.messages[topic] = append(p.messages[topic], message)
	
	// Update metrics
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_publish_total", 1, map[string]string{
			"topic": topic,
			"type":  "pulsar",
		})
		p.config.MetricsCallback("stream_publish_bytes", len(payload), map[string]string{
			"topic": topic,
		})
	}
	
	// Simulate Pulsar's async acknowledgment
	select {
	case <-time.After(2 * time.Millisecond): // Slightly higher latency than Kafka
	case <-ctx.Done():
		return ctx.Err()
	}
	
	return nil
}

// PublishBatch sends multiple messages (Pulsar supports batch publishing natively)
func (p *PulsarBus) PublishBatch(ctx context.Context, messages []Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return ErrBusNotStarted
	}
	
	// Pulsar has native batch support with configurable batching policies
	totalBytes := 0
	for i := range messages {
		message := &messages[i]
		message.ID = fmt.Sprintf("pulsar-batch-%d-%d", time.Now().UnixNano(), i)
		message.Timestamp = time.Now()
		message.Offset = int64(len(p.messages[message.Topic]))
		
		if message.Headers == nil {
			message.Headers = make(map[string]string)
		}
		message.Headers["batch_index"] = fmt.Sprintf("%d", i)
		message.Headers["batch_size"] = fmt.Sprintf("%d", len(messages))
		
		p.messages[message.Topic] = append(p.messages[message.Topic], message)
		totalBytes += len(message.Payload)
	}
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_publish_batch_total", len(messages), map[string]string{
			"type": "pulsar",
		})
		p.config.MetricsCallback("stream_publish_batch_bytes", totalBytes, map[string]string{
			"type": "pulsar",
		})
	}
	
	return nil
}

// Subscribe registers a handler for messages on a topic
func (p *PulsarBus) Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return ErrBusNotStarted
	}
	
	// Register subscriber with Pulsar-style subscription
	subscriptionKey := fmt.Sprintf("%s:%s", topic, group)
	p.subscribers[subscriptionKey] = append(p.subscribers[subscriptionKey], handler)
	p.metrics.ActiveConsumers++
	
	// Start message delivery with Pulsar's push model
	go p.consumeMessages(ctx, topic, group, handler)
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_subscribe_total", 1, map[string]string{
			"topic":        topic,
			"subscription": group,
			"type":         "pulsar",
		})
	}
	
	return nil
}

// SubscribeWithFilter subscribes with message filtering (Pulsar supports server-side filtering)
func (p *PulsarBus) SubscribeWithFilter(ctx context.Context, topic, group string, filter MessageFilter, handler MessageHandler) error {
	// In a real Pulsar implementation, this would use Pulsar's SQL-like filtering
	// For now, wrap the handler with client-side filtering
	filteredHandler := func(ctx context.Context, message *Message) error {
		if filter(message) {
			return handler(ctx, message)
		}
		// In Pulsar, filtered messages are acknowledged automatically
		return nil
	}
	
	return p.Subscribe(ctx, topic, group, filteredHandler)
}

// consumeMessages simulates Pulsar's push-based consumption model
func (p *PulsarBus) consumeMessages(ctx context.Context, topic, subscription string, handler MessageHandler) {
	// Pulsar uses push model vs Kafka's pull model
	ticker := time.NewTicker(50 * time.Millisecond) // More frequent polling for push simulation
	defer ticker.Stop()
	
	cursor := int64(0)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.RLock()
			messages := p.messages[topic]
			
			if int64(len(messages)) > cursor {
				// Process new messages with Pulsar's acknowledgment model
				for i := cursor; i < int64(len(messages)); i++ {
					message := messages[i]
					
					// Pulsar supports individual message acknowledgment
					err := p.processMessageWithAck(ctx, handler, message, subscription)
					if err != nil {
						// Pulsar automatically retries failed messages
						if p.config.MetricsCallback != nil {
							p.config.MetricsCallback("stream_handler_error_total", 1, map[string]string{
								"topic":        topic,
								"subscription": subscription,
								"error":        err.Error(),
							})
						}
					} else {
						if p.config.MetricsCallback != nil {
							p.config.MetricsCallback("stream_consume_total", 1, map[string]string{
								"topic":        topic,
								"subscription": subscription,
							})
						}
					}
				}
				cursor = int64(len(messages))
			}
			p.mu.RUnlock()
		}
	}
}

// processMessageWithAck handles message processing with Pulsar-style acknowledgment
func (p *PulsarBus) processMessageWithAck(ctx context.Context, handler MessageHandler, message *Message, subscription string) error {
	// Set processing timeout context
	processingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Attempt to process message
	err := handler(processingCtx, message)
	if err != nil {
		// In Pulsar, failed messages can be:
		// 1. Retried automatically (with exponential backoff)
		// 2. Sent to retry topic
		// 3. Sent to dead letter topic after max retries
		
		if p.config.DeadLetterConfig.Enabled {
			// Simulate retry logic
			return p.retryMessageWithDLQ(ctx, handler, message, subscription, 1)
		}
		return err
	}
	
	// Simulate positive acknowledgment
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_message_acked", 1, map[string]string{
			"topic":        message.Topic,
			"subscription": subscription,
		})
	}
	
	return nil
}

// retryMessageWithDLQ implements Pulsar's retry and dead letter queue mechanism
func (p *PulsarBus) retryMessageWithDLQ(ctx context.Context, handler MessageHandler, message *Message, subscription string, attempt int) error {
	maxRetries := p.config.DeadLetterConfig.MaxRetries
	
	if attempt > maxRetries {
		// Send to dead letter queue
		dlqMessage := *message
		dlqMessage.Topic = p.config.DeadLetterConfig.Topic
		dlqMessage.Headers = map[string]string{
			"original_topic":    message.Topic,
			"original_subscription": subscription,
			"retry_count":       fmt.Sprintf("%d", maxRetries),
			"dlq_reason":        "max_retries_exceeded",
		}
		
		// In real implementation, would publish to DLQ
		if p.config.MetricsCallback != nil {
			p.config.MetricsCallback("stream_dlq_total", 1, map[string]string{
				"original_topic": message.Topic,
				"subscription":   subscription,
			})
		}
		
		return fmt.Errorf("message sent to DLQ after %d retries", maxRetries)
	}
	
	// Calculate retry delay with exponential backoff
	delay := time.Duration(float64(p.config.RetryConfig.InitialDelay) * 
		float64(p.config.RetryConfig.BackoffFactor) * float64(attempt-1))
	
	if delay > p.config.RetryConfig.MaxDelay {
		delay = p.config.RetryConfig.MaxDelay
	}
	
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	
	// Retry message processing
	err := handler(ctx, message)
	if err != nil {
		if p.config.MetricsCallback != nil {
			p.config.MetricsCallback("stream_retries_total", 1, map[string]string{
				"topic":        message.Topic,
				"subscription": subscription,
				"attempt":      fmt.Sprintf("%d", attempt),
			})
		}
		
		// Recursive retry
		return p.retryMessageWithDLQ(ctx, handler, message, subscription, attempt+1)
	}
	
	return nil
}

// CreateTopic creates a new topic (Pulsar topics are created automatically, but this supports explicit creation)
func (p *PulsarBus) CreateTopic(ctx context.Context, config TopicConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return ErrBusNotStarted
	}
	
	// Check if topic already exists
	if _, exists := p.topics[config.Name]; exists {
		return fmt.Errorf("topic %s already exists", config.Name)
	}
	
	// Pulsar topics support both partitioned and non-partitioned modes
	partitions := make([]PartitionInfo, config.Partitions)
	for i := int32(0); i < config.Partitions; i++ {
		partitions[i] = PartitionInfo{
			ID:       i,
			Leader:   0, // Pulsar manages this automatically
			Replicas: []int32{0, 1}, // Pulsar supports configurable replication
			ISR:      []int32{0, 1},
		}
	}
	
	topicInfo := &TopicInfo{
		Name:       config.Name,
		Partitions: partitions,
		Config: map[string]string{
			"retention.time":      fmt.Sprintf("%d", config.RetentionTime.Milliseconds()),
			"retention.size":      "1GB", // Pulsar supports both time and size retention
			"max.message.size":    fmt.Sprintf("%d", config.MaxMessageSize),
			"compaction.enabled":  fmt.Sprintf("%t", config.CompactionEnabled),
			"schema.type":         "BYTES", // Default schema
		},
		CreatedAt: time.Now(),
		Stats: TopicStats{
			MessageCount: 0,
			ByteSize:     0,
			ConsumerLag:  0,
		},
	}
	
	p.topics[config.Name] = topicInfo
	p.messages[config.Name] = make([]*Message, 0)
	p.metrics.ActiveTopics++
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_topic_created", 1, map[string]string{
			"topic": config.Name,
			"type":  "pulsar",
		})
	}
	
	return nil
}

// DeleteTopic deletes a topic and all its subscriptions
func (p *PulsarBus) DeleteTopic(ctx context.Context, topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.started {
		return ErrBusNotStarted
	}
	
	if _, exists := p.topics[topic]; !exists {
		return ErrTopicNotFound
	}
	
	// In Pulsar, deleting a topic also removes all subscriptions
	delete(p.topics, topic)
	delete(p.messages, topic)
	p.metrics.ActiveTopics--
	
	// Remove subscriptions for this topic
	for key := range p.subscribers {
		if len(key) > len(topic) && key[:len(topic)] == topic && key[len(topic)] == ':' {
			delete(p.subscribers, key)
		}
	}
	
	if p.config.MetricsCallback != nil {
		p.config.MetricsCallback("stream_topic_deleted", 1, map[string]string{
			"topic": topic,
			"type":  "pulsar",
		})
	}
	
	return nil
}

// GetTopicInfo returns information about a topic
func (p *PulsarBus) GetTopicInfo(ctx context.Context, topic string) (*TopicInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if !p.started {
		return nil, ErrBusNotStarted
	}
	
	topicInfo, exists := p.topics[topic]
	if !exists {
		return nil, ErrTopicNotFound
	}
	
	// Update stats with Pulsar-specific metrics
	messages := p.messages[topic]
	totalBytes := int64(0)
	for _, msg := range messages {
		totalBytes += int64(len(msg.Payload))
	}
	
	topicInfo.Stats = TopicStats{
		MessageCount: int64(len(messages)),
		ByteSize:     totalBytes,
		ConsumerLag:  0, // Pulsar provides detailed subscription backlog metrics
		ProducerRate: 0, // Would be calculated from recent activity
		ConsumerRate: 0, // Would be calculated from recent activity  
	}
	
	// Return a copy to prevent external modification
	infoCopy := *topicInfo
	return &infoCopy, nil
}

// Health returns the current health status
func (p *PulsarBus) Health() HealthStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	status := HealthStatus{
		Healthy:   p.started,
		Status:    func() string { if p.started { return "running" } else { return "stopped" } }(),
		Metrics:   p.metrics,
		LastCheck: time.Now(),
	}
	
	// In a real implementation, would check:
	// - Broker connectivity
	// - Schema registry health
	// - Subscription backlogs
	// - BookKeeper ensemble health
	if !p.started {
		status.Errors = append(status.Errors, "bus not started")
	}
	
	return status
}

// DefaultPulsarConfig returns sensible defaults for Pulsar configuration
func DefaultPulsarConfig() BusConfig {
	return BusConfig{
		Brokers:          []string{"pulsar://localhost:6650"},
		ClientID:         "cryptorun-pulsar-client",
		SecurityProtocol: "PLAINTEXT",
		ConnectTimeout:   30 * time.Second,
		ProducerConfig: ProducerConfig{
			RequiredAcks:     1, // Pulsar always waits for acknowledgment
			CompressionType:  "lz4", // Pulsar's preferred compression
			MaxMessageBytes:  5242880, // 5MB (Pulsar's default max)
			BatchSize:        1000,
			LingerMS:         10, // Pulsar batching interval
			EnableIdempotent: true, // Pulsar supports deduplication
		},
		ConsumerConfig: ConsumerConfig{
			GroupID:                "cryptorun-subscriptions",
			AutoOffsetReset:        "latest",
			EnableAutoCommit:       false, // Pulsar uses explicit acknowledgment
			SessionTimeoutMS:       30000,
			HeartbeatIntervalMS:    5000,
			MaxPollRecords:         1000,
			FetchMinBytes:          1,
			FetchMaxWaitMS:         100, // Pulsar's push model has lower latency
		},
		RetryConfig: RetryConfig{
			MaxRetries:    5, // Pulsar supports more sophisticated retry
			InitialDelay:  200 * time.Millisecond,
			MaxDelay:      15 * time.Second,
			BackoffFactor: 1.5,
			JitterEnabled: true,
		},
		DeadLetterConfig: DeadLetterConfig{
			Enabled:         true,
			Topic:           "cryptorun-dlq",
			MaxRetries:      5,
			RetentionTime:   7 * 24 * time.Hour, // Longer retention
			QuarantineAfter: 10,
		},
		MetricsEnabled: true,
	}
}