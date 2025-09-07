package unit

import (
	"context"
	"testing"

	"github.com/sawpanic/cryptorun/internal/algo/dip"
	"github.com/sawpanic/cryptorun/internal/scan/sim"
)

func TestDipCore_DebugUptrendScenario(t *testing.T) {
	// Create uptrend scenario
	dataProvider := sim.CreateUptrendScenario("BTCUSD")

	// Create dip core with test config
	config := dip.TrendConfig{
		MALen12h:  10,
		MALen24h:  10,
		ADX4hMin:  20.0,
		HurstMin:  0.50,
		LookbackN: 15,
	}
	fibConfig := dip.FibConfig{Min: 0.35, Max: 0.65}
	rsiConfig := dip.RSIConfig{LowMin: 25, LowMax: 40, DivConfirmBars: 3}

	dipCore := dip.NewDipCore(config, fibConfig, rsiConfig)

	// Get data
	ctx := context.Background()
	data1h, err := dataProvider.GetMarketData(ctx, "BTCUSD", "1h", 100)
	if err != nil {
		t.Fatalf("Failed to get 1h data: %v", err)
	}
	data12h, err := dataProvider.GetMarketData(ctx, "BTCUSD", "12h", 50)
	if err != nil {
		t.Fatalf("Failed to get 12h data: %v", err)
	}
	data24h, err := dataProvider.GetMarketData(ctx, "BTCUSD", "24h", 50)
	if err != nil {
		t.Fatalf("Failed to get 24h data: %v", err)
	}
	data4h, err := dataProvider.GetMarketData(ctx, "BTCUSD", "4h", 50)
	if err != nil {
		t.Fatalf("Failed to get 4h data: %v", err)
	}

	t.Logf("Data lengths: 1h=%d, 12h=%d, 24h=%d, 4h=%d",
		len(data1h), len(data12h), len(data24h), len(data4h))

	if len(data1h) == 0 {
		t.Fatal("No 1h data available")
	}

	currentPrice := data1h[len(data1h)-1].Close
	t.Logf("Current price: %.2f", currentPrice)

	// Test trend qualification
	trendResult, err := dipCore.QualifyTrend(ctx, data12h, data24h, data4h, currentPrice)
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}

	t.Logf("Trend qualification:")
	t.Logf("  Qualified: %v", trendResult.Qualified)
	t.Logf("  Reason: %s", trendResult.Reason)
	t.Logf("  MA12h slope: %.4f", trendResult.MA12hSlope)
	t.Logf("  MA24h slope: %.4f", trendResult.MA24hSlope)
	t.Logf("  Price above MA: %v", trendResult.PriceAboveMA)
	t.Logf("  ADX 4h: %.2f", trendResult.ADX4h)
	t.Logf("  Hurst: %.3f", trendResult.Hurst)
	if trendResult.SwingHigh != nil {
		t.Logf("  Swing high: %.2f at index %d", trendResult.SwingHigh.Price, trendResult.SwingHigh.Index)
	} else {
		t.Log("  Swing high: nil")
	}

	if !trendResult.Qualified {
		t.Log("Trend not qualified - stopping here")
		return
	}

	// Test dip identification
	dipPoint, err := dipCore.IdentifyDip(ctx, data1h, trendResult)
	if err != nil {
		t.Fatalf("IdentifyDip failed: %v", err)
	}

	if dipPoint == nil {
		t.Log("No dip point found")

		// Debug RSI values
		rsi := calculateRSI(data1h, 14)
		t.Log("Last 10 RSI values:")
		start := len(rsi) - 10
		if start < 0 {
			start = 0
		}
		for i := start; i < len(rsi); i++ {
			t.Logf("  [%d] RSI=%.1f, Close=%.2f", i, rsi[i], data1h[i].Close)
		}

		return
	}

	t.Logf("Dip point found:")
	t.Logf("  Index: %d", dipPoint.Index)
	t.Logf("  Price: %.2f", dipPoint.Price)
	t.Logf("  RSI: %.1f", dipPoint.RSI)
	t.Logf("  Fib level: %.2f", dipPoint.FibLevel)
	t.Logf("  Red bars: %d", dipPoint.RedBarsCount)
	t.Logf("  ATR multiple: %.2f", dipPoint.ATRMultiple)
	t.Logf("  Has divergence: %v", dipPoint.HasDivergence)
	t.Logf("  Has engulfing: %v", dipPoint.HasEngulfing)
}

// Helper function copied from core.go for debugging
func calculateRSI(data []dip.MarketData, period int) []float64 {
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
