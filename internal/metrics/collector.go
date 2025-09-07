package metrics

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Collector aggregates system metrics for monitoring endpoints
type Collector struct {
	mu              sync.RWMutex
	apiHealth       map[string]*APIHealthMetrics
	circuitBreakers map[string]*CircuitBreakerState
	cacheHitRates   *CacheMetrics
	latencyMetrics  *LatencyMetrics
	decileAnalysis  *DecileAnalysis
	lastUpdate      time.Time
}

// APIHealthMetrics tracks health and budget for each provider
type APIHealthMetrics struct {
	Provider        string    `json:"provider"`
	Status          string    `json:"status"` // "healthy", "degraded", "down"
	RequestsPerHour int       `json:"requests_per_hour"`
	BudgetUsed      float64   `json:"budget_used"`      // 0.0 to 1.0
	BudgetRemaining float64   `json:"budget_remaining"` // 0.0 to 1.0
	LastError       string    `json:"last_error,omitempty"`
	LastCheck       time.Time `json:"last_check"`
	ResponseTime    float64   `json:"response_time_ms"` // Average response time
}

// CircuitBreakerState tracks circuit breaker states
type CircuitBreakerState struct {
	Name              string    `json:"name"`
	State             string    `json:"state"` // "closed", "half-open", "open"
	FailureCount      int       `json:"failure_count"`
	SuccessCount      int       `json:"success_count"`
	LastFailure       time.Time `json:"last_failure,omitempty"`
	NextRetry         time.Time `json:"next_retry,omitempty"`
	ThresholdCount    int       `json:"threshold_count"`
	ProbeRunning      bool      `json:"probe_running"`
	LastProbeTime     time.Time `json:"last_probe_time,omitempty"`
	CurrentBackoff    string    `json:"current_backoff"`
	ProbeSuccessCount int       `json:"probe_success_count"`
}

// CacheMetrics tracks cache hit rates for hot and warm tiers
type CacheMetrics struct {
	HotCache  CacheTierMetrics `json:"hot_cache"`
	WarmCache CacheTierMetrics `json:"warm_cache"`
}

// CacheTierMetrics represents metrics for a single cache tier
type CacheTierMetrics struct {
	HitCount   int64   `json:"hit_count"`
	MissCount  int64   `json:"miss_count"`
	HitRate    float64 `json:"hit_rate"` // 0.0 to 1.0
	TotalKeys  int     `json:"total_keys"`
	UsedMemory int64   `json:"used_memory_bytes"`
}

// LatencyMetrics tracks queue and scan latency
type LatencyMetrics struct {
	QueueLatencyP50 float64 `json:"queue_latency_p50_ms"`
	QueueLatencyP95 float64 `json:"queue_latency_p95_ms"`
	QueueLatencyP99 float64 `json:"queue_latency_p99_ms"`
	ScanLatencyP50  float64 `json:"scan_latency_p50_ms"`
	ScanLatencyP95  float64 `json:"scan_latency_p95_ms"`
	ScanLatencyP99  float64 `json:"scan_latency_p99_ms"`
	AvgQueueDepth   float64 `json:"avg_queue_depth"`
	TotalScansToday int     `json:"total_scans_today"`
}

// DecileAnalysis tracks score vs forward returns correlation
type DecileAnalysis struct {
	Deciles     []DecileBucket `json:"deciles"`
	Correlation float64        `json:"correlation"` // Pearson correlation
	SharpeRatio float64        `json:"sharpe_ratio"`
	MaxDrawdown float64        `json:"max_drawdown"`
	LastUpdated time.Time      `json:"last_updated"`
	SampleSize  int            `json:"sample_size"`
	TimeHorizon string         `json:"time_horizon"` // "24h", "48h"
}

// DecileBucket represents performance of a score decile
type DecileBucket struct {
	Decile           int     `json:"decile"` // 1-10
	MinScore         float64 `json:"min_score"`
	MaxScore         float64 `json:"max_score"`
	AvgForwardReturn float64 `json:"avg_forward_return"`
	MedianReturn     float64 `json:"median_return"`
	WinRate          float64 `json:"win_rate"`
	SampleCount      int     `json:"sample_count"`
	Lift             float64 `json:"lift"` // vs baseline/random
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		apiHealth:       make(map[string]*APIHealthMetrics),
		circuitBreakers: make(map[string]*CircuitBreakerState),
		cacheHitRates:   &CacheMetrics{},
		latencyMetrics:  &LatencyMetrics{},
		decileAnalysis:  &DecileAnalysis{},
		lastUpdate:      time.Now(),
	}
}

// StartCollection begins background metrics collection
func (c *Collector) StartCollection(ctx context.Context) {
	log.Info().Msg("Starting metrics collection background process")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initialize with sample data
	c.initializeSampleData()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping metrics collection")
			return
		case <-ticker.C:
			c.collectMetrics()
		}
	}
}

// collectMetrics gathers fresh metrics from all sources
func (c *Collector) collectMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.updateAPIHealth()
	c.updateCircuitBreakers()
	c.updateCacheMetrics()
	c.updateLatencyMetrics()
	c.updateDecileAnalysis()

	c.lastUpdate = time.Now()
}

// GetAPIHealth returns current API health metrics
func (c *Collector) GetAPIHealth() map[string]*APIHealthMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*APIHealthMetrics)
	for k, v := range c.apiHealth {
		// Create copy to avoid data races
		result[k] = &APIHealthMetrics{
			Provider:        v.Provider,
			Status:          v.Status,
			RequestsPerHour: v.RequestsPerHour,
			BudgetUsed:      v.BudgetUsed,
			BudgetRemaining: v.BudgetRemaining,
			LastError:       v.LastError,
			LastCheck:       v.LastCheck,
			ResponseTime:    v.ResponseTime,
		}
	}
	return result
}

// GetCircuitBreakers returns current circuit breaker states
func (c *Collector) GetCircuitBreakers() map[string]*CircuitBreakerState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*CircuitBreakerState)
	for k, v := range c.circuitBreakers {
		result[k] = &CircuitBreakerState{
			Name:              v.Name,
			State:             v.State,
			FailureCount:      v.FailureCount,
			SuccessCount:      v.SuccessCount,
			LastFailure:       v.LastFailure,
			NextRetry:         v.NextRetry,
			ThresholdCount:    v.ThresholdCount,
			ProbeRunning:      v.ProbeRunning,
			LastProbeTime:     v.LastProbeTime,
			CurrentBackoff:    v.CurrentBackoff,
			ProbeSuccessCount: v.ProbeSuccessCount,
		}
	}
	return result
}

// GetCacheMetrics returns current cache hit rate metrics
func (c *Collector) GetCacheMetrics() *CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &CacheMetrics{
		HotCache: CacheTierMetrics{
			HitCount:   c.cacheHitRates.HotCache.HitCount,
			MissCount:  c.cacheHitRates.HotCache.MissCount,
			HitRate:    c.cacheHitRates.HotCache.HitRate,
			TotalKeys:  c.cacheHitRates.HotCache.TotalKeys,
			UsedMemory: c.cacheHitRates.HotCache.UsedMemory,
		},
		WarmCache: CacheTierMetrics{
			HitCount:   c.cacheHitRates.WarmCache.HitCount,
			MissCount:  c.cacheHitRates.WarmCache.MissCount,
			HitRate:    c.cacheHitRates.WarmCache.HitRate,
			TotalKeys:  c.cacheHitRates.WarmCache.TotalKeys,
			UsedMemory: c.cacheHitRates.WarmCache.UsedMemory,
		},
	}
}

// GetLatencyMetrics returns current latency metrics
func (c *Collector) GetLatencyMetrics() *LatencyMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &LatencyMetrics{
		QueueLatencyP50: c.latencyMetrics.QueueLatencyP50,
		QueueLatencyP95: c.latencyMetrics.QueueLatencyP95,
		QueueLatencyP99: c.latencyMetrics.QueueLatencyP99,
		ScanLatencyP50:  c.latencyMetrics.ScanLatencyP50,
		ScanLatencyP95:  c.latencyMetrics.ScanLatencyP95,
		ScanLatencyP99:  c.latencyMetrics.ScanLatencyP99,
		AvgQueueDepth:   c.latencyMetrics.AvgQueueDepth,
		TotalScansToday: c.latencyMetrics.TotalScansToday,
	}
}

// GetDecileAnalysis returns current decile analysis
func (c *Collector) GetDecileAnalysis() *DecileAnalysis {
	c.mu.RLock()
	defer c.mu.RUnlock()

	deciles := make([]DecileBucket, len(c.decileAnalysis.Deciles))
	copy(deciles, c.decileAnalysis.Deciles)

	return &DecileAnalysis{
		Deciles:     deciles,
		Correlation: c.decileAnalysis.Correlation,
		SharpeRatio: c.decileAnalysis.SharpeRatio,
		MaxDrawdown: c.decileAnalysis.MaxDrawdown,
		LastUpdated: c.decileAnalysis.LastUpdated,
		SampleSize:  c.decileAnalysis.SampleSize,
		TimeHorizon: c.decileAnalysis.TimeHorizon,
	}
}

// initializeSampleData creates realistic sample metrics for demonstration
func (c *Collector) initializeSampleData() {
	// Initialize API health for common providers
	c.apiHealth["kraken"] = &APIHealthMetrics{
		Provider:        "kraken",
		Status:          "healthy",
		RequestsPerHour: 450,
		BudgetUsed:      0.23,
		BudgetRemaining: 0.77,
		LastCheck:       time.Now(),
		ResponseTime:    125.5,
	}

	c.apiHealth["binance"] = &APIHealthMetrics{
		Provider:        "binance",
		Status:          "healthy",
		RequestsPerHour: 320,
		BudgetUsed:      0.18,
		BudgetRemaining: 0.82,
		LastCheck:       time.Now(),
		ResponseTime:    89.2,
	}

	c.apiHealth["coinbase"] = &APIHealthMetrics{
		Provider:        "coinbase",
		Status:          "degraded",
		RequestsPerHour: 180,
		BudgetUsed:      0.95,
		BudgetRemaining: 0.05,
		LastError:       "rate limit exceeded",
		LastCheck:       time.Now(),
		ResponseTime:    450.8,
	}

	// Initialize circuit breakers
	c.circuitBreakers["kraken-rest"] = &CircuitBreakerState{
		Name:           "kraken-rest",
		State:          "closed",
		FailureCount:   0,
		SuccessCount:   1250,
		ThresholdCount: 5,
	}

	c.circuitBreakers["binance-ws"] = &CircuitBreakerState{
		Name:           "binance-ws",
		State:          "half-open",
		FailureCount:   3,
		SuccessCount:   890,
		LastFailure:    time.Now().Add(-5 * time.Minute),
		NextRetry:      time.Now().Add(2 * time.Minute),
		ThresholdCount: 5,
	}

	// Initialize cache metrics
	c.cacheHitRates.HotCache = CacheTierMetrics{
		HitCount:   4520,
		MissCount:  680,
		HitRate:    0.869,
		TotalKeys:  3200,
		UsedMemory: 67108864, // 64MB
	}

	c.cacheHitRates.WarmCache = CacheTierMetrics{
		HitCount:   2890,
		MissCount:  1240,
		HitRate:    0.700,
		TotalKeys:  8500,
		UsedMemory: 134217728, // 128MB
	}

	// Initialize latency metrics
	c.latencyMetrics = &LatencyMetrics{
		QueueLatencyP50: 45.2,
		QueueLatencyP95: 89.5,
		QueueLatencyP99: 142.8,
		ScanLatencyP50:  185.6,
		ScanLatencyP95:  298.4,
		ScanLatencyP99:  445.2,
		AvgQueueDepth:   3.2,
		TotalScansToday: 127,
	}

	// Initialize decile analysis with realistic fixtures
	c.decileAnalysis = c.generateDecileFixtures()
}

// updateAPIHealth simulates API health metric updates
func (c *Collector) updateAPIHealth() {
	for provider, health := range c.apiHealth {
		// Simulate minor fluctuations in metrics
		health.RequestsPerHour += rand.Intn(21) - 10 // ±10 requests
		if health.RequestsPerHour < 0 {
			health.RequestsPerHour = 0
		}

		health.ResponseTime += (rand.Float64() - 0.5) * 20 // ±10ms variation
		if health.ResponseTime < 10 {
			health.ResponseTime = 10
		}

		// Simulate budget consumption
		health.BudgetUsed += rand.Float64() * 0.001 // Small incremental usage
		if health.BudgetUsed > 1.0 {
			health.BudgetUsed = 1.0
			health.Status = "degraded"
		}
		health.BudgetRemaining = 1.0 - health.BudgetUsed

		health.LastCheck = time.Now()

		log.Debug().
			Str("provider", provider).
			Str("status", health.Status).
			Int("rph", health.RequestsPerHour).
			Float64("budget_used", health.BudgetUsed).
			Msg("Updated API health metrics")
	}
}

// updateCircuitBreakers simulates circuit breaker state changes
func (c *Collector) updateCircuitBreakers() {
	for name, cb := range c.circuitBreakers {
		// Simulate success/failure events
		if rand.Float64() > 0.95 { // 5% chance of failure
			cb.FailureCount++
			cb.LastFailure = time.Now()

			if cb.FailureCount >= cb.ThresholdCount && cb.State == "closed" {
				cb.State = "open"
				cb.NextRetry = time.Now().Add(30 * time.Second)
				log.Info().Str("circuit", name).Msg("Circuit breaker opened")
			}
		} else {
			cb.SuccessCount++

			// Recovery logic for half-open state
			if cb.State == "half-open" && cb.SuccessCount%10 == 0 {
				cb.State = "closed"
				cb.FailureCount = 0
				log.Info().Str("circuit", name).Msg("Circuit breaker closed")
			}
		}

		// Transition from open to half-open after timeout
		if cb.State == "open" && time.Now().After(cb.NextRetry) {
			cb.State = "half-open"
			log.Info().Str("circuit", name).Msg("Circuit breaker half-open")
		}
	}
}

// updateCacheMetrics simulates cache performance changes
func (c *Collector) updateCacheMetrics() {
	// Hot cache - higher hit rate, smaller fluctuations
	c.cacheHitRates.HotCache.HitCount += int64(rand.Intn(50) + 20) // 20-70 hits
	c.cacheHitRates.HotCache.MissCount += int64(rand.Intn(10))     // 0-10 misses

	totalHot := c.cacheHitRates.HotCache.HitCount + c.cacheHitRates.HotCache.MissCount
	c.cacheHitRates.HotCache.HitRate = float64(c.cacheHitRates.HotCache.HitCount) / float64(totalHot)

	// Warm cache - lower hit rate, more fluctuation
	c.cacheHitRates.WarmCache.HitCount += int64(rand.Intn(30) + 10) // 10-40 hits
	c.cacheHitRates.WarmCache.MissCount += int64(rand.Intn(20))     // 0-20 misses

	totalWarm := c.cacheHitRates.WarmCache.HitCount + c.cacheHitRates.WarmCache.MissCount
	c.cacheHitRates.WarmCache.HitRate = float64(c.cacheHitRates.WarmCache.HitCount) / float64(totalWarm)

	// Simulate memory usage changes
	c.cacheHitRates.HotCache.UsedMemory += int64(rand.Intn(2097152) - 1048576)  // ±1MB
	c.cacheHitRates.WarmCache.UsedMemory += int64(rand.Intn(4194304) - 2097152) // ±2MB
}

// updateLatencyMetrics simulates latency metric changes
func (c *Collector) updateLatencyMetrics() {
	// Add some realistic variance to latency metrics
	variance := func(base float64, factor float64) float64 {
		return base + (rand.Float64()-0.5)*factor
	}

	c.latencyMetrics.QueueLatencyP50 = math.Max(10, variance(c.latencyMetrics.QueueLatencyP50, 10))
	c.latencyMetrics.QueueLatencyP95 = math.Max(c.latencyMetrics.QueueLatencyP50, variance(c.latencyMetrics.QueueLatencyP95, 20))
	c.latencyMetrics.QueueLatencyP99 = math.Max(c.latencyMetrics.QueueLatencyP95, variance(c.latencyMetrics.QueueLatencyP99, 30))

	c.latencyMetrics.ScanLatencyP50 = math.Max(100, variance(c.latencyMetrics.ScanLatencyP50, 20))
	c.latencyMetrics.ScanLatencyP95 = math.Max(c.latencyMetrics.ScanLatencyP50, variance(c.latencyMetrics.ScanLatencyP95, 50))
	c.latencyMetrics.ScanLatencyP99 = math.Max(c.latencyMetrics.ScanLatencyP95, variance(c.latencyMetrics.ScanLatencyP99, 80))

	c.latencyMetrics.AvgQueueDepth = math.Max(0, variance(c.latencyMetrics.AvgQueueDepth, 1))

	// Increment scan count occasionally
	if rand.Float64() > 0.7 {
		c.latencyMetrics.TotalScansToday++
	}
}

// updateDecileAnalysis periodically refreshes decile analysis with new fixture data
func (c *Collector) updateDecileAnalysis() {
	// Refresh decile analysis every 5 minutes
	if time.Since(c.decileAnalysis.LastUpdated) > 5*time.Minute {
		c.decileAnalysis = c.generateDecileFixtures()
	}
}

// generateDecileFixtures creates realistic decile analysis with score vs forward returns
func (c *Collector) generateDecileFixtures() *DecileAnalysis {
	deciles := make([]DecileBucket, 10)

	// Generate realistic decile buckets with predictive power
	baseReturn := -0.5   // Slightly negative baseline
	liftPerDecile := 1.2 // Each higher decile performs ~1.2% better

	for i := 0; i < 10; i++ {
		decile := i + 1

		// Higher deciles should have higher forward returns (predictive power)
		avgReturn := baseReturn + float64(decile)*liftPerDecile

		// Add some realistic noise
		noise := (rand.Float64() - 0.5) * 1.0
		avgReturn += noise

		deciles[i] = DecileBucket{
			Decile:           decile,
			MinScore:         float64(i * 10), // 0-10, 10-20, etc.
			MaxScore:         float64((i + 1) * 10),
			AvgForwardReturn: avgReturn,
			MedianReturn:     avgReturn * 0.8,             // Median typically lower than mean
			WinRate:          0.45 + float64(decile)*0.04, // 45% base + 4% per decile
			SampleCount:      50 + rand.Intn(30),          // 50-80 samples per decile
			Lift:             float64(decile) * 0.15,      // 15% lift per decile vs random
		}
	}

	// Calculate overall correlation (should be positive for good model)
	correlation := 0.72 + (rand.Float64()-0.5)*0.1 // 0.67-0.77 range

	return &DecileAnalysis{
		Deciles:     deciles,
		Correlation: correlation,
		SharpeRatio: 0.85 + (rand.Float64()-0.5)*0.2, // 0.75-0.95
		MaxDrawdown: -8.5 - rand.Float64()*3.0,       // -8.5% to -11.5%
		LastUpdated: time.Now(),
		SampleSize:  650 + rand.Intn(100), // 650-750 samples
		TimeHorizon: "24h",
	}
}

// UpdateCircuitBreakerFromStats updates circuit breaker metrics from actual provider stats
func (c *Collector) UpdateCircuitBreakerFromStats(name string, stats interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Handle provider circuit breaker stats
	if cbStats, ok := stats.(CircuitBreakerStats); ok {
		c.circuitBreakers[name] = &CircuitBreakerState{
			Name:              cbStats.Name,
			State:             cbStats.State,
			FailureCount:      int(cbStats.FailureCount),
			SuccessCount:      int(cbStats.SuccessCount),
			LastFailure:       cbStats.LastFailureTime,
			NextRetry:         cbStats.NextProbeTime,
			ThresholdCount:    5, // Default threshold
			ProbeRunning:      cbStats.ProbeRunning,
			LastProbeTime:     cbStats.LastProbeTime,
			CurrentBackoff:    cbStats.CurrentBackoff,
			ProbeSuccessCount: cbStats.ProbeSuccessCount,
		}
	}
}

// CircuitBreakerStats represents statistics from provider circuit breakers
// This should match the provider.CircuitBreakerStats struct
type CircuitBreakerStats struct {
	Name              string    `json:"name"`
	State             string    `json:"state"`
	RequestCount      int64     `json:"request_count"`
	FailureCount      int64     `json:"failure_count"`
	SuccessCount      int64     `json:"success_count"`
	FailureRate       float64   `json:"failure_rate"`
	LastFailureTime   time.Time `json:"last_failure_time"`
	LastSuccessTime   time.Time `json:"last_success_time"`
	NextProbeTime     time.Time `json:"next_probe_time"`
	LastProbeTime     time.Time `json:"last_probe_time"`
	ProbeRunning      bool      `json:"probe_running"`
	CurrentBackoff    string    `json:"current_backoff"`
	ProbeSuccessCount int       `json:"probe_success_count"`
}
