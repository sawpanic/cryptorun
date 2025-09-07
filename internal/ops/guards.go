package ops

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// GuardResult represents the result of a guard check
type GuardResult struct {
	Name     string
	Status   GuardStatus
	Message  string
	Metadata map[string]interface{}
}

// GuardStatus represents the status of a guard check
type GuardStatus int

const (
	GuardStatusOK GuardStatus = iota
	GuardStatusWarn
	GuardStatusCritical
	GuardStatusBlock
)

func (s GuardStatus) String() string {
	switch s {
	case GuardStatusOK:
		return "OK"
	case GuardStatusWarn:
		return "WARN"
	case GuardStatusCritical:
		return "CRITICAL"
	case GuardStatusBlock:
		return "BLOCK"
	default:
		return "UNKNOWN"
	}
}

// GuardConfig holds configuration for all guards
type GuardConfig struct {
	Budget      BudgetGuardConfig      `yaml:"budget"`
	CallQuota   CallQuotaGuardConfig   `yaml:"call_quota"`
	Correlation CorrelationGuardConfig `yaml:"correlation"`
	VenueHealth VenueHealthGuardConfig `yaml:"venue_health"`
}

// BudgetGuardConfig configures the budget guard
type BudgetGuardConfig struct {
	Enabled         bool    `yaml:"enabled"`
	HourlyLimit     int     `yaml:"hourly_limit"`
	SoftWarnPercent float64 `yaml:"soft_warn_percent"`
	HardStopPercent float64 `yaml:"hard_stop_percent"`
}

// CallQuotaGuardConfig configures call quota guards
type CallQuotaGuardConfig struct {
	Enabled   bool                           `yaml:"enabled"`
	Providers map[string]ProviderQuotaConfig `yaml:"providers"`
}

// ProviderQuotaConfig configures quota for a specific provider
type ProviderQuotaConfig struct {
	CallsPerMinute int `yaml:"calls_per_minute"`
	BurstLimit     int `yaml:"burst_limit"`
}

// CorrelationGuardConfig configures correlation cap guard
type CorrelationGuardConfig struct {
	Enabled         bool    `yaml:"enabled"`
	MaxCorrelation  float64 `yaml:"max_correlation"`
	TopNSignals     int     `yaml:"top_n_signals"`
	LookbackPeriods int     `yaml:"lookback_periods"`
}

// VenueHealthGuardConfig configures venue health guard
type VenueHealthGuardConfig struct {
	Enabled          bool    `yaml:"enabled"`
	MinUptimePercent float64 `yaml:"min_uptime_percent"`
	MaxLatencyMs     int64   `yaml:"max_latency_ms"`
	MinDepthUSD      float64 `yaml:"min_depth_usd"`
	MaxSpreadBps     float64 `yaml:"max_spread_bps"`
}

// GuardManager manages all operational guards
type GuardManager struct {
	config GuardConfig
	mu     sync.RWMutex

	// Budget tracking
	hourlyCallCounts map[time.Time]int

	// Call quota tracking per provider
	providerCallTimes map[string][]time.Time

	// Correlation data storage
	signalHistory []SignalData

	// Last guard results for caching
	lastResults map[string]GuardResult
	lastCheck   time.Time
}

// SignalData represents historical signal data for correlation analysis
type SignalData struct {
	Symbol    string
	Score     float64
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// NewGuardManager creates a new guard manager
func NewGuardManager(config GuardConfig) *GuardManager {
	return &GuardManager{
		config:            config,
		hourlyCallCounts:  make(map[time.Time]int),
		providerCallTimes: make(map[string][]time.Time),
		signalHistory:     make([]SignalData, 0),
		lastResults:       make(map[string]GuardResult),
	}
}

// CheckAllGuards runs all enabled guards and returns results
func (g *GuardManager) CheckAllGuards() []GuardResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Return cached results if checked recently (within 30 seconds)
	if time.Since(g.lastCheck) < 30*time.Second {
		results := make([]GuardResult, 0, len(g.lastResults))
		for _, result := range g.lastResults {
			results = append(results, result)
		}
		return results
	}

	var results []GuardResult

	// Check budget guard
	if g.config.Budget.Enabled {
		result := g.checkBudgetGuard()
		results = append(results, result)
		g.lastResults["budget"] = result
	}

	// Check call quota guard
	if g.config.CallQuota.Enabled {
		quotaResults := g.checkCallQuotaGuards()
		results = append(results, quotaResults...)
		for _, result := range quotaResults {
			g.lastResults["quota_"+result.Name] = result
		}
	}

	// Check correlation guard
	if g.config.Correlation.Enabled {
		result := g.checkCorrelationGuard()
		results = append(results, result)
		g.lastResults["correlation"] = result
	}

	// Check venue health guard
	if g.config.VenueHealth.Enabled {
		result := g.checkVenueHealthGuard()
		results = append(results, result)
		g.lastResults["venue_health"] = result
	}

	g.lastCheck = time.Now()
	return results
}

// RecordAPICall records an API call for budget and quota tracking
func (g *GuardManager) RecordAPICall(provider string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Record for budget guard (hourly buckets)
	hourBucket := now.Truncate(time.Hour)
	g.hourlyCallCounts[hourBucket]++

	// Clean old hourly buckets (keep last 24 hours)
	cutoff := now.Add(-24 * time.Hour).Truncate(time.Hour)
	for bucket := range g.hourlyCallCounts {
		if bucket.Before(cutoff) {
			delete(g.hourlyCallCounts, bucket)
		}
	}

	// Record for quota guard (per-minute tracking)
	if g.providerCallTimes[provider] == nil {
		g.providerCallTimes[provider] = make([]time.Time, 0)
	}
	g.providerCallTimes[provider] = append(g.providerCallTimes[provider], now)

	// Clean old call times (keep last 2 minutes for burst calculation)
	cutoffMinute := now.Add(-2 * time.Minute)
	newTimes := g.providerCallTimes[provider][:0]
	for _, t := range g.providerCallTimes[provider] {
		if t.After(cutoffMinute) {
			newTimes = append(newTimes, t)
		}
	}
	g.providerCallTimes[provider] = newTimes
}

// RecordSignal records a signal for correlation analysis
func (g *GuardManager) RecordSignal(signal SignalData) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.signalHistory = append(g.signalHistory, signal)

	// Keep only recent signals for correlation analysis
	maxHistory := g.config.Correlation.LookbackPeriods * g.config.Correlation.TopNSignals * 2
	if len(g.signalHistory) > maxHistory {
		g.signalHistory = g.signalHistory[len(g.signalHistory)-maxHistory:]
	}
}

// checkBudgetGuard checks if hourly budget is exceeded
func (g *GuardManager) checkBudgetGuard() GuardResult {
	currentHour := time.Now().Truncate(time.Hour)
	currentCalls := g.hourlyCallCounts[currentHour]

	usagePercent := float64(currentCalls) / float64(g.config.Budget.HourlyLimit)

	result := GuardResult{
		Name: "budget",
		Metadata: map[string]interface{}{
			"current_calls": currentCalls,
			"hourly_limit":  g.config.Budget.HourlyLimit,
			"usage_percent": usagePercent * 100,
		},
	}

	if usagePercent >= g.config.Budget.HardStopPercent {
		result.Status = GuardStatusBlock
		result.Message = fmt.Sprintf("API budget exceeded: %d/%d calls (%.1f%%)",
			currentCalls, g.config.Budget.HourlyLimit, usagePercent*100)
	} else if usagePercent >= g.config.Budget.SoftWarnPercent {
		result.Status = GuardStatusWarn
		result.Message = fmt.Sprintf("API budget warning: %d/%d calls (%.1f%%)",
			currentCalls, g.config.Budget.HourlyLimit, usagePercent*100)
	} else {
		result.Status = GuardStatusOK
		result.Message = fmt.Sprintf("API budget OK: %d/%d calls (%.1f%%)",
			currentCalls, g.config.Budget.HourlyLimit, usagePercent*100)
	}

	return result
}

// checkCallQuotaGuards checks call quotas for all providers
func (g *GuardManager) checkCallQuotaGuards() []GuardResult {
	var results []GuardResult

	for provider, config := range g.config.CallQuota.Providers {
		result := g.checkProviderQuota(provider, config)
		results = append(results, result)
	}

	return results
}

// checkProviderQuota checks quota for a specific provider
func (g *GuardManager) checkProviderQuota(provider string, config ProviderQuotaConfig) GuardResult {
	callTimes := g.providerCallTimes[provider]
	if callTimes == nil {
		return GuardResult{
			Name:     provider,
			Status:   GuardStatusOK,
			Message:  fmt.Sprintf("Provider %s: no recent calls", provider),
			Metadata: map[string]interface{}{"provider": provider, "calls_per_minute": 0},
		}
	}

	now := time.Now()

	// Count calls in last minute
	minuteAgo := now.Add(-time.Minute)
	callsLastMinute := 0
	for _, t := range callTimes {
		if t.After(minuteAgo) {
			callsLastMinute++
		}
	}

	// Count calls in last 10 seconds for burst detection
	burstWindow := now.Add(-10 * time.Second)
	burstCalls := 0
	for _, t := range callTimes {
		if t.After(burstWindow) {
			burstCalls++
		}
	}

	result := GuardResult{
		Name: provider,
		Metadata: map[string]interface{}{
			"provider":         provider,
			"calls_per_minute": callsLastMinute,
			"minute_limit":     config.CallsPerMinute,
			"burst_calls":      burstCalls,
			"burst_limit":      config.BurstLimit,
		},
	}

	if burstCalls >= config.BurstLimit {
		result.Status = GuardStatusBlock
		result.Message = fmt.Sprintf("Provider %s burst limit exceeded: %d/%d calls in 10s",
			provider, burstCalls, config.BurstLimit)
	} else if callsLastMinute >= config.CallsPerMinute {
		result.Status = GuardStatusCritical
		result.Message = fmt.Sprintf("Provider %s rate limit exceeded: %d/%d calls/min",
			provider, callsLastMinute, config.CallsPerMinute)
	} else if float64(callsLastMinute) >= float64(config.CallsPerMinute)*0.8 {
		result.Status = GuardStatusWarn
		result.Message = fmt.Sprintf("Provider %s rate limit warning: %d/%d calls/min",
			provider, callsLastMinute, config.CallsPerMinute)
	} else {
		result.Status = GuardStatusOK
		result.Message = fmt.Sprintf("Provider %s rate OK: %d/%d calls/min",
			provider, callsLastMinute, config.CallsPerMinute)
	}

	return result
}

// checkCorrelationGuard checks if top signals have excessive correlation
func (g *GuardManager) checkCorrelationGuard() GuardResult {
	if len(g.signalHistory) < g.config.Correlation.TopNSignals {
		return GuardResult{
			Name:     "correlation",
			Status:   GuardStatusOK,
			Message:  "Insufficient signal history for correlation analysis",
			Metadata: map[string]interface{}{"signal_count": len(g.signalHistory)},
		}
	}

	// Get recent top N signals
	recentSignals := g.getTopNRecentSignals(g.config.Correlation.TopNSignals)

	// Calculate maximum pairwise correlation
	maxCorrelation := g.calculateMaxCorrelation(recentSignals)

	result := GuardResult{
		Name: "correlation",
		Metadata: map[string]interface{}{
			"max_correlation":   maxCorrelation,
			"correlation_limit": g.config.Correlation.MaxCorrelation,
			"signals_analyzed":  len(recentSignals),
		},
	}

	if maxCorrelation >= g.config.Correlation.MaxCorrelation {
		result.Status = GuardStatusBlock
		result.Message = fmt.Sprintf("Signal correlation too high: %.3f >= %.3f",
			maxCorrelation, g.config.Correlation.MaxCorrelation)
	} else if maxCorrelation >= g.config.Correlation.MaxCorrelation*0.9 {
		result.Status = GuardStatusWarn
		result.Message = fmt.Sprintf("Signal correlation elevated: %.3f (limit %.3f)",
			maxCorrelation, g.config.Correlation.MaxCorrelation)
	} else {
		result.Status = GuardStatusOK
		result.Message = fmt.Sprintf("Signal correlation OK: %.3f (limit %.3f)",
			maxCorrelation, g.config.Correlation.MaxCorrelation)
	}

	return result
}

// checkVenueHealthGuard checks overall venue health
func (g *GuardManager) checkVenueHealthGuard() GuardResult {
	// This would integrate with actual venue health data
	// For now, return a placeholder implementation

	return GuardResult{
		Name:    "venue_health",
		Status:  GuardStatusOK,
		Message: "Venue health monitoring not yet implemented",
		Metadata: map[string]interface{}{
			"implementation": "placeholder",
		},
	}
}

// getTopNRecentSignals gets the top N most recent signals by score
func (g *GuardManager) getTopNRecentSignals(n int) []SignalData {
	if len(g.signalHistory) == 0 {
		return nil
	}

	// Get recent signals within lookback period
	cutoff := time.Now().Add(-time.Duration(g.config.Correlation.LookbackPeriods) * time.Hour)
	var recentSignals []SignalData

	for _, signal := range g.signalHistory {
		if signal.Timestamp.After(cutoff) {
			recentSignals = append(recentSignals, signal)
		}
	}

	// Sort by score descending and take top N
	// Simple bubble sort for small datasets
	for i := 0; i < len(recentSignals)-1; i++ {
		for j := 0; j < len(recentSignals)-i-1; j++ {
			if recentSignals[j].Score < recentSignals[j+1].Score {
				recentSignals[j], recentSignals[j+1] = recentSignals[j+1], recentSignals[j]
			}
		}
	}

	if len(recentSignals) > n {
		recentSignals = recentSignals[:n]
	}

	return recentSignals
}

// calculateMaxCorrelation calculates maximum pairwise correlation between signals
func (g *GuardManager) calculateMaxCorrelation(signals []SignalData) float64 {
	if len(signals) < 2 {
		return 0.0
	}

	maxCorr := 0.0

	// Calculate pairwise correlations
	for i := 0; i < len(signals); i++ {
		for j := i + 1; j < len(signals); j++ {
			corr := g.calculateSignalCorrelation(signals[i], signals[j])
			if math.Abs(corr) > math.Abs(maxCorr) {
				maxCorr = corr
			}
		}
	}

	return math.Abs(maxCorr)
}

// calculateSignalCorrelation calculates correlation between two signals
func (g *GuardManager) calculateSignalCorrelation(sig1, sig2 SignalData) float64 {
	// Simplified correlation calculation based on score similarity
	// In a real implementation, this would use historical price series

	scoreDiff := math.Abs(sig1.Score - sig2.Score)
	maxScore := math.Max(math.Abs(sig1.Score), math.Abs(sig2.Score))

	if maxScore == 0 {
		return 1.0 // Both scores are zero
	}

	// Simple correlation proxy: 1 - normalized score difference
	return 1.0 - (scoreDiff / maxScore)
}
