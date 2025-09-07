package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ProviderConfig represents the provider configuration from YAML
type ProviderConfig struct {
	Defaults  DefaultConfig            `yaml:"defaults"`
	Providers map[string]ProviderSpec  `yaml:"providers"`
	Cache     CacheConfig              `yaml:"cache"`
	Security  SecurityConfig           `yaml:"security"`
}

// DefaultConfig contains default settings for all providers
type DefaultConfig struct {
	TTLSeconds      int     `yaml:"ttl_seconds"`
	BurstLimit      int     `yaml:"burst_limit"`
	SustainedRate   float64 `yaml:"sustained_rate"`
	MaxRetries      int     `yaml:"max_retries"`
	BackoffBaseMs   int     `yaml:"backoff_base_ms"`
	FailureThresh   float64 `yaml:"failure_thresh"`
	WindowRequests  int     `yaml:"window_requests"`
	ProbeInterval   int     `yaml:"probe_interval"`
	EnableFileCache bool    `yaml:"enable_file_cache"`
	CachePath       string  `yaml:"cache_path"`
}

// ProviderSpec defines configuration for a specific provider
type ProviderSpec struct {
	Name            string  `yaml:"name"`
	TTLSeconds      int     `yaml:"ttl_seconds"`
	BurstLimit      int     `yaml:"burst_limit"`
	SustainedRate   float64 `yaml:"sustained_rate"`
	MaxRetries      int     `yaml:"max_retries"`
	BackoffBaseMs   int     `yaml:"backoff_base_ms"`
	FailureThresh   float64 `yaml:"failure_thresh"`
	WindowRequests  int     `yaml:"window_requests"`
	ProbeInterval   int     `yaml:"probe_interval"`
	EnableFileCache bool    `yaml:"enable_file_cache"`
	CachePath       string  `yaml:"cache_path"`
	Priority        int     `yaml:"priority,omitempty"` // Lower number = higher priority
}

// CacheConfig contains cache-related settings
type CacheConfig struct {
	CleanupInterval    int    `yaml:"cleanup_interval"`
	MaxMemoryEntries   int    `yaml:"max_memory_entries"`
	MaxFileSizeMB      int    `yaml:"max_file_size_mb"`
	BasePath           string `yaml:"base_path"`
	EnableETag         bool   `yaml:"enable_etag"`
	EnableIfModified   bool   `yaml:"enable_if_modified"`
	StaleThreshold     int    `yaml:"stale_threshold"`
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	MaxURLLength     int      `yaml:"max_url_length"`
	MaxHeaderSize    int      `yaml:"max_header_size"`
	AllowedSchemes   []string `yaml:"allowed_schemes"`
	ExcludedHeaders  []string `yaml:"excluded_headers"`
	UserAgent        string   `yaml:"user_agent"`
}

// ConfigurableProviderRegistry extends ProviderRegistry with config-driven provider management
type ConfigurableProviderRegistry struct {
	*ProviderRegistry
	config       *ProviderConfig
	capabilities map[Capability][]string // Ordered provider names per capability
}

// NewConfigurableProviderRegistry creates a registry with config-driven fallback chains
func NewConfigurableProviderRegistry(configPath string) (*ConfigurableProviderRegistry, error) {
	config, err := LoadProviderConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load provider config: %w", err)
	}
	
	registry := &ConfigurableProviderRegistry{
		ProviderRegistry: NewProviderRegistry(),
		config:          config,
		capabilities:    make(map[Capability][]string),
	}
	
	// Initialize providers based on configuration
	if err := registry.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}
	
	return registry, nil
}

// LoadProviderConfig loads provider configuration from YAML file
func LoadProviderConfig(configPath string) (*ProviderConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config ProviderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	return &config, nil
}

// initializeProviders creates and registers providers based on configuration
func (r *ConfigurableProviderRegistry) initializeProviders() error {
	// Define provider factory functions
	providerFactories := map[string]func(*ProviderSpec) (Provider, error){
		"binance":   r.createBinanceProvider,
		"okx":       r.createOKXProvider,
		"coinbase":  r.createCoinbaseProvider,
		"kraken":    r.createKrakenProvider,
		"coingecko": r.createCoingeckoProvider,
	}
	
	// Create and register providers in priority order
	for name, spec := range r.config.Providers {
		factory, exists := providerFactories[name]
		if !exists {
			return fmt.Errorf("unknown provider: %s", name)
		}
		
		provider, err := factory(&spec)
		if err != nil {
			return fmt.Errorf("failed to create provider %s: %w", name, err)
		}
		
		if err := r.RegisterProvider(provider); err != nil {
			return fmt.Errorf("failed to register provider %s: %w", name, err)
		}
	}
	
	// Build capability fallback chains based on priority
	r.buildCapabilityChains()
	
	return nil
}

// createBinanceProvider creates a configured Binance provider
func (r *ConfigurableProviderRegistry) createBinanceProvider(spec *ProviderSpec) (Provider, error) {
	provider := NewBinanceProvider()
	
	// Apply rate limiting configuration
	rps := int(spec.SustainedRate)
	if rps <= 0 {
		rps = int(r.config.Defaults.SustainedRate)
	}
	
	burstLimit := spec.BurstLimit
	if burstLimit <= 0 {
		burstLimit = r.config.Defaults.BurstLimit
	}
	
	provider.rateLimiter = NewRateLimiter(burstLimit*rps, rps)
	
	return provider, nil
}

// createOKXProvider creates a configured OKX provider
func (r *ConfigurableProviderRegistry) createOKXProvider(spec *ProviderSpec) (Provider, error) {
	provider := NewOKXProvider()
	
	// Apply rate limiting configuration
	rps := int(spec.SustainedRate)
	if rps <= 0 {
		rps = int(r.config.Defaults.SustainedRate)
	}
	
	burstLimit := spec.BurstLimit
	if burstLimit <= 0 {
		burstLimit = r.config.Defaults.BurstLimit
	}
	
	provider.rateLimiter = NewRateLimiter(burstLimit*rps, rps)
	
	return provider, nil
}

// createCoinbaseProvider creates a configured Coinbase provider
func (r *ConfigurableProviderRegistry) createCoinbaseProvider(spec *ProviderSpec) (Provider, error) {
	provider := NewCoinbaseProvider()
	
	// Apply rate limiting configuration
	rps := int(spec.SustainedRate)
	if rps <= 0 {
		rps = int(r.config.Defaults.SustainedRate)
	}
	
	burstLimit := spec.BurstLimit
	if burstLimit <= 0 {
		burstLimit = r.config.Defaults.BurstLimit
	}
	
	provider.rateLimiter = NewRateLimiter(burstLimit*rps, rps)
	
	return provider, nil
}

// createKrakenProvider creates a configured Kraken provider
func (r *ConfigurableProviderRegistry) createKrakenProvider(spec *ProviderSpec) (Provider, error) {
	provider := NewKrakenProvider()
	
	// Apply rate limiting configuration
	rps := int(spec.SustainedRate)
	if rps <= 0 {
		rps = int(r.config.Defaults.SustainedRate)
	}
	
	burstLimit := spec.BurstLimit
	if burstLimit <= 0 {
		burstLimit = r.config.Defaults.BurstLimit
	}
	
	provider.rateLimiter = NewRateLimiter(burstLimit*rps, rps)
	
	return provider, nil
}

// createCoingeckoProvider creates a configured CoinGecko provider
func (r *ConfigurableProviderRegistry) createCoingeckoProvider(spec *ProviderSpec) (Provider, error) {
	provider := NewCoingeckoProvider()
	
	// Apply rate limiting configuration
	rps := int(spec.SustainedRate)
	if rps <= 0 {
		rps = int(r.config.Defaults.SustainedRate)
	}
	
	burstLimit := spec.BurstLimit
	if burstLimit <= 0 {
		burstLimit = r.config.Defaults.BurstLimit
	}
	
	provider.rateLimiter = NewRateLimiter(burstLimit*rps, rps)
	
	return provider, nil
}

// buildCapabilityChains builds ordered provider chains for each capability based on priority
func (r *ConfigurableProviderRegistry) buildCapabilityChains() {
	capabilities := []Capability{
		CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2,
		CapabilityKlineData, CapabilitySupplyReserves, CapabilityWhaleDetection,
		CapabilityCVD,
	}
	
	for _, cap := range capabilities {
		providers := r.GetProviders(cap)
		if len(providers) == 0 {
			continue
		}
		
		// Sort providers by priority (defined in config)
		providerNames := make([]string, 0, len(providers))
		for _, provider := range providers {
			providerNames = append(providerNames, provider.Name())
		}
		
		// For now, use the order from config (Kraken first as preferred)
		orderedNames := []string{}
		
		// Preferred order: kraken, binance, okx, coinbase, coingecko
		preferredOrder := []string{"kraken", "binance", "okx", "coinbase", "coingecko"}
		for _, preferred := range preferredOrder {
			for _, name := range providerNames {
				if name == preferred {
					orderedNames = append(orderedNames, name)
					break
				}
			}
		}
		
		// Add any remaining providers
		for _, name := range providerNames {
			found := false
			for _, ordered := range orderedNames {
				if name == ordered {
					found = true
					break
				}
			}
			if !found {
				orderedNames = append(orderedNames, name)
			}
		}
		
		r.capabilities[cap] = orderedNames
	}
}

// GetProvidersForCapability returns providers for a capability in fallback order
func (r *ConfigurableProviderRegistry) GetProvidersForCapability(cap Capability) []Provider {
	providerNames := r.capabilities[cap]
	if len(providerNames) == 0 {
		return []Provider{}
	}
	
	allProviders := r.GetProviders(cap)
	orderedProviders := make([]Provider, 0, len(allProviders))
	
	// Return providers in configured fallback order
	for _, name := range providerNames {
		for _, provider := range allProviders {
			if provider.Name() == name {
				orderedProviders = append(orderedProviders, provider)
				break
			}
		}
	}
	
	return orderedProviders
}

// GetConfig returns the loaded configuration
func (r *ConfigurableProviderRegistry) GetConfig() *ProviderConfig {
	return r.config
}

// GetTTL returns the configured TTL for a provider
func (r *ConfigurableProviderRegistry) GetTTL(providerName string) time.Duration {
	if spec, exists := r.config.Providers[providerName]; exists {
		if spec.TTLSeconds > 0 {
			return time.Duration(spec.TTLSeconds) * time.Second
		}
	}
	
	return time.Duration(r.config.Defaults.TTLSeconds) * time.Second
}

// GetCachePath returns the configured cache path for a provider
func (r *ConfigurableProviderRegistry) GetCachePath(providerName string) string {
	if spec, exists := r.config.Providers[providerName]; exists {
		if spec.CachePath != "" {
			return spec.CachePath
		}
	}
	
	// Fallback to default cache path
	basePath := r.config.Cache.BasePath
	if basePath == "" {
		basePath = "artifacts/cache"
	}
	
	return filepath.Join(basePath, fmt.Sprintf("%s.json", providerName))
}

// IsFileCache returns whether file caching is enabled for a provider
func (r *ConfigurableProviderRegistry) IsFileCacheEnabled(providerName string) bool {
	if spec, exists := r.config.Providers[providerName]; exists {
		return spec.EnableFileCache
	}
	
	return r.config.Defaults.EnableFileCache
}