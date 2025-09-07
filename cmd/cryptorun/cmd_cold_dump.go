package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/internal/data"
)

// coldDumpCmd represents the cold dump command for debugging cold tier data
var coldDumpCmd = &cobra.Command{
	Use:   "cold-dump",
	Short: "Dump and inspect cold tier data for debugging",
	Long: `Dump and inspect cold tier data (CSV or Parquet) with filtering capabilities.
	
This command helps debug cold tier data by allowing you to:
- Dump data from specific files or time ranges
- Filter by columns to reduce output
- Validate file integrity and schema compliance
- Display metadata and statistics

Examples:
  # Dump all data from a Parquet file
  cryptorun cold-dump --file data/cold/kraken/btc-usd.parquet
  
  # Dump specific columns and time range
  cryptorun cold-dump --file data.parquet --columns ts,symbol,close --from 2025-09-01T00:00:00Z --to 2025-09-02T00:00:00Z
  
  # Show only metadata without dumping data
  cryptorun cold-dump --file data.parquet --metadata-only
  
  # Validate schema compliance
  cryptorun cold-dump --file data.parquet --validate`,
	RunE: runColdDump,
}

var (
	coldDumpFile        string
	coldDumpColumns     string
	coldDumpFrom        string
	coldDumpTo          string
	coldDumpFormat      string
	coldDumpMetadataOnly bool
	coldDumpValidate    bool
	coldDumpLimit       int
	coldDumpOffset      int
)

func init() {
	rootCmd.AddCommand(coldDumpCmd)

	coldDumpCmd.Flags().StringVar(&coldDumpFile, "file", "", "Path to cold tier file (CSV or Parquet)")
	coldDumpCmd.Flags().StringVar(&coldDumpColumns, "columns", "", "Comma-separated list of columns to dump (default: all)")
	coldDumpCmd.Flags().StringVar(&coldDumpFrom, "from", "", "Start time filter (RFC3339 format)")
	coldDumpCmd.Flags().StringVar(&coldDumpTo, "to", "", "End time filter (RFC3339 format)")
	coldDumpCmd.Flags().StringVar(&coldDumpFormat, "format", "table", "Output format: table, csv, json")
	coldDumpCmd.Flags().BoolVar(&coldDumpMetadataOnly, "metadata-only", false, "Show only metadata, don't dump data")
	coldDumpCmd.Flags().BoolVar(&coldDumpValidate, "validate", false, "Validate file schema and integrity")
	coldDumpCmd.Flags().IntVar(&coldDumpLimit, "limit", 100, "Maximum number of rows to dump (0 = no limit)")
	coldDumpCmd.Flags().IntVar(&coldDumpOffset, "offset", 0, "Number of rows to skip")

	coldDumpCmd.MarkFlagRequired("file")
}

func runColdDump(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate file exists
	if _, err := os.Stat(coldDumpFile); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", coldDumpFile)
	}

	// Parse time range if provided
	var timeRange *data.TimeRange
	if coldDumpFrom != "" || coldDumpTo != "" {
		tr, err := parseTimeRange(coldDumpFrom, coldDumpTo)
		if err != nil {
			return fmt.Errorf("invalid time range: %w", err)
		}
		timeRange = &tr
	}

	// Parse columns filter
	var columns []string
	if coldDumpColumns != "" {
		columns = strings.Split(coldDumpColumns, ",")
		for i := range columns {
			columns[i] = strings.TrimSpace(columns[i])
		}
	}

	// Determine file type and create appropriate handler
	isParquet := strings.HasSuffix(strings.ToLower(coldDumpFile), ".parquet")
	
	if isParquet {
		return handleParquetFile(ctx, timeRange, columns)
	} else {
		return handleCSVFile(ctx, timeRange, columns)
	}
}

func handleParquetFile(ctx context.Context, timeRange *data.TimeRange, columns []string) error {
	fmt.Printf("Processing Parquet file: %s\n\n", coldDumpFile)

	// Create Parquet store
	config := data.ColdDataConfig{
		EnableParquet: true,
		BasePath:      filepath.Dir(coldDumpFile),
	}

	schema := data.ParquetSchema{
		Table: "ohlcv",
		Fields: []data.ParquetField{
			{Name: "ts", Type: "timestamp(ms)", Required: true, Primary: true},
			{Name: "symbol", Type: "string", Required: true},
			{Name: "venue", Type: "string", Required: true},
			{Name: "source_tier", Type: "string", Required: true},
		},
	}

	store, err := data.NewParquetStore(config, schema)
	if err != nil {
		return fmt.Errorf("failed to create Parquet store: %w", err)
	}

	// Show metadata if requested
	if coldDumpMetadataOnly || coldDumpValidate {
		metadata, err := store.GetParquetMetadata(ctx, coldDumpFile)
		if err != nil {
			return fmt.Errorf("failed to get metadata: %w", err)
		}
		displayParquetMetadata(metadata)
	}

	// Validate schema if requested
	if coldDumpValidate {
		err := store.ValidateParquetSchema(ctx, "ohlcv", schema)
		if err != nil {
			fmt.Printf("❌ Schema validation failed: %v\n\n", err)
		} else {
			fmt.Printf("✅ Schema validation passed\n\n")
		}
	}

	// Return early if only metadata was requested
	if coldDumpMetadataOnly {
		return nil
	}

	// Set up time range for query
	var tr data.TimeRange
	if timeRange != nil {
		tr = *timeRange
	} else {
		// Default to last 24 hours
		tr = data.TimeRange{
			From: time.Now().Add(-24 * time.Hour),
			To:   time.Now(),
		}
	}

	// Read data
	iterator, err := store.ReadParquet(ctx, "ohlcv", tr, columns)
	if err != nil {
		return fmt.Errorf("failed to read Parquet data: %w", err)
	}
	defer iterator.Close()

	// Display data
	return displayRows(iterator, columns)
}

func handleCSVFile(ctx context.Context, timeRange *data.TimeRange, columns []string) error {
	fmt.Printf("Processing CSV file: %s\n\n", coldDumpFile)

	// Create CSV reader
	reader := &data.CSVReader{}

	// Show file validation if requested
	if coldDumpValidate {
		err := reader.ValidateFile(coldDumpFile)
		if err != nil {
			fmt.Printf("❌ CSV validation failed: %v\n\n", err)
		} else {
			fmt.Printf("✅ CSV validation passed\n\n")
		}
	}

	// Return early if only validation was requested
	if coldDumpMetadataOnly {
		// For CSV, show basic file stats
		info, err := os.Stat(coldDumpFile)
		if err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		}
		
		fmt.Printf("CSV File Metadata:\n")
		fmt.Printf("  File path: %s\n", coldDumpFile)
		fmt.Printf("  File size: %d bytes\n", info.Size())
		fmt.Printf("  Modified: %s\n", info.ModTime().Format(time.RFC3339))
		return nil
	}

	// Load data with time filter if specified
	var envelopes []*data.Envelope
	var err error

	if timeRange != nil {
		envelopes, err = reader.LoadFileWithTimeFilter(coldDumpFile, "unknown", "unknown", timeRange.From, timeRange.To)
	} else {
		envelopes, err = reader.LoadFile(coldDumpFile, "unknown", "unknown")
	}

	if err != nil {
		return fmt.Errorf("failed to load CSV data: %w", err)
	}

	// Convert envelopes to rows and display
	return displayEnvelopes(envelopes, columns)
}

func parseTimeRange(fromStr, toStr string) (data.TimeRange, error) {
	var from, to time.Time
	var err error

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return data.TimeRange{}, fmt.Errorf("invalid from time '%s': %w", fromStr, err)
		}
	} else {
		from = time.Now().Add(-24 * time.Hour) // Default to 24 hours ago
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return data.TimeRange{}, fmt.Errorf("invalid to time '%s': %w", toStr, err)
		}
	} else {
		to = time.Now() // Default to now
	}

	if from.After(to) {
		return data.TimeRange{}, fmt.Errorf("from time must be before to time")
	}

	return data.TimeRange{From: from, To: to}, nil
}

func displayParquetMetadata(metadata *data.ParquetMetadata) {
	fmt.Printf("Parquet Metadata:\n")
	fmt.Printf("  File path: %s\n", metadata.FilePath)
	fmt.Printf("  Row count: %d\n", metadata.RowCount)
	fmt.Printf("  File size: %d bytes\n", metadata.FileSize)
	fmt.Printf("  Compression: %s\n", metadata.Compression)
	fmt.Printf("  Row groups: %d\n", metadata.RowGroupCount)
	fmt.Printf("  Created: %s\n", metadata.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Modified: %s\n", metadata.ModifiedAt.Format(time.RFC3339))

	if metadata.MinTimestamp != nil && metadata.MaxTimestamp != nil {
		fmt.Printf("  Time range: %s to %s\n", 
			metadata.MinTimestamp.Format(time.RFC3339),
			metadata.MaxTimestamp.Format(time.RFC3339))
	}

	if len(metadata.Schema) > 0 {
		fmt.Printf("  Schema:\n")
		for col, typ := range metadata.Schema {
			fmt.Printf("    %s: %s\n", col, typ)
		}
	}
	fmt.Println()
}

func displayRows(iterator data.RowIterator, columns []string) error {
	fmt.Printf("Data Dump:\n")

	// Print header
	if coldDumpFormat == "table" {
		printTableHeader(columns)
	}

	rowCount := 0
	skipped := 0

	for iterator.Next() {
		// Handle offset
		if skipped < coldDumpOffset {
			skipped++
			continue
		}

		// Handle limit
		if coldDumpLimit > 0 && rowCount >= coldDumpLimit {
			break
		}

		row := iterator.Value()
		if row == nil {
			continue
		}

		// Display row based on format
		switch coldDumpFormat {
		case "table":
			printTableRow(row, columns)
		case "csv":
			printCSVRow(row, columns, rowCount == 0)
		case "json":
			printJSONRow(row)
		default:
			return fmt.Errorf("unsupported output format: %s", coldDumpFormat)
		}

		rowCount++
	}

	fmt.Printf("\nDisplayed %d rows", rowCount)
	if coldDumpOffset > 0 {
		fmt.Printf(" (skipped %d)", coldDumpOffset)
	}
	if coldDumpLimit > 0 && rowCount == coldDumpLimit {
		fmt.Printf(" (limited to %d)", coldDumpLimit)
	}
	fmt.Println()

	return nil
}

func displayEnvelopes(envelopes []*data.Envelope, columns []string) error {
	fmt.Printf("Data Dump (%d rows):\n", len(envelopes))

	// Print header
	if coldDumpFormat == "table" {
		printTableHeader(columns)
	}

	startIdx := coldDumpOffset
	endIdx := len(envelopes)
	
	if coldDumpLimit > 0 && startIdx+coldDumpLimit < endIdx {
		endIdx = startIdx + coldDumpLimit
	}

	if startIdx >= len(envelopes) {
		fmt.Println("Offset exceeds available data")
		return nil
	}

	for i := startIdx; i < endIdx; i++ {
		envelope := envelopes[i]
		
		// Convert envelope to row for consistent display
		row, err := data.ConvertEnvelopeToRow(envelope)
		if err != nil {
			fmt.Printf("Warning: failed to convert envelope %d: %v\n", i, err)
			continue
		}

		// Display row
		switch coldDumpFormat {
		case "table":
			printTableRow(row, columns)
		case "csv":
			printCSVRow(row, columns, i == startIdx)
		case "json":
			printJSONRow(row)
		}
	}

	displayed := endIdx - startIdx
	fmt.Printf("\nDisplayed %d rows", displayed)
	if startIdx > 0 {
		fmt.Printf(" (skipped %d)", startIdx)
	}
	fmt.Println()

	return nil
}

func printTableHeader(columns []string) {
	if len(columns) == 0 {
		columns = []string{"ts", "symbol", "venue", "source_tier", "close", "volume"}
	}
	
	for i, col := range columns {
		if i > 0 {
			fmt.Printf(" | ")
		}
		fmt.Printf("%-15s", col)
	}
	fmt.Println()
	
	for i := range columns {
		if i > 0 {
			fmt.Printf("-+-")
		}
		fmt.Printf("%-15s", strings.Repeat("-", 15))
	}
	fmt.Println()
}

func printTableRow(row data.Row, columns []string) {
	if len(columns) == 0 {
		columns = []string{"ts", "symbol", "venue", "source_tier", "close", "volume"}
	}
	
	for i, col := range columns {
		if i > 0 {
			fmt.Printf(" | ")
		}
		
		value := row[col]
		var valueStr string
		
		if value == nil {
			valueStr = "NULL"
		} else if ts, ok := value.(time.Time); ok {
			valueStr = ts.Format("15:04:05")
		} else {
			valueStr = fmt.Sprintf("%v", value)
		}
		
		if len(valueStr) > 15 {
			valueStr = valueStr[:12] + "..."
		}
		
		fmt.Printf("%-15s", valueStr)
	}
	fmt.Println()
}

func printCSVRow(row data.Row, columns []string, header bool) {
	if len(columns) == 0 {
		columns = []string{"ts", "symbol", "venue", "source_tier", "close", "volume"}
	}
	
	if header {
		for i, col := range columns {
			if i > 0 {
				fmt.Printf(",")
			}
			fmt.Printf("%s", col)
		}
		fmt.Println()
	}
	
	for i, col := range columns {
		if i > 0 {
			fmt.Printf(",")
		}
		
		value := row[col]
		if value == nil {
			fmt.Printf("")
		} else if ts, ok := value.(time.Time); ok {
			fmt.Printf("%s", ts.Format(time.RFC3339))
		} else if str, ok := value.(string); ok {
			// Escape quotes in CSV
			escaped := strings.ReplaceAll(str, "\"", "\"\"")
			if strings.Contains(escaped, ",") || strings.Contains(escaped, "\"") {
				fmt.Printf("\"%s\"", escaped)
			} else {
				fmt.Printf("%s", escaped)
			}
		} else {
			fmt.Printf("%v", value)
		}
	}
	fmt.Println()
}

func printJSONRow(row data.Row) {
	// Simple JSON output (not using encoding/json for simplicity)
	fmt.Printf("{ ")
	first := true
	for key, value := range row {
		if !first {
			fmt.Printf(", ")
		}
		first = false
		
		if ts, ok := value.(time.Time); ok {
			fmt.Printf("\"%s\": \"%s\"", key, ts.Format(time.RFC3339))
		} else if str, ok := value.(string); ok {
			fmt.Printf("\"%s\": \"%s\"", key, str)
		} else if value == nil {
			fmt.Printf("\"%s\": null", key)
		} else {
			fmt.Printf("\"%s\": %v", key, value)
		}
	}
	fmt.Printf(" }\n")
}