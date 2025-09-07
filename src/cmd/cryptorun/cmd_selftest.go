package main

import (
	"fmt"
	"path/filepath"

	"github.com/sawpanic/cryptorun/internal/application/selftest"
	"github.com/spf13/cobra"
)

func newSelftestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "selftest",
		Short: "Run offline resilience self-test suite",
		Long: `Run comprehensive offline self-test suite including:
- Atomicity validation (temp-then-rename pattern)
- Universe hygiene (USD-only, min ADV $100k, valid hash)
- Gate validation on test fixtures
- Microstructure validation (spread<50bps, depth≥$100k)
- Menu integrity check

Generates out/selftest/report.md with pass/fail status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create output directory
			outDir := "out/selftest"

			// Initialize self-test runner
			runner := selftest.NewRunner()

			// Run all tests
			results, err := runner.RunAllTests()
			if err != nil {
				return fmt.Errorf("self-test execution failed: %w", err)
			}

			// Generate report
			reportPath := filepath.Join(outDir, "report.md")
			if err := runner.GenerateReport(results, reportPath); err != nil {
				return fmt.Errorf("failed to generate report: %w", err)
			}

			// Print summary
			fmt.Printf("Self-test completed. Report: %s\n", reportPath)
			fmt.Printf("Overall status: %s\n", results.OverallStatus)
			fmt.Printf("Tests passed: %d/%d\n", results.PassedCount, results.TotalCount)

			// Exit with non-zero if any test failed
			if results.OverallStatus != "PASS" {
				return fmt.Errorf("self-test failed")
			}

			return nil
		},
	}

	return cmd
}
