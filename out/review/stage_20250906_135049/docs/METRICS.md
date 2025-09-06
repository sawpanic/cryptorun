# CryptoRun Metrics & Monitoring

CryptoRun provides comprehensive metrics collection with Prometheus integration, provider health tracking, and real-time monitoring capabilities.

## UX MUST — Live Progress & Explainability

All metrics provide transparency and explainability:
- **Real-time Visibility**: Live provider health, latency percentiles, success rates
- **Attribution**: Clear data sources and methodology for all metrics
- **Audit Trails**: Complete progress traces and metric collection history

## Provider Health Metrics

### Overview

The provider health system tracks the operational status of API providers (Kraken, OKX, Coinbase) with comprehensive metrics including success rates, latency percentiles, budget utilization, and degraded status.

### Core Metrics Schema

All provider health metrics use stable Prometheus naming with consistent labels:

#### `provider_health_success_rate`
**Type**: Gauge (0.0-1.0)  
**Labels**: `provider`, `venue`  
**Description**: Provider success rate over the last measurement window

```prometheus
# Example values
provider_health_success_rate{provider="kraken",venue="spot"} 0.95
provider_health_success_rate{provider="okx",venue="spot"} 0.98
```

#### `provider_health_latency_p50`
**Type**: Gauge (milliseconds)  
**Labels**: `provider`, `venue`  
**Description**: Provider 50th percentile latency

```prometheus
provider_health_latency_p50{provider="kraken",venue="spot"} 125.5
provider_health_latency_p50{provider="okx",venue="spot"} 89.2
```

#### `provider_health_latency_p95`
**Type**: Gauge (milliseconds)  
**Labels**: `provider`, `venue`  
**Description**: Provider 95th percentile latency

```prometheus
provider_health_latency_p95{provider="kraken",venue="spot"} 450.8
provider_health_latency_p95{provider="okx",venue="spot"} 298.1
```

#### `provider_health_budget_remaining`
**Type**: Gauge (0.0-1.0)  
**Labels**: `provider`, `venue`  
**Description**: Provider remaining budget/quota as percentage

```prometheus
provider_health_budget_remaining{provider="kraken",venue="spot"} 0.75
provider_health_budget_remaining{provider="okx",venue="spot"} 0.92
```

#### `provider_health_degraded`
**Type**: Gauge (0=healthy, 1=degraded)  
**Labels**: `provider`, `venue`, `reason`  
**Description**: Provider degraded status with reason

```prometheus
provider_health_degraded{provider="kraken",venue="spot",reason="rate_limit"} 1
provider_health_degraded{provider="okx",venue="spot",reason=""} 0
```

### Health Status Calculation

A provider is considered healthy when:
- Success rate > 90%
- Last success within 5 minutes
- Budget usage < 90%
- Not manually marked as degraded

### Implementation

#### MetricsRegistry

```go
// Create registry for multiple providers
registry := NewMetricsRegistry()

// Register providers
krakenHealth := NewProviderHealth("kraken")
okxHealth := NewProviderHealth("okx")
registry.RegisterProvider("kraken", krakenHealth)
registry.RegisterProvider("okx", okxHealth)

// Record request results
krakenHealth.RecordRequest(true, 150*time.Millisecond)
okxHealth.RecordRequest(false, 2*time.Second)

// Set budget information  
krakenHealth.SetBudget(750, 1000) // 75% remaining
okxHealth.SetBudget(920, 1000)    // 92% remaining

// Mark as degraded if needed
krakenHealth.SetDegraded(true, "rate_limit")

// Get health status
status := krakenHealth.GetStatus()
fmt.Printf("Health: %v, Success Rate: %.2f", status.IsHealthy, status.SuccessRate)
```

#### Metrics Export

```go
// Export all metrics for Prometheus
exports := registry.ExportMetrics()
for _, export := range exports {
    fmt.Printf("%s{%v} %v\n", export.Name, export.Labels, export.Value)
}

// Get Prometheus exposition format
prometheusMetrics := registry.GetPrometheusMetrics()
fmt.Println(prometheusMetrics)
```

## HTTP Monitoring Endpoints

### `/health`
**Method**: GET  
**Description**: Overall system health status

**Response Structure:**
```json
{
  "status": "healthy",
  "timestamp": "2025-09-06T13:14:14Z",
  "version": "v3.2.1",
  "providers": {
    "healthy_count": 2,
    "total_count": 3,
    "degraded": ["binance"]
  },
  "uptime_seconds": 3600
}
```

### `/metrics`
**Method**: GET  
**Content-Type**: `text/plain; version=0.0.4; charset=utf-8`  
**Description**: Prometheus metrics exposition format

**Sample Output:**
```prometheus
# HELP provider_health_success_rate Provider success rate over the last measurement window (0.0-1.0)
# TYPE provider_health_success_rate gauge
provider_health_success_rate{provider="kraken",venue="spot"} 0.95
provider_health_success_rate{provider="okx",venue="spot"} 0.98

# HELP provider_health_latency_p50 Provider 50th percentile latency in milliseconds
# TYPE provider_health_latency_p50 gauge
provider_health_latency_p50{provider="kraken",venue="spot"} 125.5
provider_health_latency_p50{provider="okx",venue="spot"} 89.2
```

### `/decile`
**Method**: GET  
**Description**: Performance decile analysis

**Response Structure:**
```json
{
  "performance_deciles": {
    "latency_ms": {
      "p10": 45.2,
      "p20": 67.8,
      "p50": 125.5,
      "p90": 389.1,
      "p95": 450.8,
      "p99": 892.3
    },
    "success_rates": {
      "p10": 0.91,
      "p50": 0.95,
      "p90": 0.98
    }
  },
  "providers": ["kraken", "okx", "coinbase"],
  "window_hours": 24
}
```

## Progress Streaming Metrics

### Scan Progress Events

Progress events are structured with metrics for analysis:

```json
{
  "timestamp": "2025-09-06T13:14:14+03:00",
  "phase": "analyze",
  "symbol": "BTCUSD",
  "status": "success", 
  "progress": 75,
  "total": 5,
  "current": 4,
  "metrics": {
    "momentum_score": 2.45,
    "qualified": true,
    "processing_time_ms": 245,
    "guards_passed": ["fatigue_guard", "freshness_guard"],
    "confidence": 78.5
  }
}
```

### QA Progress Events

QA phases emit detailed metrics:

```json
{
  "event": "qa_phase",
  "timestamp": "2025-09-06T13:14:14Z",
  "phase": 2,
  "name": "Provider Health Check",
  "status": "pass",
  "duration": 1250,
  "metrics": {
    "providers_tested": 3,
    "healthy_providers": 2,
    "degraded_providers": 1,
    "avg_latency_ms": 156.7,
    "success_rate": 0.94
  }
}
```

## Monitoring Integration

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'cryptorun'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s
    metrics_path: '/metrics'
```

### Grafana Dashboard

Key panels for CryptoRun monitoring:

1. **Provider Health Overview**
   ```promql
   provider_health_success_rate
   ```

2. **Latency Percentiles**
   ```promql
   provider_health_latency_p50
   provider_health_latency_p95
   ```

3. **Budget Utilization**
   ```promql
   (1 - provider_health_budget_remaining) * 100
   ```

4. **Degraded Providers**
   ```promql
   provider_health_degraded == 1
   ```

### Alerting Rules

```yaml
# alerts.yml
groups:
  - name: cryptorun_alerts
    rules:
      - alert: ProviderDegraded
        expr: provider_health_degraded == 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Provider {{ $labels.provider }} degraded"
          description: "{{ $labels.provider }} has been degraded for {{ $labels.reason }}"

      - alert: HighLatency
        expr: provider_health_latency_p95 > 1000
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High latency on {{ $labels.provider }}"
          description: "P95 latency is {{ $value }}ms"

      - alert: LowSuccessRate  
        expr: provider_health_success_rate < 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low success rate on {{ $labels.provider }}"
          description: "Success rate is {{ $value | humanizePercentage }}"
```

## Performance Metrics

### Scan Performance

Key metrics tracked during momentum scanning:

- **Symbols per Second**: Processing throughput
- **Average Analysis Time**: Time per symbol analysis  
- **Orthogonalization Time**: Gram-Schmidt processing duration
- **Cache Hit Rate**: Data cache effectiveness
- **Memory Usage**: Peak memory consumption during scans

### QA Performance

QA suite performance tracking:

- **Phase Completion Times**: Duration of each QA phase
- **Provider Response Times**: API latency during testing
- **Test Coverage**: Percentage of codebase tested
- **Error Rates**: Test failure percentages by category

## Metrics Best Practices

### Collection Guidelines

1. **High-Frequency Metrics**: Success rates, latencies (every request)
2. **Medium-Frequency Metrics**: Budget status, degraded state (every minute)
3. **Low-Frequency Metrics**: Configuration changes, version info (on startup)

### Label Cardinality

- **Provider Labels**: Limited to supported exchanges (kraken, okx, coinbase)
- **Venue Labels**: Currently only "spot" trading
- **Reason Labels**: Controlled vocabulary for degraded reasons

### Retention Policies

- **Raw Metrics**: 7 days high resolution
- **Aggregated Metrics**: 30 days medium resolution  
- **Historical Analysis**: 1 year low resolution

### Alert Thresholds

- **Success Rate**: Alert if < 90% for 5+ minutes
- **Latency P95**: Alert if > 1000ms for 2+ minutes
- **Budget Usage**: Alert if > 90% utilized
- **Degraded State**: Immediate alert on any degraded provider

## Troubleshooting

### Common Issues

**High Latency Alerts**
1. Check provider status pages
2. Verify network connectivity  
3. Review rate limiting status
4. Check circuit breaker states

**Low Success Rate Alerts**  
1. Examine error logs in progress traces
2. Check API endpoint health
3. Verify authentication/authorization
4. Review recent configuration changes

**Missing Metrics**
1. Verify monitoring server is running (`cryptorun monitor`)
2. Check endpoint accessibility (`curl http://localhost:8080/metrics`)
3. Review Prometheus scraping configuration
4. Validate provider registration in metrics registry

### Debug Commands

```bash
# Check monitoring server
curl http://localhost:8080/health

# Get raw metrics
curl http://localhost:8080/metrics

# View progress traces
tail -f out/audit/progress_trace.jsonl | jq .

# Run QA with metrics
cryptorun qa --progress json | jq 'select(.metrics)'
```