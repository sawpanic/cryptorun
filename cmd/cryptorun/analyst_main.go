package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/analyst"
	"github.com/rs/zerolog/log"
)

func runAnalystCoverage() {
	log.Info().Msg("Starting Analyst Coverage Analysis")

	// Default paths - can be made configurable later
	outputDir := filepath.Join("data", "analyst", time.Now().Format("2006-01-02_15-04-05"))
	candidatesPath := filepath.Join("data", "scan", "latest_candidates.jsonl")
	configPath := filepath.Join("config", "quality_policies.json")

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal().Err(err).Str("dir", outputDir).Msg("Failed to create output directory")
	}

	// Check if candidates file exists, use fixtures if not
	useFixtures := true
	if _, err := os.Stat(candidatesPath); err == nil {
		log.Info().Str("path", candidatesPath).Msg("Found candidates file, using live data")
		useFixtures = false
	} else {
		log.Info().Str("path", candidatesPath).Msg("Candidates file not found, using fixtures")
	}

	// Create and run analyst
	runner := analyst.NewAnalystRunner(outputDir, candidatesPath, configPath, useFixtures)

	if err := runner.Run(); err != nil {
		log.Fatal().Err(err).Msg("Analyst coverage analysis failed")
	}

	log.Info().Str("output_dir", outputDir).Msg("Analyst coverage analysis completed")
	fmt.Printf("\n✅ Analyst Coverage Analysis Complete\n")
	fmt.Printf("📊 Results written to: %s\n", outputDir)
	fmt.Printf("📄 Files generated:\n")
	fmt.Printf("   • winners.json    - Top performers by timeframe\n")
	fmt.Printf("   • misses.jsonl    - Missed opportunities with reasons\n")
	fmt.Printf("   • coverage.json   - Coverage metrics summary\n")
	fmt.Printf("   • report.json     - Full coverage report\n")
	fmt.Printf("   • report.md       - Human-readable analysis\n")
	fmt.Printf("\n💡 Review report.md for key insights and policy status\n")
}
