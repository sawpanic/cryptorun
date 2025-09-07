package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cryptorun",
	Short: "CryptoRun v3.2.1 - Real-time cryptocurrency momentum scanner",
	Long: `CryptoRun v3.2.1 - Real-time 6–48h cryptocurrency momentum scanner
powered by free, keyless exchange-native APIs with explainable trading signals.

Key Features:
• Multi-timeframe momentum analysis (1h, 4h, 12h, 24h, 7d)
• Microstructure gates (spread <50bps, depth ±2% ≥$100k, VADR ≥1.75×)
• Safety guards (freshness, fatigue, late-fill detection)
• Regime-adaptive factor weighting
• Exchange-native data only (Kraken/Binance/Coinbase/OKX)
• Comprehensive benchmark and diagnostics suite

Architecture:
All commands route through unified application layer pipelines to ensure
consistent behavior between CLI flags and interactive menu actions.`,
	Version: "v3.2.1",
}

// Global flags
var (
	verbose bool
	logFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log file path (default: stderr)")
}

// initConfig initializes configuration and logging
func initConfig() {
	// Configure zerolog
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Configure log output
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal().Err(err).Str("file", logFile).Msg("Failed to open log file")
		}
		log.Logger = log.Output(file)
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Log startup
	log.Info().
		Str("version", "v3.2.1").
		Bool("verbose", verbose).
		Str("log_file", logFile).
		Msg("CryptoRun initialized")
}

// main executes the root command
func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
