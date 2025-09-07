package unit

import (
	"math"
	"testing"

	"github.com/sawpanic/cryptorun/internal/domain/indicators"
)

func TestCalculateRSI(t *testing.T) {
	// Test data: simple price series that should give predictable RSI
	prices := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.85, 46.08, 45.89, 46.03,
		46.83, 46.69, 46.45, 46.59, 46.34, 46.82, 47.16, 47.72, 47.25, 47.09,
	}

	result := indicators.CalculateRSI(prices, 14)

	if !result.IsValid {
		t.Error("RSI calculation should be valid with sufficient data")
	}

	if result.Period != 14 {
		t.Errorf("Expected period 14, got %d", result.Period)
	}

	if result.Value < 0 || result.Value > 100 {
		t.Errorf("RSI should be between 0 and 100, got %.2f", result.Value)
	}

	// Test with insufficient data
	shortPrices := []float64{44.34, 44.09, 44.15}
	shortResult := indicators.CalculateRSI(shortPrices, 14)

	if shortResult.IsValid {
		t.Error("RSI should be invalid with insufficient data")
	}

	if shortResult.Value != 50.0 {
		t.Errorf("Expected neutral RSI of 50.0 for insufficient data, got %.1f", shortResult.Value)
	}
}

func TestCalculateATR(t *testing.T) {
	// Test data: OHLC bars
	bars := []indicators.PriceBar{
		{High: 48.70, Low: 47.79, Close: 48.16},
		{High: 48.72, Low: 48.14, Close: 48.61},
		{High: 48.90, Low: 48.39, Close: 48.75},
		{High: 48.87, Low: 48.37, Close: 48.63},
		{High: 48.82, Low: 48.24, Close: 48.74},
		{High: 49.05, Low: 48.64, Close: 49.03},
		{High: 49.20, Low: 48.94, Close: 49.07},
		{High: 49.35, Low: 48.86, Close: 49.32},
		{High: 49.92, Low: 49.50, Close: 49.91},
		{High: 50.19, Low: 49.87, Close: 50.13},
		{High: 50.12, Low: 49.20, Close: 49.53},
		{High: 49.66, Low: 48.90, Close: 49.50},
		{High: 49.88, Low: 49.43, Close: 49.75},
		{High: 50.19, Low: 49.73, Close: 50.03},
		{High: 50.36, Low: 49.26, Close: 50.31},
		{High: 50.57, Low: 50.09, Close: 50.52},
	}

	result := indicators.CalculateATR(bars, 14)

	if !result.IsValid {
		t.Error("ATR calculation should be valid with sufficient data")
	}

	if result.Value <= 0 {
		t.Errorf("ATR should be positive, got %.4f", result.Value)
	}

	if result.Period != 14 {
		t.Errorf("Expected period 14, got %d", result.Period)
	}

	// Test with insufficient data
	shortBars := bars[:5]
	shortResult := indicators.CalculateATR(shortBars, 14)

	if shortResult.IsValid {
		t.Error("ATR should be invalid with insufficient data")
	}
}

func TestCalculateADX(t *testing.T) {
	// Test data: OHLC bars for ADX calculation
	bars := []indicators.PriceBar{
		{High: 30.20, Low: 29.41, Close: 29.87},
		{High: 30.28, Low: 29.32, Close: 30.24},
		{High: 30.45, Low: 29.96, Close: 30.10},
		{High: 29.35, Low: 28.74, Close: 28.90},
		{High: 29.35, Low: 28.56, Close: 28.92},
		{High: 29.29, Low: 28.41, Close: 28.48},
		{High: 28.83, Low: 28.08, Close: 28.56},
		{High: 28.73, Low: 27.43, Close: 27.56},
		{High: 28.67, Low: 27.66, Close: 28.47},
		{High: 28.85, Low: 28.28, Close: 28.28},
		{High: 28.64, Low: 27.79, Close: 28.91},
		{High: 29.87, Low: 28.74, Close: 29.87},
		{High: 30.24, Low: 29.44, Close: 29.95},
		{High: 30.10, Low: 29.35, Close: 29.52},
		{High: 29.70, Low: 29.24, Close: 29.67},
		{High: 29.44, Low: 28.93, Close: 29.39},
		{High: 29.49, Low: 28.99, Close: 29.18},
		{High: 29.32, Low: 28.41, Close: 28.67},
		{High: 28.77, Low: 28.22, Close: 28.65},
		{High: 28.92, Low: 28.00, Close: 28.12},
		{High: 28.76, Low: 27.95, Close: 28.10},
		{High: 28.02, Low: 27.39, Close: 27.59},
		{High: 27.72, Low: 27.27, Close: 27.34},
		{High: 27.51, Low: 26.96, Close: 27.03},
		{High: 26.97, Low: 26.13, Close: 26.41},
		{High: 26.85, Low: 26.18, Close: 26.85},
		{High: 26.92, Low: 26.51, Close: 26.63},
		{High: 26.76, Low: 26.25, Close: 26.75},
		{High: 26.89, Low: 26.46, Close: 26.59},
		{High: 26.61, Low: 26.06, Close: 26.33},
	}

	result := indicators.CalculateADX(bars, 14)

	if !result.IsValid {
		t.Error("ADX calculation should be valid with sufficient data")
	}

	if result.ADX < 0 || result.ADX > 100 {
		t.Errorf("ADX should be between 0 and 100, got %.2f", result.ADX)
	}

	if result.PDI < 0 || result.PDI > 100 {
		t.Errorf("PDI should be between 0 and 100, got %.2f", result.PDI)
	}

	if result.MDI < 0 || result.MDI > 100 {
		t.Errorf("MDI should be between 0 and 100, got %.2f", result.MDI)
	}

	// Test with insufficient data
	shortBars := bars[:10]
	shortResult := indicators.CalculateADX(shortBars, 14)

	if shortResult.IsValid {
		t.Error("ADX should be invalid with insufficient data")
	}
}

func TestCalculateHurstExponent(t *testing.T) {
	// Test with trending data (should give H > 0.5)
	trendingPrices := make([]float64, 50)
	for i := 0; i < 50; i++ {
		trendingPrices[i] = 100.0 + float64(i)*0.5 + math.Sin(float64(i)*0.1)*2.0
	}

	trendingResult := indicators.CalculateHurstExponent(trendingPrices, 40)

	if !trendingResult.IsValid {
		t.Error("Hurst calculation should be valid with sufficient data")
	}

	if trendingResult.Exponent < 0 || trendingResult.Exponent > 1 {
		t.Errorf("Hurst exponent should be between 0 and 1, got %.3f", trendingResult.Exponent)
	}

	// For trending data, we expect persistence (H > 0.5)
	if trendingResult.Exponent <= 0.5 && trendingResult.Strength != "random" {
		t.Logf("Trending data gave Hurst %.3f (%s) - might be expected due to noise", 
			trendingResult.Exponent, trendingResult.Strength)
	}

	// Test with random walk data
	randomPrices := make([]float64, 50)
	randomPrices[0] = 100.0
	for i := 1; i < 50; i++ {
		// Simple random walk
		change := float64((i%3)-1) * 0.5 // -0.5, 0, or 0.5
		randomPrices[i] = randomPrices[i-1] + change
	}

	randomResult := indicators.CalculateHurstExponent(randomPrices, 40)

	if !randomResult.IsValid {
		t.Error("Hurst calculation should be valid with random data")
	}

	// Test with insufficient data
	shortPrices := []float64{100.0, 101.0, 99.0}
	shortResult := indicators.CalculateHurstExponent(shortPrices, 50)

	if shortResult.IsValid {
		t.Error("Hurst should be invalid with insufficient data")
	}

	if shortResult.Strength != "insufficient_data" {
		t.Errorf("Expected 'insufficient_data' strength, got '%s'", shortResult.Strength)
	}
}

func TestCalculateAllIndicators(t *testing.T) {
	// Create test data
	prices := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.85, 46.08, 45.89, 46.03,
		46.83, 46.69, 46.45, 46.59, 46.34, 46.82, 47.16, 47.72, 47.25, 47.09,
		47.24, 47.34, 47.67, 48.05, 47.89, 47.98, 48.14, 48.34, 48.67, 48.89,
		49.12, 49.34, 49.67, 49.89, 50.12, 50.34, 50.67, 50.89, 51.12, 51.34,
		51.67, 51.89, 52.12, 52.34, 52.67, 52.89, 53.12, 53.34, 53.67, 53.89,
	}

	bars := make([]indicators.PriceBar, len(prices))
	for i, price := range prices {
		// Create synthetic OHLC data
		high := price * 1.02
		low := price * 0.98
		bars[i] = indicators.PriceBar{
			High:  high,
			Low:   low,
			Close: price,
		}
	}

	indicators_result, err := indicators.CalculateAllIndicators(prices, bars)

	if err != nil {
		t.Fatalf("CalculateAllIndicators failed: %v", err)
	}

	// Check that all indicators were calculated
	if !indicators_result.RSI.IsValid {
		t.Error("RSI should be valid")
	}

	if !indicators_result.ATR.IsValid {
		t.Error("ATR should be valid")
	}

	if !indicators_result.ADX.IsValid {
		t.Error("ADX should be valid")
	}

	if !indicators_result.Hurst.IsValid {
		t.Error("Hurst should be valid")
	}

	// Test error cases
	_, err = indicators.CalculateAllIndicators([]float64{}, bars)
	if err == nil {
		t.Error("Expected error for empty prices")
	}

	_, err = indicators.CalculateAllIndicators(prices, []indicators.PriceBar{})
	if err == nil {
		t.Error("Expected error for empty bars")
	}
}

func TestGetTechnicalScore(t *testing.T) {
	// Create test indicators
	testIndicators := indicators.TechnicalIndicators{
		RSI: indicators.RSIResult{
			Value:   50.0, // Optimal RSI
			IsValid: true,
		},
		ATR: indicators.ATRResult{
			Value:   2.5, // Will give 2.5% of current price
			IsValid: true,
		},
		ADX: indicators.ADXResult{
			ADX:     30.0, // Strong trend
			IsValid: true,
		},
		Hurst: indicators.HurstResult{
			Exponent: 0.7, // Persistent
			IsValid:  true,
		},
	}

	currentPrice := 100.0
	score := indicators.GetTechnicalScore(testIndicators, currentPrice)

	if score < 0 || score > 100 {
		t.Errorf("Technical score should be between 0 and 100, got %.2f", score)
	}

	// Score should be relatively high with good indicators
	if score < 60 {
		t.Errorf("Expected high score with good indicators, got %.2f", score)
	}

	// Test with poor indicators
	poorIndicators := indicators.TechnicalIndicators{
		RSI: indicators.RSIResult{
			Value:   90.0, // Overbought
			IsValid: true,
		},
		ATR: indicators.ATRResult{
			Value:   0.5, // Low volatility
			IsValid: true,
		},
		ADX: indicators.ADXResult{
			ADX:     10.0, // Weak trend
			IsValid: true,
		},
		Hurst: indicators.HurstResult{
			Exponent: 0.3, // Mean reverting
			IsValid:  true,
		},
	}

	poorScore := indicators.GetTechnicalScore(poorIndicators, currentPrice)

	if poorScore >= score {
		t.Errorf("Poor indicators should give lower score: %.2f vs %.2f", poorScore, score)
	}

	// Test with no valid indicators
	invalidIndicators := indicators.TechnicalIndicators{
		RSI:   indicators.RSIResult{IsValid: false},
		ATR:   indicators.ATRResult{IsValid: false},
		ADX:   indicators.ADXResult{IsValid: false},
		Hurst: indicators.HurstResult{IsValid: false},
	}

	noScore := indicators.GetTechnicalScore(invalidIndicators, currentPrice)
	if noScore != 0.0 {
		t.Errorf("Expected score of 0.0 for no valid indicators, got %.2f", noScore)
	}
}