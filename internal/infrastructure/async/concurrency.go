package async

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrencyManager controls the level of concurrency in async operations
type ConcurrencyManager struct {
	maxWorkers      int32
	activeWorkers   int32
	queuedTasks     int64
	completedTasks  int64
	failedTasks     int64
	
	// Adaptive settings
	targetLatency   time.Duration
	adaptiveEnabled bool
	lastAdjustment  time.Time
	
	// Rate limiting
	rateLimiter     *TokenBucket
	
	// Metrics
	metrics         *ConcurrencyMetrics
	mu              sync.RWMutex
}

// ConcurrencyMetrics tracks concurrency performance
type ConcurrencyMetrics struct {
	MaxWorkers        int32
	ActiveWorkers     int32
	QueuedTasks       int64
	CompletedTasks    int64
	FailedTasks       int64
	AverageLatency    time.Duration
	ThroughputPerSec  float64
	QueueWaitTime     time.Duration
	WorkerUtilization float64
	
	// Historical data
	LastHour          HistoricalMetrics
	LastDay           HistoricalMetrics
}

// HistoricalMetrics provides historical performance data
type HistoricalMetrics struct {
	TasksCompleted    int64
	AverageLatency    time.Duration
	PeakConcurrency   int32
	ThroughputPerSec  float64
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(maxWorkers int, targetLatency time.Duration) *ConcurrencyManager {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU() * 2
	}
	
	return &ConcurrencyManager{
		maxWorkers:      int32(maxWorkers),
		targetLatency:   targetLatency,
		adaptiveEnabled: true,
		rateLimiter:     NewTokenBucket(1000, time.Second), // 1000 tokens/second
		metrics:         &ConcurrencyMetrics{MaxWorkers: int32(maxWorkers)},
	}
}

// AcquireWorker attempts to acquire a worker slot
func (cm *ConcurrencyManager) AcquireWorker(ctx context.Context) error {
	// Rate limiting check
	if !cm.rateLimiter.TakeToken(ctx) {
		return fmt.Errorf("rate limit exceeded")
	}
	
	// Increment queued tasks
	atomic.AddInt64(&cm.queuedTasks, 1)
	
	start := time.Now()
	
	// Try to acquire worker
	for {
		current := atomic.LoadInt32(&cm.activeWorkers)
		max := atomic.LoadInt32(&cm.maxWorkers)
		
		if current >= max {
			// Wait for available worker
			select {
			case <-time.After(10 * time.Millisecond):
				continue
			case <-ctx.Done():
				atomic.AddInt64(&cm.queuedTasks, -1)
				return ctx.Err()
			}
		}
		
		// Try to increment active workers
		if atomic.CompareAndSwapInt32(&cm.activeWorkers, current, current+1) {
			break
		}
	}
	
	// Update queue wait time
	waitTime := time.Since(start)
	cm.updateQueueWaitTime(waitTime)
	
	atomic.AddInt64(&cm.queuedTasks, -1)
	return nil
}

// ReleaseWorker releases a worker slot
func (cm *ConcurrencyManager) ReleaseWorker(success bool, latency time.Duration) {
	atomic.AddInt32(&cm.activeWorkers, -1)
	
	if success {
		atomic.AddInt64(&cm.completedTasks, 1)
	} else {
		atomic.AddInt64(&cm.failedTasks, 1)
	}
	
	// Update metrics
	cm.updateLatencyMetrics(latency)
	cm.updateThroughputMetrics()
	
	// Trigger adaptive adjustment if enabled
	if cm.adaptiveEnabled {
		cm.maybeAdjustConcurrency()
	}
}

// SetMaxWorkers adjusts the maximum number of workers
func (cm *ConcurrencyManager) SetMaxWorkers(maxWorkers int) {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU() * 2
	}
	
	atomic.StoreInt32(&cm.maxWorkers, int32(maxWorkers))
	cm.metrics.MaxWorkers = int32(maxWorkers)
}

// GetActiveWorkers returns the current number of active workers
func (cm *ConcurrencyManager) GetActiveWorkers() int32 {
	return atomic.LoadInt32(&cm.activeWorkers)
}

// GetQueuedTasks returns the current number of queued tasks
func (cm *ConcurrencyManager) GetQueuedTasks() int64 {
	return atomic.LoadInt64(&cm.queuedTasks)
}

// GetMetrics returns current concurrency metrics
func (cm *ConcurrencyManager) GetMetrics() ConcurrencyMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	return ConcurrencyMetrics{
		MaxWorkers:        atomic.LoadInt32(&cm.maxWorkers),
		ActiveWorkers:     atomic.LoadInt32(&cm.activeWorkers),
		QueuedTasks:       atomic.LoadInt64(&cm.queuedTasks),
		CompletedTasks:    atomic.LoadInt64(&cm.completedTasks),
		FailedTasks:       atomic.LoadInt64(&cm.failedTasks),
		AverageLatency:    cm.metrics.AverageLatency,
		ThroughputPerSec:  cm.metrics.ThroughputPerSec,
		QueueWaitTime:     cm.metrics.QueueWaitTime,
		WorkerUtilization: cm.calculateUtilization(),
		LastHour:          cm.metrics.LastHour,
		LastDay:           cm.metrics.LastDay,
	}
}

// updateLatencyMetrics updates average latency using exponential moving average
func (cm *ConcurrencyManager) updateLatencyMetrics(latency time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cm.metrics.AverageLatency == 0 {
		cm.metrics.AverageLatency = latency
	} else {
		// 90% old, 10% new
		cm.metrics.AverageLatency = time.Duration(
			float64(cm.metrics.AverageLatency)*0.9 + float64(latency)*0.1,
		)
	}
}

// updateQueueWaitTime updates queue wait time metrics
func (cm *ConcurrencyManager) updateQueueWaitTime(waitTime time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cm.metrics.QueueWaitTime == 0 {
		cm.metrics.QueueWaitTime = waitTime
	} else {
		// 95% old, 5% new (queue times are more volatile)
		cm.metrics.QueueWaitTime = time.Duration(
			float64(cm.metrics.QueueWaitTime)*0.95 + float64(waitTime)*0.05,
		)
	}
}

// updateThroughputMetrics calculates current throughput
func (cm *ConcurrencyManager) updateThroughputMetrics() {
	// This would be called periodically in a real implementation
	// For now, we'll calculate based on recent completed tasks
	completed := atomic.LoadInt64(&cm.completedTasks)
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Simple throughput calculation (would be more sophisticated in production)
	cm.metrics.ThroughputPerSec = float64(completed) / time.Since(time.Now().Add(-time.Minute)).Seconds()
}

// calculateUtilization calculates worker utilization percentage
func (cm *ConcurrencyManager) calculateUtilization() float64 {
	active := atomic.LoadInt32(&cm.activeWorkers)
	max := atomic.LoadInt32(&cm.maxWorkers)
	
	if max == 0 {
		return 0.0
	}
	
	return float64(active) / float64(max) * 100.0
}

// maybeAdjustConcurrency automatically adjusts concurrency based on performance
func (cm *ConcurrencyManager) maybeAdjustConcurrency() {
	now := time.Now()
	
	// Don't adjust too frequently
	if now.Sub(cm.lastAdjustment) < 30*time.Second {
		return
	}
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	avgLatency := cm.metrics.AverageLatency
	utilization := cm.calculateUtilization()
	maxWorkers := atomic.LoadInt32(&cm.maxWorkers)
	
	// Adjustment logic
	if avgLatency > cm.targetLatency && utilization > 80 {
		// High latency and high utilization - increase workers
		newMax := int(float64(maxWorkers) * 1.2)
		if newMax <= runtime.NumCPU()*4 { // Cap at 4x CPU cores
			atomic.StoreInt32(&cm.maxWorkers, int32(newMax))
		}
	} else if avgLatency < cm.targetLatency/2 && utilization < 50 {
		// Low latency and low utilization - decrease workers
		newMax := int(float64(maxWorkers) * 0.8)
		if newMax >= runtime.NumCPU() { // Minimum of 1x CPU cores
			atomic.StoreInt32(&cm.maxWorkers, int32(newMax))
		}
	}
	
	cm.lastAdjustment = now
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	tokens    int64
	maxTokens int64
	refillRate time.Duration
	lastRefill time.Time
	mu        sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(maxTokens int64, refillRate time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// TakeToken attempts to take a token from the bucket
func (tb *TokenBucket) TakeToken(ctx context.Context) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	
	if elapsed >= tb.refillRate {
		tokensToAdd := int64(elapsed / tb.refillRate)
		tb.tokens = min(tb.tokens+tokensToAdd, tb.maxTokens)
		tb.lastRefill = now
	}
	
	// Try to take a token
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	
	return false
}

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	workers     int
	taskQueue   chan Task
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	concurrency *ConcurrencyManager
}

// Task represents a unit of work
type Task struct {
	ID      string
	Func    func(context.Context) error
	Created time.Time
	Started time.Time
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		workers:     workers,
		taskQueue:   make(chan Task, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		concurrency: NewConcurrencyManager(workers, 100*time.Millisecond),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.taskQueue)
	wp.cancel()
	wp.wg.Wait()
}

// Submit adds a task to the worker pool
func (wp *WorkerPool) Submit(taskID string, fn func(context.Context) error) error {
	task := Task{
		ID:      taskID,
		Func:    fn,
		Created: time.Now(),
	}
	
	select {
	case wp.taskQueue <- task:
		return nil
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	default:
		return fmt.Errorf("task queue is full")
	}
}

// worker processes tasks from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	
	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // Queue closed
			}
			
			wp.processTask(id, task)
			
		case <-wp.ctx.Done():
			return
		}
	}
}

// processTask processes a single task
func (wp *WorkerPool) processTask(workerID int, task Task) {
	// Acquire worker slot
	if err := wp.concurrency.AcquireWorker(wp.ctx); err != nil {
		return
	}
	
	task.Started = time.Now()
	start := time.Now()
	
	// Execute task
	err := task.Func(wp.ctx)
	
	duration := time.Since(start)
	success := err == nil
	
	// Release worker slot
	wp.concurrency.ReleaseWorker(success, duration)
}

// GetMetrics returns worker pool metrics
func (wp *WorkerPool) GetMetrics() ConcurrencyMetrics {
	return wp.concurrency.GetMetrics()
}

// AdaptiveWorkerPool automatically adjusts worker count based on load
type AdaptiveWorkerPool struct {
	*WorkerPool
	minWorkers      int
	maxWorkers      int
	adjustmentTimer *time.Timer
}

// NewAdaptiveWorkerPool creates a worker pool that adjusts size automatically
func NewAdaptiveWorkerPool(minWorkers, maxWorkers, queueSize int) *AdaptiveWorkerPool {
	wp := NewWorkerPool(minWorkers, queueSize)
	
	return &AdaptiveWorkerPool{
		WorkerPool: wp,
		minWorkers: minWorkers,
		maxWorkers: maxWorkers,
	}
}

// Start starts the adaptive worker pool
func (awp *AdaptiveWorkerPool) Start() {
	awp.WorkerPool.Start()
	
	// Start adjustment timer
	awp.adjustmentTimer = time.AfterFunc(60*time.Second, awp.adjustWorkerCount)
}

// Stop stops the adaptive worker pool
func (awp *AdaptiveWorkerPool) Stop() {
	if awp.adjustmentTimer != nil {
		awp.adjustmentTimer.Stop()
	}
	
	awp.WorkerPool.Stop()
}

// adjustWorkerCount automatically adjusts the worker count
func (awp *AdaptiveWorkerPool) adjustWorkerCount() {
	metrics := awp.GetMetrics()
	
	// Adjustment logic based on utilization and queue length
	utilization := metrics.WorkerUtilization
	queuedTasks := metrics.QueuedTasks
	
	currentWorkers := awp.workers
	
	if utilization > 80 && queuedTasks > 10 {
		// High load - increase workers
		newWorkers := min(currentWorkers+1, awp.maxWorkers)
		if newWorkers > currentWorkers {
			// Add new worker
			awp.wg.Add(1)
			go awp.worker(currentWorkers)
			awp.workers = newWorkers
		}
	} else if utilization < 30 && queuedTasks == 0 {
		// Low load - decrease workers
		newWorkers := max(currentWorkers-1, awp.minWorkers)
		if newWorkers < currentWorkers {
			awp.workers = newWorkers
			// Note: actual worker goroutines will exit naturally
		}
	}
	
	// Schedule next adjustment
	awp.adjustmentTimer = time.AfterFunc(60*time.Second, awp.adjustWorkerCount)
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}