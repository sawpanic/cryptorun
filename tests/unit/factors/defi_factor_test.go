package factors

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/providers/defi"
	"github.com/sawpanic/cryptorun/internal/score/factors"
)

// MockDeFiProvider implements defi.DeFiProvider for testing
type MockDeFiProvider struct {
	name     string
	metrics  map[string]*defi.DeFiMetrics
	healthy  bool
	latency  float64
}

// NewMockDeFiProvider creates a new mock provider
func NewMockDeFiProvider(name string) *MockDeFiProvider {
	return &MockDeFiProvider{
		name:    name,
		metrics: make(map[string]*defi.DeFiMetrics),
		healthy: true,
		latency: 100.0,
	}
}

// AddMetrics adds mock metrics for a protocol
func (m *MockDeFiProvider) AddMetrics(protocol string, metrics *defi.DeFiMetrics) {
	m.metrics[protocol] = metrics
}

// GetProtocolTVL returns mock TVL metrics
func (m *MockDeFiProvider) GetProtocolTVL(ctx context.Context, protocol string, tokenSymbol string) (*defi.DeFiMetrics, error) {
	if metrics, ok := m.metrics[protocol]; ok {
		// Clone and customize for TVL
		result := *metrics
		result.Protocol = protocol
		result.TokenSymbol = tokenSymbol
		result.DataSource = m.name
		return &result, nil
	}
	return nil, nil
}

// GetPoolMetrics returns mock pool metrics
func (m *MockDeFiProvider) GetPoolMetrics(ctx context.Context, protocol string, tokenA, tokenB string) (*defi.DeFiMetrics, error) {
	return m.GetProtocolTVL(ctx, protocol, tokenA)
}

// GetLendingMetrics returns mock lending metrics
func (m *MockDeFiProvider) GetLendingMetrics(ctx context.Context, protocol string, tokenSymbol string) (*defi.DeFiMetrics, error) {
	if metrics, ok := m.metrics[protocol]; ok {
		// Only return lending metrics if APY data exists
		if metrics.SupplyAPY > 0 || metrics.BorrowAPY > 0 {
			result := *metrics
			result.Protocol = protocol
			result.TokenSymbol = tokenSymbol
			result.DataSource = m.name
			return &result, nil
		}
	}
	return nil, nil
}

// GetTopTVLTokens returns mock top tokens
func (m *MockDeFiProvider) GetTopTVLTokens(ctx context.Context, limit int) ([]defi.DeFiMetrics, error) {
	return nil, nil // Not needed for factor tests
}

// Health returns mock health status
func (m *MockDeFiProvider) Health(ctx context.Context) (*defi.ProviderHealth, error) {
	return &defi.ProviderHealth{
		Healthy:            m.healthy,
		DataSource:         m.name,
		LastUpdate:         time.Now(),
		LatencyMS:          m.latency,
		ErrorRate:          0.0,
		SupportedProtocols: len(m.metrics),
		DataFreshness:      make(map[string]time.Duration),
	}, nil
}

// GetSupportedProtocols returns mock supported protocols
func (m *MockDeFiProvider) GetSupportedProtocols(ctx context.Context) ([]string, error) {
	protocols := make([]string, 0, len(m.metrics))
	for protocol := range m.metrics {
		protocols = append(protocols, protocol)
	}
	return protocols, nil
}

func TestDeFiFactorCalculator(t *testing.T) {
	// Setup mock providers
	provider1 := NewMockDeFiProvider("thegraph")
	provider1.AddMetrics("uniswap-v3", &defi.DeFiMetrics{
		TVL:              2000000.0,
		TVLChange24h:     5.2,
		TVLChange7d:      12.5,
		PoolVolume24h:    1500000.0,
		PoolLiquidity:    1800000.0,
		ConfidenceScore:  0.9,
	})
	provider1.AddMetrics("aave-v3", &defi.DeFiMetrics{
		TVL:              1500000.0,
		TVLChange24h:     3.8,
		TVLChange7d:      -2.1,
		SupplyAPY:        4.5,
		BorrowAPY:        8.2,
		UtilizationRate:  0.68,
		ConfidenceScore:  0.85,
	})

	provider2 := NewMockDeFiProvider("defillama")
	provider2.AddMetrics("uniswap-v3", &defi.DeFiMetrics{
		TVL:              1950000.0, // Slightly different for consensus testing
		TVLChange24h:     4.8,
		TVLChange7d:      11.2,
		PoolVolume24h:    1450000.0,
		ConfidenceScore:  0.88,
	})
	provider2.AddMetrics("curve", &defi.DeFiMetrics{
		TVL:              800000.0,
		TVLChange24h:     1.2,
		TVLChange7d:      -5.5,
		PoolVolume24h:    600000.0,
		ConfidenceScore:  0.80,
	})

	providers := map[string]defi.DeFiProvider{
		"thegraph":  provider1,
		"defillama": provider2,
	}

	config := factors.DefaultDeFiFactorConfig()
	calculator := factors.NewDeFiFactorCalculator(config, providers)

	t.Run("ValidDeFiFactorCalculation", func(t *testing.T) {
		input := factors.DeFiFactorInput{
			TokenSymbol:    "USDT",
			ProtocolList:   []string{"uniswap-v3", "aave-v3", "curve"},
			TimestampStart: time.Now().Add(-24 * time.Hour).Unix(),
			TimestampEnd:   time.Now().Unix(),
		}

		ctx := context.Background()
		result, err := calculator.Calculate(ctx, input)
		if err != nil {
			t.Fatalf("Calculate failed: %v", err)
		}

		// Validate basic structure
		if result == nil {
			t.Fatal("Result is nil")
		}

		// Check core metrics
		if result.DeFiScore < 0.0 || result.DeFiScore > 1.0 {
			t.Errorf("DeFi score should be 0-1, got %f", result.DeFiScore)
		}

		if result.TotalTVL <= 0 {
			t.Error("Total TVL should be positive")
		}

		if result.ProtocolCount != 3 {
			t.Errorf("Expected 3 protocols, got %d", result.ProtocolCount)
		}

		// Check TVL momentum (should be positive due to positive changes)
		if result.TVLMomentum <= 0 {
			t.Error("TVL momentum should be positive with positive TVL changes")
		}

		// Check protocol diversity (should be > 0 with multiple protocols)
		if result.ProtocolDiversity <= 0 {
			t.Error("Protocol diversity should be positive with multiple protocols")
		}

		// Check data consensus (should be high with multiple providers)
		if result.DataConsensus <= result.QualityScore {
			t.Error("Data consensus should benefit from multiple providers")
		}

		// Check provider count
		if result.ProviderCount != 2 {
			t.Errorf("Expected 2 providers, got %d", result.ProviderCount)
		}

		// Check lending metrics
		if result.WeightedSupplyAPY <= 0 {
			t.Error("Weighted supply APY should be positive with Aave data")
		}

		if result.WeightedBorrowAPY <= result.WeightedSupplyAPY {
			t.Error("Borrow APY should be higher than supply APY")
		}

		t.Logf("DeFi Score: %f", result.DeFiScore)
		t.Logf("Total TVL: $%.0f", result.TotalTVL)
		t.Logf("TVL Momentum: %f", result.TVLMomentum)
		t.Logf("Protocol Diversity: %f", result.ProtocolDiversity)
		t.Logf("Data Consensus: %f", result.DataConsensus)
	})

	t.Run("USDPairsOnlyEnforcement", func(t *testing.T) {
		input := factors.DeFiFactorInput{
			TokenSymbol:  "BTC", // Non-USD token
			ProtocolList: []string{"uniswap-v3"},
		}

		ctx := context.Background()
		_, err := calculator.Calculate(ctx, input)
		if err == nil {
			t.Error("Should reject non-USD token")
		}

		if err.Error() == "" || err.Error()[:len("non-USD token")] != "non-USD token" {
			t.Errorf("Expected USD enforcement error, got: %v", err)
		}
	})

	t.Run("InsufficientProtocols", func(t *testing.T) {
		input := factors.DeFiFactorInput{
			TokenSymbol:  "USDT",
			ProtocolList: []string{"uniswap-v3"}, // Only 1 protocol, need 2 minimum
		}

		ctx := context.Background()
		_, err := calculator.Calculate(ctx, input)
		if err == nil {
			t.Error("Should reject insufficient protocols")
		}

		validationErr, ok := err.(*factors.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else {
			if validationErr.Field != "protocol_list" {
				t.Errorf("Expected protocol_list field error, got %s", validationErr.Field)
			}
		}
	})

	t.Run("QualityThresholdEnforcement", func(t *testing.T) {
		// Setup low-quality provider
		lowQualityProvider := NewMockDeFiProvider("lowquality")
		lowQualityProvider.AddMetrics("test-protocol", &defi.DeFiMetrics{
			TVL:             500000.0,
			ConfidenceScore: 0.5, // Below default threshold of 0.7
		})

		lowQualityProviders := map[string]defi.DeFiProvider{
			"lowquality": lowQualityProvider,
		}

		lowQualityConfig := factors.DefaultDeFiFactorConfig()
		lowQualityConfig.Providers = []string{"lowquality"}
		lowQualityCalculator := factors.NewDeFiFactorCalculator(lowQualityConfig, lowQualityProviders)

		input := factors.DeFiFactorInput{
			TokenSymbol:  "USDT",
			ProtocolList: []string{"test-protocol", "test-protocol2"},
		}

		ctx := context.Background()
		result, err := calculator.Calculate(ctx, input)
		if err != nil {
			t.Fatalf("Calculate failed: %v", err)
		}

		// Should return 0 score due to quality threshold
		if result.DeFiScore != 0.0 {
			t.Errorf("Expected 0 DeFi score for low quality data, got %f", result.DeFiScore)
		}
	})

	t.Run("ConcentrationRiskEnforcement", func(t *testing.T) {
		// Setup highly concentrated provider (single large protocol)
		concentratedProvider := NewMockDeFiProvider("concentrated")
		concentratedProvider.AddMetrics("dominant-protocol", &defi.DeFiMetrics{
			TVL:             5000000.0, // Very large
			ConfidenceScore: 0.9,
		})
		concentratedProvider.AddMetrics("small-protocol", &defi.DeFiMetrics{
			TVL:             200000.0, // Much smaller
			ConfidenceScore: 0.9,
		})

		concentratedProviders := map[string]defi.DeFiProvider{
			"concentrated": concentratedProvider,
		}

		concentratedConfig := factors.DefaultDeFiFactorConfig()
		concentratedConfig.Providers = []string{"concentrated"}
		concentratedConfig.ConcentrationLimit = 0.80 // 80% limit
		concentratedCalculator := factors.NewDeFiFactorCalculator(concentratedConfig, concentratedProviders)

		input := factors.DeFiFactorInput{
			TokenSymbol:  "USDT",
			ProtocolList: []string{"dominant-protocol", "small-protocol"},
		}

		ctx := context.Background()
		result, err := concentratedCalculator.Calculate(ctx, input)
		if err != nil {
			t.Fatalf("Calculate failed: %v", err)
		}

		// Should return 0 score due to concentration risk
		if result.DeFiScore != 0.0 {
			t.Errorf("Expected 0 DeFi score for high concentration risk, got %f", result.DeFiScore)
		}

		if result.ConcentrationRisk <= 0.80 {
			t.Errorf("Expected high concentration risk (>0.80), got %f", result.ConcentrationRisk)
		}
	})
}

func TestDeFiFactorConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := factors.DefaultDeFiFactorConfig()

		// Validate required fields
		if len(config.Providers) == 0 {
			t.Error("Default config should have providers")
		}

		if config.MinProtocols <= 0 {
			t.Error("Min protocols should be positive")
		}

		if config.TVLThreshold <= 0 {
			t.Error("TVL threshold should be positive")
		}

		// Validate weights sum to reasonable value
		totalWeight := config.WeightTVLMomentum + config.WeightDiversity + config.WeightActivity + config.WeightYield
		if totalWeight <= 0.8 || totalWeight >= 1.2 {
			t.Errorf("Total weights should be ~1.0, got %f", totalWeight)
		}

		// Validate limits
		if config.ConcentrationLimit <= 0.5 || config.ConcentrationLimit >= 1.0 {
			t.Errorf("Concentration limit should be 0.5-1.0, got %f", config.ConcentrationLimit)
		}

		if config.QualityThreshold <= 0.0 || config.QualityThreshold >= 1.0 {
			t.Errorf("Quality threshold should be 0.0-1.0, got %f", config.QualityThreshold)
		}
	})
}

func TestUSDTokenValidation(t *testing.T) {
	testCases := []struct {
		token    string
		expected bool
	}{
		{"USDT", true},
		{"USDC", true},
		{"DAI", true},
		{"BUSD", true},
		{"TUSD", true},
		{"USDP", true},
		{"FRAX", true},
		{"GUSD", true},
		{"BTC", false},
		{"ETH", false},
		{"MATIC", false},
		{"", false},
	}

	// This would test the isUSDTokenSymbol function if it were exported
	// For now, we test through the calculator interface
	providers := map[string]defi.DeFiProvider{
		"mock": NewMockDeFiProvider("mock"),
	}

	config := factors.DefaultDeFiFactorConfig()
	calculator := factors.NewDeFiFactorCalculator(config, providers)

	for _, tc := range testCases {
		t.Run(tc.token, func(t *testing.T) {
			input := factors.DeFiFactorInput{
				TokenSymbol:  tc.token,
				ProtocolList: []string{"test1", "test2"}, // Minimum required
			}

			ctx := context.Background()
			_, err := calculator.Calculate(ctx, input)

			if tc.expected {
				// Valid USD tokens might still fail due to missing data, but not due to USD constraint
				if err != nil && err.Error()[:len("non-USD token")] == "non-USD token" {
					t.Errorf("Valid USD token %s should not be rejected for USD constraint", tc.token)
				}
			} else {
				// Invalid tokens should fail with USD constraint error
				if err == nil {
					t.Errorf("Non-USD token %s should be rejected", tc.token)
				} else if err.Error()[:len("non-USD token")] != "non-USD token" {
					t.Errorf("Expected USD constraint error for %s, got: %v", tc.token, err)
				}
			}
		})
	}
}