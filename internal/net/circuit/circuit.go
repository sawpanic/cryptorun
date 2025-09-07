package circuit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrRequestTimeout is returned when a request times out
	ErrRequestTimeout = errors.New("request timeout")
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // Circuit is closed, requests allowed
	StateOpen                  // Circuit is open, requests blocked
	StateHalfOpen              // Circuit is half-open, limited requests allowed
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config represents circuit breaker configuration
type Config struct {
	FailureThreshold int           // Consecutive failures to open circuit
	SuccessThreshold int           // Consecutive successes to close circuit from half-open
	Timeout          time.Duration // Time to wait before transitioning to half-open
	RequestTimeout   time.Duration // Individual request timeout
}

// Breaker represents a circuit breaker
type Breaker struct {
	mu              sync.RWMutex
	config          Config
	state           State
	failures        int       // Consecutive failure count
	successes       int       // Consecutive success count in half-open state
	lastFailureTime time.Time // Last failure timestamp
	lastStateChange time.Time // Last state change timestamp
	totalRequests   int64     // Total request count
	totalSuccesses  int64     // Total success count
	totalFailures   int64     // Total failure count
	totalTimeouts   int64     // Total timeout count
}

// NewBreaker creates a new circuit breaker with the specified configuration
func NewBreaker(config Config) *Breaker {
	return &Breaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Call executes the given function if the circuit breaker allows it
func (b *Breaker) Call(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if request is allowed
	if !b.allowRequest() {
		return ErrCircuitOpen
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, b.config.RequestTimeout)
	defer cancel()

	// Track request
	b.mu.Lock()
	b.totalRequests++
	b.mu.Unlock()

	// Execute function
	done := make(chan error, 1)
	go func() {
		done <- fn(timeoutCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			b.onFailure()
			return err
		}
		b.onSuccess()
		return nil
	case <-timeoutCtx.Done():
		b.onTimeout()
		return ErrRequestTimeout
	}
}

// allowRequest determines if a request should be allowed based on circuit state
func (b *Breaker) allowRequest() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if enough time has passed to attempt recovery
		if time.Since(b.lastFailureTime) > b.config.Timeout {
			b.setState(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// onSuccess handles successful request completion
func (b *Breaker) onSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalSuccesses++

	switch b.state {
	case StateClosed:
		b.failures = 0 // Reset failure count
	case StateHalfOpen:
		b.successes++
		if b.successes >= b.config.SuccessThreshold {
			b.setState(StateClosed)
			b.failures = 0
			b.successes = 0
		}
	}
}

// onFailure handles failed request completion
func (b *Breaker) onFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalFailures++
	b.lastFailureTime = time.Now()

	switch b.state {
	case StateClosed:
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.setState(StateOpen)
		}
	case StateHalfOpen:
		b.setState(StateOpen)
		b.successes = 0
	}
}

// onTimeout handles request timeout
func (b *Breaker) onTimeout() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalTimeouts++
	b.totalFailures++
	b.lastFailureTime = time.Now()

	switch b.state {
	case StateClosed:
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.setState(StateOpen)
		}
	case StateHalfOpen:
		b.setState(StateOpen)
		b.successes = 0
	}
}

// setState changes the circuit breaker state and updates timestamp
func (b *Breaker) setState(state State) {
	if b.state != state {
		b.state = state
		b.lastStateChange = time.Now()

		// Reset failure count when transitioning to half-open
		if state == StateHalfOpen {
			b.failures = 0
		}
	}
}

// State returns the current circuit breaker state
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Stats returns current circuit breaker statistics
func (b *Breaker) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	successRate := float64(0)
	if b.totalRequests > 0 {
		successRate = float64(b.totalSuccesses) / float64(b.totalRequests)
	}

	timeoutRate := float64(0)
	if b.totalRequests > 0 {
		timeoutRate = float64(b.totalTimeouts) / float64(b.totalRequests)
	}

	return Stats{
		State:                b.state,
		TotalRequests:        b.totalRequests,
		TotalSuccesses:       b.totalSuccesses,
		TotalFailures:        b.totalFailures,
		TotalTimeouts:        b.totalTimeouts,
		ConsecutiveFailures:  b.failures,
		ConsecutiveSuccesses: b.successes,
		LastStateChange:      b.lastStateChange,
		LastFailureTime:      b.lastFailureTime,
		SuccessRate:          successRate,
		TimeoutRate:          timeoutRate,
	}
}

// Reset resets the circuit breaker to its initial state
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = StateClosed
	b.failures = 0
	b.successes = 0
	b.totalRequests = 0
	b.totalSuccesses = 0
	b.totalFailures = 0
	b.totalTimeouts = 0
	b.lastStateChange = time.Now()
	b.lastFailureTime = time.Time{}
}

// ForceOpen forces the circuit breaker to open state
func (b *Breaker) ForceOpen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.setState(StateOpen)
}

// ForceHalfOpen forces the circuit breaker to half-open state
func (b *Breaker) ForceHalfOpen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.setState(StateHalfOpen)
}

// ForceClosed forces the circuit breaker to closed state
func (b *Breaker) ForceClosed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.setState(StateClosed)
	b.failures = 0
	b.successes = 0
}

// Stats represents circuit breaker statistics
type Stats struct {
	State                State     `json:"state"`
	TotalRequests        int64     `json:"total_requests"`
	TotalSuccesses       int64     `json:"total_successes"`
	TotalFailures        int64     `json:"total_failures"`
	TotalTimeouts        int64     `json:"total_timeouts"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	LastStateChange      time.Time `json:"last_state_change"`
	LastFailureTime      time.Time `json:"last_failure_time,omitempty"`
	SuccessRate          float64   `json:"success_rate"`
	TimeoutRate          float64   `json:"timeout_rate"`
}

// IsHealthy returns true if the circuit breaker indicates healthy service
func (s *Stats) IsHealthy() bool {
	return s.State == StateClosed && (s.TotalRequests == 0 || s.SuccessRate >= 0.9)
}

// Manager manages multiple circuit breakers for different providers
type Manager struct {
	breakers map[string]*Breaker
	mu       sync.RWMutex
}

// NewManager creates a new circuit breaker manager
func NewManager() *Manager {
	return &Manager{
		breakers: make(map[string]*Breaker),
	}
}

// AddProvider adds a circuit breaker for a specific provider
func (m *Manager) AddProvider(name string, config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.breakers[name] = NewBreaker(config)
}

// GetBreaker returns the circuit breaker for a specific provider
func (m *Manager) GetBreaker(provider string) (*Breaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	breaker, exists := m.breakers[provider]
	return breaker, exists
}

// Call executes a function through the circuit breaker for a specific provider
func (m *Manager) Call(ctx context.Context, provider string, fn func(ctx context.Context) error) error {
	breaker, exists := m.GetBreaker(provider)
	if !exists {
		// No circuit breaker configured, execute directly
		return fn(ctx)
	}
	return breaker.Call(ctx, fn)
}

// Stats returns statistics for all providers
func (m *Manager) Stats() map[string]Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]Stats)
	for provider, breaker := range m.breakers {
		stats[provider] = breaker.Stats()
	}
	return stats
}

// IsHealthy returns true if all circuit breakers are healthy
func (m *Manager) IsHealthy() bool {
	stats := m.Stats()
	for _, stat := range stats {
		if !stat.IsHealthy() {
			return false
		}
	}
	return true
}

// Reset resets all circuit breakers
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}

// GetUnhealthyProviders returns a list of providers with unhealthy circuit breakers
func (m *Manager) GetUnhealthyProviders() []string {
	stats := m.Stats()
	var unhealthy []string

	for provider, stat := range stats {
		if !stat.IsHealthy() {
			unhealthy = append(unhealthy, fmt.Sprintf("%s (state: %s, success: %.1f%%)",
				provider, stat.State, stat.SuccessRate*100))
		}
	}

	return unhealthy
}
