package guards

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

func (cs CircuitState) String() string {
	switch cs {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements a circuit breaker pattern for provider resilience
type CircuitBreaker struct {
	state           CircuitState
	failureCount    int
	successCount    int
	requestCount    int
	lastFailureTime time.Time
	lastSuccessTime time.Time
	mutex           sync.RWMutex
	config          CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold float64       // Failure rate threshold (0.0-1.0)
	MinRequests      int           // Minimum requests before considering failure rate
	OpenTimeout      time.Duration // Time to stay open before half-open
	ProbeInterval    time.Duration // Interval for half-open probes
	MaxFailures      int           // Absolute failure count to trigger open
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config ProviderConfig) *CircuitBreaker {
	failureThresh := config.FailureThresh
	if failureThresh <= 0 || failureThresh > 1.0 {
		failureThresh = 0.5 // Default 50% failure rate
	}

	windowRequests := config.WindowRequests
	if windowRequests <= 0 {
		windowRequests = 10 // Default minimum 10 requests
	}

	probeInterval := time.Duration(config.ProbeInterval) * time.Second
	if probeInterval <= 0 {
		probeInterval = 30 * time.Second // Default 30 seconds
	}

	cbConfig := CircuitBreakerConfig{
		FailureThreshold: failureThresh,
		MinRequests:      windowRequests,
		OpenTimeout:      probeInterval * 2, // Open timeout is 2x probe interval
		ProbeInterval:    probeInterval,
		MaxFailures:      windowRequests, // Absolute limit based on window
	}

	return &CircuitBreaker{
		state:  Closed,
		config: cbConfig,
	}
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case Open:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.config.OpenTimeout {
			return false // Allow probe request
		}
		return true
	case HalfOpen:
		// In half-open state, allow requests through
		return false
	default:
		return false
	}
}

// CanRequest returns true if a request can proceed
func (cb *CircuitBreaker) CanRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case Closed:
		return true
	case Open:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.config.OpenTimeout {
			cb.state = HalfOpen
			cb.successCount = 0
			cb.failureCount = 0
			return true
		}
		return false
	case HalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successCount++
	cb.requestCount++
	cb.lastSuccessTime = time.Now()

	switch cb.state {
	case HalfOpen:
		// After first success in half-open, return to closed
		cb.state = Closed
		cb.resetCounts()
	case Closed:
		// Reset failure count on success to prevent accumulation
		if cb.successCount > 0 && cb.successCount%5 == 0 {
			cb.failureCount = max(0, cb.failureCount-1)
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.requestCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case Closed:
		if cb.shouldOpen() {
			cb.state = Open
		}
	case HalfOpen:
		// Failure in half-open immediately returns to open
		cb.state = Open
	}
}

// shouldOpen determines if the circuit should open based on failure rate
func (cb *CircuitBreaker) shouldOpen() bool {
	if cb.requestCount < cb.config.MinRequests {
		return false
	}

	// Check absolute failure count
	if cb.failureCount >= cb.config.MaxFailures {
		return true
	}

	// Check failure rate
	failureRate := float64(cb.failureCount) / float64(cb.requestCount)
	return failureRate >= cb.config.FailureThreshold
}

// resetCounts resets success and failure counts
func (cb *CircuitBreaker) resetCounts() {
	cb.failureCount = 0
	cb.successCount = 0
	cb.requestCount = 0
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// Update state based on timeout if needed
	if cb.state == Open && time.Since(cb.lastFailureTime) >= cb.config.OpenTimeout {
		return HalfOpen
	}

	return cb.state
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	var failureRate float64
	if cb.requestCount > 0 {
		failureRate = float64(cb.failureCount) / float64(cb.requestCount)
	}

	var timeToNextProbe time.Duration
	if cb.state == Open {
		timeToNextProbe = cb.config.OpenTimeout - time.Since(cb.lastFailureTime)
		if timeToNextProbe < 0 {
			timeToNextProbe = 0
		}
	}

	return CircuitBreakerStats{
		State:            cb.state.String(),
		FailureCount:     cb.failureCount,
		SuccessCount:     cb.successCount,
		RequestCount:     cb.requestCount,
		FailureRate:      failureRate,
		LastFailureTime:  cb.lastFailureTime,
		LastSuccessTime:  cb.lastSuccessTime,
		TimeToNextProbe:  timeToNextProbe,
		FailureThreshold: cb.config.FailureThreshold,
		MinRequests:      cb.config.MinRequests,
	}
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	State            string        `json:"state"`
	FailureCount     int           `json:"failure_count"`
	SuccessCount     int           `json:"success_count"`
	RequestCount     int           `json:"request_count"`
	FailureRate      float64       `json:"failure_rate"`
	LastFailureTime  time.Time     `json:"last_failure_time"`
	LastSuccessTime  time.Time     `json:"last_success_time"`
	TimeToNextProbe  time.Duration `json:"time_to_next_probe"`
	FailureThreshold float64       `json:"failure_threshold"`
	MinRequests      int           `json:"min_requests"`
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = Closed
	cb.resetCounts()
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = Open
	cb.lastFailureTime = time.Now()
}

// MultiProviderCircuitBreaker manages circuit breakers for multiple providers
type MultiProviderCircuitBreaker struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewMultiProviderCircuitBreaker creates a circuit breaker manager
func NewMultiProviderCircuitBreaker() *MultiProviderCircuitBreaker {
	return &MultiProviderCircuitBreaker{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// AddProvider adds a circuit breaker for a specific provider
func (mcb *MultiProviderCircuitBreaker) AddProvider(name string, config ProviderConfig) {
	mcb.mutex.Lock()
	defer mcb.mutex.Unlock()

	mcb.breakers[name] = NewCircuitBreaker(config)
}

// CanRequest checks if a request can proceed for the given provider
func (mcb *MultiProviderCircuitBreaker) CanRequest(provider string) bool {
	mcb.mutex.RLock()
	breaker, exists := mcb.breakers[provider]
	mcb.mutex.RUnlock()

	if !exists {
		return true // No circuit breaker configured
	}

	return breaker.CanRequest()
}

// RecordSuccess records success for a provider
func (mcb *MultiProviderCircuitBreaker) RecordSuccess(provider string) {
	mcb.mutex.RLock()
	breaker, exists := mcb.breakers[provider]
	mcb.mutex.RUnlock()

	if exists {
		breaker.RecordSuccess()
	}
}

// RecordFailure records failure for a provider
func (mcb *MultiProviderCircuitBreaker) RecordFailure(provider string) {
	mcb.mutex.RLock()
	breaker, exists := mcb.breakers[provider]
	mcb.mutex.RUnlock()

	if exists {
		breaker.RecordFailure()
	}
}

// GetStats returns statistics for all providers
func (mcb *MultiProviderCircuitBreaker) GetStats() map[string]CircuitBreakerStats {
	mcb.mutex.RLock()
	defer mcb.mutex.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, breaker := range mcb.breakers {
		stats[name] = breaker.Stats()
	}

	return stats
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
