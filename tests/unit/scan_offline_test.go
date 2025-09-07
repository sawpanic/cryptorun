package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/scan"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade"
)

func TestOfflineScanner(t *testing.T) {
	// Setup data facade with fake data
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	// Setup scan configuration
	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD", "ETH-USD", "ADA-USD"}
	scanConfig.MaxResults = 10
	scanConfig.AttributionLevel = scan.AttributionBasic

	// Create scanner
	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)

	// Execute scan
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	if len(output.Results) == 0 {
		t.Error("Expected scan results, got none")
	}

	if len(output.Results) > scanConfig.MaxResults {
		t.Errorf("Expected max %d results, got %d", scanConfig.MaxResults, len(output.Results))
	}

	// Check result structure
	firstResult := output.Results[0]
	if firstResult.Symbol == "" {
		t.Error("Expected non-empty symbol")
	}

	if firstResult.FinalScore == 0 {
		t.Error("Expected non-zero final score")
	}

	if firstResult.Regime == "" {
		t.Error("Expected non-empty regime")
	}

	// Verify weights sum to 1.0 (excluding social)
	weights := firstResult.Weights
	coreWeightSum := weights.MomentumCore + weights.Technical + weights.Volume + weights.Quality
	if coreWeightSum < 0.95 || coreWeightSum > 1.05 {
		t.Errorf("Core weights should sum to ~1.0, got %.3f", coreWeightSum)
	}

	// Verify attribution is present for basic level
	if firstResult.Attribution == nil {
		t.Error("Expected attribution for basic level")
	}

	if len(firstResult.Attribution.DataSources) == 0 {
		t.Error("Expected non-empty data sources in attribution")
	}

	// Check summary statistics
	summary := output.Summary
	if summary.TotalSymbols != len(scanConfig.Symbols) {
		t.Errorf("Expected %d total symbols, got %d", len(scanConfig.Symbols), summary.TotalSymbols)
	}

	if summary.SuccessfulScans == 0 {
		t.Error("Expected some successful scans")
	}

	if summary.TotalTime <= 0 {
		t.Error("Expected positive total time")
	}
}

func TestOfflineScannerSorting(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	// Test sorting by score (descending)
	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD", "ETH-USD", "ADA-USD", "DOT-USD"}
	scanConfig.SortBy = scan.SortByScore
	scanConfig.MaxResults = 10

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results are sorted by score descending
	for i := 1; i < len(output.Results); i++ {
		if output.Results[i-1].FinalScore < output.Results[i].FinalScore {
			t.Errorf("Results not sorted by score descending: %.2f < %.2f at positions %d, %d",
				output.Results[i-1].FinalScore, output.Results[i].FinalScore, i-1, i)
		}
	}

	// Test sorting by symbol
	scanConfig.SortBy = scan.SortBySymbol
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)

	output, err = scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Symbol sort scan failed: %v", err)
	}

	// Verify results are sorted by symbol ascending
	for i := 1; i < len(output.Results); i++ {
		if output.Results[i-1].Symbol > output.Results[i].Symbol {
			t.Errorf("Results not sorted by symbol ascending: %s > %s at positions %d, %d",
				output.Results[i-1].Symbol, output.Results[i].Symbol, i-1, i)
		}
	}
}

func TestOfflineScannerFiltering(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	// Test minimum score filtering
	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD", "ETH-USD", "ADA-USD", "DOT-USD"}
	scanConfig.MinScore = 50.0 // High threshold
	scanConfig.MaxResults = 100

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Filtered scan failed: %v", err)
	}

	// Verify all results meet minimum score
	for _, result := range output.Results {
		if result.FinalScore < scanConfig.MinScore {
			t.Errorf("Result score %.2f below minimum %.2f for symbol %s",
				result.FinalScore, scanConfig.MinScore, result.Symbol)
		}
	}

	// Test max results limiting
	scanConfig.MinScore = 0.0 // Remove score filter
	scanConfig.MaxResults = 2  // Limit to 2 results

	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	output, err = scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Limited scan failed: %v", err)
	}

	if len(output.Results) > scanConfig.MaxResults {
		t.Errorf("Expected max %d results, got %d", scanConfig.MaxResults, len(output.Results))
	}
}

func TestOfflineScannerAttributionLevels(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	symbols := []string{"BTC-USD", "ETH-USD"}

	// Test minimal attribution
	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = symbols
	scanConfig.AttributionLevel = scan.AttributionMinimal

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Minimal attribution scan failed: %v", err)
	}

	// Should have no attribution
	if output.Results[0].Attribution != nil {
		t.Error("Expected no attribution for minimal level")
	}

	// Test basic attribution
	scanConfig.AttributionLevel = scan.AttributionBasic
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)

	output, err = scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Basic attribution scan failed: %v", err)
	}

	// Should have basic attribution
	attr := output.Results[0].Attribution
	if attr == nil {
		t.Error("Expected attribution for basic level")
	}

	if len(attr.DataSources) == 0 {
		t.Error("Expected data sources in basic attribution")
	}

	if attr.ProcessingTime <= 0 {
		t.Error("Expected positive processing time in basic attribution")
	}

	// Test full attribution
	scanConfig.AttributionLevel = scan.AttributionFull
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)

	output, err = scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Full attribution scan failed: %v", err)
	}

	// Should have full attribution with factor breakdowns
	attr = output.Results[0].Attribution
	if attr == nil {
		t.Error("Expected attribution for full level")
	}

	// Note: Factor breakdowns would be populated if the scoring system provided them
	// For now, just verify the structure is present

	// Test debug attribution
	scanConfig.AttributionLevel = scan.AttributionDebug
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)

	output, err = scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Debug attribution scan failed: %v", err)
	}

	// Should have debug attribution
	attr = output.Results[0].Attribution
	if attr == nil {
		t.Error("Expected attribution for debug level")
	}

	// Debug level would include raw factors if available
}

func TestOfflineScannerDryRun(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD"}
	scanConfig.DryRun = true

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	// Scan should still work in dry run mode
	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Dry run scan failed: %v", err)
	}

	// Should still get results
	if len(output.Results) == 0 {
		t.Error("Expected results even in dry run mode")
	}

	// WriteOutput should not create files in dry run
	err = scanner.WriteOutput(output, "test-output.json")
	if err != nil {
		t.Fatalf("WriteOutput failed in dry run: %v", err)
	}
	// Note: In dry run, file shouldn't actually be created
}

func TestOfflineScannerOutputFormats(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD", "ETH-USD"}
	scanConfig.DryRun = true // Don't actually write files

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Test JSON format
	scanConfig.OutputFormat = scan.OutputJSON
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	err = scanner.WriteOutput(output, "test.json")
	if err != nil {
		t.Errorf("JSON output failed: %v", err)
	}

	// Test CSV format
	scanConfig.OutputFormat = scan.OutputCSV
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	err = scanner.WriteOutput(output, "test.csv")
	if err != nil {
		t.Errorf("CSV output failed: %v", err)
	}

	// Test TSV format
	scanConfig.OutputFormat = scan.OutputTSV
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	err = scanner.WriteOutput(output, "test.tsv")
	if err != nil {
		t.Errorf("TSV output failed: %v", err)
	}

	// Test unsupported format
	scanConfig.OutputFormat = "xml"
	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	err = scanner.WriteOutput(output, "test.xml")
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestOfflineScannerErrorHandling(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	// Test with unsupported symbols
	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"INVALID-SYMBOL", "ANOTHER-INVALID"}

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err == nil {
		t.Error("Expected error for unsupported symbols")
	}

	// Test with timeout
	scanConfig.Symbols = []string{"BTC-USD"}
	scanConfig.Timeout = 1 * time.Nanosecond // Very short timeout

	scanner = scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx, cancel := context.WithTimeout(context.Background(), scanConfig.Timeout)
	defer cancel()

	_, err = scanner.Scan(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestDefaultScanConfig(t *testing.T) {
	config := scan.DefaultScanConfig()

	// Verify defaults
	if config.OutputFormat != scan.OutputJSON {
		t.Errorf("Expected JSON output format, got %s", config.OutputFormat)
	}

	if config.SortBy != scan.SortByScore {
		t.Errorf("Expected sort by score, got %s", config.SortBy)
	}

	if config.AttributionLevel != scan.AttributionBasic {
		t.Errorf("Expected basic attribution, got %s", config.AttributionLevel)
	}

	if config.MaxResults != 50 {
		t.Errorf("Expected max results 50, got %d", config.MaxResults)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected 30s timeout, got %v", config.Timeout)
	}

	if config.DryRun {
		t.Error("Expected dry run false by default")
	}

	if !config.IncludeHeaders {
		t.Error("Expected include headers true by default")
	}
}

func TestScanResultStructure(t *testing.T) {
	facadeConfig := datafacade.DefaultFacadeConfig()
	facadeConfig.UseFakesForTesting = true
	dataFacade := datafacade.NewDataFacade(facadeConfig)
	defer dataFacade.Close()

	scanConfig := scan.DefaultScanConfig()
	scanConfig.Symbols = []string{"BTC-USD"}
	scanConfig.AttributionLevel = scan.AttributionFull

	scanner := scan.NewOfflineScanner(dataFacade, scanConfig)
	ctx := context.Background()

	output, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	result := output.Results[0]

	// Check all required fields are present
	if result.Symbol == "" {
		t.Error("Missing symbol")
	}

	if result.Timestamp.IsZero() {
		t.Error("Missing timestamp")
	}

	if result.FinalScore == 0 {
		t.Error("Missing final score")
	}

	if result.Regime == "" {
		t.Error("Missing regime")
	}

	// Check factor components
	if result.MomentumCore == 0 {
		t.Error("Missing momentum core")
	}

	// Check weights
	if result.Weights.MomentumCore == 0 {
		t.Error("Missing momentum weight")
	}

	// Check quality metrics
	if result.OrthogonalityScore == 0 {
		t.Error("Missing orthogonality score")
	}

	// Check attribution
	if result.Attribution == nil {
		t.Error("Missing attribution for full level")
	}
}