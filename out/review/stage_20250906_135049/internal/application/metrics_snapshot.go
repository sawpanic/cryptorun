package application

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MetricsSnapshot represents a snapshot of system health from /metrics endpoint
type MetricsSnapshot struct {
	Timestamp    time.Time                `json:"timestamp"`
	APIHealth    map[string]APIHealthInfo `json:"api_health"`
	CircuitBreakerStates map[string]string `json:"circuit_breaker_states"`
	CacheHitRates CacheStats              `json:"cache_hit_rates"`
	Latency      LatencyStats             `json:"latency"`
	QueueDepth   int                      `json:"queue_depth"`
	Status       string                   `json:"status"` // "healthy", "degraded", "unhealthy"
}

// APIHealthInfo contains health info for an API provider
type APIHealthInfo struct {
	Status       string    `json:"status"`
	ResponseTime float64   `json:"response_time_ms"`
	LastCheck    time.Time `json:"last_check"`
	BudgetUsed   float64   `json:"budget_used_pct"`
}

// CacheStats contains cache performance metrics
type CacheStats struct {
	Hot  CacheTierStats `json:"hot"`
	Warm CacheTierStats `json:"warm"`
}

// CacheTierStats contains stats for a cache tier
type CacheTierStats struct {
	HitRate   float64 `json:"hit_rate"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	MemoryMB  float64 `json:"memory_mb"`
}

// LatencyStats contains latency percentiles
type LatencyStats struct {
	ScanP50 float64 `json:"scan_p50_ms"`
	ScanP95 float64 `json:"scan_p95_ms"`
	ScanP99 float64 `json:"scan_p99_ms"`
	QueueP50 float64 `json:"queue_p50_ms"`
	QueueP95 float64 `json:"queue_p95_ms"`
	QueueP99 float64 `json:"queue_p99_ms"`
}

// MetricsCollector collects metrics snapshots from the monitor endpoint
type MetricsCollector struct {
	endpoint string
	timeout  time.Duration
	client   *http.Client
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(host, port string, timeoutMs int) *MetricsCollector {
	endpoint := fmt.Sprintf("http://%s:%s/metrics", host, port)
	timeout := time.Duration(timeoutMs) * time.Millisecond
	
	return &MetricsCollector{
		endpoint: endpoint,
		timeout:  timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CollectSnapshot fetches current metrics from monitor endpoint
func (mc *MetricsCollector) CollectSnapshot() (*MetricsSnapshot, error) {
	resp, err := mc.client.Get(mc.endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	var snapshot MetricsSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse metrics JSON: %w", err)
	}

	// Add timestamp if not present
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	return &snapshot, nil
}

// IsHealthy evaluates if the system is in good operational health
func (ms *MetricsSnapshot) IsHealthy(policies map[string]interface{}) bool {
	// Check cache hit rates
	minHotRate := 0.80  // default
	minWarmRate := 0.60 // default
	
	if opHealth, ok := policies["operational_health"].(map[string]interface{}); ok {
		if rate, ok := opHealth["min_cache_hit_rate_hot"].(float64); ok {
			minHotRate = rate
		}
		if rate, ok := opHealth["min_cache_hit_rate_warm"].(float64); ok {
			minWarmRate = rate
		}
	}

	if ms.CacheHitRates.Hot.HitRate < minHotRate {
		return false
	}
	if ms.CacheHitRates.Warm.HitRate < minWarmRate {
		return false
	}

	// Check API health
	for _, api := range ms.APIHealth {
		if api.Status != "healthy" {
			return false
		}
	}

	// Check circuit breakers
	for _, state := range ms.CircuitBreakerStates {
		if state == "open" {
			return false
		}
	}

	// Check scan latency P99
	maxLatencyMs := 500.0 // default
	if opHealth, ok := policies["operational_health"].(map[string]interface{}); ok {
		if lat, ok := opHealth["max_scan_latency_p99_ms"].(float64); ok {
			maxLatencyMs = lat
		}
	}

	if ms.Latency.ScanP99 > maxLatencyMs {
		return false
	}

	return true
}

// FormatSummary creates a text summary of operational health
func (ms *MetricsSnapshot) FormatSummary() string {
	summary := fmt.Sprintf("**Monitor Health Snapshot** (%s)\n\n", ms.Timestamp.Format(time.RFC3339))
	
	// API Health
	summary += "**API Health:**\n"
	for provider, health := range ms.APIHealth {
		status := "✅"
		if health.Status != "healthy" {
			status = "❌"
		}
		summary += fmt.Sprintf("- %s %s: %.1fms, budget %.1f%%\n", 
			status, provider, health.ResponseTime, health.BudgetUsed)
	}
	
	// Cache Hit Rates
	summary += "\n**Cache Performance:**\n"
	summary += fmt.Sprintf("- Hot tier: %.1f%% hit rate (%.1fMB)\n", 
		ms.CacheHitRates.Hot.HitRate*100, ms.CacheHitRates.Hot.MemoryMB)
	summary += fmt.Sprintf("- Warm tier: %.1f%% hit rate (%.1fMB)\n", 
		ms.CacheHitRates.Warm.HitRate*100, ms.CacheHitRates.Warm.MemoryMB)
	
	// Circuit Breakers
	summary += "\n**Circuit Breakers:**\n"
	for name, state := range ms.CircuitBreakerStates {
		status := "✅"
		if state != "closed" {
			status = "⚠️"
		}
		summary += fmt.Sprintf("- %s %s: %s\n", status, name, state)
	}
	
	// Latency
	summary += "\n**Latency:**\n"
	summary += fmt.Sprintf("- Scan P50/P95/P99: %.1f/%.1f/%.1fms\n", 
		ms.Latency.ScanP50, ms.Latency.ScanP95, ms.Latency.ScanP99)
	summary += fmt.Sprintf("- Queue depth: %d\n", ms.QueueDepth)
	
	return summary
}