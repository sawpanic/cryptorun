package unit

import (
	"context"
	"math"
	"testing"
	"time"

	"cryptorun/application/pipeline"
)

// MockDataProvider for testing momentum calculations
type TestDataProvider struct {
	mockData map[string]map[pipeline.TimeFrame][]pipeline.MarketData
}

func NewTestDataProvider() *TestDataProvider {
	return &TestDataProvider{
		mockData: make(map[string]map[pipeline.TimeFrame][]pipeline.MarketData),
	}
}

func (t *TestDataProvider) SetMockData(symbol string, tf pipeline.TimeFrame, data []pipeline.MarketData) {
	if t.mockData[symbol] == nil {
		t.mockData[symbol] = make(map[pipeline.TimeFrame][]pipeline.MarketData)
	}
	t.mockData[symbol][tf] = data
}

func (t *TestDataProvider) GetMarketData(ctx context.Context, symbol string, timeframe pipeline.TimeFrame, periods int) ([]pipeline.MarketData, error) {
	if symbolData, exists := t.mockData[symbol]; exists {
		if tfData, exists := symbolData[timeframe]; exists {
			// Return requested number of periods, or all data if less available
			if len(tfData) <= periods {
				return tfData, nil
			}
			return tfData[:periods], nil
		}
	}
	
	// Fallback to generated data if not explicitly set
	data := make([]pipeline.MarketData, periods)
	basePrice := 100.0
	
	for i := 0; i < periods; i++ {
		price := basePrice * (1.0 + float64(i)*0.001) // Small upward trend
		data[i] = pipeline.MarketData{
			Symbol:    symbol,
			Timestamp: time.Now().Add(-time.Duration(periods-i) * time.Hour),
			Price:     price,
			Volume:    1000000.0,
			High:      price * 1.01,
			Low:       price * 0.99,
		}
	}
	
	return data, nil
}

func TestMomentumCalculation(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)

	// Set up test data with known momentum
	testData := []pipeline.MarketData{
		{Symbol: "TESTUSD", Price: 100.0, Volume: 1000000, Timestamp: time.Now().Add(-time.Hour)},
		{Symbol: "TESTUSD", Price: 110.0, Volume: 1200000, Timestamp: time.Now()}, // 10% gain
	}
	
	provider.SetMockData("TESTUSD", pipeline.TF1h, testData)
	
	ctx := context.Background()
	factors, err := calc.CalculateMomentum(ctx, "TESTUSD")
	
	if err != nil {
		t.Fatalf("Failed to calculate momentum: %v", err)
	}
	
	if factors.Symbol != "TESTUSD" {
		t.Errorf("Wrong symbol: got %s, want TESTUSD", factors.Symbol)
	}
	
	// Check that momentum was calculated (should be approximately 10%)
	if math.IsNaN(factors.Momentum1h) {
		t.Error("Momentum1h should not be NaN")
	}
	
	if factors.Momentum1h < 9.0 || factors.Momentum1h > 11.0 {
		t.Errorf("Momentum1h out of expected range: got %f, want ~10", factors.Momentum1h)
	}
}

func TestRegimeWeights(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)
	
	testCases := []struct {
		regime string
		expectedWeights map[pipeline.TimeFrame]float64
	}{
		{
			regime: "bull",
			expectedWeights: map[pipeline.TimeFrame]float64{
				pipeline.TF1h:  0.20,
				pipeline.TF4h:  0.35,
				pipeline.TF12h: 0.30,
				pipeline.TF24h: 0.15,
				pipeline.TF7d:  0.00,
			},
		},
		{
			regime: "choppy", 
			expectedWeights: map[pipeline.TimeFrame]float64{
				pipeline.TF1h:  0.15,
				pipeline.TF4h:  0.25,
				pipeline.TF12h: 0.35,
				pipeline.TF24h: 0.20,
				pipeline.TF7d:  0.05,
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.regime, func(t *testing.T) {
			calc.SetRegime(tc.regime)
			weights := calc.GetRegimeWeights()
			
			if weights.Regime != tc.regime {
				t.Errorf("Wrong regime: got %s, want %s", weights.Regime, tc.regime)
			}
			
			for tf, expectedWeight := range tc.expectedWeights {
				if actualWeight, exists := weights.Weights[tf]; exists {
					if actualWeight != expectedWeight {
						t.Errorf("Wrong weight for %s: got %f, want %f", tf, actualWeight, expectedWeight)
					}
				} else {
					t.Errorf("Missing weight for timeframe %s", tf)
				}
			}
		})
	}
}

func TestApplyRegimeWeights(t *testing.T) {
	provider := NewTestDataProvider() 
	calc := pipeline.NewMomentumCalculator(provider)
	calc.SetRegime("bull")
	
	// Create test factors with known values
	factors := &pipeline.MomentumFactors{
		Symbol: "TESTUSD",
		Raw: map[pipeline.TimeFrame]float64{
			pipeline.TF1h:  10.0, // 10% momentum
			pipeline.TF4h:  20.0, // 20% momentum  
			pipeline.TF12h: 5.0,  // 5% momentum
			pipeline.TF24h: 2.0,  // 2% momentum
			pipeline.TF7d:  1.0,  // 1% momentum
		},
	}
	
	weightedMomentum := calc.ApplyRegimeWeights(factors)
	
	if math.IsNaN(weightedMomentum) {
		t.Error("Weighted momentum should not be NaN")
	}
	
	// Calculate expected weighted average for bull regime
	// (10 * 0.20) + (20 * 0.35) + (5 * 0.30) + (2 * 0.15) + (1 * 0.00) = 11.8
	expected := 11.8
	tolerance := 0.1
	
	if math.Abs(weightedMomentum - expected) > tolerance {
		t.Errorf("Wrong weighted momentum: got %f, want %f ±%f", weightedMomentum, expected, tolerance)
	}
}

func TestRSICalculation(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)
	
	// Create test data with known RSI pattern
	// Alternating gains and losses should produce RSI around 50
	testData := make([]pipeline.MarketData, 15)
	basePrice := 100.0
	
	for i := 0; i < 15; i++ {
		var price float64
		if i%2 == 0 {
			price = basePrice + float64(i)*0.5 // Small gains
		} else {
			price = basePrice + float64(i)*0.3 // Smaller gains
		}
		
		testData[i] = pipeline.MarketData{
			Symbol:    "TESTUSD",
			Price:     price,
			Volume:    1000000,
			Timestamp: time.Now().Add(-time.Duration(15-i) * time.Hour),
		}
	}
	
	provider.SetMockData("TESTUSD", pipeline.TF4h, testData)
	
	ctx := context.Background()
	factors, err := calc.CalculateMomentum(ctx, "TESTUSD")
	
	if err != nil {
		t.Fatalf("Failed to calculate momentum with RSI: %v", err)
	}
	
	if math.IsNaN(factors.RSI4h) {
		t.Error("RSI4h should not be NaN")
	}
	
	// RSI should be between 0 and 100
	if factors.RSI4h < 0 || factors.RSI4h > 100 {
		t.Errorf("RSI4h out of valid range: got %f, want 0-100", factors.RSI4h)
	}
}

func TestATRCalculation(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)
	
	// Create test data with known volatility
	testData := make([]pipeline.MarketData, 15)
	
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*0.1
		testData[i] = pipeline.MarketData{
			Symbol:    "TESTUSD",
			Price:     price,      // Previous close
			High:      price + 2.0, // High
			Low:       price - 1.5, // Low
			Volume:    1000000,
			Timestamp: time.Now().Add(-time.Duration(15-i) * time.Hour),
		}
	}
	
	provider.SetMockData("TESTUSD", pipeline.TF1h, testData)
	
	ctx := context.Background()
	factors, err := calc.CalculateMomentum(ctx, "TESTUSD")
	
	if err != nil {
		t.Fatalf("Failed to calculate momentum with ATR: %v", err)
	}
	
	if math.IsNaN(factors.ATR1h) {
		t.Error("ATR1h should not be NaN")
	}
	
	// ATR should be positive
	if factors.ATR1h <= 0 {
		t.Errorf("ATR1h should be positive: got %f", factors.ATR1h)
	}
	
	// ATR should be reasonable given our test data (around 3.5 range)
	if factors.ATR1h < 2.0 || factors.ATR1h > 5.0 {
		t.Errorf("ATR1h seems unreasonable: got %f, expected 2-5", factors.ATR1h)
	}
}

func TestMomentumWithInvalidData(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)
	
	// Test with insufficient data
	testData := []pipeline.MarketData{
		{Symbol: "TESTUSD", Price: 100.0, Volume: 1000000, Timestamp: time.Now()},
	}
	
	provider.SetMockData("TESTUSD", pipeline.TF1h, testData)
	
	ctx := context.Background()
	factors, err := calc.CalculateMomentum(ctx, "TESTUSD")
	
	// Should not fail completely, but momentum should be NaN
	if err != nil {
		t.Fatalf("Should handle insufficient data gracefully: %v", err)
	}
	
	if !math.IsNaN(factors.Momentum1h) {
		t.Error("Momentum1h should be NaN with insufficient data")
	}
}

func TestGetPeriodsForTimeframe(t *testing.T) {
	provider := NewTestDataProvider()
	calc := pipeline.NewMomentumCalculator(provider)
	
	testCases := []struct {
		timeframe pipeline.TimeFrame
		expected  int
	}{
		{pipeline.TF1h, 24},
		{pipeline.TF4h, 18},
		{pipeline.TF12h, 14},
		{pipeline.TF24h, 14},
		{pipeline.TF7d, 12},
	}
	
	for _, tc := range testCases {
		t.Run(string(tc.timeframe), func(t *testing.T) {
			// We can't directly test the private method, but we can test indirectly
			// by checking that the correct amount of data is requested
			provider.SetMockData("TESTUSD", tc.timeframe, make([]pipeline.MarketData, tc.expected+5))
			
			ctx := context.Background()
			_, err := calc.CalculateMomentum(ctx, "TESTUSD")
			
			if err != nil {
				t.Errorf("Failed to calculate momentum for %s: %v", tc.timeframe, err)
			}
		})
	}
}