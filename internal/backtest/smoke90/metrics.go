package smoke90

import (
	"fmt"
	"sync"
)

// Metrics collects and aggregates statistics during the backtest run
type Metrics struct {
	mu sync.RWMutex

	totalCandidates  int
	passedCandidates int
	failedCandidates int

	guardStats     map[string]*GuardStat
	throttleEvents []*ThrottleEvent
	relaxEvents    []*RelaxEvent
	skipReasons    []string
	errors         []string

	// TopGainers alignment tracking
	topGainersHits map[string]*HitRate // "1h", "24h", "7d"
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		guardStats:     make(map[string]*GuardStat),
		throttleEvents: make([]*ThrottleEvent, 0),
		relaxEvents:    make([]*RelaxEvent, 0),
		skipReasons:    make([]string, 0),
		errors:         make([]string, 0),
		topGainersHits: map[string]*HitRate{
			"1h":  {Total: 0, Hits: 0, Misses: 0, HitRate: 0.0},
			"24h": {Total: 0, Hits: 0, Misses: 0, HitRate: 0.0},
			"7d":  {Total: 0, Hits: 0, Misses: 0, HitRate: 0.0},
		},
	}
}

// RecordWindow records metrics for a completed window
func (m *Metrics) RecordWindow(window *WindowResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update candidate counts
	for _, candidate := range window.Candidates {
		m.totalCandidates++
		if candidate.Passed {
			m.passedCandidates++
		} else {
			m.failedCandidates++
		}
	}

	// Aggregate guard statistics
	for guardName, guardStat := range window.GuardStats {
		if existingStat, exists := m.guardStats[guardName]; exists {
			existingStat.Total += guardStat.Total
			existingStat.Passed += guardStat.Passed
			existingStat.Failed += guardStat.Failed
			existingStat.PassRate = float64(existingStat.Passed) / float64(existingStat.Total) * 100
		} else {
			// Copy guard stat
			m.guardStats[guardName] = &GuardStat{
				Name:     guardStat.Name,
				Type:     guardStat.Type,
				Total:    guardStat.Total,
				Passed:   guardStat.Passed,
				Failed:   guardStat.Failed,
				PassRate: guardStat.PassRate,
			}
		}
	}

	// Record throttling events
	m.throttleEvents = append(m.throttleEvents, window.ThrottleEvents...)

	// Record relaxation events
	m.relaxEvents = append(m.relaxEvents, window.RelaxEvents...)

	// Record skip reasons
	m.skipReasons = append(m.skipReasons, window.SkipReasons...)
}

// RecordTopGainersAlignment records hit/miss vs actual TopGainers
func (m *Metrics) RecordTopGainersAlignment(timeframe string, hit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hitRate, exists := m.topGainersHits[timeframe]; exists {
		hitRate.Total++
		if hit {
			hitRate.Hits++
		} else {
			hitRate.Misses++
		}
		hitRate.HitRate = float64(hitRate.Hits) / float64(hitRate.Total) * 100
	}
}

// RecordError records an error that occurred during backtesting
func (m *Metrics) RecordError(error string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors = append(m.errors, error)
}

// GetSummary returns a comprehensive metrics summary
func (m *Metrics) GetSummary() *MetricsSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	overallPassRate := 0.0
	if m.totalCandidates > 0 {
		overallPassRate = float64(m.passedCandidates) / float64(m.totalCandidates) * 100
	}

	return &MetricsSummary{
		TotalCandidates:   m.totalCandidates,
		PassedCandidates:  m.passedCandidates,
		FailedCandidates:  m.failedCandidates,
		OverallPassRate:   overallPassRate,
		GuardStats:        m.copyGuardStats(),
		TopGainersHitRate: m.getTopGainersHitRate(),
		ThrottleStats:     m.getThrottleStats(),
		RelaxStats:        m.getRelaxStats(),
		SkipStats:         m.getSkipStats(),
		ErrorCount:        len(m.errors),
		Errors:            m.getTopErrors(10), // Limit to top 10 errors
	}
}

// copyGuardStats creates a copy of guard statistics
func (m *Metrics) copyGuardStats() map[string]*GuardStat {
	copy := make(map[string]*GuardStat)
	for name, stat := range m.guardStats {
		copy[name] = &GuardStat{
			Name:     stat.Name,
			Type:     stat.Type,
			Total:    stat.Total,
			Passed:   stat.Passed,
			Failed:   stat.Failed,
			PassRate: stat.PassRate,
		}
	}
	return copy
}

// getTopGainersHitRate returns TopGainers hit rate statistics
func (m *Metrics) getTopGainersHitRate() *HitRateStats {
	return &HitRateStats{
		OneHour: &HitRate{
			Hits:    m.topGainersHits["1h"].Hits,
			Misses:  m.topGainersHits["1h"].Misses,
			Total:   m.topGainersHits["1h"].Total,
			HitRate: m.topGainersHits["1h"].HitRate,
		},
		TwentyFourHour: &HitRate{
			Hits:    m.topGainersHits["24h"].Hits,
			Misses:  m.topGainersHits["24h"].Misses,
			Total:   m.topGainersHits["24h"].Total,
			HitRate: m.topGainersHits["24h"].HitRate,
		},
		SevenDay: &HitRate{
			Hits:    m.topGainersHits["7d"].Hits,
			Misses:  m.topGainersHits["7d"].Misses,
			Total:   m.topGainersHits["7d"].Total,
			HitRate: m.topGainersHits["7d"].HitRate,
		},
	}
}

// getThrottleStats returns provider throttling statistics
func (m *Metrics) getThrottleStats() *ThrottleStats {
	totalEvents := len(m.throttleEvents)
	providerCounts := make(map[string]int)
	mostThrottled := ""
	maxCount := 0

	for _, event := range m.throttleEvents {
		providerCounts[event.Provider]++
		if providerCounts[event.Provider] > maxCount {
			maxCount = providerCounts[event.Provider]
			mostThrottled = event.Provider
		}
	}

	eventsPer100 := 0.0
	if m.totalCandidates > 0 {
		eventsPer100 = float64(totalEvents) / float64(m.totalCandidates) * 100
	}

	return &ThrottleStats{
		TotalEvents:    totalEvents,
		EventsPer100:   eventsPer100,
		ProviderCounts: providerCounts,
		MostThrottled:  mostThrottled,
	}
}

// getRelaxStats returns P99 relaxation statistics
func (m *Metrics) getRelaxStats() *RelaxStats {
	totalEvents := len(m.relaxEvents)

	eventsPer100 := 0.0
	if m.totalCandidates > 0 {
		eventsPer100 = float64(totalEvents) / float64(m.totalCandidates) * 100
	}

	avgP99Ms := 0.0
	avgGraceMs := 0.0
	if totalEvents > 0 {
		totalP99 := 0.0
		totalGrace := 0.0
		for _, event := range m.relaxEvents {
			totalP99 += event.P99Ms
			totalGrace += event.GraceMs
		}
		avgP99Ms = totalP99 / float64(totalEvents)
		avgGraceMs = totalGrace / float64(totalEvents)
	}

	return &RelaxStats{
		TotalEvents:  totalEvents,
		EventsPer100: eventsPer100,
		AvgP99Ms:     avgP99Ms,
		AvgGraceMs:   avgGraceMs,
	}
}

// getSkipStats returns window skip statistics
func (m *Metrics) getSkipStats() *SkipStats {
	skipReasons := make(map[string]int)
	mostCommon := ""
	maxCount := 0

	for _, reason := range m.skipReasons {
		skipReasons[reason]++
		if skipReasons[reason] > maxCount {
			maxCount = skipReasons[reason]
			mostCommon = reason
		}
	}

	return &SkipStats{
		TotalSkips:  len(m.skipReasons),
		SkipReasons: skipReasons,
		MostCommon:  mostCommon,
	}
}

// getTopErrors returns the most frequent errors (limited to n)
func (m *Metrics) getTopErrors(n int) []string {
	if len(m.errors) <= n {
		return m.errors
	}

	// Count error frequencies
	errorCounts := make(map[string]int)
	for _, err := range m.errors {
		errorCounts[err]++
	}

	// Sort by frequency and return top n
	type errorCount struct {
		error string
		count int
	}

	var sorted []errorCount
	for err, count := range errorCounts {
		sorted = append(sorted, errorCount{err, count})
	}

	// Simple sorting by count (descending)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var topErrors []string
	for i := 0; i < n && i < len(sorted); i++ {
		if sorted[i].count > 1 {
			topErrors = append(topErrors, fmt.Sprintf("%s (×%d)", sorted[i].error, sorted[i].count))
		} else {
			topErrors = append(topErrors, sorted[i].error)
		}
	}

	return topErrors
}

// GetGuardPassRate returns the pass rate for a specific guard
func (m *Metrics) GetGuardPassRate(guardName string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if stat, exists := m.guardStats[guardName]; exists {
		return stat.PassRate
	}
	return 0.0
}

// GetHardGuardPassRate returns the overall pass rate for hard guards only
func (m *Metrics) GetHardGuardPassRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalHard := 0
	passedHard := 0

	for _, stat := range m.guardStats {
		if stat.Type == "hard" {
			totalHard += stat.Total
			passedHard += stat.Passed
		}
	}

	if totalHard == 0 {
		return 0.0
	}

	return float64(passedHard) / float64(totalHard) * 100
}

// GetThrottleRate returns the overall throttling rate per 100 signals
func (m *Metrics) GetThrottleRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalCandidates == 0 {
		return 0.0
	}

	return float64(len(m.throttleEvents)) / float64(m.totalCandidates) * 100
}

// GetRelaxRate returns the overall relaxation rate per 100 signals
func (m *Metrics) GetRelaxRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalCandidates == 0 {
		return 0.0
	}

	return float64(len(m.relaxEvents)) / float64(m.totalCandidates) * 100
}

// Reset clears all collected metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalCandidates = 0
	m.passedCandidates = 0
	m.failedCandidates = 0
	m.guardStats = make(map[string]*GuardStat)
	m.throttleEvents = make([]*ThrottleEvent, 0)
	m.relaxEvents = make([]*RelaxEvent, 0)
	m.skipReasons = make([]string, 0)
	m.errors = make([]string, 0)

	for _, hitRate := range m.topGainersHits {
		hitRate.Total = 0
		hitRate.Hits = 0
		hitRate.Misses = 0
		hitRate.HitRate = 0.0
	}
}
