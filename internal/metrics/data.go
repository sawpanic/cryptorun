package metrics

import (
	"fmt"
	"strings"
	"sync"
)

// DataMetrics holds all data-related metrics for replication and validation
type DataMetrics struct {
	// Replication metrics
	replicationLag          map[string]*Gauge
	replicationPlanSteps    map[string]*Counter
	replicationStepFailures map[string]*Counter
	replicationStepDuration map[string]*Histogram
	
	// Data consistency metrics
	dataConsistencyErrors map[string]*Counter
	quarantineCount       map[string]*Counter
	
	// Regional health metrics
	regionHealth        map[string]*Gauge
	crossRegionRTT      map[string]*Gauge
	
	// Cache for metric instances to avoid recreation
	mu sync.RWMutex
}

// NewDataMetrics creates a new instance of data metrics
func NewDataMetrics() *DataMetrics {
	return &DataMetrics{
		replicationLag:          make(map[string]*Gauge),
		replicationPlanSteps:    make(map[string]*Counter),
		replicationStepFailures: make(map[string]*Counter),
		replicationStepDuration: make(map[string]*Histogram),
		dataConsistencyErrors:   make(map[string]*Counter),
		quarantineCount:        make(map[string]*Counter),
		regionHealth:           make(map[string]*Gauge),
		crossRegionRTT:         make(map[string]*Gauge),
	}
}

// buildKey creates a metric key from labels
func buildKey(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

// RecordReplicationLag records replication lag for a tier/region/source combination
func (dm *DataMetrics) RecordReplicationLag(tier, region, source string, lagSeconds float64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"region": region,
		"source": source,
	})
	
	if gauge, exists := dm.replicationLag[key]; exists {
		gauge.Set(lagSeconds)
	} else {
		gauge := NewGauge(fmt.Sprintf("cryptorun_replication_lag_seconds{tier=\"%s\",region=\"%s\",source=\"%s\"}", tier, region, source))
		gauge.Set(lagSeconds)
		dm.replicationLag[key] = gauge
	}
}

// IncrementPlanSteps increments the counter for replication plan steps
func (dm *DataMetrics) IncrementPlanSteps(tier, from, to string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := buildKey(map[string]string{
		"tier": tier,
		"from": from,
		"to":   to,
	})
	
	if counter, exists := dm.replicationPlanSteps[key]; exists {
		counter.Inc()
	} else {
		counter := NewCounter(fmt.Sprintf("cryptorun_replication_plan_steps_total{tier=\"%s\",from=\"%s\",to=\"%s\"}", tier, from, to))
		counter.Inc()
		dm.replicationPlanSteps[key] = counter
	}
}

// IncrementStepFailures increments the counter for replication step failures
func (dm *DataMetrics) IncrementStepFailures(tier, from, to, reason string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"from":   from,
		"to":     to,
		"reason": reason,
	})
	
	if counter, exists := dm.replicationStepFailures[key]; exists {
		counter.Inc()
	} else {
		counter := NewCounter(fmt.Sprintf("cryptorun_replication_step_failures_total{tier=\"%s\",from=\"%s\",to=\"%s\",reason=\"%s\"}", tier, from, to, reason))
		counter.Inc()
		dm.replicationStepFailures[key] = counter
	}
}

// RecordStepDuration records the duration of a replication step
func (dm *DataMetrics) RecordStepDuration(tier string, durationSeconds float64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := fmt.Sprintf("tier=%s", tier)
	
	if histogram, exists := dm.replicationStepDuration[key]; exists {
		histogram.Observe(durationSeconds)
	} else {
		histogram := NewHistogram(fmt.Sprintf("cryptorun_replication_step_seconds{tier=\"%s\"}", tier))
		histogram.Observe(durationSeconds)
		dm.replicationStepDuration[key] = histogram
	}
}

// IncrementConsistencyErrors increments data consistency error counter
func (dm *DataMetrics) IncrementConsistencyErrors(checkType string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := fmt.Sprintf("check=%s", checkType)
	
	if counter, exists := dm.dataConsistencyErrors[key]; exists {
		counter.Inc()
	} else {
		counter := NewCounter(fmt.Sprintf("cryptorun_data_consistency_errors_total{check=\"%s\"}", checkType))
		counter.Inc()
		dm.dataConsistencyErrors[key] = counter
	}
}

// IncrementQuarantine increments quarantine counter
func (dm *DataMetrics) IncrementQuarantine(tier, region, kind string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"region": region,
		"kind":   kind,
	})
	
	if counter, exists := dm.quarantineCount[key]; exists {
		counter.Inc()
	} else {
		counter := NewCounter(fmt.Sprintf("cryptorun_quarantine_total{tier=\"%s\",region=\"%s\",kind=\"%s\"}", tier, region, kind))
		counter.Inc()
		dm.quarantineCount[key] = counter
	}
}

// SetRegionHealth sets the health score for a region (0.0-1.0)
func (dm *DataMetrics) SetRegionHealth(region string, healthScore float64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := fmt.Sprintf("region=%s", region)
	
	// Clamp health score to valid range
	if healthScore < 0.0 {
		healthScore = 0.0
	} else if healthScore > 1.0 {
		healthScore = 1.0
	}
	
	if gauge, exists := dm.regionHealth[key]; exists {
		gauge.Set(healthScore)
	} else {
		gauge := NewGauge(fmt.Sprintf("cryptorun_region_health_score{region=\"%s\"}", region))
		gauge.Set(healthScore)
		dm.regionHealth[key] = gauge
	}
}

// RecordCrossRegionRTT records round-trip time between regions
func (dm *DataMetrics) RecordCrossRegionRTT(from, to string, rttSeconds float64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	key := buildKey(map[string]string{
		"from": from,
		"to":   to,
	})
	
	if gauge, exists := dm.crossRegionRTT[key]; exists {
		gauge.Set(rttSeconds)
	} else {
		gauge := NewGauge(fmt.Sprintf("cryptorun_cross_region_rtt_seconds{from=\"%s\",to=\"%s\"}", from, to))
		gauge.Set(rttSeconds)
		dm.crossRegionRTT[key] = gauge
	}
}

// GetReplicationLag returns the current replication lag for a tier/region/source
func (dm *DataMetrics) GetReplicationLag(tier, region, source string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"region": region,
		"source": source,
	})
	
	if gauge, exists := dm.replicationLag[key]; exists {
		return gauge.Get()
	}
	return 0.0
}

// GetPlanStepsCount returns the total plan steps count for a tier/from/to combination
func (dm *DataMetrics) GetPlanStepsCount(tier, from, to string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := buildKey(map[string]string{
		"tier": tier,
		"from": from,
		"to":   to,
	})
	
	if counter, exists := dm.replicationPlanSteps[key]; exists {
		return counter.Get()
	}
	return 0.0
}

// GetStepFailuresCount returns the total step failures count for a tier/from/to/reason combination
func (dm *DataMetrics) GetStepFailuresCount(tier, from, to, reason string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"from":   from,
		"to":     to,
		"reason": reason,
	})
	
	if counter, exists := dm.replicationStepFailures[key]; exists {
		return counter.Get()
	}
	return 0.0
}

// GetStepDurationP99 returns the 99th percentile duration for a tier
func (dm *DataMetrics) GetStepDurationP99(tier string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := fmt.Sprintf("tier=%s", tier)
	
	if histogram, exists := dm.replicationStepDuration[key]; exists {
		return histogram.Quantile(0.99)
	}
	return 0.0
}

// GetConsistencyErrorsCount returns the total consistency errors count for a check type
func (dm *DataMetrics) GetConsistencyErrorsCount(checkType string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := fmt.Sprintf("check=%s", checkType)
	
	if counter, exists := dm.dataConsistencyErrors[key]; exists {
		return counter.Get()
	}
	return 0.0
}

// GetQuarantineCount returns the total quarantine count for a tier/region/kind combination
func (dm *DataMetrics) GetQuarantineCount(tier, region, kind string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := buildKey(map[string]string{
		"tier":   tier,
		"region": region,
		"kind":   kind,
	})
	
	if counter, exists := dm.quarantineCount[key]; exists {
		return counter.Get()
	}
	return 0.0
}

// GetRegionHealth returns the health score for a region
func (dm *DataMetrics) GetRegionHealth(region string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := fmt.Sprintf("region=%s", region)
	
	if gauge, exists := dm.regionHealth[key]; exists {
		return gauge.Get()
	}
	return 1.0 // Default to healthy if not set
}

// GetCrossRegionRTT returns the RTT between two regions
func (dm *DataMetrics) GetCrossRegionRTT(from, to string) float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	key := buildKey(map[string]string{
		"from": from,
		"to":   to,
	})
	
	if gauge, exists := dm.crossRegionRTT[key]; exists {
		return gauge.Get()
	}
	return 0.0
}

// GetAllMetrics returns a snapshot of all current metrics
func (dm *DataMetrics) GetAllMetrics() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	result := make(map[string]interface{})
	
	// Replication lag metrics
	lagMetrics := make(map[string]float64)
	for key, gauge := range dm.replicationLag {
		lagMetrics[key] = gauge.Get()
	}
	result["replication_lag"] = lagMetrics
	
	// Plan steps metrics
	stepsMetrics := make(map[string]float64)
	for key, counter := range dm.replicationPlanSteps {
		stepsMetrics[key] = counter.Get()
	}
	result["plan_steps"] = stepsMetrics
	
	// Step failures metrics
	failuresMetrics := make(map[string]float64)
	for key, counter := range dm.replicationStepFailures {
		failuresMetrics[key] = counter.Get()
	}
	result["step_failures"] = failuresMetrics
	
	// Consistency errors metrics
	consistencyMetrics := make(map[string]float64)
	for key, counter := range dm.dataConsistencyErrors {
		consistencyMetrics[key] = counter.Get()
	}
	result["consistency_errors"] = consistencyMetrics
	
	// Quarantine metrics
	quarantineMetrics := make(map[string]float64)
	for key, counter := range dm.quarantineCount {
		quarantineMetrics[key] = counter.Get()
	}
	result["quarantine_count"] = quarantineMetrics
	
	// Region health metrics
	healthMetrics := make(map[string]float64)
	for key, gauge := range dm.regionHealth {
		healthMetrics[key] = gauge.Get()
	}
	result["region_health"] = healthMetrics
	
	// Cross-region RTT metrics
	rttMetrics := make(map[string]float64)
	for key, gauge := range dm.crossRegionRTT {
		rttMetrics[key] = gauge.Get()
	}
	result["cross_region_rtt"] = rttMetrics
	
	return result
}

// Reset clears all metrics (useful for testing)
func (dm *DataMetrics) Reset() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	dm.replicationLag = make(map[string]*Gauge)
	dm.replicationPlanSteps = make(map[string]*Counter)
	dm.replicationStepFailures = make(map[string]*Counter)
	dm.replicationStepDuration = make(map[string]*Histogram)
	dm.dataConsistencyErrors = make(map[string]*Counter)
	dm.quarantineCount = make(map[string]*Counter)
	dm.regionHealth = make(map[string]*Gauge)
	dm.crossRegionRTT = make(map[string]*Gauge)
}

// Global instance for data metrics
var GlobalDataMetrics = NewDataMetrics()