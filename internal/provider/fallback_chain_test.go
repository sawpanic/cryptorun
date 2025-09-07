package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProviderChain_GetOrderBookWithFallback(t *testing.T) {
	// Create mock providers
	healthyProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "healthy",
			venue:   "healthy-exchange",
			healthy: true,
		},
	}
	
	unhealthyProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "unhealthy",
			venue:   "unhealthy-exchange",
			healthy: false,
		},
	}
	
	failingProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "failing",
			venue:   "failing-exchange",
			healthy: true,
		},
		shouldFail: true,
	}
	
	// Test case 1: First provider succeeds
	t.Run("first_provider_succeeds", func(t *testing.T) {
		chain := NewProviderChain("test-chain", []ExchangeProvider{healthyProvider, unhealthyProvider})
		
		data, err := chain.GetOrderBookWithFallback(context.Background(), "BTC-USD")
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		
		if data.Venue != "healthy-exchange" {
			t.Errorf("Expected venue 'healthy-exchange', got '%s'", data.Venue)
		}
	})
	
	// Test case 2: First provider fails, second succeeds
	t.Run("fallback_to_second_provider", func(t *testing.T) {
		chain := NewProviderChain("test-chain", []ExchangeProvider{unhealthyProvider, healthyProvider})
		
		data, err := chain.GetOrderBookWithFallback(context.Background(), "BTC-USD")
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		
		if data.Venue != "healthy-exchange" {
			t.Errorf("Expected venue 'healthy-exchange', got '%s'", data.Venue)
		}
	})
	
	// Test case 3: All providers fail
	t.Run("all_providers_fail", func(t *testing.T) {
		chain := NewProviderChain("test-chain", []ExchangeProvider{unhealthyProvider, failingProvider})
		
		_, err := chain.GetOrderBookWithFallback(context.Background(), "BTC-USD")
		if err == nil {
			t.Fatal("Expected error when all providers fail")
		}
		
		providerErr, ok := err.(*ProviderError)
		if !ok {
			t.Errorf("Expected ProviderError, got %T", err)
		}
		
		if providerErr.Code != ErrCodeAPIError {
			t.Errorf("Expected API error code, got %s", providerErr.Code)
		}
	})
}

func TestProviderChain_GetChainHealth(t *testing.T) {
	healthyProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "healthy",
			venue:   "healthy-exchange",
			healthy: true,
		},
	}
	
	unhealthyProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "unhealthy",
			venue:   "unhealthy-exchange",
			healthy: false,
		},
	}
	
	chain := NewProviderChain("test-chain", []ExchangeProvider{healthyProvider, unhealthyProvider})
	
	health := chain.GetChainHealth()
	
	if !health.Healthy {
		t.Error("Expected chain to be healthy when at least one provider is healthy")
	}
	
	if health.TotalProviders != 2 {
		t.Errorf("Expected 2 total providers, got %d", health.TotalProviders)
	}
	
	if health.HealthyProviders != 1 {
		t.Errorf("Expected 1 healthy provider, got %d", health.HealthyProviders)
	}
	
	if health.HealthRatio != 0.5 {
		t.Errorf("Expected health ratio of 0.5, got %f", health.HealthRatio)
	}
	
	if len(health.ProviderStatuses) != 2 {
		t.Errorf("Expected 2 provider statuses, got %d", len(health.ProviderStatuses))
	}
}

func TestProviderChain_ReorderProviders(t *testing.T) {
	slowProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "slow",
			venue:   "slow-exchange",
			healthy: true,
		},
		responseTime: 2 * time.Second,
	}
	
	fastProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "fast",
			venue:   "fast-exchange",
			healthy: true,
		},
		responseTime: 50 * time.Millisecond,
	}
	
	unhealthyProvider := &EnhancedMockExchangeProvider{
		MockExchangeProvider: MockExchangeProvider{
			name:    "unhealthy",
			venue:   "unhealthy-exchange",
			healthy: false,
		},
	}
	
	// Start with slow provider first
	chain := NewProviderChain("test-chain", []ExchangeProvider{slowProvider, fastProvider, unhealthyProvider})
	
	// Reorder based on performance
	chain.ReorderProviders()
	
	// First provider should now be the fast one
	health := chain.GetChainHealth()
	if health.ProviderStatuses[0].Venue != "fast-exchange" {
		t.Errorf("Expected first provider to be 'fast-exchange', got '%s'", health.ProviderStatuses[0].Venue)
	}
	
	// Last provider should be the unhealthy one
	lastIndex := len(health.ProviderStatuses) - 1
	if health.ProviderStatuses[lastIndex].Venue != "unhealthy-exchange" {
		t.Errorf("Expected last provider to be 'unhealthy-exchange', got '%s'", health.ProviderStatuses[lastIndex].Venue)
	}
}

func TestProviderScore_Calculation(t *testing.T) {
	// Test healthy provider with good metrics
	healthyHealth := ProviderHealth{
		Healthy:      true,
		ResponseTime: 100 * time.Millisecond,
		CircuitState: "closed",
		Metrics: ProviderMetrics{
			SuccessRate: 0.95,
		},
	}
	
	score1 := calculateProviderScore(healthyHealth)
	
	// Test unhealthy provider
	unhealthyHealth := ProviderHealth{
		Healthy:      false,
		ResponseTime: 100 * time.Millisecond,
		CircuitState: "closed",
		Metrics: ProviderMetrics{
			SuccessRate: 0.50,
		},
	}
	
	score2 := calculateProviderScore(unhealthyHealth)
	
	// Test provider with open circuit
	openCircuitHealth := ProviderHealth{
		Healthy:      true,
		ResponseTime: 100 * time.Millisecond,
		CircuitState: "open",
		Metrics: ProviderMetrics{
			SuccessRate: 0.95,
		},
	}
	
	score3 := calculateProviderScore(openCircuitHealth)
	
	// Healthy provider should score higher than unhealthy
	if score1 <= score2 {
		t.Errorf("Healthy provider should score higher: %f vs %f", score1, score2)
	}
	
	// Healthy provider should score higher than open circuit
	if score1 <= score3 {
		t.Errorf("Healthy provider should score higher than open circuit: %f vs %f", score1, score3)
	}
}

func TestCircuitBreakerEnhancements(t *testing.T) {
	config := CircuitConfig{
		Enabled:          true,
		FailureThreshold: 0.5,
		MinRequests:      2,
		OpenTimeout:      100 * time.Millisecond,
		ProbeInterval:    50 * time.Millisecond,
		MaxFailures:      3,
	}
	
	cb := NewCircuitBreaker("test-enhanced", config)
	defer cb.stopRestoreProbe()
	
	t.Run("exponential_backoff", func(t *testing.T) {
		// Generate some failures to trigger backoff
		for i := 0; i < 4; i++ {
			err := cb.Call(func() error {
				return errors.New("test failure")
			})
			if err == nil && i >= 3 {
				// Circuit should be open after enough failures
				t.Error("Expected circuit to be open")
			}
		}
		
		stats := cb.GetStats()
		if stats.State != "open" {
			t.Errorf("Expected circuit to be open, got %s", stats.State)
		}
		
		// Check that backoff is being tracked
		if stats.CurrentBackoff == "" {
			t.Error("Expected current backoff to be tracked")
		}
	})
	
	t.Run("restore_probe_functionality", func(t *testing.T) {
		// The probe should be running even if circuit is closed
		stats := cb.GetStats()
		if !stats.ProbeRunning {
			t.Error("Expected restore probe to be running")
		}
		
		// The probe only acts when circuit is open, so we shouldn't expect
		// LastProbeTime to be set unless circuit was open
		// This is correct behavior
	})
}

// Enhanced MockExchangeProvider for testing
type EnhancedMockExchangeProvider struct {
	MockExchangeProvider
	responseTime time.Duration
	shouldFail   bool
	failureCount int
	circuitState string
}

func (m *EnhancedMockExchangeProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	// Simulate response time
	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}
	
	if m.shouldFail {
		m.failureCount++
		return nil, &ProviderError{
			Provider: m.venue,
			Code:     ErrCodeAPIError,
			Message:  "simulated failure",
		}
	}
	
	return &OrderBookData{
		Venue:     m.venue,
		Symbol:    symbol,
		BestBid:   50000.0,
		BestAsk:   50010.0,
		Timestamp: time.Now(),
	}, nil
}

func (m *EnhancedMockExchangeProvider) Health() ProviderHealth {
	successRate := 1.0
	if m.failureCount > 0 {
		successRate = 0.5
	}
	
	circuitState := m.circuitState
	if circuitState == "" {
		circuitState = "closed"
	}
	
	return ProviderHealth{
		Healthy:      m.healthy,
		Status:       "mock",
		ResponseTime: m.responseTime,
		CircuitState: circuitState,
		Metrics: ProviderMetrics{
			SuccessRate: successRate,
		},
	}
}