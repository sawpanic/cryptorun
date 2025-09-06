# CryptoRun Factor System Documentation

## Overview

The CryptoRun factor system implements a unified composite scoring architecture with protected MomentumCore and orthogonalized residual factors. This document describes each factor category and their implementation.

## Factor Architecture

### Protected Factor
- **MomentumCore**: Never orthogonalized, maintains direct price/volume momentum signal

### Residualized Factors (Gram-Schmidt Order)
1. **TechnicalResidual**: Technical indicators after removing correlation with MomentumCore
2. **VolumeResidual**: Volume analysis after removing correlation with MomentumCore + Technical
3. **QualityResidual**: Quality signals after removing correlation with all previous factors
4. **Social**: Social sentiment (applied as +10 cap outside base allocation)

## Factor Implementations

### MomentumCore (Protected)
Multi-timeframe momentum calculation that is never orthogonalized to preserve direct price signal.

**Components:**
- 1h, 4h, 12h, 24h momentum calculations
- Volume-weighted momentum scoring
- Acceleration detection
- Mean reversion detection

**Weight Range:** 25-50% of total allocation (regime-dependent)

### TechnicalResidual
Technical indicators with MomentumCore correlation removed via Gram-Schmidt orthogonalization.

**Components:**
- RSI (Relative Strength Index)
- MACD (Moving Average Convergence Divergence)  
- Bollinger Band position
- Volume-weighted moving averages
- Support/resistance levels

**Weight Range:** 15-30% of total allocation

### VolumeResidual  
Volume analysis with correlation to MomentumCore and Technical factors removed.

**Components:**
- Volume surge detection
- On-balance volume trends
- Volume-price divergence
- Average daily volume ratios
- Intraday volume patterns

**Weight Range:** 10-25% of total allocation

### QualityResidual (Derivatives)
Quality signals derived from derivatives markets, with correlation to all previous factors removed.

**Components:**
- Cross-venue funding z-score
- Delta open interest residual
- Near/far basis dispersion
- Funding rate divergence signals

**Data Sources (Free Venue-Native APIs):**
- Binance Futures API (`/fapi/v1/fundingHistory`, `/fapi/v1/openInterest`)
- OKX Public API (`/api/v5/public/funding-history`, `/api/v5/public/open-interest`)
- Bybit Public API (`/v5/market/funding/history`, `/v5/market/open-interest`)

**Implementation Details:**

#### Cross-Venue Funding Z-Score
Calculates volume-weighted median funding rate across venues and compares to 30-day historical distribution:

```
funding_z = (current_vwm_funding - historical_mean) / historical_std
```

- **Volume Weighting**: Uses 24h quote volume to weight each venue's contribution
- **Z-Score Clipping**: Negative values clipped to 0, positive values capped at 3.0
- **Quality Assessment**: Based on venue count, data freshness, and historical depth

#### Delta OI Residual
Removes price correlation from open interest changes using OLS regression:

```
delta_oi_residual = actual_oi_change - predicted_oi_change_from_price
```

- **OLS Model**: `ΔOI = α + β * ΔPrice + ε`  
- **Window**: 1-hour sliding window for regression
- **Quality Threshold**: R² ≥ 0.1 required for meaningful correlation

#### Basis Dispersion
Analyzes disagreement in basis curves across venues and time horizons:

```
basis_dispersion = std_dev(venue_basis_rates)
```

- **Basis Calculation**: Uses funding rates as proxy for perpetual vs quarterly basis
- **Cross-Venue Spread**: Max basis - Min basis across venues
- **Signal Categories**: backwardation_stress, contango_normal, high_disagreement, neutral

**Quality Blend Formula:**
```
quality_score = w1 * clip(-funding_z, 0, 3) + w2 * |oi_residual| + w3 * basis_dispersion
```

Default weights: w1=0.4, w2=0.35, w3=0.25

**Weight Range:** 8-20% of total allocation

### Catalyst-Heat
Event-driven catalyst heat calculation with time-decay buckets matching PRD specifications.

**Time-Decay Buckets (PRD Specifications):**
- **Imminent** (0-4w): 1.2× multiplier - highest impact for near-term events
- **Near-term** (4-8w): 1.0× multiplier - baseline impact for medium-horizon events  
- **Medium** (8-16w): 0.8× multiplier - reduced impact for longer-term events
- **Distant** (16w+): 0.6× multiplier - minimal impact for far-future events

**Event Sources:**
- CoinMarketCal (free tier) - regulatory decisions, upgrades, partnerships
- Exchange announcements (robots.txt compliant) - listings, maintenance, staking launches
- Comprehensive caching with Redis and source-specific TTLs

**Heat Calculation:**
- Base Heat = decay_multiplier × tier_weight × 100
- Event Heat = base_heat × polarity (-1 for negative, +1 for positive, 0 for neutral)
- Aggregation: "smooth" (diminishing returns) or "max" (highest event)
- Output: 0-100 range (negative events <50, positive events >50)

**Implementation:** `src/application/factors/catalyst.go`, `src/domain/catalyst/heat.go`

### Social (Additive Cap)
Social sentiment factors applied as additive bonus outside the base 100% allocation.

**Components:**
- Reddit sentiment analysis
- Twitter/X mention tracking  
- Social volume indicators
- Viral content detection

**Hard Cap:** +10 points maximum (never part of base weight allocation)
**Application:** Added after all other factors are calculated and normalized

## Entry Gates Integration

### Funding Divergence Gate
Uses QualityResidual derivatives data for entry gate validation:

**Condition:** Funding divergence present if:
- Volume-weighted median funding z-score ≤ -2.0 (negative funding stress)
- Current price ≥ 102% of 24h VWAP (price above recent average)
- Minimum 2 venues with valid data
- Funding data ≤ 2 hours old

**Implementation:** `src/application/gates/funding_divergence.go`

## Data Sources & Rate Limits

### Free Exchange APIs (No API Keys Required)

**Binance Futures:**
- Endpoint: `https://fapi.binance.com`
- Rate Limit: 1200/min (20/sec)
- TTL: 120s for funding history, 60s for open interest

**OKX Public:**
- Endpoint: `https://www.okx.com`
- Rate Limit: 10/sec (conservative)
- TTL: 120s for funding history, 60s for open interest

**Bybit Public:**
- Endpoint: `https://api.bybit.com`  
- Rate Limit: 10/sec (conservative)
- TTL: 120s for funding history, 60s for open interest

### Cache Configuration
```yaml
cache:
  funding_history:
    ttl_seconds: 120
    max_entries: 1000
  open_interest: 
    ttl_seconds: 60
    max_entries: 500
  metrics:
    ttl_seconds: 300
    max_entries: 100
```

### Budget Guards
- Max 100 requests/minute across all providers
- Max 5000 requests/hour total budget
- Circuit breaker at 20% error rate
- 5-minute cooldown period

## Configuration

Factor weights are regime-adaptive via `config/regimes.yaml`:

```yaml
weights:
  trending_bull:
    momentum: 50.0
    technical: 20.0 
    volume: 15.0
    quality: 10.0
    catalyst: 5.0
  
  choppy:
    momentum: 35.0
    technical: 30.0
    volume: 15.0
    quality: 15.0
    catalyst: 5.0
    
  high_vol:
    momentum: 30.0
    technical: 25.0
    volume: 20.0
    quality: 20.0  
    catalyst: 5.0
```

Derivatives-specific configuration in `config/derivs.yaml`:

```yaml
quality_weights:
  funding_zscore: 0.4
  delta_oi_residual: 0.35
  basis_dispersion: 0.25
  
  funding_z_clip_max: 3.0
  funding_z_clip_min: 0.0
```

## Testing

### Unit Tests
- `tests/unit/derivs/metrics_test.go`: Z-score calculation, OLS regression, basis analysis
- Volume-weighted median calculation validation
- Edge case handling (insufficient data, identical rates, etc.)

### Integration Tests  
- `tests/integration/derivs/cache_test.go`: TTL compliance, rate limiting, performance
- Mock provider testing with failure simulation
- Cache hit rate validation (target >70%)

### Performance Requirements
- API calls: <2s P99 latency
- Cache hit rate: >70% target
- Memory usage: <50MB for derivatives data
- TTL compliance: Strict adherence to configured cache periods

## Monitoring

### Key Metrics
- Cache hit rates by data type
- API response times by provider
- Error rates and circuit breaker triggers
- Budget consumption tracking

### Quality Indicators
- Funding z-score data quality assessment
- OI residual R² values
- Cross-venue basis dispersion levels
- Signal strength distributions

### Alerts
- Cache hit rate <70%
- API latency >2s P99
- Error rate >20%
- Budget utilization >80%

---

**Last Updated:** 2025-01-06  
**Version:** 3.2.1  
**Next Review:** Q2 2025