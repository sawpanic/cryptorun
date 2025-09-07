package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
	
	"github.com/sawpanic/cryptorun/internal/data"
	"github.com/sawpanic/cryptorun/internal/data/cold"
	"github.com/sawpanic/cryptorun/internal/data/schema"
)

// TestStreamingToColdTier tests the integration between streaming and cold tier storage
func TestStreamingToColdTier(t *testing.T) {
	// Set up temporary directory
	tempDir := t.TempDir()
	
	// Create streaming bus
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create event bus: %v", err)
	}
	
	ctx := context.Background()
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start event bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Create streaming topic
	topicConfig := TopicConfig{
		Name:       "market-data-stream",
		Partitions: 1,
	}
	err = bus.CreateTopic(ctx, topicConfig)
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}
	
	// Set up cold tier storage
	schemaRegistry := schema.NewSchemaRegistry(tempDir + "/schemas")
	err = schemaRegistry.CreateDefaultSchemas()
	if err != nil {
		t.Fatalf("Failed to create schemas: %v", err)
	}
	err = schemaRegistry.LoadSchemas()
	if err != nil {
		t.Fatalf("Failed to load schemas: %v", err)
	}
	
	parquetConfig := cold.DefaultParquetStoreConfig()
	parquetStore := cold.NewParquetStore(parquetConfig, schemaRegistry)
	
	// Set up data collection
	var collectedEnvelopes []*data.Envelope
	var collectionMu sync.Mutex
	
	// Subscribe to stream and convert to cold tier format
	handler := func(ctx context.Context, message *Message) error {
		// Parse message payload as market data
		var marketData map[string]interface{}
		if err := json.Unmarshal(message.Payload, &marketData); err != nil {
			return fmt.Errorf("failed to parse market data: %w", err)
		}
		
		// Convert to data envelope
		envelope := data.NewEnvelope(
			marketData["venue"].(string),
			marketData["symbol"].(string),
			data.TierCold,
			data.WithConfidenceScore(0.9),
		)
		envelope.Timestamp = message.Timestamp
		envelope.Provenance.OriginalSource = "stream_integration_test"
		envelope.OrderBook = marketData
		envelope.Checksum = envelope.GenerateChecksum(marketData, "stream_to_cold")
		
		collectionMu.Lock()
		collectedEnvelopes = append(collectedEnvelopes, envelope)
		collectionMu.Unlock()
		
		return nil
	}
	
	err = bus.Subscribe(ctx, "market-data-stream", "cold-tier-collector", handler)
	if err != nil {
		t.Fatalf("Failed to subscribe to stream: %v", err)
	}
	
	// Publish test market data
	testData := []map[string]interface{}{
		{
			"venue":          "kraken",
			"symbol":         "BTC-USD",
			"best_bid_price": 50000.0,
			"best_ask_price": 50010.0,
			"best_bid_qty":   1.5,
			"best_ask_qty":   2.0,
			"mid_price":      50005.0,
			"spread_bps":     20.0,
		},
		{
			"venue":          "coinbase",
			"symbol":         "ETH-USD",
			"best_bid_price": 3000.0,
			"best_ask_price": 3005.0,
			"best_bid_qty":   10.0,
			"best_ask_qty":   8.0,
			"mid_price":      3002.5,
			"spread_bps":     16.7,
		},
		{
			"venue":          "binance",
			"symbol":         "SOL-USD",
			"best_bid_price": 100.0,
			"best_ask_price": 100.5,
			"best_bid_qty":   50.0,
			"best_ask_qty":   45.0,
			"mid_price":      100.25,
			"spread_bps":     50.0,
		},
	}
	
	for i, data := range testData {
		payload, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Failed to marshal test data %d: %v", i, err)
		}
		
		err = bus.Publish(ctx, "market-data-stream", fmt.Sprintf("key-%d", i), payload)
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}
	
	// Wait for message processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify data was collected
	collectionMu.Lock()
	if len(collectedEnvelopes) != len(testData) {
		t.Fatalf("Expected %d collected envelopes, got %d", len(testData), len(collectedEnvelopes))
	}
	collectionMu.Unlock()
	
	// Write collected data to cold tier
	filePath := tempDir + "/stream_integration_test.parquet"
	err = parquetStore.WriteBatch(ctx, filePath, collectedEnvelopes)
	if err != nil {
		t.Fatalf("Failed to write to cold tier: %v", err)
	}
	
	// Validate PIT integrity
	err = parquetStore.ValidatePIT(filePath)
	if err != nil {
		t.Fatalf("PIT validation failed: %v", err)
	}
	
	// Read back and verify
	readEnvelopes, err := parquetStore.ReadBatch(ctx, filePath, 0)
	if err != nil {
		t.Fatalf("Failed to read from cold tier: %v", err)
	}
	
	if len(readEnvelopes) == 0 {
		t.Fatal("No data read back from cold tier")
	}
	
	t.Logf("Successfully integrated stream→cold tier: %d messages processed", len(readEnvelopes))
}

// TestStreamReplay tests replaying data from cold tier back to streaming
func TestStreamReplay(t *testing.T) {
	tempDir := t.TempDir()
	
	// Set up cold tier with test data first
	schemaRegistry := schema.NewSchemaRegistry(tempDir + "/schemas")
	err := schemaRegistry.CreateDefaultSchemas()
	if err != nil {
		t.Fatalf("Failed to create schemas: %v", err)
	}
	err = schemaRegistry.LoadSchemas()
	if err != nil {
		t.Fatalf("Failed to load schemas: %v", err)
	}
	
	parquetConfig := cold.DefaultParquetStoreConfig()
	parquetStore := cold.NewParquetStore(parquetConfig, schemaRegistry)
	
	// Create historical test data
	historicalData := createHistoricalTestData(50)
	filePath := tempDir + "/historical_replay.parquet"
	
	ctx := context.Background()
	err = parquetStore.WriteBatch(ctx, filePath, historicalData)
	if err != nil {
		t.Fatalf("Failed to write historical data: %v", err)
	}
	
	// Set up streaming bus for replay
	config := DefaultStubConfig()
	bus, err := NewEventBus(BusTypeStub, config)
	if err != nil {
		t.Fatalf("Failed to create event bus: %v", err)
	}
	
	err = bus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start event bus: %v", err)
	}
	defer bus.Stop(ctx)
	
	// Create replay topic
	topicConfig := TopicConfig{
		Name:       "historical-replay",
		Partitions: 1,
	}
	err = bus.CreateTopic(ctx, topicConfig)
	if err != nil {
		t.Fatalf("Failed to create replay topic: %v", err)
	}
	
	// Set up replay subscriber
	var replayedMessages []*Message
	var replayMu sync.Mutex
	
	replayHandler := func(ctx context.Context, message *Message) error {
		replayMu.Lock()
		defer replayMu.Unlock()
		replayedMessages = append(replayedMessages, message)
		return nil
	}
	
	err = bus.Subscribe(ctx, "historical-replay", "replay-consumer", replayHandler)
	if err != nil {
		t.Fatalf("Failed to subscribe to replay: %v", err)
	}
	
	// Read historical data and replay to stream
	readData, err := parquetStore.ReadBatch(ctx, filePath, 0)
	if err != nil {
		t.Fatalf("Failed to read historical data: %v", err)
	}
	
	// Replay with time-based ordering
	for _, envelope := range readData {
		// Convert envelope back to stream message
		orderBookData, ok := envelope.OrderBook.(map[string]interface{})
		if !ok {
			t.Fatalf("Invalid order book data in envelope")
		}
		
		payload, err := json.Marshal(orderBookData)
		if err != nil {
			t.Fatalf("Failed to marshal replay data: %v", err)
		}
		
		// Use original timestamp in message
		message := Message{
			Topic:     "historical-replay",
			Key:       fmt.Sprintf("replay-%s-%s", envelope.Venue, envelope.Symbol),
			Payload:   payload,
			Timestamp: envelope.Timestamp,
			Headers: map[string]string{
				"source":      "cold_tier_replay",
				"venue":       envelope.Venue,
				"symbol":      envelope.Symbol,
				"confidence":  fmt.Sprintf("%.2f", envelope.Provenance.ConfidenceScore),
			},
		}
		
		err = bus.PublishBatch(ctx, []Message{message})
		if err != nil {
			t.Fatalf("Failed to replay message: %v", err)
		}
	}
	
	// Wait for replay processing
	time.Sleep(300 * time.Millisecond)
	
	// Verify replay
	replayMu.Lock()
	defer replayMu.Unlock()
	
	if len(replayedMessages) != len(historicalData) {
		t.Fatalf("Expected %d replayed messages, got %d", len(historicalData), len(replayedMessages))
	}
	
	// Verify message ordering and content
	for i, replayedMsg := range replayedMessages {
		if replayedMsg.Headers["venue"] != historicalData[i].Venue {
			t.Errorf("Message %d: venue mismatch, expected %s, got %s", 
				i, historicalData[i].Venue, replayedMsg.Headers["venue"])
		}
		
		if replayedMsg.Headers["symbol"] != historicalData[i].Symbol {
			t.Errorf("Message %d: symbol mismatch, expected %s, got %s",
				i, historicalData[i].Symbol, replayedMsg.Headers["symbol"])
		}
	}
	
	t.Logf("Successfully replayed %d messages from cold tier to stream", len(replayedMessages))
}

// TestMultiRegionMirrorSimulation simulates multi-region mirroring
func TestMultiRegionMirrorSimulation(t *testing.T) {
	// Create two "regions" with separate buses
	primaryConfig := DefaultStubConfig()
	primaryConfig.ClientID = "primary-region"
	
	secondaryConfig := DefaultStubConfig() 
	secondaryConfig.ClientID = "secondary-region"
	
	primaryBus, err := NewEventBus(BusTypeStub, primaryConfig)
	if err != nil {
		t.Fatalf("Failed to create primary bus: %v", err)
	}
	
	secondaryBus, err := NewEventBus(BusTypeStub, secondaryConfig)
	if err != nil {
		t.Fatalf("Failed to create secondary bus: %v", err)
	}
	
	ctx := context.Background()
	
	err = primaryBus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start primary bus: %v", err)
	}
	defer primaryBus.Stop(ctx)
	
	err = secondaryBus.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start secondary bus: %v", err)
	}
	defer secondaryBus.Stop(ctx)
	
	// Create mirror topics
	topicConfig := TopicConfig{
		Name:       "critical-signals",
		Partitions: 2,
	}
	
	primaryBus.CreateTopic(ctx, topicConfig)
	secondaryBus.CreateTopic(ctx, topicConfig)
	
	// Set up mirroring: primary → secondary
	var mirroredMessages []*Message
	var mirrorMu sync.Mutex
	
	mirrorHandler := func(ctx context.Context, message *Message) error {
		// Add mirror metadata
		mirrorMessage := *message
		if mirrorMessage.Headers == nil {
			mirrorMessage.Headers = make(map[string]string)
		}
		mirrorMessage.Headers["mirrored_from"] = "primary-region"
		mirrorMessage.Headers["mirror_timestamp"] = time.Now().Format(time.RFC3339)
		
		// Publish to secondary region
		err := secondaryBus.Publish(ctx, message.Topic, message.Key, message.Payload)
		if err != nil {
			return fmt.Errorf("mirror publish failed: %w", err)
		}
		
		mirrorMu.Lock()
		mirroredMessages = append(mirroredMessages, &mirrorMessage)
		mirrorMu.Unlock()
		
		return nil
	}
	
	// Subscribe to primary for mirroring
	err = primaryBus.Subscribe(ctx, "critical-signals", "mirror-group", mirrorHandler)
	if err != nil {
		t.Fatalf("Failed to set up mirroring: %v", err)
	}
	
	// Set up secondary consumer to verify mirroring
	var consumedFromSecondary []*Message
	var consumeMu sync.Mutex
	
	consumeHandler := func(ctx context.Context, message *Message) error {
		consumeMu.Lock()
		defer consumeMu.Unlock()
		consumedFromSecondary = append(consumedFromSecondary, message)
		return nil
	}
	
	err = secondaryBus.Subscribe(ctx, "critical-signals", "secondary-consumer", consumeHandler)
	if err != nil {
		t.Fatalf("Failed to set up secondary consumer: %v", err)
	}
	
	// Publish to primary region
	testSignals := []string{
		"BTC signal: momentum breakout detected",
		"ETH signal: regime change to volatile", 
		"SOL signal: funding divergence alert",
	}
	
	for i, signal := range testSignals {
		err = primaryBus.Publish(ctx, "critical-signals", fmt.Sprintf("signal-%d", i), []byte(signal))
		if err != nil {
			t.Fatalf("Failed to publish signal %d: %v", i, err)
		}
	}
	
	// Wait for mirroring and consumption
	time.Sleep(300 * time.Millisecond)
	
	// Verify mirroring worked
	mirrorMu.Lock()
	if len(mirroredMessages) != len(testSignals) {
		t.Errorf("Expected %d mirrored messages, got %d", len(testSignals), len(mirroredMessages))
	}
	mirrorMu.Unlock()
	
	consumeMu.Lock()
	if len(consumedFromSecondary) != len(testSignals) {
		t.Errorf("Expected %d consumed messages in secondary, got %d", len(testSignals), len(consumedFromSecondary))
	}
	
	// Verify mirror metadata
	for i, consumedMsg := range consumedFromSecondary {
		if string(consumedMsg.Payload) != testSignals[i] {
			t.Errorf("Message %d content mismatch: expected %s, got %s",
				i, testSignals[i], string(consumedMsg.Payload))
		}
	}
	consumeMu.Unlock()
	
	t.Logf("Successfully simulated multi-region mirroring: %d signals", len(testSignals))
}

// Helper function to create test data
func createHistoricalTestData(count int) []*data.Envelope {
	var envelopes []*data.Envelope
	baseTime := time.Now().Add(-24 * time.Hour)
	
	venues := []string{"kraken", "coinbase", "binance"}
	symbols := []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	
	for i := 0; i < count; i++ {
		venue := venues[i%len(venues)]
		symbol := symbols[i%len(symbols)]
		
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		envelope := data.NewEnvelope(venue, symbol, data.TierCold,
			data.WithConfidenceScore(0.85),
		)
		envelope.Timestamp = timestamp
		envelope.Provenance.OriginalSource = "historical_test_data"
		envelope.Provenance.LatencyMS = int64(5 + i%20)
		
		basePrice := 50000.0 + float64(i*100)
		orderBookData := map[string]interface{}{
			"venue":          venue,
			"symbol":         symbol,
			"timestamp":      timestamp,
			"best_bid_price": basePrice - 5,
			"best_ask_price": basePrice + 5,
			"best_bid_qty":   1.5 + float64(i%10)*0.1,
			"best_ask_qty":   2.0 + float64(i%10)*0.1,
			"mid_price":      basePrice,
			"spread_bps":     20.0,
			"data_source":    "historical_replay_test",
		}
		
		envelope.OrderBook = orderBookData
		envelope.Checksum = envelope.GenerateChecksum(orderBookData, "historical")
		
		envelopes = append(envelopes, envelope)
	}
	
	return envelopes
}