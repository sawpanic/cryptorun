package latency

import (
	"math"
	"sort"
	"sync"
	"time"
)

// StageType represents different pipeline stages for latency tracking
type StageType string

const (
	StageData  StageType = "data"
	StageScore StageType = "score"
	StageGate  StageType = "gate"
	StageOrder StageType = "order"
)

// Histogram provides thread-safe latency tracking with percentile calculation
type Histogram struct {
	mu      sync.RWMutex
	buckets []float64 // Latency values in milliseconds
	maxSize int       // Rolling window size
	current int       // Current position in circular buffer
	full    bool      // Whether buffer is full
	stage   StageType
}

// NewHistogram creates a new histogram for latency tracking
func NewHistogram(stage StageType, maxSize int) *Histogram {
	if maxSize <= 0 {
		maxSize = 1000 // Default rolling window
	}

	return &Histogram{
		buckets: make([]float64, maxSize),
		maxSize: maxSize,
		stage:   stage,
	}
}

// Record adds a latency measurement to the histogram
func (h *Histogram) Record(duration time.Duration) {
	latencyMs := float64(duration.Nanoseconds()) / 1e6

	h.mu.Lock()
	defer h.mu.Unlock()

	h.buckets[h.current] = latencyMs
	h.current = (h.current + 1) % h.maxSize

	if !h.full && h.current == 0 {
		h.full = true
	}
}

// Percentile calculates the specified percentile (0.0-1.0) from recorded latencies
func (h *Histogram) Percentile(p float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	size := h.size()
	if size == 0 {
		return 0.0
	}

	// Create sorted copy of active data
	values := make([]float64, size)
	if h.full {
		// Copy entire buffer
		copy(values, h.buckets)
	} else {
		// Copy only filled portion
		copy(values, h.buckets[:h.current])
	}

	sort.Float64s(values)

	// Calculate percentile index
	index := p * float64(size-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return values[lower]
	}

	// Linear interpolation between bounds
	weight := index - float64(lower)
	return values[lower]*(1-weight) + values[upper]*weight
}

// P50 returns the 50th percentile (median)
func (h *Histogram) P50() float64 {
	return h.Percentile(0.5)
}

// P95 returns the 95th percentile
func (h *Histogram) P95() float64 {
	return h.Percentile(0.95)
}

// P99 returns the 99th percentile
func (h *Histogram) P99() float64 {
	return h.Percentile(0.99)
}

// Count returns the current number of recorded measurements
func (h *Histogram) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.size()
}

// size returns the current buffer size (internal, assumes lock held)
func (h *Histogram) size() int {
	if h.full {
		return h.maxSize
	}
	return h.current
}

// Reset clears all recorded latencies
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.current = 0
	h.full = false
	// Clear buckets
	for i := range h.buckets {
		h.buckets[i] = 0
	}
}

// LatencyMetrics aggregates percentile metrics for a stage
type LatencyMetrics struct {
	Stage StageType `json:"stage"`
	P50   float64   `json:"p50_ms"`
	P95   float64   `json:"p95_ms"`
	P99   float64   `json:"p99_ms"`
	Count int       `json:"count"`
}

// Metrics returns current latency metrics for the histogram
func (h *Histogram) Metrics() LatencyMetrics {
	return LatencyMetrics{
		Stage: h.stage,
		P50:   h.P50(),
		P95:   h.P95(),
		P99:   h.P99(),
		Count: h.Count(),
	}
}

// StageTracker manages histograms for all pipeline stages
type StageTracker struct {
	histograms map[StageType]*Histogram
	mu         sync.RWMutex
}

// NewStageTracker creates a new stage tracker with histograms for all stages
func NewStageTracker() *StageTracker {
	tracker := &StageTracker{
		histograms: make(map[StageType]*Histogram),
	}

	// Initialize histograms for all known stages
	stages := []StageType{StageData, StageScore, StageGate, StageOrder}
	for _, stage := range stages {
		tracker.histograms[stage] = NewHistogram(stage, 1000)
	}

	return tracker
}

// Record adds a latency measurement for the specified stage
func (st *StageTracker) Record(stage StageType, duration time.Duration) {
	st.mu.RLock()
	hist, exists := st.histograms[stage]
	st.mu.RUnlock()

	if !exists {
		// Create histogram for unknown stage
		st.mu.Lock()
		hist = NewHistogram(stage, 1000)
		st.histograms[stage] = hist
		st.mu.Unlock()
	}

	hist.Record(duration)
}

// GetP99 returns the current P99 latency for a stage
func (st *StageTracker) GetP99(stage StageType) float64 {
	st.mu.RLock()
	hist, exists := st.histograms[stage]
	st.mu.RUnlock()

	if !exists {
		return 0.0
	}

	return hist.P99()
}

// AllMetrics returns metrics for all tracked stages
func (st *StageTracker) AllMetrics() map[StageType]LatencyMetrics {
	st.mu.RLock()
	defer st.mu.RUnlock()

	metrics := make(map[StageType]LatencyMetrics)
	for stage, hist := range st.histograms {
		metrics[stage] = hist.Metrics()
	}

	return metrics
}

// Global stage tracker instance
var globalTracker = NewStageTracker()

// Record adds a latency measurement to the global tracker
func Record(stage StageType, duration time.Duration) {
	globalTracker.Record(stage, duration)
}

// GetP99 returns the current P99 latency from the global tracker
func GetP99(stage StageType) float64 {
	return globalTracker.GetP99(stage)
}

// GetAllMetrics returns all metrics from the global tracker
func GetAllMetrics() map[StageType]LatencyMetrics {
	return globalTracker.AllMetrics()
}

// Timer provides convenient latency measurement with automatic recording
type Timer struct {
	stage StageType
	start time.Time
}

// StartTimer creates a new timer for the specified stage
func StartTimer(stage StageType) *Timer {
	return &Timer{
		stage: stage,
		start: time.Now(),
	}
}

// Stop records the elapsed time and returns the duration
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.start)
	Record(t.stage, duration)
	return duration
}

// StopWithResult records the elapsed time with success/failure context
func (t *Timer) StopWithResult(success bool) time.Duration {
	duration := time.Since(t.start)
	Record(t.stage, duration)

	// Could extend to track success/failure rates if needed
	_ = success

	return duration
}
