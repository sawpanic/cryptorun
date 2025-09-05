package scoring

import (
	"math"
)

// VolatilityMetrics contains volatility scoring results
type VolatilityMetrics struct {
	Score         float64 `json:"score"`
	Capped        bool    `json:"capped"`
	OriginalValue float64 `json:"original_value"`
}

// NormalizeVolatilityScore converts volatility factor to 0-100 scoring range with capping and smooth scaling
// Policy: apply cap & smooth scaling so high-volatility dampens score but never NaNs
// Guard NaN/Inf to neutral score
func NormalizeVolatilityScore(volatility float64) VolatilityMetrics {
	originalValue := volatility

	// Handle NaN/Inf by returning 0 score as per task requirements  
	if math.IsNaN(volatility) || math.IsInf(volatility, 0) {
		return VolatilityMetrics{
			Score:         0.0, // NaN/Inf → 0 score
			Capped:        false,
			OriginalValue: originalValue,
		}
	}

	// Work with absolute volatility
	absVolatility := math.Abs(volatility)

	// Cap extreme volatility to prevent score explosion
	const maxVolatility = 80.0
	capped := absVolatility >= maxVolatility
	if capped {
		absVolatility = maxVolatility
	}

	var score float64

	// Optimal volatility around 15-25% (gets highest scores)
	if absVolatility >= 15.0 && absVolatility <= 25.0 {
		score = 100.0
	} else if absVolatility < 15.0 {
		// Low volatility: score decreases as volatility approaches 0
		// Use smooth scaling to avoid sharp transitions
		score = (absVolatility / 15.0) * 100.0
	} else {
		// High volatility: apply aggressive penalty to ensure scores stay low
		// This ensures high volatility gets significantly penalized
		excessVol := absVolatility - 25.0
		
		// Use exponential decay for smoother transitions but with stronger penalty
		// Ensure 40% vol gets ≤40 score, 50% vol gets ≤30 score 
		decayFactor := math.Exp(-excessVol / 15.0) // Faster decay
		score = 15.0 + (85.0 * decayFactor)        // Start from 15, decay to 15
		
		// Additional cap for high volatility ranges
		if absVolatility >= 40.0 {
			score = math.Min(score, 40.0) // Cap 40%+ volatility at 40 score
		}
		if absVolatility >= 50.0 {
			score = math.Min(score, 30.0) // Cap 50%+ volatility at 30 score
		}
	}

	// Ensure score stays in valid range
	score = math.Max(0.0, math.Min(100.0, score))

	return VolatilityMetrics{
		Score:         score,
		Capped:        capped,
		OriginalValue: originalValue,
	}
}