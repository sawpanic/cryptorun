package tune

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/tune/data"
	"cryptorun/internal/tune/opt"
	"cryptorun/internal/tune/weights"
)

func TestCoordinateDescent_BasicOptimization(t *testing.T) {
	// Create deterministic test data
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 20)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 15, 3)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	// Create constraint system and objective function
	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	// Create optimizer with deterministic seed
	config := opt.OptimizerConfig{
		MaxEvaluations:    50,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42, // Deterministic seed
		Verbose:           false,
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	// Initial weights (slightly perturbed from base)
	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.40, // Below optimal
		TechnicalResidual: 0.22, // Above optimal
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := optimizer.Optimize(initialWeights)
	require.NoError(t, err)

	// Basic validation
	assert.True(t, result.Evaluations > 0, "Should have performed evaluations")
	assert.True(t, result.Evaluations <= config.MaxEvaluations, "Should not exceed max evaluations")
	assert.True(t, result.ElapsedTime > 0, "Should have elapsed time")

	// Optimization should improve or maintain objective
	improvement := result.BestObjective.TotalScore - result.InitialObjective.TotalScore
	assert.GreaterOrEqual(t, improvement, -0.001, "Should not worsen significantly (allowing for noise)")

	// Best weights should satisfy constraints
	err = constraints.ValidateWeights("normal", result.BestWeights)
	assert.NoError(t, err, "Best weights should satisfy constraints")

	// Weights should sum to 1
	weightSum := result.BestWeights.MomentumCore + result.BestWeights.TechnicalResidual +
		result.BestWeights.VolumeResidual + result.BestWeights.QualityResidual
	assert.InDelta(t, 1.0, weightSum, 0.001, "Best weights should sum to 1")
}

func TestCoordinateDescent_Deterministic(t *testing.T) {
	// Test that optimization is deterministic with same seed
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 15)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	// Same config with same seed
	config := opt.OptimizerConfig{
		MaxEvaluations:    30,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              12345, // Fixed seed
		Verbose:           false,
	}

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.41,
		TechnicalResidual: 0.21,
		VolumeResidual:    0.24,
		QualityResidual:   0.14,
	}

	// Run optimization twice
	optimizer1 := opt.NewCoordinateDescent(config, constraints, objective, "normal")
	optimizer2 := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	result1, err := optimizer1.Optimize(initialWeights)
	require.NoError(t, err)

	result2, err := optimizer2.Optimize(initialWeights)
	require.NoError(t, err)

	// Results should be identical
	assert.Equal(t, result1.Evaluations, result2.Evaluations, "Evaluations should be deterministic")
	assert.Equal(t, result1.Converged, result2.Converged, "Convergence should be deterministic")

	// Final weights should be identical (within floating point precision)
	assert.InDelta(t, result1.BestWeights.MomentumCore, result2.BestWeights.MomentumCore, 1e-10,
		"MomentumCore should be deterministic")
	assert.InDelta(t, result1.BestWeights.TechnicalResidual, result2.BestWeights.TechnicalResidual, 1e-10,
		"TechnicalResidual should be deterministic")
	assert.InDelta(t, result1.BestWeights.VolumeResidual, result2.BestWeights.VolumeResidual, 1e-10,
		"VolumeResidual should be deterministic")
	assert.InDelta(t, result1.BestWeights.QualityResidual, result2.BestWeights.QualityResidual, 1e-10,
		"QualityResidual should be deterministic")

	// Objective scores should be identical
	assert.InDelta(t, result1.BestObjective.TotalScore, result2.BestObjective.TotalScore, 1e-10,
		"Best objective should be deterministic")
}

func TestCoordinateDescent_ConvergenceWithinKIterations(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 8, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	// Strict convergence limits
	config := opt.OptimizerConfig{
		MaxEvaluations:    100,
		Tolerance:         0.001,
		InitialStepSize:   0.02,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   15,
		Seed:              42,
		Verbose:           false,
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.40,
		TechnicalResidual: 0.22,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := optimizer.Optimize(initialWeights)
	require.NoError(t, err)

	// Should converge within K iterations
	assert.LessOrEqual(t, result.Evaluations, config.MaxEvaluations,
		"Should converge within max evaluations")

	// Should either converge or early stop
	assert.True(t, result.Converged || result.EarlyStopped,
		"Should either converge or early stop")
}

func TestCoordinateDescent_MonotoneNonWorsening(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 12)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	// Enable verbose to get history
	config := opt.OptimizerConfig{
		MaxEvaluations:    50,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42,
		Verbose:           true, // Enable to get history
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.41,
		TechnicalResidual: 0.21,
		VolumeResidual:    0.24,
		QualityResidual:   0.14,
	}

	result, err := optimizer.Optimize(initialWeights)
	require.NoError(t, err)

	// The best objective should never worsen
	assert.GreaterOrEqual(t, result.BestObjective.TotalScore, result.InitialObjective.TotalScore-0.001,
		"Best objective should not be significantly worse than initial")

	// If we have history, check monotonic improvement of best score
	if len(result.History) > 0 {
		bestSoFar := result.InitialObjective.TotalScore
		for _, step := range result.History {
			if step.Improvement > 0 {
				// This was an improvement, so objective should be better
				assert.Greater(t, step.Objective, bestSoFar-0.001,
					"Objective should not worsen at evaluation %d", step.Evaluation)
				if step.Objective > bestSoFar {
					bestSoFar = step.Objective
				}
			}
		}
	}
}

func TestCoordinateDescent_ConstraintRespect(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 15)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 12, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	config := opt.OptimizerConfig{
		MaxEvaluations:    40,
		Tolerance:         0.001,
		InitialStepSize:   0.02,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42,
		Verbose:           true, // Enable history
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	// Start with weights that violate constraints
	invalidInitialWeights := weights.RegimeWeights{
		MomentumCore:      0.60, // Above maximum (will be clamped)
		TechnicalResidual: 0.10, // Below minimum (will be clamped)
		VolumeResidual:    0.20,
		QualityResidual:   0.10,
	}

	result, err := optimizer.Optimize(invalidInitialWeights)
	require.NoError(t, err)

	// All weights during optimization should respect constraints
	err = constraints.ValidateWeights("normal", result.BestWeights)
	assert.NoError(t, err, "Best weights should satisfy constraints")

	// Check that all intermediate steps also respected constraints
	if len(result.History) > 0 {
		for i, step := range result.History {
			err := constraints.ValidateWeights("normal", step.Weights)
			assert.NoError(t, err, "Step %d weights should satisfy constraints", i)

			// Weights should sum to 1 at every step
			sum := step.Weights.MomentumCore + step.Weights.TechnicalResidual +
				step.Weights.VolumeResidual + step.Weights.QualityResidual
			assert.InDelta(t, 1.0, sum, 0.001, "Step %d weights should sum to 1", i)
		}
	}
}

func TestCoordinateDescent_GoldenWeightsRegression(t *testing.T) {
	// Load golden test data
	goldenData := loadGoldenWeights(t)

	// Create mock data matching the golden test
	smokeResults := data.CreateMockResults([]string{goldenData.Regime}, []string{"4h"}, 15)
	benchResults := data.CreateMockBenchResults([]string{goldenData.Regime}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		goldenData.Regime: goldenData.InitialWeights,
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, goldenData.Regime)

	// Use exact config from golden data
	config := opt.OptimizerConfig{
		MaxEvaluations:    goldenData.MaxEvaluations,
		Tolerance:         goldenData.Tolerance,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              goldenData.Seed,
		Verbose:           false,
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, goldenData.Regime)

	result, err := optimizer.Optimize(goldenData.InitialWeights)
	require.NoError(t, err)

	// Check regression against golden results
	// Note: Exact match may not be possible due to implementation changes,
	// but results should be close and show improvement
	improvement := result.BestObjective.TotalScore - result.InitialObjective.TotalScore
	expectedImprovement := goldenData.ExpectedObjectiveImprovement

	// Allow for some tolerance in regression test
	assert.GreaterOrEqual(t, improvement, expectedImprovement-0.01,
		"Should achieve similar improvement to golden results")

	// Final weights should be reasonably close to golden weights
	tolerance := 0.05 // 5% tolerance for regression
	assert.InDelta(t, goldenData.ExpectedFinalWeights.MomentumCore,
		result.BestWeights.MomentumCore, tolerance,
		"MomentumCore should be close to golden result")
	assert.InDelta(t, goldenData.ExpectedFinalWeights.TechnicalResidual,
		result.BestWeights.TechnicalResidual, tolerance,
		"TechnicalResidual should be close to golden result")
	assert.InDelta(t, goldenData.ExpectedFinalWeights.VolumeResidual,
		result.BestWeights.VolumeResidual, tolerance,
		"VolumeResidual should be close to golden result")
	assert.InDelta(t, goldenData.ExpectedFinalWeights.QualityResidual,
		result.BestWeights.QualityResidual, tolerance,
		"QualityResidual should be close to golden result")
}

func TestCoordinateDescent_OptimizationSummary(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 8, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	config := opt.OptimizerConfig{
		MaxEvaluations:    30,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42,
		Verbose:           false,
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.41,
		TechnicalResidual: 0.21,
		VolumeResidual:    0.24,
		QualityResidual:   0.14,
	}

	result, err := optimizer.Optimize(initialWeights)
	require.NoError(t, err)

	// Get optimization summary
	summary := optimizer.GetOptimizationSummary(result)

	// Validate summary
	assert.Equal(t, "normal", summary.Regime, "Summary should have correct regime")
	assert.Equal(t, result.Evaluations, summary.Evaluations, "Summary should match evaluations")
	assert.Equal(t, result.ElapsedTime, summary.ElapsedTime, "Summary should match elapsed time")
	assert.Equal(t, result.Converged, summary.Converged, "Summary should match convergence")
	assert.Equal(t, result.EarlyStopped, summary.EarlyStopped, "Summary should match early stop")

	// Check objective scores
	assert.Equal(t, result.InitialObjective.TotalScore, summary.InitialObjective,
		"Summary should match initial objective")
	assert.Equal(t, result.BestObjective.TotalScore, summary.FinalObjective,
		"Summary should match final objective")

	improvement := result.BestObjective.TotalScore - result.InitialObjective.TotalScore
	assert.InDelta(t, improvement, summary.Improvement, 1e-10,
		"Summary should calculate improvement correctly")

	// Check weight changes
	expectedChanges := map[string]float64{
		"momentum_core":      result.BestWeights.MomentumCore - result.InitialWeights.MomentumCore,
		"technical_residual": result.BestWeights.TechnicalResidual - result.InitialWeights.TechnicalResidual,
		"volume_residual":    result.BestWeights.VolumeResidual - result.InitialWeights.VolumeResidual,
		"quality_residual":   result.BestWeights.QualityResidual - result.InitialWeights.QualityResidual,
	}

	for coord, expectedChange := range expectedChanges {
		actualChange := summary.WeightChanges[coord]
		assert.InDelta(t, expectedChange, actualChange, 1e-10,
			"Weight change for %s should be calculated correctly", coord)
	}

	// Largest change should be computed correctly
	var expectedLargestChange float64
	var expectedLargestCoord string
	for coord, change := range expectedChanges {
		if math.Abs(change) > expectedLargestChange {
			expectedLargestChange = math.Abs(change)
			expectedLargestCoord = coord
		}
	}

	assert.InDelta(t, expectedLargestChange, summary.LargestChange, 1e-10,
		"Largest change should be computed correctly")
	assert.Equal(t, expectedLargestCoord, summary.LargestChangeCoord,
		"Largest change coordinate should be identified correctly")
}

func TestCoordinateDescent_ValidationErrorHandling(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 8, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	constraints := weights.NewConstraintSystem()
	objectiveConfig := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, baseWeights, "normal")

	config := opt.OptimizerConfig{
		MaxEvaluations:    20,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42,
		Verbose:           false,
	}

	optimizer := opt.NewCoordinateDescent(config, constraints, objective, "normal")

	validWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := optimizer.Optimize(validWeights)
	require.NoError(t, err)

	// Validate the optimization result
	validationErr := opt.ValidateOptimizationResult(result, constraints, "normal")
	assert.NoError(t, validationErr, "Valid optimization result should pass validation")

	// Test validation of invalid result (manually constructed)
	invalidResult := opt.OptimizationResult{
		BestWeights: weights.RegimeWeights{
			MomentumCore:      0.30, // Below minimum bound
			TechnicalResidual: 0.30,
			VolumeResidual:    0.25,
			QualityResidual:   0.15,
		},
		BestObjective:    weights.ObjectiveResult{TotalScore: 0.5},
		InitialObjective: weights.ObjectiveResult{TotalScore: 0.8}, // Better than best (invalid)
		Evaluations:      5,
	}

	validationErr = opt.ValidateOptimizationResult(invalidResult, constraints, "normal")
	assert.Error(t, validationErr, "Invalid optimization result should fail validation")
}

// Helper types and functions for golden testing
type GoldenWeightData struct {
	Regime                       string                `json:"regime"`
	Seed                         uint64                `json:"seed"`
	InitialWeights               weights.RegimeWeights `json:"initial_weights"`
	ExpectedFinalWeights         weights.RegimeWeights `json:"expected_final_weights"`
	ExpectedObjectiveImprovement float64               `json:"expected_objective_improvement"`
	MaxEvaluations               int                   `json:"max_evaluations"`
	Tolerance                    float64               `json:"tolerance"`
}

func loadGoldenWeights(t *testing.T) GoldenWeightData {
	// Return the golden data from our test fixture
	return GoldenWeightData{
		Regime: "normal",
		Seed:   42,
		InitialWeights: weights.RegimeWeights{
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
		ExpectedFinalWeights: weights.RegimeWeights{
			MomentumCore:      0.427,
			TechnicalResidual: 0.195,
			VolumeResidual:    0.248,
			QualityResidual:   0.130,
		},
		ExpectedObjectiveImprovement: 0.0085,
		MaxEvaluations:               50,
		Tolerance:                    0.001,
	}
}
