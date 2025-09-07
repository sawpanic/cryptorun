# CryptoRun Operations Guide

## UX MUST — Live Progress & Explainability

This document provides comprehensive guidance for operational monitoring, KPIs, circuit breakers, and emergency controls in CryptoRun v3.2.1.

## Overview

The Operations module (`internal/ops/`) provides real-time operational monitoring and emergency controls for the CryptoRun cryptocurrency momentum scanner. It implements:

- **KPI Tracking**: Rolling window metrics for performance monitoring
- **Guard System**: Budget limits, call quotas, and correlation caps
- **Emergency Switches**: Immediate system disable/enable controls
- **Circuit Breakers**: Provider-aware failure handling
- **Status Reporting**: Console tables and CSV artifacts

## Architecture

```
internal/ops/
├── kpi.go         # Rolling KPI metrics tracking
├── guards.go      # Operational guards and limits
├── switches.go    # Emergency toggles and controls
└── render.go      # Status rendering and artifacts

cmd/cryptorun/
└── cmd_ops_status.go  # Hidden CLI command

config/
└── ops.yaml       # Operational configuration
```

## Configuration

### `config/ops.yaml` Structure

```yaml
kpi:
  windows:
    requests_per_min: 60    # Window for request rate calculation (seconds)
    error_rate: 300         # Window for error rate calculation (5min)
    cache_hit_rate: 300     # Window for cache hit calculation (5min)
    
  thresholds:
    error_rate_warn: 0.05       # 5% error rate warning
    error_rate_critical: 0.15   # 15% error rate critical
    cache_hit_rate_warn: 0.75   # 75% cache hit rate warning
    requests_per_min_warn: 100  # requests/min warning

guards:
  budget:
    enabled: true
    hourly_limit: 3600          # API calls per hour
    soft_warn_percent: 0.8      # Warn at 80% of budget
    hard_stop_percent: 0.95     # Block at 95% of budget
    
  call_quota:
    enabled: true
    providers:
      kraken:
        calls_per_minute: 60
        burst_limit: 10
        
  correlation:
    enabled: true
    max_correlation: 0.85       # Maximum signal correlation
    top_n_signals: 10           # Analyze top N signals
    lookback_periods: 24        # Hours of history
    
  venue_health:
    enabled: true
    min_uptime_percent: 0.95    # 95% uptime required
    max_latency_ms: 5000        # 5 second max latency

switches:
  emergency:
    disable_all_scanners: false
    disable_live_data: false
    read_only_mode: false
    
  providers:
    kraken:
      enabled: true
      allow_websocket: true
      allow_rest: true
      
  venues:
    kraken_usd: true
    binance_usd: true
```

## KPI Metrics

### Tracked Metrics

1. **Requests Per Minute**: API call rate with configurable window
2. **Error Rate Percentage**: Failed requests / total requests
3. **Cache Hit Rate**: Cache hits / total cache operations
4. **Open Breaker Count**: Number of active circuit breakers
5. **Venue Health**: Healthy vs unhealthy venue count

### Rolling Windows

- **Request Rate**: 60-second rolling window
- **Error Rate**: 5-minute rolling window for stability
- **Cache Metrics**: 5-minute rolling window

### Implementation

```go
// Create KPI tracker
tracker := ops.NewKPITracker(
    60*time.Second,   // request window
    300*time.Second,  // error window  
    300*time.Second,  // cache window
)

// Record operations
tracker.RecordRequest()
tracker.RecordError()
tracker.RecordCacheHit()
tracker.RecordCacheMiss()

// Get current metrics
metrics := tracker.GetMetrics()
```

## Guard System

### Budget Guard

Prevents API quota exhaustion with hourly limits:

- **Soft Warning**: 80% of budget (configurable)
- **Hard Stop**: 95% of budget (blocks new requests)
- **Tracking**: Hourly buckets with automatic cleanup

### Call Quota Guard

Per-provider rate limiting with burst protection:

- **Rate Limits**: Calls per minute per provider
- **Burst Detection**: High-frequency call detection
- **Provider-Specific**: Different limits for each exchange

### Correlation Guard

Prevents highly correlated signals from triggering simultaneously:

- **Correlation Threshold**: Maximum allowed correlation (default 0.85)
- **Signal Analysis**: Top N signals correlation check
- **Historical Data**: Lookback period for correlation calculation

### Venue Health Guard

Monitors exchange health metrics:

- **Uptime Requirements**: Minimum uptime percentage
- **Latency Limits**: Maximum acceptable latency
- **Depth Requirements**: Minimum market depth
- **Spread Limits**: Maximum bid-ask spread

### Guard Results

```go
type GuardResult struct {
    Name     string                 // Guard name
    Status   GuardStatus           // OK/WARN/CRITICAL/BLOCK
    Message  string                // Human-readable message
    Metadata map[string]interface{} // Additional data
}
```

## Emergency Switches

### Emergency Controls

- **Disable All Scanners**: Stops all momentum scanning
- **Disable Live Data**: Blocks live market data feeds
- **Read-Only Mode**: Prevents any write operations

### Provider Controls

Per-provider granular controls:

- **Enable/Disable**: Complete provider shutdown
- **WebSocket Control**: Allow/block WebSocket connections
- **REST Control**: Allow/block REST API calls

### Venue Controls

Per-venue enable/disable switches for fine-grained control.

### Runtime Management

```go
// Emergency switches
switchManager.SetEmergencySwitch("disable_all_scanners", true)
switchManager.SetEmergencySwitch("read_only_mode", true)

// Provider controls
switchManager.SetProviderSwitch("kraken", "enabled", false)
switchManager.SetProviderSwitch("binance", "websocket", false)

// Venue controls
switchManager.SetVenueSwitch("kraken_usd", false)

// Check status
status := switchManager.GetStatus()
```

## CLI Usage

### Hidden Command

The ops status command is hidden from normal help output:

```bash
# Show operational status
cryptorun ops status

# Specify custom config
cryptorun ops status --config custom/ops.yaml

# Custom output directory
cryptorun ops status --output /custom/artifacts/ops
```

### Console Output

The command displays a comprehensive status table:

```
=== CryptoRun Operational Status ===
Timestamp: 2024-12-07 15:30:45

📊 KEY PERFORMANCE INDICATORS
┌─────────────────────┬──────────┬────────────┐
│ Metric              │ Value    │ Status     │
├─────────────────────┼──────────┼────────────┤
│ Requests/min        │     45.0 │ OK         │
│ Error rate          │      6.3% │ WARN       │
│ Cache hit rate      │     80.0% │ OK         │
│ Open breakers       │        0 │ OK         │
│ Healthy venues      │      4/4 │ OK         │
└─────────────────────┴──────────┴────────────┘

🛡️  OPERATIONAL GUARDS
┌─────────────────────┬──────────┬─────────────────────────────────┐
│ Guard               │ Status   │ Message                         │
├─────────────────────┼──────────┼─────────────────────────────────┤
│ budget              │ ✅OK     │ API budget OK: 1205/3600 (33.5%) │
│ kraken              │ ✅OK     │ Provider kraken rate OK: 25/60   │
│ correlation         │ ✅OK     │ Signal correlation OK: 0.23      │
└─────────────────────┴──────────┴─────────────────────────────────┘
```

## Artifacts

### CSV Snapshots

Operational snapshots are written to `./artifacts/ops/`:

- **Timestamped Files**: `status_snapshot_20241207_153045.csv`
- **Standard File**: `status_snapshot.csv` (always latest)
- **Retention**: Configurable cleanup (30 days default)

### CSV Format

```csv
timestamp,category,name,value,status,message
2024-12-07 15:30:45,kpi,requests_per_minute,45.0,OK,
2024-12-07 15:30:45,kpi,error_rate_percent,6.3,WARN,
2024-12-07 15:30:45,guard,budget,,OK,API budget OK: 1205/3600 calls
2024-12-07 15:30:45,switch,emergency_scanners,ON,,
```

## Integration Patterns

### Application Integration

```go
// Initialize ops components
kpiTracker := ops.NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)
guardManager := ops.NewGuardManager(guardConfig)
switchManager := ops.NewSwitchManager(switchConfig)

// Record operations during API calls
func makeAPICall(provider string) error {
    // Check if provider is enabled
    if !switchManager.IsProviderEnabled(provider) {
        return fmt.Errorf("provider %s disabled", provider)
    }
    
    // Check emergency switches
    if switchManager.IsReadOnlyMode() {
        return fmt.Errorf("system in read-only mode")
    }
    
    // Record API call for tracking
    guardManager.RecordAPICall(provider)
    
    // Check guards before proceeding
    guards := guardManager.CheckAllGuards()
    for _, guard := range guards {
        if guard.Status == ops.GuardStatusBlock {
            return fmt.Errorf("guard blocked: %s", guard.Message)
        }
    }
    
    // Make actual API call
    err := actualAPICall(provider)
    if err != nil {
        kpiTracker.RecordError()
        return err
    }
    
    kpiTracker.RecordRequest()
    return nil
}
```

### Pipeline Integration

```go
// In scanning pipeline
func processCandidates(candidates []Candidate) error {
    // Check emergency switches first
    if !switchManager.IsScannersEnabled() {
        return fmt.Errorf("scanners disabled")
    }
    
    // Record signals for correlation analysis
    for _, candidate := range candidates {
        signal := ops.SignalData{
            Symbol:    candidate.Symbol,
            Score:     candidate.Score,
            Timestamp: time.Now(),
        }
        guardManager.RecordSignal(signal)
    }
    
    // Check correlation guard
    guards := guardManager.CheckAllGuards()
    for _, guard := range guards {
        if guard.Name == "correlation" && guard.Status == ops.GuardStatusBlock {
            return fmt.Errorf("correlation too high: %s", guard.Message)
        }
    }
    
    return processCandidatesInternal(candidates)
}
```

## Monitoring and Alerts

### Status Levels

1. **OK**: Normal operation
2. **WARN**: Approaching limits, attention needed
3. **CRITICAL**: Exceeding limits, degraded operation
4. **BLOCK**: Hard limits reached, operations blocked

### Alert Integration

The ops system provides structured data for external monitoring:

```go
// Get current status for external monitoring
metrics := kpiTracker.GetMetrics()
guards := guardManager.CheckAllGuards()
switches := switchManager.GetStatus()

// Send to monitoring system
if metrics.ErrorRatePercent > 10.0 {
    sendAlert("High error rate: %.1f%%", metrics.ErrorRatePercent)
}

for _, guard := range guards {
    if guard.Status == ops.GuardStatusCritical {
        sendAlert("Guard critical: %s - %s", guard.Name, guard.Message)
    }
}
```

## Performance Considerations

### Memory Management

- **Rolling Windows**: Automatic cleanup of old data points
- **Fixed Buffers**: Bounded memory usage for KPI tracking
- **Efficient Cleanup**: Time-based expiration of historical data

### CPU Optimization

- **Lazy Evaluation**: Guards checked on-demand with caching
- **Batch Operations**: Correlation calculations optimized for small datasets
- **Minimal Locking**: Read-heavy operations with minimal write contention

### Storage

- **Compressed CSV**: Efficient artifact storage
- **Configurable Retention**: Automatic cleanup of old snapshots
- **Streaming Writes**: Large datasets handled efficiently

## Troubleshooting

### Common Issues

1. **High Error Rates**
   - Check provider health and connectivity
   - Verify rate limiting configuration
   - Review circuit breaker status

2. **Low Cache Hit Rates**
   - Examine cache TTL configuration
   - Check cache backend connectivity
   - Review cache key distribution

3. **Correlation Blocks**
   - Analyze signal diversity
   - Adjust correlation threshold
   - Review lookback period

4. **Budget Exhaustion**
   - Check hourly limits vs actual usage
   - Review provider quota allocation
   - Consider rate limiting adjustments

### Debug Mode

Enable verbose logging for detailed operational insights:

```bash
CRYPTORUN_LOG_LEVEL=debug cryptorun ops status
```

## Security Considerations

- **Read-Only Artifacts**: CSV files written with restrictive permissions
- **No Sensitive Data**: KPI tracking excludes sensitive operational data
- **Access Controls**: Emergency switches require appropriate authorization
- **Audit Trail**: All switch changes logged with timestamps

## Future Enhancements

- **Real-time Dashboards**: Web interface for operational monitoring
- **Alert Integration**: Direct integration with PagerDuty/Slack
- **Predictive Analysis**: ML-based anomaly detection
- **Automated Response**: Smart circuit breaker actions
- **Multi-tenant**: Per-user operational controls