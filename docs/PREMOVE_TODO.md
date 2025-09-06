# Premove Implementation TODO

## UX MUST — Live Progress & Explainability

This document tracks the implementation rooms created in the premove application layer. Each room references specific tests that need to be implemented to complete the milestone.

## Empty Rooms Created

### 📁 src/application/premove/portfolio.go
**Status:** Empty shell with TODOs  
**Test References:** `tests/unit/premove/portfolio_test.go`

**Acceptance Checklist:**
- [ ] TestPortfolioCorrelation_ValidInput
- [ ] TestPortfolioCorrelation_ExceedsLimit  
- [ ] TestSectorCaps_L1Enforcement
- [ ] TestSectorCaps_DeFiEnforcement
- [ ] TestSectorCaps_InfrastructureEnforcement
- [ ] TestSectorCaps_GamingEnforcement
- [ ] TestBetaBudget_WithinLimit
- [ ] TestBetaBudget_ExceedsLimit
- [ ] TestPositionLimits_SinglePosition
- [ ] TestPositionLimits_TotalExposure

### 📁 src/application/premove/alerts.go
**Status:** Empty shell with TODOs  
**Test References:** `tests/unit/premove/alerts_test.go`

**Acceptance Checklist:**
- [ ] TestAlertRateLimit_PerHour
- [ ] TestAlertRateLimit_PerDay
- [ ] TestAlertRateLimit_WithinLimits
- [ ] TestHighVolAlerts_SpecialRate
- [ ] TestHighVolAlerts_VolatilityDetection
- [ ] TestAlertThrottling_QueueBehavior
- [ ] TestAlertThrottling_DropBehavior
- [ ] TestAlertDelivery_Success
- [ ] TestAlertDelivery_Failure
- [ ] TestAlertDelivery_Retry

### 📁 src/application/premove/execution.go
**Status:** Empty shell with TODOs  
**Test References:** `tests/unit/premove/execution_test.go`

**Acceptance Checklist:**
- [ ] TestSlippageMonitoring_BelowThreshold
- [ ] TestSlippageMonitoring_AboveThreshold
- [ ] TestSlippageMonitoring_TightenTrigger
- [ ] TestExecutionQuality_MetricsCollection
- [ ] TestExecutionQuality_QualityScore
- [ ] TestVenueExecution_PerformanceTracking
- [ ] TestVenueExecution_ComparisonMetrics
- [ ] TestExecutionCosts_Tracking
- [ ] TestExecutionCosts_Analysis
- [ ] TestExecutionCosts_Optimization

### 📁 src/application/premove/backtest.go
**Status:** Empty shell with TODOs  
**Test References:** `tests/unit/premove/backtest_test.go`

**Acceptance Checklist:**
- [ ] TestPatternExhaustion_Detection
- [ ] TestPatternExhaustion_Threshold
- [ ] TestPatternExhaustion_Historical
- [ ] TestLearningAlgorithms_PatternUpdate
- [ ] TestLearningAlgorithms_WeightAdjustment
- [ ] TestHistoricalPerformance_BacktestRun
- [ ] TestHistoricalPerformance_MetricsCalculation
- [ ] TestStrategyEffectiveness_Measurement
- [ ] TestStrategyEffectiveness_Comparison
- [ ] TestStrategyEffectiveness_Adaptation

## Implementation Priority

1. **Portfolio Management** - Core risk controls and correlation limits
2. **Alert System** - Rate limiting and delivery mechanisms  
3. **Execution Quality** - Slippage monitoring and venue analysis
4. **Backtesting** - Pattern learning and strategy validation

## Configuration Integration

All implementations must integrate with `config/premove.yaml`:

- **Portfolio**: `pairwise_corr_max`, `sector_caps`, `beta_budget_to_btc`
- **Alerts**: `per_hour`, `per_day`, `high_vol_per_hour`  
- **Execution**: `slippage_bps_tighten_threshold`
- **Learning**: `pattern_exhaustion` settings

## Progress Tracking

These empty rooms correspond to milestones in `PROGRESS.yaml`:
- `portfolio_risk_controls` (20% progress)
- `premove_application_layer` (20% progress)

**Next Step:** Implement test suites, then fill the TODO blocks to complete the milestones.