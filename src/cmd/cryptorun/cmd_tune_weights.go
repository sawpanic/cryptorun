package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"cryptorun/internal/tune/data"
	"cryptorun/internal/tune/opt"
	"cryptorun/internal/tune/report"
	"cryptorun/internal/tune/weights"
)

// tuneCmd represents the tune command
var tuneCmd = &cobra.Command{
	Use:   "tune",
	Short: "Optimization and tuning utilities",
	Long:  "Utilities for optimizing and tuning CryptoRun parameters using historical data",
}

// tuneWeightsCmd represents the tune weights command
var tuneWeightsCmd = &cobra.Command{
	Use:   "weights",
	Short: "Tune regime weight allocations using backtest data",
	Long: `Tune regime weight allocations using cached backtest data from Smoke90 and bench artifacts.

This is a CONSTRAINED optimizer that respects all regime bounds and policy constraints:
- MomentumCore remains protected in orthogonalization
- Supply/Demand block = VolumeResidual + QualityResidual  
- Social stays outside with hard +10 cap
- Per-regime bounds enforced (e.g., Momentum: 40-45%, Technical: 18-22%)

OUTPUT IS ADVISORY ONLY: Creates candidate YAML + report with "DO NOT AUTO-APPLY" banner.`,
	RunE: runTuneWeights,
}

var (
	tuneRegimes    []string
	tuneInputFile  string
	tuneOutputFile string
	tuneProgress   bool
	tuneEvals      int
	tuneLambda     float64
	tunePrimary    string
	tuneWindows    []string
)

func init() {
	tuneCmd.AddCommand(tuneWeightsCmd)
	rootCmd.AddCommand(tuneCmd)

	// Tune weights flags
	tuneWeightsCmd.Flags().StringSliceVar(&tuneRegimes, "regimes", []string{"normal"},
		"Regimes to tune (calm,normal,volatile)")
	tuneWeightsCmd.Flags().StringVar(&tuneInputFile, "in", "config/regime_weights.yaml",
		"Input regime weights file")
	tuneWeightsCmd.Flags().StringVar(&tuneOutputFile, "out", "",
		"Output candidate weights file (required)")
	tuneWeightsCmd.Flags().BoolVar(&tuneProgress, "progress", false,
		"Show optimization progress")
	tuneWeightsCmd.Flags().IntVar(&tuneEvals, "evals", 200,
		"Maximum evaluations per regime")
	tuneWeightsCmd.Flags().Float64Var(&tuneLambda, "lambda", 0.005,
		"L2 regularization strength")
	tuneWeightsCmd.Flags().StringVar(&tunePrimary, "primary", "hitrate",
		"Primary metric (hitrate|spearman)")
	tuneWeightsCmd.Flags().StringSliceVar(&tuneWindows, "windows", []string{"1h", "4h", "12h", "24h"},
		"Time windows to include")

	// Make output required
	tuneWeightsCmd.MarkFlagRequired("out")
}

func runTuneWeights(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔧 CryptoRun Regime Weight Tuner\n")
	fmt.Printf("═══════════════════════════════\n\n")

	// Validate flags
	if err := validateTuneFlags(); err != nil {
		return err
	}

	// Create output directory
	outputDir := filepath.Dir(tuneOutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load current weights
	currentWeights, err := loadRegimeWeights(tuneInputFile)
	if err != nil {
		return fmt.Errorf("failed to load input weights: %w", err)
	}

	// Initialize constraint system
	constraints := weights.NewConstraintSystem()

	// Load historical data
	smokeLoader := data.NewSmokeDataLoader("./artifacts/smoke90")
	benchLoader := data.NewBenchDataLoader("./artifacts/bench")

	fmt.Printf("Loading historical data...\n")
	smokeResults, err := smokeLoader.LoadResults(tuneRegimes, tuneWindows)
	if err != nil {
		return fmt.Errorf("failed to load smoke90 data: %w", err)
	}

	benchResults, err := benchLoader.LoadResults(tuneRegimes, tuneWindows)
	if err != nil {
		return fmt.Errorf("failed to load bench data: %w", err)
	}

	fmt.Printf("Loaded %d smoke results, %d bench results\n", len(smokeResults), len(benchResults))

	if len(smokeResults) == 0 && len(benchResults) == 0 {
		return fmt.Errorf("no historical data found for regimes %v", tuneRegimes)
	}

	// Initialize results storage
	candidateWeights := make(map[string]weights.RegimeWeights)
	optimizationResults := make(map[string]opt.OptimizationResult)

	// Tune each regime
	for _, regime := range tuneRegimes {
		fmt.Printf("\n📊 Tuning regime: %s\n", regime)
		fmt.Printf("───────────────────────\n")

		// Get current weights for this regime
		initialWeights, exists := currentWeights[regime]
		if !exists {
			fmt.Printf("Warning: no current weights for regime %s, using defaults\n", regime)
			initialWeights, _ = constraints.GenerateRandomValidWeights(regime, weights.NewRandGen(12345))
		}

		// Configure objective function
		objectiveConfig := weights.ObjectiveConfig{
			HitRateWeight:    0.7,
			SpearmanWeight:   0.3,
			RegularizationL2: tuneLambda,
			PrimaryMetric:    tunePrimary,
		}

		// Adjust weights based on primary metric
		if tunePrimary == "spearman" {
			objectiveConfig.HitRateWeight = 0.3
			objectiveConfig.SpearmanWeight = 0.7
		}

		objective := weights.NewObjectiveFunction(objectiveConfig, smokeResults, benchResults, currentWeights, regime)

		// Configure optimizer
		optimizerConfig := opt.DefaultOptimizerConfig()
		optimizerConfig.MaxEvaluations = tuneEvals
		optimizerConfig.Verbose = tuneProgress
		optimizerConfig.Seed = 42 // Deterministic for reproducibility

		optimizer := opt.NewCoordinateDescent(optimizerConfig, constraints, objective, regime)

		// Run optimization
		result, err := optimizer.Optimize(initialWeights)
		if err != nil {
			return fmt.Errorf("optimization failed for regime %s: %w", regime, err)
		}

		// Validate result
		if err := opt.ValidateOptimizationResult(result, constraints, regime); err != nil {
			return fmt.Errorf("optimization result invalid for regime %s: %w", regime, err)
		}

		// Store results
		candidateWeights[regime] = result.BestWeights
		optimizationResults[regime] = result

		// Show summary
		summary := optimizer.GetOptimizationSummary(result)
		fmt.Printf("✅ Completed: %.6f → %.6f (%.6f improvement) in %d evals\n",
			summary.InitialObjective, summary.FinalObjective, summary.Improvement, summary.Evaluations)
	}

	// Generate candidate weights file
	fmt.Printf("\n📝 Generating candidate weights...\n")
	if err := writeCandidateWeights(tuneOutputFile, candidateWeights, currentWeights); err != nil {
		return fmt.Errorf("failed to write candidate weights: %w", err)
	}

	// Generate report
	reportPath := strings.Replace(tuneOutputFile, ".yaml", "_report.md", 1)
	fmt.Printf("📋 Generating report...\n")

	reportGenerator := report.NewReportGenerator()
	if err := reportGenerator.GenerateReport(reportPath, optimizationResults, currentWeights, candidateWeights, constraints); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Final summary
	fmt.Printf("\n✨ Tuning Complete\n")
	fmt.Printf("═════════════════\n")
	fmt.Printf("📄 Candidate weights: %s\n", tuneOutputFile)
	fmt.Printf("📊 Detailed report:   %s\n", reportPath)
	fmt.Printf("\n⚠️  MANUAL REVIEW REQUIRED - DO NOT AUTO-APPLY\n")
	fmt.Printf("   Review the report and validate improvements before applying.\n")

	return nil
}

func validateTuneFlags() error {
	// Validate regimes
	validRegimes := map[string]bool{"calm": true, "normal": true, "volatile": true}
	for _, regime := range tuneRegimes {
		if !validRegimes[regime] {
			return fmt.Errorf("invalid regime: %s (must be calm, normal, or volatile)", regime)
		}
	}

	// Validate primary metric
	if tunePrimary != "hitrate" && tunePrimary != "spearman" {
		return fmt.Errorf("invalid primary metric: %s (must be hitrate or spearman)", tunePrimary)
	}

	// Validate lambda
	if tuneLambda < 0 || tuneLambda > 1 {
		return fmt.Errorf("invalid lambda: %.6f (must be between 0 and 1)", tuneLambda)
	}

	// Validate evaluations
	if tuneEvals < 10 || tuneEvals > 1000 {
		return fmt.Errorf("invalid evals: %d (must be between 10 and 1000)", tuneEvals)
	}

	// Validate output path
	if tuneOutputFile == "" {
		return fmt.Errorf("output file is required")
	}

	return nil
}

func loadRegimeWeights(filePath string) (map[string]weights.RegimeWeights, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var rawWeights map[string]map[string]float64
	if err := yaml.Unmarshal(data, &rawWeights); err != nil {
		return nil, err
	}

	// Convert to RegimeWeights structure
	regimeWeights := make(map[string]weights.RegimeWeights)

	for regime, weightMap := range rawWeights {
		w := weights.RegimeWeights{
			MomentumCore:      weightMap["momentum_core"],
			TechnicalResidual: weightMap["technical_residual"],
			VolumeResidual:    weightMap["volume_residual"],
			QualityResidual:   weightMap["quality_residual"],
		}
		regimeWeights[regime] = w
	}

	return regimeWeights, nil
}

func writeCandidateWeights(filePath string, candidateWeights, currentWeights map[string]weights.RegimeWeights) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Create YAML structure
	output := make(map[string]interface{})

	// Add header comment
	header := fmt.Sprintf(`# CANDIDATE REGIME WEIGHTS - DO NOT AUTO-APPLY
# Generated by CryptoRun Weight Tuner at %s
# 
# ⚠️  MANUAL REVIEW REQUIRED ⚠️
# These are SUGGESTED weight adjustments based on historical data.
# Review the accompanying report before applying any changes.
#
# Changes made:`, timestamp)

	for regime, candidate := range candidateWeights {
		current, exists := currentWeights[regime]
		if !exists {
			continue
		}

		momentumChange := candidate.MomentumCore - current.MomentumCore
		technicalChange := candidate.TechnicalResidual - current.TechnicalResidual
		volumeChange := candidate.VolumeResidual - current.VolumeResidual
		qualityChange := candidate.QualityResidual - current.QualityResidual

		header += fmt.Sprintf("\n# %s: momentum %+.3f, technical %+.3f, volume %+.3f, quality %+.3f",
			regime, momentumChange, technicalChange, volumeChange, qualityChange)
	}

	header += "\n"

	// Add weights
	for regime, candidate := range candidateWeights {
		output[regime] = map[string]float64{
			"momentum_core":      candidate.MomentumCore,
			"technical_residual": candidate.TechnicalResidual,
			"volume_residual":    candidate.VolumeResidual,
			"quality_residual":   candidate.QualityResidual,
		}
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(output)
	if err != nil {
		return err
	}

	// Combine header and YAML
	finalContent := header + "\n" + string(yamlData)

	return os.WriteFile(filePath, []byte(finalContent), 0644)
}
