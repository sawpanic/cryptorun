# CryptoRun Scheduler System

CryptoRun's production scheduler backbone provides automated execution of 5 core workflows with configurable cadences and comprehensive output. Built for CryptoRun v3.2.1 MVP.

## Overview

The scheduler manages five production-ready jobs:
- **Hot Scan** (*/15m): Top-30 ADV universe with momentum + premove analysis
- **Warm Scan** (*/2h): Remaining universe with cached sources and lower QPS  
- **Regime Refresh** (*/4h): Market regime detection with 3-indicator majority vote
- **Provider Health** (*/5m): Rate limits, circuit breakers, fallback chains
- **Premove Hourly** (*/1h): 2-of-3 gate enforcement with volume confirmation

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    CryptoRun Production Scheduler MVP                                          │
├─────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐     │
│  │   Hot Scan      │  │   Warm Scan     │  │ Regime Refresh  │  │Provider Health  │  │Premove Hourly   │     │
│  │   */15 * * * *  │  │   0 */2 * * *   │  │   0 */4 * * *   │  │   */5 * * * *   │  │   0 * * * *     │     │
│  │                 │  │                 │  │                 │  │                 │  │                 │     │
│  │ • Top-30 ADV    │  │ • Remaining     │  │ • Realized Vol  │  │ • Rate Limits   │  │ • 2-of-3 Gates  │     │
│  │ • Momentum +    │  │   Universe      │  │ • %>20MA        │  │ • Circuit       │  │ • Volume Confirm│     │
│  │   Premove       │  │ • Cached Sources│  │ • Breadth Thrust│  │   Breakers      │  │ • Risk_off/BTC  │     │
│  │ • Regime-aware  │  │ • Lower QPS     │  │ • Majority Vote │  │ • Fallback      │  │   Driven        │     │
│  │   Weights       │  │ • Score >= 65   │  │ • Weight Blends │  │   Chains        │  │ • Alert Engine │     │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘     │
│           │                       │                       │                       │                   │       │
│           └───────────────────────┼───────────────────────┼───────────────────────┼───────────────────┘       │
│                                   │                       │                       │                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│  │                                    Artifact Engine                                                        │
│  └─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                   │                                                         │
│  artifacts/signals/{timestamp}/                   │ [Fresh ●] [Depth ✓] [Venue] [Sources n]               │
│  ├── signals.csv                                  │ columns + deterministic gate reasons                  │
│  ├── premove.csv                                  │                                                         │
│  ├── warm_signals.csv                             │ Provider health metrics + fallback                    │
│  ├── explain.json                                 │ chains + cache TTL doubling                           │
│  ├── regime.json                                  │                                                         │
│  ├── health.json                                  │ Hourly premove alerts with 2-of-3                     │
│  └── premove_alerts.json                          │ gate enforcement logic                                 │
│                                                   │                                                         │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
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

### Provider Health Job (`providers.health`)
- **Schedule**: Every 5 minutes (`*/5 * * * *`)
- **Monitors**: Rate limits, response times, error rates, circuit breaker states
- **Fallback Logic**: 
  - Unhealthy provider → fallback chain (okx→coinbase, binance→okx)
  - Usage >80% or circuit OPEN → double cache TTL
- **Artifacts**:
  - `health.json` - Provider status with fallback assignments and TTL adjustments

### Premove Hourly Job (`premove.hourly`)
- **Schedule**: Every hour at minute 0 (`0 * * * *`)
- **Universe**: Top-50 ADV pairs with comprehensive gate enforcement
- **Gate Logic**: 2-of-3 enforcement on [funding_divergence, supply_squeeze, whale_accumulation]
- **Volume Confirmation**: Required in risk_off/btc_driven regime
- **Features**:
  - funding_divergence: Score ≥2.0
  - supply_squeeze: Quality >70 AND depth <80k USD
  - whale_accumulation: Volume >75 AND momentum >70
- **Artifacts**:
  - `premove_alerts.json` - Filtered alerts with gate attribution and volume confirmation status

## CLI Commands

### List Jobs
```bash
cryptorun schedule list
```
Shows all 5 enabled jobs with expected cadences and health banner:
```
🚀 CryptoRun Scheduler MVP
Regime: normal | Latency: avg 140ms | Fallbacks: 1 active
API Health: kraken ✓ (150ms) | okx ✗ (180ms) | coinbase ✓ (120ms) | binance ✓ (110ms)
Last Update: 14:05:13 UTC

📋 Scheduled Jobs (5)
JOB NAME             SCHEDULE        STATUS                                    DESCRIPTION
--------             --------        ------                                    -----------
scan.hot             */15 * * * *    ✓ enabled [Fresh ●] [Depth ✓] [Venues 3] [Sources n]  Hot momentum + premove scans with regime-aware weights on top30 ADV
scan.warm            0 */2 * * *     ✓ enabled                                 Warm scan with cached sources on remaining universe, lower QPS
regime.refresh       0 */4 * * *     ✓ enabled                                 Refresh regime with realized_vol_7d, %>20MA, breadth thrust; majority vote → cached regime + weight blend
providers.health     */5 * * * *     ✓ enabled                                 Monitor provider health, rate-limits, circuit breakers, fallbacks; double cache_ttl on degradation
premove.hourly       0 * * * *       ✓ enabled                                 Hourly premove sweep with 2-of-3 gate enforcement; require volume confirm in risk_off/btc_driven
```

### Run Jobs Manually
```bash
# Dry-run testing
cryptorun schedule run scan.hot --dry-run
cryptorun schedule run scan.warm --dry-run  
cryptorun schedule run regime.refresh --dry-run
cryptorun schedule run providers.health --dry-run
cryptorun schedule run premove.hourly --dry-run

# Live execution
cryptorun schedule run scan.hot
cryptorun schedule run premove.hourly
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

### Provider Health Status (`health.json`)
```json
{
  "timestamp": "2025-09-07T11:15:13Z",
  "providers": [
    {
      "provider": "kraken",
      "healthy": true,
      "response_time": 150,
      "rate_limit": {"used": 450, "limit": 1000, "usage": 45.0},
      "circuit_state": "CLOSED",
      "error_rate": 0.02,
      "cache_ttl": 300,
      "fallback": ""
    },
    {
      "provider": "okx", 
      "healthy": false,
      "response_time": 280,
      "rate_limit": {"used": 950, "limit": 1000, "usage": 95.0},
      "circuit_state": "OPEN",
      "error_rate": 0.15,
      "cache_ttl": 600,
      "fallback": "coinbase"
    }
  ],
  "next_check": "2025-09-07T11:20:13Z"
}
```

### Premove Alerts (`premove_alerts.json`)
```json
{
  "timestamp": "2025-09-07T11:00:13Z",
  "regime": "normal",
  "alerts": [
    {
      "symbol": "BTC/USD",
      "total_score": 78.5,
      "gates_passed": ["funding_divergence", "supply_squeeze", "whale_accumulation"],
      "gate_scores": {
        "funding_divergence": 2.3,
        "supply_squeeze": {"quality": 72, "depth_usd": 75000},
        "whale_accumulation": {"volume": 78, "momentum": 73}
      },
      "volume_confirmed": true,
      "risk_level": "medium"
    }
  ],
  "stats": {
    "total_candidates": 50,
    "gates_passed": 12,
    "alerts_generated": 1,
    "volume_confirmations": 1
  }
}
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

Complete scheduler configuration in `config/scheduler.yaml`:
```yaml
global:
  artifacts_dir: "artifacts"
  log_level: "info"
  timezone: "UTC"

jobs:
  # Hot scan: Top-30 ADV universe with momentum + premove every 15 minutes
  - name: "scan.hot"
    schedule: "*/15 * * * *"
    type: "scan.hot" 
    description: "Hot momentum + premove scans with regime-aware weights on top30 ADV"
    enabled: true
    config:
      universe: "top30"
      venues: ["kraken", "okx", "coinbase"]
      max_sample: 30
      ttl: 300
      top_n: 10
      premove: true
      regime_aware: true
      output_dir: "signals"
      
  # Warm scan: Remaining universe with cached sources every 2 hours  
  - name: "scan.warm"
    schedule: "0 */2 * * *"
    type: "scan.warm"
    description: "Warm scan with cached sources on remaining universe, lower QPS"
    enabled: true
    config:
      universe: "remaining"
      venues: ["kraken", "okx"]
      max_sample: 100
      ttl: 1800
      top_n: 20
      premove: false
      output_dir: "warm_signals"
      
  # Regime refresh: Detect market regime every 4 hours
  - name: "regime.refresh"
    schedule: "0 */4 * * *"
    type: "regime.refresh"
    description: "Refresh regime with realized_vol_7d, %>20MA, breadth thrust; majority vote → cached regime + weight blend"
    enabled: true
    config:
      universe: "top100"
      venues: ["kraken", "coinbase"]
      max_sample: 100
      ttl: 3600
      output_dir: "regime"

  # Provider health monitoring: Every 5 minutes
  - name: "providers.health"
    schedule: "*/5 * * * *"
    type: "providers.health"
    description: "Monitor provider health, rate-limits, circuit breakers, fallbacks; double cache_ttl on degradation"
    enabled: true
    config:
      venues: ["kraken", "okx", "coinbase", "binance"]
      ttl: 300
      output_dir: "health"

  # Premove hourly sweep: Every hour with 2-of-3 gate enforcement
  - name: "premove.hourly"
    schedule: "0 * * * *"
    type: "premove.hourly"
    description: "Hourly premove sweep with 2-of-3 gate enforcement; require volume confirm in risk_off/btc_driven"
    enabled: true
    config:
      universe: "top50"
      venues: ["kraken", "okx", "coinbase"]
      max_sample: 50
      ttl: 600
      top_n: 15
      output_dir: "premove"
      require_gates: ["funding_divergence", "supply_squeeze", "whale_accumulation"]
      min_gates_passed: 2
      regime_aware: true
      volume_confirm: true
```

## UX MUST — Live Progress & Explainability

All scheduler operations provide real-time feedback:
- **Job execution**: Live logging with structured output
- **Progress indicators**: Duration tracking and artifact counts
- **Gate attribution**: Deterministic reasons for entry/rejection
- **Regime explanations**: Full indicator breakdown with confidence scores
- **CLI headers**: Show current regime, API health, latency, and source counts

## Implementation Status

### Scheduler Engine (`internal/scheduler/scheduler.go`)
✅ **Core Implementation**: Complete with all 5 job types  
✅ **Cron Integration**: Proper schedule parsing and execution  
✅ **Provider Health Monitoring**: Rate limits, circuit breakers, fallback chains  
✅ **Regime Detection Logic**: 3-indicator majority voting with weight blends  
✅ **Premove Gate Enforcement**: 2-of-3 gate logic with volume confirmation  
✅ **CLI Integration**: Health banners and job management commands  

### Test Coverage (`tests/unit/scheduler/scheduler_test.go`)
✅ **Gate Combinations**: 6 test cases for 2-of-3 enforcement logic  
✅ **Provider Fallback**: 4 test cases for health monitoring and TTL doubling  
✅ **Regime Voting**: 4 test cases for majority vote logic  
✅ **Job Configuration**: YAML parsing and validation tests  

### Configuration (`config/scheduler.yaml`)
✅ **5 Production Jobs**: All jobs configured with proper schedules and descriptions  
✅ **Gate Requirements**: funding_divergence, supply_squeeze, whale_accumulation definitions  
✅ **Volume Confirmation**: Regime-aware volume confirm in risk_off/btc_driven  
✅ **Artifact Organization**: Separate output directories per job type  

## Acceptance Criteria

✅ **5 Enabled Jobs**: `cryptorun schedule list` shows all jobs with health banner  
✅ **Hot Loop Output**: Top-N rows with [Fresh ●] [Depth ✓] [Venue] [Sources n] columns  
✅ **Provider Fallback**: Unhealthy providers trigger fallback chains (okx→coinbase, binance→okx)  
✅ **Cache TTL Doubling**: High usage (>80%) or circuit OPEN doubles cache TTL  
✅ **2-of-3 Gate Enforcement**: Premove alerts require minimum 2 gates passed  
✅ **Volume Confirmation**: Required in risk_off/btc_driven regime  
✅ **Deterministic Gates**: Clear reasons for entry/rejection with gate attribution  
✅ **VADR Freeze**: <20 bars detection implemented  
✅ **No Aggregators**: Compile-time enforcement of venue-native microstructure  
✅ **Artifact Emission**: Timestamped artifacts per run cycle with comprehensive schemas