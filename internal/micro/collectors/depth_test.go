package collectors

import (
	"testing"

	"github.com/sawpanic/cryptorun/internal/micro"
)

func TestBinanceCalculateDepthUSD(t *testing.T) {
	config := micro.DefaultConfig("binance")
	collector, _ := NewBinanceCollector(config)

	// Mock order book levels [price, size]
	bidLevels := [][]string{
		{"100.0", "1.0"}, // $100
		{"99.5", "2.0"},  // $199
		{"99.0", "1.5"},  // $148.5
		{"98.5", "0.5"},  // $49.25
		{"98.0", "1.0"},  // $98 (should be excluded at -2%)
	}

	askLevels := [][]string{
		{"100.5", "1.0"}, // $100.5
		{"101.0", "1.5"}, // $151.5
		{"101.5", "2.0"}, // $203
		{"102.0", "0.5"}, // $51 (should be excluded at +2%)
		{"102.5", "1.0"}, // $102.5 (should be excluded at +2%)
	}

	midPrice := 100.25 // (100 + 100.5) / 2

	t.Run("bid depth calculation", func(t *testing.T) {
		// -2% from midPrice = 98.245, so levels at 99.5, 99.0, 98.5 should be included
		depth, levels := collector.calculateDepthUSD(bidLevels, midPrice, -0.02)

		expectedDepth := 100.0 + 199.0 + 148.5 + 49.25 // = 496.75
		expectedLevels := 4

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected bid depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d bid levels, got %d", expectedLevels, levels)
		}
	})

	t.Run("ask depth calculation", func(t *testing.T) {
		// +2% from midPrice = 102.255, so levels at 100.5, 101.0, 101.5 should be included
		depth, levels := collector.calculateDepthUSD(askLevels, midPrice, 0.02)

		expectedDepth := 100.5 + 151.5 + 203.0 // = 455.0
		expectedLevels := 3

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected ask depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d ask levels, got %d", expectedLevels, levels)
		}
	})

	t.Run("empty levels", func(t *testing.T) {
		depth, levels := collector.calculateDepthUSD([][]string{}, midPrice, -0.02)

		if depth != 0.0 {
			t.Errorf("Expected 0 depth for empty levels, got %f", depth)
		}

		if levels != 0 {
			t.Errorf("Expected 0 levels for empty levels, got %d", levels)
		}
	})

	t.Run("invalid mid price", func(t *testing.T) {
		depth, levels := collector.calculateDepthUSD(bidLevels, 0.0, -0.02)

		if depth != 0.0 {
			t.Errorf("Expected 0 depth for invalid mid price, got %f", depth)
		}

		if levels != 0 {
			t.Errorf("Expected 0 levels for invalid mid price, got %d", levels)
		}
	})
}

func TestOKXCalculateDepthUSD(t *testing.T) {
	config := micro.DefaultConfig("okx")
	collector, _ := NewOKXCollector(config)

	// Mock OKX order book levels [price, size, liquidated_orders, order_count]
	bidLevels := [][]string{
		{"50000.0", "0.5", "0", "1"}, // $25,000
		{"49900.0", "1.0", "0", "2"}, // $49,900
		{"49800.0", "0.8", "0", "1"}, // $39,840
		{"49500.0", "0.2", "0", "1"}, // $9,900 (should be excluded at -2%)
	}

	askLevels := [][]string{
		{"50100.0", "0.3", "0", "1"}, // $15,030
		{"50200.0", "0.7", "0", "2"}, // $35,140
		{"50300.0", "1.2", "0", "3"}, // $60,360
		{"50600.0", "0.5", "0", "1"}, // $25,300 (should be excluded at +2%)
	}

	midPrice := 50050.0 // (50000 + 50100) / 2

	t.Run("bid depth calculation with OKX format", func(t *testing.T) {
		// -2% from midPrice = 49049, so levels at 50000, 49900, 49800 should be included
		depth, levels := collector.calculateDepthUSD(bidLevels, midPrice, -0.02)

		expectedDepth := 25000.0 + 49900.0 + 39840.0 // = 114,740
		expectedLevels := 3

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected bid depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d bid levels, got %d", expectedLevels, levels)
		}
	})

	t.Run("ask depth calculation with OKX format", func(t *testing.T) {
		// +2% from midPrice = 51051, so all levels should be included
		depth, levels := collector.calculateDepthUSD(askLevels, midPrice, 0.02)

		expectedDepth := 15030.0 + 35140.0 + 60360.0 // = 110,530
		expectedLevels := 3

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected ask depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d ask levels, got %d", expectedLevels, levels)
		}
	})

	t.Run("malformed level data", func(t *testing.T) {
		malformedLevels := [][]string{
			{"50000.0"}, // Missing size
			{"", "1.0"}, // Missing price
		}

		depth, levels := collector.calculateDepthUSD(malformedLevels, midPrice, -0.02)

		// Should handle malformed data gracefully
		if depth != 0.0 {
			t.Errorf("Expected 0 depth for malformed levels, got %f", depth)
		}

		if levels != 0 {
			t.Errorf("Expected 0 levels for malformed levels, got %d", levels)
		}
	})
}

func TestCoinbaseCalculateDepthUSD(t *testing.T) {
	config := micro.DefaultConfig("coinbase")
	collector, _ := NewCoinbaseCollector(config)

	// Mock Coinbase order book levels [price, size, num_orders]
	bidLevels := [][]string{
		{"65000.0", "0.15", "5"}, // $9,750
		{"64950.0", "0.25", "3"}, // $16,237.5
		{"64900.0", "0.10", "2"}, // $6,490
		{"64800.0", "0.05", "1"}, // $3,240 (should be excluded at -2%)
	}

	askLevels := [][]string{
		{"65050.0", "0.12", "4"},  // $7,806
		{"65100.0", "0.18", "6"},  // $11,718
		{"65150.0", "0.22", "8"},  // $14,333
		{"65400.0", "0.30", "10"}, // $19,620 (should be excluded at +2%)
	}

	midPrice := 65025.0 // (65000 + 65050) / 2

	t.Run("bid depth calculation with Coinbase format", func(t *testing.T) {
		// -2% from midPrice = 63724.5, so levels at 65000, 64950, 64900 should be included
		depth, levels := collector.calculateDepthUSD(bidLevels, midPrice, -0.02)

		expectedDepth := 9750.0 + 16237.5 + 6490.0 // = 32,477.5
		expectedLevels := 3

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected bid depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d bid levels, got %d", expectedLevels, levels)
		}
	})

	t.Run("ask depth calculation with Coinbase format", func(t *testing.T) {
		// +2% from midPrice = 66325.5, so all levels should be included
		depth, levels := collector.calculateDepthUSD(askLevels, midPrice, 0.02)

		expectedDepth := 7806.0 + 11718.0 + 14333.0 // = 33,857
		expectedLevels := 3

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected ask depth %f, got %f", expectedDepth, depth)
		}

		if levels != expectedLevels {
			t.Errorf("Expected %d ask levels, got %d", expectedLevels, levels)
		}
	})
}

func TestLiquidityGradientMonotonicity(t *testing.T) {
	// Test that liquidity gradient behaves monotonically
	// (higher concentration at 0.5% should never result in lower gradient)

	config := micro.DefaultConfig("binance")
	collector, _ := NewBinanceCollector(config)

	tests := []struct {
		name      string
		depth05   float64
		depth2    float64
		expectGTE float64 // Should be >= this previous gradient
	}{
		{"increasing concentration", 10000, 50000, 0},
		{"higher concentration", 20000, 50000, 0.2},
		{"even higher concentration", 35000, 50000, 0.4},
		{"maximum concentration", 50000, 50000, 0.7},
	}

	var prevGradient float64 = 0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gradient := collector.calculateLiquidityGradient(tt.depth05, tt.depth2)

			if gradient < prevGradient {
				t.Errorf("Gradient %f should not be less than previous %f (monotonicity violation)",
					gradient, prevGradient)
			}

			if gradient < tt.expectGTE {
				t.Errorf("Gradient %f should be >= %f", gradient, tt.expectGTE)
			}

			prevGradient = gradient
		})
	}
}

func TestDepthCalculationEdgeCases(t *testing.T) {
	config := micro.DefaultConfig("test")
	collector, _ := NewBinanceCollector(config)

	t.Run("very tight percentage range", func(t *testing.T) {
		levels := [][]string{
			{"100.00", "1.0"},
			{"99.99", "1.0"},
			{"99.98", "1.0"},
		}

		midPrice := 100.0
		// Very tight range: -0.001% = 99.999
		depth, count := collector.calculateDepthUSD(levels, midPrice, -0.001)

		expectedDepth := 100.0 // Only the first level should qualify
		expectedCount := 1

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected depth %f, got %f", expectedDepth, depth)
		}

		if count != expectedCount {
			t.Errorf("Expected %d levels, got %d", expectedCount, count)
		}
	})

	t.Run("very wide percentage range", func(t *testing.T) {
		levels := [][]string{
			{"100.0", "1.0"},
			{"95.0", "1.0"},
			{"90.0", "1.0"},
			{"80.0", "1.0"},
		}

		midPrice := 100.0
		// Very wide range: -20%
		depth, count := collector.calculateDepthUSD(levels, midPrice, -0.20)

		expectedDepth := 100.0 + 95.0 + 90.0 + 80.0 // All levels should qualify
		expectedCount := 4

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected depth %f, got %f", expectedDepth, depth)
		}

		if count != expectedCount {
			t.Errorf("Expected %d levels, got %d", expectedCount, count)
		}
	})

	t.Run("price exactly at boundary", func(t *testing.T) {
		levels := [][]string{
			{"100.0", "1.0"},
			{"98.0", "1.0"}, // Exactly -2%
			{"97.9", "1.0"}, // Just outside -2%
		}

		midPrice := 100.0
		depth, count := collector.calculateDepthUSD(levels, midPrice, -0.02)

		// 98.0 is exactly at the -2% boundary and should be included
		expectedDepth := 100.0 + 98.0
		expectedCount := 2

		if abs(depth-expectedDepth) > 0.01 {
			t.Errorf("Expected depth %f, got %f", expectedDepth, depth)
		}

		if count != expectedCount {
			t.Errorf("Expected %d levels, got %d", expectedCount, count)
		}
	})
}
