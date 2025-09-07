package march_aug

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// GateEvaluatorImpl implements the GateEvaluator interface
type GateEvaluatorImpl struct {
	rsiCalculator   *RSICalculator
	adxCalculator   *ADXCalculator
	hurstCalculator *HurstCalculator
}

// NewGateEvaluator creates a new gate evaluator with technical indicators
func NewGateEvaluator() *GateEvaluatorImpl {
	return &GateEvaluatorImpl{
		rsiCalculator:   NewRSICalculator(14),   // 14-period RSI
		adxCalculator:   NewADXCalculator(14),   // 14-period ADX
		hurstCalculator: NewHurstCalculator(20), // 20-period Hurst
	}
}

// EvaluateGates performs comprehensive gate evaluation for entry signals
func (g *GateEvaluatorImpl) EvaluateGates(scores CompositeScores, market MarketData) (EntryGates, error) {
	var failReasons []string
	gateScores := make(map[string]float64)

	// Gate 1: Composite Score >= 75
	compositeGate := scores.FinalScore >= 75.0
	gateScores["composite_score"] = scores.FinalScore
	if !compositeGate {
		failReasons = append(failReasons, fmt.Sprintf("composite score %.1f < 75", scores.FinalScore))
	}

	// Gate 2: Movement >= 2.5% (4h) or fallback to 24h
	movementGate, movement4h, movement24h := g.evaluateMovementGate(market)
	gateScores["movement_4h"] = movement4h * 100 // Convert to percentage
	gateScores["movement_24h"] = movement24h * 100
	if !movementGate {
		failReasons = append(failReasons, fmt.Sprintf("movement 4h: %.1f%%, 24h: %.1f%% both < 2.5%%",
			movement4h*100, movement24h*100))
	}

	// Gate 3: Volume Surge >= 1.8x average
	volumeSurgeGate, surgeMultiple := g.evaluateVolumeSurgeGate(market)
	gateScores["volume_surge"] = surgeMultiple
	if !volumeSurgeGate {
		failReasons = append(failReasons, fmt.Sprintf("volume surge %.1fx < 1.8x", surgeMultiple))
	}

	// Gate 4: Liquidity >= $500k 24h volume
	liquidityGate, volume24h := g.evaluateLiquidityGate(market)
	gateScores["liquidity_24h"] = volume24h
	if !liquidityGate {
		failReasons = append(failReasons, fmt.Sprintf("24h volume $%.0fk < $500k", volume24h/1000))
	}

	// Gate 5: Trend Gate - ADX >= 25 OR Hurst > 0.55
	trendGate, adxValue, hurstValue := g.evaluateTrendGate(market)
	gateScores["adx"] = adxValue
	gateScores["hurst"] = hurstValue
	if !trendGate {
		failReasons = append(failReasons, fmt.Sprintf("trend: ADX %.1f < 25 AND Hurst %.3f <= 0.55",
			adxValue, hurstValue))
	}

	// Gate 6: Fatigue Gate - block if 24h > +12% & RSI4h > 70 unless acceleration > 0
	fatigueGate, rsiValue, accelValue := g.evaluateFatigueGate(market)
	gateScores["rsi_4h"] = rsiValue
	gateScores["acceleration"] = accelValue
	if !fatigueGate {
		failReasons = append(failReasons, "fatigue: 24h > +12% & RSI4h > 70 & acceleration <= 0")
	}

	// Gate 7: Freshness Gate - <= 2 bars old & late-fill < 30s
	freshnessGate, barsAge, lateFillTime := g.evaluateFreshnessGate(market)
	gateScores["bars_age"] = float64(barsAge)
	gateScores["late_fill_seconds"] = lateFillTime.Seconds()
	if !freshnessGate {
		failReasons = append(failReasons, fmt.Sprintf("freshness: %d bars > 2 OR late-fill %.0fs >= 30s",
			barsAge, lateFillTime.Seconds()))
	}

	// Overall pass requires all gates
	overallPass := compositeGate && movementGate && volumeSurgeGate &&
		liquidityGate && trendGate && fatigueGate && freshnessGate

	return EntryGates{
		Symbol:          scores.Symbol,
		Timestamp:       scores.Timestamp,
		CompositeGate:   compositeGate,
		MovementGate:    movementGate,
		VolumeSurgeGate: volumeSurgeGate,
		LiquidityGate:   liquidityGate,
		TrendGate:       trendGate,
		FatigueGate:     fatigueGate,
		FreshnessGate:   freshnessGate,
		OverallPass:     overallPass,
		FailReasons:     failReasons,
		GateScores:      gateScores,
	}, nil
}

// evaluateMovementGate checks for >= 2.5% movement in 4h or 24h fallback
func (g *GateEvaluatorImpl) evaluateMovementGate(market MarketData) (bool, float64, float64) {
	// Mock implementation - in production would use historical price data
	// Simulate 4h and 24h price movements
	movement4h := (rand.Float64() - 0.3) * 0.1   // -3% to +7% range
	movement24h := (rand.Float64() - 0.2) * 0.15 // -2% to +13% range

	movement4hAbs := math.Abs(movement4h)
	movement24hAbs := math.Abs(movement24h)

	// Pass if either 4h or 24h movement >= 2.5%
	passed := movement4hAbs >= 0.025 || movement24hAbs >= 0.025

	return passed, movement4h, movement24h
}

// evaluateVolumeSurgeGate checks for >= 1.8x average volume
func (g *GateEvaluatorImpl) evaluateVolumeSurgeGate(market MarketData) (bool, float64) {
	// Mock average volume calculation
	avgVolume := market.Volume * (0.5 + rand.Float64()*0.8) // 50%-130% of current
	surgeMultiple := market.Volume / avgVolume

	passed := surgeMultiple >= 1.8
	return passed, surgeMultiple
}

// evaluateLiquidityGate checks for >= $500k 24h volume
func (g *GateEvaluatorImpl) evaluateLiquidityGate(market MarketData) (bool, float64) {
	// Calculate 24h volume in USD (mock calculation)
	volume24h := market.Volume * market.Close * 24 // Approximate 24h volume

	passed := volume24h >= 500000 // $500k threshold
	return passed, volume24h
}

// evaluateTrendGate checks ADX >= 25 OR Hurst > 0.55
func (g *GateEvaluatorImpl) evaluateTrendGate(market MarketData) (bool, float64, float64) {
	// Mock technical indicator calculations
	adxValue := 15 + rand.Float64()*25     // ADX range 15-40
	hurstValue := 0.4 + rand.Float64()*0.3 // Hurst range 0.4-0.7

	passed := adxValue >= 25.0 || hurstValue > 0.55
	return passed, adxValue, hurstValue
}

// evaluateFatigueGate checks for overextended conditions
func (g *GateEvaluatorImpl) evaluateFatigueGate(market MarketData) (bool, float64, float64) {
	// Mock 24h return and RSI calculation
	return24h := (rand.Float64() - 0.3) * 0.2 // -6% to +14% range
	rsiValue := 30 + rand.Float64()*40        // RSI range 30-70

	// Mock acceleration calculation (price momentum change)
	accelValue := (rand.Float64() - 0.5) * 0.02 // -1% to +1% acceleration

	// Fatigue condition: 24h > +12% AND RSI4h > 70 AND acceleration <= 0
	fatigued := return24h > 0.12 && rsiValue > 70.0 && accelValue <= 0

	passed := !fatigued // Pass if not fatigued
	return passed, rsiValue, accelValue
}

// evaluateFreshnessGate checks signal freshness and fill timing
func (g *GateEvaluatorImpl) evaluateFreshnessGate(market MarketData) (bool, int, time.Duration) {
	// Mock bars age (in production would track signal generation time)
	barsAge := rand.Intn(4) // 0-3 bars old

	// Mock late-fill time (time between signal and potential fill)
	lateFillSeconds := rand.Float64() * 60 // 0-60 seconds
	lateFillTime := time.Duration(lateFillSeconds) * time.Second

	// Pass if <= 2 bars old AND late-fill < 30s
	passed := barsAge <= 2 && lateFillTime < 30*time.Second

	return passed, barsAge, lateFillTime
}

// Technical Indicator Calculators

// RSICalculator computes Relative Strength Index
type RSICalculator struct {
	period int
}

func NewRSICalculator(period int) *RSICalculator {
	return &RSICalculator{period: period}
}

func (r *RSICalculator) Calculate(prices []float64) float64 {
	if len(prices) < r.period+1 {
		return 50.0 // Neutral RSI
	}

	gains := 0.0
	losses := 0.0

	// Calculate average gains and losses
	for i := 1; i <= r.period; i++ {
		change := prices[len(prices)-i] - prices[len(prices)-i-1]
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}

	avgGain := gains / float64(r.period)
	avgLoss := losses / float64(r.period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// ADXCalculator computes Average Directional Index
type ADXCalculator struct {
	period int
}

func NewADXCalculator(period int) *ADXCalculator {
	return &ADXCalculator{period: period}
}

func (a *ADXCalculator) Calculate(highs, lows, closes []float64) float64 {
	if len(highs) < a.period+1 || len(lows) < a.period+1 || len(closes) < a.period+1 {
		return 20.0 // Neutral ADX
	}

	// Simplified ADX calculation
	var trueRanges []float64
	var plusDMs []float64
	var minusDMs []float64

	for i := 1; i < len(closes); i++ {
		// True Range
		tr1 := highs[i] - lows[i]
		tr2 := math.Abs(highs[i] - closes[i-1])
		tr3 := math.Abs(lows[i] - closes[i-1])
		tr := math.Max(tr1, math.Max(tr2, tr3))
		trueRanges = append(trueRanges, tr)

		// Directional Movement
		plusDM := 0.0
		minusDM := 0.0

		if highs[i]-highs[i-1] > lows[i-1]-lows[i] && highs[i]-highs[i-1] > 0 {
			plusDM = highs[i] - highs[i-1]
		}
		if lows[i-1]-lows[i] > highs[i]-highs[i-1] && lows[i-1]-lows[i] > 0 {
			minusDM = lows[i-1] - lows[i]
		}

		plusDMs = append(plusDMs, plusDM)
		minusDMs = append(minusDMs, minusDM)
	}

	// Simple average for demonstration
	if len(trueRanges) < a.period {
		return 20.0
	}

	avgTR := a.average(trueRanges[len(trueRanges)-a.period:])
	avgPlusDM := a.average(plusDMs[len(plusDMs)-a.period:])
	avgMinusDM := a.average(minusDMs[len(minusDMs)-a.period:])

	if avgTR == 0 {
		return 0.0
	}

	plusDI := 100 * avgPlusDM / avgTR
	minusDI := 100 * avgMinusDM / avgTR

	if plusDI+minusDI == 0 {
		return 0.0
	}

	dx := 100 * math.Abs(plusDI-minusDI) / (plusDI + minusDI)
	return dx
}

func (a *ADXCalculator) average(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// HurstCalculator computes Hurst exponent for trend persistence
type HurstCalculator struct {
	period int
}

func NewHurstCalculator(period int) *HurstCalculator {
	return &HurstCalculator{period: period}
}

func (h *HurstCalculator) Calculate(prices []float64) float64 {
	if len(prices) < h.period {
		return 0.5 // Random walk
	}

	// Simplified R/S analysis for Hurst exponent
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] != 0 {
			returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
		}
	}

	if len(returns) == 0 {
		return 0.5
	}

	// Calculate mean return
	meanReturn := 0.0
	for _, ret := range returns {
		meanReturn += ret
	}
	meanReturn /= float64(len(returns))

	// Calculate cumulative deviations
	cumDev := make([]float64, len(returns))
	cumDev[0] = returns[0] - meanReturn
	for i := 1; i < len(returns); i++ {
		cumDev[i] = cumDev[i-1] + returns[i] - meanReturn
	}

	// Calculate range
	maxCum := cumDev[0]
	minCum := cumDev[0]
	for _, dev := range cumDev {
		if dev > maxCum {
			maxCum = dev
		}
		if dev < minCum {
			minCum = dev
		}
	}

	rangeRS := maxCum - minCum

	// Calculate standard deviation
	variance := 0.0
	for _, ret := range returns {
		variance += math.Pow(ret-meanReturn, 2)
	}
	variance /= float64(len(returns) - 1)
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0.5
	}

	// R/S ratio
	rsRatio := rangeRS / stdDev

	// Hurst exponent approximation
	if rsRatio <= 0 {
		return 0.5
	}

	hurst := math.Log(rsRatio) / math.Log(float64(len(returns)))

	// Clamp to reasonable range
	if hurst < 0 {
		hurst = 0.0
	}
	if hurst > 1 {
		hurst = 1.0
	}

	return hurst
}

// RegimeDetector for market regime classification
type RegimeDetector struct{}

func NewRegimeDetector() *RegimeDetector {
	return &RegimeDetector{}
}

func (r *RegimeDetector) DetectRegime(timestamp time.Time, marketData []MarketData) RegimeData {
	// Mock regime detection based on volatility and trends
	// In production, would use breadth thrust, realized vol 7d, % above 20MA

	// Calculate recent volatility
	volatility := r.calculateVolatility(marketData)

	// Calculate trend strength
	trendStrength := r.calculateTrendStrength(marketData)

	// Determine regime
	regime := "choppy" // Default
	regimeNumeric := 0.0
	confidence := 0.7

	if volatility < 0.03 && trendStrength > 0.6 {
		regime = "trending_bull"
		regimeNumeric = 1.0
		confidence = 0.85
	} else if volatility > 0.06 {
		regime = "high_vol"
		regimeNumeric = 2.0
		confidence = 0.75
	}

	// Mock indicators
	breadthThrust := 0.5 + (rand.Float64()-0.5)*0.4 // 0.3-0.7
	realizedVol7d := volatility
	aboveMA20Pct := 0.4 + rand.Float64()*0.4 // 0.4-0.8

	return RegimeData{
		Timestamp:     timestamp,
		BreadthThrust: breadthThrust,
		RealizedVol7d: realizedVol7d,
		AboveMA20Pct:  aboveMA20Pct,
		Regime:        regime,
		RegimeNumeric: regimeNumeric,
		Confidence:    confidence,
	}
}

func (r *RegimeDetector) calculateVolatility(data []MarketData) float64 {
	if len(data) < 2 {
		return 0.04 // Default volatility
	}

	var returns []float64
	for i := 1; i < len(data) && i < 25; i++ { // Use up to 24 hours
		if data[i-1].Close > 0 {
			ret := (data[i].Close - data[i-1].Close) / data[i-1].Close
			returns = append(returns, ret)
		}
	}

	if len(returns) == 0 {
		return 0.04
	}

	// Calculate standard deviation
	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, ret := range returns {
		variance += math.Pow(ret-mean, 2)
	}
	variance /= float64(len(returns) - 1)

	return math.Sqrt(variance)
}

func (r *RegimeDetector) calculateTrendStrength(data []MarketData) float64 {
	if len(data) < 10 {
		return 0.5 // Neutral
	}

	// Simple trend strength based on price direction consistency
	upMoves := 0
	totalMoves := 0

	for i := 1; i < len(data) && i < 25; i++ {
		if data[i].Close > data[i-1].Close {
			upMoves++
		}
		totalMoves++
	}

	if totalMoves == 0 {
		return 0.5
	}

	return float64(upMoves) / float64(totalMoves)
}
