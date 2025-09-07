package endpoints

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sawpanic/cryptorun/internal/metrics"
)

// MetricsResponse represents the complete metrics response
type MetricsResponse struct {
	Timestamp       time.Time                               `json:"timestamp"`
	APIHealth       map[string]*metrics.APIHealthMetrics    `json:"api_health"`
	CircuitBreakers map[string]*metrics.CircuitBreakerState `json:"circuit_breakers"`
	Cache           *metrics.CacheMetrics                   `json:"cache"`
	Latency         *metrics.LatencyMetrics                 `json:"latency"`
	Summary         MetricsSummary                          `json:"summary"`
}

// MetricsSummary provides high-level system health overview
type MetricsSummary struct {
	OverallHealth    string  `json:"overall_health"` // "healthy", "degraded", "critical"
	TotalAPIRequests int     `json:"total_api_requests"`
	AvgResponseTime  float64 `json:"avg_response_time_ms"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	ActiveCircuits   int     `json:"active_circuits"`
	OpenCircuits     int     `json:"open_circuits"`
	AvgScanLatency   float64 `json:"avg_scan_latency_ms"`
	SystemLoad       string  `json:"system_load"` // "low", "medium", "high"
}

// MetricsHandler returns the metrics endpoint
func MetricsHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Gather all metrics from collector
		apiHealth := collector.GetAPIHealth()
		circuitBreakers := collector.GetCircuitBreakers()
		cacheMetrics := collector.GetCacheMetrics()
		latencyMetrics := collector.GetLatencyMetrics()

		// Calculate summary metrics
		summary := calculateMetricsSummary(apiHealth, circuitBreakers, cacheMetrics, latencyMetrics)

		response := MetricsResponse{
			Timestamp:       time.Now(),
			APIHealth:       apiHealth,
			CircuitBreakers: circuitBreakers,
			Cache:           cacheMetrics,
			Latency:         latencyMetrics,
			Summary:         summary,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		// Set status based on overall health
		switch summary.OverallHealth {
		case "healthy":
			w.WriteHeader(http.StatusOK)
		case "degraded":
			w.WriteHeader(http.StatusOK)
		case "critical":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// calculateMetricsSummary computes high-level summary from detailed metrics
func calculateMetricsSummary(
	apiHealth map[string]*metrics.APIHealthMetrics,
	circuitBreakers map[string]*metrics.CircuitBreakerState,
	cache *metrics.CacheMetrics,
	latency *metrics.LatencyMetrics,
) MetricsSummary {
	// Calculate API summary
	totalRequests := 0
	totalResponseTime := 0.0
	healthyAPIs := 0
	totalAPIs := len(apiHealth)

	for _, api := range apiHealth {
		totalRequests += api.RequestsPerHour
		totalResponseTime += api.ResponseTime
		if api.Status == "healthy" {
			healthyAPIs++
		}
	}

	avgResponseTime := 0.0
	if totalAPIs > 0 {
		avgResponseTime = totalResponseTime / float64(totalAPIs)
	}

	// Calculate circuit breaker summary
	openCircuits := 0
	activeCircuits := len(circuitBreakers)

	for _, cb := range circuitBreakers {
		if cb.State == "open" {
			openCircuits++
		}
	}

	// Calculate overall cache hit rate
	totalHits := cache.HotCache.HitCount + cache.WarmCache.HitCount
	totalRequests64 := totalHits + cache.HotCache.MissCount + cache.WarmCache.MissCount

	cacheHitRate := 0.0
	if totalRequests64 > 0 {
		cacheHitRate = float64(totalHits) / float64(totalRequests64)
	}

	// Determine overall health
	overallHealth := "healthy"
	if openCircuits > 0 || healthyAPIs < totalAPIs/2 {
		overallHealth = "critical"
	} else if cacheHitRate < 0.7 || latency.ScanLatencyP99 > 500 {
		overallHealth = "degraded"
	}

	// Determine system load based on latency and queue depth
	systemLoad := "low"
	if latency.AvgQueueDepth > 5 || latency.ScanLatencyP95 > 400 {
		systemLoad = "high"
	} else if latency.AvgQueueDepth > 2 || latency.ScanLatencyP95 > 250 {
		systemLoad = "medium"
	}

	return MetricsSummary{
		OverallHealth:    overallHealth,
		TotalAPIRequests: totalRequests,
		AvgResponseTime:  avgResponseTime,
		CacheHitRate:     cacheHitRate,
		ActiveCircuits:   activeCircuits,
		OpenCircuits:     openCircuits,
		AvgScanLatency:   latency.ScanLatencyP50,
		SystemLoad:       systemLoad,
	}
}
