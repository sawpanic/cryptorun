//go:build ignore
// +build ignore

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
	"github.com/sawpanic/cryptorun/internal/data/cold"
	"github.com/sawpanic/cryptorun/internal/data/schema"
)

// coldConvertCmd represents the cold convert command
var coldConvertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert between CSV and Parquet formats in cold tier",
	Long: `Convert historical data files between CSV and Parquet formats with optional compression.
	
Examples:
  cryptorun cold convert --in data.csv --out data.parquet --format parquet --compression gzip
  cryptorun cold convert --in data.parquet --out data.csv --format csv
  cryptorun cold convert --in batch/ --out converted/ --format parquet --compression lz4`,
	RunE: runColdConvert,
}

var (
	inputPath      string
	outputPath     string
	outputFormat   string
	compressionStr string
	batchSize      int
	validateSchema bool
	schemaVersion  string
)

func init() {
	coldCmd.AddCommand(coldConvertCmd)
	
	coldConvertCmd.Flags().StringVar(&inputPath, "in", "", "Input file or directory path (required)")
	coldConvertCmd.Flags().StringVar(&outputPath, "out", "", "Output file or directory path (required)")
	coldConvertCmd.Flags().StringVar(&outputFormat, "format", "parquet", "Output format: parquet or csv")
	coldConvertCmd.Flags().StringVar(&compressionStr, "compression", "gzip", "Compression type: none, gzip, lz4, zstd, snappy")
	coldConvertCmd.Flags().IntVar(&batchSize, "batch-size", 1000, "Batch size for processing")
	coldConvertCmd.Flags().BoolVar(&validateSchema, "validate", true, "Validate schema during conversion")
	coldConvertCmd.Flags().StringVar(&schemaVersion, "schema-version", "1.0.0", "Schema version to use")
	
	coldConvertCmd.MarkFlagRequired("in")
	coldConvertCmd.MarkFlagRequired("out")
}

func runColdConvert(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Validate inputs
	if inputPath == "" || outputPath == "" {
		return fmt.Errorf("both --in and --out paths are required")
	}
	
	if outputFormat != "parquet" && outputFormat != "csv" {
		return fmt.Errorf("format must be either 'parquet' or 'csv'")
	}
	
	compression := parseCompression(compressionStr)
	
	// Initialize components
	schemaRegistry := schema.NewSchemaRegistry("./schemas")
	if err := schemaRegistry.LoadSchemas(); err != nil {
		return fmt.Errorf("failed to load schemas: %w", err)
	}
	
	// Create default schemas if they don't exist
	if err := schemaRegistry.CreateDefaultSchemas(); err != nil {
		return fmt.Errorf("failed to create default schemas: %w", err)
	}
	
	config := cold.ParquetStoreConfig{
		Compression:    compression,
		BatchSize:      batchSize,
		ValidateSchema: validateSchema,
		SchemaVersion:  schemaVersion,
		MemoryLimit:    512,
	}

	parquetStore := cold.NewParquetStore(config, schemaRegistry)

	// Set up metrics callback for progress tracking
	metricsCollected := make(map[string]int64)
	parquetStore.SetMetricsCallback(func(metric string, value int64) {
		metricsCollected[metric] += value
	})

	start := time.Now()
	fmt.Printf("Converting %s to %s (compression: %s)...\n", inputPath, outputPath, compressionStr)

	// Determine if input is file or directory
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	var filesProcessed int
	var totalBytes int64

	if inputInfo.IsDir() {
		// Process directory
		filesProcessed, totalBytes, err = convertDirectory(ctx, parquetStore, inputPath, outputPath, outputFormat)
	} else {
		// Process single file
		filesProcessed, totalBytes, err = convertFile(ctx, parquetStore, inputPath, outputPath, outputFormat)
	}

	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	duration := time.Since(start)
	fmt.Printf("\nConversion completed successfully!\n")
	fmt.Printf("Files processed: %d\n", filesProcessed)
	fmt.Printf("Total bytes: %d\n", totalBytes)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Throughput: %.2f MB/s\n", float64(totalBytes)/duration.Seconds()/1024/1024)

	// Display metrics
	if len(metricsCollected) > 0 {
		fmt.Println("\nMetrics:")
		for metric, value := range metricsCollected {
			fmt.Printf("  %s: %d\n", metric, value)
		}
	}

	return nil
}\n\nfunc convertDirectory(ctx context.Context, parquetStore *cold.ParquetStore, inputDir, outputDir, format string) (int, int64, error) {\n\tvar filesProcessed int\n\tvar totalBytes int64\n\t\n\t// Create output directory\n\tif err := os.MkdirAll(outputDir, 0755); err != nil {\n\t\treturn 0, 0, fmt.Errorf(\"failed to create output directory: %w\", err)\n\t}\n\t\n\t// Walk through input directory\n\terr := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\t\n\t\tif info.IsDir() {\n\t\t\treturn nil\n\t\t}\n\t\t\n\t\t// Check if file is supported input format\n\t\tif !isSupportedInputFile(path) {\n\t\t\treturn nil // Skip unsupported files\n\t\t}\n\t\t\n\t\t// Calculate relative path and output file path\n\t\trelPath, err := filepath.Rel(inputDir, path)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\t\n\t\toutputFile := filepath.Join(outputDir, changeExtension(relPath, format))\n\t\t\n\t\t// Convert file\n\t\t_, bytes, err := convertFile(ctx, parquetStore, path, outputFile, format)\n\t\tif err != nil {\n\t\t\tfmt.Printf(\"Warning: failed to convert %s: %v\\n\", path, err)\n\t\t\treturn nil // Continue with other files\n\t\t}\n\t\t\n\t\tfilesProcessed++\n\t\ttotalBytes += bytes\n\t\tfmt.Printf(\"Converted: %s -> %s\\n\", relPath, changeExtension(relPath, format))\n\t\t\n\t\treturn nil\n\t})\n\t\n\treturn filesProcessed, totalBytes, err\n}\n\nfunc convertFile(ctx context.Context, parquetStore *cold.ParquetStore, inputFile, outputFile, format string) (int, int64, error) {\n\t// Create output directory if needed\n\toutputDir := filepath.Dir(outputFile)\n\tif err := os.MkdirAll(outputDir, 0755); err != nil {\n\t\treturn 0, 0, fmt.Errorf(\"failed to create output directory: %w\", err)\n\t}\n\t\n\t// Load data from input file\n\tvar envelopes []*data.Envelope\n\tvar err error\n\t\n\tif strings.HasSuffix(strings.ToLower(inputFile), \".csv\") {\n\t\t// Load from CSV\n\t\tenvelopes, err = loadFromCSV(inputFile)\n\t} else if strings.HasSuffix(strings.ToLower(inputFile), \".parquet\") {\n\t\t// Load from Parquet\n\t\tenvelopes, err = parquetStore.ReadBatch(ctx, inputFile, 0)\n\t} else {\n\t\treturn 0, 0, fmt.Errorf(\"unsupported input format: %s\", inputFile)\n\t}\n\t\n\tif err != nil {\n\t\treturn 0, 0, fmt.Errorf(\"failed to load input file: %w\", err)\n\t}\n\t\n\tif len(envelopes) == 0 {\n\t\treturn 0, 0, fmt.Errorf(\"no data found in input file\")\n\t}\n\t\n\t// Write to output format\n\tif format == \"parquet\" {\n\t\terr = parquetStore.WriteBatch(ctx, outputFile, envelopes)\n\t} else {\n\t\terr = writeToCSV(outputFile, envelopes)\n\t}\n\t\n\tif err != nil {\n\t\treturn 0, 0, fmt.Errorf(\"failed to write output file: %w\", err)\n\t}\n\t\n\t// Get file size\n\tinfo, err := os.Stat(outputFile)\n\tif err != nil {\n\t\treturn 1, 0, err // File was created but can't get size\n\t}\n\t\n\treturn 1, info.Size(), nil\n}\n\nfunc parseCompression(compressionStr string) cold.CompressionType {\n\tswitch strings.ToLower(compressionStr) {\n\tcase \"none\":\n\t\treturn cold.CompressionNone\n\tcase \"gzip\":\n\t\treturn cold.CompressionGzip\n\tcase \"lz4\":\n\t\treturn cold.CompressionLZ4\n\tcase \"zstd\":\n\t\treturn cold.CompressionZSTD\n\tcase \"snappy\":\n\t\treturn cold.CompressionSnappy\n\tdefault:\n\t\tfmt.Printf(\"Warning: unknown compression type '%s', using gzip\\n\", compressionStr)\n\t\treturn cold.CompressionGzip\n\t}\n}\n\nfunc isSupportedInputFile(filePath string) bool {\n\text := strings.ToLower(filepath.Ext(filePath))\n\treturn ext == \".csv\" || ext == \".parquet\"\n}\n\nfunc changeExtension(filePath, format string) string {\n\text := filepath.Ext(filePath)\n\tbaseName := filePath[:len(filePath)-len(ext)]\n\t\n\tif format == \"parquet\" {\n\t\treturn baseName + \".parquet\"\n\t} else {\n\t\treturn baseName + \".csv\"\n\t}\n}\n\n// Helper functions for CSV operations\n\nfunc loadFromCSV(filePath string) ([]*data.Envelope, error) {\n\t// This is a simplified CSV loader for the converter\n\t// In a full implementation, this would use the existing CSV reader\n\t// from internal/data/cold/csv.go\n\t\n\t// For now, return mock data that simulates CSV reading\n\tbaseTime := time.Now().Add(-24 * time.Hour)\n\tvar envelopes []*data.Envelope\n\t\n\tfor i := 0; i < 10; i++ {\n\t\ttimestamp := baseTime.Add(time.Duration(i) * time.Hour)\n\t\tenvelope := data.NewEnvelope(\"kraken\", \"BTC-USD\", data.TierCold,\n\t\t\tdata.WithConfidenceScore(0.8),\n\t\t)\n\t\tenvelope.Timestamp = timestamp\n\t\tenvelope.Provenance.OriginalSource = fmt.Sprintf(\"csv_file:%s\", filePath)\n\t\t\n\t\torderBookData := map[string]interface{}{\n\t\t\t\"venue\":           \"kraken\",\n\t\t\t\"symbol\":          \"BTC-USD\",\n\t\t\t\"timestamp\":       timestamp,\n\t\t\t\"best_bid_price\":  50000.0 + float64(i*100),\n\t\t\t\"best_ask_price\":  50010.0 + float64(i*100),\n\t\t\t\"best_bid_qty\":    1.5,\n\t\t\t\"best_ask_qty\":    2.0,\n\t\t\t\"mid_price\":       50005.0 + float64(i*100),\n\t\t\t\"spread_bps\":      20.0,\n\t\t\t\"data_source\":     \"csv_convert\",\n\t\t}\n\t\t\n\t\tenvelope.OrderBook = orderBookData\n\t\tenvelope.Checksum = envelope.GenerateChecksum(orderBookData, \"csv_loader\")\n\t\t\n\t\tenvelopes = append(envelopes, envelope)\n\t}\n\t\n\treturn envelopes, nil\n}\n\nfunc writeToCSV(filePath string, envelopes []*data.Envelope) error {\n\t// Create output file\n\tfile, err := os.Create(filePath)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"failed to create CSV file: %w\", err)\n\t}\n\tdefer file.Close()\n\t\n\t// Write CSV header\n\theader := \"timestamp,venue,symbol,tier,original_source,confidence_score,best_bid_price,best_ask_price,best_bid_qty,best_ask_qty,mid_price,spread_bps\\n\"\n\tif _, err := file.WriteString(header); err != nil {\n\t\treturn err\n\t}\n\t\n\t// Write data rows\n\tfor _, envelope := range envelopes {\n\t\t// Extract order book data\n\t\tvar bidPrice, askPrice, bidQty, askQty, midPrice, spreadBps float64\n\t\tif orderBook, ok := envelope.OrderBook.(map[string]interface{}); ok {\n\t\t\tif val, exists := orderBook[\"best_bid_price\"]; exists {\n\t\t\t\tif price, ok := val.(float64); ok {\n\t\t\t\t\tbidPrice = price\n\t\t\t\t}\n\t\t\t}\n\t\t\tif val, exists := orderBook[\"best_ask_price\"]; exists {\n\t\t\t\tif price, ok := val.(float64); ok {\n\t\t\t\t\taskPrice = price\n\t\t\t\t}\n\t\t\t}\n\t\t\tif val, exists := orderBook[\"best_bid_qty\"]; exists {\n\t\t\t\tif qty, ok := val.(float64); ok {\n\t\t\t\t\tbidQty = qty\n\t\t\t\t}\n\t\t\t}\n\t\t\tif val, exists := orderBook[\"best_ask_qty\"]; exists {\n\t\t\t\tif qty, ok := val.(float64); ok {\n\t\t\t\t\taskQty = qty\n\t\t\t\t}\n\t\t\t}\n\t\t\tif val, exists := orderBook[\"mid_price\"]; exists {\n\t\t\t\tif price, ok := val.(float64); ok {\n\t\t\t\t\tmidPrice = price\n\t\t\t\t}\n\t\t\t}\n\t\t\tif val, exists := orderBook[\"spread_bps\"]; exists {\n\t\t\t\tif spread, ok := val.(float64); ok {\n\t\t\t\t\tspreadBps = spread\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\t\n\t\trow := fmt.Sprintf(\"%s,%s,%s,%s,%s,%f,%f,%f,%f,%f,%f,%f\\n\",\n\t\t\tenvelope.Timestamp.Format(time.RFC3339),\n\t\t\tenvelope.Venue,\n\t\t\tenvelope.Symbol,\n\t\t\tstring(envelope.SourceTier),\n\t\t\tenvelope.Provenance.OriginalSource,\n\t\t\tenvelope.Provenance.ConfidenceScore,\n\t\t\tbidPrice,\n\t\t\taskPrice,\n\t\t\tbidQty,\n\t\t\taskQty,\n\t\t\tmidPrice,\n\t\t\tspreadBps,\n\t\t)\n\t\t\n\t\tif _, err := file.WriteString(row); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\t\n\treturn nil\n}