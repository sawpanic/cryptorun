package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/scan"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade"
)

// scanOfflineCmd implements the offline scanning CLI command
func scanOfflineCmd(args []string) error {
	config := scan.DefaultScanConfig()
	
	// Parse command line arguments
	if err := parseOfflineScanArgs(args, &config); err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}
	
	// Print configuration if verbose
	if config.DryRun {
		printScanConfig(config)
	}
	
	// Initialize data facade with fake data for offline operation
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()
	
	// Create offline scanner
	scanner := scan.NewOfflineScanner(dataFacade, config)
	
	// Execute scan
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	fmt.Fprintf(os.Stderr, "🔍 Starting offline momentum scan...\n")
	startTime := time.Now()
	
	output, err := scanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	
	scanDuration := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "✅ Scan completed in %v\n", scanDuration)
	
	// Print summary to stderr
	printScanSummary(output.Summary)
	
	// Write results to specified output
	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = "stdout"
	}
	
	if err := scanner.WriteOutput(output, outputFile); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	
	// Print errors if any
	if len(output.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  %d errors occurred during scan:\n", len(output.Errors))
		for i, err := range output.Errors {
			fmt.Fprintf(os.Stderr, "  %d. %v\n", i+1, err)
		}
	}
	
	return nil
}

// parseOfflineScanArgs parses command line arguments for offline scan
func parseOfflineScanArgs(args []string, config *scan.ScanConfig) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		
		switch {
		case arg == "--help" || arg == "-h":
			printOfflineScanHelp()
			os.Exit(0)
			
		case arg == "--symbols" || arg == "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("--symbols requires a value")
			}
			i++
			symbolsList := args[i]
			config.Symbols = strings.Split(symbolsList, ",")
			
		case arg == "--output" || arg == "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("--output requires a value")
			}
			i++
			config.OutputFile = args[i]
			
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("--format requires a value")
			}
			i++
			format := strings.ToLower(args[i])
			switch format {
			case "json":
				config.OutputFormat = scan.OutputJSON
			case "csv":
				config.OutputFormat = scan.OutputCSV
			case "tsv":
				config.OutputFormat = scan.OutputTSV
			default:
				return fmt.Errorf("unsupported format: %s (supported: json, csv, tsv)", format)
			}
			
		case arg == "--min-score":
			if i+1 >= len(args) {
				return fmt.Errorf("--min-score requires a value")
			}
			i++
			score, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return fmt.Errorf("invalid min-score: %w", err)
			}
			config.MinScore = score
			
		case arg == "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			i++
			count, err := strconv.Atoi(args[i])
			if err != nil {
				return fmt.Errorf("invalid max-results: %w", err)
			}
			config.MaxResults = count
			
		case arg == "--sort-by":
			if i+1 >= len(args) {
				return fmt.Errorf("--sort-by requires a value")
			}
			i++
			sortBy := strings.ToLower(args[i])
			switch sortBy {
			case "score":
				config.SortBy = scan.SortByScore
			case "momentum":
				config.SortBy = scan.SortByMomentum
			case "symbol":
				config.SortBy = scan.SortBySymbol
			case "volume":
				config.SortBy = scan.SortByVolume
			case "timestamp":
				config.SortBy = scan.SortByTimestamp
			default:
				return fmt.Errorf("unsupported sort criteria: %s", sortBy)
			}
			
		case arg == "--attribution":
			if i+1 >= len(args) {
				return fmt.Errorf("--attribution requires a value")
			}
			i++
			level := strings.ToLower(args[i])
			switch level {
			case "minimal":
				config.AttributionLevel = scan.AttributionMinimal
			case "basic":
				config.AttributionLevel = scan.AttributionBasic
			case "full":
				config.AttributionLevel = scan.AttributionFull
			case "debug":
				config.AttributionLevel = scan.AttributionDebug
			default:
				return fmt.Errorf("unsupported attribution level: %s", level)
			}
			
		case arg == "--dry-run":
			config.DryRun = true
			
		case arg == "--parallel":
			config.Parallel = true
			
		case arg == "--no-headers":
			config.IncludeHeaders = false
			
		case arg == "--timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			i++
			timeout, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("invalid timeout: %w", err)
			}
			config.Timeout = timeout
			
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			// Treat as symbol if no flag prefix
			if config.Symbols == nil {
				config.Symbols = []string{}
			}
			config.Symbols = append(config.Symbols, arg)
		}
	}
	
	return nil
}

// printOfflineScanHelp prints help information for the offline scan command
func printOfflineScanHelp() {
	help := `CryptoRun Offline Momentum Scanner

USAGE:
    cryptorun scan-offline [OPTIONS] [SYMBOLS...]

DESCRIPTION:
    Performs offline cryptocurrency momentum scanning using deterministic fake data.
    Generates ranked results with comprehensive attribution and scoring breakdown.

OPTIONS:
    -s, --symbols SYMBOLS       Comma-separated list of symbols to scan (default: all supported)
    -o, --output FILE           Output file path (default: stdout)  
    -f, --format FORMAT         Output format: json, csv, tsv (default: json)
    --min-score SCORE           Minimum score filter (default: 0.0)
    --max-results COUNT         Maximum number of results (default: 50)
    --sort-by CRITERIA          Sort by: score, momentum, symbol, volume, timestamp (default: score)
    --attribution LEVEL         Attribution level: minimal, basic, full, debug (default: basic)
    --dry-run                   Show what would be done without executing
    --parallel                  Enable parallel processing
    --no-headers                Skip CSV/TSV headers
    --timeout DURATION          Scan timeout (default: 30s)
    -h, --help                  Show this help

EXAMPLES:
    # Scan all symbols, output to stdout as JSON
    cryptorun scan-offline

    # Scan specific symbols, save as CSV
    cryptorun scan-offline --symbols BTC-USD,ETH-USD --format csv --output results.csv

    # Top 10 results with full attribution
    cryptorun scan-offline --max-results 10 --attribution full

    # Filter by minimum score, sort by momentum
    cryptorun scan-offline --min-score 70 --sort-by momentum

    # Debug mode with comprehensive attribution
    cryptorun scan-offline --attribution debug --format json --output debug.json

SUPPORTED SYMBOLS:
    BTC-USD, ETH-USD, ADA-USD, DOT-USD, LINK-USD, LTC-USD, XLM-USD, XRP-USD, SOL-USD, MATIC-USD

OUTPUT FORMATS:
    json - Structured JSON with full attribution
    csv  - Comma-separated values with headers
    tsv  - Tab-separated values with headers

ATTRIBUTION LEVELS:
    minimal - Core scores only
    basic   - Core scores + processing metadata
    full    - Basic + factor breakdowns
    debug   - Full + raw data and internal state
`
	fmt.Print(help)
}

// printScanConfig prints the current scan configuration
func printScanConfig(config scan.ScanConfig) {
	fmt.Fprintf(os.Stderr, "📋 Scan Configuration:\n")
	fmt.Fprintf(os.Stderr, "  Symbols: %v\n", config.Symbols)
	fmt.Fprintf(os.Stderr, "  Output Format: %s\n", config.OutputFormat)
	fmt.Fprintf(os.Stderr, "  Output File: %s\n", config.OutputFile)
	fmt.Fprintf(os.Stderr, "  Min Score: %.1f\n", config.MinScore)
	fmt.Fprintf(os.Stderr, "  Max Results: %d\n", config.MaxResults)
	fmt.Fprintf(os.Stderr, "  Sort By: %s\n", config.SortBy)
	fmt.Fprintf(os.Stderr, "  Attribution: %s\n", config.AttributionLevel)
	fmt.Fprintf(os.Stderr, "  Parallel: %t\n", config.Parallel)
	fmt.Fprintf(os.Stderr, "  Timeout: %v\n", config.Timeout)
	fmt.Fprintf(os.Stderr, "\n")
}

// printScanSummary prints scan summary statistics
func printScanSummary(summary scan.ScanSummary) {
	fmt.Fprintf(os.Stderr, "\n📊 Scan Summary:\n")
	fmt.Fprintf(os.Stderr, "  Total Symbols: %d\n", summary.TotalSymbols)
	fmt.Fprintf(os.Stderr, "  Successful: %d\n", summary.SuccessfulScans)
	fmt.Fprintf(os.Stderr, "  Failed: %d\n", summary.FailedScans)
	fmt.Fprintf(os.Stderr, "  Total Time: %v\n", summary.TotalTime)
	fmt.Fprintf(os.Stderr, "  Average Time: %v\n", summary.AverageTime)
	fmt.Fprintf(os.Stderr, "  Cache Hit Rate: %.1f%%\n", summary.CacheHitRate)
	
	if summary.SuccessfulScans > 0 {
		fmt.Fprintf(os.Stderr, "  Regime Detected: %s\n", summary.RegimeDetected)
		fmt.Fprintf(os.Stderr, "  Top Score: %.2f\n", summary.TopScore)
		fmt.Fprintf(os.Stderr, "  Bottom Score: %.2f\n", summary.BottomScore)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

// Add scan-offline to main command dispatcher
func init() {
	// This would be integrated into the main command dispatcher
	// For now, just ensure it's available for testing
}

// Example of how to integrate into main.go:
//
// func main() {
//     if len(os.Args) < 2 {
//         printUsage()
//         os.Exit(1)
//     }
//     
//     command := os.Args[1]
//     args := os.Args[2:]
//     
//     switch command {
//     case "scan-offline":
//         if err := scanOfflineCmd(args); err != nil {
//             log.Fatalf("scan-offline failed: %v", err)
//         }
//     // ... other commands
//     }
// }