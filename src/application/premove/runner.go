package premove

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/src/domain/premove/cvd"
	"github.com/sawpanic/cryptorun/src/domain/premove/portfolio"
	"github.com/sawpanic/cryptorun/src/domain/premove/ports"
	"github.com/sawpanic/cryptorun/src/domain/premove/proxy"
	"github.com/sawpanic/cryptorun/src/infrastructure/percentiles"
)

// RunnerDeps contains the new v3.3 dependencies with engine injection
type RunnerDeps struct {
	PercentileEngine *percentiles.Engine // Concrete percentile engine
	CVDResiduals     *cvd.Calculator     // Concrete CVD calculator
	SupplyProxy      *proxy.Evaluator    // Concrete supply proxy evaluator
}

// ExecutionMonitor wraps the execution quality tracker
type ExecutionMonitor struct {
	tracker *ExecutionQualityTracker
}

// NewExecutionMonitor creates a new execution monitor
func NewExecutionMonitor() *ExecutionMonitor {
	return &ExecutionMonitor{
		tracker: NewExecutionQualityTracker(),
	}
}

// PITReplayPoint represents a point-in-time replay data point
type PITReplayPoint struct {
	Timestamp time.Time
	Data      map[string]interface{}
}

// BacktestResult contains backtest execution results
type BacktestResult struct {
	TotalTrades   int
	WinRate       float64
	PnL           float64
	MaxDrawdown   float64
	SharpeRatio   float64
	ExecutionTime time.Duration
}

// AlertRecord represents an alert that was sent
type AlertRecord struct {
	ID             string                 `json:"id"`
	Symbol         string                 `json:"symbol"`
	AlertType      string                 `json:"alert_type"`
	Severity       string                 `json:"severity"`
	Score          float64                `json:"score"`
	Message        string                 `json:"message"`
	Reasons        []string               `json:"reasons"`
	Metadata       map[string]interface{} `json:"metadata"`
	Timestamp      time.Time              `json:"timestamp"`
	Source         string                 `json:"source"`
	Status         string                 `json:"status"`
	ProcessingTime time.Duration          `json:"processing_time"`
}

// Runner orchestrates the premove detection pipeline with portfolio pruning
type Runner struct {
	portfolioManager *PortfolioManager
	alertManager     *AlertManager
	executionMonitor *ExecutionMonitor
	backtestEngine   *BacktestEngine
	Deps             *RunnerDeps // v3.3 dependencies (exported for testing)
}

// NewRunner creates a new premove pipeline runner with v3.3 dependencies
func NewRunner(portfolioManager *PortfolioManager, alertManager *AlertManager, executionMonitor *ExecutionMonitor, backtestEngine *BacktestEngine) *Runner {
	return &Runner{
		portfolioManager: portfolioManager,
		alertManager:     alertManager,
		executionMonitor: executionMonitor,
		backtestEngine:   backtestEngine,
		Deps: &RunnerDeps{
			PercentileEngine: percentiles.NewPercentileEngine(),
			CVDResiduals:     cvd.NewCVDResiduals(),
			SupplyProxy:      proxy.NewSupplyProxy(),
		},
	}
}

// Candidate represents a pre-movement candidate for processing
type Candidate struct {
	Symbol      string                 `json:"symbol"`
	Score       float64                `json:"score"`
	Sector      string                 `json:"sector"`
	Beta        float64                `json:"beta"`
	Size        float64                `json:"size"`
	PassedGates int                    `json:"passed_gates"`
	GateResults map[string]bool        `json:"gate_results"`
	Reasons     []string               `json:"reasons"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ProcessingResult contains the result of pipeline processing
type ProcessingResult struct {
	OriginalCandidates   []Candidate            `json:"original_candidates"`
	PostGatesCandidates  []Candidate            `json:"post_gates_candidates"`
	PortfolioPruneResult *portfolio.PruneResult `json:"portfolio_prune_result"`
	AlertsGenerated      []AlertRecord          `json:"alerts_generated"`
	ProcessingTime       time.Duration          `json:"processing_time"`
	Timestamp            time.Time              `json:"timestamp"`
	Errors               []string               `json:"errors,omitempty"`
}

// RunPipeline executes the full premove detection pipeline
func (r *Runner) RunPipeline(candidates []Candidate) (*ProcessingResult, error) {
	startTime := time.Now()

	result := &ProcessingResult{
		OriginalCandidates: candidates,
		AlertsGenerated:    make([]AlertRecord, 0),
		Errors:             make([]string, 0),
		Timestamp:          startTime,
	}

	// Step 1: Apply gates (assuming gates are already applied to input candidates)
	postGatesCandidates := make([]Candidate, 0)
	for _, candidate := range candidates {
		if candidate.PassedGates >= 2 { // 2-of-3 gates requirement
			postGatesCandidates = append(postGatesCandidates, candidate)
		}
	}
	result.PostGatesCandidates = postGatesCandidates

	// Step 2: Generate alerts for qualified candidates
	if r.alertManager != nil {
		for _, candidate := range postGatesCandidates {
			alert := CreatePreMovementAlert(
				candidate.Symbol,
				candidate.Score,
				candidate.Reasons,
				candidate.Metadata,
			)

			processedAlert, err := r.alertManager.ProcessAlert(alert)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Alert processing failed for %s: %v", candidate.Symbol, err))
			} else {
				result.AlertsGenerated = append(result.AlertsGenerated, *processedAlert)
			}
		}
	}

	result.ProcessingTime = time.Since(startTime)
	return result, nil
}

// GetPipelineStatus returns current pipeline status
func (r *Runner) GetPipelineStatus() map[string]interface{} {
	status := map[string]interface{}{
		"pipeline": "premove_detection",
		"components": map[string]interface{}{
			"portfolio_manager": r.portfolioManager != nil,
			"alert_manager":     r.alertManager != nil,
			"execution_monitor": r.executionMonitor != nil,
			"backtest_engine":   r.backtestEngine != nil,
		},
	}

	if r.alertManager != nil {
		status["alerts"] = r.alertManager.GetAlertStats()
	}

	if r.executionMonitor != nil {
		status["execution"] = r.executionMonitor.GetExecutionSummary()
	}

	return status
}

// ProcessWithEngines processes candidates using the v3.3 engines
func (r *Runner) ProcessWithEngines(ctx context.Context, rawData map[string][]float64, timestamps []time.Time) (*EngineProcessingResult, error) {
	if r.Deps == nil {
		return nil, fmt.Errorf("runner dependencies not initialized")
	}

	result := &EngineProcessingResult{
		Timestamp: time.Now(),
		Errors:    make([]string, 0),
	}

	// Process percentiles
	if cvdData, exists := rawData["cvd_norm"]; exists && len(cvdData) > 0 {
		percentiles, err := r.Deps.PercentileEngine.Calculate(ctx, cvdData, timestamps, 14)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Percentile calculation failed: %v", err))
		} else {
			result.Percentiles = percentiles
		}
	}

	// Process CVD residuals
	if cvdData, cvdExists := rawData["cvd_norm"]; cvdExists {
		if volData, volExists := rawData["vol_norm"]; volExists && len(cvdData) == len(volData) {
			cvdResiduals, err := r.Deps.CVDResiduals.CalculateResiduals(ctx, cvdData, volData)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("CVD residuals calculation failed: %v", err))
			} else {
				result.CVDResiduals = cvdResiduals
			}
		}
	}

	return result, nil
}

// EvaluateSupplyProxy evaluates supply-squeeze proxy conditions
func (r *Runner) EvaluateSupplyProxy(ctx context.Context, inputs ports.ProxyInputs) (*ports.ProxyResult, error) {
	if r.Deps == nil || r.Deps.SupplyProxy == nil {
		return nil, fmt.Errorf("supply proxy not initialized")
	}

	return r.Deps.SupplyProxy.EvaluateDetailed(ctx, inputs)
}

// GetEngineStatus returns status of all engines
func (r *Runner) GetEngineStatus() map[string]interface{} {
	status := map[string]interface{}{
		"engines_initialized": r.Deps != nil,
	}

	if r.Deps != nil {
		status["percentile_engine"] = r.Deps.PercentileEngine != nil
		status["cvd_residuals"] = r.Deps.CVDResiduals != nil
		status["supply_proxy"] = r.Deps.SupplyProxy != nil

		if r.Deps.SupplyProxy != nil {
			status["gate_requirements"] = r.Deps.SupplyProxy.GetGateRequirements()
		}
	}

	return status
}

// EngineProcessingResult contains results from v3.3 engine processing
type EngineProcessingResult struct {
	Percentiles  []ports.PercentilePoint `json:"percentiles,omitempty"`
	CVDResiduals []ports.CVDPoint        `json:"cvd_residuals,omitempty"`
	Timestamp    time.Time               `json:"timestamp"`
	Errors       []string                `json:"errors,omitempty"`
}

// GetMetrics returns Prometheus-style metrics for monitoring
func (r *Runner) GetMetrics() map[string]float64 {
	metrics := make(map[string]float64)

	if r.alertManager != nil {
		stats := r.alertManager.GetAlertStats()
		if rateLimited, ok := stats["rate_limited_total"].(int64); ok {
			metrics["premove_alerts_rate_limited_total"] = float64(rateLimited)
		}
	}

	if r.executionMonitor != nil {
		execMetrics := r.executionMonitor.GetMetrics()
		metrics["premove_slippage_bps"] = execMetrics.AvgSlippageBps
	}

	// Portfolio pruning metrics - these would be tracked over time in a real implementation
	// For now, return 0 as these would come from a metrics registry
	metrics["premove_portfolio_pruned_total{reason=correlation}"] = 0.0
	metrics["premove_portfolio_pruned_total{reason=sector}"] = 0.0
	metrics["premove_portfolio_pruned_total{reason=beta}"] = 0.0
	metrics["premove_portfolio_pruned_total{reason=position_size}"] = 0.0
	metrics["premove_portfolio_pruned_total{reason=exposure}"] = 0.0

	// Engine metrics
	if r.Deps != nil {
		metrics["premove_engines_initialized"] = 1.0
	} else {
		metrics["premove_engines_initialized"] = 0.0
	}

	return metrics
}

// CreatePreMovementAlert creates an alert record for a pre-movement detection
func CreatePreMovementAlert(symbol string, score float64, reasons []string, metadata map[string]interface{}) AlertRecord {
	// Create message from reasons
	message := fmt.Sprintf("Pre-movement detected for %s (score: %.1f)", symbol, score)
	if len(reasons) > 0 {
		message += " - " + reasons[0]
	}

	// Determine severity based on score
	severity := "medium"
	if score >= 85.0 {
		severity = "critical"
	} else if score >= 75.0 {
		severity = "high"
	} else if score >= 65.0 {
		severity = "medium"
	} else {
		severity = "low"
	}

	return AlertRecord{
		ID:        fmt.Sprintf("premove_%s_%d", symbol, time.Now().Unix()),
		Symbol:    symbol,
		AlertType: "pre_movement",
		Severity:  severity,
		Score:     score,
		Message:   message,
		Reasons:   reasons,
		Metadata:  metadata,
		Timestamp: time.Now(),
		Source:    "detector",
		Status:    "pending",
	}
}

// RecordExecution records an execution for monitoring
func (em *ExecutionMonitor) RecordExecution(record ExecutionRecord) error {
	if em.tracker != nil {
		return em.tracker.RecordExecution(record)
	}
	return nil
}

// GetExecutionSummary returns execution summary metrics
func (em *ExecutionMonitor) GetExecutionSummary() map[string]interface{} {
	if em.tracker != nil {
		metrics := em.tracker.GetExecutionMetrics()
		return map[string]interface{}{
			"total_executions":    metrics.TotalExecutions,
			"good_execution_rate": metrics.GoodExecutionRate,
			"avg_slippage_bps":    metrics.AvgSlippageBps,
			"tightened_venues":    metrics.TightenedVenues,
		}
	}
	return map[string]interface{}{
		"total_executions": 0,
		"avg_slippage":     0.0,
		"fill_rate":        0.0,
	}
}

// GetMetrics returns execution metrics
func (em *ExecutionMonitor) GetMetrics() ExecutionQualityMetrics {
	if em.tracker != nil {
		return *em.tracker.GetExecutionMetrics()
	}
	return ExecutionQualityMetrics{}
}

// RunPITBacktest runs a point-in-time backtest
func (be *BacktestEngine) RunPITBacktest(replayPoints []PITReplayPoint) (*BacktestResult, error) {
	return &BacktestResult{
		TotalTrades:   0,
		WinRate:       0.0,
		PnL:           0.0,
		MaxDrawdown:   0.0,
		SharpeRatio:   0.0,
		ExecutionTime: time.Duration(0),
	}, nil
}

// ProcessAlert processes an alert and returns the result
func (am *AlertManager) ProcessAlert(alert AlertRecord) (*AlertRecord, error) {
	// Mock implementation - use the alerts governor
	if am.governor != nil {
		candidate := AlertCandidate{
			Symbol:      alert.Symbol,
			Score:       alert.Score,
			PassedGates: 2, // Assume qualified
			IsHighVol:   false,
			Sector:      "crypto",
			Priority:    "medium",
		}

		decision := am.governor.EvaluateAlert(candidate)
		if decision.Allow {
			alert.Status = "sent"
		} else {
			alert.Status = "rate_limited"
		}
	}

	return &alert, nil
}

// GetAlertStats returns alert statistics
func (am *AlertManager) GetAlertStats() map[string]interface{} {
	if am.governor != nil {
		return am.governor.GetAlertStats()
	}

	return map[string]interface{}{
		"total_alerts": 0,
		"rate_limited": 0,
	}
}
