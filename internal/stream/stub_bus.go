package stream

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// StubBus is a minimal in-memory implementation for testing and development
type StubBus struct {
	config      BusConfig
	started     bool
	mu          sync.RWMutex
	
	topics      map[string]*TopicInfo
	subscribers map[string][]MessageHandler
	messages    map[string][]*Message
	metrics     HealthMetrics
}

// NewStubBus creates a new stub event bus for testing
func NewStubBus(config BusConfig) (EventBus, error) {
	bus := &StubBus{
		config:      config,
		topics:      make(map[string]*TopicInfo),
		subscribers: make(map[string][]MessageHandler),
		messages:    make(map[string][]*Message),
		metrics: HealthMetrics{
			ConnectedBrokers:  1, // Stub always has one "broker"
			ActiveTopics:      0,
			ActiveConsumers:   0,
			ProducerLatencyMS: 0.1, // Very fast in-memory
			ConsumerLatencyMS: 0.1,
		},
	}
	
	return bus, nil
}

// Start initializes the stub bus
func (s *StubBus) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		return nil
	}
	
	s.started = true
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_bus_started", 1, map[string]string{
			"type": "stub",
		})
	}
	
	log.Printf("StubBus started with client_id: %s", s.config.ClientID)
	return nil
}

// Stop shuts down the stub bus
func (s *StubBus) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return nil
	}
	
	s.started = false
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_bus_stopped", 1, map[string]string{
			"type": "stub",
		})
	}
	
	log.Printf("StubBus stopped")
	return nil
}

// Publish sends a message to a topic (immediate in-memory delivery)
func (s *StubBus) Publish(ctx context.Context, topic, key string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return ErrBusNotStarted
	}
	
	// Create message
	message := &Message{
		ID:        fmt.Sprintf("stub-%s-%d", topic, time.Now().UnixNano()),
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"producer": "stub",
		},
		Partition: 0,
		Offset:    int64(len(s.messages[topic])),
	}
	
	// Store message
	s.messages[topic] = append(s.messages[topic], message)
	
	// Immediately deliver to subscribers (synchronous for testing)
	s.deliverToSubscribers(ctx, topic, message)
	
	// Update metrics
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_publish_total", 1, map[string]string{
			"topic": topic,
			"type":  "stub",
		})
		s.config.MetricsCallback("stream_publish_bytes", len(payload), map[string]string{
			"topic": topic,
		})
	}
	
	return nil
}

// PublishBatch sends multiple messages
func (s *StubBus) PublishBatch(ctx context.Context, messages []Message) error {
	// For stub, just call Publish for each message
	for _, msg := range messages {
		if err := s.Publish(ctx, msg.Topic, msg.Key, msg.Payload); err != nil {
			return err
		}
	}
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_publish_batch_total", len(messages), map[string]string{
			"type": "stub",
		})
	}
	
	return nil
}

// Subscribe registers a handler for messages on a topic
func (s *StubBus) Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return ErrBusNotStarted
	}
	
	// Register subscriber
	subscriptionKey := fmt.Sprintf("%s:%s", topic, group)
	s.subscribers[subscriptionKey] = append(s.subscribers[subscriptionKey], handler)
	s.metrics.ActiveConsumers++
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_subscribe_total", 1, map[string]string{
			"topic": topic,
			"group": group,
			"type":  "stub",
		})
	}
	
	log.Printf("StubBus: Subscribed to topic=%s group=%s", topic, group)
	return nil
}

// SubscribeWithFilter subscribes with message filtering
func (s *StubBus) SubscribeWithFilter(ctx context.Context, topic, group string, filter MessageFilter, handler MessageHandler) error {
	filteredHandler := func(ctx context.Context, message *Message) error {
		if filter(message) {
			return handler(ctx, message)
		}
		return nil
	}
	
	return s.Subscribe(ctx, topic, group, filteredHandler)
}

// deliverToSubscribers delivers a message to all subscribers immediately
func (s *StubBus) deliverToSubscribers(ctx context.Context, topic string, message *Message) {
	// Find all subscribers for this topic
	for subscriptionKey, handlers := range s.subscribers {
		if len(subscriptionKey) > len(topic) && subscriptionKey[:len(topic)] == topic && subscriptionKey[len(topic)] == ':' {
			group := subscriptionKey[len(topic)+1:]
			
			// Deliver to all handlers in the group (stub doesn't do load balancing)
			for _, handler := range handlers {
				go s.callHandler(ctx, handler, message, topic, group)
			}
		}
	}
}

// callHandler calls a message handler with error handling
func (s *StubBus) callHandler(ctx context.Context, handler MessageHandler, message *Message, topic, group string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("StubBus: Handler panic for topic=%s group=%s: %v", topic, group, r)
		}
	}()
	
	err := handler(ctx, message)
	if err != nil {
		if s.config.MetricsCallback != nil {
			s.config.MetricsCallback("stream_handler_error_total", 1, map[string]string{
				"topic": topic,
				"group": group,
				"error": err.Error(),
			})
		}
		log.Printf("StubBus: Handler error for topic=%s group=%s: %v", topic, group, err)
	} else {
		if s.config.MetricsCallback != nil {
			s.config.MetricsCallback("stream_consume_total", 1, map[string]string{
				"topic": topic,
				"group": group,
			})
		}
	}
}

// CreateTopic creates a new topic
func (s *StubBus) CreateTopic(ctx context.Context, config TopicConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return ErrBusNotStarted
	}
	
	if _, exists := s.topics[config.Name]; exists {
		return fmt.Errorf("topic %s already exists", config.Name)
	}
	
	// Create simple topic info
	partitions := make([]PartitionInfo, config.Partitions)
	for i := int32(0); i < config.Partitions; i++ {
		partitions[i] = PartitionInfo{
			ID:       i,
			Leader:   0,
			Replicas: []int32{0},
			ISR:      []int32{0},
		}
	}
	
	topicInfo := &TopicInfo{
		Name:       config.Name,
		Partitions: partitions,
		Config: map[string]string{
			"type": "stub",
		},
		CreatedAt: time.Now(),
		Stats:     TopicStats{},
	}
	
	s.topics[config.Name] = topicInfo
	s.messages[config.Name] = make([]*Message, 0)
	s.metrics.ActiveTopics++
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_topic_created", 1, map[string]string{
			"topic": config.Name,
			"type":  "stub",
		})
	}
	
	log.Printf("StubBus: Created topic %s with %d partitions", config.Name, config.Partitions)
	return nil
}

// DeleteTopic deletes a topic
func (s *StubBus) DeleteTopic(ctx context.Context, topic string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return ErrBusNotStarted
	}
	
	if _, exists := s.topics[topic]; !exists {
		return ErrTopicNotFound
	}
	
	delete(s.topics, topic)
	delete(s.messages, topic)
	s.metrics.ActiveTopics--
	
	// Remove subscribers for this topic
	for key := range s.subscribers {
		if len(key) > len(topic) && key[:len(topic)] == topic && key[len(topic)] == ':' {
			delete(s.subscribers, key)
			s.metrics.ActiveConsumers--
		}
	}
	
	if s.config.MetricsCallback != nil {
		s.config.MetricsCallback("stream_topic_deleted", 1, map[string]string{
			"topic": topic,
			"type":  "stub",
		})
	}
	
	log.Printf("StubBus: Deleted topic %s", topic)
	return nil
}

// GetTopicInfo returns information about a topic
func (s *StubBus) GetTopicInfo(ctx context.Context, topic string) (*TopicInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.started {
		return nil, ErrBusNotStarted
	}
	
	topicInfo, exists := s.topics[topic]
	if !exists {
		return nil, ErrTopicNotFound
	}
	
	// Update stats
	messages := s.messages[topic]
	totalBytes := int64(0)
	for _, msg := range messages {
		totalBytes += int64(len(msg.Payload))
	}
	
	topicInfo.Stats = TopicStats{
		MessageCount: int64(len(messages)),
		ByteSize:     totalBytes,
		ConsumerLag:  0, // No lag in stub
		ProducerRate: 0, // Would calculate from recent activity
		ConsumerRate: 0,
	}
	
	// Return a copy
	infoCopy := *topicInfo
	return &infoCopy, nil
}

// Health returns the current health status
func (s *StubBus) Health() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	status := HealthStatus{
		Healthy:   s.started,
		Status:    func() string { if s.started { return "running" } else { return "stopped" } }(),
		Metrics:   s.metrics,
		LastCheck: time.Now(),
	}
	
	if !s.started {
		status.Errors = append(status.Errors, "bus not started")
	}
	
	return status
}

// GetAllMessages returns all messages for a topic (testing helper)
func (s *StubBus) GetAllMessages(topic string) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	messages := s.messages[topic]
	result := make([]*Message, len(messages))
	copy(result, messages)
	return result
}

// ClearTopic removes all messages from a topic (testing helper)
func (s *StubBus) ClearTopic(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages[topic] = nil
}

// DefaultStubConfig returns a minimal config for testing
func DefaultStubConfig() BusConfig {
	return BusConfig{
		Brokers:        []string{"stub://localhost:0"},
		ClientID:       "cryptorun-stub-client",
		ConnectTimeout: 1 * time.Second,
		ProducerConfig: ProducerConfig{
			RequiredAcks:     1,
			CompressionType:  "none",
			MaxMessageBytes:  1048576,
			BatchSize:        100,
			LingerMS:         1,
			EnableIdempotent: false,
		},
		ConsumerConfig: ConsumerConfig{
			GroupID:             "test-group",
			AutoOffsetReset:     "latest",
			EnableAutoCommit:    true,
			MaxPollRecords:      100,
		},
		RetryConfig: RetryConfig{
			MaxRetries:    1,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			BackoffFactor: 1.0,
		},
		DeadLetterConfig: DeadLetterConfig{
			Enabled: false, // Disabled for simplicity in testing
		},
		MetricsEnabled: true,
	}
}