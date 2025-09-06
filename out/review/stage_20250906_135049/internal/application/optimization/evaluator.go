package optimization

import (
	"context"
	"math"
	"sort"
)

// StandardEvaluator implements the Evaluator interface
type StandardEvaluator struct {
	precisionCalculator *PrecisionCalculator
}

// NewStandardEvaluator creates a new standard evaluator
func NewStandardEvaluator() *StandardEvaluator {
	return &StandardEvaluator{
		precisionCalculator: NewPrecisionCalculator(),
	}
}

// EvaluateParameters evaluates a parameter set against historical data
func (se *StandardEvaluator) EvaluateParameters(ctx context.Context, params ParameterSet, data []LedgerEntry) (EvaluationMetrics, error) {
	if len(data) == 0 {
		return EvaluationMetrics{}, nil
	}
	
	// Convert ledger entries to predictions based on parameter set
	predictions := se.convertToPredictions(data, params)
	
	// Calculate precision metrics
	metrics := se.CalculatePrecisionMetrics(predictions)
	
	// Add parameter-specific penalties and adjustments
	metrics = se.applyParameterPenalties(metrics, params)
	
	return metrics, nil
}

// CalculatePrecisionMetrics calculates precision@N metrics from predictions
func (se *StandardEvaluator) CalculatePrecisionMetrics(predictions []Prediction) EvaluationMetrics {
	if len(predictions) == 0 {
		return EvaluationMetrics{}
	}
	
	metrics := EvaluationMetrics{
		TotalPredictions: len(predictions),
	}
	
	// Sort predictions by composite score (descending)
	sortedPredictions := make([]Prediction, len(predictions))
	copy(sortedPredictions, predictions)
	
	sort.Slice(sortedPredictions, func(i, j int) bool {
		return sortedPredictions[i].CompositeScore > sortedPredictions[j].CompositeScore
	})
	
	// Calculate precision@10, @20, @50 for both 24h and 48h
	metrics.Precision10_24h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 10, func(p Prediction) bool { return p.Success24h })
	metrics.Precision10_48h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 10, func(p Prediction) bool { return p.Success48h })
	
	metrics.Precision20_24h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 20, func(p Prediction) bool { return p.Success24h })
	metrics.Precision20_48h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 20, func(p Prediction) bool { return p.Success48h })
	
	metrics.Precision50_24h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 50, func(p Prediction) bool { return p.Success24h })
	metrics.Precision50_48h = se.precisionCalculator.PrecisionAtN(sortedPredictions, 50, func(p Prediction) bool { return p.Success48h })
	
	// Calculate win rates
	metrics.WinRate24h = se.calculateWinRate(predictions, func(p Prediction) bool { return p.Success24h })
	metrics.WinRate48h = se.calculateWinRate(predictions, func(p Prediction) bool { return p.Success48h })
	
	// Calculate false positive rate
	metrics.FalsePositiveRate = se.calculateFalsePositiveRate(predictions)
	
	// Calculate max drawdown penalty
	metrics.MaxDrawdownPenalty = se.calculateMaxDrawdownPenalty(sortedPredictions)
	
	// Count valid predictions (those that passed all gates)
	validCount := 0
	for _, pred := range predictions {
		if pred.Gates.AllPass {
			validCount++
		}
	}
	metrics.ValidPredictions = validCount
	
	return metrics
}

// convertToPredictions converts ledger entries to predictions using parameter set
func (se *StandardEvaluator) convertToPredictions(data []LedgerEntry, params ParameterSet) []Prediction {
	predictions := make([]Prediction, len(data))
	
	// Determine prediction threshold based on target type
	threshold := 75.0 // Default momentum threshold
	if params.Target == TargetDip {
		threshold = 60.0 // Lower threshold for dip patterns
	}
	
	// Extract relevant parameters for prediction logic
	// (In practice, this would apply the parameter set to recalculate composite scores)
	
	for i, entry := range data {
		predictions[i] = Prediction{
			Symbol:         entry.Symbol,
			Timestamp:      entry.TsScan,
			CompositeScore: entry.Composite, // Would be recalculated with new parameters
			Predicted24h:   entry.Composite >= threshold,
			Predicted48h:   entry.Composite >= threshold,
			Actual24h:      entry.Realized.H24,
			Actual48h:      entry.Realized.H48,
			Success24h:     entry.Pass.H24,
			Success48h:     entry.Pass.H48,
			Gates: GateStatus{
				AllPass: entry.GatesPass,
			},
		}
	}
	
	return predictions
}

// applyParameterPenalties applies parameter-specific penalties to metrics
func (se *StandardEvaluator) applyParameterPenalties(metrics EvaluationMetrics, params ParameterSet) EvaluationMetrics {
	// Apply penalties based on parameter choices
	
	switch params.Target {
	case TargetMomentum:
		// Momentum-specific penalties
		metrics = se.applyMomentumPenalties(metrics, params)
	case TargetDip:
		// Dip-specific penalties
		metrics = se.applyDipPenalties(metrics, params)
	}
	
	return metrics
}

// applyMomentumPenalties applies momentum-specific penalties
func (se *StandardEvaluator) applyMomentumPenalties(metrics EvaluationMetrics, params ParameterSet) EvaluationMetrics {
	penalty := 0.0
	
	// Penalty for extreme weight concentrations
	regimes := []string{"bull", "choppy", "high_vol"}
	for _, regime := range regimes {
		maxWeight := 0.0
		for _, tf := range []string{"1h", "4h", "12h", "24h", "7d"} {
			paramName := regime + "_weight_" + tf
			if param, exists := params.Parameters[paramName]; exists {
				if weight, ok := param.Value.(float64); ok && weight > maxWeight {
					maxWeight = weight
				}
			}
		}
		
		// Penalty if any single timeframe weight > 50%
		if maxWeight > 0.5 {
			penalty += 0.05 * (maxWeight - 0.5) // Linear penalty
		}
	}
	
	// Apply penalty to false positive rate
	metrics.FalsePositiveRate += penalty
	
	return metrics
}

// applyDipPenalties applies dip-specific penalties
func (se *StandardEvaluator) applyDipPenalties(metrics EvaluationMetrics, params ParameterSet) EvaluationMetrics {
	penalty := 0.0
	
	// Penalty for extreme RSI triggers (too aggressive)
	if param, exists := params.Parameters["rsi_trigger_1h"]; exists {
		if rsi, ok := param.Value.(float64); ok {
			if rsi < 20 { // Very oversold
				penalty += 0.02 * (20 - rsi) / 10 // Scaled penalty
			}
		}
	}
	
	// Penalty for very deep dip requirements (might miss opportunities)
	if param, exists := params.Parameters["dip_depth_min"]; exists {
		if depth, ok := param.Value.(float64); ok {
			if depth < -15 { // Very deep dips
				penalty += 0.01 * (math.Abs(depth) - 15) / 5
			}
		}
	}
	
	// Apply penalty to false positive rate
	metrics.FalsePositiveRate += penalty
	
	return metrics
}

// calculateWinRate calculates win rate using success function
func (se *StandardEvaluator) calculateWinRate(predictions []Prediction, successFn func(Prediction) bool) float64 {
	if len(predictions) == 0 {
		return 0.0
	}
	
	wins := 0
	for _, pred := range predictions {
		if successFn(pred) {
			wins++
		}
	}
	
	return float64(wins) / float64(len(predictions))
}

// calculateFalsePositiveRate calculates false positive rate
func (se *StandardEvaluator) calculateFalsePositiveRate(predictions []Prediction) float64 {
	if len(predictions) == 0 {
		return 0.0
	}
	
	positives := 0
	falsePositives := 0
	
	for _, pred := range predictions {
		if pred.Predicted24h || pred.Predicted48h {
			positives++
			if !pred.Success24h && !pred.Success48h {
				falsePositives++
			}
		}
	}
	
	if positives == 0 {
		return 0.0
	}
	
	return float64(falsePositives) / float64(positives)
}

// calculateMaxDrawdownPenalty calculates penalty based on maximum drawdown
func (se *StandardEvaluator) calculateMaxDrawdownPenalty(sortedPredictions []Prediction) float64 {
	if len(sortedPredictions) == 0 {
		return 0.0
	}
	
	// Calculate running P&L to find maximum drawdown
	runningPnL := 0.0
	maxPnL := 0.0
	maxDrawdown := 0.0
	
	for _, pred := range sortedPredictions {
		// Simplified P&L calculation (would be more sophisticated in practice)
		pnl := pred.Actual24h
		if pred.Predicted24h {
			runningPnL += pnl
		}
		
		if runningPnL > maxPnL {
			maxPnL = runningPnL
		}
		
		drawdown := maxPnL - runningPnL
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	
	// Convert drawdown to penalty (scaled)
	return maxDrawdown * 0.1 // Penalty factor
}

// PrecisionCalculator calculates precision@N metrics
type PrecisionCalculator struct{}

// NewPrecisionCalculator creates a new precision calculator
func NewPrecisionCalculator() *PrecisionCalculator {
	return &PrecisionCalculator{}
}

// PrecisionAtN calculates precision at top N predictions
func (pc *PrecisionCalculator) PrecisionAtN(sortedPredictions []Prediction, n int, successFn func(Prediction) bool) float64 {
	if len(sortedPredictions) == 0 || n <= 0 {
		return 0.0
	}
	
	// Take top N predictions
	limit := n
	if limit > len(sortedPredictions) {
		limit = len(sortedPredictions)
	}
	
	topN := sortedPredictions[:limit]
	
	successes := 0
	for _, pred := range topN {
		if successFn(pred) {
			successes++
		}
	}
	
	return float64(successes) / float64(len(topN))
}

// RecallAtN calculates recall at top N predictions
func (pc *PrecisionCalculator) RecallAtN(sortedPredictions []Prediction, n int, successFn func(Prediction) bool) float64 {
	if len(sortedPredictions) == 0 || n <= 0 {
		return 0.0
	}
	
	// Count total true positives in entire dataset
	totalTruePositives := 0
	for _, pred := range sortedPredictions {
		if successFn(pred) {
			totalTruePositives++
		}
	}
	
	if totalTruePositives == 0 {
		return 0.0
	}
	
	// Count true positives in top N
	limit := n
	if limit > len(sortedPredictions) {
		limit = len(sortedPredictions)
	}
	
	topN := sortedPredictions[:limit]
	topNTruePositives := 0
	for _, pred := range topN {
		if successFn(pred) {
			topNTruePositives++
		}
	}
	
	return float64(topNTruePositives) / float64(totalTruePositives)
}