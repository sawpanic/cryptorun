package data

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// TimeRange represents a time window for queries
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ParquetOptions holds Parquet-specific configuration
type ParquetOptions struct {
	Compression  string `json:"compression"`   // "gzip"|"lz4"|"snappy"|"zstd"|"uncompressed"
	RowGroupSize int    `json:"row_group_size"` // e.g., 128*1024 bytes
	EnableIndex  bool   `json:"enable_index"`   // Create column indices
	SortColumns  []string `json:"sort_columns"` // Columns to sort by
}

// DefaultParquetOptions returns sensible defaults
func DefaultParquetOptions() ParquetOptions {
	return ParquetOptions{
		Compression:  "snappy",
		RowGroupSize: 128 * 1024, // 128KB
		EnableIndex:  true,
		SortColumns:  []string{"ts", "symbol"},
	}
}

// ParquetSchema represents the schema configuration
type ParquetSchema struct {
	Table        string              `yaml:"table"`
	Fields       []ParquetField      `yaml:"fields"`
	Path         string              `yaml:"path"`
	Partitioning ParquetPartitioning `yaml:"partitioning"`
	RowGroupSize int                 `yaml:"row_group_size"`
	Compression  string              `yaml:"compression"`
}

// ParquetField represents a single field in the schema
type ParquetField struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Primary  bool   `yaml:"primary,omitempty"`
	Index    bool   `yaml:"index,omitempty"`
}

// ParquetPartitioning configuration
type ParquetPartitioning struct {
	Enabled        bool   `yaml:"enabled"`
	Scheme         string `yaml:"scheme"`          // "dt" for date partitioning
	RetentionDays  int    `yaml:"retention_days"`  // Data retention policy
}

// Row represents a single data row for Parquet operations
type Row map[string]interface{}

// RowIterator interface for iterating over rows
type RowIterator interface {
	Next() bool
	Value() Row
	Close() error
}

// ColdStore interface with Parquet support
type ColdStore interface {
	WriteParquet(ctx context.Context, table string, rows []Row, opts ParquetOptions) error
	ReadParquet(ctx context.Context, table string, tr TimeRange, columns []string) (RowIterator, error)
	ValidateParquetSchema(ctx context.Context, table string, schema ParquetSchema) error
	GetParquetMetadata(ctx context.Context, filePath string) (*ParquetMetadata, error)
}

// ParquetMetadata contains file metadata
type ParquetMetadata struct {
	FilePath      string            `json:"file_path"`
	Schema        map[string]string `json:"schema"`          // column_name -> type
	RowCount      int64             `json:"row_count"`
	FileSize      int64             `json:"file_size"`
	Compression   string            `json:"compression"`
	RowGroupCount int               `json:"row_group_count"`
	CreatedAt     time.Time         `json:"created_at"`
	ModifiedAt    time.Time         `json:"modified_at"`
	MinTimestamp  *time.Time        `json:"min_timestamp,omitempty"`
	MaxTimestamp  *time.Time        `json:"max_timestamp,omitempty"`
}

// ParquetStore implements ColdStore interface with Parquet support
type ParquetStore struct {
	config    ColdDataConfig
	schema    ParquetSchema
	basePath  string
}

// NewParquetStore creates a new Parquet-backed cold store
func NewParquetStore(config ColdDataConfig, schema ParquetSchema) (*ParquetStore, error) {
	return &ParquetStore{
		config:   config,
		schema:   schema,
		basePath: config.BasePath,
	}, nil
}

// WriteParquet writes rows to Parquet file with partitioning
func (p *ParquetStore) WriteParquet(ctx context.Context, table string, rows []Row, opts ParquetOptions) error {
	if len(rows) == 0 {
		return fmt.Errorf("no rows to write")
	}

	// Validate schema compatibility
	if err := p.validateRowsAgainstSchema(rows); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Determine partition path
	partitionPath, err := p.getPartitionPath(table, rows[0])
	if err != nil {
		return fmt.Errorf("failed to determine partition path: %w", err)
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("part-%s-%d.parquet", timestamp, len(rows))
	filePath := filepath.Join(partitionPath, filename)

	// For now, this is a mock implementation
	// In production, this would use a real Parquet library like:
	// - github.com/apache/arrow/go/v12/parquet
	// - github.com/xitongsys/parquet-go
	return p.writeParquetFile(filePath, rows, opts)
}

// ReadParquet reads data from Parquet files within time range
func (p *ParquetStore) ReadParquet(ctx context.Context, table string, tr TimeRange, columns []string) (RowIterator, error) {
	// Find relevant partition files
	files, err := p.findPartitionFiles(table, tr)
	if err != nil {
		return nil, fmt.Errorf("failed to find partition files: %w", err)
	}

	if len(files) == 0 {
		return p.emptyIterator(), nil
	}

	// Create iterator that reads from multiple files
	return p.createMultiFileIterator(files, tr, columns)
}

// ValidateParquetSchema validates schema against configuration
func (p *ParquetStore) ValidateParquetSchema(ctx context.Context, table string, schema ParquetSchema) error {
	// Check required fields are present
	requiredFields := []string{"ts", "symbol", "venue", "source_tier"}
	fieldMap := make(map[string]bool)
	
	for _, field := range schema.Fields {
		fieldMap[field.Name] = true
	}
	
	for _, required := range requiredFields {
		if !fieldMap[required] {
			return fmt.Errorf("required field '%s' missing from schema", required)
		}
	}
	
	// Validate primary key exists
	hasPrimary := false
	for _, field := range schema.Fields {
		if field.Primary {
			hasPrimary = true
			break
		}
	}
	
	if !hasPrimary {
		return fmt.Errorf("schema must have at least one primary key field")
	}
	
	return nil
}

// GetParquetMetadata returns metadata for a Parquet file
func (p *ParquetStore) GetParquetMetadata(ctx context.Context, filePath string) (*ParquetMetadata, error) {
	// Mock implementation - would use real Parquet reader in production
	return &ParquetMetadata{
		FilePath:      filePath,
		Schema:        map[string]string{"ts": "timestamp", "symbol": "string", "close": "double"},
		RowCount:      1000,
		FileSize:      4096,
		Compression:   "snappy",
		RowGroupCount: 1,
		CreatedAt:     time.Now(),
		ModifiedAt:    time.Now(),
	}, nil
}

// validateRowsAgainstSchema validates data rows against the schema
func (p *ParquetStore) validateRowsAgainstSchema(rows []Row) error {
	if len(rows) == 0 {
		return nil
	}

	// Check first row for required fields
	firstRow := rows[0]
	for _, field := range p.schema.Fields {
		if field.Required {
			if _, exists := firstRow[field.Name]; !exists {
				return fmt.Errorf("required field '%s' missing from row data", field.Name)
			}
		}
	}

	// Validate timestamp field
	if tsValue, exists := firstRow["ts"]; exists {
		switch tsValue.(type) {
		case time.Time, int64, float64:
			// Valid timestamp formats
		default:
			return fmt.Errorf("timestamp field 'ts' has invalid type: %T", tsValue)
		}
	}

	return nil
}

// getPartitionPath determines the partition directory for a row
func (p *ParquetStore) getPartitionPath(table string, row Row) (string, error) {
	basePath := filepath.Join(p.basePath, table)
	
	if !p.schema.Partitioning.Enabled {
		return basePath, nil
	}

	// Extract timestamp for partitioning
	tsValue, exists := row["ts"]
	if !exists {
		return "", fmt.Errorf("timestamp field 'ts' required for partitioning")
	}

	var timestamp time.Time
	switch v := tsValue.(type) {
	case time.Time:
		timestamp = v
	case int64:
		timestamp = time.Unix(v/1000, (v%1000)*1000000) // Assume milliseconds
	case float64:
		timestamp = time.Unix(int64(v)/1000, int64((v-float64(int64(v)))*1000000000))
	default:
		return "", fmt.Errorf("unsupported timestamp type: %T", tsValue)
	}

	// Create date partition path
	if p.schema.Partitioning.Scheme == "dt" {
		dateStr := timestamp.Format("2006-01-02")
		return filepath.Join(basePath, fmt.Sprintf("dt=%s", dateStr)), nil
	}

	return basePath, nil
}

// findPartitionFiles finds Parquet files that might contain data in the time range
func (p *ParquetStore) findPartitionFiles(table string, tr TimeRange) ([]string, error) {
	// Mock implementation - would scan partition directories in production
	basePath := filepath.Join(p.basePath, table)
	
	// For now, return a mock file path
	mockFile := filepath.Join(basePath, "dt=2025-01-01", "part-mock-1000.parquet")
	return []string{mockFile}, nil
}

// writeParquetFile writes data to a Parquet file (mock implementation)
func (p *ParquetStore) writeParquetFile(filePath string, rows []Row, opts ParquetOptions) error {
	// Mock implementation
	// In production, this would:
	// 1. Create Parquet writer with schema
	// 2. Configure compression and row group size
	// 3. Write rows in batches
	// 4. Close and finalize file
	
	fmt.Printf("[MOCK] Writing %d rows to Parquet file: %s (compression: %s)\n", 
		len(rows), filePath, opts.Compression)
	
	return nil
}

// emptyIterator returns an empty row iterator
func (p *ParquetStore) emptyIterator() RowIterator {
	return &EmptyRowIterator{}
}

// createMultiFileIterator creates iterator that reads from multiple Parquet files
func (p *ParquetStore) createMultiFileIterator(files []string, tr TimeRange, columns []string) (RowIterator, error) {
	return &MockParquetIterator{
		files:     files,
		timeRange: tr,
		columns:   columns,
		position:  0,
	}, nil
}

// EmptyRowIterator implements an empty iterator
type EmptyRowIterator struct{}

func (e *EmptyRowIterator) Next() bool { return false }
func (e *EmptyRowIterator) Value() Row { return nil }
func (e *EmptyRowIterator) Close() error { return nil }

// MockParquetIterator provides mock data for testing
type MockParquetIterator struct {
	files     []string
	timeRange TimeRange
	columns   []string
	position  int
	maxRows   int
}

func (m *MockParquetIterator) Next() bool {
	if m.maxRows == 0 {
		m.maxRows = 10 // Return 10 mock rows
	}
	m.position++
	return m.position <= m.maxRows
}

func (m *MockParquetIterator) Value() Row {
	if m.position <= 0 || m.position > m.maxRows {
		return nil
	}

	// Generate mock row data within the requested time range
	duration := m.timeRange.To.Sub(m.timeRange.From)
	increment := duration / time.Duration(m.maxRows)
	timestamp := m.timeRange.From.Add(time.Duration(m.position-1) * increment)
	
	row := Row{
		"ts":          timestamp,
		"symbol":      "BTC-USD",
		"venue":       "kraken",
		"open":        50000.0 + float64(m.position*10),
		"high":        50100.0 + float64(m.position*10),
		"low":         49900.0 + float64(m.position*10),
		"close":       50050.0 + float64(m.position*10),
		"volume":      1000.0 + float64(m.position*5),
		"source_tier": "cold",
	}

	// Filter columns if specified
	if len(m.columns) > 0 {
		filteredRow := make(Row)
		for _, col := range m.columns {
			if val, exists := row[col]; exists {
				filteredRow[col] = val
			}
		}
		return filteredRow
	}

	return row
}

func (m *MockParquetIterator) Close() error {
	return nil
}

// ConvertEnvelopeToRow converts data.Envelope to Parquet Row format
func ConvertEnvelopeToRow(envelope *Envelope) (Row, error) {
	row := Row{
		"ts":          envelope.Timestamp,
		"symbol":      envelope.Symbol,
		"venue":       envelope.Venue,
		"source_tier": string(envelope.SourceTier),
	}

	// Extract price data
	if envelope.PriceData != nil {
		if priceMap, ok := envelope.PriceData.(map[string]interface{}); ok {
			for key, value := range priceMap {
				row[key] = value
			}
		}
	}

	// Extract volume data
	if envelope.VolumeData != nil {
		if volumeMap, ok := envelope.VolumeData.(map[string]interface{}); ok {
			for key, value := range volumeMap {
				row[key] = value
			}
		}
	}

	// Extract order book data
	if envelope.OrderBook != nil {
		if obMap, ok := envelope.OrderBook.(map[string]interface{}); ok {
			// Map order book fields to schema
			if bidPrice, exists := obMap["best_bid_price"]; exists {
				row["bid_price"] = bidPrice
			}
			if askPrice, exists := obMap["best_ask_price"]; exists {
				row["ask_price"] = askPrice
			}
			if bidQty, exists := obMap["best_bid_qty"]; exists {
				row["bid_qty"] = bidQty
			}
			if askQty, exists := obMap["best_ask_qty"]; exists {
				row["ask_qty"] = askQty
			}
			if spreadBps, exists := obMap["spread_bps"]; exists {
				row["spread_bps"] = spreadBps
			}
		}
	}

	// Add confidence score
	row["confidence"] = envelope.Provenance.ConfidenceScore

	return row, nil
}

// ConvertRowToEnvelope converts Parquet Row to data.Envelope
func ConvertRowToEnvelope(row Row) (*Envelope, error) {
	// Extract required fields
	symbol, ok := row["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol field missing or invalid type")
	}

	venue, ok := row["venue"].(string)
	if !ok {
		return nil, fmt.Errorf("venue field missing or invalid type")
	}

	sourceTier, ok := row["source_tier"].(string)
	if !ok {
		sourceTier = string(TierCold) // Default to cold tier
	}

	// Extract timestamp
	var timestamp time.Time
	if ts, exists := row["ts"]; exists {
		switch v := ts.(type) {
		case time.Time:
			timestamp = v
		case int64:
			timestamp = time.Unix(v/1000, (v%1000)*1000000)
		default:
			return nil, fmt.Errorf("invalid timestamp type: %T", ts)
		}
	} else {
		return nil, fmt.Errorf("timestamp field 'ts' is required")
	}

	// Create envelope
	envelope := NewEnvelope(venue, symbol, SourceTier(sourceTier))
	envelope.Timestamp = timestamp

	// Build price data
	priceData := make(map[string]interface{})
	priceFields := []string{"open", "high", "low", "close"}
	for _, field := range priceFields {
		if value, exists := row[field]; exists {
			priceData[field] = value
		}
	}
	if len(priceData) > 0 {
		envelope.PriceData = priceData
	}

	// Build volume data
	volumeData := make(map[string]interface{})
	if volume, exists := row["volume"]; exists {
		volumeData["volume"] = volume
	}
	if len(volumeData) > 0 {
		envelope.VolumeData = volumeData
	}

	// Build order book data
	orderBook := make(map[string]interface{})
	if bidPrice, exists := row["bid_price"]; exists {
		orderBook["best_bid_price"] = bidPrice
	}
	if askPrice, exists := row["ask_price"]; exists {
		orderBook["best_ask_price"] = askPrice
	}
	if bidQty, exists := row["bid_qty"]; exists {
		orderBook["best_bid_qty"] = bidQty
	}
	if askQty, exists := row["ask_qty"]; exists {
		orderBook["best_ask_qty"] = askQty
	}
	if spreadBps, exists := row["spread_bps"]; exists {
		orderBook["spread_bps"] = spreadBps
	}
	if len(orderBook) > 0 {
		envelope.OrderBook = orderBook
	}

	// Set confidence score
	if confidence, exists := row["confidence"]; exists {
		if conf, ok := confidence.(float64); ok {
			envelope.Provenance.ConfidenceScore = conf
		}
	}

	// Set provenance
	envelope.Provenance.OriginalSource = fmt.Sprintf("%s_parquet", venue)
	envelope.Provenance.RetrievedAt = time.Now()
	envelope.CalculateFreshness()

	return envelope, nil
}