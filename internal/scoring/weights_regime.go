package scoring

import (
	"fmt"
)

type RegimeWeights struct {
	Momentum  float64 `json:"momentum"`
	Technical float64 `json:"technical"`
	Volume    float64 `json:"volume"`
	Quality   float64 `json:"quality"`
}

type RegimeWeightConfig struct {
	trending RegimeWeights
	choppy   RegimeWeights
	highVol  RegimeWeights
}

func NewRegimeWeights() *RegimeWeightConfig {
	return &RegimeWeightConfig{
		trending: RegimeWeights{
			Momentum:  0.55, // 55%
			Technical: 0.25, // 25%
			Volume:    0.15, // 15%
			Quality:   0.05, // 5%
		},
		choppy: RegimeWeights{
			Momentum:  0.40, // 40%
			Technical: 0.35, // 35%
			Volume:    0.15, // 15%
			Quality:   0.10, // 10%
		},
		highVol: RegimeWeights{
			Momentum:  0.30, // 30%
			Technical: 0.30, // 30%
			Volume:    0.25, // 25%
			Quality:   0.15, // 15%
		},
	}
}

func (rwc *RegimeWeightConfig) GetWeights(regime Regime) RegimeWeights {
	switch regime {
	case RegimeTrending:
		return rwc.trending
	case RegimeChoppy:
		return rwc.choppy
	case RegimeHighVol:
		return rwc.highVol
	default:
		return rwc.choppy
	}
}

func (rwc *RegimeWeightConfig) ValidateWeights() error {
	regimes := []struct {
		name    string
		weights RegimeWeights
	}{
		{"trending", rwc.trending},
		{"choppy", rwc.choppy},
		{"highVol", rwc.highVol},
	}

	for _, regime := range regimes {
		sum := regime.weights.Momentum + regime.weights.Technical + regime.weights.Volume + regime.weights.Quality
		if sum < 0.99 || sum > 1.01 {
			return fmt.Errorf("regime %s weights sum to %.3f, expected 1.000", regime.name, sum)
		}
	}

	return nil
}

func (rw RegimeWeights) Sum() float64 {
	return rw.Momentum + rw.Technical + rw.Volume + rw.Quality
}

func (rwc *RegimeWeightConfig) GetAllRegimes() map[Regime]RegimeWeights {
	return map[Regime]RegimeWeights{
		RegimeTrending: rwc.trending,
		RegimeChoppy:   rwc.choppy,
		RegimeHighVol:  rwc.highVol,
	}
}

func (rwc *RegimeWeightConfig) GetRegimeDescription(regime Regime) string {
	switch regime {
	case RegimeTrending:
		return "Trending market with clear directional momentum - momentum factors dominate"
	case RegimeChoppy:
		return "Choppy/sideways market - balanced between momentum and technical factors"
	case RegimeHighVol:
		return "High volatility market - quality and volume factors more important"
	default:
		return "Unknown regime"
	}
}
