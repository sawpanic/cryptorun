# 🔀 Regime Detection System

## Overview

CryptoRun's regime detection system implements a **4-hour market regime classifier** that automatically adapts factor weights based on market conditions. The system uses three key market signals with majority voting to classify markets into **Trending Bull**, **Choppy**, or **High Volatility** regimes.

## UX MUST — Live Progress & Explainability

Real-time regime detection with transparent signal attribution and voting breakdown. Users can track regime transitions, stability metrics, and understand exactly why each classification was made.

## Signal Definitions

### 1. Realized Volatility (7-day)
- **Threshold**: 0.25 (25%)
- **Vote Logic**: 
  - `> 0.25` → High Vol vote
  - `≤ 0.25` → Low Vol vote
- **Calculation**: 7-day realized volatility of price returns

### 2. Breadth Above 20MA
- **Threshold**: 0.60 (60%)
- **Vote Logic**:
  - `> 0.60` → Trending Bull vote
  - `≤ 0.60` → Choppy vote
- **Calculation**: Percentage of assets trading above their 20-period moving average

### 3. Breadth Thrust (ADX Proxy)
- **Threshold**: 0.70
- **Vote Logic**:
  - `> 0.70` → Trending Bull vote
  - `≤ 0.70` → Choppy vote
- **Calculation**: Market-wide directional strength indicator

## Majority Voting Logic

The detector performs **majority voting** across all three signals:

1. **High Volatility Override**: If realized volatility > 0.25, regime = High Vol (overrides other votes)
2. **Trending Bull**: Requires 2+ votes for "trending_bull" 
3. **Choppy**: Default regime when trending conditions aren't met

### Confidence Calculation
```
Confidence = (Winning Votes) / (Total Votes)
```

## Regime Weight Presets

### Trending Bull Regime
**Characteristics**: Sustained momentum, lower volatility, directional bias

| Factor | Weight | Notes |
|--------|--------|-------|
| momentum_1h | 0.25 | Core short-term momentum |
| momentum_4h | 0.20 | Medium-term trend |
| momentum_12h | 0.15 | Longer-term momentum |
| momentum_24h | 0.10 | Extended timeframe |
| **weekly_7d_carry** | **0.10** | **Trending-only factor** |
| volume_surge | 0.08 | Volume confirmation |
| volatility_score | 0.05 | Reduced in trends |
| quality_score | 0.04 | Quality overlay |
| social_sentiment | 0.03 | Social factor (capped) |

**Movement Gates**:
- Min Movement: **3.5%** (lower threshold in trends)
- Time Window: 48 hours
- Volume Surge Required: **False**
- Tightened Thresholds: **False**

### Choppy Regime
**Characteristics**: Mixed signals, range-bound, mean-reverting

| Factor | Weight | Notes |
|--------|--------|-------|
| momentum_1h | 0.20 | Reduced short-term |
| momentum_4h | 0.18 | Core timeframe |
| momentum_12h | 0.15 | Medium-term |
| momentum_24h | 0.12 | Extended |
| **weekly_7d_carry** | **0.00** | **No weekly carry in chop** |
| volume_surge | 0.12 | Higher volume emphasis |
| volatility_score | 0.10 | Volatility important |
| quality_score | 0.08 | Quality matters more |
| social_sentiment | 0.05 | Social factor (capped) |

**Movement Gates**:
- Min Movement: **5.0%** (standard threshold)
- Time Window: 48 hours
- Volume Surge Required: **True**
- Tightened Thresholds: **False**

### High Volatility Regime
**Characteristics**: High volatility, defensive positioning, quality focus

| Factor | Weight | Notes |
|--------|--------|-------|
| momentum_1h | 0.15 | Reduced (noisy in vol) |
| momentum_4h | 0.15 | Reduced medium-term |
| momentum_12h | 0.18 | Favor longer timeframes |
| momentum_24h | 0.15 | Extended view |
| **weekly_7d_carry** | **0.00** | **No weekly carry in volatility** |
| volume_surge | 0.08 | Lower weight (can mislead) |
| volatility_score | 0.15 | High volatility awareness |
| quality_score | 0.12 | Quality crucial |
| social_sentiment | 0.02 | Minimal (noise) |

**Movement Gates**:
- Min Movement: **7.0%** (tightened threshold)
- Time Window: **36 hours** (shorter window)
- Volume Surge Required: **True**
- Tightened Thresholds: **True** (higher bars for entry)

## API Interface

### Core Detection
```go
// Start 4-hour detection cycle
api := regime.NewAPI(inputs)
err := api.Start(ctx)

// Get current regime
currentRegime, err := api.GetCurrentRegime(ctx)

// Force manual update (bypasses 4h interval)
result, err := api.ForceUpdate(ctx)
```

### Weight Management
```go
// Get active regime's weights
weights := api.GetActiveWeights()

// Get specific regime weights
trendingWeights, err := api.GetAllWeightPresets()[regime.TrendingBull]

// Validate weight configuration
err := api.ValidateConfiguration()
```

### Monitoring & Analysis
```go
// Get regime change history
history := api.GetRegimeHistory()

// Get API status
status := api.GetAPIStatus()

// Get transition matrix (future: populated with historical data)
transitions := api.GetRegimeTransitions()
```

## Stability Analysis

### Stability Detection
A regime is considered **stable** if:
- No regime changes in the last **2 detection cycles** (8 hours)
- OR it's the first detection (no history)

### Change Tracking
Each regime transition is recorded with:
- **Timestamp**: When the change occurred
- **From/To Regimes**: Previous and new regime
- **Confidence**: Vote confidence for new regime
- **Trigger Hour**: Hour of day when change happened

## Implementation Details

### Update Interval
- **Fixed**: Every 4 hours
- **Override**: `ForceUpdate()` bypasses interval check
- **Scheduling**: Automatic timer-based updates when API is running

### Error Handling
- **Signal Fetch Failures**: Return error and maintain previous regime
- **Invalid Inputs**: Detector validates signal availability
- **API Failures**: Graceful degradation with logging

### Thread Safety
- **Mutex Protection**: All API operations are thread-safe
- **Atomic Updates**: Weight changes are atomic
- **Concurrent Access**: Safe for multiple goroutines

## Configuration

### Detector Config
```yaml
update_interval_hours: 4        # Fixed 4-hour cycle
realized_vol_threshold: 0.25    # 25% volatility threshold
breadth_threshold: 0.60         # 60% breadth threshold  
breadth_thrust_threshold: 0.70  # ADX proxy threshold
min_samples_required: 3         # Minimum data points
```

### Usage Examples

#### Basic Detection
```go
inputs := &MyDetectorInputs{}
api := regime.NewAPI(inputs)

// Start detection cycle
if err := api.Start(ctx); err != nil {
    log.Fatal(err)
}
defer api.Stop()

// Get current weights for scoring
weights := api.GetActiveWeights()
momentumWeight := weights.Weights["momentum_4h"] // 0.20 in choppy, 0.15 in high vol
```

#### Monitoring Regime Changes
```go
for {
    select {
    case <-time.After(time.Minute):
        status := api.GetAPIStatus()
        fmt.Printf("Current regime: %s (last check: %s)\n", 
            status["current_regime"], status["last_check"])
        
        if !status["is_stable"].(bool) {
            fmt.Println("⚠️  Regime change detected recently")
        }
    }
}
```

## Integration with Scoring

The regime detection system integrates with CryptoRun's unified composite scoring:

1. **Weight Updates**: Active regime automatically updates factor weights
2. **Movement Gates**: Regime-specific thresholds for entry/exit decisions  
3. **Social Cap**: Applied consistently across all regimes (+10 points max)
4. **MomentumCore**: Always protected from orthogonalization regardless of regime

## Testing

Comprehensive test coverage includes:
- **Voting Logic**: Majority vote calculations and boundary conditions
- **Stability Detection**: Regime change tracking and stability analysis
- **Weight Validation**: Ensuring all presets sum to ~1.0
- **Error Handling**: Invalid inputs and API failures
- **Boundary Conditions**: Exact threshold testing (≥ vs >)

See `tests/unit/regime/detector_test.go` for full test implementation.

## Future Enhancements

1. **Historical Transition Data**: Populate transition matrix with real data
2. **Machine Learning**: Advanced regime classification beyond voting
3. **Regime Prediction**: Forecast upcoming regime changes
4. **Custom Presets**: User-defined weight configurations per regime
5. **Performance Metrics**: Track regime detection accuracy over time