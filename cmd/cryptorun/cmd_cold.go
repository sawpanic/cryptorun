//go:build ignore
// +build ignore

package main

import (
	"github.com/spf13/cobra"
)

// coldCmd represents the cold tier command group
var coldCmd = &cobra.Command{
	Use:   "cold",
	Short: "Cold tier data operations",
	Long: `Commands for working with cold tier historical data storage.
	
The cold tier handles historical data in compressed formats with point-in-time integrity.
Supports both CSV and Parquet formats with optional compression.`,
}

func init() {
	rootCmd.AddCommand(coldCmd)
}