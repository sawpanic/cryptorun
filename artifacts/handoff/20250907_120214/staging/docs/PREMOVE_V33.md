# Pre-Movement v3.3 Intelligence Module

## UX MUST — Live Progress & Explainability

The Pre-Movement v3.3 system provides **real-time candidate analysis** with complete transparency into scoring, gate evaluation, and data quality. Every recommendation includes detailed attribution and confidence metrics to support operator decision-making.

**Live Progress Indicators:**
- 🔥 **STRONG**: Score ≥85 + 2-of-3 confirmations + significant CVD residual
- 📈 **MODERATE**: Score ≥75 + confirmations OR score ≥90 alone  
- 📊 **WEAK**: Below thresholds but not blocked
- ❌ **BLOCKED**: Failed confirmation gates or critical data issues

**Explainability Features:**
- Component-by-component score breakdown (derivatives, smart money, catalyst, etc.)
- Gate-by-gate confirmation details with precedence ranking
- CVD residual analysis with regression diagnostics and fallback explanations
- Data freshness tracking with "worst feed wins" penalty calculation
- Performance metrics and warnings for all analysis steps

---

## Architecture Overview

Pre-Movement v3.3 is a **standalone intelligence module** that analyzes cryptocurrency momentum using:

1. **100-Point Scoring System**: Multi-factor analysis across structural, behavioral, and catalyst dimensions
2. **2-of-3 Confirmation Gates**: Funding divergence, whale activity, and supply squeeze validation
3. **CVD Residual Analysis**: Robust regression with R² fallback for volume-price relationship anomalies
4. **Microstructure Consultation**: L1/L2 order book analysis for execution feasibility

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Pre-Movement v3.3 Engine                │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ Score Engine  │  │ Gate Eval    │  │ CVD Analyzer    │  │
│  │ (100 points)  │  │ (2-of-3)     │  │ (Regression)    │  │
│  └───────────────┘  └──────────────┘  └─────────────────┘  │
│                           │                                 │
│  ┌────────────────────────┴─────────────────────────────┐   │
│  │            Microstructure Consultation              │   │
│  │          (L1/L2 spreads, depth, VADR)               │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌────────────────────────┴─────────────────────────────┐   │
│  │                 ListCandidates API                  │   │
│  │         (Ranked alerts for UI/menu integration)     │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Integration Points

- **Read-Only API**: `ListCandidates(n int)` returns ranked candidates without modifying global scanner state
- **Microstructure Integration**: Reuses existing `internal/microstructure` APIs for L1/L2 validation
- **Menu-Only Deployment**: No CLI commands; accessed through interactive menu system
- **Data Independence**: Operates on provided data inputs without direct market connections

---

## 100-Point Scoring System

### Component Breakdown

| Category | Component | Weight | Description |
|----------|-----------|---------|-------------|
| **Structural (40pts)** | Derivatives | 15pts | Funding z-score, OI residuals, ETF flows |
| | Supply/Demand | 15pts | Reserve depletion, whale movements |
| | Microstructure | 10pts | L1/L2 order book stress indicators |
| **Behavioral (35pts)** | Smart Money | 20pts | Institutional flow patterns |
| | CVD Residual | 15pts | Volume-price relationship anomalies |
| **Catalyst (25pts)** | News/Events | 15pts | Catalyst heat with time-decay |
| | Compression | 10pts | Volatility compression percentiles |

### Scoring Thresholds

**Per-Component Calculation:**
- **Derivatives**: Funding z-score contributes 0-7pts (linear scale), OI residual 0-4pts ($1M = 4pts), ETF tint 0-4pts
- **Supply/Demand**: Reserve depletion 0-8pts (-20% = 8pts), whale composite 0-7pts  
- **Microstructure**: L1/L2 dynamics 0-10pts (linear scale from normalized stress)
- **Smart Money**: Institutional flow patterns 0-20pts (linear scale from flow composite)
- **CVD Residual**: Residual strength 0-15pts (absolute residual magnitude)
- **Catalyst**: News significance 0-15pts, volatility compression 0-10pts

**Freshness Penalty ("Worst Feed Wins"):**
- No penalty: Data ≤2 hours old
- Linear penalty: 0% to 20% for data 2-4 hours old
- Maximum 20% score reduction for stale data

---

## 2-of-3 Confirmation Gates

### Core Gate Logic

**Required**: 2 of 3 confirmations must pass:
1. **Funding Divergence**: Cross-venue funding z-score ≥2.0σ
2. **Whale Composite**: Large transaction activity ≥70%
3. **Supply Squeeze Proxy**: Composite score ≥60% from 2-of-4 components

### Supply Squeeze Proxy (2-of-4 Components)

| Component | Threshold | Weight | Description |
|-----------|-----------|---------|-------------|
| Reserve Depletion | ≤-5% | 0.3 | Cross-venue exchange reserves decline |
| Large Withdrawals | ≥$50M/24h | 0.25 | Abnormal withdrawal patterns |
| Staking Inflows | ≥$10M/24h | 0.2 | On-chain staking activity |
| Derivatives OI | ≥15% increase | 0.25 | Open interest expansion |

**Proxy Score Calculation:**
```
proxy_score = Σ(component_strength × weight) for passed components
Requires ≥2 components passing for valid proxy score
```

### Volume Confirmation (Regime-Dependent)

**Additive boost** in specific regimes:
- **Risk-Off**: Volume ≥2.5× average reduces requirement to 1-of-3 + volume
- **BTC-Driven**: Same volume boost logic applies
- **Normal**: No volume boost; strict 2-of-3 requirement

### Precedence Ranking

When multiple candidates pass gates, ranking by weighted precedence:
- **Funding Divergence**: 3.0 (highest priority - cross-venue signal)
- **Whale Composite**: 2.0 (medium priority - on-chain activity)  
- **Supply Squeeze**: 1.0 (lowest priority - proxy-based)
- **Volume Confirmation**: +0.5 (additive boost)

---

## CVD Residual Analysis

### Robust Regression Method

**Primary approach** when sufficient data available:

1. **Data Collection**: Minimum 50 CVD-price change pairs
2. **Winsorization**: Remove extreme outliers (5th-95th percentile bounds)
3. **Regression Fitting**: `CVD = β₀ + β₁ × PriceChange + ε`
4. **Quality Check**: Require R² ≥0.30 for model validity
5. **Residual Calculation**: Latest point residual vs. predicted CVD
6. **Significance Test**: |residual/std_error| ≥2.0σ threshold

### Fallback Methods

**When regression fails** (insufficient data, low R², etc.):

#### Percentile Method (Default Fallback)
- Use raw CVD value as "residual"
- Calculate percentile rank vs. recent 20-period lookback
- Significance threshold: ≥80th percentile

#### Z-Score Method (Alternative Fallback)  
- Calculate z-score vs. recent mean/std deviation
- Significance threshold: |z-score| ≥2.0σ

### Data Quality Monitoring

**Quality Metrics Tracked:**
- Points available vs. points used (after winsorization)
- Winsorized outlier percentage
- Data time span coverage
- Missing data gap analysis
- Model R² and standard error
- Compute time performance

**Fallback Triggers:**
- Insufficient data points (<50)
- Excessive outliers (>50% winsorized)
- Low model R² (<0.30)
- Regression fitting errors
- Performance timeouts (>200ms)

---

## Exact Thresholds & Configuration

### Default Thresholds (Production)

```yaml
# Scoring System
score_config:
  derivatives_weight: 15.0      # Funding, OI, ETF
  supply_demand_weight: 15.0    # Reserves, whales  
  microstructure_weight: 10.0   # L1/L2 stress
  smart_money_weight: 20.0      # Institutional flows
  cvd_residual_weight: 15.0     # Volume-price residual
  catalyst_weight: 15.0         # News/events
  compression_weight: 10.0      # Volatility compression
  max_freshness_hours: 2.0      # 2-hour freshness limit
  freshness_penalty_pct: 20.0   # Max 20% penalty

# Gate Evaluation  
gate_config:
  funding_divergence_threshold: 2.0    # 2.0σ z-score
  supply_squeeze_threshold: 0.6        # 60% proxy score
  whale_composite_threshold: 0.7       # 70% activity level
  
  # Supply squeeze components (2-of-4 required)
  reserve_depletion_threshold: -5.0    # -5% reserves
  large_withdrawals_threshold: 50e6    # $50M withdrawals
  staking_inflow_threshold: 10e6       # $10M staking
  derivatives_leverage_threshold: 15.0 # 15% OI increase
  
  # Volume confirmation
  volume_confirmation_enabled: true
  volume_confirmation_threshold: 2.5   # 2.5× average volume
  
  # Precedence weights
  funding_precedence: 3.0              # Highest priority
  whale_precedence: 2.0                # Medium priority
  supply_precedence: 1.0               # Lowest priority

# CVD Analysis
cvd_config:
  min_data_points: 50              # Minimum regression data
  winsorize_pct_lower: 5.0         # 5th percentile lower bound
  winsorize_pct_upper: 95.0        # 95th percentile upper bound  
  min_r_squared: 0.30              # Minimum R² for regression
  fallback_method: "percentile"    # Fallback analysis method
  fallback_lookback: 20            # Lookback periods
  fallback_threshold: 80.0         # 80th percentile threshold
  residual_min_std_dev: 2.0        # 2σ significance threshold

# Performance Limits
engine_config:
  max_candidates: 50               # Max candidates returned
  max_process_time_ms: 2000        # 2-second processing limit
  max_data_staleness: 1800         # 30-minute staleness limit
  stale_data_warning: 600          # 10-minute warning threshold
```

### Microstructure Tiers

**Consultation thresholds** (non-blocking for Pre-Movement):
- **Spread**: <50bps preferred
- **Depth**: ≥$100k within ±2% preferred  
- **VADR**: ≥1.8× preferred

---

## SOL Case Study Examples

### Example 1: Strong Pre-Movement Signal

**Solana (SOL-USD) - March 2024 Setup**

**Input Data:**
```json
{
  "symbol": "SOL-USD",
  "premove_data": {
    "funding_z_score": 3.2,           // Strong cross-venue divergence
    "oi_residual": 1.8e6,             // $1.8M OI anomaly  
    "etf_flow_tint": 0.75,            // 75% net ETF inflows
    "reserve_change_7d": -18.0,       // -18% exchange reserves
    "whale_composite": 0.85,          // 85% whale activity spike
    "micro_dynamics": 0.7,            // L1/L2 stress indicators
    "smart_money_flow": 0.8,          // 80% institutional accumulation
    "cvd_residual": 0.65,             // Strong CVD divergence
    "catalyst_heat": 0.9,             // Ecosystem upgrade news
    "vol_compression_rank": 0.92,     // 92nd percentile compression
    "oldest_feed_hours": 0.8          // Fresh data
  },
  "confirmation_data": {
    "funding_z_score": 3.2,           // ✅ Pass (>2.0)
    "whale_composite": 0.85,          // ✅ Pass (>0.7)  
    "supply_proxy_score": 0.78,       // ✅ Pass (calculated from components)
    "reserve_change_7d": -18.0,       // ✅ Reserve depletion
    "large_withdrawals_24h": 120e6,   // ✅ $120M withdrawals
    "staking_inflow_24h": 25e6,       // ✅ $25M staking
    "derivatives_oi_change": 28.0,    // ✅ 28% OI increase  
    "volume_ratio_24h": 4.2,          // Strong volume
    "current_regime": "risk_off"      // Enables volume boost
  }
}
```

**Analysis Output:**
```
🔥 STRONG CANDIDATE — SOL-USD (Rank #1)
Pre-Movement Score: 91.3/100 (freshness: A, 89ms)

Component Breakdown:
✅ derivatives: 14.2 pts (funding 7.0 + OI 3.6 + ETF 3.6)
✅ supply_demand: 13.8 pts (reserves 7.2 + whale 6.6)  
✅ microstructure: 7.0 pts
✅ smart_money: 16.0 pts
✅ cvd_residual: 9.8 pts
✅ catalyst: 13.5 pts (upgrade catalyst)
✅ compression: 9.2 pts

Gate Confirmation: ✅ CONFIRMED (3/2 gates +VOL, 6.5 precedence)
✅ funding_divergence: 3.2σ ≥ 2.0σ
✅ whale_composite: 0.85 ≥ 0.70  
✅ supply_squeeze: 0.78 ≥ 0.60 (4/4 components)
✅ volume_confirmation: 4.2× ≥ 2.5× (risk_off boost)

CVD Analysis: ⚠️ SIGNIFICANT (regression, R²=0.84, 67ms)
Residual: +1,247 (89.2%ile, 2.8σ significance)

Microstructure: Spread 22bps | Depth $280k | VADR 2.3×

Recommendation: STRONG signal with full confirmation
```

### Example 2: Moderate Signal with Fallback

**Solana (SOL-USD) - Choppy Market Conditions**

**Input Data:**
```json  
{
  "symbol": "SOL-USD",
  "premove_data": {
    "funding_z_score": 1.8,           // Below funding threshold
    "oi_residual": 400000,            // $400k OI (moderate)
    "etf_flow_tint": 0.45,            // Mixed ETF flows
    "reserve_change_7d": -6.0,        // -6% reserves
    "whale_composite": 0.75,          // Strong whale activity
    "micro_dynamics": 0.4,            // Moderate dynamics
    "smart_money_flow": 0.55,         // Mixed smart money
    "cvd_residual": 0.35,             // Moderate CVD
    "catalyst_heat": 0.3,             // Low catalyst activity
    "vol_compression_rank": 0.65,     // Moderate compression
    "oldest_feed_hours": 2.8          // Somewhat stale data
  },
  "confirmation_data": {
    "funding_z_score": 1.8,           // ❌ Fail (<2.0)
    "whale_composite": 0.75,          // ✅ Pass (>0.7)
    "supply_proxy_score": 0.65,       // ✅ Pass (2-of-4 components)
    "reserve_change_7d": -6.0,        // ✅ Reserve component
    "large_withdrawals_24h": 35e6,    // ❌ Below $50M threshold
    "staking_inflow_24h": 18e6,       // ✅ Above $10M threshold  
    "derivatives_oi_change": 12.0,    // ❌ Below 15% threshold
    "volume_ratio_24h": 1.6,          // Below volume threshold
    "current_regime": "normal"        // No volume boost
  }
}
```

**Analysis Output:**
```
📈 MODERATE CANDIDATE — SOL-USD (Rank #3)  
Pre-Movement Score: 61.7/100 (freshness: C-, 124ms)
⚠️ Freshness penalty: -8.0% (2.8h old data)

Component Breakdown:
◐ derivatives: 7.8 pts (funding 2.7 + OI 1.6 + ETF 1.8)
◐ supply_demand: 9.2 pts (reserves 2.4 + whale 5.3)
◐ microstructure: 4.0 pts  
◐ smart_money: 11.0 pts
◐ cvd_residual: 5.3 pts
◐ catalyst: 4.5 pts
◐ compression: 6.5 pts

Gate Confirmation: ✅ CONFIRMED (2/2 gates, 3.0 precedence)
❌ funding_divergence: 1.8σ < 2.0σ  
✅ whale_composite: 0.75 ≥ 0.70
✅ supply_squeeze: 0.65 ≥ 0.60 (2/4 components)
    ✅ reserve_depletion: -6.0% ≤ -5.0%
    ❌ large_withdrawals: $35M < $50M
    ✅ staking_inflow: $18M ≥ $10M  
    ❌ derivatives_oi: 12.0% < 15.0%

CVD Analysis: 📊 NORMAL (percentile fallback, 45ms)
Fallback reason: Low R² (0.21 < 0.30)  
Percentile rank: 67.3% (below 80% threshold)

Recommendation: MODERATE signal, consider position sizing
```

### Example 3: Blocked Signal

**Solana (SOL-USD) - Failed Confirmation Gates**

**Analysis Output:**
```
❌ BLOCKED CANDIDATE — SOL-USD
Pre-Movement Score: 73.4/100 (freshness: B+, 76ms)

Gate Confirmation: ❌ BLOCKED (1/2 gates, 2.0 precedence)
❌ funding_divergence: 1.2σ < 2.0σ
✅ whale_composite: 0.72 ≥ 0.70
❌ supply_squeeze: 0.38 ≥ 0.60 (1/4 components)

Recommendation: BLOCKED - insufficient confirmations
```

---

## Performance & Monitoring

### Performance Requirements

- **Individual Analysis**: <150ms per candidate
- **Batch Processing**: <2s for 50 candidates  
- **Memory Usage**: <10MB working set
- **Success Rate**: >95% analysis completion

### Monitoring Metrics

**System-Level:**
- Total candidates processed/hour
- Average processing time per candidate
- Success rate (completed analyses / attempted)
- Data freshness grade distribution
- Strong/moderate/weak candidate ratios

**Component-Level:**
- Score calculation time (target: <20ms)
- Gate evaluation time (target: <50ms)
- CVD analysis time (target: <80ms)
- Regression success rate vs fallback rate
- Data quality metrics (outlier rates, R² distribution)

### Warning Conditions

**Performance Warnings:**
- Analysis time >500ms per candidate
- Batch processing >5s for 50 candidates
- Memory usage >20MB working set

**Data Quality Warnings:**  
- Freshness grade D or F
- >30% candidates using CVD fallback
- >20% candidates with stale data
- Model R² <0.5 average across candidates

**System Health Warnings:**
- Success rate <90% over 1-hour window
- >10% candidates blocked by gates
- Microstructure API failures >5%

---

## Integration with CryptoRun

### Menu Integration

Pre-Movement v3.3 integrates with the existing CryptoRun menu system:

```
CryptoRun Menu > Intelligence > Pre-Movement Analysis
┌─────────────────────────────────────────────┐
│  Pre-Movement v3.3 Intelligence            │  
├─────────────────────────────────────────────┤
│  [1] Scan Current Candidates (Top 20)      │
│  [2] Analyze Specific Symbol               │  
│  [3] View Recent Analysis History          │
│  [4] Configure Thresholds                  │
│  [5] System Health & Performance           │
│  [0] Back to Main Menu                     │
└─────────────────────────────────────────────┘
```

### API Integration

**Primary Interface:**
```go
engine := premove.NewPreMovementEngine(microEvaluator, config)
result, err := engine.ListCandidates(ctx, inputs, limit)
```

**Input Requirements:**
- Pre-Movement scoring data (funding, reserves, whale activity, etc.)
- Gate confirmation data (z-scores, composites, volume ratios)
- CVD time series data (minimum 50 points for regression)
- Current market regime classification

**Output Format:**
- Ranked candidate list with scores, statuses, and detailed attribution
- Complete analysis breakdown for each candidate
- Data quality assessment and system warnings
- Performance metrics and timing information

### No CLI Changes

Pre-Movement v3.3 is **menu-only** and does not modify existing CLI commands:
- No changes to `cryptorun scan`
- No changes to `cryptorun monitor`  
- No changes to global scanner logic
- Maintains separation from production scanning workflows

This ensures that Pre-Movement remains a **consultative intelligence layer** that enhances decision-making without interfering with existing operational processes.

---

*Last updated: September 2024*  
*Version: Pre-Movement v3.3.0*  
*Status: Production Ready*