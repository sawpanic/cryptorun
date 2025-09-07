# CryptoRun Streaming Architecture

## UX MUST — Live Progress & Explainability

Real-time event streaming infrastructure with comprehensive mirroring, dead letter queues, and multi-region failover capabilities for CryptoRun's 6-48h momentum detection system.

## Overview

CryptoRun's streaming layer provides event-driven architecture for real-time market data ingestion, analysis pipeline coordination, and multi-region data replication. Built with pluggable backends supporting Kafka, Pulsar, and stub implementations for testing.

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   PUBLISHERS    │    │   EVENT TOPICS   │    │   SUBSCRIBERS   │
├─────────────────┤    ├──────────────────┤    ├─────────────────┤
│ • Exchange APIs │───▶│ market-prices    │───▶│ • Momentum Calc │
│ • Regime Detect │───▶│ market-trades    │───▶│ • Gate Eval     │
│ • Scoring Eng   │───▶│ momentum-scores  │───▶│ • Alert System  │
│ • Alert System  │───▶│ regime-detection │───▶│ • Cold Tier     │
│ • Health Checks │───▶│ signals/alerts   │───▶│ • Monitoring    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                    ┌──────────────────┐
                    │   MULTI-REGION   │
                    │     MIRRORING    │
                    └──────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
 ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
 │  US-EAST-1   │    │  US-WEST-2   │    │  EU-WEST-1   │
 │  (PRIMARY)   │    │ (SECONDARY)  │    │  (BACKUP)    │
 └──────────────┘    └──────────────┘    └──────────────┘
```

## Event Bus Interface

### Core Operations

```go
type EventBus interface {
    // Core pub/sub operations
    Publish(ctx context.Context, topic, key string, payload []byte) error
    Subscribe(ctx context.Context, topic, group string, handler MessageHandler) error
    
    // Batch operations for high throughput
    PublishBatch(ctx context.Context, messages []Message) error
    SubscribeWithFilter(ctx context.Context, topic, group string, 
                       filter MessageFilter, handler MessageHandler) error
    
    // Topic management
    CreateTopic(ctx context.Context, config TopicConfig) error
    DeleteTopic(ctx context.Context, topic string) error
    GetTopicInfo(ctx context.Context, topic string) (*TopicInfo, error)
    
    // Health and lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() HealthStatus
}
```

### Message Structure

```go
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
```

## Topic Schema

### Market Data Topics

| Topic | Partitions | Retention | Description |
|-------|------------|-----------|-------------|
| `cryptorun-market-prices` | 12 | 24h | Real-time price updates |
| `cryptorun-market-trades` | 24 | 7d | Trade execution data |
| `cryptorun-market-orderbook` | 12 | 24h | L2 order book snapshots |

### Analysis Topics

| Topic | Partitions | Retention | Description |
|-------|------------|-----------|-------------|
| `cryptorun-momentum-scores` | 6 | 72h | Calculated momentum scores |
| `cryptorun-regime-detection` | 3 | 7d | Market regime classifications |
| `cryptorun-gate-evaluations` | 6 | 48h | Entry/exit gate results |

### Alert Topics

| Topic | Partitions | Retention | Description |
|-------|------------|-----------|-------------|
| `cryptorun-signals` | 6 | 7d | Trading signals and alerts |
| `cryptorun-notifications` | 3 | 72h | User notifications |

### System Topics

| Topic | Partitions | Retention | Description |
|-------|------------|-----------|-------------|
| `cryptorun-metrics` | 3 | 7d | Application metrics |
| `cryptorun-health-checks` | 1 | 24h | System health status |
| `cryptorun-audit-log` | 6 | 90d | Compliance and audit trail |

## Backend Implementations

### Kafka Backend

```go
config := DefaultKafkaConfig()
config.Brokers = []string{"kafka1:9092", "kafka2:9092", "kafka3:9092"}
config.ProducerConfig.RequiredAcks = -1  // Wait for all replicas
config.ProducerConfig.EnableIdempotent = true
config.ConsumerConfig.GroupID = "cryptorun-consumers"

kafkaBus, err := NewEventBus(BusTypeKafka, config)
```

**Features:**
- Exactly-once semantics with idempotent producers
- Consumer groups for load balancing
- Configurable acknowledgment levels
- Schema registry integration

### Pulsar Backend

```go
config := DefaultPulsarConfig()
config.Brokers = []string{"pulsar://pulsar1:6650", "pulsar://pulsar2:6650"}
config.ProducerConfig.CompressionType = "lz4"
config.DeadLetterConfig.Enabled = true

pulsarBus, err := NewEventBus(BusTypePulsar, config)
```

**Features:**
- Native multi-tenancy support
- Built-in schema evolution
- Pulsar Functions for stream processing
- Geo-replication with BookKeeper

### Stub Backend (Testing)

```go
config := DefaultStubConfig()
config.ClientID = "test-client"

stubBus, err := NewEventBus(BusTypeStub, config)
```

**Features:**
- In-memory implementation for testing
- Synchronous message delivery
- No external dependencies
- Complete API compatibility

## Multi-Region Mirroring

### Configuration

```yaml
# configs/stream/mirroring.yaml
topology:
  primary_region: "us-east-1"
  secondary_regions: ["us-west-2", "eu-west-1"]

policies:
  active_active:
    topics: ["cryptorun-market-prices", "cryptorun-signals"]
    lag_threshold_ms: 500
    cutover_policy: "automatic"
    
  active_passive:
    topics: ["cryptorun-momentum-scores", "cryptorun-metrics"]
    lag_threshold_ms: 5000
    cutover_policy: "manual"
```

### Conflict Resolution

1. **Timestamp Wins**: Latest message timestamp takes precedence
2. **Region Priority**: Primary region always wins in conflicts  
3. **Merge Strategy**: Custom logic for complex data structures

### Automatic Cutover

Triggers:
- Primary region unhealthy > 60s
- Replication lag > threshold
- Error rate > 5%

Conditions:
- Primary healthy > 300s for rollback
- Replication sync complete
- Operator approval required

## Retry Logic and Dead Letter Queues

### Retry Configuration

```go
retryConfig := RetryConfig{
    MaxRetries:    3,
    InitialDelay:  100 * time.Millisecond,
    MaxDelay:      10 * time.Second,
    BackoffFactor: 2.0,
    JitterEnabled: true,
}
```

### Dead Letter Queue Flow

```
Message Processing Failed
         │
         ▼
    Retry Attempt 1 (100ms delay)
         │
      Failed?
         ▼
    Retry Attempt 2 (200ms delay)
         │
      Failed?
         ▼
    Retry Attempt 3 (400ms delay)
         │
      Failed?
         ▼
   Send to Dead Letter Queue
   (Topic: cryptorun-dlq)
         │
         ▼
    Quarantine after 5 failures
```

## Usage Examples

### Basic Publishing

```go
bus, _ := NewEventBus(BusTypeKafka, DefaultKafkaConfig())
ctx := context.Background()
bus.Start(ctx)

// Publish market data
marketData := MarketPrice{
    Venue:  "kraken",
    Symbol: "BTC-USD", 
    Price:  50000.0,
    Time:   time.Now(),
}

payload, _ := json.Marshal(marketData)
err := bus.Publish(ctx, "cryptorun-market-prices", "BTC-USD", payload)
```

### Subscribing with Processing

```go
handler := func(ctx context.Context, message *Message) error {
    var price MarketPrice
    json.Unmarshal(message.Payload, &price)
    
    // Process price update
    momentumScore := calculateMomentum(price)
    
    // Publish to next stage
    scorePayload, _ := json.Marshal(momentumScore)
    return bus.Publish(ctx, "cryptorun-momentum-scores", price.Symbol, scorePayload)
}

bus.Subscribe(ctx, "cryptorun-market-prices", "momentum-calculator", handler)
```

### Filtered Subscription

```go
// Only process signals for major pairs
majorPairFilter := func(message *Message) bool {
    majorPairs := []string{"BTC-USD", "ETH-USD", "SOL-USD"}
    for _, pair := range majorPairs {
        if strings.Contains(string(message.Payload), pair) {
            return true
        }
    }
    return false
}

bus.SubscribeWithFilter(ctx, "cryptorun-signals", "major-pair-alerts", 
                       majorPairFilter, alertHandler)
```

### Historical Replay from Cold Tier

```go
// Read historical data from Parquet files
historicalData, _ := parquetStore.ReadBatch(ctx, "historical_btc.parquet", 0)

// Replay through streaming system
for _, envelope := range historicalData {
    payload, _ := json.Marshal(envelope.OrderBook)
    
    message := Message{
        Topic:     "cryptorun-historical-replay",
        Key:       fmt.Sprintf("%s-%s", envelope.Venue, envelope.Symbol),
        Payload:   payload,
        Timestamp: envelope.Timestamp,  // Preserve original timing
        Headers: map[string]string{
            "source": "cold_tier_replay",
            "confidence": fmt.Sprintf("%.2f", envelope.Provenance.ConfidenceScore),
        },
    }
    
    bus.PublishBatch(ctx, []Message{message})
}
```

## Monitoring and Observability

### Health Checks

```go
health := bus.Health()
if !health.Healthy {
    log.Printf("Bus unhealthy: %v", health.Errors)
}

// Metrics available
fmt.Printf("Connected brokers: %d\n", health.Metrics.ConnectedBrokers)
fmt.Printf("Producer latency: %.2fms\n", health.Metrics.ProducerLatencyMS)
fmt.Printf("Consumer latency: %.2fms\n", health.Metrics.ConsumerLatencyMS)
```

### Metrics Collection

```go
metricsCallback := func(metric string, value interface{}, tags map[string]string) {
    prometheus.CounterVec.WithLabelValues(tags["topic"]).Add(value.(float64))
}

config.MetricsCallback = metricsCallback
```

**Available Metrics:**
- `stream_publish_total` - Messages published
- `stream_consume_total` - Messages consumed  
- `stream_publish_bytes` - Bytes published
- `stream_handler_error_total` - Handler failures
- `stream_retries_total` - Retry attempts
- `stream_dlq_total` - Messages sent to DLQ
- `stream_mirror_lag` - Cross-region replication lag

## Error Handling and Resilience

### Circuit Breaker Pattern

```go
// Automatic circuit breaking on repeated failures
config.RetryConfig.MaxRetries = 3
config.DeadLetterConfig.QuarantineAfter = 5

// Circuit opens after 5 consecutive DLQ messages
// Allows partial recovery with reduced load
```

### Poison Message Handling

1. **Detection**: Messages that consistently fail processing
2. **Quarantine**: Move to dedicated quarantine topic
3. **Analysis**: Debug problematic message structure
4. **Resolution**: Fix handler logic or message format
5. **Replay**: Reprocess quarantined messages

### Network Partitions

- **Producer**: Buffer messages locally with disk spillover
- **Consumer**: Continue processing from last committed offset
- **Mirroring**: Automatic failover to secondary region
- **Recovery**: Sync replication when connectivity restored

## Performance Characteristics

### Throughput Benchmarks

| Backend | Messages/sec | Latency P99 | Memory Usage |
|---------|--------------|-------------|--------------|
| Kafka | 100K+ | <10ms | 512MB |
| Pulsar | 80K+ | <15ms | 768MB |
| Stub | 1M+ | <1ms | 64MB |

### Scaling Guidelines

- **Partitions**: 2-3x number of consumer instances
- **Batch Size**: 1K-10K messages for optimal throughput
- **Retention**: Balance storage cost vs replay requirements
- **Replication**: 3x for production critical topics

## Security

### Authentication
- Kafka: SASL/SCRAM or mTLS
- Pulsar: JWT tokens or mTLS
- Network: VPC isolation with security groups

### Encryption
- In-transit: TLS 1.2+ for all broker communication
- At-rest: Encrypted storage for message persistence
- Keys: Vault-managed certificates with rotation

### Authorization
- Topic-level ACLs for fine-grained access control
- Consumer group isolation
- Producer authentication per service

---

This streaming architecture provides the real-time data backbone for CryptoRun's momentum detection system, ensuring reliable event flow with comprehensive failure handling and multi-region resilience.