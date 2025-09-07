package unit

import (
	"context"
	"math"
	"testing"
	"time"

	"cryptorun/internal/algo/dip"
)

func TestDipCore_QualifyTrend_StrongUptrend(t *testing.T) {
	config := dip.TrendConfig{
		MALen12h:  10,
		MALen24h:  10,
		ADX4hMin:  25.0,
		HurstMin:  0.55,
		LookbackN: 5,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	// Create strong uptrend data
	data12h := createUptrendData(20, 0.02) // 2% per period
	data24h := createUptrendData(15, 0.03) // 3% per period
	data4h := createStrongTrendData(30, 0.01) // For ADX calculation
	
	currentPrice := 120.0 // Above MA
	
	result, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	if !result.Qualified {
		t.Errorf("Strong uptrend should qualify, but got: %s", result.Reason)
	}
	
	// Verify trend metrics
	if !result.PriceAboveMA {
		t.Error("Price should be above MA in strong uptrend")
	}
	
	if result.MA12hSlope <= 0 {
		t.Errorf("12h MA slope should be positive, got: %f", result.MA12hSlope)
	}
	
	if result.MA24hSlope <= 0 {
		t.Errorf("24h MA slope should be positive, got: %f", result.MA24hSlope)
	}
	
	if result.SwingHigh == nil {
		t.Error("Should find swing high in trending market")
	}
	
	t.Logf("✅ Strong uptrend qualified: ADX=%.1f, Hurst=%.2f", result.ADX4h, result.Hurst)
}

func TestDipCore_QualifyTrend_SidewaysMarket(t *testing.T) {
	config := dip.TrendConfig{
		MALen12h:  10,
		MALen24h:  10,
		ADX4hMin:  25.0,
		HurstMin:  0.55,
		LookbackN: 5,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	// Create sideways/choppy data
	data12h := createSidewaysData(20, 0.05) // 5% noise, no trend
	data24h := createSidewaysData(15, 0.04) // 4% noise, no trend
	data4h := createSidewaysData(30, 0.03)  // Choppy for low ADX
	
	currentPrice := 100.0 // Around MA level
	
	result, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	if result.Qualified {
		t.Error("Sideways market should not qualify for dip trading")
	}
	
	// Should fail on trend strength criteria
	if result.Reason == "" {
		t.Error("Should provide reason for disqualification")
	}
	
	t.Logf("✅ Sideways market rejected: %s (ADX=%.1f, Hurst=%.2f)", 
		result.Reason, result.ADX4h, result.Hurst)
}

func TestDipCore_QualifyTrend_InsufficientData(t *testing.T) {
	config := dip.TrendConfig{
		MALen12h:  50, // Require more data than available
		MALen24h:  50,
		ADX4hMin:  25.0,
		HurstMin:  0.55,
		LookbackN: 5,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	// Insufficient data
	data12h := createUptrendData(10, 0.02) // Only 10 points, need 50
	data24h := createUptrendData(10, 0.03)
	data4h := createStrongTrendData(10, 0.01)
	
	currentPrice := 110.0
	
	result, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	if result.Qualified {
		t.Error("Should not qualify with insufficient data")
	}
	
	expectedReason := "insufficient data for MA calculation"
	if result.Reason != expectedReason {
		t.Errorf("Expected reason '%s', got '%s'", expectedReason, result.Reason)
	}
	
	t.Logf("✅ Insufficient data handled correctly")
}

func TestDipCore_QualifyTrend_WeakTrendStrongMAs(t *testing.T) {
	// Test case: price above MAs but ADX/Hurst too low
	config := dip.TrendConfig{
		MALen12h:  10,
		MALen24h:  10,
		ADX4hMin:  30.0, // High requirement
		HurstMin:  0.65, // High requirement
		LookbackN: 5,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	data12h := createUptrendData(20, 0.005) // Very slow uptrend
	data24h := createUptrendData(15, 0.005)
	data4h := createSidewaysData(30, 0.02)  // Choppy = low ADX/Hurst
	
	currentPrice := 110.0 // Above MA
	
	result, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	if result.Qualified {
		t.Error("Weak trend should not qualify despite price above MA")
	}
	
	// Should be price above MA but weak trend strength
	if !result.PriceAboveMA {
		t.Error("Price should be detected as above MA")
	}
	
	if result.ADX4h >= config.ADX4hMin && result.Hurst > config.HurstMin {
		t.Error("ADX/Hurst should be below thresholds")
	}
	
	t.Logf("✅ Weak trend correctly rejected despite MA position")
}

func TestDipCore_QualifyTrend_MALevelsVsSlopes(t *testing.T) {
	// Test both level-based (price > MA) and slope-based qualification
	config := dip.TrendConfig{
		MALen12h:  10,
		MALen24h:  10,
		ADX4hMin:  20.0, // Lower threshold
		HurstMin:  0.50, // Lower threshold
		LookbackN: 5,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	// Test Case 1: Price below MA but strong positive slopes
	data12h := createUptrendData(20, 0.03) // Strong trend
	data24h := createUptrendData(15, 0.03)
	data4h := createStrongTrendData(30, 0.02)
	
	currentPrice := 95.0 // Below MA but slopes are positive
	
	result1, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	// Should qualify due to positive slopes even if price below MA
	if !result1.Qualified {
		t.Error("Should qualify with strong positive MA slopes")
	}
	
	if result1.PriceAboveMA {
		t.Error("Price should be detected as below MA")
	}
	
	// Test Case 2: Price above MA but flat/negative slopes  
	data12h_flat := createSidewaysData(20, 0.02) // Flat trend
	data24h_flat := createSidewaysData(15, 0.02)
	
	currentPrice2 := 105.0 // Above MA level
	
	result2, err := core.QualifyTrend(context.Background(), data12h_flat, data24h_flat, data4h, currentPrice2)
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	// Should qualify if price above MA, even with flat slopes
	if !result2.Qualified {
		t.Error("Should qualify when price is above MA levels")
	}
	
	if !result2.PriceAboveMA {
		t.Error("Price should be detected as above MA")
	}
	
	t.Logf("✅ Both MA level and slope criteria working correctly")
}

func TestDipCore_QualifyTrend_SwingHighDetection(t *testing.T) {
	config := dip.TrendConfig{
		MALen12h:  5,
		MALen24h:  5,
		ADX4hMin:  20.0,
		HurstMin:  0.50,
		LookbackN: 10,
	}
	
	core := dip.NewDipCore(config, dip.FibConfig{}, dip.RSIConfig{})
	
	// Create data with clear swing high pattern
	data12h := make([]dip.MarketData, 15)
	baseTime := time.Now().Add(-15 * 12 * time.Hour)
	
	for i := 0; i < 15; i++ {
		var price float64
		if i < 8 {
			price = 100 + float64(i)*2 // Uptrend to 114
		} else if i == 8 {
			price = 116 // Peak (swing high)
		} else {
			price = 115 - float64(i-8)*0.5 // Slight decline
		}
		
		data12h[i] = dip.MarketData{
			Timestamp: baseTime.Add(time.Duration(i) * 12 * time.Hour),
			Open:      price * 0.99,
			High:      price * 1.01,
			Low:       price * 0.98,
			Close:     price,
			Volume:    1000000,
		}
	}
	
	data24h := createUptrendData(10, 0.02)
	data4h := createStrongTrendData(20, 0.01)
	
	currentPrice := 114.0
	
	result, err := core.QualifyTrend(context.Background(), data12h, data24h, data4h, currentPrice)
	if err != nil {
		t.Fatalf("QualifyTrend failed: %v", err)
	}
	
	if !result.Qualified {
		t.Error("Should qualify with clear swing high pattern")
	}
	
	if result.SwingHigh == nil {
		t.Fatal("Should detect swing high")
	}
	
	// Swing high should be at index 8 with price 116
	if result.SwingHigh.Index != 8 {
		t.Errorf("Swing high index should be 8, got: %d", result.SwingHigh.Index)
	}
	
	if math.Abs(result.SwingHigh.Price-116.0) > 0.1 {
		t.Errorf("Swing high price should be ~116, got: %f", result.SwingHigh.Price)
	}
	
	t.Logf("✅ Swing high detected correctly at index %d, price %.1f", 
		result.SwingHigh.Index, result.SwingHigh.Price)
}

// Helper functions for creating test data

func createUptrendData(periods int, growthRate float64) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)
	
	for i := 0; i < periods; i++ {
		price := basePrice * (1 + float64(i)*growthRate)
		
		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 0.999,
			High:      price * 1.005,
			Low:       price * 0.995,
			Close:     price,
			Volume:    1000000,
		}
	}
	
	return data
}

func createSidewaysData(periods int, volatility float64) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)
	
	for i := 0; i < periods; i++ {
		// Oscillate around base price
		noise := math.Sin(float64(i)*0.3) * volatility
		price := basePrice * (1 + noise)
		
		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 0.998,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    1000000,
		}
	}
	
	return data
}

func createStrongTrendData(periods int, trendRate float64) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)
	
	for i := 0; i < periods; i++ {
		price := basePrice * (1 + float64(i)*trendRate)
		
		// Add directional movement for ADX calculation
		momentum := trendRate * 10 // Amplify for directional indicators
		
		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * (1 - momentum),
			High:      price * (1 + momentum*2),
			Low:       price * (1 - momentum*0.5),
			Close:     price,
			Volume:    1000000,
		}
	}
	
	return data
}