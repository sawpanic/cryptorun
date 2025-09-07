package streaming

import (
	"context"
	"fmt"
	"strings"

	"github.com/sawpanic/cryptorun/internal/stream"
	"github.com/sawpanic/cryptorun/internal/stream/kafka"
	"github.com/sawpanic/cryptorun/internal/stream/pulsar"
)

// ProducerType represents the type of message producer
type ProducerType string

const (
	ProducerTypeKafka  ProducerType = "kafka"
	ProducerTypePulsar ProducerType = "pulsar"
)

// ProducerConfig holds configuration for creating producers
type ProducerConfig struct {
	Type   ProducerType `yaml:"type"`
	Kafka  kafka.Config `yaml:"kafka,omitempty"`
	Pulsar pulsar.Config `yaml:"pulsar,omitempty"`
}

// Producer interface that both Kafka and Pulsar producers implement
type Producer interface {
	Send(ctx context.Context, topic string, msgs []*stream.Envelope) error
	Close() error
	SetMetricsCallback(callback func(string, int64))
	GetStats() map[string]interface{}
	IsHealthy(ctx context.Context) (bool, error)
	FlushAndWait(ctx context.Context) error
}

// NewProducer creates a new producer based on configuration
func NewProducer(config ProducerConfig) (Producer, error) {
	switch strings.ToLower(string(config.Type)) {
	case string(ProducerTypeKafka):
		return kafka.NewProducer(config.Kafka)
	case string(ProducerTypePulsar):
		return pulsar.NewProducer(config.Pulsar)
	default:
		return nil, fmt.Errorf("unsupported producer type: %s", config.Type)
	}
}

// ValidateProducerConfig validates the producer configuration
func ValidateProducerConfig(config ProducerConfig) error {
	switch config.Type {
	case ProducerTypeKafka:
		return kafka.ValidateConfig(config.Kafka)
	case ProducerTypePulsar:
		return pulsar.ValidateConfig(config.Pulsar)
	default:
		return fmt.Errorf("unsupported producer type: %s", config.Type)
	}
}

// DefaultProducerConfig returns default configuration for the specified producer type
func DefaultProducerConfig(producerType ProducerType) ProducerConfig {
	config := ProducerConfig{Type: producerType}
	
	switch producerType {
	case ProducerTypeKafka:
		config.Kafka = kafka.DefaultConfig()
	case ProducerTypePulsar:
		config.Pulsar = pulsar.DefaultConfig()
	}
	
	return config
}

// GetSupportedProducerTypes returns a list of supported producer types
func GetSupportedProducerTypes() []ProducerType {
	return []ProducerType{
		ProducerTypeKafka,
		ProducerTypePulsar,
	}
}

// ProducerStats holds statistics from a producer
type ProducerStats struct {
	Type       ProducerType           `json:"type"`
	Connected  bool                   `json:"connected"`
	MessagesSent int64                `json:"messages_sent"`
	Details    map[string]interface{} `json:"details"`
}

// GetProducerStats returns formatted statistics from a producer
func GetProducerStats(producer Producer, producerType ProducerType) ProducerStats {
	stats := producer.GetStats()
	connected, _ := stats["connected"].(bool)
	messagesSent, _ := stats["messages_sent"].(int64)
	
	return ProducerStats{
		Type:         producerType,
		Connected:    connected,
		MessagesSent: messagesSent,
		Details:      stats,
	}
}