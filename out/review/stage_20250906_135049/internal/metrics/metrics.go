package metrics

import (
	"math"
	"sync"
	"sync/atomic"
)

// Counter represents a monotonically increasing counter
type Counter struct {
	value uint64
}

// NewCounter creates a new counter with the given name
func NewCounter(name string) *Counter {
	return &Counter{}
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

// Add adds the given value to the counter
func (c *Counter) Add(value float64) {
	if value >= 0 {
		atomic.AddUint64(&c.value, uint64(value))
	}
}

// Get returns the current counter value
func (c *Counter) Get() float64 {
	return float64(atomic.LoadUint64(&c.value))
}

// Gauge represents a metric that can go up and down
type Gauge struct {
	value uint64 // Store as bits of float64
	mu    sync.RWMutex
}

// NewGauge creates a new gauge with the given name
func NewGauge(name string) *Gauge {
	return &Gauge{}
}

// Set sets the gauge to the given value
func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	atomic.StoreUint64(&g.value, math.Float64bits(value))
	g.mu.Unlock()
}

// Inc increments the gauge by 1
func (g *Gauge) Inc() {
	g.Add(1.0)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec() {
	g.Add(-1.0)
}

// Add adds the given value to the gauge
func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	current := math.Float64frombits(atomic.LoadUint64(&g.value))
	atomic.StoreUint64(&g.value, math.Float64bits(current+value))
	g.mu.Unlock()
}

// Get returns the current gauge value
func (g *Gauge) Get() float64 {
	return math.Float64frombits(atomic.LoadUint64(&g.value))
}

// Histogram tracks distributions of values and calculates quantiles
type Histogram struct {
	name    string
	buckets []float64
	counts  []uint64
	sum     uint64 // Sum of all observed values (as uint64 bits)
	count   uint64 // Total number of observations
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram with the given name
func NewHistogram(name string) *Histogram {
	// Default buckets for latency measurements (milliseconds)
	buckets := []float64{1, 5, 10, 25, 50, 100, 200, 300, 500, 1000}
	return &Histogram{
		name:    name,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1), // +1 for +Inf bucket
	}
}

// Observe records a new observation
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Update sum and count
	atomic.AddUint64(&h.sum, math.Float64bits(value))
	atomic.AddUint64(&h.count, 1)
	
	// Find the appropriate bucket
	bucketIndex := len(h.buckets) // Default to +Inf bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIndex = i
			break
		}
	}
	
	atomic.AddUint64(&h.counts[bucketIndex], 1)
}

// Quantile calculates the given quantile (0.0 to 1.0)
func (h *Histogram) Quantile(q float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	totalCount := atomic.LoadUint64(&h.count)
	if totalCount == 0 {
		return 0
	}
	
	targetCount := float64(totalCount) * q
	cumulativeCount := uint64(0)
	
	for i, count := range h.counts {
		cumulativeCount += atomic.LoadUint64(&count)
		if float64(cumulativeCount) >= targetCount {
			if i < len(h.buckets) {
				return h.buckets[i]
			} else {
				// +Inf bucket, return a large value
				return 10000.0
			}
		}
	}
	
	return 0
}

// Count returns the total number of observations
func (h *Histogram) Count() uint64 {
	return atomic.LoadUint64(&h.count)
}

// Sum returns the sum of all observed values
func (h *Histogram) Sum() float64 {
	return math.Float64frombits(atomic.LoadUint64(&h.sum))
}

// Mean returns the mean of all observed values
func (h *Histogram) Mean() float64 {
	count := atomic.LoadUint64(&h.count)
	if count == 0 {
		return 0
	}
	return h.Sum() / float64(count)
}