package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DefaultProviderRegistry implements ProviderRegistry
type DefaultProviderRegistry struct {
	providers       map[string]ExchangeProvider
	circuitManager  *CircuitBreakerManager
	mu              sync.RWMutex
	started         bool
	
	// Health monitoring
	healthCheckInterval time.Duration
	stopHealthCheck     chan bool
	
	// Metrics callback
	metricsCallback func(string, interface{})
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *DefaultProviderRegistry {
	return &DefaultProviderRegistry{
		providers:           make(map[string]ExchangeProvider),
		circuitManager:      NewCircuitBreakerManager(),
		healthCheckInterval: 30 * time.Second,
		stopHealthCheck:     make(chan bool),
	}
}

// SetMetricsCallback sets a callback for metrics collection
func (r *DefaultProviderRegistry) SetMetricsCallback(callback func(string, interface{})) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metricsCallback = callback
}

// SetHealthCheckInterval configures health check frequency
func (r *DefaultProviderRegistry) SetHealthCheckInterval(interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.healthCheckInterval = interval
}

// Register adds a provider to the registry
func (r *DefaultProviderRegistry) Register(provider ExchangeProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	venue := provider.GetVenue()
	if venue == "" {
		return fmt.Errorf("provider must have a non-empty venue")
	}
	
	if _, exists := r.providers[venue]; exists {
		return fmt.Errorf("provider for venue %s already registered", venue)
	}
	
	r.providers[venue] = provider
	
	if r.metricsCallback != nil {
		r.metricsCallback("provider_registered", 1)
	}
	
	return nil
}

// Get retrieves a provider by venue
func (r *DefaultProviderRegistry) Get(venue string) (ExchangeProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	provider, exists := r.providers[venue]
	if !exists {
		return nil, fmt.Errorf("no provider registered for venue: %s", venue)
	}
	
	return provider, nil
}

// GetAll returns all registered providers
func (r *DefaultProviderRegistry) GetAll() []ExchangeProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	providers := make([]ExchangeProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	
	return providers
}

// GetHealthy returns only healthy providers
func (r *DefaultProviderRegistry) GetHealthy() []ExchangeProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var healthy []ExchangeProvider
	for _, provider := range r.providers {
		if provider.Health().Healthy {
			healthy = append(healthy, provider)
		}
	}
	
	if r.metricsCallback != nil {
		r.metricsCallback("providers_healthy_count", len(healthy))
		r.metricsCallback("providers_total_count", len(r.providers))
	}
	
	return healthy
}

// GetSupportsDerivatives returns providers that support derivatives
func (r *DefaultProviderRegistry) GetSupportsDerivatives() []ExchangeProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var derivsProviders []ExchangeProvider
	for _, provider := range r.providers {
		if provider.GetSupportsDerivatives() {
			derivsProviders = append(derivsProviders, provider)
		}
	}
	
	return derivsProviders
}

// Start initializes all providers
func (r *DefaultProviderRegistry) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.started {
		return nil
	}
	
	// Start all providers
	for venue, provider := range r.providers {
		if err := provider.Start(ctx); err != nil {
			return fmt.Errorf("failed to start provider %s: %w", venue, err)
		}
	}
	
	// Start health monitoring
	go r.runHealthCheck()
	
	r.started = true
	
	if r.metricsCallback != nil {
		r.metricsCallback("provider_registry_started", 1)
	}
	
	return nil
}

// Stop shuts down all providers
func (r *DefaultProviderRegistry) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if !r.started {
		return nil
	}
	
	// Stop health monitoring
	r.stopHealthCheck <- true
	
	// Stop all providers
	var errors []error
	for venue, provider := range r.providers {
		if err := provider.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop provider %s: %w", venue, err))
		}
	}
	
	r.started = false
	
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping providers: %v", errors)
	}
	
	if r.metricsCallback != nil {
		r.metricsCallback("provider_registry_stopped", 1)
	}
	
	return nil
}

// Health returns health status of all providers
func (r *DefaultProviderRegistry) Health() map[string]ProviderHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	health := make(map[string]ProviderHealth)
	for venue, provider := range r.providers {
		health[venue] = provider.Health()
	}
	
	return health
}

// runHealthCheck periodically monitors provider health
func (r *DefaultProviderRegistry) runHealthCheck() {
	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.performHealthCheck()
		case <-r.stopHealthCheck:
			return
		}
	}
}

// performHealthCheck checks health of all providers
func (r *DefaultProviderRegistry) performHealthCheck() {
	r.mu.RLock()
	providers := make(map[string]ExchangeProvider)
	for venue, provider := range r.providers {
		providers[venue] = provider
	}
	r.mu.RUnlock()
	
	totalProviders := len(providers)
	healthyProviders := 0
	
	for venue, provider := range providers {
		health := provider.Health()
		
		if health.Healthy {
			healthyProviders++
		}
		
		// Emit detailed metrics
		if r.metricsCallback != nil {
			r.metricsCallback(fmt.Sprintf("provider_%s_healthy", venue), boolToInt(health.Healthy))
			r.metricsCallback(fmt.Sprintf("provider_%s_response_time_ms", venue), health.ResponseTime.Milliseconds())
			r.metricsCallback(fmt.Sprintf("provider_%s_success_rate", venue), health.Metrics.SuccessRate)
		}
	}
	
	// Emit aggregate metrics
	if r.metricsCallback != nil {
		r.metricsCallback("providers_total", totalProviders)
		r.metricsCallback("providers_healthy", healthyProviders)
		r.metricsCallback("providers_unhealthy", totalProviders-healthyProviders)
		
		if totalProviders > 0 {
			healthRatio := float64(healthyProviders) / float64(totalProviders)
			r.metricsCallback("providers_health_ratio", healthRatio)
		}
	}
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (r *DefaultProviderRegistry) GetCircuitBreakerStats() map[string]CircuitBreakerStats {
	return r.circuitManager.GetStats()
}

// ResetCircuitBreakers resets all circuit breakers
func (r *DefaultProviderRegistry) ResetCircuitBreakers() {
	r.circuitManager.ResetAll()
}

// boolToInt converts boolean to int for metrics
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ProviderRegistryBuilder helps construct provider registries with configuration
type ProviderRegistryBuilder struct {
	registry        *DefaultProviderRegistry
	configs         map[string]ProviderConfig
	metricsCallback func(string, interface{})
}

// NewProviderRegistryBuilder creates a new builder
func NewProviderRegistryBuilder() *ProviderRegistryBuilder {
	return &ProviderRegistryBuilder{
		registry: NewProviderRegistry(),
		configs:  make(map[string]ProviderConfig),
	}
}

// WithMetricsCallback sets the metrics callback
func (b *ProviderRegistryBuilder) WithMetricsCallback(callback func(string, interface{})) *ProviderRegistryBuilder {
	b.metricsCallback = callback
	b.registry.SetMetricsCallback(callback)
	return b
}

// WithHealthCheckInterval sets the health check interval
func (b *ProviderRegistryBuilder) WithHealthCheckInterval(interval time.Duration) *ProviderRegistryBuilder {
	b.registry.SetHealthCheckInterval(interval)
	return b
}

// AddConfig adds a provider configuration
func (b *ProviderRegistryBuilder) AddConfig(config ProviderConfig) *ProviderRegistryBuilder {
	b.configs[config.Venue] = config
	return b
}

// Build creates the final provider registry with all configured providers
func (b *ProviderRegistryBuilder) Build() (*DefaultProviderRegistry, error) {
	for venue, config := range b.configs {
		var provider ExchangeProvider
		var err error
		
		switch venue {
		case "binance":
			provider, err = NewBinanceProvider(config, b.metricsCallback)
		case "kraken":
			provider, err = NewKrakenProvider(config, b.metricsCallback)
		case "coinbase":
			provider, err = NewCoinbaseProvider(config, b.metricsCallback)
		case "okx":
			provider, err = NewOKXProvider(config, b.metricsCallback)
		default:
			err = fmt.Errorf("unsupported venue: %s", venue)
		}
		
		if err != nil {
			return nil, fmt.Errorf("failed to create provider for %s: %w", venue, err)
		}
		
		if err := b.registry.Register(provider); err != nil {
			return nil, fmt.Errorf("failed to register provider %s: %w", venue, err)
		}
	}
	
	return b.registry, nil
}

// LoadConfigsFromFile loads provider configurations from file
func (b *ProviderRegistryBuilder) LoadConfigsFromFile(filePath string) error {
	// This would load configurations from YAML/JSON file
	// For now, return a stub implementation
	return fmt.Errorf("config loading not implemented yet")
}

// Provider constructors are implemented in separate files:
// - binance_provider.go: NewBinanceProvider
// - kraken_provider.go: NewKrakenProvider  
// - coinbase_provider.go: NewCoinbaseProvider
// - okx_provider.go: NewOKXProvider

// These are now actual implementations, not stubs