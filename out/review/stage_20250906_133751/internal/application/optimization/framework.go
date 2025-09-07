package optimization

import (
	"context"
	"fmt"
	"math"
	"time"
)

// OptimizationTarget defines what we're optimizing
type OptimizationTarget string

const (
	TargetMomentum OptimizationTarget = "momentum"
	TargetDip      OptimizationTarget = "dip"
)

// Parameter represents a tunable parameter with bounds
type Parameter struct {
	Name    string        `json:"name"`
	Value   interface{}   `json:"value"`
	Min     interface{}   `json:"min,omitempty"`
	Max     interface{}   `json:"max,omitempty"`
	Options []interface{} `json:"options,omitempty"` // For discrete parameters
	Type    string        `json:"type"`              // "float", "int", "bool", "discrete"
}

// ParameterSet represents a complete set of parameters
type ParameterSet struct {
	ID         string               `json:"id"`
	Target     OptimizationTarget   `json:"target"`
	Parameters map[string]Parameter `json:"parameters"`
	Timestamp  time.Time            `json:"timestamp"`
}

// ValidationResult holds parameter validation results
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// EvaluationMetrics holds optimization objective metrics
type EvaluationMetrics struct {
	Precision20_24h    float64 `json:"precision_20_24h"`
	Precision20_48h    float64 `json:"precision_20_48h"`
	Precision10_24h    float64 `json:"precision_10_24h"`
	Precision10_48h    float64 `json:"precision_10_48h"`
	Precision50_24h    float64 `json:"precision_50_24h"`
	Precision50_48h    float64 `json:"precision_50_48h"`
	FalsePositiveRate  float64 `json:"false_positive_rate"`
	MaxDrawdownPenalty float64 `json:"max_drawdown_penalty"`
	ObjectiveScore     float64 `json:"objective_score"`
	WinRate24h         float64 `json:"win_rate_24h"`
	WinRate48h         float64 `json:"win_rate_48h"`
	TotalPredictions   int     `json:"total_predictions"`
	ValidPredictions   int     `json:"valid_predictions"`
	Regime             string  `json:"regime,omitempty"`
}

// CVFoldResult holds results for a single cross-validation fold
type CVFoldResult struct {
	Fold        int               `json:"fold"`
	TrainPeriod TimeRange         `json:"train_period"`
	TestPeriod  TimeRange         `json:"test_period"`
	Metrics     EvaluationMetrics `json:"metrics"`
	Predictions []Prediction      `json:"predictions"`
	Error       string            `json:"error,omitempty"`
}

// OptimizationResult holds the complete optimization results
type OptimizationResult struct {
	ID               string                       `json:"id"`
	Target           OptimizationTarget           `json:"target"`
	Parameters       ParameterSet                 `json:"parameters"`
	CVResults        []CVFoldResult               `json:"cv_results"`
	AggregateMetrics EvaluationMetrics            `json:"aggregate_metrics"`
	RegimeMetrics    map[string]EvaluationMetrics `json:"regime_metrics"`
	Stability        StabilityMetrics             `json:"stability"`
	Duration         time.Duration                `json:"duration"`
	StartTime        time.Time                    `json:"start_time"`
	EndTime          time.Time                    `json:"end_time"`
}

// StabilityMetrics measures parameter stability across folds
type StabilityMetrics struct {
	PrecisionStdDev   float64 `json:"precision_std_dev"`
	ObjectiveStdDev   float64 `json:"objective_std_dev"`
	FoldConsistency   float64 `json:"fold_consistency"`   // 0-1, higher better
	RegimeConsistency float64 `json:"regime_consistency"` // 0-1, higher better
}

// TimeRange represents a time period
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Prediction represents a single prediction and its outcome
type Prediction struct {
	Symbol         string     `json:"symbol"`
	Timestamp      time.Time  `json:"timestamp"`
	CompositeScore float64    `json:"composite_score"`
	Predicted24h   bool       `json:"predicted_24h"`
	Predicted48h   bool       `json:"predicted_48h"`
	Actual24h      float64    `json:"actual_24h"`
	Actual48h      float64    `json:"actual_48h"`
	Success24h     bool       `json:"success_24h"`
	Success48h     bool       `json:"success_48h"`
	Regime         string     `json:"regime"`
	Gates          GateStatus `json:"gates"`
}

// GateStatus tracks which gates passed/failed
type GateStatus struct {
	AllPass        bool `json:"all_pass"`
	Freshness      bool `json:"freshness"`
	Microstructure bool `json:"microstructure"`
	LateFill       bool `json:"late_fill"`
	Fatigue        bool `json:"fatigue"`
}

// OptimizerConfig configures the optimization process
type OptimizerConfig struct {
	Target            OptimizationTarget `json:"target"`
	MaxIterations     int                `json:"max_iterations"`
	CVFolds           int                `json:"cv_folds"`
	PurgeGap          time.Duration      `json:"purge_gap"`
	WalkForwardWindow time.Duration      `json:"walk_forward_window"`
	MinimumSamples    int                `json:"minimum_samples"`
	RegimeAware       bool               `json:"regime_aware"`
	ParallelFolds     bool               `json:"parallel_folds"`
	RandomSeed        int64              `json:"random_seed"`
	OutputDir         string             `json:"output_dir"`
}

// Optimizer interface defines optimization behavior
type Optimizer interface {
	Optimize(ctx context.Context, config OptimizerConfig) (*OptimizationResult, error)
	ValidateParameters(params ParameterSet) ValidationResult
	EvaluateParameters(ctx context.Context, params ParameterSet, folds []CVFold) ([]CVFoldResult, error)
}

// CVFold represents a single cross-validation fold
type CVFold struct {
	Index      int       `json:"index"`
	TrainStart time.Time `json:"train_start"`
	TrainEnd   time.Time `json:"train_end"`
	TestStart  time.Time `json:"test_start"`
	TestEnd    time.Time `json:"test_end"`
	Regime     string    `json:"regime,omitempty"`
}

// BaseOptimizer provides common optimization functionality
type BaseOptimizer struct {
	config       OptimizerConfig
	dataProvider DataProvider
	evaluator    Evaluator
}

// DataProvider interface for accessing historical data
type DataProvider interface {
	GetLedgerData(ctx context.Context, start, end time.Time) ([]LedgerEntry, error)
	GetMarketData(ctx context.Context, symbol string, start, end time.Time) ([]MarketDataPoint, error)
	ValidateDataAvailability() error
	GetDataSummary(ctx context.Context) (*DataSummary, error)
	ClearCache()
	GetCacheStats() map[string]int
}

// Evaluator interface for evaluating parameter sets
type Evaluator interface {
	EvaluateParameters(ctx context.Context, params ParameterSet, data []LedgerEntry) (EvaluationMetrics, error)
	CalculatePrecisionMetrics(predictions []Prediction) EvaluationMetrics
}

// LedgerEntry represents a single ledger entry from results
type LedgerEntry struct {
	TsScan    time.Time `json:"ts_scan"`
	Symbol    string    `json:"symbol"`
	Composite float64   `json:"composite"`
	GatesPass bool      `json:"gates_all_pass"`
	Horizons  struct {
		H24 time.Time `json:"24h"`
		H48 time.Time `json:"48h"`
	} `json:"horizons"`
	Realized struct {
		H24 float64 `json:"24h"`
		H48 float64 `json:"48h"`
	} `json:"realized"`
	Pass struct {
		H24 bool `json:"24h"`
		H48 bool `json:"48h"`
	} `json:"pass"`
}

// MarketDataPoint represents market data for a point in time
type MarketDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
}

// DataSummary provides a summary of available data
type DataSummary struct {
	TotalEntries    int            `json:"total_entries"`
	UniqueSymbols   int            `json:"unique_symbols"`
	Symbols         []string       `json:"symbols"`
	StartTime       time.Time      `json:"start_time"`
	EndTime         time.Time      `json:"end_time"`
	GatePassRate    float64        `json:"gate_pass_rate"`
	RegimeBreakdown map[string]int `json:"regime_breakdown"`
}

// NewBaseOptimizer creates a new base optimizer
func NewBaseOptimizer(config OptimizerConfig, provider DataProvider, evaluator Evaluator) *BaseOptimizer {
	return &BaseOptimizer{
		config:       config,
		dataProvider: provider,
		evaluator:    evaluator,
	}
}

// ValidateParameters validates parameter bounds and constraints
func (bo *BaseOptimizer) ValidateParameters(params ParameterSet) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []string{}}

	for name, param := range params.Parameters {
		err := validateParameter(name, param)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Target-specific validation
	switch params.Target {
	case TargetMomentum:
		result = bo.validateMomentumParameters(params, result)
	case TargetDip:
		result = bo.validateDipParameters(params, result)
	}

	return result
}

// validateParameter validates a single parameter
func validateParameter(name string, param Parameter) error {
	switch param.Type {
	case "float":
		val, ok := param.Value.(float64)
		if !ok {
			return fmt.Errorf("parameter %s: expected float64, got %T", name, param.Value)
		}

		if param.Min != nil {
			if min, ok := param.Min.(float64); ok && val < min {
				return fmt.Errorf("parameter %s: value %.4f below minimum %.4f", name, val, min)
			}
		}

		if param.Max != nil {
			if max, ok := param.Max.(float64); ok && val > max {
				return fmt.Errorf("parameter %s: value %.4f above maximum %.4f", name, val, max)
			}
		}

	case "int":
		val, ok := param.Value.(int)
		if !ok {
			return fmt.Errorf("parameter %s: expected int, got %T", name, param.Value)
		}

		if param.Min != nil {
			if min, ok := param.Min.(int); ok && val < min {
				return fmt.Errorf("parameter %s: value %d below minimum %d", name, val, min)
			}
		}

		if param.Max != nil {
			if max, ok := param.Max.(int); ok && val > max {
				return fmt.Errorf("parameter %s: value %d above maximum %d", name, val, max)
			}
		}

	case "discrete":
		if param.Options == nil || len(param.Options) == 0 {
			return fmt.Errorf("parameter %s: discrete parameter missing options", name)
		}

		found := false
		for _, option := range param.Options {
			if param.Value == option {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("parameter %s: value %v not in options %v", name, param.Value, param.Options)
		}
	}

	return nil
}

// validateMomentumParameters validates momentum-specific constraints
func (bo *BaseOptimizer) validateMomentumParameters(params ParameterSet, result ValidationResult) ValidationResult {
	// Check that regime weights sum to 1.0
	regimes := []string{"bull", "choppy", "high_vol"}
	timeframes := []string{"1h", "4h", "12h", "24h", "7d"}

	for _, regime := range regimes {
		weightSum := 0.0
		allWeightsPresent := true

		for _, tf := range timeframes {
			paramName := fmt.Sprintf("%s_weight_%s", regime, tf)
			if param, exists := params.Parameters[paramName]; exists {
				if weight, ok := param.Value.(float64); ok {
					weightSum += weight
				} else {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("weight %s must be float64", paramName))
					allWeightsPresent = false
				}
			} else if tf != "7d" { // 7d is optional
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("missing required weight %s", paramName))
				allWeightsPresent = false
			}
		}

		if allWeightsPresent && math.Abs(weightSum-1.0) > 0.001 {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("regime %s weights sum to %.4f, must sum to 1.0", regime, weightSum))
		}
	}

	return result
}

// validateDipParameters validates dip-specific constraints
func (bo *BaseOptimizer) validateDipParameters(params ParameterSet, result ValidationResult) ValidationResult {
	// Validate RSI trigger range
	if param, exists := params.Parameters["rsi_trigger_1h"]; exists {
		if val, ok := param.Value.(float64); ok {
			if val < 18 || val > 32 {
				result.Valid = false
				result.Errors = append(result.Errors, "rsi_trigger_1h must be between 18 and 32")
			}
		}
	}

	// Validate dip depth range
	if param, exists := params.Parameters["dip_depth_min"]; exists {
		if val, ok := param.Value.(float64); ok {
			if val < -20.0 || val > -6.0 {
				result.Valid = false
				result.Errors = append(result.Errors, "dip_depth_min must be between -20% and -6%")
			}
		}
	}

	// Validate volume flush range
	if param, exists := params.Parameters["volume_flush_min"]; exists {
		if val, ok := param.Value.(float64); ok {
			if val < 1.25 || val > 2.5 {
				result.Valid = false
				result.Errors = append(result.Errors, "volume_flush_min must be between 1.25x and 2.5x")
			}
		}
	}

	return result
}

// CalculateObjective computes the optimization objective function
func CalculateObjective(metrics EvaluationMetrics) float64 {
	// J = 1.0·precision@20(24h) + 0.5·precision@20(48h) – 0.2·false_positive_rate – 0.2·max_drawdown_penalty
	objective := 1.0*metrics.Precision20_24h +
		0.5*metrics.Precision20_48h -
		0.2*metrics.FalsePositiveRate -
		0.2*metrics.MaxDrawdownPenalty

	return objective
}

// GenerateParameterID creates a unique ID for a parameter set
func GenerateParameterID(params ParameterSet) string {
	// Simple ID generation based on target and timestamp
	return fmt.Sprintf("%s_%d", params.Target, time.Now().UnixNano())
}
