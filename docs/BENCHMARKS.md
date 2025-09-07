# CryptoRun Benchmark Methodology

## UX MUST — Live Progress & Explainability

This document explains CryptoRun's spec-compliant benchmark methodology and diagnostics system, ensuring all recommendations are based on realistic entry/exit simulation rather than raw 24h price changes.

## Overview

CryptoRun's benchmark system evaluates scanner performance against external references (like CoinGecko top gainers) using **spec-compliant P&L calculations** that simulate actual entry and exit conditions according to our gates, guards, and exit hierarchy.

## Core Principle: Spec-Compliant P&L Only

### ❌ What We Don't Do (Raw 24h Approach)
```
ETH: +42.8% raw gain → "missed opportunity, tune gates"
SOL: +38.4% raw gain → "missed opportunity, relax guards"
```

### ✅ What We Do (Spec-Compliant Approach)
```
ETH: +42.8% raw, -2.1% spec P&L → "correctly filtered by system"
SOL: +38.4% raw, +0.5% spec P&L → "minimal recoverable gain"
ADA: +13.4% raw, +8.2% spec P&L → "actionable miss, consider tuning"
```

## Spec-Compliant P&L Calculation

### Entry Logic
**Entry Point**: First bar AFTER signal timestamp that passes ALL gates and guards:

1. **Gates Validation**:
   - Composite score ≥ configured minimum (default: 2.0)
   - Spread < 50 basis points
   - Depth ±2% ≥ $100k USD
   - VADR ≥ 1.75×

2. **Guards Validation** (regime-aware):
   - **Fatigue Guard**: 24h momentum and RSI thresholds
   - **Freshness Guard**: Data age and ATR price movement limits  
   - **Late-Fill Guard**: Maximum execution delay from bar close

### Exit Hierarchy
**Exit Point**: Earliest condition from this priority-ordered list:

1. **Hard Stop**: 5% loss limit
2. **Venue Health**: Spread >100bps OR depth <$50k
3. **Time Limit**: 48-hour maximum holding period
4. **Acceleration Reversal**: Momentum acceleration turning negative
5. **Momentum Fade**: RSI overbought >80
6. **Profit Target**: 15% gain target

### P&L Calculation
```
Spec P&L % = ((Exit Price - Entry Price) / Entry Price) × 100
```

## Data Sources and Labeling

### Exchange-Native Priority
1. **Binance** (preferred for CEX data)
2. **Kraken** (backup exchange-native)
3. **Coinbase** (tertiary exchange-native)
4. **OKX** (additional exchange-native)

### Fallback Labeling
If exchange-native data unavailable:
- **CoinGecko**: `aggregator_fallback_coingecko`
- **DexScreener**: `aggregator_fallback_dexscreener`

All aggregator sources must be clearly labeled as fallbacks.

## Sample Size Requirements

### n≥20 Rule
- **Minimum sample size**: 20 gainers per window
- **Recommendation threshold**: Only generate tuning advice when n≥20
- **Insufficient sample handling**: Disable recommendations, show "Sample size n < 20: recommendations disabled"

### Window-Specific Enforcement
Each time window (1h, 24h, 7d) enforced independently:
```yaml
sample_validation:
  required_minimum: 20
  window_sample_sizes:
    "1h": 15    # ← Recommendations disabled
    "24h": 25   # ← Recommendations enabled
    "7d": 18    # ← Recommendations disabled
  recommendations_enabled: false  # ← Any window n<20 disables all
```

## Diagnostic Output Format

### Dual-Column Display
All diagnostic outputs show both metrics for transparency:

```json
{
  "misses": [
    {
      "symbol": "ETHUSD",
      "gain_percentage": 42.8,           // Raw 24h (context only)  
      "raw_gain_percentage": 42.8,       // Explicit raw column
      "spec_compliant_pnl": -2.1,       // THE decision metric
      "series_source": "exchange_native_binance",
      "primary_reason": "correctly_filtered_negative_spec_pnl",
      "config_tweak": "None - spec P&L negative"
    },
    {
      "symbol": "ADAUSD", 
      "gain_percentage": 13.4,           // Raw 24h (context only)
      "raw_gain_percentage": 13.4,       // Explicit raw column
      "spec_compliant_pnl": 8.2,        // THE decision metric  
      "series_source": "exchange_native_binance",
      "primary_reason": "freshness_guard_failure",
      "config_tweak": "Reduce freshness.max_bars_age to 3" // Based on spec P&L
    }
  ]
}
```

### Recommendation Rules
- **Base recommendations ONLY on `spec_compliant_pnl`**
- **Never use `raw_gain_percentage` for tuning advice**
- **Show `raw_gain_percentage` for context/transparency**
- **Suppress recommendations when spec P&L ≤ 1.0%**

## Regime-Aware Simulation

### Regime Detection
Use regime snapshot from actual scan time:
- **Trending**: Relaxed guard thresholds
- **Choppy**: Baseline guard thresholds  
- **High Volatility**: Baseline guard thresholds

**All benchmark snapshots record complete regime state**:
- Active regime (trending_bull/choppy/high_vol)
- 5-way factor weight map (momentum/technical/volume/quality/catalyst)
- Regime health indicators (volatility_7d, above_ma_pct, breadth_thrust)  
- Regime switch timestamps and duration
- Weight allocation validation (sum=100%, social cap enforcement)

See **[Regime Tuner System](./REGIME_TUNER.md)** for complete regime detection logic, weight adaptation strategies, and empirical validation results.

### Regime-Specific Thresholds
```yaml
guards:
  fatigue:
    baseline_threshold: 12.0      # Chop/high-vol
    trending_multiplier: 1.5      # 18% for trending
  freshness:  
    max_bars_age: 2               # Baseline
    trending_max_bars_age: 3      # Relaxed for trending
  late_fill:
    max_delay_seconds: 30         # Baseline  
    trending_max_delay_seconds: 45 # Relaxed for trending
```

## Compliance Testing

### Unit Test Requirements
```go
// Fail if advice uses raw_24h_change instead of spec_pnl_pct
func TestSpecPnLComplianceEnforcement(t *testing.T) {
    // High raw gain but negative spec P&L
    result := calculateMissRecommendation("ETHUSD", 42.8, -2.1)
    if result.HasRecommendation {
        t.Errorf("VIOLATION: Recommending action based on raw gain despite negative spec P&L")
    }
}

// Sample size guard enforcement
func TestSampleSizeGuardEnforcement(t *testing.T) {
    recommendations := generateRecommendations(sampleSize: 15)
    if len(recommendations) > 0 {
        t.Errorf("VIOLATION: Generated recommendations with n=%d < 20", 15)
    }
}
```

### CI Conformance Checks
- **Spec P&L basis**: All recommendations must reference `spec_compliant_pnl`
- **Sample size enforcement**: n≥20 requirement for recommendations
- **Series source labeling**: Exchange-native vs aggregator fallback attribution
- **Methodology verification**: Documentation must mention spec-compliant approach

## Key Behavioral Changes

### Before (Raw 24h Approach)
- ❌ "ETH +42.8%/SOL +38.4% missed - tune gates"
- ❌ Recommendations based on raw price changes
- ❌ No entry/exit simulation
- ❌ No sample size guards

### After (Spec-Compliant Approach)  
- ✅ ETH/SOL still show raw 24h for context
- ✅ Recommendations ONLY when spec P&L > 1.0%
- ✅ Full entry/exit simulation with gates/guards
- ✅ n≥20 sample size requirement enforced
- ✅ Regime-aware threshold adjustment
- ✅ Exchange-native data priority with fallback labeling

## Configuration Reference

See `config/bench.yaml` for complete configuration including:
- Sample size requirements (`diagnostics.sample_size`)
- Series source preferences (`diagnostics.series`)  
- Simulation parameters (`diagnostics.simulation`)
- Gate/guard thresholds (`diagnostics.gates`, `diagnostics.guards`)
- Output format controls (`diagnostics.output`)
- Recommendation rules (`diagnostics.recommendations`)

---

**Result**: Diagnostic recommendations reflect realistic trading opportunities rather than raw price movements, ensuring actionable insights align with actual system behavior.

---

# FactorWeights vs Unified Composite Benchmark

## UX MUST — Live Progress & Explainability

The FactorWeights benchmark provides real-time scoring comparison with step-by-step factor breakdowns, correlation calculations, and detailed explanations for every metric and disagreement identified.

## Purpose

Compare the legacy FactorWeights scoring system against the new Unified Composite system to quantify improvements, measure behavioral differences, and identify potential regression risks during system transitions.

## Key Differences Tested

### Legacy FactorWeights System
- **No Orthogonalization**: Raw factor combination without correlation removal
- **Uncapped Social Factor**: Social sentiment can dominate scoring (up to 85+ points)
- **Equal-Weight Timeframes**: All momentum windows weighted equally (1h=4h=12h=24h)
- **Simple Linear Combination**: Direct summation of factor scores

### Unified Composite System  
- **MomentumCore Protection**: Core momentum factor protected from orthogonalization
- **Social Factor Capping**: Social contribution limited to +10 points maximum
- **Gram-Schmidt Residualization**: Technical, volume, quality, and social factors orthogonalized
- **Regime-Adaptive Weights**: Different weight profiles per market regime (trending/choppy/volatile)

## Benchmark Execution

### Command Interface
```bash
# Full benchmark with default settings
cryptorun bench factorweights --universe topN:30 --windows 1h,4h,12h,24h --n 20

# Menu access: Main Menu → 2. Bench → 2. Benchmark — Legacy FactorWeights vs Unified
```

### Configuration Options
- `--universe topN:N` or `--universe path:/path/to/file` - Asset universe specification
- `--windows 1h,4h,12h,24h` - Time windows for forward returns analysis
- `--n 20` - Minimum sample size (enforced: n≥20 for statistical validity)
- `--out /path/` - Output directory (default: auto-timestamped artifacts/ folder)
- `--progress` - Show live progress indicators with ETA calculations

### Data Sources & Validation
- **Price Data**: CoinGecko API (free tier with rate limiting)
- **Guard Evaluation**: Shared validation logic applied identically to both systems
- **Microstructure**: Exchange-native L1/L2 validation (Binance/OKX/Coinbase)
- **Mock Fallback**: Deterministic mock data for development/testing environments

## Metrics Calculated

### 1. Spearman Rank Correlation
**Purpose**: Measure how similarly the two systems rank the same set of assets

**Calculation**: Standard Spearman's ρ per time window
- Range: [-1, 1] where 1 = perfect agreement, -1 = perfect disagreement  
- **High Correlation (>0.9)**: Systems agree on asset ranking despite score differences
- **Low Correlation (<0.7)**: Significant behavioral differences requiring investigation

### 2. Hit Rate Analysis
**Purpose**: Compare forward return performance between systems

**Method**: Assets scoring ≥75 points that achieve ≥2% forward returns
- **Unified Hit Rate**: Percentage of high-scoring Unified assets hitting return threshold
- **Legacy Hit Rate**: Percentage of high-scoring Legacy assets hitting return threshold
- **Delta**: Unified - Legacy (positive = Unified improvement)

### 3. Disagreement Rate
**Purpose**: Quantify how often systems disagree on trading decisions

**Calculation**: Percentage where systems disagree on ≥75 score threshold
- **Agreement**: Both systems score asset above or below 75
- **Disagreement**: One system scores ≥75, other scores <75
- **Rate**: (Disagreements) / (Assets above threshold in either system)

### 4. Average Score Delta
**Purpose**: Measure typical magnitude of scoring differences

**Calculation**: Mean absolute difference between system scores
- **Small Delta (<5)**: Systems generally agree with minor adjustments
- **Large Delta (>15)**: Fundamental scoring differences, often social capping effects

## Output Formats

### 1. Console Summary
Real-time formatted table with key metrics and top disagreements:
```
📊 SUMMARY METRICS:
  Universe: topN:30 (28 eligible after guards)
  Sample windows: 47 total across 4 time horizons
  Spearman ρ (1h): 0.923 | (4h): 0.887 | (12h): 0.901 | (24h): 0.845
  Hit rate (unified): 78.3% | Hit rate (legacy): 74.1% | Improvement: +4.2pp
  Disagreement rate (≥75 threshold): 23.4%
  Avg |delta|: 8.3 pts | Gate pass-through: 87% | UNPROVEN micro: 2 assets (excluded)
```

### 2. CSV Export (`side_by_side.csv`)
Machine-readable per-asset comparison with columns:
- `symbol,ts,window,unified_score,legacy_score,delta`
- `u_hit,l_hit,fwd_return,guards_passed`
- `spread_bps,depth_usd_pm2,vadr,venue`

### 3. JSONL Export (`results.jsonl`)  
Complete structured data with factor breakdowns:
```json
{
  "ts": "2025-09-06T14:30:52Z",
  "symbol": "BTCUSD",
  "windows": ["1h","4h","12h","24h"],
  "unified": {
    "score": 85.0,
    "factors": {
      "momentum_core": 75.0,
      "technical_resid": 5.0,
      "volume_resid": 3.0,
      "social_resid": 2.0,
      "capped_social": true
    }
  },
  "legacy": {
    "score": 92.0,
    "factors": {
      "momentum": 75.0,
      "volume": 15.0, 
      "social": 85.0,
      "volatility": 12.0
    }
  },
  "guards": {"passed": true, "reasons": []},
  "micro": {"venue": "binance", "spread_bps": 45.0, "status": "PASS"},
  "fwd": {"1h": 0.025, "4h": 0.048, "12h": 0.072, "24h": 0.089}
}
```

### 4. Markdown Report (`report.md`)
Executive summary with methodology, disagreement analysis, and caveats:
- **Configuration summary** and asset universe details
- **Key system differences** explanation  
- **Top disagreements table** with probable causes
- **Hit rate comparison** across time windows
- **Methodology section** documenting data sources and calculations
- **Caveats section** noting sample size limitations and mock data usage

## Quality Gates & Validation

### Sample Size Enforcement
- **Minimum Requirement**: n≥20 sample windows for statistical validity
- **Error Behavior**: Command fails if insufficient data available
- **Warning Behavior**: Sample sizes 15-19 generate warnings but don't fail

### Data Quality Checks
- **Price Data Freshness**: All price data must be ≤1 hour old for live runs
- **Guard Consistency**: Identical guard evaluation logic applied to both systems
- **Microstructure Completeness**: Assets failing venue validation excluded with UNPROVEN label

### Reproducibility Requirements
- **Deterministic Mock Data**: Seeded random generation for consistent test results
- **Identical Inputs**: Both systems receive exactly the same feature data and timestamps
- **Artifact Preservation**: All intermediate calculations saved for debugging and audit

## Common Analysis Patterns

### Social Factor Capping Effects
**Pattern**: High legacy scores (>90) with moderate unified scores (~75-85)
**Cause**: Legacy social factor contributing 20-30 points, unified capped at +10
**Interpretation**: Social capping working as designed to prevent sentiment-driven overweighting

### Momentum Protection Effects  
**Pattern**: Unified scores consistently higher for momentum-driven assets
**Cause**: MomentumCore protected from orthogonalization in unified system
**Interpretation**: Core momentum signal preserved while removing correlated noise

### Regime-Adaptive Differences
**Pattern**: Score differences vary by market conditions and time periods
**Cause**: Unified system uses regime-aware weight profiles, legacy uses static weights
**Interpretation**: Unified system adapts to market conditions for better performance

## Integration with CI/CD

### Pre-Merge Validation
```bash
# Quick validation with minimal universe
cryptorun bench factorweights --universe topN:10 --n 15 --progress plain --out ci_temp/
```

### Post-Deploy Verification  
```bash
# Full validation with complete universe
cryptorun bench factorweights --universe topN:50 --n 25 --progress --out artifacts/production/
```

### Regression Detection
- **Correlation Degradation**: Alert if Spearman ρ drops below 0.8 in any window
- **Hit Rate Regression**: Alert if unified hit rate falls behind legacy by >5%
- **Excessive Disagreement**: Alert if disagreement rate exceeds 40%

This benchmark ensures scoring system improvements are validated through rigorous statistical comparison while maintaining full explainability and reproducibility for system evolution tracking.