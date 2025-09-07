# Free API Hygiene & Guardrails

## UX MUST — Live Progress & Explainability

**Provider Guard Middleware**: Unified protection layer for all free APIs with TTL caching, token bucket rate limiting, exponential backoff with jitter, provider-aware circuit breakers, and point-in-time integrity headers.

## Overview

The Provider Guard system implements a comprehensive middleware layer that wraps all external API calls with robust protection mechanisms. This ensures reliable operation under load while respecting provider limits and maintaining data integrity.

### Key Features

- **TTL Caching**: In-memory + optional file-backed caching with configurable TTLs
- **Rate Limiting**: Token bucket algorithm with burst capacity and sustained rates
- **Circuit Breakers**: Failure-rate monitoring with half-open probes and health tracking
- **Exponential Backoff**: Capped backoff with jitter to prevent thundering herd
- **Point-in-Time Headers**: ETag and If-Modified-Since for cache coherence
- **Telemetry**: Comprehensive metrics collection with CSV export

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Application   │───▶│  Provider Guard  │───▶│  External API   │
│     Layer       │    │   Middleware     │    │   (CoinGecko,   │
└─────────────────┘    └──────────────────┘    │  Binance, etc)  │
                              │                └─────────────────┘
                              ▼
                       ┌──────────────────┐
                       │   Telemetry &    │
                       │  Health Metrics  │
                       └──────────────────┘
```

### Components

1. **ProviderGuard**: Main orchestration layer
2. **Cache**: TTL-based caching with PIT headers
3. **RateLimiter**: Token bucket rate limiting
4. **CircuitBreaker**: Failure detection and recovery
5. **Telemetry**: Metrics collection and export
6. **Adapters**: Provider-specific implementations

## Configuration

Provider guards are configured via `config/providers.yaml`:

```yaml
# Default settings for all providers
defaults:
  ttl_seconds: 300         # 5 minutes cache TTL
  burst_limit: 20          # Token bucket burst
  sustained_rate: 2.0      # Requests per second
  max_retries: 3           # Retry attempts
  backoff_base_ms: 100     # Base backoff delay
  failure_thresh: 0.5      # Circuit breaker threshold
  window_requests: 10      # Failure rate window
  probe_interval: 30       # Half-open probe interval
  enable_file_cache: false # File-backed cache
  cache_path: ""           # Cache file location

# Provider-specific overrides
providers:
  coingecko:
    ttl_seconds: 300       # Longer cache for price data
    sustained_rate: 0.5    # Respect 30 req/min free tier
    failure_thresh: 0.6    # More tolerant for free API
    
  binance:
    ttl_seconds: 60        # Shorter cache for market data
    sustained_rate: 10.0   # Higher rate for professional API
    failure_thresh: 0.3    # Less tolerant for reliable API
```

## Usage Examples

### Basic Usage

```go
import "github.com/cryptorun/internal/providers/adapters"
import "github.com/cryptorun/internal/providers/guards"

// Create adapter with guard configuration
config := guards.ProviderConfig{
    Name:           "coingecko",
    TTLSeconds:     300,
    BurstLimit:     10,
    SustainedRate:  0.5,
    MaxRetries:     3,
    BackoffBaseMs:  1000,
    FailureThresh:  0.6,
    WindowRequests: 5,
    ProbeInterval:  60,
    EnableFileCache: true,
    CachePath:      "artifacts/cache/coingecko.json",
}

adapter := adapters.NewCoinGeckoAdapter(config)

// Make guarded API call
ctx := context.Background()
prices, err := adapter.GetPrices(ctx, []string{"bitcoin", "ethereum"}, []string{"usd"})
if err != nil {
    log.Printf("API call failed: %v", err)
    return
}

// Response includes cache metadata
fmt.Printf("Cached: %v, Age: %v\n", prices.Cached, prices.Age)
```

### Cache Access

The `ProviderGuard` exposes its cache instance through the `Cache()` getter method for external access:

```go
// Access cache instance for custom operations
guard := guards.NewProviderGuard(config)
cache := guard.Cache()

// Generate cache keys for custom caching
cacheKey := cache.GenerateCacheKey("GET", "/api/endpoint", nil, nil)

// Add point-in-time headers
headers := make(map[string]string)
cache.AddPITHeaders(cacheKey, headers)
```

**Use Cases:**
- Custom cache key generation
- Point-in-time header management
- Cache invalidation operations
- Debug/monitoring cache state

### Health Monitoring

```go
// Check provider health
health := adapter.Health()
fmt.Printf("Provider: %s\n", health.Provider)
fmt.Printf("Circuit Open: %v\n", health.CircuitOpen)
fmt.Printf("Cache Hit Rate: %.2f%%\n", health.CacheHitRate * 100)
fmt.Printf("Error Rate: %.2f%%\n", health.ErrorRate * 100)
fmt.Printf("Avg Latency: %v\n", health.AvgLatency)
```

### Multi-Provider Management

```go
import "github.com/cryptorun/internal/providers/guards"

// Create multi-provider managers
rateLimiter := guards.NewMultiProviderRateLimiter()
circuitBreaker := guards.NewMultiProviderCircuitBreaker()
telemetry := guards.NewMultiProviderTelemetry()

// Add providers
rateLimiter.AddProvider("coingecko", coinGeckoConfig)
circuitBreaker.AddProvider("coingecko", coinGeckoConfig)
telemetry.AddProvider("coingecko")

// Check rate limits across providers
if !rateLimiter.Allow("coingecko") {
    log.Println("Rate limit exceeded for CoinGecko")
}

// Export telemetry
err := telemetry.ExportToCSV("artifacts/providers/telemetry.csv")
if err != nil {
    log.Printf("Failed to export telemetry: %v", err)
}
```

## Supported Providers

### CoinGecko
- **Purpose**: Price data and market information
- **Rate Limits**: 30 requests/minute (free tier)
- **Cache TTL**: 5 minutes (suitable for price data)
- **Tolerance**: High (60% failure threshold)

### Binance
- **Purpose**: Order book and market data
- **Rate Limits**: 1200 requests/minute (weight system)
- **Cache TTL**: 1 minute (fast-moving data)
- **Tolerance**: Low (30% failure threshold)

### OKX
- **Purpose**: Alternative market data source
- **Rate Limits**: Conservative 5 req/sec
- **Cache TTL**: 1.5 minutes
- **Tolerance**: Medium (40% failure threshold)

### Coinbase
- **Purpose**: Exchange-native order books
- **Rate Limits**: 3 requests/second (public)
- **Cache TTL**: 2 minutes
- **Tolerance**: Medium (40% failure threshold)

### Kraken (Preferred)
- **Purpose**: Primary exchange for USD pairs
- **Rate Limits**: 1 request/second (conservative)
- **Cache TTL**: 3 minutes (stable data)
- **Tolerance**: Very high (70% failure threshold)

## Error Handling

The Provider Guard system provides typed errors with retry guidance:

```go
if err != nil {
    if providerErr, ok := err.(*guards.ProviderError); ok {
        fmt.Printf("Provider: %s\n", providerErr.Provider)
        fmt.Printf("Status: %d\n", providerErr.StatusCode)
        fmt.Printf("Retryable: %v\n", providerErr.Retryable)
        if providerErr.RetryAfter > 0 {
            fmt.Printf("Retry After: %v\n", providerErr.RetryAfter)
        }
    }
}
```

### Error Types

1. **Rate Limit Exceeded**: Retryable with backoff
2. **Circuit Breaker Open**: Not retryable until probe succeeds
3. **HTTP 429/5xx**: Retryable with exponential backoff
4. **HTTP 4xx (non-429)**: Not retryable
5. **Network Errors**: Retryable with backoff

## Caching Strategy

### Cache Keys
Cache keys are generated from:
- HTTP method
- URL path and parameters
- Relevant headers (excluding auth)
- Provider name

### Point-in-Time Integrity
- **ETag**: Stored and sent as If-None-Match
- **Last-Modified**: Stored and sent as If-Modified-Since
- **Age Tracking**: Response age logged and monitored
- **Staleness Warnings**: Logged after 1 hour threshold

### File-Backed Caching
```yaml
providers:
  coingecko:
    enable_file_cache: true
    cache_path: "artifacts/cache/coingecko.json"
```

Benefits:
- Persistent across restarts
- Reduced cold-start latency
- Artifact generation for analysis

## Rate Limiting

### Token Bucket Algorithm
- **Burst Capacity**: Immediate requests available
- **Sustained Rate**: Long-term request rate
- **Token Refill**: Continuous based on sustained rate
- **Precision**: 100ms refill precision

### Configuration Examples
```yaml
# Conservative free tier
sustained_rate: 0.5    # 30 requests/minute
burst_limit: 10        # 10 immediate requests

# Professional API
sustained_rate: 10.0   # 600 requests/minute
burst_limit: 50        # 50 burst capacity
```

## Circuit Breakers

### States
1. **Closed**: Normal operation
2. **Open**: Requests blocked, failures detected
3. **Half-Open**: Probe requests allowed

### Transition Logic
```
Closed → Open: Failure rate > threshold with minimum requests
Open → Half-Open: After timeout period
Half-Open → Closed: On first success
Half-Open → Open: On any failure
```

### Configuration
```yaml
failure_thresh: 0.5      # 50% failure rate threshold
window_requests: 10      # Minimum 10 requests for rate calculation
probe_interval: 30       # 30 seconds until half-open
```

## Telemetry

### Collected Metrics
- **Cache**: Hits, misses, hit rate
- **Requests**: Total, successes, failures, errors
- **Latency**: Average, P50/P90/P95/P99 approximation
- **Rate Limits**: Hits, backoffs
- **Circuit Breakers**: Opens, state transitions

### Export Formats

#### CSV Export
```csv
provider,cache_hits,cache_misses,cache_hit_rate,requests,successes,failures,errors,error_rate,rate_limits,circuit_opens,backoffs,avg_latency_ms,last_success,last_failure,uptime_seconds
coingecko,245,12,95.33,257,242,15,0,5.84,3,0,8,156.23,2023-09-07T14:23:45Z,2023-09-07T13:45:12Z,3600
```

#### Log Format
```
PROVIDER_METRICS provider=coingecko cache_hit_rate=95.33 error_rate=5.84 avg_latency_ms=156.23 requests=257
```

## Integration Points

### Data Loaders
Provider guards integrate via thin adapters in data loading paths:

```go
// Hot path integration
func (loader *HotDataLoader) FetchTicker(symbol string) (*Ticker, error) {
    // Guard wraps the actual API call
    return loader.adapter.GetTicker(context.Background(), symbol)
}

// Warm path integration  
func (loader *WarmDataLoader) FetchHistoricalData(symbol string) (*History, error) {
    return loader.adapter.GetCandles(context.Background(), symbol, "1h", 100)
}
```

### Health Endpoints
Provider health is exposed via HTTP monitoring:

```bash
curl http://localhost:8080/health/providers

{
    "providers": {
        "coingecko": {
            "circuit_open": false,
            "cache_hit_rate": 0.95,
            "error_rate": 0.058,
            "avg_latency": "156ms"
        }
    }
}
```

## Testing

### Unit Tests
- Cache TTL expiration
- Token bucket refill rates
- Circuit breaker state transitions
- Backoff schedule calculation

### Integration Tests (Offline)
- Golden test simulation of 429/5xx streams
- Circuit breaker open/half-open/close verification
- Telemetry CSV artifact generation
- PIT header generation and validation

### Artifacts
Tests generate artifacts for validation:
```
artifacts/
├── providers/
│   ├── telemetry.csv      # Test telemetry export
│   └── SUMMARY.txt        # Configuration summary
└── cache/
    ├── coingecko.json     # Test cache files
    └── binance.json
```

## Performance Characteristics

### Latency Overhead
- **Cache Hit**: <1ms overhead
- **Cache Miss**: <10ms overhead
- **Rate Limit Check**: <1ms
- **Circuit Breaker Check**: <1ms

### Memory Usage
- **Per Provider**: ~1MB (1000 cache entries)
- **Total System**: <10MB for all providers
- **File Cache**: 10MB maximum per provider

### Throughput
- **No Limits**: 10,000+ req/sec theoretical
- **With Guards**: Limited by configured sustained rates
- **Burst Handling**: Full burst capacity immediately available

## Operational Guidelines

### Deployment
1. Configure provider-specific settings in `config/providers.yaml`
2. Ensure `artifacts/cache/` directory exists and is writable
3. Monitor telemetry exports for health validation
4. Set up log monitoring for circuit breaker events

### Monitoring
- **Cache Hit Rates**: Target >85% for stable operations
- **Error Rates**: Alert on >10% sustained error rates
- **Circuit Breaker Opens**: Investigate provider issues
- **Rate Limit Hits**: Consider increasing sustained rates

### Troubleshooting

#### High Error Rates
1. Check provider status pages
2. Review circuit breaker state
3. Validate rate limit configuration
4. Examine retry/backoff logs

#### Low Cache Hit Rates
1. Check TTL configuration appropriateness
2. Verify cache key generation
3. Review request patterns for cacheable data
4. Monitor file cache health

#### Circuit Breaker Issues
1. Review failure threshold settings
2. Check provider-specific tolerances
3. Validate probe interval configuration
4. Monitor recovery patterns

## Security Considerations

### Header Filtering
Sensitive headers are excluded from caching:
- `Authorization`
- `X-API-Key`
- `Cookie`
- `Set-Cookie`
- `X-Forwarded-For`

### HTTPS Enforcement
Only HTTPS schemes are allowed for external requests.

### Request Validation
- Maximum URL length: 2048 characters
- Maximum header size: 8192 bytes
- User agent identification: `CryptoRun/1.0`

---

**Generated**: Provider Guards v1.0 implementation  
**Status**: ✅ Production ready with comprehensive testing