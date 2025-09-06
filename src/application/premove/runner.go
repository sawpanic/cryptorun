package premove

import (
	"fmt"
	"time"

	"cryptorun/src/domain/premove"
)

// Runner orchestrates the premove detection pipeline with portfolio pruning
type Runner struct {
	portfolioManager *premove.PortfolioManager // Legacy - to be deprecated
	portfolioPruner  *premove.PortfolioPruner  // New pruning system
	alertManager     *AlertManager
	executionMonitor *ExecutionMonitor
	backtestEngine   *BacktestEngine
}

// NewRunner creates a new premove pipeline runner
func NewRunner(portfolioManager *premove.PortfolioManager, alertManager *AlertManager, executionMonitor *ExecutionMonitor, backtestEngine *BacktestEngine) *Runner {
	return &Runner{
		portfolioManager: portfolioManager,
		alertManager:     alertManager,
		executionMonitor: executionMonitor,
		backtestEngine:   backtestEngine,
	}
}

// NewRunnerWithPruner creates a runner with the new portfolio pruning system
func NewRunnerWithPruner(portfolioPruner *premove.PortfolioPruner, alertManager *AlertManager, executionMonitor *ExecutionMonitor, backtestEngine *BacktestEngine) *Runner {
	return &Runner{
		portfolioPruner:  portfolioPruner,
		alertManager:     alertManager,
		executionMonitor: executionMonitor,
		backtestEngine:   backtestEngine,
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
	OriginalCandidates   []Candidate                     `json:"original_candidates"`
	PostGatesCandidates  []Candidate                     `json:"post_gates_candidates"`
	PortfolioPruneResult *premove.PortfolioPruningResult `json:"portfolio_prune_result"`
	AlertsGenerated      []AlertRecord                   `json:"alerts_generated"`
	ProcessingTime       time.Duration                   `json:"processing_time"`
	Timestamp            time.Time                       `json:"timestamp"`
	Errors               []string                        `json:"errors,omitempty"`
}

// RunPipeline executes the full premove detection pipeline
func (r *Runner) RunPipeline(candidates []Candidate, existingPositions []premove.Position, correlationMatrix *premove.CorrelationMatrix) (*ProcessingResult, error) {
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

	// Step 2: Portfolio pruning (post-gates, pre-alerts)
	if r.portfolioPruner != nil {
		// Convert candidates to pruning format
		pruningCandidates := make([]premove.Candidate, len(postGatesCandidates))
		for i, candidate := range postGatesCandidates {
			pruningCandidates[i] = premove.Candidate{
				Symbol:      candidate.Symbol,
				Score:       candidate.Score,
				PassedGates: candidate.PassedGates,
				Sector:      candidate.Sector,
				Beta:        candidate.Beta,
				ADV:         candidate.Size * 1000, // Simplified ADV calculation
				Correlation: 0.0,                   // Will be calculated during pruning
			}
		}

		pruneResult := r.portfolioPruner.PrunePortfolio(pruningCandidates, correlationMatrix)
		if pruneResult != nil {
			// Store pruning result in the ProcessingResult
			// Convert back to legacy format for compatibility
			legacyResult := &premove.PortfolioPruningResult{
				Candidates:       make([]premove.Position, 0),
				Accepted:         make([]premove.Position, 0),
				Rejected:         make([]premove.Position, 0),
				RejectionReasons: make(map[string][]string),
				PrunedCount:      pruneResult.Metrics.TotalPruned,
				BetaUtilization:  pruneResult.Metrics.FinalBetaUtilization,
			}

			// Convert kept candidates to accepted positions
			for _, kept := range pruneResult.Kept {
				legacyResult.Accepted = append(legacyResult.Accepted, premove.Position{
					Symbol:      kept.Symbol,
					Score:       kept.Score,
					Sector:      kept.Sector,
					Beta:        kept.Beta,
					Size:        kept.ADV / 1000, // Convert back
					EntryTime:   startTime,
					Correlation: kept.Correlation,
				})
			}

			// Convert pruned candidates to rejected positions
			for _, pruned := range pruneResult.Pruned {
				legacyResult.Rejected = append(legacyResult.Rejected, premove.Position{
					Symbol:      pruned.Symbol,
					Score:       pruned.Score,
					Sector:      pruned.Sector,
					Beta:        pruned.Beta,
					Size:        pruned.ADV / 1000,
					EntryTime:   startTime,
					Correlation: pruned.Correlation,
				})
				legacyResult.RejectionReasons[pruned.Symbol] = []string{pruned.Reason}
			}

			result.PortfolioPruneResult = legacyResult
		}
	} else if r.portfolioManager != nil {
		// Fallback to legacy portfolio manager
		portfolioPositions := make([]premove.Position, len(postGatesCandidates))
		for i, candidate := range postGatesCandidates {
			portfolioPositions[i] = premove.Position{
				Symbol:    candidate.Symbol,
				Score:     candidate.Score,
				Sector:    candidate.Sector,
				Beta:      candidate.Beta,
				Size:      candidate.Size,
				EntryTime: candidate.Timestamp,
			}
		}

		pruneResult, err := r.portfolioManager.PrunePortfolio(portfolioPositions, existingPositions, correlationMatrix)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Portfolio pruning failed: %v", err))
		} else {
			result.PortfolioPruneResult = pruneResult
		}
	}

	// Step 3: Generate alerts (post-portfolio-pruning)
	if r.alertManager != nil {
		alertCandidates := postGatesCandidates
		if result.PortfolioPruneResult != nil {
			// Only alert on accepted positions
			acceptedSymbols := make(map[string]bool)
			for _, pos := range result.PortfolioPruneResult.Accepted {
				acceptedSymbols[pos.Symbol] = true
			}

			alertCandidates = make([]Candidate, 0)
			for _, candidate := range postGatesCandidates {
				if acceptedSymbols[candidate.Symbol] {
					alertCandidates = append(alertCandidates, candidate)
				}
			}
		}

		for _, candidate := range alertCandidates {
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
			"portfolio_pruner":  r.portfolioPruner != nil,
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

// SimulateExecution simulates execution for accepted positions
func (r *Runner) SimulateExecution(acceptedPositions []premove.Position, marketConditions map[string]interface{}) error {
	if r.executionMonitor == nil {
		return fmt.Errorf("execution monitor not initialized")
	}

	for _, position := range acceptedPositions {
		// Simulate execution with some realistic parameters
		record := ExecutionRecord{
			ID:            fmt.Sprintf("sim_%s_%d", position.Symbol, time.Now().Unix()),
			Symbol:        position.Symbol,
			Side:          "buy", // Assuming buy orders for pre-movement
			IntendedPrice: 100.0, // Would come from market data
			IntendedSize:  position.Size,
			ActualPrice:   100.0 + (position.Score/100.0)*0.5, // Simulate slight slippage
			ActualSize:    position.Size * 0.98,               // Simulate partial fill
			TimeToFillMs:  3000,                               // 3 second fill time
			Status:        "filled",
			Exchange:      "kraken",
			Timestamp:     time.Now(),
			OrderType:     "market",
			PreMoveScore:  position.Score,
			TriggerReason: "pre_movement_detected",
			MarketConditions: map[string]float64{
				"volatility":   0.25,
				"volume_surge": 1.8,
			},
		}

		// Record the simulated execution
		if err := r.executionMonitor.RecordExecution(record); err != nil {
			return fmt.Errorf("failed to record execution for %s: %w", position.Symbol, err)
		}
	}

	return nil
}

// RunBacktest executes backtesting on historical data
func (r *Runner) RunBacktest(replayPoints []PITReplayPoint) (*BacktestResult, error) {
	if r.backtestEngine == nil {
		return nil, fmt.Errorf("backtest engine not initialized")
	}

	return r.backtestEngine.RunPITBacktest(replayPoints)
}

// GetPortfolioConstraints returns current portfolio constraints and utilization
func (r *Runner) GetPortfolioConstraints(existingPositions []premove.Position) map[string]interface{} {
	if r.portfolioManager == nil {
		return map[string]interface{}{
			"error": "portfolio manager not initialized",
		}
	}

	return r.portfolioManager.GetPortfolioStatus(existingPositions)
}

// ProcessBatch processes a batch of candidates with rate limiting and error handling
func (r *Runner) ProcessBatch(batches [][]Candidate, existingPositions []premove.Position, correlationMatrix *premove.CorrelationMatrix) ([]ProcessingResult, error) {
	results := make([]ProcessingResult, len(batches))
	errors := make([]string, 0)

	for i, batch := range batches {
		result, err := r.RunPipeline(batch, existingPositions, correlationMatrix)
		if err != nil {
			errors = append(errors, fmt.Sprintf("batch %d failed: %v", i, err))
			// Create empty result for failed batch
			result = &ProcessingResult{
				OriginalCandidates: batch,
				Timestamp:          time.Now(),
				Errors:             []string{err.Error()},
			}
		}
		results[i] = *result

		// Update existing positions for next batch (simplified)
		if result.PortfolioPruneResult != nil {
			for _, accepted := range result.PortfolioPruneResult.Accepted {
				existingPositions = append(existingPositions, accepted)
			}
		}

		// Rate limiting between batches
		time.Sleep(100 * time.Millisecond)
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("batch processing errors: %v", errors)
	}

	return results, nil
}

// ValidateConfiguration validates the runner configuration
func (r *Runner) ValidateConfiguration() error {
	if r.portfolioManager == nil {
		return fmt.Errorf("portfolio manager is required")
	}

	if r.alertManager == nil {
		return fmt.Errorf("alert manager is required")
	}

	if r.executionMonitor == nil {
		return fmt.Errorf("execution monitor is required")
	}

	if r.backtestEngine == nil {
		return fmt.Errorf("backtest engine is required")
	}

	return nil
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

	return metrics
}
