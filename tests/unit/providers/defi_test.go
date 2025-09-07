package providers

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/providers/defi"
)

func TestDeFiProviderFactory(t *testing.T) {
	factory := defi.NewDeFiProviderFactory()

	t.Run("GetAvailableProviders", func(t *testing.T) {
		providers := factory.GetAvailableProviders()
		expectedProviders := []string{"thegraph", "defillama"}
		
		if len(providers) != len(expectedProviders) {
			t.Errorf("Expected %d providers, got %d", len(expectedProviders), len(providers))
		}
		
		providerMap := make(map[string]bool)
		for _, provider := range providers {
			providerMap[provider] = true
		}
		
		for _, expected := range expectedProviders {
			if !providerMap[expected] {
				t.Errorf("Expected provider '%s' not found", expected)
			}
		}
	})

	t.Run("CreateTheGraphProvider", func(t *testing.T) {
		config := defi.CreateDefaultConfig("thegraph")
		
		provider, err := factory.CreateTheGraphProvider(config)
		if err != nil {
			t.Fatalf("Failed to create The Graph provider: %v", err)
		}
		
		if provider == nil {
			t.Fatal("Provider is nil")
		}
		
		// Test health check
		ctx := context.Background()
		health, err := provider.Health(ctx)
		if err != nil {
			t.Logf("Health check failed (expected with no network): %v", err)
		} else {
			if health.DataSource != "thegraph" {
				t.Errorf("Expected data source 'thegraph', got '%s'", health.DataSource)
			}
		}
	})

	t.Run("CreateDeFiLlamaProvider", func(t *testing.T) {
		config := defi.CreateDefaultConfig("defillama")
		
		provider, err := factory.CreateDeFiLlamaProvider(config)
		if err != nil {
			t.Fatalf("Failed to create DeFiLlama provider: %v", err)
		}
		
		if provider == nil {
			t.Fatal("Provider is nil")
		}
		
		// Test health check
		ctx := context.Background()
		health, err := provider.Health(ctx)
		if err != nil {
			t.Logf("Health check failed (expected with no network): %v", err)
		} else {
			if health.DataSource != "defillama" {
				t.Errorf("Expected data source 'defillama', got '%s'", health.DataSource)
			}
		}
	})
}

func TestDeFiProviderConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := defi.CreateDefaultConfig("thegraph")
		
		// Test required fields
		if config.DataSource != "thegraph" {
			t.Errorf("Expected data source 'thegraph', got '%s'", config.DataSource)
		}
		
		if !config.USDPairsOnly {
			t.Error("USD pairs only should be enforced by default")
		}
		
		if config.RateLimitRPS <= 0 {
			t.Error("Rate limit should be positive")
		}
		
		if config.RequestTimeout <= 0 {
			t.Error("Request timeout should be positive")
		}
		
		if config.BaseURL == "" {
			t.Error("Base URL should be set")
		}
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		factory := defi.NewDeFiProviderFactory()
		
		// Test invalid config - USDPairsOnly false
		invalidConfig := defi.CreateDefaultConfig("thegraph")
		invalidConfig.USDPairsOnly = false
		
		_, err := factory.CreateTheGraphProvider(invalidConfig)
		if err == nil {
			t.Error("Expected error for USDPairsOnly=false")
		}
		
		// Test invalid config - excessive rate limit
		invalidConfig2 := defi.CreateDefaultConfig("thegraph")
		invalidConfig2.RateLimitRPS = 1000.0 // Way too high for free tier
		
		_, err = factory.CreateTheGraphProvider(invalidConfig2)
		if err == nil {
			t.Error("Expected error for excessive rate limit")
		}
	})
}

func TestDeFiMetrics(t *testing.T) {
	t.Run("DeFiMetricsStructure", func(t *testing.T) {
		metrics := &defi.DeFiMetrics{
			Timestamp:        time.Now(),
			Protocol:         "uniswap-v3",
			TokenSymbol:      "USDT",
			TVL:              1000000.0,
			TVLChange24h:     5.2,
			TVLChange7d:      -2.1,
			PoolVolume24h:    500000.0,
			PoolLiquidity:    750000.0,
			PoolFees24h:      2500.0,
			DataSource:       "thegraph",
			ConfidenceScore:  0.9,
			PITShift:         0,
		}
		
		// Validate structure
		if metrics.Protocol != "uniswap-v3" {
			t.Errorf("Expected protocol 'uniswap-v3', got '%s'", metrics.Protocol)
		}
		
		if metrics.TokenSymbol != "USDT" {
			t.Errorf("Expected token symbol 'USDT', got '%s'", metrics.TokenSymbol)
		}
		
		if metrics.TVL != 1000000.0 {
			t.Errorf("Expected TVL 1000000.0, got %f", metrics.TVL)
		}
		
		if metrics.ConfidenceScore < 0.0 || metrics.ConfidenceScore > 1.0 {
			t.Errorf("Confidence score should be between 0.0-1.0, got %f", metrics.ConfidenceScore)
		}
	})

	t.Run("LendingMetricsStructure", func(t *testing.T) {
		metrics := &defi.DeFiMetrics{
			Timestamp:        time.Now(),
			Protocol:         "aave-v3",
			TokenSymbol:      "USDC",
			TVL:              2000000.0,
			BorrowAPY:        8.5,
			SupplyAPY:        3.2,
			UtilizationRate:  0.65,
			DataSource:       "thegraph",
			ConfidenceScore:  0.85,
		}
		
		// Validate lending-specific fields
		if metrics.BorrowAPY <= 0 {
			t.Error("Borrow APY should be positive for lending protocols")
		}
		
		if metrics.SupplyAPY <= 0 {
			t.Error("Supply APY should be positive for lending protocols")
		}
		
		if metrics.UtilizationRate < 0 || metrics.UtilizationRate > 1 {
			t.Errorf("Utilization rate should be between 0-1, got %f", metrics.UtilizationRate)
		}
	})
}

func TestProviderHealth(t *testing.T) {
	t.Run("HealthStructure", func(t *testing.T) {
		health := &defi.ProviderHealth{
			Healthy:            true,
			DataSource:         "thegraph",
			LastUpdate:         time.Now(),
			LatencyMS:          125.5,
			ErrorRate:          0.02,
			SupportedProtocols: 8,
			DataFreshness:      make(map[string]time.Duration),
			Errors:             []string{},
		}
		
		// Validate health structure
		if !health.Healthy {
			t.Error("Expected healthy status to be true")
		}
		
		if health.DataSource != "thegraph" {
			t.Errorf("Expected data source 'thegraph', got '%s'", health.DataSource)
		}
		
		if health.LatencyMS < 0 {
			t.Error("Latency should not be negative")
		}
		
		if health.ErrorRate < 0 || health.ErrorRate > 1 {
			t.Errorf("Error rate should be between 0-1, got %f", health.ErrorRate)
		}
		
		if health.SupportedProtocols <= 0 {
			t.Error("Supported protocols count should be positive")
		}
	})
}

func TestAggregatedDeFiMetrics(t *testing.T) {
	t.Run("AggregatedStructure", func(t *testing.T) {
		aggregated := &defi.AggregatedDeFiMetrics{
			TokenSymbol:          "USDT",
			Timestamp:            time.Now(),
			ProtocolCount:        3,
			ProtocolMetrics:      make(map[string]*defi.DeFiMetrics),
			TotalTVL:             5000000.0,
			WeightedTVLChange24h: 2.8,
			TotalVolume24h:       1500000.0,
			TVLConsensus:         0.92,
			DataQuality:          0.88,
			OutlierProtocols:     []string{"curve"},
		}
		
		// Validate aggregated structure
		if aggregated.TokenSymbol != "USDT" {
			t.Errorf("Expected token symbol 'USDT', got '%s'", aggregated.TokenSymbol)
		}
		
		if aggregated.ProtocolCount <= 0 {
			t.Error("Protocol count should be positive")
		}
		
		if aggregated.TotalTVL <= 0 {
			t.Error("Total TVL should be positive")
		}
		
		if aggregated.TVLConsensus < 0 || aggregated.TVLConsensus > 1 {
			t.Errorf("TVL consensus should be between 0-1, got %f", aggregated.TVLConsensus)
		}
		
		if aggregated.DataQuality < 0 || aggregated.DataQuality > 1 {
			t.Errorf("Data quality should be between 0-1, got %f", aggregated.DataQuality)
		}
	})
}

func TestConsistencyReport(t *testing.T) {
	t.Run("ConsistencyStructure", func(t *testing.T) {
		report := &defi.ConsistencyReport{
			TokenSymbol:          "USDC",
			Timestamp:            time.Now(),
			ProtocolCount:        4,
			TVLConsistency:       0.85,
			VolumeConsistency:    0.78,
			OverallConsistency:   0.82,
			Outliers:             make(map[string]defi.OutlierInfo),
			OutlierThreshold:     2.0,
			InsufficientData:     false,
			StaleDataDetected:    false,
			HighVarianceWarning:  true,
			Recommendations:      []string{"Monitor curve protocol data", "Increase sampling frequency"},
		}
		
		// Validate consistency report structure
		if report.TokenSymbol != "USDC" {
			t.Errorf("Expected token symbol 'USDC', got '%s'", report.TokenSymbol)
		}
		
		if report.ProtocolCount <= 0 {
			t.Error("Protocol count should be positive")
		}
		
		// Validate consistency scores
		consistencyScores := []float64{
			report.TVLConsistency,
			report.VolumeConsistency,
			report.OverallConsistency,
		}
		
		for i, score := range consistencyScores {
			if score < 0 || score > 1 {
				t.Errorf("Consistency score %d should be between 0-1, got %f", i, score)
			}
		}
		
		if report.OutlierThreshold <= 0 {
			t.Error("Outlier threshold should be positive")
		}
		
		if len(report.Recommendations) == 0 {
			t.Error("Recommendations should not be empty when warnings are present")
		}
	})
}

func TestUSDTokenValidation(t *testing.T) {
	// This would test the isUSDToken function if it were exported
	// For now, we test through the provider interface
	
	t.Run("USDTokenEnforcement", func(t *testing.T) {
		factory := defi.NewDeFiProviderFactory()
		config := defi.CreateDefaultConfig("thegraph")
		
		provider, err := factory.CreateTheGraphProvider(config)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		
		ctx := context.Background()
		
		// Test with valid USD token (should work in principle, may fail due to network)
		_, err = provider.GetProtocolTVL(ctx, "uniswap-v3", "USDT")
		if err != nil {
			// Check if error is due to USD constraint or network issues
			if containsSubstring(err.Error(), "non-USD token not allowed") {
				t.Error("USDT should be allowed as a USD token")
			}
			// Network errors are expected in tests
		}
		
		// Test with invalid non-USD token (should fail)
		_, err = provider.GetProtocolTVL(ctx, "uniswap-v3", "BTC")
		if err == nil {
			t.Error("BTC should be rejected as non-USD token")
		} else if !containsSubstring(err.Error(), "USD pairs only") {
			t.Errorf("Expected USD pairs only error, got: %v", err)
		}
	})
}