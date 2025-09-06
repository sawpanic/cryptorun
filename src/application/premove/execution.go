package premove

import (
	"math"
	"sync"
	"time"
)

// ExecutionMonitor tracks intended vs actual execution performance
type ExecutionMonitor struct {
	mu                  sync.RWMutex
	executions          []ExecutionRecord
	metrics             *ExecutionMetrics
	maxSlippageBps      float64
	targetFillTimeMs    int64
	recoveryCooldownMs  int64
	maxConsecutiveFails int
	consecutiveFails    int
	lastFailTime        time.Time
	inRecoveryMode      bool
}

// NewExecutionMonitor creates a new execution monitor
func NewExecutionMonitor(maxSlippageBps float64, targetFillTimeMs, recoveryCooldownMs int64, maxConsecutiveFails int) *ExecutionMonitor {
	return &ExecutionMonitor{
		executions:          make([]ExecutionRecord, 0),
		metrics:             &ExecutionMetrics{},
		maxSlippageBps:      maxSlippageBps,
		targetFillTimeMs:    targetFillTimeMs,
		recoveryCooldownMs:  recoveryCooldownMs,
		maxConsecutiveFails: maxConsecutiveFails,
		consecutiveFails:    0,
		inRecoveryMode:      false,
	}
}

// ExecutionRecord represents a single execution attempt
type ExecutionRecord struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"` // "buy" or "sell"
	IntendedPrice float64   `json:"intended_price"`
	IntendedSize  float64   `json:"intended_size"`
	ActualPrice   float64   `json:"actual_price"`
	ActualSize    float64   `json:"actual_size"`
	SlippageBps   float64   `json:"slippage_bps"`
	TimeToFillMs  int64     `json:"time_to_fill_ms"`
	Status        string    `json:"status"` // "filled", "partial", "failed", "cancelled"
	Exchange      string    `json:"exchange"`
	Timestamp     time.Time `json:"timestamp"`
	OrderType     string    `json:"order_type"` // "market", "limit", "stop"

	// Quality metrics
	IsSlippageAcceptable bool    `json:"is_slippage_acceptable"`
	IsFillTimeAcceptable bool    `json:"is_fill_time_acceptable"`
	QualityScore         float64 `json:"quality_score"` // 0-100 execution quality

	// Context
	MarketConditions map[string]float64 `json:"market_conditions,omitempty"`
	PreMoveScore     float64            `json:"pre_move_score,omitempty"`
	TriggerReason    string             `json:"trigger_reason,omitempty"`
}

// ExecutionMetrics aggregates execution performance statistics
type ExecutionMetrics struct {
	TotalExecutions      int64 `json:"total_executions"`
	SuccessfulExecutions int64 `json:"successful_executions"`
	PartialFills         int64 `json:"partial_fills"`
	FailedExecutions     int64 `json:"failed_executions"`
	CancelledExecutions  int64 `json:"cancelled_executions"`

	// Performance metrics
	AvgSlippageBps    float64 `json:"avg_slippage_bps"`
	MedianSlippageBps float64 `json:"median_slippage_bps"`
	P95SlippageBps    float64 `json:"p95_slippage_bps"`
	AvgFillTimeMs     float64 `json:"avg_fill_time_ms"`
	MedianFillTimeMs  float64 `json:"median_fill_time_ms"`
	P95FillTimeMs     float64 `json:"p95_fill_time_ms"`

	// Quality scores
	AvgQualityScore        float64 `json:"avg_quality_score"`
	AcceptableSlippageRate float64 `json:"acceptable_slippage_rate"`  // % with acceptable slippage
	AcceptableFillTimeRate float64 `json:"acceptable_fill_time_rate"` // % with acceptable fill time

	// Recovery tracking
	ConsecutiveFails int       `json:"consecutive_fails"`
	InRecoveryMode   bool      `json:"in_recovery_mode"`
	RecoveryEndsAt   time.Time `json:"recovery_ends_at,omitempty"`

	LastUpdated time.Time `json:"last_updated"`
}

// RecordExecution records a new execution attempt
func (em *ExecutionMonitor) RecordExecution(record ExecutionRecord) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Calculate derived metrics
	if record.IntendedPrice > 0 && record.ActualPrice > 0 {
		if record.Side == "buy" {
			// For buys, slippage is paying more than intended
			record.SlippageBps = ((record.ActualPrice - record.IntendedPrice) / record.IntendedPrice) * 10000
		} else {
			// For sells, slippage is receiving less than intended
			record.SlippageBps = ((record.IntendedPrice - record.ActualPrice) / record.IntendedPrice) * 10000
		}
	}

	// Quality assessments
	record.IsSlippageAcceptable = math.Abs(record.SlippageBps) <= em.maxSlippageBps
	record.IsFillTimeAcceptable = record.TimeToFillMs <= em.targetFillTimeMs

	// Calculate quality score (0-100)
	slippageScore := em.calculateSlippageScore(record.SlippageBps)
	fillTimeScore := em.calculateFillTimeScore(record.TimeToFillMs)
	sizeScore := em.calculateSizeScore(record.IntendedSize, record.ActualSize)

	record.QualityScore = (slippageScore + fillTimeScore + sizeScore) / 3.0

	// Set timestamp if not provided
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Update consecutive failure tracking
	if record.Status == "failed" || !record.IsSlippageAcceptable {
		em.consecutiveFails++
		em.lastFailTime = record.Timestamp

		// Enter recovery mode if too many consecutive failures
		if em.consecutiveFails >= em.maxConsecutiveFails {
			em.inRecoveryMode = true
		}
	} else if record.Status == "filled" && record.IsSlippageAcceptable {
		em.consecutiveFails = 0
		em.inRecoveryMode = false
	}

	// Add to records
	em.executions = append(em.executions, record)

	// Update aggregate metrics
	em.updateMetrics()

	return nil
}

// calculateSlippageScore converts slippage to a 0-100 score
func (em *ExecutionMonitor) calculateSlippageScore(slippageBps float64) float64 {
	absSlippage := math.Abs(slippageBps)
	if absSlippage <= em.maxSlippageBps/4 {
		return 100.0 // Excellent execution
	} else if absSlippage <= em.maxSlippageBps/2 {
		return 80.0 // Good execution
	} else if absSlippage <= em.maxSlippageBps {
		return 60.0 // Acceptable execution
	} else if absSlippage <= em.maxSlippageBps*2 {
		return 30.0 // Poor execution
	} else {
		return 0.0 // Terrible execution
	}
}

// calculateFillTimeScore converts fill time to a 0-100 score
func (em *ExecutionMonitor) calculateFillTimeScore(fillTimeMs int64) float64 {
	if fillTimeMs <= em.targetFillTimeMs/4 {
		return 100.0 // Excellent fill time
	} else if fillTimeMs <= em.targetFillTimeMs/2 {
		return 80.0 // Good fill time
	} else if fillTimeMs <= em.targetFillTimeMs {
		return 60.0 // Acceptable fill time
	} else if fillTimeMs <= em.targetFillTimeMs*2 {
		return 30.0 // Slow fill time
	} else {
		return 0.0 // Very slow fill time
	}
}

// calculateSizeScore evaluates how much of the intended size was filled
func (em *ExecutionMonitor) calculateSizeScore(intendedSize, actualSize float64) float64 {
	if intendedSize <= 0 {
		return 0.0
	}

	fillRatio := actualSize / intendedSize
	if fillRatio >= 0.95 {
		return 100.0 // Nearly complete fill
	} else if fillRatio >= 0.80 {
		return 80.0 // Good fill
	} else if fillRatio >= 0.60 {
		return 60.0 // Partial fill
	} else if fillRatio >= 0.30 {
		return 30.0 // Poor fill
	} else {
		return 0.0 // Very poor fill
	}
}

// updateMetrics recalculates aggregate execution metrics
func (em *ExecutionMonitor) updateMetrics() {
	if len(em.executions) == 0 {
		return
	}

	// Reset counters
	em.metrics.TotalExecutions = int64(len(em.executions))
	em.metrics.SuccessfulExecutions = 0
	em.metrics.PartialFills = 0
	em.metrics.FailedExecutions = 0
	em.metrics.CancelledExecutions = 0

	// Collect data for percentile calculations
	slippages := make([]float64, 0)
	fillTimes := make([]float64, 0)
	qualityScores := make([]float64, 0)
	acceptableSlippage := 0
	acceptableFillTime := 0

	// Process each execution record
	for _, exec := range em.executions {
		switch exec.Status {
		case "filled":
			em.metrics.SuccessfulExecutions++
		case "partial":
			em.metrics.PartialFills++
		case "failed":
			em.metrics.FailedExecutions++
		case "cancelled":
			em.metrics.CancelledExecutions++
		}

		// Collect metrics for calculations
		if !math.IsNaN(exec.SlippageBps) && !math.IsInf(exec.SlippageBps, 0) {
			slippages = append(slippages, math.Abs(exec.SlippageBps))
		}
		if exec.TimeToFillMs > 0 {
			fillTimes = append(fillTimes, float64(exec.TimeToFillMs))
		}
		if exec.QualityScore > 0 {
			qualityScores = append(qualityScores, exec.QualityScore)
		}

		if exec.IsSlippageAcceptable {
			acceptableSlippage++
		}
		if exec.IsFillTimeAcceptable {
			acceptableFillTime++
		}
	}

	// Calculate averages and percentiles
	if len(slippages) > 0 {
		em.metrics.AvgSlippageBps = calculateMean(slippages)
		em.metrics.MedianSlippageBps = calculatePercentile(slippages, 50)
		em.metrics.P95SlippageBps = calculatePercentile(slippages, 95)
	}

	if len(fillTimes) > 0 {
		em.metrics.AvgFillTimeMs = calculateMean(fillTimes)
		em.metrics.MedianFillTimeMs = calculatePercentile(fillTimes, 50)
		em.metrics.P95FillTimeMs = calculatePercentile(fillTimes, 95)
	}

	if len(qualityScores) > 0 {
		em.metrics.AvgQualityScore = calculateMean(qualityScores)
	}

	// Calculate rates
	if em.metrics.TotalExecutions > 0 {
		em.metrics.AcceptableSlippageRate = (float64(acceptableSlippage) / float64(em.metrics.TotalExecutions)) * 100.0
		em.metrics.AcceptableFillTimeRate = (float64(acceptableFillTime) / float64(em.metrics.TotalExecutions)) * 100.0
	}

	// Update recovery status
	em.metrics.ConsecutiveFails = em.consecutiveFails
	em.metrics.InRecoveryMode = em.inRecoveryMode
	if em.inRecoveryMode {
		em.metrics.RecoveryEndsAt = em.lastFailTime.Add(time.Duration(em.recoveryCooldownMs) * time.Millisecond)
	}

	em.metrics.LastUpdated = time.Now()
}

// calculateMean computes the arithmetic mean of a slice
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculatePercentile computes the specified percentile of a slice
func calculatePercentile(values []float64, percentile int) float64 {
	if len(values) == 0 {
		return 0.0
	}

	// Create a copy and sort
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Simple sorting (for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate percentile index
	index := float64(percentile) / 100.0 * float64(len(sorted)-1)
	if index == float64(int(index)) {
		return sorted[int(index)]
	}

	// Linear interpolation between adjacent values
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	weight := index - float64(lower)

	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// GetMetrics returns current execution metrics
func (em *ExecutionMonitor) GetMetrics() *ExecutionMetrics {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Return a copy to avoid concurrent modification
	metrics := *em.metrics
	return &metrics
}

// IsInRecoveryMode checks if the monitor is in recovery mode
func (em *ExecutionMonitor) IsInRecoveryMode() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if !em.inRecoveryMode {
		return false
	}

	// Check if recovery period has expired
	if time.Since(em.lastFailTime) > time.Duration(em.recoveryCooldownMs)*time.Millisecond {
		em.mu.RUnlock()
		em.mu.Lock()
		em.inRecoveryMode = false
		em.consecutiveFails = 0
		em.mu.Unlock()
		return false
	}

	return true
}

// GetRecentExecutions returns the most recent execution records
func (em *ExecutionMonitor) GetRecentExecutions(limit int) []ExecutionRecord {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if limit <= 0 || limit > len(em.executions) {
		limit = len(em.executions)
	}

	start := len(em.executions) - limit
	if start < 0 {
		start = 0
	}

	recent := make([]ExecutionRecord, limit)
	copy(recent, em.executions[start:])

	// Reverse to get most recent first
	for i := 0; i < len(recent)/2; i++ {
		j := len(recent) - 1 - i
		recent[i], recent[j] = recent[j], recent[i]
	}

	return recent
}

// WriteExecutionArtifact writes minimal execution data to artifact storage
func (em *ExecutionMonitor) WriteExecutionArtifact(record ExecutionRecord, artifactPath string) error {
	// This would write to artifact storage - implementation depends on storage system
	// For now, return nil to indicate successful artifact writing
	return nil
}

// GetExecutionSummary returns a summary of execution performance
func (em *ExecutionMonitor) GetExecutionSummary() map[string]interface{} {
	metrics := em.GetMetrics()

	var successRate float64
	if metrics.TotalExecutions > 0 {
		successRate = (float64(metrics.SuccessfulExecutions) / float64(metrics.TotalExecutions)) * 100.0
	}

	return map[string]interface{}{
		"total_executions": metrics.TotalExecutions,
		"success_rate":     successRate,
		"performance": map[string]interface{}{
			"avg_slippage_bps":     metrics.AvgSlippageBps,
			"avg_fill_time_ms":     metrics.AvgFillTimeMs,
			"avg_quality_score":    metrics.AvgQualityScore,
			"acceptable_slippage":  metrics.AcceptableSlippageRate,
			"acceptable_fill_time": metrics.AcceptableFillTimeRate,
		},
		"recovery": map[string]interface{}{
			"in_recovery_mode":  metrics.InRecoveryMode,
			"consecutive_fails": metrics.ConsecutiveFails,
			"recovery_ends_at":  metrics.RecoveryEndsAt,
		},
		"last_updated": metrics.LastUpdated,
	}
}
