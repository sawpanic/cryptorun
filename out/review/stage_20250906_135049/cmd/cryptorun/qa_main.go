package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/qa"
)

func runQA(cmd *cobra.Command, args []string) error {
	progress, _ := cmd.Flags().GetString("progress")
	resume, _ := cmd.Flags().GetBool("resume")
	ttl, _ := cmd.Flags().GetInt("ttl")
	venuesFlag, _ := cmd.Flags().GetString("venues")
	maxSample, _ := cmd.Flags().GetInt("max-sample")
	verify, _ := cmd.Flags().GetBool("verify")
	failOnStubs, _ := cmd.Flags().GetBool("fail-on-stubs")

	// Parse venues
	var venues []string
	if venuesFlag != "" {
		venues = strings.Split(venuesFlag, ",")
		for i, v := range venues {
			venues[i] = strings.TrimSpace(v)
		}
	} else {
		venues = []string{"kraken", "okx", "coinbase"}
	}

	// Validate progress mode
	switch progress {
	case "auto", "plain", "json":
		// valid
	default:
		return fmt.Errorf("invalid progress mode: %s (must be auto|plain|json)", progress)
	}

	// Setup QA configuration
	config := qa.Config{
		Progress:      progress,
		Resume:        resume,
		TTL:           time.Duration(ttl) * time.Second,
		Venues:        venues,
		MaxSample:     maxSample,
		ArtifactsDir:  "out/qa",
		AuditDir:      "out/audit",
		ProviderTTL:   300 * time.Second,
		Verify:        verify,
		FailOnStubs:   failOnStubs,
	}

	log.Info().
		Str("progress", progress).
		Bool("resume", resume).
		Int("ttl_seconds", ttl).
		Strs("venues", venues).
		Int("max_sample", maxSample).
		Bool("verify", verify).
		Bool("fail_on_stubs", failOnStubs).
		Msg("Starting QA runner")

	// Create QA runner
	runner := qa.NewRunner(config)

	// Set timeout for entire QA process
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Execute QA phases
	result, err := runner.Run(ctx)
	if err != nil {
		log.Error().Err(err).Msg("QA runner failed")
		return err
	}

	// Handle results
	if !result.Success {
		fmt.Printf("❌ QA FAIL: %s\n", result.FailureReason)
		if result.Hint != "" {
			fmt.Printf("💡 Hint: %s\n", result.Hint)
		}
		os.Exit(1)
	}

	fmt.Printf("✅ QA PASS: All %d phases completed successfully\n", len(result.PhaseResults))
	fmt.Printf("📁 Artifacts: %s\n", result.ArtifactsPath)
	
	if progress == "plain" || progress == "auto" {
		fmt.Printf("📊 Summary: %d/%d phases passed, %d providers healthy\n", 
			result.PassedPhases, result.TotalPhases, result.HealthyProviders)
	}

	return nil
}