# CryptoRun Unified Scoring Model

## UX MUST — Live Progress & Explainability

The CryptoRun scoring model provides **real-time, explainable momentum signals** with full attribution and regime-adaptive weighting. Every score includes component breakdown and correlation-adjusted residuals.

## Overview

CryptoRun uses a **unified orthogonal scoring system** that combines momentum, technical, volume, quality, and social factors through Gram-Schmidt residualization. This ensures factors are uncorrelated while preserving the protected MomentumCore.

## Architecture

### Protected MomentumCore (Never Orthogonalized)

The MomentumCore forms the foundation and is **never residualized**:

```
MomentumCore = (Return1h × 20%) + (Return4h × 35%) + (Return12h × 30%) + (Return24h × 15%) + AccelBoost
```

**Trending Regime Variation:**
```
MomentumCore = (Return1h × 15%) + (Return4h × 30%) + (Return12h × 25%) + (Return24h × 20%) + (Return7d × 10%) + AccelBoost
```

The 4h acceleration boost uses: `tanh(|accel4h|/5) × ±2.0`

### Orthogonal Residual Layers

Each factor is residualized against all previous factors in strict order:

1. **MomentumCore** (Protected baseline)
2. **TechnicalResidual** ← Orthogonalized against Momentum
3. **VolumeResidual** ← Orthogonalized against Momentum + Technical  
4. **QualityResidual** ← Orthogonalized against Momentum + Technical + Volume
5. **SocialResidual** ← Orthogonalized against all previous, then capped at ±10

## Factor Composition

### Technical Factors
- **RSI14** (30%): Relative Strength Index 14-period
- **MACD** (35%): Moving Average Convergence Divergence
- **BBWidth** (20%): Bollinger Band Width (volatility)
- **ATRRatio** (15%): Average True Range ratio

### Volume Factors  
- **VolumeRatio24h** (40%): 24h volume vs average
- **VWAP** (25%): Volume Weighted Average Price deviation
- **OBV** (20%): On Balance Volume
- **VolSpike** (15%): Volume spike indicator

### Quality Factors
- **Spread** (25%): Bid-ask spread (lower is better)
- **Depth** (35%): Order book depth at ±2%
- **VADR** (30%): Volume-Adjusted Daily Range
- **MarketCap** (10%): Market capitalization tier

### Social Factors (Capped at ±10)
- **Sentiment** (30%): Aggregated social sentiment
- **Mentions** (25%): Social mention volume
- **SocialVolume** (25%): Social engagement volume
- **RedditScore** (20%): Reddit discussion quality

## Regime-Adaptive Weights

The composite score combines residuals using regime-specific weights:

### Trending Regime (55/25/15/5)
```
Score = Momentum×55% + Technical×25% + Volume×15% + Quality×5% + Social(±10)
```
*Momentum dominates in clear directional moves*

### Choppy Regime (40/35/15/10) 
```
Score = Momentum×40% + Technical×35% + Volume×15% + Quality×10% + Social(±10)
```
*Balanced between momentum and technical factors*

### High Volatility Regime (30/30/25/15)
```
Score = Momentum×30% + Technical×30% + Volume×25% + Quality×15% + Social(±10)
```
*Quality and volume factors more important during volatility*

## Social Factor Cap

Social factors are strictly capped at **±10 points** and applied **outside** the 100% weight normalization:

```go
if socialScore > 10.0 { socialScore = 10.0 }
if socialScore < -10.0 { socialScore = -10.0 }

FinalScore = CompositeScore + socialScore
```

This prevents social hype from overwhelming price/volume signals while still allowing meaningful contribution.

## Orthogonalization Method

Each residual uses estimated correlations based on factor magnitudes:

```go
correlation := baseCorrelation + tanh(|factor|/threshold) × adjustment
projection := correlation × previousFactor
residual := rawFactor - projection
```

This adaptive approach handles varying market conditions while maintaining orthogonality.

## API Usage

```go
import "cryptorun/internal/scoring"

calc := scoring.NewCalculator(scoring.RegimeChoppy)

input := scoring.FactorInput{
    Symbol: "BTC-USD",
    Momentum: scoring.MomentumFactors{
        Return1h: 5.0, Return4h: 8.0, Return12h: 10.0, Return24h: 12.0,
        Accel4h: 2.0,
    },
    Technical: scoring.TechnicalFactors{RSI14: 70.0, MACD: 1.5},
    // ... other factors
}

result, err := calc.Calculate(input)
// result.Score = final composite score
// result.Parts = map of component scores  
// result.Meta = attribution and metadata
```

## Output Format

```json
{
  "score": 73.45,
  "parts": {
    "momentum": 45.2,
    "technical": 12.8,
    "volume": 8.3,
    "quality": 4.1,
    "social": 3.0,
    "social_capped": 3.0
  },
  "meta": {
    "timestamp": "2024-01-15T10:30:00Z",
    "regime": "choppy", 
    "symbol": "BTC-USD",
    "attribution": "unified_orthogonal_v1",
    "is_orthogonal": true
  }
}
```

## Validation Requirements

- **Weight Sum**: All regime weights must sum to 100%
- **Social Cap**: Social contribution limited to ±10
- **Monotonicity**: Higher momentum should yield higher momentum component
- **Orthogonality**: Pairwise correlations between residuals < 0.3
- **Protected Core**: MomentumCore never modified by orthogonalization

## Migration from Legacy FactorWeights

The legacy parallel factor weighting system has been **retired**. All scoring now flows through the unified orthogonal path. A feature flag maintains backward compatibility during transition:

```go
// DEPRECATED - will be removed in v3.3
// Use unified scoring.Calculator instead
```

This ensures a single, explainable, and consistent scoring methodology across all CryptoRun components.