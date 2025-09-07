# Operations Health Monitoring

This document describes the health monitoring system for CryptoRun data sources, including health snapshot format, interpretation, and operational procedures.

## UX MUST — Live Progress & Explainability

The health system provides real-time visibility into all data source components with transparent metrics, clear status indicators, and explainable failover decisions.

## Health Snapshot Overview

The health snapshot provides a comprehensive view of all datasource components in JSON format accessible via the health API or CLI.

### Snapshot Structure

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "overall_health": "healthy",
  "providers": {
    "binance": { /* Provider health details */ },
    "kraken": { /* Provider health details */ },
    "coingecko": { /* Provider health details */ }
  },
  "cache": { /* Cache system health */ },
  "circuits": {
    "binance": { /* Circuit breaker status */ },
    "kraken": { /* Circuit breaker status */ }
  },
  "summary": { /* High-level metrics */ }
}
```

## Provider Health Details

### Provider Status Fields

Each provider includes the following health metrics:

```json
{
  "name": "Binance",
  "status": "healthy",           // healthy | degraded | unhealthy
  "requests_today": 1247,
  "requests_month": 45892,
  "daily_quota": 0,             // 0 = no limit
  "monthly_quota": 0,           // 0 = no limit  
  "health_percent": 100.0,      // Budget remaining %
  "weight_used": 156,           // Binance-specific weight
  "latency": {
    "p50": "150ms",
    "p95": "300ms", 
    "p99": "500ms",
    "max": "1.2s",
    "avg": "200ms"
  },
  "cost": 0.0,                  // Free APIs
  "last_request": "2024-01-15T10:29:45Z",
  "circuit_state": "closed"
}
```

### Status Interpretation

| Status | Condition | Action Required |
|--------|-----------|----------------|
| `healthy` | Circuit closed, health >50% | None |
| `degraded` | Circuit half-open OR health 10-50% | Monitor closely |
| `unhealthy` | Circuit open OR health <10% | Check provider, use fallbacks |

### Health Percentage Calculation

```
Health % = 100 - max(daily_usage_%, monthly_usage_%)

Where:
- daily_usage_% = (requests_today / daily_quota) * 100
- monthly_usage_% = (requests_month / monthly_quota) * 100
```

**Special Cases**:
- If quota = 0 (unlimited): usage_% = 0
- If quota exceeded: health_% = 0

## Cache System Health

### Cache Health Fields

```json
{
  "status": "healthy",          // healthy | degraded
  "total_entries": 1524,
  "active_entries": 1203,       // Non-expired entries
  "expired_entries": 321,
  "hit_rate_percent": 85.2      // Cache effectiveness
}
```

### Cache Status Logic

- **Healthy**: `expired_entries <= active_entries`
- **Degraded**: `expired_entries > active_entries`

## Circuit Breaker Health

### Circuit Health Fields

```json
{
  "provider": "binance",
  "state": "closed",            // closed | open | half-open
  "error_rate_percent": 2.1,
  "avg_latency": "180ms",
  "max_latency": "850ms",
  "last_failure": "2024-01-15T09:15:20Z"
}
```

### Circuit States

| State | Meaning | Request Behavior |
|-------|---------|------------------|
| `closed` | Normal operation | All requests allowed |
| `open` | Provider failing | All requests blocked, use fallbacks |
| `half-open` | Testing recovery | Limited requests to test health |

## Health Summary Metrics

### Summary Fields

```json
{
  "providers_healthy": 4,       // Count of healthy providers
  "providers_total": 5,         // Total providers configured
  "circuits_closed": 4,         // Count of closed circuits
  "circuits_total": 5,          // Total circuits
  "overall_latency_p99": "500ms",
  "cache_hit_rate": 85.2,
  "budget_utilization_percent": 15.3
}
```

### Overall Health Calculation

The system overall health is determined by:

```
Healthy if:
- >50% providers are healthy AND
- >50% circuits are closed AND  
- Cache hit rate >70% AND
- P99 latency <10s

Degraded if:
- >30% providers are healthy AND
- >30% circuits are closed AND
- Cache hit rate >50%

Otherwise: Unhealthy
```

## Health API Endpoints

### Full Health Snapshot

```bash
GET /health/snapshot
```

Returns complete health snapshot as JSON.

### Health Summary

```bash  
GET /health/summary
```

Returns brief text summary:
```
Health: healthy | Providers: 4/5 healthy | Circuits: 4/5 closed | Cache: healthy (85.2% hit rate) | Latency P99: 500ms
```

### Provider-Specific Health

```bash
GET /health/providers/{provider}
```

Returns detailed health for specific provider.

## CLI Health Commands

### Health Snapshot

```bash
./cryptorun health snapshot
```

Outputs formatted health snapshot to console.

### Health Summary

```bash
./cryptorun health summary  
```

Outputs brief health summary.

### Watch Mode

```bash
./cryptorun health watch
```

Continuously monitors and displays health changes.

## Operational Procedures

### Daily Health Checks

1. **Review Overall Health**
   ```bash
   ./cryptorun health summary
   ```

2. **Check Budget Utilization**
   - Warning if any provider >80% budget used
   - Critical if any provider >95% budget used

3. **Verify Circuit States**
   - All circuits should be `closed` under normal conditions
   - Investigate any `open` or `half-open` circuits

### Incident Response

#### Provider Down

If provider status shows `unhealthy`:

1. **Check Circuit State**
   ```bash
   ./cryptorun health providers binance
   ```

2. **Review Error Patterns**
   - Check `error_rate_percent`
   - Review `last_failure` timestamp

3. **Verify Fallback Operation**
   ```bash
   # System should automatically use fallback providers
   ./cryptorun scan --dry-run
   ```

#### High Latency

If P99 latency >5s:

1. **Identify Slow Provider**
   ```bash
   ./cryptorun health snapshot | grep -A5 latency
   ```

2. **Check Circuit Configuration**
   - Verify `latency_threshold` settings
   - Consider lowering thresholds for problem providers

#### Low Cache Hit Rate

If cache hit rate <70%:

1. **Review TTL Configuration**
   - Check if TTLs are too short for data patterns
   - Consider increasing TTLs for stable data

2. **Monitor Cache Size**
   - Verify cache isn't being cleared too frequently
   - Check `expired_entries` vs `active_entries`

### Budget Management

#### Monthly Quota Monitoring

For providers with monthly limits (CoinGecko):

1. **Daily Budget Check**
   ```bash
   ./cryptorun health providers coingecko
   ```

2. **Proactive Circuit Opening**
   - System automatically opens circuit at <5% remaining
   - Manually open circuit if approaching limit:
   ```bash
   ./cryptorun circuit open coingecko
   ```

#### Daily Quota Monitoring  

For providers with daily limits (Moralis):

1. **Hourly CU Check**
   ```bash
   ./cryptorun health providers moralis
   ```

2. **Usage Pattern Analysis**
   - Monitor peak usage hours
   - Adjust request patterns if approaching limits

### Performance Tuning

#### Latency Optimization

1. **Review Provider Latencies**
   ```bash
   ./cryptorun health snapshot --format=table
   ```

2. **Adjust Circuit Thresholds**
   - Lower `latency_threshold` for consistently fast providers
   - Raise thresholds for inherently slower providers

#### Cache Optimization

1. **Analyze Hit Rates by Category**
   ```bash
   ./cryptorun cache stats --by-category
   ```

2. **Tune TTL Values**
   - Increase TTLs for stable data (exchange info, asset info)
   - Decrease TTLs for volatile data (prices, volumes)

## Alerting Configuration

### Critical Alerts

- Provider circuit open on primary exchanges (Binance, Kraken)
- Overall health = `unhealthy`
- Cache hit rate <50%

### Warning Alerts

- Provider budget utilization >80%
- P95 latency >5s for any provider
- Circuit in `half-open` state >10 minutes

### Info Alerts

- Provider status = `degraded`
- Cache hit rate 50-70%
- Any circuit state change

## Monitoring Integration

### Metrics Export

Health metrics are exported for monitoring systems:

```bash
# Prometheus metrics endpoint
GET /metrics

# Key metrics exported:
# - cryptorun_provider_health_percent
# - cryptorun_circuit_state  
# - cryptorun_request_rate
# - cryptorun_error_rate
# - cryptorun_latency_p95
# - cryptorun_cache_hit_rate
```

### Log Aggregation

Health events are logged for analysis:

```json
{
  "level": "warn",
  "provider": "coingecko", 
  "event": "budget_threshold",
  "health_percent": 8.2,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Health Check Examples

### All Systems Healthy

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "overall_health": "healthy",
  "summary": {
    "providers_healthy": 5,
    "providers_total": 5,
    "circuits_closed": 5, 
    "circuits_total": 5,
    "overall_latency_p99": "400ms",
    "cache_hit_rate": 87.5,
    "budget_utilization_percent": 23.1
  }
}
```

### Degraded System (CoinGecko Budget Low)

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "overall_health": "degraded",
  "providers": {
    "coingecko": {
      "name": "CoinGecko",
      "status": "degraded",
      "health_percent": 8.2,
      "circuit_state": "open"
    }
  },
  "circuits": {
    "coingecko": {
      "state": "open",
      "provider": "coingecko"
    }
  }
}
```

### Unhealthy System (Multiple Providers Down)

```json
{
  "timestamp": "2024-01-15T10:30:00Z", 
  "overall_health": "unhealthy",
  "summary": {
    "providers_healthy": 1,
    "providers_total": 5,
    "circuits_closed": 1,
    "circuits_total": 5,
    "overall_latency_p99": "15s"
  }
}
```