package unit

import (
	"context"
	"testing"
	"time"

	"cryptorun/internal/algo/momentum"
)

func TestMomentumCore_Calculate(t *testing.T) {
	// Test configuration
	config := momentum.MomentumConfig{
		Weights: momentum.WeightConfig{
			TF1h:  0.20,
			TF4h:  0.35,
			TF12h: 0.30,
			TF24h: 0.15,
		},
		Fatigue: momentum.FatigueConfig{
			Return24hThreshold: 12.0,
			RSI4hThreshold:     70.0,
			AccelRenewal:       true,
		},
		Freshness: momentum.FreshnessConfig{
			MaxBarsAge: 2,
			ATRWindow:  14,
			ATRFactor:  1.2,
		},
		LateFill: momentum.LateFillConfig{
			MaxDelaySeconds: 30,
		},
		Regime: momentum.RegimeConfig{
			AdaptWeights: true,
			UpdatePeriod: 4,
		},
	}

	core := momentum.NewMomentumCore(config)

	// Create test market data
	baseTime := time.Now().Add(-24 * time.Hour)
	data := map[string][]momentum.MarketData{
		"1h":  generateTestData(baseTime, time.Hour, 24, 100.0, 0.02),
		"4h":  generateTestData(baseTime, 4*time.Hour, 6, 100.0, 0.05),
		"12h": generateTestData(baseTime, 12*time.Hour, 2, 100.0, 0.08),
		"24h": generateTestData(baseTime, 24*time.Hour, 1, 100.0, 0.10),
	}

	tests := []struct {
		name    string
		symbol  string
		regime  string
		wantErr bool
	}{
		{
			name:    "valid calculation",
			symbol:  "BTCUSD",
			regime:  "trending",
			wantErr: false,
		},
		{
			name:    "choppy regime",
			symbol:  "ETHUSD",
			regime:  "choppy",
			wantErr: false,
		},
		{
			name:    "volatile regime",
			symbol:  "ADAUSD",
			regime:  "volatile",
			wantErr: false,
		},
		{
			name:    "unknown regime",
			symbol:  "SOLUSD",
			regime:  "unknown",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := core.Calculate(context.Background(), tt.symbol, data, tt.regime)

			if (err != nil) != tt.wantErr {
				t.Errorf("MomentumCore.Calculate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify result structure
				if result.Symbol != tt.symbol {
					t.Errorf("Expected symbol %s, got %s", tt.symbol, result.Symbol)
				}

				if result.Regime != tt.regime {
					t.Errorf("Expected regime %s, got %s", tt.regime, result.Regime)
				}

				if !result.Protected {
					t.Error("MomentumCore should be marked as protected")
				}

				// Verify timeframe scores exist
				expectedTimeframes := []string{"1h", "4h", "12h", "24h"}
				for _, tf := range expectedTimeframes {
					if _, exists := result.TimeframeScores[tf]; !exists {
						t.Errorf("Missing timeframe score for %s", tf)
					}
				}

				// Verify guards are applied
				if result.GuardResults.Fatigue.Value == 0 &&
					result.GuardResults.Freshness.Value == 0 &&
					result.GuardResults.LateFill.Value == 0 {
					t.Error("Guards should have some values")
				}
			}
		})
	}
}

func TestMomentumCore_RegimeAdaptation(t *testing.T) {
	config := momentum.MomentumConfig{
		Weights: momentum.WeightConfig{
			TF1h:  0.20,
			TF4h:  0.35,
			TF12h: 0.30,
			TF24h: 0.15,
		},
		Regime: momentum.RegimeConfig{
			AdaptWeights: true,
			UpdatePeriod: 4,
		},
	}

	core := momentum.NewMomentumCore(config)

	// Test different regimes produce different weights
	baseWeights := core.GetRegimeWeights("unknown")
	trendingWeights := core.GetRegimeWeights("trending")
	choppyWeights := core.GetRegimeWeights("choppy")
	volatileWeights := core.GetRegimeWeights("volatile")

	// Trending should favor longer timeframes
	if trendingWeights.TF24h <= baseWeights.TF24h {
		t.Error("Trending regime should boost 24h weights")
	}

	// Choppy should favor shorter timeframes
	if choppyWeights.TF1h <= baseWeights.TF1h {
		t.Error("Choppy regime should boost 1h weights")
	}

	// Volatile should be balanced
	if volatileWeights.TF4h != baseWeights.TF4h {
		t.Error("Volatile regime should keep 4h weights stable")
	}
}

func TestMomentumCore_EmptyData(t *testing.T) {
	config := momentum.MomentumConfig{
		Weights: momentum.WeightConfig{
			TF1h:  0.25,
			TF4h:  0.35,
			TF12h: 0.25,
			TF24h: 0.15,
		},
	}

	core := momentum.NewMomentumCore(config)

	// Test with empty data
	emptyData := map[string][]momentum.MarketData{}

	result, err := core.Calculate(context.Background(), "BTCUSD", emptyData, "trending")
	if err != nil {
		t.Errorf("Expected no error with empty data, got %v", err)
	}

	if result.CoreScore != 0.0 {
		t.Errorf("Expected zero core score with empty data, got %f", result.CoreScore)
	}
}

func TestCalculateAcceleration(t *testing.T) {
	config := momentum.MomentumConfig{}
	core := momentum.NewMomentumCore(config)

	// Test data with positive acceleration
	data := []momentum.MarketData{
		{Close: 100.0, Timestamp: time.Now().Add(-3 * time.Hour)},
		{Close: 102.0, Timestamp: time.Now().Add(-2 * time.Hour)},
		{Close: 105.0, Timestamp: time.Now().Add(-1 * time.Hour)},
	}

	accel := core.CalculateAcceleration(data)

	// Should be positive since momentum is increasing
	if accel <= 0 {
		t.Errorf("Expected positive acceleration, got %f", accel)
	}

	// Test with insufficient data
	shortData := []momentum.MarketData{
		{Close: 100.0, Timestamp: time.Now()},
	}

	accel = core.CalculateAcceleration(shortData)
	if accel != 0.0 {
		t.Errorf("Expected zero acceleration with insufficient data, got %f", accel)
	}
}

// Helper function to generate test market data
func generateTestData(startTime time.Time, interval time.Duration, count int, basePrice, volatility float64) []momentum.MarketData {
	data := make([]momentum.MarketData, count)

	for i := 0; i < count; i++ {
		timestamp := startTime.Add(time.Duration(i) * interval)

		// Create slight upward trend with some volatility
		price := basePrice * (1.0 + float64(i)*0.001 + (float64(i%3)-1)*volatility)

		data[i] = momentum.MarketData{
			Timestamp: timestamp,
			Open:      price * 0.999,
			High:      price * 1.002,
			Low:       price * 0.998,
			Close:     price,
			Volume:    1000 + float64(i*100),
		}
	}

	return data
}
