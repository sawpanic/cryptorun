package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"   // Normal operation
	StateOpen     CircuitBreakerState = "open"     // Blocking requests
	StateHalfOpen CircuitBreakerState = "half_open" // Testing recovery
)

// CircuitBreakerImpl implements the CircuitBreaker interface
type CircuitBreakerImpl struct {
	breakers map[string]*circuitBreaker
	mu       sync.RWMutex
}

type circuitBreaker struct {
	name            string
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	nextRetryTime   time.Time
	
	// Configuration
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	maxRequests      int // Max requests in half-open state
	
	// Counters for half-open state
	halfOpenRequests int
	halfOpenSuccesses int
	
	mu sync.RWMutex
}

// NewCircuitBreakerImpl creates a new circuit breaker implementation
func NewCircuitBreakerImpl() *CircuitBreakerImpl {
	return &CircuitBreakerImpl{
		breakers: make(map[string]*circuitBreaker),
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreakerImpl) Call(ctx context.Context, operation string, fn func() error) error {
	cb.mu.RLock()
	breaker, exists := cb.breakers[operation]
	cb.mu.RUnlock()
	
	if !exists {
		// Create default circuit breaker for operation
		breaker = cb.createDefaultBreaker(operation)
		cb.mu.Lock()
		cb.breakers[operation] = breaker
		cb.mu.Unlock()
	}
	
	// Check if request should be allowed
	if err := breaker.allowRequest(); err != nil {
		return err
	}
	
	// Execute the function
	err := fn()
	
	// Record result
	if err != nil {
		breaker.recordFailure()
		return err
	}
	
	breaker.recordSuccess()
	return nil
}

// GetState returns the current state of a circuit breaker
func (cb *CircuitBreakerImpl) GetState(ctx context.Context, operation string) (*interfaces.CircuitState, error) {
	cb.mu.RLock()
	breaker, exists := cb.breakers[operation]
	cb.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for operation: %s", operation)
	}
	
	breaker.mu.RLock()
	defer breaker.mu.RUnlock()
	
	errorRate := 0.0
	total := breaker.failureCount + breaker.successCount
	if total > 0 {
		errorRate = float64(breaker.failureCount) / float64(total)
	}
	
	return &interfaces.CircuitState{
		State:           string(breaker.state),
		FailureCount:    breaker.failureCount,
		SuccessCount:    breaker.successCount,
		LastFailureTime: breaker.lastFailureTime,
		NextRetryTime:   breaker.nextRetryTime,
		ErrorRate:       errorRate,
	}, nil
}

// ForceOpen forces a circuit breaker to open state
func (cb *CircuitBreakerImpl) ForceOpen(ctx context.Context, operation string) error {
	cb.mu.RLock()
	breaker, exists := cb.breakers[operation]
	cb.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("circuit breaker not found for operation: %s", operation)
	}
	
	breaker.forceOpen()
	return nil
}

// ForceClose forces a circuit breaker to closed state
func (cb *CircuitBreakerImpl) ForceClose(ctx context.Context, operation string) error {
	cb.mu.RLock()
	breaker, exists := cb.breakers[operation]
	cb.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("circuit breaker not found for operation: %s", operation)
	}
	
	breaker.forceClose()
	return nil
}

// ConfigureBreaker configures a circuit breaker for an operation
func (cb *CircuitBreakerImpl) ConfigureBreaker(operation string, config CircuitBreakerConfig) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	breaker := &circuitBreaker{
		name:             operation,
		state:            StateClosed,
		failureThreshold: config.FailureThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
		maxRequests:      config.MaxRequests,
	}
	
	cb.breakers[operation] = breaker
}

// Circuit breaker methods

func (br *circuitBreaker) allowRequest() error {
	br.mu.Lock()
	defer br.mu.Unlock()
	
	switch br.state {
	case StateClosed:
		return nil // Always allow in closed state
		
	case StateOpen:
		if time.Now().After(br.nextRetryTime) {
			br.state = StateHalfOpen
			br.halfOpenRequests = 0
			br.halfOpenSuccesses = 0
			return nil
		}
		return fmt.Errorf("circuit breaker is open for %s", br.name)
		
	case StateHalfOpen:
		if br.halfOpenRequests >= br.maxRequests {
			return fmt.Errorf("circuit breaker max requests exceeded in half-open state for %s", br.name)
		}
		br.halfOpenRequests++
		return nil
		
	default:
		return fmt.Errorf("unknown circuit breaker state: %s", br.state)
	}
}

func (br *circuitBreaker) recordSuccess() {
	br.mu.Lock()
	defer br.mu.Unlock()
	
	br.successCount++
	
	switch br.state {
	case StateClosed:
		// Reset failure count on success
		br.failureCount = 0
		
	case StateHalfOpen:
		br.halfOpenSuccesses++
		if br.halfOpenSuccesses >= br.successThreshold {
			br.state = StateClosed
			br.failureCount = 0
			br.successCount = 0
		}
	}
}

func (br *circuitBreaker) recordFailure() {
	br.mu.Lock()
	defer br.mu.Unlock()
	
	br.failureCount++
	br.lastFailureTime = time.Now()
	
	switch br.state {
	case StateClosed:
		if br.failureCount >= br.failureThreshold {
			br.state = StateOpen
			br.nextRetryTime = time.Now().Add(br.timeout)
		}
		
	case StateHalfOpen:
		br.state = StateOpen
		br.nextRetryTime = time.Now().Add(br.timeout)
	}
}

func (br *circuitBreaker) forceOpen() {
	br.mu.Lock()
	defer br.mu.Unlock()
	
	br.state = StateOpen
	br.nextRetryTime = time.Now().Add(br.timeout)
}

func (br *circuitBreaker) forceClose() {
	br.mu.Lock()
	defer br.mu.Unlock()
	
	br.state = StateClosed
	br.failureCount = 0
	br.successCount = 0
}

func (cb *CircuitBreakerImpl) createDefaultBreaker(operation string) *circuitBreaker {
	// Default configuration based on operation type
	config := cb.getDefaultConfig(operation)
	
	return &circuitBreaker{
		name:             operation,
		state:            StateClosed,
		failureThreshold: config.FailureThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
		maxRequests:      config.MaxRequests,
	}
}

func (cb *CircuitBreakerImpl) getDefaultConfig(operation string) CircuitBreakerConfig {
	// Operation-specific defaults
	configs := map[string]CircuitBreakerConfig{
		"fetch_trades": {
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
			MaxRequests:      3,
		},
		"fetch_orderbook": {
			FailureThreshold: 8, // Order books are critical
			SuccessThreshold: 4,
			Timeout:          20 * time.Second,
			MaxRequests:      2,
		},
		"fetch_klines": {
			FailureThreshold: 4,
			SuccessThreshold: 2,
			Timeout:          45 * time.Second,
			MaxRequests:      3,
		},
		"fetch_funding": {
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          60 * time.Second,
			MaxRequests:      2,
		},
		"health_check": {
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
			MaxRequests:      2,
		},
	}
	
	if config, exists := configs[operation]; exists {
		return config
	}
	
	// Default configuration
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          60 * time.Second,
		MaxRequests:      3,
	}
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening
	SuccessThreshold int           // Number of successes before closing from half-open
	Timeout          time.Duration // How long to stay open
	MaxRequests      int           // Max requests in half-open state
}

// VenueCircuitBreaker manages circuit breakers for a specific venue
type VenueCircuitBreaker struct {
	venue        string
	httpBreaker  *circuitBreaker
	wsBreaker    *circuitBreaker
	operations   map[string]*circuitBreaker
	
	// Fallback strategy
	fallbackEnabled bool
	fallbackVenues  []string
	
	mu sync.RWMutex
}

// NewVenueCircuitBreaker creates circuit breakers for a venue
func NewVenueCircuitBreaker(venue string, config VenueConfig) *VenueCircuitBreaker {
	vcb := &VenueCircuitBreaker{
		venue:           venue,
		operations:      make(map[string]*circuitBreaker),
		fallbackEnabled: config.FallbackEnabled,
		fallbackVenues:  config.FallbackVenues,
	}
	
	// Create HTTP circuit breaker
	vcb.httpBreaker = &circuitBreaker{
		name:             venue + "_http",
		state:            StateClosed,
		failureThreshold: config.HTTP.FailureThreshold,
		successThreshold: config.HTTP.SuccessThreshold,
		timeout:          config.HTTP.Timeout,
		maxRequests:      config.HTTP.MaxRequests,
	}
	
	// Create WebSocket circuit breaker
	vcb.wsBreaker = &circuitBreaker{
		name:             venue + "_websocket",
		state:            StateClosed,
		failureThreshold: config.WebSocket.FailureThreshold,
		successThreshold: config.WebSocket.SuccessThreshold,
		timeout:          config.WebSocket.Timeout,
		maxRequests:      config.WebSocket.MaxRequests,
	}
	
	return vcb
}

// IsHTTPHealthy checks if HTTP requests are allowed
func (vcb *VenueCircuitBreaker) IsHTTPHealthy() bool {
	vcb.mu.RLock()
	defer vcb.mu.RUnlock()
	
	return vcb.httpBreaker.state == StateClosed || 
		   (vcb.httpBreaker.state == StateHalfOpen && vcb.httpBreaker.halfOpenRequests < vcb.httpBreaker.maxRequests)
}

// IsWSHealthy checks if WebSocket connections are allowed
func (vcb *VenueCircuitBreaker) IsWSHealthy() bool {
	vcb.mu.RLock()
	defer vcb.mu.RUnlock()
	
	return vcb.wsBreaker.state == StateClosed ||
		   (vcb.wsBreaker.state == StateHalfOpen && vcb.wsBreaker.halfOpenRequests < vcb.wsBreaker.maxRequests)
}

// RecordHTTPSuccess records a successful HTTP request
func (vcb *VenueCircuitBreaker) RecordHTTPSuccess() {
	vcb.httpBreaker.recordSuccess()
}

// RecordHTTPFailure records a failed HTTP request
func (vcb *VenueCircuitBreaker) RecordHTTPFailure() {
	vcb.httpBreaker.recordFailure()
}

// RecordWSSuccess records a successful WebSocket operation
func (vcb *VenueCircuitBreaker) RecordWSSuccess() {
	vcb.wsBreaker.recordSuccess()
}

// RecordWSFailure records a failed WebSocket operation
func (vcb *VenueCircuitBreaker) RecordWSFailure() {
	vcb.wsBreaker.recordFailure()
}

// GetStatus returns the overall health status
func (vcb *VenueCircuitBreaker) GetStatus() VenueCircuitStatus {
	vcb.mu.RLock()
	defer vcb.mu.RUnlock()
	
	return VenueCircuitStatus{
		Venue:        vcb.venue,
		HTTPState:    string(vcb.httpBreaker.state),
		WSState:      string(vcb.wsBreaker.state),
		IsHealthy:    vcb.IsHTTPHealthy() && vcb.IsWSHealthy(),
		LastFailure:  maxTime(vcb.httpBreaker.lastFailureTime, vcb.wsBreaker.lastFailureTime),
		NextRetry:    maxTime(vcb.httpBreaker.nextRetryTime, vcb.wsBreaker.nextRetryTime),
	}
}

// VenueConfig holds venue-specific circuit breaker configuration
type VenueConfig struct {
	HTTP struct {
		FailureThreshold int
		SuccessThreshold int
		Timeout          time.Duration
		MaxRequests      int
	}
	WebSocket struct {
		FailureThreshold int
		SuccessThreshold int
		Timeout          time.Duration
		MaxRequests      int
	}
	FallbackEnabled bool
	FallbackVenues  []string
}

// VenueCircuitStatus represents the status of venue circuit breakers
type VenueCircuitStatus struct {
	Venue       string    `json:"venue"`
	HTTPState   string    `json:"http_state"`
	WSState     string    `json:"ws_state"`
	IsHealthy   bool      `json:"is_healthy"`
	LastFailure time.Time `json:"last_failure"`
	NextRetry   time.Time `json:"next_retry"`
}

// Helper functions

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// HealthProbe performs health checks for circuit breaker recovery
type HealthProbe struct {
	venue      string
	endpoint   string
	timeout    time.Duration
	httpClient interface{} // Would be *http.Client in real implementation
}

func NewHealthProbe(venue, endpoint string, timeout time.Duration) *HealthProbe {
	return &HealthProbe{
		venue:    venue,
		endpoint: endpoint,
		timeout:  timeout,
	}
}

func (hp *HealthProbe) Check(ctx context.Context) error {
	// Implement actual health check logic
	// This would make a simple HTTP request to the health endpoint
	return nil
}