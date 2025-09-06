package compact

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// MarkdownCompactor compacts Markdown files by removing empty sections and canonicalizing headers
type MarkdownCompactor struct {
	config MarkdownConfig
}

// MarkdownConfig configures Markdown compaction behavior
type MarkdownConfig struct {
	Enabled             bool `yaml:"enabled"`
	MinSizeKB           int  `yaml:"min_size_kb"`
	RemoveEmptySections bool `yaml:"remove_empty_sections"`
	CanonicalHeaders    bool `yaml:"canonical_headers"`
	PreserveTimestamps  bool `yaml:"preserve_timestamps"`
}

// MarkdownSection represents a section of a Markdown document
type MarkdownSection struct {
	Header    string   `json:"header"`
	Level     int      `json:"level"`
	Content   []string `json:"content"`
	IsEmpty   bool     `json:"is_empty"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
}

// NewMarkdownCompactor creates a new Markdown compactor
func NewMarkdownCompactor(config MarkdownConfig) *MarkdownCompactor {
	return &MarkdownCompactor{
		config: config,
	}
}

// CompactFile compacts a single Markdown file
func (mc *MarkdownCompactor) CompactFile(inputPath string) (*CompactResult, error) {
	if !mc.config.Enabled {
		return nil, fmt.Errorf("Markdown compaction is disabled")
	}

	// Check file size threshold
	stat, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	if stat.Size() < int64(mc.config.MinSizeKB*1024) {
		return nil, fmt.Errorf("file too small for compaction: %d bytes < %d KB threshold",
			stat.Size(), mc.config.MinSizeKB)
	}

	// Create output path
	outputPath := strings.TrimSuffix(inputPath, ".md") + ".compact.md"

	result := &CompactResult{
		OriginalPath:  inputPath,
		CompactedPath: outputPath,
		OriginalSize:  stat.Size(),
	}

	// Parse the Markdown file into sections
	sections, err := mc.parseMarkdown(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Markdown: %w", err)
	}

	// Apply compaction rules
	compactedSections := mc.applyCompactionRules(sections)

	// Write compacted file
	if err := mc.writeCompactedMarkdown(outputPath, compactedSections); err != nil {
		return nil, fmt.Errorf("failed to write compacted file: %w", err)
	}

	// Calculate final statistics
	compactStat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	result.CompactedSize = compactStat.Size()
	result.LinesProcessed = len(compactedSections)
	if result.OriginalSize > 0 {
		result.CompressionRatio = float64(result.CompactedSize) / float64(result.OriginalSize)
	}

	return result, nil
}

// parseMarkdown parses a Markdown file into structured sections
func (mc *MarkdownCompactor) parseMarkdown(inputPath string) ([]MarkdownSection, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sections []MarkdownSection
	var currentSection *MarkdownSection
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Regex patterns
	headerPattern := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	_ = regexp.MustCompile(`^\s*$`) // emptyLinePattern for future use

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check if this is a header line
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			// Finish current section if exists
			if currentSection != nil {
				currentSection.EndLine = lineNum - 1
				currentSection.IsEmpty = mc.isSectionEmpty(*currentSection)
				sections = append(sections, *currentSection)
			}

			// Start new section
			level := len(matches[1]) // Number of # characters
			header := strings.TrimSpace(matches[2])

			currentSection = &MarkdownSection{
				Header:    header,
				Level:     level,
				Content:   make([]string, 0),
				StartLine: lineNum,
			}

			// Add the header line to content
			if mc.config.CanonicalHeaders {
				currentSection.Content = append(currentSection.Content, mc.canonicalizeHeader(level, header))
			} else {
				currentSection.Content = append(currentSection.Content, line)
			}
		} else {
			// Regular content line
			if currentSection == nil {
				// Create an implicit "document header" section for content before first header
				currentSection = &MarkdownSection{
					Header:    "Document",
					Level:     0,
					Content:   make([]string, 0),
					StartLine: 1,
				}
			}

			currentSection.Content = append(currentSection.Content, line)
		}
	}

	// Finish final section
	if currentSection != nil {
		currentSection.EndLine = lineNum
		currentSection.IsEmpty = mc.isSectionEmpty(*currentSection)
		sections = append(sections, *currentSection)
	}

	return sections, scanner.Err()
}

// applyCompactionRules applies the configured compaction rules to sections
func (mc *MarkdownCompactor) applyCompactionRules(sections []MarkdownSection) []MarkdownSection {
	var compacted []MarkdownSection

	for _, section := range sections {
		// Skip empty sections if configured
		if mc.config.RemoveEmptySections && section.IsEmpty {
			continue
		}

		// Apply content compaction
		compactedSection := mc.compactSectionContent(section)
		compacted = append(compacted, compactedSection)
	}

	return compacted
}

// compactSectionContent compacts the content within a section
func (mc *MarkdownCompactor) compactSectionContent(section MarkdownSection) MarkdownSection {
	compactedContent := make([]string, 0)

	// Track consecutive empty lines
	consecutiveEmpty := 0

	for _, line := range section.Content {
		isEmpty := strings.TrimSpace(line) == ""

		if isEmpty {
			consecutiveEmpty++
			// Only keep one empty line maximum between content
			if consecutiveEmpty <= 1 {
				compactedContent = append(compactedContent, line)
			}
		} else {
			consecutiveEmpty = 0

			// Apply line-specific compaction
			compactedLine := mc.compactLine(line)
			compactedContent = append(compactedContent, compactedLine)
		}
	}

	// Remove trailing empty lines
	for len(compactedContent) > 1 && strings.TrimSpace(compactedContent[len(compactedContent)-1]) == "" {
		compactedContent = compactedContent[:len(compactedContent)-1]
	}

	return MarkdownSection{
		Header:    section.Header,
		Level:     section.Level,
		Content:   compactedContent,
		IsEmpty:   len(compactedContent) <= 1, // Only header line
		StartLine: section.StartLine,
		EndLine:   section.EndLine,
	}
}

// compactLine applies line-specific compaction rules
func (mc *MarkdownCompactor) compactLine(line string) string {
	// Trim trailing whitespace
	line = strings.TrimRight(line, " \t")

	// Normalize multiple spaces (but preserve code formatting)
	if !strings.HasPrefix(strings.TrimSpace(line), "```") &&
		!strings.HasPrefix(line, "    ") && // Code block
		!strings.HasPrefix(line, "\t") { // Tab-indented

		// Normalize multiple spaces in regular text
		spacePattern := regexp.MustCompile(`\s+`)
		line = spacePattern.ReplaceAllString(line, " ")
	}

	return line
}

// canonicalizeHeader creates a canonical header format
func (mc *MarkdownCompactor) canonicalizeHeader(level int, text string) string {
	// Create consistent header format: # Text (no extra spaces)
	hashes := strings.Repeat("#", level)
	return fmt.Sprintf("%s %s", hashes, strings.TrimSpace(text))
}

// isSectionEmpty determines if a section has meaningful content
func (mc *MarkdownCompactor) isSectionEmpty(section MarkdownSection) bool {
	if len(section.Content) <= 1 {
		return true // Only header, no content
	}

	// Check if all content lines (except header) are empty or whitespace
	for i := 1; i < len(section.Content); i++ {
		if strings.TrimSpace(section.Content[i]) != "" {
			return false
		}
	}

	return true
}

// writeCompactedMarkdown writes the compacted sections to a file
func (mc *MarkdownCompactor) writeCompactedMarkdown(outputPath string, sections []MarkdownSection) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Add compaction header if preserving timestamps
	if mc.config.PreserveTimestamps {
		fmt.Fprintf(file, "<!-- Compacted on %s -->\n\n", time.Now().Format("2006-01-02 15:04:05"))
	}

	for i, section := range sections {
		// Write section content
		for _, line := range section.Content {
			fmt.Fprintln(file, line)
		}

		// Add spacing between sections (but not after the last one)
		if i < len(sections)-1 {
			fmt.Fprintln(file)
		}
	}

	return nil
}

// ValidateMarkdown performs basic validation of Markdown structure
func (mc *MarkdownCompactor) ValidateMarkdown(filePath string) error {
	sections, err := mc.parseMarkdown(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse Markdown for validation: %w", err)
	}

	// Basic structural validation
	headerLevels := make([]int, 0)

	for _, section := range sections {
		if section.Level > 0 {
			headerLevels = append(headerLevels, section.Level)
		}

		// Check for extremely long sections (possible parsing error)
		if len(section.Content) > 1000 {
			return fmt.Errorf("section '%s' has unusually many lines: %d", section.Header, len(section.Content))
		}
	}

	// Check header level consistency (should not skip levels drastically)
	for i := 1; i < len(headerLevels); i++ {
		if headerLevels[i] > headerLevels[i-1]+2 {
			return fmt.Errorf("header level jump detected: from %d to %d", headerLevels[i-1], headerLevels[i])
		}
	}

	return nil
}

// GetCompactionPreview generates a preview of what would be compacted
func (mc *MarkdownCompactor) GetCompactionPreview(inputPath string) (*CompactionPreview, error) {
	sections, err := mc.parseMarkdown(inputPath)
	if err != nil {
		return nil, err
	}

	preview := &CompactionPreview{
		OriginalSections: len(sections),
		EmptySections:    0,
		CanonicalHeaders: 0,
	}

	for _, section := range sections {
		if section.IsEmpty {
			preview.EmptySections++
		}

		if mc.config.CanonicalHeaders && section.Level > 0 {
			preview.CanonicalHeaders++
		}
	}

	// Calculate estimated size reduction
	compactedSections := mc.applyCompactionRules(sections)
	preview.CompactedSections = len(compactedSections)

	// Rough estimate based on section count
	if len(sections) > 0 {
		preview.EstimatedReduction = 1.0 - float64(len(compactedSections))/float64(len(sections))
	}

	return preview, nil
}

// CompactionPreview provides preview information before compacting
type CompactionPreview struct {
	OriginalSections   int     `json:"original_sections"`
	CompactedSections  int     `json:"compacted_sections"`
	EmptySections      int     `json:"empty_sections"`
	CanonicalHeaders   int     `json:"canonical_headers"`
	EstimatedReduction float64 `json:"estimated_reduction"`
}
