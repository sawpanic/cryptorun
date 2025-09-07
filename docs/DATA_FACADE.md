# Data Facade Architecture

## Overview

The Data Facade implements a two-tier architecture for cryptocurrency data ingestion with HOT (WebSocket streaming) and WARM (REST + cache) data paths. This design ensures optimal performance for active trading pairs while maintaining comprehensive coverage of the broader universe.

## Architecture Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HOT TIER      │    │    WARM TIER     │    │   COLD TIER     │
│   (WebSocket)   │    │   (REST+Cache)   │    │ (PIT Snapshots) │
├─────────────────┤    ├──────────────────┤    ├─────────────────┤
│ • Top 30 pairs │    │ • Full universe  │    │ • Historical    │
│ • Real-time     │    │ • TTL cached     │    │ • Immutable     │
│ • <100ms lat    │    │ • 5s-30s fresh   │    │ • Compressed    │
│ • Auto failover │    │ • Rate limited   │    │ • Audit trail   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                       │                       │
        └───────────────────────┼───────────────────────┘
                                │
                    ┌──────────────────┐
                    │   DATA FACADE    │
                    │   (Unified API)  │
                    └──────────────────┘
```

## Core Components

### 1. Data Facade Interface (`internal/data/facade/`)

**Primary Interface:**
```go
type DataFacade interface {
    // Streaming subscriptions (HOT tier)
    SubscribeTrades(ctx context.Context, venue string, symbol string, callback TradesCallback) error
    SubscribeBookL2(ctx context.Context, venue string, symbol string, callback BookCallback) error
    
    // REST data fetching (WARM tier)
    GetKlines(ctx context.Context, venue string, symbol string, interval string, limit int) ([]Kline, error)
    GetTrades(ctx context.Context, venue string, symbol string, limit int) ([]Trade, error)
    GetBookL2(ctx context.Context, venue string, symbol string) (*BookL2, error)
    
    // Health and monitoring
    VenueHealth(venue string) VenueHealth
    CacheStats() CacheStats
    SourceAttribution(venue string) Attribution
    
    // Lifecycle
    Start(ctx context.Context) error
    Stop() error
}
```

### 2. TTL Cache System (`internal/data/cache/`)

**Multi-tier caching with different TTLs:**
- **PricesHot**: 5 seconds (streaming price updates)
- **PricesWarm**: 30 seconds (REST API responses)
- **VolumesVADR**: 120 seconds (volume analysis)
- **TokenMeta**: 24 hours (token metadata)

**Features:**
- LRU eviction when capacity exceeded
- Thread-safe operations
- Hit/miss statistics tracking
- Automatic expiration cleanup

### 3. Rate Limiting (`internal/data/rl/`)

**Venue-specific budget guards:**
- Per-venue request budgets
- Header parsing for API limits
- Exponential backoff on failures
- Circuit breaker patterns

**Example Usage:**
```go
rateLimiter := rl.NewRateLimiter()
allowed, remaining := rateLimiter.CheckBudget("kraken", 10)
if !allowed {
    delay := rateLimiter.GetBackoff("kraken", attemptCount)
    time.Sleep(delay)
}
```

### 4. Point-in-Time Storage (`internal/data/pit/`)

**Immutable snapshot storage:**
- Gzip compression for efficiency
- Atomic file operations
- Metadata tracking (timestamps, record counts, size)
- Cleanup policies for old snapshots

**Snapshot Structure:**
```
artifacts/pit/
├── btcusd/
│   ├── 2025-09-07/
│   │   ├── 20250907_143022_abc123.json.gz
│   │   └── 20250907_143122_def456.json.gz
│   └── metadata/
│       ├── abc123.meta.json
│       └── def456.meta.json
└── ethusd/
    └── 2025-09-07/
        └── 20250907_143222_ghi789.json.gz
```

### 5. Exchange Adapters (`internal/data/exchanges/`)

**Kraken Adapter Implementation:**
- WebSocket streaming for live data
- REST API integration with rate limiting
- Error handling and reconnection logic
- Data normalization to facade types

**Extensible Pattern:**
```go
type ExchangeAdapter interface {
    Connect(ctx context.Context) error
    Subscribe(pairs []string) error
    GetOrderBook(symbol string) (*BookL2, error)
    GetRecentTrades(symbol string, limit int) ([]Trade, error)
}
```

## Configuration

### Hot Tier Configuration
```go
hotCfg := facade.HotConfig{
    Venues:       []string{"kraken", "binance", "okx", "coinbase"},
    MaxPairs:     30,           // Top pairs for streaming
    ReconnectSec: 5,            // WebSocket reconnect interval
    BufferSize:   1000,         // Message buffer size
    Timeout:      10 * time.Second,
}
```

### Warm Tier Configuration
```go
warmCfg := facade.WarmConfig{
    Venues:       []string{"kraken", "binance", "okx", "coinbase"},
    DefaultTTL:   30 * time.Second,
    MaxRetries:   3,
    BackoffBase:  1 * time.Second,
    RequestLimit: 100,          // Requests per minute
}
```

### Cache Configuration
```go
cacheCfg := facade.CacheConfig{
    PricesHot:   5 * time.Second,    // Hot price updates
    PricesWarm:  30 * time.Second,   // Warm price fetches
    VolumesVADR: 120 * time.Second,  // Volume analysis
    TokenMeta:   24 * time.Hour,     // Token metadata
    MaxEntries:  10000,              // Total cache capacity
}
```

## Usage Patterns

### 1. Streaming Data Consumption

```go
// Initialize facade
df := facade.New(hotCfg, warmCfg, cacheCfg, rateLimiter)
ctx := context.Background()

// Start services
if err := df.Start(ctx); err != nil {
    log.Fatal(err)
}
defer df.Stop()

// Subscribe to real-time trades
tradesCallback := func(trades []facade.Trade) error {
    for _, trade := range trades {
        processTradeSignal(trade)
    }
    return nil
}

err := df.SubscribeTrades(ctx, "kraken", "BTCUSD", tradesCallback)
```

### 2. Historical Data Analysis

```go
// Fetch recent klines for analysis
klines, err := df.GetKlines(ctx, "kraken", "BTCUSD", "1h", 24)
if err != nil {
    return err
}

// Calculate VADR with freeze logic
vadrCalc := metrics.NewVADRCalculator()
vadrValue, frozen, err := vadrCalc.Calculate(klines, 24*time.Hour)
if frozen {
    log.Warn("VADR frozen due to insufficient data")
    return nil
}

// Use VADR in scoring logic
if vadrValue >= 1.8 {
    proceedWithEntry()
}
```

### 3. Health Monitoring

```go
// Check venue health
health := df.VenueHealth("kraken")
if !health.WSConnected || !health.RESTHealthy {
    log.Warn("Kraken experiencing connectivity issues")
    // Switch to backup venue or pause trading
}

// Monitor cache performance
stats := df.CacheStats()
if stats.PricesHot.HitRatio < 0.8 {
    log.Warn("Low cache hit ratio, consider adjusting TTLs")
}
```

## Performance Characteristics

### Latency Targets
- **Hot Path**: <100ms (WebSocket to callback)
- **Warm Path**: <500ms (REST with cache hit)
- **Cold Path**: <2s (REST with cache miss)

### Throughput Capacity
- **Streaming**: 1000+ updates/sec per venue
- **REST**: 60+ requests/min per venue (rate limited)
- **Cache**: 10,000+ ops/sec (in-memory)

### Memory Usage
- **Cache**: ~50MB for 10,000 entries
- **Buffers**: ~10MB for WebSocket buffers
- **Overhead**: ~20MB for facade operations

## Error Handling

### WebSocket Failures
- Automatic reconnection with exponential backoff
- Fallback to REST API for critical data
- Health status tracking and alerting

### Rate Limit Handling
- Request budget tracking per venue
- Respect `Retry-After` headers
- Circuit breaker on persistent failures

### Data Quality Issues
- Freshness validation with penalties
- VADR freeze logic for insufficient data
- Attribution tracking for audit trails

## Monitoring and Observability

### Key Metrics
- Connection health per venue
- Cache hit ratios by tier
- Request latencies (P50, P99)
- Data freshness by source
- Error rates and types

### Health Endpoints
```go
// Venue status
health := df.VenueHealth("kraken")
// Returns: status, WS connected, REST healthy, P99 latency, recommendation

// Cache performance
stats := df.CacheStats()
// Returns: hit/miss counts, ratios, entry counts by tier

// Data attribution
attr := df.SourceAttribution("kraken")
// Returns: sources, cache hits/misses, last update, latency
```

## Testing Strategy

### Unit Tests
- TTL cache operations and expiration
- Rate limiter budget and backoff logic
- PIT store compression and retrieval
- VADR calculation with freeze logic
- Freshness penalty calculations

### Integration Tests
- WebSocket connection and failover
- REST API with mock servers
- Cache integration with real TTL behavior
- Circuit breaker behavior under failures

### Load Tests
- Streaming throughput under load
- Cache performance with high hit rates
- Memory usage under sustained load
- P99 latency validation

## CLI Integration

### Data Probe Command
```bash
# Static analysis
cryptorun probe data --venue kraken --pair BTCUSD --mins 5

# Streaming mode
cryptorun probe data --venue kraken --pair BTCUSD --stream --mins 10
```

### Menu Integration
```
d) Data Facade Status
   📊 Venue Health, 💾 Cache Performance, 🔄 Data Freshness
   ⏱️ Rate Limiting Status, 💡 Available Commands
```

## Security Considerations

### API Key Management
- No hardcoded credentials
- Environment variable configuration
- Keyless APIs preferred (free tiers)

### Rate Limiting Compliance
- Respect venue-specific limits
- Budget guards prevent overruns
- Exponential backoff on violations

### Data Integrity
- Immutable PIT snapshots
- Compressed storage with checksums
- Audit trails for all operations

## Future Enhancements

### Planned Features
- Multi-venue aggregation with conflict resolution
- Predictive cache warming based on trading patterns
- Advanced health scoring with ML-based anomaly detection
- Real-time data quality scoring

### Scalability Improvements
- Horizontal scaling with venue sharding
- Distributed caching with Redis backend
- Load balancing across multiple facade instances
- Advanced circuit breaker configurations

## UX MUST — Live Progress & Explainability

The Data Facade provides comprehensive real-time monitoring and explainability:

- **Live Health Dashboard**: Real-time venue connectivity, WebSocket status, REST health
- **Cache Performance Metrics**: Hit ratios, entry counts, TTL effectiveness by tier
- **Data Freshness Indicators**: Age tracking with visual freshness status (🟢🟡🔴)
- **Rate Limiting Status**: Budget remaining, reset timers, backoff status per venue
- **Source Attribution**: Complete data lineage with cache hits, source counts, latency tracking
- **Interactive Probe Mode**: Stream mode shows live trades, orderbook updates with spread analysis
- **Comprehensive Error Context**: Detailed error messages with venue, operation, and recovery suggestions