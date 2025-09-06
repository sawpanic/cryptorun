package scoring

import (
	"fmt"
	"time"
)

type Regime string

const (
	RegimeTrending Regime = "trending"
	RegimeChoppy   Regime = "choppy"
	RegimeHighVol  Regime = "high_vol"
)

type CompositeScore struct {
	Score float64            `json:"score"`
	Parts map[string]float64 `json:"parts"`
	Meta  ScoreMeta          `json:"meta"`
}

type ScoreMeta struct {
	Timestamp    time.Time `json:"timestamp"`
	Regime       Regime    `json:"regime"`
	Symbol       string    `json:"symbol"`
	Attribution  string    `json:"attribution"`
	IsOrthogonal bool      `json:"is_orthogonal"`
}

type FactorInput struct {
	Symbol    string
	Momentum  MomentumFactors
	Technical TechnicalFactors
	Volume    VolumeFactors
	Quality   QualityFactors
	Social    SocialFactors
}

type MomentumFactors struct {
	Return1h  float64
	Return4h  float64
	Return12h float64
	Return24h float64
	Return7d  float64
	Accel4h   float64
}

type TechnicalFactors struct {
	RSI14     float64
	MACD      float64
	BBWidth   float64
	ATRRatio  float64
}

type VolumeFactors struct {
	VolumeRatio24h float64
	VWAP          float64
	OBV           float64
	VolSpike      float64
}

type QualityFactors struct {
	Spread        float64
	Depth         float64
	VADR          float64
	MarketCap     float64
}

type SocialFactors struct {
	Sentiment     float64
	Mentions      float64
	SocialVolume  float64
	RedditScore   float64
}

type Calculator struct {
	regime       Regime
	momentumCore *MomentumCore
	weights      *RegimeWeightConfig
}

func NewCalculator(regime Regime) *Calculator {
	return &Calculator{
		regime:       regime,
		momentumCore: NewMomentumCore(),
		weights:      NewRegimeWeights(),
	}
}

func (c *Calculator) Calculate(input FactorInput) (*CompositeScore, error) {
	if input.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	meta := ScoreMeta{
		Timestamp:    time.Now(),
		Regime:       c.regime,
		Symbol:       input.Symbol,
		Attribution:  "unified_orthogonal_v1",
		IsOrthogonal: true,
	}

	parts := make(map[string]float64)

	momentumScore := c.momentumCore.Calculate(input.Momentum, c.regime)
	parts["momentum"] = momentumScore

	techResidual := c.calculateTechnicalResidual(input.Technical, momentumScore)
	parts["technical"] = techResidual

	volResidual := c.calculateVolumeResidual(input.Volume, momentumScore, techResidual)
	parts["volume"] = volResidual

	qualityResidual := c.calculateQualityResidual(input.Quality, momentumScore, techResidual, volResidual)
	parts["quality"] = qualityResidual

	socialResidual := c.calculateSocialResidual(input.Social, momentumScore, techResidual, volResidual, qualityResidual)
	parts["social"] = socialResidual

	regimeWeights := c.weights.GetWeights(c.regime)
	
	compositeScore := momentumScore*regimeWeights.Momentum +
		techResidual*regimeWeights.Technical +
		volResidual*regimeWeights.Volume +
		qualityResidual*regimeWeights.Quality

	socialCapped := applySocialCap(socialResidual)
	parts["social_capped"] = socialCapped
	
	finalScore := compositeScore + socialCapped

	return &CompositeScore{
		Score: finalScore,
		Parts: parts,
		Meta:  meta,
	}, nil
}

func (c *Calculator) calculateTechnicalResidual(tech TechnicalFactors, momentum float64) float64 {
	return NewTechnicalResiduals().Calculate(tech, momentum)
}

func (c *Calculator) calculateVolumeResidual(vol VolumeFactors, momentum, technical float64) float64 {
	return NewVolumeResiduals().Calculate(vol, momentum, technical)
}

func (c *Calculator) calculateQualityResidual(qual QualityFactors, momentum, technical, volume float64) float64 {
	return NewQualityResiduals().Calculate(qual, momentum, technical, volume)
}

func (c *Calculator) calculateSocialResidual(soc SocialFactors, momentum, technical, volume, quality float64) float64 {
	return NewSocialResiduals().Calculate(soc, momentum, technical, volume, quality)
}

func applySocialCap(socialScore float64) float64 {
	const socialCap = 10.0
	if socialScore > socialCap {
		return socialCap
	}
	if socialScore < -socialCap {
		return -socialCap
	}
	return socialScore
}