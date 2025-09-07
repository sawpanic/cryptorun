package spec

import (
	"fmt"
	"math"

	"github.com/sawpanic/cryptorun/internal/domain"
)

// RegimeSwitchSpec tests regime switching compliance
type RegimeSwitchSpec struct{}

// Name returns the section name
func (rss *RegimeSwitchSpec) Name() string {
	return "Regime Switch"
}

// Description returns the section description
func (rss *RegimeSwitchSpec) Description() string {
	return "Weights update & re-normalize to 1.0 on regime transitions"
}

// RunSpecs executes all regime switch specification tests
func (rss *RegimeSwitchSpec) RunSpecs() []SpecResult {
	var results []SpecResult

	// Test 1: Weight Normalization
	results = append(results, rss.testWeightNormalization())

	// Test 2: Regime Transition
	results = append(results, rss.testRegimeTransition())

	// Test 3: Weight Set Consistency
	results = append(results, rss.testWeightSetConsistency())

	// Test 4: Regime Detection Logic
	results = append(results, rss.testRegimeDetectionLogic())

	return results
}

// testWeightNormalization verifies weights sum to 1.0 in all regimes
func (rss *RegimeSwitchSpec) testWeightNormalization() SpecResult {
	spec := NewSpecResult("WeightNormalization", "Factor weights sum to 1.0 across all regime states")

	// Test all regime types
	regimeTypes := []string{"bull", "chop", "high_vol"}

	for _, regimeType := range regimeTypes {
		weights := domain.GetRegimeWeights(regimeType)

		// Calculate weight sum
		weightSum := weights.Momentum1h + weights.Momentum4h + weights.Momentum12h +
			weights.Momentum24h + weights.Momentum7d

		// Verify sum equals 1.0 within tolerance
		tolerance := 1e-10
		if math.Abs(weightSum-1.0) > tolerance {
			return NewFailedSpecResult(spec.Name, spec.Description,
				fmt.Sprintf("Regime %s weights sum to %.10f, expected 1.0", regimeType, weightSum))
		}

		// Verify all weights are non-negative
		allWeights := []float64{weights.Momentum1h, weights.Momentum4h, weights.Momentum12h,
			weights.Momentum24h, weights.Momentum7d}
		for i, weight := range allWeights {
			if weight < 0 {
				timeframes := []string{"1h", "4h", "12h", "24h", "7d"}
				return NewFailedSpecResult(spec.Name, spec.Description,
					fmt.Sprintf("Regime %s has negative weight for %s: %.6f",
						regimeType, timeframes[i], weight))
			}
		}
	}

	return spec.WithDetails("Weight normalization verified: all regimes sum to 1.0, no negative weights")
}

// testRegimeTransition verifies proper weight switching on regime changes
func (rss *RegimeSwitchSpec) testRegimeTransition() SpecResult {
	spec := NewSpecResult("RegimeTransition", "Weights switch correctly on regime state transitions")

	// Create regime detector
	detector := domain.NewRegimeDetector()

	// Test transition from bull to chop
	bullInputs := domain.RegimeInputs{
		RealizedVol7d: 0.15, // Low volatility
		PctAbove20MA:  75.0, // High percentage above MA
		BreadthThrust: 0.8,  // Strong breadth
	}

	chopInputs := domain.RegimeInputs{
		RealizedVol7d: 0.25, // Higher volatility
		PctAbove20MA:  45.0, // Lower percentage above MA
		BreadthThrust: 0.4,  // Weak breadth
	}

	// Detect regimes
	bullRegime := detector.DetectRegime(bullInputs)
	chopRegime := detector.DetectRegime(chopInputs)

	// Verify different regimes detected
	if bullRegime == chopRegime {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Same regime detected for different inputs: %s", bullRegime))
	}

	// Get weights for each regime
	bullWeights := domain.GetRegimeWeights(bullRegime)
	chopWeights := domain.GetRegimeWeights(chopRegime)

	// Verify weights are different (at least one significant difference)
	weightDifferences := []float64{
		math.Abs(bullWeights.Momentum1h - chopWeights.Momentum1h),
		math.Abs(bullWeights.Momentum4h - chopWeights.Momentum4h),
		math.Abs(bullWeights.Momentum12h - chopWeights.Momentum12h),
		math.Abs(bullWeights.Momentum24h - chopWeights.Momentum24h),
		math.Abs(bullWeights.Momentum7d - chopWeights.Momentum7d),
	}

	maxDifference := 0.0
	for _, diff := range weightDifferences {
		if diff > maxDifference {
			maxDifference = diff
		}
	}

	// Should have meaningful weight difference (>5%)
	minSignificantDiff := 0.05
	if maxDifference < minSignificantDiff {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Weight differences too small between regimes: max diff %.3f < %.3f",
				maxDifference, minSignificantDiff))
	}

	return spec.WithDetails(fmt.Sprintf("Regime transition verified: %s → %s, max weight diff: %.3f",
		bullRegime, chopRegime, maxDifference))
}

// testWeightSetConsistency verifies weight sets are internally consistent
func (rss *RegimeSwitchSpec) testWeightSetConsistency() SpecResult {
	spec := NewSpecResult("WeightSetConsistency", "Regime weight sets follow expected patterns and constraints")

	// Get all regime weights
	bullWeights := domain.GetRegimeWeights("bull")
	chopWeights := domain.GetRegimeWeights("chop")
	highVolWeights := domain.GetRegimeWeights("high_vol")

	// Test bull regime characteristics (should favor longer timeframes)
	if bullWeights.Momentum24h < bullWeights.Momentum1h {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Bull regime should favor longer timeframes: 24h weight < 1h weight")
	}

	// Test chop regime characteristics (should favor shorter timeframes)
	if chopWeights.Momentum1h < chopWeights.Momentum24h {
		return NewFailedSpecResult(spec.Name, spec.Description,
			"Chop regime should favor shorter timeframes: 1h weight < 24h weight")
	}

	// Test high-vol regime has balanced distribution
	highVolVariance := calculateWeightVariance([]float64{
		highVolWeights.Momentum1h, highVolWeights.Momentum4h,
		highVolWeights.Momentum12h, highVolWeights.Momentum24h,
		highVolWeights.Momentum7d,
	})

	// High-vol should have lower variance (more balanced)
	bullVariance := calculateWeightVariance([]float64{
		bullWeights.Momentum1h, bullWeights.Momentum4h,
		bullWeights.Momentum12h, bullWeights.Momentum24h,
		bullWeights.Momentum7d,
	})

	if highVolVariance > bullVariance {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("High-vol regime should be more balanced: variance %.4f > bull %.4f",
				highVolVariance, bullVariance))
	}

	// Test that 4h weight is significant in all regimes (core timeframe)
	minCore4hWeight := 0.15 // Minimum 15% for core 4h
	regimeName := []string{"bull", "chop", "high_vol"}
	regimeWeights := []*domain.RegimeWeights{&bullWeights, &chopWeights, &highVolWeights}

	for i, weights := range regimeWeights {
		if weights.Momentum4h < minCore4hWeight {
			return NewFailedSpecResult(spec.Name, spec.Description,
				fmt.Sprintf("Regime %s has insufficient 4h core weight: %.3f < %.3f",
					regimeName[i], weights.Momentum4h, minCore4hWeight))
		}
	}

	return spec.WithDetails("Weight set consistency verified: regime patterns, core 4h weight, balanced high-vol")
}

// testRegimeDetectionLogic verifies regime detection algorithm
func (rss *RegimeSwitchSpec) testRegimeDetectionLogic() SpecResult {
	spec := NewSpecResult("RegimeDetectionLogic", "Regime detection uses volatility, MA, and breadth correctly")

	detector := domain.NewRegimeDetector()

	// Test clear bull market conditions
	clearBullInputs := domain.RegimeInputs{
		RealizedVol7d: 0.10, // Very low volatility
		PctAbove20MA:  85.0, // Most prices above MA
		BreadthThrust: 0.9,  // Very strong breadth
	}

	bullRegime := detector.DetectRegime(clearBullInputs)
	if bullRegime != "bull" {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Clear bull conditions detected as: %s", bullRegime))
	}

	// Test clear high-volatility conditions
	highVolInputs := domain.RegimeInputs{
		RealizedVol7d: 0.45, // Very high volatility
		PctAbove20MA:  60.0, // Mixed MA situation
		BreadthThrust: 0.6,  // Moderate breadth
	}

	highVolRegime := detector.DetectRegime(highVolInputs)
	if highVolRegime != "high_vol" {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("High volatility conditions detected as: %s", highVolRegime))
	}

	// Test choppy market conditions
	choppyInputs := domain.RegimeInputs{
		RealizedVol7d: 0.20, // Moderate volatility
		PctAbove20MA:  35.0, // Most prices below MA
		BreadthThrust: 0.3,  // Weak breadth
	}

	choppyRegime := detector.DetectRegime(choppyInputs)
	if choppyRegime != "chop" {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Choppy conditions detected as: %s", choppyRegime))
	}

	// Test edge cases and boundary conditions
	boundaryInputs := domain.RegimeInputs{
		RealizedVol7d: 0.25, // At boundary
		PctAbove20MA:  50.0, // Neutral
		BreadthThrust: 0.5,  // Neutral
	}

	boundaryRegime := detector.DetectRegime(boundaryInputs)
	validRegimes := []string{"bull", "chop", "high_vol"}
	isValid := false
	for _, validRegime := range validRegimes {
		if boundaryRegime == validRegime {
			isValid = true
			break
		}
	}

	if !isValid {
		return NewFailedSpecResult(spec.Name, spec.Description,
			fmt.Sprintf("Invalid regime detected at boundary: %s", boundaryRegime))
	}

	return spec.WithDetails("Regime detection logic verified: bull/chop/high-vol correctly identified")
}

// calculateWeightVariance computes variance of weight distribution
func calculateWeightVariance(weights []float64) float64 {
	if len(weights) == 0 {
		return 0.0
	}

	// Calculate mean
	mean := 0.0
	for _, weight := range weights {
		mean += weight
	}
	mean /= float64(len(weights))

	// Calculate variance
	variance := 0.0
	for _, weight := range weights {
		diff := weight - mean
		variance += diff * diff
	}
	variance /= float64(len(weights))

	return variance
}
