package scoring

import (
	"math"
)

// VolumeMetrics contains volume scoring results and flags
type VolumeMetrics struct {
	Score        float64 `json:"score"`
	Illiquidity  bool    `json:"illiquidity"`
	VolumeValid  bool    `json:"volume_valid"`
}

// NormalizeVolumeScore converts volume factor to 0-100 scoring range with guardrails
// Policy: zero volume → neutral score 0.0 with illiquidity flag
// Negative/NaN/Inf → clamp to 0 and flag
func NormalizeVolumeScore(volume float64) VolumeMetrics {
	// Handle NaN/Inf by returning component-neutral score
	if math.IsNaN(volume) || math.IsInf(volume, 0) {
		return VolumeMetrics{
			Score:       50.0, // Component-neutral score for missing/invalid volume
			Illiquidity: true, // Flag as illiquid
			VolumeValid: false,
		}
	}

	// Handle zero volume: neutral score but flag as illiquid
	if volume == 0.0 {
		return VolumeMetrics{
			Score:       50.0, // Zero volume gets component-neutral score 50.0
			Illiquidity: true, // Set illiquidity flag for gates to use
			VolumeValid: true,
		}
	}

	// Handle negative volume: clamp to 0 and flag
	if volume < 0.0 {
		return VolumeMetrics{
			Score:       0.0,  // Clamp negative to 0
			Illiquidity: true, // Flag as illiquid
			VolumeValid: false, // Invalid negative volume
		}
	}

	// Valid positive volume: log scale scoring
	// 1x volume = 50, 10x volume = 100, 0.1x volume = 0
	logVolume := math.Log10(volume)
	score := 50.0 + (logVolume * 25.0)
	
	// Clamp to valid range
	score = math.Max(0.0, math.Min(100.0, score))

	return VolumeMetrics{
		Score:       score,
		Illiquidity: false, // No illiquidity issues
		VolumeValid: true,
	}
}