package providers

import (
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

type CircuitBreakerManager struct {
	breakers  map[string]*gobreaker.CircuitBreaker
	configs   map[string]*CircuitBreakerConfig
	fallbacks map[string][]string
	mutex     sync.RWMutex
}

type CircuitBreakerConfig struct {
	Name                string
	MaxRequests         uint32
	Interval            time.Duration
	Timeout             time.Duration
	ErrorRateThreshold  float64
	ConsecutiveFailures uint32
	LatencyThreshold    time.Duration
	MonthlyRemaining    int
}

type BreakerStatus struct {
	Name               string
	State              string
	Counts             gobreaker.Counts
	ErrorRate          float64
	ConsecutiveFailures uint32
	NextReset          time.Time
	FallbackChain      []string
}

func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers:  make(map[string]*gobreaker.CircuitBreaker),
		configs:   make(map[string]*CircuitBreakerConfig),
		fallbacks: make(map[string][]string),
	}
}

func (cbm *CircuitBreakerManager) InitializeProvider(name string, config *CircuitBreakerConfig, fallbackChain []string) {
	cbm.mutex.Lock()
	defer cbm.mutex.Unlock()
	
	cbm.configs[name] = config
	cbm.fallbacks[name] = fallbackChain
	
	settings := gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: cbm.createTripCondition(config),
		OnStateChange: cbm.createStateChangeHandler(name),
	}
	
	cbm.breakers[name] = gobreaker.NewCircuitBreaker(settings)
}

func (cbm *CircuitBreakerManager) Execute(provider string, fn func() (interface{}, error)) (interface{}, error) {
	cbm.mutex.RLock()
	breaker, exists := cbm.breakers[provider]
	cbm.mutex.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for provider: %s", provider)
	}
	
	result, err := breaker.Execute(fn)
	
	// If primary fails and circuit is open, try fallback chain
	if err != nil {
		if cbm.isCircuitOpen(provider) {
			return cbm.executeFallbackChain(provider, fn)
		}
	}
	
	return result, err
}

func (cbm *CircuitBreakerManager) GetStatus(provider string) *BreakerStatus {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()
	
	breaker, exists := cbm.breakers[provider]
	if !exists {
		return nil
	}
	
	config := cbm.configs[provider]
	counts := breaker.Counts()
	
	var errorRate float64
	if counts.Requests > 0 {
		errorRate = float64(counts.TotalFailures) / float64(counts.Requests) * 100
	}
	
	var nextReset time.Time
	if breaker.State() == gobreaker.StateOpen {
		nextReset = time.Now().Add(config.Timeout)
	}
	
	return &BreakerStatus{
		Name:               config.Name,
		State:              breaker.State().String(),
		Counts:             counts,
		ErrorRate:          errorRate,
		ConsecutiveFailures: counts.ConsecutiveFailures,
		NextReset:          nextReset,
		FallbackChain:      cbm.fallbacks[provider],
	}
}

func (cbm *CircuitBreakerManager) createTripCondition(config *CircuitBreakerConfig) func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		// Trip on error rate threshold
		if counts.Requests >= 10 {
			errorRate := float64(counts.TotalFailures) / float64(counts.Requests) * 100
			if errorRate >= config.ErrorRateThreshold {
				return true
			}
		}
		
		// Trip on consecutive failures
		if counts.ConsecutiveFailures >= config.ConsecutiveFailures {
			return true
		}
		
		// Trip on monthly quota exhaustion (simulated)
		if config.MonthlyRemaining <= 0 {
			return true
		}
		
		return false
	}
}

func (cbm *CircuitBreakerManager) createStateChangeHandler(provider string) func(name string, from, to gobreaker.State) {
	return func(name string, from, to gobreaker.State) {
		fmt.Printf("🔀 Circuit breaker %s changed: %s → %s\n", provider, from.String(), to.String())
		
		switch to {
		case gobreaker.StateOpen:
			fmt.Printf("❌ Provider %s circuit OPEN - switching to fallback\n", provider)
			cbm.switchToSecondary(provider)
		case gobreaker.StateHalfOpen:
			fmt.Printf("🔍 Provider %s circuit HALF-OPEN - probing recovery\n", provider)
		case gobreaker.StateClosed:
			fmt.Printf("✅ Provider %s circuit CLOSED - service restored\n", provider)
		}
	}
}

func (cbm *CircuitBreakerManager) isCircuitOpen(provider string) bool {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()
	
	breaker, exists := cbm.breakers[provider]
	if !exists {
		return false
	}
	
	return breaker.State() == gobreaker.StateOpen
}

func (cbm *CircuitBreakerManager) executeFallbackChain(provider string, fn func() (interface{}, error)) (interface{}, error) {
	cbm.mutex.RLock()
	fallbackChain := cbm.fallbacks[provider]
	cbm.mutex.RUnlock()
	
	for _, fallback := range fallbackChain {
		fmt.Printf("🔄 Trying fallback provider: %s\n", fallback)
		
		fallbackBreaker, exists := cbm.breakers[fallback]
		if !exists || fallbackBreaker.State() == gobreaker.StateOpen {
			continue
		}
		
		result, err := fallbackBreaker.Execute(fn)
		if err == nil {
			fmt.Printf("✅ Fallback %s succeeded\n", fallback)
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("all fallback providers failed for %s", provider)
}

func (cbm *CircuitBreakerManager) switchToSecondary(provider string) {
	// Implement fallback switching logic
	fmt.Printf("🔀 Switching %s to secondary providers\n", provider)
	
	// Could trigger cache TTL extension here
	cbm.extendCacheTTL(provider)
}

func (cbm *CircuitBreakerManager) extendCacheTTL(provider string) {
	// Extend cache TTL when primary provider fails
	fmt.Printf("⏱️ Extending cache TTL for %s (circuit breaker activated)\n", provider)
	// Implementation would integrate with cache layer to double TTLs
}

// Default provider configurations
func GetDefaultConfigs() map[string]*CircuitBreakerConfig {
	return map[string]*CircuitBreakerConfig{
		"binance": {
			Name:                "Binance",
			MaxRequests:         5,
			Interval:            60 * time.Second,
			Timeout:             30 * time.Second,
			ErrorRateThreshold:  30.0,
			ConsecutiveFailures: 3,
			LatencyThreshold:    5 * time.Second,
			MonthlyRemaining:    100000, // Simulated quota
		},
		"kraken": {
			Name:                "Kraken", 
			MaxRequests:         3,
			Interval:            60 * time.Second,
			Timeout:             45 * time.Second,
			ErrorRateThreshold:  25.0,
			ConsecutiveFailures: 2,
			LatencyThreshold:    3 * time.Second,
			MonthlyRemaining:    50000,
		},
		"coingecko": {
			Name:                "CoinGecko",
			MaxRequests:         2,
			Interval:            60 * time.Second,
			Timeout:             60 * time.Second,
			ErrorRateThreshold:  20.0,
			ConsecutiveFailures: 2,
			LatencyThreshold:    2 * time.Second,
			MonthlyRemaining:    10000,
		},
		"moralis": {
			Name:                "Moralis",
			MaxRequests:         2,
			Interval:            60 * time.Second,
			Timeout:             90 * time.Second,
			ErrorRateThreshold:  15.0,
			ConsecutiveFailures: 1,
			LatencyThreshold:    1 * time.Second,
			MonthlyRemaining:    5000,
		},
	}
}