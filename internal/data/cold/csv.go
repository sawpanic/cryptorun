package cold

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"cryptorun/internal/data"
)

// CSVReader handles reading historical data from CSV files
type CSVReader struct {
	dateFormats []string // Support multiple date formats
}

// NewCSVReader creates a new CSV reader
func NewCSVReader() *CSVReader {
	return &CSVReader{
		dateFormats: []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05.000",
		},
	}
}

// LoadFile reads a CSV file and converts to envelopes
func (r *CSVReader) LoadFile(filePath, venue, symbol string) ([]*data.Envelope, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)

	// Read header to understand format
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	columnMap := r.mapColumns(header)
	var envelopes []*data.Envelope

	// Read data rows
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		envelope, err := r.parseRecord(record, columnMap, venue, symbol, filePath)
		if err != nil {
			// Log error but continue processing
			continue
		}

		if envelope != nil {
			envelopes = append(envelopes, envelope)
		}
	}

	return envelopes, nil
}

// ValidateFile checks if CSV file format is supported
func (r *CSVReader) ValidateFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)

	// Check header
	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	columnMap := r.mapColumns(header)

	// Validate required columns exist
	if _, exists := columnMap["timestamp"]; !exists {
		return fmt.Errorf("CSV missing required 'timestamp' column")
	}

	return nil
}

// mapColumns creates a mapping from column names to indices
func (r *CSVReader) mapColumns(header []string) map[string]int {
	columnMap := make(map[string]int)

	for i, column := range header {
		// Normalize column names
		normalized := r.normalizeColumnName(column)
		columnMap[normalized] = i
	}

	return columnMap
}

// normalizeColumnName converts various column name formats to standard
func (r *CSVReader) normalizeColumnName(column string) string {
	// Convert to lowercase and handle common variations
	switch column {
	case "ts", "time", "datetime", "timestamp_utc":
		return "timestamp"
	case "pair", "instrument", "symbol_id":
		return "symbol"
	case "exchange", "source", "provider":
		return "venue"
	case "bid", "best_bid", "bid_price":
		return "bid_price"
	case "ask", "best_ask", "ask_price":
		return "ask_price"
	case "bid_size", "bid_qty", "bid_volume":
		return "bid_qty"
	case "ask_size", "ask_qty", "ask_volume":
		return "ask_qty"
	case "mid", "mid_price":
		return "mid_price"
	case "spread", "spread_bps":
		return "spread_bps"
	default:
		return column
	}
}

// parseRecord converts CSV record to envelope
func (r *CSVReader) parseRecord(record []string, columnMap map[string]int, venue, symbol, filePath string) (*data.Envelope, error) {
	if len(record) == 0 {
		return nil, fmt.Errorf("empty record")
	}

	// Parse timestamp
	timestampIdx, exists := columnMap["timestamp"]
	if !exists || timestampIdx >= len(record) {
		return nil, fmt.Errorf("timestamp column not found or out of range")
	}

	timestamp, err := r.parseTimestamp(record[timestampIdx])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Create envelope
	envelope := data.NewEnvelope(venue, symbol, data.TierCold,
		data.WithConfidenceScore(0.7), // Historical data confidence
	)
	envelope.Timestamp = timestamp
	envelope.Provenance.OriginalSource = fmt.Sprintf("csv:%s", filePath)

	// Parse order book data
	orderBookData := r.buildOrderBookData(record, columnMap, venue, symbol, timestamp)
	envelope.OrderBook = orderBookData
	envelope.Checksum = envelope.GenerateChecksum(orderBookData, "csv_record")

	return envelope, nil
}

// parseTimestamp handles multiple timestamp formats
func (r *CSVReader) parseTimestamp(timestampStr string) (time.Time, error) {
	for _, format := range r.dateFormats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t, nil
		}
	}

	// Try parsing as Unix timestamp
	if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		// Check if it's seconds or milliseconds
		if unixTime > 1e12 { // Milliseconds
			return time.Unix(0, unixTime*1e6), nil
		} else { // Seconds
			return time.Unix(unixTime, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", timestampStr)
}

// buildOrderBookData constructs order book data from CSV record
func (r *CSVReader) buildOrderBookData(record []string, columnMap map[string]int, venue, symbol string, timestamp time.Time) map[string]interface{} {
	data := map[string]interface{}{
		"venue":     venue,
		"symbol":    symbol,
		"timestamp": timestamp,
	}

	// Add available fields
	if idx, exists := columnMap["bid_price"]; exists && idx < len(record) {
		if price, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["best_bid_price"] = price
		}
	}

	if idx, exists := columnMap["ask_price"]; exists && idx < len(record) {
		if price, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["best_ask_price"] = price
		}
	}

	if idx, exists := columnMap["bid_qty"]; exists && idx < len(record) {
		if qty, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["best_bid_qty"] = qty
		}
	}

	if idx, exists := columnMap["ask_qty"]; exists && idx < len(record) {
		if qty, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["best_ask_qty"] = qty
		}
	}

	if idx, exists := columnMap["mid_price"]; exists && idx < len(record) {
		if price, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["mid_price"] = price
		}
	}

	if idx, exists := columnMap["spread_bps"]; exists && idx < len(record) {
		if spread, err := strconv.ParseFloat(record[idx], 64); err == nil {
			data["spread_bps"] = spread
		}
	}

	return data
}
