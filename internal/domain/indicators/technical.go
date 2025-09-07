package indicators

import (
	"fmt"
	"math"
)

// RSIResult represents the result of RSI calculation
type RSIResult struct {
	Value     float64 `json:"value"`
	Period    int     `json:"period"`
	IsValid   bool    `json:"is_valid"`
	DataCount int     `json:"data_count"`
}

// CalculateRSI calculates the Relative Strength Index (RSI) for given price data
func CalculateRSI(prices []float64, period int) RSIResult {
	if len(prices) < period+1 {
		return RSIResult{
			Value:     50.0, // Neutral RSI when insufficient data
			Period:    period,
			IsValid:   false,
			DataCount: len(prices),
		}
	}

	// Calculate price changes
	changes := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		changes[i-1] = prices[i] - prices[i-1]
	}

	// Separate gains and losses
	gains := make([]float64, len(changes))
	losses := make([]float64, len(changes))
	
	for i, change := range changes {
		if change > 0 {
			gains[i] = change
			losses[i] = 0
		} else {
			gains[i] = 0
			losses[i] = -change
		}
	}

	// Calculate initial averages (SMA for first period)
	avgGain := 0.0
	avgLoss := 0.0
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Use EMA (Wilder's smoothing) for subsequent periods
	alpha := 1.0 / float64(period)
	for i := period; i < len(changes); i++ {
		avgGain = avgGain*(1-alpha) + gains[i]*alpha
		avgLoss = avgLoss*(1-alpha) + losses[i]*alpha
	}

	// Calculate RSI
	if avgLoss == 0 {
		return RSIResult{
			Value:     100.0,
			Period:    period,
			IsValid:   true,
			DataCount: len(prices),
		}
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return RSIResult{
		Value:     rsi,
		Period:    period,
		IsValid:   true,
		DataCount: len(prices),
	}
}

// ATRResult represents the result of ATR calculation
type ATRResult struct {
	Value     float64 `json:"value"`
	Period    int     `json:"period"`
	IsValid   bool    `json:"is_valid"`
	DataCount int     `json:"data_count"`
}

// PriceBar represents OHLC price data
type PriceBar struct {
	High  float64
	Low   float64
	Close float64
}

// CalculateATR calculates the Average True Range (ATR) for given OHLC data
func CalculateATR(bars []PriceBar, period int) ATRResult {
	if len(bars) < period+1 {
		return ATRResult{
			Value:     0.0,
			Period:    period,
			IsValid:   false,
			DataCount: len(bars),
		}
	}

	// Calculate True Range values
	trueRanges := make([]float64, len(bars)-1)
	for i := 1; i < len(bars); i++ {
		currentBar := bars[i]
		previousClose := bars[i-1].Close
		
		// True Range = max(high-low, |high-prevClose|, |low-prevClose|)
		hl := currentBar.High - currentBar.Low
		hc := math.Abs(currentBar.High - previousClose)
		lc := math.Abs(currentBar.Low - previousClose)
		
		trueRanges[i-1] = math.Max(hl, math.Max(hc, lc))
	}

	if len(trueRanges) < period {
		return ATRResult{
			Value:     0.0,
			Period:    period,
			IsValid:   false,
			DataCount: len(bars),
		}
	}

	// Calculate initial ATR (SMA for first period)
	atr := 0.0
	for i := 0; i < period; i++ {
		atr += trueRanges[i]
	}
	atr /= float64(period)

	// Use EMA (Wilder's smoothing) for subsequent periods
	alpha := 1.0 / float64(period)
	for i := period; i < len(trueRanges); i++ {
		atr = atr*(1-alpha) + trueRanges[i]*alpha
	}

	return ATRResult{
		Value:     atr,
		Period:    period,
		IsValid:   true,
		DataCount: len(bars),
	}
}

// ADXResult represents the result of ADX calculation
type ADXResult struct {
	ADX       float64 `json:"adx"`
	PDI       float64 `json:"pdi"`       // Plus Directional Indicator
	MDI       float64 `json:"mdi"`       // Minus Directional Indicator
	Period    int     `json:"period"`
	IsValid   bool    `json:"is_valid"`
	DataCount int     `json:"data_count"`
}

// CalculateADX calculates the Average Directional Index (ADX) for trend strength
func CalculateADX(bars []PriceBar, period int) ADXResult {
	if len(bars) < period*2+1 { // Need extra data for smoothing
		return ADXResult{
			ADX:       0.0,
			PDI:       0.0,
			MDI:       0.0,
			Period:    period,
			IsValid:   false,
			DataCount: len(bars),
		}
	}

	// Calculate True Range and Directional Movements
	trueRanges := make([]float64, len(bars)-1)
	plusDM := make([]float64, len(bars)-1)
	minusDM := make([]float64, len(bars)-1)

	for i := 1; i < len(bars); i++ {
		currentBar := bars[i]
		previousBar := bars[i-1]
		
		// True Range
		hl := currentBar.High - currentBar.Low
		hc := math.Abs(currentBar.High - previousBar.Close)
		lc := math.Abs(currentBar.Low - previousBar.Close)
		trueRanges[i-1] = math.Max(hl, math.Max(hc, lc))
		
		// Directional Movements
		plusMove := currentBar.High - previousBar.High
		minusMove := previousBar.Low - currentBar.Low
		
		if plusMove > minusMove && plusMove > 0 {
			plusDM[i-1] = plusMove
		} else {
			plusDM[i-1] = 0
		}
		
		if minusMove > plusMove && minusMove > 0 {
			minusDM[i-1] = minusMove
		} else {
			minusDM[i-1] = 0
		}
	}

	if len(trueRanges) < period {
		return ADXResult{
			ADX:       0.0,
			Period:    period,
			IsValid:   false,
			DataCount: len(bars),
		}
	}

	// Calculate initial smoothed values (SMA for first period)
	smoothedTR := 0.0
	smoothedPlusDM := 0.0
	smoothedMinusDM := 0.0
	
	for i := 0; i < period; i++ {
		smoothedTR += trueRanges[i]
		smoothedPlusDM += plusDM[i]
		smoothedMinusDM += minusDM[i]
	}

	// Apply Wilder's smoothing for subsequent periods
	alpha := 1.0 / float64(period)
	for i := period; i < len(trueRanges); i++ {
		smoothedTR = smoothedTR*(1-alpha) + trueRanges[i]*alpha
		smoothedPlusDM = smoothedPlusDM*(1-alpha) + plusDM[i]*alpha
		smoothedMinusDM = smoothedMinusDM*(1-alpha) + minusDM[i]*alpha
	}

	// Calculate Directional Indicators
	var pdi, mdi, adx float64
	if smoothedTR > 0 {
		pdi = 100.0 * smoothedPlusDM / smoothedTR
		mdi = 100.0 * smoothedMinusDM / smoothedTR
		
		// Calculate ADX
		sum := pdi + mdi
		if sum > 0 {
			dx := 100.0 * math.Abs(pdi-mdi) / sum
			adx = dx // Simplified - in full implementation would smooth DX values
		}
	}

	return ADXResult{
		ADX:       adx,
		PDI:       pdi,
		MDI:       mdi,
		Period:    period,
		IsValid:   true,
		DataCount: len(bars),
	}
}

// HurstResult represents the result of Hurst Exponent calculation
type HurstResult struct {
	Exponent  float64 `json:"exponent"`
	Period    int     `json:"period"`
	IsValid   bool    `json:"is_valid"`
	DataCount int     `json:"data_count"`
	Strength  string  `json:"strength"` // "persistent", "random", "mean_reverting"
}

// CalculateHurstExponent calculates the Hurst Exponent for persistence analysis
// Uses R/S (Rescaled Range) analysis method
func CalculateHurstExponent(prices []float64, period int) HurstResult {
	if len(prices) < period {
		return HurstResult{
			Exponent:  0.5, // Random walk default
			Period:    period,
			IsValid:   false,
			DataCount: len(prices),
			Strength:  "insufficient_data",
		}
	}

	// Use the most recent 'period' prices
	recentPrices := prices
	if len(prices) > period {
		recentPrices = prices[len(prices)-period:]
	}

	// Calculate log returns
	logReturns := make([]float64, len(recentPrices)-1)
	for i := 1; i < len(recentPrices); i++ {
		if recentPrices[i] > 0 && recentPrices[i-1] > 0 {
			logReturns[i-1] = math.Log(recentPrices[i] / recentPrices[i-1])
		}
	}

	if len(logReturns) < 10 { // Need minimum data for reliable calculation
		return HurstResult{
			Exponent:  0.5,
			Period:    period,
			IsValid:   false,
			DataCount: len(prices),
			Strength:  "insufficient_data",
		}
	}

	// Calculate mean of log returns
	mean := 0.0
	for _, ret := range logReturns {
		mean += ret
	}
	mean /= float64(len(logReturns))

	// Calculate cumulative deviations from mean
	cumDeviations := make([]float64, len(logReturns))
	cumDeviations[0] = logReturns[0] - mean
	for i := 1; i < len(logReturns); i++ {
		cumDeviations[i] = cumDeviations[i-1] + (logReturns[i] - mean)
	}

	// Calculate range
	maxCumDev := cumDeviations[0]
	minCumDev := cumDeviations[0]
	for _, dev := range cumDeviations {
		if dev > maxCumDev {
			maxCumDev = dev
		}
		if dev < minCumDev {
			minCumDev = dev
		}
	}
	rRange := maxCumDev - minCumDev

	// Calculate standard deviation
	variance := 0.0
	for _, ret := range logReturns {
		variance += (ret - mean) * (ret - mean)
	}
	variance /= float64(len(logReturns) - 1)
	stdDev := math.Sqrt(variance)

	// Calculate R/S ratio
	var rsRatio float64
	if stdDev > 0 {
		rsRatio = rRange / stdDev
	} else {
		rsRatio = 1.0
	}

	// Calculate Hurst Exponent: H = log(R/S) / log(n)
	// Simplified calculation - full implementation would use multiple time scales
	var hurst float64
	n := float64(len(logReturns))
	if rsRatio > 0 && n > 1 {
		hurst = math.Log(rsRatio) / math.Log(n)
	} else {
		hurst = 0.5
	}

	// Ensure Hurst is within reasonable bounds
	if hurst < 0 {
		hurst = 0.0
	} else if hurst > 1 {
		hurst = 1.0
	}

	// Determine strength classification
	var strength string
	if hurst > 0.55 {
		strength = "persistent"     // Trending behavior
	} else if hurst < 0.45 {
		strength = "mean_reverting" // Anti-persistent
	} else {
		strength = "random"         // Random walk
	}

	return HurstResult{
		Exponent:  hurst,
		Period:    period,
		IsValid:   true,
		DataCount: len(prices),
		Strength:  strength,
	}
}

// TechnicalIndicators aggregates all technical indicators
type TechnicalIndicators struct {
	RSI   RSIResult   `json:"rsi"`
	ATR   ATRResult   `json:"atr"`
	ADX   ADXResult   `json:"adx"`
	Hurst HurstResult `json:"hurst"`
}

// GetTechnicalScore computes a composite technical score from all indicators
func (ti *TechnicalIndicators) GetTechnicalScore() float64 {
	score := 0.0
	
	// RSI contribution (0-30: bullish, 70-100: bearish, 30-70: neutral)
	if ti.RSI.IsValid {
		if ti.RSI.Value < 30 {
			score += (30 - ti.RSI.Value) / 30 * 25 // 0-25 points for oversold
		} else if ti.RSI.Value > 70 {
			score -= (ti.RSI.Value - 70) / 30 * 25 // -25 to 0 points for overbought
		}
	}
	
	// ADX contribution (trending strength)
	if ti.ADX.IsValid && ti.ADX.ADX > 25 {
		score += (ti.ADX.ADX - 25) / 75 * 25 // 0-25 points for strong trend
	}
	
	// Hurst contribution (trend persistence)
	if ti.Hurst.IsValid {
		if ti.Hurst.Exponent > 0.5 {
			score += (ti.Hurst.Exponent - 0.5) / 0.5 * 25 // 0-25 points for persistent trend
		}
	}
	
	// ATR contribution normalized to recent volatility
	if ti.ATR.IsValid {
		// Higher ATR suggests more movement potential but is context-dependent
		// For now, just add a small positive contribution for adequate volatility
		if ti.ATR.Value > 0.01 { // Above 1% volatility
			score += 5
		}
	}
	
	// Normalize to -50 to +50 range, then shift to 0-100
	return math.Max(0, math.Min(100, score + 50))
}

// CalculateAllIndicators calculates all technical indicators for the given data
func CalculateAllIndicators(prices []float64, bars []PriceBar) (TechnicalIndicators, error) {
	if len(prices) == 0 {
		return TechnicalIndicators{}, fmt.Errorf("no price data provided")
	}

	if len(bars) == 0 {
		return TechnicalIndicators{}, fmt.Errorf("no OHLC bar data provided")
	}

	// Standard periods as per CryptoRun specifications
	const (
		RSI_PERIOD   = 14
		ATR_PERIOD   = 14
		ADX_PERIOD   = 14
		HURST_PERIOD = 50
	)

	indicators := TechnicalIndicators{
		RSI:   CalculateRSI(prices, RSI_PERIOD),
		ATR:   CalculateATR(bars, ATR_PERIOD),
		ADX:   CalculateADX(bars, ADX_PERIOD),
		Hurst: CalculateHurstExponent(prices, HURST_PERIOD),
	}

	return indicators, nil
}

// GetTechnicalScore returns a normalized technical score (0-100) based on all indicators
func GetTechnicalScore(indicators TechnicalIndicators, currentPrice float64) float64 {
	score := 0.0
	validIndicators := 0

	// RSI contribution (0-30 points) - prefer 40-60 range, penalize extremes
	if indicators.RSI.IsValid {
		rsiScore := 0.0
		if indicators.RSI.Value >= 40 && indicators.RSI.Value <= 60 {
			// Optimal RSI range
			rsiScore = 30.0
		} else if indicators.RSI.Value >= 30 && indicators.RSI.Value <= 70 {
			// Acceptable range with slight penalty
			rsiScore = 25.0
		} else if indicators.RSI.Value >= 20 && indicators.RSI.Value <= 80 {
			// Borderline range
			rsiScore = 15.0
		} else {
			// Extreme values - low score
			rsiScore = 5.0
		}
		score += rsiScore
		validIndicators++
	}

	// ADX contribution (0-25 points) - higher ADX indicates stronger trend
	if indicators.ADX.IsValid {
		adxScore := 0.0
		if indicators.ADX.ADX >= 25 {
			// Strong trend
			adxScore = 25.0
		} else if indicators.ADX.ADX >= 20 {
			// Moderate trend
			adxScore = 20.0
		} else if indicators.ADX.ADX >= 15 {
			// Weak trend
			adxScore = 15.0
		} else {
			// Very weak trend
			adxScore = 5.0
		}
		score += adxScore
		validIndicators++
	}

	// Hurst contribution (0-25 points) - persistence is preferred for momentum
	if indicators.Hurst.IsValid {
		hurstScore := 0.0
		if indicators.Hurst.Exponent >= 0.6 {
			// High persistence - excellent for momentum
			hurstScore = 25.0
		} else if indicators.Hurst.Exponent >= 0.55 {
			// Good persistence
			hurstScore = 20.0
		} else if indicators.Hurst.Exponent >= 0.5 {
			// Random walk - neutral
			hurstScore = 10.0
		} else {
			// Mean reverting - not ideal for momentum
			hurstScore = 5.0
		}
		score += hurstScore
		validIndicators++
	}

	// ATR relative contribution (0-20 points) - higher relative volatility preferred
	if indicators.ATR.IsValid && currentPrice > 0 {
		atrPercent := (indicators.ATR.Value / currentPrice) * 100.0
		atrScore := 0.0
		if atrPercent >= 3.0 {
			// High volatility - good for momentum
			atrScore = 20.0
		} else if atrPercent >= 2.0 {
			// Moderate volatility
			atrScore = 15.0
		} else if atrPercent >= 1.0 {
			// Low volatility
			atrScore = 10.0
		} else {
			// Very low volatility
			atrScore = 5.0
		}
		score += atrScore
		validIndicators++
	}

	// Normalize score based on available indicators
	if validIndicators > 0 {
		maxPossibleScore := float64(validIndicators) * 25.0 // Each indicator contributes max 25 points on average
		return (score / maxPossibleScore) * 100.0
	}

	return 0.0 // No valid indicators
}