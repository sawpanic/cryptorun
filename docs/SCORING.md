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
MomentumCore (Protected) → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (cap +10 applied after)
```

**One pipeline: MomentumCore (protected) → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (cap +10 applied after).**

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
4. **SocialResidual** = Social - proj(Social onto all previous) → hard cap ±10

## Regime Weight Profiles

All weight sets are normalized to sum exactly 1.0:

### Bull Market (Default)
```yaml
momentum_core: 0.50      # 50% - Protected momentum signal
technical_residual: 0.20 # 20% - Technical indicators (residualized)
volume_residual: 0.20    # 20% - Volume confirmation (residualized)
quality_residual: 0.05   # 5%  - Fundamental quality (residualized)  
social_residual: 0.05    # 5%  - Social sentiment (residualized + capped)
```

### Choppy Market
```yaml
momentum_core: 0.40      # 40% - Reduced momentum in sideways markets
technical_residual: 0.25 # 25% - Higher technical weight for mean reversion
volume_residual: 0.15    # 15% - Less reliable volume confirmation
quality_residual: 0.15   # 15% - Quality matters more in chop
social_residual: 0.05    # 5%  - Consistent social weight
```

### High Volatility
```yaml
momentum_core: 0.45      # 45% - Moderate momentum weight
technical_residual: 0.15 # 15% - Technical indicators noisy in high vol
volume_residual: 0.25    # 25% - Volume crucial for liquidity validation
quality_residual: 0.10   # 10% - Quality for stability
social_residual: 0.05    # 5%  - Consistent social weight
```

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
                 (SocialResidual × W_social)

Where: W_momentum + W_technical + W_volume + W_quality + W_social = 1.0
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