package scoring

import (
	"math"
)

type MomentumCore struct {
	weights MomentumWeights
}

type MomentumWeights struct {
	Return1h  float64
	Return4h  float64
	Return12h float64
	Return24h float64
	Return7d  float64
}

func NewMomentumCore() *MomentumCore {
	return &MomentumCore{
		weights: getMomentumWeights(),
	}
}

func getMomentumWeights() MomentumWeights {
	return MomentumWeights{
		Return1h:  0.20, // 20%
		Return4h:  0.35, // 35%
		Return12h: 0.30, // 30%
		Return24h: 0.15, // 15%
		Return7d:  0.00, // 0% (only in trending regimes)
	}
}

func getMomentumWeightsTrending() MomentumWeights {
	return MomentumWeights{
		Return1h:  0.15, // 15%
		Return4h:  0.30, // 30%
		Return12h: 0.25, // 25%
		Return24h: 0.20, // 20%
		Return7d:  0.10, // 10% (weekly component in trending)
	}
}

func (mc *MomentumCore) Calculate(factors MomentumFactors, regime Regime) float64 {
	weights := mc.getRegimeWeights(regime)

	momentumScore := factors.Return1h*weights.Return1h +
		factors.Return4h*weights.Return4h +
		factors.Return12h*weights.Return12h +
		factors.Return24h*weights.Return24h +
		factors.Return7d*weights.Return7d

	accelBoost := mc.calculateAccelerationBoost(factors.Accel4h)

	return momentumScore + accelBoost
}

func (mc *MomentumCore) getRegimeWeights(regime Regime) MomentumWeights {
	switch regime {
	case RegimeTrending:
		return getMomentumWeightsTrending()
	case RegimeChoppy, RegimeHighVol:
		return getMomentumWeights()
	default:
		return getMomentumWeights()
	}
}

func (mc *MomentumCore) calculateAccelerationBoost(accel4h float64) float64 {
	const maxBoost = 2.0

	if accel4h == 0 {
		return 0
	}

	absAccel := math.Abs(accel4h)
	normalizedAccel := math.Tanh(absAccel / 5.0)

	boost := normalizedAccel * maxBoost
	if accel4h < 0 {
		boost = -boost
	}

	return boost
}

func (mc *MomentumCore) GetWeightSum(regime Regime) float64 {
	weights := mc.getRegimeWeights(regime)
	return weights.Return1h + weights.Return4h + weights.Return12h + weights.Return24h + weights.Return7d
}

func (mc *MomentumCore) IsProtected() bool {
	return true
}
