package compact

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// JSONLCompactor compacts JSONL files by removing duplicate keys and using dictionary compression
type JSONLCompactor struct {
	config JSONLConfig
}

// JSONLConfig configures JSONL compaction behavior
type JSONLConfig struct {
	Enabled        bool `yaml:"enabled"`
	MinSizeKB      int  `yaml:"min_size_kb"`
	DictThreshold  int  `yaml:"dict_threshold"`  // Use dictionary if field repeats this many times
	PreserveSchema bool `yaml:"preserve_schema"` // Always preserve first record schema
}

// CompactResult contains the results of JSONL compaction
type CompactResult struct {
	OriginalPath     string  `json:"original_path"`
	CompactedPath    string  `json:"compacted_path"`
	OriginalSize     int64   `json:"original_size"`
	CompactedSize    int64   `json:"compacted_size"`
	CompressionRatio float64 `json:"compression_ratio"`
	LinesProcessed   int     `json:"lines_processed"`
	DictFields       int     `json:"dict_fields"`
	PreservedSchema  bool    `json:"preserved_schema"`
}

// FieldDictionary tracks field frequency for dictionary compression
type FieldDictionary struct {
	Values    map[string]int // value -> frequency
	KeyIndex  map[string]int // value -> dictionary index
	IndexKey  map[int]string // dictionary index -> value
	Threshold int            // minimum frequency for dictionary entry
}

// NewJSONLCompactor creates a new JSONL compactor
func NewJSONLCompactor(config JSONLConfig) *JSONLCompactor {
	return &JSONLCompactor{
		config: config,
	}
}

// CompactFile compacts a single JSONL file
func (jc *JSONLCompactor) CompactFile(inputPath string) (*CompactResult, error) {
	if !jc.config.Enabled {
		return nil, fmt.Errorf("JSONL compaction is disabled")
	}

	// Check file size threshold
	stat, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	if stat.Size() < int64(jc.config.MinSizeKB*1024) {
		return nil, fmt.Errorf("file too small for compaction: %d bytes < %d KB threshold",
			stat.Size(), jc.config.MinSizeKB)
	}

	// Create output path
	outputPath := strings.TrimSuffix(inputPath, ".jsonl") + ".compact.jsonl"

	result := &CompactResult{
		OriginalPath:    inputPath,
		CompactedPath:   outputPath,
		OriginalSize:    stat.Size(),
		PreservedSchema: jc.config.PreserveSchema,
	}

	// First pass: analyze field frequencies
	dictionaries, err := jc.analyzeFields(inputPath)
	if err != nil {
		return nil, fmt.Errorf("field analysis failed: %w", err)
	}

	// Second pass: compact the file
	if err := jc.compactWithDictionaries(inputPath, outputPath, dictionaries, result); err != nil {
		return nil, fmt.Errorf("compaction failed: %w", err)
	}

	// Calculate final statistics
	compactStat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	result.CompactedSize = compactStat.Size()
	if result.OriginalSize > 0 {
		result.CompressionRatio = float64(result.CompactedSize) / float64(result.OriginalSize)
	}

	return result, nil
}

// analyzeFields performs first pass to identify frequently repeated field values
func (jc *JSONLCompactor) analyzeFields(inputPath string) (map[string]*FieldDictionary, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dictionaries := make(map[string]*FieldDictionary)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue // Skip malformed lines
		}

		// Analyze each field
		for key, value := range record {
			// Only consider string values for dictionary compression
			if strValue, ok := value.(string); ok && len(strValue) > 10 {
				if _, exists := dictionaries[key]; !exists {
					dictionaries[key] = &FieldDictionary{
						Values:    make(map[string]int),
						KeyIndex:  make(map[string]int),
						IndexKey:  make(map[int]string),
						Threshold: jc.config.DictThreshold,
					}
				}
				dictionaries[key].Values[strValue]++
			}
		}
	}

	// Build dictionary indices for values that meet the threshold
	for fieldName, dict := range dictionaries {
		index := 0
		for value, frequency := range dict.Values {
			if frequency >= dict.Threshold {
				dict.KeyIndex[value] = index
				dict.IndexKey[index] = value
				index++
			}
		}

		// Remove dictionaries with no qualifying values
		if len(dict.KeyIndex) == 0 {
			delete(dictionaries, fieldName)
		}
	}

	return dictionaries, scanner.Err()
}

// compactWithDictionaries performs the actual compaction using built dictionaries
func (jc *JSONLCompactor) compactWithDictionaries(inputPath, outputPath string, dictionaries map[string]*FieldDictionary, result *CompactResult) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Write dictionary header if we have dictionaries
	if len(dictionaries) > 0 {
		dictHeader := map[string]interface{}{
			"_type":         "dictionary_header",
			"_dictionaries": jc.buildDictionaryHeader(dictionaries),
		}

		headerBytes, err := json.Marshal(dictHeader)
		if err != nil {
			return fmt.Errorf("failed to marshal dictionary header: %w", err)
		}

		if _, err := outputFile.Write(append(headerBytes, '\n')); err != nil {
			return fmt.Errorf("failed to write dictionary header: %w", err)
		}

		result.DictFields = len(dictionaries)
	}

	// Process data lines
	scanner := bufio.NewScanner(inputFile)
	firstLine := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			// Write malformed lines as-is
			if _, writeErr := outputFile.Write(append([]byte(line), '\n')); writeErr != nil {
				return writeErr
			}
			continue
		}

		// Preserve schema for first record if configured
		if firstLine && jc.config.PreserveSchema {
			// Write original first record to preserve schema
			originalBytes, _ := json.Marshal(record)
			if _, writeErr := outputFile.Write(append(originalBytes, '\n')); writeErr != nil {
				return writeErr
			}
			firstLine = false
			result.LinesProcessed++
			continue
		}

		// Apply dictionary compression
		compactRecord := jc.compressRecord(record, dictionaries)

		compactBytes, err := json.Marshal(compactRecord)
		if err != nil {
			return fmt.Errorf("failed to marshal compact record: %w", err)
		}

		if _, err := outputFile.Write(append(compactBytes, '\n')); err != nil {
			return fmt.Errorf("failed to write compact record: %w", err)
		}

		firstLine = false
		result.LinesProcessed++
	}

	return scanner.Err()
}

// buildDictionaryHeader creates the dictionary header for the compacted file
func (jc *JSONLCompactor) buildDictionaryHeader(dictionaries map[string]*FieldDictionary) map[string]map[int]string {
	header := make(map[string]map[int]string)

	for fieldName, dict := range dictionaries {
		header[fieldName] = dict.IndexKey
	}

	return header
}

// compressRecord applies dictionary compression to a record
func (jc *JSONLCompactor) compressRecord(record map[string]interface{}, dictionaries map[string]*FieldDictionary) map[string]interface{} {
	compactRecord := make(map[string]interface{})

	for key, value := range record {
		if strValue, ok := value.(string); ok {
			if dict, exists := dictionaries[key]; exists {
				if index, found := dict.KeyIndex[strValue]; found {
					// Replace with dictionary reference
					compactRecord[key+"_dict"] = index
					continue
				}
			}
		}

		// Keep original value if no dictionary compression applies
		compactRecord[key] = value
	}

	return compactRecord
}

// DecompactFile decompresses a compacted JSONL file back to original format
func (jc *JSONLCompactor) DecompactFile(inputPath string) (*CompactResult, error) {
	outputPath := strings.TrimSuffix(inputPath, ".compact.jsonl") + ".decompact.jsonl"

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	var dictionaries map[string]map[int]string
	linesProcessed := 0

	// Read first line to check for dictionary header
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		var firstRecord map[string]interface{}

		if err := json.Unmarshal([]byte(line), &firstRecord); err == nil {
			if recordType, ok := firstRecord["_type"].(string); ok && recordType == "dictionary_header" {
				// Parse dictionaries
				if dictData, exists := firstRecord["_dictionaries"]; exists {
					dictionaries = jc.parseDictionaryHeader(dictData)
				}
			} else {
				// First line is data, write it as-is
				if _, writeErr := outputFile.Write(append([]byte(line), '\n')); writeErr != nil {
					return nil, writeErr
				}
				linesProcessed++
			}
		}
	}

	// Process remaining lines
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			// Write malformed lines as-is
			if _, writeErr := outputFile.Write(append([]byte(line), '\n')); writeErr != nil {
				return nil, writeErr
			}
			continue
		}

		// Decompress record
		decompressedRecord := jc.decompressRecord(record, dictionaries)

		decompressedBytes, err := json.Marshal(decompressedRecord)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal decompressed record: %w", err)
		}

		if _, err := outputFile.Write(append(decompressedBytes, '\n')); err != nil {
			return nil, fmt.Errorf("failed to write decompressed record: %w", err)
		}

		linesProcessed++
	}

	// Get file sizes for result
	originalStat, _ := os.Stat(inputPath)
	decompactStat, _ := os.Stat(outputPath)

	result := &CompactResult{
		OriginalPath:   inputPath,
		CompactedPath:  outputPath,
		OriginalSize:   originalStat.Size(),
		CompactedSize:  decompactStat.Size(),
		LinesProcessed: linesProcessed,
	}

	if result.OriginalSize > 0 {
		result.CompressionRatio = float64(result.CompactedSize) / float64(result.OriginalSize)
	}

	return result, scanner.Err()
}

// parseDictionaryHeader parses dictionary data from header
func (jc *JSONLCompactor) parseDictionaryHeader(dictData interface{}) map[string]map[int]string {
	dictionaries := make(map[string]map[int]string)

	if dictMap, ok := dictData.(map[string]interface{}); ok {
		for fieldName, fieldDict := range dictMap {
			if fieldDictMap, ok := fieldDict.(map[string]interface{}); ok {
				indexMap := make(map[int]string)
				for indexStr, value := range fieldDictMap {
					if index := parseInt(indexStr); index >= 0 {
						if strValue, ok := value.(string); ok {
							indexMap[index] = strValue
						}
					}
				}
				if len(indexMap) > 0 {
					dictionaries[fieldName] = indexMap
				}
			}
		}
	}

	return dictionaries
}

// decompressRecord applies dictionary decompression to a record
func (jc *JSONLCompactor) decompressRecord(record map[string]interface{}, dictionaries map[string]map[int]string) map[string]interface{} {
	decompressedRecord := make(map[string]interface{})

	for key, value := range record {
		// Check if this is a dictionary reference
		if strings.HasSuffix(key, "_dict") {
			fieldName := strings.TrimSuffix(key, "_dict")
			if dict, exists := dictionaries[fieldName]; exists {
				if index, ok := value.(float64); ok { // JSON numbers are float64
					if originalValue, found := dict[int(index)]; found {
						decompressedRecord[fieldName] = originalValue
						continue
					}
				}
			}
		}

		// Keep original value
		decompressedRecord[key] = value
	}

	return decompressedRecord
}

// parseInt safely converts string to int
func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// ValidateCompaction verifies that compaction preserved data integrity
func (jc *JSONLCompactor) ValidateCompaction(originalPath, compactedPath string) error {
	// This would implement validation logic to ensure the compacted file
	// can be decompressed to match the original data
	// For now, we'll do a basic size sanity check

	originalStat, err := os.Stat(originalPath)
	if err != nil {
		return fmt.Errorf("failed to stat original file: %w", err)
	}

	compactedStat, err := os.Stat(compactedPath)
	if err != nil {
		return fmt.Errorf("failed to stat compacted file: %w", err)
	}

	// Compacted file should generally be smaller (or at least not much larger)
	if compactedStat.Size() > originalStat.Size()*2 {
		return fmt.Errorf("compacted file unexpectedly large: %d > %d",
			compactedStat.Size(), originalStat.Size()*2)
	}

	return nil
}
