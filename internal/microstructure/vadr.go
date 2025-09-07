package microstructure

import (
	"fmt"
	"math"
)

// VADRCalculator computes Volume-Adjusted Daily Range with tier precedence
// VADR = max(p80_threshold, tier_minimum) where p80 is 80th percentile of 24h VADR
type VADRCalculator struct {
	historicalVADR []float64 // 24h rolling history
	maxHistory     int       // Maximum history size
}

// NewVADRCalculator creates a VADR calculator
func NewVADRCalculator() *VADRCalculator {
	return &VADRCalculator{
		historicalVADR: make([]float64, 0, 288), // 24h * 12 (5min intervals)
		maxHistory:     288,
	}
}

// VADRResult contains VADR calculation results
type VADRResult struct {
	Current      float64         `json:"current"`       // Current VADR multiple
	P80Threshold float64         `json:"p80_threshold"` // 80th percentile of 24h history
	TierMinimum  float64         `json:"tier_minimum"`  // Tier minimum requirement
	EffectiveMin float64         `json:"effective_min"` // max(p80, tier_min)
	PassesGate   bool            `json:"passes_gate"`   // current >= effective_min
	HistoryCount int             `json:"history_count"` // Samples in 24h window
	Percentiles  VADRPercentiles `json:"percentiles"`   // Full percentile breakdown
}

// VADRPercentiles contains percentile analysis of historical VADR
type VADRPercentiles struct {
	P10  float64 `json:"p10"`  // 10th percentile
	P25  float64 `json:"p25"`  // 25th percentile
	P50  float64 `json:"p50"`  // Median
	P75  float64 `json:"p75"`  // 75th percentile
	P80  float64 `json:"p80"`  // 80th percentile (key threshold)
	P90  float64 `json:"p90"`  // 90th percentile
	P95  float64 `json:"p95"`  // 95th percentile
	Min  float64 `json:"min"`  // Minimum value
	Max  float64 `json:"max"`  // Maximum value
	Mean float64 `json:"mean"` // Average value
}

// VADRInput contains inputs for VADR calculation
type VADRInput struct {
	High         float64 `json:"high"`          // 24h high price
	Low          float64 `json:"low"`           // 24h low price
	Volume       float64 `json:"volume"`        // 24h volume in base units
	ADV          float64 `json:"adv"`           // Average Daily Volume (USD)
	CurrentPrice float64 `json:"current_price"` // Current price for validation
}

// CalculateVADR computes current VADR and evaluates against tier requirements
func (vc *VADRCalculator) CalculateVADR(input *VADRInput, tier *LiquidityTier) (*VADRResult, error) {
	if input == nil || tier == nil {
		return nil, fmt.Errorf("invalid inputs: input=%v, tier=%v", input, tier)
	}

	if input.High <= 0 || input.Low <= 0 || input.Volume <= 0 || input.ADV <= 0 {
		return nil, fmt.Errorf("invalid input values: high=%.6f, low=%.6f, volume=%.2f, adv=%.0f",
			input.High, input.Low, input.Volume, input.ADV)
	}

	if input.High < input.Low {
		return nil, fmt.Errorf("invalid price range: high=%.6f < low=%.6f", input.High, input.Low)
	}

	// Calculate current VADR
	// VADR = (High - Low) / (Volume / ADV) = Range / Volume_Multiple
	priceRange := input.High - input.Low
	volumeMultiple := input.Volume * input.CurrentPrice / input.ADV

	if volumeMultiple <= 0 {
		return nil, fmt.Errorf("invalid volume multiple: %.6f", volumeMultiple)
	}

	currentVADR := priceRange / (input.CurrentPrice * volumeMultiple)

	// Add to history for percentile calculation
	vc.addToHistory(currentVADR)

	// Calculate percentiles from history
	percentiles := vc.calculatePercentiles()

	// Determine effective minimum: max(p80, tier_minimum)
	p80Threshold := percentiles.P80
	tierMinimum := tier.VADRMinimum
	effectiveMin := math.Max(p80Threshold, tierMinimum)

	// Check if current VADR passes gate
	passesGate := currentVADR >= effectiveMin

	result := &VADRResult{
		Current:      currentVADR,
		P80Threshold: p80Threshold,
		TierMinimum:  tierMinimum,
		EffectiveMin: effectiveMin,
		PassesGate:   passesGate,
		HistoryCount: len(vc.historicalVADR),
		Percentiles:  percentiles,
	}

	return result, nil
}

// addToHistory adds VADR value and maintains rolling 24h window
func (vc *VADRCalculator) addToHistory(vadr float64) {
	if !math.IsNaN(vadr) && !math.IsInf(vadr, 0) && vadr > 0 {
		vc.historicalVADR = append(vc.historicalVADR, vadr)

		// Trim to max history
		if len(vc.historicalVADR) > vc.maxHistory {
			vc.historicalVADR = vc.historicalVADR[1:]
		}
	}
}

// calculatePercentiles computes percentile breakdown from history
func (vc *VADRCalculator) calculatePercentiles() VADRPercentiles {
	if len(vc.historicalVADR) == 0 {
		// Return defaults when no history
		return VADRPercentiles{
			P10: 1.0, P25: 1.2, P50: 1.5, P75: 1.8, P80: 2.0,
			P90: 2.5, P95: 3.0, Min: 1.0, Max: 3.0, Mean: 1.75,
		}
	}

	// Create sorted copy for percentile calculation
	sorted := make([]float64, len(vc.historicalVADR))
	copy(sorted, vc.historicalVADR)

	// Simple bubble sort (acceptable for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)

	// Calculate percentiles using linear interpolation
	percentiles := VADRPercentiles{
		P10: calculatePercentile(sorted, 0.10),
		P25: calculatePercentile(sorted, 0.25),
		P50: calculatePercentile(sorted, 0.50),
		P75: calculatePercentile(sorted, 0.75),
		P80: calculatePercentile(sorted, 0.80), // Key threshold
		P90: calculatePercentile(sorted, 0.90),
		P95: calculatePercentile(sorted, 0.95),
		Min: sorted[0],
		Max: sorted[n-1],
	}

	// Calculate mean
	sum := 0.0
	for _, val := range sorted {
		sum += val
	}
	percentiles.Mean = sum / float64(n)

	return percentiles
}

// calculatePercentile computes a specific percentile using linear interpolation
func calculatePercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0.0
	}

	if len(sorted) == 1 {
		return sorted[0]
	}

	// Calculate index position
	pos := p * float64(len(sorted)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))

	if lower == upper {
		return sorted[lower]
	}

	// Linear interpolation
	weight := pos - float64(lower)
	return sorted[lower]*(1.0-weight) + sorted[upper]*weight
}

// ValidateVADRRequirement checks if VADR meets effective minimum
func (vc *VADRCalculator) ValidateVADRRequirement(vadrResult *VADRResult) (bool, string) {
	if vadrResult == nil {
		return false, "no VADR data"
	}

	if vadrResult.PassesGate {
		return true, fmt.Sprintf("VADR %.3f ≥ %.3f (max of p80=%.3f, tier_min=%.3f)",
			vadrResult.Current, vadrResult.EffectiveMin,
			vadrResult.P80Threshold, vadrResult.TierMinimum)
	}

	return false, fmt.Sprintf("VADR insufficient: %.3f < %.3f (need max of p80=%.3f, tier_min=%.3f)",
		vadrResult.Current, vadrResult.EffectiveMin,
		vadrResult.P80Threshold, vadrResult.TierMinimum)
}

// GetVADRSummary returns human-readable VADR summary
func (vc *VADRCalculator) GetVADRSummary(vadrResult *VADRResult) string {
	if vadrResult == nil {
		return "no VADR data"
	}

	status := "PASS"
	if !vadrResult.PassesGate {
		status = "FAIL"
	}

	return fmt.Sprintf("VADR: %.3f× %s (need ≥%.3f, p80=%.3f, tier=%.3f, %d samples)",
		vadrResult.Current,
		status,
		vadrResult.EffectiveMin,
		vadrResult.P80Threshold,
		vadrResult.TierMinimum,
		vadrResult.HistoryCount)
}

// IsVADRHistoryAdequate checks if we have sufficient history for reliable p80
func (vc *VADRCalculator) IsVADRHistoryAdequate() bool {
	return len(vc.historicalVADR) >= 50 // Need at least ~4h of data for reliable p80
}

// GetVADRHistoryStats returns current history statistics
func (vc *VADRCalculator) GetVADRHistoryStats() map[string]interface{} {
	if len(vc.historicalVADR) == 0 {
		return map[string]interface{}{
			"count":  0,
			"status": "no_data",
		}
	}

	percentiles := vc.calculatePercentiles()

	status := "sparse"
	if len(vc.historicalVADR) >= 50 {
		status = "adequate"
	}
	if len(vc.historicalVADR) >= 200 {
		status = "excellent"
	}

	return map[string]interface{}{
		"count":       len(vc.historicalVADR),
		"max_history": vc.maxHistory,
		"status":      status,
		"percentiles": percentiles,
		"utilization": float64(len(vc.historicalVADR)) / float64(vc.maxHistory),
	}
}

// ClearHistory clears VADR history (useful for testing)
func (vc *VADRCalculator) ClearHistory() {
	vc.historicalVADR = vc.historicalVADR[:0]
}

// LoadHistoricalVADR loads historical VADR values (for initialization)
func (vc *VADRCalculator) LoadHistoricalVADR(values []float64) {
	vc.historicalVADR = make([]float64, 0, vc.maxHistory)
	for _, val := range values {
		vc.addToHistory(val)
	}
}
