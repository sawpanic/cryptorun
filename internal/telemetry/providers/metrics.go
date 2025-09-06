package providers

import (
	"fmt"
	"sync"
	"time"
)

// MetricsCollector collects and aggregates provider metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*ProviderMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*ProviderMetrics),
	}
}

// ProviderMetrics holds metrics for a single provider
type ProviderMetrics struct {
	Provider string `json:"provider"`

	// Request metrics
	TotalRequests      int64 `json:"total_requests"`
	SuccessfulRequests int64 `json:"successful_requests"`
	FailedRequests     int64 `json:"failed_requests"`
	TimeoutRequests    int64 `json:"timeout_requests"`
	CachedRequests     int64 `json:"cached_requests"`

	// Rate metrics
	RPS          float64 `json:"rps"`            // Current requests per second
	ErrorRate    float64 `json:"error_rate"`     // Percentage of failed requests
	CacheHitRate float64 `json:"cache_hit_rate"` // Percentage of cached responses

	// Latency metrics
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	P50LatencyMS float64 `json:"p50_latency_ms"`
	P95LatencyMS float64 `json:"p95_latency_ms"`
	P99LatencyMS float64 `json:"p99_latency_ms"`

	// Circuit breaker state
	CircuitState string `json:"circuit_state"` // "closed", "open", "half-open"

	// Budget utilization
	BudgetUsed        int64   `json:"budget_used"`
	BudgetLimit       int64   `json:"budget_limit"`
	BudgetUtilization float64 `json:"budget_utilization"` // Percentage

	// Time-series data (last 60 data points for sparkline)
	LatencyHistory []float64 `json:"latency_history,omitempty"`
	RPSHistory     []float64 `json:"rps_history,omitempty"`
	ErrorHistory   []float64 `json:"error_history,omitempty"`

	// Metadata
	LastUpdated   time.Time `json:"last_updated"`
	LastRequestAt time.Time `json:"last_request_at,omitempty"`
}

// RecordRequest records a successful request with latency
func (m *MetricsCollector) RecordRequest(provider string, latencyMS float64, cached bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(provider)
	metrics.TotalRequests++
	metrics.SuccessfulRequests++
	metrics.LastRequestAt = time.Now()
	metrics.LastUpdated = time.Now()

	if cached {
		metrics.CachedRequests++
	}

	// Update latency (simple moving average for now)
	if metrics.AvgLatencyMS == 0 {
		metrics.AvgLatencyMS = latencyMS
	} else {
		// Exponential moving average with alpha = 0.1
		metrics.AvgLatencyMS = 0.9*metrics.AvgLatencyMS + 0.1*latencyMS
	}

	// Add to latency history (keep last 60 points)
	metrics.LatencyHistory = append(metrics.LatencyHistory, latencyMS)
	if len(metrics.LatencyHistory) > 60 {
		metrics.LatencyHistory = metrics.LatencyHistory[1:]
	}

	m.updateRates(metrics)
}

// RecordError records a failed request
func (m *MetricsCollector) RecordError(provider string, errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(provider)
	metrics.TotalRequests++
	metrics.FailedRequests++

	if errorType == "timeout" {
		metrics.TimeoutRequests++
	}

	metrics.LastRequestAt = time.Now()
	metrics.LastUpdated = time.Now()

	m.updateRates(metrics)
}

// UpdateCircuitState updates the circuit breaker state for a provider
func (m *MetricsCollector) UpdateCircuitState(provider string, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(provider)
	metrics.CircuitState = state
	metrics.LastUpdated = time.Now()
}

// UpdateBudget updates the budget utilization for a provider
func (m *MetricsCollector) UpdateBudget(provider string, used, limit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := m.getOrCreateMetrics(provider)
	metrics.BudgetUsed = used
	metrics.BudgetLimit = limit

	if limit > 0 {
		metrics.BudgetUtilization = float64(used) / float64(limit) * 100
	}

	metrics.LastUpdated = time.Now()
}

// getOrCreateMetrics gets or creates metrics for a provider
func (m *MetricsCollector) getOrCreateMetrics(provider string) *ProviderMetrics {
	if metrics, exists := m.metrics[provider]; exists {
		return metrics
	}

	metrics := &ProviderMetrics{
		Provider:       provider,
		LatencyHistory: make([]float64, 0, 60),
		RPSHistory:     make([]float64, 0, 60),
		ErrorHistory:   make([]float64, 0, 60),
		CircuitState:   "closed",
		LastUpdated:    time.Now(),
	}

	m.metrics[provider] = metrics
	return metrics
}

// updateRates recalculates derived metrics
func (m *MetricsCollector) updateRates(metrics *ProviderMetrics) {
	// Update error rate
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests) * 100
	}

	// Update cache hit rate
	if metrics.TotalRequests > 0 {
		metrics.CacheHitRate = float64(metrics.CachedRequests) / float64(metrics.TotalRequests) * 100
	}

	// Calculate simple RPS (requests in last update period)
	// This is simplified - in practice you'd want a sliding window
	metrics.RPS = float64(metrics.TotalRequests) / time.Since(metrics.LastUpdated.Add(-time.Minute)).Seconds()
	if metrics.RPS < 0 {
		metrics.RPS = 0
	}

	// Add to rate histories
	metrics.RPSHistory = append(metrics.RPSHistory, metrics.RPS)
	if len(metrics.RPSHistory) > 60 {
		metrics.RPSHistory = metrics.RPSHistory[1:]
	}

	metrics.ErrorHistory = append(metrics.ErrorHistory, metrics.ErrorRate)
	if len(metrics.ErrorHistory) > 60 {
		metrics.ErrorHistory = metrics.ErrorHistory[1:]
	}
}

// GetMetrics returns metrics for a specific provider
func (m *MetricsCollector) GetMetrics(provider string) (*ProviderMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[provider]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	copy := *metrics
	return &copy, true
}

// GetAllMetrics returns metrics for all providers
func (m *MetricsCollector) GetAllMetrics() map[string]*ProviderMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ProviderMetrics)
	for provider, metrics := range m.metrics {
		copy := *metrics
		result[provider] = &copy
	}

	return result
}

// Reset clears all metrics
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make(map[string]*ProviderMetrics)
}

// GetSummary returns a summary of all provider metrics
func (m *MetricsCollector) GetSummary() Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := Summary{
		TotalProviders: len(m.metrics),
		Timestamp:      time.Now(),
	}

	var totalRequests, totalSuccesses, totalFailures, totalCached int64
	var totalLatency float64
	var healthyCount, unhealthyCount int

	for _, metrics := range m.metrics {
		totalRequests += metrics.TotalRequests
		totalSuccesses += metrics.SuccessfulRequests
		totalFailures += metrics.FailedRequests
		totalCached += metrics.CachedRequests
		totalLatency += metrics.AvgLatencyMS

		// Consider provider healthy if error rate < 10% and circuit closed
		if metrics.ErrorRate < 10.0 && metrics.CircuitState == "closed" {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	// Calculate aggregate metrics
	if len(m.metrics) > 0 {
		summary.AvgLatencyMS = totalLatency / float64(len(m.metrics))
	}

	if totalRequests > 0 {
		summary.OverallErrorRate = float64(totalFailures) / float64(totalRequests) * 100
		summary.OverallCacheHitRate = float64(totalCached) / float64(totalRequests) * 100
	}

	summary.TotalRequests = totalRequests
	summary.HealthyProviders = healthyCount
	summary.UnhealthyProviders = unhealthyCount

	return summary
}

// Summary represents aggregate metrics across all providers
type Summary struct {
	TotalProviders      int       `json:"total_providers"`
	HealthyProviders    int       `json:"healthy_providers"`
	UnhealthyProviders  int       `json:"unhealthy_providers"`
	TotalRequests       int64     `json:"total_requests"`
	OverallErrorRate    float64   `json:"overall_error_rate"`
	OverallCacheHitRate float64   `json:"overall_cache_hit_rate"`
	AvgLatencyMS        float64   `json:"avg_latency_ms"`
	Timestamp           time.Time `json:"timestamp"`
}

// IsHealthy returns true if the overall system is healthy
func (s *Summary) IsHealthy() bool {
	return s.OverallErrorRate < 10.0 && s.UnhealthyProviders == 0
}

// GetHealthPercentage returns the percentage of healthy providers
func (s *Summary) GetHealthPercentage() float64 {
	if s.TotalProviders == 0 {
		return 100.0
	}
	return float64(s.HealthyProviders) / float64(s.TotalProviders) * 100.0
}

// MetricsExporter exports metrics in various formats
type MetricsExporter struct {
	collector *MetricsCollector
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(collector *MetricsCollector) *MetricsExporter {
	return &MetricsExporter{
		collector: collector,
	}
}

// ExportPrometheus exports metrics in Prometheus format
func (e *MetricsExporter) ExportPrometheus() string {
	metrics := e.collector.GetAllMetrics()

	result := "# HELP provider_requests_total Total number of requests by provider\n"
	result += "# TYPE provider_requests_total counter\n"

	for provider, m := range metrics {
		result += fmt.Sprintf("provider_requests_total{provider=\"%s\"} %d\n", provider, m.TotalRequests)
	}

	result += "\n# HELP provider_error_rate Error rate by provider\n"
	result += "# TYPE provider_error_rate gauge\n"

	for provider, m := range metrics {
		result += fmt.Sprintf("provider_error_rate{provider=\"%s\"} %.2f\n", provider, m.ErrorRate)
	}

	result += "\n# HELP provider_circuit_state Circuit breaker state (0=closed, 1=half-open, 2=open)\n"
	result += "# TYPE provider_circuit_state gauge\n"

	for provider, m := range metrics {
		stateValue := 0
		switch m.CircuitState {
		case "half-open":
			stateValue = 1
		case "open":
			stateValue = 2
		}
		result += fmt.Sprintf("provider_circuit_state{provider=\"%s\"} %d\n", provider, stateValue)
	}

	result += "\n# HELP provider_budget_utilization Budget utilization percentage by provider\n"
	result += "# TYPE provider_budget_utilization gauge\n"

	for provider, m := range metrics {
		result += fmt.Sprintf("provider_budget_utilization{provider=\"%s\"} %.2f\n", provider, m.BudgetUtilization)
	}

	result += "\n# HELP provider_cache_hit_rate Cache hit rate by provider\n"
	result += "# TYPE provider_cache_hit_rate gauge\n"

	for provider, m := range metrics {
		result += fmt.Sprintf("provider_cache_hit_rate{provider=\"%s\"} %.2f\n", provider, m.CacheHitRate)
	}

	return result
}

// ExportJSON exports metrics as JSON
func (e *MetricsExporter) ExportJSON() map[string]interface{} {
	return map[string]interface{}{
		"providers": e.collector.GetAllMetrics(),
		"summary":   e.collector.GetSummary(),
		"timestamp": time.Now(),
	}
}
