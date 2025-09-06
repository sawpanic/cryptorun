# CryptoRun Data Sources

## UX MUST — Live Progress & Explainability

This document outlines all data sources used by CryptoRun for enhanced composite scoring, including the new measurement data pipelines implemented to provide comprehensive market insights.

## Overview

CryptoRun combines traditional momentum factors with three new measurement data sources to provide enhanced scoring and explainability:

1. **Cross-Venue Funding Divergence** - Z-score analysis of funding rates across venues
2. **Open Interest Residual** - 1-hour OI delta after price regression
3. **ETF Flow Tint** - Daily net flows normalized by 7-day average daily volume

All measurement sources use free/keyless endpoints with comprehensive caching (TTL ≥ 600s) and point-in-time integrity.

## Funding Rate Divergence

### Overview
Tracks funding rate divergence across major cryptocurrency venues using 7-day rolling statistics to identify significant cross-venue arbitrage opportunities.

### Data Sources
- **Binance** - Perpetual futures funding rates (free API)
- **OKX** - Derivatives funding rates (free API)  
- **Bybit** - Perpetual contract funding rates (free API)
- **Deribit** - Options and futures funding (free API, BTC/ETH only)

### Calculation Method
```
1. Collect current funding rates across all venues
2. Calculate venue median (robust to outliers)
3. Compute 7-day rolling mean (μ) and standard deviation (σ)
4. Z-score = (current_median - μ_7d) / σ_7d
5. Entry gate: |z-score| >= 2.0 AND max_venue_divergence >= 5bps
```

### Implementation
- **Provider**: `internal/data/derivs/funding.go`
- **Cache TTL**: 600 seconds (10 minutes)
- **Disk Cache**: `./cache/funding/{symbol}.json`
- **Historical Data**: 7-day rolling window + 1-day buffer
- **Signature Hash**: SHA256-based data integrity validation

### Entry Gate Impact
- **Z-score ≥ 2.5σ**: +2.0 points composite boost
- **Z-score ≥ 2.0σ**: +1.0 points composite boost
- **Required**: Must have funding divergence present for maximum impact

## Open Interest Residual

### Overview
Measures the portion of 1-hour open interest changes that cannot be explained by price movements, indicating independent position building or unwinding.

### Data Sources
- **Binance** - Perpetual futures open interest (free API)
- **OKX** - Derivatives open interest data (free API)
- **Exchange-native only** - No aggregators permitted

### Calculation Method
```
1. Collect 1h ΔOI and 1h ΔPrice data
2. Perform 7-day rolling OLS regression: ΔOI = α + β*ΔPrice + ε
3. Current residual = ΔOI_1h - β*ΔPrice_1h
4. β bounds check: 0.1 ≤ β ≤ 10.0 (regression validity)
5. R² reporting for regression quality assessment
```

### Implementation
- **Provider**: `internal/data/derivs/openinterest.go`
- **Cache TTL**: 600 seconds (10 minutes)
- **Disk Cache**: `./cache/oi/{symbol}.json`
- **Historical Data**: 7-day regression window
- **Beta Calculation**: Ordinary Least Squares (OLS) with bounds checking

### Entry Gate Impact
- **|OI Residual| ≥ $2M**: +1.5 points composite boost
- **|OI Residual| ≥ $1M**: +0.5 points composite boost
- **Direction**: Positive residual = position buildup, Negative = unwinding

## ETF Flow Tint

### Overview
Tracks daily net creation/redemption flows for major cryptocurrency ETFs, normalized by 7-day average daily volume to provide flow intensity measurement.

### Data Sources (Free/Keyless)
- **BlackRock iShares** - IBIT daily creation/redemption data
- **Grayscale** - GBTC net flows from public dashboards
- **Fidelity** - FBTC daily net flows
- **ARK Invest** - ARKB creation/redemption data
- **VanEck** - HODL daily flows

### Calculation Method
```
1. Aggregate daily net flows across all major ETFs
2. Calculate 7-day average daily volume (ADV)
3. Flow tint = clamp(net_flow_USD / ADV_USD_7d, -0.02, 0.02)
4. Clamping ensures ±2% maximum impact relative to daily volume
```

### Implementation
- **Provider**: `internal/data/etf/flows.go`
- **Cache TTL**: 86400 seconds (24 hours, daily data)
- **Disk Cache**: `./cache/etf/{symbol}.json`
- **Historical Data**: 7-day ADV calculation window
- **Flow Aggregation**: Sum across all contributing ETFs

### Entry Gate Impact
- **|Flow Tint| ≥ 1.5% ADV**: +1.0 points composite boost
- **|Flow Tint| ≥ 1.0% ADV**: +0.5 points composite boost
- **Direction**: Positive tint = net inflows, Negative = net outflows

## Caching Architecture

### Multi-Tier Caching Strategy
1. **Memory Cache** - Immediate access, per-provider instance
2. **Disk Cache** - Persistent JSON files with TTL enforcement
3. **TTL Enforcement** - All providers respect minimum 600s cache TTL
4. **Cache Invalidation** - Automatic based on monotonic timestamps

### Cache File Structure
```json
{
  "symbol": "BTCUSD",
  "monotonic_timestamp": 1704067200,
  "signature_hash": "abc123def456",
  "cache_hit": false,
  "source": "binance-rest",
  // ... measurement-specific fields
}
```

### Data Integrity
- **Monotonic Timestamps** - Ensures point-in-time consistency
- **Signature Hashes** - SHA256-based content verification
- **Source Attribution** - Clear data provenance tracking
- **Cache Hit Reporting** - Performance and freshness monitoring

## Integration with Composite Scoring

### Enhanced Scoring Pipeline
1. **Base Factors** - Traditional momentum, technical, volume, quality
2. **Measurement Enhancement** - Gather funding, OI, ETF data
3. **Boost Calculation** - Apply measurement-based score boosts (max +4 points)
4. **Orthogonalization** - Gram-Schmidt with MomentumCore protection
5. **Final Scoring** - Enhanced score with measurement insights

### Score Boost Caps
- **Individual Measurements**: Funding (+2 max), OI (+1.5 max), ETF (+1 max)
- **Total Measurement Boost**: Capped at +4.0 points maximum
- **Final Score Range**: 0-114 (100 base + 10 social + 4 measurement)

## Explainability and Attribution

### Enhanced Insights
- **Funding Insight**: "Strong funding premium (2.8σ)" 
- **OI Insight**: "Significant OI buildup ($3.2M residual)"
- **ETF Insight**: "Moderate ETF inflow (1.2% of ADV)"
- **Data Quality**: "Complete (3/3 sources)" or "Limited (1/3 sources)"

### Attribution Strings
- **Funding**: "Cross-venue 7d σ analysis from Binance/OKX/Bybit"
- **OI**: "1h Δ with β-regression residual from Binance/OKX"
- **ETF**: "Daily net flows from issuer dashboards vs 7d ADV"

### Performance Metrics
- **Cache Hit Rates** - Monitored per data source
- **Data Freshness** - Age since last update in attribution
- **Latency Tracking** - Full pipeline timing for optimization

## API Endpoints and Rate Limits

### Funding Rate APIs
- **Binance**: `https://fapi.binance.com/fapi/v1/fundingRate`
- **OKX**: `https://www.okx.com/api/v5/public/funding-rate`
- **Bybit**: `https://api.bybit.com/v2/public/funding-rate`
- **Rate Limits**: Respect provider-specific limits with exponential backoff

### Open Interest APIs  
- **Binance**: `https://fapi.binance.com/fapi/v1/openInterest`
- **OKX**: `https://www.okx.com/api/v5/public/open-interest`
- **Historical**: 24h endpoint with 1h granularity

### ETF Data Sources
- **Public Dashboards**: Issuer websites with daily updates
- **SEC Filings**: Creation/redemption disclosures (daily lag)
- **Free APIs**: CoinGecko, CoinMarketCap for backup data

## Error Handling and Fallbacks

### Provider Failures
- **Graceful Degradation** - System continues with available data sources
- **Cache Fallback** - Use stale cache data with warnings if providers fail
- **Quality Assessment** - Report data completeness in scoring output

### Data Quality Monitoring
- **Source Availability** - Track successful vs failed data fetches
- **Cache Performance** - Monitor hit rates and TTL effectiveness  
- **Signature Validation** - Detect data corruption through hash mismatches
- **Regression Quality** - R² monitoring for OI beta calculations

## Configuration

### Environment Variables
```bash
FUNDING_CACHE_TTL=600      # Funding data cache TTL (seconds)
OI_CACHE_TTL=600          # Open interest cache TTL (seconds)  
ETF_CACHE_TTL=86400       # ETF data cache TTL (seconds)
MEASUREMENT_MAX_BOOST=4.0  # Maximum measurement boost points
```

### Cache Directories
```
./cache/funding/          # Funding rate cache files
./cache/oi/              # Open interest cache files  
./cache/etf/             # ETF flow cache files
```

### Test Fixtures
```
./testdata/funding/      # Funding test fixtures
./testdata/oi/          # OI test fixtures
./testdata/etf/         # ETF test fixtures
```

## Compliance and Data Usage

### Free Tier Compliance
- **All endpoints**: Free tier, no API keys required
- **Rate Limiting**: Implemented per provider specifications
- **Terms of Service**: Compliant with all provider ToS
- **No Redistribution**: Data used only for internal scoring

### Data Retention
- **Hot Cache**: Memory-based, cleared on restart
- **Disk Cache**: TTL-based automatic cleanup
- **Historical Data**: 8-day retention (7d window + 1d buffer)
- **No Long-term Storage**: All data refreshed regularly

## Future Enhancements

### Additional Data Sources
- **Funding Rate Curves** - Term structure analysis across maturities
- **Cross-Asset OI** - Correlation analysis across crypto derivatives
- **Options Flow** - Put/call ratios and flow analysis
- **Institutional Activity** - Large trader positions and flows

### Advanced Analytics
- **Machine Learning** - Pattern recognition in measurement combinations
- **Cross-Validation** - Measurement effectiveness backtesting
- **Dynamic Thresholds** - Regime-adaptive measurement sensitivity
- **Real-time Alerts** - Threshold breach notifications