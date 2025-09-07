package guards

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/indicators"
)

// SafetyGuardResult represents the result of a safety guard check (renamed to avoid conflict)
type SafetyGuardResult struct {
	Passed      bool      `json:"passed"`
	GuardName   string    `json:"guard_name"`
	Reason      string    `json:"reason"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Timestamp   time.Time `json:"timestamp"`
	IsWarning   bool      `json:"is_warning"`   // If true, warning only (don't block)
	Confidence  float64   `json:"confidence"`   // 0.0-1.0 confidence in guard decision
}

// CandidateData represents the data needed for guard evaluation
type CandidateData struct {
	Symbol         string                         `json:"symbol"`
	CurrentPrice   float64                        `json:"current_price"`
	Momentum24h    float64                        `json:"momentum_24h"`
	Indicators     indicators.TechnicalIndicators `json:"indicators"`
	SignalTime     time.Time                      `json:"signal_time"`
	ExecutionTime  time.Time                      `json:"execution_time,omitempty"`
	LastATR        float64                        `json:"last_atr"`
	BarsAge        int                            `json:"bars_age"`        // Age in number of bars
	P99LatencyMs   int                            `json:"p99_latency_ms"`  // System P99 latency
	ATRProximity   float64                        `json:"atr_proximity"`   // Distance from signal in ATR units
}

// SafetyGuards implements all safety guard checks
type SafetyGuards struct {
	config application.GuardsConfig
}

// NewSafetyGuards creates a new safety guards instance
func NewSafetyGuards(config application.GuardsConfig) *SafetyGuards {
	return &SafetyGuards{
		config: config,
	}
}

// EvaluateAllGuards runs all safety guards for the given regime and candidate
func (sg *SafetyGuards) EvaluateAllGuards(regime string, candidate CandidateData) ([]SafetyGuardResult, error) {
	guardSettings, err := sg.config.GetActiveGuardSettings(regime)
	if err != nil {
		return nil, fmt.Errorf("failed to get guard settings for regime %s: %w", regime, err)
	}

	var results []SafetyGuardResult

	// 1. Fatigue Guard - prevents chasing overextended moves
	fatigueResult := sg.evaluateFatigueGuard(candidate, guardSettings.Fatigue)
	results = append(results, fatigueResult)

	// 2. Freshness Guard - ensures signal recency
	freshnessResult := sg.evaluateFreshnessGuard(candidate, guardSettings.Freshness)
	results = append(results, freshnessResult)

	// 3. Late Fill Guard - prevents late execution
	lateFillResult := sg.evaluateLateFillGuard(candidate, guardSettings.LateFill)
	results = append(results, lateFillResult)

	return results, nil
}

// evaluateFatigueGuard checks if the move is overextended
func (sg *SafetyGuards) evaluateFatigueGuard(candidate CandidateData, config application.FatigueGuardConfig) SafetyGuardResult {
	result := SafetyGuardResult{
		GuardName: "FatigueGuard",
		Timestamp: time.Now(),
		Threshold: config.Threshold24h,
		Value:     candidate.Momentum24h,
		Confidence: 0.8, // Base confidence
	}

	// Check 24h momentum threshold
	momentumExceeded := candidate.Momentum24h > config.Threshold24h
	
	// Check RSI 4h threshold
	rsiExceeded := candidate.Indicators.RSI.IsValid && 
				   candidate.Indicators.RSI.Value > float64(config.RSI4h)

	// Enhanced logic: Both conditions must be met to trigger fatigue guard
	if momentumExceeded && rsiExceeded {
		// Additional check: If momentum is accelerating, we might allow the trade
		// This would require additional data (momentum acceleration)
		// For now, we apply the guard if both conditions are met
		
		result.Passed = false
		result.Reason = fmt.Sprintf("Fatigued: 24h momentum %.2f%% > %.1f%% threshold and RSI %.1f > %d", 
			candidate.Momentum24h, config.Threshold24h, candidate.Indicators.RSI.Value, config.RSI4h)
		result.IsWarning = false
		result.Confidence = 0.9 // High confidence when both indicators align
	} else if momentumExceeded || rsiExceeded {
		// One condition met - issue warning but don't block
		result.Passed = true
		result.IsWarning = true
		if momentumExceeded {
			result.Reason = fmt.Sprintf("Warning: High 24h momentum %.2f%% (threshold %.1f%%)", 
				candidate.Momentum24h, config.Threshold24h)
		} else {
			result.Reason = fmt.Sprintf("Warning: High RSI %.1f (threshold %d)", 
				candidate.Indicators.RSI.Value, config.RSI4h)
		}
		result.Confidence = 0.6 // Lower confidence for single indicator
	} else {
		result.Passed = true
		result.Reason = "Not fatigued: momentum and RSI within acceptable ranges"
		result.Confidence = 0.8
	}

	return result
}

// evaluateFreshnessGuard ensures signal is recent and within acceptable price range
func (sg *SafetyGuards) evaluateFreshnessGuard(candidate CandidateData, config application.FreshnessGuardConfig) SafetyGuardResult {
	result := SafetyGuardResult{
		GuardName: "FreshnessGuard",
		Timestamp: time.Now(),
		Threshold: float64(config.MaxBarsAge),
		Value:     float64(candidate.BarsAge),
		Confidence: 0.9, // High confidence in time-based checks
	}

	// Check bars age
	barsOld := candidate.BarsAge > config.MaxBarsAge
	
	// Check ATR proximity (if we have ATR data)
	atrDistant := false
	if candidate.LastATR > 0 && candidate.CurrentPrice > 0 {
		// This would need the original signal price to calculate properly
		// For now, we assume ATRProximity is pre-calculated
		atrDistant = candidate.ATRProximity > config.ATRFactor
		result.Threshold = config.ATRFactor
		result.Value = candidate.ATRProximity
	}

	if barsOld {
		result.Passed = false
		result.Reason = fmt.Sprintf("Stale signal: %d bars old (max %d allowed)", 
			candidate.BarsAge, config.MaxBarsAge)
		result.IsWarning = false
	} else if atrDistant {
		result.Passed = false
		result.Reason = fmt.Sprintf("Price moved too far: %.2f ATRs from signal (max %.1f)", 
			candidate.ATRProximity, config.ATRFactor)
		result.IsWarning = false
	} else {
		result.Passed = true
		result.Reason = fmt.Sprintf("Fresh signal: %d bars age, %.2f ATR proximity", 
			candidate.BarsAge, candidate.ATRProximity)
	}

	return result
}

// evaluateLateFillGuard prevents execution delays that could impact performance
func (sg *SafetyGuards) evaluateLateFillGuard(candidate CandidateData, config application.LateFillGuardConfig) SafetyGuardResult {
	result := SafetyGuardResult{
		GuardName: "LateFillGuard",
		Timestamp: time.Now(),
		Confidence: 0.95, // Very high confidence in timing checks
	}

	var delaySeconds float64
	var hasExecutionTime bool

	// Calculate execution delay if we have execution time
	if !candidate.ExecutionTime.IsZero() {
		delay := candidate.ExecutionTime.Sub(candidate.SignalTime)
		delaySeconds = delay.Seconds()
		hasExecutionTime = true
		result.Value = delaySeconds
		result.Threshold = float64(config.MaxDelaySeconds)
	}

	// Check execution delay
	delayExceeded := hasExecutionTime && delaySeconds > float64(config.MaxDelaySeconds)
	
	// Check system latency (P99)
	latencyHigh := candidate.P99LatencyMs > config.P99LatencyReq
	
	// Check ATR proximity (price movement since signal)
	atrDistant := candidate.ATRProximity > config.ATRProximity

	if delayExceeded {
		result.Passed = false
		result.Reason = fmt.Sprintf("Late execution: %.1fs delay (max %ds)", 
			delaySeconds, config.MaxDelaySeconds)
		result.IsWarning = false
	} else if latencyHigh {
		result.Passed = false
		result.Reason = fmt.Sprintf("High system latency: %dms P99 (max %dms)", 
			candidate.P99LatencyMs, config.P99LatencyReq)
		result.IsWarning = false
	} else if atrDistant {
		result.Passed = false
		result.Reason = fmt.Sprintf("Price moved too far during execution: %.2f ATRs (max %.1f)", 
			candidate.ATRProximity, config.ATRProximity)
		result.IsWarning = false
	} else {
		result.Passed = true
		if hasExecutionTime {
			result.Reason = fmt.Sprintf("Timely execution: %.1fs delay, %dms latency, %.2f ATR movement", 
				delaySeconds, candidate.P99LatencyMs, candidate.ATRProximity)
		} else {
			result.Reason = fmt.Sprintf("Pre-execution check: %dms latency acceptable", 
				candidate.P99LatencyMs)
		}
	}

	return result
}

// GetGuardSummary returns a summary of all guard results
type GuardSummary struct {
	AllPassed     bool          `json:"all_passed"`
	BlockingIssues int          `json:"blocking_issues"`
	Warnings       int          `json:"warnings"`
	TotalGuards    int          `json:"total_guards"`
	Results        []SafetyGuardResult `json:"results"`
	Recommendation string       `json:"recommendation"`
	OverallScore   float64      `json:"overall_score"` // 0-100 score based on guard results
}

// GetGuardSummary analyzes guard results and provides trading recommendation
func GetGuardSummary(results []SafetyGuardResult) GuardSummary {
	summary := GuardSummary{
		Results:     results,
		TotalGuards: len(results),
		AllPassed:   true,
	}

	blockingIssues := 0
	warnings := 0
	scoreSum := 0.0
	
	for _, result := range results {
		if !result.Passed && !result.IsWarning {
			blockingIssues++
			summary.AllPassed = false
		} else if result.IsWarning {
			warnings++
		}
		
		// Calculate score contribution (100 for pass, 0 for blocking fail, 50 for warning)
		if result.Passed && !result.IsWarning {
			scoreSum += 100.0 * result.Confidence
		} else if result.IsWarning {
			scoreSum += 50.0 * result.Confidence
		} else {
			scoreSum += 0.0
		}
	}
	
	summary.BlockingIssues = blockingIssues
	summary.Warnings = warnings
	
	// Calculate overall score
	if len(results) > 0 {
		summary.OverallScore = scoreSum / float64(len(results))
	}
	
	// Generate recommendation
	if blockingIssues > 0 {
		summary.Recommendation = "REJECT: Blocking safety issues detected"
	} else if warnings > 0 {
		summary.Recommendation = "CAUTION: Proceed with warnings noted"
	} else {
		summary.Recommendation = "APPROVE: All safety guards passed"
	}
	
	return summary
}

// IsTradeAllowed returns true if all blocking guards pass (warnings are allowed)
func IsTradeAllowed(results []SafetyGuardResult) bool {
	for _, result := range results {
		if !result.Passed && !result.IsWarning {
			return false
		}
	}
	return true
}

// ValidateGuardConfig validates that guard configuration is reasonable
func ValidateGuardConfig(config application.GuardsConfig) error {
	for profileName, profile := range config.Profiles {
		for regimeName, regime := range profile.Regimes {
			// Validate fatigue guard
			if regime.Fatigue.Threshold24h <= 0 || regime.Fatigue.Threshold24h > 100 {
				return fmt.Errorf("invalid fatigue threshold for %s/%s: %.1f", profileName, regimeName, regime.Fatigue.Threshold24h)
			}
			if regime.Fatigue.RSI4h <= 0 || regime.Fatigue.RSI4h > 100 {
				return fmt.Errorf("invalid RSI threshold for %s/%s: %d", profileName, regimeName, regime.Fatigue.RSI4h)
			}
			
			// Validate freshness guard
			if regime.Freshness.MaxBarsAge <= 0 || regime.Freshness.MaxBarsAge > 100 {
				return fmt.Errorf("invalid max bars age for %s/%s: %d", profileName, regimeName, regime.Freshness.MaxBarsAge)
			}
			if regime.Freshness.ATRFactor <= 0 || regime.Freshness.ATRFactor > 10 {
				return fmt.Errorf("invalid ATR factor for %s/%s: %.1f", profileName, regimeName, regime.Freshness.ATRFactor)
			}
			
			// Validate late fill guard
			if regime.LateFill.MaxDelaySeconds <= 0 || regime.LateFill.MaxDelaySeconds > 3600 {
				return fmt.Errorf("invalid max delay for %s/%s: %d", profileName, regimeName, regime.LateFill.MaxDelaySeconds)
			}
			if regime.LateFill.P99LatencyReq <= 0 || regime.LateFill.P99LatencyReq > 10000 {
				return fmt.Errorf("invalid P99 latency requirement for %s/%s: %d", profileName, regimeName, regime.LateFill.P99LatencyReq)
			}
		}
	}
	
	return nil
}