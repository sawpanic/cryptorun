package microstructure

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cryptorun/internal/microstructure/adapters"
)

// MockMicrostructureAdapter for testing
type MockMicrostructureAdapter struct {
	l1Data      *adapters.L1Data
	l2Data      *adapters.L2Data
	orderbook   *adapters.OrderBookSnapshot
	shouldError bool
	latency     time.Duration
}

func NewMockMicrostructureAdapter() *MockMicrostructureAdapter {
	return &MockMicrostructureAdapter{
		latency: 100 * time.Millisecond,
	}
}

func (m *MockMicrostructureAdapter) SetL1Data(data *adapters.L1Data) {
	m.l1Data = data
}

func (m *MockMicrostructureAdapter) SetL2Data(data *adapters.L2Data) {
	m.l2Data = data
}

func (m *MockMicrostructureAdapter) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockMicrostructureAdapter) SetLatency(latency time.Duration) {
	m.latency = latency
}

func (m *MockMicrostructureAdapter) GetL1Data(ctx context.Context, symbol string) (*adapters.L1Data, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}

	time.Sleep(m.latency) // Simulate latency

	if m.l1Data == nil {
		// Return default mock data
		return &adapters.L1Data{
			Symbol:    symbol,
			Venue:     "mock",
			Timestamp: time.Now(),
			BidPrice:  49950.0,
			BidSize:   1.5,
			AskPrice:  50050.0,
			AskSize:   1.2,
			SpreadBps: 20.0,
			MidPrice:  50000.0,
			Quality:   "excellent",
			DataAge:   0,
		}, nil
	}

	return m.l1Data, nil
}

func (m *MockMicrostructureAdapter) GetL2Data(ctx context.Context, symbol string) (*adapters.L2Data, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}

	time.Sleep(m.latency) // Simulate latency

	if m.l2Data == nil {
		// Return default mock data
		return &adapters.L2Data{
			Symbol:            symbol,
			Venue:             "mock",
			Timestamp:         time.Now(),
			BidDepthUSD:       75000.0,
			AskDepthUSD:       75000.0,
			TotalDepthUSD:     150000.0,
			BidLevels:         10,
			AskLevels:         10,
			LiquidityGradient: 0.8,
			VADRInputVolume:   0,
			VADRInputRange:    0,
			Quality:           "excellent",
			IsUSDQuote:        true,
		}, nil
	}

	return m.l2Data, nil
}

func (m *MockMicrostructureAdapter) GetOrderBookSnapshot(ctx context.Context, symbol string) (*adapters.OrderBookSnapshot, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}

	return &adapters.OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "mock",
		Timestamp: time.Now(),
		Bids:      []adapters.PriceLevel{{Price: 49950, Size: 1.5}},
		Asks:      []adapters.PriceLevel{{Price: 50050, Size: 1.2}},
		LastPrice: 50000.0,
	}, nil
}

// Test fixture creation
func createTestTieredCalculator() *TieredGateCalculator {
	config := &TieredGateConfig{
		VenuePriority:       []string{"binance", "okx", "coinbase"},
		CrossVenueEnabled:   false,
		MaxVenueAge:         30 * time.Second,
		UseWorstFeedVADR:    true,
		SpreadToleranceBps:  5.0,
		DepthTolerancePct:   0.10,
		MinDataQualityScore: 0.8,
		MaxLatencyMs:        2000,
		RequiredVenues:      1,
	}

	calculator := NewTieredGateCalculator(config)

	// Replace adapters with mocks
	calculator.adapters = map[string]adapters.MicrostructureAdapter{
		"binance":  NewMockMicrostructureAdapter(),
		"okx":      NewMockMicrostructureAdapter(),
		"coinbase": NewMockMicrostructureAdapter(),
	}

	return calculator
}

// Test tier determination based on ADV
func TestTieredGateCalculator_TierDetermination(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	testCases := []struct {
		name         string
		adv          float64
		expectedTier string
		symbol       string
	}{
		{
			name:         "Tier1 - High ADV",
			adv:          10000000.0, // $10M ADV
			expectedTier: "tier1",
			symbol:       "BTC/USD",
		},
		{
			name:         "Tier2 - Medium ADV",
			adv:          2500000.0, // $2.5M ADV
			expectedTier: "tier2",
			symbol:       "ETH/USD",
		},
		{
			name:         "Tier3 - Low ADV",
			adv:          500000.0, // $500k ADV
			expectedTier: "tier3",
			symbol:       "SOL/USD",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vadrInput := &VADRInput{
				High:         51000.0,
				Low:          49000.0,
				Volume:       tc.adv / 1000.0,
				ADV:          tc.adv,
				CurrentPrice: 50000.0,
			}

			result, err := calculator.EvaluateTieredGates(ctx, tc.symbol, tc.adv, vadrInput)
			if err != nil {
				t.Fatalf("EvaluateTieredGates failed: %v", err)
			}

			if result.Tier.Name != tc.expectedTier {
				t.Errorf("Expected tier %s, got %s", tc.expectedTier, result.Tier.Name)
			}

			if result.ADV != tc.adv {
				t.Errorf("Expected ADV %.0f, got %.0f", tc.adv, result.ADV)
			}
		})
	}
}

// Test depth gate evaluation with different tiers
func TestTieredGateCalculator_DepthGatesByTier(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	testCases := []struct {
		name         string
		adv          float64
		mockDepthUSD float64
		expectedPass bool
		expectedTier string
	}{
		{
			name:         "Tier1 - Sufficient depth",
			adv:          10000000.0, // Tier1 requires $150k
			mockDepthUSD: 200000.0,   // $200k provided
			expectedPass: true,
			expectedTier: "tier1",
		},
		{
			name:         "Tier1 - Insufficient depth",
			adv:          10000000.0, // Tier1 requires $150k
			mockDepthUSD: 100000.0,   // Only $100k provided
			expectedPass: false,
			expectedTier: "tier1",
		},
		{
			name:         "Tier2 - Sufficient depth",
			adv:          2500000.0, // Tier2 requires $75k
			mockDepthUSD: 100000.0,  // $100k provided
			expectedPass: true,
			expectedTier: "tier2",
		},
		{
			name:         "Tier3 - Minimal depth passes",
			adv:          500000.0, // Tier3 requires $25k
			mockDepthUSD: 30000.0,  // $30k provided
			expectedPass: true,
			expectedTier: "tier3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Configure mock adapter for binance (primary venue)
			mockAdapter := calculator.adapters["binance"].(*MockMicrostructureAdapter)
			mockAdapter.SetL2Data(&adapters.L2Data{
				Symbol:            "BTC/USD",
				Venue:             "binance",
				Timestamp:         time.Now(),
				BidDepthUSD:       tc.mockDepthUSD / 2,
				AskDepthUSD:       tc.mockDepthUSD / 2,
				TotalDepthUSD:     tc.mockDepthUSD,
				BidLevels:         10,
				AskLevels:         10,
				LiquidityGradient: 0.8,
				Quality:           "excellent",
				IsUSDQuote:        true,
			})

			vadrInput := &VADRInput{
				High:         51000.0,
				Low:          49000.0,
				Volume:       tc.adv / 1000.0,
				ADV:          tc.adv,
				CurrentPrice: 50000.0,
			}

			result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", tc.adv, vadrInput)
			if err != nil {
				t.Fatalf("EvaluateTieredGates failed: %v", err)
			}

			if result.Tier.Name != tc.expectedTier {
				t.Errorf("Expected tier %s, got %s", tc.expectedTier, result.Tier.Name)
			}

			if result.DepthGate == nil {
				t.Fatal("DepthGate is nil")
			}

			if result.DepthGate.Pass != tc.expectedPass {
				t.Errorf("Expected depth gate pass=%t, got %t (measured: $%.0f, required: $%.0f)",
					tc.expectedPass, result.DepthGate.Pass, result.DepthGate.Measured, result.DepthGate.Required)
			}

			if result.DepthGate.Measured != tc.mockDepthUSD {
				t.Errorf("Expected measured depth $%.0f, got $%.0f", tc.mockDepthUSD, result.DepthGate.Measured)
			}
		})
	}
}

// Test spread gate evaluation with different tiers
func TestTieredGateCalculator_SpreadGatesByTier(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	testCases := []struct {
		name          string
		adv           float64
		mockSpreadBps float64
		expectedPass  bool
		expectedTier  string
	}{
		{
			name:          "Tier1 - Tight spread passes",
			adv:           10000000.0, // Tier1 cap: 25bps
			mockSpreadBps: 20.0,       // 20bps
			expectedPass:  true,
			expectedTier:  "tier1",
		},
		{
			name:          "Tier1 - Wide spread fails",
			adv:           10000000.0, // Tier1 cap: 25bps
			mockSpreadBps: 30.0,       // 30bps
			expectedPass:  false,
			expectedTier:  "tier1",
		},
		{
			name:          "Tier2 - Medium spread passes",
			adv:           2500000.0, // Tier2 cap: 50bps
			mockSpreadBps: 45.0,      // 45bps
			expectedPass:  true,
			expectedTier:  "tier2",
		},
		{
			name:          "Tier3 - Wide spread still passes",
			adv:           500000.0, // Tier3 cap: 80bps
			mockSpreadBps: 75.0,     // 75bps
			expectedPass:  true,
			expectedTier:  "tier3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Configure mock adapter for binance (primary venue)
			mockAdapter := calculator.adapters["binance"].(*MockMicrostructureAdapter)
			mockAdapter.SetL1Data(&adapters.L1Data{
				Symbol:    "BTC/USD",
				Venue:     "binance",
				Timestamp: time.Now(),
				BidPrice:  50000.0 - (tc.mockSpreadBps * 50000.0 / 20000.0), // Calculate bid from spread
				BidSize:   1.5,
				AskPrice:  50000.0 + (tc.mockSpreadBps * 50000.0 / 20000.0), // Calculate ask from spread
				AskSize:   1.2,
				SpreadBps: tc.mockSpreadBps,
				MidPrice:  50000.0,
				Quality:   "excellent",
				DataAge:   0,
			})

			vadrInput := &VADRInput{
				High:         51000.0,
				Low:          49000.0,
				Volume:       tc.adv / 1000.0,
				ADV:          tc.adv,
				CurrentPrice: 50000.0,
			}

			result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", tc.adv, vadrInput)
			if err != nil {
				t.Fatalf("EvaluateTieredGates failed: %v", err)
			}

			if result.Tier.Name != tc.expectedTier {
				t.Errorf("Expected tier %s, got %s", tc.expectedTier, result.Tier.Name)
			}

			if result.SpreadGate == nil {
				t.Fatal("SpreadGate is nil")
			}

			if result.SpreadGate.Pass != tc.expectedPass {
				t.Errorf("Expected spread gate pass=%t, got %t (measured: %.1f bps, cap: %.1f bps)",
					tc.expectedPass, result.SpreadGate.Pass, result.SpreadGate.MeasuredBps, result.SpreadGate.CapBps)
			}

			if result.SpreadGate.MeasuredBps != tc.mockSpreadBps {
				t.Errorf("Expected measured spread %.1f bps, got %.1f bps", tc.mockSpreadBps, result.SpreadGate.MeasuredBps)
			}
		})
	}
}

// Test VADR precedence rules (max of tier minimum and p80 historical)
func TestTieredGateCalculator_VADRPrecedenceRules(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	testCases := []struct {
		name              string
		adv               float64
		vadrMeasured      float64
		expectedPass      bool
		expectedEffective float64 // Expected effective minimum after precedence
		tierName          string
	}{
		{
			name:              "Tier1 - VADR exceeds both tier and p80",
			adv:               10000000.0, // Tier1 min: 1.85
			vadrMeasured:      2.0,        // Higher than both 1.85 and mock p80
			expectedPass:      true,
			expectedEffective: 1.9425, // Mock p80 = 1.85 * 1.05 = 1.9425
			tierName:          "tier1",
		},
		{
			name:              "Tier1 - VADR between tier and p80",
			adv:               10000000.0, // Tier1 min: 1.85, p80: ~1.94
			vadrMeasured:      1.90,       // Between 1.85 and 1.94
			expectedPass:      false,      // Fails p80 requirement
			expectedEffective: 1.9425,     // p80 wins
			tierName:          "tier1",
		},
		{
			name:              "Tier2 - VADR meets p80 precedence",
			adv:               2500000.0, // Tier2 min: 1.80, p80: 1.89
			vadrMeasured:      1.90,      // Exceeds p80
			expectedPass:      true,
			expectedEffective: 1.89, // p80 = 1.80 * 1.05
			tierName:          "tier2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vadrInput := &VADRInput{
				High:         51000.0,
				Low:          49000.0,
				Volume:       tc.vadrMeasured * tc.adv / 1000.0, // Adjust volume to achieve desired VADR
				ADV:          tc.adv,
				CurrentPrice: 50000.0,
			}

			result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", tc.adv, vadrInput)
			if err != nil {
				t.Fatalf("EvaluateTieredGates failed: %v", err)
			}

			if result.Tier.Name != tc.tierName {
				t.Errorf("Expected tier %s, got %s", tc.tierName, result.Tier.Name)
			}

			if result.VADRGate == nil {
				t.Fatal("VADRGate is nil")
			}

			// Check precedence rule application
			if result.VADRGate.PrecedenceRule != "max(tier_min, p80_historical)" {
				t.Errorf("Expected precedence rule 'max(tier_min, p80_historical)', got %s", result.VADRGate.PrecedenceRule)
			}

			// Check effective minimum (allowing for small floating point differences)
			if abs(result.VADRGate.EffectiveMin-tc.expectedEffective) > 0.01 {
				t.Errorf("Expected effective min %.4f, got %.4f", tc.expectedEffective, result.VADRGate.EffectiveMin)
			}

			if result.VADRGate.Pass != tc.expectedPass {
				t.Errorf("Expected VADR gate pass=%t, got %t (measured: %.2f, effective min: %.2f)",
					tc.expectedPass, result.VADRGate.Pass, result.VADRGate.Measured, result.VADRGate.EffectiveMin)
			}
		})
	}
}

// Test venue failover and degraded mode
func TestTieredGateCalculator_VenueFailover(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	t.Run("Primary venue failure triggers fallback", func(t *testing.T) {
		// Make binance (primary) fail
		mockBinance := calculator.adapters["binance"].(*MockMicrostructureAdapter)
		mockBinance.SetError(true)

		// Set up OKX (fallback) with good data
		mockOKX := calculator.adapters["okx"].(*MockMicrostructureAdapter)
		mockOKX.SetL1Data(&adapters.L1Data{
			Symbol:    "BTC/USD",
			Venue:     "okx",
			Timestamp: time.Now(),
			BidPrice:  49975.0,
			BidSize:   1.0,
			AskPrice:  50025.0,
			AskSize:   1.0,
			SpreadBps: 10.0,
			MidPrice:  50000.0,
			Quality:   "excellent",
			DataAge:   0,
		})

		mockOKX.SetL2Data(&adapters.L2Data{
			Symbol:        "BTC/USD",
			Venue:         "okx",
			Timestamp:     time.Now(),
			TotalDepthUSD: 200000.0,
			Quality:       "excellent",
			IsUSDQuote:    true,
		})

		vadrInput := &VADRInput{
			High:         51000.0,
			Low:          49000.0,
			Volume:       5000.0,
			ADV:          5000000.0, // Tier1
			CurrentPrice: 50000.0,
		}

		result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", 5000000.0, vadrInput)
		if err != nil {
			t.Fatalf("EvaluateTieredGates failed: %v", err)
		}

		// Should use OKX as primary venue after Binance failure
		if result.PrimaryVenue != "okx" {
			t.Errorf("Expected primary venue 'okx', got '%s'", result.PrimaryVenue)
		}

		// Should still pass with good fallback data
		if !result.AllGatesPass {
			t.Errorf("Expected gates to pass with good fallback data")
		}
	})

	t.Run("All venues fail except one triggers degraded mode", func(t *testing.T) {
		// Make binance and okx fail
		calculator.adapters["binance"].(*MockMicrostructureAdapter).SetError(true)
		calculator.adapters["okx"].(*MockMicrostructureAdapter).SetError(true)

		// Only coinbase works
		mockCoinbase := calculator.adapters["coinbase"].(*MockMicrostructureAdapter)
		mockCoinbase.SetError(false)
		mockCoinbase.SetL2Data(&adapters.L2Data{
			Symbol:        "BTC/USD",
			Venue:         "coinbase",
			Timestamp:     time.Now(),
			TotalDepthUSD: 80000.0,
			Quality:       "good",
			IsUSDQuote:    true,
		})

		vadrInput := &VADRInput{
			High:         51000.0,
			Low:          49000.0,
			Volume:       2000.0,
			ADV:          2000000.0,
			CurrentPrice: 50000.0,
		}

		result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", 2000000.0, vadrInput)
		if err != nil {
			t.Fatalf("EvaluateTieredGates failed: %v", err)
		}

		if !result.DegradedMode {
			t.Errorf("Expected degraded mode with only one venue available")
		}

		if result.PrimaryVenue != "coinbase" {
			t.Errorf("Expected primary venue 'coinbase', got '%s'", result.PrimaryVenue)
		}

		if result.RecommendedAction != "halve_size" {
			t.Errorf("Expected recommended action 'halve_size' in degraded mode, got '%s'", result.RecommendedAction)
		}
	})
}

// Test latency thresholds
func TestTieredGateCalculator_LatencyThresholds(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	// Set high latency on binance
	mockBinance := calculator.adapters["binance"].(*MockMicrostructureAdapter)
	mockBinance.SetLatency(3 * time.Second) // Exceeds 2s threshold

	vadrInput := &VADRInput{
		High:         51000.0,
		Low:          49000.0,
		Volume:       1000.0,
		ADV:          1000000.0,
		CurrentPrice: 50000.0,
	}

	result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", 1000000.0, vadrInput)
	if err != nil {
		t.Fatalf("EvaluateTieredGates failed: %v", err)
	}

	// Check that binance is marked as unavailable due to latency
	binanceResult, exists := result.VenueResults["binance"]
	if !exists {
		t.Fatal("Binance venue result not found")
	}

	if binanceResult.Available {
		t.Errorf("Expected binance to be unavailable due to high latency")
	}

	if binanceResult.Error == "" {
		t.Errorf("Expected error message for high latency venue")
	}
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Test recommended action determination
func TestTieredGateCalculator_RecommendedActions(t *testing.T) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	testCases := []struct {
		name              string
		setupMocks        func()
		adv               float64
		expectedAction    string
		expectedGatesPass bool
		expectedDegraded  bool
	}{
		{
			name: "All gates pass - proceed",
			setupMocks: func() {
				// All venues healthy with good data
				for _, adapter := range calculator.adapters {
					mockAdapter := adapter.(*MockMicrostructureAdapter)
					mockAdapter.SetError(false)
					mockAdapter.SetL1Data(&adapters.L1Data{
						SpreadBps: 15.0,
						Quality:   "excellent",
					})
					mockAdapter.SetL2Data(&adapters.L2Data{
						TotalDepthUSD: 200000.0,
						Quality:       "excellent",
					})
				}
			},
			adv:               5000000.0, // Tier1
			expectedAction:    "proceed",
			expectedGatesPass: true,
			expectedDegraded:  false,
		},
		{
			name: "Gates fail - defer",
			setupMocks: func() {
				// Set insufficient depth
				for _, adapter := range calculator.adapters {
					mockAdapter := adapter.(*MockMicrostructureAdapter)
					mockAdapter.SetError(false)
					mockAdapter.SetL2Data(&adapters.L2Data{
						TotalDepthUSD: 50000.0, // Below tier1 requirement
						Quality:       "excellent",
					})
				}
			},
			adv:               5000000.0, // Tier1 (requires $150k depth)
			expectedAction:    "defer",
			expectedGatesPass: false,
			expectedDegraded:  false,
		},
		{
			name: "Degraded mode - halve size",
			setupMocks: func() {
				// Only one venue available
				calculator.adapters["binance"].(*MockMicrostructureAdapter).SetError(true)
				calculator.adapters["okx"].(*MockMicrostructureAdapter).SetError(true)

				coinbase := calculator.adapters["coinbase"].(*MockMicrostructureAdapter)
				coinbase.SetError(false)
				coinbase.SetL1Data(&adapters.L1Data{
					SpreadBps: 30.0,
					Quality:   "good",
				})
				coinbase.SetL2Data(&adapters.L2Data{
					TotalDepthUSD: 100000.0,
					Quality:       "good",
				})
			},
			adv:               2000000.0, // Tier2
			expectedAction:    "halve_size",
			expectedGatesPass: true,
			expectedDegraded:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			vadrInput := &VADRInput{
				High:         51000.0,
				Low:          49000.0,
				Volume:       tc.adv / 1000.0,
				ADV:          tc.adv,
				CurrentPrice: 50000.0,
			}

			result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", tc.adv, vadrInput)
			if err != nil {
				t.Fatalf("EvaluateTieredGates failed: %v", err)
			}

			if result.RecommendedAction != tc.expectedAction {
				t.Errorf("Expected action '%s', got '%s'", tc.expectedAction, result.RecommendedAction)
			}

			if result.AllGatesPass != tc.expectedGatesPass {
				t.Errorf("Expected gates pass=%t, got %t", tc.expectedGatesPass, result.AllGatesPass)
			}

			if result.DegradedMode != tc.expectedDegraded {
				t.Errorf("Expected degraded mode=%t, got %t", tc.expectedDegraded, result.DegradedMode)
			}
		})
	}
}

// Benchmark tiered gate evaluation performance
func BenchmarkTieredGateCalculator_EvaluateGates(b *testing.B) {
	calculator := createTestTieredCalculator()
	ctx := context.Background()

	vadrInput := &VADRInput{
		High:         51000.0,
		Low:          49000.0,
		Volume:       5000.0,
		ADV:          5000000.0,
		CurrentPrice: 50000.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := calculator.EvaluateTieredGates(ctx, "BTC/USD", 5000000.0, vadrInput)
		if err != nil {
			b.Fatalf("EvaluateTieredGates failed: %v", err)
		}

		// Ensure result is used to prevent compiler optimizations
		_ = result.AllGatesPass
	}
}
