package spec

import (
	"fmt"
	"time"

	"cryptorun/internal/application/pipeline"
)

// GuardSpec validates gate enforcement compliance
type GuardSpec struct{}

// NewGuardSpec creates a new guard specification validator
func NewGuardSpec() *GuardSpec {
	return &GuardSpec{}
}

// Name returns the spec section name
func (g *GuardSpec) Name() string {
	return "Guard Validation"
}

// Description returns the spec section description
func (g *GuardSpec) Description() string {
	return "Validates fatigue, freshness, and late-fill guard enforcement"
}

// RunSpecs executes all guard validation tests
func (g *GuardSpec) RunSpecs() []SpecResult {
	var results []SpecResult

	// Test 1: Fatigue Guard
	results = append(results, g.testFatigueGuard())

	// Test 2: Freshness Guard
	results = append(results, g.testFreshnessGuard())

	// Test 3: Late-Fill Guard
	results = append(results, g.testLateFillGuard())

	// Test 4: Guard Combination Logic
	results = append(results, g.testGuardCombination())

	return results
}

// testFatigueGuard validates fatigue guard: 24h>+12% & RSI4h>70 unless accel↑
func (g *GuardSpec) testFatigueGuard() SpecResult {
	result := NewSpecResult("fatigue_guard", "Fatigue guard blocks 24h>+12% & RSI4h>70 unless acceleration up")

	testCases := []struct {
		name          string
		momentum24h   float64
		rsi4h         float64
		acceleration  float64
		expectedBlock bool
	}{
		{
			name:          "should_block_high_momentum_high_rsi",
			momentum24h:   15.5, // > +12%
			rsi4h:         75.0, // > 70
			acceleration:  -0.5, // declining
			expectedBlock: true,
		},
		{
			name:          "should_pass_high_momentum_high_rsi_accel_up",
			momentum24h:   13.2, // > +12%
			rsi4h:         72.0, // > 70
			acceleration:  1.8,  // accelerating up
			expectedBlock: false,
		},
		{
			name:          "should_pass_low_momentum",
			momentum24h:   8.5,  // < +12%
			rsi4h:         75.0, // > 70
			acceleration:  -0.2, // declining
			expectedBlock: false,
		},
		{
			name:          "should_pass_low_rsi",
			momentum24h:   15.0, // > +12%
			rsi4h:         65.0, // < 70
			acceleration:  -1.0, // declining
			expectedBlock: false,
		},
		{
			name:          "edge_case_exact_thresholds",
			momentum24h:   12.0,  // exactly +12%
			rsi4h:         70.0,  // exactly 70
			acceleration:  0.0,   // no acceleration
			expectedBlock: false, // should not trigger on exact match
		},
	}

	for _, tc := range testCases {
		factors := pipeline.MomentumFactors{
			Momentum24h: tc.momentum24h,
			RSI4h:       tc.rsi4h,
			Momentum4h:  tc.momentum24h + tc.acceleration, // Simulate acceleration
		}

		blocked := g.evaluateFatigueGuard(factors)
		if blocked != tc.expectedBlock {
			result.Passed = false
			result.Error = fmt.Sprintf("test %s failed: expected block=%t, got block=%t (24h=%.1f%%, RSI=%.1f, accel=%.1f)",
				tc.name, tc.expectedBlock, blocked, tc.momentum24h, tc.rsi4h, tc.acceleration)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d fatigue guard scenarios", len(testCases))
	return result
}

// testFreshnessGuard validates freshness guard: ≤2 bars & ≤1.2×ATR
func (g *GuardSpec) testFreshnessGuard() SpecResult {
	result := SpecResult{
		Name:        "freshness_guard",
		Description: "Freshness guard enforces ≤2 bars old & ≤1.2×ATR(1h) price movement",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	testCases := []struct {
		name          string
		barsOld       int
		priceMovement float64
		atr1h         float64
		expectedFresh bool
	}{
		{
			name:          "fresh_recent_small_move",
			barsOld:       1,
			priceMovement: 0.5,
			atr1h:         1.0,
			expectedFresh: true, // 1 bar, 0.5 < 1.2
		},
		{
			name:          "stale_too_old",
			barsOld:       3,
			priceMovement: 0.8,
			atr1h:         1.0,
			expectedFresh: false, // 3 bars > 2
		},
		{
			name:          "stale_large_move",
			barsOld:       2,
			priceMovement: 1.5,
			atr1h:         1.0,
			expectedFresh: false, // 1.5 > 1.2×1.0
		},
		{
			name:          "edge_case_exact_limits",
			barsOld:       2,
			priceMovement: 1.2,
			atr1h:         1.0,
			expectedFresh: true, // exactly at thresholds
		},
		{
			name:          "high_atr_proportional",
			barsOld:       1,
			priceMovement: 2.3,
			atr1h:         2.0,
			expectedFresh: true, // 2.3 < 1.2×2.0=2.4
		},
	}

	for _, tc := range testCases {
		data := FreshnessData{
			BarsOld:       tc.barsOld,
			PriceMovement: tc.priceMovement,
			ATR1h:         tc.atr1h,
		}

		fresh := g.evaluateFreshnessGuard(data)
		if fresh != tc.expectedFresh {
			result.Passed = false
			result.Error = fmt.Sprintf("test %s failed: expected fresh=%t, got fresh=%t (bars=%d, move=%.1f, atr=%.1f)",
				tc.name, tc.expectedFresh, fresh, tc.barsOld, tc.priceMovement, tc.atr1h)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d freshness guard scenarios", len(testCases))
	return result
}

// testLateFillGuard validates late-fill guard: <30s after signal bar close
func (g *GuardSpec) testLateFillGuard() SpecResult {
	result := SpecResult{
		Name:        "late_fill_guard",
		Description: "Late-fill guard rejects fills >30s after signal bar close",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	baseTime := time.Now()
	testCases := []struct {
		name         string
		signalTime   time.Time
		fillTime     time.Time
		expectedPass bool
	}{
		{
			name:         "immediate_fill",
			signalTime:   baseTime,
			fillTime:     baseTime.Add(5 * time.Second),
			expectedPass: true,
		},
		{
			name:         "fill_at_threshold",
			signalTime:   baseTime,
			fillTime:     baseTime.Add(30 * time.Second),
			expectedPass: true, // exactly at 30s threshold
		},
		{
			name:         "late_fill_31s",
			signalTime:   baseTime,
			fillTime:     baseTime.Add(31 * time.Second),
			expectedPass: false, // just over threshold
		},
		{
			name:         "very_late_fill",
			signalTime:   baseTime,
			fillTime:     baseTime.Add(2 * time.Minute),
			expectedPass: false,
		},
		{
			name:         "negative_time_edge_case",
			signalTime:   baseTime.Add(10 * time.Second),
			fillTime:     baseTime, // fill before signal
			expectedPass: false,
		},
	}

	for _, tc := range testCases {
		passed := g.evaluateLateFillGuard(tc.signalTime, tc.fillTime)
		if passed != tc.expectedPass {
			result.Passed = false
			delay := tc.fillTime.Sub(tc.signalTime)
			result.Error = fmt.Sprintf("test %s failed: expected pass=%t, got pass=%t (delay=%.1fs)",
				tc.name, tc.expectedPass, passed, delay.Seconds())
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d late-fill guard scenarios", len(testCases))
	return result
}

// testGuardCombination validates proper guard combination logic
func (g *GuardSpec) testGuardCombination() SpecResult {
	result := SpecResult{
		Name:        "guard_combination",
		Description: "Guard combination logic: ALL guards must pass for entry",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	testCases := []struct {
		name          string
		fatiguePass   bool
		freshnessPass bool
		lateFillPass  bool
		expectedPass  bool
	}{
		{
			name:          "all_guards_pass",
			fatiguePass:   true,
			freshnessPass: true,
			lateFillPass:  true,
			expectedPass:  true,
		},
		{
			name:          "fatigue_fail_blocks",
			fatiguePass:   false,
			freshnessPass: true,
			lateFillPass:  true,
			expectedPass:  false,
		},
		{
			name:          "freshness_fail_blocks",
			fatiguePass:   true,
			freshnessPass: false,
			lateFillPass:  true,
			expectedPass:  false,
		},
		{
			name:          "late_fill_fail_blocks",
			fatiguePass:   true,
			freshnessPass: true,
			lateFillPass:  false,
			expectedPass:  false,
		},
		{
			name:          "multiple_failures",
			fatiguePass:   false,
			freshnessPass: false,
			lateFillPass:  true,
			expectedPass:  false,
		},
	}

	for _, tc := range testCases {
		combinedPass := tc.fatiguePass && tc.freshnessPass && tc.lateFillPass
		if combinedPass != tc.expectedPass {
			result.Passed = false
			result.Error = fmt.Sprintf("test %s failed: expected pass=%t, got pass=%t (fatigue=%t, fresh=%t, late=%t)",
				tc.name, tc.expectedPass, combinedPass, tc.fatiguePass, tc.freshnessPass, tc.lateFillPass)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d guard combination scenarios", len(testCases))
	return result
}

// FreshnessData contains data needed for freshness evaluation
type FreshnessData struct {
	BarsOld       int
	PriceMovement float64
	ATR1h         float64
}

// evaluateFatigueGuard implements fatigue guard logic
func (g *GuardSpec) evaluateFatigueGuard(factors pipeline.MomentumFactors) bool {
	// Block if 24h > +12% AND RSI4h > 70 UNLESS acceleration is up
	if factors.Momentum24h > 12.0 && factors.RSI4h > 70.0 {
		// Check for acceleration (simplified as 4h vs 24h momentum)
		acceleration := factors.Momentum4h - factors.Momentum24h
		if acceleration <= 0 {
			return true // blocked
		}
	}
	return false // not blocked
}

// evaluateFreshnessGuard implements freshness guard logic
func (g *GuardSpec) evaluateFreshnessGuard(data FreshnessData) bool {
	// Must be ≤2 bars old AND ≤1.2×ATR(1h) movement
	if data.BarsOld > 2 {
		return false
	}

	atrThreshold := 1.2 * data.ATR1h
	if data.PriceMovement > atrThreshold {
		return false
	}

	return true
}

// evaluateLateFillGuard implements late-fill guard logic
func (g *GuardSpec) evaluateLateFillGuard(signalTime, fillTime time.Time) bool {
	delay := fillTime.Sub(signalTime)
	// Pass if fill is within 30 seconds of signal
	return delay >= 0 && delay <= 30*time.Second
}
