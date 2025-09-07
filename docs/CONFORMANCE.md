# CryptoRun Conformance Suite

## UX MUST — Live Progress & Explainability

The Conformance Suite enforces critical system contracts through CI validation, ensuring no configuration drift or policy violations can enter the codebase. Any violation immediately fails CI with specific error attribution.

## Overview

CryptoRun's conformance testing validates 5 unbreakable contracts that define system integrity:

1. **Weights Validation** - All regime weights sum to 1.0, momentum dominates
2. **Momentum Protection** - MomentumCore never residualized in Gram-Schmidt 
3. **Guards Regime Enforcement** - Regime-aware behavior when enabled, legacy when disabled
4. **Microstructure Aggregator Ban** - Depth/spread data MUST be exchange-native only
5. **Diagnostics Spec Compliance** - Recommendations based on spec P&L, never raw 24h

## Weight System Conformance

### Weight Sum Validation
- **Requirement**: All timeframe weights must sum to exactly 1.0
- **Tolerance**: ±0.000001 (6 decimal places)
- **Scope**: `config/momentum.yaml`, `config/dip.yaml`
- **Test**: `TestMomentumWeightSumConformance`

### Weight Boundaries  
- **24h weight**: Must be within [0.10, 0.15] range
- **7d weight**: Must be within [0.05, 0.10] range
- **Other timeframes**: Must be positive values
- **Test**: `TestWeightBoundaryConformance`

## Factor System Conformance

### Momentum Protection
- **Requirement**: `MomentumCore` must be in `protected_factors` list
- **Purpose**: Prevents residualization in Gram-Schmidt orthogonalization
- **Scope**: `config/momentum.yaml`
- **Test**: `TestMomentumProtectionConformance`

### Social/Brand Factor Caps
- **Max contribution**: ≤ +10 points
- **Cap value**: ≤ +10 points  
- **Scope**: Social and Brand factor configurations
- **Test**: `TestSocialCapConformance`

### Orthogonalization Protection
- **Requirement**: Protected factors must not be residualized
- **Implementation**: Source code must check protection before residualization
- **Files**: `internal/domain/scoring/orthogonal.go`, `src/application/pipeline/orthogonalization.go`
- **Test**: `TestOrthogonalizationConformance`

## Guards Regime Behavior

### Fatigue Guard Requirements
- **Baseline (Chop/High-Vol)**: 12% momentum, 70 RSI threshold
- **Trending**: 18% momentum ONLY when `accel_renewal=true`, 70 RSI  
- **Safety limits**: Max 25% momentum, max 80 RSI
- **Test**: `TestGuardRegimeBehaviorConformance`

### Late-Fill Guard Requirements
- **Baseline**: 30s max execution delay
- **Trending**: 45s ONLY when `infra_health=true` AND `atr_proximity ≤ 1.2×ATR`
- **Safety limits**: Max 60s absolute
- **Test**: `TestGuardRegimeBehaviorConformance`

### Freshness Guard Requirements  
- **Baseline**: 2 bars max age, 1.2×ATR price movement limit
- **Trending**: 3 bars ONLY when `VADR ≥ 1.75×` AND `spread < 50bps`
- **Safety limits**: Max 5 bars absolute, min 0.8×ATR factor
- **Test**: `TestGuardRegimeBehaviorConformance`

## Microstructure Conformance

### Exchange-Native Only
- **Requirement**: `exchange_native_only: true` in microstructure configs
- **Allowed exchanges**: binance, kraken, coinbase, okx
- **Test**: `TestAggregatorBanConformance`

### Aggregator Ban Enforcement
- **Banned aggregators**: dexscreener, coingecko, coinmarketcap
- **Must be listed**: In `banned_aggregators` configuration
- **Source code check**: No banned patterns in microstructure context
- **Test**: `TestAggregatorBanConformance`, `TestSourceCodeAggregatorBanConformance`

### Gate Requirements
- **Spread**: < 50 basis points maximum
- **Depth**: ≥ $100k within ±2% tolerance  
- **VADR**: ≥ 1.75× minimum multiplier
- **Test**: `TestMicrostructureGateConformance`

## Benchmark Diagnostic Conformance

### Sample Size Requirements
- **Minimum sample size**: n ≥ 20 for recommendations
- **Insufficient samples**: Must disable recommendations when n < 20
- **Windows tracking**: Insufficient windows must be listed explicitly
- **Test**: `TestBenchmarkSampleSizeConformance`

### Spec-Compliant P&L Usage
- **Primary metric**: Must use spec-compliant P&L when available
- **Relationship**: Spec-compliant ≤ raw due to entry/exit timing
- **Config recommendations**: Must not mention raw gains without spec alternative
- **Insights basis**: Must indicate spec-compliant vs raw-gain basis
- **Test**: `TestBenchmarkSpecCompliantPnLConformance`

### Methodology Validation
- **Required terms**: Must mention "spec-compliant", "entry", "exit", "simulation" (≥2 terms)
- **Series attribution**: Must label exchange-native vs aggregator_fallback sources
- **Exchange sources**: binance, kraken, coinbase, exchange_native labels required
- **Test**: `TestBenchmarkMethodologyConformance`

## Running Conformance Tests

### Local Execution
```bash
# Run all conformance tests
go test -v ./tests/conformance -timeout 30s

# Run specific contract tests
go test -v ./tests/conformance -run TestNoDuplicateScoringPaths
```

### Post-Merge Verification Integration
The preferred way to run conformance is via the integrated verification command:

```bash
# Complete post-merge verification (conformance + alignment + diagnostics)
cryptorun verify postmerge --windows 1h,24h --n 20 --progress

# Results include conformance contract status:
# 📊 CONFORMANCE CONTRACTS
# ┌─────────────────────────────┬────────┐
# │ Contract                    │ Status │
# ├─────────────────────────────┼────────┤
# │ Single Scoring Path         │ ✅ PASS │
# │ Weight Normalization        │ ✅ PASS │
# │ Social Hard Cap             │ ✅ PASS │
# │ Menu-CLI Alignment          │ ✅ PASS │
# └─────────────────────────────┴────────┘
```

### CI Integration
The conformance suite runs automatically in CI before regular tests:
```yaml
- name: Conformance Suite
  run: |
    echo "🛡️ Running Conformance Suite..."
    go test -v ./tests/conformance -timeout 30s
    echo "✅ All conformance requirements verified"

- name: Post-Merge Verification
  run: |
    echo "🔍 Running post-merge verification..."
    go run ./src/cmd/cryptorun verify postmerge --windows 1h,24h --n 20
```

### Test Structure
- **Package**: `conformance_test`
- **Location**: `tests/conformance/`
- **Files**: 5 test files covering all conformance areas
- **Naming**: `Test*Conformance` pattern for easy identification

## Violation Response

### Error Format
All conformance violations use the prefix:
```
CONFORMANCE VIOLATION: [specific description]
```

### CI Failure Policy
- **Any violation**: Fails the CI build
- **No bypassing**: Conformance tests cannot be skipped
- **Fix required**: Must resolve violations before merge

## UX MUST — Live Progress & Explainability

The conformance suite provides immediate feedback on system invariant drift, ensuring CryptoRun maintains its safety guarantees and PRD compliance throughout development.