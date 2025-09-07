package unit

import (
	"testing"
	"time"

	"cryptorun/internal/algo/momentum"
)

func TestFatigueGuard(t *testing.T) {
	config := momentum.MomentumConfig{
		Fatigue: momentum.FatigueConfig{
			Return24hThreshold: 18.0, // Calibrated threshold
			RSI4hThreshold:     70.0,
			AccelRenewal:       true,
		},
	}

	core := momentum.NewMomentumCore(config)

	tests := []struct {
		name           string
		return24h      float64
		rsi4h          float64
		acceleration   float64
		expectedPass   bool
		expectedReason string
	}{
		{
			name:           "low return should pass",
			return24h:      8.0,  // Below threshold
			rsi4h:          65.0,
			acceleration:   0.0,
			expectedPass:   true,
			expectedReason: "24h return below fatigue threshold",
		},
		{
			name:           "high return low RSI should pass", 
			return24h:      15.0, // Below new 18% threshold
			rsi4h:          60.0, // Below RSI threshold
			acceleration:   0.0,
			expectedPass:   true,
			expectedReason: "24h return below fatigue threshold",
		},
		{
			name:           "high return high RSI should fail",
			return24h:      20.0, // Above new 18% threshold
			rsi4h:          75.0, // Above RSI threshold
			acceleration:   -0.1, // Negative acceleration
			expectedPass:   false,
			expectedReason: "fatigue guard: 24h return excessive with high RSI",
		},
		{
			name:           "acceleration renewal should pass",
			return24h:      20.0, // Above new 18% threshold
			rsi4h:          75.0, // Above RSI threshold  
			acceleration:   0.5,  // Positive acceleration
			expectedPass:   true,
			expectedReason: "acceleration renewal override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data that produces the desired 24h return
			baseTime := time.Now().Add(-25 * time.Hour)
			data24h := []momentum.MarketData{
				{Close: 100.0, Timestamp: baseTime.Add(23 * time.Hour)},
				{Close: 100.0 * (1.0 + tt.return24h/100.0), Timestamp: baseTime.Add(24 * time.Hour)},
			}

			// Create 4h data for RSI calculation that produces desired RSI
			data4h := generateRSITestData(tt.rsi4h)

			data := map[string][]momentum.MarketData{
				"24h": data24h,
				"4h":  data4h,
			}

			result := &momentum.MomentumResult{
				Acceleration4h: tt.acceleration,
			}

			guard := core.ApplyFatigueGuard(data, result)

			if guard.Pass != tt.expectedPass {
				t.Errorf("Expected pass=%v, got pass=%v", tt.expectedPass, guard.Pass)
			}

			if guard.Reason != tt.expectedReason {
				t.Errorf("Expected reason '%s', got '%s'", tt.expectedReason, guard.Reason)
			}

			// Verify return value is approximately correct
			expectedReturn := tt.return24h
			if abs(guard.Value-expectedReturn) > 0.1 {
				t.Errorf("Expected return ~%f, got %f", expectedReturn, guard.Value)
			}
		})
	}
}

func TestFreshnessGuard(t *testing.T) {
	config := momentum.MomentumConfig{
		Freshness: momentum.FreshnessConfig{
			MaxBarsAge: 3, // Calibrated threshold
			ATRWindow:  14,
			ATRFactor:  1.2,
		},
	}

	core := momentum.NewMomentumCore(config)

	tests := []struct {
		name         string
		barsAge      int
		priceMove    float64
		atr          float64
		expectedPass bool
	}{
		{
			name:         "fresh data small move should pass",
			barsAge:      1,
			priceMove:    0.5,  // Small move
			atr:          1.0,
			expectedPass: true,
		},
		{
			name:         "fresh data large move should fail",
			barsAge:      1,
			priceMove:    2.0,  // Large move (> 1.2 * ATR)
			atr:          1.0,
			expectedPass: false,
		},
		{
			name:         "old data should fail",
			barsAge:      4,    // > MaxBarsAge (now 3)
			priceMove:    0.5,
			atr:          1.0,
			expectedPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data with specified age and price movement
			now := time.Now()
			baseTime := now.Add(-time.Duration(tt.barsAge) * time.Hour)
			
			data1h := make([]momentum.MarketData, 16) // Enough for ATR calculation
			for i := 0; i < 16; i++ {
				timestamp := baseTime.Add(time.Duration(i) * time.Hour)
				price := 100.0
				
				// Set the last price move to desired amount
				if i == 15 {
					price = 100.0 + tt.priceMove
				}
				
				data1h[i] = momentum.MarketData{
					Timestamp: timestamp,
					Open:      price,
					High:      price + tt.atr*0.5,
					Low:       price - tt.atr*0.5,
					Close:     price,
					Volume:    1000,
				}
			}

			data := map[string][]momentum.MarketData{
				"1h": data1h,
			}

			result := &momentum.MomentumResult{}
			guard := core.ApplyFreshnessGuard(data, result)

			if guard.Pass != tt.expectedPass {
				t.Errorf("Expected pass=%v, got pass=%v", tt.expectedPass, guard.Pass)
			}

			// Verify bars age is approximately correct
			if tt.expectedPass && int(guard.Value) != tt.barsAge {
				t.Errorf("Expected age %d, got %f", tt.barsAge, guard.Value)
			}
		})
	}
}

func TestLateFillGuard(t *testing.T) {
	config := momentum.MomentumConfig{
		LateFill: momentum.LateFillConfig{
			MaxDelaySeconds: 45, // Calibrated threshold
		},
	}

	core := momentum.NewMomentumCore(config)

	tests := []struct {
		name         string
		delaySeconds int
		expectedPass bool
	}{
		{
			name:         "quick fill should pass",
			delaySeconds: 15, // Within 45s limit
			expectedPass: true,
		},
		{
			name:         "late fill should fail",
			delaySeconds: 50, // Beyond 45s limit
			expectedPass: false,
		},
		{
			name:         "edge case should pass",
			delaySeconds: 45, // Exactly at limit
			expectedPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data with bar that closed at a specific time
			now := time.Now()
			barCloseTime := now.Add(-time.Duration(tt.delaySeconds) * time.Second)
			
			// Truncate to hour and add hour to simulate bar close time
			barTimestamp := barCloseTime.Truncate(time.Hour)
			
			data1h := []momentum.MarketData{
				{
					Timestamp: barTimestamp,
					Close:     100.0,
				},
			}

			data := map[string][]momentum.MarketData{
				"1h": data1h,
			}

			result := &momentum.MomentumResult{}
			guard := core.ApplyLateFillGuard(data, result)

			if guard.Pass != tt.expectedPass {
				t.Errorf("Expected pass=%v, got pass=%v", tt.expectedPass, guard.Pass)
			}

			// Verify delay time is reasonable (allow some tolerance)
			if abs(guard.Value-float64(tt.delaySeconds)) > 5.0 {
				t.Errorf("Expected delay ~%d seconds, got %f", tt.delaySeconds, guard.Value)
			}
		})
	}
}

func TestGuardResults(t *testing.T) {
	config := momentum.MomentumConfig{
		Fatigue: momentum.FatigueConfig{
			Return24hThreshold: 18.0, // Calibrated threshold
			RSI4hThreshold:     70.0,
			AccelRenewal:       true,
		},
		Freshness: momentum.FreshnessConfig{
			MaxBarsAge: 3, // Calibrated threshold
			ATRWindow:  14,
			ATRFactor:  1.2,
		},
		LateFill: momentum.LateFillConfig{
			MaxDelaySeconds: 45, // Calibrated threshold
		},
	}

	core := momentum.NewMomentumCore(config)

	// Create comprehensive test data
	now := time.Now()
	baseTime := now.Add(-25 * time.Hour)

	data := map[string][]momentum.MarketData{
		"24h": {
			{Close: 100.0, Timestamp: baseTime.Add(23 * time.Hour)},
			{Close: 105.0, Timestamp: baseTime.Add(24 * time.Hour)}, // 5% return
		},
		"4h": generateRSITestData(65.0), // Below RSI threshold
		"1h": generateFreshTestData(now, 1), // Fresh data
	}

	result := &momentum.MomentumResult{
		Acceleration4h: 0.1, // Positive acceleration
	}

	guards := core.ApplyGuards(data, result)

	// All guards should pass with this test data
	if !guards.Fatigue.Pass {
		t.Errorf("Fatigue guard should pass: %s", guards.Fatigue.Reason)
	}

	if !guards.Freshness.Pass {
		t.Errorf("Freshness guard should pass: %s", guards.Freshness.Reason)
	}

	if !guards.LateFill.Pass {
		t.Errorf("Late-fill guard should pass: %s", guards.LateFill.Reason)
	}

	// Verify guard values are reasonable
	if guards.Fatigue.Value < 0 || guards.Fatigue.Value > 100 {
		t.Errorf("Fatigue guard value out of range: %f", guards.Fatigue.Value)
	}

	if guards.Freshness.Value < 0 || guards.Freshness.Value > 10 {
		t.Errorf("Freshness guard value out of range: %f", guards.Freshness.Value)
	}

	if guards.LateFill.Value < 0 || guards.LateFill.Value > 3600 {
		t.Errorf("Late-fill guard value out of range: %f", guards.LateFill.Value)
	}
}

// Helper functions
func generateRSITestData(targetRSI float64) []momentum.MarketData {
	// Generate data that produces approximately the target RSI
	data := make([]momentum.MarketData, 20)
	baseTime := time.Now().Add(-20 * 4 * time.Hour)
	
	// Simple approach: create alternating up/down moves to achieve target RSI
	basePrice := 100.0
	for i := 0; i < 20; i++ {
		timestamp := baseTime.Add(time.Duration(i) * 4 * time.Hour)
		
		// Adjust price based on target RSI
		var price float64
		if targetRSI > 70 {
			// Mostly up moves for high RSI
			price = basePrice * (1.0 + float64(i)*0.01)
		} else if targetRSI < 30 {
			// Mostly down moves for low RSI
			price = basePrice * (1.0 - float64(i)*0.01)
		} else {
			// Balanced moves for neutral RSI
			price = basePrice * (1.0 + float64(i%2)*0.005 - 0.0025)
		}
		
		data[i] = momentum.MarketData{
			Timestamp: timestamp,
			Open:      price * 0.999,
			High:      price * 1.001,
			Low:       price * 0.999,
			Close:     price,
			Volume:    1000,
		}
	}
	
	return data
}

func generateFreshTestData(now time.Time, ageHours int) []momentum.MarketData {
	// Generate fresh data for testing
	data := make([]momentum.MarketData, 16)
	baseTime := now.Add(-time.Duration(ageHours+15) * time.Hour)
	
	for i := 0; i < 16; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		price := 100.0 + float64(i)*0.1
		
		data[i] = momentum.MarketData{
			Timestamp: timestamp,
			Open:      price,
			High:      price + 0.5,
			Low:       price - 0.5,
			Close:     price,
			Volume:    1000,
		}
	}
	
	return data
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}