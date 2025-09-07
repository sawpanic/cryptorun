package momentum

import (
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/regime"
)

// MTW represents Multi-TimeFrame Weights normalized to 1.0 within active set
type MTW struct {
	W1h  float64 // 1-hour weight
	W4h  float64 // 4-hour weight
	W12h float64 // 12-hour weight
	W24h float64 // 24-hour weight
	W7d  float64 // 7-day weight (bull regime only)
}

// Sum returns the sum of all active weights (should be 1.0)
func (mtw MTW) Sum() float64 {
	return mtw.W1h + mtw.W4h + mtw.W12h + mtw.W24h + mtw.W7d
}

// Normalize ensures weights sum to 1.0 within active set
func (mtw MTW) Normalize() MTW {
	sum := mtw.Sum()
	if sum == 0 {
		return MTW{}
	}
	return MTW{
		W1h:  mtw.W1h / sum,
		W4h:  mtw.W4h / sum,
		W12h: mtw.W12h / sum,
		W24h: mtw.W24h / sum,
		W7d:  mtw.W7d / sum,
	}
}

// CoreInputs represents the input data for MomentumCore calculation
type CoreInputs struct {
	Timestamp time.Time // Data timestamp
	R1h       float64   // 1-hour return (simple or log)
	R4h       float64   // 4-hour return (simple or log)
	R12h      float64   // 12-hour return (simple or log)
	R24h      float64   // 24-hour return (simple or log)
	R7d       float64   // 7-day return (simple or log)
	ATR1h     float64   // 1-hour Average True Range for normalization
	ATR4h     float64   // 4-hour Average True Range for normalization
	Accel4h   float64   // d/dt of R4h over last 2-3 bars
}

// CoreResult represents the output of MomentumCore calculation
type CoreResult struct {
	Score float64 // Final score scaled 0..100
	Parts struct {
		R1h   float64 // Normalized 1h contribution
		R4h   float64 // Normalized 4h contribution
		R12h  float64 // Normalized 12h contribution
		R24h  float64 // Normalized 24h contribution
		R7d   float64 // Normalized 7d contribution
		Accel float64 // Acceleration boost contribution
	}
}

// WeightsForRegime returns regime-specific multi-timeframe weights
func WeightsForRegime(r regime.RegimeType) MTW {
	switch r {
	case regime.TrendingBull:
		// Bull markets: emphasize longer timeframes with 7d carry
		return MTW{
			W1h:  0.15, // Reduced short-term noise
			W4h:  0.30, // Primary signal
			W12h: 0.35, // Strong medium-term trend
			W24h: 0.15, // Daily confirmation
			W7d:  0.05, // Weekly carry (bull only)
		}.Normalize()

	case regime.Choppy:
		// Choppy markets: emphasize shorter timeframes, no 7d carry
		return MTW{
			W1h:  0.25, // Higher short-term weight
			W4h:  0.35, // Primary signal
			W12h: 0.25, // Reduced medium-term
			W24h: 0.15, // Daily confirmation
			W7d:  0.00, // No weekly carry in chop
		}.Normalize()

	case regime.HighVol:
		// High volatility: balanced approach with quality focus
		return MTW{
			W1h:  0.20, // Standard short-term
			W4h:  0.35, // Primary signal
			W12h: 0.30, // Medium-term stability
			W24h: 0.15, // Daily confirmation
			W7d:  0.00, // No weekly carry in volatility
		}.Normalize()

	default:
		// Default to choppy weights
		return MTW{
			W1h:  0.20,
			W4h:  0.35,
			W12h: 0.30,
			W24h: 0.15,
			W7d:  0.00,
		}.Normalize()
	}
}

// ComputeCore calculates the MomentumCore score with ATR normalization and acceleration boost
func ComputeCore(in CoreInputs, w MTW, useCarry bool, accelBoost float64, atrNorm bool) CoreResult {
	result := CoreResult{}

	// Normalize weights to ensure sum = 1.0
	weights := w.Normalize()

	// Apply ATR normalization if enabled
	var r1h, r4h, r12h, r24h, r7d float64
	if atrNorm && in.ATR1h > 0 && in.ATR4h > 0 {
		// Normalize by appropriate ATR to prevent unit drift
		r1h = in.R1h / in.ATR1h
		r4h = in.R4h / in.ATR4h
		r12h = in.R12h / in.ATR4h // Use 4h ATR for longer timeframes
		r24h = in.R24h / in.ATR4h
		r7d = in.R7d / in.ATR4h
	} else {
		// Use raw returns if ATR normalization disabled or invalid
		r1h = in.R1h
		r4h = in.R4h
		r12h = in.R12h
		r24h = in.R24h
		r7d = in.R7d
	}

	// Calculate weighted momentum components
	result.Parts.R1h = r1h * weights.W1h
	result.Parts.R4h = r4h * weights.W4h
	result.Parts.R12h = r12h * weights.W12h
	result.Parts.R24h = r24h * weights.W24h

	// 7d carry only in bull markets if useCarry is enabled
	if useCarry {
		result.Parts.R7d = r7d * weights.W7d
	} else {
		result.Parts.R7d = 0.0
	}

	// Base momentum score (sum of weighted components)
	baseScore := result.Parts.R1h + result.Parts.R4h + result.Parts.R12h +
		result.Parts.R24h + result.Parts.R7d

	// 4h acceleration boost: only if fresh (≤2 bars) and sign-aligned with R4h
	result.Parts.Accel = 0.0
	if accelBoost > 0 && isFresh(in.Accel4h) && isSignAligned(r4h, in.Accel4h) {
		result.Parts.Accel = accelBoost * math.Abs(in.Accel4h)
		if r4h < 0 {
			// If R4h is negative, acceleration should subtract from score
			result.Parts.Accel = -result.Parts.Accel
		}
	}

	// Final score with acceleration boost
	rawScore := baseScore + result.Parts.Accel

	// Scale to 0..100 range (assuming normalized returns are roughly -3 to +3)
	// Use tanh scaling to bound the output
	result.Score = 50.0 * (1.0 + math.Tanh(rawScore/2.0)) * 2.0

	// Ensure score is within bounds
	result.Score = math.Max(0.0, math.Min(100.0, result.Score))

	return result
}

// isFresh determines if acceleration data is fresh (≤2 bars old)
func isFresh(accel float64) bool {
	// Simple freshness check: non-zero acceleration indicates recent data
	// In real implementation, this would check timestamp freshness
	return math.Abs(accel) > 1e-10
}

// isSignAligned checks if acceleration and return have the same sign
func isSignAligned(return4h, accel4h float64) bool {
	if math.Abs(return4h) < 1e-10 || math.Abs(accel4h) < 1e-10 {
		return false
	}
	return (return4h > 0 && accel4h > 0) || (return4h < 0 && accel4h < 0)
}

// ValidateInputs performs basic validation of CoreInputs
func ValidateInputs(in CoreInputs) error {
	// Check for NaN or infinite values
	if math.IsNaN(in.R1h) || math.IsInf(in.R1h, 0) {
		return fmt.Errorf("invalid R1h: %f", in.R1h)
	}
	if math.IsNaN(in.R4h) || math.IsInf(in.R4h, 0) {
		return fmt.Errorf("invalid R4h: %f", in.R4h)
	}
	if math.IsNaN(in.R12h) || math.IsInf(in.R12h, 0) {
		return fmt.Errorf("invalid R12h: %f", in.R12h)
	}
	if math.IsNaN(in.R24h) || math.IsInf(in.R24h, 0) {
		return fmt.Errorf("invalid R24h: %f", in.R24h)
	}
	if math.IsNaN(in.R7d) || math.IsInf(in.R7d, 0) {
		return fmt.Errorf("invalid R7d: %f", in.R7d)
	}

	// Check ATR values are positive
	if in.ATR1h < 0 {
		return fmt.Errorf("ATR1h must be non-negative: %f", in.ATR1h)
	}
	if in.ATR4h < 0 {
		return fmt.Errorf("ATR4h must be non-negative: %f", in.ATR4h)
	}

	return nil
}

// GetMomentumBreakdown returns detailed breakdown of momentum components
func GetMomentumBreakdown(result CoreResult) map[string]interface{} {
	return map[string]interface{}{
		"final_score": result.Score,
		"components": map[string]float64{
			"r1h_contribution":   result.Parts.R1h,
			"r4h_contribution":   result.Parts.R4h,
			"r12h_contribution":  result.Parts.R12h,
			"r24h_contribution":  result.Parts.R24h,
			"r7d_contribution":   result.Parts.R7d,
			"accel_contribution": result.Parts.Accel,
		},
		"base_momentum": result.Parts.R1h + result.Parts.R4h + result.Parts.R12h +
			result.Parts.R24h + result.Parts.R7d,
		"total_with_accel": result.Parts.R1h + result.Parts.R4h + result.Parts.R12h +
			result.Parts.R24h + result.Parts.R7d + result.Parts.Accel,
	}
}
