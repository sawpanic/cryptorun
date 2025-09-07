package stream

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestStubBus_BasicOperations(t *testing.T) {
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create stub bus: %v", err)
	}
	
	ctx := context.Background()
	
	// Test lifecycle
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Test health check
	health := bus.Health()
	if !health.Healthy {
		t.Errorf("Expected bus to be healthy, got unhealthy: %+v", health)
	}
	
	// Test topic creation
	topicConfig := TopicConfig{
		Name:              "test-topic",
		Partitions:        3,
		ReplicationFactor: 1,
		RetentionTime:     24 * time.Hour,
		MaxMessageSize:    1024 * 1024,
	}
	
	err = bus.CreateTopic(ctx, topicConfig)
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}
	
	// Test topic info
	info, err := bus.GetTopicInfo(ctx, "test-topic")
	if err != nil {
		t.Fatalf("Failed to get topic info: %v", err)
	}
	
	if info.Name != "test-topic" {
		t.Errorf("Expected topic name 'test-topic', got '%s'", info.Name)
	}
	
	if len(info.Partitions) != 3 {
		t.Errorf("Expected 3 partitions, got %d", len(info.Partitions))
	}
}

func TestStubBus_PublishSubscribe(t *testing.T) {
	config := DefaultStubConfig()
	
	// Set up metrics collection
	metricsCollected := make(map[string]int)
	var metricsMu sync.Mutex
	config.MetricsCallback = func(metric string, value interface{}, tags map[string]string) {
		metricsMu.Lock()
		defer metricsMu.Unlock()
		if intVal, ok := value.(int); ok {
			metricsCollected[metric] += intVal
		}
	}
	
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create stub bus: %v", err)
	}
	
	ctx := context.Background()
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Create topic
	topicConfig := TopicConfig{
		Name:       "test-pubsub",
		Partitions: 1,
	}
	bus.CreateTopic(ctx, topicConfig)
	
	// Set up subscriber
	var receivedMessages []*Message
	var messagesMu sync.Mutex
	
	handler := func(ctx context.Context, message *Message) error {
		messagesMu.Lock()
		defer messagesMu.Unlock()
		receivedMessages = append(receivedMessages, message)
		return nil
	}
	
	err = bus.Subscribe(ctx, "test-pubsub", "test-group", handler)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	
	// Publish messages
	testMessages := []string{
		"message 1",
		"message 2", 
		"message 3",
	}
	
	for i, msg := range testMessages {
		err = bus.Publish(ctx, "test-pubsub", "key-"+string(rune(i)), []byte(msg))
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify messages were received
	messagesMu.Lock()
	if len(receivedMessages) != len(testMessages) {
		t.Errorf("Expected %d messages, got %d", len(testMessages), len(receivedMessages))
	}
	
	for i, receivedMsg := range receivedMessages {
		if string(receivedMsg.Payload) != testMessages[i] {
			t.Errorf("Message %d: expected '%s', got '%s'", i, testMessages[i], string(receivedMsg.Payload))
		}
		
		if receivedMsg.Topic != "test-pubsub" {
			t.Errorf("Message %d: expected topic 'test-pubsub', got '%s'", i, receivedMsg.Topic)
		}
	}
	messagesMu.Unlock()
	
	// Check metrics
	metricsMu.Lock()
	if metricsCollected["stream_publish_total"] != len(testMessages) {
		t.Errorf("Expected %d publish metrics, got %d", len(testMessages), metricsCollected["stream_publish_total"])
	}
	
	if metricsCollected["stream_consume_total"] != len(testMessages) {
		t.Errorf("Expected %d consume metrics, got %d", len(testMessages), metricsCollected["stream_consume_total"])
	}
	metricsMu.Unlock()
}

func TestStubBus_BatchPublish(t *testing.T) {
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create stub bus: %v", err)
	}
	
	ctx := context.Background()
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Create topic
	topicConfig := TopicConfig{Name: "batch-test", Partitions: 1}
	bus.CreateTopic(ctx, topicConfig)
	
	// Prepare batch messages
	messages := []Message{
		{Topic: "batch-test", Key: "key1", Payload: []byte("batch message 1")},
		{Topic: "batch-test", Key: "key2", Payload: []byte("batch message 2")},
		{Topic: "batch-test", Key: "key3", Payload: []byte("batch message 3")},
	}
	
	// Publish batch
	err = bus.PublishBatch(ctx, messages)
	if err != nil {
		t.Fatalf("Failed to publish batch: %v", err)
	}
	
	// Verify messages were stored
	stubBus := bus.(*StubBus)
	storedMessages := stubBus.GetAllMessages("batch-test")
	
	if len(storedMessages) != len(messages) {
		t.Errorf("Expected %d stored messages, got %d", len(messages), len(storedMessages))
	}
}

func TestStubBus_FilteredSubscription(t *testing.T) {
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create stub bus: %v", err)
	}
	
	ctx := context.Background()
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Create topic
	topicConfig := TopicConfig{Name: "filter-test", Partitions: 1}
	bus.CreateTopic(ctx, topicConfig)
	
	// Set up filtered subscriber (only accept messages with key starting with "important")
	var receivedMessages []*Message
	var messagesMu sync.Mutex
	
	filter := func(message *Message) bool {
		return len(message.Key) > 9 && message.Key[:9] == "important"
	}
	
	handler := func(ctx context.Context, message *Message) error {
		messagesMu.Lock()
		defer messagesMu.Unlock()
		receivedMessages = append(receivedMessages, message)
		return nil
	}
	
	err = bus.SubscribeWithFilter(ctx, "filter-test", "filtered-group", filter, handler)
	if err != nil {
		t.Fatalf("Failed to subscribe with filter: %v", err)
	}
	
	// Publish mixed messages
	testMessages := map[string]string{
		"important-1":   "This should be received",
		"regular-1":     "This should be filtered",
		"important-2":   "This should be received",
		"regular-2":     "This should be filtered",
		"important-3":   "This should be received",
	}
	
	for key, payload := range testMessages {
		bus.Publish(ctx, "filter-test", key, []byte(payload))
	}
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify only "important" messages were received
	messagesMu.Lock()
	expectedCount := 3 // Only important-1, important-2, important-3
	if len(receivedMessages) != expectedCount {
		t.Errorf("Expected %d filtered messages, got %d", expectedCount, len(receivedMessages))
	}
	
	for _, msg := range receivedMessages {
		if len(msg.Key) <= 9 || msg.Key[:9] != "important" {
			t.Errorf("Received message with key '%s' that should have been filtered", msg.Key)
		}
	}
	messagesMu.Unlock()
}

func TestEventBusTypes(t *testing.T) {
	config := DefaultStubConfig()
	
	// Test supported bus types
	supportedTypes := []BusType{BusTypeStub, BusTypeKafka, BusTypePulsar}
	
	for _, busType := range supportedTypes {
		t.Run(string(busType), func(t *testing.T) {
			bus, err := NewEventBus(busType, config)
			if err != nil {
				t.Fatalf("Failed to create %s bus: %v", busType, err)
			}
			
			// Basic lifecycle test
			ctx := context.Background()
			err = bus.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start %s bus: %v", busType, err)
			}
			
			health := bus.Health()
			if !health.Healthy {
				t.Errorf("%s bus is not healthy: %+v", busType, health)
			}
			
			err = bus.Stop(ctx)
			if err != nil {
				t.Fatalf("Failed to stop %s bus: %v", busType, err)
			}
		})
	}
	
	// Test unsupported bus type
	_, err := NewEventBus(BusType("unsupported"), config)
	if err != ErrUnsupportedBusType {
		t.Errorf("Expected ErrUnsupportedBusType, got: %v", err)
	}
}

func TestTopicManagement(t *testing.T) {
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create bus: %v", err)
	}
	
	ctx := context.Background()
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Test topic creation
	topicConfig := TopicConfig{
		Name:              "mgmt-test",
		Partitions:        2,
		ReplicationFactor: 1,
		RetentionTime:     12 * time.Hour,
		MaxMessageSize:    512 * 1024,
	}
	
	err = bus.CreateTopic(ctx, topicConfig)
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}
	
	// Test duplicate creation (should fail)
	err = bus.CreateTopic(ctx, topicConfig)
	if err == nil {
		t.Error("Expected error when creating duplicate topic, got nil")
	}
	
	// Test topic info retrieval
	info, err := bus.GetTopicInfo(ctx, "mgmt-test")
	if err != nil {
		t.Fatalf("Failed to get topic info: %v", err)
	}
	
	if info.Name != "mgmt-test" {
		t.Errorf("Expected topic name 'mgmt-test', got '%s'", info.Name)
	}
	
	// Test topic deletion
	err = bus.DeleteTopic(ctx, "mgmt-test")
	if err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}
	
	// Test accessing deleted topic
	_, err = bus.GetTopicInfo(ctx, "mgmt-test")
	if err != ErrTopicNotFound {
		t.Errorf("Expected ErrTopicNotFound, got: %v", err)
	}
}

func TestBusConfiguration(t *testing.T) {
	// Test default configurations
	configs := map[BusType]func() BusConfig{
		BusTypeStub:   DefaultStubConfig,
		BusTypeKafka:  DefaultKafkaConfig,
		BusTypePulsar: DefaultPulsarConfig,
	}
	
	for busType, configFunc := range configs {
		t.Run(string(busType), func(t *testing.T) {
			config := configFunc()
			
			// Verify basic config fields
			if len(config.Brokers) == 0 {
				t.Errorf("Default config for %s has no brokers", busType)
			}
			
			if config.ClientID == "" {
				t.Errorf("Default config for %s has empty client ID", busType)
			}
			
			if config.ConnectTimeout <= 0 {
				t.Errorf("Default config for %s has invalid connect timeout", busType)
			}
			
			// Verify retry config
			if config.RetryConfig.MaxRetries < 0 {
				t.Errorf("Default config for %s has negative max retries", busType)
			}
			
			if config.RetryConfig.InitialDelay <= 0 {
				t.Errorf("Default config for %s has invalid initial delay", busType)
			}
		})
	}
}

// Benchmark tests
func BenchmarkStubBus_Publish(b *testing.B) {
	config := DefaultStubConfig()
	config.MetricsCallback = nil // Disable metrics for cleaner benchmark
	
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		b.Fatalf("Failed to create bus: %v", err)
	}
	
	ctx := context.Background()
	bus.Start(ctx)
	defer bus.Stop(ctx)
	
	topicConfig := TopicConfig{Name: "bench-topic", Partitions: 1}
	bus.CreateTopic(ctx, topicConfig)
	
	payload := []byte("benchmark message payload")
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			bus.Publish(ctx, "bench-topic", "key", payload)
			i++
		}
	})
}

func BenchmarkStubBus_BatchPublish(b *testing.B) {
	config := DefaultStubConfig()
	config.MetricsCallback = nil
	
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		b.Fatalf("Failed to create bus: %v", err)
	}
	
	ctx := context.Background()
	bus.Start(ctx)
	defer bus.Stop(ctx)
	
	topicConfig := TopicConfig{Name: "bench-batch-topic", Partitions: 1}
	bus.CreateTopic(ctx, topicConfig)
	
	// Prepare batch of 10 messages
	batchSize := 10
	messages := make([]Message, batchSize)
	for i := 0; i < batchSize; i++ {
		messages[i] = Message{
			Topic:   "bench-batch-topic",
			Key:     "batch-key",
			Payload: []byte("batch message payload"),
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.PublishBatch(ctx, messages)
	}
}