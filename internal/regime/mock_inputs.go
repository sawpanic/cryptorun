package regime

import (
	"context"
	"math/rand"
	"time"
)

// MockDetectorInputs provides simulated market data for regime detection testing
type MockDetectorInputs struct {
	currentTime       time.Time
	realizedVol7d     float64
	breadthAbove20MA  float64
	breadthThrustADX  float64
	volatilityTrend   string // "increasing", "decreasing", "stable"
	simulateVariation bool
}

// NewMockDetectorInputs creates mock inputs with realistic market data patterns
func NewMockDetectorInputs() *MockDetectorInputs {
	return &MockDetectorInputs{
		currentTime:       time.Now(),
		realizedVol7d:     0.20, // 20% default volatility
		breadthAbove20MA:  0.65, // 65% of assets above 20MA (trending)
		breadthThrustADX:  0.72, // 72% thrust (strong directional movement)
		volatilityTrend:   "stable",
		simulateVariation: true,
	}
}

// NewMockDetectorInputsForRegime creates mock inputs that will result in a specific regime
func NewMockDetectorInputsForRegime(targetRegime Regime) *MockDetectorInputs {
	inputs := &MockDetectorInputs{
		currentTime:       time.Now(),
		simulateVariation: false, // Fixed values for deterministic regime
	}

	switch targetRegime {
	case TrendingBull:
		inputs.realizedVol7d = 0.18    // Low volatility (below 25% threshold)
		inputs.breadthAbove20MA = 0.72 // Strong breadth (above 60% threshold)
		inputs.breadthThrustADX = 0.75 // Strong thrust (above 70% threshold)
		inputs.volatilityTrend = "stable"
	case Choppy:
		inputs.realizedVol7d = 0.22    // Moderate volatility (below 25% threshold)
		inputs.breadthAbove20MA = 0.45 // Weak breadth (below 60% threshold)
		inputs.breadthThrustADX = 0.55 // Weak thrust (below 70% threshold)
		inputs.volatilityTrend = "stable"
	case HighVol:
		inputs.realizedVol7d = 0.32    // High volatility (above 25% threshold)
		inputs.breadthAbove20MA = 0.40 // Weak breadth due to volatility
		inputs.breadthThrustADX = 0.65 // Moderate thrust but volatile
		inputs.volatilityTrend = "increasing"
	}

	return inputs
}

// GetRealizedVolatility7d returns simulated 7-day realized volatility
func (m *MockDetectorInputs) GetRealizedVolatility7d(ctx context.Context) (float64, error) {
	if m.simulateVariation {
		// Add some realistic variation (±5% from base)
		variation := (rand.Float64() - 0.5) * 0.10 // ±5% variation
		return m.realizedVol7d + variation, nil
	}
	return m.realizedVol7d, nil
}

// GetBreadthAbove20MA returns simulated percentage of assets above 20-day MA
func (m *MockDetectorInputs) GetBreadthAbove20MA(ctx context.Context) (float64, error) {
	if m.simulateVariation {
		// Add variation based on volatility trend
		variation := 0.0
		switch m.volatilityTrend {
		case "increasing":
			variation = (rand.Float64() - 0.7) * 0.15 // Bias toward lower breadth in volatility
		case "decreasing":
			variation = (rand.Float64() - 0.3) * 0.15 // Bias toward higher breadth in calm
		default:
			variation = (rand.Float64() - 0.5) * 0.10 // Neutral variation
		}
		result := m.breadthAbove20MA + variation
		if result < 0.0 {
			result = 0.0
		}
		if result > 1.0 {
			result = 1.0
		}
		return result, nil
	}
	return m.breadthAbove20MA, nil
}

// GetBreadthThrustADXProxy returns simulated breadth thrust using ADX proxy
func (m *MockDetectorInputs) GetBreadthThrustADXProxy(ctx context.Context) (float64, error) {
	if m.simulateVariation {
		// Thrust varies with volatility - high vol can mean strong moves or choppiness
		variation := 0.0
		switch m.volatilityTrend {
		case "increasing":
			variation = (rand.Float64() - 0.5) * 0.20 // High variation in volatile markets
		case "decreasing":
			variation = (rand.Float64() - 0.3) * 0.10 // Lower thrust in calm markets
		default:
			variation = (rand.Float64() - 0.5) * 0.15 // Moderate variation
		}
		result := m.breadthThrustADX + variation
		if result < 0.0 {
			result = 0.0
		}
		if result > 1.0 {
			result = 1.0
		}
		return result, nil
	}
	return m.breadthThrustADX, nil
}

// GetTimestamp returns the current timestamp for the mock data
func (m *MockDetectorInputs) GetTimestamp(ctx context.Context) (time.Time, error) {
	return m.currentTime, nil
}

// SetVolatilityTrend updates the volatility trend for dynamic simulation
func (m *MockDetectorInputs) SetVolatilityTrend(trend string) {
	m.volatilityTrend = trend
}

// SetValues allows manual override of all input values
func (m *MockDetectorInputs) SetValues(realizedVol, breadth, thrust float64) {
	m.realizedVol7d = realizedVol
	m.breadthAbove20MA = breadth
	m.breadthThrustADX = thrust
	m.simulateVariation = false // Fixed values
}

// AdvanceTime moves the mock time forward (useful for testing update intervals)
func (m *MockDetectorInputs) AdvanceTime(duration time.Duration) {
	m.currentTime = m.currentTime.Add(duration)
}

// CreateMarketScenario creates inputs that simulate specific market conditions
func CreateMarketScenario(scenario string) *MockDetectorInputs {
	inputs := NewMockDetectorInputs()

	switch scenario {
	case "bull_run":
		inputs.SetValues(0.15, 0.80, 0.85) // Low vol, high breadth, strong thrust
		inputs.SetVolatilityTrend("decreasing")

	case "bear_market":
		inputs.SetValues(0.35, 0.25, 0.60) // High vol, low breadth, moderate thrust
		inputs.SetVolatilityTrend("increasing")

	case "sideways_chop":
		inputs.SetValues(0.20, 0.50, 0.45) // Moderate vol, neutral breadth, low thrust
		inputs.SetVolatilityTrend("stable")

	case "volatility_spike":
		inputs.SetValues(0.45, 0.35, 0.55) // Very high vol, weakening breadth, confused thrust
		inputs.SetVolatilityTrend("increasing")

	case "recovery_phase":
		inputs.SetValues(0.28, 0.60, 0.68) // Elevated vol, improving breadth, building thrust
		inputs.SetVolatilityTrend("decreasing")

	case "distribution_phase":
		inputs.SetValues(0.22, 0.55, 0.40) // Moderate vol, weakening breadth, low thrust
		inputs.SetVolatilityTrend("stable")
	}

	return inputs
}
