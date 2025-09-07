package provider

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (cs CircuitState) String() string {
	switch cs {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for provider resilience
type CircuitBreaker struct {
	name            string
	config          CircuitConfig
	state           CircuitState
	failureCount    int64
	successCount    int64
	requestCount    int64
	lastFailureTime time.Time
	lastSuccessTime time.Time
	nextProbeTime   time.Time
	mu              sync.RWMutex
	
	// Metrics callback
	metricsCallback func(string, interface{})
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 0.5 // Default 50% failure rate
	}
	if config.MinRequests <= 0 {
		config.MinRequests = 10
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = 30 * time.Second
	}
	if config.ProbeInterval <= 0 {
		config.ProbeInterval = 5 * time.Second
	}
	
	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  CircuitClosed,
	}
}

// SetMetricsCallback sets a callback for metrics collection
func (cb *CircuitBreaker) SetMetricsCallback(callback func(string, interface{})) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.metricsCallback = callback
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.config.Enabled {
		return fn() // Circuit breaker disabled, execute directly
	}
	
	// Check if we can make the call
	if !cb.canCall() {
		cb.emitMetric("circuit_breaker_rejected", 1)
		return &ProviderError{
			Provider:  cb.name,
			Code:      ErrCodeCircuitOpen,
			Message:   fmt.Sprintf("circuit breaker is %s", cb.state),
			Temporary: true,
		}
	}
	
	// Execute the function
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	// Record the result
	cb.recordResult(err, duration)
	
	return err
}

// canCall determines if a call can be made based on circuit breaker state
func (cb *CircuitBreaker) canCall() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	now := time.Now()
	
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if we should transition to half-open
		if now.After(cb.nextProbeTime) {
			cb.state = CircuitHalfOpen
			cb.emitMetric("circuit_breaker_half_open", 1)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult updates circuit breaker state based on call result
func (cb *CircuitBreaker) recordResult(err error, duration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	now := time.Now()
	cb.requestCount++
	
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = now
		
		cb.emitMetric("circuit_breaker_failure", 1)
		cb.emitMetric("circuit_breaker_request_duration_ms", duration.Milliseconds())
		
		// Check if we should open the circuit
		if cb.shouldOpen() {
			cb.openCircuit()
		}
	} else {
		cb.successCount++
		cb.lastSuccessTime = now
		
		cb.emitMetric("circuit_breaker_success", 1)
		cb.emitMetric("circuit_breaker_request_duration_ms", duration.Milliseconds())
		
		// If we're in half-open state and got a success, close the circuit
		if cb.state == CircuitHalfOpen {
			cb.closeCircuit()
		}
	}
}

// shouldOpen determines if the circuit should open based on failure rate
func (cb *CircuitBreaker) shouldOpen() bool {
	// Need minimum requests to calculate meaningful failure rate
	if cb.requestCount < int64(cb.config.MinRequests) {
		return false
	}
	
	// Check absolute failure count
	if cb.config.MaxFailures > 0 && cb.failureCount >= int64(cb.config.MaxFailures) {
		return true
	}
	
	// Check failure rate
	failureRate := float64(cb.failureCount) / float64(cb.requestCount)
	return failureRate >= cb.config.FailureThreshold
}

// openCircuit transitions the circuit to open state
func (cb *CircuitBreaker) openCircuit() {
	if cb.state != CircuitOpen {
		cb.state = CircuitOpen
		cb.nextProbeTime = time.Now().Add(cb.config.OpenTimeout)
		cb.emitMetric("circuit_breaker_opened", 1)
	}
}

// closeCircuit transitions the circuit to closed state
func (cb *CircuitBreaker) closeCircuit() {
	if cb.state != CircuitClosed {
		cb.state = CircuitClosed
		cb.resetCounts()
		cb.emitMetric("circuit_breaker_closed", 1)
	}
}

// resetCounts resets failure and success counters
func (cb *CircuitBreaker) resetCounts() {
	cb.failureCount = 0
	cb.successCount = 0
	cb.requestCount = 0
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	var failureRate float64
	if cb.requestCount > 0 {
		failureRate = float64(cb.failureCount) / float64(cb.requestCount)
	}
	
	return CircuitBreakerStats{
		Name:            cb.name,
		State:           cb.state.String(),
		RequestCount:    cb.requestCount,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		FailureRate:     failureRate,
		LastFailureTime: cb.lastFailureTime,
		LastSuccessTime: cb.lastSuccessTime,
		NextProbeTime:   cb.nextProbeTime,
	}
}

// CircuitBreakerStats provides circuit breaker statistics
type CircuitBreakerStats struct {
	Name            string    `json:"name"`
	State           string    `json:"state"`
	RequestCount    int64     `json:"request_count"`
	FailureCount    int64     `json:"failure_count"`
	SuccessCount    int64     `json:"success_count"`
	FailureRate     float64   `json:"failure_rate"`
	LastFailureTime time.Time `json:"last_failure_time"`
	LastSuccessTime time.Time `json:"last_success_time"`
	NextProbeTime   time.Time `json:"next_probe_time"`
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.state = CircuitClosed
	cb.resetCounts()
	cb.emitMetric("circuit_breaker_reset", 1)
}

// ForceOpen manually opens the circuit breaker
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.openCircuit()
	cb.emitMetric("circuit_breaker_forced_open", 1)
}

// emitMetric sends metrics if callback is set
func (cb *CircuitBreaker) emitMetric(metric string, value interface{}) {
	if cb.metricsCallback != nil {
		cb.metricsCallback(metric, value)
	}
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (cbm *CircuitBreakerManager) GetOrCreate(name string, config CircuitConfig) *CircuitBreaker {
	cbm.mu.RLock()
	if breaker, exists := cbm.breakers[name]; exists {
		cbm.mu.RUnlock()
		return breaker
	}
	cbm.mu.RUnlock()
	
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	
	// Double-check after acquiring write lock
	if breaker, exists := cbm.breakers[name]; exists {
		return breaker
	}
	
	breaker := NewCircuitBreaker(name, config)
	cbm.breakers[name] = breaker
	return breaker
}

// GetAll returns all circuit breakers
func (cbm *CircuitBreakerManager) GetAll() map[string]*CircuitBreaker {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()
	
	result := make(map[string]*CircuitBreaker)
	for name, breaker := range cbm.breakers {
		result[name] = breaker
	}
	return result
}

// GetStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetStats() map[string]CircuitBreakerStats {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()
	
	result := make(map[string]CircuitBreakerStats)
	for name, breaker := range cbm.breakers {
		result[name] = breaker.GetStats()
	}
	return result
}

// ResetAll resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()
	
	for _, breaker := range cbm.breakers {
		breaker.Reset()
	}
}

// DefaultCircuitConfig returns sensible default configuration
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		Enabled:          true,
		FailureThreshold: 0.5,  // 50% failure rate
		MinRequests:      10,   // Minimum requests before considering failure rate
		OpenTimeout:      30 * time.Second,
		ProbeInterval:    5 * time.Second,
		MaxFailures:      20,   // Absolute failure count to trigger open
	}
}