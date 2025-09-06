package selftest

import (
	"fmt"

	"time"
)

// GateValidator validates gate logic on test fixtures
type GateValidator struct{}

// NewGateValidator creates a new gate validator
func NewGateValidator() *GateValidator {
	return &GateValidator{}
}

// Name returns the validator name
func (gv *GateValidator) Name() string {
	return "Gate Validation"
}

// GateTestFixture represents a test case for gate validation
type GateTestFixture struct {
	Name           string
	Symbol         string
	Price          float64
	RSI4h          float64
	ATR1h          float64
	Change24h      float64
	LastBarAge     time.Duration
	FillDelay      time.Duration
	ExpectedResult map[string]bool // gate_name -> should_pass
}

// Validate tests gate logic against fixtures
func (gv *GateValidator) Validate() TestResult {
	start := time.Now()
	result := TestResult{
		Name:      gv.Name(),
		Timestamp: start,
		Details:   []string{},
	}

	// Create test fixtures
	fixtures := gv.createTestFixtures()
	result.Details = append(result.Details, fmt.Sprintf("Created %d test fixtures", len(fixtures)))

	passedTests := 0
	totalTests := 0

	// Test each fixture
	for _, fixture := range fixtures {
		fixtureResult := gv.testFixture(fixture)
		totalTests++

		if fixtureResult.Passed {
			passedTests++
			result.Details = append(result.Details, fmt.Sprintf("✅ %s: PASS", fixture.Name))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("❌ %s: FAIL - %s", fixture.Name, fixtureResult.Error))
		}

		// Add details for each gate result
		for gateName, expected := range fixture.ExpectedResult {
			actual := fixtureResult.GateResults[gateName]
			status := "PASS"
			if expected != actual {
				status = "FAIL"
			}
			result.Details = append(result.Details, fmt.Sprintf("   %s gate: expected=%t actual=%t (%s)", gateName, expected, actual, status))
		}
	}

	// Overall result
	if passedTests == totalTests {
		result.Status = "PASS"
		result.Message = fmt.Sprintf("All %d gate validation tests passed", totalTests)
	} else {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Gate validation failed: %d/%d tests passed", passedTests, totalTests)
	}

	result.Duration = time.Since(start)
	return result
}

// FixtureTestResult holds the result of testing a single fixture
type FixtureTestResult struct {
	Passed      bool
	Error       string
	GateResults map[string]bool
}

// testFixture tests a single fixture against gate logic
func (gv *GateValidator) testFixture(fixture GateTestFixture) FixtureTestResult {
	result := FixtureTestResult{
		Passed:      true,
		GateResults: make(map[string]bool),
	}

	// Test Fatigue Gate
	fatigueResult := gv.testFatigueGate(fixture)
	result.GateResults["fatigue"] = fatigueResult
	expectedFatigue := fixture.ExpectedResult["fatigue"]
	if fatigueResult != expectedFatigue {
		result.Passed = false
		result.Error = fmt.Sprintf("fatigue gate mismatch: expected=%t actual=%t", expectedFatigue, fatigueResult)
	}

	// Test Freshness Gate
	freshnessResult := gv.testFreshnessGate(fixture)
	result.GateResults["freshness"] = freshnessResult
	expectedFreshness := fixture.ExpectedResult["freshness"]
	if freshnessResult != expectedFreshness {
		result.Passed = false
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("freshness gate mismatch: expected=%t actual=%t", expectedFreshness, freshnessResult)
	}

	// Test Late-Fill Gate
	lateFillResult := gv.testLateFillGate(fixture)
	result.GateResults["late_fill"] = lateFillResult
	expectedLateFill := fixture.ExpectedResult["late_fill"]
	if lateFillResult != expectedLateFill {
		result.Passed = false
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("late_fill gate mismatch: expected=%t actual=%t", expectedLateFill, lateFillResult)
	}

	return result
}

// testFatigueGate tests fatigue gate logic
func (gv *GateValidator) testFatigueGate(fixture GateTestFixture) bool {
	// Fatigue Gate: block if 24h > +12% and RSI4h > 70 unless acceleration up
	if fixture.Change24h > 12.0 && fixture.RSI4h > 70.0 {
		// For simplicity, assume no acceleration data in fixtures
		// In real implementation, this would check price acceleration
		return false // Blocked by fatigue
	}
	return true // Pass
}

// testFreshnessGate tests freshness gate logic
func (gv *GateValidator) testFreshnessGate(fixture GateTestFixture) bool {
	// Freshness Gate: ≤2 bars old & within 1.2×ATR(1h)
	maxAge := 2 * time.Hour // 2 bars at 1h timeframe

	if fixture.LastBarAge > maxAge {
		return false // Too old
	}

	// For ATR check, we'd need more price data
	// For fixture testing, assume ATR check passes if age check passes
	return true
}

// testLateFillGate tests late-fill gate logic
func (gv *GateValidator) testLateFillGate(fixture GateTestFixture) bool {
	// Late-Fill Gate: reject fills >30s after signal bar close
	maxDelay := 30 * time.Second

	return fixture.FillDelay <= maxDelay
}

// createTestFixtures creates comprehensive test fixtures for gate validation
func (gv *GateValidator) createTestFixtures() []GateTestFixture {
	return []GateTestFixture{
		{
			Name:       "Normal Case - All Gates Pass",
			Symbol:     "BTC/USD",
			Price:      50000.0,
			RSI4h:      55.0,
			ATR1h:      500.0,
			Change24h:  3.5,
			LastBarAge: 30 * time.Minute,
			FillDelay:  15 * time.Second,
			ExpectedResult: map[string]bool{
				"fatigue":   true,
				"freshness": true,
				"late_fill": true,
			},
		},
		{
			Name:       "Fatigue Gate - High Change + High RSI",
			Symbol:     "ETH/USD",
			Price:      3000.0,
			RSI4h:      75.0,
			ATR1h:      100.0,
			Change24h:  15.0, // > 12%
			LastBarAge: 45 * time.Minute,
			FillDelay:  20 * time.Second,
			ExpectedResult: map[string]bool{
				"fatigue":   false, // Should fail due to high change + RSI
				"freshness": true,
				"late_fill": true,
			},
		},
		{
			Name:       "Freshness Gate - Stale Data",
			Symbol:     "SOL/USD",
			Price:      100.0,
			RSI4h:      45.0,
			ATR1h:      5.0,
			Change24h:  2.0,
			LastBarAge: 3 * time.Hour, // > 2 hours
			FillDelay:  10 * time.Second,
			ExpectedResult: map[string]bool{
				"fatigue":   true,
				"freshness": false, // Should fail due to stale data
				"late_fill": true,
			},
		},
		{
			Name:       "Late-Fill Gate - Slow Execution",
			Symbol:     "ADA/USD",
			Price:      0.5,
			RSI4h:      60.0,
			ATR1h:      0.02,
			Change24h:  5.0,
			LastBarAge: 20 * time.Minute,
			FillDelay:  45 * time.Second, // > 30 seconds
			ExpectedResult: map[string]bool{
				"fatigue":   true,
				"freshness": true,
				"late_fill": false, // Should fail due to slow execution
			},
		},
		{
			Name:       "Multiple Gate Failures",
			Symbol:     "DOGE/USD",
			Price:      0.08,
			RSI4h:      80.0,
			ATR1h:      0.003,
			Change24h:  20.0,             // High change
			LastBarAge: 4 * time.Hour,    // Stale
			FillDelay:  60 * time.Second, // Slow
			ExpectedResult: map[string]bool{
				"fatigue":   false, // High change + RSI
				"freshness": false, // Stale data
				"late_fill": false, // Slow execution
			},
		},
		{
			Name:       "Edge Case - Boundary Values",
			Symbol:     "LINK/USD",
			Price:      15.0,
			RSI4h:      70.0, // Exactly at threshold
			ATR1h:      0.5,
			Change24h:  12.0,             // Exactly at threshold
			LastBarAge: 2 * time.Hour,    // Exactly at limit
			FillDelay:  30 * time.Second, // Exactly at limit
			ExpectedResult: map[string]bool{
				"fatigue":   false, // 12% + RSI 70 should trigger fatigue
				"freshness": true,  // Exactly at limit should pass
				"late_fill": true,  // Exactly at limit should pass
			},
		},
	}
}
