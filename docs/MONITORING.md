# CryptoRun Monitoring Guide

## UX MUST — Live Progress & Explainability

Complete monitoring and observability guide for CryptoRun v3.2.1 covering metrics, dashboards, alerts, and troubleshooting procedures with hardened alerting thresholds.

## Overview

CryptoRun provides comprehensive monitoring through:

- **Prometheus metrics** for time-series data collection
- **Grafana dashboards** for visualization and analysis
- **Health endpoints** for service status monitoring
- **Structured logging** for debugging and audit trails
- **Alert management** for proactive issue detection

## Metrics Reference

### System Health Metrics

```prometheus
# Overall system health (0.0 - 1.0)
cryptorun_system_health{instance="localhost:8080"}

# Active scan operations
cryptorun_active_scans{instance="localhost:8080"}

# Total scans completed
cryptorun_scans_total{exchange="kraken",status="success"} counter

# Current market regime (0=choppy, 1=bull, 2=high-vol)
cryptorun_active_regime{instance="localhost:8080"}
```

### Performance Metrics

```prometheus
# Portfolio performance
cryptorun_portfolio_value{currency="USD",instance="localhost:8080"}
cryptorun_performance_sharpe_ratio{strategy="momentum",instance="localhost:8080"}
cryptorun_performance_hit_rate{strategy="momentum",instance="localhost:8080"}
cryptorun_performance_max_drawdown{strategy="momentum",instance="localhost:8080"}

# P&L tracking
cryptorun_pnl_total{currency="USD",type="net",instance="localhost:8080"}
cryptorun_pnl_unrealized{currency="USD",instance="localhost:8080"}
```

### Provider Health Metrics

```prometheus
# Provider availability (0=unhealthy, 1=healthy)
cryptorun_provider_health{provider="kraken",instance="localhost:8080"}

# Rate limiting status
cryptorun_provider_rate_limit{provider="kraken",instance="localhost:8080"}

# Provider response times
cryptorun_provider_latency_seconds{provider="kraken",quantile="0.95"}
cryptorun_provider_latency_seconds{provider="kraken",quantile="0.50"}

# API error rates
cryptorun_provider_errors_total{provider="kraken",error_type="timeout"}

# Kraken venue health metrics (legacy from original)
cryptorun_kraken_reject_rate{instance="localhost:8080"}
cryptorun_kraken_heartbeat_gap_seconds{instance="localhost:8080"}
cryptorun_kraken_error_rate{instance="localhost:8080"}
cryptorun_kraken_latency_p99_seconds{instance="localhost:8080"}
```

### Cache Performance

```prometheus
# Cache hit ratio (0.0 - 1.0)
cryptorun_cache_hit_ratio{cache_type="hot",instance="localhost:8080"}

# Cache operations
cryptorun_cache_hits_total{cache_type="hot",instance="localhost:8080"} counter
cryptorun_cache_misses_total{cache_type="hot",instance="localhost:8080"} counter
cryptorun_cache_size_bytes{cache_type="hot",instance="localhost:8080"}

# API usage budgets
cryptorun_api_usage_budget{provider="kraken",instance="localhost:8080"}
```

### Pipeline Metrics

```prometheus
# Step execution times
cryptorun_step_duration_seconds{step="data_collection",quantile="0.95"}
cryptorun_step_duration_seconds{step="scoring",quantile="0.50"}
cryptorun_step_duration_seconds{step="gating",quantile="0.95"}

# Pipeline error rates
cryptorun_pipeline_errors_total{step="data_collection",error_type="provider_timeout"}
```

### Circuit Breaker Metrics

```prometheus
# Circuit breaker states (0=closed, 1=open, 2=half-open)
cryptorun_circuit_breaker_state{provider="kraken",instance="localhost:8080"}

# Probe recovery attempts
cryptorun_circuit_breaker_probes_total{provider="kraken",result="success"}
cryptorun_circuit_breaker_probes_total{provider="kraken",result="failure"}
```

### Streaming Metrics

```prometheus
# SSE connections
cryptorun_sse_connections{stream="scans",instance="localhost:8080"}

# Message throughput
cryptorun_sse_throughput_total{stream="scans",instance="localhost:8080"} counter
```

### Data Quality Metrics

```prometheus
# Cross-venue consensus (0.0 - 1.0)
cryptorun_data_consensus{symbol="BTC-USD",instance="localhost:8080"}

# Data freshness (seconds since last update)
cryptorun_data_freshness_seconds{source="kraken",symbol="BTC-USD"}
```

## Grafana Dashboard Configuration

### 1. Overview Dashboard

The main CryptoRun dashboard (`deploy/grafana/cryptorun-overview-dashboard.json`) provides:

- **System Health Overview**: Real-time health status with color-coded thresholds
- **Active Scans**: Current scanning operations and hourly scan rates
- **Current Regime**: Market regime indicator with bull/choppy/volatile status
- **Portfolio Value**: Real-time portfolio valuation in USD
- **Performance Metrics**: Sharpe ratio and hit rate trends
- **Provider Health**: Exchange connection status table
- **Cache Performance**: Hit ratios and operation rates
- **Pipeline Durations**: P95 and P50 execution times by step
- **Provider Latency**: API response time monitoring with SLA thresholds
- **Error Rates**: Pipeline and provider error tracking
- **SSE Connections**: WebSocket connection monitoring
- **Data Quality**: Consensus and freshness metrics

### 2. Circuit Breaker Dashboard

```json
{
  "title": "Circuit Breaker Status",
  "type": "stat",
  "targets": [
    {
      "expr": "cryptorun_circuit_breaker_state{instance=~\"$instance\"}",
      "legendFormat": "{{provider}}"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "mappings": [
        {"options": {"0": {"text": "Closed", "color": "green"}}, "type": "value"},
        {"options": {"1": {"text": "Open", "color": "red"}}, "type": "value"},
        {"options": {"2": {"text": "Half-Open", "color": "yellow"}}, "type": "value"}
      ]
    }
  }
}
```

### 3. Dashboard Variables

```yaml
# Instance selector
- name: instance
  type: query
  query: label_values(cryptorun_system_health, instance)
  includeAll: true
  allValue: ".*"

# Exchange selector  
- name: exchange
  type: query
  query: label_values(cryptorun_provider_health, provider)
  includeAll: true
  allValue: ".*"

# Strategy selector
- name: strategy
  type: query
  query: label_values(cryptorun_performance_sharpe_ratio, strategy)
  includeAll: true
  allValue: ".*"
```

### 4. Custom Dashboard Panels

```json
{
  "title": "Top Performers",
  "type": "table",
  "targets": [
    {
      "expr": "topk(10, cryptorun_performance_sharpe_ratio{instance=~\"$instance\"})",
      "format": "table",
      "instant": true
    }
  ]
}
```

## Alerting Configuration

### 1. Critical Alerts

```yaml
# System health degraded
- alert: CryptoRunSystemUnhealthy
  expr: cryptorun_system_health < 0.8
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "CryptoRun system health below 80%"
    description: "Instance {{ $labels.instance }} health is {{ $value | humanizePercentage }}"

# High error rate
- alert: CryptoRunHighErrorRate
  expr: rate(cryptorun_pipeline_errors_total[5m]) > 0.1
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "High error rate detected"
    description: "Pipeline errors: {{ $value | humanize }} errors/second"

# Circuit breaker open
- alert: CryptoRunCircuitBreakerOpen
  expr: cryptorun_circuit_breaker_state == 1
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Circuit breaker open for provider {{ $labels.provider }}"
    description: "Provider {{ $labels.provider }} circuit breaker has been open for 1 minute"
```

### 2. Performance Alerts

```yaml
# Poor hit rate
- alert: CryptoRunLowHitRate
  expr: cryptorun_performance_hit_rate < 0.6
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Hit rate below 60%"
    description: "Strategy {{ $labels.strategy }} hit rate: {{ $value | humanizePercentage }}"

# High drawdown
- alert: CryptoRunHighDrawdown
  expr: cryptorun_performance_max_drawdown > 0.15
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Maximum drawdown exceeded 15%"
    description: "Current drawdown: {{ $value | humanizePercentage }}"

# Kraken venue health degraded
- alert: CryptoRunKrakenUnhealthy
  expr: cryptorun_kraken_error_rate > 0.05 or cryptorun_kraken_latency_p99_seconds > 2.0
  for: 3m
  labels:
    severity: warning
  annotations:
    summary: "Kraken venue health degraded"
    description: "Kraken error rate or P99 latency exceeded thresholds"
```

### 3. Infrastructure Alerts

```yaml
# Provider outage
- alert: CryptoRunProviderDown
  expr: cryptorun_provider_health == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Provider {{ $labels.provider }} is unhealthy"
    description: "Exchange API provider {{ $labels.provider }} has been down for 1 minute"

# Cache degradation
- alert: CryptoRunLowCacheHitRatio
  expr: cryptorun_cache_hit_ratio < 0.7
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Cache hit ratio below 70%"
    description: "Cache {{ $labels.cache_type }} hit ratio: {{ $value | humanizePercentage }}"

# API budget exhaustion
- alert: CryptoRunAPIBudgetLow
  expr: cryptorun_api_usage_budget < 0.2
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "API usage budget below 20%"
    description: "Provider {{ $labels.provider }} API budget: {{ $value | humanizePercentage }}"
```

### 4. Alert Handlers

```yaml
# Slack notifications
slack_configs:
  - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
    channel: '#cryptorun-alerts'
    title: 'CryptoRun Alert'
    text: '{{ .CommonAnnotations.summary }}\n{{ .CommonAnnotations.description }}'

# Email notifications
email_configs:
  - to: 'ops-team@yourdomain.com'
    from: 'alerts@yourdomain.com'
    subject: 'CryptoRun Alert: {{ .GroupLabels.alertname }}'
    body: |
      Alert: {{ .GroupLabels.alertname }}
      
      {{ range .Alerts }}
      Instance: {{ .Labels.instance }}
      Summary: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

## Health Endpoints

### 1. Health Check Endpoint

```bash
# Basic health check
curl http://localhost:8080/health

# JSON response format
{
  "overall": "healthy",
  "timestamp": "2025-09-07T14:30:00Z",
  "components": {
    "database": {
      "status": "healthy",
      "latency_ms": 12,
      "last_check": "2025-09-07T14:29:58Z"
    },
    "cache": {
      "status": "healthy",
      "hit_ratio": 0.87,
      "last_check": "2025-09-07T14:29:59Z"
    },
    "exchanges": {
      "kraken": {
        "status": "healthy",
        "latency_ms": 145,
        "rate_limit_remaining": 85,
        "circuit_breaker_state": "closed"
      }
    }
  }
}
```

### 2. CLI Health Command

```bash
# Comprehensive health check
./cryptorun health --json --timeout 30s

# Text output format
./cryptorun health --format text
```

### 3. Kubernetes Health Probes

```yaml
# Liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 3

# Readiness probe
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 3
```

## Troubleshooting Procedures

### 1. Circuit Breaker Issues

```bash
# Check circuit breaker status
curl -s localhost:8081/metrics | grep cryptorun_circuit_breaker_state

# Force circuit breaker reset (if available)
curl -X POST localhost:8080/admin/circuit-breaker/reset?provider=kraken

# Check probe recovery progress
curl -s localhost:8081/metrics | grep cryptorun_circuit_breaker_probes
```

### 2. High Latency Investigation

```bash
# Check provider latencies
curl -s localhost:8081/metrics | grep cryptorun_provider_latency | grep quantile

# Check Kraken-specific metrics
curl -s localhost:8081/metrics | grep cryptorun_kraken_latency_p99

# Analyze slow operations
tail -f /var/log/cryptorun/app.log | jq 'select(.duration_ms > 500)'

# Check cache performance
curl -s localhost:8081/metrics | grep cryptorun_cache_hit_ratio
```

### 3. Exchange Connectivity

```bash
# Check provider health
curl -s localhost:8080/health | jq '.components.exchanges'

# Analyze rate limiting
curl -s localhost:8081/metrics | grep cryptorun_provider_rate_limit

# Check API budget usage
curl -s localhost:8081/metrics | grep cryptorun_api_usage_budget

# Test direct connectivity
kubectl exec deployment/cryptorun -- curl -s https://api.kraken.com/0/public/SystemStatus
```

### 4. Performance Degradation

```bash
# Check hit rate trends
curl -s localhost:8081/metrics | grep cryptorun_performance_hit_rate

# Analyze drawdown levels
curl -s localhost:8081/metrics | grep cryptorun_performance_max_drawdown

# Check venue-specific error rates
curl -s localhost:8081/metrics | grep cryptorun_kraken_error_rate
```

## Performance Optimization

### 1. Cache Tuning

```yaml
# Optimize cache TTLs based on data patterns
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

### 2. Circuit Breaker Tuning

```yaml
# Adjust circuit breaker thresholds
circuit_breakers:
  kraken:
    failure_threshold: 5      # Open after 5 failures
    recovery_timeout: 60s     # Stay open for 60s
    probe_timeout: 10s        # Probe timeout
    success_threshold: 2      # Close after 2 successes
```

### 3. Provider Optimization

```yaml
# Optimize API usage patterns
providers:
  kraken:
    rate_limit: 100           # Requests per minute
    batch_size: 10            # Batch multiple symbols
    retry_backoff: exponential
    timeout: 30s
```
