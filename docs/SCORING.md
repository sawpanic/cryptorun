# Unified Composite Scoring (MomentumCore-Protected)

## UX MUST — Live Progress & Explainability

Real-time unified composite scoring with MomentumCore protection: single-path pipeline architecture, Gram-Schmidt residualization, active factor weight normalization, and comprehensive validation guards for deterministic, explainable momentum detection.

**Updated for PROMPT_ID=FIX.COMPOSITE.INPUTS.ALIASES**  
**Last Updated:** 2025-09-07  
**Version:** v3.3.2 Type Bridge Pipeline  
**Breaking Changes:** Retired legacy FactorWeights path - SINGLE PATH ONLY

The unified composite scoring system implements a single-path pipeline architecture that ensures all scoring routes through one consistent composite system, providing deterministic, explainable momentum scoring for the 6-48 hour trading horizon.

## Pipeline Architecture

### Single Pipeline Flow

```
MomentumCore (Protected) → TechnicalResidual → VolumeResidual → QualityResidual → CatalystResidual → SocialResidual (cap +10 applied after)
```

**One pipeline: MomentumCore (protected) → TechnicalResidual → VolumeResidual → QualityResidual → CatalystResidual → SocialResidual (cap +10 applied after).**

**Multi-timeframe momentum weights: 1h/4h/12h/24h = 20/35/30/15.**

**Residualization: remove projections on earlier components (Gram–Schmidt-style) to avoid double counting.**

**Normalization: active factor weights sum to 100% (ex-social).**

**Validation: weight-sum guard, NaN guard, monotone decile sanity.**

**API surfaces: ScoreRow, FactorAttribution, Residuals block.**

**Type Bridge: Pipeline types aliased in composite for seamless integration:**
```go
type RawFactors = pipeline.FactorSet
type RegimeWeights = pipeline.RegimeWeights  
```

### Core Principles

1. **Single Implementation**: One `UnifiedFactorEngine` eliminates duplicate scoring paths
2. **MomentumCore Protection**: Primary momentum signal never residualized in Gram-Schmidt
3. **Weight Normalization**: All regime profiles sum exactly to 1.0 
4. **Social Hard Cap**: Social contribution capped at ±10 after orthogonalization
5. **Correlation Control**: |ρ| < 0.6 between residual factor buckets

### Menu & CLI Route Through UnifiedFactorEngine

**CRITICAL**: Both menu and CLI use the EXACT same scoring path:
- **CLI Scan**: `runScanMomentum()` → `pipeline.Run()` → `UnifiedFactorEngine`
- **Menu Scan**: Menu calls `runScanMomentum()` → `pipeline.Run()` → `UnifiedFactorEngine`
- **CLI Bench**: `runBenchTopGainers()` → `bench.Run()` → Uses scan results from `UnifiedFactorEngine`
- **Menu Bench**: Menu calls `runBenchTopGainers()` → `bench.Run()` → Uses scan results from `UnifiedFactorEngine`

No duplicate implementations exist - menu routes to identical CLI functions.

## Orthogonalization Hierarchy

### Protected Factor (Never Residualized)
- **MomentumCore**: Multi-timeframe momentum (1h=20%, 4h=35%, 12h=30%, 24h=15%)

### Gram-Schmidt Sequence (Applied in Order)
1. **TechnicalResidual** = Technical - proj(Technical onto MomentumCore)
2. **VolumeResidual** = Volume - proj(Volume onto [MomentumCore, TechnicalResidual])  
3. **QualityResidual** = Quality - proj(Quality onto [MomentumCore, TechnicalResidual, VolumeResidual])
4. **CatalystResidual** = Catalyst - proj(Catalyst onto [MomentumCore, TechnicalResidual, VolumeResidual, QualityResidual])
5. **SocialResidual** = Social - proj(Social onto all previous) → hard cap ±10

## Regime Weight Profiles

All weight sets are normalized to sum exactly 1.0:

### Trending Bull Market (Default)
```yaml
momentum_core: 0.42        # 42% - Protected momentum signal
technical_residual: 0.20   # 20% - Technical indicators (residualized)
supply_demand_block: 0.28  # 28% - Split between volume (55%) and quality (45%)
catalyst_block: 0.10       # 10% - Catalyst compression + events (residualized)
```

### Choppy Market
```yaml
momentum_core: 0.27        # 27% - Reduced momentum in sideways markets
technical_residual: 0.25   # 25% - Higher technical weight for mean reversion
supply_demand_block: 0.33  # 33% - Higher supply/demand focus in chop
catalyst_block: 0.15       # 15% - Higher catalyst weight in uncertain markets
```

### High Volatility
```yaml
momentum_core: 0.32        # 32% - Moderate momentum weight
technical_residual: 0.22   # 22% - Technical indicators stabilize in high vol
supply_demand_block: 0.35  # 35% - Supply/demand crucial for liquidity
catalyst_block: 0.11       # 11% - Lower catalyst in volatile conditions
```

**Note**: The `supply_demand_block` is internally split as:
- **Volume weight**: 55% of supply_demand_block allocation
- **Quality weight**: 45% of supply_demand_block allocation

## Factor Definitions

### MomentumCore (Protected)
- **Description**: Multi-timeframe momentum with regime-adaptive weighting
- **Calculation**: Weighted average of 1h, 4h, 12h, 24h momentum
- **Range**: -50% to +50% percentage momentum
- **Protection**: Never orthogonalized against - always pure signal

### TechnicalFactor → TechnicalResidual  
- **Description**: RSI(14), MACD, ADX technical indicators
- **Residualization**: Against MomentumCore only
- **Range**: 0-100 normalized technical score
- **Purpose**: Capture technical patterns independent of momentum

### VolumeFactor → VolumeResidual
- **Description**: Volume surge ratio, ADV confirmation
- **Residualization**: Against MomentumCore + TechnicalResidual
- **Range**: 0.5x to 5.0x average volume multiple
- **Purpose**: Volume confirmation independent of momentum and technicals

### QualityFactor → QualityResidual
- **Description**: Fundamental quality metrics when available
- **Residualization**: Against MomentumCore + TechnicalResidual + VolumeResidual
- **Range**: 0-100 quality score
- **Purpose**: Quality assessment independent of price/volume signals

### CatalystFactor → CatalystResidual
- **Description**: Bollinger Band compression + time-decayed catalyst events
- **Residualization**: Against MomentumCore + TechnicalResidual + VolumeResidual + QualityResidual
- **Components**: 60% BB width compression (0-1) + 40% catalyst events (0-1)
- **Range**: 0-1 normalized catalyst score
- **Purpose**: Market compression and catalyst event timing independent of other factors

### SocialFactor → SocialResidual (Capped)
- **Description**: Social sentiment from multiple sources
- **Residualization**: Against all previous factors
- **Post-Processing**: Hard capped at ±10 after orthogonalization
- **Range**: -10 to +10 (post-cap)
- **Purpose**: Social sentiment independent of all other factors

## Composite Score Calculation

```
CompositeScore = (MomentumCore × W_momentum) +
                 (TechnicalResidual × W_technical) +
                 (VolumeResidual × W_volume) +
                 (QualityResidual × W_quality) +
                 (CatalystResidual × W_catalyst) +
                 (SocialResidual × W_social)

Where: W_momentum + W_technical + W_volume + W_quality + W_catalyst + W_social = 1.0

Internal weight allocation:
W_volume = 0.55 × W_supply_demand_block
W_quality = 0.45 × W_supply_demand_block  
W_catalyst = W_catalyst_block
```

## Anti-Collinearity Measures

### Eliminated Duplicates
- **Whale/On-chain**: Merged into single VolumeFactor to prevent double-counting
- **Multiple Social**: Consolidated to prevent triple-counting sentiment
- **Technical Overlap**: RSI/MACD/ADX combined to prevent correlation

### Orthogonalization Verification
- **Correlation Matrix**: |ρ| < 0.6 between all residual factors
- **Independence Test**: MomentumCore vs all residuals should have |ρ| < 0.1
- **QA Requirement**: Automated correlation testing on n≥100 samples

## Quality Assurance

### Weight Validation
- **Sum Exactness**: All regime weights sum to 1.000 ± 0.001
- **Momentum Minimum**: MomentumCore ≥ 40% across all regimes
- **Social Maximum**: SocialResidual ≤ 15% across all regimes
- **Non-negative**: All weights ≥ 0

### Correlation Constraints  
- **Protected vs Residuals**: |ρ(MomentumCore, *Residual)| < 0.1
- **Residual Cross-Correlation**: |ρ(Residual_i, Residual_j)| < 0.6
- **Sample Size**: n≥100 required for statistical validity

### Social Cap Enforcement
- **Hard Cap**: Social contribution cannot exceed ±10 after residualization
- **Cap Timing**: Applied AFTER orthogonalization to preserve factor independence
- **Verification**: Automated testing ensures cap enforcement

## Implementation Examples

### Factor Processing Pipeline
```go
// Single unified path - no duplicates
engine := factors.NewUnifiedFactorEngine("bull", bullWeights)

// Process all factors through orthogonalization
processed, err := engine.ProcessFactors(factorRows)

// Results include:
// - MomentumCore (unchanged)
// - TechnicalResidual (orthogonalized)
// - VolumeResidual (orthogonalized)
// - QualityResidual (orthogonalized)
// - SocialResidual (orthogonalized + capped)
// - CompositeScore (weighted sum)
// - Rank (highest to lowest)
```

### Regime Switching
```go
// Switch regime and validate new weights
err := engine.SetRegime("choppy", choppyWeights)
if err != nil {
    // Weight validation failed
}

// All future scoring uses new regime weights
processed, err := engine.ProcessFactors(newFactors)
```

### Correlation Analysis
```go
// Get correlation matrix for debugging
corrMatrix := engine.GetCorrelationMatrix(processed)

// Verify orthogonalization effectiveness
momentumVsTech := corrMatrix["MomentumCore"]["TechnicalResidual"]
// Should be near 0.0 for proper orthogonalization
```

## Testing Strategy

### Unit Tests
- **Weight Normalization**: Verify all regime profiles sum to 1.0
- **Orthogonalization**: Verify MomentumCore protection and residual independence
- **Social Cap**: Verify ±10 cap applied after residualization
- **Ranking**: Verify proper score ordering and rank assignment

### Integration Tests  
- **Correlation Matrix**: Generate n≥100 samples and verify correlation constraints
- **Regime Switching**: Test all regime profiles with same factor data
- **End-to-End**: Full pipeline from raw factors to ranked candidates

### Conformance Tests
- **Single Path**: Verify no duplicate scoring implementations exist
- **Weight Sum**: Automated validation that all configurations sum to 1.0
- **Factor Count**: Verify exactly 5 factors (1 protected + 4 residualized)

## Migration from Legacy Systems

### Removed Components
- ~~FactorWeights~~ → Replaced by RegimeWeights
- ~~calcOptimizedCompositeScore~~ → Replaced by UnifiedFactorEngine.ProcessFactors
- ~~ComprehensiveScanner~~ → Merged into UnifiedFactorEngine
- ~~Parallel Scoring Paths~~ → Single unified implementation

### Configuration Migration
- ~~momentum/volume/social/volatility~~ → momentum_core/technical_residual/volume_residual/quality_residual/social_residual
- ~~Weight flexibility~~ → Strict sum=1.0 requirement
- ~~Social post-weighting~~ → Social post-residualization capping

This unified system eliminates scoring path duplicates while ensuring mathematical rigor through orthogonalization, normalized weights, and comprehensive quality assurance testing.

## Enhanced Measurement Integration

### New Data Sources (v3.2.1)

The enhanced scoring system integrates three new measurement data sources to provide additional market insights:

1. **Cross-Venue Funding Divergence** - Z-score analysis of funding rates (up to +2.0 boost)
2. **Open Interest Residual** - 1h ΔOI after price regression (up to +1.5 boost)  
3. **ETF Flow Tint** - Daily net flows vs 7d ADV (up to +1.0 boost)

### Enhanced Score Range
- **Base Score**: 0-100 (traditional unified scoring)
- **Social Cap**: +10 maximum (unchanged)
- **Measurement Boost**: +4 maximum (new enhancement)
- **Total Enhanced Score**: 0-114 maximum

### Measurement Boost Calculation
```
EnhancedScore = BaseScore + min(4.0, FundingBoost + OIBoost + ETFBoost)

Where:
- FundingBoost = f(|Z-score|, divergence_present)
- OIBoost = f(|OI_residual|)  
- ETFBoost = f(|flow_tint|)
```

### Funding Divergence Boost Logic
```go
// Z-score from 7-day cross-venue funding analysis
if fundingDivergencePresent {
    switch {
    case abs(fundingZ) >= 2.5:
        boost += 2.0  // Very strong signal
    case abs(fundingZ) >= 2.0:
        boost += 1.0  // Strong signal
    }
}
```

### OI Residual Boost Logic
```go
// OI residual after β*ΔPrice regression
absResidual := abs(oiResidual)
switch {
case absResidual >= 2_000_000:  // $2M+
    boost += 1.5
case absResidual >= 1_000_000:  // $1M+
    boost += 0.5
}
```

### ETF Flow Tint Boost Logic
```go
// Daily net flows / 7d ADV (clamped ±2%)
absTint := abs(etfFlowTint)  
switch {
case absTint >= 0.015:  // ±1.5% ADV
    boost += 1.0
case absTint >= 0.010:  // ±1.0% ADV
    boost += 0.5
}
```

### Enhanced Explainability Output
```json
{
  "enhanced_score": {
    "base_score": 84.3,
    "measurements_boost": 2.5,
    "final_enhanced_score": 86.8,
    "data_quality": "Good (2/3 sources)"
  },
  "measurement_insights": {
    "funding_insight": "Strong funding premium (2.8σ)",
    "oi_insight": "Moderate OI buildup ($1.2M residual)",
    "etf_insight": "ETF flows balanced"
  },
  "attribution": {
    "funding": "Cross-venue 7d σ analysis from Binance/OKX/Bybit",
    "oi": "1h Δ with β-regression from Binance/OKX", 
    "etf": "No ETF data sources available"
  }
}
```

### Implementation Integration
```go
// Enhanced scoring pipeline
scorer := composite.NewUnifiedScorer()
result, err := scorer.ScoreWithMeasurements(ctx, input)

// Enhanced explanation
explainer := composite.NewEnhancedExplainer(gates, orthogonalizer)
explanation := explainer.ExplainWithMeasurements(
    result, weights, orthogonalized, gateResult, input, latencyMs)
```

### Cache and Performance
- **Measurement Cache TTL**: 600s minimum (10 minutes)
- **ETF Cache TTL**: 86400s (24 hours, daily data)
- **Enhanced Scoring Latency**: < 300ms P99 target
- **Data Integrity**: SHA256 signature hashes, monotonic timestamps

### Quality Assurance Extensions
- **Measurement Boost Cap**: Hard limit at +4.0 points
- **Score Boundary**: Enhanced scores ≤ 114.0
- **Data Quality Reporting**: "Complete/Good/Limited/Incomplete" based on source availability
- **Attribution Tracking**: Full data provenance in explanations

## Isotonic Score Calibration

### Calibration System Overview

The isotonic calibration system provides monotone score-to-probability mapping to convert composite scores (0-114 range) into calibrated success probabilities (0-1 range) using real trading outcomes.

**Key Features:**
- **Isotonic Regression**: Pool-Adjacent-Violators algorithm ensures monotonic probability mapping
- **Regime-Aware**: Separate calibration curves per market regime (bull/bear/choppy)
- **Live Data Collection**: Tracks actual 48-hour trading outcomes for calibration samples
- **Performance Metrics**: Reliability (calibration error), resolution (discrimination), sharpness
- **Governance Controls**: Monthly refresh schedule with validation gates

### Mathematical Foundation

**Isotonic Regression (Pool-Adjacent-Violators):**
```
For calibration points (s₁,p₁), (s₂,p₂), ..., (sₙ,pₙ):
If pᵢ > pⱼ where sᵢ < sⱼ, pool adjacent points until monotonic

Pooled probability = Σ(wᵢ × pᵢ) / Σ(wᵢ)
Where wᵢ = sample count for each bin
```

**Score-to-Probability Mapping:**
```
P(success|score,regime) = IsotonicCalibrator[regime].Predict(score)

Where:
- score ∈ [0, 114] (enhanced composite score)
- P(success|score,regime) ∈ [0, 1] (calibrated probability)
- regime ∈ {"bull", "bear", "choppy", "general"}
```

### Calibration Data Collection

**Position Tracking Lifecycle:**
1. **Entry**: Track new position with score, entry price, regime
2. **Monitoring**: Update position with real-time price movements  
3. **Outcome**: Determine success/failure based on 48-hour performance
4. **Sample Creation**: Generate CalibrationSample with outcome data

**Success Criteria:**
```
Success = |price_movement| ≥ move_threshold AND holding_period ≤ 48h

Default thresholds:
- move_threshold: 5.0% (configurable)
- target_holding_period: 48 hours
- max_tracking_time: 72 hours (timeout)
```

**Sample Structure:**
```go
type CalibrationSample struct {
    Score         float64   // Composite score (0-114)
    Outcome       bool      // True if success criteria met
    Timestamp     time.Time // When position entered
    Symbol        string    // Asset symbol (e.g., "BTCUSD")
    Regime        string    // Market regime during entry
    HoldingPeriod time.Duration // Actual holding time
    MaxMove       float64   // Maximum price movement observed
    FinalMove     float64   // Final price movement at close
}
```

### Regime-Aware Calibration

**Calibration Harness Architecture:**
```go
type CalibrationHarness struct {
    calibrators   map[string]*IsotonicCalibrator  // Per-regime calibrators
    sampleBuffer  []CalibrationSample             // Training data buffer
    config        CalibrationConfig               // System configuration
}
```

**Regime-Specific Calibration:**
- **Bull Regime**: Higher scores more predictive of continued momentum
- **Bear Regime**: Calibration adjusted for reversal patterns
- **Choppy Regime**: Higher uncertainty, wider confidence intervals
- **General Fallback**: Used when regime-specific calibrator unavailable

**Fallback Hierarchy:**
1. Regime-specific calibrator (e.g., "bull")
2. General calibrator ("general") 
3. Uncalibrated probability mapping (score/100)

### Performance Validation

**Calibration Quality Metrics:**
```
Reliability = Σ|observed_freq - predicted_prob|² / N
Resolution = Variance(predicted_probabilities) 
Sharpness = max(probabilities) - min(probabilities)
```

**Validation Gates:**
- **Calibration Error**: < 10% maximum allowed error
- **AUC Threshold**: > 0.55 (better than random by 5%)
- **Sample Sufficiency**: ≥ 100 samples minimum for fitting
- **Monotonicity**: Enforced by isotonic regression algorithm

**Validation Process:**
1. Split samples into 80% training / 20% validation
2. Fit isotonic calibrator on training set
3. Evaluate performance on validation set
4. Accept/reject calibrator based on quality gates

### Refresh and Governance

**Scheduled Refresh Cycle:**
- **Default Interval**: 30 days (configurable)
- **Minimum Samples**: 100 samples required before refresh
- **Validation Required**: New calibrators must pass quality gates
- **Governance Freeze**: Manual intervention required for major changes

**Refresh Process:**
```go
// Automatic refresh check
if harness.NeedsRefresh() {
    err := harness.RefreshCalibration(ctx)
    if err != nil {
        // Validation failed - keep existing calibrators
        log.Warn("Calibration refresh failed", "error", err)
    }
}
```

**Data Management:**
- **Buffer Size**: 10x minimum samples (1000 default)
- **Sample Cleanup**: Remove samples older than 90 days
- **Memory Limits**: Automatic buffer trimming to prevent overflow

### Integration with Scoring Pipeline

**Enhanced Score Conversion:**
```go
// Get calibrated probability for enhanced score
prob, err := harness.PredictProbability(enhancedScore, currentRegime)
if err != nil {
    // Fallback to uncalibrated probability
    prob = math.Max(0, math.Min(1, enhancedScore/100.0))
}
```

**Output Enhancement:**
```json
{
  "enhanced_score": 86.8,
  "calibrated_probability": 0.73,
  "calibration_info": {
    "regime": "bull", 
    "calibrator_age": "15 days",
    "sample_count": 247,
    "reliability": 0.08,
    "calibration_quality": "good"
  }
}
```

**Entry Gate Integration:**
```go
// Use calibrated probability in entry decisions
if enhancedScore >= 75.0 && calibratedProb >= 0.65 {
    // High-confidence entry signal
    return EntrySignal{Confidence: "high", Probability: calibratedProb}
}
```

### Performance Characteristics

**Computational Complexity:**
- **Calibrator Fitting**: O(n log n) for n samples (sorting + PAV)
- **Prediction**: O(log k) for k calibration points (binary search)
- **Memory Usage**: ~1KB per 100 samples stored

**Benchmark Performance:**
```
IsotonicCalibrator.Fit():     ~50ms for 200 samples
IsotonicCalibrator.Predict(): ~8μs per prediction
CalibrationHarness refresh:   ~200ms for 3 regimes
Position tracking overhead:   ~1μs per price update
```

**Scalability Limits:**
- **Maximum Samples**: 10,000 per calibrator (buffer management)
- **Maximum Regimes**: 10 regime-specific calibrators
- **Memory Footprint**: <5MB for full system with 1000 symbols

### Quality Assurance

**Deterministic Results:**
- **Reproducible**: Same samples → identical calibration curve
- **Monotonic**: Guaranteed non-decreasing probability mapping
- **Bounded**: All predictions ∈ [0, 1] range

**Test Coverage:**
- **Unit Tests**: Isotonic regression algorithm, validation gates
- **Integration Tests**: End-to-end calibration workflow
- **Performance Tests**: Latency and memory usage under load
- **Statistical Tests**: Calibration accuracy, AUC validation

**Error Handling:**
- **Insufficient Data**: Graceful fallback to uncalibrated probability
- **Validation Failure**: Keep existing calibrators, log warnings
- **Memory Limits**: Automatic sample buffer management
- **Regime Mismatch**: Fallback hierarchy (regime → general → uncalibrated)