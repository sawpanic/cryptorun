package opt

import (
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/internal/tune/weights"
)

// OptimizerConfig defines the configuration for coordinate descent optimization
type OptimizerConfig struct {
	MaxEvaluations    int     `json:"max_evaluations"`    // Maximum function evaluations (default: 200)
	Tolerance         float64 `json:"tolerance"`          // Convergence tolerance (default: 0.001)
	InitialStepSize   float64 `json:"initial_step_size"`  // Initial step size (default: 0.01)
	BacktrackingRatio float64 `json:"backtracking_ratio"` // Step size reduction factor (default: 0.5)
	MinStepSize       float64 `json:"min_step_size"`      // Minimum step size (default: 1e-6)
	EarlyStopWindow   int     `json:"early_stop_window"`  // Evaluations to check for improvement (default: 10)
	Seed              uint64  `json:"seed"`               // Random seed for deterministic behavior
	Verbose           bool    `json:"verbose"`            // Progress output
}

// DefaultOptimizerConfig returns the default optimizer configuration
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		MaxEvaluations:    200,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              uint64(time.Now().UnixNano()),
		Verbose:           false,
	}
}

// OptimizationResult holds the result of the optimization process
type OptimizationResult struct {
	BestWeights      weights.RegimeWeights   `json:"best_weights"`
	BestObjective    weights.ObjectiveResult `json:"best_objective"`
	InitialWeights   weights.RegimeWeights   `json:"initial_weights"`
	InitialObjective weights.ObjectiveResult `json:"initial_objective"`
	Evaluations      int                     `json:"evaluations"`
	Converged        bool                    `json:"converged"`
	EarlyStopped     bool                    `json:"early_stopped"`
	ElapsedTime      time.Duration           `json:"elapsed_time"`
	History          []OptimizationStep      `json:"history,omitempty"`
}

// OptimizationStep represents a single step in the optimization history
type OptimizationStep struct {
	Evaluation  int                   `json:"evaluation"`
	Weights     weights.RegimeWeights `json:"weights"`
	Objective   float64               `json:"objective"`
	StepSize    float64               `json:"step_size"`
	Direction   string                `json:"direction"`
	Improvement float64               `json:"improvement"`
}

// CoordinateDescent implements constrained coordinate descent optimization
type CoordinateDescent struct {
	config      OptimizerConfig
	constraints *weights.ConstraintSystem
	objective   *weights.ObjectiveFunction
	regime      string
	rng         *weights.RandGen
	history     []OptimizationStep
}

// NewCoordinateDescent creates a new coordinate descent optimizer
func NewCoordinateDescent(config OptimizerConfig, constraints *weights.ConstraintSystem, objective *weights.ObjectiveFunction, regime string) *CoordinateDescent {
	return &CoordinateDescent{
		config:      config,
		constraints: constraints,
		objective:   objective,
		regime:      regime,
		rng:         weights.NewRandGen(config.Seed),
		history:     make([]OptimizationStep, 0, config.MaxEvaluations),
	}
}

// Optimize performs constrained coordinate descent optimization
func (cd *CoordinateDescent) Optimize(initialWeights weights.RegimeWeights) (OptimizationResult, error) {
	startTime := time.Now()

	// Validate and clamp initial weights
	currentWeights, err := cd.constraints.ClampWeights(cd.regime, initialWeights)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("failed to clamp initial weights: %w", err)
	}

	// Evaluate initial weights
	initialObj, err := cd.objective.Evaluate(currentWeights)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("failed to evaluate initial weights: %w", err)
	}

	// Initialize optimization state
	bestWeights := currentWeights
	bestObjective := initialObj
	evaluations := 1
	stepSize := cd.config.InitialStepSize

	// Track improvement for early stopping
	lastBestValue := initialObj.TotalScore
	noImprovementCount := 0

	// Define coordinate names for cycling
	coordinates := []string{"momentum_core", "technical_residual", "volume_residual", "quality_residual"}

	if cd.config.Verbose {
		fmt.Printf("Starting optimization: initial objective = %.6f\n", initialObj.TotalScore)
	}

	for evaluations < cd.config.MaxEvaluations {
		improved := false

		// Cycle through all coordinates
		for _, coord := range coordinates {
			if evaluations >= cd.config.MaxEvaluations {
				break
			}

			// Try both positive and negative directions
			directions := []float64{1.0, -1.0}

			// Randomize direction order for better exploration
			if cd.rng.Float64() < 0.5 {
				directions[0], directions[1] = directions[1], directions[0]
			}

			for _, direction := range directions {
				if evaluations >= cd.config.MaxEvaluations {
					break
				}

				// Calculate step for this coordinate
				step := direction * stepSize
				newWeights := cd.applyCoordinateStep(currentWeights, coord, step)

				// Clamp to constraints
				clampedWeights, err := cd.constraints.ClampWeights(cd.regime, newWeights)
				if err != nil {
					continue // Skip invalid steps
				}

				// Evaluate new weights
				newObj, err := cd.objective.Evaluate(clampedWeights)
				if err != nil {
					continue // Skip evaluation errors
				}

				evaluations++

				// Record history
				improvement := newObj.TotalScore - bestObjective.TotalScore
				historyStep := OptimizationStep{
					Evaluation:  evaluations,
					Weights:     clampedWeights,
					Objective:   newObj.TotalScore,
					StepSize:    stepSize,
					Direction:   fmt.Sprintf("%s%+.3f", coord, direction),
					Improvement: improvement,
				}
				cd.history = append(cd.history, historyStep)

				// Check for improvement
				if newObj.TotalScore > bestObjective.TotalScore {
					bestWeights = clampedWeights
					bestObjective = newObj
					currentWeights = clampedWeights
					improved = true

					if cd.config.Verbose {
						fmt.Printf("Eval %d: improved to %.6f (+%.6f) via %s\n",
							evaluations, newObj.TotalScore, improvement, historyStep.Direction)
					}

					break // Move to next coordinate after improvement
				}
			}
		}

		// Adaptive step size control
		if improved {
			// Reset step size after improvement
			stepSize = cd.config.InitialStepSize
			lastBestValue = bestObjective.TotalScore
			noImprovementCount = 0
		} else {
			// Reduce step size if no improvement found
			stepSize *= cd.config.BacktrackingRatio
			noImprovementCount++

			if stepSize < cd.config.MinStepSize {
				if cd.config.Verbose {
					fmt.Printf("Converged: step size %.2e below minimum\n", stepSize)
				}
				break
			}
		}

		// Early stopping check
		if noImprovementCount >= cd.config.EarlyStopWindow {
			if cd.config.Verbose {
				fmt.Printf("Early stopping: no improvement for %d evaluations\n", cd.config.EarlyStopWindow)
			}
			break
		}

		// Convergence check
		if evaluations > 1 && math.Abs(bestObjective.TotalScore-lastBestValue) < cd.config.Tolerance {
			if cd.config.Verbose {
				fmt.Printf("Converged: improvement %.2e below tolerance\n",
					math.Abs(bestObjective.TotalScore-lastBestValue))
			}
			break
		}
	}

	elapsedTime := time.Since(startTime)

	result := OptimizationResult{
		BestWeights:      bestWeights,
		BestObjective:    bestObjective,
		InitialWeights:   initialWeights,
		InitialObjective: initialObj,
		Evaluations:      evaluations,
		Converged:        evaluations < cd.config.MaxEvaluations,
		EarlyStopped:     noImprovementCount >= cd.config.EarlyStopWindow,
		ElapsedTime:      elapsedTime,
	}

	// Include history if verbose
	if cd.config.Verbose {
		result.History = cd.history
	}

	if cd.config.Verbose {
		fmt.Printf("Optimization complete: %d evals, %.3fs, final objective = %.6f (%.6f improvement)\n",
			evaluations, elapsedTime.Seconds(), bestObjective.TotalScore,
			bestObjective.TotalScore-initialObj.TotalScore)
	}

	return result, nil
}

// applyCoordinateStep applies a step to a specific coordinate
func (cd *CoordinateDescent) applyCoordinateStep(weights weights.RegimeWeights, coord string, step float64) weights.RegimeWeights {
	newWeights := weights

	switch coord {
	case "momentum_core":
		newWeights.MomentumCore += step
	case "technical_residual":
		newWeights.TechnicalResidual += step
	case "volume_residual":
		newWeights.VolumeResidual += step
	case "quality_residual":
		newWeights.QualityResidual += step
	}

	return newWeights
}

// GetOptimizationSummary returns a summary of the optimization process
func (cd *CoordinateDescent) GetOptimizationSummary(result OptimizationResult) OptimizationSummary {
	summary := OptimizationSummary{
		Regime:           cd.regime,
		Evaluations:      result.Evaluations,
		ElapsedTime:      result.ElapsedTime,
		Converged:        result.Converged,
		EarlyStopped:     result.EarlyStopped,
		InitialObjective: result.InitialObjective.TotalScore,
		FinalObjective:   result.BestObjective.TotalScore,
		Improvement:      result.BestObjective.TotalScore - result.InitialObjective.TotalScore,
		WeightChanges:    make(map[string]float64),
	}

	// Calculate weight changes
	summary.WeightChanges["momentum_core"] = result.BestWeights.MomentumCore - result.InitialWeights.MomentumCore
	summary.WeightChanges["technical_residual"] = result.BestWeights.TechnicalResidual - result.InitialWeights.TechnicalResidual
	summary.WeightChanges["volume_residual"] = result.BestWeights.VolumeResidual - result.InitialWeights.VolumeResidual
	summary.WeightChanges["quality_residual"] = result.BestWeights.QualityResidual - result.InitialWeights.QualityResidual

	// Find largest change
	var maxChange float64
	var maxChangeCoord string
	for coord, change := range summary.WeightChanges {
		if math.Abs(change) > maxChange {
			maxChange = math.Abs(change)
			maxChangeCoord = coord
		}
	}
	summary.LargestChange = maxChange
	summary.LargestChangeCoord = maxChangeCoord

	return summary
}

// OptimizationSummary provides a high-level summary of optimization results
type OptimizationSummary struct {
	Regime             string             `json:"regime"`
	Evaluations        int                `json:"evaluations"`
	ElapsedTime        time.Duration      `json:"elapsed_time"`
	Converged          bool               `json:"converged"`
	EarlyStopped       bool               `json:"early_stopped"`
	InitialObjective   float64            `json:"initial_objective"`
	FinalObjective     float64            `json:"final_objective"`
	Improvement        float64            `json:"improvement"`
	WeightChanges      map[string]float64 `json:"weight_changes"`
	LargestChange      float64            `json:"largest_change"`
	LargestChangeCoord string             `json:"largest_change_coord"`
}

// ValidateOptimizationResult checks the optimization result for consistency
func ValidateOptimizationResult(result OptimizationResult, constraints *weights.ConstraintSystem, regime string) error {
	// Validate best weights satisfy constraints
	if err := constraints.ValidateWeights(regime, result.BestWeights); err != nil {
		return fmt.Errorf("best weights violate constraints: %w", err)
	}

	// Check that we actually improved (or at least didn't worsen significantly)
	if result.BestObjective.TotalScore < result.InitialObjective.TotalScore-0.001 {
		return fmt.Errorf("optimization worsened objective from %.6f to %.6f",
			result.InitialObjective.TotalScore, result.BestObjective.TotalScore)
	}

	// Validate evaluation count
	if result.Evaluations < 1 {
		return fmt.Errorf("invalid evaluation count: %d", result.Evaluations)
	}

	return nil
}
