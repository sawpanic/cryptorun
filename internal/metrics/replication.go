package metrics

import (
	"fmt"
	"sync"
	"time"
)

// ReplicationMetrics holds all replication-related metrics for Prometheus exposition
type ReplicationMetrics struct {
	// Replication lag by tier and region
	ReplicationLag *GaugeVec
	
	// Replication plan execution metrics  
	PlanStepsTotal *CounterVec
	StepFailuresTotal *CounterVec
	
	// Data consistency monitoring
	ConsistencyErrorsTotal *CounterVec
	QuarantineTotal *CounterVec
	
	// Cross-region health metrics
	RegionHealthScore *GaugeVec
	CrossRegionRTT *GaugeVec
	
	// Step duration histograms
	StepDurationSeconds *HistogramVec
	
	mu sync.RWMutex
}

// GaugeVec represents a gauge with label dimensions
type GaugeVec struct {
	name   string
	gauges map[string]*Gauge
	mu     sync.RWMutex
}

// CounterVec represents a counter with label dimensions  
type CounterVec struct {
	name     string
	counters map[string]*Counter
	mu       sync.RWMutex
}

// HistogramVec represents a histogram with label dimensions
type HistogramVec struct {
	name       string
	histograms map[string]*Histogram
	mu         sync.RWMutex
}

// NewGaugeVec creates a new gauge vector
func NewGaugeVec(name string) *GaugeVec {
	return &GaugeVec{
		name:   name,
		gauges: make(map[string]*Gauge),
	}
}

// NewCounterVec creates a new counter vector
func NewCounterVec(name string) *CounterVec {
	return &CounterVec{
		name:     name,
		counters: make(map[string]*Counter),
	}
}

// NewHistogramVec creates a new histogram vector
func NewHistogramVec(name string) *HistogramVec {
	return &HistogramVec{
		name:       name,
		histograms: make(map[string]*Histogram),
	}
}

// With returns a gauge with the specified labels
func (gv *GaugeVec) With(labels map[string]string) *Gauge {
	key := labelMapToKey(labels)
	
	gv.mu.RLock()
	gauge, exists := gv.gauges[key]
	gv.mu.RUnlock()
	
	if exists {
		return gauge
	}
	
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	// Check again after acquiring write lock
	if gauge, exists := gv.gauges[key]; exists {
		return gauge
	}
	
	gauge = NewGauge(fmt.Sprintf("%s{%s}", gv.name, key))
	gv.gauges[key] = gauge
	return gauge
}

// With returns a counter with the specified labels
func (cv *CounterVec) With(labels map[string]string) *Counter {
	key := labelMapToKey(labels)
	
	cv.mu.RLock()
	counter, exists := cv.counters[key]
	cv.mu.RUnlock()
	
	if exists {
		return counter
	}
	
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	// Check again after acquiring write lock  
	if counter, exists := cv.counters[key]; exists {
		return counter
	}
	
	counter = NewCounter(fmt.Sprintf("%s{%s}", cv.name, key))
	cv.counters[key] = counter
	return counter
}

// With returns a histogram with the specified labels
func (hv *HistogramVec) With(labels map[string]string) *Histogram {
	key := labelMapToKey(labels)
	
	hv.mu.RLock()
	histogram, exists := hv.histograms[key]
	hv.mu.RUnlock()
	
	if exists {
		return histogram
	}
	
	hv.mu.Lock()
	defer hv.mu.Unlock()
	
	// Check again after acquiring write lock
	if histogram, exists := hv.histograms[key]; exists {
		return histogram
	}
	
	histogram = NewHistogram(fmt.Sprintf("%s{%s}", hv.name, key))
	hv.histograms[key] = histogram
	return histogram
}

// labelMapToKey converts a label map to a consistent string key
func labelMapToKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	
	result := ""
	for k, v := range labels {
		if result != "" {
			result += ","
		}
		result += fmt.Sprintf("%s=%s", k, v)
	}
	return result
}

// NewReplicationMetrics creates a new set of replication metrics
func NewReplicationMetrics() *ReplicationMetrics {
	return &ReplicationMetrics{
		ReplicationLag: NewGaugeVec("cryptorun_replication_lag_seconds"),
		PlanStepsTotal: NewCounterVec("cryptorun_replication_plan_steps_total"),
		StepFailuresTotal: NewCounterVec("cryptorun_replication_step_failures_total"),
		ConsistencyErrorsTotal: NewCounterVec("cryptorun_data_consistency_errors_total"),
		QuarantineTotal: NewCounterVec("cryptorun_quarantine_total"),
		RegionHealthScore: NewGaugeVec("cryptorun_region_health_score"),
		CrossRegionRTT: NewGaugeVec("cryptorun_cross_region_rtt_seconds"),
		StepDurationSeconds: NewHistogramVec("cryptorun_replication_step_seconds"),
	}
}

// RecordReplicationLag records replication lag for a tier/region/source combination
func (rm *ReplicationMetrics) RecordReplicationLag(tier, region, source string, lagSeconds float64) {
	rm.ReplicationLag.With(map[string]string{
		"tier":   tier,
		"region": region,
		"source": source,
	}).Set(lagSeconds)
}

// RecordPlanStep records a replication plan step execution
func (rm *ReplicationMetrics) RecordPlanStep(tier, from, to string) {
	rm.PlanStepsTotal.With(map[string]string{
		"tier": tier,
		"from": from,
		"to":   to,
	}).Inc()
}

// RecordStepFailure records a replication step failure
func (rm *ReplicationMetrics) RecordStepFailure(tier, from, to, reason string) {
	rm.StepFailuresTotal.With(map[string]string{
		"tier":   tier,
		"from":   from,
		"to":     to,
		"reason": reason,
	}).Inc()
}

// RecordConsistencyError records a data consistency error
func (rm *ReplicationMetrics) RecordConsistencyError(check string) {
	rm.ConsistencyErrorsTotal.With(map[string]string{
		"check": check,
	}).Inc()
}

// RecordQuarantine records a data quarantine event
func (rm *ReplicationMetrics) RecordQuarantine(tier, region, kind string) {
	rm.QuarantineTotal.With(map[string]string{
		"tier":   tier,
		"region": region,
		"kind":   kind,
	}).Inc()
}

// RecordRegionHealth records the health score for a region
func (rm *ReplicationMetrics) RecordRegionHealth(region string, healthScore float64) {
	rm.RegionHealthScore.With(map[string]string{
		"region": region,
	}).Set(healthScore)
}

// RecordCrossRegionRTT records round-trip time between regions
func (rm *ReplicationMetrics) RecordCrossRegionRTT(from, to string, rttSeconds float64) {
	rm.CrossRegionRTT.With(map[string]string{
		"from": from,
		"to":   to,
	}).Set(rttSeconds)
}

// RecordStepDuration records the duration of a replication step
func (rm *ReplicationMetrics) RecordStepDuration(tier string, duration time.Duration) {
	rm.StepDurationSeconds.With(map[string]string{
		"tier": tier,
	}).Observe(duration.Seconds())
}

// GetAllMetrics returns all metrics for exposition (simplified for this implementation)
func (rm *ReplicationMetrics) GetAllMetrics() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	metrics := make(map[string]interface{})
	
	// In a real implementation, this would serialize all metrics in Prometheus format
	// For now, return a simplified representation
	metrics["replication_lag"] = rm.getGaugeVecValues(rm.ReplicationLag)
	metrics["plan_steps_total"] = rm.getCounterVecValues(rm.PlanStepsTotal)
	metrics["step_failures_total"] = rm.getCounterVecValues(rm.StepFailuresTotal)
	metrics["consistency_errors_total"] = rm.getCounterVecValues(rm.ConsistencyErrorsTotal)
	metrics["quarantine_total"] = rm.getCounterVecValues(rm.QuarantineTotal)
	metrics["region_health_score"] = rm.getGaugeVecValues(rm.RegionHealthScore)
	metrics["cross_region_rtt"] = rm.getGaugeVecValues(rm.CrossRegionRTT)
	
	return metrics
}

// getGaugeVecValues extracts values from a gauge vector
func (rm *ReplicationMetrics) getGaugeVecValues(gv *GaugeVec) map[string]float64 {
	gv.mu.RLock()
	defer gv.mu.RUnlock()
	
	values := make(map[string]float64)
	for key, gauge := range gv.gauges {
		values[key] = gauge.Get()
	}
	return values
}

// getCounterVecValues extracts values from a counter vector
func (rm *ReplicationMetrics) getCounterVecValues(cv *CounterVec) map[string]float64 {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	
	values := make(map[string]float64)
	for key, counter := range cv.counters {
		values[key] = counter.Get()
	}
	return values
}

// Reset resets all metrics (useful for testing)
func (rm *ReplicationMetrics) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.ReplicationLag = NewGaugeVec("cryptorun_replication_lag_seconds")
	rm.PlanStepsTotal = NewCounterVec("cryptorun_replication_plan_steps_total")
	rm.StepFailuresTotal = NewCounterVec("cryptorun_replication_step_failures_total")
	rm.ConsistencyErrorsTotal = NewCounterVec("cryptorun_data_consistency_errors_total")
	rm.QuarantineTotal = NewCounterVec("cryptorun_quarantine_total")
	rm.RegionHealthScore = NewGaugeVec("cryptorun_region_health_score")
	rm.CrossRegionRTT = NewGaugeVec("cryptorun_cross_region_rtt_seconds")
	rm.StepDurationSeconds = NewHistogramVec("cryptorun_replication_step_seconds")
}

// Global instance for easy access
var GlobalReplicationMetrics = NewReplicationMetrics()