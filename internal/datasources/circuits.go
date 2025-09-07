package datasources

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
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

// CircuitConfig defines configuration for a circuit breaker
type CircuitConfig struct {
	Provider            string        `json:"provider"`
	ErrorThreshold      int           `json:"error_threshold"`        // Number of errors before opening
	SuccessThreshold    int           `json:"success_threshold"`      // Successes needed to close from half-open
	Timeout             time.Duration `json:"timeout"`                // How long to stay open before half-open
	LatencyThreshold    time.Duration `json:"latency_threshold"`      // Max acceptable latency
	BudgetThreshold     float64       `json:"budget_threshold"`       // Remaining budget % before opening
	WindowSize          int           `json:"window_size"`            // Size of sliding window for error rate
	MinRequestsInWindow int           `json:"min_requests_in_window"` // Min requests before calculating error rate
}

// CircuitBreaker implements a circuit breaker pattern for data providers
type CircuitBreaker struct {
	config          CircuitConfig
	state           CircuitState
	errorCount      int
	successCount    int
	requestCount    int
	lastFailTime    time.Time
	lastSuccessTime time.Time
	mu              sync.RWMutex

	// Sliding window for request tracking
	requests    []RequestResult
	windowStart int

	// Fallback providers in order of preference
	fallbackProviders []string
	currentProvider   string
}

// RequestResult tracks the result of a request
type RequestResult struct {
	Timestamp time.Time
	Success   bool
	Latency   time.Duration
	Error     error
}

// CircuitManager manages circuit breakers for all providers
type CircuitManager struct {
	circuits       map[string]*CircuitBreaker
	fallbackChains map[string][]string
	mu             sync.RWMutex
}

// Default circuit configurations
var DefaultCircuitConfigs = map[string]CircuitConfig{
	"binance": {
		Provider:            "binance",
		ErrorThreshold:      5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		LatencyThreshold:    5 * time.Second,
		BudgetThreshold:     0.1, // Open when <10% budget remaining
		WindowSize:          20,
		MinRequestsInWindow: 5,
	},
	"coingecko": {
		Provider:            "coingecko",
		ErrorThreshold:      3,
		SuccessThreshold:    2,
		Timeout:             60 * time.Second,
		LatencyThreshold:    10 * time.Second,
		BudgetThreshold:     0.05, // Open when <5% monthly budget remaining
		WindowSize:          15,
		MinRequestsInWindow: 3,
	},
	"moralis": {
		Provider:            "moralis",
		ErrorThreshold:      3,
		SuccessThreshold:    2,
		Timeout:             45 * time.Second,
		LatencyThreshold:    8 * time.Second,
		BudgetThreshold:     0.1, // Open when <10% CU remaining
		WindowSize:          15,
		MinRequestsInWindow: 3,
	},
	"dexscreener": {
		Provider:            "dexscreener",
		ErrorThreshold:      4,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		LatencyThreshold:    6 * time.Second,
		BudgetThreshold:     0.0, // No budget limit
		WindowSize:          20,
		MinRequestsInWindow: 4,
	},
	"kraken": {
		Provider:            "kraken",
		ErrorThreshold:      2,
		SuccessThreshold:    1,
		Timeout:             60 * time.Second,
		LatencyThreshold:    15 * time.Second,
		BudgetThreshold:     0.0, // No budget limit
		WindowSize:          10,
		MinRequestsInWindow: 2,
	},
}

// Default fallback chains - preferred order when primary fails
var DefaultFallbackChains = map[string][]string{
	"binance":     {"kraken", "coingecko"},
	"coingecko":   {"binance", "kraken"},
	"moralis":     {"coingecko", "binance"},
	"dexscreener": {"coingecko", "binance"},
	"kraken":      {"binance", "coingecko"},
}

// NewCircuitManager creates a new circuit manager with default configurations
func NewCircuitManager() *CircuitManager {
	cm := &CircuitManager{
		circuits:       make(map[string]*CircuitBreaker),
		fallbackChains: make(map[string][]string),
	}

	// Initialize circuits for all providers
	for provider, config := range DefaultCircuitConfigs {
		cm.circuits[provider] = &CircuitBreaker{
			config:          config,
			state:           CircuitClosed,
			requests:        make([]RequestResult, config.WindowSize),
			currentProvider: provider,
		}

		// Set fallback chains
		if fallbacks, exists := DefaultFallbackChains[provider]; exists {
			cm.fallbackChains[provider] = fallbacks
			cm.circuits[provider].fallbackProviders = fallbacks
		}
	}

	return cm
}

// CanMakeRequest checks if a request can be made to the provider
func (cm *CircuitManager) CanMakeRequest(provider string) bool {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if !exists {
		return false
	}

	return circuit.canMakeRequest()
}

// RecordRequest records the result of a request
func (cm *CircuitManager) RecordRequest(provider string, success bool, latency time.Duration, err error) {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if !exists {
		return
	}

	circuit.recordRequest(success, latency, err)
}

// GetActiveProvider returns the currently active provider (may be a fallback)
func (cm *CircuitManager) GetActiveProvider(originalProvider string) string {
	cm.mu.RLock()
	circuit, exists := cm.circuits[originalProvider]
	cm.mu.RUnlock()

	if !exists {
		return originalProvider
	}

	if circuit.canMakeRequest() {
		return originalProvider
	}

	// Check fallback providers
	for _, fallback := range circuit.fallbackProviders {
		if fallbackCircuit, exists := cm.circuits[fallback]; exists {
			if fallbackCircuit.canMakeRequest() {
				return fallback
			}
		}
	}

	// Return original provider if no fallbacks available (will fail)
	return originalProvider
}

// GetCircuitState returns the current state of a circuit
func (cm *CircuitManager) GetCircuitState(provider string) CircuitState {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if !exists {
		return CircuitClosed // Default to closed for unknown providers
	}

	return circuit.getState()
}

// ForceOpen forces a circuit to open (for testing or manual intervention)
func (cm *CircuitManager) ForceOpen(provider string) {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if exists {
		circuit.forceOpen()
	}
}

// ForceClose forces a circuit to close (for testing or manual intervention)
func (cm *CircuitManager) ForceClose(provider string) {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if exists {
		circuit.forceClose()
	}
}

// CheckBudgetThreshold checks if circuit should open due to low budget
func (cm *CircuitManager) CheckBudgetThreshold(provider string, healthPercent float64) {
	cm.mu.RLock()
	circuit, exists := cm.circuits[provider]
	cm.mu.RUnlock()

	if !exists {
		return
	}

	circuit.checkBudgetThreshold(healthPercent)
}

func (cb *CircuitBreaker) canMakeRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	now := time.Now()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if now.Sub(cb.lastFailTime) >= cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == CircuitOpen { // Double-check after acquiring write lock
				cb.state = CircuitHalfOpen
				cb.successCount = 0
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}

	return false
}

func (cb *CircuitBreaker) recordRequest(success bool, latency time.Duration, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	// Check latency threshold first
	if success && latency > cb.config.LatencyThreshold {
		success = false // Treat slow requests as failures
		err = fmt.Errorf("request too slow: %v > %v", latency, cb.config.LatencyThreshold)
	}

	// Add request to sliding window with corrected success value
	cb.requests[cb.requestCount%cb.config.WindowSize] = RequestResult{
		Timestamp: now,
		Success:   success,
		Latency:   latency,
		Error:     err,
	}
	cb.requestCount++

	if success {
		cb.successCount++
		cb.lastSuccessTime = now

		// If half-open and enough successes, close the circuit
		if cb.state == CircuitHalfOpen && cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.errorCount = 0
		}
	} else {
		cb.errorCount++
		cb.lastFailTime = now
		cb.successCount = 0

		// Check if we should open the circuit
		if cb.shouldOpen() {
			cb.state = CircuitOpen
		}
	}
}

func (cb *CircuitBreaker) shouldOpen() bool {
	// Only consider opening if we have enough requests in the window
	if cb.requestCount < cb.config.MinRequestsInWindow {
		return false
	}

	// Calculate error rate over sliding window
	windowSize := minInt(cb.requestCount, cb.config.WindowSize)
	errorCount := 0

	for i := 0; i < windowSize; i++ {
		if !cb.requests[i].Success {
			errorCount++
		}
	}

	errorRate := float64(errorCount) / float64(windowSize)
	threshold := float64(cb.config.ErrorThreshold) / float64(cb.config.WindowSize)

	return errorRate >= threshold
}

func (cb *CircuitBreaker) checkBudgetThreshold(healthPercent float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.config.BudgetThreshold > 0 && healthPercent < cb.config.BudgetThreshold*100 {
		if cb.state == CircuitClosed {
			cb.state = CircuitOpen
			cb.lastFailTime = time.Now()
		}
	}
}

func (cb *CircuitBreaker) getState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) forceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitOpen
	cb.lastFailTime = time.Now()
}

func (cb *CircuitBreaker) forceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.errorCount = 0
	cb.successCount = 0
}

// GetStats returns current statistics for the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Calculate current window stats
	windowSize := minInt(cb.requestCount, cb.config.WindowSize)
	var totalLatency time.Duration
	var maxLatency time.Duration
	errorCount := 0

	for i := 0; i < windowSize; i++ {
		req := cb.requests[i]
		totalLatency += req.Latency
		if req.Latency > maxLatency {
			maxLatency = req.Latency
		}
		if !req.Success {
			errorCount++
		}
	}

	var avgLatency time.Duration
	if windowSize > 0 {
		avgLatency = totalLatency / time.Duration(windowSize)
	}

	errorRate := 0.0
	if windowSize > 0 {
		errorRate = float64(errorCount) / float64(windowSize) * 100
	}

	return CircuitStats{
		Provider:          cb.config.Provider,
		State:             cb.state.String(),
		ErrorCount:        cb.errorCount,
		SuccessCount:      cb.successCount,
		RequestCount:      cb.requestCount,
		ErrorRate:         errorRate,
		AvgLatency:        avgLatency,
		MaxLatency:        maxLatency,
		LastFailTime:      cb.lastFailTime,
		LastSuccessTime:   cb.lastSuccessTime,
		FallbackProviders: cb.fallbackProviders,
	}
}

// CircuitStats represents statistics for a circuit breaker
type CircuitStats struct {
	Provider          string        `json:"provider"`
	State             string        `json:"state"`
	ErrorCount        int           `json:"error_count"`
	SuccessCount      int           `json:"success_count"`
	RequestCount      int           `json:"request_count"`
	ErrorRate         float64       `json:"error_rate_percent"`
	AvgLatency        time.Duration `json:"avg_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	LastFailTime      time.Time     `json:"last_fail_time"`
	LastSuccessTime   time.Time     `json:"last_success_time"`
	FallbackProviders []string      `json:"fallback_providers"`
}

// GetAllStats returns statistics for all circuits
func (cm *CircuitManager) GetAllStats() map[string]CircuitStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]CircuitStats)
	for provider, circuit := range cm.circuits {
		stats[provider] = circuit.GetStats()
	}

	return stats
}
