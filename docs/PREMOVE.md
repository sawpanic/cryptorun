# Pre-Movement Detector v3.3 — Feature Specification

> **Status:** Menu-only, Top-100 universe  
> **Model:** 100-point composite with 2-of-3 gates  
> **Scope:** Coiled-spring detection before breakouts

## UX MUST — Live Progress & Explainability

The Pre-Movement Detector provides real-time insight into "coiled-spring" setups before major price movements through:
- **Live state transitions** (QUIET → WATCH → PREPARE → PRIME → EXECUTE)
- **Detailed attribution** showing which factors contribute most to the composite score
- **Gate status visualization** indicating which of the 2-of-3 critical gates are satisfied
- **Portfolio-level risk controls** with correlation, sector, and beta budget management
- **SSE-Throttled Live Dashboard** (≤1 Hz updates) accessible via Monitor Menu > PreMove Detection Board
- **Interactive Console Interface** with real-time candidate display and manual refresh capabilities

---

## Overview

The Pre-Movement Detector identifies assets showing signs of accumulation and preparation before significant price movements (>5% in 48h). It uses a 100-point composite scoring system across three main categories:

- **Structural (45 pts):** Derivatives, microstructure, exchange flows
- **Behavioral (30 pts):** Whale activity, CVD residuals, volume profiling  
- **Catalyst & Compression (25 pts):** Technical squeeze indicators, event catalysts

## Scoring States

| Score Range | State    | Description                    |
|-------------|----------|--------------------------------|
| < 60        | QUIET    | Low activity, monitoring only  |
| 60-79       | WATCH    | Initial interest detected      |
| 80-99       | PREPARE  | Building momentum              |
| 100-119     | PRIME    | High probability setup         |
| ≥ 120       | EXECUTE  | Maximum conviction             |

## Critical Gates (2-of-3 Required)

The detector requires at least 2 of the following 3 gates to be satisfied:

1. **Funding Divergence:** Perp funding rate diverges from price action
2. **Supply Squeeze:** Primary reserves declining or proxy indicators showing scarcity  
3. **Accumulation:** CVD residual + iceberg detection ≥ 80th percentile

In risk-off or BTC-driven regimes, **volume confirmation** provides additional conviction but doesn't replace the 2-of-3 requirement.

## Regime Adaptivity

The detector adjusts its sensitivity and decay rates based on market regime:

| Regime     | Half-life | Characteristics              |
|------------|-----------|------------------------------|
| Risk-on    | 8h        | High risk appetite           |
| Selective  | 6h        | Mixed signals                |
| BTC-driven | 5h        | Correlated moves             |
| Risk-off   | 4h        | Flight to quality            |

## Data Freshness & Penalties

Freshness penalties are applied based on data staleness across all feeds:

- **Soft penalty:** Starts at 8 seconds
- **Exponential decay:** τ = 30 seconds  
- **Hard failure:** 90 seconds (worst-feed precedence) ✅ **IMPLEMENTED v3.3**
- **Feeds monitored:** Funding, trades, depth, basis

## Portfolio Controls

Risk management is applied after scoring and gates but before alerts: ✅ **IMPLEMENTED v3.3**

- **Pairwise correlation:** ≤ 0.65 maximum (Pearson correlation with exchange-native data)
- **Sector concentration:** ≤ 2 positions per sector  
- **Beta budget:** ≤ 15.0 exposure limit (adjustable per portfolio constraints)
- **Position sizing:** Dynamic based on beta utilization and correlation matrix
- **Tie-breaking:** By composite score then symbol

## Alert Governance

Rate-limited alerting prevents operator fatigue: ✅ **IMPLEMENTED v3.3**

- **Standard rates:** 3 per hour, 10 per day (configurable with volatility allowance)
- **High volatility:** Additional allowance for critical/high severity alerts
- **Manual override:** Support for emergency alert-only mode with duration controls
- **SSE Integration:** Real-time alert streaming via Server-Sent Events (≤1 Hz throttled)

## Execution Quality Tracking

The system monitors execution performance and adapts: ✅ **IMPLEMENTED v3.3**

- **Slippage monitoring:** Tracks intended vs actual fills with BPS calculations
- **Fill time quality:** Comprehensive quality scoring (0-100) across slippage, time, and size dimensions
- **Recovery mode:** Automatic trigger after consecutive failures with cooldown periods
- **Performance metrics:** P95/median tracking, acceptable rate percentages, quality score aggregation

## Calibration & Governance

### Point-in-Time Replay

The backtest harness processes historical artifacts from `artifacts/premove/*.jsonl` to compute: ✅ **IMPLEMENTED v3.3**

- **Hit rates by state and regime** for model validation with isotonic calibration
- **Daily CVD residual R² scores** to monitor signal quality degradation
- **Performance attribution** across different market conditions and asset sectors
- **PIT replay system** with deterministic fixture processing and monotonic curve generation

### Isotonic Calibration

The system maintains a monotonic mapping from composite scores to movement probabilities:

- **Calibration curve:** Score → P(move > 5% in 48h)
- **Monthly refresh:** Automatic recalibration using trailing data
- **Immutability guarantee:** No changes during freeze windows
- **Governance process:** Propose → backtest → paper trade → gradual rollout

### Freeze Windows & Change Control

- **Minimum stability:** 30 days between material changes
- **Emergency exceptions:** Critical bugs or abort conditions only
- **Change process:** All modifications require backtested validation
- **Burn-in period:** 30 days read-only operation for new versions

### Calibration Artifacts

The system produces three key calibration outputs:

1. **`hit_rates_by_state_and_regime.json`** — Success rates by detector state and market regime
2. **`isotonic_calibration_curve.json`** — Monotonic score-to-probability mapping  
3. **`cvd_resid_r2_daily.csv`** — Daily correlation quality for CVD residual signals

These artifacts enable:
- Model validation and performance tracking
- Confidence interval estimation for predictions
- Signal degradation detection and recovery
- Governance decision support for parameter changes

## Data Sources

| Signal Type        | Primary Source           | Fallback            | Update Frequency |
|--------------------|--------------------------|---------------------|------------------|
| Price/Volume       | Exchange WebSocket       | Exchange REST       | Real-time/30s    |
| Depth/Spread       | Exchange native only     | —                   | Real-time/15s    |
| Funding rates      | Exchange REST            | Alt exchange        | 5-10 minutes     |
| Open interest      | Exchange REST            | —                   | 5-10 minutes     |
| Options data       | Deribit public API       | —                   | 10 minutes       |
| Catalysts          | CoinMarketCal, DefiLlama | TokenUnlocks        | 30-60 minutes    |

**Note:** Microstructure data (depth, spread) uses **exchange-native feeds only** — no aggregators.

## Metrics & Monitoring

Prometheus metrics exposed at `/metrics`:

- `premove_score{symbol}` — Current composite score
- `premove_state{symbol}` — Detector state (QUIET/WATCH/PREPARE/PRIME/EXECUTE)  
- `premove_gate_count{symbol}` — Number of gates satisfied
- `premove_data_staleness_seconds{feed}` — Data freshness by source
- `premove_slippage_bps` — Execution slippage tracking
- `premove_alerts_rate_limited_total` — Rate limiting activity
- `premove_portfolio_pruned_total` — Risk control interventions

## Configuration

Key configuration parameters in `config/premove.yaml`:

### Portfolio Risk Management
```yaml
portfolio:
  pairwise_corr_max: 0.65              # Maximum correlation between positions  
  sector_caps: { L1: 2, DeFi: 2 }      # Position limits per sector
  beta_budget_to_btc: 2.0              # Beta exposure budget
  max_single_position_pct: 5           # Maximum single position size
  max_total_exposure_pct: 20           # Maximum total exposure
  apply_stage: post_gates_pre_alerts   # When pruning occurs in pipeline
```

### Alert Governance
```yaml
alerts:
  per_hour: 3                          # Standard hourly rate limit
  per_day: 10                          # Daily rate limit
  high_vol_per_hour: 6                 # Increased limit during volatility
  manual_override:                     # Emergency override settings
    condition: "score>90 && gates<2"   # Override trigger condition
    mode: alert_only                   # Override behavior mode
```

### Execution Quality
```yaml
execution_quality:
  slippage_bps_tighten_threshold: 30   # Tighten at 30 bps slippage
  recovery:                            # Quality recovery criteria
    good_trades: 20                    # Good trades needed for recovery
    hours: 48                          # OR time-based recovery
```

### Learning System
```yaml
learning:
  pattern_exhaustion:                  # Pattern degradation monitoring
    degrade_confidence_when_7d_lt_0_7_of_30d: true  # Confidence degradation rule
```

### Data Freshness
```yaml
decay: 
  freshness: { soft_start_s: 8, tau_s: 30, hard_fail_s: 90 }
```

---

*CryptoRun Pre-Movement Detector — Real-time coiled-spring detection with robust risk controls*