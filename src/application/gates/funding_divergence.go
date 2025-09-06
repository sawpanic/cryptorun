package gates

import (
	"fmt"
	"math"
	"time"

	"cryptorun/src/domain/derivs"
	"cryptorun/src/infrastructure/derivs"
)

// FundingDivergenceGate implements funding divergence entry gate logic
type FundingDivergenceGate struct {
	config        FundingGateConfig
	providerMgr   *derivs.ProviderManager
	derivsMetrics *derivs.DerivativesMetrics
}

// FundingGateConfig holds configuration for funding divergence gate
type FundingGateConfig struct {
	Enabled              bool    `yaml:"enabled"`
	MinZScoreMagnitude   float64 `yaml:"min_zscore_magnitude"`    // Minimum |z-score| for signal
	PriceVsVWAPThreshold float64 `yaml:"price_vs_vwap_threshold"` // Price vs VWAP ratio threshold
	MinVenueVolume       float64 `yaml:"min_venue_volume"`        // Minimum venue volume for inclusion
	VolumeLookbackHours  int     `yaml:"volume_lookback_hours"`   // Hours for volume calculation
	MinVenuesRequired    int     `yaml:"min_venues_required"`     // Minimum venues for valid signal
	MaxFundingAgeHours   int     `yaml:"max_funding_age_hours"`   // Maximum age of funding data
}

// FundingDivergenceResult contains gate evaluation result
type FundingDivergenceResult struct {
	DivergencePresent  bool               `json:"divergence_present"`       // Gate pass/fail
	FundingZScore      float64            `json:"funding_zscore"`           // Volume-weighted funding z-score
	PriceVsVWAP        float64            `json:"price_vs_vwap"`            // Price relative to VWAP
	ValidVenues        int                `json:"valid_venues"`             // Number of venues used
	VenueContributions map[string]float64 `json:"venue_contributions"`      // Individual venue funding rates
	DataFreshness      time.Duration      `json:"data_freshness"`           // Age of newest funding data
	SignalQuality      string             `json:"signal_quality"`           // Quality assessment
	FailureReason      string             `json:"failure_reason,omitempty"` // Why gate failed (if applicable)
	Timestamp          time.Time          `json:"timestamp"`                // Evaluation timestamp
}

// VWAPData represents volume-weighted average price data
type VWAPData struct {
	Symbol       string    `json:"symbol"`
	VWAP24h      float64   `json:"vwap_24h"`      // 24-hour VWAP
	CurrentPrice float64   `json:"current_price"` // Current mark/last price
	Volume24h    float64   `json:"volume_24h"`    // 24-hour volume
	LastUpdated  time.Time `json:"last_updated"`  // Data timestamp
}

func NewFundingDivergenceGate(
	config FundingGateConfig,
	providerMgr *derivs.ProviderManager,
	derivsMetrics *derivs.DerivativesMetrics) *FundingDivergenceGate {

	return &FundingDivergenceGate{
		config:        config,
		providerMgr:   providerMgr,
		derivsMetrics: derivsMetrics,
	}
}

// EvaluateDivergence checks if funding divergence conditions are met
func (fdg *FundingDivergenceGate) EvaluateDivergence(symbol string) (*FundingDivergenceResult, error) {
	if !fdg.config.Enabled {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			SignalQuality:     "disabled",
			FailureReason:     "funding divergence gate disabled",
			Timestamp:         time.Now(),
		}, nil
	}

	// Gather venue data
	venueData, err := fdg.gatherVenueData(symbol)
	if err != nil {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			SignalQuality:     "error",
			FailureReason:     fmt.Sprintf("data gathering failed: %v", err),
			Timestamp:         time.Now(),
		}, nil
	}

	// Check minimum venues requirement
	if len(venueData) < fdg.config.MinVenuesRequired {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			ValidVenues:       len(venueData),
			SignalQuality:     "insufficient_venues",
			FailureReason:     fmt.Sprintf("need ≥%d venues, got %d", fdg.config.MinVenuesRequired, len(venueData)),
			Timestamp:         time.Now(),
		}, nil
	}

	// Calculate funding z-score
	fundingResult, err := fdg.derivsMetrics.FundingZ(venueData)
	if err != nil {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			SignalQuality:     "calculation_error",
			FailureReason:     fmt.Sprintf("funding z-score calculation failed: %v", err),
			Timestamp:         time.Now(),
		}, nil
	}

	// Gather VWAP data for price comparison
	vwapData, err := fdg.getVWAPData(symbol)
	if err != nil {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			FundingZScore:     fundingResult.ZScore,
			ValidVenues:       fundingResult.ValidVenues,
			SignalQuality:     "vwap_error",
			FailureReason:     fmt.Sprintf("VWAP data unavailable: %v", err),
			Timestamp:         time.Now(),
		}, nil
	}

	// Calculate price vs VWAP ratio
	priceVsVWAP := vwapData.CurrentPrice / vwapData.VWAP24h

	// Check data freshness
	dataFreshness := fdg.checkDataFreshness(venueData)
	if dataFreshness > time.Duration(fdg.config.MaxFundingAgeHours)*time.Hour {
		return &FundingDivergenceResult{
			DivergencePresent: false,
			FundingZScore:     fundingResult.ZScore,
			PriceVsVWAP:       priceVsVWAP,
			ValidVenues:       fundingResult.ValidVenues,
			DataFreshness:     dataFreshness,
			SignalQuality:     "stale_data",
			FailureReason:     fmt.Sprintf("funding data too old: %v", dataFreshness),
			Timestamp:         time.Now(),
		}, nil
	}

	// Evaluate divergence conditions
	divergencePresent := fdg.evaluateDivergenceConditions(fundingResult.ZScore, priceVsVWAP)

	// Determine signal quality
	signalQuality := fdg.assessSignalQuality(fundingResult, len(venueData), dataFreshness)

	result := &FundingDivergenceResult{
		DivergencePresent:  divergencePresent,
		FundingZScore:      fundingResult.ZScore,
		PriceVsVWAP:        priceVsVWAP,
		ValidVenues:        fundingResult.ValidVenues,
		VenueContributions: fundingResult.VenueContributions,
		DataFreshness:      dataFreshness,
		SignalQuality:      signalQuality,
		Timestamp:          time.Now(),
	}

	// Add failure reason if gate doesn't pass
	if !divergencePresent {
		result.FailureReason = fdg.getDivergenceFailureReason(fundingResult.ZScore, priceVsVWAP)
	}

	return result, nil
}

// gatherVenueData collects funding and volume data from all providers
func (fdg *FundingDivergenceGate) gatherVenueData(symbol string) ([]derivs.VenueData, error) {
	providers := fdg.providerMgr.GetProviders()
	var venueData []derivs.VenueData

	for _, provider := range providers {
		// Get funding history (recent rates for z-score calculation)
		fundingRates, err := provider.GetFundingHistory(nil, symbol, 100) // Last 100 funding periods
		if err != nil {
			continue // Skip provider on error, don't fail entire gate
		}

		// Get volume data for weighting
		tickerData, err := provider.GetTickerData(nil, symbol)
		if err != nil {
			continue
		}

		// Filter by minimum volume requirement
		if tickerData.QuoteVolume < fdg.config.MinVenueVolume {
			continue
		}

		// Convert to DerivativesMetrics format
		var fundingPoints []derivs.FundingRatePoint
		for _, rate := range fundingRates {
			fundingPoints = append(fundingPoints, derivs.FundingRatePoint{
				Rate:      rate.Rate,
				Timestamp: rate.Timestamp,
				MarkPrice: rate.MarkPrice,
			})
		}

		venueData = append(venueData, derivs.VenueData{
			VenueName:    provider.Name(),
			FundingRates: fundingPoints,
			Volume:       tickerData.QuoteVolume,
			LastUpdated:  tickerData.Timestamp,
		})
	}

	return venueData, nil
}

// getVWAPData retrieves volume-weighted average price data
func (fdg *FundingDivergenceGate) getVWAPData(symbol string) (*VWAPData, error) {
	providers := fdg.providerMgr.GetProviders()

	// Use first available provider for VWAP data
	// In production, might want to aggregate across providers
	for _, provider := range providers {
		tickerData, err := provider.GetTickerData(nil, symbol)
		if err != nil {
			continue
		}

		return &VWAPData{
			Symbol:       symbol,
			VWAP24h:      tickerData.WeightedAvgPrice,
			CurrentPrice: tickerData.LastPrice,
			Volume24h:    tickerData.QuoteVolume,
			LastUpdated:  tickerData.Timestamp,
		}, nil
	}

	return nil, fmt.Errorf("no providers returned valid VWAP data for %s", symbol)
}

// checkDataFreshness determines how old the newest funding data is
func (fdg *FundingDivergenceGate) checkDataFreshness(venueData []derivs.VenueData) time.Duration {
	newestTime := time.Time{}

	for _, venue := range venueData {
		if len(venue.FundingRates) > 0 {
			latest := venue.FundingRates[len(venue.FundingRates)-1].Timestamp
			if latest.After(newestTime) {
				newestTime = latest
			}
		}
	}

	if newestTime.IsZero() {
		return time.Hour * 24 // Return max age if no data
	}

	return time.Since(newestTime)
}

// evaluateDivergenceConditions checks if divergence conditions are satisfied
func (fdg *FundingDivergenceGate) evaluateDivergenceConditions(zScore, priceVsVWAP float64) bool {
	// Condition 1: Funding z-score magnitude exceeds threshold
	zScoreCondition := math.Abs(zScore) >= fdg.config.MinZScoreMagnitude

	// Condition 2: Price is sufficiently above VWAP (for negative funding divergence)
	// OR price is sufficiently below VWAP (for positive funding divergence)
	priceCondition := false

	if zScore <= -fdg.config.MinZScoreMagnitude {
		// Negative funding (shorts paying longs) + price above VWAP = bearish divergence
		priceCondition = priceVsVWAP >= fdg.config.PriceVsVWAPThreshold
	} else if zScore >= fdg.config.MinZScoreMagnitude {
		// Positive funding (longs paying shorts) + price below VWAP = bullish divergence
		priceCondition = priceVsVWAP <= (2.0 - fdg.config.PriceVsVWAPThreshold)
	}

	return zScoreCondition && priceCondition
}

// assessSignalQuality evaluates the reliability of the divergence signal
func (fdg *FundingDivergenceGate) assessSignalQuality(fundingResult *derivs.FundingZResult, venueCount int, freshness time.Duration) string {
	qualityScore := 0

	// Venue count quality
	if venueCount >= 4 {
		qualityScore += 2
	} else if venueCount >= 3 {
		qualityScore += 1
	}

	// Data quality from funding calculation
	switch fundingResult.DataQuality {
	case "high":
		qualityScore += 2
	case "medium":
		qualityScore += 1
	}

	// Data freshness quality
	if freshness <= time.Hour {
		qualityScore += 1
	} else if freshness <= time.Hour*2 {
		qualityScore += 0
	} else {
		qualityScore -= 1
	}

	// Map to quality categories
	switch {
	case qualityScore >= 4:
		return "high"
	case qualityScore >= 2:
		return "medium"
	case qualityScore >= 0:
		return "low"
	default:
		return "poor"
	}
}

// getDivergenceFailureReason explains why the gate didn't pass
func (fdg *FundingDivergenceGate) getDivergenceFailureReason(zScore, priceVsVWAP float64) string {
	zScoreMag := math.Abs(zScore)

	if zScoreMag < fdg.config.MinZScoreMagnitude {
		return fmt.Sprintf("funding z-score magnitude %.2f below threshold %.2f",
			zScoreMag, fdg.config.MinZScoreMagnitude)
	}

	if zScore <= -fdg.config.MinZScoreMagnitude && priceVsVWAP < fdg.config.PriceVsVWAPThreshold {
		return fmt.Sprintf("negative funding divergence but price/VWAP %.3f below threshold %.3f",
			priceVsVWAP, fdg.config.PriceVsVWAPThreshold)
	}

	if zScore >= fdg.config.MinZScoreMagnitude && priceVsVWAP > (2.0-fdg.config.PriceVsVWAPThreshold) {
		return fmt.Sprintf("positive funding divergence but price/VWAP %.3f above threshold %.3f",
			priceVsVWAP, 2.0-fdg.config.PriceVsVWAPThreshold)
	}

	return "unknown divergence failure condition"
}

// IsFundingDivergencePresent is a convenience method for simple gate checks
func (fdg *FundingDivergenceGate) IsFundingDivergencePresent(symbol string) (bool, error) {
	result, err := fdg.EvaluateDivergence(symbol)
	if err != nil {
		return false, err
	}

	return result.DivergencePresent, nil
}

// GetDivergenceStrength returns a normalized divergence strength (0-1)
func (fdg *FundingDivergenceGate) GetDivergenceStrength(symbol string) (float64, error) {
	result, err := fdg.EvaluateDivergence(symbol)
	if err != nil {
		return 0, err
	}

	if !result.DivergencePresent {
		return 0, nil
	}

	// Calculate strength based on z-score magnitude and price deviation
	zStrength := math.Min(1.0, math.Abs(result.FundingZScore)/5.0) // Cap at 5 std devs
	priceStrength := math.Abs(result.PriceVsVWAP-1.0) * 2.0        // Price deviation from VWAP

	// Combined strength (weighted average)
	strength := 0.7*zStrength + 0.3*priceStrength

	return math.Min(1.0, strength), nil
}
