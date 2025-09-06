# Microstructure Gates & Venue Health

Execution feasibility gates using exchange-native L1/L2 data (Binance/OKX/Coinbase). **No aggregators allowed.**

## UX MUST — Live Progress & Explainability

Real-time gate evaluation with comprehensive reporting: depth within ±2%, spread caps by tier, VADR precedence (max of p80 vs tier minimum), and venue health monitoring with "halve_size" triggers.

---

## Overview

The microstructure gates system validates execution feasibility through three primary gates:

1. **Depth Gate**: Sufficient liquidity within ±2% price bounds
2. **Spread Gate**: Bid-ask spread within tier caps (25-80 bps)
3. **VADR Gate**: Volume-Adjusted Daily Range meets tier requirements (1.75-1.85×)

Plus venue health monitoring that triggers size adjustments when reject rates >5%, P99 latency >2000ms, or error rates >3%.

## Liquidity Tiers

### Tier Structure by ADV (Average Daily Volume)

| Tier | ADV Range | Depth Min | Spread Cap | VADR Min | Examples |
|------|-----------|-----------|------------|----------|----------|
| **Tier 1** | $5M+ | $150k | 25 bps | 1.85× | BTC, ETH, USDT |
| **Tier 2** | $1M-$5M | $75k | 50 bps | 1.80× | ADA, SOL, MATIC |
| **Tier 3** | $100k-$1M | $25k | 80 bps | 1.75× | Small caps, new tokens |

### Tier Assignment Logic

```go
// Determined by 24h Average Daily Volume (USD)
func GetTierByADV(adv float64) *LiquidityTier {
    if adv >= 5000000 { return "tier1" }
    if adv >= 1000000 { return "tier2" }
    return "tier3" // For ADV >= $100k
}
```

## Gate Calculations

### 1. Depth Gate

**Calculation**: Sum liquidity within ±2% of last trade price

```go
bidBound := lastPrice * 0.98  // -2%
askBound := lastPrice * 1.02  // +2%

bidDepth := sum(bid.Price * bid.Size) where bid.Price >= bidBound
askDepth := sum(ask.Price * ask.Size) where ask.Price <= askBound
totalDepth := bidDepth + askDepth

passDepth := totalDepth >= tier.DepthMinUSD
```

**Requirements by Tier**:
- Tier 1: $150k total depth
- Tier 2: $75k total depth  
- Tier 3: $25k total depth

### 2. Spread Gate

**Calculation**: Rolling 60-second average of bid-ask spread in basis points

```go
spread := (bestAsk - bestBid) / midPrice * 10000  // bps
rollingAvg := average(spread, 60s_window)

passSpread := rollingAvg <= tier.SpreadCapBps
```

**Caps by Tier**:
- Tier 1: 25 bps maximum
- Tier 2: 50 bps maximum
- Tier 3: 80 bps maximum

### 3. VADR Gate (Volume-Adjusted Daily Range)

**Formula**: VADR = (High - Low) / (Volume × Price / ADV)

**Precedence Logic**: `effectiveMinimum = max(p80_threshold, tier_minimum)`

```go
vadr := priceRange / (volume * currentPrice / adv)
p80 := percentile(historicalVADR, 0.80)  // 80th percentile of 24h history
effectiveMin := max(p80, tier.VADRMinimum)

passVADR := vadr >= effectiveMin
```

**Tier Minimums**:
- Tier 1: 1.85× baseline
- Tier 2: 1.80× baseline
- Tier 3: 1.75× baseline

**Why P80 Precedence**: Market conditions can elevate VADR requirements above tier minimums. The 80th percentile ensures gates adapt to current market liquidity while maintaining tier-appropriate floors.

## Venue Health Monitoring

### Health Triggers

System monitors venue performance over 15-minute rolling windows:

| Metric | Threshold | Action |
|--------|-----------|--------|
| Reject Rate | >5% | halve_size |
| P99 Latency | >2000ms | halve_size |
| Error Rate | >3% | halve_size |
| **Multiple Triggers** | 2+ failures | avoid |

### Implementation

```go
// Track all API requests
healthMonitor.RecordRequest(venue, endpoint, latencyMs, success, statusCode, errorCode)

// Evaluate health every gate check
health, _ := healthMonitor.GetVenueHealth(venue)
if !health.Healthy {
    recommendation = health.Recommendation  // "halve_size" or "avoid"
}
```

### Venue Recovery

Venues recover to "full_size" when metrics return below thresholds for sustained periods:
- **Automatic recovery**: 20 consecutive successful requests
- **Time-based recovery**: 48 hours without threshold violations
- **Manual override**: Available for operational emergencies

## Gate Evaluation Process

### 1. Input Validation

```go
// Required inputs
symbol string        // e.g., "BTC-USD"
venue string         // "binance", "okx", "coinbase"
orderbook *OrderBookSnapshot  // L1/L2 data
adv float64         // 24h Average Daily Volume (USD)

// Validation
- venue must be in supportedVenues
- orderbook age <= 5 seconds
- minimum 5 levels each side
- ADV > 0
```

### 2. Tier Assignment

```go
tier := getTierByADV(adv)
```

### 3. Gate Evaluation

```go
depthPass := calculateDepth(orderbook) >= tier.DepthMinUSD
spreadPass := calculateSpread(orderbook) <= tier.SpreadCapBps  
vadrPass := calculateVADR(adv, volume, priceRange) >= max(p80, tier.VADRMinimum)

executionFeasible := depthPass && spreadPass && vadrPass
```

### 4. Venue Health Check

```go
venueHealth := healthMonitor.GetVenueHealth(venue)
if executionFeasible && venueHealth.Healthy {
    recommendation = "proceed"
} else if executionFeasible && venueHealth.Recommendation == "halve_size" {
    recommendation = "halve_size"
} else {
    recommendation = "defer"
}
```

### 5. Report Generation

```go
type GateReport struct {
    DepthOK, SpreadOK, VadrOK bool
    ExecutionFeasible bool
    RecommendedAction string  // "proceed", "halve_size", "defer"
    FailureReasons []string
    Details GateDetails
}
```

## Usage Examples

### Basic Gate Evaluation

```go
evaluator := NewMicrostructureEvaluator(DefaultConfig())

report, err := evaluator.EvaluateGates(
    ctx, 
    "BTC-USD", 
    "binance", 
    orderbook, 
    10000000,  // $10M ADV
)

if report.ExecutionFeasible {
    switch report.RecommendedAction {
    case "proceed":
        // Execute at full size
    case "halve_size":
        // Execute at 50% of intended size
    case "defer":
        // Wait for better conditions
    }
}
```

### Venue Health Monitoring

```go
// Record API performance
evaluator.RecordVenueRequest("binance", "orderbook", 150, true, 200, "")
evaluator.RecordVenueRequest("binance", "orders", 2500, false, 429, "rate_limit")

// Check health status
health, _ := evaluator.GetVenueHealth("binance")
if !health.Healthy {
    log.Printf("Venue unhealthy: reject=%.1f%%, latency=%dms, errors=%.1f%%",
        health.RejectRate, health.LatencyP99Ms, health.ErrorRate)
}
```

### Tier Analysis

```go
tierManager := NewLiquidityTierManager()

tier, _ := tierManager.GetTierByADV(3000000)  // $3M ADV
fmt.Printf("Symbol assigned to %s: depth=$%.0f, spread=%.0fbps, vadr=%.2f",
    tier.Name, tier.DepthMinUSD, tier.SpreadCapBps, tier.VADRMinimum)
```

## Configuration

### Default Config

```yaml
spread_window_seconds: 60    # Rolling average window
depth_window_seconds: 60
max_data_age_seconds: 5      # Stale data threshold
min_book_levels: 5           # Minimum L1/L2 levels

# Venue health thresholds
reject_rate_threshold: 5.0   # 5%
latency_threshold_ms: 2000   # 2 seconds P99
error_rate_threshold: 3.0    # 3%

supported_venues: ["binance", "okx", "coinbase"]
```

### Custom Tiers

```yaml
liquidity_tiers:
  - name: "tier1"
    adv_min: 5000000
    depth_min_usd: 150000
    spread_cap_bps: 25
    vadr_minimum: 1.85
```

## Testing

### Synthetic Order Books

The test suite includes synthetic order book generators for each tier:

```go
func TestTier1Gates(t *testing.T) {
    orderbook := createTier1OrderBook("BTC-USD", 50000.0)
    // Tight spread (20 bps), deep liquidity ($5M+ each side)
    
    report, _ := evaluator.EvaluateGates(ctx, "BTC-USD", "binance", orderbook, 10000000)
    assert.True(t, report.ExecutionFeasible)
    assert.Equal(t, "proceed", report.RecommendedAction)
}
```

### Venue Health Simulation

```go
func TestVenueHealthIntegration(t *testing.T) {
    // Simulate unhealthy venue
    evaluator.RecordVenueRequest("binance", "orderbook", 3000, false, 500, "timeout")
    
    report, _ := evaluator.EvaluateGates(ctx, "BTC-USD", "binance", orderbook, 10000000)
    assert.Equal(t, "halve_size", report.RecommendedAction)
}
```

## Monitoring & Metrics

### Key Metrics

- **Gate Pass Rates**: % of symbols passing each gate by tier
- **Venue Health**: Uptime, reject rates, latency percentiles
- **VADR Precedence**: How often P80 > tier_minimum
- **Data Quality**: Freshness, completeness, stability

### Alerting

- Venue health degradation (reject rate >5%)
- Cross-venue spread divergence (>50 bps difference)
- VADR compression (P80 drops below tier minimums)
- Order book staleness (>5 second update gaps)

## Limitations & Future Enhancements

### Current Limitations

- **Historical VADR**: Requires 4+ hours of data for reliable P80
- **Single Venue**: Gates evaluate per-venue, no cross-venue optimization
- **Static Tiers**: ADV-based only, no dynamic market regime adjustments

### Planned Enhancements

- **Smart Routing**: Cross-venue execution when individual venues fail gates
- **Dynamic Tiers**: Market regime-aware tier adjustments
- **Predictive Health**: ML-based venue health forecasting
- **Latency Optimization**: Sub-100ms gate evaluation targets

---

*CryptoRun Microstructure Gates — Exchange-native execution feasibility validation with venue health monitoring*