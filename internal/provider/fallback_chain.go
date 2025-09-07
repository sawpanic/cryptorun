package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProviderChain manages fallback between multiple providers for resilience
type ProviderChain struct {
	name        string
	providers   []ExchangeProvider
	mu          sync.RWMutex
	
	// Metrics callback
	metricsCallback func(string, interface{})
}

// NewProviderChain creates a new provider chain with fallback capability
func NewProviderChain(name string, providers []ExchangeProvider) *ProviderChain {
	if len(providers) == 0 {
		panic("provider chain must have at least one provider")
	}
	
	return &ProviderChain{
		name:      name,
		providers: providers,
	}
}

// SetMetricsCallback sets a callback for metrics collection
func (pc *ProviderChain) SetMetricsCallback(callback func(string, interface{})) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.metricsCallback = callback
}

// GetOrderBookWithFallback attempts to get order book data with provider fallback
func (pc *ProviderChain) GetOrderBookWithFallback(ctx context.Context, symbol string) (*OrderBookData, error) {
	pc.mu.RLock()
	providers := make([]ExchangeProvider, len(pc.providers))
	copy(providers, pc.providers)
	pc.mu.RUnlock()
	
	var lastErr error
	
	for i, provider := range providers {
		// Check provider health first
		health := provider.Health()
		if !health.Healthy {
			pc.emitMetric(fmt.Sprintf("provider_%s_skipped_unhealthy", provider.GetVenue()), 1)
			lastErr = fmt.Errorf("provider %s is unhealthy: %s", provider.GetVenue(), health.Status)
			continue
		}
		
		start := time.Now()
		data, err := provider.GetOrderBook(ctx, symbol)
		duration := time.Since(start)
		
		pc.emitMetric("provider_chain_attempt", 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_attempt", provider.GetVenue()), 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_latency_ms", provider.GetVenue()), duration.Milliseconds())
		
		if err == nil {
			// Success - emit success metrics
			pc.emitMetric("provider_chain_success", 1)
			pc.emitMetric(fmt.Sprintf("provider_%s_success", provider.GetVenue()), 1)
			pc.emitMetric("provider_chain_attempts_to_success", i+1)
			
			return data, nil
		}
		
		// Check if it's a circuit breaker error
		if providerErr, ok := err.(*ProviderError); ok {
			if providerErr.Code == ErrCodeCircuitOpen {
				pc.emitMetric(fmt.Sprintf("provider_%s_circuit_open", provider.GetVenue()), 1)
			} else if providerErr.Code == ErrCodeRateLimit {
				pc.emitMetric(fmt.Sprintf("provider_%s_rate_limited", provider.GetVenue()), 1)
			}
		}
		
		pc.emitMetric(fmt.Sprintf("provider_%s_failure", provider.GetVenue()), 1)
		lastErr = err
		
		// Add small delay between provider attempts to avoid hammering
		if i < len(providers)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	
	// All providers failed
	pc.emitMetric("provider_chain_all_failed", 1)
	
	return nil, &ProviderError{
		Provider:  pc.name,
		Code:      ErrCodeAPIError,
		Message:   fmt.Sprintf("all providers in chain failed, last error: %v", lastErr),
		Temporary: true,
		Cause:     lastErr,
	}
}

// GetTradesWithFallback attempts to get trade data with provider fallback
func (pc *ProviderChain) GetTradesWithFallback(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	pc.mu.RLock()
	providers := make([]ExchangeProvider, len(pc.providers))
	copy(providers, pc.providers)
	pc.mu.RUnlock()
	
	var lastErr error
	
	for i, provider := range providers {
		// Check provider health first
		health := provider.Health()
		if !health.Healthy {
			pc.emitMetric(fmt.Sprintf("provider_%s_skipped_unhealthy", provider.GetVenue()), 1)
			lastErr = fmt.Errorf("provider %s is unhealthy: %s", provider.GetVenue(), health.Status)
			continue
		}
		
		start := time.Now()
		data, err := provider.GetTrades(ctx, symbol, limit)
		duration := time.Since(start)
		
		pc.emitMetric("provider_chain_attempt", 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_attempt", provider.GetVenue()), 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_latency_ms", provider.GetVenue()), duration.Milliseconds())
		
		if err == nil {
			// Success - emit success metrics
			pc.emitMetric("provider_chain_success", 1)
			pc.emitMetric(fmt.Sprintf("provider_%s_success", provider.GetVenue()), 1)
			pc.emitMetric("provider_chain_attempts_to_success", i+1)
			
			return data, nil
		}
		
		// Check if it's a circuit breaker error
		if providerErr, ok := err.(*ProviderError); ok {
			if providerErr.Code == ErrCodeCircuitOpen {
				pc.emitMetric(fmt.Sprintf("provider_%s_circuit_open", provider.GetVenue()), 1)
			} else if providerErr.Code == ErrCodeRateLimit {
				pc.emitMetric(fmt.Sprintf("provider_%s_rate_limited", provider.GetVenue()), 1)
			}
		}
		
		pc.emitMetric(fmt.Sprintf("provider_%s_failure", provider.GetVenue()), 1)
		lastErr = err
		
		// Add small delay between provider attempts
		if i < len(providers)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	
	// All providers failed
	pc.emitMetric("provider_chain_all_failed", 1)
	
	return nil, &ProviderError{
		Provider:  pc.name,
		Code:      ErrCodeAPIError,
		Message:   fmt.Sprintf("all providers in chain failed, last error: %v", lastErr),
		Temporary: true,
		Cause:     lastErr,
	}
}

// GetKlinesWithFallback attempts to get klines data with provider fallback
func (pc *ProviderChain) GetKlinesWithFallback(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	pc.mu.RLock()
	providers := make([]ExchangeProvider, len(pc.providers))
	copy(providers, pc.providers)
	pc.mu.RUnlock()
	
	var lastErr error
	
	for i, provider := range providers {
		// Check provider health first
		health := provider.Health()
		if !health.Healthy {
			pc.emitMetric(fmt.Sprintf("provider_%s_skipped_unhealthy", provider.GetVenue()), 1)
			lastErr = fmt.Errorf("provider %s is unhealthy: %s", provider.GetVenue(), health.Status)
			continue
		}
		
		start := time.Now()
		data, err := provider.GetKlines(ctx, symbol, interval, limit)
		duration := time.Since(start)
		
		pc.emitMetric("provider_chain_attempt", 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_attempt", provider.GetVenue()), 1)
		pc.emitMetric(fmt.Sprintf("provider_%s_latency_ms", provider.GetVenue()), duration.Milliseconds())
		
		if err == nil {
			// Success - emit success metrics
			pc.emitMetric("provider_chain_success", 1)
			pc.emitMetric(fmt.Sprintf("provider_%s_success", provider.GetVenue()), 1)
			pc.emitMetric("provider_chain_attempts_to_success", i+1)
			
			return data, nil
		}
		
		// Check if it's a circuit breaker error
		if providerErr, ok := err.(*ProviderError); ok {
			if providerErr.Code == ErrCodeCircuitOpen {
				pc.emitMetric(fmt.Sprintf("provider_%s_circuit_open", provider.GetVenue()), 1)
			} else if providerErr.Code == ErrCodeRateLimit {
				pc.emitMetric(fmt.Sprintf("provider_%s_rate_limited", provider.GetVenue()), 1)
			}
		}
		
		pc.emitMetric(fmt.Sprintf("provider_%s_failure", provider.GetVenue()), 1)
		lastErr = err
		
		// Add small delay between provider attempts
		if i < len(providers)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	
	// All providers failed
	pc.emitMetric("provider_chain_all_failed", 1)
	
	return nil, &ProviderError{
		Provider:  pc.name,
		Code:      ErrCodeAPIError,
		Message:   fmt.Sprintf("all providers in chain failed, last error: %v", lastErr),
		Temporary: true,
		Cause:     lastErr,
	}
}

// GetHealthyProviders returns only healthy providers from the chain
func (pc *ProviderChain) GetHealthyProviders() []ExchangeProvider {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	var healthy []ExchangeProvider
	for _, provider := range pc.providers {
		if provider.Health().Healthy {
			healthy = append(healthy, provider)
		}
	}
	
	return healthy
}

// GetChainHealth returns the health status of the entire chain
func (pc *ProviderChain) GetChainHealth() ProviderChainHealth {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	var totalProviders = len(pc.providers)
	var healthyProviders = 0
	var providerStatuses []ProviderStatus
	
	for _, provider := range pc.providers {
		health := provider.Health()
		status := ProviderStatus{
			Venue:   provider.GetVenue(),
			Healthy: health.Healthy,
			Status:  health.Status,
			CircuitState: health.CircuitState,
			ResponseTime: health.ResponseTime,
			Metrics: health.Metrics,
		}
		providerStatuses = append(providerStatuses, status)
		
		if health.Healthy {
			healthyProviders++
		}
	}
	
	// Chain is healthy if at least one provider is healthy
	chainHealthy := healthyProviders > 0
	healthRatio := float64(healthyProviders) / float64(totalProviders)
	
	return ProviderChainHealth{
		ChainName:         pc.name,
		Healthy:           chainHealthy,
		TotalProviders:    totalProviders,
		HealthyProviders:  healthyProviders,
		HealthRatio:       healthRatio,
		ProviderStatuses:  providerStatuses,
		LastCheck:         time.Now(),
	}
}

// ReorderProviders reorders the provider chain based on health and performance
func (pc *ProviderChain) ReorderProviders() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	// Sort providers by health and performance
	// Healthy providers with better response times go first
	providers := make([]providerWithScore, len(pc.providers))
	
	for i, provider := range pc.providers {
		health := provider.Health()
		score := calculateProviderScore(health)
		providers[i] = providerWithScore{
			provider: provider,
			score:    score,
		}
	}
	
	// Sort by score (higher is better)
	for i := 0; i < len(providers)-1; i++ {
		for j := i + 1; j < len(providers); j++ {
			if providers[j].score > providers[i].score {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}
	
	// Update provider order
	for i, p := range providers {
		pc.providers[i] = p.provider
	}
	
	pc.emitMetric("provider_chain_reordered", 1)
}

// emitMetric sends metrics if callback is set
func (pc *ProviderChain) emitMetric(metric string, value interface{}) {
	if pc.metricsCallback != nil {
		pc.metricsCallback(metric, value)
	}
}

// ProviderChainHealth represents the health of an entire provider chain
type ProviderChainHealth struct {
	ChainName         string           `json:"chain_name"`
	Healthy           bool             `json:"healthy"`
	TotalProviders    int              `json:"total_providers"`
	HealthyProviders  int              `json:"healthy_providers"`
	HealthRatio       float64          `json:"health_ratio"`
	ProviderStatuses  []ProviderStatus `json:"provider_statuses"`
	LastCheck         time.Time        `json:"last_check"`
}

// ProviderStatus represents the status of a single provider in the chain
type ProviderStatus struct {
	Venue         string            `json:"venue"`
	Healthy       bool              `json:"healthy"`
	Status        string            `json:"status"`
	CircuitState  string            `json:"circuit_state"`
	ResponseTime  time.Duration     `json:"response_time"`
	Metrics       ProviderMetrics   `json:"metrics"`
}

// Helper types for provider reordering
type providerWithScore struct {
	provider ExchangeProvider
	score    float64
}

// calculateProviderScore calculates a score for provider ordering
// Higher scores indicate better providers that should be tried first
func calculateProviderScore(health ProviderHealth) float64 {
	score := 0.0
	
	// Base score for health
	if health.Healthy {
		score += 100.0
	}
	
	// Success rate contribution (0-50 points)
	score += health.Metrics.SuccessRate * 50.0
	
	// Response time contribution (lower is better, 0-25 points)
	if health.ResponseTime > 0 {
		// Penalty for slow responses (>1s gets 0 points, <100ms gets full points)
		responseMs := float64(health.ResponseTime.Milliseconds())
		if responseMs < 100 {
			score += 25.0
		} else if responseMs < 1000 {
			score += 25.0 * (1000.0 - responseMs) / 900.0
		}
	}
	
	// Circuit breaker state penalty
	if health.CircuitState == "open" {
		score -= 50.0  // Heavy penalty for open circuits
	} else if health.CircuitState == "half-open" {
		score -= 10.0  // Light penalty for half-open
	}
	
	return score
}