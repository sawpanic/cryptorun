package pulsar

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/stream"
)

// PulsarProducer implements the Producer interface for Apache Pulsar
type PulsarProducer struct {
	config        Config
	topic         string
	batchSize     int
	batchTimeout  time.Duration
	retryAttempts int
	
	// Mock fields for compilation without Pulsar client dependency
	isConnected bool
	messagesSent int64
	
	// Metrics callback
	metricsCallback func(string, int64)
}

// Config holds Pulsar-specific configuration
type Config struct {
	ServiceURL    string        `yaml:"service_url"`
	Topic         string        `yaml:"topic"`
	BatchSize     int           `yaml:"batch_size"`
	BatchTimeout  string        `yaml:"batch_timeout"`
	RetryAttempts int           `yaml:"retry_attempts"`
	Compression   string        `yaml:"compression"`    // "none", "lz4", "zlib", "zstd"
	ProducerName  string        `yaml:"producer_name"`
	MaxPendingMessages int     `yaml:"max_pending_messages"`
	
	// Authentication (optional)
	Auth AuthConfig `yaml:"auth,omitempty"`
	TLS  TLSConfig  `yaml:"tls,omitempty"`
}

// AuthConfig holds Pulsar authentication configuration
type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Method  string `yaml:"method"` // "token", "jwt", "oauth2"
	Token   string `yaml:"token,omitempty"`
	JWT     string `yaml:"jwt,omitempty"`
	OAuth2  OAuth2Config `yaml:"oauth2,omitempty"`
}

// OAuth2Config holds OAuth2 authentication configuration
type OAuth2Config struct {
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Audience     string `yaml:"audience"`
}

// TLSConfig holds TLS configuration for Pulsar
type TLSConfig struct {
	Enabled               bool   `yaml:"enabled"`
	CertFile              string `yaml:"cert_file,omitempty"`
	KeyFile               string `yaml:"key_file,omitempty"`
	TrustCertsFile        string `yaml:"trust_certs_file,omitempty"`
	AllowInsecureConnection bool `yaml:"allow_insecure_connection"`
	HostnameVerification    bool `yaml:"hostname_verification"`
}

// NewProducer creates a new Pulsar producer
func NewProducer(config Config) (*PulsarProducer, error) {
	// Parse batch timeout
	batchTimeout, err := time.ParseDuration(config.BatchTimeout)
	if err != nil {
		batchTimeout = 5 * time.Second // Default
	}

	producer := &PulsarProducer{
		config:        config,
		topic:         config.Topic,
		batchSize:     config.BatchSize,
		batchTimeout:  batchTimeout,
		retryAttempts: config.RetryAttempts,
		isConnected:   false, // Will be set to true when actual Pulsar client connects
	}

	// Initialize connection (mock implementation)
	if err := producer.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Pulsar: %w", err)
	}

	return producer, nil
}

// Send publishes messages to Pulsar
func (p *PulsarProducer) Send(ctx context.Context, topic string, msgs []*stream.Envelope) error {
	if len(msgs) == 0 {
		return fmt.Errorf("no messages to send")
	}

	// Use configured topic if not specified
	if topic == "" {
		topic = p.topic
	}

	// Mock implementation - in production this would use Pulsar client
	for _, msg := range msgs {
		if err := p.sendSingleMessage(ctx, topic, msg); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		
		p.messagesSent++
		if p.metricsCallback != nil {
			p.metricsCallback("pulsar_messages_sent", 1)
		}
	}

	return nil
}

// sendSingleMessage sends a single message (mock implementation)
func (p *PulsarProducer) sendSingleMessage(ctx context.Context, topic string, msg *stream.Envelope) error {
	if !p.isConnected {
		return fmt.Errorf("producer not connected to Pulsar")
	}

	// Validate message
	if err := stream.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Mock Pulsar producer message creation and sending
	// In production, this would:
	// 1. Create a Pulsar ProducerMessage with key, payload, properties
	// 2. Set partition key based on symbol for ordering
	// 3. Add message properties for tracing
	// 4. Send via Pulsar client with callback
	
	// Simulate network latency
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Millisecond): // Simulate send time (slightly higher than Kafka)
		// Message sent successfully
	}

	if p.metricsCallback != nil {
		p.metricsCallback("pulsar_send_latency_ms", 2)
	}

	return nil
}

// Close gracefully shuts down the Pulsar producer
func (p *PulsarProducer) Close() error {
	if !p.isConnected {
		return nil
	}

	// Mock cleanup
	p.isConnected = false
	
	if p.metricsCallback != nil {
		p.metricsCallback("pulsar_producer_closed", 1)
	}

	return nil
}

// SetMetricsCallback sets the metrics callback function
func (p *PulsarProducer) SetMetricsCallback(callback func(string, int64)) {
	p.metricsCallback = callback
}

// GetStats returns producer statistics
func (p *PulsarProducer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"connected":             p.isConnected,
		"messages_sent":         p.messagesSent,
		"topic":                 p.topic,
		"batch_size":            p.batchSize,
		"retry_attempts":        p.retryAttempts,
		"compression":           p.config.Compression,
		"producer_name":         p.config.ProducerName,
		"max_pending_messages":  p.config.MaxPendingMessages,
	}
}

// IsHealthy returns true if the producer is connected and healthy
func (p *PulsarProducer) IsHealthy(ctx context.Context) (bool, error) {
	if !p.isConnected {
		return false, fmt.Errorf("producer not connected")
	}

	// Mock health check - would ping Pulsar cluster in production
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(15 * time.Millisecond): // Simulate health check (slightly higher than Kafka)
		return true, nil
	}
}

// connect establishes connection to Pulsar cluster (mock implementation)
func (p *PulsarProducer) connect() error {
	if p.config.ServiceURL == "" {
		return fmt.Errorf("no service URL configured")
	}

	// Mock connection logic
	// In production, this would:
	// 1. Create Pulsar client with service URL
	// 2. Set up authentication if enabled (token, JWT, OAuth2)
	// 3. Configure TLS if enabled
	// 4. Create producer with topic and options (compression, batching, etc.)
	// 5. Test connection with a metadata request

	p.isConnected = true
	
	if p.metricsCallback != nil {
		p.metricsCallback("pulsar_connections_established", 1)
	}

	return nil
}

// FlushAndWait ensures all pending messages are sent
func (p *PulsarProducer) FlushAndWait(ctx context.Context) error {
	if !p.isConnected {
		return fmt.Errorf("producer not connected")
	}

	// Mock flush operation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(150 * time.Millisecond): // Simulate flush time (slightly higher than Kafka)
		if p.metricsCallback != nil {
			p.metricsCallback("pulsar_flush_completed", 1)
		}
		return nil
	}
}

// GetTopicMetadata returns metadata about the topic
func (p *PulsarProducer) GetTopicMetadata(ctx context.Context, topic string) (*TopicMetadata, error) {
	if !p.isConnected {
		return nil, fmt.Errorf("producer not connected")
	}

	// Mock metadata - in production would query Pulsar cluster
	return &TopicMetadata{
		Topic:      topic,
		Partitions: 1, // Pulsar topics are partitioned differently than Kafka
		Type:       "persistent", // Pulsar topic type
		Schema:     "none",
		Brokers:    []string{"broker-1", "broker-2"}, // Pulsar brokers
	}, nil
}

// TopicMetadata contains information about a Pulsar topic
type TopicMetadata struct {
	Topic      string   `json:"topic"`
	Partitions int      `json:"partitions"`
	Type       string   `json:"type"`     // "persistent" or "non-persistent"
	Schema     string   `json:"schema"`   // Schema type
	Brokers    []string `json:"brokers"` // Pulsar brokers
}

// Default Pulsar configuration
func DefaultConfig() Config {
	return Config{
		ServiceURL:         "pulsar://localhost:6650",
		Topic:              "cryptorun-default",
		BatchSize:          100,
		BatchTimeout:       "5s",
		RetryAttempts:      3,
		Compression:        "lz4",
		ProducerName:       "cryptorun-producer",
		MaxPendingMessages: 1000,
		
		Auth: AuthConfig{
			Enabled: false,
		},
		TLS: TLSConfig{
			Enabled: false,
		},
	}
}

// ValidateConfig validates Pulsar producer configuration
func ValidateConfig(config Config) error {
	if config.ServiceURL == "" {
		return fmt.Errorf("service URL cannot be empty")
	}

	if config.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	if config.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive, got %d", config.BatchSize)
	}

	if config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative, got %d", config.RetryAttempts)
	}

	if config.MaxPendingMessages <= 0 {
		return fmt.Errorf("max pending messages must be positive, got %d", config.MaxPendingMessages)
	}

	// Validate compression type
	validCompressions := map[string]bool{
		"none": true, "lz4": true, "zlib": true, "zstd": true,
	}
	if !validCompressions[config.Compression] {
		return fmt.Errorf("invalid compression type: %s", config.Compression)
	}

	// Validate auth method if enabled
	if config.Auth.Enabled {
		validMethods := map[string]bool{
			"token": true, "jwt": true, "oauth2": true,
		}
		if !validMethods[config.Auth.Method] {
			return fmt.Errorf("invalid auth method: %s", config.Auth.Method)
		}
	}

	return nil
}