package optimization

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// CLIRunner handles command-line optimization execution
type CLIRunner struct {
	config       OptimizerConfig
	dataProvider DataProvider
	evaluator    Evaluator
	reporter     *ReportGenerator
}

// NewCLIRunner creates a new CLI runner
func NewCLIRunner(ledgerPath, outputDir string) *CLIRunner {
	// Set up default configuration
	config := OptimizerConfig{
		MaxIterations:     1000,
		CVFolds:           5,
		PurgeGap:          24 * time.Hour,
		WalkForwardWindow: 30 * 24 * time.Hour, // 30 days
		MinimumSamples:    100,
		RegimeAware:       true,
		ParallelFolds:     false, // Keep simple for now
		RandomSeed:        time.Now().UnixNano(),
		OutputDir:         outputDir,
	}

	// Create components
	dataProvider := NewFileDataProvider(ledgerPath, "", "")
	evaluator := NewStandardEvaluator()
	reporter := NewReportGenerator(outputDir)

	return &CLIRunner{
		config:       config,
		dataProvider: dataProvider,
		evaluator:    evaluator,
		reporter:     reporter,
	}
}

// RunMomentumOptimization runs momentum parameter optimization
func (cr *CLIRunner) RunMomentumOptimization(ctx context.Context) error {
	log.Info().Msg("Starting momentum optimization run")

	// Validate data availability
	err := cr.dataProvider.ValidateDataAvailability()
	if err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}

	// Get data summary
	summary, err := cr.dataProvider.GetDataSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get data summary: %w", err)
	}

	log.Info().
		Int("total_entries", summary.TotalEntries).
		Int("unique_symbols", summary.UniqueSymbols).
		Time("start", summary.StartTime).
		Time("end", summary.EndTime).
		Float64("gate_pass_rate", summary.GatePassRate).
		Msg("Data summary")

	// Check minimum data requirements
	if summary.TotalEntries < cr.config.MinimumSamples {
		return fmt.Errorf("insufficient data: got %d entries, need %d", summary.TotalEntries, cr.config.MinimumSamples)
	}

	// Create momentum optimizer
	cr.config.Target = TargetMomentum
	optimizer := NewMomentumOptimizer(cr.config, cr.dataProvider, cr.evaluator)

	// Run optimization
	result, err := optimizer.Optimize(ctx, cr.config)
	if err != nil {
		return fmt.Errorf("momentum optimization failed: %w", err)
	}

	// Generate reports
	err = cr.reporter.GenerateOptimizationReport(result)
	if err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	log.Info().
		Float64("objective", result.AggregateMetrics.ObjectiveScore).
		Float64("precision_24h", result.AggregateMetrics.Precision20_24h*100).
		Float64("precision_48h", result.AggregateMetrics.Precision20_48h*100).
		Str("id", result.ID).
		Msg("Momentum optimization completed")

	return nil
}

// RunDipOptimization runs dip/reversal parameter optimization
func (cr *CLIRunner) RunDipOptimization(ctx context.Context) error {
	log.Info().Msg("Starting dip optimization run")

	// Validate data availability
	err := cr.dataProvider.ValidateDataAvailability()
	if err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}

	// Get data summary
	summary, err := cr.dataProvider.GetDataSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get data summary: %w", err)
	}

	log.Info().
		Int("total_entries", summary.TotalEntries).
		Time("start", summary.StartTime).
		Time("end", summary.EndTime).
		Msg("Data summary for dip optimization")

	// Check minimum data requirements
	if summary.TotalEntries < cr.config.MinimumSamples {
		return fmt.Errorf("insufficient data: got %d entries, need %d", summary.TotalEntries, cr.config.MinimumSamples)
	}

	// Create dip optimizer
	cr.config.Target = TargetDip
	optimizer := NewDipOptimizer(cr.config, cr.dataProvider, cr.evaluator)

	// Run optimization
	result, err := optimizer.Optimize(ctx, cr.config)
	if err != nil {
		return fmt.Errorf("dip optimization failed: %w", err)
	}

	// Generate reports
	err = cr.reporter.GenerateOptimizationReport(result)
	if err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	log.Info().
		Float64("objective", result.AggregateMetrics.ObjectiveScore).
		Float64("precision_12h", result.AggregateMetrics.Precision20_24h*100).
		Float64("precision_24h", result.AggregateMetrics.Precision20_48h*100).
		Str("id", result.ID).
		Msg("Dip optimization completed")

	return nil
}

// SetMaxIterations sets the maximum number of optimization iterations
func (cr *CLIRunner) SetMaxIterations(maxIter int) {
	cr.config.MaxIterations = maxIter
	log.Info().Int("max_iterations", maxIter).Msg("Updated max iterations")
}

// SetCVFolds sets the number of cross-validation folds
func (cr *CLIRunner) SetCVFolds(folds int) {
	cr.config.CVFolds = folds
	log.Info().Int("cv_folds", folds).Msg("Updated CV folds")
}

// SetRandomSeed sets the random seed for reproducibility
func (cr *CLIRunner) SetRandomSeed(seed int64) {
	cr.config.RandomSeed = seed
	log.Info().Int64("random_seed", seed).Msg("Updated random seed")
}

// ValidateSetup validates that the optimization setup is correct
func (cr *CLIRunner) ValidateSetup(ctx context.Context) error {
	log.Info().Msg("Validating optimization setup")

	// Check data provider
	err := cr.dataProvider.ValidateDataAvailability()
	if err != nil {
		return fmt.Errorf("data provider validation failed: %w", err)
	}

	// Check output directory
	err = cr.reporter.ValidateOutputDirectory()
	if err != nil {
		return fmt.Errorf("output directory validation failed: %w", err)
	}

	// Get data summary to check coverage
	summary, err := cr.dataProvider.GetDataSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get data summary: %w", err)
	}

	// Check data quality
	if summary.GatePassRate < 0.1 {
		log.Warn().Float64("rate", summary.GatePassRate).Msg("Low gate pass rate may affect optimization quality")
	}

	if summary.UniqueSymbols < 10 {
		log.Warn().Int("symbols", summary.UniqueSymbols).Msg("Low symbol diversity may affect generalization")
	}

	timeSpan := summary.EndTime.Sub(summary.StartTime)
	if timeSpan < 30*24*time.Hour {
		log.Warn().Dur("span", timeSpan).Msg("Short time span may limit CV quality")
	}

	log.Info().
		Int("entries", summary.TotalEntries).
		Int("symbols", summary.UniqueSymbols).
		Dur("timespan", timeSpan).
		Float64("gate_pass_rate", summary.GatePassRate).
		Msg("Setup validation completed")

	return nil
}

// GetStatus returns the current status of optimization components
func (cr *CLIRunner) GetStatus() OptimizerStatus {
	return OptimizerStatus{
		Config:       cr.config,
		DataProvider: cr.dataProvider != nil,
		Evaluator:    cr.evaluator != nil,
		Reporter:     cr.reporter != nil,
		CacheStats:   cr.dataProvider.GetCacheStats(),
	}
}

// OptimizerStatus represents the status of optimization components
type OptimizerStatus struct {
	Config       OptimizerConfig `json:"config"`
	DataProvider bool            `json:"data_provider_ready"`
	Evaluator    bool            `json:"evaluator_ready"`
	Reporter     bool            `json:"reporter_ready"`
	CacheStats   map[string]int  `json:"cache_stats"`
}

// ValidateOutputDirectory validates the output directory exists and is writable
func (rg *ReportGenerator) ValidateOutputDirectory() error {
	// Create output directory if it doesn't exist
	err := os.MkdirAll(rg.outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", rg.outputDir, err)
	}

	// Test write access
	testFile := filepath.Join(rg.outputDir, "test_write.tmp")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		return fmt.Errorf("output directory is not writable: %w", err)
	}

	// Clean up test file
	os.Remove(testFile)

	log.Info().Str("dir", rg.outputDir).Msg("Output directory validated")
	return nil
}
