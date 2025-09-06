package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cryptorun/internal/application/verify"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verification commands for quality assurance",
	Long: `Verification commands for quality assurance and conformance checking.

Available subcommands:
  postmerge    Run post-merge verification (conformance + alignment)`,
}

var postmergeCmd = &cobra.Command{
	Use:   "postmerge",
	Short: "Run post-merge verification (conformance + alignment)",
	Long: `Run comprehensive post-merge verification including:

1. Conformance suite (single path, weight normalization, social cap)
2. Top gainers alignment with CoinGecko (n≥20 for 1h and 24h windows)  
3. Diagnostics policy check (spec_pnl_pct basis validation)

Outputs compact summary to console and writes detailed artifacts to out/verify/`,
	Example: `  # Run post-merge verification with default windows
  cryptorun verify postmerge

  # Specify custom windows and sample size
  cryptorun verify postmerge --windows 1h,24h --n 20 --progress

  # Run with progress indicators
  cryptorun verify postmerge --progress`,
	RunE: runVerifyPostmerge,
}

var (
	postmergeWindows  []string
	postmergeN        int
	postmergeProgress bool
)

func init() {
	// Add verify command to root
	rootCmd.AddCommand(verifyCmd)
	verifyCmd.AddCommand(postmergeCmd)

	// Postmerge flags
	postmergeCmd.Flags().StringSliceVar(&postmergeWindows, "windows", []string{"1h", "24h"},
		"Time windows for alignment check (1h,24h,7d)")
	postmergeCmd.Flags().IntVar(&postmergeN, "n", 20,
		"Minimum sample size for alignment recommendations")
	postmergeCmd.Flags().BoolVar(&postmergeProgress, "progress", false,
		"Show progress indicators during verification")
}

func runVerifyPostmerge(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Println("🔍 CryptoRun Post-Merge Verification")
	fmt.Println("====================================")

	if postmergeProgress {
		fmt.Println("⏳ Starting verification process...")
	}

	// Build options from flags
	opts := verify.PostmergeOptions{
		Windows:       postmergeWindows,
		MinSampleSize: postmergeN,
		ShowProgress:  postmergeProgress,
		Timestamp:     time.Now(),
	}

	// Run verification
	result, err := verify.RunPostmerge(ctx, opts)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print compact summary
	printVerificationSummary(result)

	// Print artifact locations
	fmt.Println("\n📁 Artifacts:")
	fmt.Printf("   Report: %s\n", result.ReportPath)
	fmt.Printf("   Data:   %s\n", result.DataPath)
	if len(result.BenchmarkPaths) > 0 {
		fmt.Printf("   Bench:  %s\n", strings.Join(result.BenchmarkPaths, ", "))
	}

	// Exit with error code if any failures
	if !result.ConformancePass || !result.AlignmentPass {
		fmt.Println("\n❌ Verification FAILED - see artifacts for details")
		os.Exit(1)
	}

	fmt.Println("\n✅ Verification PASSED - ready for deployment")
	return nil
}

func printVerificationSummary(result *verify.PostmergeResult) {
	fmt.Println("\n📊 CONFORMANCE CONTRACTS")
	fmt.Println("┌─────────────────────────────┬────────┐")
	fmt.Println("│ Contract                    │ Status │")
	fmt.Println("├─────────────────────────────┼────────┤")

	for _, contract := range result.ConformanceResults {
		status := "✅ PASS"
		if !contract.Pass {
			status = "❌ FAIL"
		}
		fmt.Printf("│ %-27s │ %-6s │\n", contract.Name, status)
	}
	fmt.Println("└─────────────────────────────┴────────┘")

	fmt.Println("\n📈 TOPGAINERS ALIGNMENT")
	fmt.Println("┌────────┬─────────┬──────┬──────┬──────┬─────────┐")
	fmt.Println("│ Window │ Jaccard │   τ  │   ρ  │ MAE  │ Overlap │")
	fmt.Println("├────────┼─────────┼──────┼──────┼──────┼─────────┤")

	for _, alignment := range result.AlignmentResults {
		fmt.Printf("│ %-6s │  %.3f  │ %.3f│ %.3f│ %.3f│   %2d/%2d │\n",
			alignment.Window,
			alignment.Jaccard,
			alignment.KendallTau,
			alignment.SpearmanRho,
			alignment.MAE,
			alignment.OverlapCount,
			alignment.TotalCandidates)
	}
	fmt.Println("└────────┴─────────┴──────┴──────┴──────┴─────────┘")

	// Diagnostics policy status
	fmt.Printf("\n🩺 DIAGNOSTICS POLICY: ")
	if result.DiagnosticsPass {
		fmt.Println("✅ spec_pnl_pct basis confirmed")
	} else {
		fmt.Println("❌ raw_24h_change basis detected")
	}
}
