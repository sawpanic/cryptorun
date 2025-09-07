# Data Sources Configuration

This document details the data source providers, their rate limits, cache configurations, and circuit breaker settings for CryptoRun.

## UX MUST — Live Progress & Explainability

All data sources provide real-time health monitoring, transparent rate limiting, and explainable failover behavior through the health snapshot API.

## Provider Overview

| Provider | Type | Auth Required | Primary Use | Rate Limit | Key Features |
|----------|------|---------------|-------------|------------|--------------|
| Binance | Exchange | No | Price/Volume/OrderBook | Weight-based (1200/min) | Native L1/L2 data |
| Kraken | Exchange | No | Price/Volume/OrderBook/Trades | 1 req/sec | Primary exchange, USD pairs |
| OKX | Exchange | No | Derivatives/Futures | 15 req/sec | Perpetual futures, funding rates |
| CoinGecko | Aggregator | No | Market data/metadata | 10 req/sec, 10K/month | **FALLBACK ONLY** - Coin information |
| CoinPaprika | Aggregator | No | Market data/ticker | 100 req/min, 25K/day | **FALLBACK ONLY** - Market data |
| DEXScreener | DEX Data | No | DEX volume/events | 30 req/sec | **VOLUME ONLY** - Banned for microstructure |
| DeFiLlama | DeFi | No | Protocol TVL/metrics | 3 req/sec | DeFi protocol analytics |
| TheGraph | Blockchain | No | DeFi subgraph data | Variable | AMM pool data, on-chain metrics |

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

### OKX  
```yaml
provider: okx
requests_per_sec: 15
burst_limit: 30
monthly_quota: 0
daily_quota: 0
weight_based: false
```

**Exchange-Native**: Full microstructure access for derivatives data.

### CoinPaprika
```yaml
provider: coinpaprika
requests_per_sec: 100
burst_limit: 200
monthly_quota: 750000
daily_quota: 25000
weight_based: false
```

**⚠️ Fallback Only**: CoinPaprika is banned for microstructure data. Use only as fallback provider.

### DEXScreener
```yaml
provider: dexscreener
requests_per_sec: 30
burst_limit: 60
monthly_quota: 0
daily_quota: 0
weight_based: false
```

**⚠️ Volume Only**: DEXScreener is **banned** for spread/depth data. Use only for volume/events.

### DeFiLlama
```yaml
provider: defillama
requests_per_sec: 3
burst_limit: 10
monthly_quota: 0
daily_quota: 0
weight_based: false
```

**Protocol Analytics**: TVL, volume, and DeFi metrics (free tier).

### TheGraph
```yaml
provider: thegraph
requests_per_sec: 5
burst_limit: 15
monthly_quota: 0
daily_quota: 100000  # 100K queries
weight_based: false
```

**Subgraph Queries**: On-chain DeFi data via GraphQL (free hosted service).

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

okx:
  derivatives: 60s      # Derivatives metrics
  funding_rates: 300s   # Funding rate data
  
coinpaprika:
  coin_info: 1800s      # Coin information
  market_data: 300s     # Market data (fallback only)
  
dexscreener:
  token_info: 600s      # Token information
  volume_data: 120s     # Volume data (volume-only policy)
  events_data: 300s     # Event data (volume-only policy)
  
defillama:
  tvl_data: 1800s       # Protocol TVL metrics
  protocol_info: 3600s  # Protocol information
  
thegraph:
  subgraph_data: 300s   # Subgraph query results
  pool_metrics: 600s    # AMM pool metrics
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

### OKX Circuit
```yaml
provider: okx
error_threshold: 3
success_threshold: 2
timeout: 45s
latency_threshold: 8s
budget_threshold: 0.0        # No budget limit
window_size: 15
min_requests_in_window: 3
fallback_providers: [binance, kraken]
```

### CoinPaprika Circuit
```yaml
provider: coinpaprika
error_threshold: 4
success_threshold: 2
timeout: 60s
latency_threshold: 12s
budget_threshold: 0.1        # Open when <10% daily budget
window_size: 20
min_requests_in_window: 4
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
fallback_providers: [defillama, thegraph]  # DeFi-focused fallbacks
```

### DeFiLlama Circuit
```yaml
provider: defillama
error_threshold: 2
success_threshold: 1
timeout: 90s
latency_threshold: 15s
budget_threshold: 0.0        # No budget limit (free tier)
window_size: 10
min_requests_in_window: 2
fallback_providers: [thegraph, dexscreener]
```

### TheGraph Circuit
```yaml
provider: thegraph
error_threshold: 3
success_threshold: 2
timeout: 120s                # Longer timeout for complex queries
latency_threshold: 20s
budget_threshold: 0.15       # Open when <15% daily quota
window_size: 15
min_requests_in_window: 3
fallback_providers: [defillama, dexscreener]
```

## Fallback Strategy

### Primary → Fallback Chain

**Exchange-Native Data:**
- **Binance** → Kraken → OKX
- **Kraken** → Binance → OKX
- **OKX** → Binance → Kraken

**Market Data (Fallback Only):**
- **CoinGecko** → CoinPaprika → Binance
- **CoinPaprika** → CoinGecko → Kraken

**DeFi/On-Chain Data:**
- **DeFiLlama** → TheGraph → DEXScreener
- **TheGraph** → DeFiLlama → DEXScreener  
- **DEXScreener** → DeFiLlama → TheGraph

**Derivatives Data:**
- **OKX** → Binance → Kraken (spot equivalents)
- **Binance** → OKX (perpetuals only)

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
- **Aggregator Ban Enforcement**: 
  - DEXScreener **banned** for spread/depth data (volume/events only)
  - CoinGecko **fallback only** for market data (no microstructure)  
  - CoinPaprika **fallback only** for market data (no microstructure)
- **Exchange-Native Priority**: Binance/Kraken/OKX for L1/L2 data
- **DeFi Data Integration**: DeFiLlama/TheGraph for on-chain volume metrics
- **Derivatives Coverage**: OKX/Binance for funding rates, OI, basis calculations
- **Rate Limit Respect**: Conservative limits with exponential backoff
- **Budget Monitoring**: Proactive circuit opening before quota exhaustion
- **Volume Residual Enhancement**: On-chain DEX volume integrated into factor calculations

## API Endpoint Summary

### Exchange-Native (Microstructure Allowed)
- **Binance**: `api.binance.com` - L1/L2 data, trades, ticker
- **Kraken**: `api.kraken.com` - L1/L2 data, trades, ticker, server time
- **OKX**: `www.okx.com` - Derivatives, perpetuals, funding rates

### DeFi/On-Chain (Volume Only)
- **DeFiLlama**: `api.llama.fi` - Protocol TVL, DeFi metrics
- **TheGraph**: `api.thegraph.com` - Subgraph data, AMM pools
- **DEXScreener**: `api.dexscreener.com` - **Volume/Events Only** (microstructure banned)

### Fallback Aggregators (Market Data Only)
- **CoinGecko**: `api.coingecko.com` - **Fallback only** (microstructure banned)
- **CoinPaprika**: `api.coinpaprika.com` - **Fallback only** (microstructure banned)