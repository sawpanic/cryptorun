package data_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/stream"
	"github.com/sawpanic/cryptorun/internal/stream/kafka"
	"github.com/sawpanic/cryptorun/internal/stream/pulsar"
	"github.com/sawpanic/cryptorun/internal/streaming"
)

func TestStreamingEnvelope(t *testing.T) {
	t.Run("envelope_creation", func(t *testing.T) {
		// Test envelope creation using the builder pattern
		envelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"price": 50000.0, "volume": 1000.0}`)).
			WithTimestamp(time.Now()).
			Build()

		require.NoError(t, err)
		require.NotNil(t, envelope)
		assert.NotEmpty(t, envelope.Checksum)

		// Validate envelope
		err = stream.Validate(envelope)
		assert.NoError(t, err)
	})

	t.Run("envelope_validation", func(t *testing.T) {
		// Test various validation scenarios
		validationTests := []struct {
			name          string
			envelope      *stream.Envelope
			expectError   bool
			errorContains string
		}{
			{
				name: "valid_envelope",
				envelope: func() *stream.Envelope {
					envelope, _ := stream.NewBuilder("BTC-USD", "kraken").
						WithPayload([]byte(`{"test": "data"}`)).
						Build()
					return envelope
				}(),
				expectError: false,
			},
			{
				name: "missing_symbol",
				envelope: &stream.Envelope{
					Timestamp: time.Now(),
					Symbol:    "",
					Source:    "kraken",
					Payload:   []byte(`{"test": "data"}`),
					Version:   1,
				},
				expectError:   true,
				errorContains: "symbol",
			},
			{
				name: "missing_source",
				envelope: &stream.Envelope{
					Timestamp: time.Now(),
					Symbol:    "BTC-USD",
					Source:    "",
					Payload:   []byte(`{"test": "data"}`),
					Version:   1,
				},
				expectError:   true,
				errorContains: "source",
			},
			{
				name: "empty_payload",
				envelope: &stream.Envelope{
					Timestamp: time.Now(),
					Symbol:    "BTC-USD",
					Source:    "kraken",
					Payload:   []byte{},
					Version:   1,
				},
				expectError:   true,
				errorContains: "payload",
			},
			{
				name: "zero_timestamp",
				envelope: &stream.Envelope{
					Timestamp: time.Time{},
					Symbol:    "BTC-USD",
					Source:    "kraken",
					Payload:   []byte(`{"test": "data"}`),
					Version:   1,
				},
				expectError:   true,
				errorContains: "timestamp",
			},
		}

		for _, tt := range validationTests {
			t.Run(tt.name, func(t *testing.T) {
				err := stream.Validate(tt.envelope)
				if tt.expectError {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("checksum_validation", func(t *testing.T) {
		envelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"price": 50000.0}`)).
			Build()
		require.NoError(t, err)

		originalChecksum := envelope.Checksum

		// Verify checksum is valid
		assert.True(t, envelope.IsValid())

		// Modify payload and verify checksum is invalid
		envelope.Payload = []byte(`{"price": 51000.0}`)
		assert.False(t, envelope.IsValid())

		// Recalculate checksum
		envelope.SetChecksum()
		assert.NotEqual(t, originalChecksum, envelope.Checksum)

		// Verify new checksum is valid
		assert.True(t, envelope.IsValid())
	})
}

func TestKafkaProducer(t *testing.T) {
	t.Run("kafka_producer_creation", func(t *testing.T) {
		config := kafka.DefaultConfig()
		config.Topic = "test-topic"
		
		producer, err := kafka.NewProducer(config)
		require.NoError(t, err)
		assert.NotNil(t, producer)
		
		defer producer.Close()
		
		// Test stats
		stats := producer.GetStats()
		assert.Equal(t, "test-topic", stats["topic"])
		assert.Equal(t, true, stats["connected"])
	})

	t.Run("kafka_config_validation", func(t *testing.T) {
		// Valid config
		validConfig := kafka.DefaultConfig()
		err := kafka.ValidateConfig(validConfig)
		assert.NoError(t, err)

		// Invalid config - empty brokers
		invalidConfig := validConfig
		invalidConfig.Brokers = []string{}
		err = kafka.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "brokers")

		// Invalid config - empty topic
		invalidConfig = validConfig
		invalidConfig.Topic = ""
		err = kafka.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic")

		// Invalid config - negative batch size
		invalidConfig = validConfig
		invalidConfig.BatchSize = -1
		err = kafka.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch size")

		// Invalid config - invalid compression
		invalidConfig = validConfig
		invalidConfig.Compression = "invalid"
		err = kafka.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "compression")
	})

	t.Run("kafka_message_sending", func(t *testing.T) {
		config := kafka.DefaultConfig()
		producer, err := kafka.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		// Create test envelope
		envelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"price": 50000.0, "volume": 1000.0}`)).
			Build()
		require.NoError(t, err)

		// Send message
		ctx := context.Background()
		err = producer.Send(ctx, "", []*stream.Envelope{envelope})
		assert.NoError(t, err)

		// Check stats
		stats := producer.GetStats()
		messagesSent, _ := stats["messages_sent"].(int64)
		assert.Greater(t, messagesSent, int64(0))
	})

	t.Run("kafka_health_check", func(t *testing.T) {
		config := kafka.DefaultConfig()
		producer, err := kafka.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		ctx := context.Background()
		healthy, err := producer.IsHealthy(ctx)
		assert.NoError(t, err)
		assert.True(t, healthy)
	})

	t.Run("kafka_flush_and_wait", func(t *testing.T) {
		config := kafka.DefaultConfig()
		producer, err := kafka.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		ctx := context.Background()
		err = producer.FlushAndWait(ctx)
		assert.NoError(t, err)
	})

	t.Run("kafka_metrics_callback", func(t *testing.T) {
		config := kafka.DefaultConfig()
		producer, err := kafka.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		// Set metrics callback
		var metricsCalled bool
		var metricName string
		var metricValue int64
		
		producer.SetMetricsCallback(func(name string, value int64) {
			metricsCalled = true
			metricName = name
			metricValue = value
		})

		// Send a message to trigger metrics
		envelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"test": "data"}`)).
			Build()
		require.NoError(t, err)

		ctx := context.Background()
		err = producer.Send(ctx, "", []*stream.Envelope{envelope})
		require.NoError(t, err)

		// Check that metrics callback was called
		assert.True(t, metricsCalled)
		assert.NotEmpty(t, metricName)
		assert.Greater(t, metricValue, int64(0))
	})
}

func TestPulsarProducer(t *testing.T) {
	t.Run("pulsar_producer_creation", func(t *testing.T) {
		config := pulsar.DefaultConfig()
		config.Topic = "test-topic"
		
		producer, err := pulsar.NewProducer(config)
		require.NoError(t, err)
		assert.NotNil(t, producer)
		
		defer producer.Close()
		
		// Test stats
		stats := producer.GetStats()
		assert.Equal(t, "test-topic", stats["topic"])
		assert.Equal(t, true, stats["connected"])
	})

	t.Run("pulsar_config_validation", func(t *testing.T) {
		// Valid config
		validConfig := pulsar.DefaultConfig()
		err := pulsar.ValidateConfig(validConfig)
		assert.NoError(t, err)

		// Invalid config - empty service URL
		invalidConfig := validConfig
		invalidConfig.ServiceURL = ""
		err = pulsar.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service URL")

		// Invalid config - empty topic
		invalidConfig = validConfig
		invalidConfig.Topic = ""
		err = pulsar.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic")

		// Invalid config - negative batch size
		invalidConfig = validConfig
		invalidConfig.BatchSize = -1
		err = pulsar.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch size")

		// Invalid config - invalid compression
		invalidConfig = validConfig
		invalidConfig.Compression = "invalid"
		err = pulsar.ValidateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "compression")
	})

	t.Run("pulsar_message_sending", func(t *testing.T) {
		config := pulsar.DefaultConfig()
		producer, err := pulsar.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		// Create test envelope
		envelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"price": 50000.0, "volume": 1000.0}`)).
			Build()
		require.NoError(t, err)

		// Send message
		ctx := context.Background()
		err = producer.Send(ctx, "", []*stream.Envelope{envelope})
		assert.NoError(t, err)

		// Check stats
		stats := producer.GetStats()
		messagesSent, _ := stats["messages_sent"].(int64)
		assert.Greater(t, messagesSent, int64(0))
	})

	t.Run("pulsar_health_check", func(t *testing.T) {
		config := pulsar.DefaultConfig()
		producer, err := pulsar.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		ctx := context.Background()
		healthy, err := producer.IsHealthy(ctx)
		assert.NoError(t, err)
		assert.True(t, healthy)
	})
}

func TestProducerFactory(t *testing.T) {
	t.Run("kafka_factory_creation", func(t *testing.T) {
		config := streaming.ProducerConfig{
			Type:  streaming.ProducerTypeKafka,
			Kafka: kafka.DefaultConfig(),
		}

		producer, err := streaming.NewProducer(config)
		require.NoError(t, err)
		assert.NotNil(t, producer)
		defer producer.Close()

		// Verify it's working
		ctx := context.Background()
		healthy, err := producer.IsHealthy(ctx)
		assert.NoError(t, err)
		assert.True(t, healthy)
	})

	t.Run("pulsar_factory_creation", func(t *testing.T) {
		config := streaming.ProducerConfig{
			Type:   streaming.ProducerTypePulsar,
			Pulsar: pulsar.DefaultConfig(),
		}

		producer, err := streaming.NewProducer(config)
		require.NoError(t, err)
		assert.NotNil(t, producer)
		defer producer.Close()

		// Verify it's working
		ctx := context.Background()
		healthy, err := producer.IsHealthy(ctx)
		assert.NoError(t, err)
		assert.True(t, healthy)
	})

	t.Run("unsupported_producer_type", func(t *testing.T) {
		config := streaming.ProducerConfig{
			Type: "unsupported",
		}

		producer, err := streaming.NewProducer(config)
		assert.Error(t, err)
		assert.Nil(t, producer)
		assert.Contains(t, err.Error(), "unsupported producer type")
	})

	t.Run("config_validation", func(t *testing.T) {
		// Valid Kafka config
		kafkaConfig := streaming.ProducerConfig{
			Type:  streaming.ProducerTypeKafka,
			Kafka: kafka.DefaultConfig(),
		}
		err := streaming.ValidateProducerConfig(kafkaConfig)
		assert.NoError(t, err)

		// Valid Pulsar config
		pulsarConfig := streaming.ProducerConfig{
			Type:   streaming.ProducerTypePulsar,
			Pulsar: pulsar.DefaultConfig(),
		}
		err = streaming.ValidateProducerConfig(pulsarConfig)
		assert.NoError(t, err)

		// Invalid config
		invalidConfig := streaming.ProducerConfig{
			Type: "invalid",
		}
		err = streaming.ValidateProducerConfig(invalidConfig)
		assert.Error(t, err)
	})

	t.Run("default_configs", func(t *testing.T) {
		// Test default Kafka config
		kafkaConfig := streaming.DefaultProducerConfig(streaming.ProducerTypeKafka)
		assert.Equal(t, streaming.ProducerTypeKafka, kafkaConfig.Type)
		assert.NotEmpty(t, kafkaConfig.Kafka.Brokers)
		assert.NotEmpty(t, kafkaConfig.Kafka.Topic)

		// Test default Pulsar config
		pulsarConfig := streaming.DefaultProducerConfig(streaming.ProducerTypePulsar)
		assert.Equal(t, streaming.ProducerTypePulsar, pulsarConfig.Type)
		assert.NotEmpty(t, pulsarConfig.Pulsar.ServiceURL)
		assert.NotEmpty(t, pulsarConfig.Pulsar.Topic)
	})

	t.Run("supported_producer_types", func(t *testing.T) {
		types := streaming.GetSupportedProducerTypes()
		assert.Contains(t, types, streaming.ProducerTypeKafka)
		assert.Contains(t, types, streaming.ProducerTypePulsar)
		assert.Len(t, types, 2)
	})

	t.Run("producer_stats", func(t *testing.T) {
		config := streaming.ProducerConfig{
			Type:  streaming.ProducerTypeKafka,
			Kafka: kafka.DefaultConfig(),
		}

		producer, err := streaming.NewProducer(config)
		require.NoError(t, err)
		defer producer.Close()

		// Get formatted stats
		stats := streaming.GetProducerStats(producer, streaming.ProducerTypeKafka)
		assert.Equal(t, streaming.ProducerTypeKafka, stats.Type)
		assert.True(t, stats.Connected)
		assert.NotNil(t, stats.Details)
	})
}

// Benchmark streaming operations
func BenchmarkStreamingOperations(b *testing.B) {
	// Create test envelope
	envelope, err := stream.NewBuilder("BTC-USD", "kraken").
		WithPayload([]byte(`{"price": 50000.0, "volume": 1000.0}`)).
		Build()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("kafka_send", func(b *testing.B) {
		config := kafka.DefaultConfig()
		producer, err := kafka.NewProducer(config)
		if err != nil {
			b.Fatal(err)
		}
		defer producer.Close()

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := producer.Send(ctx, "", []*stream.Envelope{envelope})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("pulsar_send", func(b *testing.B) {
		config := pulsar.DefaultConfig()
		producer, err := pulsar.NewProducer(config)
		if err != nil {
			b.Fatal(err)
		}
		defer producer.Close()

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := producer.Send(ctx, "", []*stream.Envelope{envelope})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("envelope_checksum", func(b *testing.B) {
		testEnvelope, err := stream.NewBuilder("BTC-USD", "kraken").
			WithPayload([]byte(`{"price": 50000.0, "volume": 1000.0}`)).
			Build()
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testEnvelope.SetChecksum()
		}
	})

	b.Run("envelope_validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := stream.Validate(envelope)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}