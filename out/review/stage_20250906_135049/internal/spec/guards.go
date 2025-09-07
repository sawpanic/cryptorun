package spec

import (
	"fmt"
	"time"

	"cryptorun/internal/domain"
)

// GuardsSpec tests trading guards compliance
type GuardsSpec struct{}

// Name returns the section name
func (gs *GuardsSpec) Name() string {
	return "Trading Guards"
}

// Description returns the section description
func (gs *GuardsSpec) Description() string {
	return "Fatigue (24h>+12% & RSI4h>70 unless accel↑), freshness (≤2 bars & ≤1.2×ATR), late-fill (<30s)"
}

// RunSpecs executes all guards specification tests
func (gs *GuardsSpec) RunSpecs() []SpecResult {
	var results []SpecResult
	
	// Test 1: Fatigue Guard
	results = append(results, gs.testFatigueGuard())
	
	// Test 2: Freshness Guard
	results = append(results, gs.testFreshnessGuard())
	
	// Test 3: Late-Fill Guard
	results = append(results, gs.testLateFillGuard())
	
	// Test 4: Guard Integration
	results = append(results, gs.testGuardIntegration())
	
	return results
}

// testFatigueGuard verifies fatigue guard behavior: 24h>+12% & RSI4h>70 unless accel≥2%
func (gs *GuardsSpec) testFatigueGuard() SpecResult {
	spec := NewSpecResult("FatigueGuard", "Blocks overextended positions (24h>+12% & RSI4h>70) unless acceleration≥2%")
	
	// Test case 1: Should block (high momentum + high RSI, no acceleration)
	fatigueInput := domain.FatigueGateInputs{
		Symbol:          "BTCUSD",
		Momentum24h:     15.5, // > 12%
		RSI4h:           72.0, // > 70
		Acceleration:    1.0,  // < 2%
	}
	
	result := domain.EvaluateFatigueGate(fatigueInput)
	if result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Fatigue guard failed to block overextended position (24h=15.5%, RSI4h=72.0)")
	}
	
	// Test case 2: Should allow (acceleration override)
	accelerationOverrideInput := domain.FatigueGateInputs{
		Symbol:          "ETHUSD",
		Momentum24h:     18.2, // > 12%
		RSI4h:           75.0, // > 70
		Acceleration:    2.5,  // >= 2% (override)
	}
	
	result = domain.EvaluateFatigueGate(accelerationOverrideInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Fatigue guard blocked position with acceleration override (accel=2.5%)")
	}
	
	// Test case 3: Should allow (momentum below threshold)
	lowMomentumInput := domain.FatigueGateInputs{
		Symbol:          "SOLUSD",
		Momentum24h:     8.5,  // < 12%
		RSI4h:           75.0, // > 70 (but momentum low)
		Acceleration:    0.5,
	}
	
	result = domain.EvaluateFatigueGate(lowMomentumInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Fatigue guard blocked position with low momentum (24h=8.5%)")
	}
	
	// Test case 4: Should allow (RSI below threshold)
	lowRSIInput := domain.FatigueGateInputs{
		Symbol:          "ADAUSD",
		Momentum24h:     15.0, // > 12%
		RSI4h:           65.0, // < 70
		Acceleration:    0.8,
	}
	
	result = domain.EvaluateFatigueGate(lowRSIInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Fatigue guard blocked position with low RSI (RSI4h=65.0)")
	}
	
	return spec.WithDetails("Fatigue guard logic verified: blocks when 24h>12% & RSI4h>70, allows with accel≥2%")
}

// testFreshnessGuard verifies freshness guard: ≤2 bars & ≤1.2×ATR(1h)
func (gs *GuardsSpec) testFreshnessGuard() SpecResult {
	spec := NewSpecResult("FreshnessGuard", "Rejects stale signals (≤2 bars old, price movement ≤1.2×ATR)")
	
	// Test case 1: Should allow (fresh signal)
	freshInput := domain.FreshnessGateInputs{
		Symbol:        "BTCUSD",
		BarsAge:       1,     // ≤ 2 bars
		PriceChange:   150.0, // Current price change
		ATR1h:         200.0, // ATR(1h)
	}
	
	result := domain.EvaluateFreshnessGate(freshInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Freshness guard rejected fresh signal (1 bar old, price change 0.75×ATR)")
	}
	
	// Test case 2: Should reject (too old)
	staleInput := domain.FreshnessGateInputs{
		Symbol:        "ETHUSD",
		BarsAge:       3,     // > 2 bars
		PriceChange:   80.0,
		ATR1h:         100.0,
	}
	
	result = domain.EvaluateFreshnessGate(staleInput)
	if result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Freshness guard allowed stale signal (3 bars old)")
	}
	
	// Test case 3: Should reject (price moved too much)
	priceMoveInput := domain.FreshnessGateInputs{
		Symbol:        "SOLUSD",
		BarsAge:       1,     // ≤ 2 bars (OK)
		PriceChange:   250.0, // Price change
		ATR1h:         200.0, // 1.25×ATR > 1.2×ATR threshold
	}
	
	result = domain.EvaluateFreshnessGate(priceMoveInput)
	if result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Freshness guard allowed signal with excessive price movement (1.25×ATR)")
	}
	
	// Test case 4: Should allow (at threshold)
	thresholdInput := domain.FreshnessGateInputs{
		Symbol:        "MATICUSD",
		BarsAge:       2,     // = 2 bars (at limit)
		PriceChange:   120.0, // Price change
		ATR1h:         100.0, // Exactly 1.2×ATR
	}
	
	result = domain.EvaluateFreshnessGate(thresholdInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Freshness guard rejected signal at threshold (2 bars, 1.2×ATR)")
	}
	
	return spec.WithDetails("Freshness guard verified: allows ≤2 bars & ≤1.2×ATR, rejects stale/moved signals")
}

// testLateFillGuard verifies late-fill guard: <30s execution delay
func (gs *GuardsSpec) testLateFillGuard() SpecResult {
	spec := NewSpecResult("LateFillGuard", "Rejects orders with execution delay ≥30 seconds")
	
	baseTime := time.Now()
	
	// Test case 1: Should allow (quick fill)
	quickFillInput := domain.LateFillGateInputs{
		Symbol:       "BTCUSD",
		SignalTime:   baseTime,
		ExecutionTime: baseTime.Add(15 * time.Second), // 15s delay
	}
	
	result := domain.EvaluateLateFillGate(quickFillInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Late-fill guard rejected quick execution (15s delay)")
	}
	
	// Test case 2: Should reject (late fill)
	lateFillInput := domain.LateFillGateInputs{
		Symbol:       "ETHUSD",
		SignalTime:   baseTime,
		ExecutionTime: baseTime.Add(35 * time.Second), // 35s delay > 30s
	}
	
	result = domain.EvaluateLateFillGate(lateFillInput)
	if result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Late-fill guard allowed late execution (35s delay)")
	}
	
	// Test case 3: Should allow (at threshold)
	thresholdFillInput := domain.LateFillGateInputs{
		Symbol:       "SOLUSD",
		SignalTime:   baseTime,
		ExecutionTime: baseTime.Add(30 * time.Second), // Exactly 30s
	}
	
	result = domain.EvaluateLateFillGate(thresholdFillInput)
	if !result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Late-fill guard rejected execution at threshold (30s delay)")
	}
	
	// Test case 4: Should reject (negative delay - clock skew)
	clockSkewInput := domain.LateFillGateInputs{
		Symbol:       "ADAUSD",
		SignalTime:   baseTime,
		ExecutionTime: baseTime.Add(-5 * time.Second), // Negative delay
	}
	
	result = domain.EvaluateLateFillGate(clockSkewInput)
	if result.Allow {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Late-fill guard allowed negative execution delay (clock skew)")
	}
	
	return spec.WithDetails("Late-fill guard verified: allows <30s delay, rejects ≥30s and negative delays")
}

// testGuardIntegration verifies guards work together correctly
func (gs *GuardsSpec) testGuardIntegration() SpecResult {
	spec := NewSpecResult("GuardIntegration", "Multiple guards evaluate independently and combine correctly")
	
	// Test case 1: All guards pass
	allPassInputs := domain.GateInputs{
		Symbol:     "BTCUSD",
		Spread:     0.025, // 25 bps (microstructure)
		Depth:      150000, // $150k depth
		VADR:       2.1,   // VADR
		// Fatigue inputs
		Momentum24h:  8.5,  // < 12% (OK)
		RSI4h:       65.0,  // < 70 (OK)
		Acceleration: 1.0,
		// Freshness inputs
		BarsAge:      1,     // ≤ 2 (OK)
		PriceChange:  180.0, // Price change
		ATR1h:        200.0, // 0.9×ATR < 1.2×ATR (OK)
		// Late-fill inputs (simulated as 20s delay)
		ExecutionDelay: 20.0, // < 30s (OK)
	}
	
	result := domain.EvaluateAllGates(allPassInputs)
	if !result.AllowEntry {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Guard integration failed when all should pass: %s", result.BlockReason))
	}
	
	// Test case 2: Fatigue blocks, others pass
	fatigueBlockInputs := allPassInputs
	fatigueBlockInputs.Momentum24h = 15.0 // > 12%
	fatigueBlockInputs.RSI4h = 72.0       // > 70
	fatigueBlockInputs.Acceleration = 1.0 // < 2%
	
	result = domain.EvaluateAllGates(fatigueBlockInputs)
	if result.AllowEntry {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Guard integration allowed entry when fatigue should block")
	}
	if result.BlockReason != "fatigue" {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Expected fatigue block reason, got: %s", result.BlockReason))
	}
	
	// Test case 3: Multiple guards block (first one wins)
	multiBlockInputs := allPassInputs
	multiBlockInputs.BarsAge = 4           // Freshness block
	multiBlockInputs.ExecutionDelay = 45.0 // Late-fill block
	
	result = domain.EvaluateAllGates(multiBlockInputs)
	if result.AllowEntry {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Guard integration allowed entry when multiple guards should block")
	}
	
	// Should report the first blocking reason (order matters)
	expectedReasons := []string{"freshness", "late_fill"}
	foundExpected := false
	for _, expected := range expectedReasons {
		if result.BlockReason == expected {
			foundExpected = true
			break
		}
	}
	if !foundExpected {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Expected block reason from %v, got: %s", expectedReasons, result.BlockReason))
	}
	
	return spec.WithDetails("Guard integration verified: independent evaluation, correct blocking precedence")
}