package stream

import (
	"context"
	"fmt"
	"time"
)

// EventBus defines the interface for publishing and subscribing to events
type EventBus interface {
	// Core pub/sub operations
	Publish(ctx context.Context, topic, key string, payload []byte) error
	Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error
	
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() HealthStatus
	
	// Advanced features
	PublishBatch(ctx context.Context, messages []Message) error
	SubscribeWithFilter(ctx context.Context, topic, group string, filter MessageFilter, handler MessageHandler) error
	
	// Administrative operations
	CreateTopic(ctx context.Context, config TopicConfig) error
	DeleteTopic(ctx context.Context, topic string) error
	GetTopicInfo(ctx context.Context, topic string) (*TopicInfo, error)
}

// Message represents a single message in the event bus
type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Key       string            `json:"key"`
	Payload   []byte            `json:"payload"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Partition int32             `json:"partition,omitempty"`
	Offset    int64             `json:"offset,omitempty"`
}

// MessageHandler processes incoming messages
type MessageHandler func(ctx context.Context, message *Message) error

// MessageFilter allows selective message processing
type MessageFilter func(message *Message) bool

// TopicConfig holds topic configuration
type TopicConfig struct {
	Name              string        `json:"name"`
	Partitions        int32         `json:"partitions"`
	ReplicationFactor int16         `json:"replication_factor"`
	RetentionTime     time.Duration `json:"retention_time"`
	CompactionEnabled bool          `json:"compaction_enabled"`
	MaxMessageSize    int64         `json:"max_message_size"`
}

// TopicInfo provides topic metadata
type TopicInfo struct {
	Name       string              `json:"name"`
	Partitions []PartitionInfo     `json:"partitions"`
	Config     map[string]string   `json:"config"`
	CreatedAt  time.Time           `json:"created_at"`
	Stats      TopicStats          `json:"stats"`
}

// PartitionInfo describes a topic partition
type PartitionInfo struct {
	ID       int32 `json:"id"`
	Leader   int32 `json:"leader"`
	Replicas []int32 `json:"replicas"`
	ISR      []int32 `json:"isr"` // In-sync replicas
}

// TopicStats provides topic statistics
type TopicStats struct {
	MessageCount int64  `json:"message_count"`
	ByteSize     int64  `json:"byte_size"`
	ConsumerLag  int64  `json:"consumer_lag"`
	ProducerRate float64 `json:"producer_rate"` // Messages per second
	ConsumerRate float64 `json:"consumer_rate"` // Messages per second
}

// HealthStatus indicates the health of the event bus
type HealthStatus struct {
	Healthy   bool              `json:"healthy"`
	Status    string            `json:"status"`
	Errors    []string          `json:"errors,omitempty"`
	Metrics   HealthMetrics     `json:"metrics"`
	LastCheck time.Time         `json:"last_check"`
}

// HealthMetrics provides operational metrics
type HealthMetrics struct {
	ConnectedBrokers int     `json:"connected_brokers"`
	ActiveTopics     int     `json:"active_topics"`
	ActiveConsumers  int     `json:"active_consumers"`
	ProducerLatencyMS float64 `json:"producer_latency_ms"`
	ConsumerLatencyMS float64 `json:"consumer_latency_ms"`
}

// RetryConfig defines retry behavior for failed operations
type RetryConfig struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	JitterEnabled bool          `json:"jitter_enabled"`
}

// DeadLetterConfig defines dead letter queue behavior
type DeadLetterConfig struct {
	Enabled         bool   `json:"enabled"`
	Topic           string `json:"topic"`
	MaxRetries      int    `json:"max_retries"`
	RetentionTime   time.Duration `json:"retention_time"`
	QuarantineAfter int    `json:"quarantine_after"` // Quarantine after N failures
}

// BusConfig holds general event bus configuration
type BusConfig struct {
	// Connection settings
	Brokers           []string      `json:"brokers"`
	ClientID          string        `json:"client_id"`
	SecurityProtocol  string        `json:"security_protocol"`
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	
	// Producer settings
	ProducerConfig    ProducerConfig `json:"producer"`
	
	// Consumer settings
	ConsumerConfig    ConsumerConfig `json:"consumer"`
	
	// Retry and error handling
	RetryConfig       RetryConfig       `json:"retry"`
	DeadLetterConfig  DeadLetterConfig  `json:"dead_letter"`
	
	// Metrics
	MetricsEnabled    bool              `json:"metrics_enabled"`
	MetricsCallback   MetricsCallback   `json:"-"` // Function pointer
}

// ProducerConfig holds producer-specific settings
type ProducerConfig struct {
	RequiredAcks      int           `json:"required_acks"`      // 0=none, 1=leader, -1=all
	CompressionType   string        `json:"compression_type"`   // gzip, snappy, lz4, zstd
	MaxMessageBytes   int           `json:"max_message_bytes"`
	BatchSize         int           `json:"batch_size"`
	LingerMS          int           `json:"linger_ms"`
	EnableIdempotent  bool          `json:"enable_idempotent"`
	TransactionID     string        `json:"transaction_id,omitempty"`
}

// ConsumerConfig holds consumer-specific settings  
type ConsumerConfig struct {
	GroupID                string        `json:"group_id"`
	AutoOffsetReset        string        `json:"auto_offset_reset"` // earliest, latest, none
	EnableAutoCommit       bool          `json:"enable_auto_commit"`
	AutoCommitIntervalMS   int           `json:"auto_commit_interval_ms"`
	SessionTimeoutMS       int           `json:"session_timeout_ms"`
	HeartbeatIntervalMS    int           `json:"heartbeat_interval_ms"`
	MaxPollRecords         int           `json:"max_poll_records"`
	FetchMinBytes          int           `json:"fetch_min_bytes"`
	FetchMaxWaitMS         int           `json:"fetch_max_wait_ms"`
	EnablePartitionEOF     bool          `json:"enable_partition_eof"`
}

// MetricsCallback is called when metrics are collected
type MetricsCallback func(metric string, value interface{}, tags map[string]string)

// BusType represents different event bus implementations
type BusType string

const (
	BusTypeKafka  BusType = "kafka"
	BusTypePulsar BusType = "pulsar" 
	BusTypeStub   BusType = "stub"   // For testing/development
)

// NewEventBus creates a new event bus of the specified type
func NewEventBus(busType BusType, config BusConfig) (EventBus, error) {
	switch busType {
	case BusTypeKafka:
		return NewKafkaBus(config)
	case BusTypePulsar:
		return NewPulsarBus(config)
	case BusTypeStub:
		return NewStubBus(config)
	default:
		return nil, ErrUnsupportedBusType
	}
}

// Common errors
var (
	ErrUnsupportedBusType = fmt.Errorf("unsupported bus type")
	ErrTopicNotFound      = fmt.Errorf("topic not found")
	ErrInvalidMessage     = fmt.Errorf("invalid message")
	ErrPublishTimeout     = fmt.Errorf("publish timeout")
	ErrConsumerClosed     = fmt.Errorf("consumer closed")
	ErrBusNotStarted      = fmt.Errorf("bus not started")
)