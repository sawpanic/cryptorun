package dip

import (
	"context"
	"math"
	"time"
)

// MarketData represents OHLCV data for dip analysis
type MarketData struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// TrendConfig contains trend qualification parameters
type TrendConfig struct {
	MALen12h  int     `yaml:"ma_len_12h"`
	MALen24h  int     `yaml:"ma_len_24h"`
	ADX4hMin  float64 `yaml:"adx_4h_min"`
	HurstMin  float64 `yaml:"hurst_min"`
	LookbackN int     `yaml:"lookback_n"`
}

// FibConfig contains Fibonacci retracement parameters
type FibConfig struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
}

// RSIConfig contains RSI-based dip identification parameters
type RSIConfig struct {
	LowMin         int `yaml:"low_min"`
	LowMax         int `yaml:"low_max"`
	DivConfirmBars int `yaml:"div_confirm_bars"`
}

// TrendResult contains trend qualification analysis
type TrendResult struct {
	Qualified    bool        `json:"qualified"`
	MA12hSlope   float64     `json:"ma_12h_slope"`
	MA24hSlope   float64     `json:"ma_24h_slope"`
	PriceAboveMA bool        `json:"price_above_ma"`
	ADX4h        float64     `json:"adx_4h"`
	Hurst        float64     `json:"hurst"`
	SwingHigh    *SwingPoint `json:"swing_high,omitempty"`
	Reason       string      `json:"reason,omitempty"`
}

// SwingPoint represents a significant high or low
type SwingPoint struct {
	Index     int       `json:"index"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// DipPoint represents a potential dip entry point
type DipPoint struct {
	Index         int       `json:"index"`
	Price         float64   `json:"price"`
	Timestamp     time.Time `json:"timestamp"`
	RSI           float64   `json:"rsi"`
	FibLevel      float64   `json:"fib_level"`
	RedBarsCount  int       `json:"red_bars_count"`
	ATRMultiple   float64   `json:"atr_multiple"`
	HasDivergence bool      `json:"has_divergence"`
	HasEngulfing  bool      `json:"has_engulfing"`
}

// DipCore provides core dip detection algorithms
type DipCore struct {
	trendConfig TrendConfig
	fibConfig   FibConfig
	rsiConfig   RSIConfig
}

// NewDipCore creates a new dip core analyzer
func NewDipCore(trendConfig TrendConfig, fibConfig FibConfig, rsiConfig RSIConfig) *DipCore {
	return &DipCore{
		trendConfig: trendConfig,
		fibConfig:   fibConfig,
		rsiConfig:   rsiConfig,
	}
}

// QualifyTrend checks if the market is in a qualified uptrend
func (dc *DipCore) QualifyTrend(ctx context.Context, data12h, data24h []MarketData, data4h []MarketData, currentPrice float64) (*TrendResult, error) {
	if len(data12h) < dc.trendConfig.MALen12h || len(data24h) < dc.trendConfig.MALen24h {
		return &TrendResult{
			Qualified: false,
			Reason:    "insufficient data for MA calculation",
		}, nil
	}

	// Calculate moving averages
	ma12h := calculateSMA(data12h, dc.trendConfig.MALen12h)
	ma24h := calculateSMA(data24h, dc.trendConfig.MALen24h)

	// Calculate MA slopes
	ma12hSlope := calculateSlope(ma12h, 3)
	ma24hSlope := calculateSlope(ma24h, 3)

	// Check price above MA condition
	priceAboveMA := currentPrice > ma12h[len(ma12h)-1] && currentPrice > ma24h[len(ma24h)-1]

	// Calculate ADX and Hurst
	adx4h := calculateADX(data4h, 14)
	hurst := calculateHurst(data4h, 20)

	// Find prior swing high
	swingHigh := dc.findSwingHigh(data12h, dc.trendConfig.LookbackN)

	// Trend qualification logic
	trendStrong := (priceAboveMA || (ma12hSlope > 0 && ma24hSlope > 0)) &&
		(adx4h >= dc.trendConfig.ADX4hMin || hurst > dc.trendConfig.HurstMin)

	qualified := trendStrong && swingHigh != nil

	result := &TrendResult{
		Qualified:    qualified,
		MA12hSlope:   ma12hSlope,
		MA24hSlope:   ma24hSlope,
		PriceAboveMA: priceAboveMA,
		ADX4h:        adx4h,
		Hurst:        hurst,
		SwingHigh:    swingHigh,
	}

	if !qualified {
		if !trendStrong {
			result.Reason = "trend not strong enough (ADX/Hurst/MA criteria)"
		} else {
			result.Reason = "no valid swing high found"
		}
	}

	return result, nil
}

// IdentifyDip finds potential dip entry points
func (dc *DipCore) IdentifyDip(ctx context.Context, data1h []MarketData, trendResult *TrendResult) (*DipPoint, error) {
	if !trendResult.Qualified || len(data1h) < 20 {
		return nil, nil
	}

	// Calculate RSI and ATR
	rsi := calculateRSI(data1h, 14)
	atr := calculateATR(data1h, 14)

	// Look for local low after red bars or large down bar
	for i := len(data1h) - 1; i >= 3; i-- {
		current := data1h[i]

		// Check RSI in target range
		if rsi[i] < float64(dc.rsiConfig.LowMin) || rsi[i] > float64(dc.rsiConfig.LowMax) {
			continue
		}

		// Check for red bar sequence or large down move
		// Count red bars leading up to current position (not including current bar)
		redBarsCount := 0
		if i > 0 {
			redBarsCount = dc.countRedBars(data1h, i-1)
		}
		atrMultiple := math.Abs(current.Close-current.Open) / atr[i]

		isValidDip := redBarsCount >= 3 || atrMultiple >= 1.2

		if !isValidDip {
			continue
		}

		// Check Fibonacci retracement if we have swing high
		fibLevel := 0.0
		if trendResult.SwingHigh != nil {
			swingHigh := trendResult.SwingHigh.Price
			// Look much further back to find true prior swing low (before uptrend)
			swingLow := dc.findSwingLow(data1h, 20, 20) // Look in early data for swing low
			if swingLow > 0 {
				retracement := (swingHigh - current.Low) / (swingHigh - swingLow)
				fibLevel = retracement

				// Must be within Fib window
				if retracement < dc.fibConfig.Min || retracement > dc.fibConfig.Max {
					continue
				}
			}
		}

		// Check for positive divergence or bullish engulfing
		hasDivergence := dc.checkRSIDivergence(data1h, rsi, i)
		hasEngulfing := dc.checkBullishEngulfing(data1h, i)

		if hasDivergence || hasEngulfing {
			return &DipPoint{
				Index:         i,
				Price:         current.Low,
				Timestamp:     current.Timestamp,
				RSI:           rsi[i],
				FibLevel:      fibLevel,
				RedBarsCount:  redBarsCount,
				ATRMultiple:   atrMultiple,
				HasDivergence: hasDivergence,
				HasEngulfing:  hasEngulfing,
			}, nil
		}
	}

	return nil, nil
}

// Helper functions

func calculateSMA(data []MarketData, period int) []float64 {
	if len(data) < period {
		return nil
	}

	result := make([]float64, len(data)-period+1)
	for i := period - 1; i < len(data); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += data[j].Close
		}
		result[i-period+1] = sum / float64(period)
	}
	return result
}

func calculateSlope(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}

	start := len(values) - period
	end := len(values) - 1

	return (values[end] - values[start]) / float64(period)
}

func calculateRSI(data []MarketData, period int) []float64 {
	if len(data) < period+1 {
		return nil
	}

	gains := make([]float64, len(data)-1)
	losses := make([]float64, len(data)-1)

	for i := 1; i < len(data); i++ {
		change := data[i].Close - data[i-1].Close
		if change > 0 {
			gains[i-1] = change
		} else {
			losses[i-1] = -change
		}
	}

	result := make([]float64, len(data))

	// Calculate first RSI
	avgGain := 0.0
	avgLoss := 0.0
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		result[period] = 100
	} else {
		rs := avgGain / avgLoss
		result[period] = 100 - (100 / (1 + rs))
	}

	// Calculate subsequent RSI values
	alpha := 1.0 / float64(period)
	for i := period + 1; i < len(data); i++ {
		if gains[i-1] > 0 {
			avgGain = avgGain*(1-alpha) + gains[i-1]*alpha
		} else {
			avgGain = avgGain * (1 - alpha)
		}

		if losses[i-1] > 0 {
			avgLoss = avgLoss*(1-alpha) + losses[i-1]*alpha
		} else {
			avgLoss = avgLoss * (1 - alpha)
		}

		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - (100 / (1 + rs))
		}
	}

	return result
}

func calculateATR(data []MarketData, period int) []float64 {
	if len(data) < period+1 {
		return nil
	}

	tr := make([]float64, len(data)-1)
	for i := 1; i < len(data); i++ {
		current := data[i]
		previous := data[i-1]

		tr1 := current.High - current.Low
		tr2 := math.Abs(current.High - previous.Close)
		tr3 := math.Abs(current.Low - previous.Close)

		tr[i-1] = math.Max(tr1, math.Max(tr2, tr3))
	}

	result := make([]float64, len(data))

	// Calculate first ATR
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	result[period] = sum / float64(period)

	// Calculate subsequent ATR values using Wilder's smoothing
	alpha := 1.0 / float64(period)
	for i := period + 1; i < len(data); i++ {
		result[i] = result[i-1]*(1-alpha) + tr[i-1]*alpha
	}

	return result
}

func calculateADX(data []MarketData, period int) float64 {
	// Simplified ADX calculation
	if len(data) < period*2 {
		return 0
	}

	// Calculate directional movement
	plusDM := make([]float64, len(data)-1)
	minusDM := make([]float64, len(data)-1)
	tr := make([]float64, len(data)-1)

	for i := 1; i < len(data); i++ {
		current := data[i]
		previous := data[i-1]

		highDiff := current.High - previous.High
		lowDiff := previous.Low - current.Low

		if highDiff > lowDiff && highDiff > 0 {
			plusDM[i-1] = highDiff
		}
		if lowDiff > highDiff && lowDiff > 0 {
			minusDM[i-1] = lowDiff
		}

		tr1 := current.High - current.Low
		tr2 := math.Abs(current.High - previous.Close)
		tr3 := math.Abs(current.Low - previous.Close)
		tr[i-1] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Smooth directional indicators
	plusDI := smoothDirectional(plusDM, tr, period)
	minusDI := smoothDirectional(minusDM, tr, period)

	if len(plusDI) == 0 || len(minusDI) == 0 {
		return 0
	}

	// Calculate DX and ADX
	dx := make([]float64, len(plusDI))
	for i := 0; i < len(plusDI); i++ {
		sum := plusDI[i] + minusDI[i]
		if sum == 0 {
			dx[i] = 0
		} else {
			dx[i] = 100 * math.Abs(plusDI[i]-minusDI[i]) / sum
		}
	}

	// ADX is the smoothed DX
	if len(dx) < period {
		return 0
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += dx[i]
	}

	return sum / float64(period)
}

func smoothDirectional(dm, tr []float64, period int) []float64 {
	if len(dm) < period || len(tr) < period {
		return nil
	}

	result := make([]float64, len(dm)-period+1)

	// Calculate initial smoothed values
	dmSum := 0.0
	trSum := 0.0
	for i := 0; i < period; i++ {
		dmSum += dm[i]
		trSum += tr[i]
	}

	if trSum == 0 {
		result[0] = 0
	} else {
		result[0] = 100 * dmSum / trSum
	}

	// Calculate subsequent values using Wilder's smoothing
	alpha := 1.0 / float64(period)
	for i := 1; i < len(result); i++ {
		dmSum = dmSum*(1-alpha) + dm[period+i-1]*alpha
		trSum = trSum*(1-alpha) + tr[period+i-1]*alpha

		if trSum == 0 {
			result[i] = 0
		} else {
			result[i] = 100 * dmSum / trSum
		}
	}

	return result
}

func calculateHurst(data []MarketData, period int) float64 {
	if len(data) < period {
		return 0.5 // Default to random walk
	}

	// Simplified Hurst exponent calculation
	prices := make([]float64, len(data))
	for i, d := range data {
		prices[i] = d.Close
	}

	// Calculate log returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Use R/S analysis with different lag periods
	lags := []int{5, 10, 20}
	rsValues := make([]float64, len(lags))

	for i, lag := range lags {
		if lag >= len(returns) {
			rsValues[i] = 1.0
			continue
		}

		// Calculate mean
		mean := 0.0
		for j := 0; j < lag; j++ {
			mean += returns[len(returns)-lag+j]
		}
		mean /= float64(lag)

		// Calculate cumulative deviations
		cumDev := make([]float64, lag)
		cumDev[0] = returns[len(returns)-lag] - mean
		for j := 1; j < lag; j++ {
			cumDev[j] = cumDev[j-1] + (returns[len(returns)-lag+j] - mean)
		}

		// Find range
		minDev := cumDev[0]
		maxDev := cumDev[0]
		for _, dev := range cumDev {
			if dev < minDev {
				minDev = dev
			}
			if dev > maxDev {
				maxDev = dev
			}
		}

		// Calculate standard deviation
		variance := 0.0
		for j := 0; j < lag; j++ {
			diff := returns[len(returns)-lag+j] - mean
			variance += diff * diff
		}
		std := math.Sqrt(variance / float64(lag))

		// R/S ratio
		if std == 0 {
			rsValues[i] = 1.0
		} else {
			rsValues[i] = (maxDev - minDev) / std
		}
	}

	// Estimate Hurst exponent from slope of log(R/S) vs log(n)
	if len(rsValues) < 2 {
		return 0.5
	}

	// Simple linear regression on log values
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0
	n := float64(len(lags))

	for i, lag := range lags {
		x := math.Log(float64(lag))
		y := math.Log(rsValues[i])

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		return 0.5
	}

	hurst := (n*sumXY - sumX*sumY) / denominator

	// Clamp to reasonable range
	if hurst < 0 {
		hurst = 0
	}
	if hurst > 1 {
		hurst = 1
	}

	return hurst
}

func (dc *DipCore) findSwingHigh(data []MarketData, lookback int) *SwingPoint {
	if len(data) < 5 {
		return nil
	}

	highest := 0.0
	highestIdx := -1

	// Look for highest point in lookback period, excluding very recent bars
	end := len(data) - 2 // Don't include last 2 bars
	start := end - lookback
	if start < 2 {
		start = 2
	}

	for i := start; i <= end; i++ {
		if data[i].High > highest {
			// Check if it's a local high (optional neighbor check)
			isLocalHigh := true
			if i > 1 && i < len(data)-1 {
				// Prefer peaks that are higher than at least one neighbor
				isLocalHigh = data[i].High >= data[i-1].High || data[i].High >= data[i+1].High
			}

			if isLocalHigh {
				highest = data[i].High
				highestIdx = i
			}
		}
	}

	if highestIdx == -1 || highest == 0 {
		return nil
	}

	return &SwingPoint{
		Index:     highestIdx,
		Price:     highest,
		Timestamp: data[highestIdx].Timestamp,
	}
}

func (dc *DipCore) findSwingLow(data []MarketData, fromIndex, lookback int) float64 {
	start := fromIndex - lookback
	if start < 0 {
		start = 0
	}

	lowest := data[start].Low
	for i := start; i <= fromIndex; i++ {
		if data[i].Low < lowest {
			lowest = data[i].Low
		}
	}

	return lowest
}

func (dc *DipCore) countRedBars(data []MarketData, fromIndex int) int {
	count := 0
	for i := fromIndex; i >= 0 && count < 10; i-- {
		if data[i].Close < data[i].Open {
			count++
		} else {
			break
		}
	}
	return count
}

func (dc *DipCore) checkRSIDivergence(data []MarketData, rsi []float64, currentIndex int) bool {
	if currentIndex < dc.rsiConfig.DivConfirmBars {
		return false
	}

	// Look for price making lower low while RSI makes higher low
	priceWindow := dc.rsiConfig.DivConfirmBars

	currentLow := data[currentIndex].Low
	currentRSI := rsi[currentIndex]

	for i := currentIndex - 1; i >= currentIndex-priceWindow; i-- {
		if data[i].Low < currentLow && rsi[i] > currentRSI {
			return true // Positive divergence
		}
	}

	return false
}

func (dc *DipCore) checkBullishEngulfing(data []MarketData, currentIndex int) bool {
	if currentIndex >= len(data)-1 {
		return false
	}

	current := data[currentIndex]
	next := data[currentIndex+1]

	// Current bar is red, next bar is green and engulfs current
	return current.Close < current.Open &&
		next.Close > next.Open &&
		next.Open < current.Close &&
		next.Close > current.Open
}
