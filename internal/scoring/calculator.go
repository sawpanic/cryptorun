package scoring

import "time"

// Regime represents market regime types
type Regime int

const (
	RegimeChoppy Regime = iota
	RegimeTrending
	RegimeVolatile
)

// Calculator provides scoring functionality
type Calculator struct {
	regime Regime
}

// NewCalculator creates a new scoring calculator
func NewCalculator(regime Regime) *Calculator {
	return &Calculator{regime: regime}
}

// CompositeScore represents a composite scoring result
type CompositeScore struct {
	Score float64
	Parts map[string]float64
	Meta  ScoreMeta
}

// ScoreMeta contains scoring metadata
type ScoreMeta struct {
	Regime    Regime
	Timestamp time.Time
}

// FactorInput represents input factors for scoring
type FactorInput struct {
	Symbol    string
	Momentum  MomentumFactors
	Technical TechnicalFactors
	Volume    VolumeFactors
	Quality   QualityFactors
	Social    SocialFactors
}

// MomentumFactors represents momentum-based factors
type MomentumFactors struct {
	Return1h  float64
	Return4h  float64
	Return12h float64
	Return24h float64
	Return7d  float64
	Accel4h   float64
}

// TechnicalFactors represents technical indicators
type TechnicalFactors struct {
	RSI14    float64
	MACD     float64
	BBWidth  float64
	ATRRatio float64
}

// VolumeFactors represents volume-based factors
type VolumeFactors struct {
	VolumeRatio24h float64
	VWAP           float64
	OBV            float64
	VolSpike       float64
}

// QualityFactors represents quality metrics
type QualityFactors struct {
	Spread    float64
	Depth     float64
	VADR      float64
	MarketCap float64
}

// SocialFactors represents social sentiment factors
type SocialFactors struct {
	Sentiment    float64
	Mentions     float64
	SocialVolume float64
	RedditScore  float64
}

// Calculate performs composite scoring calculation
func (c *Calculator) Calculate(input FactorInput) (*CompositeScore, error) {
	// Mock calculation - in production would perform real composite scoring
	momentumScore := (input.Momentum.Return1h + input.Momentum.Return4h + input.Momentum.Return12h + input.Momentum.Return24h) / 4
	technicalScore := input.Technical.RSI14 - 50
	volumeScore := (input.Volume.VolumeRatio24h - 1) * 50
	qualityScore := (input.Quality.VADR - 1) * 25
	socialScore := input.Social.Sentiment * 10

	totalScore := momentumScore + technicalScore + volumeScore + qualityScore + socialScore

	return &CompositeScore{
		Score: totalScore,
		Parts: map[string]float64{
			"momentum":  momentumScore,
			"technical": technicalScore,
			"volume":    volumeScore,
			"quality":   qualityScore,
			"social":    socialScore,
		},
		Meta: ScoreMeta{
			Regime:    c.regime,
			Timestamp: time.Now(),
		},
	}, nil
}