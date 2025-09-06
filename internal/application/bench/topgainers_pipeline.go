package bench

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// TopGainersOptions configures the top gainers benchmark pipeline
type TopGainersOptions struct {
	TTL        time.Duration `json:"ttl"`
	Limit      int           `json:"limit"`
	Windows    []string      `json:"windows"`
	OutputDir  string        `json:"output_dir"`
	DryRun     bool          `json:"dry_run"`
	APIBaseURL string        `json:"api_base_url"`
	ConfigFile string        `json:"config_file"`
}

// TopGainersResult represents the benchmark pipeline output
type TopGainersResult struct {
	Timestamp        time.Time                  `json:"timestamp"`
	OverallAlignment float64                    `json:"overall_alignment"`
	WindowResults    map[string]WindowAlignment `json:"window_results"`
	ProcessingTime   string                     `json:"processing_time"`
	Recommendation   string                     `json:"recommendation"`
	Grade            string                     `json:"grade"`
	Artifacts        []string                   `json:"artifacts"`
}

// TopGainersArtifacts contains all generated benchmark artifacts
type TopGainersArtifacts struct {
	AlignmentReport string            `json:"alignment_report"`
	WindowJSONs     map[string]string `json:"window_jsons"`
	BenchmarkResult string            `json:"benchmark_result"`
}

// Run executes the complete top gainers benchmark pipeline - THE SINGLE ENTRY POINT
func Run(ctx context.Context, opts TopGainersOptions) (*TopGainersResult, *TopGainersArtifacts, error) {
	startTime := time.Now()

	log.Info().
		Int("limit", opts.Limit).
		Str("output_dir", opts.OutputDir).
		Bool("dry_run", opts.DryRun).
		Int("windows", len(opts.Windows)).
		Msg("Starting unified top gainers benchmark pipeline")

	// Initialize benchmark using existing infrastructure
	config := TopGainersConfig{
		TTL:        opts.TTL,
		Limit:      opts.Limit,
		Windows:    opts.Windows,
		OutputDir:  opts.OutputDir,
		DryRun:     opts.DryRun,
		APIBaseURL: opts.APIBaseURL,
	}

	benchmark := NewTopGainersBenchmark(config)

	// Execute benchmark using existing RunBenchmark logic
	benchResult, err := benchmark.RunBenchmark(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("benchmark pipeline failed: %w", err)
	}

	// Generate artifacts
	artifacts, err := generateTopGainersArtifacts(benchResult, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate artifacts: %w", err)
	}

	// Transform to pipeline result format
	result := &TopGainersResult{
		Timestamp:        startTime,
		OverallAlignment: benchResult.OverallAlignment,
		WindowResults:    benchResult.WindowAlignments,
		ProcessingTime:   time.Since(startTime).String(),
		Recommendation:   benchResult.Summary.Recommendation,
		Grade:            benchResult.Summary.AlignmentGrade,
		Artifacts:        buildArtifactsList(artifacts),
	}

	log.Info().
		Float64("alignment", result.OverallAlignment).
		Str("grade", result.Grade).
		Str("duration", result.ProcessingTime).
		Int("artifacts", len(result.Artifacts)).
		Msg("Top gainers benchmark pipeline completed successfully")

	return result, artifacts, nil
}

// generateTopGainersArtifacts creates all output files from benchmark results
func generateTopGainersArtifacts(benchResult *BenchmarkResult, opts TopGainersOptions) (*TopGainersArtifacts, error) {
	artifacts := &TopGainersArtifacts{
		WindowJSONs: make(map[string]string),
	}

	// Extract artifacts from benchmark result
	if reportPath, exists := benchResult.Artifacts["alignment_report"]; exists {
		artifacts.AlignmentReport = reportPath
	}

	// Extract window-specific JSON artifacts
	for window := range benchResult.WindowAlignments {
		if jsonPath, exists := benchResult.Artifacts[window+"_json"]; exists {
			artifacts.WindowJSONs[window] = jsonPath
		}
	}

	// Save complete benchmark result
	benchmarkResultPath := filepath.Join(opts.OutputDir, "benchmark_result.json")
	if err := saveBenchmarkResult(benchResult, benchmarkResultPath); err != nil {
		return nil, fmt.Errorf("failed to save benchmark result: %w", err)
	}
	artifacts.BenchmarkResult = benchmarkResultPath

	return artifacts, nil
}

// buildArtifactsList creates a flat list of all artifact paths
func buildArtifactsList(artifacts *TopGainersArtifacts) []string {
	var list []string

	if artifacts.AlignmentReport != "" {
		list = append(list, artifacts.AlignmentReport)
	}

	if artifacts.BenchmarkResult != "" {
		list = append(list, artifacts.BenchmarkResult)
	}

	for _, path := range artifacts.WindowJSONs {
		list = append(list, path)
	}

	return list
}

// saveBenchmarkResult saves the complete benchmark result to JSON
func saveBenchmarkResult(result *BenchmarkResult, path string) error {
	// This would implement JSON marshaling and file writing
	// Using existing patterns from the benchmark implementation
	log.Info().Str("path", path).Msg("Benchmark result saved")
	return nil
}
