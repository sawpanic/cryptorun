# CryptoRun Explainability System

## UX MUST — Live Progress & Explainability

The CryptoRun explainability system provides **complete transparency** into every scoring decision with per-asset attribution, gate evaluations, and system health metrics. Every output includes numerical breakdowns and top-3 "why included/excluded" reasons.

## Overview

CryptoRun's explainability layer generates **audit-ready artifacts** that capture the complete decision-making process for each asset evaluation. The system produces both machine-readable JSON (full fidelity) and human-readable CSV (console-optimized) outputs with stable ordering for CI/CD integration.

## Output Formats

### JSON Format (Full Fidelity)
Complete explainability report with nested structures and full attribution:

```json
{
  "meta": {
    "timestamp": "2024-01-15T10:30:00Z",
    "input_hash": "v1_timestamp:obj_regime:choppy_scan_type:momentum",
    "version": "v3.2.1",
    "report_type": "full_explainability",
    "assets_count": 100,
    "included_count": 12,
    "excluded_count": 88
  },
  "universe": [
    {
      "symbol": "BTC-USD",
      "decision": "included",
      "score": 85.45,
      "rank": 1,
      "factor_parts": {
        "momentum": 45.2,
        "technical": 12.8,
        "volume": 8.3,
        "quality": 4.1,
        "social": 3.0,
        "social_capped": 3.0
      },
      "gate_results": {
        "entry_gate": {"passed": true, "value": 85.45, "threshold": 75.0, "reason": "composite_score_threshold"},
        "freshness_gate": {"passed": true, "value": 1.0, "threshold": 1.2, "reason": "within_atr_bounds"},
        "fatigue_gate": {"passed": true, "value": 65.0, "threshold": 70.0, "reason": "rsi_not_overbought"},
        "late_fill_gate": {"passed": true, "value": 15.0, "threshold": 30.0, "reason": "signal_timing_acceptable"},
        "micro_gate": {"passed": true, "value": 2.1, "threshold": 1.8, "reason": "vadr_liquidity_requirement"},
        "overall_result": true
      },
      "microstructure": {
        "spread_bps": 0.025,
        "depth_usd": 150000,
        "vadr": 2.1,
        "exchange": "kraken",
        "is_exchange_native": true
      },
      "catalyst_profile": {
        "heat_score": 75.0,
        "time_decay": 0.8,
        "event_types": ["earnings", "token_unlock"],
        "next_event": "2024-01-18T10:30:00Z"
      },
      "attribution": {
        "top_inclusion_reasons": [
          "strong_composite_score",
          "strong_momentum",
          "sufficient_liquidity"
        ],
        "top_exclusion_reasons": [],
        "regime_influence": "choppy",
        "weight_breakdown": {
          "momentum": "high_contribution",
          "technical": "medium_contribution",
          "volume": "low_contribution",
          "quality": "low_contribution",
          "social": "low_contribution"
        }
      },
      "data_quality": {
        "ttls": {
          "price_data": "2024-01-15T10:35:00Z",
          "volume_data": "2024-01-15T10:40:00Z",
          "social_data": "2024-01-15T11:30:00Z",
          "micro_data": "2024-01-15T10:32:00Z"
        },
        "cache_hits": {
          "price_data": true,
          "volume_data": true,
          "social_data": false,
          "micro_data": true
        },
        "freshness_age": {
          "price_data": "45s",
          "volume_data": "1m30s",
          "social_data": "15m",
          "micro_data": "30s"
        },
        "missing_fields": []
      }
    }
  ],
  "config": {
    "regime_weights": {"momentum": 0.40, "technical": 0.35, "volume": 0.15, "quality": 0.10},
    "current_regime": "choppy",
    "gate_thresholds": {"entry_score": 75.0, "vadr": 1.8, "spread_bps": 50.0, "depth_usd": 100000.0},
    "social_cap": 10.0,
    "momentum_weights": {"1h": 0.20, "4h": 0.35, "12h": 0.30, "24h": 0.15},
    "config_version": "v3.2.1"
  },
  "health": {
    "provider_status": {
      "kraken": {"status": "healthy", "last_success": "2024-01-15T10:28:00Z", "error_rate": 0.05, "latency": "150ms"},
      "coingecko": {"status": "degraded", "last_success": "2024-01-15T10:20:00Z", "error_rate": 0.15, "latency": "800ms"}
    },
    "circuit_breakers": {"kraken_api": false, "coingecko_api": true},
    "rate_limits": {
      "kraken": {"remaining": 85, "reset": "2024-01-15T10:35:00Z", "limit": 100}
    },
    "cache_stats": {"hit_rate": 0.87, "total_hits": 1250, "total_misses": 185, "eviction_rate": 0.02}
  }
}
```

### CSV Format (Console-Optimized)
Compact 18-column format for quick analysis:

```csv
symbol,decision,score,rank,momentum,technical,volume,quality,social,entry_gate,spread_bps,depth_usd,vadr,heat_score,regime,exchange,top_reason,cache_hit_rate
BTC-USD,included,85.450000,1,45.200000,12.800000,8.300000,4.100000,3.000000,true,0.025000,150000.000000,2.100000,75.000000,choppy,kraken,strong_composite_score,0.750000
ETH-USD,excluded,68.200000,2,32.100000,15.600000,12.400000,5.800000,2.300000,false,0.035000,120000.000000,1.650000,62.000000,choppy,kraken,weak_composite_score,0.800000
```

## Schema Components

### Per-Asset Attribution
Each asset includes complete scoring breakdown:

- **Factor Parts**: Orthogonal component scores (momentum/technical/volume/quality/social)
- **Gate Results**: Pass/fail status for all 5 gates with values, thresholds, and reasons
- **Microstructure Metrics**: Spread, depth, VADR, exchange validation
- **Catalyst Profile**: Heat score, time decay, event types, next event timing
- **Data Quality**: TTLs, cache hits, freshness age, missing fields

### Top-3 Attribution Logic
```go
// Inclusion reasons (when asset passes)
- "strong_composite_score" (score ≥ 75)
- "strong_momentum" (momentum component > 20)  
- "sufficient_liquidity" (VADR ≥ 1.8)
- "favorable_catalyst" (heat score > 70)
- "quality_microstructure" (spread < 30bps)

// Exclusion reasons (when asset fails)
- "weak_composite_score" (score < 75)
- "insufficient_liquidity" (VADR < 1.8)
- "poor_microstructure" (spread > 50bps)
- "regime_mismatch" (poor fit for current regime)
- "data_quality_issues" (missing/stale data)
```

### Stable Ordering
Reports use deterministic sorting for CI/CD compatibility:
1. **Primary**: Symbol (alphabetical)
2. **Secondary**: Score (descending)
3. **Rank Assignment**: After sorting, rank = index + 1

### Input Hash Generation
Consistent hashing for cache validation and diff detection:
```go
// Sorted key-value pairs ensure consistent hashing
hash = "v1_" + sortedKeys.map(k => k + ":" + toString(inputs[k])).join("_")
// Example: "v1_regime:choppy_scan_type:momentum_timestamp:obj"
```

## API Usage

### Data Collector
```go
import "cryptorun/internal/explain"

collector := explain.NewDataCollector("v3.2.1", "./artifacts/explain")

symbols := []string{"BTC-USD", "ETH-USD", "ADA-USD"}
inputs := map[string]interface{}{
    "timestamp": time.Now().UTC(),
    "regime":    "choppy", 
    "scan_type": "momentum",
}

report, err := collector.GenerateReport(ctx, symbols, inputs)
if err != nil {
    log.Fatal(err)
}

// Report contains complete explainability data
fmt.Printf("Generated report for %d assets (%d included, %d excluded)\n",
    report.Meta.AssetsCount, report.Meta.IncludedCount, report.Meta.ExcludedCount)
```

### Atomic Writer
```go
import "cryptorun/src/internal/artifacts"

writer := artifacts.NewAtomicWriter("./artifacts/explain")

// Atomic write: temp file → rename (no partial writes)
err := writer.WriteExplainReport(report, "momentum_scan")
// Creates: 20240115-103000-momentum_scan-explain.json
//          20240115-103000-momentum_scan-explain.csv
```

## Configuration

### Artifacts Configuration
```yaml
# config/artifacts.yaml
explain:
  output_dir: "./artifacts/explain"
  retention_days: 30
  max_files: 1000
  compression: false

formats:
  json:
    enabled: true
    indent: 2
    include_nulls: false
  csv:
    enabled: true
    max_columns: 20
    precision: 6
```

### Gate Thresholds
```yaml
# config/gates.yaml
thresholds:
  entry_score: 75.0      # Composite score minimum
  vadr: 1.8              # Volume-Adjusted Daily Range  
  spread_bps: 50.0       # Max spread in basis points
  depth_usd: 100000.0    # Min depth within ±2%
  rsi_fatigue: 70.0      # Max RSI before fatigue
  freshness_atr: 1.2     # Max ATR multiplier for freshness
  late_fill_seconds: 30  # Max seconds after signal bar
```

## Data Quality Tracking

### TTL (Time-To-Live) Monitoring
```json
"ttls": {
  "price_data": "2024-01-15T10:35:00Z",    // 5min TTL
  "volume_data": "2024-01-15T10:40:00Z",   // 10min TTL  
  "social_data": "2024-01-15T11:30:00Z",   // 1hr TTL
  "micro_data": "2024-01-15T10:32:00Z"     // 2min TTL
}
```

### Cache Hit Tracking
```json
"cache_hits": {
  "price_data": true,   // Retrieved from cache
  "volume_data": true,  // Retrieved from cache
  "social_data": false, // Fetched from provider
  "micro_data": true    // Retrieved from cache
}
```

### Freshness Age Tracking
```json
"freshness_age": {
  "price_data": "45s",    // Last updated 45 seconds ago
  "volume_data": "1m30s", // Last updated 1 minute 30 seconds ago  
  "social_data": "15m",   // Last updated 15 minutes ago
  "micro_data": "30s"     // Last updated 30 seconds ago
}
```

## System Health Integration

### Provider Status
Real-time API health with error rates and latency:
```json
"provider_status": {
  "kraken": {
    "status": "healthy",
    "last_success": "2024-01-15T10:28:00Z",
    "error_rate": 0.05,     // 5% error rate
    "latency": "150ms"
  },
  "coingecko": {
    "status": "degraded", 
    "last_success": "2024-01-15T10:20:00Z",
    "error_rate": 0.15,     // 15% error rate (degraded)
    "latency": "800ms"
  }
}
```

### Circuit Breaker Status
```json
"circuit_breakers": {
  "kraken_api": false,    // Circuit closed (healthy)
  "coingecko_api": true   // Circuit open (failing)
}
```

### Rate Limit Tracking
```json
"rate_limits": {
  "kraken": {
    "remaining": 85,                    // 85 requests remaining
    "reset": "2024-01-15T10:35:00Z",   // Reset time
    "limit": 100                       // Total limit per window
  }
}
```

## Testing & Validation

### Schema Round-Trip Testing
```go
func TestSchemaRoundTrip(t *testing.T) {
    report, _ := collector.GenerateReport(ctx, symbols, inputs)
    
    // JSON round-trip
    data, _ := json.Marshal(report)
    var parsed ExplainReport
    json.Unmarshal(data, &parsed)
    
    // Validate critical fields preserved
    assert.Equal(t, report.Meta.AssetsCount, parsed.Meta.AssetsCount)
    assert.Equal(t, len(report.Universe), len(parsed.Universe))
}
```

### Stable Ordering Validation
```go
func TestStableOrdering(t *testing.T) {
    // Same inputs should produce identical ordering
    report1, _ := collector.GenerateReport(ctx, symbols, inputs)
    report2, _ := collector.GenerateReport(ctx, symbols, inputs)
    
    assert.Equal(t, report1.Meta.InputHash, report2.Meta.InputHash)
    for i := range report1.Universe {
        assert.Equal(t, report1.Universe[i].Symbol, report2.Universe[i].Symbol)
        assert.Equal(t, report1.Universe[i].Score, report2.Universe[i].Score)
    }
}
```

### CSV Format Validation
```go
func TestCSVGeneration(t *testing.T) {
    asset := AssetExplain{...}
    csvRow := asset.ToCSVRow()
    
    // Validate CSV structure matches header
    assert.Equal(t, "BTC-USD", csvRow.Symbol)
    assert.Equal(t, "included", csvRow.Decision)
    assert.Equal(t, 85.5, csvRow.Score)
    assert.Equal(t, "strong_momentum", csvRow.TopReason)
}
```

## File Naming Convention

### Timestamp-Based Naming
```
Format: YYYYMMDD-HHMMSS-{prefix}-explain.{ext}
Examples:
  20240115-103000-momentum_scan-explain.json
  20240115-103000-momentum_scan-explain.csv
  20240115-154500-regime_change-explain.json
```

### Directory Structure
```
artifacts/
└── explain/
    ├── 20240115-103000-momentum_scan-explain.json
    ├── 20240115-103000-momentum_scan-explain.csv
    ├── 20240115-154500-regime_change-explain.json
    ├── 20240115-154500-regime_change-explain.csv
    └── archived/
        └── 2024-01/
            ├── older files...
```

## Performance Characteristics

### Report Generation
- **Small Universe** (≤25 assets): <100ms
- **Medium Universe** (26-100 assets): <500ms  
- **Large Universe** (101-500 assets): <2000ms

### File I/O
- **JSON Write**: ~1-5MB files, <50ms atomic write
- **CSV Write**: ~10-50KB files, <10ms atomic write
- **Concurrent Safety**: Multiple goroutines can generate reports safely

### Memory Usage
- **Base Report**: ~1KB per asset
- **Full Attribution**: ~5KB per asset
- **System Health**: ~2KB fixed overhead

This explainability system ensures complete transparency and auditability for all CryptoRun scoring decisions while maintaining high performance and CI/CD compatibility.