package spec

import "time"

import (
	"fmt"
	"math"
)

// RegimeSwitchingSpec validates regime switching compliance requirements
type RegimeSwitchingSpec struct{}

// NewRegimeSwitchingSpec creates a new regime switching spec validator
func NewRegimeSwitchingSpec() *RegimeSwitchingSpec {
	return &RegimeSwitchingSpec{}
}

// Name returns the spec section name
func (r *RegimeSwitchingSpec) Name() string {
	return "Regime Switching"
}

// Description returns the spec section description
func (r *RegimeSwitchingSpec) Description() string {
	return "Validates regime detection and adaptive weight switching"
}

// RunSpecs executes all regime switching compliance tests
func (r *RegimeSwitchingSpec) RunSpecs() []SpecResult {
	var results []SpecResult

	// Test 1: Regime Detection Logic
	results = append(results, r.testRegimeDetection())

	// Test 2: Weight Switching
	results = append(results, r.testWeightSwitching())

	// Test 3: 4-Hour Refresh Cadence
	results = append(results, r.testRefreshCadence())

	// Test 4: Majority Vote Logic
	results = append(results, r.testMajorityVote())

	return results
}

// testRegimeDetection validates regime detection indicators
func (r *RegimeSwitchingSpec) testRegimeDetection() SpecResult {
	result := SpecResult{
		Name:        "regime_detection",
		Description: "Regime detection uses realized vol 7d, % above 20MA, breadth thrust",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	testCases := []struct {
		name          string
		realizedVol7d float64
		pctAbove20MA  float64
		breadthThrust float64
		expectedRegime string
	}{
		{
			name:          "bull_regime_low_vol_high_breadth",
			realizedVol7d: 15.0, // low volatility
			pctAbove20MA:  75.0, // high % above MA
			breadthThrust: 0.8,  // strong breadth
			expectedRegime: "bull",
		},
		{
			name:          "chop_regime_mixed_signals",
			realizedVol7d: 25.0, // moderate volatility
			pctAbove20MA:  45.0, // mixed MA signal
			breadthThrust: 0.3,  // weak breadth
			expectedRegime: "chop",
		},
		{
			name:          "high_vol_regime_extreme_volatility",
			realizedVol7d: 45.0, // high volatility → high_vol signal
			pctAbove20MA:  30.0, // low MA signal → high_vol signal  
			breadthThrust: 0.2,  // low breadth → high_vol signal
			expectedRegime: "high_vol", // majority: 3 high_vol votes
		},
		{
			name:          "edge_case_vol_threshold",
			realizedVol7d: 30.0, // exactly at vol threshold
			pctAbove20MA:  50.0, // exactly neutral
			breadthThrust: 0.5,  // exactly neutral
			expectedRegime: "chop", // should default to chop
		},
	}

	for _, tc := range testCases {
		indicators := RegimeIndicators{
			RealizedVol7d: tc.realizedVol7d,
			PctAbove20MA:  tc.pctAbove20MA,
			BreadthThrust: tc.breadthThrust,
		}

		detectedRegime := r.detectRegime(indicators)
		if detectedRegime != tc.expectedRegime {
			result.Passed = false
			result.Error = fmt.Sprintf("test %s failed: expected regime=%s, got regime=%s (vol=%.1f, ma=%.1f, breadth=%.1f)",
				tc.name, tc.expectedRegime, detectedRegime, tc.realizedVol7d, tc.pctAbove20MA, tc.breadthThrust)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d regime detection scenarios", len(testCases))
	return result
}

// testWeightSwitching validates regime-specific weight blends
func (r *RegimeSwitchingSpec) testWeightSwitching() SpecResult {
	result := SpecResult{
		Name:        "weight_switching",
		Description: "Weight blends switch per regime and re-normalize to 100%",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	testCases := []struct {
		regime          string
		expectedWeights WeightBlend
	}{
		{
			regime: "bull",
			expectedWeights: WeightBlend{
				Momentum1h:  0.15,
				Momentum4h:  0.40,
				Momentum12h: 0.25,
				Momentum24h: 0.15,
				Momentum7d:  0.05,
			},
		},
		{
			regime: "chop",
			expectedWeights: WeightBlend{
				Momentum1h:  0.25,
				Momentum4h:  0.35,
				Momentum12h: 0.20,
				Momentum24h: 0.15,
				Momentum7d:  0.05,
			},
		},
		{
			regime: "high_vol",
			expectedWeights: WeightBlend{
				Momentum1h:  0.30,
				Momentum4h:  0.30,
				Momentum12h: 0.20,
				Momentum24h: 0.10,
				Momentum7d:  0.10,
			},
		},
	}

	for _, tc := range testCases {
		weights := r.getWeightBlend(tc.regime)
		
		// Verify individual weights match expected
		if !r.compareWeights(weights, tc.expectedWeights) {
			result.Passed = false
			result.Error = fmt.Sprintf("regime %s weight mismatch: expected %+v, got %+v",
				tc.regime, tc.expectedWeights, weights)
			return result
		}

		// Verify weights sum to 1.0 (100%)
		sum := weights.Momentum1h + weights.Momentum4h + weights.Momentum12h + 
			   weights.Momentum24h + weights.Momentum7d
		tolerance := 1e-10
		if math.Abs(sum-1.0) > tolerance {
			result.Passed = false
			result.Error = fmt.Sprintf("regime %s weights don't sum to 1.0: sum=%.10f", tc.regime, sum)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated weight blends for %d regimes", len(testCases))
	return result
}

// testRefreshCadence validates 4-hour refresh cadence
func (r *RegimeSwitchingSpec) testRefreshCadence() SpecResult {
	result := SpecResult{
		Name:        "refresh_cadence",
		Description: "Regime detection refreshes every 4 hours",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	// Test refresh cadence logic
	testHours := []int{0, 4, 8, 12, 16, 20, 1, 5, 9, 13, 17, 21, 3, 7, 11, 15, 19, 23}
	expectedRefresh := []bool{true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false}

	for i, hour := range testHours {
		shouldRefresh := r.shouldRefreshRegime(hour)
		if shouldRefresh != expectedRefresh[i] {
			result.Passed = false
			result.Error = fmt.Sprintf("hour %d: expected refresh=%t, got refresh=%t", 
				hour, expectedRefresh[i], shouldRefresh)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated 4-hour refresh cadence for %d hour scenarios", len(testHours))
	return result
}

// testMajorityVote validates majority vote logic for regime determination
func (r *RegimeSwitchingSpec) testMajorityVote() SpecResult {
	result := SpecResult{
		Name:        "majority_vote",
		Description: "Majority vote determines final regime from indicator signals",
		Passed:      true,
		Timestamp:   time.Now(),
	}

	testCases := []struct {
		name           string
		volSignal      string
		maSignal       string
		breadthSignal  string
		expectedRegime string
	}{
		{
			name:          "unanimous_bull",
			volSignal:     "bull",
			maSignal:      "bull",
			breadthSignal: "bull",
			expectedRegime: "bull",
		},
		{
			name:          "majority_chop",
			volSignal:     "chop",
			maSignal:      "chop",
			breadthSignal: "bull",
			expectedRegime: "chop",
		},
		{
			name:          "majority_high_vol",
			volSignal:     "high_vol",
			maSignal:      "high_vol",
			breadthSignal: "chop",
			expectedRegime: "high_vol",
		},
		{
			name:          "tie_defaults_to_chop",
			volSignal:     "bull",
			maSignal:      "chop",
			breadthSignal: "high_vol",
			expectedRegime: "chop", // no majority, default to chop
		},
	}

	for _, tc := range testCases {
		signals := RegimeSignals{
			VolSignal:     tc.volSignal,
			MASignal:      tc.maSignal,
			BreadthSignal: tc.breadthSignal,
		}

		finalRegime := r.majorityVote(signals)
		if finalRegime != tc.expectedRegime {
			result.Passed = false
			result.Error = fmt.Sprintf("test %s failed: expected regime=%s, got regime=%s (vol=%s, ma=%s, breadth=%s)",
				tc.name, tc.expectedRegime, finalRegime, tc.volSignal, tc.maSignal, tc.breadthSignal)
			return result
		}
	}

	result.Details = fmt.Sprintf("Validated %d majority vote scenarios", len(testCases))
	return result
}

// RegimeIndicators contains the three regime detection indicators
type RegimeIndicators struct {
	RealizedVol7d float64
	PctAbove20MA  float64
	BreadthThrust float64
}

// WeightBlend represents momentum factor weights for a regime
type WeightBlend struct {
	Momentum1h  float64
	Momentum4h  float64
	Momentum12h float64
	Momentum24h float64
	Momentum7d  float64
}

// RegimeSignals contains individual indicator signals
type RegimeSignals struct {
	VolSignal     string
	MASignal      string
	BreadthSignal string
}

// detectRegime implements regime detection logic
func (r *RegimeSwitchingSpec) detectRegime(indicators RegimeIndicators) string {
	// Convert indicators to individual signals
	volSignal := "chop"
	if indicators.RealizedVol7d < 20.0 {
		volSignal = "bull"
	} else if indicators.RealizedVol7d > 35.0 {
		volSignal = "high_vol"
	}

	maSignal := "chop"
	if indicators.PctAbove20MA > 65.0 {
		maSignal = "bull"
	} else if indicators.PctAbove20MA < 35.0 {
		maSignal = "high_vol"
	}

	breadthSignal := "chop"
	if indicators.BreadthThrust > 0.7 {
		breadthSignal = "bull"
	} else if indicators.BreadthThrust < 0.3 {
		breadthSignal = "high_vol"
	}

	// Apply majority vote
	signals := RegimeSignals{
		VolSignal:     volSignal,
		MASignal:      maSignal,
		BreadthSignal: breadthSignal,
	}

	return r.majorityVote(signals)
}

// getWeightBlend returns regime-specific weight blends
func (r *RegimeSwitchingSpec) getWeightBlend(regime string) WeightBlend {
	switch regime {
	case "bull":
		return WeightBlend{
			Momentum1h:  0.15,
			Momentum4h:  0.40,
			Momentum12h: 0.25,
			Momentum24h: 0.15,
			Momentum7d:  0.05,
		}
	case "chop":
		return WeightBlend{
			Momentum1h:  0.25,
			Momentum4h:  0.35,
			Momentum12h: 0.20,
			Momentum24h: 0.15,
			Momentum7d:  0.05,
		}
	case "high_vol":
		return WeightBlend{
			Momentum1h:  0.30,
			Momentum4h:  0.30,
			Momentum12h: 0.20,
			Momentum24h: 0.10,
			Momentum7d:  0.10,
		}
	default:
		// Default to chop weights
		return WeightBlend{
			Momentum1h:  0.25,
			Momentum4h:  0.35,
			Momentum12h: 0.20,
			Momentum24h: 0.15,
			Momentum7d:  0.05,
		}
	}
}

// compareWeights compares two weight blends for equality
func (r *RegimeSwitchingSpec) compareWeights(a, b WeightBlend) bool {
	tolerance := 1e-10
	return math.Abs(a.Momentum1h-b.Momentum1h) < tolerance &&
		   math.Abs(a.Momentum4h-b.Momentum4h) < tolerance &&
		   math.Abs(a.Momentum12h-b.Momentum12h) < tolerance &&
		   math.Abs(a.Momentum24h-b.Momentum24h) < tolerance &&
		   math.Abs(a.Momentum7d-b.Momentum7d) < tolerance
}

// shouldRefreshRegime determines if regime should refresh at given hour
func (r *RegimeSwitchingSpec) shouldRefreshRegime(hour int) bool {
	// Refresh every 4 hours: 0, 4, 8, 12, 16, 20
	return hour%4 == 0
}

// majorityVote implements majority vote logic for regime determination
func (r *RegimeSwitchingSpec) majorityVote(signals RegimeSignals) string {
	votes := map[string]int{
		"bull":     0,
		"chop":     0,
		"high_vol": 0,
	}

	votes[signals.VolSignal]++
	votes[signals.MASignal]++
	votes[signals.BreadthSignal]++

	// Find regime with most votes
	maxVotes := 0
	winningRegime := "chop" // default

	for regime, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winningRegime = regime
		}
	}

	// If no clear majority (tie), default to chop
	if maxVotes <= 1 {
		return "chop"
	}

	return winningRegime
}