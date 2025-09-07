# Isotonic Score Calibration

## UX MUST — Live Progress & Explainability

Real-time isotonic score calibration: monotone score-to-probability mapping using Pool-Adjacent-Violators algorithm, regime-aware calibration curves, live position tracking for outcome data collection, and comprehensive validation gates for reliable probability prediction in cryptocurrency momentum trading.

**Last Updated:** 2025-09-07  
**Version:** v3.3.2 Calibration Integration  
**Breaking Changes:** New calibration system integrated into composite scoring pipeline

The isotonic calibration system transforms composite scores (0-114) into calibrated success probabilities (0-1) through real trading outcome analysis, providing statistically sound probability estimates for entry decisions.

## Mathematical Foundation

### Isotonic Regression Algorithm

**Pool-Adjacent-Violators (PAV) Algorithm:**
```
Input: Sorted pairs (score₁, outcome₁), (score₂, outcome₂), ..., (scoreₙ, outcomeₙ)
Output: Monotonic calibration curve

1. Group samples into bins by score ranges
2. Calculate initial probabilities: pᵢ = positives_i / total_i  
3. For each violation where pᵢ > pⱼ and scoreᵢ < scoreⱼ:
   - Pool adjacent bins: p_pooled = Σ(wᵢ × pᵢ) / Σ(wᵢ)
   - Replace both bins with pooled values
4. Repeat until no violations remain
```

**Monotonicity Guarantee:**
```
∀ score₁ < score₂: P(success|score₁) ≤ P(success|score₂)
```

**Wilson Score Confidence Intervals:**
```
CI_half_width = z × √[(p(1-p) + z²/(4n)) / n] / (1 + z²/n)

Where:
- z = 1.96 (95% confidence)
- p = observed probability
- n = sample count
```

### Score Binning Strategy

**Adaptive Binning:**
```
optimal_bins = max(5, min(50, log₂(sample_count) + 1))
constrained_by_samples = min(optimal_bins, sample_count / 10)

Final bins = constrained_by_samples ensuring ≥10 samples per bin average
```

**Bin Statistics:**
```go
type CalibrationBin struct {
    MeanScore   float64 // Average score in this bin
    Probability float64 // Observed probability of positive outcome  
    Count       int     // Number of samples in bin
    Confidence  float64 // Wilson score confidence interval width (±)
}
```

## Regime-Aware Calibration

### Multi-Regime Architecture

**Calibration Harness Design:**
```go
type CalibrationHarness struct {
    calibrators   map[string]*IsotonicCalibrator  // "bull", "bear", "choppy", "general"
    sampleBuffer  []CalibrationSample             // Training data buffer
    config        CalibrationConfig               // System configuration
    mutex         sync.RWMutex                    // Thread safety
}
```

**Regime-Specific Behavior:**
- **Bull Markets**: Higher scores more predictive of momentum continuation
- **Bear Markets**: Calibration accounts for trend reversal patterns  
- **Choppy Markets**: Higher uncertainty, wider confidence bounds
- **General Fallback**: Used when regime-specific data insufficient

### Prediction Fallback Hierarchy

**Priority Order:**
1. **Regime-Specific**: Use calibrator for current regime (e.g., "bull")
2. **General Calibrator**: Fall back to "general" if regime calibrator unavailable
3. **Uncalibrated Mapping**: Simple score/110 transformation as last resort

```go
func (ch *CalibrationHarness) PredictProbability(score float64, regime string) (float64, error) {
    // 1. Try regime-specific calibrator
    if calibrator, exists := ch.calibrators[regime]; exists && calibrator.IsValid() {
        return calibrator.Predict(score), nil
    }
    
    // 2. Fall back to general calibrator
    if general, exists := ch.calibrators["general"]; exists && general.IsValid() {
        return general.Predict(score), nil
    }
    
    // 3. Uncalibrated probability as last resort
    return ch.uncalibratedProbability(score), nil
}
```

## Live Data Collection System

### Position Tracking Lifecycle

**CalibrationCollector Architecture:**
```go
type CalibrationCollector struct {
    harness           *CalibrationHarness
    activePositions   map[string]*TrackedPosition  // Live position tracking
    targetHoldingPeriod time.Duration              // 48 hours default
    moveThreshold     float64                      // 5% success threshold
    maxTrackingTime   time.Duration                // 72 hours timeout
}
```

**Position Lifecycle:**
```
1. Entry: TrackNewPosition(symbol, score, compositeResult, entryPrice, regime)
2. Updates: UpdatePosition(symbol, currentPrice) → calculate moves
3. Outcomes: 
   - Success: |move| ≥ threshold within 48h
   - Timeout: 72h maximum tracking period
   - Manual: ForceClosePosition() for immediate closure
4. Sample: Create CalibrationSample with outcome data
```

### Success Criteria Configuration

**Configurable Thresholds:**
```yaml
calibration:
  move_threshold: 0.05          # 5% price movement for success
  target_holding_period: 48h    # Target evaluation window
  max_tracking_time: 72h        # Maximum tracking before timeout
```

**Success Determination:**
```go
func determineSuccess(position *TrackedPosition, collector *CalibrationCollector) bool {
    actualMove := math.Abs(position.CurrentMove)
    withinTargetPeriod := time.Since(position.EntryTime) <= collector.targetHoldingPeriod
    
    return actualMove >= collector.moveThreshold && withinTargetPeriod
}
```

### Sample Data Structure

**Comprehensive Sample Storage:**
```go
type CalibrationSample struct {
    Score         float64       `json:"score"`          // Composite score (0-114)
    Outcome       bool          `json:"outcome"`        // Success/failure determination
    Timestamp     time.Time     `json:"timestamp"`      // Position entry time
    Symbol        string        `json:"symbol"`         // Asset symbol (e.g., "BTCUSD")
    Regime        string        `json:"regime"`         // Market regime at entry
    
    // Performance metrics
    HoldingPeriod time.Duration `json:"holding_period"` // Actual holding time
    MaxMove       float64       `json:"max_move"`       // Maximum observed move
    FinalMove     float64       `json:"final_move"`     // Move at position close
}
```

## Quality Validation System

### Performance Metrics

**Calibration Quality Assessment:**
```
Reliability (Calibration Error):
    reliability = Σ|observed_frequency - predicted_probability|² / N
    Target: < 0.1 (10% maximum calibration error)

Resolution (Discrimination Ability):
    resolution = Variance(predicted_probabilities) / max_possible_variance
    Target: > 0.0 (any discrimination better than none)

Sharpness (Probability Spread):
    sharpness = max(probabilities) - min(probabilities)
    Target: > 0.0 (meaningful probability range)
```

**AUC Calculation (Area Under ROC Curve):**
```go
func calculateAUC(calibrator *IsotonicCalibrator, samples []CalibrationSample) float64 {
    // Sort by predicted probability (descending)
    // Calculate TPR and FPR at each threshold
    // Integrate using trapezoidal rule
    // Return AUC ∈ [0, 1] where 0.5 = random, 1.0 = perfect
}
```

### Validation Gates

**Acceptance Criteria:**
```yaml
validation_gates:
  min_calibration_error: 0.10    # Maximum 10% calibration error
  min_auc_threshold: 0.55        # Must beat random (0.5) by 5%
  min_sample_count: 100          # Minimum samples for statistical validity
  max_age_days: 90               # Maximum calibrator age before forced refresh
```

**Validation Process:**
1. **Data Split**: 80% training / 20% validation using temporal split
2. **Fit Calibrator**: Train isotonic regression on training set
3. **Validate Performance**: Evaluate on held-out validation set
4. **Gate Checks**: Verify all quality gates passed
5. **Accept/Reject**: Deploy new calibrator or keep existing

## Refresh and Governance

### Scheduled Refresh System

**Refresh Configuration:**
```yaml
calibration_refresh:
  refresh_interval: 720h        # 30 days between automatic refreshes
  min_samples_required: 100     # Minimum data before refresh allowed
  validation_split: 0.2         # 20% holdout for validation
  governance_freeze: false      # Manual override for production stability
```

**Refresh Triggers:**
```go
func (ch *CalibrationHarness) needsRefresh() bool {
    // Check sample sufficiency
    if len(ch.sampleBuffer) < ch.config.MinSamples {
        return false
    }
    
    // Check time-based refresh
    if time.Since(ch.lastRefresh) > ch.config.RefreshInterval {
        return true
    }
    
    // Check calibrator validity/age
    for _, calibrator := range ch.calibrators {
        if !calibrator.IsValid() || calibrator.NeedsRefresh(ch.config) {
            return true
        }
    }
    
    return false
}
```

### Data Management

**Buffer Management:**
```
Max Buffer Size: min_samples × 10 = 1000 samples (default)
Cleanup Policy: Remove samples > 90 days old
Memory Protection: Trim oldest samples when buffer exceeds maximum
```

**Sample Distribution:**
```
Target: ≥50 samples per regime for regime-aware calibration
Fallback: Combine into "general" calibrator if insufficient regime data
Quality: Prefer recent samples, maintain temporal diversity
```

## Performance Characteristics

### Computational Complexity

**Algorithm Performance:**
```
Isotonic Regression (PAV): O(n log n) - dominated by sorting
Binary Search Prediction: O(log k) - k calibration points
Calibration Refresh: O(3n log n) - for 3 regimes
Memory Usage: ~1KB per 100 stored samples
```

**Benchmark Results:**
```
Operation                     Latency    Throughput
Fit 200 samples              ~50ms      20 fits/sec
Single prediction            ~8μs       125k predictions/sec
Harness refresh (3 regimes)  ~200ms     5 refreshes/sec
Position tracking update     ~1μs       1M updates/sec
```

### Scalability Limits

**System Boundaries:**
```yaml
performance_limits:
  max_samples_per_calibrator: 10000    # Memory and fitting time constraints
  max_regimes: 10                      # Calibrator management overhead  
  max_active_positions: 5000           # Position tracking memory
  memory_footprint_mb: 5               # Total system memory usage
```

**Scale Testing:**
- **Load Test**: 1000 concurrent positions, 10k samples, 5 regimes
- **Memory Test**: <5MB total footprint under full load
- **Latency Test**: <100μs P99 for prediction requests

## Integration with Composite Scoring

### Enhanced Score Processing

**Calibration Integration Pipeline:**
```go
// Enhanced scoring with calibration
result, err := unifiedScorer.Score(ctx, input)
if err != nil {
    return nil, err
}

// Apply calibration to get probability
probability, err := calibrationHarness.PredictProbability(
    result.EnhancedScore, 
    currentRegime,
)
if err != nil {
    // Fallback to uncalibrated probability
    probability = math.Max(0, math.Min(1, result.EnhancedScore/100.0))
}

return &CalibratedResult{
    EnhancedScore: result.EnhancedScore,
    Probability:   probability,
    CalibrationInfo: getCalibrationInfo(currentRegime),
}, nil
```

### Entry Gate Enhancement

**Calibrated Entry Decisions:**
```go
// Traditional score-based gates
if enhancedScore < 75.0 {
    return EntryDecision{Allow: false, Reason: "score_too_low"}
}

// Enhanced probability-based gates  
if calibratedProb < 0.65 {
    return EntryDecision{Allow: false, Reason: "probability_too_low"}
}

// Combined confidence gates
confidence := "high"
if calibratedProb < 0.75 || enhancedScore < 85.0 {
    confidence = "medium" 
}
if calibratedProb < 0.65 || enhancedScore < 75.0 {
    confidence = "low"
}

return EntryDecision{
    Allow: true, 
    Confidence: confidence,
    Probability: calibratedProb,
    Score: enhancedScore,
}
```

## Output Format and Explainability

### Enhanced Explanation Structure

**Calibrated Scoring Output:**
```json
{
  "enhanced_score": 86.8,
  "calibrated_probability": 0.73,
  "calibration_info": {
    "regime": "bull",
    "calibrator_type": "regime_specific", 
    "calibrator_age_days": 15,
    "sample_count": 247,
    "last_refresh": "2025-08-23T14:30:00Z",
    "reliability": 0.08,
    "resolution": 0.42,
    "sharpness": 0.67,
    "auc": 0.71,
    "calibration_quality": "good"
  },
  "fallback_used": false,
  "prediction_method": "isotonic_regression"
}
```

**Quality Assessment Labels:**
```
calibration_quality calculation:
- "excellent": reliability < 0.05, AUC > 0.75, samples > 500
- "good": reliability < 0.10, AUC > 0.65, samples > 200  
- "fair": reliability < 0.15, AUC > 0.55, samples > 100
- "poor": reliability ≥ 0.15 OR AUC ≤ 0.55 OR samples < 100
```

### Confidence Intervals

**Prediction Uncertainty:**
```json
{
  "calibrated_probability": 0.73,
  "confidence_interval": {
    "lower_bound": 0.68,
    "upper_bound": 0.78, 
    "confidence_level": 0.95,
    "interval_width": 0.10
  },
  "prediction_stability": "stable"  // "stable", "uncertain", "volatile"
}
```

## Error Handling and Resilience

### Graceful Degradation

**Failure Modes and Recovery:**
```go
// Insufficient calibration data
if len(samples) < config.MinSamples {
    return fmt.Errorf("insufficient samples for calibration: need %d, have %d", 
                      config.MinSamples, len(samples))
}

// Validation failure recovery
if validationError := validateCalibrator(calibrator, validationSamples); validationError != nil {
    log.Warn("Calibrator validation failed", "error", validationError)
    // Keep existing calibrators, don't deploy new one
    return validationError
}

// Prediction fallback hierarchy
func (ch *CalibrationHarness) PredictProbability(score float64, regime string) (float64, error) {
    // Try regime-specific → general → uncalibrated
    // Always returns valid probability ∈ [0, 1]
}
```

### Monitoring and Alerting

**Health Check Metrics:**
```go
type CalibrationHealth struct {
    ActiveCalibrators   int     `json:"active_calibrators"`
    OldestCalibratorAge string  `json:"oldest_calibrator_age"`
    TotalSamples        int     `json:"total_samples"`
    BufferUtilization   float64 `json:"buffer_utilization"`
    LastRefreshStatus   string  `json:"last_refresh_status"`
    RefreshSuccess      bool    `json:"refresh_success"`
    QualityGatesPassed  bool    `json:"quality_gates_passed"`
}
```

**Alert Conditions:**
- Calibrator age > 45 days (stale calibration)
- Validation failure rate > 20% (quality degradation)
- Buffer overflow events > 1/hour (memory pressure)
- Prediction fallback rate > 10% (missing calibrators)

This comprehensive calibration system ensures statistically sound probability predictions while maintaining high performance, reliability, and explainability for cryptocurrency momentum trading decisions.