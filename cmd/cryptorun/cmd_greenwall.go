package main

import (
	"fmt"
	"os"
	"strconv"

	"cryptorun/internal/verify/greenwall"
	"github.com/spf13/cobra"
)

func init() {
	var verifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Run verification commands",
		Long:  "Run various verification and validation commands for the CryptoRun system",
	}

	var greenwallCmd = &cobra.Command{
		Use:   "all",
		Short: "Run complete GREEN-WALL verification suite",
		Long: `Run the complete GREEN-WALL verification suite including:
- Unit/E2E tests with coverage
- Microstructure proofs sample
- TopGainers bench sanity
- Smoke90 cached backtest
- Post-merge verifier

Prints a compact ✅/❌ wall with artifact links and exits non-zero on any failures.`,
		RunE: runGreenwall,
	}

	greenwallCmd.Flags().IntP("n", "n", 30, "Number of samples for tests requiring sample size")
	greenwallCmd.Flags().Bool("progress", false, "Show progress indicators during execution")
	greenwallCmd.Flags().Duration("timeout", 0, "Overall timeout for the verification suite (0 for no timeout)")

	verifyCmd.AddCommand(greenwallCmd)
	rootCmd.AddCommand(verifyCmd)
}

func runGreenwall(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("n")
	progress, _ := cmd.Flags().GetBool("progress")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	config := greenwall.Config{
		SampleSize:   n,
		ShowProgress: progress,
		Timeout:      timeout,
	}

	runner := greenwall.NewRunner(config)
	result, err := runner.RunAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GREEN-WALL execution failed: %v\n", err)
		return err
	}

	// Print the GREEN-WALL status
	fmt.Print(result.FormatWall())

	// Exit with non-zero code if any checks failed
	if !result.AllPassed() {
		os.Exit(1)
	}

	return nil
}

// Also add postmerge subcommand for individual use
func init() {
	var postmergeCmd = &cobra.Command{
		Use:   "postmerge",
		Short: "Run post-merge verification checks",
		Long:  "Run post-merge verification to ensure the system is in a consistent state after code changes",
		RunE:  runPostmerge,
	}

	postmergeCmd.Flags().IntP("n", "n", 20, "Number of samples for verification")
	postmergeCmd.Flags().Bool("progress", false, "Show progress indicators")

	// Find the verify command and add postmerge to it
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "verify" {
			cmd.AddCommand(postmergeCmd)
			break
		}
	}
}

func runPostmerge(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("n")
	progress, _ := cmd.Flags().GetBool("progress")

	config := greenwall.Config{
		SampleSize:   n,
		ShowProgress: progress,
	}

	runner := greenwall.NewRunner(config)
	result, err := runner.RunPostmerge()
	if err != nil {
		return fmt.Errorf("post-merge verification failed: %v", err)
	}

	fmt.Printf("Post-merge verification: %s\n", result.PostmergeStatus())

	if !result.PostmergePassed {
		os.Exit(1)
	}

	return nil
}
