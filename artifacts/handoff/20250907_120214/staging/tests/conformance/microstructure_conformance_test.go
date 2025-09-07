package conformance_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// MicrostructureConfig represents microstructure gate configuration
type MicrostructureConfig struct {
	Microstructure struct {
		Spread struct {
			MaxBasisPoints float64 `yaml:"max_basis_points"`
		} `yaml:"spread"`

		Depth struct {
			MinUSD    float64 `yaml:"min_usd"`
			Tolerance float64 `yaml:"tolerance"`
		} `yaml:"depth"`

		VADR struct {
			MinMultiplier float64 `yaml:"min_multiplier"`
		} `yaml:"vadr"`

		ExchangeNativeOnly bool     `yaml:"exchange_native_only"`
		AllowedExchanges   []string `yaml:"allowed_exchanges"`
		BannedAggregators  []string `yaml:"banned_aggregators"`
	} `yaml:"microstructure"`
}

// TestAggregatorBanConformance verifies aggregator ban for microstructure data
func TestAggregatorBanConformance(t *testing.T) {
	// Test configuration files for aggregator ban
	configs := []string{"config/momentum.yaml", "config/dip.yaml", "config/bench.yaml"}

	for _, configPath := range configs {
		t.Run(strings.ReplaceAll(configPath, "/", "_"), func(t *testing.T) {
			configData, err := os.ReadFile(configPath)
			if err != nil {
				t.Skipf("Config file %s not found", configPath)
			}

			var config MicrostructureConfig
			if err := yaml.Unmarshal(configData, &config); err != nil {
				// Skip if microstructure section doesn't exist
				t.Skipf("No microstructure config in %s", configPath)
			}

			// Verify exchange-native only flag
			if !config.Microstructure.ExchangeNativeOnly {
				t.Errorf("CONFORMANCE VIOLATION: %s must have microstructure.exchange_native_only: true",
					configPath)
			}

			// Verify banned aggregators list exists and includes common ones
			expectedBannedAggregators := []string{"dexscreener", "coingecko", "coinmarketcap"}

			if len(config.Microstructure.BannedAggregators) == 0 {
				t.Errorf("CONFORMANCE VIOLATION: %s must specify banned_aggregators for microstructure",
					configPath)
			}

			for _, expected := range expectedBannedAggregators {
				found := false
				for _, banned := range config.Microstructure.BannedAggregators {
					if strings.Contains(strings.ToLower(banned), expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CONFORMANCE VIOLATION: %s must ban aggregator '%s' for microstructure",
						configPath, expected)
				}
			}

			// Verify allowed exchanges are exchange-native only
			allowedExchanges := []string{"binance", "kraken", "coinbase", "okx"}
			if len(config.Microstructure.AllowedExchanges) == 0 {
				t.Errorf("CONFORMANCE VIOLATION: %s must specify allowed_exchanges", configPath)
			}

			for _, allowed := range config.Microstructure.AllowedExchanges {
				isValidExchange := false
				for _, valid := range allowedExchanges {
					if strings.Contains(strings.ToLower(allowed), valid) {
						isValidExchange = true
						break
					}
				}
				if !isValidExchange {
					t.Errorf("CONFORMANCE VIOLATION: %s contains non-exchange-native source '%s'",
						configPath, allowed)
				}
			}
		})
	}
}

// TestMicrostructureGateConformance verifies microstructure gate requirements
func TestMicrostructureGateConformance(t *testing.T) {
	configs := []string{"config/momentum.yaml", "config/dip.yaml"}

	for _, configPath := range configs {
		t.Run(strings.ReplaceAll(configPath, "/", "_"), func(t *testing.T) {
			configData, err := os.ReadFile(configPath)
			if err != nil {
				t.Skipf("Config file %s not found", configPath)
			}

			var config MicrostructureConfig
			if err := yaml.Unmarshal(configData, &config); err != nil {
				t.Skipf("No microstructure config in %s", configPath)
			}

			// Verify spread < 50bps requirement
			if config.Microstructure.Spread.MaxBasisPoints > 50.0 {
				t.Errorf("CONFORMANCE VIOLATION: %s spread limit %.1f bps exceeds 50 bps maximum",
					configPath, config.Microstructure.Spread.MaxBasisPoints)
			}

			// Verify depth ±2% ≥ $100k requirement
			if config.Microstructure.Depth.MinUSD < 100000.0 {
				t.Errorf("CONFORMANCE VIOLATION: %s depth minimum $%.0f below $100k requirement",
					configPath, config.Microstructure.Depth.MinUSD)
			}

			if config.Microstructure.Depth.Tolerance > 0.02 {
				t.Errorf("CONFORMANCE VIOLATION: %s depth tolerance %.3f exceeds ±2%% requirement",
					configPath, config.Microstructure.Depth.Tolerance)
			}

			// Verify VADR ≥ 1.75× requirement
			if config.Microstructure.VADR.MinMultiplier < 1.75 {
				t.Errorf("CONFORMANCE VIOLATION: %s VADR minimum %.2f below 1.75× requirement",
					configPath, config.Microstructure.VADR.MinMultiplier)
			}
		})
	}
}

// TestSourceCodeAggregatorBanConformance verifies source code doesn't use banned aggregators
func TestSourceCodeAggregatorBanConformance(t *testing.T) {
	// Check source files for banned aggregator usage
	sourceFiles := []string{
		"internal/infrastructure/apis/reference/dexscreener_client.go",
		"internal/infrastructure/apis/reference/binance_weight.go",
		"internal/infrastructure/apis/kraken/client.go",
		"internal/infrastructure/apis/kraken/rest_client.go",
	}

	bannedPatterns := []string{
		"dexscreener.com",
		"api.coingecko.com/api/v3/simple/price", // Banned for microstructure
		"coinmarketcap.com/api",
		"/depth",     // Should only be from exchange APIs
		"/orderbook", // Should only be from exchange APIs
	}

	for _, filePath := range sourceFiles {
		t.Run(strings.ReplaceAll(filePath, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Skipf("Source file %s not found", filePath)
			}

			content := strings.ToLower(string(data))

			for _, banned := range bannedPatterns {
				if strings.Contains(content, strings.ToLower(banned)) {
					// Check if it's used for microstructure data
					contextLines := strings.Split(content, "\n")
					for i, line := range contextLines {
						if strings.Contains(line, strings.ToLower(banned)) {
							// Look for microstructure keywords in surrounding context
							start := max(0, i-5)
							end := min(len(contextLines), i+5)
							context := strings.Join(contextLines[start:end], "\n")

							microstructureKeywords := []string{"spread", "depth", "orderbook", "bid", "ask", "vadr"}
							for _, keyword := range microstructureKeywords {
								if strings.Contains(context, keyword) {
									t.Errorf("CONFORMANCE VIOLATION: %s uses banned aggregator '%s' for microstructure data",
										filePath, banned)
									break
								}
							}
						}
					}
				}
			}
		})
	}
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
