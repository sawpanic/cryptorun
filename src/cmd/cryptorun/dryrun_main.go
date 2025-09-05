package main

import (
	"context"
	"fmt"
	"time"

	"cryptorun/application"
)

// handleDryrun handles the dry-run menu option
func (ui *MenuUI) handleDryrun(ctx context.Context) error {
	fmt.Println("🧪 Dry-run - Testing scanning pipeline with mock data")
	
	executor := application.NewDryrunExecutor()
	
	// Set a reasonable timeout for dry run
	dryrunCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	
	fmt.Println("• Running scan pipeline...")
	result, err := executor.ExecuteDryrun(dryrunCtx)
	if err != nil {
		return fmt.Errorf("dry run failed: %w", err)
	}
	
	fmt.Println("• Analyzing coverage...")
	fmt.Println("• Updating changelog...")
	
	// Print the 4-line summary
	executor.PrintSummary(result)
	
	fmt.Printf("\n📄 DRYRUN line appended to CHANGELOG.md\n")
	
	return nil
}