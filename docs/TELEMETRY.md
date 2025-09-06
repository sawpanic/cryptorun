# CryptoRun Telemetry System

## UX MUST — Live Progress & Explainability

All telemetry collection provides complete transparency into pipeline performance with real-time metrics, percentile tracking, and stage-by-stage latency attribution for operational visibility and p99-based guard relaxation decisions.

## Overview

CryptoRun implements comprehensive latency telemetry to track pipeline performance and enable intelligent guard relaxation. The system uses rolling histograms to calculate real-time percentiles (p50, p95, p99) across all pipeline stages.

## Pipeline Stages

### Stage Classification
```go
type StageType string

const (
    StageData  StageType = "data"   // Market data fetching
    StageScore StageType = "score"  // Factor calculation and scoring  
    StageGate  StageType = "gate"   // Guard evaluation and gating
    StageOrder StageType = "order"  // Order preparation and simulation
)
```

### Measurement Points
- **Data Stage**: WebSocket/REST API calls, cache operations
- **Score Stage**: Factor calculation, orthogonalization, composite scoring
- **Gate Stage**: Guard evaluation (freshness, fatigue, liquidity, etc.)
- **Order Stage**: Order simulation, position sizing, execution preparation

## Histogram Implementation

### Rolling Window Design
```go
type Histogram struct {
    buckets  []float64 // Latency values in milliseconds
    maxSize  int       // Rolling window size (default: 1000)
    current  int       // Current position in circular buffer
    full     bool      // Whether buffer is full
    stage    StageType // Associated pipeline stage
}
```

### Thread-Safe Operations
- **RWMutex Protection**: All read/write operations use proper locking
- **Atomic Updates**: Buffer position updates are atomic
- **Lock-Free Reads**: Percentile calculations minimize lock contention

### Percentile Calculation
```go
// Linear interpolation for accurate percentiles
func (h *Histogram) Percentile(p float64) float64 {
    values := sort.Float64s(activeSamples)
    index := p * float64(len(values)-1)
    lower := int(math.Floor(index))
    upper := int(math.Ceil(index))
    
    if lower == upper {
        return values[lower]
    }
    
    weight := index - float64(lower)
    return values[lower]*(1-weight) + values[upper]*weight
}
```

## Global Tracking Interface

### Recording Latencies
```go
import "cryptorun/internal/telemetry/latency"

// Manual recording
latency.Record(latency.StageData, 150*time.Millisecond)

// Timer-based recording (recommended)
timer := latency.StartTimer(latency.StageScore)
defer timer.Stop()

// Result-aware recording
timer := latency.StartTimer(latency.StageGate)
success := performGuardEvaluation()
timer.StopWithResult(success)
```

### Real-Time Metrics Access
```go
// Get current p99 for late-fill guard decisions
currentP99 := latency.GetP99(latency.StageOrder)

// Get all stage metrics
allMetrics := latency.GetAllMetrics()
for stage, metrics := range allMetrics {
    fmt.Printf("%s: p50=%.1fms, p95=%.1fms, p99=%.1fms (%d samples)\n",
        stage, metrics.P50, metrics.P95, metrics.P99, metrics.Count)
}
```

## Integration with Guards System

### P99 Threshold Monitoring
The Late-Fill guard uses order stage p99 latency for relaxation decisions:

```go
// Check if infrastructure is degraded
orderP99 := latency.GetP99(latency.StageOrder)
if orderP99 > 400.0 { // 400ms threshold
    // Apply bounded grace window for late fills
    return allowWithRelaxation(input, orderP99)
}
```

### Relaxation Logic Integration
```go
type LateFillResult struct {
    Allowed       bool      // Whether execution is allowed
    RelaxReason   string    // e.g. "latefill_relax[p99_exceeded:450.2ms,grace:30s]"
    DelayMs       float64   // Actual execution delay
    RelaxUsed     bool      // Whether p99 relaxation was applied
    NextRelaxTime time.Time // When next relaxation is allowed (30m cooldown)
}
```

## Metrics Export

### Prometheus Integration
Latency metrics are exported for monitoring dashboards:

```
# Stage latency histograms
cryptorun_stage_latency_seconds{stage="data",quantile="0.5"} 0.045
cryptorun_stage_latency_seconds{stage="data",quantile="0.95"} 0.150  
cryptorun_stage_latency_seconds{stage="data",quantile="0.99"} 0.280

cryptorun_stage_latency_seconds{stage="score",quantile="0.5"} 0.012
cryptorun_stage_latency_seconds{stage="score",quantile="0.95"} 0.034
cryptorun_stage_latency_seconds{stage="score",quantile="0.99"} 0.067

cryptorun_stage_latency_seconds{stage="gate",quantile="0.5"} 0.003
cryptorun_stage_latency_seconds{stage="gate",quantile="0.95"} 0.008
cryptorun_stage_latency_seconds{stage="gate",quantile="0.99"} 0.015

cryptorun_stage_latency_seconds{stage="order",quantile="0.5"} 0.125
cryptorun_stage_latency_seconds{stage="order",quantile="0.95"} 0.350
cryptorun_stage_latency_seconds{stage="order",quantile="0.99"} 0.450
```

### Guard Relaxation Metrics
```
# Late-fill guard p99 relaxation usage
cryptorun_latefill_p99_relaxations_total{result="applied"} 12
cryptorun_latefill_p99_relaxations_total{result="blocked_cooldown"} 8
cryptorun_latefill_p99_relaxations_total{result="blocked_excessive"} 3

# Current p99 vs threshold
cryptorun_latefill_current_p99_ms 450.2
cryptorun_latefill_p99_threshold_ms 400.0
cryptorun_latefill_p99_exceeded 1

# Active relaxation cooldowns
cryptorun_latefill_active_cooldowns 5
```

## Menu Integration

### Progress Display with Latency Context
Menu progress logs show stage latencies and p99 relaxation context:

```
⚡ Momentum Pipeline [████████████████░░░░] 6/8 (75.0%) ETA: 3s
  ✅ Universe: 50 symbols (45ms p99: 67ms)
  ✅ Data Fetch: 50/50 symbols (2.1s p99: 280ms)
  ✅ Guards: 37/50 passed (156ms p99: 15ms)
  ✅ Factors: 4-timeframe momentum (847ms p99: 67ms)
  ✅ Score: Composite scoring (189ms p99: 45ms)  
  🔄 Gates: Entry validation [██████████░░░░░░░░░░] ETA: 2s (p99: 125ms)
```

### P99 Relaxation Notifications
```
🛡️ [80%] Evaluating late-fill guards (p99: 450.2ms > 400ms threshold)...
🔄 P99 relax applied to ETHUSD: latefill_relax[p99_exceeded:450.2ms,grace:30s]

🔄 P99 Relaxations Applied:
   ETHUSD: latefill_relax[p99_exceeded:450.2ms,grace:30s]
Note: 1 asset(s) used late-fill p99 relaxation (30m cooldown active)
```

## Performance Characteristics

### Memory Usage
- **Per-Stage Histogram**: ~8KB (1000 samples × 8 bytes per float64)
- **Total Overhead**: ~32KB for 4 stages + metadata
- **Allocation Pattern**: Pre-allocated circular buffers, no runtime allocation

### CPU Performance
- **Recording**: ~100ns per sample (atomic writes)
- **Percentile Calculation**: ~10μs for 1000 samples (sort + interpolation)
- **Concurrent Access**: RWMutex allows parallel reads during recording

### Retention Policy
- **Rolling Window**: 1000 most recent samples per stage
- **Memory Bounded**: Fixed size prevents memory growth
- **Temporal Coverage**: ~10-60 minutes depending on scan frequency

## Configuration

### Histogram Sizing
```go
// Default configuration
defaultHistogram := NewHistogram(StageData, 1000)

// Custom sizing for high-frequency stages
highFreqHistogram := NewHistogram(StageOrder, 5000)
```

### P99 Thresholds
```go
// Default Late-Fill guard thresholds
guard := NewLateFillGuard(
    30000, // 30s base threshold
    400,   // 400ms p99 threshold
    30000, // 30s grace window
)
```

## Testing Support

### Mock Clock Integration
```go
func TestLatencyTracking(t *testing.T) {
    mockClock := NewMockClock(baseTime)
    
    // Record controlled latencies
    timer := StartTimerWithClock(StageData, mockClock)
    mockClock.Advance(150 * time.Millisecond)
    timer.Stop()
    
    // Verify metrics
    metrics := GetStageMetrics(StageData)
    assert.Equal(t, 150.0, metrics.P99)
}
```

### Deterministic Testing
```go
// Reset global state for consistent tests
func TestLateFillRelaxation(t *testing.T) {
    // Clear existing samples
    globalTracker.Reset()
    
    // Inject controlled p99 samples
    for i := 0; i < 100; i++ {
        Record(StageOrder, 500*time.Millisecond)
    }
    
    // Test relaxation logic with known p99
    assert.True(t, GetP99(StageOrder) > 400)
}
```

This telemetry system provides comprehensive latency visibility while enabling intelligent guard relaxation based on real-time infrastructure performance metrics.