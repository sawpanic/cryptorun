# API Integration Guide

## UX MUST — Live Progress & Explainability

Comprehensive API integration guide covering exchange-native data sources, rate limiting, circuit breakers, and provider health monitoring with real-time progress indicators and full explainability.

## Overview

CryptoRun integrates with multiple cryptocurrency data providers with a **free-first**, **exchange-native** approach for microstructure data and **aggregator-allowed** for warm/cold context data.

## Supported Providers

### Primary Exchange APIs (Free Tier)

#### Kraken (Primary)
- **Base URL**: `https://api.kraken.com`
- **WebSocket**: `wss://ws.kraken.com`
- **Rate Limit**: 60 requests/minute
- **Usage**: L1/L2 order book, trades, ticker, funding rates
- **Authentication**: None required (public endpoints)

```go
// Example configuration
kraken := &providers.KrakenProvider{
    APIBase:     "https://api.kraken.com",
    WSUrl:       "wss://ws.kraken.com", 
    RateLimit:   60, // per minute
    CircuitBreaker: circuitbreaker.New(5, time.Minute),
}
```

#### OKX
- **Base URL**: `https://www.okx.com/api/v5`
- **WebSocket**: `wss://ws.okx.com:8443/ws/v5/public`
- **Rate Limit**: 100 requests/minute
- **Usage**: Order book, trades, derivatives data

#### Coinbase Pro
- **Base URL**: `https://api.exchange.coinbase.com`
- **WebSocket**: `wss://ws-feed.exchange.coinbase.com`
- **Rate Limit**: 100 requests/minute
- **Usage**: Order book, trades, market data

#### Binance
- **Base URL**: `https://api.binance.com/api/v3`
- **WebSocket**: `wss://stream.binance.com:9443/ws`
- **Rate Limit**: 1200 requests/minute
- **Usage**: Derivatives data, funding rates, reference prices

### Aggregator APIs (Context Data Only)

#### CoinGecko (Free Tier)
- **Base URL**: `https://api.coingecko.com/api/v3`
- **Rate Limit**: 50 requests/minute
- **Usage**: Market cap, social data, catalyst events
- **Restrictions**: NOT used for order book or spread data

#### DefiLlama
- **Base URL**: `https://api.llama.fi`
- **Rate Limit**: No official limit
- **Usage**: TVL data, protocol metrics
- **Restrictions**: DeFi context only

### Data Source Hierarchy

```
Exchange-Native (Required for microstructure):
├── Kraken (Primary)
├── OKX (Secondary)  
├── Coinbase (Secondary)
└── Binance (Derivatives)

Aggregators (Context only):
├── CoinGecko (Social, Catalyst)
├── DefiLlama (DeFi metrics)
└── Other APIs (News, fundamentals)
```

## Rate Limiting Strategy

### Global Rate Limiting
```go
// Global rate limiter configuration
rateLimiter := &RateLimiter{
    GlobalLimit:     100, // requests per minute across all providers
    ProviderLimits: map[string]int{
        "kraken":   60,
        "okx":      100,
        "coinbase": 100,
        "binance":  1200,
        "coingecko": 50,
    },
}
```

### Provider-Specific Strategies

#### Weight-Based Limiting (Binance)
- Monitor `X-MBX-USED-WEIGHT` header
- Exponential backoff when approaching limits
- Circuit breaker at 80% weight usage

#### Time-Based Windows (Kraken)
- 60 requests per minute sliding window
- Request queuing with priority
- Fallback to cached data when limits hit

### Circuit Breaker Configuration

```yaml
# config/circuits.yaml
circuit_breakers:
  kraken:
    failure_threshold: 5
    recovery_timeout: 60s
    probe_timeout: 10s
    success_threshold: 2
  
  okx:
    failure_threshold: 3
    recovery_timeout: 30s
    success_threshold: 1
```

## Authentication

### Public Endpoints Only
CryptoRun operates exclusively on public, keyless API endpoints:

- ✅ **Public market data**: Order books, trades, ticker
- ✅ **Public derivatives**: Funding rates, open interest  
- ✅ **Public aggregated data**: Market cap, social metrics
- ❌ **Private endpoints**: Account data, trading operations

### Environment Variables
```bash
# Optional API base URLs (defaults provided)
KRAKEN_API_BASE=https://api.kraken.com
KRAKEN_WS_URL=wss://ws.kraken.com
OKX_API_BASE=https://www.okx.com/api/v5
COINBASE_API_BASE=https://api.exchange.coinbase.com

# Rate limiting configuration
GLOBAL_RATE_LIMIT=100
KRAKEN_RATE_LIMIT=60
```

## Error Handling & Resilience

### Circuit Breaker States
- **Closed**: Normal operation
- **Open**: Provider unavailable, using cached data
- **Half-Open**: Testing recovery with limited requests

### Fallback Strategies
1. **Cache First**: Check Redis cache before API calls
2. **Provider Fallback**: Switch to secondary provider
3. **Graceful Degradation**: Operate with reduced data set
4. **Circuit Recovery**: Automatic recovery testing

### Retry Logic
```go
retryConfig := RetryConfig{
    MaxRetries:    3,
    InitialDelay:  time.Second,
    MaxDelay:      30 * time.Second,
    Exponential:   true,
    Jitter:        true,
}
```

## Caching Strategy

### TTL Configuration
```yaml
# config/cache.yaml
cache:
  hot:
    ttl: 30s      # Real-time price data
    size: 1000
  warm:  
    ttl: 300s     # Technical indicators
    size: 5000
  cold:
    ttl: 3600s    # Historical data
    size: 10000
```

### Cache Keys
- `price:{venue}:{symbol}:{timestamp}`
- `book:{venue}:{symbol}:{level}`
- `funding:{venue}:{symbol}:{period}`

## Monitoring & Metrics

### Provider Health Metrics
```go
// Prometheus metrics
providerLatency := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "cryptorun_provider_latency_seconds",
        Help: "API response time by provider",
    },
    []string{"provider", "endpoint"},
)

errorRate := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "cryptorun_provider_errors_total", 
        Help: "API error count by provider and type",
    },
    []string{"provider", "error_type"},
)
```

### Health Check Endpoints
- `GET /health/providers` - Provider status overview
- `GET /metrics` - Prometheus metrics export
- `GET /debug/circuits` - Circuit breaker states

## Best Practices

### Exchange-Native Requirements
1. **Never use aggregators** for order book depth or spread data
2. **Always verify** data freshness and cross-venue consensus
3. **Implement fallbacks** for each critical data type
4. **Monitor rate limits** proactively

### Performance Optimization
1. **Batch requests** where possible
2. **Use WebSockets** for real-time data
3. **Cache aggressively** with appropriate TTLs
4. **Implement request prioritization**

### Security Considerations
1. **No API keys** stored in configuration
2. **TLS verification** for all HTTPS requests
3. **Input validation** for all API responses
4. **Rate limit compliance** to avoid IP blocking

## Troubleshooting

### Common Issues
1. **Rate Limit Exceeded**: Check provider limits and implement backoff
2. **Circuit Breaker Open**: Verify provider health and recovery logic  
3. **Stale Data**: Review cache TTLs and data freshness checks
4. **WebSocket Disconnects**: Implement reconnection with exponential backoff

### Debug Commands
```bash
# Check provider health
./cryptorun health --providers

# Test API connectivity  
./cryptorun test --provider kraken

# View circuit breaker status
curl localhost:8080/debug/circuits
```

## Integration Examples

### Kraken Order Book
```go
book, err := krakenProvider.GetOrderBook(ctx, "BTC-USD", 10)
if err != nil {
    return fmt.Errorf("kraken orderbook: %w", err)
}

spread := book.Spread()
depth := book.DepthUSD(0.02) // Within 2%
```

### Multi-Provider Funding Rates
```go
rates := make(map[string]float64)
for _, provider := range []string{"kraken", "okx", "binance"} {
    rate, err := getFundingRate(ctx, provider, "BTC-PERP")
    if err == nil {
        rates[provider] = rate
    }
}
```

For more detailed integration examples, see the provider implementation files in `internal/infrastructure/providers/`.
