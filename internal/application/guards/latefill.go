package guards

import (
	"fmt"
	"sync"
	"time"

	"cryptorun/internal/telemetry/latency"
)

// LateFillGuard implements late-fill protection with p99 latency-based relaxation
type LateFillGuard struct {
	baseThresholdMs  float64              // Base threshold in milliseconds
	p99ThresholdMs   float64              // P99 latency threshold for relaxation
	graceWindowMs    float64              // Grace window when p99 exceeded
	relaxTracker     map[string]time.Time // Per-asset relax tracking
	relaxMutex       sync.RWMutex         // Protects relaxTracker
	cooldownDuration time.Duration        // Cooldown between relaxations (30m)
}

// LateFillInput represents input data for late-fill guard evaluation
type LateFillInput struct {
	Symbol        string    // Asset symbol
	SignalTime    time.Time // When signal was generated
	ExecutionTime time.Time // When execution would occur
	FreshnessAge  int       // Bar age (must be ≤2)
	ATRDistance   float64   // Distance from trigger in ATR units (must be ≤1.2)
	ATRCurrent    float64   // Current ATR value
}

// LateFillResult represents the outcome of late-fill guard evaluation
type LateFillResult struct {
	Allowed       bool      // Whether execution is allowed
	Reason        string    // Detailed reason for decision
	RelaxReason   string    // Relaxation reason if applicable
	DelayMs       float64   // Actual delay in milliseconds
	RelaxUsed     bool      // Whether p99 relaxation was applied
	NextRelaxTime time.Time // When next relaxation is allowed
}

// NewLateFillGuard creates a new late-fill guard with specified thresholds
func NewLateFillGuard(baseThresholdMs, p99ThresholdMs, graceWindowMs float64) *LateFillGuard {
	return &LateFillGuard{
		baseThresholdMs:  baseThresholdMs,
		p99ThresholdMs:   p99ThresholdMs,
		graceWindowMs:    graceWindowMs,
		relaxTracker:     make(map[string]time.Time),
		cooldownDuration: 30 * time.Minute, // 30-minute cooldown per spec
	}
}

// DefaultLateFillGuard creates a guard with standard thresholds
func DefaultLateFillGuard() *LateFillGuard {
	return NewLateFillGuard(
		30000, // 30s base threshold
		400,   // 400ms p99 threshold
		30000, // 30s grace window
	)
}

// Evaluate performs late-fill guard evaluation with p99 relaxation logic
func (g *LateFillGuard) Evaluate(input LateFillInput) LateFillResult {
	// Calculate execution delay
	delay := input.ExecutionTime.Sub(input.SignalTime)
	delayMs := float64(delay.Nanoseconds()) / 1e6

	result := LateFillResult{
		DelayMs: delayMs,
		Reason:  "",
	}

	// Check freshness constraints (hard limits per spec)
	if input.FreshnessAge > 2 {
		result.Allowed = false
		result.Reason = fmt.Sprintf("freshness violation: bar age %d > 2 bars maximum", input.FreshnessAge)
		return result
	}

	if input.ATRCurrent > 0 && input.ATRDistance > 1.2 {
		result.Allowed = false
		result.Reason = fmt.Sprintf("freshness violation: price distance %.2f×ATR > 1.2×ATR maximum", input.ATRDistance)
		return result
	}

	// Check base threshold
	if delayMs <= g.baseThresholdMs {
		result.Allowed = true
		result.Reason = fmt.Sprintf("within base threshold: %.1fms ≤ %.1fms", delayMs, g.baseThresholdMs)
		return result
	}

	// Check if p99 relaxation can be applied
	currentP99 := latency.GetP99(latency.StageOrder) // Get current order stage p99

	if currentP99 > g.p99ThresholdMs {
		// P99 threshold exceeded - check if relaxation is available
		if g.canRelax(input.Symbol) {
			// Apply relaxation with grace window
			maxAllowedMs := g.baseThresholdMs + g.graceWindowMs
			if delayMs <= maxAllowedMs {
				// Mark relaxation as used
				g.markRelaxUsed(input.Symbol)

				result.Allowed = true
				result.RelaxUsed = true
				result.RelaxReason = fmt.Sprintf("latefill_relax[p99_exceeded:%.1fms,grace:%.0fs]",
					currentP99, g.graceWindowMs/1000)
				result.Reason = fmt.Sprintf("p99 relaxation applied: %.1fms ≤ %.1fms (base + grace)",
					delayMs, maxAllowedMs)
				result.NextRelaxTime = time.Now().Add(g.cooldownDuration)

				return result
			} else {
				// Even with grace window, delay is too high
				result.Allowed = false
				result.Reason = fmt.Sprintf("excessive delay even with p99 grace: %.1fms > %.1fms (base + grace)",
					delayMs, maxAllowedMs)
				return result
			}
		} else {
			// Relaxation not available (cooldown or already used)
			nextRelaxTime := g.getNextRelaxTime(input.Symbol)
			result.NextRelaxTime = nextRelaxTime

			result.Allowed = false
			result.Reason = fmt.Sprintf("late fill: %.1fms > %.1fms base threshold (p99 relax on cooldown until %s)",
				delayMs, g.baseThresholdMs, nextRelaxTime.Format("15:04:05"))
			return result
		}
	}

	// P99 threshold not exceeded, normal late-fill blocking
	result.Allowed = false
	result.Reason = fmt.Sprintf("late fill: %.1fms > %.1fms base threshold (p99 %.1fms ≤ %.1fms threshold)",
		delayMs, g.baseThresholdMs, currentP99, g.p99ThresholdMs)
	return result
}

// canRelax checks if relaxation is available for the given symbol
func (g *LateFillGuard) canRelax(symbol string) bool {
	g.relaxMutex.RLock()
	defer g.relaxMutex.RUnlock()

	lastUsed, exists := g.relaxTracker[symbol]
	if !exists {
		return true // Never used relaxation
	}

	// Check if cooldown period has passed
	return time.Since(lastUsed) >= g.cooldownDuration
}

// markRelaxUsed records that relaxation was used for the given symbol
func (g *LateFillGuard) markRelaxUsed(symbol string) {
	g.relaxMutex.Lock()
	defer g.relaxMutex.Unlock()

	g.relaxTracker[symbol] = time.Now()
}

// getNextRelaxTime returns when the next relaxation is allowed for the symbol
func (g *LateFillGuard) getNextRelaxTime(symbol string) time.Time {
	g.relaxMutex.RLock()
	defer g.relaxMutex.RUnlock()

	lastUsed, exists := g.relaxTracker[symbol]
	if !exists {
		return time.Now() // Available now
	}

	return lastUsed.Add(g.cooldownDuration)
}

// GetStatus returns current guard status including p99 metrics and relax availability
func (g *LateFillGuard) GetStatus() map[string]interface{} {
	currentP99 := latency.GetP99(latency.StageOrder)

	g.relaxMutex.RLock()
	activeRelaxes := len(g.relaxTracker)
	g.relaxMutex.RUnlock()

	return map[string]interface{}{
		"base_threshold_ms": g.baseThresholdMs,
		"p99_threshold_ms":  g.p99ThresholdMs,
		"grace_window_ms":   g.graceWindowMs,
		"current_p99_ms":    currentP99,
		"p99_exceeded":      currentP99 > g.p99ThresholdMs,
		"active_relaxes":    activeRelaxes,
		"cooldown_duration": g.cooldownDuration,
	}
}

// Reset clears all relax tracking (useful for testing)
func (g *LateFillGuard) Reset() {
	g.relaxMutex.Lock()
	defer g.relaxMutex.Unlock()

	g.relaxTracker = make(map[string]time.Time)
}

// BatchEvaluate evaluates multiple inputs efficiently
func (g *LateFillGuard) BatchEvaluate(inputs []LateFillInput) []LateFillResult {
	results := make([]LateFillResult, len(inputs))

	for i, input := range inputs {
		results[i] = g.Evaluate(input)
	}

	return results
}

// LateFillMetrics provides summary metrics for monitoring
type LateFillMetrics struct {
	TotalEvaluations   int                  `json:"total_evaluations"`
	AllowedCount       int                  `json:"allowed_count"`
	BlockedCount       int                  `json:"blocked_count"`
	RelaxUsedCount     int                  `json:"relax_used_count"`
	CurrentP99Ms       float64              `json:"current_p99_ms"`
	P99Exceeded        bool                 `json:"p99_exceeded"`
	ActiveRelaxSymbols []string             `json:"active_relax_symbols"`
	RelaxAvailability  map[string]time.Time `json:"relax_availability"`
}

// GetMetrics returns current operational metrics
func (g *LateFillGuard) GetMetrics() LateFillMetrics {
	g.relaxMutex.RLock()
	defer g.relaxMutex.RUnlock()

	metrics := LateFillMetrics{
		CurrentP99Ms:       latency.GetP99(latency.StageOrder),
		RelaxAvailability:  make(map[string]time.Time),
		ActiveRelaxSymbols: make([]string, 0),
	}

	metrics.P99Exceeded = metrics.CurrentP99Ms > g.p99ThresholdMs

	// Build relax status
	now := time.Now()
	for symbol, lastUsed := range g.relaxTracker {
		nextAvailable := lastUsed.Add(g.cooldownDuration)
		metrics.RelaxAvailability[symbol] = nextAvailable

		if nextAvailable.After(now) {
			metrics.ActiveRelaxSymbols = append(metrics.ActiveRelaxSymbols, symbol)
		}
	}

	return metrics
}
