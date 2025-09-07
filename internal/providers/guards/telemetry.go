package guards

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Telemetry collects and exports provider metrics
type Telemetry struct {
	providerName   string
	cacheHits      int64
	cacheMisses    int64
	requests       int64
	successes      int64
	failures       int64
	errors         int64
	rateLimits     int64
	circuitOpens   int64
	backoffs       int64
	totalLatency   int64 // microseconds
	latencyCount   int64
	lastSuccess    int64 // unix timestamp
	lastFailure    int64 // unix timestamp
	startTime      time.Time
	mutex          sync.RWMutex
	latencyBuckets [10]int64 // P50, P90, P95, P99 approximation
}

// NewTelemetry creates a new telemetry collector
func NewTelemetry(providerName string) *Telemetry {
	return &Telemetry{
		providerName: providerName,
		startTime:    time.Now(),
	}
}

// RecordCacheHit records a cache hit with latency
func (t *Telemetry) RecordCacheHit(latency time.Duration) {
	atomic.AddInt64(&t.cacheHits, 1)
	t.recordLatency(latency)
}

// RecordCacheMiss records a cache miss
func (t *Telemetry) RecordCacheMiss() {
	atomic.AddInt64(&t.cacheMisses, 1)
}

// RecordSuccess records a successful API call
func (t *Telemetry) RecordSuccess(latency time.Duration) {
	atomic.AddInt64(&t.requests, 1)
	atomic.AddInt64(&t.successes, 1)
	atomic.StoreInt64(&t.lastSuccess, time.Now().Unix())
	t.recordLatency(latency)
}

// RecordFailure records a failed API call with status code
func (t *Telemetry) RecordFailure(statusCode int) {
	atomic.AddInt64(&t.requests, 1)
	atomic.AddInt64(&t.failures, 1)
	atomic.StoreInt64(&t.lastFailure, time.Now().Unix())
}

// RecordError records a network or other error
func (t *Telemetry) RecordError() {
	atomic.AddInt64(&t.requests, 1)
	atomic.AddInt64(&t.errors, 1)
	atomic.StoreInt64(&t.lastFailure, time.Now().Unix())
}

// RecordRateLimit records a rate limit hit
func (t *Telemetry) RecordRateLimit() {
	atomic.AddInt64(&t.rateLimits, 1)
}

// RecordCircuitOpen records a circuit breaker opening
func (t *Telemetry) RecordCircuitOpen() {
	atomic.AddInt64(&t.circuitOpens, 1)
}

// RecordBackoff records a retry backoff
func (t *Telemetry) RecordBackoff(duration time.Duration) {
	atomic.AddInt64(&t.backoffs, 1)
}

// recordLatency updates latency statistics
func (t *Telemetry) recordLatency(latency time.Duration) {
	microseconds := latency.Microseconds()
	atomic.AddInt64(&t.totalLatency, microseconds)
	atomic.AddInt64(&t.latencyCount, 1)

	// Simple histogram for percentile approximation
	bucketIndex := t.getBucketIndex(latency)
	if bucketIndex < len(t.latencyBuckets) {
		atomic.AddInt64(&t.latencyBuckets[bucketIndex], 1)
	}
}

// getBucketIndex returns histogram bucket for latency
func (t *Telemetry) getBucketIndex(latency time.Duration) int {
	ms := latency.Milliseconds()
	switch {
	case ms <= 10:
		return 0
	case ms <= 25:
		return 1
	case ms <= 50:
		return 2
	case ms <= 100:
		return 3
	case ms <= 250:
		return 4
	case ms <= 500:
		return 5
	case ms <= 1000:
		return 6
	case ms <= 2500:
		return 7
	case ms <= 5000:
		return 8
	default:
		return 9
	}
}

// CacheHitRate returns the cache hit rate as a percentage
func (t *Telemetry) CacheHitRate() float64 {
	hits := atomic.LoadInt64(&t.cacheHits)
	misses := atomic.LoadInt64(&t.cacheMisses)
	total := hits + misses

	if total == 0 {
		return 0.0
	}

	return float64(hits) / float64(total)
}

// RequestCount returns the total number of requests
func (t *Telemetry) RequestCount() int64 {
	return atomic.LoadInt64(&t.requests)
}

// ErrorRate returns the error rate as a percentage
func (t *Telemetry) ErrorRate() float64 {
	requests := atomic.LoadInt64(&t.requests)
	failures := atomic.LoadInt64(&t.failures)
	errors := atomic.LoadInt64(&t.errors)
	totalErrors := failures + errors

	if requests == 0 {
		return 0.0
	}

	return float64(totalErrors) / float64(requests)
}

// AvgLatency returns the average latency
func (t *Telemetry) AvgLatency() time.Duration {
	totalMicros := atomic.LoadInt64(&t.totalLatency)
	count := atomic.LoadInt64(&t.latencyCount)

	if count == 0 {
		return 0
	}

	return time.Duration(totalMicros/count) * time.Microsecond
}

// LastSuccess returns the timestamp of the last successful request
func (t *Telemetry) LastSuccess() time.Time {
	timestamp := atomic.LoadInt64(&t.lastSuccess)
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// LastFailure returns the timestamp of the last failed request
func (t *Telemetry) LastFailure() time.Time {
	timestamp := atomic.LoadInt64(&t.lastFailure)
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// GetMetrics returns current metrics snapshot
func (t *Telemetry) GetMetrics() TelemetryMetrics {
	return TelemetryMetrics{
		Provider:      t.providerName,
		CacheHits:     atomic.LoadInt64(&t.cacheHits),
		CacheMisses:   atomic.LoadInt64(&t.cacheMisses),
		CacheHitRate:  t.CacheHitRate(),
		Requests:      atomic.LoadInt64(&t.requests),
		Successes:     atomic.LoadInt64(&t.successes),
		Failures:      atomic.LoadInt64(&t.failures),
		Errors:        atomic.LoadInt64(&t.errors),
		ErrorRate:     t.ErrorRate(),
		RateLimits:    atomic.LoadInt64(&t.rateLimits),
		CircuitOpens:  atomic.LoadInt64(&t.circuitOpens),
		Backoffs:      atomic.LoadInt64(&t.backoffs),
		AvgLatency:    t.AvgLatency(),
		LastSuccess:   t.LastSuccess(),
		LastFailure:   t.LastFailure(),
		UptimeSeconds: int64(time.Since(t.startTime).Seconds()),
	}
}

// TelemetryMetrics represents a snapshot of telemetry metrics
type TelemetryMetrics struct {
	Provider      string        `json:"provider"`
	CacheHits     int64         `json:"cache_hits"`
	CacheMisses   int64         `json:"cache_misses"`
	CacheHitRate  float64       `json:"cache_hit_rate"`
	Requests      int64         `json:"requests"`
	Successes     int64         `json:"successes"`
	Failures      int64         `json:"failures"`
	Errors        int64         `json:"errors"`
	ErrorRate     float64       `json:"error_rate"`
	RateLimits    int64         `json:"rate_limits"`
	CircuitOpens  int64         `json:"circuit_opens"`
	Backoffs      int64         `json:"backoffs"`
	AvgLatency    time.Duration `json:"avg_latency"`
	LastSuccess   time.Time     `json:"last_success"`
	LastFailure   time.Time     `json:"last_failure"`
	UptimeSeconds int64         `json:"uptime_seconds"`
}

// Reset resets all telemetry counters
func (t *Telemetry) Reset() {
	atomic.StoreInt64(&t.cacheHits, 0)
	atomic.StoreInt64(&t.cacheMisses, 0)
	atomic.StoreInt64(&t.requests, 0)
	atomic.StoreInt64(&t.successes, 0)
	atomic.StoreInt64(&t.failures, 0)
	atomic.StoreInt64(&t.errors, 0)
	atomic.StoreInt64(&t.rateLimits, 0)
	atomic.StoreInt64(&t.circuitOpens, 0)
	atomic.StoreInt64(&t.backoffs, 0)
	atomic.StoreInt64(&t.totalLatency, 0)
	atomic.StoreInt64(&t.latencyCount, 0)
	atomic.StoreInt64(&t.lastSuccess, 0)
	atomic.StoreInt64(&t.lastFailure, 0)
	t.startTime = time.Now()

	// Reset latency buckets
	for i := range t.latencyBuckets {
		atomic.StoreInt64(&t.latencyBuckets[i], 0)
	}
}

// MultiProviderTelemetry manages telemetry for multiple providers
type MultiProviderTelemetry struct {
	collectors map[string]*Telemetry
	mutex      sync.RWMutex
}

// NewMultiProviderTelemetry creates a telemetry manager
func NewMultiProviderTelemetry() *MultiProviderTelemetry {
	return &MultiProviderTelemetry{
		collectors: make(map[string]*Telemetry),
	}
}

// AddProvider adds telemetry for a specific provider
func (mt *MultiProviderTelemetry) AddProvider(name string) {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()

	mt.collectors[name] = NewTelemetry(name)
}

// GetCollector returns the telemetry collector for a provider
func (mt *MultiProviderTelemetry) GetCollector(provider string) *Telemetry {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()

	return mt.collectors[provider]
}

// GetAllMetrics returns metrics for all providers
func (mt *MultiProviderTelemetry) GetAllMetrics() map[string]TelemetryMetrics {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()

	metrics := make(map[string]TelemetryMetrics)
	for name, collector := range mt.collectors {
		metrics[name] = collector.GetMetrics()
	}

	return metrics
}

// ExportToCSV exports telemetry data to CSV file
func (mt *MultiProviderTelemetry) ExportToCSV(filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	header := []string{
		"provider", "cache_hits", "cache_misses", "cache_hit_rate",
		"requests", "successes", "failures", "errors", "error_rate",
		"rate_limits", "circuit_opens", "backoffs", "avg_latency_ms",
		"last_success", "last_failure", "uptime_seconds",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write metrics for each provider
	mt.mutex.RLock()
	for _, collector := range mt.collectors {
		metrics := collector.GetMetrics()
		row := []string{
			metrics.Provider,
			fmt.Sprintf("%d", metrics.CacheHits),
			fmt.Sprintf("%d", metrics.CacheMisses),
			fmt.Sprintf("%.2f", metrics.CacheHitRate),
			fmt.Sprintf("%d", metrics.Requests),
			fmt.Sprintf("%d", metrics.Successes),
			fmt.Sprintf("%d", metrics.Failures),
			fmt.Sprintf("%d", metrics.Errors),
			fmt.Sprintf("%.2f", metrics.ErrorRate),
			fmt.Sprintf("%d", metrics.RateLimits),
			fmt.Sprintf("%d", metrics.CircuitOpens),
			fmt.Sprintf("%d", metrics.Backoffs),
			fmt.Sprintf("%.2f", float64(metrics.AvgLatency.Nanoseconds())/1e6),
			metrics.LastSuccess.Format(time.RFC3339),
			metrics.LastFailure.Format(time.RFC3339),
			fmt.Sprintf("%d", metrics.UptimeSeconds),
		}

		if err := writer.Write(row); err != nil {
			mt.mutex.RUnlock()
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}
	mt.mutex.RUnlock()

	return nil
}

// LogMetrics writes metrics to log in structured format
func (mt *MultiProviderTelemetry) LogMetrics() {
	metrics := mt.GetAllMetrics()

	for provider, m := range metrics {
		fmt.Printf("PROVIDER_METRICS provider=%s cache_hit_rate=%.2f error_rate=%.2f avg_latency_ms=%.2f requests=%d\n",
			provider, m.CacheHitRate, m.ErrorRate,
			float64(m.AvgLatency.Nanoseconds())/1e6, m.Requests)
	}
}
