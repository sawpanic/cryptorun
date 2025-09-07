# CryptoRun Scheduler System

CryptoRun's scheduling system provides automated execution of core workflows with configurable cadences and comprehensive output. Built for CryptoRun v3.2.1.

## Overview

The scheduler manages three core production jobs:
- **Hot Scan** (*/15m): Top-30 ADV universe with momentum + premove analysis
- **Warm Scan** (0 */2h): Remaining universe with cached sources and lower QPS
- **Regime Refresh** (0 */4h): Market regime detection with 3-indicator majority vote

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    CryptoRun Production Scheduler                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│  │   Hot Scan      │    │   Warm Scan     │    │ Regime Refresh  │     │
│  │   */15 * * * *  │    │   0 */2 * * *   │    │   0 */4 * * *   │     │
│  │                 │    │                 │    │                 │     │
│  │ • Top-30 ADV    │    │ • Remaining     │    │ • Realized Vol  │     │
│  │ • Momentum +    │    │   Universe      │    │ • %>20MA        │     │
│  │   Premove       │    │ • Cached Sources│    │ • Breadth Thrust│     │
│  │ • Regime-aware  │    │ • Lower QPS     │    │ • Majority Vote │     │
│  │   Weights       │    │ • Score >= 65   │    │ • Weight Blends │     │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘     │
│           │                       │                       │             │
│           └───────────────────────┼───────────────────────┘             │
│                                   │                                     │
│  ┌─────────────────────────────────────────────────────────────────────┤
│  │                          Artifact Engine                            │
│  └─────────────────────────────────────────────────────────────────────┤
│                                   │                                     │
│  artifacts/signals/{timestamp}/   │                                     │
│  ├── signals.csv                  │ [Fresh ●] [Depth ✓] [Venue]       │
│  ├── premove.csv                  │ [Sources n] columns + deterministic│
│  ├── warm_signals.csv             │ gate reasons                       │
│  ├── explain.json                 │                                     │
│  └── regime.json                  │                                     │
│                                   │                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

## Job Specifications

### Hot Scan Job (`scan.hot`)
- **Schedule**: Every 15 minutes (`*/15 * * * *`)  
- **Universe**: Top-30 ADV pairs (highest liquidity)
- **Features**: Momentum + premove analysis with regime-aware weight blending
- **Gates**: Score≥75 + VADR≥1.8 + venue-native L1/L2 depth/spread validation
- **Artifacts**: 
  - `signals.csv` - Hot signals with [Fresh ●] [Depth ✓] [Venue] [Sources n] columns
  - `premove.csv` - Premove execution signals and risk assessment
  - `explain.json` - Gate attribution and scoring explanations

### Warm Scan Job (`scan.warm`)
- **Schedule**: Every 2 hours at minute 0 (`0 */2 * * *`)
- **Universe**: Remaining universe (post top-30)
- **Features**: Cached sources, lower QPS, score threshold ≥65
- **Gates**: Relaxed thresholds for discovery
- **Artifacts**:
  - `warm_signals.csv` - Cached scan results with TTL indicators

### Regime Refresh Job (`regime.refresh`)
- **Schedule**: Every 4 hours at minute 0 (`0 */4 * * *`)
- **Indicators**: 
  - Realized volatility (7-day)
  - % above 20MA (breadth)
  - Breadth thrust indicator
- **Logic**: 3-way majority vote → {calm, normal, volatile}
- **Output**: Weight blends cached for hot/warm scans
- **Artifacts**:
  - `regime.json` - Full regime detection with indicator breakdown and weight blends

## CLI Commands

### List Jobs
```bash
cryptorun schedule list
```
Shows all 3 enabled jobs with expected cadences:
```
📋 Scheduled Jobs (3)
JOB NAME             SCHEDULE        STATUS   DESCRIPTION
--------             --------        ------   -----------
scan.hot             */15 * * * *    enabled  Hot momentum + premove scan for top-30 ADV universe
scan.warm            0 */2 * * *     enabled  Warm scan for remaining universe with cached sources  
regime.refresh       0 */4 * * *     enabled  Refresh market regime detection
```

### Run Jobs Manually
```bash
# Dry-run testing
cryptorun schedule run scan.hot --dry-run
cryptorun schedule run scan.warm --dry-run  
cryptorun schedule run regime.refresh --dry-run

# Live execution
cryptorun schedule run scan.hot
```

### Start Scheduler Daemon
```bash
cryptorun schedule start
```

### Check Status
```bash
cryptorun schedule status
```

## Artifact Structure

### Hot Scan Signals (`signals.csv`)
```csv
timestamp,symbol,score,momentum_core,vadr,spread_bps,depth_usd,regime,fresh,venue,sources
2025-09-07T11:15:13Z,BTC/USD,78.5,65.2,2.1,15,150000,normal,●,kraken,3
2025-09-07T11:15:13Z,ETH/USD,82.1,71.8,1.9,12,200000,normal,●,okx,4
```

### Regime Detection (`regime.json`)
```json
{
  "timestamp": "2025-09-07T11:15:13Z",
  "regime": "normal",
  "indicators": {
    "realized_vol_7d": 0.35,
    "pct_above_20ma": 65,
    "breadth_thrust": 0.42
  },
  "confidence": 0.85,
  "votes": {
    "vol_vote": "normal",
    "ma_vote": "normal", 
    "breadth_vote": "normal"
  },
  "weight_blend": {
    "momentum": 0.35,
    "technical": 0.25,
    "volume": 0.25,
    "quality": 0.15
  },
  "next_refresh": "2025-09-07T15:15:13Z"
}
```

## Technical Implementation

### Scheduler Engine
- **Location**: `internal/scheduler/scheduler.go`
- **Config**: `config/scheduler.yaml`
- **Integration**: Uses existing `internal/application` scan pipelines

### VADR Freeze Logic
- VADR calculation frozen when <20 bars of data available
- No aggregator microstructure calls enforced at compile time
- Exchange-native L1/L2 data precedence: max(p80, tier_min)

### Regime Weight Blending
```go
// Calm regime (low vol, strong trend)
"momentum": 0.4, "technical": 0.3, "volume": 0.2, "quality": 0.1

// Normal regime (balanced conditions)  
"momentum": 0.35, "technical": 0.25, "volume": 0.25, "quality": 0.15

// Volatile regime (high vol, weak breadth)
"momentum": 0.3, "technical": 0.2, "volume": 0.3, "quality": 0.2
```

## Configuration

Scheduler configuration in `config/scheduler.yaml`:
```yaml
global:
  artifacts_dir: "artifacts/signals"
  log_level: "info"
  timezone: "UTC"

jobs:
  - name: "scan.hot"
    schedule: "*/15 * * * *"
    type: "scan.hot" 
    enabled: true
    config:
      universe: "top30"
      venues: ["kraken", "okx", "coinbase"]
      max_sample: 30
      premove: true
      
  - name: "scan.warm"
    schedule: "0 */2 * * *"
    type: "scan.warm"
    enabled: true
    config:
      universe: "remaining"
      venues: ["kraken"]
      max_sample: 100
      premove: false
      
  - name: "regime.refresh"
    schedule: "0 */4 * * *"
    type: "regime.refresh"
    enabled: true
```

## UX MUST — Live Progress & Explainability

All scheduler operations provide real-time feedback:
- **Job execution**: Live logging with structured output
- **Progress indicators**: Duration tracking and artifact counts
- **Gate attribution**: Deterministic reasons for entry/rejection
- **Regime explanations**: Full indicator breakdown with confidence scores
- **CLI headers**: Show current regime, API health, latency, and source counts

## Acceptance Criteria

✅ **3 Enabled Jobs**: `cryptorun schedule list` shows scan.hot (15m), scan.warm (2h), regime.refresh (4h)  
✅ **Hot Loop Output**: Top-N rows with [Fresh ●] [Depth ✓] [Venue] [Sources n] columns  
✅ **Deterministic Gates**: Clear reasons for entry/rejection in explain.json  
✅ **VADR Freeze**: <20 bars detection implemented  
✅ **No Aggregators**: Compile-time enforcement of venue-native microstructure  
✅ **Artifact Emission**: Timestamped artifacts per run cycle