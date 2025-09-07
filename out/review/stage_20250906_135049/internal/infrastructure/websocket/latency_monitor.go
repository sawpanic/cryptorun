package websocket

import (
	"fmt"
	"sync"
	"time"

	"cryptorun/internal/metrics"
)

// LatencyMonitor tracks end-to-end latency metrics for the hot set system
type LatencyMonitor struct {
	ingestLatency    *metrics.Histogram
	normalizeLatency *metrics.Histogram
	processLatency   *metrics.Histogram
	serveLatency     *metrics.Histogram
	e2eLatency       *metrics.Histogram
	
	freshnessGauge   *metrics.Gauge
	staleCounter     *metrics.Counter
	errorCounter     *metrics.Counter
	
	// Latency targets
	p99Target        float64
	freshnessTarget  time.Duration
	
	mu               sync.RWMutex
}

// LatencyProbe represents a timing measurement point
type LatencyProbe struct {
	Symbol         string
	StartTime      time.Time
	IngestTime     time.Time
	NormalizeTime  time.Time
	ProcessTime    time.Time
	ServeTime      time.Time
	EndTime        time.Time
}

// NewLatencyMonitor creates a new latency monitor
func NewLatencyMonitor() *LatencyMonitor {
	return &LatencyMonitor{
		ingestLatency:    metrics.NewHistogram("hotset_ingest_latency_ms"),
		normalizeLatency: metrics.NewHistogram("hotset_normalize_latency_ms"),
		processLatency:   metrics.NewHistogram("hotset_process_latency_ms"),
		serveLatency:     metrics.NewHistogram("hotset_serve_latency_ms"),
		e2eLatency:       metrics.NewHistogram("hotset_e2e_latency_ms"),
		
		freshnessGauge:   metrics.NewGauge("hotset_freshness_seconds"),
		staleCounter:     metrics.NewCounter("hotset_stale_total"),
		errorCounter:     metrics.NewCounter("hotset_latency_errors_total"),
		
		p99Target:        300.0, // 300ms P99 target
		freshnessTarget:  5 * time.Second,
	}
}

// StartProbe begins a new latency measurement
func (lm *LatencyMonitor) StartProbe(symbol string) *LatencyProbe {
	now := time.Now()
	return &LatencyProbe{
		Symbol:    symbol,
		StartTime: now,
	}
}

// RecordIngest records when message ingestion completed
func (probe *LatencyProbe) RecordIngest() {
	probe.IngestTime = time.Now()
}

// RecordNormalize records when message normalization completed
func (probe *LatencyProbe) RecordNormalize() {
	probe.NormalizeTime = time.Now()
}

// RecordProcess records when microstructure processing completed
func (probe *LatencyProbe) RecordProcess() {
	probe.ProcessTime = time.Now()
}

// RecordServe records when distribution to subscribers completed
func (probe *LatencyProbe) RecordServe() {
	probe.ServeTime = time.Now()
}

// Finish completes the probe and records all metrics
func (lm *LatencyMonitor) Finish(probe *LatencyProbe) {
	probe.EndTime = time.Now()
	
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	// Calculate latencies in milliseconds
	if !probe.IngestTime.IsZero() {
		ingestLatency := float64(probe.IngestTime.Sub(probe.StartTime).Nanoseconds()) / 1e6
		lm.ingestLatency.Observe(ingestLatency)
	}
	
	if !probe.NormalizeTime.IsZero() && !probe.IngestTime.IsZero() {
		normalizeLatency := float64(probe.NormalizeTime.Sub(probe.IngestTime).Nanoseconds()) / 1e6
		lm.normalizeLatency.Observe(normalizeLatency)
	}
	
	if !probe.ProcessTime.IsZero() && !probe.NormalizeTime.IsZero() {
		processLatency := float64(probe.ProcessTime.Sub(probe.NormalizeTime).Nanoseconds()) / 1e6
		lm.processLatency.Observe(processLatency)
	}
	
	if !probe.ServeTime.IsZero() && !probe.ProcessTime.IsZero() {
		serveLatency := float64(probe.ServeTime.Sub(probe.ProcessTime).Nanoseconds()) / 1e6
		lm.serveLatency.Observe(serveLatency)
	}
	
	// End-to-end latency
	e2eLatency := float64(probe.EndTime.Sub(probe.StartTime).Nanoseconds()) / 1e6
	lm.e2eLatency.Observe(e2eLatency)
	
	// Update freshness gauge
	freshness := float64(time.Since(probe.StartTime).Seconds())
	lm.freshnessGauge.Set(freshness)
	
	// Check for stale data
	if time.Since(probe.StartTime) > lm.freshnessTarget {
		lm.staleCounter.Inc()
	}
}

// GetP99Latency returns the P99 latency for end-to-end processing
func (lm *LatencyMonitor) GetP99Latency() float64 {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	return lm.e2eLatency.Quantile(0.99)
}

// GetLatencyBreakdown returns latency histograms for each stage
func (lm *LatencyMonitor) GetLatencyBreakdown() map[string]*metrics.Histogram {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	return map[string]*metrics.Histogram{
		"ingest":    lm.ingestLatency,
		"normalize": lm.normalizeLatency,
		"process":   lm.processLatency,
		"serve":     lm.serveLatency,
		"e2e":       lm.e2eLatency,
	}
}

// IsP99TargetMet returns true if P99 latency is below target
func (lm *LatencyMonitor) IsP99TargetMet() bool {
	return lm.GetP99Latency() < lm.p99Target
}

// GetMetricsSummary returns a summary of latency metrics
func (lm *LatencyMonitor) GetMetricsSummary() LatencyMetricsSummary {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	return LatencyMetricsSummary{
		P50E2E:         lm.e2eLatency.Quantile(0.50),
		P95E2E:         lm.e2eLatency.Quantile(0.95),
		P99E2E:         lm.e2eLatency.Quantile(0.99),
		P99Target:      lm.p99Target,
		P99TargetMet:   lm.GetP99Latency() < lm.p99Target,
		
		P99Ingest:      lm.ingestLatency.Quantile(0.99),
		P99Normalize:   lm.normalizeLatency.Quantile(0.99),
		P99Process:     lm.processLatency.Quantile(0.99),
		P99Serve:       lm.serveLatency.Quantile(0.99),
		
		CurrentFreshness: time.Duration(lm.freshnessGauge.Get()) * time.Second,
		FreshnessTarget:  lm.freshnessTarget,
		FreshnessOK:      time.Duration(lm.freshnessGauge.Get())*time.Second < lm.freshnessTarget,
		
		StaleCount:       lm.staleCounter.Get(),
		ErrorCount:       lm.errorCounter.Get(),
	}
}

// LatencyMetricsSummary provides a summary of latency performance
type LatencyMetricsSummary struct {
	P50E2E         float64       `json:"p50_e2e_ms"`
	P95E2E         float64       `json:"p95_e2e_ms"`
	P99E2E         float64       `json:"p99_e2e_ms"`
	P99Target      float64       `json:"p99_target_ms"`
	P99TargetMet   bool          `json:"p99_target_met"`
	
	P99Ingest      float64       `json:"p99_ingest_ms"`
	P99Normalize   float64       `json:"p99_normalize_ms"`
	P99Process     float64       `json:"p99_process_ms"`
	P99Serve       float64       `json:"p99_serve_ms"`
	
	CurrentFreshness time.Duration `json:"current_freshness"`
	FreshnessTarget  time.Duration `json:"freshness_target"`
	FreshnessOK      bool          `json:"freshness_ok"`
	
	StaleCount       float64       `json:"stale_count"`
	ErrorCount       float64       `json:"error_count"`
}

// String provides a human-readable summary
func (lms LatencyMetricsSummary) String() string {
	status := "✅"
	if !lms.P99TargetMet {
		status = "❌"
	}
	
	return fmt.Sprintf("Latency %s P99: %.1fms (target: %.1fms) | Freshness: %v | Stale: %.0f | Errors: %.0f",
		status, lms.P99E2E, lms.P99Target, lms.CurrentFreshness, lms.StaleCount, lms.ErrorCount)
}