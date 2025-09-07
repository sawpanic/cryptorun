package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sawpanic/cryptorun/internal/artifacts/compact"
)

func TestJSONLCompactor_SchemaPreserved(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.jsonl")

	// Create test JSONL with repetitive data
	jsonlContent := `{"timestamp": "2025-09-06T14:30:22Z", "symbol": "BTCUSD", "status": "success", "message": "Order filled", "details": "Large order successfully executed"}
{"timestamp": "2025-09-06T14:31:15Z", "symbol": "ETHUSD", "status": "success", "message": "Order filled", "details": "Medium order successfully executed"}
{"timestamp": "2025-09-06T14:32:08Z", "symbol": "BTCUSD", "status": "success", "message": "Order filled", "details": "Small order successfully executed"}
{"timestamp": "2025-09-06T14:33:42Z", "symbol": "ADAUSD", "status": "failed", "message": "Order rejected", "details": "Insufficient liquidity for requested size"}`

	if err := os.WriteFile(inputPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.JSONLConfig{
		Enabled:        true,
		MinSizeKB:      0, // Allow small files for testing
		DictThreshold:  2, // Low threshold to trigger compression
		PreserveSchema: true,
	}

	compactor := compact.NewJSONLCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Check that compaction occurred
	if result.CompressionRatio >= 1.0 {
		t.Errorf("Expected compression ratio < 1.0, got %.2f", result.CompressionRatio)
	}

	if result.LinesProcessed != 4 {
		t.Errorf("Expected 4 lines processed, got %d", result.LinesProcessed)
	}

	// Read compacted file
	compactedBytes, err := os.ReadFile(result.CompactedPath)
	if err != nil {
		t.Fatalf("Failed to read compacted file: %v", err)
	}

	compactedContent := string(compactedBytes)
	lines := strings.Split(strings.TrimSpace(compactedContent), "\n")

	// Should have dictionary header + preserved first record + compressed records
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines (header + preserved + compressed), got %d", len(lines))
	}

	// First line should be dictionary header
	if !strings.Contains(lines[0], "_type") || !strings.Contains(lines[0], "dictionary_header") {
		t.Error("First line should be dictionary header")
	}

	// Second line should be preserved original schema
	if !strings.Contains(lines[1], "2025-09-06T14:30:22Z") {
		t.Error("Second line should be preserved first record")
	}

	// Subsequent lines should use dictionary compression
	foundDictRef := false
	for i := 2; i < len(lines); i++ {
		if strings.Contains(lines[i], "_dict") {
			foundDictRef = true
			break
		}
	}
	if !foundDictRef {
		t.Error("Expected to find dictionary references in compacted lines")
	}
}

func TestJSONLCompactor_BytesReduced(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "large_test.jsonl")

	// Create larger test file with highly repetitive content
	var content strings.Builder
	for i := 0; i < 100; i++ {
		content.WriteString(`{"timestamp": "2025-09-06T14:30:22Z", "symbol": "BTCUSD", "status": "success", "message": "Order filled successfully with optimal execution", "venue": "kraken", "order_type": "market"}`)
		content.WriteString("\n")
	}

	if err := os.WriteFile(inputPath, []byte(content.String()), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.JSONLConfig{
		Enabled:        true,
		MinSizeKB:      0,
		DictThreshold:  3,
		PreserveSchema: true,
	}

	compactor := compact.NewJSONLCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Should achieve significant compression with repetitive data
	if result.CompressionRatio > 0.8 {
		t.Errorf("Expected compression ratio < 0.8, got %.2f", result.CompressionRatio)
	}

	if result.OriginalSize <= result.CompactedSize {
		t.Errorf("Compacted size should be smaller: original=%d compacted=%d",
			result.OriginalSize, result.CompactedSize)
	}

	if result.DictFields == 0 {
		t.Error("Expected dictionary fields to be used")
	}
}

func TestMarkdownCompactor_RemovesEmptySections(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.md")

	// Create test Markdown with empty sections
	mdContent := `# Main Title

This is the introduction.

## Empty Section 1

## Section With Content

This section has actual content.

### Subsection

More content here.

## Another Empty Section


## Final Section

Final content.

`

	if err := os.WriteFile(inputPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.MarkdownConfig{
		Enabled:             true,
		MinSizeKB:           0, // Allow small files
		RemoveEmptySections: true,
		CanonicalHeaders:    true,
		PreserveTimestamps:  false,
	}

	compactor := compact.NewMarkdownCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Should have some compression
	if result.CompressionRatio >= 1.0 {
		t.Errorf("Expected some compression, got ratio %.2f", result.CompressionRatio)
	}

	// Read compacted file
	compactedBytes, err := os.ReadFile(result.CompactedPath)
	if err != nil {
		t.Fatalf("Failed to read compacted file: %v", err)
	}

	compactedContent := string(compactedBytes)

	// Should not contain empty section headers
	if strings.Contains(compactedContent, "## Empty Section 1") {
		t.Error("Empty section should have been removed")
	}

	if strings.Contains(compactedContent, "## Another Empty Section") {
		t.Error("Another empty section should have been removed")
	}

	// Should still contain sections with content
	if !strings.Contains(compactedContent, "## Section With Content") {
		t.Error("Section with content should be preserved")
	}

	if !strings.Contains(compactedContent, "### Subsection") {
		t.Error("Subsection should be preserved")
	}
}

func TestMarkdownCompactor_CanonicalizesHeaders(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.md")

	// Create test Markdown with inconsistent header formatting
	mdContent := `#    Title With Extra Spaces   

##Compact Header

### Normal Header

####    Another Inconsistent Header    

Content here.
`

	if err := os.WriteFile(inputPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.MarkdownConfig{
		Enabled:             true,
		MinSizeKB:           0,
		RemoveEmptySections: false, // Keep all sections for header testing
		CanonicalHeaders:    true,
		PreserveTimestamps:  false,
	}

	compactor := compact.NewMarkdownCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Read compacted file
	compactedBytes, err := os.ReadFile(result.CompactedPath)
	if err != nil {
		t.Fatalf("Failed to read compacted file: %v", err)
	}

	compactedContent := string(compactedBytes)
	lines := strings.Split(compactedContent, "\n")

	// Check that headers are canonicalized
	expectedHeaders := []string{
		"# Title With Extra Spaces",
		"## Compact Header",
		"### Normal Header",
		"#### Another Inconsistent Header",
	}

	headerCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			if headerCount >= len(expectedHeaders) {
				t.Fatalf("Too many headers found")
			}

			if line != expectedHeaders[headerCount] {
				t.Errorf("Header %d: expected '%s', got '%s'",
					headerCount, expectedHeaders[headerCount], line)
			}
			headerCount++
		}
	}

	if headerCount != len(expectedHeaders) {
		t.Errorf("Expected %d headers, found %d", len(expectedHeaders), headerCount)
	}
}

func TestMarkdownCompactor_PreservesTimestamps(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.md")

	mdContent := `# Test Document

Some content.
`

	if err := os.WriteFile(inputPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.MarkdownConfig{
		Enabled:             true,
		MinSizeKB:           0,
		RemoveEmptySections: true,
		CanonicalHeaders:    true,
		PreserveTimestamps:  true,
	}

	compactor := compact.NewMarkdownCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Read compacted file
	compactedBytes, err := os.ReadFile(result.CompactedPath)
	if err != nil {
		t.Fatalf("Failed to read compacted file: %v", err)
	}

	compactedContent := string(compactedBytes)

	// Should contain timestamp comment
	if !strings.Contains(compactedContent, "<!-- Compacted on") {
		t.Error("Expected timestamp comment when PreserveTimestamps is true")
	}
}

func TestCompaction_ValidateIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.jsonl")

	// Create test content
	jsonlContent := `{"id": 1, "name": "test"}
{"id": 2, "name": "another test"}`

	if err := os.WriteFile(inputPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.JSONLConfig{
		Enabled:        true,
		MinSizeKB:      0,
		DictThreshold:  1,
		PreserveSchema: true,
	}

	compactor := compact.NewJSONLCompactor(config)
	result, err := compactor.CompactFile(inputPath)
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Validate the compaction
	err = compactor.ValidateCompaction(inputPath, result.CompactedPath)
	if err != nil {
		t.Errorf("Compaction validation failed: %v", err)
	}
}

func TestCompaction_PreviewFeature(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test.md")

	// Create test Markdown with empty sections
	mdContent := `# Main Title

Content here.

## Empty Section

## Another Section

More content.

## Yet Another Empty Section

`

	if err := os.WriteFile(inputPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := compact.MarkdownConfig{
		Enabled:             true,
		MinSizeKB:           0,
		RemoveEmptySections: true,
		CanonicalHeaders:    true,
		PreserveTimestamps:  false,
	}

	compactor := compact.NewMarkdownCompactor(config)
	preview, err := compactor.GetCompactionPreview(inputPath)
	if err != nil {
		t.Fatalf("Failed to get preview: %v", err)
	}

	// Should predict section removal
	if preview.OriginalSections <= 0 {
		t.Error("Expected positive original section count")
	}

	if preview.CompactedSections >= preview.OriginalSections {
		t.Error("Expected compacted sections to be fewer than original")
	}

	if preview.EmptySections <= 0 {
		t.Error("Expected to detect empty sections")
	}

	if preview.EstimatedReduction <= 0 {
		t.Error("Expected positive estimated reduction")
	}
}
