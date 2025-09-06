# CryptoRun Guards System

## UX MUST — Live Progress & Explainability

Guards provide real-time entry validation with live progress indicators and detailed explanations for every decision. Each guard evaluation shows:

- **Progress breadcrumbs**: Start → Processing → Evaluating → Complete
- **Per-asset outcomes**: PASS/FAIL with specific reasons
- **Fix hints**: Actionable guidance for failures
- **Regime context**: How current market conditions affect thresholds

## Guard Matrix Overview

CryptoRun implements a comprehensive guard system that prevents poor entries through multiple validation layers. Guards are **regime-aware**, adjusting thresholds based on current market conditions detected every 4 hours.

### Guard Types by Category

| Category | Guard | Purpose | Hard/Soft | Regime Aware |
|----------|-------|---------|-----------|--------------|
| **Freshness** | Bar Age | Prevent stale data entries | Hard | ✅ |
| | Price Movement | Detect excessive intra-bar moves | Hard | ✅ |
| **Fatigue** | Momentum + RSI | Block overextended moves | Hard | ✅ |
| **Liquidity** | Spread | Ensure tight bid-ask spreads | Hard | ❌ |
| | Depth | Require minimum order book depth | Hard | ❌ |
| | VADR | Volume-adjusted daily range check | Hard | ❌ |
| **Caps** | Social Score | Limit social media influence | Soft | ✅ |
| | Brand Score | Limit brand/narrative influence | Soft | ✅ |
| | Catalyst Heat | Prevent event-driven FOMO | Soft | ❌ |
| **Policy** | Late Fill | Block fills >30s after signal | Hard | ❌ |
| | Budget | Prevent oversizing | Hard | ❌ |

## Regime-Aware Thresholds

### Market Regimes
- **Calm**: Low volatility, trending conditions (relaxed thresholds)
- **Normal**: Baseline market conditions (standard thresholds)  
- **Volatile**: High volatility, choppy conditions (strict thresholds)

### Freshness Guard Thresholds

| Parameter | Calm | Normal | Volatile | Unit |
|-----------|------|--------|----------|------|
| Max Bar Age | 3 | 2 | 1 | bars |
| ATR Factor | 1.5× | 1.2× | 1.0× | multiplier |

**Logic**: Stricter freshness requirements in volatile markets prevent entering on stale signals that may have reversed.

### Fatigue Guard Thresholds

| Parameter | Calm | Normal | Volatile | Unit |
|-----------|------|--------|----------|------|
| 24h Momentum Limit | 10.0% | 12.0% | 15.0% | percentage |
| RSI 4h Limit | 70.0 | 70.0 | 70.0 | RSI units |

**Logic**: Higher momentum tolerance in volatile regimes, but RSI overbought level stays constant.

### Social/Brand Cap Thresholds

| Parameter | Calm | Normal | Volatile | Unit |
|-----------|------|--------|----------|------|
| Social Score Cap | 12.0 | 10.0 | 8.0 | score points |
| Brand Score Cap | 8.0 | 6.0 | 5.0 | score points |

**Logic**: Stricter social/narrative limits in volatile markets to prevent FOMO-driven entries.

## Static Thresholds (All Regimes)

### Liquidity Guards
| Guard | Threshold | Unit | Rationale |
|-------|-----------|------|-----------|
| Spread | 50.0 | basis points | Ensure reasonable execution costs |
| Depth | $100,000 | USD within ±2% | Minimum liquidity for position sizing |
| VADR | 1.75× | multiplier | Volume-adjusted daily range validation |

### Cap Guards  
| Guard | Threshold | Unit | Purpose |
|-------|-----------|------|---------|
| Catalyst Heat | 10.0 | score points | Prevent event-driven FOMO entries |

### Policy Guards
| Guard | Threshold | Unit | Purpose |
|-------|-----------|------|---------|
| Late Fill | 30 | seconds | Prevent fills on stale signals |
| Budget | varies | USD/percentage | Position sizing limits |

## Guard Evaluation Flow

### 1. Freshness Validation
```
Bar Age Check → Price Movement Check → PASS/FAIL
```
- **Bar Age**: Data recency within regime limits
- **Price Movement**: Intra-bar price change within ATR bounds

### 2. Fatigue Validation  
```
24h Momentum Check → RSI Check → Combined Logic → PASS/FAIL
```
- **Momentum Check**: 24h price change vs regime threshold
- **RSI Check**: 4h RSI vs overbought level (70)
- **Combined**: FAIL if both momentum AND RSI exceed limits

### 3. Liquidity Validation
```
Spread Check → Depth Check → VADR Check → PASS/FAIL
```
- **Spread**: Bid-ask spread in basis points
- **Depth**: Order book depth within ±2% of mid price
- **VADR**: Volume-adjusted daily range validation

### 4. Cap Validation
```
Social Cap → Brand Cap → Catalyst Cap → Soft PASS/FAIL
```
- **Social Cap**: Social media score vs regime limit
- **Brand Cap**: Brand/narrative score vs regime limit  
- **Catalyst Cap**: Event heat vs static limit

### 5. Policy Validation
```  
Late Fill Check → Budget Check → Hard PASS/FAIL
```
- **Late Fill**: Time since signal bar close with p99 latency relaxation
- **Budget**: Position size vs available capital

## Late-Fill Guard with P99 Relaxation

### Overview
The Late-Fill Guard prevents execution on stale signals while providing intelligent relaxation during infrastructure slowdowns. It uses real-time p99 latency monitoring to grant bounded grace windows when pipeline latency degrades.

### Core Logic
```
Base Threshold: 30s maximum delay under normal conditions
P99 Threshold: 400ms pipeline latency limit
Grace Window: 30s additional delay when p99 exceeded
Hard Limits: ≤2 bars age AND ≤1.2×ATR price movement
```

### Evaluation Flow
1. **Freshness Check** (Hard Limits - Never Relaxed)
   - Bar age must be ≤2 bars maximum
   - Price movement must be ≤1.2×ATR from trigger point

2. **Base Threshold Check**
   - If delay ≤30s: ALLOW immediately
   - If delay >30s: Proceed to p99 evaluation

3. **P99 Relaxation Logic**
   - Check current pipeline p99 latency from order stage
   - If p99 ≤400ms: BLOCK with standard late-fill reason
   - If p99 >400ms: Check relaxation availability

4. **Single-Fire Relaxation**
   - Each asset can use relaxation once per 30-minute window
   - Grace extends threshold to 60s total (30s base + 30s grace)
   - Reason logged as: `latefill_relax[p99_exceeded:<ms>,grace:30s]`

### Golden Reason Strings
```
✅ PASS: "within base threshold: 25000.0ms ≤ 30000.0ms"
✅ PASS: "p99 relaxation applied: 45000.0ms ≤ 60000.0ms (base + grace)"
❌ FAIL: "freshness violation: bar age 3 > 2 bars maximum" 
❌ FAIL: "freshness violation: price distance 1.50×ATR > 1.2×ATR maximum"
❌ FAIL: "late fill: 35000.0ms > 30000.0ms base threshold (p99 350.0ms ≤ 400.0ms threshold)"
❌ FAIL: "late fill: 35000.0ms > 30000.0ms base threshold (p99 relax on cooldown until 14:35:00)"
❌ FAIL: "excessive delay even with p99 grace: 70000.0ms > 60000.0ms (base + grace)"
🔄 RELAX: "latefill_relax[p99_exceeded:450.2ms,grace:30s]"
```

### Menu Integration
Progress logs show p99 relaxation when applied:
```
🛡️ [80%] Evaluating late-fill guards (p99: 450.2ms > 400ms threshold)...
🔄 P99 relax applied to ETHUSD: latefill_relax[p99_exceeded:450.2ms,grace:30s]

🔄 P99 Relaxations Applied:
   ETHUSD: latefill_relax[p99_exceeded:450.2ms,grace:30s]
Note: 1 asset(s) used late-fill p99 relaxation (30m cooldown active)
```

### Performance Characteristics
- **Latency Tracking**: Rolling histogram with 1000-sample window
- **P99 Calculation**: Real-time percentile from order stage telemetry  
- **Cooldown Tracking**: Per-asset timestamp map with 30m expiry
- **Thread Safety**: All operations use proper mutex synchronization

## Guard Failure Reasons & Fix Hints

### Freshness Guard Failures
| Reason | Fix Hint |
|--------|----------|
| `Bar age 3 > 2 bars maximum` | Wait for fresh data or increase bar age tolerance |
| `Price move 1.40×ATR > 1.20×ATR limit` | Wait for price stabilization or increase ATR tolerance |

### Fatigue Guard Failures
| Reason | Fix Hint |
|--------|----------|
| `24h momentum 17.0% > 15.0% + RSI4h 75.0 > 70.0` | Wait for momentum cooldown or RSI retreat |

### Liquidity Guard Failures
| Reason | Fix Hint |
|--------|----------|
| `Spread 75.0 bps > 50.0 bps limit` | Wait for tighter spread or increase spread tolerance |
| `Depth $80k < $100k minimum, VADR 1.5× < 1.75× minimum` | Wait for improved liquidity or reduce depth requirements |

### Cap Guard Failures
| Reason | Fix Hint |
|--------|----------|
| `Social score 12.0 exceeds 10.0 cap` | Reduce social factor weighting or wait for cooling |
| `Brand score 7.5 exceeds 6.0 cap` | Reduce brand factor weighting or wait for normalization |
| `Catalyst heat 15.0 exceeds 10.0 cap` | Wait for event cooling or reduce catalyst sensitivity |

## Exit Codes & CI Integration

### Exit Code Behavior
- **Exit Code 0**: All candidates pass all guards
- **Exit Code 1**: Any hard guard failure detected
- **Hard Guards**: Freshness, Fatigue, Liquidity, Late Fill, Budget
- **Soft Guards**: Social Cap, Brand Cap, Catalyst Cap (warnings only)

### CI/CD Integration
Guards automatically fail CI builds when hard guard violations are detected, preventing deployment of configurations that would generate poor entries.

## Testing & Validation

### End-to-End Test Suite

CryptoRun provides comprehensive guard testing through the `internal/application/guards/e2e/` test suite with seeded fixtures and golden file validation.

**Test Infrastructure:**
- **Seeded Fixtures**: Deterministic test data in `testdata/guards/*.json`
- **Golden Files**: Expected outputs in `testdata/guards/golden/*.golden`
- **Mock Evaluator**: `testkit.MockGuardEvaluator` for consistent test results
- **Menu Integration**: `internal/application/menu/e2e/` tests for UX validation

**Running Tests:**
```bash
# Run all guard tests
go test ./... -run Guards -count=1

# Run specific test categories
go test ./internal/application/guards/e2e -v
go test ./internal/application/menu/e2e -v -run MenuGuard

# Benchmark guard evaluation performance
go test ./internal/application/guards/e2e -bench=.
```

### Golden File Testing
Each guard type has deterministic golden file tests that validate:
- **Threshold calculations** across all regimes
- **Reason message stability** for consistent UX
- **Progress breadcrumb generation** for live feedback
- **Exit code behavior** for CI integration

**Example Golden File Test:**
```go
func TestFatigueGuardCalmRegime(t *testing.T) {
    fixture := testkit.LoadFixture(t, "fatigue_calm.json")
    evaluator := testkit.NewMockEvaluator(fixture)
    
    result := evaluator.EvaluateAllGuards()
    fixture.AssertExpectedResults(t, result)
    
    // Golden file comparison
    actualOutput := result.FormatTableOutput()
    expectedOutput := testkit.LoadGoldenFile(t, "fatigue_calm.golden")
    
    if actualOutput != expectedOutput {
        t.Errorf("Output mismatch with golden file")
    }
}
```

### Test Coverage Matrix
| Guard Type | Calm Regime | Normal Regime | Volatile Regime | Edge Cases |
|------------|-------------|---------------|-----------------|-------------|
| Freshness | ✅ | ✅ | ✅ | ✅ |
| Fatigue | ✅ | ✅ | ✅ | ✅ |
| Liquidity | ✅ | ✅ | ✅ | ✅ |
| Social Cap | ✅ | ✅ | ✅ | ✅ |
| Brand Cap | ✅ | ✅ | ✅ | ✅ |
| Catalyst Cap | ✅ | ✅ | ✅ | ✅ |

### Seeded Test Data
All tests use seeded, deterministic data fixtures located in `testdata/guards/`:

**Core Guard Fixtures:**
- `fatigue_calm.json` - Fatigue guard testing in calm regime (10% momentum limit)
- `freshness_normal.json` - Freshness guard testing in normal regime (2 bar age, 1.2×ATR)
- `liquidity_gates.json` - Liquidity guard testing (spread 50bps, depth $100k, VADR 1.75×)
- `social_caps.json` - Social/Brand cap testing in volatile regime (8.0/5.0 caps)

**Test Coverage:**
```json
{
  "fatigue_calm.json": {
    "regime": "calm",
    "thresholds": {"momentum_24h": 10.0, "rsi_4h": 70.0},
    "expected": {"pass": 2, "fail": 1, "exit_code": 1}
  },
  "liquidity_gates.json": {
    "regime": "normal", 
    "thresholds": {"spread_bps": 50.0, "depth_usd": 100000, "vadr": 1.75},
    "expected": {"pass": 1, "fail": 2, "exit_code": 1}
  }
}
```

**Golden File Outputs:**
- `testdata/guards/golden/*.golden` - Expected ASCII table outputs for UX regression testing
- Validates emoji usage, table formatting, summary statistics, and failure reasons

## Progress & UX Integration

### Menu Integration
The CryptoRun menu system provides comprehensive guard status viewing:

```
🛡️ Guard Status & Results
┌──────────┬────────┬─────────────┬──────────────────────────────────────┐
│ Symbol   │ Status │ Failed Guard│ Reason                               │
├──────────┼────────┼─────────────┼──────────────────────────────────────┤
│ BTCUSD   │ ✅ PASS │ -           │ -                                    │
│ ETHUSD   │ ❌ FAIL │ fatigue     │ 24h momentum 17.0% > 15.0% + RSI... │
│ SOLUSD   │ ✅ PASS │ -           │ -                                    │
└──────────┴────────┴─────────────┴──────────────────────────────────────┘
Summary: 2 passed, 1 failed (exit code 1)
```

### Progress Breadcrumbs
Live progress indicators during guard evaluation:
```
⏳ Starting guard evaluation (regime: normal)
📊 Processing 3 candidates  
🛡️ [20%] Evaluating freshness guards...
🛡️ [40%] Evaluating fatigue guards...
🛡️ [60%] Evaluating liquidity guards...
🛡️ [80%] Evaluating caps guards...
🛡️ [100%] Evaluating final guards...
✅ Guard evaluation completed
```

## Configuration & Customization

### Config File Integration
Guard thresholds are externalized in `config/quality_policies.json`:

```json
{
  "guards": {
    "freshness": {
      "max_bar_age": {"calm": 3, "normal": 2, "volatile": 1},
      "atr_factor": {"calm": 1.5, "normal": 1.2, "volatile": 1.0}
    },
    "fatigue": {
      "momentum_24h": {"calm": 10.0, "normal": 12.0, "volatile": 15.0}
    }
  }
}
```

### Runtime Adjustment
The menu system allows quick threshold adjustments:
- **Tighten Guards**: Reduce all thresholds by 20%
- **Relax Guards**: Increase all thresholds by 20%  
- **Reset to Defaults**: Restore config file values

## Performance Characteristics

### Evaluation Speed
- **Target**: <50ms per candidate for all guards combined
- **Actual**: ~15-25ms per candidate (well within target)
- **Bottlenecks**: None identified in current implementation

### Memory Usage
- **Per-candidate**: ~2KB for guard evaluation context
- **Total**: Linear scaling with candidate count
- **Optimization**: Reusable evaluation contexts reduce allocation

## Future Enhancements

### Planned Features
- **Dynamic thresholds**: ML-based threshold adjustment
- **Guard composition**: Custom guard combinations per strategy
- **Real-time monitoring**: Live guard performance metrics
- **Alert integration**: Slack/Discord notifications for systematic failures

### Research Areas
- **Predictive guards**: Using price prediction to set dynamic thresholds
- **Cross-asset guards**: Portfolio-level risk constraints
- **Regime prediction**: Forward-looking regime detection for guard tuning

This Guards system ensures safe, regime-aware risk management with comprehensive testing coverage and explainable UX integration.