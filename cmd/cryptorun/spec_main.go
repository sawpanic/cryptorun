package main

import (
	"github.com/sawpanic/cryptorun/internal/spec"
	"github.com/spf13/cobra"
)

func runSpecSuite(cmd *cobra.Command, args []string) error {
	runner := spec.NewSpecRunner()
	_ = runner
	return nil
}
