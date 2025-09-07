package defi

import (
	"fmt"
)

// DefaultDeFiProviderFactory implements DeFiProviderFactory
type DefaultDeFiProviderFactory struct{}

// NewDeFiProviderFactory creates a new DeFi provider factory
func NewDeFiProviderFactory() DeFiProviderFactory {
	return &DefaultDeFiProviderFactory{}
}

// CreateTheGraphProvider creates The Graph subgraph provider (free tier)
func (f *DefaultDeFiProviderFactory) CreateTheGraphProvider(config DeFiProviderConfig) (DeFiProvider, error) {
	if config.DataSource == "" {
		config.DataSource = "thegraph"
	}
	
	// Validate configuration
	if err := f.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration for The Graph provider: %w", err)
	}
	
	provider, err := NewTheGraphProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create The Graph provider: %w", err)
	}
	
	return provider, nil
}

// CreateDeFiLlamaProvider creates DeFiLlama API provider (free tier)
func (f *DefaultDeFiProviderFactory) CreateDeFiLlamaProvider(config DeFiProviderConfig) (DeFiProvider, error) {
	if config.DataSource == "" {
		config.DataSource = "defillama"
	}
	
	// Validate configuration
	if err := f.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration for DeFiLlama provider: %w", err)
	}
	
	provider, err := NewDeFiLlamaProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create DeFiLlama provider: %w", err)
	}
	
	return provider, nil
}

// GetAvailableProviders returns list of available DeFi data sources
func (f *DefaultDeFiProviderFactory) GetAvailableProviders() []string {
	return []string{
		"thegraph",
		"defillama",
	}
}

// validateConfig validates DeFi provider configuration
func (f *DefaultDeFiProviderFactory) validateConfig(config DeFiProviderConfig) error {
	// Ensure USD pairs only constraint is enabled
	if !config.USDPairsOnly {
		return fmt.Errorf("USDPairsOnly must be enabled for CryptoRun compliance")
	}
	
	// Validate rate limits to respect free tier limits
	maxRateLimits := map[string]float64{
		"thegraph":  10.0, // Conservative limit for The Graph
		"defillama": 5.0,  // Conservative limit for DeFiLlama
	}
	
	if maxRate, ok := maxRateLimits[config.DataSource]; ok {
		if config.RateLimitRPS > maxRate {
			return fmt.Errorf("rate limit %f RPS exceeds maximum %f for %s free tier", 
				config.RateLimitRPS, maxRate, config.DataSource)
		}
	}
	
	// Validate timeout settings
	if config.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	
	if config.RetryBackoff <= 0 {
		return fmt.Errorf("retry backoff must be positive")
	}
	
	// Validate PIT shift periods
	if config.PITShiftPeriods < 0 {
		return fmt.Errorf("PIT shift periods cannot be negative")
	}
	
	if config.PITShiftPeriods > 24 {
		return fmt.Errorf("PIT shift periods cannot exceed 24 hours")
	}
	
	return nil
}

// CreateDefaultConfig creates a default configuration for a DeFi provider
func CreateDefaultConfig(dataSource string) DeFiProviderConfig {
	baseConfig := DeFiProviderConfig{
		DataSource:      dataSource,
		RequestTimeout:  30000000000, // 30 seconds in nanoseconds
		MaxRetries:      3,
		RetryBackoff:    1000000000,  // 1 second in nanoseconds
		PITShiftPeriods: 0,           // No PIT shift by default
		EnableMetrics:   true,
		USDPairsOnly:    true,        // Always enforce USD pairs constraint
		UserAgent:       "CryptoRun/1.0 (DeFi-metrics)",
	}
	
	// Data source specific defaults
	switch dataSource {
	case "thegraph":
		baseConfig.BaseURL = "https://api.thegraph.com/subgraphs/name"
		baseConfig.RateLimitRPS = 5.0 // Conservative for free tier
		
	case "defillama":
		baseConfig.BaseURL = "https://api.llama.fi"
		baseConfig.RateLimitRPS = 3.0 // Conservative for free tier
		
	default:
		// Generic defaults
		baseConfig.RateLimitRPS = 2.0
	}
	
	return baseConfig
}

// CreateProviderFromConfig creates a provider from configuration
func CreateProviderFromConfig(config DeFiProviderConfig) (DeFiProvider, error) {
	factory := NewDeFiProviderFactory()
	
	switch config.DataSource {
	case "thegraph":
		return factory.CreateTheGraphProvider(config)
		
	case "defillama":
		return factory.CreateDeFiLlamaProvider(config)
		
	default:
		return nil, fmt.Errorf("unsupported DeFi provider data source: %s", config.DataSource)
	}
}