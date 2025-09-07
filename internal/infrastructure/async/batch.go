package async

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// BatchProcessor efficiently processes items in batches to optimize throughput
type Batcher[T any] struct {
	processor    BatchFunc[T]
	config       BatchConfig
	buffer       []T
	bufferMu     sync.Mutex
	metrics      *BatchMetrics
	flushTimer   *time.Timer
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      int32
}

// BatchFunc defines the function signature for batch processing
type BatchFunc[T any] func(ctx context.Context, batch []T) error

// BatchConfig defines batch processing parameters
type BatchConfig struct {
	MaxBatchSize    int           // Maximum items per batch
	FlushInterval   time.Duration // Maximum time to wait before flushing
	MaxConcurrency  int           // Maximum concurrent batch processors
	BufferCapacity  int           // Buffer size before blocking
	FlushOnShutdown bool          // Whether to flush remaining items on shutdown
}

// DefaultBatchConfig returns optimized batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:    100,
		FlushInterval:   5 * time.Second,
		MaxConcurrency:  4,
		BufferCapacity:  10000,
		FlushOnShutdown: true,
	}
}

// BatchMetrics tracks batch processing performance
type BatchMetrics struct {
	TotalItems        int64
	TotalBatches      int64
	ProcessedBatches  int64
	FailedBatches     int64
	AverageBatchSize  float64
	AverageLatency    time.Duration
	CurrentBuffer     int64
	ThroughputPerSec  float64
	
	// Timing metrics
	LastFlush         time.Time
	LastSuccess       time.Time
	LastError         time.Time
	
	mu sync.RWMutex
}

// NewBatcher creates a new batch processor
func NewBatcher[T any](processor BatchFunc[T], config BatchConfig) *Batcher[T] {
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 4
	}
	if config.BufferCapacity <= 0 {
		config.BufferCapacity = 10000
	}
	
	return &Batcher[T]{
		processor: processor,
		config:    config,
		buffer:    make([]T, 0, config.MaxBatchSize),
		metrics:   &BatchMetrics{},
		stopCh:    make(chan struct{}),
	}
}

// Start begins batch processing
func (b *Batcher[T]) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&b.running, 0, 1) {
		return fmt.Errorf("batcher is already running")
	}
	
	// Start flush timer
	b.resetFlushTimer()
	
	// Start throughput calculator
	b.wg.Add(1)
	go b.throughputCalculator(ctx)
	
	return nil
}

// Stop gracefully stops the batcher
func (b *Batcher[T]) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&b.running, 1, 0) {
		return nil // Already stopped
	}
	
	// Stop flush timer
	if b.flushTimer != nil {
		b.flushTimer.Stop()
	}
	
	// Flush remaining items if configured
	if b.config.FlushOnShutdown {
		b.flushBuffer(ctx)
	}
	
	// Signal stop and wait for goroutines
	close(b.stopCh)
	
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Submit adds an item to the batch
func (b *Batcher[T]) Submit(ctx context.Context, item T) error {
	if atomic.LoadInt32(&b.running) == 0 {
		return fmt.Errorf("batcher is not running")
	}
	
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()
	
	// Check buffer capacity
	if len(b.buffer) >= b.config.BufferCapacity {
		return fmt.Errorf("buffer is full")
	}
	
	// Add item to buffer
	b.buffer = append(b.buffer, item)
	atomic.AddInt64(&b.metrics.TotalItems, 1)
	atomic.StoreInt64(&b.metrics.CurrentBuffer, int64(len(b.buffer)))
	
	// Check if batch is ready
	if len(b.buffer) >= b.config.MaxBatchSize {
		b.flushBuffer(ctx)
	}
	
	return nil
}

// SubmitBatch adds multiple items to the batch
func (b *Batcher[T]) SubmitBatch(ctx context.Context, items []T) error {
	if atomic.LoadInt32(&b.running) == 0 {
		return fmt.Errorf("batcher is not running")
	}
	
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()
	
	// Check buffer capacity
	if len(b.buffer)+len(items) > b.config.BufferCapacity {
		return fmt.Errorf("batch would exceed buffer capacity")
	}
	
	// Add all items
	b.buffer = append(b.buffer, items...)
	atomic.AddInt64(&b.metrics.TotalItems, int64(len(items)))
	atomic.StoreInt64(&b.metrics.CurrentBuffer, int64(len(b.buffer)))
	
	// Flush if batch is ready
	for len(b.buffer) >= b.config.MaxBatchSize {
		b.flushBuffer(ctx)
	}
	
	return nil
}

// Flush forces processing of current buffer
func (b *Batcher[T]) Flush(ctx context.Context) error {
	if atomic.LoadInt32(&b.running) == 0 {
		return fmt.Errorf("batcher is not running")
	}
	
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()
	
	return b.flushBuffer(ctx)
}

// flushBuffer processes the current buffer (must hold bufferMu)
func (b *Batcher[T]) flushBuffer(ctx context.Context) error {
	if len(b.buffer) == 0 {
		return nil
	}
	
	// Create batch from buffer
	batch := make([]T, len(b.buffer))
	copy(batch, b.buffer)
	
	// Clear buffer
	b.buffer = b.buffer[:0]
	atomic.StoreInt64(&b.metrics.CurrentBuffer, 0)
	
	// Reset flush timer
	b.resetFlushTimer()
	
	// Process batch asynchronously
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.processBatch(ctx, batch)
	}()
	
	return nil
}

// processBatch handles batch processing with concurrency control
func (b *Batcher[T]) processBatch(ctx context.Context, batch []T) {
	start := time.Now()
	
	atomic.AddInt64(&b.metrics.TotalBatches, 1)
	
	// Process the batch
	err := b.processor(ctx, batch)
	
	duration := time.Since(start)
	
	// Update metrics
	b.metrics.mu.Lock()
	b.metrics.LastFlush = time.Now()
	
	if err != nil {
		atomic.AddInt64(&b.metrics.FailedBatches, 1)
		b.metrics.LastError = time.Now()
	} else {
		atomic.AddInt64(&b.metrics.ProcessedBatches, 1)
		b.metrics.LastSuccess = time.Now()
	}
	
	// Update average batch size
	totalBatches := atomic.LoadInt64(&b.metrics.TotalBatches)
	if totalBatches > 0 {
		b.metrics.AverageBatchSize = float64(atomic.LoadInt64(&b.metrics.TotalItems)) / float64(totalBatches)
	}
	
	// Update average latency (exponential moving average)
	if b.metrics.AverageLatency == 0 {
		b.metrics.AverageLatency = duration
	} else {
		// 90% old, 10% new
		b.metrics.AverageLatency = time.Duration(
			float64(b.metrics.AverageLatency)*0.9 + float64(duration)*0.1,
		)
	}
	
	b.metrics.mu.Unlock()
}

// resetFlushTimer resets the flush timer
func (b *Batcher[T]) resetFlushTimer() {
	if b.flushTimer != nil {
		b.flushTimer.Stop()
	}
	
	b.flushTimer = time.AfterFunc(b.config.FlushInterval, func() {
		if atomic.LoadInt32(&b.running) == 1 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			b.bufferMu.Lock()
			b.flushBuffer(ctx)
			b.bufferMu.Unlock()
		}
	})
}

// throughputCalculator periodically calculates throughput metrics
func (b *Batcher[T]) throughputCalculator(ctx context.Context) {
	defer b.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastItems int64
	var lastTime time.Time = time.Now()
	
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			currentItems := atomic.LoadInt64(&b.metrics.TotalItems)
			
			if !lastTime.IsZero() {
				elapsed := now.Sub(lastTime).Seconds()
				if elapsed > 0 {
					itemsProcessed := currentItems - lastItems
					throughput := float64(itemsProcessed) / elapsed
					
					b.metrics.mu.Lock()
					b.metrics.ThroughputPerSec = throughput
					b.metrics.mu.Unlock()
				}
			}
			
			lastItems = currentItems
			lastTime = now
			
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		}
	}
}

// GetMetrics returns current batch processing metrics
func (b *Batcher[T]) GetMetrics() BatchMetrics {
	b.metrics.mu.RLock()
	defer b.metrics.mu.RUnlock()
	
	return BatchMetrics{
		TotalItems:        atomic.LoadInt64(&b.metrics.TotalItems),
		TotalBatches:      atomic.LoadInt64(&b.metrics.TotalBatches),
		ProcessedBatches:  atomic.LoadInt64(&b.metrics.ProcessedBatches),
		FailedBatches:     atomic.LoadInt64(&b.metrics.FailedBatches),
		AverageBatchSize:  b.metrics.AverageBatchSize,
		AverageLatency:    b.metrics.AverageLatency,
		CurrentBuffer:     atomic.LoadInt64(&b.metrics.CurrentBuffer),
		ThroughputPerSec:  b.metrics.ThroughputPerSec,
		LastFlush:         b.metrics.LastFlush,
		LastSuccess:       b.metrics.LastSuccess,
		LastError:         b.metrics.LastError,
	}
}

// AdaptiveBatcher automatically adjusts batch size based on performance
type AdaptiveBatcher[T any] struct {
	*Batcher[T]
	targetLatency   time.Duration
	minBatchSize    int
	maxBatchSize    int
	adjustmentTimer *time.Timer
	lastAdjustment  time.Time
}

// NewAdaptiveBatcher creates a batcher that adjusts batch size automatically
func NewAdaptiveBatcher[T any](processor BatchFunc[T], config BatchConfig, targetLatency time.Duration) *AdaptiveBatcher[T] {
	batcher := NewBatcher(processor, config)
	
	return &AdaptiveBatcher[T]{
		Batcher:       batcher,
		targetLatency: targetLatency,
		minBatchSize:  config.MaxBatchSize / 4, // 25% of max
		maxBatchSize:  config.MaxBatchSize * 2, // 200% of max
	}
}

// Start begins adaptive batch processing
func (ab *AdaptiveBatcher[T]) Start(ctx context.Context) error {
	if err := ab.Batcher.Start(ctx); err != nil {
		return err
	}
	
	// Start adjustment timer
	ab.adjustmentTimer = time.AfterFunc(30*time.Second, ab.adjustBatchSize)
	
	return nil
}

// Stop stops the adaptive batcher
func (ab *AdaptiveBatcher[T]) Stop(ctx context.Context) error {
	if ab.adjustmentTimer != nil {
		ab.adjustmentTimer.Stop()
	}
	
	return ab.Batcher.Stop(ctx)
}

// adjustBatchSize automatically adjusts batch size based on performance
func (ab *AdaptiveBatcher[T]) adjustBatchSize() {
	if atomic.LoadInt32(&ab.running) == 0 {
		return
	}
	
	metrics := ab.GetMetrics()
	
	// Only adjust if we have enough data
	if metrics.ProcessedBatches < 5 {
		ab.scheduleNextAdjustment()
		return
	}
	
	currentBatchSize := ab.config.MaxBatchSize
	avgLatency := metrics.AverageLatency
	
	// Adjust based on latency vs target
	if avgLatency > ab.targetLatency {
		// Too slow, reduce batch size
		newSize := int(float64(currentBatchSize) * 0.8)
		if newSize >= ab.minBatchSize {
			ab.config.MaxBatchSize = newSize
		}
	} else if avgLatency < ab.targetLatency/2 {
		// Too fast, increase batch size
		newSize := int(float64(currentBatchSize) * 1.2)
		if newSize <= ab.maxBatchSize {
			ab.config.MaxBatchSize = newSize
		}
	}
	
	ab.lastAdjustment = time.Now()
	ab.scheduleNextAdjustment()
}

// scheduleNextAdjustment schedules the next batch size adjustment
func (ab *AdaptiveBatcher[T]) scheduleNextAdjustment() {
	ab.adjustmentTimer = time.AfterFunc(30*time.Second, ab.adjustBatchSize)
}

// PriorityBatcher processes items with different priority levels
type PriorityBatcher[T any] struct {
	processors map[Priority]BatchFunc[T]
	batchers   map[Priority]*Batcher[T]
	config     BatchConfig
}

// Priority defines processing priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// NewPriorityBatcher creates a priority-aware batch processor
func NewPriorityBatcher[T any](processors map[Priority]BatchFunc[T], config BatchConfig) *PriorityBatcher[T] {
	batchers := make(map[Priority]*Batcher[T])
	
	for priority, processor := range processors {
		// Adjust config based on priority
		priorityConfig := config
		switch priority {
		case PriorityCritical:
			priorityConfig.MaxBatchSize = config.MaxBatchSize / 4
			priorityConfig.FlushInterval = config.FlushInterval / 4
		case PriorityHigh:
			priorityConfig.MaxBatchSize = config.MaxBatchSize / 2
			priorityConfig.FlushInterval = config.FlushInterval / 2
		case PriorityNormal:
			// Use default config
		case PriorityLow:
			priorityConfig.MaxBatchSize = config.MaxBatchSize * 2
			priorityConfig.FlushInterval = config.FlushInterval * 2
		}
		
		batchers[priority] = NewBatcher(processor, priorityConfig)
	}
	
	return &PriorityBatcher[T]{
		processors: processors,
		batchers:   batchers,
		config:     config,
	}
}

// Start starts all priority batchers
func (pb *PriorityBatcher[T]) Start(ctx context.Context) error {
	for priority, batcher := range pb.batchers {
		if err := batcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start batcher for priority %d: %w", priority, err)
		}
	}
	return nil
}

// Stop stops all priority batchers
func (pb *PriorityBatcher[T]) Stop(ctx context.Context) error {
	var errors []error
	for priority, batcher := range pb.batchers {
		if err := batcher.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop batcher for priority %d: %w", priority, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("multiple stop errors: %v", errors)
	}
	
	return nil
}

// Submit submits an item with specified priority
func (pb *PriorityBatcher[T]) Submit(ctx context.Context, item T, priority Priority) error {
	batcher, exists := pb.batchers[priority]
	if !exists {
		return fmt.Errorf("no batcher configured for priority %d", priority)
	}
	
	return batcher.Submit(ctx, item)
}

// GetMetrics returns metrics for all priority levels
func (pb *PriorityBatcher[T]) GetMetrics() map[Priority]BatchMetrics {
	metrics := make(map[Priority]BatchMetrics)
	
	for priority, batcher := range pb.batchers {
		metrics[priority] = batcher.GetMetrics()
	}
	
	return metrics
}