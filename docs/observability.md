# CryptoRun Observability Guide

This guide covers the comprehensive observability stack for CryptoRun, including health monitoring, metrics collection, visualization, and alerting.

## Overview

The CryptoRun observability stack consists of:

- **Health Endpoints**: `/health`, `/ready`, `/live` for service status monitoring
- **Metrics Endpoints**: `/metrics` with Prometheus-format metrics
- **Grafana Dashboard**: Real-time visualization of system performance
- **Prometheus**: Time-series metrics collection and storage
- **AlertManager**: Alert routing and notification management

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  CryptoRun  │    │ Prometheus  │    │   Grafana   │
│   Service   │───▶│   Server    │───▶│  Dashboard  │
│             │    │             │    │             │
└─────────────┘    └─────────────┘    └─────────────┘
      │                     │                │
      │                     ▼                │
      │            ┌─────────────┐           │
      │            │ AlertManager│◀──────────┘
      │            │             │
      ▼            └─────────────┘
┌─────────────┐           │
│  Log Files  │           ▼
│             │    ┌─────────────┐
└─────────────┘    │    Slack    │
                   │   Discord   │
                   │    Email    │
                   └─────────────┘
```

## Health Endpoints

### `/health` - Comprehensive Health Check

Returns detailed service status including:

```json
{
  "status": "healthy|degraded|unhealthy",
  "timestamp": "2025-09-07T12:00:00Z",
  "uptime": "2h15m30s",
  "version": "v1.0.0",
  "build_stamp": "20250907-120000",
  "system": {
    "go_version": "go1.21.0",
    "num_goroutines": 42,
    "mem_alloc_bytes": 12345678,
    "mem_sys_bytes": 23456789,
    "num_gc": 15
  },
  "providers": {
    "kraken": {
      "healthy": true,
      "status": "healthy",
      "response_time": "150ms",
      "metrics": {
        "request_count": 1000,
        "success_count": 995,
        "success_rate": 0.995
      }
    }
  },
  "summary": {
    "total": 4,
    "healthy": 3,
    "degraded": 1,
    "failed": 0
  },
  "checks": {
    "critical_providers": {
      "status": "pass",
      "message": "All critical providers healthy"
    },
    "memory": {
      "status": "pass",
      "message": "Memory usage normal: 52.8%"
    }
  }
}
```

**HTTP Status Codes:**
- `200 OK`: Service healthy or degraded
- `503 Service Unavailable`: Service unhealthy

### `/ready` - Readiness Check

Kubernetes-style readiness probe. Returns `200 OK` if:
- Service is running
- At least one provider is healthy

### `/live` - Liveness Check

Kubernetes-style liveness probe. Always returns `200 OK` if service is responsive.

## Metrics Endpoints

### `/metrics` - Prometheus Metrics

Exports comprehensive metrics in Prometheus format:

#### System Metrics
```
# Go runtime information
go_info{version="go1.21.0"} 1
go_goroutines 42
go_memstats_alloc_bytes 12345678
go_memstats_sys_bytes 23456789
process_uptime_seconds 8130.45
```

#### Provider Metrics
```
# Provider health status (1=healthy, 0=unhealthy)
cryptorun_provider_healthy{venue="kraken"} 1
cryptorun_provider_healthy{venue="binance"} 1

# Request counts and success rates
cryptorun_provider_requests_total{venue="kraken"} 1000
cryptorun_provider_success_rate{venue="kraken"} 0.995
cryptorun_provider_response_time_seconds{venue="kraken"} 0.150
```

#### Circuit Breaker Metrics
```
# Circuit breaker states (0=closed, 1=half-open, 2=open)
cryptorun_circuit_breaker_state{name="kraken"} 0
cryptorun_circuit_breaker_failure_rate{name="kraken"} 0.005
```

#### Pipeline Metrics
```
# Step execution times and throughput
cryptorun_step_duration_seconds_bucket{step="data_fetch",result="success",le="0.1"} 450
cryptorun_pipeline_steps_total{step="data_fetch",status="success"} 1000
cryptorun_pipeline_errors_total{step="data_fetch",error_type="timeout"} 5
```

#### Cache Metrics
```
# Cache performance
cryptorun_cache_hit_ratio 0.87
cryptorun_cache_hits_total{cache_type="market_data"} 8700
cryptorun_cache_misses_total{cache_type="market_data"} 1300
```

#### WebSocket Metrics
```
# WebSocket latencies
cryptorun_ws_latency_ms_bucket{exchange="kraken",endpoint="ticker",le="100"} 850
```

## Grafana Dashboard

The CryptoRun Grafana dashboard (`configs/observability/grafana-dashboard.json`) provides:

### System Overview Panel
- Service status and uptime
- Active scans counter
- Provider health summary

### Provider Health Monitoring
- Real-time provider status
- Success rates over time
- Response time trends
- Circuit breaker states

### Performance Metrics
- P95/P99 latency tracking
- Pipeline throughput rates
- Memory usage trends
- Goroutine counts

### Cache Performance
- Hit ratio monitoring
- Cache utilization by type
- Eviction rates

### Market Regime Indicators
- Current regime display
- Regime health indicators
- Switch frequency tracking

### Error Analysis
- Error rates by pipeline step
- Error categorization
- Failure trend analysis

### Alerting Integration
- Visual alert status
- Annotation overlay for incidents
- Alert history timeline

## Alert Rules

### Critical Alerts (Immediate Response Required)

#### ServiceDown
- **Trigger**: `up{job="cryptorun"} == 0` for 30s
- **Action**: Page on-call, check service logs

#### NoHealthyProviders  
- **Trigger**: `sum(cryptorun_provider_healthy) == 0` for 1m
- **Action**: Check exchange status, network connectivity

#### HighMemoryUsage
- **Trigger**: Memory usage > 90% for 5m
- **Action**: Check for memory leaks, restart if needed

### Warning Alerts (Monitor and Investigate)

#### ProviderDown
- **Trigger**: Individual provider unhealthy for 2m
- **Action**: Check provider-specific issues

#### HighLatency
- **Trigger**: P95 latency > 1s for 5m
- **Action**: Investigate performance bottlenecks

#### LowCacheHitRate
- **Trigger**: Cache hit rate < 70% for 10m
- **Action**: Review cache configuration

### Info Alerts (Awareness)

#### FrequentRegimeSwitches
- **Trigger**: >5 regime switches per hour
- **Action**: Monitor market conditions

## Quick Start

### 1. Development Mode (Local)

Start the service with observability enabled:

```bash
# Build and run CryptoRun
go build ./src/cmd/cryptorun
./cryptorun monitor --http-port 8080 --metrics-port 9090

# Check health
curl http://localhost:8080/health | jq

# Check metrics
curl http://localhost:9090/metrics
```

### 2. Production Mode (Docker Compose)

```bash
cd configs/observability
docker-compose up -d

# Access services
# Grafana: http://localhost:3000 (admin/cryptorun123)
# Prometheus: http://localhost:9091
# AlertManager: http://localhost:9093
```

### 3. Kubernetes Deployment

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cryptorun-metrics
spec:
  ports:
  - name: http
    port: 8080
  - name: metrics
    port: 9090
  selector:
    app: cryptorun
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cryptorun
spec:
  selector:
    matchLabels:
      app: cryptorun
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Configuration

### Environment Variables

```bash
# Service configuration
HTTP_PORT=8080
METRICS_PORT=9090
LOG_LEVEL=info

# Prometheus scrape configuration
PROMETHEUS_URL=http://prometheus:9090

# Alert routing
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
ALERT_EMAIL=admin@cryptorun.com
```

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'cryptorun'
    static_configs:
      - targets: ['cryptorun:9090']
    scrape_interval: 30s
    metrics_path: /metrics
```

### Grafana Provisioning

```yaml
# datasources.yml
datasources:
  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090
    access: proxy
    isDefault: true
```

## Monitoring Best Practices

### 1. Health Check Strategy
- Use `/ready` for load balancer health checks
- Use `/live` for container orchestrator probes
- Monitor `/health` for detailed diagnostics

### 2. Metrics Collection
- Scrape interval: 30s for real-time metrics
- Retention: 7 days minimum for trend analysis
- Storage: Size appropriately for metric cardinality

### 3. Alerting Guidelines
- **Critical**: Requires immediate action (page/call)
- **Warning**: Investigate within business hours
- **Info**: Awareness only, no action required

### 4. Dashboard Design
- Use consistent time ranges across panels
- Group related metrics together
- Include threshold lines for SLOs
- Use annotations for deployments/incidents

## Troubleshooting

### Common Issues

#### High Memory Usage
```bash
# Check memory metrics
curl -s http://localhost:9090/metrics | grep go_memstats

# Force garbage collection
curl -X POST http://localhost:8080/debug/gc
```

#### Provider Connection Issues
```bash
# Check provider status
curl http://localhost:8080/debug/provider/ | jq

# Reset circuit breakers
curl -X POST http://localhost:8080/debug/circuit/reset
```

#### Missing Metrics
```bash
# Verify metrics endpoint
curl http://localhost:9090/metrics | head -20

# Check Prometheus targets
curl http://localhost:9091/api/v1/targets
```

### Log Analysis

#### Structured Logging
```bash
# Filter by component
tail -f cryptorun.log | jq 'select(.component=="provider")'

# Error analysis
tail -f cryptorun.log | jq 'select(.level=="error")' 
```

## Security Considerations

### 1. Access Control
- Bind health/metrics to localhost only in production
- Use reverse proxy for external access
- Implement authentication for Grafana

### 2. Data Exposure
- Avoid exposing sensitive data in metrics labels
- Use aggregation to prevent information leakage
- Regular security review of dashboard permissions

### 3. Network Security
- Use TLS for external metric collection
- Implement network policies in Kubernetes
- Monitor access logs for unauthorized requests

## Performance Impact

### Resource Usage
- **Memory**: ~5MB additional for metrics collection
- **CPU**: <1% overhead for metric emission
- **Network**: ~10KB/s metrics export bandwidth
- **Storage**: ~1GB/month for 30s scrape interval

### Optimization
- Use metric sampling for high-cardinality data
- Implement metric rotation for long-running services  
- Cache expensive metric calculations

## Integration Examples

### Custom Metrics

```go
// Add custom business metrics
metrics.SetCustomMetric("cryptorun_signals_generated_total", signalCount)
metrics.IncrementCustomMetric("cryptorun_trades_executed", 1)
```

### Alert Integration

```go
// Trigger alert from application
if criticalError {
    metrics.RecordError()
    log.Error().Msg("Critical error occurred")
}
```

### Dashboard Integration

```javascript
// Grafana dashboard variable
const exchange = getTemplateVar('exchange');
const query = `cryptorun_provider_healthy{venue="${exchange}"}`;
```

This comprehensive observability setup ensures complete visibility into CryptoRun's performance, health, and operational status, enabling proactive monitoring and rapid incident response.