# Provider Safety Layer

CryptoRun's provider safety infrastructure implements rate limiting, circuit breakers, and budget monitoring to ensure reliable operation within free-tier API limits.

## Architecture Overview

The safety layer consists of three core components:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Provider Safety Layer                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐ │
│  │  Rate Limiter   │    │ Circuit Breaker │    │ Budget Guard │ │
│  │                 │    │                 │    │              │ │
│  │ • Token bucket  │    │ • Error rate    │    │ • Monthly    │ │
│  │ • Exponential   │    │ • Latency P99   │    │ • Daily      │ │
│  │   backoff       │    │ • Consecutive   │    │ • Hourly     │ │
│  │ • Header        │    │   failures      │    │ • CU limits  │ │
│  │   parsing       │    │ • Auto recovery │    │              │ │
│  └─────────────────┘    └─────────────────┘    └──────────────┘ │
│           │                       │                       │     │
│           └───────────────────────┼───────────────────────┘     │
│                                   │                             │
│  ┌─────────────────────────────────────────────────────────────┤
│  │                   Fallback Chains                          │
│  └─────────────────────────────────────────────────────────────┤
│                                   │                             │
│  Market Data: Binance → Kraken → CoinGecko                     │
│  Token Metadata: CoinGecko → Moralis                           │
│  On-Chain: Moralis (no fallback)                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Rate Limiting

### Token Bucket Algorithm
- **Per-provider rate limits** with burst allowances
- **Exponential backoff** with jitter on rate limit violations
- **Header parsing** for dynamic limit adjustment (Retry-After, X-RateLimit-*)

### Provider Configurations
```yaml
providers:
  binance:
    rps: 10.0                    # 10 requests per second
    burst: 20                    # 20 request burst capacity
    weight_limit: 1200           # Weight limit per minute
    
  kraken:
    rps: 3.0                     # Conservative for API tiers
    burst: 5                     # Lower burst capacity
    tier_limit: 15               # API counter limit
    
  coingecko:
    rps: 3.0                     # Free tier: 10-50 calls/minute
    monthly_limit: 10000         # 10k calls/month free
    
  moralis:
    rps: 2.0                     # 40k CU/month free tier
    cu_per_call: 3               # Compute units per call
```

### Backoff Strategy
- **Base delay**: 1-5 seconds (provider-specific)
- **Multiplier**: 1.5-2.5x per retry
- **Max delay**: 30-300 seconds
- **Jitter**: ±25% to prevent thundering herd

## Circuit Breakers

### Trip Conditions
Circuit breakers open when any condition is met:

1. **Error Rate**: >15-30% error rate (provider-specific thresholds)
2. **Consecutive Failures**: 2-5 consecutive failures  
3. **Latency**: P99 latency >1-5 seconds
4. **Quota Exhaustion**: Monthly/daily limits reached

### State Management
- **Closed**: Normal operation, all requests allowed
- **Open**: All requests blocked, fallback chain activated
- **Half-Open**: Limited probe requests to test recovery

### Recovery Logic
```go
// Gradual recovery with increasing success thresholds
initial_success_threshold: 3      // Need 3 successes to close
success_threshold_increment: 1    // +1 on each failure
max_success_threshold: 10         // Max before giving up
```

## Budget Monitoring

### Multi-Tier Limits
Each provider has hierarchical budget limits:

- **Hourly**: Immediate protection (50-2000 calls/hour)
- **Daily**: Medium-term protection (500-20k calls/day)  
- **Monthly**: Long-term quota management (5k-4M calls/month)

### Budget Status
```go
type BudgetStatus struct {
    MonthlyUtilization float64  // Percentage of monthly quota used
    DailyUtilization   float64  // Daily quota utilization
    HourlyUtilization  float64  // Hourly quota utilization
    RemainingCalls     int      // Most restrictive limit remaining
    Status            string    // "ACTIVE", "WARNING", "LIMIT_REACHED"
}
```

### Enforcement Actions
- **80% utilization**: WARNING status, increase cache TTL
- **95% utilization**: Reduce request frequency
- **100% utilization**: LIMIT_REACHED, activate circuit breaker

## Fallback Chains

### Market Data Chain
```
Primary:   Binance (high capacity, exchange-native)
Secondary: Kraken (reliable, lower capacity)
Tertiary:  CoinGecko (aggregated data, free tier)
```

### Token Metadata Chain  
```
Primary:   CoinGecko (comprehensive metadata)
Secondary: Moralis (on-chain focus)
```

### On-Chain Data
```
Primary: Moralis (no fallbacks - specialized provider)
```

### Chain Behavior
- **Max attempts**: 0-2 fallbacks per chain
- **Timeout**: 10-30s per attempt
- **Abandonment**: After 1-3 total failures

## Provider Banner

The startup banner displays real-time provider health:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    🚀 CryptoRun Provider Health                     │
└─────────────────────────────────────────────────────────────────────┘

📊 System Overview: 🟢 EXCELLENT (4/4 providers active)
📈 Cache Hit Rate: 87.5%

Provider Status:
─────────────────────────────────────────────────────────────────────
Provider     │ Circuit  │ Rate Limit   │ Budget         │ Status
─────────────────────────────────────────────────────────────────────
binance      │ 🟢 OK    │ 🟢 LOW      │ 🟢 OK         │ 🟢 HEALTHY
kraken       │ 🟢 OK    │ 🟡 MED      │ 🟢 OK         │ 🟡 DEGRADED
coingecko    │ 🟢 OK    │ 🟢 LOW      │ 🟡 WARN       │ 🟡 DEGRADED
moralis      │ 🟢 OK    │ 🟢 LOW      │ 🟢 OK         │ 🟢 HEALTHY
─────────────────────────────────────────────────────────────────────

🕐 Status as of: 14:23:15 MST
```

### Health Indicators
- **🟢 GREEN**: Optimal operation
- **🟡 YELLOW**: Warning thresholds exceeded  
- **🔴 RED**: Critical issues or limits reached

## Integration with Scheduler

The safety layer integrates seamlessly with the scheduler:

```go
// Every job execution checks provider health
func (s *Scheduler) RunSignalsOnce(jobName string) error {
    // Display provider banner
    banner.DisplayStartupBanner()
    
    // Execute with safety middleware
    results, err := s.signalsScanner.ScanUniverse(scanType)
    
    // Generate health artifacts
    banner.WriteHealthJSON(healthPath)
    banner.WriteBannerText(bannerPath)
}
```

### Artifacts Generated
- **`health.json`**: Machine-readable provider status
- **`banner.txt`**: Human-readable startup banner

## Configuration

### Rate Limits (`config/rate_limits.yaml`)
```yaml
global:
  default_timeout_ms: 10000
  max_concurrent_requests: 50
  circuit_breaker_enabled: true
  budget_guard_enabled: true
```

### Circuit Breakers (`config/circuits.yaml`)  
```yaml
global:
  default_timeout: 30s
  default_max_requests: 5
  state_change_logging: true
```

### Budget Limits (`config/cache.yaml`)
```yaml
budgets:
  enabled: true
  alert_threshold: 0.8  # Alert at 80% utilization
```

## Monitoring & Alerting

### Metrics Collected
- **Request rates**: RPS by provider
- **Error rates**: Failure percentages  
- **Circuit state**: Open/closed/half-open counts
- **Budget usage**: Utilization by time window
- **Latency**: P50/P90/P99 percentiles

### Alert Conditions
```yaml
alerts:
  circuit_breaker_opened:
    severity: "warning"
    condition: "state == open"
    
  budget_exhaustion_warning:
    severity: "warning" 
    condition: "utilization > 80%"
    
  multiple_providers_down:
    severity: "critical"
    condition: "active_count < 2"
```

## UX MUST — Live Progress & Explainability

All provider operations provide:
- **Real-time status**: Live health indicators and utilization metrics
- **Attribution**: Clear provider sources and fallback chains used
- **Transparency**: Detailed error messages with recovery suggestions
- **Automation**: Self-healing with circuit breaker recovery and budget resets