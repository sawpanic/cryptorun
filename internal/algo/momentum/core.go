package momentum

import (
	"context"
	"math"
	"time"
)

// MomentumConfig defines multi-timeframe momentum parameters
type MomentumConfig struct {
	Weights   WeightConfig    `yaml:"weights"`
	Fatigue   FatigueConfig   `yaml:"fatigue"`
	Freshness FreshnessConfig `yaml:"freshness"`
	LateFill  LateFillConfig  `yaml:"late_fill"`
	Regime    RegimeConfig    `yaml:"regime"`
}

// WeightConfig defines timeframe weights per PRD v3.2.1
type WeightConfig struct {
	TF1h  float64 `yaml:"tf_1h"`  // 20%
	TF4h  float64 `yaml:"tf_4h"`  // 35%
	TF12h float64 `yaml:"tf_12h"` // 30%
	TF24h float64 `yaml:"tf_24h"` // 10-15%
}

// FatigueConfig defines fatigue guard parameters
type FatigueConfig struct {
	Return24hThreshold float64 `yaml:"return_24h_threshold"` // +12%
	RSI4hThreshold     float64 `yaml:"rsi_4h_threshold"`     // 70
	AccelRenewal       bool    `yaml:"accel_renewal"`        // true
}

// FreshnessConfig defines freshness guard parameters
type FreshnessConfig struct {
	MaxBarsAge int     `yaml:"max_bars_age"` // ≤2 bars
	ATRWindow  int     `yaml:"atr_window"`   // 14
	ATRFactor  float64 `yaml:"atr_factor"`   // 1.2x
}

// LateFillConfig defines late-fill guard parameters
type LateFillConfig struct {
	MaxDelaySeconds int `yaml:"max_delay_seconds"` // 30s
}

// RegimeConfig defines regime-adaptive parameters
type RegimeConfig struct {
	AdaptWeights bool `yaml:"adapt_weights"`
	UpdatePeriod int  `yaml:"update_period"` // 4h
}

// MarketData represents OHLCV data point
type MarketData struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// MomentumCore implements multi-timeframe momentum calculation
type MomentumCore struct {
	config MomentumConfig
}

// NewMomentumCore creates a new momentum core engine
func NewMomentumCore(config MomentumConfig) *MomentumCore {
	return &MomentumCore{
		config: config,
	}
}

// MomentumResult contains momentum analysis results
type MomentumResult struct {
	Symbol          string             `json:"symbol"`
	Timestamp       time.Time          `json:"timestamp"`
	CoreScore       float64            `json:"core_score"`
	TimeframeScores map[string]float64 `json:"timeframe_scores"`
	Acceleration4h  float64            `json:"acceleration_4h"`
	GuardResults    GuardResults       `json:"guard_results"`
	Regime          string             `json:"regime"`
	Protected       bool               `json:"protected"` // MomentumCore protection
}

// GuardResults contains guard validation results
type GuardResults struct {
	Fatigue   GuardResult `json:"fatigue"`
	Freshness GuardResult `json:"freshness"`
	LateFill  GuardResult `json:"late_fill"`
}

// GuardResult represents individual guard validation
type GuardResult struct {
	Pass   bool    `json:"pass"`
	Value  float64 `json:"value"`
	Reason string  `json:"reason,omitempty"`
}

// Calculate performs multi-timeframe momentum analysis
func (mc *MomentumCore) Calculate(ctx context.Context, symbol string, data map[string][]MarketData, regime string) (*MomentumResult, error) {
	result := &MomentumResult{
		Symbol:          symbol,
		Timestamp:       time.Now(),
		TimeframeScores: make(map[string]float64),
		Regime:          regime,
		Protected:       true, // MomentumCore is protected in Gram-Schmidt
	}

	// Calculate momentum for each timeframe
	if tf1h, exists := data["1h"]; exists && len(tf1h) > 0 {
		result.TimeframeScores["1h"] = mc.calculateTimeframeMomentum(tf1h, "1h")
	}

	if tf4h, exists := data["4h"]; exists && len(tf4h) > 0 {
		result.TimeframeScores["4h"] = mc.calculateTimeframeMomentum(tf4h, "4h")
		result.Acceleration4h = mc.CalculateAcceleration(tf4h)
	}

	if tf12h, exists := data["12h"]; exists && len(tf12h) > 0 {
		result.TimeframeScores["12h"] = mc.calculateTimeframeMomentum(tf12h, "12h")
	}

	if tf24h, exists := data["24h"]; exists && len(tf24h) > 0 {
		result.TimeframeScores["24h"] = mc.calculateTimeframeMomentum(tf24h, "24h")
	}

	// Apply regime-adaptive weights and calculate core score
	weights := mc.GetRegimeWeights(regime)
	result.CoreScore = mc.calculateWeightedScore(result.TimeframeScores, weights)

	// Apply guards
	result.GuardResults = mc.ApplyGuards(data, result)

	return result, nil
}

// calculateTimeframeMomentum calculates momentum for a specific timeframe
func (mc *MomentumCore) calculateTimeframeMomentum(data []MarketData, timeframe string) float64 {
	if len(data) < 2 {
		return 0.0
	}

	// Simple price momentum calculation
	current := data[len(data)-1].Close
	previous := data[len(data)-2].Close

	if previous == 0 {
		return 0.0
	}

	momentum := (current - previous) / previous * 100.0

	// Apply timeframe-specific adjustments
	switch timeframe {
	case "1h":
		return momentum * 1.0 // Base momentum
	case "4h":
		return momentum * 1.2 // Slight boost for primary timeframe
	case "12h":
		return momentum * 0.9 // Slight damping for longer timeframe
	case "24h":
		return momentum * 0.8 // More damping for longest timeframe
	default:
		return momentum
	}
}

// CalculateAcceleration calculates 4h momentum acceleration
func (mc *MomentumCore) CalculateAcceleration(data []MarketData) float64 {
	if len(data) < 3 {
		return 0.0
	}

	// Calculate current and previous momentum
	current := data[len(data)-1].Close
	prev1 := data[len(data)-2].Close
	prev2 := data[len(data)-3].Close

	if prev1 == 0 || prev2 == 0 {
		return 0.0
	}

	currentMom := (current - prev1) / prev1
	prevMom := (prev1 - prev2) / prev2

	return (currentMom - prevMom) * 100.0
}

// GetRegimeWeights returns regime-adaptive weights
func (mc *MomentumCore) GetRegimeWeights(regime string) WeightConfig {
	baseWeights := mc.config.Weights

	if !mc.config.Regime.AdaptWeights {
		return baseWeights
	}

	// Adjust weights based on regime
	switch regime {
	case "trending":
		// Favor longer timeframes in trending markets
		return WeightConfig{
			TF1h:  baseWeights.TF1h * 0.8,
			TF4h:  baseWeights.TF4h * 1.1,
			TF12h: baseWeights.TF12h * 1.2,
			TF24h: baseWeights.TF24h * 1.3,
		}
	case "choppy":
		// Favor shorter timeframes in choppy markets
		return WeightConfig{
			TF1h:  baseWeights.TF1h * 1.3,
			TF4h:  baseWeights.TF4h * 1.2,
			TF12h: baseWeights.TF12h * 0.9,
			TF24h: baseWeights.TF24h * 0.7,
		}
	case "volatile":
		// Balanced approach in volatile markets
		return WeightConfig{
			TF1h:  baseWeights.TF1h * 1.1,
			TF4h:  baseWeights.TF4h * 1.0,
			TF12h: baseWeights.TF12h * 0.95,
			TF24h: baseWeights.TF24h * 0.9,
		}
	default:
		return baseWeights
	}
}

// calculateWeightedScore calculates weighted momentum score
func (mc *MomentumCore) calculateWeightedScore(scores map[string]float64, weights WeightConfig) float64 {
	totalWeight := 0.0
	weightedSum := 0.0

	if score, exists := scores["1h"]; exists {
		weightedSum += score * weights.TF1h
		totalWeight += weights.TF1h
	}

	if score, exists := scores["4h"]; exists {
		weightedSum += score * weights.TF4h
		totalWeight += weights.TF4h
	}

	if score, exists := scores["12h"]; exists {
		weightedSum += score * weights.TF12h
		totalWeight += weights.TF12h
	}

	if score, exists := scores["24h"]; exists {
		weightedSum += score * weights.TF24h
		totalWeight += weights.TF24h
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedSum / totalWeight
}

// calculateRSI calculates RSI for given period
func calculateRSI(data []MarketData, period int) float64 {
	if len(data) < period+1 {
		return 50.0 // Neutral RSI
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gains and losses
	for i := len(data) - period; i < len(data); i++ {
		change := data[i].Close - data[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return rsi
}

// calculateATR calculates Average True Range
func calculateATR(data []MarketData, period int) float64 {
	if len(data) < period+1 {
		return 0.0
	}

	trSum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		tr := math.Max(
			data[i].High-data[i].Low,
			math.Max(
				math.Abs(data[i].High-data[i-1].Close),
				math.Abs(data[i].Low-data[i-1].Close),
			),
		)
		trSum += tr
	}

	return trSum / float64(period)
}
