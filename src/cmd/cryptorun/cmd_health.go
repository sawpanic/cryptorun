package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/application/metrics"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check system health and metrics",
	Long: `Check the health status of all system components and optionally
collect performance metrics and operational counters.

Examples:
  cryptorun health
  cryptorun health --metrics --counters
  cryptorun health --format json --output health.json`,
	RunE: runHealth,
}

// Health command flags
var (
	healthIncludeMetrics  bool
	healthIncludeCounters bool
	healthFormat          string
	healthOutputFile      string
)

func init() {
	rootCmd.AddCommand(healthCmd)

	// Optional flags
	healthCmd.Flags().BoolVar(&healthIncludeMetrics, "metrics", false, "Include performance metrics")
	healthCmd.Flags().BoolVar(&healthIncludeCounters, "counters", false, "Include operational counters")
	healthCmd.Flags().StringVar(&healthFormat, "format", "table", "Output format (table|json|yaml)")
	healthCmd.Flags().StringVar(&healthOutputFile, "output", "", "Output file (default: stdout)")
}

// runHealth executes the health check via unified pipeline
func runHealth(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate inputs
	if err := validateHealthInputs(); err != nil {
		return fmt.Errorf("invalid health parameters: %w", err)
	}

	// Configure pipeline options
	opts := metrics.HealthOptions{
		IncludeMetrics:  healthIncludeMetrics,
		IncludeCounters: healthIncludeCounters,
		Format:          healthFormat,
		OutputFile:      healthOutputFile,
	}

	log.Info().
		Str("command", "health").
		Bool("metrics", opts.IncludeMetrics).
		Bool("counters", opts.IncludeCounters).
		Str("format", opts.Format).
		Msg("Executing health check via unified pipeline")

	// SINGLE PIPELINE CALL - unified health snapshot
	snapshot, err := metrics.Snapshot(ctx, opts)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Output results based on format
	if err := outputHealthResults(snapshot, opts); err != nil {
		return fmt.Errorf("failed to output health results: %w", err)
	}

	// Exit with appropriate code based on system status
	exitCode := getHealthExitCode(snapshot.SystemStatus)

	log.Info().
		Str("system_status", snapshot.SystemStatus).
		Int("components", len(snapshot.Components)).
		Str("uptime", snapshot.Uptime).
		Int("exit_code", exitCode).
		Msg("Health check completed")

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// validateHealthInputs ensures health parameters are valid
func validateHealthInputs() error {
	validFormats := []string{"table", "json", "yaml"}
	if !contains(validFormats, healthFormat) {
		return fmt.Errorf("invalid format '%s', must be one of: %v", healthFormat, validFormats)
	}

	return nil
}

// outputHealthResults formats and outputs health snapshot
func outputHealthResults(snapshot *metrics.HealthSnapshot, opts metrics.HealthOptions) error {
	switch opts.Format {
	case "json":
		return outputHealthJSON(snapshot, opts.OutputFile)
	case "yaml":
		return outputHealthYAML(snapshot, opts.OutputFile)
	default: // table
		return outputHealthTable(snapshot, opts.OutputFile)
	}
}

// outputHealthJSON outputs health results in JSON format
func outputHealthJSON(snapshot *metrics.HealthSnapshot, outputFile string) error {
	jsonData, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if outputFile != "" {
		return os.WriteFile(outputFile, jsonData, 0644)
	}

	fmt.Println(string(jsonData))
	return nil
}

// outputHealthYAML outputs health results in YAML format
func outputHealthYAML(snapshot *metrics.HealthSnapshot, outputFile string) error {
	// For now, use JSON as placeholder - would implement YAML marshaling
	return outputHealthJSON(snapshot, outputFile)
}

// outputHealthTable outputs health results in human-readable table format
func outputHealthTable(snapshot *metrics.HealthSnapshot, outputFile string) error {
	var output string

	// System overview
	output += fmt.Sprintf("\n🏥 CryptoRun System Health\n")
	output += fmt.Sprintf("═══════════════════════════\n")
	output += fmt.Sprintf("Timestamp:     %s\n", snapshot.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	output += fmt.Sprintf("System Status: %s\n", getStatusIcon(snapshot.SystemStatus))
	output += fmt.Sprintf("Uptime:        %s\n", snapshot.Uptime)
	output += fmt.Sprintf("Version:       %s\n", snapshot.Version)

	// Component health
	output += fmt.Sprintf("\n🔧 Component Health:\n")
	for name, component := range snapshot.Components {
		icon := getStatusIcon(component.Status)
		output += fmt.Sprintf("  %-15s %s (response: %s)\n", name, icon, component.ResponseTime)
		if component.Message != "" {
			output += fmt.Sprintf("    %s\n", component.Message)
		}
		if component.Errors > 0 || component.Warnings > 0 {
			output += fmt.Sprintf("    Errors: %d, Warnings: %d\n", component.Errors, component.Warnings)
		}
	}

	// Performance metrics (if included)
	if snapshot.Metrics.RequestsTotal > 0 {
		output += fmt.Sprintf("\n📊 Performance Metrics:\n")
		output += fmt.Sprintf("  Requests Total:    %d\n", snapshot.Metrics.RequestsTotal)
		output += fmt.Sprintf("  Requests/Second:   %.1f\n", snapshot.Metrics.RequestsPerSecond)
		output += fmt.Sprintf("  P50 Latency:       %.1fms\n", snapshot.Metrics.P50Latency)
		output += fmt.Sprintf("  P95 Latency:       %.1fms\n", snapshot.Metrics.P95Latency)
		output += fmt.Sprintf("  P99 Latency:       %.1fms\n", snapshot.Metrics.P99Latency)
		output += fmt.Sprintf("  Error Rate:        %.2f%%\n", snapshot.Metrics.ErrorRate*100)
		output += fmt.Sprintf("  Memory Usage:      %.1f MB\n", float64(snapshot.Metrics.MemoryUsage)/1024/1024)
		output += fmt.Sprintf("  CPU Usage:         %.1f%%\n", snapshot.Metrics.CPUUsage)
	}

	// Operational counters (if included)
	if snapshot.Counters.ScanExecutions > 0 {
		output += fmt.Sprintf("\n🔢 Operational Counters:\n")
		output += fmt.Sprintf("  Scan Executions:     %d\n", snapshot.Counters.ScanExecutions)
		output += fmt.Sprintf("  Benchmark Runs:      %d\n", snapshot.Counters.BenchmarkRuns)
		output += fmt.Sprintf("  Diagnostic Runs:     %d\n", snapshot.Counters.DiagnosticRuns)
		output += fmt.Sprintf("  Health Checks:       %d\n", snapshot.Counters.HealthChecks)
		output += fmt.Sprintf("  Cache Hits:          %d\n", snapshot.Counters.CacheHits)
		output += fmt.Sprintf("  Cache Misses:        %d\n", snapshot.Counters.CacheMisses)
		output += fmt.Sprintf("  API Calls Total:     %d\n", snapshot.Counters.APICallsTotal)
		output += fmt.Sprintf("  Circuit Breaker Trips: %d\n", snapshot.Counters.CircuitBreakerTrips)

		if snapshot.Counters.CacheHits+snapshot.Counters.CacheMisses > 0 {
			hitRate := float64(snapshot.Counters.CacheHits) / float64(snapshot.Counters.CacheHits+snapshot.Counters.CacheMisses) * 100
			output += fmt.Sprintf("  Cache Hit Rate:      %.1f%%\n", hitRate)
		}
	}

	output += fmt.Sprintf("\n")

	// Output to file or stdout
	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(output), 0644)
	}

	fmt.Print(output)
	return nil
}

// getStatusIcon returns an icon and text for status
func getStatusIcon(status string) string {
	switch status {
	case "healthy":
		return "✅ HEALTHY"
	case "warning", "degraded":
		return "⚠️  WARNING"
	case "down", "error":
		return "❌ DOWN"
	default:
		return "❓ UNKNOWN"
	}
}

// getHealthExitCode returns appropriate exit code for system status
func getHealthExitCode(status string) int {
	switch status {
	case "healthy":
		return 0
	case "warning", "degraded":
		return 1
	case "down", "error":
		return 2
	default:
		return 3
	}
}
