package metrics

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"cryptorun/internal/data/facade"
)

// VADR (Volume-Adjusted Daily Range) calculation with freeze logic
// Measures intraday volatility adjusted for volume
// VADR = (High - Low) / Close * Volume^0.5 * 100

// VADRCalculator handles VADR computation with freeze logic
type VADRCalculator struct {
	minBars int  // Minimum bars required (20)
}

// NewVADRCalculator creates a new VADR calculator
func NewVADRCalculator() *VADRCalculator {
	return &VADRCalculator{
		minBars: 20,
	}
}

// Calculate computes VADR for a given time window
// Returns VADR value or 0.0 if frozen due to insufficient data
func (vc *VADRCalculator) Calculate(klines []facade.Kline, window time.Duration) (float64, bool, error) {
	if len(klines) == 0 {
		return 0.0, false, fmt.Errorf("no klines provided")
	}
	
	// Filter klines within the window
	now := time.Now()
	cutoff := now.Add(-window)
	
	var windowKlines []facade.Kline
	for _, kline := range klines {
		if kline.Timestamp.After(cutoff) {
			windowKlines = append(windowKlines, kline)
		}
	}
	
	// Check freeze condition
	if len(windowKlines) < vc.minBars {
		log.Debug().Int("bars", len(windowKlines)).Int("min_bars", vc.minBars).
			Msg("VADR frozen - insufficient bars")
		return 0.0, true, nil  // frozen=true
	}
	
	// Calculate VADR for each bar
	var vadrValues []float64
	
	for _, kline := range windowKlines {
		if kline.Close <= 0 || kline.Volume <= 0 {
			continue  // Skip invalid data
		}
		
		// VADR = (High - Low) / Close * sqrt(Volume) * 100
		range_ := kline.High - kline.Low
		relativeRange := range_ / kline.Close
		volumeAdjustment := math.Sqrt(kline.Volume)
		vadr := relativeRange * volumeAdjustment * 100
		
		vadrValues = append(vadrValues, vadr)
	}
	
	if len(vadrValues) == 0 {
		return 0.0, false, fmt.Errorf("no valid VADR values calculated")
	}
	
	// Return average VADR
	sum := 0.0
	for _, val := range vadrValues {
		sum += val
	}
	avgVADR := sum / float64(len(vadrValues))
	
	log.Debug().Float64("vadr", avgVADR).Int("bars", len(windowKlines)).
		Dur("window", window).Bool("frozen", false).
		Msg("VADR calculated")
	
	return avgVADR, false, nil  // frozen=false
}

// CalculateWithPrecedence computes VADR with tier precedence rule
// gate_vadr = max(p80(24h), tier_min)
func (vc *VADRCalculator) CalculateWithPrecedence(klines []facade.Kline, tierMin float64) (float64, bool, error) {
	// Calculate 24h VADR distribution
	vadr24h, frozen, err := vc.Calculate(klines, 24*time.Hour)
	if err != nil {
		return 0.0, frozen, err
	}
	
	if frozen {
		return 0.0, true, nil
	}
	
	// Calculate P80 from recent VADR values
	p80VADR, err := vc.calculatePercentile(klines, 24*time.Hour, 0.8)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate VADR P80, using current value")
		p80VADR = vadr24h
	}
	
	// Apply precedence rule: max(p80, tier_min)
	gateVADR := math.Max(p80VADR, tierMin)
	
	log.Debug().Float64("vadr_24h", vadr24h).Float64("p80_vadr", p80VADR).
		Float64("tier_min", tierMin).Float64("gate_vadr", gateVADR).
		Msg("VADR precedence applied")
	
	return gateVADR, false, nil
}

// calculatePercentile computes the specified percentile of VADR values
func (vc *VADRCalculator) calculatePercentile(klines []facade.Kline, window time.Duration, percentile float64) (float64, error) {
	now := time.Now()
	cutoff := now.Add(-window)
	
	var vadrValues []float64
	
	for _, kline := range klines {
		if kline.Timestamp.Before(cutoff) || kline.Close <= 0 || kline.Volume <= 0 {
			continue
		}
		
		// Calculate VADR for this bar
		range_ := kline.High - kline.Low
		relativeRange := range_ / kline.Close
		volumeAdjustment := math.Sqrt(kline.Volume)
		vadr := relativeRange * volumeAdjustment * 100
		
		vadrValues = append(vadrValues, vadr)
	}
	
	if len(vadrValues) == 0 {
		return 0.0, fmt.Errorf("no VADR values for percentile calculation")
	}
	
	// Sort values and find percentile
	sort.Float64s(vadrValues)
	index := int(percentile * float64(len(vadrValues)-1))
	
	if index >= len(vadrValues) {
		index = len(vadrValues) - 1
	}
	
	return vadrValues[index], nil
}

// VADRTier represents liquidity tiers with minimum VADR requirements
type VADRTier struct {
	Name     string  `json:"name"`
	MinADV   float64 `json:"min_adv"`    // Minimum Average Daily Volume (USD)
	MinVADR  float64 `json:"min_vadr"`   // Minimum VADR requirement
	MinDepth float64 `json:"min_depth"`  // Minimum depth (USD)
	MaxSpread float64 `json:"max_spread"` // Maximum spread (bps)
}

// GetVADRTier returns the appropriate tier for a given ADV
func GetVADRTier(adv float64) VADRTier {
	tiers := []VADRTier{
		{Name: "tier_1", MinADV: 50000000, MinVADR: 2.5, MinDepth: 500000, MaxSpread: 25},  // $50M+ ADV
		{Name: "tier_2", MinADV: 10000000, MinVADR: 2.0, MinDepth: 250000, MaxSpread: 35},  // $10M+ ADV
		{Name: "tier_3", MinADV: 5000000,  MinVADR: 1.8, MinDepth: 150000, MaxSpread: 45},  // $5M+ ADV
		{Name: "tier_4", MinADV: 1000000,  MinVADR: 1.5, MinDepth: 100000, MaxSpread: 55},  // $1M+ ADV
		{Name: "tier_5", MinADV: 0,        MinVADR: 1.2, MinDepth: 50000,  MaxSpread: 75},  // Default
	}
	
	for _, tier := range tiers {
		if adv >= tier.MinADV {
			return tier
		}
	}
	
	return tiers[len(tiers)-1] // Return lowest tier as fallback
}

// ValidateVADR checks if VADR meets tier requirements
func ValidateVADR(vadr float64, frozen bool, adv float64) (bool, VADRTier, string) {
	if frozen {
		return false, VADRTier{}, "VADR frozen - insufficient data (<20 bars)"
	}
	
	tier := GetVADRTier(adv)
	passes := vadr >= tier.MinVADR
	
	reason := ""
	if !passes {
		reason = fmt.Sprintf("VADR %.2f < tier minimum %.2f", vadr, tier.MinVADR)
	}
	
	log.Debug().Float64("vadr", vadr).Str("tier", tier.Name).
		Float64("tier_min", tier.MinVADR).Bool("passes", passes).
		Str("reason", reason).Msg("VADR validation")
	
	return passes, tier, reason
}

// VADRMetrics provides comprehensive VADR analysis
type VADRMetrics struct {
	Value    float64   `json:"value"`
	Frozen   bool      `json:"frozen"`
	Tier     VADRTier  `json:"tier"`
	P80_24h  float64   `json:"p80_24h"`
	Bars     int       `json:"bars"`
	Window   string    `json:"window"`
	Valid    bool      `json:"valid"`
	Reason   string    `json:"reason,omitempty"`
}

// GetVADRMetrics returns comprehensive VADR analysis
func (vc *VADRCalculator) GetVADRMetrics(klines []facade.Kline, adv float64, window time.Duration) VADRMetrics {
	vadr, frozen, err := vc.Calculate(klines, window)
	
	metrics := VADRMetrics{
		Value:  vadr,
		Frozen: frozen,
		Tier:   GetVADRTier(adv),
		Window: window.String(),
		Bars:   len(klines),
	}
	
	if err != nil {
		metrics.Valid = false
		metrics.Reason = err.Error()
		return metrics
	}
	
	// Calculate P80 if not frozen
	if !frozen {
		p80, _ := vc.calculatePercentile(klines, 24*time.Hour, 0.8)
		metrics.P80_24h = p80
	}
	
	// Validate against tier
	valid, _, reason := ValidateVADR(vadr, frozen, adv)
	metrics.Valid = valid
	metrics.Reason = reason
	
	return metrics
}