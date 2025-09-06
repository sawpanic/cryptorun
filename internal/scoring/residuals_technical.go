package scoring

import (
	"math"
)

type TechnicalResiduals struct {
	weights TechnicalWeights
}

type TechnicalWeights struct {
	RSI14    float64
	MACD     float64
	BBWidth  float64
	ATRRatio float64
}

func NewTechnicalResiduals() *TechnicalResiduals {
	return &TechnicalResiduals{
		weights: TechnicalWeights{
			RSI14:    0.30,
			MACD:     0.35,
			BBWidth:  0.20,
			ATRRatio: 0.15,
		},
	}
}

func (tr *TechnicalResiduals) Calculate(factors TechnicalFactors, momentumScore float64) float64 {
	rawTechnical := factors.RSI14*tr.weights.RSI14 +
		factors.MACD*tr.weights.MACD +
		factors.BBWidth*tr.weights.BBWidth +
		factors.ATRRatio*tr.weights.ATRRatio

	return tr.orthogonalize(rawTechnical, momentumScore)
}

func (tr *TechnicalResiduals) orthogonalize(technical, momentum float64) float64 {
	correlation := tr.estimateCorrelation(technical, momentum)
	
	projection := correlation * momentum
	residual := technical - projection
	
	return math.Max(-50, math.Min(50, residual))
}

func (tr *TechnicalResiduals) estimateCorrelation(technical, momentum float64) float64 {
	const baseCorr = 0.25
	
	if momentum == 0 {
		return 0
	}
	
	magnitudeAdj := math.Tanh(math.Abs(momentum)/20) * 0.15
	signAdj := 1.0
	if (technical > 0 && momentum < 0) || (technical < 0 && momentum > 0) {
		signAdj = -1.0
	}
	
	return (baseCorr + magnitudeAdj) * signAdj
}

func (tr *TechnicalResiduals) GetWeights() TechnicalWeights {
	return tr.weights
}