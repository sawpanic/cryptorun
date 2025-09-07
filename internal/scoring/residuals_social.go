package scoring

import (
	"math"
)

type SocialResiduals struct {
	weights SocialWeights
	cap     float64
}

type SocialWeights struct {
	Sentiment    float64
	Mentions     float64
	SocialVolume float64
	RedditScore  float64
}

func NewSocialResiduals() *SocialResiduals {
	return &SocialResiduals{
		weights: SocialWeights{
			Sentiment:    0.30,
			Mentions:     0.25,
			SocialVolume: 0.25,
			RedditScore:  0.20,
		},
		cap: 10.0,
	}
}

func (sr *SocialResiduals) Calculate(factors SocialFactors, momentum, technical, volume, quality float64) float64 {
	rawSocial := factors.Sentiment*sr.weights.Sentiment +
		factors.Mentions*sr.weights.Mentions +
		factors.SocialVolume*sr.weights.SocialVolume +
		factors.RedditScore*sr.weights.RedditScore

	orthogonalized := sr.orthogonalizeMultiple(rawSocial, momentum, technical, volume, quality)

	return sr.applyCap(orthogonalized)
}

func (sr *SocialResiduals) orthogonalizeMultiple(social, momentum, technical, volume, quality float64) float64 {
	momentumCorr := sr.estimateMomentumCorrelation(social, momentum)
	technicalCorr := sr.estimateTechnicalCorrelation(social, technical)
	volumeCorr := sr.estimateVolumeCorrelation(social, volume)
	qualityCorr := sr.estimateQualityCorrelation(social, quality)

	momentumProjection := momentumCorr * momentum
	technicalProjection := technicalCorr * technical
	volumeProjection := volumeCorr * volume
	qualityProjection := qualityCorr * quality

	residual := social - momentumProjection - technicalProjection - volumeProjection - qualityProjection

	return residual
}

func (sr *SocialResiduals) estimateMomentumCorrelation(social, momentum float64) float64 {
	const baseCorr = 0.40

	if momentum == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(momentum)/15) * 0.25
	signAdj := 1.0
	if (social > 0 && momentum < 0) || (social < 0 && momentum > 0) {
		signAdj = -0.3
	}

	return (baseCorr + magnitudeAdj) * signAdj
}

func (sr *SocialResiduals) estimateTechnicalCorrelation(social, technical float64) float64 {
	const baseCorr = 0.15

	if technical == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(technical)/25) * 0.10

	return baseCorr + magnitudeAdj
}

func (sr *SocialResiduals) estimateVolumeCorrelation(social, volume float64) float64 {
	const baseCorr = 0.50

	if volume == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(volume)/20) * 0.20
	signAdj := 1.0
	if (social > 0 && volume < 0) || (social < 0 && volume > 0) {
		signAdj = -0.2
	}

	return (baseCorr + magnitudeAdj) * signAdj
}

func (sr *SocialResiduals) estimateQualityCorrelation(social, quality float64) float64 {
	const baseCorr = 0.05

	if quality == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(quality)/30) * 0.05

	return baseCorr + magnitudeAdj
}

func (sr *SocialResiduals) applyCap(socialScore float64) float64 {
	if socialScore > sr.cap {
		return sr.cap
	}
	if socialScore < -sr.cap {
		return -sr.cap
	}
	return socialScore
}

func (sr *SocialResiduals) GetCap() float64 {
	return sr.cap
}

func (sr *SocialResiduals) GetWeights() SocialWeights {
	return sr.weights
}
