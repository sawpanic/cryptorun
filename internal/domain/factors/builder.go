package factors

import (
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/internal/config/regime"
	"github.com/sawpanic/cryptorun/internal/domain/indicators"
)

// FactorData represents raw data needed for factor calculation
type FactorData struct {
	Symbol          string                         `json:"symbol"`
	CurrentPrice    float64                        `json:"current_price"`
	PriceHistory    []float64                      `json:"price_history"`    // For momentum calculations
	VolumeHistory   []float64                      `json:"volume_history"`   // For volume factors
	OHLCHistory     []indicators.PriceBar          `json:"ohlc_history"`     // For technical indicators
	TechnicalData   indicators.TechnicalIndicators `json:"technical_data"`   // Pre-calculated indicators
	FundingRate     float64                        `json:"funding_rate"`     // Current funding rate
	OpenInterest    float64                        `json:"open_interest"`    // Current OI
	SocialScore     float64                        `json:"social_score"`     // Aggregated social sentiment
	QualityScore    float64                        `json:"quality_score"`    // Fundamental quality metrics
	MarketCap       float64                        `json:"market_cap"`       // Market capitalization
	Volume24h       float64                        `json:"volume_24h"`       // 24h volume
	Timestamp       time.Time                      `json:"timestamp"`
}

// RawFactorRow represents the raw, unorthogonalized factor values
type RawFactorRow struct {
	Symbol           string    `json:"symbol"`
	MomentumCore     float64   `json:"momentum_core"`     // Protected - never orthogonalized
	TechnicalFactor  float64   `json:"technical_factor"`  // Raw technical score
	VolumeFactor     float64   `json:"volume_factor"`     // Raw volume score
	QualityFactor    float64   `json:"quality_factor"`    // Raw quality score
	SocialFactor     float64   `json:"social_factor"`     // Raw social score
	Timestamp        time.Time `json:"timestamp"`
	
	// Metadata for debugging and explainability
	FactorDetails FactorDetails `json:"factor_details"`
}

// OrthogonalizedFactorRow represents factors after Gram-Schmidt orthogonalization
type OrthogonalizedFactorRow struct {
	Symbol              string    `json:"symbol"`
	MomentumCore        float64   `json:"momentum_core"`        // Always preserved
	TechnicalResidual   float64   `json:"technical_residual"`   // Orthogonalized vs MomentumCore
	VolumeResidual      float64   `json:"volume_residual"`      // Orthogonalized vs Momentum + Technical
	QualityResidual     float64   `json:"quality_residual"`     // Orthogonalized vs all previous
	SocialResidual      float64   `json:"social_residual"`      // Orthogonalized vs all previous + capped
	SocialCapped        float64   `json:"social_capped"`        // Final social contribution after hard cap
	Timestamp           time.Time `json:"timestamp"`
	
	// Orthogonalization metadata
	OrthogonalizationInfo OrthogonalizationInfo `json:"orthogonalization_info"`
}

// FactorDetails provides detailed breakdown of factor calculations
type FactorDetails struct {
	MomentumBreakdown MomentumBreakdown `json:"momentum_breakdown"`
	TechnicalInputs   TechnicalInputs   `json:"technical_inputs"`
	VolumeInputs      VolumeInputs      `json:"volume_inputs"`
	QualityInputs     QualityInputs     `json:"quality_inputs"`
	SocialInputs      SocialInputs      `json:"social_inputs"`
}

// MomentumBreakdown shows how momentum core is calculated
type MomentumBreakdown struct {
	Momentum1h  float64 `json:"momentum_1h"`  // 1h momentum (20% weight)
	Momentum4h  float64 `json:"momentum_4h"`  // 4h momentum (35% weight)
	Momentum12h float64 `json:"momentum_12h"` // 12h momentum (30% weight)
	Momentum24h float64 `json:"momentum_24h"` // 24h momentum (15% weight)
	Composite   float64 `json:"composite"`    // Weighted composite
}

// TechnicalInputs shows technical indicator inputs
type TechnicalInputs struct {
	RSI        float64 `json:"rsi"`
	ADX        float64 `json:"adx"`
	HurstExp   float64 `json:"hurst_exponent"`
	ATRPercent float64 `json:"atr_percent"`
	RawScore   float64 `json:"raw_score"`
}

// VolumeInputs shows volume analysis inputs
type VolumeInputs struct {
	CurrentVolume   float64 `json:"current_volume"`
	AverageVolume   float64 `json:"average_volume"`
	VolumeRatio     float64 `json:"volume_ratio"`
	VolumeSurge     bool    `json:"volume_surge"`
	RelativeVolume  float64 `json:"relative_volume"`
}

// QualityInputs shows fundamental quality inputs
type QualityInputs struct {
	MarketCapRank   int     `json:"market_cap_rank"`
	LiquidityScore  float64 `json:"liquidity_score"`
	StabilityScore  float64 `json:"stability_score"`
	CompositeScore  float64 `json:"composite_score"`
}

// SocialInputs shows social sentiment inputs
type SocialInputs struct {
	TwitterSentiment  float64 `json:"twitter_sentiment"`
	RedditSentiment   float64 `json:"reddit_sentiment"`
	NewsScore         float64 `json:"news_score"`
	InfluencerScore   float64 `json:"influencer_score"`
	AggregatedScore   float64 `json:"aggregated_score"`
	PreCapScore       float64 `json:"pre_cap_score"`
	PostCapScore      float64 `json:"post_cap_score"`
}

// OrthogonalizationInfo provides metadata about the orthogonalization process
type OrthogonalizationInfo struct {
	CorrelationMatrix    map[string]map[string]float64 `json:"correlation_matrix"`
	ProjectionMagnitudes map[string]float64            `json:"projection_magnitudes"`
	ResidualizationOrder []string                      `json:"residualization_order"`
	QualityMetrics       QualityMetrics                `json:"quality_metrics"`
}

// QualityMetrics tracks orthogonalization quality
type QualityMetrics struct {
	MaxCorrelation      float64 `json:"max_correlation"`       // Highest correlation after orthogonalization
	MomentumPreserved   float64 `json:"momentum_preserved"`    // % of momentum variance preserved
	TotalVarianceKept   float64 `json:"total_variance_kept"`   // % of total variance retained
	OrthogonalityScore  float64 `json:"orthogonality_score"`   // Overall orthogonality (0-100)
}

// FactorBuilder constructs factor rows from market data
type FactorBuilder struct {
	config regime.WeightsConfig
}

// NewFactorBuilder creates a new factor builder with configuration
func NewFactorBuilder(config regime.WeightsConfig) *FactorBuilder {
	return &FactorBuilder{config: config}
}

// BuildRawFactorRow calculates raw factors before orthogonalization
func (fb *FactorBuilder) BuildRawFactorRow(data FactorData) (RawFactorRow, error) {
	if data.CurrentPrice <= 0 {
		return RawFactorRow{}, fmt.Errorf("invalid current price: %.2f", data.CurrentPrice)
	}

	// 1. Calculate Momentum Core (protected factor)
	momentumCore, momentumDetails := fb.calculateMomentumCore(data.PriceHistory, data.CurrentPrice)

	// 2. Calculate Technical Factor
	technicalFactor, technicalInputs := fb.calculateTechnicalFactor(data.TechnicalData, data.CurrentPrice)

	// 3. Calculate Volume Factor
	volumeFactor, volumeInputs := fb.calculateVolumeFactor(data.VolumeHistory, data.Volume24h)

	// 4. Calculate Quality Factor
	qualityFactor, qualityInputs := fb.calculateQualityFactor(data.MarketCap, data.Volume24h, data.CurrentPrice)

	// 5. Calculate Social Factor (raw, before capping)
	socialFactor, socialInputs := fb.calculateSocialFactor(data.SocialScore)

	return RawFactorRow{
		Symbol:          data.Symbol,
		MomentumCore:    momentumCore,
		TechnicalFactor: technicalFactor,
		VolumeFactor:    volumeFactor,
		QualityFactor:   qualityFactor,
		SocialFactor:    socialFactor,
		Timestamp:       data.Timestamp,
		FactorDetails: FactorDetails{
			MomentumBreakdown: momentumDetails,
			TechnicalInputs:   technicalInputs,
			VolumeInputs:      volumeInputs,
			QualityInputs:     qualityInputs,
			SocialInputs:      socialInputs,
		},
	}, nil
}

// calculateMomentumCore calculates the protected momentum factor
func (fb *FactorBuilder) calculateMomentumCore(priceHistory []float64, currentPrice float64) (float64, MomentumBreakdown) {
	breakdown := MomentumBreakdown{}

	if len(priceHistory) < 24 { // Need at least 24 hours of hourly data
		return 0.0, breakdown
	}

	// Calculate momentum for different timeframes
	// Assuming hourly price data
	if len(priceHistory) >= 1 {
		price1hAgo := priceHistory[len(priceHistory)-1]
		if price1hAgo > 0 {
			breakdown.Momentum1h = ((currentPrice - price1hAgo) / price1hAgo) * 100.0
		}
	}

	if len(priceHistory) >= 4 {
		price4hAgo := priceHistory[len(priceHistory)-4]
		if price4hAgo > 0 {
			breakdown.Momentum4h = ((currentPrice - price4hAgo) / price4hAgo) * 100.0
		}
	}

	if len(priceHistory) >= 12 {
		price12hAgo := priceHistory[len(priceHistory)-12]
		if price12hAgo > 0 {
			breakdown.Momentum12h = ((currentPrice - price12hAgo) / price12hAgo) * 100.0
		}
	}

	if len(priceHistory) >= 24 {
		price24hAgo := priceHistory[len(priceHistory)-24]
		if price24hAgo > 0 {
			breakdown.Momentum24h = ((currentPrice - price24hAgo) / price24hAgo) * 100.0
		}
	}

	// Weighted composite: 1h=20%, 4h=35%, 12h=30%, 24h=15%
	breakdown.Composite = breakdown.Momentum1h*0.20 + breakdown.Momentum4h*0.35 + breakdown.Momentum12h*0.30 + breakdown.Momentum24h*0.15

	return breakdown.Composite, breakdown
}

// calculateTechnicalFactor calculates technical indicator score
func (fb *FactorBuilder) calculateTechnicalFactor(technicalIndicators indicators.TechnicalIndicators, currentPrice float64) (float64, TechnicalInputs) {
	inputs := TechnicalInputs{}

	if technicalIndicators.RSI.IsValid {
		inputs.RSI = technicalIndicators.RSI.Value
	}

	if technicalIndicators.ADX.IsValid {
		inputs.ADX = technicalIndicators.ADX.ADX
	}

	if technicalIndicators.Hurst.IsValid {
		inputs.HurstExp = technicalIndicators.Hurst.Exponent
	}

	if technicalIndicators.ATR.IsValid && currentPrice > 0 {
		inputs.ATRPercent = (technicalIndicators.ATR.Value / currentPrice) * 100.0
	}

	// Get technical score from indicators package
	inputs.RawScore = indicators.GetTechnicalScore(technicalIndicators, currentPrice)

	return inputs.RawScore, inputs
}

// calculateVolumeFactor calculates volume-based score
func (fb *FactorBuilder) calculateVolumeFactor(volumeHistory []float64, volume24h float64) (float64, VolumeInputs) {
	inputs := VolumeInputs{
		CurrentVolume: volume24h,
	}

	if len(volumeHistory) == 0 || volume24h <= 0 {
		return 0.0, inputs
	}

	// Calculate average volume (last 7 days if available)
	avgDays := 7
	if len(volumeHistory) < avgDays {
		avgDays = len(volumeHistory)
	}

	avgVolume := 0.0
	for i := len(volumeHistory) - avgDays; i < len(volumeHistory); i++ {
		avgVolume += volumeHistory[i]
	}
	avgVolume /= float64(avgDays)
	inputs.AverageVolume = avgVolume

	if avgVolume > 0 {
		inputs.VolumeRatio = volume24h / avgVolume
		inputs.VolumeSurge = inputs.VolumeRatio >= 1.75 // Volume surge threshold
	}

	inputs.RelativeVolume = inputs.VolumeRatio

	// Volume score: higher for volume surges, normalized to 0-100
	score := 0.0
	if inputs.VolumeRatio >= 3.0 {
		score = 100.0
	} else if inputs.VolumeRatio >= 2.0 {
		score = 80.0
	} else if inputs.VolumeRatio >= 1.75 {
		score = 60.0
	} else if inputs.VolumeRatio >= 1.5 {
		score = 40.0
	} else if inputs.VolumeRatio >= 1.2 {
		score = 20.0
	} else {
		score = 10.0
	}

	return score, inputs
}

// calculateQualityFactor calculates fundamental quality score
func (fb *FactorBuilder) calculateQualityFactor(marketCap, volume24h, currentPrice float64) (float64, QualityInputs) {
	inputs := QualityInputs{}

	// Market cap ranking (simplified)
	if marketCap >= 100_000_000_000 { // Top 10 coins
		inputs.MarketCapRank = 1
	} else if marketCap >= 10_000_000_000 { // Top 50 coins
		inputs.MarketCapRank = 2
	} else if marketCap >= 1_000_000_000 { // Top 200 coins
		inputs.MarketCapRank = 3
	} else {
		inputs.MarketCapRank = 4
	}

	// Liquidity score based on volume
	if volume24h >= 1_000_000_000 {
		inputs.LiquidityScore = 100.0
	} else if volume24h >= 100_000_000 {
		inputs.LiquidityScore = 80.0
	} else if volume24h >= 10_000_000 {
		inputs.LiquidityScore = 60.0
	} else {
		inputs.LiquidityScore = 30.0
	}

	// Stability score (simplified - would need historical volatility data)
	inputs.StabilityScore = 50.0 // Placeholder

	// Composite quality score
	marketCapWeight := 0.4
	liquidityWeight := 0.4
	stabilityWeight := 0.2

	marketCapScore := float64(5-inputs.MarketCapRank) * 25.0 // Convert rank to score
	inputs.CompositeScore = marketCapScore*marketCapWeight + inputs.LiquidityScore*liquidityWeight + inputs.StabilityScore*stabilityWeight

	return inputs.CompositeScore, inputs
}

// calculateSocialFactor calculates social sentiment score (before capping)
func (fb *FactorBuilder) calculateSocialFactor(socialScore float64) (float64, SocialInputs) {
	inputs := SocialInputs{
		AggregatedScore: socialScore,
		PreCapScore:     socialScore,
	}

	// For now, use the aggregated score directly
	// In a full implementation, this would break down into components:
	inputs.TwitterSentiment = socialScore * 0.4  // 40% Twitter
	inputs.RedditSentiment = socialScore * 0.3   // 30% Reddit  
	inputs.NewsScore = socialScore * 0.2         // 20% News
	inputs.InfluencerScore = socialScore * 0.1   // 10% Influencers

	// Apply preliminary normalization (0-100 scale)
	normalizedScore := math.Max(0, math.Min(100, socialScore))
	inputs.PreCapScore = normalizedScore

	// Hard cap will be applied during orthogonalization
	inputs.PostCapScore = normalizedScore

	return normalizedScore, inputs
}

// BuildFactorRowBatch builds multiple raw factor rows efficiently
func (fb *FactorBuilder) BuildFactorRowBatch(dataList []FactorData) ([]RawFactorRow, []error) {
	results := make([]RawFactorRow, len(dataList))
	errors := make([]error, len(dataList))

	for i, data := range dataList {
		row, err := fb.BuildRawFactorRow(data)
		results[i] = row
		errors[i] = err
	}

	return results, errors
}

// ValidateFactorRow checks if a factor row has reasonable values
func ValidateFactorRow(row RawFactorRow) error {
	// Check for NaN or infinite values
	factors := []float64{
		row.MomentumCore,
		row.TechnicalFactor,
		row.VolumeFactor,
		row.QualityFactor,
		row.SocialFactor,
	}

	factorNames := []string{
		"MomentumCore",
		"TechnicalFactor", 
		"VolumeFactor",
		"QualityFactor",
		"SocialFactor",
	}

	for i, factor := range factors {
		if math.IsNaN(factor) {
			return fmt.Errorf("%s is NaN", factorNames[i])
		}
		if math.IsInf(factor, 0) {
			return fmt.Errorf("%s is infinite", factorNames[i])
		}
	}

	// Check reasonable ranges (allowing for outliers)
	if math.Abs(row.MomentumCore) > 1000 { // ±1000% seems like a reasonable extreme
		return fmt.Errorf("MomentumCore %.2f%% exceeds reasonable range", row.MomentumCore)
	}

	if row.TechnicalFactor < 0 || row.TechnicalFactor > 100 {
		return fmt.Errorf("TechnicalFactor %.2f outside 0-100 range", row.TechnicalFactor)
	}

	if row.VolumeFactor < 0 || row.VolumeFactor > 100 {
		return fmt.Errorf("VolumeFactor %.2f outside 0-100 range", row.VolumeFactor)
	}

	if row.QualityFactor < 0 || row.QualityFactor > 100 {
		return fmt.Errorf("QualityFactor %.2f outside 0-100 range", row.QualityFactor)
	}

	if row.SocialFactor < 0 || row.SocialFactor > 100 {
		return fmt.Errorf("SocialFactor %.2f outside 0-100 range", row.SocialFactor)
	}

	return nil
}