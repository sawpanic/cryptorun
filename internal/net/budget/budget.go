package budget

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrBudgetExhausted is returned when daily budget is exceeded
	ErrBudgetExhausted = errors.New("daily budget exhausted")
	// ErrBudgetWarning is returned when approaching budget limit
	ErrBudgetWarning = errors.New("budget warning threshold exceeded")
)

// BudgetExhaustedError provides detailed information about budget exhaustion
type BudgetExhaustedError struct {
	Provider string
	Used     int64
	Limit    int64
	ETA      time.Time
}

func (e *BudgetExhaustedError) Error() string {
	return fmt.Sprintf("budget exhausted for %s: %d/%d requests used, resets at %s",
		e.Provider, e.Used, e.Limit, e.ETA.Format("15:04 UTC"))
}

// BudgetWarningError provides information about budget warning
type BudgetWarningError struct {
	Provider  string
	Used      int64
	Limit     int64
	Threshold float64
}

func (e *BudgetWarningError) Error() string {
	utilization := float64(e.Used) / float64(e.Limit) * 100
	return fmt.Sprintf("budget warning for %s: %.1f%% used (%d/%d), threshold %.1f%%",
		e.Provider, utilization, e.Used, e.Limit, e.Threshold*100)
}

// Tracker tracks daily budget usage for a single provider
type Tracker struct {
	limit         int64     // Daily budget limit
	used          int64     // Requests used today (atomic)
	resetHour     int       // UTC hour to reset (0-23)
	warnThreshold float64   // Warning threshold (0.0-1.0)
	lastReset     time.Time // Last reset timestamp
	mu            sync.RWMutex
}

// NewTracker creates a new budget tracker
func NewTracker(limit int64, resetHour int, warnThreshold float64) *Tracker {
	if resetHour < 0 || resetHour > 23 {
		resetHour = 0
	}
	if warnThreshold <= 0 || warnThreshold > 1 {
		warnThreshold = 0.8
	}

	now := time.Now().UTC()
	return &Tracker{
		limit:         limit,
		resetHour:     resetHour,
		warnThreshold: warnThreshold,
		lastReset:     getLastResetTime(now, resetHour),
	}
}

// getLastResetTime calculates the last reset time based on current time and reset hour
func getLastResetTime(now time.Time, resetHour int) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), resetHour, 0, 0, 0, time.UTC)
	if now.Hour() >= resetHour {
		return today
	}
	return today.AddDate(0, 0, -1)
}

// getNextResetTime calculates the next reset time
func (t *Tracker) getNextResetTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.lastReset.Add(24 * time.Hour)
}

// checkAndResetIfNeeded checks if budget should be reset and resets if necessary
func (t *Tracker) checkAndResetIfNeeded() {
	now := time.Now().UTC()
	nextReset := t.getNextResetTime()

	if now.After(nextReset) {
		t.mu.Lock()
		defer t.mu.Unlock()

		// Double-check after acquiring write lock
		if now.After(t.lastReset.Add(24 * time.Hour)) {
			atomic.StoreInt64(&t.used, 0)
			t.lastReset = getLastResetTime(now, t.resetHour)
		}
	}
}

// Allow returns true if a request is allowed within budget
func (t *Tracker) Allow() error {
	t.checkAndResetIfNeeded()

	currentUsed := atomic.LoadInt64(&t.used)

	// Check hard limit
	if currentUsed >= t.limit {
		return &BudgetExhaustedError{
			Used:  currentUsed,
			Limit: t.limit,
			ETA:   t.getNextResetTime(),
		}
	}

	// Check warning threshold
	utilizationRate := float64(currentUsed) / float64(t.limit)
	if utilizationRate >= t.warnThreshold {
		return &BudgetWarningError{
			Used:      currentUsed,
			Limit:     t.limit,
			Threshold: t.warnThreshold,
		}
	}

	return nil
}

// Consume increments the usage counter and returns error if budget exceeded
func (t *Tracker) Consume() error {
	t.checkAndResetIfNeeded()

	newUsed := atomic.AddInt64(&t.used, 1)

	// Check hard limit after increment
	if newUsed > t.limit {
		// Decrement back since we exceeded
		atomic.AddInt64(&t.used, -1)
		return &BudgetExhaustedError{
			Used:  newUsed - 1,
			Limit: t.limit,
			ETA:   t.getNextResetTime(),
		}
	}

	// Check warning threshold
	utilizationRate := float64(newUsed) / float64(t.limit)
	if utilizationRate >= t.warnThreshold {
		return &BudgetWarningError{
			Used:      newUsed,
			Limit:     t.limit,
			Threshold: t.warnThreshold,
		}
	}

	return nil
}

// Stats returns current budget statistics
func (t *Tracker) Stats() Stats {
	t.checkAndResetIfNeeded()

	t.mu.RLock()
	defer t.mu.RUnlock()

	currentUsed := atomic.LoadInt64(&t.used)
	utilizationRate := float64(currentUsed) / float64(t.limit)

	return Stats{
		Limit:           t.limit,
		Used:            currentUsed,
		Remaining:       t.limit - currentUsed,
		UtilizationRate: utilizationRate,
		WarnThreshold:   t.warnThreshold,
		ResetHour:       t.resetHour,
		LastReset:       t.lastReset,
		NextReset:       t.getNextResetTime(),
		IsWarning:       utilizationRate >= t.warnThreshold,
		IsExhausted:     currentUsed >= t.limit,
	}
}

// Reset manually resets the budget counter
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	atomic.StoreInt64(&t.used, 0)
	t.lastReset = time.Now().UTC()
}

// SetLimit updates the daily budget limit
func (t *Tracker) SetLimit(limit int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.limit = limit
}

// SetWarnThreshold updates the warning threshold
func (t *Tracker) SetWarnThreshold(threshold float64) {
	if threshold <= 0 || threshold > 1 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.warnThreshold = threshold
}

// Stats represents budget tracker statistics
type Stats struct {
	Limit           int64     `json:"limit"`
	Used            int64     `json:"used"`
	Remaining       int64     `json:"remaining"`
	UtilizationRate float64   `json:"utilization_rate"`
	WarnThreshold   float64   `json:"warn_threshold"`
	ResetHour       int       `json:"reset_hour"`
	LastReset       time.Time `json:"last_reset"`
	NextReset       time.Time `json:"next_reset"`
	IsWarning       bool      `json:"is_warning"`
	IsExhausted     bool      `json:"is_exhausted"`
}

// TimeToReset returns the duration until next budget reset
func (s *Stats) TimeToReset() time.Duration {
	return time.Until(s.NextReset)
}

// Manager manages budget trackers for multiple providers
type Manager struct {
	trackers map[string]*Tracker
	mu       sync.RWMutex
}

// NewManager creates a new budget manager
func NewManager() *Manager {
	return &Manager{
		trackers: make(map[string]*Tracker),
	}
}

// AddProvider adds a budget tracker for a specific provider
func (m *Manager) AddProvider(name string, limit int64, resetHour int, warnThreshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.trackers[name] = NewTracker(limit, resetHour, warnThreshold)
}

// GetTracker returns the budget tracker for a specific provider
func (m *Manager) GetTracker(provider string) (*Tracker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tracker, exists := m.trackers[provider]
	return tracker, exists
}

// Allow checks if a request is allowed for the specified provider
func (m *Manager) Allow(provider string) error {
	tracker, exists := m.GetTracker(provider)
	if !exists {
		return nil // No budget tracking configured, allow request
	}
	return tracker.Allow()
}

// Consume records usage for the specified provider
func (m *Manager) Consume(provider string) error {
	tracker, exists := m.GetTracker(provider)
	if !exists {
		return nil // No budget tracking configured
	}
	return tracker.Consume()
}

// Stats returns statistics for all providers
func (m *Manager) Stats() map[string]Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]Stats)
	for provider, tracker := range m.trackers {
		stats[provider] = tracker.Stats()
	}
	return stats
}

// GetWarnings returns a list of providers with budget warnings
func (m *Manager) GetWarnings() []string {
	stats := m.Stats()
	var warnings []string

	for provider, stat := range stats {
		if stat.IsWarning {
			warnings = append(warnings, fmt.Sprintf("%s (%.1f%% used)",
				provider, stat.UtilizationRate*100))
		}
	}

	return warnings
}

// GetExhausted returns a list of providers with exhausted budgets
func (m *Manager) GetExhausted() []string {
	stats := m.Stats()
	var exhausted []string

	for provider, stat := range stats {
		if stat.IsExhausted {
			exhausted = append(exhausted, fmt.Sprintf("%s (%d/%d used, resets in %v)",
				provider, stat.Used, stat.Limit, stat.TimeToReset().Round(time.Minute)))
		}
	}

	return exhausted
}

// Reset resets all budget trackers
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tracker := range m.trackers {
		tracker.Reset()
	}
}
