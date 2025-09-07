package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CircuitState represents the current state of a circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half_open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig defines circuit breaker parameters
type CircuitBreakerConfig struct {
	Provider           string        `yaml:"provider"`
	FailureThreshold   int           `yaml:"failure_threshold"`   // Failures to trip circuit
	SuccessThreshold   int           `yaml:"success_threshold"`   // Successes to close circuit
	Timeout            time.Duration `yaml:"timeout"`             // How long to stay open
	MaxConcurrent      int           `yaml:"max_concurrent"`      // Max concurrent requests
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

// Default circuit breaker configurations per provider
var CircuitBreakerConfigs = map[string]CircuitBreakerConfig{
	"binance": {
		Provider:           "binance",
		FailureThreshold:   5,
		SuccessThreshold:   3,
		Timeout:            time.Minute * 2,
		MaxConcurrent:      10,
		HealthCheckInterval: time.Second * 30,
	},
	"dexscreener": {
		Provider:           "dexscreener",
		FailureThreshold:   3,
		SuccessThreshold:   2,
		Timeout:            time.Minute * 5,
		MaxConcurrent:      5,
		HealthCheckInterval: time.Minute,
	},
	"coingecko": {
		Provider:           "coingecko",
		FailureThreshold:   4,
		SuccessThreshold:   2,
		Timeout:            time.Minute * 3,
		MaxConcurrent:      3,
		HealthCheckInterval: time.Minute,
	},
	"moralis": {
		Provider:           "moralis",
		FailureThreshold:   3,
		SuccessThreshold:   2,
		Timeout:            time.Minute * 10,
		MaxConcurrent:      2,
		HealthCheckInterval: time.Minute * 2,
	},
	"cmc": {
		Provider:           "cmc",
		FailureThreshold:   2,
		SuccessThreshold:   1,
		Timeout:            time.Minute * 15,
		MaxConcurrent:      2,
		HealthCheckInterval: time.Minute * 3,
	},
	"etherscan": {
		Provider:           "etherscan",
		FailureThreshold:   2,
		SuccessThreshold:   1,
		Timeout:            time.Minute * 30,
		MaxConcurrent:      1,
		HealthCheckInterval: time.Minute * 5,
	},
	"paprika": {
		Provider:           "paprika",
		FailureThreshold:   4,
		SuccessThreshold:   2,
		Timeout:            time.Minute * 4,
		MaxConcurrent:      5,
		HealthCheckInterval: time.Minute,
	},
}

// CircuitBreaker implements provider-aware circuit breaking
type CircuitBreaker struct {
	mu               sync.RWMutex
	config           CircuitBreakerConfig
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	nextRetryTime    time.Time
	concurrentCalls  int
	totalCalls       int64
	totalFailures    int64
	lastHealthCheck  time.Time
	healthStatus     string
}

// NewCircuitBreaker creates a provider-specific circuit breaker
func NewCircuitBreaker(provider string) *CircuitBreaker {
	config, exists := CircuitBreakerConfigs[provider]
	if !exists {
		log.Warn().Str("provider", provider).Msg("Unknown provider, using default circuit breaker")
		config = CircuitBreakerConfig{
			Provider:           provider,
			FailureThreshold:   3,
			SuccessThreshold:   2,
			Timeout:            time.Minute * 5,
			MaxConcurrent:      5,
			HealthCheckInterval: time.Minute,
		}
	}

	return &CircuitBreaker{
		config:       config,
		state:        StateClosed,
		healthStatus: "healthy",
	}
}

// Call executes a function through the circuit breaker
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	cb.mu.Lock()
	
	// Check if circuit is open
	if cb.state == StateOpen {
		if time.Now().Before(cb.nextRetryTime) {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker open for %s, next retry: %s", 
				cb.config.Provider, cb.nextRetryTime.Format(time.RFC3339))
		}
		// Transition to half-open
		cb.state = StateHalfOpen
		cb.successCount = 0
		log.Info().Str("provider", cb.config.Provider).Msg("Circuit breaker transitioning to half-open")
	}

	// Check concurrency limit
	if cb.concurrentCalls >= cb.config.MaxConcurrent {
		cb.mu.Unlock()
		return fmt.Errorf("max concurrent calls exceeded for %s: %d/%d", 
			cb.config.Provider, cb.concurrentCalls, cb.config.MaxConcurrent)
	}

	cb.concurrentCalls++
	cb.totalCalls++
	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.mu.Lock()
	cb.concurrentCalls--

	if err != nil {
		cb.handleFailure()
	} else {
		cb.handleSuccess()
	}
	cb.mu.Unlock()

	return err
}

// ForceOpen manually opens the circuit (for testing/maintenance)
func (cb *CircuitBreaker) ForceOpen(reason string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateOpen
	cb.nextRetryTime = time.Now().Add(cb.config.Timeout)
	cb.healthStatus = fmt.Sprintf("forced_open: %s", reason)
	
	log.Warn().
		Str("provider", cb.config.Provider).
		Str("reason", reason).
		Time("retry_time", cb.nextRetryTime).
		Msg("Circuit breaker manually opened")
}

// ForceClose manually closes the circuit
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.healthStatus = "manually_closed"
	
	log.Info().Str("provider", cb.config.Provider).Msg("Circuit breaker manually closed")
}

// GetStatus returns current circuit breaker status
func (cb *CircuitBreaker) GetStatus() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"provider":         cb.config.Provider,
		"state":           cb.state.String(),
		"failure_count":   cb.failureCount,
		"success_count":   cb.successCount,
		"concurrent_calls": cb.concurrentCalls,
		"total_calls":     cb.totalCalls,
		"total_failures":  cb.totalFailures,
		"failure_rate":    cb.calculateFailureRate(),
		"next_retry":      cb.nextRetryTime,
		"health_status":   cb.healthStatus,
		"last_health_check": cb.lastHealthCheck,
		"config": map[string]interface{}{
			"failure_threshold": cb.config.FailureThreshold,
			"success_threshold": cb.config.SuccessThreshold,
			"timeout":          cb.config.Timeout.String(),
			"max_concurrent":   cb.config.MaxConcurrent,
		},
	}
}

// IsHealthy returns true if the circuit is allowing calls
func (cb *CircuitBreaker) IsHealthy() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state != StateOpen
}

// RunHealthCheck performs a health check if interval has elapsed
func (cb *CircuitBreaker) RunHealthCheck(ctx context.Context, healthFn func() error) {
	cb.mu.Lock()
	now := time.Now()
	if now.Sub(cb.lastHealthCheck) < cb.config.HealthCheckInterval {
		cb.mu.Unlock()
		return
	}
	cb.lastHealthCheck = now
	cb.mu.Unlock()

	// Perform health check
	err := healthFn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.healthStatus = fmt.Sprintf("unhealthy: %s", err.Error())
		log.Warn().Str("provider", cb.config.Provider).Err(err).Msg("Health check failed")
		
		// Consider opening circuit if health checks consistently fail
		if cb.state == StateClosed {
			cb.handleFailure()
		}
	} else {
		cb.healthStatus = "healthy"
		log.Debug().Str("provider", cb.config.Provider).Msg("Health check passed")
	}
}

// Private methods
func (cb *CircuitBreaker) handleFailure() {
	cb.failureCount++
	cb.totalFailures++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen {
		// Failed during half-open, go back to open
		cb.state = StateOpen
		cb.nextRetryTime = time.Now().Add(cb.config.Timeout)
		cb.failureCount = 0 // Reset for next attempt
		
		log.Warn().
			Str("provider", cb.config.Provider).
			Msg("Circuit breaker failed in half-open state, returning to open")
	} else if cb.failureCount >= cb.config.FailureThreshold {
		// Trip the circuit
		cb.state = StateOpen
		cb.nextRetryTime = time.Now().Add(cb.config.Timeout)
		
		log.Error().
			Str("provider", cb.config.Provider).
			Int("failures", cb.failureCount).
			Time("retry_time", cb.nextRetryTime).
			Msg("Circuit breaker tripped to open state")
	}
}

func (cb *CircuitBreaker) handleSuccess() {
	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			// Close the circuit
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			
			log.Info().
				Str("provider", cb.config.Provider).
				Msg("Circuit breaker closed after successful recovery")
		}
	} else if cb.state == StateClosed {
		// Reset failure count on successful call
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) calculateFailureRate() float64 {
	if cb.totalCalls == 0 {
		return 0.0
	}
	return float64(cb.totalFailures) / float64(cb.totalCalls)
}