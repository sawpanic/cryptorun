# Exit Rules and Logic

This document describes the comprehensive exit evaluation system for CryptoRun positions, including rule precedence, trigger conditions, and operational procedures.

## UX MUST — Live Progress & Explainability

All exit evaluations provide clear reasoning with specific trigger values, precedence explanations, and detailed position metrics for transparent decision-making.

## Exit System Overview

The exit system continuously monitors open positions and applies exit rules in strict precedence order. Exit evaluation is **deterministic** - higher precedence rules always override lower ones.

**Core Principle**: Exit immediately when any rule triggers, with reason attribution and timing metrics.

## Exit Rule Precedence

Exit rules are evaluated in strict order from highest to lowest precedence:

1. **Hard Stop** (Highest) - Absolute loss protection
2. **Venue Health Cut** - Exchange performance degradation  
3. **Time Limit** - Maximum hold duration (48h default)
4. **Acceleration Reversal** - Momentum dynamics deterioration
5. **Momentum Fade** - Score degradation from entry
6. **Trailing Stop** - Profit protection mechanism
7. **Profit Target** (Lowest) - Systematic profit taking

**Critical**: Once any rule triggers, evaluation stops - higher precedence rules always win.

## Exit Rule Specifications

### 1. Hard Stop (Precedence: Highest)
```yaml
description: "Absolute loss protection at predetermined price level"
trigger: "current_price ≤ hard_stop_price"
rationale: "Capital preservation takes precedence over all other considerations"
```

**Configuration**:
- Stop loss price calculated at position entry
- Typically 4-6% below entry price depending on volatility regime
- Cannot be disabled - always active for capital protection

### 2. Venue Health Cut (Precedence: 2nd)
```yaml
description: "Exit when exchange performance degrades significantly"
triggers:
  - "P99 latency > 2000ms"
  - "error_rate > 3.0%"  
  - "reject_rate > 5.0%"
logic: "OR (any condition triggers exit)"
rationale: "Cannot trade effectively on degraded venues"
```

### 3. Time Limit (Precedence: 3rd)
```yaml
description: "Maximum position hold duration"
trigger: "hours_held ≥ max_hold_hours"
default: 48.0  # hours
rationale: "Prevents indefinite holds, forces periodic re-evaluation"
```

### 4. Acceleration Reversal (Precedence: 4th)
```yaml
description: "Momentum acceleration has declined significantly"
trigger: "acceleration_change ≤ -50%"
calculation: "((current_accel / entry_accel) - 1.0) * 100"
threshold: -50.0  # percent decline
rationale: "Momentum dynamics deteriorating, trend weakening"
```

### 5. Momentum Fade (Precedence: 5th)
```yaml
description: "Composite momentum score has faded from entry"
trigger: "momentum_change ≤ -30%"
calculation: "((current_score / entry_score) - 1.0) * 100"
threshold: -30.0  # percent decline
rationale: "Underlying momentum factors weakening"
```

### 6. Trailing Stop (Precedence: 6th)
```yaml
description: "Protect profits by following high water mark"
trigger: "current_price ≤ (high_water_mark × (1 - trailing_pct/100))"
default_trailing_pct: 5.0  # 5% trailing distance
requirements:
  - "high_water_mark > entry_price"  # Must have profits first
  - "enable_trailing_stop: true"
```

### 7. Profit Target (Precedence: Lowest)
```yaml
description: "Systematic profit taking at predetermined levels"
targets:
  target_1: 15.0  # 15% profit
  target_2: 30.0  # 30% profit
logic: "Check target_2 first (higher target takes precedence)"
calculation: "target_price = entry_price × (1 + target_pct/100)"
```

## Configuration Examples

### Default Exit Configuration
```yaml
# Hard stop
enable_hard_stop: true

# Venue health thresholds  
max_venue_p99_latency_ms: 2000  # 2 seconds
max_venue_error_rate: 3.0       # 3%
max_venue_reject_rate: 5.0      # 5%

# Time management
default_max_hold_hours: 48.0    # 48 hours

# Momentum thresholds
momentum_fade_threshold: 30.0   # 30% decline
accel_reversal_threshold: 50.0  # 50% decline

# Trailing stop
enable_trailing_stop: true
default_trailing_pct: 5.0       # 5%

# Profit targets
enable_profit_targets: true
default_profit_target_1: 15.0   # 15%
default_profit_target_2: 30.0   # 30%
```

### High-Volatility Regime Adjustments
```yaml
default_trailing_pct: 7.0       # Wider trailing (7% vs 5%)
momentum_fade_threshold: 25.0   # Stricter fade threshold (25% vs 30%)
max_venue_p99_latency_ms: 1500  # Tighter latency (1.5s vs 2s)
```

## Exit Result Format

```json
{
  "symbol": "BTCUSD",
  "timestamp": "2024-01-15T14:30:00Z", 
  "should_exit": true,
  "exit_reason": "hard_stop",
  "reason_string": "hard_stop",
  "triggered_by": "Price 47800.00 ≤ stop 48000.00",
  "current_price": 47800.00,
  "entry_price": 50000.00,
  "unrealized_pnl": -4.4,
  "hours_held": 3.2,
  "evaluation_time_ms": 12
}
```

## Exit Evaluation Process

### Evaluation Flow
1. Calculate position metrics (PnL, duration, etc.)
2. Check Hard Stop → EXIT if triggered  
3. Check Venue Health → EXIT if triggered
4. Check Time Limit → EXIT if triggered
5. Check Acceleration Reversal → EXIT if triggered  
6. Check Momentum Fade → EXIT if triggered
7. Check Trailing Stop → EXIT if triggered
8. Check Profit Targets → EXIT if triggered
9. If no exits triggered → HOLD position

### Short-Circuit Logic
- **First rule match wins** - evaluation stops at first trigger
- Higher precedence rules **always** override lower ones
- No combination logic - single rule determines exit

## Testing and Integration

### Unit Test Coverage
- ✅ Each exit rule triggers independently with correct precedence
- ✅ Multiple conditions met → highest precedence wins
- ✅ All exit rules can be disabled/enabled via configuration
- ✅ Position metrics calculated correctly (PnL, duration)
- ✅ Thresholds properly configurable and validated

### API Integration
```go
import "cryptorun/internal/exits"

evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

inputs := exits.ExitInputs{
    Symbol:       "BTCUSD",
    EntryPrice:   50000.0,
    CurrentPrice: 52000.0,
    EntryTime:    entryTime,
    CurrentTime:  time.Now(),
    HardStopPrice: 48000.0,
}

result, err := evaluator.EvaluateExit(ctx, inputs)
if result.ShouldExit {
    fmt.Printf("🚪 Exit: %s\n", result.TriggeredBy)
}
```