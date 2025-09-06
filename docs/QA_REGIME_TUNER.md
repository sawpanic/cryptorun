# QA Regime Tuner Documentation

## Overview

This document describes the **conformance and empirical QA suite** for the CryptoRun Regime Tuner system. The QA suite provides automated validation that the tuner maintains critical invariants and aligns with backtest evidence.

## UX MUST — Live Progress & Explainability

The QA system provides real-time validation with detailed explanations:
- **Conformance tests** validate architectural constraints continuously
- **Empirical tests** prove signal quality using synthetic backtests  
- **CSV exports** provide interpretable analysis artifacts
- **CI integration** blocks weight-sum drift and social cap violations

## Architecture

### Conformance Tests (`tests/conformance/regime/`)

These tests validate fundamental system invariants that MUST never be violated:

#### Weight Sum Constraints (`weights_sum_100_test.go`)
- **Purpose**: Ensure all regime weights sum to exactly 100% (excluding social cap)
- **Validation**: Tests weight normalization, clamping, and edge cases
- **Failure Condition**: Any regime weights summing outside 100% ±0.1%
- **CI Impact**: BLOCKS deployment if violated

#### Social Cap Enforcement (`social_cap_test.go`) 
- **Purpose**: Verify social factors are hard-capped at +10 points
- **Validation**: Tests cap enforcement, confidence weighting, negative sentiment
- **Failure Condition**: Social contribution exceeds +10 points
- **CI Impact**: BLOCKS deployment if violated

#### Gram-Schmidt Order (`gram_schmidt_order_test.go`)
- **Purpose**: Validate MomentumCore protection and orthogonalization sequence
- **Validation**: Tests factor residualization order and protection
- **Failure Condition**: MomentumCore gets residualized or order violation
- **CI Impact**: BLOCKS deployment if violated

#### Detector Thresholds (`detector_thresholds_table_test.go`)
- **Purpose**: Test regime detection boundary conditions
- **Validation**: Tests all threshold edge cases from `config/regimes.yaml`
- **Failure Condition**: Detection logic inconsistent with config thresholds
- **CI Impact**: BLOCKS deployment if violated

### Empirical Tests (`tests/empirical/`)

These tests validate signal quality using synthetic backtest data:

#### Decile Lift Analysis (`decile_lift_regime_test.go`)
- **Purpose**: Prove higher composite scores → higher forward returns
- **Validation**: Monotonicity check across deciles, top-bottom spreads
- **Acceptance**: ≥8/10 deciles maintain monotonic improvement
- **Data**: Synthetic panel with 10 assets across score deciles

#### Gate Alignment (`gate_alignment_test.go`) 
- **Purpose**: Verify entry gates {Score≥75, VADR≥1.8, funding≥2σ} outperform controls
- **Validation**: Gate-passing vs gate-failing performance comparison
- **Acceptance**: Gate-passing entries outperform controls on average
- **Analysis**: Individual gate contribution and combined gate strength

### CSV Artifacts (`artifacts/`)

Automated export of analysis results for external review:

#### `regime_decile_lift.csv`
```csv
decile,count,avg_composite_score,avg_forward_return_4h,avg_forward_return_24h,min_score,max_score,return_4h_pct,return_24h_pct
1,1,51.700,0.002000,0.014000,51.700,51.700,0.200,1.400
2,1,56.100,0.006000,0.021000,56.100,56.100,0.600,2.100
...
10,1,92.500,0.045000,0.082000,92.500,92.500,4.500,8.200
```

#### `gate_winrate.csv` 
```csv
gate_config,timeframe,regime,pass_count,fail_count,pass_avg_return,fail_avg_return,outperformance_gap,pass_hit_rate,fail_hit_rate,hit_rate_lift
standard,4h,all,3,7,0.037333,0.016000,0.021333,1.0000,0.0000,1.0000
lenient,4h,normal,4,2,0.033000,0.019500,0.013500,0.7500,0.0000,0.7500
...
```

## Test Configuration

### Synthetic Test Data

The QA suite uses deterministic synthetic data (`testdata/tuner/synthetic_panel.json`) with:

- **10 assets** spanning score deciles 1-10  
- **Multiple regimes**: normal, volatile, calm
- **Gate compliance**: Assets 1-3 pass all entry gates
- **Return patterns**: Monotonic increase by score decile
- **Realistic ranges**: Scores 51.7-92.5, returns 0.2%-8.2%

### Test Execution

```bash
# Run conformance tests (MUST pass for deployment)
go test ./tests/conformance/regime/... -v

# Run empirical tests with CSV export
go test ./tests/empirical/... -v

# Combined regime QA suite
go test ./tests/conformance/regime/... ./tests/empirical/... -v
```

### CI Integration

The QA suite is integrated into CI pipeline with strict gates:

```yaml
- name: Regime QA Suite
  run: |
    go test ./tests/conformance/regime/... -count=1
    go test ./tests/empirical/... -count=1
    # CI fails if any conformance test fails
    # Empirical tests generate warnings but don't block
```

## Expected Results

### Conformance Test Expectations

All conformance tests MUST pass consistently:

- ✅ **Weight sums**: All regimes sum to 100% ±0.1%
- ✅ **Social cap**: Never exceeds +10 points 
- ✅ **Gram-Schmidt**: MomentumCore never residualized
- ✅ **Detector**: All boundary conditions correct

### Empirical Test Expectations

Empirical tests validate signal quality:

- ✅ **Monotonicity**: ≥8/10 deciles show increasing returns
- ✅ **Gate alignment**: Gate-passing entries outperform controls
- ✅ **Spreads**: Top decile outperforms bottom by ≥2% (4h), ≥3.5% (24h)
- ✅ **Regime consistency**: Gates work across all regimes

### Performance Benchmarks

QA suite performance requirements:

- **Execution time**: <5 seconds total
- **Memory usage**: <50MB peak
- **File I/O**: <10 file operations
- **Determinism**: 100% reproducible results

## Failure Analysis

### Common Failure Modes

#### Weight Sum Violations
```
FAIL: weights for regime normal sum to 1.050000, must equal 1.000 (±0.001)
```
**Root Cause**: Constraint system bounds mathematically impossible  
**Resolution**: Adjust regime constraint bounds in `internal/tune/weights/constraints.go`

#### Social Cap Breaches  
```
FAIL: social contribution 12.500 exceeds +10 cap
```
**Root Cause**: Social factor aggregation not properly capped  
**Resolution**: Fix cap enforcement in composite scoring system

#### Monotonicity Failures
```  
FAIL: 4h return monotonicity failed: 4 violations > 2 allowed
```
**Root Cause**: Insufficient signal quality or noise in scoring  
**Resolution**: Review factor weights or increase sample size

#### Gate Misalignment
```
FAIL: gate-passing entries underperformed by -1.200%
```
**Root Cause**: Entry gates not predictive of performance  
**Resolution**: Recalibrate gate thresholds or factor contributions

### Debugging Workflows

1. **Run individual test suites** to isolate failures
2. **Check CSV artifacts** for detailed analysis
3. **Validate test data** against live backtest results  
4. **Review constraint bounds** for mathematical consistency
5. **Analyze regime-specific performance** for detection issues

## Maintenance

### Updating Test Data

When updating synthetic panel data:

1. Maintain **monotonic score-return relationship**
2. Ensure **sufficient regime diversity** 
3. Verify **gate threshold coverage**
4. Update **expected result baselines**

### Adding New Tests

New QA tests should follow patterns:

1. **Conformance tests**: Test invariants that MUST never be violated
2. **Empirical tests**: Test signal quality with statistical validation
3. **CSV export**: Provide interpretable artifacts for external review
4. **CI integration**: Configure appropriate pass/fail thresholds

### Version Compatibility

The QA suite maintains backwards compatibility:

- **Test data format**: Stable JSON schema
- **CSV output format**: Additive column changes only
- **Acceptance criteria**: Versioned thresholds in test configuration
- **API contracts**: Stable interfaces for tuner components

---

**Last Updated**: 2025-01-06  
**Version**: 1.0  
**Maintainer**: CryptoRun QA Team