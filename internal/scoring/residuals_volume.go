package scoring

import (
	"math"
)

type VolumeResiduals struct {
	weights VolumeWeights
}

type VolumeWeights struct {
	VolumeRatio24h float64
	VWAP           float64
	OBV            float64
	VolSpike       float64
}

func NewVolumeResiduals() *VolumeResiduals {
	return &VolumeResiduals{
		weights: VolumeWeights{
			VolumeRatio24h: 0.40,
			VWAP:           0.25,
			OBV:            0.20,
			VolSpike:       0.15,
		},
	}
}

func (vr *VolumeResiduals) Calculate(factors VolumeFactors, momentum, technical float64) float64 {
	rawVolume := factors.VolumeRatio24h*vr.weights.VolumeRatio24h +
		factors.VWAP*vr.weights.VWAP +
		factors.OBV*vr.weights.OBV +
		factors.VolSpike*vr.weights.VolSpike

	return vr.orthogonalizeMultiple(rawVolume, momentum, technical)
}

func (vr *VolumeResiduals) orthogonalizeMultiple(volume, momentum, technical float64) float64 {
	momentumCorr := vr.estimateMomentumCorrelation(volume, momentum)
	technicalCorr := vr.estimateTechnicalCorrelation(volume, technical)

	momentumProjection := momentumCorr * momentum
	technicalProjection := technicalCorr * technical

	residual := volume - momentumProjection - technicalProjection

	return math.Max(-40, math.Min(40, residual))
}

func (vr *VolumeResiduals) estimateMomentumCorrelation(volume, momentum float64) float64 {
	const baseCorr = 0.35

	if momentum == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(momentum)/15) * 0.20
	signAdj := 1.0
	if (volume > 0 && momentum < 0) || (volume < 0 && momentum > 0) {
		signAdj = -0.5
	}

	return (baseCorr + magnitudeAdj) * signAdj
}

func (vr *VolumeResiduals) estimateTechnicalCorrelation(volume, technical float64) float64 {
	const baseCorr = 0.15

	if technical == 0 {
		return 0
	}

	magnitudeAdj := math.Tanh(math.Abs(technical)/25) * 0.10
	signAdj := 1.0
	if (volume > 0 && technical < 0) || (volume < 0 && technical > 0) {
		signAdj = -0.3
	}

	return (baseCorr + magnitudeAdj) * signAdj
}

func (vr *VolumeResiduals) GetWeights() VolumeWeights {
	return vr.weights
}
