package factors

import (
	"fmt"
	"math"
)

// CatalystCompressionInput contains the required data for catalyst compression calculations
type CatalystCompressionInput struct {
	Close        []float64 `json:"close"`         // Close prices
	TypicalPrice []float64 `json:"typical_price"` // (High + Low + Close) / 3
	High         []float64 `json:"high"`          // High prices for ATR
	Low          []float64 `json:"low"`           // Low prices for ATR
	Volume       []float64 `json:"volume"`        // Volume data
	Timestamp    []int64   `json:"timestamp"`     // Unix timestamps
}

// CatalystCompressionResult contains the output of catalyst compression analysis
type CatalystCompressionResult struct {
	// Core compression metrics
	CompressionScore float64 `json:"compression_score"` // 0-1 normalized compression score
	InSqueeze        bool    `json:"in_squeeze"`        // Boolean squeeze state
	BBWidth          float64 `json:"bb_width"`          // Raw Bollinger Band width
	BBWidthNorm      float64 `json:"bb_width_norm"`     // Normalized BB width (0-1)

	// Bollinger Band components
	BBUpper  float64 `json:"bb_upper"`  // Upper Bollinger Band
	BBLower  float64 `json:"bb_lower"`  // Lower Bollinger Band
	BBMiddle float64 `json:"bb_middle"` // Middle Bollinger Band (SMA)

	// Keltner Channel components
	KeltnerUpper  float64 `json:"keltner_upper"`  // Upper Keltner Channel
	KeltnerLower  float64 `json:"keltner_lower"`  // Lower Keltner Channel
	KeltnerMiddle float64 `json:"keltner_middle"` // Middle Keltner Channel (EMA)

	// Catalyst event weighting
	CatalystWeight float64 `json:"catalyst_weight"` // Time-decayed catalyst multiplier
	TierSignal     float64 `json:"tier_signal"`     // Weighted tier signal (0-1)

	// Final composite
	FinalScore float64 `json:"final_score"` // Combined score: 0.6*compression + 0.4*catalyst
}

// CatalystCompressionConfig holds configuration for catalyst compression calculations
type CatalystCompressionConfig struct {
	// Bollinger Band parameters
	BBPeriod int     `yaml:"bb_period"` // Period for BB calculation (default: 20)
	BBStdDev float64 `yaml:"bb_std_dev"` // Standard deviations for BB (default: 2.0)

	// Keltner Channel parameters
	KeltnerPeriod     int     `yaml:"keltner_period"`      // Period for Keltner EMA (default: 20)
	KeltnerMultiplier float64 `yaml:"keltner_multiplier"`  // ATR multiplier for Keltner (default: 1.5)
	ATRPeriod         int     `yaml:"atr_period"`          // Period for ATR calculation (default: 14)

	// Compression scoring
	CompressionLookback int     `yaml:"compression_lookback"` // Lookback for BB width z-score (default: 50)
	CompressionClamp    float64 `yaml:"compression_clamp"`    // Max z-score for normalization (default: 3.0)

	// Catalyst weighting
	CatalystDecayRate float64 `yaml:"catalyst_decay_rate"` // Decay rate per hour (default: 0.1)
	TierWeights       map[string]float64 `yaml:"tier_weights"` // Tier multipliers
}

// DefaultCatalystCompressionConfig returns sensible defaults
func DefaultCatalystCompressionConfig() CatalystCompressionConfig {
	return CatalystCompressionConfig{
		BBPeriod:            20,
		BBStdDev:            2.0,
		KeltnerPeriod:       20,
		KeltnerMultiplier:   1.5,
		ATRPeriod:           14,
		CompressionLookback: 50,
		CompressionClamp:    3.0,
		CatalystDecayRate:   0.1,
		TierWeights: map[string]float64{
			"imminent":   1.2,
			"near_term":  1.0,
			"medium":     0.8,
			"distant":    0.6,
		},
	}
}

// CatalystCompressionCalculator computes catalyst compression scores
type CatalystCompressionCalculator struct {
	config CatalystCompressionConfig
}

// NewCatalystCompressionCalculator creates a new calculator
func NewCatalystCompressionCalculator(config CatalystCompressionConfig) *CatalystCompressionCalculator {
	return &CatalystCompressionCalculator{
		config: config,
	}
}

// Calculate computes catalyst compression score from input data
func (ccc *CatalystCompressionCalculator) Calculate(input CatalystCompressionInput) (*CatalystCompressionResult, error) {
	if len(input.Close) < ccc.config.CompressionLookback {
		return nil, &ValidationError{
			Field:   "close",
			Message: "insufficient data for compression analysis",
			MinLen:  ccc.config.CompressionLookback,
			ActLen:  len(input.Close),
		}
	}

	// Use the most recent data point for current state
	
	// Calculate Bollinger Bands
	bbResult, err := ccc.calculateBollingerBands(input.Close, input.TypicalPrice)
	if err != nil {
		return nil, err
	}

	// Calculate Keltner Channels
	keltnerResult, err := ccc.calculateKeltnerChannels(input.TypicalPrice, input.High, input.Low)
	if err != nil {
		return nil, err
	}

	// Calculate compression score
	compressionScore := ccc.calculateCompressionScore(input.Close, bbResult.Width)
	
	// Determine squeeze state
	inSqueeze := ccc.isInSqueeze(bbResult, keltnerResult)

	// Calculate catalyst weighting (placeholder - will be enhanced with registry)
	// Use the provided timestamp (assumed to be current time for calculation)
	timestamp := input.Timestamp[0]
	if len(input.Timestamp) > 1 {
		timestamp = input.Timestamp[len(input.Timestamp)-1]
	}
	catalystWeight, tierSignal := ccc.calculateCatalystWeight(timestamp)

	// Combine final score: 60% compression, 40% catalyst
	finalScore := 0.6*compressionScore + 0.4*tierSignal

	return &CatalystCompressionResult{
		CompressionScore: compressionScore,
		InSqueeze:        inSqueeze,
		BBWidth:          bbResult.Width,
		BBWidthNorm:      compressionScore, // Same as compression score for now
		BBUpper:          bbResult.Upper,
		BBLower:          bbResult.Lower,
		BBMiddle:         bbResult.Middle,
		KeltnerUpper:     keltnerResult.Upper,
		KeltnerLower:     keltnerResult.Lower,
		KeltnerMiddle:    keltnerResult.Middle,
		CatalystWeight:   catalystWeight,
		TierSignal:       tierSignal,
		FinalScore:       finalScore,
	}, nil
}

// BollingerBandResult holds BB calculation results
type BollingerBandResult struct {
	Upper  float64 // Upper band
	Lower  float64 // Lower band
	Middle float64 // Middle band (SMA)
	Width  float64 // (Upper - Lower) / Middle
}

// calculateBollingerBands computes Bollinger Bands
func (ccc *CatalystCompressionCalculator) calculateBollingerBands(close, typical []float64) (*BollingerBandResult, error) {
	period := ccc.config.BBPeriod
	if len(close) < period {
		return nil, &ValidationError{
			Field:   "close",
			Message: "insufficient data for Bollinger Bands",
			MinLen:  period,
			ActLen:  len(close),
		}
	}

	// Use typical price if available, otherwise close
	prices := close
	if len(typical) == len(close) {
		prices = typical
	}

	// Calculate SMA (Middle Band)
	recentPrices := prices[len(prices)-period:]
	middle := calculateSMA(recentPrices)

	// Calculate standard deviation
	variance := 0.0
	for _, price := range recentPrices {
		diff := price - middle
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(period))

	// Calculate bands
	upper := middle + ccc.config.BBStdDev*stdDev
	lower := middle - ccc.config.BBStdDev*stdDev
	
	// Width as percentage of middle band
	width := (upper - lower) / middle

	return &BollingerBandResult{
		Upper:  upper,
		Lower:  lower,
		Middle: middle,
		Width:  width,
	}, nil
}

// KeltnerChannelResult holds Keltner Channel calculation results
type KeltnerChannelResult struct {
	Upper  float64 // Upper channel
	Lower  float64 // Lower channel
	Middle float64 // Middle channel (EMA)
}

// calculateKeltnerChannels computes Keltner Channels
func (ccc *CatalystCompressionCalculator) calculateKeltnerChannels(typical, high, low []float64) (*KeltnerChannelResult, error) {
	period := ccc.config.KeltnerPeriod
	atrPeriod := ccc.config.ATRPeriod
	
	minLen := max(period, atrPeriod)
	if len(typical) < minLen || len(high) < minLen || len(low) < minLen {
		return nil, &ValidationError{
			Field:   "price_data",
			Message: "insufficient data for Keltner Channels",
			MinLen:  minLen,
			ActLen:  len(typical),
		}
	}

	// Calculate EMA of typical price (middle line)
	middle := calculateEMA(typical, period)

	// Calculate ATR
	atr := calculateATR(high, low, typical, atrPeriod)

	// Calculate channels
	upper := middle + ccc.config.KeltnerMultiplier*atr
	lower := middle - ccc.config.KeltnerMultiplier*atr

	return &KeltnerChannelResult{
		Upper:  upper,
		Lower:  lower,
		Middle: middle,
	}, nil
}

// calculateCompressionScore converts BB width to 0-1 compression score using inverted z-score
func (ccc *CatalystCompressionCalculator) calculateCompressionScore(close []float64, currentWidth float64) float64 {
	lookback := ccc.config.CompressionLookback
	if len(close) < lookback {
		return 0.0 // Not enough data
	}

	// Calculate historical BB widths for z-score normalization
	var widths []float64
	for i := ccc.config.BBPeriod; i <= lookback && i <= len(close); i++ {
		subset := close[len(close)-i:]
		if len(subset) >= ccc.config.BBPeriod {
			// Quick BB width calculation for historical points
			sma := calculateSMA(subset[len(subset)-ccc.config.BBPeriod:])
			variance := 0.0
			for _, price := range subset[len(subset)-ccc.config.BBPeriod:] {
				diff := price - sma
				variance += diff * diff
			}
			stdDev := math.Sqrt(variance / float64(ccc.config.BBPeriod))
			width := (2 * ccc.config.BBStdDev * stdDev) / sma
			widths = append(widths, width)
		}
	}

	if len(widths) < 10 {
		return 0.0 // Not enough historical data
	}

	// Calculate mean and std dev of historical widths
	mean := 0.0
	for _, w := range widths {
		mean += w
	}
	mean /= float64(len(widths))

	variance := 0.0
	for _, w := range widths {
		diff := w - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(widths)))

	if stdDev == 0 {
		return 0.0 // No variation in width
	}

	// Calculate z-score (how many std devs below mean)
	zScore := (mean - currentWidth) / stdDev // Inverted: compression = width below mean

	// Normalize to 0-1 using sigmoid-like function with clamping
	clampedZ := math.Max(-ccc.config.CompressionClamp, math.Min(ccc.config.CompressionClamp, zScore))
	normalized := (clampedZ + ccc.config.CompressionClamp) / (2 * ccc.config.CompressionClamp)

	return math.Max(0.0, math.Min(1.0, normalized))
}

// isInSqueeze determines if BB is inside Keltner Channels
func (ccc *CatalystCompressionCalculator) isInSqueeze(bb *BollingerBandResult, keltner *KeltnerChannelResult) bool {
	// Squeeze occurs when BB is completely inside Keltner Channels
	return bb.Upper <= keltner.Upper && bb.Lower >= keltner.Lower
}

// calculateCatalystWeight computes time-decayed catalyst weighting (placeholder)
func (ccc *CatalystCompressionCalculator) calculateCatalystWeight(timestamp int64) (float64, float64) {
	// Placeholder implementation - will be enhanced with event registry
	// For now, return moderate catalyst activity
	baseWeight := 0.5 // Neutral catalyst environment
	tierSignal := 0.6 // Moderate tier signal
	
	return baseWeight, tierSignal
}

// Helper functions

// calculateSMA computes Simple Moving Average
func calculateSMA(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateEMA computes Exponential Moving Average
func calculateEMA(values []float64, period int) float64 {
	if len(values) < period {
		return calculateSMA(values) // Fallback to SMA
	}

	alpha := 2.0 / (float64(period) + 1.0)
	
	// Initialize with SMA of first period values
	ema := calculateSMA(values[:period])
	
	// Apply EMA formula to remaining values
	for i := period; i < len(values); i++ {
		ema = alpha*values[i] + (1-alpha)*ema
	}
	
	return ema
}

// calculateATR computes Average True Range
func calculateATR(high, low, close []float64, period int) float64 {
	if len(high) < period+1 || len(low) < period+1 || len(close) < period+1 {
		return 0.0
	}

	var trueRanges []float64
	
	// Calculate True Range for each period
	for i := 1; i < len(high) && len(trueRanges) < period; i++ {
		highLow := high[i] - low[i]
		highClose := math.Abs(high[i] - close[i-1])
		lowClose := math.Abs(low[i] - close[i-1])
		
		trueRange := math.Max(highLow, math.Max(highClose, lowClose))
		trueRanges = append(trueRanges, trueRange)
	}
	
	// Return SMA of true ranges
	return calculateSMA(trueRanges)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ValidationError represents a validation error in factor calculations
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	MinLen  int    `json:"min_length"`
	ActLen  int    `json:"actual_length"`
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s (need %d, got %d)", 
		ve.Field, ve.Message, ve.MinLen, ve.ActLen)
}