package domain

// Regime stub functions for build compatibility
type RegimeWeights struct {
	Momentum1h  float64
	Momentum4h  float64
	Momentum12h float64
	Momentum24h float64
	Momentum7d  float64
}

func GetRegimeWeights(regime string) RegimeWeights {
	return RegimeWeights{
		Momentum1h: 0.2, Momentum4h: 0.35, Momentum12h: 0.3, 
		Momentum24h: 0.1, Momentum7d: 0.05,
	}
}

type RegimeInputs struct {
	RealizedVol7d float64
	PctAbove20MA  float64  
	BreadthThrust float64
}

type RegimeDetector struct{}

func (r *RegimeDetector) DetectRegime(inputs RegimeInputs) string {
	return "bull" // stub
}

func NewRegimeDetector() *RegimeDetector { 
	return &RegimeDetector{} 
}
