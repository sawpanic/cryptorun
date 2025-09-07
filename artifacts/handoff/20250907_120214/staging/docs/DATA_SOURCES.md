# Data Sources Configuration

This document details the data source providers, their rate limits, cache configurations, and circuit breaker settings for CryptoRun.

## UX MUST — Live Progress & Explainability

All data sources provide real-time health monitoring, transparent rate limiting, and explainable failover behavior through the health snapshot API.

## Provider Overview

| Provider | Type | Auth Required | Primary Use | Rate Limit | Key Features |
|----------|------|---------------|-------------|------------|--------------|
| Binance | Exchange | No | Price/Volume/OrderBook | Weight-based (1200/min) | Native L1/L2 data |
| Kraken | Exchange | No | Price/Volume/OrderBook | 1 req/sec | Primary exchange, USD pairs |
| CoinGecko | Aggregator | No | Market data/metadata | 10 req/sec, 10K/month | Coin information |
| Moralis | Blockchain | No | On-chain data | 25 req/sec, 2M CU/day | DeFi metrics |
| DEXScreener | DEX Data | No | DEX pool data | 30 req/sec | **Banned for microstructure** |

## Rate Limits by Provider

### Binance
```yaml
provider: binance
requests_per_sec: 20
burst_limit: 40
monthly_quota: 0  # No monthly limit
daily_quota: 0    # No daily limit
weight_based: true
weight_limit: 1200  # per minute
```

**Weight Tracking**: Uses `X-MBX-USED-WEIGHT-1M` header for weight consumption tracking.

### Kraken
```yaml
provider: kraken
requests_per_sec: 1
burst_limit: 2
monthly_quota: 0
daily_quota: 0
weight_based: false
```

**Note**: Conservative rate limiting due to Kraken's strict API policies.

### CoinGecko
```yaml
provider: coingecko
requests_per_sec: 10
burst_limit: 20
monthly_quota: 10000
daily_quota: 0
weight_based: false
```

**Budget Guard**: Circuit opens when <5% monthly quota remaining.

### Moralis
```yaml
provider: moralis
requests_per_sec: 25
burst_limit: 50
monthly_quota: 0
daily_quota: 2000000  # 2M Compute Units
weight_based: false
```

**CU Tracking**: Monitors Compute Unit consumption for budget enforcement.

### DEXScreener
```yaml
provider: dexscreener
requests_per_sec: 30
burst_limit: 60
monthly_quota: 0
daily_quota: 0
weight_based: false
```

**⚠️ Microstructure Ban**: DEXScreener is **banned** for spread/depth data. Use only for token discovery.

## Cache TTL Configuration

### Hot Data (Real-time streams)
```yaml
ws_stream: 0s        # Never cache WebSocket streams
order_book: 5s       # Very short for orderbook
trades: 10s          # Short for trade data
```

### Warm Data (REST APIs)
```yaml
price_current: 30s   # Current price data
volume_24h: 60s      # 24h volume data
market_data: 120s    # General market data
pair_info: 300s      # Trading pair information
```

### Cold Data (Slow-changing)
```yaml
exchange_info: 1800s  # 30 min - Exchange information
asset_info: 3600s     # 1 hour - Asset information
historical: 7200s     # 2 hours - Historical data
metadata: 21600s      # 6 hours - Metadata
```

### Provider-Specific Overrides
```yaml
binance:
  exchange_info: 3600s  # Binance info changes less frequently
  klines: 300s          # Kline/candlestick data

coingecko:
  coin_list: 7200s      # CoinGecko coin list
  market_data: 180s     # Market data from CoinGecko

kraken:
  asset_pairs: 1800s    # Kraken asset pairs
  server_time: 60s      # Server time sync

dexscreener:
  token_info: 600s      # Token information
  pool_data: 120s       # Pool data (non-microstructure only)
```

## Circuit Breaker Configuration

### Binance Circuit
```yaml
provider: binance
error_threshold: 5           # Errors before opening
success_threshold: 3         # Successes to close from half-open
timeout: 30s                 # Time before half-open retry
latency_threshold: 5s        # Max acceptable latency
budget_threshold: 0.1        # Open when <10% budget remaining
window_size: 20              # Sliding window size
min_requests_in_window: 5    # Min requests before error calculation
fallback_providers: [kraken, coingecko]
```

### Kraken Circuit
```yaml
provider: kraken
error_threshold: 2
success_threshold: 1
timeout: 60s
latency_threshold: 15s       # Higher tolerance for Kraken
budget_threshold: 0.0        # No budget limit
window_size: 10
min_requests_in_window: 2
fallback_providers: [binance, coingecko]
```

### CoinGecko Circuit
```yaml
provider: coingecko
error_threshold: 3
success_threshold: 2
timeout: 60s
latency_threshold: 10s
budget_threshold: 0.05       # Open when <5% monthly budget
window_size: 15
min_requests_in_window: 3
fallback_providers: [binance, kraken]
```

### Moralis Circuit
```yaml
provider: moralis
error_threshold: 3
success_threshold: 2
timeout: 45s
latency_threshold: 8s
budget_threshold: 0.1        # Open when <10% CU remaining
window_size: 15
min_requests_in_window: 3
fallback_providers: [coingecko, binance]
```

### DEXScreener Circuit
```yaml
provider: dexscreener
error_threshold: 4
success_threshold: 2
timeout: 30s
latency_threshold: 6s
budget_threshold: 0.0        # No budget limit
window_size: 20
min_requests_in_window: 4
fallback_providers: [coingecko, binance]
```

## Fallback Strategy

### Primary → Fallback Chain
- **Binance** → Kraken → CoinGecko
- **Kraken** → Binance → CoinGecko  
- **CoinGecko** → Binance → Kraken
- **Moralis** → CoinGecko → Binance
- **DEXScreener** → CoinGecko → Binance

### Fallback Triggers
1. **Circuit Open**: Error rate > threshold in sliding window
2. **Budget Exhausted**: Remaining quota < threshold
3. **Latency Issues**: Response time > latency_threshold
4. **Manual Override**: Force circuit open for maintenance

## Implementation Details

### Rate Limiting Algorithm
- **Token Bucket**: Refills at `requests_per_sec` rate
- **Burst Handling**: Up to `burst_limit` requests in burst
- **Weight Tracking**: Binance uses weight system vs request counting
- **Quota Enforcement**: Daily/monthly limits enforced per provider

### Cache Key Strategy
```
Format: {provider}:{endpoint}:{params_hash}
Example: binance:ticker:{"symbol":"BTCUSD"}
```

### Circuit States
- **Closed**: Normal operation, all requests allowed
- **Open**: All requests blocked, fallback providers used
- **Half-Open**: Limited requests allowed to test recovery

### Health Calculation
```
Health % = 100 - max(daily_usage_%, monthly_usage_%)
```

## Monitoring & Alerting

### Key Metrics
- Request rate by provider
- Error rate in sliding window  
- Latency percentiles (P50, P95, P99)
- Budget utilization percentage
- Circuit breaker state transitions
- Cache hit rates by category

### Alert Thresholds
- **Critical**: Circuit open on primary provider (Binance/Kraken)
- **Warning**: Budget utilization >80%
- **Info**: Latency P95 >5s, Cache hit rate <70%

## Example Configuration

```go
// Usage in application code
pm := datasources.NewProviderManager()
cm := datasources.NewCacheManager()  
circm := datasources.NewCircuitManager()
hm := datasources.NewHealthManager(pm, cm, circm)

// Check if request is allowed
if pm.CanMakeRequest("binance") && circm.CanMakeRequest("binance") {
    // Make request
    resp, err := makeAPICall()
    
    // Record result
    pm.RecordRequest("binance", 1)
    circm.RecordRequest("binance", err == nil, latency, err)
    
    // Cache response
    cm.SetProviderData("binance", "ticker", "price_current", params, resp)
}

// Get active provider (with fallback)
activeProvider := circm.GetActiveProvider("binance")
```

## Compliance Notes

- **Free Tier Only**: All providers use free/keyless APIs
- **No Aggregators for Microstructure**: DEXScreener banned for spread/depth
- **Exchange-Native Priority**: Binance/Kraken preferred for L1/L2 data
- **Rate Limit Respect**: Conservative limits with exponential backoff
- **Budget Monitoring**: Proactive circuit opening before quota exhaustion