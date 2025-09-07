# Data Architecture — Hot/Warm Facade

> **Status:** Implemented  
> **Tier Strategy:** Hot WebSocket streams + Warm REST caching  
> **Authority:** Exchange-native for microstructure, aggregated for price/volume  

## UX MUST — Live Progress & Explainability

The data facade provides transparent data sourcing with real-time visibility into:
- **Source attribution** for all price and volume data showing origin and confidence
- **Cache hit rates** and TTL status for performance monitoring  
- **PIT integrity** with immutable snapshots and temporal consistency
- **Reconciliation transparency** showing which sources were used vs dropped as outliers

---

## Overview

The data facade implements a two-tier architecture optimizing for both real-time responsiveness and cost efficiency:

- **Hot Tier:** WebSocket streams for top symbols requiring real-time data
- **Warm Tier:** REST API calls with aggressive caching for broader universe
- **Point-in-Time Integrity:** Immutable snapshots with source attribution
- **Canonical Authority:** Exchange-native for microstructure, reconciled aggregation for price/volume

## Architecture Components

### DataFacade Interface

```go
type DataFacade interface {
    // Hot data - WebSocket streams for real-time data
    HotSubscribe(symbols []string) (Stream, error)
    
    // Warm data - REST with TTL caching and PIT snapshots  
    WarmKlines(req KlineReq) (KlineResp, error)
    
    // Exchange-native order books (no aggregators)
    L2Book(symbol string) (BookSnapshot, error)
    
    // Health and metrics
    Health() FacadeHealth
}
```

### Canonical Source Authority

| Data Type | Authority Rule | Primary Sources | Fallback | Aggregators Allowed |
|-----------|---------------|-----------------|----------|-------------------|
| **Microstructure** | Exchange-native ONLY | Binance, OKX, Coinbase, Kraken | — | ❌ BANNED |
| **Price/Volume** | Reconciled aggregation | CoinGecko, CoinPaprika | Exchange REST | ✅ Allowed |
| **Derivatives** | Exchange-native only | Binance, OKX, Kraken | — | ❌ BANNED |

### TTL Configuration

| Data Type | Hot TTL | Warm TTL | Cache Strategy |
|-----------|---------|----------|---------------|
| Prices (hot) | 5s | — | Stream + cache |
| Prices (warm) | — | 30s | REST + Redis |
| Volume/VADR | — | 120s | REST + Redis |
| Depth/Spread | 15s | — | Exchange-native only |
| Funding Rates | — | 300s | Exchange REST |

## Cache Management

### PIT Snapshots

Point-in-time snapshots ensure temporal consistency:

```go
type CacheEntry struct {
    Data        interface{} `json:"data"`
    Source      string      `json:"source"`
    CachedAt    time.Time   `json:"cached_at"`
    ExpiresAt   time.Time   `json:"expires_at"`
    PIT         bool        `json:"point_in_time"`
    Attribution string      `json:"attribution"`
}
```

### Redis Implementation

- **Keys:** `cryptorun:` prefix with hierarchical structure
- **PIT References:** Sorted sets for temporal lookups
- **Expiration:** TTL-based with LRU eviction policy
- **Connection Pooling:** 10 connections with retry logic

## Data Reconciliation

### Outlier Detection

The reconciler filters sources using trimmed median:

1. **Deviation Threshold:** 1% maximum from median
2. **Minimum Sources:** Require ≥2 sources after filtering
3. **Confidence Scoring:** Based on deviation + source count
4. **Attribution:** Full transparency of dropped vs used sources

### Reconciliation Methods

- **Primary:** Median of valid sources (default)
- **Alternative:** Trimmed mean (configurable)
- **Outlier Removal:** Sources >1% deviation from initial median
- **Quality Gates:** Minimum confidence threshold (70%)

## Stream Management

### Hot Stream Multiplexing

```go
type MultiplexedStream struct {
    streams   map[string]Stream  // Per-exchange streams
    symbols   []string           // Subscribed symbols
    tradesCh  chan Trade         // Multiplexed trades
    booksCh   chan BookSnapshot  // Multiplexed books
    barsCh    chan Bar           // Multiplexed bars
}
```

### Health Monitoring

- **Connection Status:** Per-exchange health tracking
- **Message Counts:** Throughput monitoring
- **Latency Tracking:** Average latency across exchanges
- **Error Recovery:** Automatic reconnection with exponential backoff

## Configuration

### Cache Configuration (`config/cache.yaml`)

```yaml
ttls:
  prices_hot: 5          # Hot price data - 5 seconds
  prices_warm: 30        # Warm price data - 30 seconds  
  volumes_vadr: 120      # Volume and VADR - 2 minutes
  depth_spread: 15       # Order book depth/spread - 15 seconds
  
pit_snapshots:
  enabled: true
  retention_hours: 24    # Keep PIT snapshots for 24 hours
```

### Source Configuration (`config/data_sources.yaml`)

```yaml
authority:
  microstructure:
    allowed_sources: ["binance", "okx", "coinbase", "kraken"]
    banned_sources: ["coingecko", "coinpaprika"]  # Aggregators banned
    
  price_volume:
    primary: ["coingecko", "coinpaprika"]
    reconciliation_method: "trimmed_median"
```

## Integration Testing

The facade includes comprehensive integration tests:

- **Hot Stream Subscription:** WebSocket attach/detach with mock data
- **Warm Data Caching:** TTL expiration and cache hit rate validation
- **Microstructure Authority:** Exchange-native source enforcement
- **Outlier Filtering:** Reconciliation with deviation testing
- **PIT Snapshots:** Storage and temporal retrieval validation

## Performance Characteristics

### Target Metrics

- **Cache Hit Rate:** >85% for warm data
- **Stream Latency:** <50ms for hot data
- **Reconciliation Time:** <100ms for multi-source aggregation
- **Memory Usage:** <512MB Redis cache footprint

### Circuit Breakers

- **Failure Threshold:** 5 failures before circuit opens
- **Recovery Timeout:** 60 seconds before retry attempts
- **Half-Open Testing:** 3 test requests during recovery

---

*Data Facade — Exchange-native microstructure with reconciled price/volume aggregation*