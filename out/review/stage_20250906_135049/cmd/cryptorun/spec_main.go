package main

import (
	"github.com/spf13/cobra"
	"cryptorun/internal/spec"
)

func runSpecSuite(cmd *cobra.Command, args []string) error {
	runner := spec.NewSpecRunner()
	_ = runner
	return nil
}