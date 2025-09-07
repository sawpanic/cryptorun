package optimization

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
	
	"github.com/rs/zerolog/log"
)

// MomentumOptimizer optimizes momentum parameters within policy bounds
type MomentumOptimizer struct {
	*BaseOptimizer
	bounds MomentumParameterBounds
}

// MomentumParameterBounds defines the search space for momentum parameters
type MomentumParameterBounds struct {
	// Regime momentum weights (must sum to 1.0)
	Weight1h_Min  float64 `json:"weight_1h_min"`
	Weight1h_Max  float64 `json:"weight_1h_max"`
	Weight4h_Min  float64 `json:"weight_4h_min"`
	Weight4h_Max  float64 `json:"weight_4h_max"`
	Weight12h_Min float64 `json:"weight_12h_min"`
	Weight12h_Max float64 `json:"weight_12h_max"`
	Weight24h_Min float64 `json:"weight_24h_min"`
	Weight24h_Max float64 `json:"weight_24h_max"`
	Weight7d_Min  float64 `json:"weight_7d_min"`  // Optional, bull market only
	Weight7d_Max  float64 `json:"weight_7d_max"`
	
	// Acceleration parameters
	AccelEMASpans []int `json:"accel_ema_spans"` // {3,5,8,13}
	RobustSmoothing bool `json:"robust_smoothing_toggle"`
	
	// ATR parameters
	ATRLookbacks []int `json:"atr_lookbacks"` // {14,20,28}
	VolumeConfirm bool `json:"volume_confirm_toggle"`
	
	// Movement thresholds by regime
	BullThreshold_Min  float64 `json:"bull_threshold_min"`   // ≥2.5%
	ChoppyThreshold_Min float64 `json:"choppy_threshold_min"` // ≥3.0%
	BearThreshold_Min   float64 `json:"bear_threshold_min"`   // ≥4.0%
}

// DefaultMomentumBounds returns the default parameter bounds
func DefaultMomentumBounds() MomentumParameterBounds {
	return MomentumParameterBounds{
		Weight1h_Min:  0.15,
		Weight1h_Max:  0.25,
		Weight4h_Min:  0.30,
		Weight4h_Max:  0.40,
		Weight12h_Min: 0.25,
		Weight12h_Max: 0.35,
		Weight24h_Min: 0.10,
		Weight24h_Max: 0.15,
		Weight7d_Min:  0.0,
		Weight7d_Max:  0.10,
		
		AccelEMASpans: []int{3, 5, 8, 13},
		RobustSmoothing: true,
		
		ATRLookbacks: []int{14, 20, 28},
		VolumeConfirm: true,
		
		BullThreshold_Min:  2.5,
		ChoppyThreshold_Min: 3.0,
		BearThreshold_Min:   4.0,
	}
}

// NewMomentumOptimizer creates a new momentum optimizer
func NewMomentumOptimizer(config OptimizerConfig, provider DataProvider, evaluator Evaluator) *MomentumOptimizer {
	return &MomentumOptimizer{
		BaseOptimizer: NewBaseOptimizer(config, provider, evaluator),
		bounds:       DefaultMomentumBounds(),
	}
}

// Optimize performs momentum parameter optimization
func (mo *MomentumOptimizer) Optimize(ctx context.Context, config OptimizerConfig) (*OptimizationResult, error) {
	log.Info().Msg("Starting momentum parameter optimization")
	
	startTime := time.Now()
	
	// Create time series cross-validation folds
	folds, err := mo.createTSCVFolds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create CV folds: %w", err)
	}
	
	log.Info().Int("folds", len(folds)).Msg("Created time series CV folds")
	
	// Initialize random search
	rand.Seed(config.RandomSeed)
	bestResult := &OptimizationResult{
		StartTime: startTime,
		Target:    TargetMomentum,
		AggregateMetrics: EvaluationMetrics{ObjectiveScore: math.Inf(-1)},
	}
	
	// Random search with bounded parameters
	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Generate random parameter set within bounds
		params := mo.generateRandomParameters()
		
		// Validate parameters
		validation := mo.ValidateParameters(params)
		if !validation.Valid {
			log.Debug().Strs("errors", validation.Errors).Msg("Invalid parameters, skipping")
			continue
		}
		
		// Evaluate parameters across all folds
		cvResults, err := mo.evaluateParametersOnFolds(ctx, params, folds)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to evaluate parameters")
			continue
		}
		
		// Calculate aggregate metrics
		aggMetrics := mo.aggregateMetrics(cvResults)
		aggMetrics.ObjectiveScore = CalculateObjective(aggMetrics)
		
		// Check if this is the best result so far
		if aggMetrics.ObjectiveScore > bestResult.AggregateMetrics.ObjectiveScore {
			bestResult = &OptimizationResult{
				ID:               GenerateParameterID(params),
				Target:          TargetMomentum,
				Parameters:      params,
				CVResults:       cvResults,
				AggregateMetrics: aggMetrics,
				RegimeMetrics:   mo.calculateRegimeMetrics(cvResults),
				Stability:       mo.calculateStabilityMetrics(cvResults),
				StartTime:       startTime,
			}
			
			log.Info().
				Int("iteration", iteration).
				Float64("objective", aggMetrics.ObjectiveScore).
				Float64("precision_24h", aggMetrics.Precision20_24h).
				Float64("precision_48h", aggMetrics.Precision20_48h).
				Msg("New best parameter set found")
		}
		
		if iteration%100 == 0 {
			log.Info().Int("iteration", iteration).Int("total", config.MaxIterations).
				Float64("best_objective", bestResult.AggregateMetrics.ObjectiveScore).
				Msg("Optimization progress")
		}
	}
	
	bestResult.EndTime = time.Now()
	bestResult.Duration = bestResult.EndTime.Sub(bestResult.StartTime)
	
	log.Info().
		Float64("best_objective", bestResult.AggregateMetrics.ObjectiveScore).
		Float64("precision_24h", bestResult.AggregateMetrics.Precision20_24h).
		Float64("precision_48h", bestResult.AggregateMetrics.Precision20_48h).
		Dur("duration", bestResult.Duration).
		Msg("Momentum optimization completed")
	
	return bestResult, nil
}

// generateRandomParameters generates a random parameter set within bounds
func (mo *MomentumOptimizer) generateRandomParameters() ParameterSet {
	params := ParameterSet{
		Target:     TargetMomentum,
		Parameters: make(map[string]Parameter),
		Timestamp:  time.Now(),
	}
	
	regimes := []string{"bull", "choppy", "high_vol"}
	
	// Generate regime weights that sum to 1.0
	for _, regime := range regimes {
		weights := mo.generateValidWeights()
		
		params.Parameters[fmt.Sprintf("%s_weight_1h", regime)] = Parameter{
			Name: fmt.Sprintf("%s_weight_1h", regime),
			Value: weights[0],
			Min: mo.bounds.Weight1h_Min,
			Max: mo.bounds.Weight1h_Max,
			Type: "float",
		}
		
		params.Parameters[fmt.Sprintf("%s_weight_4h", regime)] = Parameter{
			Name: fmt.Sprintf("%s_weight_4h", regime),
			Value: weights[1],
			Min: mo.bounds.Weight4h_Min,
			Max: mo.bounds.Weight4h_Max,
			Type: "float",
		}
		
		params.Parameters[fmt.Sprintf("%s_weight_12h", regime)] = Parameter{
			Name: fmt.Sprintf("%s_weight_12h", regime),
			Value: weights[2],
			Min: mo.bounds.Weight12h_Min,
			Max: mo.bounds.Weight12h_Max,
			Type: "float",
		}
		
		params.Parameters[fmt.Sprintf("%s_weight_24h", regime)] = Parameter{
			Name: fmt.Sprintf("%s_weight_24h", regime),
			Value: weights[3],
			Min: mo.bounds.Weight24h_Min,
			Max: mo.bounds.Weight24h_Max,
			Type: "float",
		}
		
		// 7d weight only for bull regime
		if regime == "bull" && len(weights) > 4 {
			params.Parameters[fmt.Sprintf("%s_weight_7d", regime)] = Parameter{
				Name: fmt.Sprintf("%s_weight_7d", regime),
				Value: weights[4],
				Min: mo.bounds.Weight7d_Min,
				Max: mo.bounds.Weight7d_Max,
				Type: "float",
			}
		}
	}
	
	// Acceleration EMA span
	params.Parameters["accel_ema_span"] = Parameter{
		Name: "accel_ema_span",
		Value: mo.bounds.AccelEMASpans[rand.Intn(len(mo.bounds.AccelEMASpans))],
		Options: mo.intSliceToInterface(mo.bounds.AccelEMASpans),
		Type: "discrete",
	}
	
	// Robust smoothing toggle
	params.Parameters["robust_smoothing"] = Parameter{
		Name: "robust_smoothing",
		Value: rand.Float64() > 0.5,
		Type: "bool",
	}
	
	// ATR lookback
	params.Parameters["atr_lookback"] = Parameter{
		Name: "atr_lookback",
		Value: mo.bounds.ATRLookbacks[rand.Intn(len(mo.bounds.ATRLookbacks))],
		Options: mo.intSliceToInterface(mo.bounds.ATRLookbacks),
		Type: "discrete",
	}
	
	// Volume confirmation toggle
	params.Parameters["volume_confirm"] = Parameter{
		Name: "volume_confirm",
		Value: rand.Float64() > 0.5,
		Type: "bool",
	}
	
	// Movement thresholds (respect minimums)
	params.Parameters["bull_threshold"] = Parameter{
		Name: "bull_threshold",
		Value: mo.bounds.BullThreshold_Min + rand.Float64()*2.0, // 2.5% to 4.5%
		Min: mo.bounds.BullThreshold_Min,
		Type: "float",
	}
	
	params.Parameters["choppy_threshold"] = Parameter{
		Name: "choppy_threshold", 
		Value: mo.bounds.ChoppyThreshold_Min + rand.Float64()*2.0, // 3.0% to 5.0%
		Min: mo.bounds.ChoppyThreshold_Min,
		Type: "float",
	}
	
	params.Parameters["bear_threshold"] = Parameter{
		Name: "bear_threshold",
		Value: mo.bounds.BearThreshold_Min + rand.Float64()*2.0, // 4.0% to 6.0%
		Min: mo.bounds.BearThreshold_Min,
		Type: "float",
	}
	
	params.ID = GenerateParameterID(params)
	return params
}

// generateValidWeights generates weights that sum to 1.0 and respect bounds
func (mo *MomentumOptimizer) generateValidWeights() []float64 {
	maxAttempts := 1000
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate random weights within bounds
		w1h := mo.bounds.Weight1h_Min + rand.Float64()*(mo.bounds.Weight1h_Max-mo.bounds.Weight1h_Min)
		w4h := mo.bounds.Weight4h_Min + rand.Float64()*(mo.bounds.Weight4h_Max-mo.bounds.Weight4h_Min)
		w12h := mo.bounds.Weight12h_Min + rand.Float64()*(mo.bounds.Weight12h_Max-mo.bounds.Weight12h_Min)
		w24h := mo.bounds.Weight24h_Min + rand.Float64()*(mo.bounds.Weight24h_Max-mo.bounds.Weight24h_Min)
		w7d := mo.bounds.Weight7d_Min + rand.Float64()*(mo.bounds.Weight7d_Max-mo.bounds.Weight7d_Min)
		
		weights := []float64{w1h, w4h, w12h, w24h, w7d}
		sum := w1h + w4h + w12h + w24h + w7d
		
		// Normalize to sum to 1.0
		for i := range weights {
			weights[i] /= sum
		}
		
		// Check if normalized weights still respect bounds (with small tolerance)
		tolerance := 0.02
		if weights[0] >= mo.bounds.Weight1h_Min-tolerance && weights[0] <= mo.bounds.Weight1h_Max+tolerance &&
		   weights[1] >= mo.bounds.Weight4h_Min-tolerance && weights[1] <= mo.bounds.Weight4h_Max+tolerance &&
		   weights[2] >= mo.bounds.Weight12h_Min-tolerance && weights[2] <= mo.bounds.Weight12h_Max+tolerance &&
		   weights[3] >= mo.bounds.Weight24h_Min-tolerance && weights[3] <= mo.bounds.Weight24h_Max+tolerance &&
		   weights[4] >= mo.bounds.Weight7d_Min-tolerance && weights[4] <= mo.bounds.Weight7d_Max+tolerance {
			return weights
		}
	}
	
	// Fallback: use midpoint of bounds and normalize
	w1h := (mo.bounds.Weight1h_Min + mo.bounds.Weight1h_Max) / 2
	w4h := (mo.bounds.Weight4h_Min + mo.bounds.Weight4h_Max) / 2
	w12h := (mo.bounds.Weight12h_Min + mo.bounds.Weight12h_Max) / 2
	w24h := (mo.bounds.Weight24h_Min + mo.bounds.Weight24h_Max) / 2
	w7d := (mo.bounds.Weight7d_Min + mo.bounds.Weight7d_Max) / 2
	
	weights := []float64{w1h, w4h, w12h, w24h, w7d}
	sum := w1h + w4h + w12h + w24h + w7d
	
	for i := range weights {
		weights[i] /= sum
	}
	
	return weights
}

// intSliceToInterface converts []int to []interface{}
func (mo *MomentumOptimizer) intSliceToInterface(ints []int) []interface{} {
	result := make([]interface{}, len(ints))
	for i, v := range ints {
		result[i] = v
	}
	return result
}

// createTSCVFolds creates time series cross-validation folds with purged gaps
func (mo *MomentumOptimizer) createTSCVFolds(ctx context.Context) ([]CVFold, error) {
	// Get all available data to determine time range
	allData, err := mo.dataProvider.GetLedgerData(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger data: %w", err)
	}
	
	if len(allData) < mo.config.MinimumSamples {
		return nil, fmt.Errorf("insufficient data: got %d samples, need %d", len(allData), mo.config.MinimumSamples)
	}
	
	// Sort by timestamp
	// Note: This would typically use sort.Slice, but we'll implement a simple sort
	// since we need to respect the data structure
	
	startTime := allData[0].TsScan
	endTime := allData[len(allData)-1].TsScan
	
	folds := []CVFold{}
	foldIndex := 0
	
	// Walk-forward time series splits
	for trainStart := startTime; trainStart.Add(mo.config.WalkForwardWindow).Before(endTime); foldIndex++ {
		trainEnd := trainStart.Add(mo.config.WalkForwardWindow)
		testStart := trainEnd.Add(mo.config.PurgeGap) // Purge gap to prevent leakage
		testEnd := testStart.Add(mo.config.WalkForwardWindow / 4) // Test window is 1/4 of train window
		
		if testEnd.After(endTime) {
			break
		}
		
		fold := CVFold{
			Index:      foldIndex,
			TrainStart: trainStart,
			TrainEnd:   trainEnd,
			TestStart:  testStart,
			TestEnd:    testEnd,
		}
		
		folds = append(folds, fold)
		
		// Move to next fold (overlapping windows)
		trainStart = trainStart.Add(mo.config.WalkForwardWindow / 2)
	}
	
	if len(folds) < 3 {
		return nil, fmt.Errorf("insufficient time range for cross-validation: got %d folds, need at least 3", len(folds))
	}
	
	return folds, nil
}

// evaluateParametersOnFolds evaluates parameters across all CV folds
func (mo *MomentumOptimizer) evaluateParametersOnFolds(ctx context.Context, params ParameterSet, folds []CVFold) ([]CVFoldResult, error) {
	results := make([]CVFoldResult, len(folds))
	
	for i, fold := range folds {
		// Get test data for this fold's time range
		testData, err := mo.dataProvider.GetLedgerData(ctx, fold.TestStart, fold.TestEnd)
		if err != nil {
			results[i] = CVFoldResult{
				Fold: i,
				TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
				TestPeriod: TimeRange{Start: fold.TestStart, End: fold.TestEnd},
				Error: fmt.Sprintf("failed to get test data: %v", err),
			}
			continue
		}
		
		// Evaluate parameters on test data
		metrics, err := mo.evaluator.EvaluateParameters(ctx, params, testData)
		if err != nil {
			results[i] = CVFoldResult{
				Fold: i,
				TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
				TestPeriod: TimeRange{Start: fold.TestStart, End: fold.TestEnd},
				Error: fmt.Sprintf("failed to evaluate parameters: %v", err),
			}
			continue
		}
		
		// Convert ledger data to predictions for detailed analysis
		predictions := mo.ledgerToPredictions(testData)
		
		results[i] = CVFoldResult{
			Fold: i,
			TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
			TestPeriod: TimeRange{Start: fold.TestStart, End: fold.TestEnd},
			Metrics: metrics,
			Predictions: predictions,
		}
	}
	
	return results, nil
}

// ledgerToPredictions converts ledger entries to predictions
func (mo *MomentumOptimizer) ledgerToPredictions(data []LedgerEntry) []Prediction {
	predictions := make([]Prediction, len(data))
	
	for i, entry := range data {
		predictions[i] = Prediction{
			Symbol:         entry.Symbol,
			Timestamp:      entry.TsScan,
			CompositeScore: entry.Composite,
			Predicted24h:   entry.Composite >= 75.0, // Top decile threshold
			Predicted48h:   entry.Composite >= 75.0,
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

// aggregateMetrics calculates aggregate metrics across CV folds
func (mo *MomentumOptimizer) aggregateMetrics(results []CVFoldResult) EvaluationMetrics {
	validResults := []CVFoldResult{}
	for _, result := range results {
		if result.Error == "" {
			validResults = append(validResults, result)
		}
	}
	
	if len(validResults) == 0 {
		return EvaluationMetrics{}
	}
	
	// Calculate weighted averages
	totalSamples := 0
	metrics := EvaluationMetrics{}
	
	for _, result := range validResults {
		weight := float64(result.Metrics.TotalPredictions)
		totalSamples += result.Metrics.TotalPredictions
		
		metrics.Precision20_24h += result.Metrics.Precision20_24h * weight
		metrics.Precision20_48h += result.Metrics.Precision20_48h * weight
		metrics.Precision10_24h += result.Metrics.Precision10_24h * weight
		metrics.Precision10_48h += result.Metrics.Precision10_48h * weight
		metrics.Precision50_24h += result.Metrics.Precision50_24h * weight
		metrics.Precision50_48h += result.Metrics.Precision50_48h * weight
		metrics.FalsePositiveRate += result.Metrics.FalsePositiveRate * weight
		metrics.MaxDrawdownPenalty += result.Metrics.MaxDrawdownPenalty * weight
		metrics.WinRate24h += result.Metrics.WinRate24h * weight
		metrics.WinRate48h += result.Metrics.WinRate48h * weight
		metrics.ValidPredictions += result.Metrics.ValidPredictions
	}
	
	// Normalize by total weight
	if totalSamples > 0 {
		weightTotal := float64(totalSamples)
		metrics.Precision20_24h /= weightTotal
		metrics.Precision20_48h /= weightTotal
		metrics.Precision10_24h /= weightTotal
		metrics.Precision10_48h /= weightTotal
		metrics.Precision50_24h /= weightTotal
		metrics.Precision50_48h /= weightTotal
		metrics.FalsePositiveRate /= weightTotal
		metrics.MaxDrawdownPenalty /= weightTotal
		metrics.WinRate24h /= weightTotal
		metrics.WinRate48h /= weightTotal
	}
	
	metrics.TotalPredictions = totalSamples
	
	return metrics
}

// calculateRegimeMetrics calculates metrics by regime
func (mo *MomentumOptimizer) calculateRegimeMetrics(results []CVFoldResult) map[string]EvaluationMetrics {
	regimeMetrics := make(map[string]EvaluationMetrics)
	
	// Group predictions by regime and calculate metrics
	regimePredictions := make(map[string][]Prediction)
	
	for _, result := range results {
		if result.Error != "" {
			continue
		}
		
		for _, pred := range result.Predictions {
			regime := pred.Regime
			if regime == "" {
				regime = "unknown"
			}
			
			regimePredictions[regime] = append(regimePredictions[regime], pred)
		}
	}
	
	// Calculate metrics for each regime
	for regime, predictions := range regimePredictions {
		if len(predictions) > 0 {
			regimeMetrics[regime] = mo.evaluator.CalculatePrecisionMetrics(predictions)
		}
	}
	
	return regimeMetrics
}

// calculateStabilityMetrics calculates parameter stability across folds
func (mo *MomentumOptimizer) calculateStabilityMetrics(results []CVFoldResult) StabilityMetrics {
	validResults := []CVFoldResult{}
	for _, result := range results {
		if result.Error == "" {
			validResults = append(validResults, result)
		}
	}
	
	if len(validResults) < 2 {
		return StabilityMetrics{}
	}
	
	// Calculate standard deviations
	precision24h := make([]float64, len(validResults))
	objectives := make([]float64, len(validResults))
	
	for i, result := range validResults {
		precision24h[i] = result.Metrics.Precision20_24h
		objectives[i] = CalculateObjective(result.Metrics)
	}
	
	return StabilityMetrics{
		PrecisionStdDev: calculateStdDev(precision24h),
		ObjectiveStdDev: calculateStdDev(objectives),
		FoldConsistency: calculateConsistency(objectives),
		RegimeConsistency: 0.0, // Would need regime-specific analysis
	}
}

// calculateStdDev calculates standard deviation
func calculateStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)
	
	return math.Sqrt(variance)
}

// calculateConsistency calculates fold consistency (0-1, higher is better)
func calculateConsistency(values []float64) float64 {
	if len(values) < 2 {
		return 1.0
	}
	
	stdDev := calculateStdDev(values)
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	
	if mean == 0 {
		return 0.0
	}
	
	// Coefficient of variation inverted (lower CV = higher consistency)
	cv := stdDev / math.Abs(mean)
	consistency := 1.0 / (1.0 + cv)
	
	return consistency
}