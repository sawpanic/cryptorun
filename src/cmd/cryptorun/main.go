package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/src/infrastructure/providers"
)

var (
	probeTimeout   time.Duration
	probeFormat    string
	probeVerbose   bool
	probeConfigPath string
)

// rootCmd is the base command for the CryptoRun CLI
var rootCmd = &cobra.Command{
	Use:   "cryptorun",
	Short: "CryptoRun cryptocurrency momentum scanner",
	Long: `CryptoRun is a 6-48 hour cryptocurrency momentum scanner that provides
explainable trading signals with comprehensive safeguards and regime awareness.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("CryptoRun v3.2.1 - Provider Interface & Live Connector System")
		fmt.Println("Use 'cryptorun providers probe' to test provider capabilities")
	},
}

// providersCmd is the parent command for provider-related operations
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage and test data providers",
	Long: `Commands for managing and testing cryptocurrency data providers.
CryptoRun supports multiple exchanges (Binance, OKX, Coinbase, Kraken) with
configurable fallback chains and capability-based routing.`,
}

// probeCmd implements the 'cryptorun providers probe' command
var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Probe provider capabilities and health status",
	Long: `Probe all configured providers to test their capabilities and health status.
This command connects to each provider, tests their endpoints, and reports
latency and availability information with full provenance data.

Example usage:
  cryptorun providers probe                    # Probe all providers
  cryptorun providers probe --format=json     # JSON output
  cryptorun providers probe --verbose         # Detailed output
  cryptorun providers probe --timeout=10s     # Custom timeout`,
	RunE: runProvidersProbe,
}

func init() {
	// Add providers command to root
	rootCmd.AddCommand(providersCmd)
	
	// Add probe command to providers
	providersCmd.AddCommand(probeCmd)
	
	// Set up probe command flags
	probeCmd.Flags().DurationVar(&probeTimeout, "timeout", 30*time.Second, "Timeout for provider probes")
	probeCmd.Flags().StringVar(&probeFormat, "format", "table", "Output format: table, json, csv")
	probeCmd.Flags().BoolVar(&probeVerbose, "verbose", false, "Show detailed capability information")
	probeCmd.Flags().StringVar(&probeConfigPath, "config", "config/providers.yaml", "Path to provider configuration file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runProvidersProbe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	
	// Get absolute path for config
	configPath, err := filepath.Abs(probeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}
	
	// Initialize configurable provider registry
	registry, err := providers.NewConfigurableProviderRegistry(configPath)
	if err != nil {
		return fmt.Errorf("failed to initialize provider registry: %w", err)
	}
	
	// Probe all providers
	report, err := registry.ProbeCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe capabilities: %w", err)
	}
	
	// Output results in requested format
	switch strings.ToLower(probeFormat) {
	case "json":
		return outputJSON(report)
	case "csv":
		return outputCSV(report)
	case "table":
		fallthrough
	default:
		return outputTable(report)
	}
}

func outputJSON(report *providers.CapabilityReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func outputCSV(report *providers.CapabilityReport) error {
	// CSV header
	fmt.Println("Provider,Capability,Supported,Available,LatencyMs,Error")
	
	for _, provider := range report.Providers {
		capabilities := []string{
			"funding", "spot_trades", "orderbook_l2", "kline_data",
			"supply_reserves", "whale_detection", "cvd",
		}
		
		for _, cap := range capabilities {
			status, exists := provider.Capabilities[cap]
			if !exists {
				status = providers.CapabilityStatus{Supported: false, Available: false}
			}
			
			fmt.Printf("%s,%s,%t,%t,%d,\"%s\"\n",
				provider.Name,
				cap,
				status.Supported,
				status.Available,
				status.LatencyMs,
				strings.ReplaceAll(status.Error, "\"", "\"\""), // Escape quotes
			)
		}
	}
	
	return nil
}

func outputTable(report *providers.CapabilityReport) error {
	fmt.Printf("Provider Capability Report (Generated: %s)\n\n", report.Timestamp.Format(time.RFC3339))
	
	// Summary table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	
	fmt.Fprintln(w, "Provider\tStatus\tLatency\tCapabilities")
	fmt.Fprintln(w, "--------\t------\t-------\t------------")
	
	// Sort providers by name for consistent output
	sort.Slice(report.Providers, func(i, j int) bool {
		return report.Providers[i].Name < report.Providers[j].Name
	})
	
	for _, provider := range report.Providers {
		status := "❌ DOWN"
		avgLatency := 0
		supportedCount := 0
		availableCount := 0
		totalLatency := 0
		hasAvailable := false
		
		for _, capStatus := range provider.Capabilities {
			if capStatus.Supported {
				supportedCount++
				if capStatus.Available {
					availableCount++
					hasAvailable = true
					totalLatency += capStatus.LatencyMs
				}
			}
		}
		
		if hasAvailable {
			status = "✅ UP"
			if availableCount > 0 {
				avgLatency = totalLatency / availableCount
			}
		}
		
		capSummary := fmt.Sprintf("%d/%d available", availableCount, supportedCount)
		
		fmt.Fprintf(w, "%s\t%s\t%dms\t%s\n",
			provider.Name,
			status,
			avgLatency,
			capSummary,
		)
	}
	
	w.Flush()
	
	// Detailed capability matrix if verbose
	if probeVerbose {
		fmt.Println("\nDetailed Capability Matrix:")
		fmt.Println()
		
		capabilityOrder := []string{
			"funding", "spot_trades", "orderbook_l2", "kline_data",
			"supply_reserves", "whale_detection", "cvd",
		}
		
		// Header
		w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprint(w2, "Provider")
		for _, cap := range capabilityOrder {
			fmt.Fprintf(w2, "\t%s", strings.Title(strings.ReplaceAll(cap, "_", " ")))
		}
		fmt.Fprintln(w2)
		
		// Separator
		fmt.Fprint(w2, "--------")
		for range capabilityOrder {
			fmt.Fprint(w2, "\t--------")
		}
		fmt.Fprintln(w2)
		
		// Provider rows
		for _, provider := range report.Providers {
			fmt.Fprint(w2, provider.Name)
			for _, cap := range capabilityOrder {
				status, exists := provider.Capabilities[cap]
				symbol := "❌"
				
				if exists && status.Supported {
					if status.Available {
						symbol = "✅"
						if status.LatencyMs > 0 {
							symbol = fmt.Sprintf("✅(%dms)", status.LatencyMs)
						}
					} else {
						symbol = "⚠️"
						if status.Error != "" {
							symbol = fmt.Sprintf("⚠️(%s)", truncateError(status.Error))
						}
					}
				} else if exists {
					symbol = "➖" // Not supported
				}
				
				fmt.Fprintf(w2, "\t%s", symbol)
			}
			fmt.Fprintln(w2)
		}
		
		w2.Flush()
		
		// Error details if any
		hasErrors := false
		for _, provider := range report.Providers {
			for _, status := range provider.Capabilities {
				if status.Error != "" {
					hasErrors = true
					break
				}
			}
			if hasErrors {
				break
			}
		}
		
		if hasErrors {
			fmt.Println("\nError Details:")
			for _, provider := range report.Providers {
				for cap, status := range provider.Capabilities {
					if status.Error != "" {
						fmt.Printf("  %s/%s: %s\n", provider.Name, cap, status.Error)
					}
				}
			}
		}
	}
	
	// Configuration summary
	fmt.Printf("\nConfiguration: %s\n", probeConfigPath)
	fmt.Printf("Probe timeout: %v\n", probeTimeout)
	
	return nil
}

func truncateError(err string) string {
	if len(err) <= 20 {
		return err
	}
	return err[:17] + "..."
}