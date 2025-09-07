package optimization

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
)

// DipOptimizer optimizes dip/reversal scanner parameters for oversold bounces
type DipOptimizer struct {
	*BaseOptimizer
	bounds DipParameterBounds
}

// DipParameterBounds defines the search space for dip parameters
type DipParameterBounds struct {
	// RSI trigger bounds
	RSI1h_Min float64 `json:"rsi_1h_min"` // 18
	RSI1h_Max float64 `json:"rsi_1h_max"` // 32

	// Quality dip depth bounds (negative percentages)
	DipDepth_Min float64 `json:"dip_depth_min"` // -20%
	DipDepth_Max float64 `json:"dip_depth_max"` // -6%

	// Volume flush multipliers
	VolumeFlush_Min float64 `json:"volume_flush_min"` // 1.25x
	VolumeFlush_Max float64 `json:"volume_flush_max"` // 2.5x

	// Confirmation options
	RSI4h_Rising     bool `json:"rsi_4h_rising_option"`
	Momentum1h_Cross bool `json:"momentum_1h_cross_option"`

	// Divergence detection
	EnableDivergence bool `json:"enable_divergence_option"`

	// 20MA proximity (ATR multipliers)
	MA20_Proximity_Min float64 `json:"ma20_proximity_min"` // ATR multipliers
	MA20_Proximity_Max float64 `json:"ma20_proximity_max"`

	// Fixed constraints (not tunable)
	MinADX       float64 `json:"min_adx"`        // 25
	MinHurst     float64 `json:"min_hurst"`      // 0.55
	MinVADR      float64 `json:"min_vadr"`       // 1.75
	MaxSpreadBps float64 `json:"max_spread_bps"` // 50
	MinDepthUSD  float64 `json:"min_depth_usd"`  // 100k
	MaxFreshness int     `json:"max_freshness"`  // 2 bars
	MaxATRFresh  float64 `json:"max_atr_fresh"`  // 1.2x ATR
	MaxLateFill  int     `json:"max_late_fill"`  // 30 seconds
}

// DefaultDipBounds returns the default parameter bounds for dip optimization
func DefaultDipBounds() DipParameterBounds {
	return DipParameterBounds{
		RSI1h_Min:       18.0,
		RSI1h_Max:       32.0,
		DipDepth_Min:    -20.0,
		DipDepth_Max:    -6.0,
		VolumeFlush_Min: 1.25,
		VolumeFlush_Max: 2.5,

		RSI4h_Rising:     true,
		Momentum1h_Cross: true,
		EnableDivergence: true,

		MA20_Proximity_Min: 0.5, // 0.5x ATR from 20MA
		MA20_Proximity_Max: 2.0, // 2.0x ATR from 20MA

		// Fixed constraints
		MinADX:       25.0,
		MinHurst:     0.55,
		MinVADR:      1.75,
		MaxSpreadBps: 50.0,
		MinDepthUSD:  100000.0,
		MaxFreshness: 2,
		MaxATRFresh:  1.2,
		MaxLateFill:  30,
	}
}

// NewDipOptimizer creates a new dip optimizer
func NewDipOptimizer(config OptimizerConfig, provider DataProvider, evaluator Evaluator) *DipOptimizer {
	return &DipOptimizer{
		BaseOptimizer: NewBaseOptimizer(config, provider, evaluator),
		bounds:        DefaultDipBounds(),
	}
}

// Optimize performs dip parameter optimization
func (do *DipOptimizer) Optimize(ctx context.Context, config OptimizerConfig) (*OptimizationResult, error) {
	log.Info().Msg("Starting dip/reversal parameter optimization")

	startTime := time.Now()

	// Create time series cross-validation folds
	folds, err := do.createTSCVFolds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create CV folds: %w", err)
	}

	log.Info().Int("folds", len(folds)).Msg("Created time series CV folds for dip optimization")

	// Initialize random search
	rand.Seed(config.RandomSeed)
	bestResult := &OptimizationResult{
		StartTime:        startTime,
		Target:           TargetDip,
		AggregateMetrics: EvaluationMetrics{ObjectiveScore: math.Inf(-1)},
	}

	// Random search with bounded parameters
	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Generate random parameter set within bounds
		params := do.generateRandomParameters()

		// Validate parameters
		validation := do.ValidateParameters(params)
		if !validation.Valid {
			log.Debug().Strs("errors", validation.Errors).Msg("Invalid dip parameters, skipping")
			continue
		}

		// Evaluate parameters across all folds
		cvResults, err := do.evaluateParametersOnFolds(ctx, params, folds)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to evaluate dip parameters")
			continue
		}

		// Calculate aggregate metrics with dip-specific objective
		aggMetrics := do.aggregateMetrics(cvResults)

		// Dip-specific objective: precision@20(12h) + 0.5·precision@20(24h) – 0.2·FPR – 0.2·maxDD_penalty
		aggMetrics.ObjectiveScore = do.calculateDipObjective(aggMetrics)

		// Check if this is the best result so far
		if aggMetrics.ObjectiveScore > bestResult.AggregateMetrics.ObjectiveScore {
			bestResult = &OptimizationResult{
				ID:               GenerateParameterID(params),
				Target:           TargetDip,
				Parameters:       params,
				CVResults:        cvResults,
				AggregateMetrics: aggMetrics,
				RegimeMetrics:    do.calculateRegimeMetrics(cvResults),
				Stability:        do.calculateStabilityMetrics(cvResults),
				StartTime:        startTime,
			}

			log.Info().
				Int("iteration", iteration).
				Float64("objective", aggMetrics.ObjectiveScore).
				Float64("precision_12h", aggMetrics.Precision20_24h). // Using 24h field for 12h data
				Float64("precision_24h", aggMetrics.Precision20_48h). // Using 48h field for 24h data
				Msg("New best dip parameter set found")
		}

		if iteration%100 == 0 {
			log.Info().Int("iteration", iteration).Int("total", config.MaxIterations).
				Float64("best_objective", bestResult.AggregateMetrics.ObjectiveScore).
				Msg("Dip optimization progress")
		}
	}

	bestResult.EndTime = time.Now()
	bestResult.Duration = bestResult.EndTime.Sub(bestResult.StartTime)

	log.Info().
		Float64("best_objective", bestResult.AggregateMetrics.ObjectiveScore).
		Float64("precision_12h", bestResult.AggregateMetrics.Precision20_24h).
		Float64("precision_24h", bestResult.AggregateMetrics.Precision20_48h).
		Dur("duration", bestResult.Duration).
		Msg("Dip optimization completed")

	return bestResult, nil
}

// calculateDipObjective calculates dip-specific objective function
func (do *DipOptimizer) calculateDipObjective(metrics EvaluationMetrics) float64 {
	// For dip optimization: precision@20(12h) + 0.5·precision@20(24h) – 0.2·FPR – 0.2·maxDD_penalty
	// Using Precision20_24h for 12h results, Precision20_48h for 24h results
	objective := 1.0*metrics.Precision20_24h + // 12h precision (stored in 24h field)
		0.5*metrics.Precision20_48h + // 24h precision (stored in 48h field)
		-0.2*metrics.FalsePositiveRate -
		0.2*metrics.MaxDrawdownPenalty

	return objective
}

// generateRandomParameters generates a random dip parameter set within bounds
func (do *DipOptimizer) generateRandomParameters() ParameterSet {
	params := ParameterSet{
		Target:     TargetDip,
		Parameters: make(map[string]Parameter),
		Timestamp:  time.Now(),
	}

	// RSI 1h trigger
	params.Parameters["rsi_trigger_1h"] = Parameter{
		Name:  "rsi_trigger_1h",
		Value: do.bounds.RSI1h_Min + rand.Float64()*(do.bounds.RSI1h_Max-do.bounds.RSI1h_Min),
		Min:   do.bounds.RSI1h_Min,
		Max:   do.bounds.RSI1h_Max,
		Type:  "float",
	}

	// Quality dip depth
	params.Parameters["dip_depth_min"] = Parameter{
		Name:  "dip_depth_min",
		Value: do.bounds.DipDepth_Min + rand.Float64()*(do.bounds.DipDepth_Max-do.bounds.DipDepth_Min),
		Min:   do.bounds.DipDepth_Min,
		Max:   do.bounds.DipDepth_Max,
		Type:  "float",
	}

	// Volume flush multiplier
	params.Parameters["volume_flush_min"] = Parameter{
		Name:  "volume_flush_min",
		Value: do.bounds.VolumeFlush_Min + rand.Float64()*(do.bounds.VolumeFlush_Max-do.bounds.VolumeFlush_Min),
		Min:   do.bounds.VolumeFlush_Min,
		Max:   do.bounds.VolumeFlush_Max,
		Type:  "float",
	}

	// Confirmation method (either RSI 4h rising OR 1h momentum cross)
	useRSI4h := rand.Float64() > 0.5
	params.Parameters["confirm_rsi_4h_rising"] = Parameter{
		Name:  "confirm_rsi_4h_rising",
		Value: useRSI4h,
		Type:  "bool",
	}

	params.Parameters["confirm_momentum_1h_cross"] = Parameter{
		Name:  "confirm_momentum_1h_cross",
		Value: !useRSI4h, // Mutually exclusive
		Type:  "bool",
	}

	// Optional divergence detection
	params.Parameters["enable_divergence"] = Parameter{
		Name:  "enable_divergence",
		Value: rand.Float64() > 0.3, // 70% chance to enable
		Type:  "bool",
	}

	// 20MA proximity (in ATR multiples)
	params.Parameters["ma20_proximity_max"] = Parameter{
		Name:  "ma20_proximity_max",
		Value: do.bounds.MA20_Proximity_Min + rand.Float64()*(do.bounds.MA20_Proximity_Max-do.bounds.MA20_Proximity_Min),
		Min:   do.bounds.MA20_Proximity_Min,
		Max:   do.bounds.MA20_Proximity_Max,
		Type:  "float",
	}

	// Fixed constraints (not optimized)
	params.Parameters["min_adx"] = Parameter{
		Name:  "min_adx",
		Value: do.bounds.MinADX,
		Type:  "float",
	}

	params.Parameters["min_hurst"] = Parameter{
		Name:  "min_hurst",
		Value: do.bounds.MinHurst,
		Type:  "float",
	}

	params.Parameters["min_vadr"] = Parameter{
		Name:  "min_vadr",
		Value: do.bounds.MinVADR,
		Type:  "float",
	}

	params.Parameters["max_spread_bps"] = Parameter{
		Name:  "max_spread_bps",
		Value: do.bounds.MaxSpreadBps,
		Type:  "float",
	}

	params.Parameters["min_depth_usd"] = Parameter{
		Name:  "min_depth_usd",
		Value: do.bounds.MinDepthUSD,
		Type:  "float",
	}

	params.Parameters["max_freshness_bars"] = Parameter{
		Name:  "max_freshness_bars",
		Value: do.bounds.MaxFreshness,
		Type:  "int",
	}

	params.Parameters["max_atr_freshness"] = Parameter{
		Name:  "max_atr_freshness",
		Value: do.bounds.MaxATRFresh,
		Type:  "float",
	}

	params.Parameters["max_late_fill_seconds"] = Parameter{
		Name:  "max_late_fill_seconds",
		Value: do.bounds.MaxLateFill,
		Type:  "int",
	}

	params.ID = GenerateParameterID(params)
	return params
}

// createTSCVFolds creates time series cross-validation folds specific to dip patterns
func (do *DipOptimizer) createTSCVFolds(ctx context.Context) ([]CVFold, error) {
	// Get all available data to determine time range
	allData, err := do.dataProvider.GetLedgerData(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger data: %w", err)
	}

	if len(allData) < do.config.MinimumSamples {
		return nil, fmt.Errorf("insufficient data: got %d samples, need %d", len(allData), do.config.MinimumSamples)
	}

	startTime := allData[0].TsScan
	endTime := allData[len(allData)-1].TsScan

	folds := []CVFold{}
	foldIndex := 0

	// Walk-forward with smaller windows for dip patterns (more frequent patterns)
	dipWindow := do.config.WalkForwardWindow / 2 // Smaller windows for dip patterns

	for trainStart := startTime; trainStart.Add(dipWindow).Before(endTime); foldIndex++ {
		trainEnd := trainStart.Add(dipWindow)
		testStart := trainEnd.Add(do.config.PurgeGap)
		testEnd := testStart.Add(dipWindow / 6) // Shorter test periods for dip validation

		if testEnd.After(endTime) {
			break
		}

		// Determine regime for this fold (simplified)
		regime := do.detectRegimeForPeriod(trainStart, trainEnd)

		fold := CVFold{
			Index:      foldIndex,
			TrainStart: trainStart,
			TrainEnd:   trainEnd,
			TestStart:  testStart,
			TestEnd:    testEnd,
			Regime:     regime,
		}

		folds = append(folds, fold)

		// Move to next fold with more overlap for dip patterns
		trainStart = trainStart.Add(dipWindow / 3)
	}

	if len(folds) < 5 {
		return nil, fmt.Errorf("insufficient time range for dip CV: got %d folds, need at least 5", len(folds))
	}

	return folds, nil
}

// detectRegimeForPeriod determines the market regime for a time period (simplified)
func (do *DipOptimizer) detectRegimeForPeriod(start, end time.Time) string {
	// This would typically analyze volatility, trend, etc.
	// For now, use a simplified approach based on time
	duration := end.Sub(start)

	// Rotate through regimes to ensure coverage
	switch (start.Unix() / int64(duration.Seconds())) % 3 {
	case 0:
		return "bull"
	case 1:
		return "choppy"
	default:
		return "high_vol"
	}
}

// evaluateParametersOnFolds evaluates dip parameters across all CV folds
func (do *DipOptimizer) evaluateParametersOnFolds(ctx context.Context, params ParameterSet, folds []CVFold) ([]CVFoldResult, error) {
	results := make([]CVFoldResult, len(folds))

	for i, fold := range folds {
		// Get data for this fold's time range
		testData, err := do.dataProvider.GetLedgerData(ctx, fold.TestStart, fold.TestEnd)
		if err != nil {
			results[i] = CVFoldResult{
				Fold:        i,
				TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
				TestPeriod:  TimeRange{Start: fold.TestStart, End: fold.TestEnd},
				Error:       fmt.Sprintf("failed to get test data: %v", err),
			}
			continue
		}

		// Filter for dip patterns in test data
		dipData := do.filterForDipPatterns(testData, params)

		if len(dipData) == 0 {
			results[i] = CVFoldResult{
				Fold:        i,
				TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
				TestPeriod:  TimeRange{Start: fold.TestStart, End: fold.TestEnd},
				Metrics:     EvaluationMetrics{}, // Empty metrics
			}
			continue
		}

		// Evaluate dip parameters on filtered data
		metrics, err := do.evaluator.EvaluateParameters(ctx, params, dipData)
		if err != nil {
			results[i] = CVFoldResult{
				Fold:        i,
				TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
				TestPeriod:  TimeRange{Start: fold.TestStart, End: fold.TestEnd},
				Error:       fmt.Sprintf("failed to evaluate dip parameters: %v", err),
			}
			continue
		}

		// Convert to predictions with dip-specific logic
		predictions := do.ledgerToDipPredictions(dipData, params)

		results[i] = CVFoldResult{
			Fold:        i,
			TrainPeriod: TimeRange{Start: fold.TrainStart, End: fold.TrainEnd},
			TestPeriod:  TimeRange{Start: fold.TestStart, End: fold.TestEnd},
			Metrics:     metrics,
			Predictions: predictions,
		}
	}

	return results, nil
}

// filterForDipPatterns filters ledger data for potential dip patterns
func (do *DipOptimizer) filterForDipPatterns(data []LedgerEntry, params ParameterSet) []LedgerEntry {
	filtered := []LedgerEntry{}

	// Extract parameters for filtering
	// rsiTrigger := 30.0 // Default - would be used for actual RSI filtering
	// dipDepthMin := -15.0 // Default - would be used for depth filtering
	//
	// In practice, these would be used to filter the data based on the actual
	// RSI values and price movements, but for this simplified implementation,
	// we use composite score as a proxy for dip quality

	for _, entry := range data {
		// Simple filtering logic - in production this would be much more sophisticated

		// Check if this looks like a potential dip scenario
		// (This is simplified - real implementation would analyze RSI, depth, volume patterns)

		// Use composite score as proxy for "dip quality"
		// Lower scores might indicate oversold conditions
		if entry.Composite >= 60.0 && entry.Composite <= 80.0 { // Mid-range scores
			// Check if realized returns show bounce patterns
			if entry.Realized.H24 > 0 || entry.Realized.H48 > 0 { // Some positive movement
				filtered = append(filtered, entry)
			}
		}
	}

	return filtered
}

// ledgerToDipPredictions converts ledger entries to dip-specific predictions
func (do *DipOptimizer) ledgerToDipPredictions(data []LedgerEntry, params ParameterSet) []Prediction {
	predictions := make([]Prediction, len(data))

	for i, entry := range data {
		// For dip patterns, use lower threshold for predictions
		dipThreshold := 60.0 // Lower than momentum threshold

		predictions[i] = Prediction{
			Symbol:         entry.Symbol,
			Timestamp:      entry.TsScan,
			CompositeScore: entry.Composite,
			Predicted24h:   entry.Composite >= dipThreshold, // 12h stored in 24h field
			Predicted48h:   entry.Composite >= dipThreshold, // 24h stored in 48h field
			Actual24h:      entry.Realized.H24,              // 12h results
			Actual48h:      entry.Realized.H48,              // 24h results
			Success24h:     entry.Pass.H24,                  // 12h success
			Success48h:     entry.Pass.H48,                  // 24h success
			Regime:         "dip",                           // Mark as dip regime
			Gates: GateStatus{
				AllPass: entry.GatesPass,
			},
		}
	}

	return predictions
}

// Delegate common methods to base optimizer with dip-specific implementations

// aggregateMetrics calculates aggregate metrics across CV folds for dip optimization
func (do *DipOptimizer) aggregateMetrics(results []CVFoldResult) EvaluationMetrics {
	validResults := []CVFoldResult{}
	for _, result := range results {
		if result.Error == "" {
			validResults = append(validResults, result)
		}
	}

	if len(validResults) == 0 {
		return EvaluationMetrics{}
	}

	// Calculate weighted averages (same logic as momentum but for dip patterns)
	totalSamples := 0
	metrics := EvaluationMetrics{}

	for _, result := range validResults {
		weight := float64(result.Metrics.TotalPredictions)
		totalSamples += result.Metrics.TotalPredictions

		// Note: Using 24h fields for 12h metrics, 48h fields for 24h metrics
		metrics.Precision20_24h += result.Metrics.Precision20_24h * weight // 12h precision
		metrics.Precision20_48h += result.Metrics.Precision20_48h * weight // 24h precision
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

// calculateRegimeMetrics calculates dip-specific metrics by regime
func (do *DipOptimizer) calculateRegimeMetrics(results []CVFoldResult) map[string]EvaluationMetrics {
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
			regimeMetrics[regime] = do.evaluator.CalculatePrecisionMetrics(predictions)
		}
	}

	return regimeMetrics
}

// calculateStabilityMetrics calculates parameter stability for dip patterns
func (do *DipOptimizer) calculateStabilityMetrics(results []CVFoldResult) StabilityMetrics {
	validResults := []CVFoldResult{}
	for _, result := range results {
		if result.Error == "" {
			validResults = append(validResults, result)
		}
	}

	if len(validResults) < 2 {
		return StabilityMetrics{}
	}

	// Calculate standard deviations for dip metrics
	precision12h := make([]float64, len(validResults)) // Using 24h field for 12h data
	objectives := make([]float64, len(validResults))

	for i, result := range validResults {
		precision12h[i] = result.Metrics.Precision20_24h // 12h precision stored in 24h field
		objectives[i] = do.calculateDipObjective(result.Metrics)
	}

	return StabilityMetrics{
		PrecisionStdDev:   calculateStdDev(precision12h),
		ObjectiveStdDev:   calculateStdDev(objectives),
		FoldConsistency:   calculateConsistency(objectives),
		RegimeConsistency: 0.0, // Would need regime-specific analysis
	}
}
