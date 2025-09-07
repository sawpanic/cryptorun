# Data Architecture v1 — Hot/Warm/Cold Layers

## UX MUST — Live Progress & Explainability

This document describes CryptoRun's three-tier data architecture that provides **point-in-time integrity**, **source authority**, and **deterministic fallbacks** with full provenance tracking.

## Overview

CryptoRun implements a hierarchical data system with three distinct tiers:

- **🔥 Hot Tier**: Real-time WebSocket streams (≤5s freshness)
- **🌡️ Warm Tier**: REST APIs with caching (≤60s freshness) 
- **🧊 Cold Tier**: Historical files (CSV/Parquet, no freshness limit)

The **Bridge Orchestrator** coordinates tier selection using "worst feed wins" cascade logic, ensuring higher authority sources are never overwritten by lower ones.

## Architecture Components

### Data Envelope

All data records are wrapped in a standardized `Envelope` structure:

```go
type Envelope struct {
    Timestamp   time.Time  `json:"timestamp"`
    Venue       string     `json:"venue"`
    Symbol      string     `json:"symbol"`
    SourceTier  SourceTier `json:"source_tier"`
    FreshnessMS int64      `json:"freshness_ms"`
    
    // Provenance tracking
    Provenance ProvenanceInfo `json:"provenance"`
    Checksum   string         `json:"checksum"`
    
    // Payload data
    OrderBook   interface{} `json:"order_book,omitempty"`
    PriceData   interface{} `json:"price_data,omitempty"`
    // ... other data types
}
```

### Provenance Information

Every data record includes complete lineage tracking:

```go
type ProvenanceInfo struct {
    OriginalSource  string    `json:"original_source"`
    CacheHit        bool      `json:"cache_hit"`
    FallbackChain   []string  `json:"fallback_chain,omitempty"`
    RetrievedAt     time.Time `json:"retrieved_at"`
    LatencyMS       int64     `json:"latency_ms"`
    RetryCount      int       `json:"retry_count"`
    ConfidenceScore float64   `json:"confidence_score"`
}
```

## Tier Specifications

### Hot Tier — Real-Time Streams

**Purpose**: Sub-second market data via WebSocket connections

**Sources**: 
- Binance WebSocket API
- OKX WebSocket API  
- Coinbase Pro WebSocket API

**Characteristics**:
- **Latency**: 30-100ms
- **Freshness**: ≤5 seconds
- **Confidence**: 0.90-0.98
- **Authority Level**: 3 (highest)

**Implementation**: `internal/data/hot.go` + `internal/data/ws/`

```go
// Usage example
hot := NewHotData()
hot.RegisterClient("binance", binanceWSClient)
hot.Subscribe("binance", "BTCUSD")

envelope, err := hot.GetOrderBook(ctx, "binance", "BTCUSD")
```

### Warm Tier — REST + Cache

**Purpose**: Reliable cached data with provider guards

**Sources**:
- Exchange REST APIs (Binance, OKX, Coinbase, Kraken)
- Redis/memory caching
- Circuit breakers + rate limiting

**Characteristics**:
- **Latency**: 100-500ms
- **Freshness**: ≤60 seconds  
- **Confidence**: 0.80-0.90
- **Authority Level**: 2

**Implementation**: `internal/data/warm.go` + `internal/providers/guards/`

```go
// Usage example  
warm := NewWarmData(WarmConfig{
    DefaultTTLSeconds: 60,
    GuardConfigs: guardConfigs,
})

envelope, err := warm.GetOrderBook(ctx, "kraken", "BTCUSD")
```

### Cold Tier — Historical Files

**Purpose**: Historical analysis and backfill data

**Sources**:
- CSV files with configurable formats
- Parquet files (future implementation)
- Local filesystem storage

**Characteristics**:
- **Latency**: 10-100ms (file I/O)
- **Freshness**: No limit (historical)
- **Confidence**: 0.70-0.80
- **Authority Level**: 1 (lowest)

**Implementation**: `internal/data/cold.go` + `internal/data/cold/`

```go
// Usage example
cold, _ := NewColdData(ColdConfig{
    BasePath: "./historical_data",
    CacheExpiry: "1h",
})

slice, err := cold.GetHistoricalSlice(ctx, "binance", "BTCUSD", 
    start, end)
```

## Bridge Orchestrator

### Cascade Logic

The Bridge implements **"worst feed wins"** fallback:

1. **Try Hot Tier** — If available and fresh (≤5s)
2. **Try Warm Tier** — If hot fails/stale and warm fresh (≤60s) 
3. **Try Cold Tier** — If hot+warm fail (no freshness check)
4. **Fail** — If all tiers unavailable

```go
// Bridge usage
bridge := NewBridge(hot, warm, cold, DefaultBridgeConfig())

envelope, err := bridge.GetOrderBook(ctx, "binance", "BTCUSD")
// Automatically selects best available tier
```

### Freshness Gates

Configurable age limits per tier:

```go
type BridgeConfig struct {
    MaxAgeHotMS    int64 // Default: 5000ms
    MaxAgeWarmMS   int64 // Default: 60000ms  
    EnableFallback bool  // Default: true
}
```

### Authority Validation

Source authority prevents lower-tier data from overwriting higher-tier data:

- **Hot (3)** > **Warm (2)** > **Cold (1)**
- Incoming data must have ≥ existing authority level
- Fallback chains are recorded in provenance

## Point-in-Time Integrity

### Timestamp Consistency

- All timestamps in UTC
- Nanosecond precision where available
- Source-provided timestamps preferred over receive times

### Checksum Generation

Deterministic checksums ensure data integrity:

```go
checksum := envelope.GenerateChecksum(data, "order_book")
// SHA256 of (venue, symbol, timestamp, value, unit)
```

### Time Window Enforcement

Cold tier enforces explicit time bounds:

```go
// GetHistoricalSlice ensures no data pollution
data := cold.GetHistoricalSlice(ctx, venue, symbol, start, end)
// Only returns data within [start, end) window
```

## File Format Support

### CSV Format

Flexible column mapping with multiple timestamp formats:

```csv
timestamp,symbol,venue,bid_price,ask_price,spread_bps
2023-01-01 10:00:00,BTCUSD,binance,49950.00,50050.00,20.0
```

**Supported Columns**:
- `timestamp` / `ts` / `datetime`
- `symbol` / `pair` / `instrument` 
- `venue` / `exchange` / `source`
- `bid_price` / `best_bid` / `bid`
- `ask_price` / `best_ask` / `ask`
- `spread_bps` / `spread`

### Parquet Format (Future)

Columnar storage for efficient historical queries:

```go
// Planned implementation
reader := NewParquetReader()
data, err := reader.LoadFile("btc_2023.parquet", "binance", "BTCUSD")
```

## Configuration Files

### Data Sources

`config/data_sources.yaml`:
```yaml
hot:
  binance:
    websocket_url: "wss://stream.binance.com:9443"
    max_reconnect_attempts: 5
  okx:
    websocket_url: "wss://ws.okx.com:8443"
    
warm:
  default_ttl_seconds: 60
  endpoints:
    binance:
      base_url: "https://api.binance.com"
      orderbook_path: "/api/v3/depth"
      
cold:
  base_path: "./data/historical"
  cache_expiry: "1h"
  enable_cache: true
```

### Provider Guards

`config/provider_guards.yaml`:
```yaml
binance:
  ttl_seconds: 60
  burst_limit: 10
  sustained_rate: 5.0
  max_retries: 3
  failure_thresh: 0.5
  
kraken:
  ttl_seconds: 30
  burst_limit: 5  
  sustained_rate: 2.0
  max_retries: 2
  failure_thresh: 0.6
```

## Performance Metrics

### Target Latencies

| Tier | P50 | P95 | P99 |
|------|-----|-----|-----|
| Hot | 50ms | 100ms | 200ms |
| Warm | 150ms | 300ms | 500ms |
| Cold | 20ms | 50ms | 100ms |

### Cache Hit Rates

- **Warm Tier**: >85% hit rate target
- **Cold Tier**: >95% hit rate (file system cache)
- **Bridge Cache**: >90% authority validation cache

### Freshness Compliance

- **Hot**: 99% of data ≤5s fresh
- **Warm**: 95% of data ≤60s fresh  
- **Cold**: No freshness requirement

## Error Handling

### Circuit Breaker States

Provider guards implement circuit breaker pattern:

```go
type CircuitState string
const (
    StateClosed   CircuitState = "closed"   // Normal operation
    StateOpen     CircuitState = "open"     // Failure threshold exceeded
    StateHalfOpen CircuitState = "half_open" // Probing for recovery
)
```

### Fallback Chains

All fallbacks are recorded for debugging:

```json
{
    "fallback_chain": [
        "hot_failed:connection timeout",
        "warm_stale:120000ms",
        "cold_success"
    ]
}
```

### Retry Strategies

- **Hot**: No retries (real-time priority)
- **Warm**: Exponential backoff, max 3 attempts
- **Cold**: File I/O retry, max 2 attempts

## Testing Strategy

### Unit Tests

- `bridge_test.go`: Cascade logic, authority validation
- `envelope_test.go`: Checksum generation, freshness calculation
- Individual tier implementations

### Integration Tests  

- `integration_test.go`: End-to-end tier fallbacks
- WebSocket client mock interactions
- File format parsing validation

### Performance Tests

- Latency benchmarks per tier
- Memory usage under load
- Cache performance validation

## Monitoring & Observability

### Health Endpoints

```go
GET /health/data/hot     // Hot tier status + connection counts
GET /health/data/warm    // Warm tier status + cache stats  
GET /health/data/cold    // Cold tier status + file counts
GET /health/data/bridge  // Overall bridge health
```

### Metrics

Prometheus-compatible metrics:

```
cryptorun_data_requests_total{tier="hot",venue="binance"}
cryptorun_data_latency_seconds{tier="warm",venue="kraken"}  
cryptorun_data_cache_hits_total{tier="warm"}
cryptorun_data_fallback_total{from="hot",to="warm"}
```

### Logging

Structured logging with provenance context:

```json
{
    "level": "info",
    "msg": "data_retrieved",
    "venue": "binance", 
    "symbol": "BTCUSD",
    "tier": "hot",
    "freshness_ms": 1250,
    "cache_hit": false,
    "checksum": "a1b2c3..."
}
```

## Security Considerations

### API Key Management

- No API keys required for current implementation
- Future authenticated endpoints will use environment variables
- Secrets never logged or cached

### Rate Limit Compliance

- Venue-specific rate limits enforced by provider guards
- Exponential backoff on rate limit violations
- Circuit breakers prevent API abuse

### Data Integrity

- Checksums prevent data tampering
- Timestamp validation prevents replay attacks
- Source verification ensures authentic data

## Future Enhancements

### Planned Features

1. **Parquet Support**: Full Apache Parquet integration
2. **Data Compression**: Gzip/LZ4 for historical files  
3. **Streaming Cold**: Kafka/Pulsar integration
4. **Multi-Region**: Geographic data distribution
5. **Data Validation**: Schema enforcement + anomaly detection

### Performance Improvements

1. **Async Processing**: Non-blocking tier queries
2. **Connection Pooling**: HTTP client optimization
3. **Memory Optimization**: Streaming large datasets
4. **Caching Strategy**: Multi-level cache hierarchy

## Cross-References

- [PROVIDERS.md](./PROVIDERS.md) — Provider Guard configuration
- [SCORING.md](./SCORING.md) — How data feeds into scoring
- [ARTIFACTS.md](./ARTIFACTS.md) — Artifact generation from data layers
- [CLI.md](./CLI.md) — CLI commands for data tier testing

---

**Implementation Status**: ✅ Core tiers, Bridge orchestrator, Provenance tracking  
**Next Steps**: Parquet support, Performance optimization, Multi-region deployment  
**Version**: Data Layers v1.0 (2025-09-07)