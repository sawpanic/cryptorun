//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// benchFactorweightsCmd provides backward compatibility stub
var benchFactorweightsCmd = &cobra.Command{
	Use:   "factorweights",
	Short: "Removed in v1—use Unified Composite",
	Long: `The FactorWeights vs Unified benchmark has been removed in v1.

The legacy FactorWeights scoring system has been retired in favor of a single,
unified composite scoring system. This eliminates dual-path maintenance and
improves consistency.

Use these alternatives:
• cryptorun menu → Scanner / Unified Composite (single scoring path)
• cryptorun bench topgainers (validates against market data)

For details, see CHANGELOG.md and docs/ARCHITECTURE.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "❌ FactorWeights benchmark removed in v1—use Unified Composite")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Alternatives:")
		fmt.Fprintln(os.Stderr, "  cryptorun menu → Scanner / Unified Composite")
		fmt.Fprintln(os.Stderr, "  cryptorun bench topgainers")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "See CHANGELOG.md for migration details.")
		os.Exit(1)
		return nil
	},
}

func init() {
	benchCmd.AddCommand(benchFactorweightsCmd)
}
