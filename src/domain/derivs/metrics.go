package derivs

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// DerivativesMetrics calculates derivatives-based quality signals
type DerivativesMetrics struct {
	config MetricsConfig
}

// MetricsConfig holds configuration for metrics calculation
type MetricsConfig struct {
	FundingLookbackDays int     `yaml:"funding_lookback_days"`
	OIAnalysisHours     int     `yaml:"oi_analysis_hours"`
	MinObservations     int     `yaml:"min_observations"`
	RSquaredThreshold   float64 `yaml:"r_squared_threshold"`
}

// VenueData represents data from a single venue
type VenueData struct {
	VenueName    string
	FundingRates []FundingRatePoint
	Volume       float64 // 24h quote volume for weighting
	OpenInterest []OIPoint
	LastUpdated  time.Time
}

// FundingRatePoint represents a single funding rate observation
type FundingRatePoint struct {
	Rate      float64
	Timestamp time.Time
	MarkPrice float64
}

// OIPoint represents an open interest observation
type OIPoint struct {
	Value     float64
	Price     float64 // Corresponding price for correlation analysis
	Timestamp time.Time
}

// FundingZResult contains funding z-score analysis results
type FundingZResult struct {
	ZScore               float64            `json:"z_score"`
	VolumeWeightedMedian float64            `json:"volume_weighted_median"`
	HistoricalMean       float64            `json:"historical_mean"`
	HistoricalStd        float64            `json:"historical_std"`
	VenueContributions   map[string]float64 `json:"venue_contributions"`
	ValidVenues          int                `json:"valid_venues"`
	DataQuality          string             `json:"data_quality"`
}

// DeltaOIResult contains delta OI residual analysis results
type DeltaOIResult struct {
	Residual      float64 `json:"residual"`
	PriceCorr     float64 `json:"price_correlation"`
	RSquared      float64 `json:"r_squared"`
	Beta          float64 `json:"beta"`
	Alpha         float64 `json:"alpha"`
	Observations  int     `json:"observations"`
	SignalQuality string  `json:"signal_quality"`
}

// BasisDispersionResult contains basis analysis results
type BasisDispersionResult struct {
	Dispersion       float64            `json:"dispersion"`
	CrossVenueSpread float64            `json:"cross_venue_spread"`
	Backwardation    bool               `json:"backwardation"`
	Contango         bool               `json:"contango"`
	VenueBasis       map[string]float64 `json:"venue_basis"`
	Signal           string             `json:"signal"`
}

func NewDerivativesMetrics(config MetricsConfig) *DerivativesMetrics {
	return &DerivativesMetrics{
		config: config,
	}
}

// FundingZ calculates cross-venue funding z-score with volume weighting
func (dm *DerivativesMetrics) FundingZ(venueData []VenueData) (*FundingZResult, error) {
	if len(venueData) == 0 {
		return nil, fmt.Errorf("no venue data provided")
	}

	// Calculate volume-weighted median of current funding rates
	var currentRates []WeightedValue
	totalVolume := 0.0
	venueContribs := make(map[string]float64)

	for _, venue := range venueData {
		if len(venue.FundingRates) == 0 || venue.Volume <= 0 {
			continue
		}

		// Get most recent funding rate
		latest := venue.FundingRates[len(venue.FundingRates)-1]
		currentRates = append(currentRates, WeightedValue{
			Value:  latest.Rate,
			Weight: venue.Volume,
		})

		totalVolume += venue.Volume
		venueContribs[venue.VenueName] = latest.Rate
	}

	if len(currentRates) < 2 {
		return nil, fmt.Errorf("insufficient venues with valid data (need ≥2, got %d)", len(currentRates))
	}

	volumeWeightedMedian := calculateVolumeWeightedMedian(currentRates)

	// Calculate historical statistics for z-score
	historicalRates := dm.collectHistoricalRates(venueData)
	if len(historicalRates) < dm.config.MinObservations {
		return &FundingZResult{
			VolumeWeightedMedian: volumeWeightedMedian,
			ValidVenues:          len(currentRates),
			DataQuality:          "insufficient_history",
		}, nil
	}

	mean := calculateMean(historicalRates)
	std := calculateStdDev(historicalRates, mean)

	var zScore float64
	var dataQuality string

	if std > 0 {
		zScore = (volumeWeightedMedian - mean) / std
		if len(historicalRates) >= dm.config.MinObservations*2 {
			dataQuality = "high"
		} else {
			dataQuality = "medium"
		}
	} else {
		zScore = 0
		dataQuality = "low_variance"
	}

	return &FundingZResult{
		ZScore:               zScore,
		VolumeWeightedMedian: volumeWeightedMedian,
		HistoricalMean:       mean,
		HistoricalStd:        std,
		VenueContributions:   venueContribs,
		ValidVenues:          len(currentRates),
		DataQuality:          dataQuality,
	}, nil
}

// DeltaOIResidual calculates OI change residual after removing price correlation
func (dm *DerivativesMetrics) DeltaOIResidual(oiData []OIPoint) (*DeltaOIResult, error) {
	if len(oiData) < dm.config.MinObservations {
		return nil, fmt.Errorf("insufficient OI data points (need ≥%d, got %d)",
			dm.config.MinObservations, len(oiData))
	}

	// Sort by timestamp for proper delta calculation
	sort.Slice(oiData, func(i, j int) bool {
		return oiData[i].Timestamp.Before(oiData[j].Timestamp)
	})

	// Calculate deltas
	var deltaOI, deltaPrice []float64

	for i := 1; i < len(oiData); i++ {
		prev := oiData[i-1]
		curr := oiData[i]

		// Calculate percentage changes
		if prev.Value > 0 && prev.Price > 0 {
			oiChange := (curr.Value - prev.Value) / prev.Value
			priceChange := (curr.Price - prev.Price) / prev.Price

			deltaOI = append(deltaOI, oiChange)
			deltaPrice = append(deltaPrice, priceChange)
		}
	}

	if len(deltaOI) < dm.config.MinObservations {
		return nil, fmt.Errorf("insufficient valid deltas for OLS regression")
	}

	// Perform OLS regression: deltaOI = alpha + beta * deltaPrice + residual
	beta, alpha, rSquared := performOLS(deltaPrice, deltaOI)

	// Calculate current residual (most recent observation)
	if len(deltaOI) == 0 {
		return nil, fmt.Errorf("no delta observations available")
	}

	latestPriceDelta := deltaPrice[len(deltaPrice)-1]
	latestOIDelta := deltaOI[len(deltaOI)-1]
	predicted := alpha + beta*latestPriceDelta
	residual := latestOIDelta - predicted

	// Calculate price correlation
	priceCorr := calculateCorrelation(deltaPrice, deltaOI)

	// Determine signal quality
	var signalQuality string
	if rSquared >= dm.config.RSquaredThreshold && math.Abs(priceCorr) > 0.3 {
		signalQuality = "high"
	} else if rSquared >= dm.config.RSquaredThreshold/2 {
		signalQuality = "medium"
	} else {
		signalQuality = "low"
	}

	return &DeltaOIResult{
		Residual:      residual,
		PriceCorr:     priceCorr,
		RSquared:      rSquared,
		Beta:          beta,
		Alpha:         alpha,
		Observations:  len(deltaOI),
		SignalQuality: signalQuality,
	}, nil
}

// BasisDispersion analyzes near/far basis dispersion and cross-venue disagreement
func (dm *DerivativesMetrics) BasisDispersion(venueData []VenueData) (*BasisDispersionResult, error) {
	if len(venueData) < 2 {
		return nil, fmt.Errorf("need at least 2 venues for basis dispersion analysis")
	}

	venueBasis := make(map[string]float64)
	var basisValues []float64

	// Calculate basis for each venue (simplified - using mark price vs index)
	// In real implementation, would fetch quarterly vs perpetual prices
	for _, venue := range venueData {
		if len(venue.FundingRates) == 0 {
			continue
		}

		// Simplified basis calculation using funding rate as proxy
		// Real implementation would use: (Future_Price - Index_Price) / Index_Price
		latest := venue.FundingRates[len(venue.FundingRates)-1]
		annualizedBasis := latest.Rate * 365 * 3 // Rough annualization

		venueBasis[venue.VenueName] = annualizedBasis
		basisValues = append(basisValues, annualizedBasis)
	}

	if len(basisValues) < 2 {
		return nil, fmt.Errorf("insufficient venues with basis data")
	}

	// Calculate dispersion metrics
	dispersion := calculateStdDev(basisValues, calculateMean(basisValues))

	// Find min/max for cross-venue spread
	minBasis := basisValues[0]
	maxBasis := basisValues[0]
	for _, basis := range basisValues {
		if basis < minBasis {
			minBasis = basis
		}
		if basis > maxBasis {
			maxBasis = basis
		}
	}
	crossVenueSpread := maxBasis - minBasis

	// Determine market structure signals
	avgBasis := calculateMean(basisValues)
	backwardation := avgBasis < -0.01 // Negative basis > 1%
	contango := avgBasis > 0.05       // Positive basis > 5%

	// Generate signal interpretation
	var signal string
	if dispersion > 0.1 {
		signal = "high_disagreement"
	} else if backwardation {
		signal = "backwardation_stress"
	} else if contango {
		signal = "contango_normal"
	} else {
		signal = "neutral"
	}

	return &BasisDispersionResult{
		Dispersion:       dispersion,
		CrossVenueSpread: crossVenueSpread,
		Backwardation:    backwardation,
		Contango:         contango,
		VenueBasis:       venueBasis,
		Signal:           signal,
	}, nil
}

// Helper functions

type WeightedValue struct {
	Value  float64
	Weight float64
}

func calculateVolumeWeightedMedian(values []WeightedValue) float64 {
	if len(values) == 0 {
		return 0
	}

	if len(values) == 1 {
		return values[0].Value
	}

	// Sort by value
	sort.Slice(values, func(i, j int) bool {
		return values[i].Value < values[j].Value
	})

	// Calculate cumulative weights
	totalWeight := 0.0
	for _, v := range values {
		totalWeight += v.Weight
	}

	targetWeight := totalWeight / 2
	cumWeight := 0.0

	for i, v := range values {
		cumWeight += v.Weight
		if cumWeight >= targetWeight {
			if i == 0 || cumWeight-v.Weight < targetWeight {
				return v.Value
			} else {
				// Interpolate between this and previous value
				prev := values[i-1]
				ratio := (targetWeight - (cumWeight - v.Weight)) / v.Weight
				return prev.Value + ratio*(v.Value-prev.Value)
			}
		}
	}

	return values[len(values)-1].Value
}

func (dm *DerivativesMetrics) collectHistoricalRates(venueData []VenueData) []float64 {
	cutoff := time.Now().AddDate(0, 0, -dm.config.FundingLookbackDays)
	var rates []float64

	for _, venue := range venueData {
		for _, rate := range venue.FundingRates {
			if rate.Timestamp.After(cutoff) {
				rates = append(rates, rate.Rate)
			}
		}
	}

	return rates
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	sumSquaredDiffs := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiffs += diff * diff
	}

	return math.Sqrt(sumSquaredDiffs / float64(len(values)-1))
}

func performOLS(x, y []float64) (beta, alpha, rSquared float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, 0
	}

	// Calculate means
	meanX := calculateMean(x)
	meanY := calculateMean(y)

	// Calculate beta (slope)
	numerator := 0.0
	denominator := 0.0

	for i := range x {
		numerator += (x[i] - meanX) * (y[i] - meanY)
		denominator += (x[i] - meanX) * (x[i] - meanX)
	}

	if denominator == 0 {
		return 0, meanY, 0
	}

	beta = numerator / denominator
	alpha = meanY - beta*meanX

	// Calculate R-squared
	ssRes := 0.0 // Sum of squares of residuals
	ssTot := 0.0 // Total sum of squares

	for i := range y {
		predicted := alpha + beta*x[i]
		ssRes += (y[i] - predicted) * (y[i] - predicted)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}

	if ssTot == 0 {
		rSquared = 0
	} else {
		rSquared = 1 - ssRes/ssTot
	}

	return beta, alpha, rSquared
}

func calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	meanX := calculateMean(x)
	meanY := calculateMean(y)

	numerator := 0.0
	sumXSq := 0.0
	sumYSq := 0.0

	for i := range x {
		xDiff := x[i] - meanX
		yDiff := y[i] - meanY

		numerator += xDiff * yDiff
		sumXSq += xDiff * xDiff
		sumYSq += yDiff * yDiff
	}

	denominator := math.Sqrt(sumXSq * sumYSq)
	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}
