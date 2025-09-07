package collectors

import (
	"testing"
	"time"

	"cryptorun/internal/micro"
)

func TestCalculateSpreadBps(t *testing.T) {
	config := micro.DefaultConfig("test")
	base := NewBaseCollector(config)

	tests := []struct {
		name     string
		bidPrice float64
		askPrice float64
		expected float64
	}{
		{
			name:     "normal spread",
			bidPrice: 100.0,
			askPrice: 100.5,
			expected: 49.88, // (0.5 / 100.25) * 10000 ≈ 49.88 bps
		},
		{
			name:     "tight spread",
			bidPrice: 50000.0,
			askPrice: 50001.0,
			expected: 0.2, // (1 / 50000.5) * 10000 ≈ 0.2 bps
		},
		{
			name:     "wide spread",
			bidPrice: 1.0,
			askPrice: 1.1,
			expected: 952.38, // (0.1 / 1.05) * 10000 ≈ 952.38 bps
		},
		{
			name:     "zero bid price",
			bidPrice: 0.0,
			askPrice: 100.0,
			expected: 0.0,
		},
		{
			name:     "zero ask price",
			bidPrice: 100.0,
			askPrice: 0.0,
			expected: 0.0,
		},
		{
			name:     "ask lower than bid (invalid)",
			bidPrice: 100.0,
			askPrice: 99.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.calculateSpreadBps(tt.bidPrice, tt.askPrice)

			// Allow small floating point differences
			if abs(result-tt.expected) > 0.01 {
				t.Errorf("calculateSpreadBps(%f, %f) = %f, expected %f",
					tt.bidPrice, tt.askPrice, result, tt.expected)
			}
		})
	}
}

func TestCalculateLiquidityGradient(t *testing.T) {
	config := micro.DefaultConfig("test")
	base := NewBaseCollector(config)

	tests := []struct {
		name       string
		depth05Pct float64
		depth2Pct  float64
		expected   float64
	}{
		{
			name:       "normal gradient",
			depth05Pct: 50000.0,
			depth2Pct:  100000.0,
			expected:   0.5,
		},
		{
			name:       "high concentration (steep gradient)",
			depth05Pct: 80000.0,
			depth2Pct:  100000.0,
			expected:   0.8,
		},
		{
			name:       "low concentration (flat gradient)",
			depth05Pct: 20000.0,
			depth2Pct:  100000.0,
			expected:   0.2,
		},
		{
			name:       "zero depth at 2%",
			depth05Pct: 50000.0,
			depth2Pct:  0.0,
			expected:   0.0,
		},
		{
			name:       "zero depth at 0.5%",
			depth05Pct: 0.0,
			depth2Pct:  100000.0,
			expected:   0.0,
		},
		{
			name:       "equal depths (perfect concentration)",
			depth05Pct: 100000.0,
			depth2Pct:  100000.0,
			expected:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.calculateLiquidityGradient(tt.depth05Pct, tt.depth2Pct)

			// Allow small floating point differences
			if abs(result-tt.expected) > 0.0001 {
				t.Errorf("calculateLiquidityGradient(%f, %f) = %f, expected %f",
					tt.depth05Pct, tt.depth2Pct, result, tt.expected)
			}
		})
	}
}

func TestAssessDataQuality(t *testing.T) {
	config := micro.DefaultConfig("test")
	base := NewBaseCollector(config)

	tests := []struct {
		name            string
		dataAge         time.Duration
		hasCompleteData bool
		sequenceGap     bool
		expected        micro.DataQuality
	}{
		{
			name:            "excellent quality",
			dataAge:         1 * time.Second,
			hasCompleteData: true,
			sequenceGap:     false,
			expected:        micro.QualityExcellent,
		},
		{
			name:            "good quality - moderate age",
			dataAge:         3 * time.Second,
			hasCompleteData: true,
			sequenceGap:     false,
			expected:        micro.QualityGood,
		},
		{
			name:            "good quality - sequence gap",
			dataAge:         1 * time.Second,
			hasCompleteData: true,
			sequenceGap:     true,
			expected:        micro.QualityGood,
		},
		{
			name:            "degraded quality - stale data",
			dataAge:         10 * time.Second,
			hasCompleteData: true,
			sequenceGap:     false,
			expected:        micro.QualityDegraded,
		},
		{
			name:            "degraded quality - incomplete data",
			dataAge:         1 * time.Second,
			hasCompleteData: false,
			sequenceGap:     false,
			expected:        micro.QualityDegraded,
		},
		{
			name:            "degraded quality - multiple issues",
			dataAge:         10 * time.Second,
			hasCompleteData: false,
			sequenceGap:     true,
			expected:        micro.QualityDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.assessDataQuality(tt.dataAge, tt.hasCompleteData, tt.sequenceGap)

			if result != tt.expected {
				t.Errorf("assessDataQuality(%v, %v, %v) = %v, expected %v",
					tt.dataAge, tt.hasCompleteData, tt.sequenceGap, result, tt.expected)
			}
		})
	}
}

func TestCollectorConfiguration(t *testing.T) {
	t.Run("default config creation", func(t *testing.T) {
		venues := []string{"binance", "okx", "coinbase"}

		for _, venue := range venues {
			config := micro.DefaultConfig(venue)

			if config.Venue != venue {
				t.Errorf("Expected venue %s, got %s", venue, config.Venue)
			}

			if config.AggregationWindowMs != 1000 {
				t.Errorf("Expected aggregation window 1000ms, got %d", config.AggregationWindowMs)
			}

			if config.RollingStatsWindowMs != 60000 {
				t.Errorf("Expected rolling stats window 60000ms, got %d", config.RollingStatsWindowMs)
			}

			if !config.EnableHealthCSV {
				t.Error("Expected health CSV to be enabled by default")
			}

			if config.HealthCSVPath == "" {
				t.Error("Expected health CSV path to be set")
			}
		}
	})

	t.Run("venue-specific URLs", func(t *testing.T) {
		binanceConfig := micro.DefaultConfig("binance")
		if binanceConfig.BaseURL != "https://api.binance.com" {
			t.Errorf("Expected Binance URL, got %s", binanceConfig.BaseURL)
		}

		okxConfig := micro.DefaultConfig("okx")
		if okxConfig.BaseURL != "https://www.okx.com" {
			t.Errorf("Expected OKX URL, got %s", okxConfig.BaseURL)
		}

		coinbaseConfig := micro.DefaultConfig("coinbase")
		if coinbaseConfig.BaseURL != "https://api.pro.coinbase.com" {
			t.Errorf("Expected Coinbase URL, got %s", coinbaseConfig.BaseURL)
		}
	})
}

func TestVenueHealthCSVConversion(t *testing.T) {
	health := &micro.VenueHealth{
		Venue:            "test",
		Timestamp:        time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Status:           micro.HealthGreen,
		Healthy:          true,
		Uptime:           99.5,
		HeartbeatAgeMs:   1500,
		MessageGapRate:   0.02,
		WSReconnectCount: 1,
		LatencyP50Ms:     45,
		LatencyP99Ms:     150,
		ErrorRate:        0.01,
		DataFreshness:    2 * time.Second,
		DataCompleteness: 98.5,
		Recommendation:   "proceed",
	}

	csvRecord := health.ToCSVRecord()

	if csvRecord.Venue != "test" {
		t.Errorf("Expected venue test, got %s", csvRecord.Venue)
	}

	if csvRecord.Status != "green" {
		t.Errorf("Expected status green, got %s", csvRecord.Status)
	}

	if csvRecord.Healthy != "true" {
		t.Errorf("Expected healthy true, got %s", csvRecord.Healthy)
	}

	if csvRecord.Uptime != 99.5 {
		t.Errorf("Expected uptime 99.5, got %f", csvRecord.Uptime)
	}

	if csvRecord.DataFreshnessMs != 2000 {
		t.Errorf("Expected data freshness 2000ms, got %d", csvRecord.DataFreshnessMs)
	}
}

func TestBaseCollectorLifecycle(t *testing.T) {
	config := micro.DefaultConfig("test")
	config.EnableHealthCSV = false // Disable CSV for testing

	base := NewBaseCollector(config)

	if base.venue != "test" {
		t.Errorf("Expected venue test, got %s", base.venue)
	}

	if base.Venue() != "test" {
		t.Errorf("Expected Venue() to return test, got %s", base.Venue())
	}

	// Test initial health status
	if !base.IsHealthy() {
		t.Error("Expected collector to be initially healthy")
	}

	// Test subscription management
	symbols := []string{"BTC/USD", "ETH/USD"}
	base.subscriptions = make(map[string]bool)

	for _, symbol := range symbols {
		base.subscriptions[symbol] = true
	}

	if len(base.subscriptions) != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", len(base.subscriptions))
	}

	if !base.subscriptions["BTC/USD"] {
		t.Error("Expected BTC/USD to be subscribed")
	}

	if !base.subscriptions["ETH/USD"] {
		t.Error("Expected ETH/USD to be subscribed")
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
