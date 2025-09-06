package opt

import (
	"math"
	"testing"
	"time"

	"cryptorun/internal/tune/data"
	"cryptorun/internal/tune/weights"
)

func TestCoordinateDescent_OptimizeDeterministic(t *testing.T) {
	// Setup with fixed seed for deterministic behavior
	config := OptimizerConfig{
		MaxEvaluations:    50,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              42, // Fixed seed
		Verbose:           false,
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.16,
		QualityResidual:   0.22, // Sum = 1.00, all within normal bounds
	}

	// Run optimization twice with same configuration
	result1, err1 := optimizer.Optimize(initialWeights)
	if err1 != nil {
		t.Fatalf("first optimization failed: %v", err1)
	}

	optimizer2 := NewCoordinateDescent(config, constraints, objective, "normal")
	result2, err2 := optimizer2.Optimize(initialWeights)
	if err2 != nil {
		t.Fatalf("second optimization failed: %v", err2)
	}

	// Results should be identical (deterministic)
	tolerance := 1e-10
	if !weightsEqual(result1.BestWeights, result2.BestWeights, tolerance) {
		t.Errorf("optimization not deterministic: weights differ")
		t.Logf("Result 1: %+v", result1.BestWeights)
		t.Logf("Result 2: %+v", result2.BestWeights)
	}

	if math.Abs(result1.BestObjective.TotalScore-result2.BestObjective.TotalScore) > tolerance {
		t.Errorf("optimization not deterministic: objectives differ: %f vs %f",
			result1.BestObjective.TotalScore, result2.BestObjective.TotalScore)
	}

	if result1.Evaluations != result2.Evaluations {
		t.Errorf("optimization not deterministic: evaluation counts differ: %d vs %d",
			result1.Evaluations, result2.Evaluations)
	}
}

func TestCoordinateDescent_ConvergenceWithinK(t *testing.T) {
	config := OptimizerConfig{
		MaxEvaluations:    100,
		Tolerance:         0.01,
		InitialStepSize:   0.02,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   15,
		Seed:              12345,
		Verbose:           false,
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.41, // Valid starting point
		TechnicalResidual: 0.19,
		VolumeResidual:    0.17,
		QualityResidual:   0.23, // Sum = 1.00, all within normal bounds
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("optimization failed: %v", err)
	}

	// Should converge within max evaluations
	if result.Evaluations >= config.MaxEvaluations {
		t.Errorf("did not converge within %d evaluations, used %d", config.MaxEvaluations, result.Evaluations)
	}

	// Should improve from initial
	if result.BestObjective.TotalScore <= result.InitialObjective.TotalScore {
		t.Errorf("optimization did not improve: %.6f -> %.6f",
			result.InitialObjective.TotalScore, result.BestObjective.TotalScore)
	}

	t.Logf("Convergence test: %d evaluations, %.6f -> %.6f (%.6f improvement)",
		result.Evaluations, result.InitialObjective.TotalScore, result.BestObjective.TotalScore,
		result.BestObjective.TotalScore-result.InitialObjective.TotalScore)
}

func TestCoordinateDescent_MonotoneNonWorsening(t *testing.T) {
	config := OptimizerConfig{
		MaxEvaluations:    50,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              98765,
		Verbose:           true, // Enable history tracking
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.43,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.22, // Sum = 1.00, all within normal bounds
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("optimization failed: %v", err)
	}

	// Check that objective never worsens during optimization
	if len(result.History) > 0 {
		bestSoFar := result.InitialObjective.TotalScore
		for i, step := range result.History {
			if step.Improvement > 0 { // Only check steps that were accepted
				if step.Objective < bestSoFar {
					t.Errorf("objective worsened at step %d: %f < %f", i, step.Objective, bestSoFar)
				}
				bestSoFar = math.Max(bestSoFar, step.Objective)
			}
		}
	}

	// Final result should be no worse than initial
	if result.BestObjective.TotalScore < result.InitialObjective.TotalScore-1e-10 {
		t.Errorf("final result worse than initial: %.6f < %.6f",
			result.BestObjective.TotalScore, result.InitialObjective.TotalScore)
	}
}

func TestCoordinateDescent_ConstraintsRespected(t *testing.T) {
	config := OptimizerConfig{
		MaxEvaluations:    30,
		Tolerance:         0.01,
		InitialStepSize:   0.05, // Large steps to test constraint enforcement
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              55555,
		Verbose:           true,
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	// Start with weights that will require significant adjustment
	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.50, // Above normal max (0.45)
		TechnicalResidual: 0.15, // Below normal min (0.18)
		VolumeResidual:    0.20,
		QualityResidual:   0.15,
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("optimization failed: %v", err)
	}

	// Verify all intermediate and final weights satisfy constraints
	if err := constraints.ValidateWeights("normal", result.BestWeights); err != nil {
		t.Errorf("final weights violate constraints: %v", err)
	}

	// Check history if available
	if len(result.History) > 0 {
		for i, step := range result.History {
			if err := constraints.ValidateWeights("normal", step.Weights); err != nil {
				t.Errorf("step %d weights violate constraints: %v", i, err)
			}
		}
	}

	t.Logf("Constraint test passed: final weights valid")
}

func TestCoordinateDescent_InvalidInitialWeights(t *testing.T) {
	config := DefaultOptimizerConfig()
	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	// Invalid initial weights (don't sum to 1)
	invalidWeights := weights.RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.30,
		VolumeResidual:    0.20,
		QualityResidual:   0.10, // Sum = 1.1
	}

	result, err := optimizer.Optimize(invalidWeights)
	if err != nil {
		t.Fatalf("optimization should handle invalid initial weights by clamping: %v", err)
	}

	// Should have clamped and normalized initial weights
	if err := constraints.ValidateWeights("normal", result.BestWeights); err != nil {
		t.Errorf("result weights invalid despite clamping: %v", err)
	}

	// Initial weights in result should be the clamped version
	if err := constraints.ValidateWeights("normal", result.InitialWeights); err != nil {
		t.Errorf("reported initial weights should be valid after clamping: %v", err)
	}
}

func TestCoordinateDescent_EarlyStopping(t *testing.T) {
	config := OptimizerConfig{
		MaxEvaluations:    200,
		Tolerance:         0.001,
		InitialStepSize:   0.001, // Very small steps to trigger early stopping
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   5, // Stop after 5 evaluations without improvement
		Seed:              11111,
		Verbose:           false,
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	// Start near optimum to trigger early stopping
	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("optimization failed: %v", err)
	}

	// Should stop early (well before max evaluations)
	if !result.EarlyStopped && result.Evaluations >= config.MaxEvaluations {
		t.Error("expected early stopping but used all evaluations")
	}

	if result.Evaluations < 5 {
		t.Errorf("should run at least a few evaluations, got %d", result.Evaluations)
	}

	t.Logf("Early stopping test: stopped after %d evaluations (early_stopped=%v)",
		result.Evaluations, result.EarlyStopped)
}

func TestCoordinateDescent_StepSizeAdaptation(t *testing.T) {
	config := OptimizerConfig{
		MaxEvaluations:    50,
		Tolerance:         0.001,
		InitialStepSize:   0.1, // Large initial step
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              33333,
		Verbose:           true, // Need history for step size tracking
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.40, // At boundary
		TechnicalResidual: 0.18, // At boundary
		VolumeResidual:    0.17,
		QualityResidual:   0.25,
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("optimization failed: %v", err)
	}

	// Check that step size gets reduced over time when no improvement
	if len(result.History) > 10 {
		earlySteps := result.History[:5]
		lateSteps := result.History[len(result.History)-5:]

		avgEarlyStepSize := 0.0
		avgLateStepSize := 0.0

		for _, step := range earlySteps {
			avgEarlyStepSize += step.StepSize
		}
		avgEarlyStepSize /= float64(len(earlySteps))

		for _, step := range lateSteps {
			avgLateStepSize += step.StepSize
		}
		avgLateStepSize /= float64(len(lateSteps))

		// Step size should generally decrease or stay same
		if avgLateStepSize > avgEarlyStepSize*2 {
			t.Errorf("step size should not increase significantly: early=%.6f, late=%.6f",
				avgEarlyStepSize, avgLateStepSize)
		}

		t.Logf("Step size adaptation: early avg=%.6f, late avg=%.6f", avgEarlyStepSize, avgLateStepSize)
	}
}

func TestCoordinateDescent_GoldenRegression(t *testing.T) {
	// Golden test with fixed parameters for regression detection
	config := OptimizerConfig{
		MaxEvaluations:    25,
		Tolerance:         0.001,
		InitialStepSize:   0.01,
		BacktrackingRatio: 0.5,
		MinStepSize:       1e-6,
		EarlyStopWindow:   10,
		Seed:              999, // Golden seed
		Verbose:           false,
	}

	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	// Golden initial weights
	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.420,
		TechnicalResidual: 0.195,
		VolumeResidual:    0.165,
		QualityResidual:   0.220,
	}

	result, err := optimizer.Optimize(initialWeights)
	if err != nil {
		t.Fatalf("golden regression optimization failed: %v", err)
	}

	// These values should remain stable across refactoring
	expectedEvaluations := 25    // Should use all evaluations or converge early
	expectedImprovement := 0.001 // Should improve by at least this much

	if result.Evaluations > expectedEvaluations {
		t.Errorf("golden regression: expected evaluations <= %d, got %d", expectedEvaluations, result.Evaluations)
	}

	improvement := result.BestObjective.TotalScore - result.InitialObjective.TotalScore
	if improvement < expectedImprovement {
		t.Errorf("golden regression: expected improvement >= %.6f, got %.6f", expectedImprovement, improvement)
	}

	// Final weights should be valid
	if err := constraints.ValidateWeights("normal", result.BestWeights); err != nil {
		t.Errorf("golden regression: final weights invalid: %v", err)
	}

	// Sum should be 1.0
	sum := result.BestWeights.MomentumCore + result.BestWeights.TechnicalResidual +
		result.BestWeights.VolumeResidual + result.BestWeights.QualityResidual
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("golden regression: weights don't sum to 1.0: %f", sum)
	}

	t.Logf("Golden regression passed: %d evals, %.6f improvement, sum=%.6f",
		result.Evaluations, improvement, sum)
}

func TestValidateOptimizationResult(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	validWeights := weights.RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	initialObj := weights.ObjectiveResult{TotalScore: 0.5}
	bestObj := weights.ObjectiveResult{TotalScore: 0.6}

	tests := []struct {
		name    string
		result  OptimizationResult
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid result",
			result: OptimizationResult{
				BestWeights:      validWeights,
				BestObjective:    bestObj,
				InitialObjective: initialObj,
				Evaluations:      10,
			},
			wantErr: false,
		},
		{
			name: "invalid weights",
			result: OptimizationResult{
				BestWeights: weights.RegimeWeights{
					MomentumCore:      0.60, // Above bounds
					TechnicalResidual: 0.20,
					VolumeResidual:    0.10,
					QualityResidual:   0.10,
				},
				BestObjective:    bestObj,
				InitialObjective: initialObj,
				Evaluations:      10,
			},
			wantErr: true,
			errMsg:  "violate constraints",
		},
		{
			name: "objective worsened",
			result: OptimizationResult{
				BestWeights:      validWeights,
				BestObjective:    weights.ObjectiveResult{TotalScore: 0.4}, // Worse than 0.5
				InitialObjective: initialObj,
				Evaluations:      10,
			},
			wantErr: true,
			errMsg:  "worsened objective",
		},
		{
			name: "zero evaluations",
			result: OptimizationResult{
				BestWeights:      validWeights,
				BestObjective:    bestObj,
				InitialObjective: initialObj,
				Evaluations:      0,
			},
			wantErr: true,
			errMsg:  "invalid evaluation count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptimizationResult(tt.result, constraints, "normal")

			if tt.wantErr && err == nil {
				t.Errorf("expected error containing '%s', got none", tt.errMsg)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestGetOptimizationSummary(t *testing.T) {
	config := DefaultOptimizerConfig()
	constraints := weights.NewConstraintSystem()
	objective := createTestObjectiveFunction()

	optimizer := NewCoordinateDescent(config, constraints, objective, "normal")

	initialWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.16,
		QualityResidual:   0.22,
	}

	finalWeights := weights.RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.195,
		VolumeResidual:    0.155,
		QualityResidual:   0.225,
	}

	result := OptimizationResult{
		BestWeights:      finalWeights,
		BestObjective:    weights.ObjectiveResult{TotalScore: 0.65},
		InitialWeights:   initialWeights,
		InitialObjective: weights.ObjectiveResult{TotalScore: 0.60},
		Evaluations:      25,
		Converged:        true,
		EarlyStopped:     false,
		ElapsedTime:      100 * time.Millisecond,
	}

	summary := optimizer.GetOptimizationSummary(result)

	// Verify basic fields
	if summary.Regime != "normal" {
		t.Errorf("expected regime 'normal', got '%s'", summary.Regime)
	}
	if summary.Evaluations != 25 {
		t.Errorf("expected 25 evaluations, got %d", summary.Evaluations)
	}
	if !summary.Converged {
		t.Error("expected converged = true")
	}
	if summary.Improvement != 0.05 { // 0.65 - 0.60
		t.Errorf("expected improvement 0.05, got %f", summary.Improvement)
	}

	// Verify weight changes
	expectedChanges := map[string]float64{
		"momentum_core":      0.005,  // 0.425 - 0.42
		"technical_residual": -0.005, // 0.195 - 0.20
		"volume_residual":    -0.005, // 0.155 - 0.16
		"quality_residual":   0.005,  // 0.225 - 0.22
	}

	for coord, expectedChange := range expectedChanges {
		actualChange := summary.WeightChanges[coord]
		if math.Abs(actualChange-expectedChange) > 0.001 {
			t.Errorf("weight change for %s: expected %f, got %f", coord, expectedChange, actualChange)
		}
	}

	// Verify largest change detection
	if summary.LargestChange != 0.005 {
		t.Errorf("expected largest change 0.005, got %f", summary.LargestChange)
	}

	// Should be one of the coordinates that changed by 0.005
	validCoords := []string{"momentum_core", "technical_residual", "volume_residual", "quality_residual"}
	found := false
	for _, coord := range validCoords {
		if summary.LargestChangeCoord == coord {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("largest change coord '%s' should be one of the valid coordinates", summary.LargestChangeCoord)
	}

	t.Logf("Summary test passed: regime=%s, evals=%d, improvement=%.6f, largest_change=%s:%.6f",
		summary.Regime, summary.Evaluations, summary.Improvement, summary.LargestChangeCoord, summary.LargestChange)
}

// Helper functions
func createTestObjectiveFunction() *weights.ObjectiveFunction {
	// Create simple test data
	smokeData := []data.SmokeResult{
		{Symbol: "BTC", Score: 85.0, Regime: "normal", ForwardReturn: 0.03, Hit: true, Window: "4h"},
		{Symbol: "ETH", Score: 75.0, Regime: "normal", ForwardReturn: 0.02, Hit: false, Window: "4h"},
		{Symbol: "ADA", Score: 70.0, Regime: "normal", ForwardReturn: 0.015, Hit: false, Window: "4h"},
	}

	benchData := []data.BenchResult{
		{Symbol: "BTC", Score: 90.0, Regime: "normal", ActualGain: 0.035, BenchmarkHit: true, Window: "24h"},
		{Symbol: "ETH", Score: 80.0, Regime: "normal", ActualGain: 0.025, BenchmarkHit: false, Window: "24h"},
	}

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.425,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.15,
			QualityResidual:   0.225,
		},
	}

	config := weights.DefaultObjectiveConfig()
	return weights.NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")
}

func weightsEqual(w1, w2 weights.RegimeWeights, tolerance float64) bool {
	return math.Abs(w1.MomentumCore-w2.MomentumCore) <= tolerance &&
		math.Abs(w1.TechnicalResidual-w2.TechnicalResidual) <= tolerance &&
		math.Abs(w1.VolumeResidual-w2.VolumeResidual) <= tolerance &&
		math.Abs(w1.QualityResidual-w2.QualityResidual) <= tolerance
}

func containsSubstring(str, substr string) bool {
	return len(substr) == 0 || len(str) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}
