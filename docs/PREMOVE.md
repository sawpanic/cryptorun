# Pre-Movement Detection System

CryptoRun's pre-movement detector implements v3.3 specifications with hourly 2-of-3 gate analysis for early momentum identification.

> **Status:** Production scheduler integration  
> **Model:** 100-point composite with 2-of-3 gates  
> **Scope:** Hourly coiled-spring detection with alerts

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

## Percentile Engine (14d/30d, Winsorized) ✅ **ENHANCED v3.3**

The v3.3 percentile engine provides rolling percentile calculations with improved precision:

- **14-day window:** Minimum 10 samples, used for short-term VADR p80 thresholds
- **30-day window:** Minimum 20 samples, used for longer-term volume surge detection
- **Enhanced sampling:** Default minimum 20 samples for robust statistical inference
- **Winsorization:** All values are winsorized at ±3σ before percentile calculation to reduce outlier impact
- **NaN handling:** Invalid values (NaN, Inf) are automatically filtered out
- **Context support:** Full context.Context support for cancellation and timeouts
- **Time-aware processing:** Rolling windows with proper timestamp-based filtering

The engine uses linear interpolation for percentile calculation with enhanced precision for P10/P25/P50/P75/P90 calculations, optimized for real-time streaming data with proper sample size validation.

## CVD Residuals (Robust Fit, R² Fallback) ✅ **ENHANCED v3.3**

CVD (Cumulative Volume Delta) residualization removes the systematic relationship between signed dollar flow and total volume with enhanced robustness:

- **Robust regression:** Uses iterative reweighted least squares (IRLS) with Tukey's biweight function for superior outlier resistance
- **Minimum samples:** Requires at least 200 observations for reliable regression estimates
- **R² threshold:** Requires R² ≥ 0.30 for acceptable fit quality; otherwise falls back to raw CVD values
- **Residuals formula:** `residuals = cvd_norm - β*vol_norm` where β is the robust slope estimate
- **Rolling windows:** Time-varying residuals with 200-sample rolling windows for adaptive estimation
- **Enhanced fallback:** Context-aware fallback logic with proper error handling and cancellation support
- **Quality tracking:** Per-window R² and beta coefficient tracking for signal quality monitoring

When R² < 0.30 or insufficient samples, the system returns the original CVD values as fallback with proper validity flags for downstream processing.

## Supply-Squeeze Proxy (Gates A-C) ✅ **ENHANCED v3.3**

The v3.3 supply-squeeze proxy uses three refined gates for cleaner evaluation logic:

**Gate A - Funding + Spot Positioning (weight 40%):**
- `funding_z < -1.5` AND `spot_price > vwap_24h`
- Both conditions must be satisfied for Gate A to pass
- Indicates futures-spot basis compression with spot strength

**Gate B - Reserves + Whale Accumulation (weight 35%):**
- `exchange_reserves_7d <= -5%` OR `whale_accum_2of3`
- At least one condition must be satisfied for Gate B to pass
- Primary supply constraint or accumulation signal

**Gate C - Volume Confirmation (weight 25%):**
- `volume_first_bar_p80` checks if initial 15m volume bar ≥ p80(vadr_24h)
- Required for gate passage, provides momentum confirmation

**Regime-Based Volume Requirements:**
- **Risk-off/BTC-driven regimes:** Volume confirmation **required** for signal validity
- **Risk-on/Selective regimes:** Volume confirmation **not required** (optional scoring boost)
- **Conservative default:** Unknown regimes require volume confirmation

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

## Portfolio Pruner

Risk management is applied after scoring and gates but before alerts: ✅ **IMPLEMENTED v3.3**

**Constraint Enforcement:**
- **Pairwise correlation:** ≤ 0.65 maximum (Pearson correlation with exchange-native data)
- **Sector concentration:** ≤ 2 positions per sector (configurable per sector: DeFi, Layer1, Layer2, Meme, AI, Gaming, Infrastructure)
- **Beta budget:** ≤ 2.0 exposure limit to BTC (adjustable per portfolio constraints)
- **Position sizing:** Individual positions ≤ 5%, total exposure ≤ 20%
- **Greedy selection:** Candidates sorted by score (highest first) for optimal allocation

**Pruning Pipeline:**
1. Sort candidates by composite score (descending)
2. Apply constraints sequentially: correlation → sector → beta → position size → total exposure
3. Generate detailed rejection reasons for transparency
4. Provide utilization metrics (beta utilization %, exposure utilization %)

**Output:**
- **Accepted:** Candidates passing all constraints
- **Rejected:** Candidates with specific rejection reasons
- **Summary:** Total counts, utilization metrics, top rejection reason

## Alerts Governance

Rate-limited alerting with quality control prevents operator fatigue: ✅ **IMPLEMENTED v3.3**

**Rate Limiting:**
- **Standard rates:** 3 per hour, 10 per day per symbol
- **High volatility:** 6 per hour during volatile market conditions
- **Time-based cleanup:** Alert history older than 24 hours automatically purged

**Manual Override System:**
- **Trigger condition:** `score ≥ 90 && passed_gates ≤ 2`
- **Action:** Alert-only mode (no execution, notifications only)
- **Reason:** High-quality signals that barely miss gate requirements

**Priority Classification:**
- **High priority:** Score ≥ 85, gates ≥ 3
- **Medium priority:** Score ≥ 75, gates ≥ 2
- **Low priority:** Below medium thresholds

**Governance Features:**
- **Per-symbol tracking:** Independent rate limits for each trading pair
- **Real-time statistics:** Hourly/daily counts, utilization percentages
- **Alert history:** Full audit trail with timestamps and decisions
- **SSE Integration:** Real-time alert streaming via Server-Sent Events (≤1 Hz throttled)

## Execution Quality + SSE Throttling

Comprehensive execution monitoring with adaptive thresholds: ✅ **IMPLEMENTED v3.3**

**Slippage Monitoring:**
- **Quality classification:** Good (≤10 bps), Acceptable (10-30 bps), Bad (>30 bps)
- **Directional calculations:** Buy executions penalized for higher prices, sells for lower prices
- **Real-time tracking:** All executions recorded with timestamps and venue attribution

**Venue Tightening System:**
- **Trigger condition:** Slippage > 30 bps triggers immediate venue tightening
- **Recovery criteria:** 20 consecutive good trades OR 48-hour time window
- **Per-venue tracking:** Independent statistics and tightening status per execution venue

**Quality Metrics:**
- **Execution rates:** Good execution rate percentage, average slippage in basis points
- **Venue breakdown:** Per-venue statistics (total, good, bad executions)
- **Recovery progress:** Tracking consecutive good trades and time-based recovery windows
- **Recent history:** Last 10 executions maintained for monitoring and debugging

**SSE Live Dashboard:**
- **Throttled updates:** ≤1 Hz refresh rate to prevent client overload
- **Real-time transitions:** State changes, execution records, alert decisions
- **Subscriber management:** Multi-client support with symbol filtering and connection tracking
- **Board monitoring:** Comprehensive premove board with portfolio, alerts, and execution summaries

## Guard-CI Compliance

Automated compliance testing with Guard-CI build tags: ✅ **IMPLEMENTED v3.3**

**Guard-CI Stubs:**
- **Unified Guard-CI:** `src/guardci/unified_guardci.go` with `//go:build guard_ci` tag
- **Explainer Guard-CI:** `src/guardci/explainer_guardci.go` for scoring transparency
- **Noop implementations:** Allow `go build -tags guard_ci ./...` to pass without external dependencies

**Compliance Checks:**
- **Portfolio constraints:** Validate correlation limits, sector caps, beta budgets
- **Alerts governance:** Verify rate limiting and manual override logic
- **Execution quality:** Check slippage thresholds and recovery mechanisms
- **SSE throttling:** Validate update frequency controls and subscriber limits

**Build Integration:**
- **CI compatibility:** Guard-CI builds succeed without real market data dependencies
- **Test coverage:** All major compliance areas covered with deterministic results
- **Configuration validation:** Ensures system operates within defined constraints

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

## v3.3 Part 1 Implementation Status ✅

### Core Mathematical Engines

**Percentile Engine** (`src/infrastructure/percentiles/`)
- ✅ **Time-aware rolling windows**: 14d/30d with timestamp-based filtering
- ✅ **Winsorization**: ±3σ outlier clamping before percentile calculation
- ✅ **Linear interpolation**: Precise percentile values with enhanced accuracy
- ✅ **Context support**: Full `context.Context` integration for cancellation
- ✅ **NaN/Inf handling**: Automatic filtering of invalid values
- ✅ **Sample validation**: Minimum sample requirements (10 for 14d, 20 for 30d)

**CVD Residuals Engine** (`src/domain/premove/cvd/`)
- ✅ **Robust regression**: IRLS with Huber weights for outlier resistance  
- ✅ **Statistical validation**: Minimum 200 samples, R² ≥ 0.30 threshold
- ✅ **Graceful fallback**: Returns original CVD data when regression fails
- ✅ **Rolling windows**: 200-sample rolling estimation for adaptivity
- ✅ **Quality tracking**: Per-window R² and beta coefficient monitoring
- ✅ **Context awareness**: Cancellation support for long-running calculations

**Supply-Squeeze Proxy** (`src/domain/premove/proxy/`)
- ✅ **Three-gate system**: Gates A (40%), B (35%), C (25%) with weighted evaluation
- ✅ **Gate A logic**: Funding Z-score < -1.5 AND Spot > VWAP24h 
- ✅ **Gate B logic**: Exchange reserves ≤-5% OR whale accumulation 2-of-3
- ✅ **Gate C logic**: Volume first bar ≥ P80(VADR 24h) threshold
- ✅ **Regime awareness**: Volume confirmation required for risk-off/BTC-driven regimes
- ✅ **Detailed evaluation**: Comprehensive result structure with explanations

### Runner Integration

**Dependency Injection** (`src/application/premove/runner.go`)
- ✅ **Clean architecture**: Well-defined interfaces and separation of concerns
- ✅ **Engine orchestration**: Unified access to all mathematical engines
- ✅ **Status monitoring**: Real-time engine health and configuration reporting
- ✅ **Error handling**: Comprehensive error collection and reporting
- ✅ **Testing support**: Exported dependencies for comprehensive testing

### Portfolio & Alerts Systems

**Portfolio Management** (`src/application/premove/portfolio.go`, `src/domain/premove/portfolio/`)
- ✅ **Constraint enforcement**: Correlation ≤0.65, sector caps, beta budgets
- ✅ **Greedy selection**: Score-based candidate prioritization
- ✅ **Detailed reporting**: Acceptance/rejection with specific reasons
- ✅ **Utilization tracking**: Real-time constraint utilization metrics

**Alerts Governance** (`src/application/premove/alerts.go`)
- ✅ **Rate limiting**: 3/hr, 10/day with high-volatility overrides
- ✅ **Manual override**: Score ≥90, gates ≤2 → alert-only mode
- ✅ **Priority classification**: High/medium/low based on score and gates
- ✅ **Real-time statistics**: Comprehensive alert tracking and reporting
- ✅ **History management**: Automatic cleanup of stale alert records

**Execution Quality** (`src/application/premove/execution.go`)
- ✅ **Slippage classification**: Good (≤10bps), acceptable (10-30bps), bad (>30bps)
- ✅ **Venue tightening**: Automatic threshold adjustment after bad executions
- ✅ **Recovery tracking**: 20 good trades OR 48h for threshold recovery
- ✅ **Comprehensive metrics**: Per-venue statistics and quality tracking

### Test Coverage

**Unit Tests** (`tests/unit/premove/`)
- ✅ **Runner integration**: Dependency injection and engine wiring tests
- ✅ **CVD residuals**: Known beta, short series, R² fallback scenarios
- ✅ **Percentile engine**: Winsorization, time windows, NaN handling
- ✅ **Edge cases**: Insufficient samples, mismatched inputs, error conditions
- ✅ **Mock implementations**: Clean test doubles for all external dependencies

### Implementation Quality

- ✅ **Context-first design**: All engines support cancellation and timeouts
- ✅ **Error transparency**: Clear error messages with actionable information  
- ✅ **Graceful degradation**: Fallback strategies prevent total failure
- ✅ **Memory efficiency**: Bounded memory usage with rolling windows
- ✅ **Type safety**: Strong typing with clear interfaces and contracts
- ✅ **Documentation**: Comprehensive inline documentation and examples

---

*CryptoRun Pre-Movement Detector — Real-time coiled-spring detection with robust risk controls*