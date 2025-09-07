package async

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineStage represents a single stage in the processing pipeline
type PipelineStage[T any] interface {
	Process(ctx context.Context, input T) (T, error)
	Name() string
}

// Pipeline represents an asynchronous processing pipeline
type Pipeline[T any] struct {
	stages       []PipelineStage[T]
	workers      int
	bufferSize   int
	metrics      *PipelineMetrics
	errorHandler ErrorHandler[T]
	
	// Internal channels
	input    chan T
	output   chan T
	errors   chan error
	done     chan struct{}
	
	// State management
	running  int32
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// PipelineConfig configures pipeline behavior
type PipelineConfig struct {
	Workers    int           // Number of worker goroutines
	BufferSize int           // Size of internal buffers
	Timeout    time.Duration // Per-stage timeout
	MaxRetries int           // Maximum retry attempts
}

// DefaultPipelineConfig returns sensible default configuration
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Workers:    runtime.NumCPU(),
		BufferSize: 1000,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// ErrorHandler defines how pipeline errors should be handled
type ErrorHandler[T any] interface {
	HandleError(ctx context.Context, stage string, input T, err error) ErrorAction
}

// ErrorAction defines what action to take on error
type ErrorAction int

const (
	ErrorActionRetry ErrorAction = iota
	ErrorActionSkip
	ErrorActionFail
)

// PipelineMetrics tracks pipeline performance
type PipelineMetrics struct {
	ItemsProcessed    int64
	ItemsSuccess      int64
	ItemsErrors       int64
	ItemsSkipped      int64
	TotalLatency      time.Duration
	StageLatencies    map[string]time.Duration
	CurrentQueueDepth int64
	
	mu sync.RWMutex
}

// NewPipeline creates a new asynchronous pipeline
func NewPipeline[T any](stages []PipelineStage[T], config PipelineConfig) *Pipeline[T] {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}
	
	return &Pipeline[T]{
		stages:     stages,
		workers:    config.Workers,
		bufferSize: config.BufferSize,
		metrics:    &PipelineMetrics{StageLatencies: make(map[string]time.Duration)},
		input:      make(chan T, config.BufferSize),
		output:     make(chan T, config.BufferSize),
		errors:     make(chan error, config.BufferSize),
		done:       make(chan struct{}),
	}
}

// SetErrorHandler sets the error handling strategy
func (p *Pipeline[T]) SetErrorHandler(handler ErrorHandler[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errorHandler = handler
}

// Start begins pipeline processing
func (p *Pipeline[T]) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return fmt.Errorf("pipeline is already running")
	}
	
	// Start worker goroutines
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	
	// Start metrics collection
	go p.metricsCollector(ctx)
	
	return nil
}

// Stop gracefully stops the pipeline
func (p *Pipeline[T]) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return nil // Already stopped
	}
	
	close(p.input)
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		close(p.output)
		close(p.errors)
		close(p.done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Submit adds an item to the pipeline for processing
func (p *Pipeline[T]) Submit(item T) error {
	if atomic.LoadInt32(&p.running) == 0 {
		return fmt.Errorf("pipeline is not running")
	}
	
	select {
	case p.input <- item:
		atomic.AddInt64(&p.metrics.CurrentQueueDepth, 1)
		return nil
	default:
		return fmt.Errorf("pipeline buffer is full")
	}
}

// Output returns the output channel for processed items
func (p *Pipeline[T]) Output() <-chan T {
	return p.output
}

// Errors returns the error channel
func (p *Pipeline[T]) Errors() <-chan error {
	return p.errors
}

// GetMetrics returns current pipeline metrics
func (p *Pipeline[T]) GetMetrics() PipelineMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	
	// Create copy to avoid race conditions
	metrics := *p.metrics
	metrics.StageLatencies = make(map[string]time.Duration)
	for k, v := range p.metrics.StageLatencies {
		metrics.StageLatencies[k] = v
	}
	
	return metrics
}

// worker processes items through the pipeline stages
func (p *Pipeline[T]) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()
	
	for {
		select {
		case item, ok := <-p.input:
			if !ok {
				return // Input channel closed
			}
			
			atomic.AddInt64(&p.metrics.CurrentQueueDepth, -1)
			p.processItem(ctx, item, workerID)
			
		case <-ctx.Done():
			return
		}
	}
}

// processItem processes a single item through all pipeline stages
func (p *Pipeline[T]) processItem(ctx context.Context, item T, workerID int) {
	start := time.Now()
	current := item
	
	defer func() {
		totalLatency := time.Since(start)
		atomic.AddInt64(&p.metrics.ItemsProcessed, 1)
		
		p.metrics.mu.Lock()
		p.metrics.TotalLatency += totalLatency
		p.metrics.mu.Unlock()
	}()
	
	// Process through each stage
	for i, stage := range p.stages {
		stageStart := time.Now()
		
		// Create stage-specific context with timeout
		stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		
		result, err := p.processStage(stageCtx, stage, current, workerID)
		cancel()
		
		stageDuration := time.Since(stageStart)
		p.recordStageMetrics(stage.Name(), stageDuration)
		
		if err != nil {
			p.handleStageError(ctx, stage, current, err)
			return
		}
		
		current = result
	}
	
	// Successfully processed through all stages
	select {
	case p.output <- current:
		atomic.AddInt64(&p.metrics.ItemsSuccess, 1)
	case <-ctx.Done():
		return
	default:
		// Output buffer full, record as error
		p.recordError(fmt.Errorf("output buffer full"))
	}
}

// processStage processes an item through a single stage with retry logic
func (p *Pipeline[T]) processStage(ctx context.Context, stage PipelineStage[T], item T, workerID int) (T, error) {
	var lastErr error
	
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			// Exponential backoff
			backoff := time.Duration(retry) * 100 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return item, ctx.Err()
			}
		}
		
		result, err := stage.Process(ctx, item)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !p.isRetryableError(err) {
			break
		}
	}
	
	return item, lastErr
}

// handleStageError handles errors that occur in pipeline stages
func (p *Pipeline[T]) handleStageError(ctx context.Context, stage PipelineStage[T], item T, err error) {
	atomic.AddInt64(&p.metrics.ItemsErrors, 1)
	
	if p.errorHandler != nil {
		action := p.errorHandler.HandleError(ctx, stage.Name(), item, err)
		
		switch action {
		case ErrorActionRetry:
			// Re-submit to pipeline
			select {
			case p.input <- item:
				atomic.AddInt64(&p.metrics.CurrentQueueDepth, 1)
			default:
				p.recordError(fmt.Errorf("failed to retry item: buffer full"))
			}
			return
			
		case ErrorActionSkip:
			atomic.AddInt64(&p.metrics.ItemsSkipped, 1)
			return
			
		case ErrorActionFail:
			// Fall through to record error
		}
	}
	
	// Record error
	wrappedErr := fmt.Errorf("stage %s failed: %w", stage.Name(), err)
	p.recordError(wrappedErr)
}

// recordStageMetrics updates metrics for a specific stage
func (p *Pipeline[T]) recordStageMetrics(stageName string, duration time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	
	// Update moving average of stage latency
	if existing, exists := p.metrics.StageLatencies[stageName]; exists {
		// Simple moving average with weight towards recent measurements
		p.metrics.StageLatencies[stageName] = time.Duration(
			float64(existing)*0.8 + float64(duration)*0.2,
		)
	} else {
		p.metrics.StageLatencies[stageName] = duration
	}
}

// recordError sends an error to the error channel
func (p *Pipeline[T]) recordError(err error) {
	select {
	case p.errors <- err:
	default:
		// Error channel full, log and continue
		// In production, this would use a proper logger
		fmt.Printf("Pipeline error channel full, dropping error: %v\n", err)
	}
}

// isRetryableError determines if an error should trigger a retry
func (p *Pipeline[T]) isRetryableError(err error) bool {
	// Implement retry logic based on error type
	// For now, retry all errors except context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}

// metricsCollector periodically updates pipeline metrics
func (p *Pipeline[T]) metricsCollector(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Update queue depth metric
			queueDepth := int64(len(p.input))
			atomic.StoreInt64(&p.metrics.CurrentQueueDepth, queueDepth)
			
		case <-ctx.Done():
			return
		case <-p.done:
			return
		}
	}
}

// BatchProcessor provides batch processing capabilities
type BatchProcessor[T any] struct {
	pipeline   *Pipeline[[]T]
	batchSize  int
	flushTimer *time.Timer
	buffer     []T
	mu         sync.Mutex
}

// NewBatchProcessor creates a batch processor wrapper around a pipeline
func NewBatchProcessor[T any](pipeline *Pipeline[[]T], batchSize int, flushInterval time.Duration) *BatchProcessor[T] {
	bp := &BatchProcessor[T]{
		pipeline:  pipeline,
		batchSize: batchSize,
		buffer:    make([]T, 0, batchSize),
	}
	
	// Start flush timer
	bp.flushTimer = time.AfterFunc(flushInterval, bp.flush)
	
	return bp
}

// Submit adds an item to the batch
func (bp *BatchProcessor[T]) Submit(item T) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.buffer = append(bp.buffer, item)
	
	if len(bp.buffer) >= bp.batchSize {
		return bp.flushLocked()
	}
	
	return nil
}

// flush sends the current batch to the pipeline
func (bp *BatchProcessor[T]) flush() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.flushLocked()
}

// flushLocked flushes the buffer while holding the lock
func (bp *BatchProcessor[T]) flushLocked() error {
	if len(bp.buffer) == 0 {
		return nil
	}
	
	// Copy buffer to prevent modifications
	batch := make([]T, len(bp.buffer))
	copy(batch, bp.buffer)
	
	// Reset buffer
	bp.buffer = bp.buffer[:0]
	
	// Submit batch to pipeline
	return bp.pipeline.Submit(batch)
}

// Close flushes remaining items and closes the batch processor
func (bp *BatchProcessor[T]) Close() error {
	bp.flushTimer.Stop()
	bp.flush()
	return nil
}