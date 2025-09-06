package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ProviderHealth tracks health metrics for API providers
type ProviderHealth struct {
	providerName    string
	mu              sync.RWMutex
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	lastSuccess     time.Time
	lastFailure     time.Time
	latencies       []time.Duration
	maxLatencies    int
	degraded        bool
	degradedReason  string
	budgetRemaining int
	budgetTotal     int
}

// HealthStatus represents the current health status
type HealthStatus struct {
	ProviderName    string        `json:"provider_name"`
	IsHealthy       bool          `json:"is_healthy"`
	SuccessRate     float64       `json:"success_rate"`
	P50Latency      time.Duration `json:"p50_latency_ms"`
	P95Latency      time.Duration `json:"p95_latency_ms"`
	BudgetRemaining int           `json:"budget_remaining"`
	BudgetUsedPct   float64       `json:"budget_used_pct"`
	Degraded        bool          `json:"degraded"`
	DegradedReason  string        `json:"degraded_reason,omitempty"`
	TotalRequests   int64         `json:"total_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	LastSuccess     time.Time     `json:"last_success"`
	LastFailure     time.Time     `json:"last_failure"`
	UptimePct       float64       `json:"uptime_pct"`
}

// Final stable Prometheus metrics for provider health
var (
	providerHealthSuccessRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "provider_health_success_rate",
			Help: "Provider success rate over the last measurement window (0.0-1.0)",
		},
		[]string{"provider", "venue"},
	)

	providerHealthLatencyP50 = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "provider_health_latency_p50",
			Help: "Provider 50th percentile latency in milliseconds",
		},
		[]string{"provider", "venue"},
	)

	providerHealthLatencyP95 = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "provider_health_latency_p95",
			Help: "Provider 95th percentile latency in milliseconds",
		},
		[]string{"provider", "venue"},
	)

	providerHealthBudgetRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "provider_health_budget_remaining",
			Help: "Provider remaining budget/quota as percentage (0.0-1.0)",
		},
		[]string{"provider", "venue"},
	)

	providerHealthDegraded = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "provider_health_degraded",
			Help: "Provider degraded status (1=degraded, 0=healthy) with reason",
		},
		[]string{"provider", "venue", "reason"},
	)
)

// NewProviderHealth creates a new provider health tracker
func NewProviderHealth(providerName string) *ProviderHealth {
	return &ProviderHealth{
		providerName:    providerName,
		latencies:       make([]time.Duration, 0, 1000),
		maxLatencies:    1000,
		budgetTotal:     100, // Default budget
		budgetRemaining: 100,
	}
}

// RecordRequest records the result and latency of an API request
func (ph *ProviderHealth) RecordRequest(success bool, latency time.Duration) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	ph.totalRequests++

	if success {
		ph.successRequests++
		ph.lastSuccess = time.Now()
	} else {
		ph.failedRequests++
		ph.lastFailure = time.Now()
	}

	// Record latency
	ph.latencies = append(ph.latencies, latency)

	// Keep only recent latencies to avoid memory bloat
	if len(ph.latencies) > ph.maxLatencies {
		// Keep the most recent 80% of latencies
		keepCount := int(float64(ph.maxLatencies) * 0.8)
		copy(ph.latencies, ph.latencies[len(ph.latencies)-keepCount:])
		ph.latencies = ph.latencies[:keepCount]
	}
}

// SetDegraded marks the provider as degraded
func (ph *ProviderHealth) SetDegraded(degraded bool, reason string) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	ph.degraded = degraded
	ph.degradedReason = reason
}

// SetBudget sets the budget information
func (ph *ProviderHealth) SetBudget(remaining, total int) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	ph.budgetRemaining = remaining
	ph.budgetTotal = total
}

// IsHealthy returns true if the provider is considered healthy
func (ph *ProviderHealth) IsHealthy() bool {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	if ph.degraded {
		return false
	}

	// No requests yet - consider healthy
	if ph.totalRequests == 0 {
		return true
	}

	// Check success rate (must be > 90%)
	successRate := float64(ph.successRequests) / float64(ph.totalRequests)
	if successRate < 0.9 {
		return false
	}

	// Check if we had any success in the last 5 minutes
	if ph.lastSuccess.Before(time.Now().Add(-5 * time.Minute)) {
		return false
	}

	// Check budget (must have > 10% remaining)
	if ph.budgetTotal > 0 {
		budgetUsedPct := float64(ph.budgetTotal-ph.budgetRemaining) / float64(ph.budgetTotal)
		if budgetUsedPct > 0.9 {
			return false
		}
	}

	return true
}

// GetStatus returns the current health status
func (ph *ProviderHealth) GetStatus() HealthStatus {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	status := HealthStatus{
		ProviderName:    ph.providerName,
		IsHealthy:       ph.IsHealthy(),
		Degraded:        ph.degraded,
		DegradedReason:  ph.degradedReason,
		TotalRequests:   ph.totalRequests,
		FailedRequests:  ph.failedRequests,
		LastSuccess:     ph.lastSuccess,
		LastFailure:     ph.lastFailure,
		BudgetRemaining: ph.budgetRemaining,
	}

	// Calculate success rate
	if ph.totalRequests > 0 {
		status.SuccessRate = float64(ph.successRequests) / float64(ph.totalRequests)
	}

	// Calculate budget usage
	if ph.budgetTotal > 0 {
		status.BudgetUsedPct = float64(ph.budgetTotal-ph.budgetRemaining) / float64(ph.budgetTotal)
	}

	// Calculate uptime percentage (simplified)
	if ph.totalRequests > 0 {
		status.UptimePct = status.SuccessRate
	} else {
		status.UptimePct = 1.0 // No requests yet
	}

	// Calculate latency percentiles
	if len(ph.latencies) > 0 {
		sorted := make([]time.Duration, len(ph.latencies))
		copy(sorted, ph.latencies)
		ph.sortDurations(sorted)

		p50Index := int(float64(len(sorted)) * 0.5)
		p95Index := int(float64(len(sorted)) * 0.95)

		if p50Index < len(sorted) {
			status.P50Latency = sorted[p50Index]
		}
		if p95Index < len(sorted) {
			status.P95Latency = sorted[p95Index]
		}
	}

	return status
}

// sortDurations sorts a slice of durations (simple bubble sort for small slices)
func (ph *ProviderHealth) sortDurations(durations []time.Duration) {
	n := len(durations)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if durations[j] > durations[j+1] {
				durations[j], durations[j+1] = durations[j+1], durations[j]
			}
		}
	}
}

// MetricsRegistry manages multiple provider health trackers
type MetricsRegistry struct {
	providers map[string]*ProviderHealth
	mu        sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		providers: make(map[string]*ProviderHealth),
	}
}

// RegisterProvider registers a new provider health tracker
func (mr *MetricsRegistry) RegisterProvider(name string, health *ProviderHealth) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.providers[name] = health
}

// GetProvider returns a provider health tracker
func (mr *MetricsRegistry) GetProvider(name string) *ProviderHealth {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return mr.providers[name]
}

// GetAllStatuses returns health status for all providers
func (mr *MetricsRegistry) GetAllStatuses() map[string]HealthStatus {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	statuses := make(map[string]HealthStatus)
	for name, health := range mr.providers {
		statuses[name] = health.GetStatus()
	}

	return statuses
}

// GetHealthyCount returns the number of healthy providers
func (mr *MetricsRegistry) GetHealthyCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	count := 0
	for _, health := range mr.providers {
		if health.IsHealthy() {
			count++
		}
	}

	return count
}

// MetricNames defines the stable metric names for acceptance validation
var MetricNames = struct {
	SuccessRate     string
	LatencyP50      string
	LatencyP95      string
	BudgetRemaining string
	Degraded        string
}{
	SuccessRate:     "provider_health_success_rate",
	LatencyP50:      "provider_health_latency_p50",
	LatencyP95:      "provider_health_latency_p95",
	BudgetRemaining: "provider_health_budget_remaining",
	Degraded:        "provider_health_degraded",
}

// MetricExport represents a metric for export to monitoring systems
type MetricExport struct {
	Name   string            `json:"name"`
	Value  interface{}       `json:"value"`
	Labels map[string]string `json:"labels"`
	Type   string            `json:"type"` // gauge, counter, histogram
}

// ExportMetrics returns all provider health metrics in standardized format for monitoring
func (mr *MetricsRegistry) ExportMetrics() []MetricExport {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var exports []MetricExport

	for providerName, health := range mr.providers {
		status := health.GetStatus()
		labels := map[string]string{"provider": providerName}

		exports = append(exports, []MetricExport{
			{
				Name:   MetricNames.SuccessRate,
				Value:  status.SuccessRate,
				Labels: labels,
				Type:   "gauge",
			},
			{
				Name:   MetricNames.LatencyP50,
				Value:  status.P50Latency.Milliseconds(),
				Labels: labels,
				Type:   "gauge",
			},
			{
				Name:   MetricNames.LatencyP95,
				Value:  status.P95Latency.Milliseconds(),
				Labels: labels,
				Type:   "gauge",
			},
			{
				Name:   MetricNames.BudgetRemaining,
				Value:  status.BudgetRemaining,
				Labels: labels,
				Type:   "gauge",
			},
			{
				Name:   MetricNames.Degraded,
				Value:  boolToFloat(status.Degraded),
				Labels: labels,
				Type:   "gauge",
			},
		}...)
	}

	return exports
}

// GetPrometheusMetrics returns metrics in Prometheus exposition format
func (mr *MetricsRegistry) GetPrometheusMetrics() string {
	exports := mr.ExportMetrics()
	var lines []string

	for _, export := range exports {
		labelPairs := make([]string, 0, len(export.Labels))
		for key, value := range export.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, key, value))
		}

		labelStr := ""
		if len(labelPairs) > 0 {
			labelStr = "{" + fmt.Sprintf("%v", labelPairs) + "}"
		}

		line := fmt.Sprintf("%s%s %v", export.Name, labelStr, export.Value)
		lines = append(lines, line)
	}

	return fmt.Sprintf("%v", lines)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
