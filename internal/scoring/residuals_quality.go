package scoring

import (
	"math"
)

type QualityResiduals struct {
	weights QualityWeights
}

type QualityWeights struct {
	Spread    float64
	Depth     float64
	VADR      float64
	MarketCap float64
}

func NewQualityResiduals() *QualityResiduals {
	return &QualityResiduals{
		weights: QualityWeights{
			Spread:    0.25,
			Depth:     0.35,
			VADR:      0.30,
			MarketCap: 0.10,
		},
	}
}

func (qr *QualityResiduals) Calculate(factors QualityFactors, momentum, technical, volume float64) float64 {
	rawQuality := factors.Spread*qr.weights.Spread +
		factors.Depth*qr.weights.Depth +
		factors.VADR*qr.weights.VADR +
		factors.MarketCap*qr.weights.MarketCap

	return qr.orthogonalizeMultiple(rawQuality, momentum, technical, volume)
}

func (qr *QualityResiduals) orthogonalizeMultiple(quality, momentum, technical, volume float64) float64 {
	momentumCorr := qr.estimateMomentumCorrelation(quality, momentum)
	technicalCorr := qr.estimateTechnicalCorrelation(quality, technical)
	volumeCorr := qr.estimateVolumeCorrelation(quality, volume)

	momentumProjection := momentumCorr * momentum
	technicalProjection := technicalCorr * technical
	volumeProjection := volumeCorr * volume

	residual := quality - momentumProjection - technicalProjection - volumeProjection

	return math.Max(-30, math.Min(30, residual))
}

func (qr *QualityResiduals) estimateMomentumCorrelation(quality, momentum float64) float64 {
	const baseCorr = 0.10

	if momentum == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(momentum)/30) * 0.05

	return baseCorr + magnitudeAdj
}

func (qr *QualityResiduals) estimateTechnicalCorrelation(quality, technical float64) float64 {
	const baseCorr = 0.20

	if technical == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(technical)/25) * 0.10

	return baseCorr + magnitudeAdj
}

func (qr *QualityResiduals) estimateVolumeCorrelation(quality, volume float64) float64 {
	const baseCorr = 0.30

	if volume == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(volume)/20) * 0.15
	signAdj := 1.0
	if (quality > 0 && volume < 0) || (quality < 0 && volume > 0) {
		signAdj = -0.4
	}

	return (baseCorr + magnitudeAdj) * signAdj
}

func (qr *QualityResiduals) GetWeights() QualityWeights {
	return qr.weights
}
