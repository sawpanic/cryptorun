package datasources

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// HealthManager aggregates health information from all datasource components
type HealthManager struct {
	providerManager *ProviderManager
	cacheManager    *CacheManager
	circuitManager  *CircuitManager
	mu              sync.RWMutex
	
	// Latency tracking
	latencyStats map[string]*LatencyTracker
}

// LatencyTracker tracks latency percentiles for a provider
type LatencyTracker struct {
	samples []time.Duration
	mu      sync.RWMutex
}

// HealthSnapshot represents the complete health state of all datasources
type HealthSnapshot struct {
	Timestamp     time.Time                    `json:"timestamp"`
	OverallHealth string                       `json:"overall_health"`
	Providers     map[string]ProviderHealth    `json:"providers"`
	Cache         CacheHealth                  `json:"cache"`
	Circuits      map[string]CircuitHealth     `json:"circuits"`
	Summary       HealthSummary                `json:"summary"`
}

// ProviderHealth represents health metrics for a single provider
type ProviderHealth struct {
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	RequestsToday int64         `json:"requests_today"`
	RequestsMonth int64         `json:"requests_month"`
	DailyQuota    int64         `json:"daily_quota"`
	MonthlyQuota  int64         `json:"monthly_quota"`
	HealthPercent float64       `json:"health_percent"`
	WeightUsed    int           `json:"weight_used,omitempty"`
	Latency       LatencyMetrics `json:"latency"`
	Cost          float64       `json:"cost"`
	LastRequest   time.Time     `json:"last_request"`
	Circuit       string        `json:"circuit_state"`
}

// LatencyMetrics represents latency statistics
type LatencyMetrics struct {
	P50 time.Duration `json:"p50"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
	Max time.Duration `json:"max"`
	Avg time.Duration `json:"avg"`
}

// CacheHealth represents cache system health
type CacheHealth struct {
	Status         string `json:"status"`
	TotalEntries   int    `json:"total_entries"`
	ActiveEntries  int    `json:"active_entries"`
	ExpiredEntries int    `json:"expired_entries"`
	HitRate        float64 `json:"hit_rate_percent"`
}

// CircuitHealth represents circuit breaker health
type CircuitHealth struct {
	Provider    string        `json:"provider"`
	State       string        `json:"state"`
	ErrorRate   float64       `json:"error_rate_percent"`
	AvgLatency  time.Duration `json:"avg_latency"`
	MaxLatency  time.Duration `json:"max_latency"`
	LastFailure time.Time     `json:"last_failure,omitempty"`
}

// HealthSummary provides high-level health indicators
type HealthSummary struct {
	ProvidersHealthy   int     `json:"providers_healthy"`
	ProvidersTotal     int     `json:"providers_total"`
	CircuitsClosed     int     `json:"circuits_closed"`
	CircuitsTotal      int     `json:"circuits_total"`
	OverallLatencyP99  time.Duration `json:"overall_latency_p99"`
	CacheHitRate       float64 `json:"cache_hit_rate"`
	BudgetUtilization  float64 `json:"budget_utilization_percent"`
}

// NewHealthManager creates a new health manager
func NewHealthManager(pm *ProviderManager, cm *CacheManager, circm *CircuitManager) *HealthManager {
	return &HealthManager{
		providerManager: pm,
		cacheManager:    cm,
		circuitManager:  circm,
		latencyStats:    make(map[string]*LatencyTracker),
	}
}

// RecordLatency records a latency sample for a provider
func (hm *HealthManager) RecordLatency(provider string, latency time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	tracker, exists := hm.latencyStats[provider]
	if !exists {
		tracker = &LatencyTracker{
			samples: make([]time.Duration, 0, 1000), // Keep last 1000 samples
		}
		hm.latencyStats[provider] = tracker
	}
	
	tracker.addSample(latency)
}

// GetHealthSnapshot returns a complete health snapshot
func (hm *HealthManager) GetHealthSnapshot() HealthSnapshot {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	snapshot := HealthSnapshot{
		Timestamp: time.Now(),
		Providers: make(map[string]ProviderHealth),
		Circuits:  make(map[string]CircuitHealth),
	}
	
	// Collect provider health
	providersHealthy := 0
	totalBudgetUsed := 0.0
	totalProviders := 0
	
	for providerName := range DefaultProviders {
		totalProviders++
		
		usage, err := hm.providerManager.GetUsageStats(providerName)
		if err != nil {
			continue
		}
		
		circuitState := hm.circuitManager.GetCircuitState(providerName)
		
		providerHealth := ProviderHealth{
			Name:          usage.Provider,
			RequestsToday: usage.RequestsToday,
			RequestsMonth: usage.RequestsMonth,
			DailyQuota:    usage.DailyQuota,
			MonthlyQuota:  usage.MonthlyQuota,
			HealthPercent: usage.HealthPercent,
			WeightUsed:    usage.WeightUsed,
			Cost:          0.0, // Free APIs
			LastRequest:   usage.LastRequest,
			Circuit:       circuitState.String(),
		}
		
		// Calculate provider status
		if circuitState == CircuitClosed && usage.HealthPercent > 50 {
			providerHealth.Status = "healthy"
			providersHealthy++
		} else if circuitState == CircuitHalfOpen || (circuitState == CircuitClosed && usage.HealthPercent > 10) {
			providerHealth.Status = "degraded"
		} else {
			providerHealth.Status = "unhealthy"
		}
		
		// Add latency metrics
		if tracker, exists := hm.latencyStats[providerName]; exists {
			providerHealth.Latency = tracker.getMetrics()
		}
		
		// Track budget utilization
		if usage.HealthPercent < 100 {
			totalBudgetUsed += (100.0 - usage.HealthPercent)
		}
		
		snapshot.Providers[providerName] = providerHealth
	}
	
	// Collect cache health
	cacheStats := hm.cacheManager.Stats()
	snapshot.Cache = CacheHealth{
		Status:         "healthy", // Simple status for now
		TotalEntries:   cacheStats.TotalEntries,
		ActiveEntries:  cacheStats.ActiveEntries,
		ExpiredEntries: cacheStats.ExpiredEntries,
		HitRate:        85.0, // Placeholder - would need actual hit tracking
	}
	
	if cacheStats.ExpiredEntries > cacheStats.ActiveEntries {
		snapshot.Cache.Status = "degraded"
	}
	
	// Collect circuit health
	circuitStats := hm.circuitManager.GetAllStats()
	circuitsClosed := 0
	totalCircuits := len(circuitStats)
	var maxLatencyP99 time.Duration
	
	for provider, stats := range circuitStats {
		circuitHealth := CircuitHealth{
			Provider:   stats.Provider,
			State:      stats.State,
			ErrorRate:  stats.ErrorRate,
			AvgLatency: stats.AvgLatency,
			MaxLatency: stats.MaxLatency,
		}
		
		if !stats.LastFailTime.IsZero() {
			circuitHealth.LastFailure = stats.LastFailTime
		}
		
		if stats.State == "closed" {
			circuitsClosed++
		}
		
		if stats.MaxLatency > maxLatencyP99 {
			maxLatencyP99 = stats.MaxLatency
		}
		
		snapshot.Circuits[provider] = circuitHealth
	}
	
	// Create summary
	snapshot.Summary = HealthSummary{
		ProvidersHealthy:   providersHealthy,
		ProvidersTotal:     totalProviders,
		CircuitsClosed:     circuitsClosed,
		CircuitsTotal:      totalCircuits,
		OverallLatencyP99:  maxLatencyP99,
		CacheHitRate:       snapshot.Cache.HitRate,
		BudgetUtilization:  totalBudgetUsed / float64(totalProviders),
	}
	
	// Determine overall health
	snapshot.OverallHealth = hm.calculateOverallHealth(snapshot.Summary)
	
	return snapshot
}

// GetHealthJSON returns the health snapshot as JSON string
func (hm *HealthManager) GetHealthJSON() (string, error) {
	snapshot := hm.GetHealthSnapshot()
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal health snapshot: %w", err)
	}
	return string(data), nil
}

// GetHealthSummary returns a brief health summary
func (hm *HealthManager) GetHealthSummary() string {
	snapshot := hm.GetHealthSnapshot()
	
	return fmt.Sprintf(
		"Health: %s | Providers: %d/%d healthy | Circuits: %d/%d closed | Cache: %s (%.1f%% hit rate) | Latency P99: %v",
		snapshot.OverallHealth,
		snapshot.Summary.ProvidersHealthy,
		snapshot.Summary.ProvidersTotal,
		snapshot.Summary.CircuitsClosed,
		snapshot.Summary.CircuitsTotal,
		snapshot.Cache.Status,
		snapshot.Cache.HitRate,
		snapshot.Summary.OverallLatencyP99,
	)
}

// IsHealthy returns true if the overall system is healthy
func (hm *HealthManager) IsHealthy() bool {
	snapshot := hm.GetHealthSnapshot()
	return snapshot.OverallHealth == "healthy"
}

func (hm *HealthManager) calculateOverallHealth(summary HealthSummary) string {
	// System is healthy if:
	// - >50% providers are healthy
	// - >50% circuits are closed
	// - Cache hit rate >70%
	// - P99 latency <10s
	
	providerHealthRatio := float64(summary.ProvidersHealthy) / float64(summary.ProvidersTotal)
	circuitHealthRatio := float64(summary.CircuitsClosed) / float64(summary.CircuitsTotal)
	
	if providerHealthRatio >= 0.5 &&
		circuitHealthRatio >= 0.5 &&
		summary.CacheHitRate >= 70.0 &&
		summary.OverallLatencyP99 < 10*time.Second {
		return "healthy"
	}
	
	if providerHealthRatio >= 0.3 &&
		circuitHealthRatio >= 0.3 &&
		summary.CacheHitRate >= 50.0 {
		return "degraded"
	}
	
	return "unhealthy"
}

func (lt *LatencyTracker) addSample(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	// Keep only last 1000 samples for memory efficiency
	if len(lt.samples) >= 1000 {
		lt.samples = lt.samples[1:]
	}
	
	lt.samples = append(lt.samples, latency)
}

func (lt *LatencyTracker) getMetrics() LatencyMetrics {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	
	if len(lt.samples) == 0 {
		return LatencyMetrics{}
	}
	
	// Sort samples for percentile calculation
	sorted := make([]time.Duration, len(lt.samples))
	copy(sorted, lt.samples)
	
	// Simple bubble sort for small arrays
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	var total time.Duration
	var max time.Duration
	
	for _, sample := range sorted {
		total += sample
		if sample > max {
			max = sample
		}
	}
	
	n := len(sorted)
	
	return LatencyMetrics{
		P50: sorted[n*50/100],
		P95: sorted[n*95/100],
		P99: sorted[n*99/100],
		Max: max,
		Avg: total / time.Duration(n),
	}
}

// StartHealthMonitoring starts background monitoring tasks
func (hm *HealthManager) StartHealthMonitoring() {
	// Clean expired cache entries every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			hm.cacheManager.CleanExpired()
		}
	}()
	
	// Update circuit breaker budget thresholds every minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			for providerName := range DefaultProviders {
				if usage, err := hm.providerManager.GetUsageStats(providerName); err == nil {
					hm.circuitManager.CheckBudgetThreshold(providerName, usage.HealthPercent)
				}
			}
		}
	}()
}