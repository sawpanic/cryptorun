# Test Strategy — Pre-Movement Detector v3.3

> **Speed Pack 03 — Tests-First Implementation Strategy**  
> Drive implementations through comprehensive failing tests with deterministic fixtures

## UX MUST — Live Progress & Explainability

The test strategy ensures transparent, observable development progress through comprehensive failing tests that serve as living specifications. Each test provides clear failure messages and detailed context, enabling developers to understand exactly what needs to be implemented. Progress tracking through test execution provides real-time visibility into completion status across all Pre-Movement Detector v3.3 components.

## Overview

This document outlines the test-driven development strategy for Pre-Movement Detector v3.3 components. All implementations are driven by **failing tests first**, ensuring comprehensive coverage and contract validation before any production code is written.

## Test Architecture

### Test Categories

1. **Unit Tests** (`tests/unit/premove/`)
   - Portfolio pruning logic (`portfolio_test.go`)
   - Alert governance and rate limiting (`alerts_test.go`)
   - Execution quality tracking (`execution_test.go`)
   - Percentile engine and calibration (`percentiles_test.go`)
   - CVD residual quality monitoring (`cvd_resid_test.go`)

2. **Integration Tests** (`tests/integration/premove/`)
   - Point-in-time backtest loader (`backtest_pit_test.go`)
   - Deterministic fixture processing
   - Cross-component workflow validation

3. **Test Data** (`internal/testdata/premove/`)
   - Deterministic PIT records (`pit_sample.jsonl`)
   - Price/volume fixtures (`bars.csv`, `trades.csv`)
   - No network dependencies — pure fixture-based testing

## Component Test Contracts

### Portfolio Pruner

**Contract**: Enforce risk constraints through systematic candidate filtering

**Key Test Scenarios**:
- **Correlation Matrix Validation**: Detect invalid correlation values (>1.0, non-symmetric)
- **Advanced Pruning Strategies**: Eigenvalue decomposition for correlation handling
- **Dynamic Correlation Windows**: Multi-timeframe correlation calculations (1h, 4h)
- **Sector Rotation Detection**: Identify capital flow patterns between sectors
- **Value-at-Risk Calculation**: Monte Carlo VaR with 95% confidence
- **Expected Shortfall**: Tail risk metrics beyond VaR
- **Factor Attribution**: Decompose returns by momentum, volatility factors
- **Risk Decomposition**: Component risk analysis with correlation effects

### Alerts Governor

**Contract**: Prevent operator fatigue through intelligent rate limiting and context awareness

**Key Test Scenarios**:
- **Standard Rate Limits**: 3/hour, 10/day enforcement with burst allowance
- **High Volatility Allowance**: Increased limits (6/hour) in volatile regimes
- **Manual Override Conditions**: Emergency bypass with condition parsing (`score>90 && gates<2`)
- **Priority-Based Queuing**: Critical alerts override rate limits
- **Fatigue Detection**: Adaptive throttling based on operator response patterns
- **Context Awareness**: Regime-sensitive threshold adjustments
- **Multi-Channel Routing**: Console, webhook, email delivery with fallbacks
- **Circuit Breaker Integration**: Delivery failure handling with recovery

### Execution Tracker

**Contract**: Comprehensive quality scoring and performance optimization

**Key Test Scenarios**:
- **Quality Scoring**: 100-point system (slippage 40%, fill time 30%, size deviation 20%, reject rate 10%)
- **Slippage Tolerance Adaptation**: Dynamic limits based on score and volatility
- **Fill Time Optimization**: ML-driven order parameter tuning
- **Market Impact Modeling**: Almgren-Chriss temporary/permanent impact estimation
- **Failure Pattern Detection**: Systematic failure analysis with recovery strategies
- **Recovery Strategy Selection**: Automated strategy selection (reduce size, increase patience, switch venue)
- **P99 Latency Tracking**: Percentile-based performance monitoring
- **Venue Performance Comparison**: Multi-venue execution quality analysis
- **Profitability Attribution**: PnL attribution by holding period and score correlation

### Percentile Engine

**Contract**: Isotonic calibration with regime awareness and temporal weighting

**Key Test Scenarios**:
- **Pool-Adjacent-Violators**: Monotonic calibration curve fitting
- **Binomial Confidence Intervals**: Wilson method with continuity correction
- **Regime-Aware Calibration**: Separate curves per regime (bull, choppy, high-vol)
- **Temporal Decay Weighting**: 30-day half-life with minimum weights
- **State-Based Hit Rates**: WATCH/PREPARE/PRIME/EXECUTE performance analysis
- **Stratified Analysis**: Performance by sector, market cap, volatility quartile
- **Rolling Window Analysis**: 7-day performance degradation detection
- **Distribution Fitting**: Beta, gamma, lognormal with goodness-of-fit tests
- **Tail Behavior Analysis**: Extreme value modeling with peaks-over-threshold
- **Mixture Model Fitting**: Multi-modal score distributions with EM algorithm
- **Bayesian Updating**: Conjugate priors with evidence incorporation

### CVD Residual Tracker

**Contract**: Signal quality monitoring with degradation detection and recovery

**Key Test Scenarios**:
- **R-Squared Calculation**: Linear regression quality between CVD and price movements
- **Daily Quality Tracking**: 7-day moving average with 20% degradation alerts
- **Regime-Specific Quality**: ANOVA comparison across trending/choppy/volatile regimes
- **Residual Autocorrelation**: Ljung-Box test for independence violations
- **Signal Degradation Detection**: 30-day lookback with 15% threshold and 90% confidence
- **Recovery Detection**: 10% improvement over 3 consecutive periods
- **Pattern Exhaustion Monitoring**: Short-term (7d) vs long-term (30d) performance ratios
- **Order Flow Correlation**: CVD correlation with buy/sell imbalance (60% threshold)
- **Tick-Level Analysis**: Volume-weighted analysis with 5-minute aggregation
- **Market Maker Detection**: 70% passive ratio identification with spread capture
- **Latent Liquidity Estimation**: Hawkes process modeling of hidden depth

## Fixture Strategy

### Deterministic Data

All tests use **deterministic fixtures** with no network dependencies:

- **`pit_sample.jsonl`**: 24 PIT records across 4 regimes with realistic score/outcome patterns
- **`bars.csv`**: OHLCV data aligned with PIT timestamps for movement validation
- **`trades.csv`**: Tick-level trade data for microstructure analysis

### Temporal Consistency

Fixtures maintain strict temporal ordering and cross-validation:
- PIT records in chronological order
- Price movements match outcome flags with 5% threshold
- Trade data aligns with bar summaries
- Regime transitions follow realistic patterns

### Coverage Requirements

Each test fixture ensures:
- **State Coverage**: All detector states (WATCH/PREPARE/PRIME/EXECUTE)
- **Regime Coverage**: All regimes (trending_bull, choppy, high_vol, risk_off)
- **Outcome Coverage**: Both positive and negative movement outcomes
- **Sector Coverage**: L1, DeFi, Infrastructure, Gaming sectors
- **Edge Cases**: Boundary conditions, missing data, validation errors

## Implementation Contract

### Test-First Development

1. **Write Failing Tests**: All functionality driven by comprehensive failing tests
2. **Minimal Implementations**: Write only enough code to make tests pass
3. **Refactor**: Clean up implementation while maintaining test success
4. **Integration**: Ensure component interactions work through integration tests

### Expected Failures

Until implementations are complete, tests **MUST FAIL** with:
```bash
go test ./... -count=1 || echo "expected red"
```

Expected failure reasons:
- Missing type definitions (e.g., `premove.NewIsotonicCalibrator`)
- Unimplemented methods (e.g., `FitIsotonicCurve`)
- Missing package imports
- Undefined constants and configuration structs

### Quality Gates

Before marking tests as passing:
- [ ] **Deterministic**: Multiple test runs produce identical results
- [ ] **Fast**: Unit tests complete in <100ms each
- [ ] **Isolated**: No cross-test dependencies or shared state
- [ ] **Comprehensive**: Edge cases and error conditions covered
- [ ] **Realistic**: Test scenarios match production use patterns

## Integration Points

### Cross-Component Dependencies

Tests validate integration contracts:

1. **Portfolio Pruner → Alerts Governor**: Pruned candidates trigger appropriate alerts
2. **Execution Tracker → Portfolio Pruner**: Quality degradation affects position sizing
3. **Percentile Engine → All Components**: Calibration curves inform scoring thresholds
4. **CVD Tracker → Execution Tracker**: Signal quality affects execution aggressiveness

### Workflow Validation

Integration tests ensure end-to-end functionality:
- PIT backtest loader processes fixtures deterministically
- Calibration curves maintain monotonicity across regimes
- Portfolio constraints are respected during alert generation
- Execution quality feedback loops work correctly

## Success Metrics

### Test Coverage
- **Unit Test Coverage**: >90% line coverage per component
- **Integration Coverage**: All cross-component interfaces tested
- **Fixture Coverage**: All edge cases represented in test data

### Performance
- **Test Speed**: Full suite completes in <30 seconds
- **Determinism**: 100% reproducible results across runs
- **Isolation**: Tests can run in parallel without conflicts

### Quality
- **Documentation**: All test contracts clearly documented
- **Maintainability**: Tests serve as living specification
- **Debuggability**: Clear failure messages with actionable context

---

*Test-driven development ensures robust, well-designed implementations that meet exact specifications while maintaining high quality and reliability.*