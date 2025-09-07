# Signals Output Schema

CryptoRun signals output provides comprehensive candidate information with attribution badges and transparent scoring explanations.

## Output Files

### signals.csv
Structured CSV format with attribution badges for immediate analysis:

```csv
Symbol,Score,Fresh,Depth,Venue,Sources,LatencyMs,ScoreGate,VADRGate,FundingGate,FreshnessGate,LateFillGate,FatigueGate,MomentumCore,TechnicalResidual,VolumeResidual,QualityResidual,SocialCapped
BTC-USD,82.4,●,✓,Kraken,4,245,PASS,PASS,PASS,PASS,PASS,PASS,65.200,8.150,6.300,2.750,0.000
ETH-USD,76.8,●,✓,OKX,3,189,PASS,FAIL,PASS,PASS,PASS,PASS,61.200,7.200,5.100,3.300,0.000
```

### explain.json
Detailed explanations with factor breakdown and attribution:

```json
{
  "metadata": {
    "timestamp": "2025-09-07T14:30:00Z",
    "scan_type": "hot",
    "universe_size": 20,
    "candidates": 12,
    "version": "v3.2.1"
  },
  "scoring_system": {
    "protected_momentum": "MomentumCore never orthogonalized",
    "orthogonalization": "Technical → Volume → Quality → Social (Gram-Schmidt)",
    "social_cap": "+10 points maximum, applied outside weight allocation",
    "regime_adaptive": "Weight profiles switch on 4h cadence"
  },
  "gates": {
    "core_requirement": "2 of 3: Score≥75 + VADR≥1.8 + FundingDivergence≥2σ",
    "guard_requirement": "ALL: Freshness≤2bars + LateFill<30s + Fatigue protection"
  },
  "candidates": [
    {
      "symbol": "BTC-USD",
      "score": 82.4,
      "attribution": {
        "data_freshness": {
          "fresh": true,
          "description": "Data ≤2 bars old and within 1.2×ATR(1h)"
        },
        "liquidity_validation": {
          "depth_ok": true,
          "venue": "Kraken",
          "description": "Exchange-native depth ≥$100k within ±2%"
        },
        "data_sources": {
          "count": 4,
          "latency_ms": 245,
          "description": "Number of independent data sources used"
        }
      }
    }
  ]
}
```

## Attribution Badges

### Data Quality Indicators
- **Fresh ●/○**: Data ≤2 bars old and within 1.2×ATR(1h)
  - ● = Fresh data (passes gate)  
  - ○ = Stale data (fails gate)

### Liquidity Validation
- **Depth ✓/✗**: Exchange-native depth validation
  - ✓ = Depth ≥$100k within ±2% (passes)
  - ✗ = Insufficient depth (fails)

### Data Attribution
- **Venue**: Primary exchange source (Kraken, OKX, Coinbase, Binance)
- **Sources**: Count of independent data sources (1-5)
- **LatencyMs**: End-to-end processing latency in milliseconds

## Gate Status

### Core Gates (2 of 3 Required)
- **ScoreGate**: Composite score ≥ 75.0
- **VADRGate**: Volume-Adjusted Daily Range ≥ 1.8
- **FundingGate**: Cross-venue funding divergence ≥ 2σ

### Guard Gates (ALL Required)  
- **FreshnessGate**: Data ≤2 bars old and within 1.2×ATR
- **LateFillGate**: Signal generated <30s after bar close
- **FatigueGate**: Not blocked by fatigue protection (24h>12% & RSI4h>70)

## Factor Breakdown

### Protected MomentumCore
- **Value Range**: 0-100 points
- **Protected**: Never subject to orthogonalization
- **Timeframes**: Multi-timeframe momentum (1h/4h/12h/24h)

### Orthogonalized Residuals
Applied in sequence via Gram-Schmidt orthogonalization:

1. **TechnicalResidual**: Technical indicators residualized against momentum
2. **VolumeResidual**: Volume metrics residualized against momentum + technical  
3. **QualityResidual**: Quality metrics residualized against prior factors
4. **SocialCapped**: Social sentiment, capped at +10 points, applied outside allocation

## Regime-Adaptive Output

Weight profiles automatically adjust based on 4h regime detection:

### Calm Regime (Low Vol, Strong Trend)
```
momentum: 40%, technical: 30%, volume: 20%, quality: 10%
```

### Normal Regime (Balanced Conditions)
```
momentum: 35%, technical: 25%, volume: 25%, quality: 15%  
```

### Volatile Regime (High Vol, Weak Breadth)
```
momentum: 30%, technical: 20%, volume: 30%, quality: 20%
```

## Console Display Format

```
📊 Top 5 Candidates:
Symbol   | Score | Fresh | Depth | Venue  | Sources | Latency
---------|-------|-------|-------|--------|---------|--------
BTC-USD  |  82.4 |   ●   |   ✓   | Kraken |    4    |   245ms
ETH-USD  |  76.8 |   ●   |   ✓   |  OKX   |    3    |   189ms
SOL-USD  |  74.2 |   ○   |   ✓   | Binance|    2    |   156ms
```

## UX MUST — Live Progress & Explainability

All output provides:
- **Real-time attribution**: Source tracking and latency measurements
- **Gate transparency**: Clear pass/fail status with reasons
- **Factor breakdown**: Individual component contributions to final score
- **Regime awareness**: Current market conditions and weight adjustments
- **Data quality**: Freshness and depth validation with visual indicators