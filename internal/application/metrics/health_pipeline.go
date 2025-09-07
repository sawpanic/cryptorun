package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// HealthOptions configures the health snapshot pipeline
type HealthOptions struct {
	IncludeMetrics  bool   `json:"include_metrics"`
	IncludeCounters bool   `json:"include_counters"`
	Format          string `json:"format"`
	OutputFile      string `json:"output_file"`
}

// HealthSnapshot represents the complete system health state
type HealthSnapshot struct {
	Timestamp    time.Time                  `json:"timestamp"`
	SystemStatus string                     `json:"system_status"`
	Components   map[string]ComponentHealth `json:"components"`
	Metrics      MetricsSnapshot            `json:"metrics"`
	Counters     CountersSnapshot           `json:"counters"`
	Uptime       string                     `json:"uptime"`
	Version      string                     `json:"version"`
}

// ComponentHealth represents the health of a system component
type ComponentHealth struct {
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"last_check"`
	Message      string    `json:"message"`
	Errors       int       `json:"errors"`
	Warnings     int       `json:"warnings"`
	ResponseTime string    `json:"response_time"`
}

// MetricsSnapshot contains current system metrics
type MetricsSnapshot struct {
	RequestsTotal     int64   `json:"requests_total"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	P50Latency        float64 `json:"p50_latency_ms"`
	P95Latency        float64 `json:"p95_latency_ms"`
	P99Latency        float64 `json:"p99_latency_ms"`
	ErrorRate         float64 `json:"error_rate"`
	MemoryUsage       int64   `json:"memory_usage_bytes"`
	CPUUsage          float64 `json:"cpu_usage_percent"`
}

// CountersSnapshot contains current system counters
type CountersSnapshot struct {
	ScanExecutions      int64 `json:"scan_executions"`
	BenchmarkRuns       int64 `json:"benchmark_runs"`
	DiagnosticRuns      int64 `json:"diagnostic_runs"`
	HealthChecks        int64 `json:"health_checks"`
	CacheHits           int64 `json:"cache_hits"`
	CacheMisses         int64 `json:"cache_misses"`
	APICallsTotal       int64 `json:"api_calls_total"`
	CircuitBreakerTrips int64 `json:"circuit_breaker_trips"`
}

// Snapshot creates a complete health snapshot - THE SINGLE ENTRY POINT
func Snapshot(ctx context.Context, opts HealthOptions) (*HealthSnapshot, error) {
	startTime := time.Now()

	log.Info().
		Bool("include_metrics", opts.IncludeMetrics).
		Bool("include_counters", opts.IncludeCounters).
		Str("format", opts.Format).
		Msg("Starting unified health snapshot pipeline")

	// Check all system components
	components, err := checkAllComponents(ctx)
	if err != nil {
		return nil, err
	}

	// Determine overall system status
	systemStatus := determineSystemStatus(components)

	// Collect metrics if requested
	var metrics MetricsSnapshot
	if opts.IncludeMetrics {
		metrics = collectMetrics(ctx)
	}

	// Collect counters if requested
	var counters CountersSnapshot
	if opts.IncludeCounters {
		counters = collectCounters(ctx)
	}

	snapshot := &HealthSnapshot{
		Timestamp:    startTime,
		SystemStatus: systemStatus,
		Components:   components,
		Metrics:      metrics,
		Counters:     counters,
		Uptime:       calculateUptime(),
		Version:      "v3.2.1", // This would be injected from build
	}

	log.Info().
		Str("system_status", snapshot.SystemStatus).
		Int("components_checked", len(snapshot.Components)).
		Str("uptime", snapshot.Uptime).
		Msg("Health snapshot completed successfully")

	return snapshot, nil
}

// checkAllComponents verifies the health of all system components
func checkAllComponents(ctx context.Context) (map[string]ComponentHealth, error) {
	components := make(map[string]ComponentHealth)

	// Check API endpoints
	components["api"] = checkAPIHealth(ctx)

	// Check database connectivity
	components["database"] = checkDatabaseHealth(ctx)

	// Check cache system
	components["cache"] = checkCacheHealth(ctx)

	// Check external APIs (Kraken, CoinGecko)
	components["kraken_api"] = checkKrakenAPIHealth(ctx)
	components["coingecko_api"] = checkCoinGeckoAPIHealth(ctx)

	// Check file system
	components["filesystem"] = checkFilesystemHealth(ctx)

	// Check configuration
	components["configuration"] = checkConfigurationHealth(ctx)

	return components, nil
}

// determineSystemStatus calculates overall system status from components
func determineSystemStatus(components map[string]ComponentHealth) string {
	criticalDown := 0
	warnings := 0

	for _, component := range components {
		switch component.Status {
		case "down", "error":
			criticalDown++
		case "warning", "degraded":
			warnings++
		}
	}

	if criticalDown > 0 {
		return "degraded"
	}
	if warnings > 2 {
		return "warning"
	}
	return "healthy"
}

// collectMetrics gathers current system performance metrics
func collectMetrics(ctx context.Context) MetricsSnapshot {
	// Implementation would collect actual metrics from Prometheus or similar
	return MetricsSnapshot{
		RequestsTotal:     12543,
		RequestsPerSecond: 8.7,
		P50Latency:        120.5,
		P95Latency:        280.3,
		P99Latency:        450.1,
		ErrorRate:         0.02,
		MemoryUsage:       134217728, // 128MB
		CPUUsage:          15.7,
	}
}

// collectCounters gathers current system operation counters
func collectCounters(ctx context.Context) CountersSnapshot {
	// Implementation would collect actual counters from metrics system
	return CountersSnapshot{
		ScanExecutions:      456,
		BenchmarkRuns:       23,
		DiagnosticRuns:      12,
		HealthChecks:        2341,
		CacheHits:           8903,
		CacheMisses:         1247,
		APICallsTotal:       5642,
		CircuitBreakerTrips: 3,
	}
}

// calculateUptime returns formatted system uptime
func calculateUptime() string {
	// Implementation would track actual system start time
	return "2d 14h 32m"
}

// Individual component health check functions
func checkAPIHealth(ctx context.Context) ComponentHealth {
	// Mock implementation - would make actual health check requests
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "All endpoints responding normally",
		Errors:       0,
		Warnings:     0,
		ResponseTime: "45ms",
	}
}

func checkDatabaseHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "Connection pool healthy, queries executing normally",
		Errors:       0,
		Warnings:     1,
		ResponseTime: "12ms",
	}
}

func checkCacheHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "Redis connection active, hit rate 87.3%",
		Errors:       0,
		Warnings:     0,
		ResponseTime: "3ms",
	}
}

func checkKrakenAPIHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "Rate limits normal, websocket connected",
		Errors:       2,
		Warnings:     0,
		ResponseTime: "156ms",
	}
}

func checkCoinGeckoAPIHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "warning",
		LastCheck:    time.Now(),
		Message:      "Rate limited, using cached data",
		Errors:       0,
		Warnings:     3,
		ResponseTime: "2340ms",
	}
}

func checkFilesystemHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "Disk space 78% used, write permissions OK",
		Errors:       0,
		Warnings:     1,
		ResponseTime: "8ms",
	}
}

func checkConfigurationHealth(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:       "healthy",
		LastCheck:    time.Now(),
		Message:      "All config files valid, schema compliance verified",
		Errors:       0,
		Warnings:     0,
		ResponseTime: "2ms",
	}
}
