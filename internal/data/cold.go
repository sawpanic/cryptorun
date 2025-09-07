package data

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/quality"
)

// extractFloatValue safely extracts a float64 value from interface{} data
func extractFloatValue(data interface{}, key string, defaultValue float64) float64 {
	if dataMap, ok := data.(map[string]interface{}); ok {
		if val, exists := dataMap[key]; exists {
			if floatVal, ok := val.(float64); ok {
				return floatVal
			}
		}
	}
	return defaultValue
}

// CompressionType defines compression algorithms
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionLZ4  CompressionType = "lz4"
)

// detectCompressionFromPath determines compression type from file extension
func detectCompressionFromPath(filePath string, config CompressionConfig) CompressionType {
	if !config.AutoDetect {
		return CompressionType(config.Algorithm)
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Check gzip extensions
	for _, gzipExt := range config.Extensions["gzip"] {
		if ext == gzipExt {
			return CompressionGzip
		}
	}
	
	// Check LZ4 extensions
	for _, lz4Ext := range config.Extensions["lz4"] {
		if ext == lz4Ext {
			return CompressionLZ4
		}
	}
	
	return CompressionNone
}

// createCompressedWriter creates a writer with compression support
func createCompressedWriter(file *os.File, compressionType CompressionType, level int) (io.WriteCloser, error) {
	switch compressionType {
	case CompressionGzip:
		writer, err := gzip.NewWriterLevel(file, level)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		return writer, nil
	case CompressionLZ4:
		// For now, use a pass-through wrapper since LZ4 needs external library
		// In production, this would use github.com/pierrec/lz4/v4
		return &passthroughCloser{file}, nil
	default:
		return &passthroughCloser{file}, nil
	}
}

// createCompressedReader creates a reader with decompression support
func createCompressedReader(file *os.File, compressionType CompressionType) (io.ReadCloser, error) {
	switch compressionType {
	case CompressionGzip:
		reader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return reader, nil
	case CompressionLZ4:
		// For now, use a pass-through wrapper since LZ4 needs external library
		// In production, this would use github.com/pierrec/lz4/v4
		return file, nil
	default:
		return file, nil
	}
}

// passthroughCloser wraps a file to implement WriteCloser without double-closing
type passthroughCloser struct {
	file *os.File
}

func (p *passthroughCloser) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

func (p *passthroughCloser) Close() error {
	// Don't close the underlying file, let the caller handle it
	return nil
}

// StreamingMessage represents a message in the streaming system
type StreamingMessage struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Key       string            `json:"key"`
	Payload   []byte            `json:"payload"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Partition int32             `json:"partition,omitempty"`
	Offset    int64             `json:"offset,omitempty"`
}

// StreamingProducer interface for publishing messages to streaming systems
type StreamingProducer interface {
	Publish(ctx context.Context, message *StreamingMessage) error
	PublishBatch(ctx context.Context, messages []*StreamingMessage) error
	Close() error
}

// StreamingConsumer interface for consuming messages from streaming systems
type StreamingConsumer interface {
	Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error
	Close() error
}

// MessageHandler processes incoming streaming messages
type MessageHandler func(ctx context.Context, message *StreamingMessage) error

// RegionManager manages multi-region replication and failover
type RegionManager interface {
	GetPrimaryRegion() string
	GetSecondaryRegions() []string
	IsHealthy(ctx context.Context, region string) (bool, error)
	GetReplicationLag(ctx context.Context, topic, region string) (time.Duration, error)
	TriggerFailover(ctx context.Context, fromRegion, toRegion string) error
	ResolveConflict(message1, message2 *StreamingMessage) (*StreamingMessage, error)
}

// ReplicationManager handles cross-region message replication
type ReplicationManager interface {
	ReplicateMessage(ctx context.Context, message *StreamingMessage, targetRegions []string) error
	StartReplication(ctx context.Context) error
	StopReplication(ctx context.Context) error
	GetReplicationStatus(ctx context.Context) (ReplicationStatus, error)
}

// ReplicationStatus provides current replication state
type ReplicationStatus struct {
	PrimaryRegion    string                 `json:"primary_region"`
	SecondaryRegions []string               `json:"secondary_regions"`
	RegionHealth     map[string]bool        `json:"region_health"`
	ReplicationLags  map[string]time.Duration `json:"replication_lags"`
	LastFailover     *time.Time             `json:"last_failover,omitempty"`
	ActivePolicy     string                 `json:"active_policy"`  // "active_active" or "active_passive"
}

// StubProducer is a no-op implementation for testing
type StubProducer struct{}

func (p *StubProducer) Publish(ctx context.Context, message *StreamingMessage) error {
	// No-op for stub
	return nil
}

func (p *StubProducer) PublishBatch(ctx context.Context, messages []*StreamingMessage) error {
	// No-op for stub
	return nil
}

func (p *StubProducer) Close() error {
	// No-op for stub
	return nil
}

// StubConsumer is a no-op implementation for testing
type StubConsumer struct{}

func (c *StubConsumer) Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error {
	// No-op for stub
	return nil
}

func (c *StubConsumer) Close() error {
	// No-op for stub
	return nil
}

// StubRegionManager is a stub implementation for testing
type StubRegionManager struct {
	primary     string
	secondaries []string
}

func NewStubRegionManager(primary string, secondaries []string) *StubRegionManager {
	return &StubRegionManager{
		primary:     primary,
		secondaries: secondaries,
	}
}

func (r *StubRegionManager) GetPrimaryRegion() string {
	return r.primary
}

func (r *StubRegionManager) GetSecondaryRegions() []string {
	return r.secondaries
}

func (r *StubRegionManager) IsHealthy(ctx context.Context, region string) (bool, error) {
	// Stub always returns healthy
	return true, nil
}

func (r *StubRegionManager) GetReplicationLag(ctx context.Context, topic, region string) (time.Duration, error) {
	// Stub returns minimal lag
	return 50 * time.Millisecond, nil
}

func (r *StubRegionManager) TriggerFailover(ctx context.Context, fromRegion, toRegion string) error {
	// Stub implementation - no-op but successful
	return nil
}

func (r *StubRegionManager) ResolveConflict(message1, message2 *StreamingMessage) (*StreamingMessage, error) {
	// Timestamp wins strategy
	if message1.Timestamp.After(message2.Timestamp) {
		return message1, nil
	}
	return message2, nil
}

// StubReplicationManager is a stub implementation for testing
type StubReplicationManager struct {
	status ReplicationStatus
}

func NewStubReplicationManager(primary string, secondaries []string) *StubReplicationManager {
	regionHealth := make(map[string]bool)
	regionHealth[primary] = true
	for _, secondary := range secondaries {
		regionHealth[secondary] = true
	}

	replicationLags := make(map[string]time.Duration)
	for _, region := range secondaries {
		replicationLags[region] = 50 * time.Millisecond
	}

	return &StubReplicationManager{
		status: ReplicationStatus{
			PrimaryRegion:    primary,
			SecondaryRegions: secondaries,
			RegionHealth:     regionHealth,
			ReplicationLags:  replicationLags,
			ActivePolicy:     "active_active",
		},
	}
}

func (r *StubReplicationManager) ReplicateMessage(ctx context.Context, message *StreamingMessage, targetRegions []string) error {
	// Stub implementation - always succeeds
	return nil
}

func (r *StubReplicationManager) StartReplication(ctx context.Context) error {
	// Stub implementation - no-op
	return nil
}

func (r *StubReplicationManager) StopReplication(ctx context.Context) error {
	// Stub implementation - no-op
	return nil
}

func (r *StubReplicationManager) GetReplicationStatus(ctx context.Context) (ReplicationStatus, error) {
	return r.status, nil
}

// ColdTierStreamer handles streaming operations for cold tier data
type ColdTierStreamer struct {
	config   StreamingConfig
	producer StreamingProducer
	consumer StreamingConsumer
	
	// Batching
	batchBuffer   []*StreamingMessage
	batchMutex    sync.Mutex
	batchTimer    *time.Timer
	bufferTimeout time.Duration
	
	// DLQ handling
	dlqProducer StreamingProducer
	
	// Multi-region replication
	regionManager      RegionManager
	replicationManager ReplicationManager
	
	// Metrics callback
	metricsCallback func(string, int64)
}

// NewColdTierStreamer creates a new streaming handler for cold tier
func NewColdTierStreamer(config StreamingConfig) (*ColdTierStreamer, error) {
	bufferTimeout, err := time.ParseDuration(config.BufferTimeout)
	if err != nil {
		bufferTimeout = 5 * time.Second // Default
	}

	streamer := &ColdTierStreamer{
		config:        config,
		batchBuffer:   make([]*StreamingMessage, 0, config.BatchSize),
		bufferTimeout: bufferTimeout,
	}

	// Initialize producer/consumer based on backend
	switch config.Backend {
	case "kafka":
		streamer.producer = &StubProducer{} // TODO: implement NewKafkaProducer()
		streamer.consumer = &StubConsumer{} // TODO: implement NewKafkaConsumer()
	case "pulsar":
		streamer.producer = &StubProducer{} // TODO: implement NewPulsarProducer()
		streamer.consumer = &StubConsumer{} // TODO: implement NewPulsarConsumer()
	case "stub":
		streamer.producer = &StubProducer{}
		streamer.consumer = &StubConsumer{}
	default:
		return nil, fmt.Errorf("unsupported streaming backend: %s", config.Backend)
	}

	// Initialize DLQ producer if enabled
	if config.EnableDLQ {
		streamer.dlqProducer = streamer.producer // Use same backend for DLQ
	}

	// Initialize replication components if enabled
	if config.Replication.Enable {
		streamer.regionManager = NewStubRegionManager(
			config.Replication.PrimaryRegion,
			config.Replication.SecondaryRegions,
		)
		streamer.replicationManager = NewStubReplicationManager(
			config.Replication.PrimaryRegion,
			config.Replication.SecondaryRegions,
		)
	}

	return streamer, nil
}

// StreamEnvelopes publishes envelopes to streaming system with batching
func (s *ColdTierStreamer) StreamEnvelopes(ctx context.Context, envelopes []*Envelope, topic string) error {
	if !s.config.Enable {
		return nil // Streaming disabled
	}

	for _, envelope := range envelopes {
		message, err := s.envelopeToStreamingMessage(envelope, topic)
		if err != nil {
			if s.metricsCallback != nil {
				s.metricsCallback("cold_streaming_convert_error", 1)
			}
			continue
		}

		if err := s.addToBatch(ctx, message); err != nil {
			return fmt.Errorf("failed to add message to batch: %w", err)
		}
	}

	// Flush any remaining messages in batch
	return s.flushBatch(ctx)
}

// ReplayHistoricalData streams historical data from cold storage
func (s *ColdTierStreamer) ReplayHistoricalData(ctx context.Context, filePath string, venue string, symbol string, reader FileReader) error {
	if !s.config.Enable {
		return fmt.Errorf("streaming is disabled in configuration")
	}

	// Load historical data
	envelopes, err := reader.LoadFile(filePath, venue, symbol)
	if err != nil {
		return fmt.Errorf("failed to load historical data: %w", err)
	}

	if len(envelopes) == 0 {
		return nil // No data to replay
	}

	// Stream with historical replay topic
	replayTopic := s.config.Topics["historical_replay"]
	if replayTopic == "" {
		replayTopic = "cryptorun-historical-replay"
	}

	if s.metricsCallback != nil {
		s.metricsCallback("cold_historical_replay_start", int64(len(envelopes)))
	}

	return s.StreamEnvelopes(ctx, envelopes, replayTopic)
}

// addToBatch adds message to batch buffer with automatic flushing
func (s *ColdTierStreamer) addToBatch(ctx context.Context, message *StreamingMessage) error {
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	s.batchBuffer = append(s.batchBuffer, message)

	// Start timer on first message in batch
	if len(s.batchBuffer) == 1 {
		s.batchTimer = time.AfterFunc(s.bufferTimeout, func() {
			s.flushBatch(ctx) // Async flush on timeout
		})
	}

	// Flush if batch is full
	if len(s.batchBuffer) >= s.config.BatchSize {
		if s.batchTimer != nil {
			s.batchTimer.Stop()
		}
		return s.flushBatchUnsafe(ctx)
	}

	return nil
}

// flushBatch flushes the current batch with lock
func (s *ColdTierStreamer) flushBatch(ctx context.Context) error {
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()
	return s.flushBatchUnsafe(ctx)
}

// flushBatchUnsafe flushes batch without acquiring lock (caller must hold lock)
func (s *ColdTierStreamer) flushBatchUnsafe(ctx context.Context) error {
	if len(s.batchBuffer) == 0 {
		return nil
	}

	batch := make([]*StreamingMessage, len(s.batchBuffer))
	copy(batch, s.batchBuffer)
	s.batchBuffer = s.batchBuffer[:0] // Clear buffer

	// Attempt to publish batch with retries
	var err error
	maxAttempts := s.config.RetryAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1 // At least one attempt
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = s.producer.PublishBatch(ctx, batch)
		if err == nil {
			// Handle replication after successful publish
			for _, message := range batch {
				if replicationErr := s.replicateMessage(ctx, message); replicationErr != nil {
					// Log replication error but don't fail the batch
					if s.metricsCallback != nil {
						s.metricsCallback("cold_streaming_replication_warning", 1)
					}
				}
			}

			if s.metricsCallback != nil {
				s.metricsCallback("cold_streaming_batch_success", int64(len(batch)))
			}
			return nil
		}

		// Log retry attempt
		if s.metricsCallback != nil {
			s.metricsCallback("cold_streaming_batch_retry", 1)
		}

		// Exponential backoff
		backoff := time.Duration(attempt+1) * 100 * time.Millisecond
		time.Sleep(backoff)
	}

	// Send to DLQ if enabled
	if s.config.EnableDLQ && s.dlqProducer != nil {
		dlqTopic := s.config.Topics["dlq"]
		if dlqTopic == "" {
			dlqTopic = "cryptorun-cold-dlq"
		}

		for _, msg := range batch {
			msg.Topic = dlqTopic
			msg.Headers["dlq_reason"] = "batch_publish_failed"
			msg.Headers["original_topic"] = msg.Topic
		}

		dlqErr := s.dlqProducer.PublishBatch(ctx, batch)
		if dlqErr != nil {
			if s.metricsCallback != nil {
				s.metricsCallback("cold_streaming_dlq_error", int64(len(batch)))
			}
		} else {
			if s.metricsCallback != nil {
				s.metricsCallback("cold_streaming_dlq_success", int64(len(batch)))
			}
		}
	}

	if s.metricsCallback != nil {
		s.metricsCallback("cold_streaming_batch_error", int64(len(batch)))
	}

	return fmt.Errorf("failed to publish batch after %d attempts: %w", maxAttempts, err)
}

// envelopeToStreamingMessage converts an Envelope to a StreamingMessage
func (s *ColdTierStreamer) envelopeToStreamingMessage(envelope *Envelope, topic string) (*StreamingMessage, error) {
	// Create payload from envelope data
	payload := map[string]interface{}{
		"timestamp":    envelope.Timestamp,
		"venue":        envelope.Venue,
		"symbol":       envelope.Symbol,
		"source_tier":  string(envelope.SourceTier),
		"price_data":   envelope.PriceData,
		"volume_data":  envelope.VolumeData,
		"order_book":   envelope.OrderBook,
		"generic_data": envelope.GenericData,
		"provenance":   envelope.Provenance,
		"checksum":     envelope.Checksum,
		"freshness_ms": envelope.FreshnessMS,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope payload: %w", err)
	}

	message := &StreamingMessage{
		ID:        fmt.Sprintf("%s-%s-%d", envelope.Venue, envelope.Symbol, envelope.Timestamp.UnixNano()),
		Topic:     topic,
		Key:       fmt.Sprintf("%s-%s", envelope.Venue, envelope.Symbol),
		Payload:   payloadBytes,
		Timestamp: envelope.Timestamp,
		Headers: map[string]string{
			"source":      "cold_tier",
			"venue":       envelope.Venue,
			"symbol":      envelope.Symbol,
			"tier":        string(envelope.SourceTier),
			"confidence":  fmt.Sprintf("%.2f", envelope.Provenance.ConfidenceScore),
			"cache_hit":   fmt.Sprintf("%t", envelope.Provenance.CacheHit),
		},
	}

	// Add fallback chain info if available
	if len(envelope.Provenance.FallbackChain) > 0 {
		message.Headers["fallback_chain"] = strings.Join(envelope.Provenance.FallbackChain, ",")
	}

	return message, nil
}

// SetMetricsCallback sets callback for streaming metrics
func (s *ColdTierStreamer) SetMetricsCallback(callback func(string, int64)) {
	s.metricsCallback = callback
}

// Close closes the streaming connections and flushes any pending messages
func (s *ColdTierStreamer) Close(ctx context.Context) error {
	// Flush any remaining messages
	if err := s.flushBatch(ctx); err != nil {
		// Log error but don't fail close
		if s.metricsCallback != nil {
			s.metricsCallback("cold_streaming_close_flush_error", 1)
		}
	}

	// Close producer and consumer
	var errs []error
	if s.producer != nil {
		if err := s.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
		}
	}
	if s.consumer != nil {
		if err := s.consumer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close consumer: %w", err))
		}
	}
	if s.dlqProducer != nil && s.dlqProducer != s.producer {
		if err := s.dlqProducer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close DLQ producer: %w", err))
		}
	}

	// Stop replication if enabled
	if s.replicationManager != nil {
		if err := s.replicationManager.StopReplication(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop replication: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("streaming close errors: %v", errs)
	}

	return nil
}

// Multi-region replication methods

// EnableReplication starts replication services
func (s *ColdTierStreamer) EnableReplication(ctx context.Context) error {
	if !s.config.Replication.Enable {
		return fmt.Errorf("replication is disabled in configuration")
	}

	if s.replicationManager == nil {
		return fmt.Errorf("replication manager not initialized")
	}

	return s.replicationManager.StartReplication(ctx)
}

// GetReplicationStatus returns current replication status
func (s *ColdTierStreamer) GetReplicationStatus(ctx context.Context) (ReplicationStatus, error) {
	if s.replicationManager == nil {
		return ReplicationStatus{}, fmt.Errorf("replication not enabled")
	}

	return s.replicationManager.GetReplicationStatus(ctx)
}

// TriggerFailover initiates failover to specified region
func (s *ColdTierStreamer) TriggerFailover(ctx context.Context, fromRegion, toRegion string) error {
	if s.regionManager == nil {
		return fmt.Errorf("region manager not initialized")
	}

	return s.regionManager.TriggerFailover(ctx, fromRegion, toRegion)
}

// GetRegionHealth checks health status of specified region
func (s *ColdTierStreamer) GetRegionHealth(ctx context.Context, region string) (bool, error) {
	if s.regionManager == nil {
		return false, fmt.Errorf("region manager not initialized")
	}

	return s.regionManager.IsHealthy(ctx, region)
}

// GetReplicationLag returns replication lag for specified topic and region
func (s *ColdTierStreamer) GetReplicationLag(ctx context.Context, topic, region string) (time.Duration, error) {
	if s.regionManager == nil {
		return 0, fmt.Errorf("region manager not initialized")
	}

	return s.regionManager.GetReplicationLag(ctx, topic, region)
}

// shouldReplicateToRegion checks if message should be replicated to specified region
func (s *ColdTierStreamer) shouldReplicateToRegion(topic, region string) bool {
	if !s.config.Replication.Enable {
		return false
	}

	// Check active-active topics
	for _, t := range s.config.Replication.Policies.ActiveActive.Topics {
		if t == topic {
			return true
		}
	}

	// Check active-passive topics (only replicate to primary)
	if s.regionManager != nil && region == s.regionManager.GetPrimaryRegion() {
		for _, t := range s.config.Replication.Policies.ActivePassive.Topics {
			if t == topic {
				return true
			}
		}
	}

	return false
}

// replicateMessage handles cross-region message replication
func (s *ColdTierStreamer) replicateMessage(ctx context.Context, message *StreamingMessage) error {
	if !s.config.Replication.Enable || s.replicationManager == nil {
		return nil // Replication disabled
	}

	var targetRegions []string
	for _, region := range s.regionManager.GetSecondaryRegions() {
		if s.shouldReplicateToRegion(message.Topic, region) {
			targetRegions = append(targetRegions, region)
		}
	}

	if len(targetRegions) > 0 {
		err := s.replicationManager.ReplicateMessage(ctx, message, targetRegions)
		if err != nil {
			if s.metricsCallback != nil {
				s.metricsCallback("cold_streaming_replication_error", 1)
			}
			return fmt.Errorf("failed to replicate message: %w", err)
		}

		if s.metricsCallback != nil {
			s.metricsCallback("cold_streaming_replication_success", int64(len(targetRegions)))
		}
	}

	return nil
}

// FileReader interface for different file format readers
type FileReader interface {
	LoadFile(filePath, venue, symbol string) ([]*Envelope, error)
	LoadFileWithTimeFilter(filePath, venue, symbol string, from, until time.Time) ([]*Envelope, error)
	ValidateFile(filePath string) error
	WriteFile(filePath string, data []*Envelope) error
}

// CompressedFileReader extends FileReader with compression support
type CompressedFileReader interface {
	FileReader
	SetCompressionConfig(config CompressionConfig)
}

// CSVReader handles CSV file reading with OHLCV schema
type CSVReader struct {
	compressionConfig CompressionConfig
}

// ParquetReader handles Parquet file reading with OHLCV schema  
type ParquetReader struct {
	compressionConfig CompressionConfig
}

// CompressionConfig holds compression-specific settings
type CompressionConfig struct {
	Enable     bool              `yaml:"enable"`
	Algorithm  string            `yaml:"algorithm"`     // "gzip", "lz4", "none"
	Level      int               `yaml:"level"`         // compression level
	AutoDetect bool              `yaml:"auto_detect"`   // detect from file extension
	Extensions map[string][]string `yaml:"extensions"`   // file extensions per algorithm
}

// StreamingConfig holds streaming-specific settings
type StreamingConfig struct {
	Enable        bool              `yaml:"enable"`
	Backend       string            `yaml:"backend"`        // "kafka", "pulsar", "stub"
	BatchSize     int               `yaml:"batch_size"`     // messages per batch
	BufferTimeout string            `yaml:"buffer_timeout"` // max time to wait for batch
	RetryAttempts int               `yaml:"retry_attempts"` // retry attempts for failed messages
	EnableDLQ     bool              `yaml:"enable_dlq"`     // enable dead letter queue
	Topics        map[string]string `yaml:"topics"`         // topic mappings
	Replication   ReplicationConfig `yaml:"replication"`    // multi-region replication
}

// ReplicationConfig defines multi-region replication settings
type ReplicationConfig struct {
	Enable              bool                        `yaml:"enable"`
	PrimaryRegion       string                      `yaml:"primary_region"`
	SecondaryRegions    []string                    `yaml:"secondary_regions"`
	ConflictResolution  string                      `yaml:"conflict_resolution"`  // "timestamp_wins", "region_priority", "merge"
	RegionPriority      []string                    `yaml:"region_priority"`
	Policies            ReplicationPolicies         `yaml:"policies"`
	HealthCheck         ReplicationHealthConfig     `yaml:"health_check"`
	Failover            ReplicationFailoverConfig   `yaml:"failover"`
}

// ReplicationPolicies defines active-active and active-passive policies
type ReplicationPolicies struct {
	ActiveActive  ReplicationPolicy `yaml:"active_active"`
	ActivePassive ReplicationPolicy `yaml:"active_passive"`
}

// ReplicationPolicy defines per-topic replication settings
type ReplicationPolicy struct {
	Topics           []string `yaml:"topics"`
	LagThresholdMs   int      `yaml:"lag_threshold_ms"`
	CutoverPolicy    string   `yaml:"cutover_policy"`  // "automatic", "manual"
}

// ReplicationHealthConfig defines health check settings
type ReplicationHealthConfig struct {
	Interval           string `yaml:"interval"`
	Timeout            string `yaml:"timeout"`
	FailureThreshold   int    `yaml:"failure_threshold"`
	RecoveryThreshold  int    `yaml:"recovery_threshold"`
}

// ReplicationFailoverConfig defines failover behavior
type ReplicationFailoverConfig struct {
	UnhealthyTimeout     string  `yaml:"unhealthy_timeout"`
	ErrorRateThreshold   float64 `yaml:"error_rate_threshold"`
	RecoveryTimeout      string  `yaml:"recovery_timeout"`
	OperatorApproval     bool    `yaml:"operator_approval"`
}

// ColdDataConfig holds configuration loaded from data_sources.yaml
type ColdDataConfig struct {
	EnableParquet bool              `yaml:"enable_parquet"`
	EnableCSV     bool              `yaml:"enable_csv"`
	DefaultFormat string            `yaml:"default_format"`
	BasePath      string            `yaml:"base_path"`
	CacheExpiry   string            `yaml:"cache_expiry"`
	EnableCache   bool              `yaml:"enable_cache"`
	Compression   CompressionConfig `yaml:"compression"`
	Streaming     StreamingConfig   `yaml:"streaming"`
	Quality       quality.QualityConfig `yaml:"quality"`
}

// ColdData implements historical file data tier
type ColdData struct {
	config        ColdDataConfig
	csvReader     FileReader
	parquetReader FileReader

	// Cache for loaded data
	cache       map[string][]*Envelope
	cacheExpiry time.Duration

	// Quality validation
	validator *quality.DataValidator
}

// SetCompressionConfig sets compression configuration for CSVReader
func (r *CSVReader) SetCompressionConfig(config CompressionConfig) {
	r.compressionConfig = config
}

// SetCompressionConfig sets compression configuration for ParquetReader
func (r *ParquetReader) SetCompressionConfig(config CompressionConfig) {
	r.compressionConfig = config
}

// NewColdData creates a new cold data tier
func NewColdData(config ColdDataConfig) (*ColdData, error) {
	expiry, err := time.ParseDuration(config.CacheExpiry)
	if err != nil {
		expiry = time.Hour // Default 1 hour
	}

	// Initialize readers with compression config
	csvReader := &CSVReader{}
	csvReader.SetCompressionConfig(config.Compression)
	
	parquetReader := &ParquetReader{}
	parquetReader.SetCompressionConfig(config.Compression)

	// Initialize validator if quality config is provided
	var validator *quality.DataValidator
	if config.Quality.Scoring.Enable || config.Quality.Validation.Enable || config.Quality.AnomalyDetection.Enable {
		validator, err = quality.NewDataValidator(config.Quality)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize data validator: %w", err)
		}
	}

	return &ColdData{
		config:        config,
		csvReader:     csvReader,
		parquetReader: parquetReader,
		cache:         make(map[string][]*Envelope),
		cacheExpiry:   expiry,
		validator:     validator,
	}, nil
}

// GetFormatReader returns appropriate reader based on config and file extension
func (c *ColdData) GetFormatReader(filePath string) FileReader {
	if strings.HasSuffix(filePath, ".parquet") && c.config.EnableParquet {
		return c.parquetReader
	}
	if strings.HasSuffix(filePath, ".csv") && c.config.EnableCSV {
		return c.csvReader
	}
	
	// Default based on config
	if c.config.DefaultFormat == "parquet" && c.config.EnableParquet {
		return c.parquetReader
	}
	return c.csvReader
}

// WriteData writes envelope data to cold storage using configured format
func (c *ColdData) WriteData(venue, symbol string, data []*Envelope) error {
	if len(data) == 0 {
		return fmt.Errorf("no data to write")
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02")
	extension := ".csv"
	if c.config.DefaultFormat == "parquet" {
		extension = ".parquet"
	}
	
	filename := fmt.Sprintf("%s_%s_%s%s", venue, symbol, timestamp, extension)
	venuePath := filepath.Join(c.config.BasePath, venue)
	
	// Ensure venue directory exists
	if err := os.MkdirAll(venuePath, 0755); err != nil {
		return fmt.Errorf("failed to create venue directory: %w", err)
	}
	
	filePath := filepath.Join(venuePath, filename)
	reader := c.GetFormatReader(filePath)
	
	return reader.WriteFile(filePath, data)
}

// GetOrderBook retrieves historical order book data
func (c *ColdData) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	// For cold tier, return most recent available data
	data, err := c.GetHistoricalSlice(ctx, venue, symbol,
		time.Now().Add(-24*time.Hour), // Look back 24 hours
		time.Now())
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no historical data found for %s %s", venue, symbol)
	}

	// Return most recent entry
	latest := data[len(data)-1]
	latest.SourceTier = TierCold
	latest.CalculateFreshness()

	return latest, nil
}

// GetPriceData retrieves historical price data
func (c *ColdData) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	return c.GetOrderBook(ctx, venue, symbol) // Same for cold tier
}

// IsAvailable checks if cold data files exist for venue
func (c *ColdData) IsAvailable(ctx context.Context, venue string) bool {
	venuePath := filepath.Join(c.config.BasePath, venue)
	info, err := os.Stat(venuePath)
	return err == nil && info.IsDir()
}

// GetHistoricalSlice retrieves data within time bounds
func (c *ColdData) GetHistoricalSlice(ctx context.Context, venue, symbol string, start, end time.Time) ([]*Envelope, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%d:%d", venue, symbol, start.Unix(), end.Unix())
	if cached, exists := c.cache[cacheKey]; exists {
		return cached, nil
	}

	// Find relevant files in date range
	files, err := c.findFilesInRange(venue, symbol, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to find files for %s %s: %w", venue, symbol, err)
	}

	var allData []*Envelope

	// Load data from each file
	for _, file := range files {
		reader := c.GetFormatReader(file)
		
		// Use time filter if available for better performance
		fileData, loadErr := reader.LoadFileWithTimeFilter(file, venue, symbol, start, end)
		if loadErr != nil {
			// Fallback to regular load if time filter not supported
			fileData, loadErr = reader.LoadFile(file, venue, symbol)
			if loadErr != nil {
				return nil, fmt.Errorf("failed to load file %s: %w", file, loadErr)
			}
		}

		// Filter by time bounds
		for _, envelope := range fileData {
			if envelope.Timestamp.After(start) && envelope.Timestamp.Before(end) {
				envelope.SourceTier = TierCold
				envelope.Provenance.OriginalSource = fmt.Sprintf("%s_historical", venue)
				envelope.Provenance.ConfidenceScore = 0.7 // Lower confidence for historical
				allData = append(allData, envelope)
			}
		}
	}

	// Sort by timestamp
	sort.Slice(allData, func(i, j int) bool {
		return allData[i].Timestamp.Before(allData[j].Timestamp)
	})

	// Cache results
	c.cache[cacheKey] = allData

	return allData, nil
}

// LoadFromFile loads data from a specific file path
func (c *ColdData) LoadFromFile(filePath string) error {
	if strings.HasSuffix(filePath, ".csv") {
		return c.csvReader.ValidateFile(filePath)
	} else if strings.HasSuffix(filePath, ".parquet") {
		return c.parquetReader.ValidateFile(filePath)
	}

	return fmt.Errorf("unsupported file type: %s", filePath)
}

// findFilesInRange discovers files that might contain data in the time range
func (c *ColdData) findFilesInRange(venue, symbol string, start, end time.Time) ([]string, error) {
	venuePath := filepath.Join(c.config.BasePath, venue)
	if _, err := os.Stat(venuePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("venue directory not found: %s", venuePath)
	}

	var files []string

	// Walk through venue directory
	err := filepath.Walk(venuePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if file might contain the symbol and be in date range
		fileName := info.Name()
		if strings.Contains(fileName, symbol) || strings.Contains(fileName, "all") {
			// Simple heuristic: if file was modified within extended range, include it
			fileTime := info.ModTime()
			extendedStart := start.Add(-24 * time.Hour) // Look back extra day
			extendedEnd := end.Add(24 * time.Hour)      // Look ahead extra day

			if fileTime.After(extendedStart) && fileTime.Before(extendedEnd) {
				files = append(files, path)
			}
		}

		return nil
	})

	return files, err
}

// CleanupCache removes expired cache entries
func (c *ColdData) CleanupCache() {
	// Simple cleanup - in production would track cache timestamps
	if len(c.cache) > 100 { // Arbitrary limit
		c.cache = make(map[string][]*Envelope)
	}
}

// GetStats returns cold tier statistics
func (c *ColdData) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"base_path":        c.config.BasePath,
		"cached_queries":   len(c.cache),
		"cache_expiry":     c.cacheExpiry.String(),
		"enable_parquet":   c.config.EnableParquet,
		"enable_csv":       c.config.EnableCSV,
		"default_format":   c.config.DefaultFormat,
	}

	// Count available venues
	if info, err := os.Stat(c.config.BasePath); err == nil && info.IsDir() {
		if entries, err := os.ReadDir(c.config.BasePath); err == nil {
			venueCount := 0
			for _, entry := range entries {
				if entry.IsDir() {
					venueCount++
				}
			}
			stats["available_venues"] = venueCount
		}
	}

	return stats
}

// LoadFile implements FileReader for CSVReader with OHLCV schema and decompression support
func (r *CSVReader) LoadFile(filePath, venue, symbol string) ([]*Envelope, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Determine compression type from config and file path
	compressionType := detectCompressionFromPath(filePath, r.compressionConfig)
	
	// Create decompressed reader if needed
	var rawReader io.ReadCloser
	if r.compressionConfig.Enable && compressionType != CompressionNone {
		rawReader, err = createCompressedReader(file, compressionType)
		if err != nil {
			return nil, fmt.Errorf("failed to create decompressed reader: %w", err)
		}
		defer rawReader.Close()
	} else {
		rawReader = file
	}

	reader := csv.NewReader(rawReader)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return []*Envelope{}, nil
	}

	// Expected CSV schema: timestamp,open,high,low,close,volume,venue,tier,provenance
	var envelopes []*Envelope
	for i, record := range records {
		if i == 0 {
			// Skip header row if it contains non-numeric data
			if _, err := strconv.ParseFloat(record[1], 64); err != nil {
				continue
			}
		}

		if len(record) < 6 {
			continue // Skip incomplete records
		}

		envelope, err := r.parseCSVRecord(record, venue, symbol)
		if err != nil {
			continue // Skip invalid records
		}

		envelopes = append(envelopes, envelope)
	}

	return envelopes, nil
}

// LoadFileWithTimeFilter implements time-filtered loading for CSVReader
func (r *CSVReader) LoadFileWithTimeFilter(filePath, venue, symbol string, from, until time.Time) ([]*Envelope, error) {
	// For CSV, we need to load all and filter (Parquet can do server-side filtering)
	allData, err := r.LoadFile(filePath, venue, symbol)
	if err != nil {
		return nil, err
	}

	var filtered []*Envelope
	for _, envelope := range allData {
		if envelope.Timestamp.After(from) && envelope.Timestamp.Before(until) {
			filtered = append(filtered, envelope)
		}
	}

	return filtered, nil
}

// WriteFile implements CSV writing with OHLCV schema and compression support
func (r *CSVReader) WriteFile(filePath string, data []*Envelope) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Determine compression type from config and file path
	compressionType := detectCompressionFromPath(filePath, r.compressionConfig)
	
	// Create compressed writer if compression is enabled
	var rawWriter io.WriteCloser
	if r.compressionConfig.Enable && compressionType != CompressionNone {
		rawWriter, err = createCompressedWriter(file, compressionType, r.compressionConfig.Level)
		if err != nil {
			return fmt.Errorf("failed to create compressed writer: %w", err)
		}
		defer rawWriter.Close()
	} else {
		rawWriter = &passthroughCloser{file}
		defer rawWriter.Close()
	}

	writer := csv.NewWriter(rawWriter)
	defer writer.Flush()

	// Write header
	header := []string{"timestamp", "open", "high", "low", "close", "volume", "venue", "tier", "provenance"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, envelope := range data {
		record := []string{
			envelope.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.8f", extractFloatValue(envelope.PriceData, "open", 0)),
			fmt.Sprintf("%.8f", extractFloatValue(envelope.PriceData, "high", 0)),
			fmt.Sprintf("%.8f", extractFloatValue(envelope.PriceData, "low", 0)),
			fmt.Sprintf("%.8f", extractFloatValue(envelope.PriceData, "close", 0)),
			fmt.Sprintf("%.8f", extractFloatValue(envelope.VolumeData, "volume", 0)),
			envelope.Venue,
			string(envelope.SourceTier),
			envelope.Provenance.OriginalSource,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// ValidateFile implements FileReader for CSVReader
func (r *CSVReader) ValidateFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("invalid CSV format: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Validate at least first data row has correct number of fields
	dataStart := 0
	if len(records[0]) > 0 {
		if _, err := strconv.ParseFloat(records[0][1], 64); err != nil {
			dataStart = 1 // Skip header
		}
	}

	if len(records) <= dataStart {
		return fmt.Errorf("CSV file has no data rows")
	}

	firstDataRow := records[dataStart]
	if len(firstDataRow) < 6 {
		return fmt.Errorf("CSV row has insufficient columns, expected at least 6, got %d", len(firstDataRow))
	}

	return nil
}

// parseCSVRecord converts a CSV record to an Envelope
func (r *CSVReader) parseCSVRecord(record []string, venue, symbol string) (*Envelope, error) {
	timestamp, err := time.Parse(time.RFC3339, record[0])
	if err != nil {
		// Try alternative timestamp formats
		if timestamp, err = time.Parse("2006-01-02 15:04:05", record[0]); err != nil {
			return nil, fmt.Errorf("invalid timestamp format: %w", err)
		}
	}

	open, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid open price: %w", err)
	}

	high, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid high price: %w", err)
	}

	low, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid low price: %w", err)
	}

	close, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid close price: %w", err)
	}

	volume, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid volume: %w", err)
	}

	// Extract venue and tier from record if available
	recordVenue := venue
	tier := TierCold
	provenance := "historical"

	if len(record) > 6 {
		recordVenue = record[6]
	}
	if len(record) > 7 {
		tier = SourceTier(record[7])
	}
	if len(record) > 8 {
		provenance = record[8]
	}

	envelope := &Envelope{
		Symbol:    symbol,
		Venue:     recordVenue,
		Timestamp: timestamp,
		SourceTier: tier,
		PriceData: map[string]interface{}{
			"open":  open,
			"high":  high,
			"low":   low,
			"close": close,
		},
		VolumeData: map[string]interface{}{
			"volume": volume,
		},
		Provenance: ProvenanceInfo{
			OriginalSource:    provenance,
			RetrievedAt:       timestamp,
			ConfidenceScore:   0.8, // Historical data confidence
		},
	}

	envelope.CalculateFreshness()
	return envelope, nil
}

// LoadFile implements FileReader for ParquetReader (mock implementation)
func (r *ParquetReader) LoadFile(filePath, venue, symbol string) ([]*Envelope, error) {
	// For now, return a deterministic fake for testing
	// In production, this would use a Go Parquet library like github.com/xitongsys/parquet-go
	return []*Envelope{
		{
			Symbol:     symbol,
			Venue:      venue,
			Timestamp:  time.Now().Add(-1 * time.Hour),
			SourceTier: TierCold,
			PriceData: map[string]interface{}{
				"open":  100.0,
				"high":  105.0,
				"low":   99.0,
				"close": 103.0,
			},
			VolumeData: map[string]interface{}{
				"volume": 1000.0,
			},
			Provenance: ProvenanceInfo{
				OriginalSource:  "parquet_historical",
				RetrievedAt: time.Now().Add(-1 * time.Hour),
				ConfidenceScore: 0.9,
			},
		},
	}, nil
}

// LoadFileWithTimeFilter implements time-filtered loading for ParquetReader
func (r *ParquetReader) LoadFileWithTimeFilter(filePath, venue, symbol string, from, until time.Time) ([]*Envelope, error) {
	// Mock time filtering - would use Parquet column filtering in production
	allData, err := r.LoadFile(filePath, venue, symbol)
	if err != nil {
		return nil, err
	}

	var filtered []*Envelope
	for _, envelope := range allData {
		if envelope.Timestamp.After(from) && envelope.Timestamp.Before(until) {
			filtered = append(filtered, envelope)
		}
	}

	return filtered, nil
}

// WriteFile implements Parquet writing (mock implementation)
func (r *ParquetReader) WriteFile(filePath string, data []*Envelope) error {
	// Mock implementation - in production would use Parquet library
	// For now, just create a file to indicate successful write
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create Parquet file: %w", err)
	}
	defer file.Close()

	// Write mock Parquet header/metadata
	_, err = file.WriteString("PARQUET MOCK DATA\n")
	if err != nil {
		return fmt.Errorf("failed to write Parquet data: %w", err)
	}

	return nil
}

// ValidateFile implements FileReader for ParquetReader
func (r *ParquetReader) ValidateFile(filePath string) error {
	// Mock validation - in production would validate Parquet schema
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open Parquet file: %w", err)
	}
	defer file.Close()

	// Simple validation - check if file has content
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat Parquet file: %w", err)
	}

	if stat.Size() == 0 {
		return fmt.Errorf("Parquet file is empty")
	}

	return nil
}

// Quality validation and anomaly detection methods

// ValidateEnvelope validates a single envelope using the configured validator
func (c *ColdData) ValidateEnvelope(ctx context.Context, envelope *Envelope) (*quality.ValidationResult, error) {
	if c.validator == nil {
		return nil, fmt.Errorf("data validator not initialized")
	}
	
	return c.validator.ValidateEnvelope(ctx, WrapEnvelope(envelope))
}

// ValidateEnvelopes validates multiple envelopes
func (c *ColdData) ValidateEnvelopes(ctx context.Context, envelopes []*Envelope) ([]*quality.ValidationResult, error) {
	if c.validator == nil {
		return nil, fmt.Errorf("data validator not initialized")
	}
	
	// Convert to adapters
	adapters := make([]quality.DataEnvelope, len(envelopes))
	for i, envelope := range envelopes {
		adapters[i] = WrapEnvelope(envelope)
	}
	
	return c.validator.ValidateBatch(ctx, adapters)
}

// IsSymbolQuarantined checks if a symbol is quarantined due to validation failures
func (c *ColdData) IsSymbolQuarantined(symbol string) bool {
	if c.validator == nil {
		return false
	}
	
	return c.validator.IsQuarantined(symbol)
}

// GetValidationStats returns validation statistics for a symbol
func (c *ColdData) GetValidationStats(symbol string) *quality.ValidationCounts {
	if c.validator == nil {
		return &quality.ValidationCounts{}
	}
	
	return c.validator.GetValidationStats(symbol)
}

// SetValidationMetricsCallback sets the metrics callback for the validator
func (c *ColdData) SetValidationMetricsCallback(callback func(string, float64)) {
	if c.validator != nil {
		c.validator.SetMetricsCallback(callback)
	}
}

// ValidateAndLoadFile loads file data and validates it if validation is enabled
func (c *ColdData) ValidateAndLoadFile(ctx context.Context, filePath, venue, symbol string) ([]*Envelope, []*quality.ValidationResult, error) {
	// Get the appropriate reader and load the data
	reader := c.GetFormatReader(filePath)
	envelopes, err := reader.LoadFile(filePath, venue, symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load file data: %w", err)
	}

	// Validate if validator is available
	var validationResults []*quality.ValidationResult
	if c.validator != nil {
		// Convert to adapters
		adapters := make([]quality.DataEnvelope, len(envelopes))
		for i, envelope := range envelopes {
			adapters[i] = WrapEnvelope(envelope)
		}
		validationResults, err = c.validator.ValidateBatch(ctx, adapters)
		if err != nil {
			return envelopes, nil, fmt.Errorf("validation failed: %w", err)
		}

		// Filter out invalid envelopes if fail-fast is enabled
		if c.config.Quality.Validation.FailFast {
			validEnvelopes := make([]*Envelope, 0)
			for i, result := range validationResults {
				if result.Valid {
					validEnvelopes = append(validEnvelopes, envelopes[i])
				}
			}
			return validEnvelopes, validationResults, nil
		}
	}

	return envelopes, validationResults, nil
}

// EnvelopeAdapter adapts data.Envelope to quality.DataEnvelope interface
type EnvelopeAdapter struct {
	envelope *Envelope
}

// Implement quality.DataEnvelope interface
func (e *EnvelopeAdapter) GetSymbol() string                     { return e.envelope.Symbol }
func (e *EnvelopeAdapter) GetVenue() string                      { return e.envelope.Venue }
func (e *EnvelopeAdapter) GetTimestamp() time.Time              { return e.envelope.Timestamp }
func (e *EnvelopeAdapter) GetSourceTier() string                { return string(e.envelope.SourceTier) }
func (e *EnvelopeAdapter) GetPriceData() map[string]interface{} { 
	if e.envelope.PriceData == nil {
		return nil
	}
	return e.envelope.PriceData.(map[string]interface{}) 
}
func (e *EnvelopeAdapter) GetVolumeData() map[string]interface{} { 
	if e.envelope.VolumeData == nil {
		return nil
	}
	return e.envelope.VolumeData.(map[string]interface{}) 
}
func (e *EnvelopeAdapter) GetOrderBook() map[string]interface{} { 
	if e.envelope.OrderBook == nil {
		return nil
	}
	return e.envelope.OrderBook.(map[string]interface{}) 
}
func (e *EnvelopeAdapter) GetProvenance() quality.ProvenanceInfo {
	return quality.ProvenanceInfo{
		OriginalSource:  e.envelope.Provenance.OriginalSource,
		RetrievedAt:     e.envelope.Provenance.RetrievedAt,
		ConfidenceScore: e.envelope.Provenance.ConfidenceScore,
		CacheHit:        e.envelope.Provenance.CacheHit,
		FallbackChain:   e.envelope.Provenance.FallbackChain,
	}
}

// WrapEnvelope creates an adapter for an envelope
func WrapEnvelope(envelope *Envelope) *EnvelopeAdapter {
	return &EnvelopeAdapter{envelope: envelope}
}
