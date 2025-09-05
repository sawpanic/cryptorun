package pipeline

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

// TimeFrame represents different momentum calculation timeframes
type TimeFrame string

const (
	TF1h  TimeFrame = "1h"
	TF4h  TimeFrame = "4h" 
	TF12h TimeFrame = "12h"
	TF24h TimeFrame = "24h"
	TF7d  TimeFrame = "7d"
)

// MarketData represents price/volume data for momentum calculations
type MarketData struct {
	Symbol    string
	Timestamp time.Time
	Price     float64
	Volume    float64
	High      float64
	Low       float64
}

// MomentumFactors holds calculated momentum across timeframes
type MomentumFactors struct {
	Symbol     string                 `json:"symbol"`
	Timestamp  time.Time              `json:"timestamp"`
	Momentum1h float64                `json:"momentum_1h"`
	Momentum4h float64                `json:"momentum_4h"`
	Momentum12h float64               `json:"momentum_12h"`
	Momentum24h float64               `json:"momentum_24h"`
	Momentum7d float64                `json:"momentum_7d"`
	Volume1h   float64                `json:"volume_1h"`
	Volume4h   float64                `json:"volume_4h"`
	Volume24h  float64                `json:"volume_24h"`
	RSI4h      float64                `json:"rsi_4h"`
	ATR1h      float64                `json:"atr_1h"`
	Raw        map[TimeFrame]float64  `json:"raw_momentum"`
	Meta       MomentumMeta           `json:"meta"`
}

type MomentumMeta struct {
	DataPoints  int       `json:"data_points"`
	LastUpdate  time.Time `json:"last_update"`
	Source      string    `json:"source"`
	CacheHit    bool      `json:"cache_hit"`
}

// RegimeWeights defines momentum weighting by market regime
type RegimeWeights struct {
	Regime      string             `json:"regime"`
	Weights     map[TimeFrame]float64 `json:"weights"`
	Description string             `json:"description"`
}

// Standard regime weight configurations
var (
	BullRegimeWeights = RegimeWeights{
		Regime: "bull",
		Weights: map[TimeFrame]float64{
			TF1h:  0.20, // 20%
			TF4h:  0.35, // 35%  
			TF12h: 0.30, // 30%
			TF24h: 0.15, // 15%
			TF7d:  0.00, // 0% in bull markets (focus on shorter term)
		},
		Description: "Bull market: emphasis on 4h-12h momentum",
	}
	
	ChoppyRegimeWeights = RegimeWeights{
		Regime: "choppy",
		Weights: map[TimeFrame]float64{
			TF1h:  0.15, // 15%
			TF4h:  0.25, // 25%
			TF12h: 0.35, // 35%
			TF24h: 0.20, // 20%
			TF7d:  0.05, // 5% (some longer-term context)
		},
		Description: "Choppy market: emphasis on 12h-24h for stability",
	}
	
	HighVolRegimeWeights = RegimeWeights{
		Regime: "high_vol",
		Weights: map[TimeFrame]float64{
			TF1h:  0.10, // 10%
			TF4h:  0.20, // 20%
			TF12h: 0.30, // 30%
			TF24h: 0.30, // 30%
			TF7d:  0.10, // 10% (longer-term stability)
		},
		Description: "High volatility: emphasis on longer timeframes",
	}
)

// MomentumCalculator computes multi-timeframe momentum
type MomentumCalculator struct {
	dataProvider DataProvider
	regime       string
}

// DataProvider interface for market data access
type DataProvider interface {
	GetMarketData(ctx context.Context, symbol string, timeframe TimeFrame, periods int) ([]MarketData, error)
}

// NewMomentumCalculator creates a new momentum calculator
func NewMomentumCalculator(provider DataProvider) *MomentumCalculator {
	return &MomentumCalculator{
		dataProvider: provider,
		regime:       "bull", // Default regime
	}
}

// SetRegime updates the current market regime for weight calculation
func (mc *MomentumCalculator) SetRegime(regime string) {
	mc.regime = regime
	log.Info().Str("regime", regime).Msg("Updated momentum calculation regime")
}

// CalculateMomentum computes momentum factors across all timeframes
func (mc *MomentumCalculator) CalculateMomentum(ctx context.Context, symbol string) (*MomentumFactors, error) {
	factors := &MomentumFactors{
		Symbol:    symbol,
		Timestamp: time.Now().UTC(),
		Raw:       make(map[TimeFrame]float64),
		Meta: MomentumMeta{
			Source:     "kraken",
			LastUpdate: time.Now().UTC(),
			CacheHit:   false,
		},
	}

	timeframes := []TimeFrame{TF1h, TF4h, TF12h, TF24h, TF7d}
	
	for _, tf := range timeframes {
		momentum, volume, err := mc.calculateTimeframeMomentum(ctx, symbol, tf)
		if err != nil {
			log.Warn().Err(err).Str("symbol", symbol).Str("timeframe", string(tf)).
				Msg("Failed to calculate momentum for timeframe")
			// Use NaN for failed calculations rather than failing completely
			momentum = math.NaN()
			volume = math.NaN()
		}
		
		factors.Raw[tf] = momentum
		
		// Assign to specific fields
		switch tf {
		case TF1h:
			factors.Momentum1h = momentum
			factors.Volume1h = volume
		case TF4h:
			factors.Momentum4h = momentum
			factors.Volume4h = volume
		case TF12h:
			factors.Momentum12h = momentum
		case TF24h:
			factors.Momentum24h = momentum
			factors.Volume24h = volume
		case TF7d:
			factors.Momentum7d = momentum
		}
		
		factors.Meta.DataPoints++
	}

	// Calculate technical indicators
	rsi, err := mc.calculateRSI(ctx, symbol, TF4h, 14)
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate RSI")
		rsi = math.NaN()
	}
	factors.RSI4h = rsi

	atr, err := mc.calculateATR(ctx, symbol, TF1h, 14) 
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate ATR")
		atr = math.NaN()
	}
	factors.ATR1h = atr

	return factors, nil
}

// calculateTimeframeMomentum computes momentum for a specific timeframe
func (mc *MomentumCalculator) calculateTimeframeMomentum(ctx context.Context, symbol string, tf TimeFrame) (float64, float64, error) {
	// Determine how many periods to fetch based on timeframe
	periods := mc.getPeriodsForTimeframe(tf)
	
	data, err := mc.dataProvider.GetMarketData(ctx, symbol, tf, periods)
	if err != nil {
		return math.NaN(), math.NaN(), fmt.Errorf("failed to get market data: %w", err)
	}

	if len(data) < 2 {
		return math.NaN(), math.NaN(), fmt.Errorf("insufficient data points: got %d, need at least 2", len(data))
	}

	// Calculate momentum as percentage change from first to last period
	first := data[0].Price
	last := data[len(data)-1].Price
	
	if first <= 0 {
		return math.NaN(), math.NaN(), fmt.Errorf("invalid first price: %f", first)
	}

	momentum := ((last - first) / first) * 100.0

	// Calculate average volume
	totalVolume := 0.0
	for _, point := range data {
		totalVolume += point.Volume
	}
	avgVolume := totalVolume / float64(len(data))

	return momentum, avgVolume, nil
}

// getPeriodsForTimeframe returns appropriate number of periods for each timeframe
func (mc *MomentumCalculator) getPeriodsForTimeframe(tf TimeFrame) int {
	switch tf {
	case TF1h:
		return 24 // 24 hours of 1h data
	case TF4h:
		return 18 // 72 hours of 4h data (3 days)
	case TF12h:
		return 14 // 168 hours of 12h data (7 days)
	case TF24h:
		return 14 // 14 days of 24h data
	case TF7d:
		return 12 // 12 weeks of 7d data
	default:
		return 14 // Default fallback
	}
}

// calculateRSI computes Relative Strength Index
func (mc *MomentumCalculator) calculateRSI(ctx context.Context, symbol string, tf TimeFrame, periods int) (float64, error) {
	data, err := mc.dataProvider.GetMarketData(ctx, symbol, tf, periods+1)
	if err != nil {
		return math.NaN(), err
	}

	if len(data) < periods+1 {
		return math.NaN(), fmt.Errorf("insufficient data for RSI: got %d, need %d", len(data), periods+1)
	}

	gains := 0.0
	losses := 0.0

	// Calculate average gains and losses
	for i := 1; i < len(data); i++ {
		change := data[i].Price - data[i-1].Price
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}

	if periods == 0 {
		return math.NaN(), fmt.Errorf("periods cannot be zero")
	}

	avgGain := gains / float64(periods)
	avgLoss := losses / float64(periods)

	if avgLoss == 0 {
		return 100.0, nil // Perfect RSI when no losses
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return rsi, nil
}

// calculateATR computes Average True Range
func (mc *MomentumCalculator) calculateATR(ctx context.Context, symbol string, tf TimeFrame, periods int) (float64, error) {
	data, err := mc.dataProvider.GetMarketData(ctx, symbol, tf, periods+1)
	if err != nil {
		return math.NaN(), err
	}

	if len(data) < 2 {
		return math.NaN(), fmt.Errorf("insufficient data for ATR: got %d, need at least 2", len(data))
	}

	var trueRanges []float64

	for i := 1; i < len(data); i++ {
		high := data[i].High
		low := data[i].Low
		prevClose := data[i-1].Price

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trueRanges = append(trueRanges, tr)
	}

	if len(trueRanges) == 0 {
		return math.NaN(), fmt.Errorf("no true range values calculated")
	}

	// Calculate ATR as simple average of true ranges
	total := 0.0
	for _, tr := range trueRanges {
		total += tr
	}

	atr := total / float64(len(trueRanges))
	return atr, nil
}

// GetRegimeWeights returns weights for the current regime
func (mc *MomentumCalculator) GetRegimeWeights() RegimeWeights {
	switch mc.regime {
	case "bull":
		return BullRegimeWeights
	case "choppy":
		return ChoppyRegimeWeights  
	case "high_vol":
		return HighVolRegimeWeights
	default:
		log.Warn().Str("regime", mc.regime).Msg("Unknown regime, defaulting to bull weights")
		return BullRegimeWeights
	}
}

// ApplyRegimeWeights applies regime-based weights to raw momentum factors
func (mc *MomentumCalculator) ApplyRegimeWeights(factors *MomentumFactors) float64 {
	weights := mc.GetRegimeWeights()
	
	weightedSum := 0.0
	totalWeight := 0.0
	
	for tf, weight := range weights.Weights {
		if rawMomentum, exists := factors.Raw[tf]; exists && !math.IsNaN(rawMomentum) {
			weightedSum += rawMomentum * weight
			totalWeight += weight
		}
	}
	
	if totalWeight == 0 {
		log.Warn().Str("symbol", factors.Symbol).Msg("No valid momentum data for regime weighting")
		return math.NaN()
	}
	
	// Normalize by actual total weight in case some timeframes were missing
	return weightedSum / totalWeight
}