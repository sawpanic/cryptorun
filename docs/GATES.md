# CryptoRun Entry Gates System

## UX MUST — Live Progress & Explainability

Complete guide to CryptoRun's entry gate system, providing transparent and deterministic evaluation of trading signal quality with comprehensive explanations and CLI debugging tools.

## Entry Gate Overview

The entry gate system enforces strict criteria before allowing position entries. Gates are evaluated in two phases:

1. **Hard Gates**: Mandatory scoring and microstructure requirements
2. **Guards**: Timing, fatigue, and execution quality checks

Entry is **only allowed** when ALL gates and guards pass.

## Hard Gates (Phase 1)

Hard gates are non-negotiable requirements that must be met for any entry consideration.

### Gate 1: Composite Score ≥ 75
```yaml
threshold: 75.0
description: "Unified composite score must exceed minimum threshold"
short_circuit: true  # Fail fast if not met
```

**Rationale**: Ensures fundamental momentum and quality signals are strong before considering entry.

### Gate 2: Microstructure Gates
#### VADR ≥ 1.8×
```yaml
threshold: 1.8
description: "Volume-Adjusted Daily Range must show sufficient activity"
calculation: "24h volume / (high - low) normalized to daily range"
```

#### Spread ≤ 50 bps
```yaml
threshold: 50.0  # basis points
description: "Bid-ask spread must be tight for efficient execution"
measurement: "60s rolling average spread"
```

#### Depth ≥ $100k within ±2%
```yaml
threshold: 100000.0  # USD
range: 2.0  # percent
description: "Sufficient liquidity for position entry/exit"
measurement: "Combined bid/ask depth within price range"
```

**Critical**: Must use **exchange-native** L1/L2 data only. Aggregators banned for microstructure.

**Implementation**: See [docs/MICROSTRUCTURE.md](./MICROSTRUCTURE.md) for complete L1/L2 collector architecture with health monitoring.

### Gate 3: Funding Divergence Present
```yaml
threshold: 2.0  # standard deviations
requirement: "≤ 0 with price holding"
description: "Cross-venue funding rate divergence signals opportunity"
calculation: "Z-score of venue-median funding vs 7-day rolling stats"
```

**Logic**: Funding divergence ≤ 0 indicates negative/neutral funding rates while price holds, suggesting potential supply squeeze.

### Optional Gates (Configurable)
#### Open Interest Residual ≥ $1M
```yaml
threshold: 1000000.0  # USD
optional: true
description: "Independent position building beyond price-driven OI"
calculation: "1h ΔOI - β*ΔPrice residual from 7d regression"
```

#### ETF Flow Tint ≥ 0.3
```yaml
threshold: 0.3
optional: true  
description: "Positive institutional flows via ETF creation/redemption"
calculation: "Daily net flows normalized by 7d average daily volume"
```

## Guards (Phase 2)

Guards prevent poor timing and overextended entries even when hard gates pass.

### Freshness Guard: Signal Age ≤ 2 Bars
```yaml
threshold: 2  # bars
description: "Signal must be recent to avoid stale opportunities"
calculation: "Bars elapsed since trigger signal generation"
```

**Rationale**: Prevents acting on outdated signals in fast-moving markets.

### Fatigue Guard: Overextension Check
```yaml
conditions:
  price_24h: "> 12%"
  rsi_4h: "> 70"
  logic: "AND (both conditions required for fatigue)"
exceptions:
  pullback_detected: "Override fatigue if recent pullback present"
  acceleration_renewed: "Override fatigue if momentum re-accelerating"
```

**Logic**: 
- IF `price_24h > 12% AND rsi_4h > 70` → Fatigue detected
- UNLESS `pullback OR acceleration` → Allow entry despite fatigue

### Proximity Guard: Price Distance ≤ 1.2× ATR(1h)
```yaml
threshold: 1.2  # ATR multiple
description: "Current price must be near original trigger price"
calculation: "|current_price - trigger_price| ≤ 1.2 × ATR_1h"
```

**Rationale**: Prevents chasing price that has moved significantly from signal.

### Late Fill Guard: Execution Timing < 30s
```yaml
threshold: 30  # seconds
description: "Fill must occur quickly after trigger bar close"
calculation: "fill_time - trigger_bar_close_time < 30s"
```

**Rationale**: Ensures fills are based on current market conditions, not stale triggers.

## Gate Evaluation Process

### Evaluation Order
1. **Hard Gates** (short-circuit on failure)
   - Composite Score → VADR → Spread → Depth → Funding Divergence
   - Optional: OI Residual → ETF Flow Tint
2. **Guards** (all evaluated regardless of individual failures)  
   - Freshness → Fatigue → Proximity → Late Fill

### Short-Circuit Logic
- Hard gates fail fast: if any mandatory gate fails, stop evaluation
- Guards are always fully evaluated for complete reporting
- Final decision: `entry_allowed = all_hard_gates_pass AND all_guards_pass`

## Reason Codes

### Hard Gate Failures
- `score_insufficient`: Composite score below 75.0 threshold
- `vadr_insufficient`: VADR below 1.8× threshold  
- `spread_too_wide`: Spread exceeds 50 bps limit
- `depth_insufficient`: Liquidity below $100k within ±2%
- `funding_divergence_absent`: No significant funding divergence present
- `oi_residual_low`: OI residual below $1M threshold (optional)
- `etf_flows_negative`: ETF flows below 0.3 tint threshold (optional)

### Guard Failures
- `signal_stale`: Signal older than 2 bars
- `fatigue_detected`: 24h >12% + RSI >70 without pullback/acceleration
- `price_moved_away`: Current price >1.2× ATR from trigger
- `late_fill`: Fill occurred >30s after trigger bar close

## Configuration Examples

### Production Configuration
```yaml
# Hard gates
min_composite_score: 75.0
min_vadr: 1.8
max_spread_bps: 50.0
min_depth_usd: 100000.0
depth_range_pct: 2.0

# Funding divergence
min_funding_z_score: 2.0
require_funding_divergence: true

# Optional gates
enable_oi_gate: true
min_oi_residual: 1000000.0
enable_etf_gate: true
min_etf_flow_tint: 0.3

# Guards
max_bars_age: 2
max_seconds_since_trigger: 30
fatigue_price_24h_threshold: 12.0
fatigue_rsi_4h_threshold: 70.0
proximity_atr_multiple: 1.2
```

### High-Volatility Regime Adjustments
```yaml
# Tighter thresholds for volatile markets
proximity_atr_multiple: 1.0  # Reduced from 1.2
max_seconds_since_trigger: 20  # Reduced from 30
fatigue_price_24h_threshold: 10.0  # Reduced from 12.0
```

# Entry & Exit Gate Set (6–48h Horizon)

## Updated for PROMPT_ID=DOCS.FINISHER.UNIFIED.PIPELINE.V1

**Last Updated:** 2025-09-07  
**Version:** v3.3 Unified Pipeline  
**Status:** Implemented

**Entry (all must pass)**
- Composite ≥ 75.
- Movement threshold by regime (≥2.5% bull / 3.0% chop / 4.0% bear).
- Volume surge: VADR ≥ 1.75× (freeze <20 bars).
- Liquidity: ≥$500k 24h; microstructure gates (spread/depth) pass.
- Trend quality: ADX > 25 OR Hurst > 0.55.
- Freshness: ≤2 bars from trigger; late-fill <30s.

**Exit (first trigger wins)**
1) −1.5× ATR hard stop
2) Venue health degrade → tighten +0.3× ATR
3) Time limit: 48h max
4) Acceleration reversal (4h d²<0)
5) Momentum fade (1h & 4h negative)
6) Trailing after 12h: ATR×1.8 unless accelerating
7) Profit targets: +8% / +15% / +25%

**Attribution**
- Each decision logs gate pass/fail reasons and the active regime.

## Entry Gate Set (All Must Pass)

### 1. Composite Score ≥ 75
- **Requirement**: Unified composite score must meet minimum threshold
- **Purpose**: Ensures fundamental momentum and quality signals are strong
- **Attribution**: "Score 82.5 ≥ 75.0 ✓" or "Score 68.2 below 75.0 threshold"

### 2. Movement Threshold by Regime  
- **Trending Bull**: ≥2.5% price movement required
- **Choppy**: ≥3.0% price movement required
- **High Vol**: ≥4.0% price movement required (tightened gates)
- **Purpose**: Filters out insufficient price momentum for current regime
- **Attribution**: "Movement 3.2% ≥ 2.5% (TRENDING_BULL)" or "Movement 1.8% below 3.0% threshold (CHOPPY)"

### 3. Volume Surge: VADR ≥ 1.75× (freeze <20 bars)
- **Requirement**: Volume-Adjusted Daily Range ≥1.75× average
- **Bar Validation**: ≥20 bars required (freeze if <20 bars)
- **Purpose**: Confirms volume surge behind price movement
- **Attribution**: "VADR 2.15× ≥ 1.75×" or "VADR 1.42× below threshold 1.75×" or "Freeze: only 15 bars < 20 minimum"

### 4. Liquidity: ≥$500k 24h; microstructure gates (spread/depth) pass
- **Daily Volume**: Minimum $500,000 daily trading volume
- **Spread Gate**: Bid-ask spread <50bps
- **Depth Gate**: ≥$100k depth within ±2%
- **Purpose**: Ensures sufficient liquidity for entry/exit with tight execution
- **Attribution**: "Volume $1.2M ≥ $500k ✓, Spread 28bps < 50 ✓, Depth $180k ≥ $100k ✓"

### 5. Trend Quality: ADX > 25 OR Hurst > 0.55
- **Requirement**: Either ADX >25 OR Hurst exponent >0.55
- **Purpose**: Validates trend strength or persistence
- **Attribution**: "ADX 32 > 25 ✓" or "ADX 18 ≤ 25, Hurst 0.62 > 0.55 ✓" or "ADX 19 ≤ 25 AND Hurst 0.48 ≤ 0.55 ❌"

### 6. Freshness: ≤2 bars from trigger; late-fill <30s
- **Signal Age**: ≤2 bars from original trigger
- **Fill Timing**: Execution within 30 seconds of signal
- **Purpose**: Ensures signals are recent and fills are timely
- **Attribution**: "Fresh: 1 bar ≤ 2, Fill: 18s < 30s ✓" or "Stale: 3 bars > 2 limit" or "Late fill: 45s > 30s"

## Exit Hierarchy (First Trigger Wins)

The exit system evaluates conditions in strict precedence order. **First trigger wins** - no combination logic.

### 1) −1.5× ATR hard stop
- **Calculation**: Stop = Entry Price - (1.5 × ATR1h)
- **Purpose**: Absolute loss protection (highest precedence)
- **Attribution**: "Hard stop: $43,250 ≤ $43,180 (-1.5×ATR)"

### 2) Venue health degrade → tighten +0.3× ATR
- **Triggers**: P99 latency >2000ms OR error rate >3% OR reject rate >5%
- **Calculation**: Tightened Stop = Entry Price - (0.3 × ATR1h)
- **Purpose**: Protect against venue degradation
- **Attribution**: "Venue degraded, tightened stop: $43,890 ≤ $43,850 (+0.3×ATR tightener)"

### 3) Time limit: 48h max
- **Requirement**: Position held ≥48 hours
- **Purpose**: Prevent indefinite holding
- **Attribution**: "Time limit: 48.2 hours ≥ 48.0 hour max"

### 4) Acceleration reversal (4h d²<0)
- **Requirement**: 4-hour momentum acceleration turns negative
- **Purpose**: Exit on momentum deceleration
- **Attribution**: "Acceleration reversal: 4h d² = -0.015 < 0"

### 5) Momentum fade (1h & 4h negative)
- **Requirement**: Both 1h AND 4h momentum become negative
- **Purpose**: Exit when short and medium-term momentum fade
- **Attribution**: "Momentum fade: 1h=-0.12<0 & 4h=-0.08<0"

### 6) Trailing after 12h: ATR×1.8 unless accelerating
- **Requirement**: Position held ≥12 hours AND not accelerating
- **Calculation**: Stop = High Water Mark - (1.8 × ATR1h)
- **Purpose**: Protect profits with trailing mechanism
- **Attribution**: "Trailing stop: $44,120 ≤ $43,980 (HWM $45,200 - 1.8×ATR)"

### 7) Profit targets: +8% / +15% / +25%
- **Target 1**: +8% profit from entry
- **Target 2**: +15% profit from entry
- **Target 3**: +25% profit from entry
- **Purpose**: Systematic profit taking (lowest precedence)
- **Attribution**: "Profit target 2: $45,230 ≥ $45,175 (+15%)"

## Attribution System

**Each decision logs gate pass/fail reasons and the active regime.**

### Gate Pass/Fail Logging
- **Entry Gates**: Each gate provides specific threshold comparisons
- **Exit Triggers**: Precise calculation showing which condition fired
- **Regime Context**: Active regime included in all attributions
- **Timing Data**: Evaluation timestamps and processing time

### Example Attribution Outputs

**Entry Gate Success**:
```
✅ Entry Approved (TRENDING_BULL regime)
- Score: 82.5 ≥ 75.0 ✓
- Movement: 3.2% ≥ 2.5% (TRENDING_BULL) ✓  
- Volume: VADR 2.15× ≥ 1.75× ✓
- Liquidity: $1.2M ≥ $500k ✓, Spread 28bps < 50 ✓, Depth $180k ≥ $100k ✓
- Trend: ADX 32 > 25 ✓
- Fresh: 1 bar ≤ 2, Fill: 18s < 30s ✓
```

**Exit Trigger Example**:
```
🚪 Exit: Hard Stop (precedence #1)
- Price $43,250 ≤ $43,180 (-1.5×ATR)
- Entry: $45,000, ATR: $1,213
- Duration: 3.2h, PnL: -3.9%
```

## Implementation Status

✅ **Entry Gates Implemented**: All gates from PROMPT_ID requirements  
✅ **Exit Hierarchy Implemented**: First-trigger-wins logic with ATR-based calculations  
✅ **Regime Integration**: Movement thresholds adapt to detected regime  
✅ **Attribution Complete**: Detailed pass/fail explanations for transparency  
✅ **CLI Integration**: Gate status visible in momentum signals menu

This gates system ensures only the highest-quality opportunities with proper risk management controls pass through to execution while providing complete transparency into the decision process.