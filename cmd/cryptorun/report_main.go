package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/reports/regime"
)

// runReportRegime generates weekly regime analysis report
func runReportRegime(cmd *cobra.Command, args []string) error {
	// Get flags
	since, _ := cmd.Flags().GetString("since")
	outputDir, _ := cmd.Flags().GetString("out")
	includeCharts, _ := cmd.Flags().GetBool("charts")
	pitTimestamp, _ := cmd.Flags().GetBool("pit")

	// Parse duration
	period, err := parseDuration(since)
	if err != nil {
		return fmt.Errorf("invalid duration '%s': %w", since, err)
	}

	log.Info().
		Str("period", period.String()).
		Str("output_dir", outputDir).
		Bool("include_charts", includeCharts).
		Bool("pit_timestamp", pitTimestamp).
		Msg("Generating regime weekly report")

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Configure report generator
	config := regime.ReportConfig{
		Period:        period,
		OutputDir:     outputDir,
		IncludeCharts: includeCharts,
		PITTimestamp:  pitTimestamp,
		KPIThresholds: regime.DefaultKPIThresholds,
	}

	// Generate report
	generator := regime.NewReportGenerator()
	if err := generator.Generate(config); err != nil {
		return fmt.Errorf("failed to generate regime report: %w", err)
	}

	// Display completion message
	fmt.Printf("✅ Regime weekly report generated\n")
	fmt.Printf("📁 Output directory: %s\n", outputDir)
	fmt.Printf("📊 Period analyzed: %s\n", period.String())

	// List generated files
	files, err := os.ReadDir(outputDir)
	if err == nil {
		fmt.Printf("\n📄 Generated artifacts:\n")
		for _, file := range files {
			if !file.IsDir() {
				fmt.Printf("  • %s\n", file.Name())
			}
		}
	}

	return nil
}

// parseDuration converts duration strings like "28d", "4w", "1m" to time.Duration
func parseDuration(since string) (time.Duration, error) {
	if since == "" {
		return 28 * 24 * time.Hour, nil // Default: 28 days
	}

	// Handle common duration formats
	switch {
	case since == "1w" || since == "week":
		return 7 * 24 * time.Hour, nil
	case since == "2w":
		return 14 * 24 * time.Hour, nil
	case since == "4w" || since == "month":
		return 28 * 24 * time.Hour, nil
	case since == "28d":
		return 28 * 24 * time.Hour, nil
	case since == "30d":
		return 30 * 24 * time.Hour, nil
	case since == "90d":
		return 90 * 24 * time.Hour, nil
	default:
		// Try to parse as standard duration
		return time.ParseDuration(since)
	}
}

// Additional report commands can be added here for other report types
func runReportPerformance(cmd *cobra.Command, args []string) error {
	fmt.Println("Performance reporting not yet implemented")
	return nil
}

func runReportPortfolio(cmd *cobra.Command, args []string) error {
	fmt.Println("Portfolio reporting not yet implemented")
	return nil
}
