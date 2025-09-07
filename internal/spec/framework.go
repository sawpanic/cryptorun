package spec

import (
	"fmt"
	"time"
)

// SpecResult represents the outcome of a single specification test
type SpecResult struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Passed      bool      `json:"passed"`
	Error       string    `json:"error,omitempty"`
	Details     string    `json:"details,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// SpecStatus represents pass/fail status
type SpecStatus int

const (
	SpecPass SpecStatus = iota
	SpecFail
)

func (s SpecStatus) String() string {
	if s == SpecPass {
		return "PASS"
	}
	return "FAIL"
}

// SpecSection represents a collection of related specification tests
type SpecSection interface {
	Name() string
	Description() string
	RunSpecs() []SpecResult
}

// SpecSummary represents the overall results of all specification sections
type SpecSummary struct {
	TotalSections  int              `json:"total_sections"`
	PassedSections int              `json:"passed_sections"`
	FailedSections int              `json:"failed_sections"`
	TotalSpecs     int              `json:"total_specs"`
	PassedSpecs    int              `json:"passed_specs"`
	FailedSpecs    int              `json:"failed_specs"`
	Sections       []SectionSummary `json:"sections"`
	ExecutionTime  time.Duration    `json:"execution_time"`
	OverallPassed  bool             `json:"overall_passed"`
	Timestamp      time.Time        `json:"timestamp"`
}

// SectionSummary represents the results of a single specification section
type SectionSummary struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Passed      bool         `json:"passed"`
	SpecCount   int          `json:"spec_count"`
	PassedCount int          `json:"passed_count"`
	FailedCount int          `json:"failed_count"`
	Results     []SpecResult `json:"results"`
}

// SpecRunner coordinates the execution of all specification sections
type SpecRunner struct {
	sections []SpecSection
}

// NewSpecRunner creates a spec runner with all sections
func NewSpecRunner() *SpecRunner {
	return &SpecRunner{
		sections: []SpecSection{
			NewFactorHierarchySpec(),
			SpecSection(NewGuardSpec()),
			NewMicrostructureSpec(),
			NewSocialCapSpec(),
			SpecSection(NewRegimeSwitchingSpec()),
		},
	}
}

// NewSpecRunnerWithSections creates a new SpecRunner with the provided sections
func NewSpecRunnerWithSections(sections []SpecSection) *SpecRunner {
	return &SpecRunner{
		sections: sections,
	}
}

// RunAll executes all specification sections and returns a summary
func (sr *SpecRunner) RunAll() SpecSummary {
	startTime := time.Now()

	summary := SpecSummary{
		TotalSections: len(sr.sections),
		Sections:      make([]SectionSummary, 0, len(sr.sections)),
		Timestamp:     startTime.UTC(),
	}

	for _, section := range sr.sections {
		sectionResults := section.RunSpecs()

		sectionSummary := SectionSummary{
			Name:        section.Name(),
			Description: section.Description(),
			SpecCount:   len(sectionResults),
			Results:     sectionResults,
		}

		// Count passed/failed specs for this section
		for _, result := range sectionResults {
			if result.Passed {
				sectionSummary.PassedCount++
			} else {
				sectionSummary.FailedCount++
			}
		}

		// Section passes if all specs pass
		sectionSummary.Passed = sectionSummary.FailedCount == 0

		// Update overall counters
		summary.TotalSpecs += sectionSummary.SpecCount
		summary.PassedSpecs += sectionSummary.PassedCount
		summary.FailedSpecs += sectionSummary.FailedCount

		if sectionSummary.Passed {
			summary.PassedSections++
		} else {
			summary.FailedSections++
		}

		summary.Sections = append(summary.Sections, sectionSummary)
	}

	summary.ExecutionTime = time.Since(startTime)
	summary.OverallPassed = summary.FailedSections == 0

	return summary
}

// PrintResults prints detailed results to stdout
func (sr *SpecRunner) PrintResults(summary SpecSummary) {
	fmt.Printf("\n=== CryptoRun Specification Compliance Suite ===\n")
	fmt.Printf("Executed: %s\n", summary.Timestamp.Format(time.RFC3339))
	fmt.Printf("Duration: %.1fms\n\n", float64(summary.ExecutionTime.Nanoseconds())/1e6)

	for _, section := range summary.Sections {
		status := "✅ PASS"
		if !section.Passed {
			status = "❌ FAIL"
		}

		fmt.Printf("%s %s: %s\n", status, section.Name, section.Description)
		fmt.Printf("    Specs: %d/%d passed\n", section.PassedCount, section.SpecCount)

		// Print failed specs with details
		for _, result := range section.Results {
			if !result.Passed {
				fmt.Printf("    ❌ %s: %s\n", result.Name, result.Description)
				if result.Error != "" {
					fmt.Printf("       Error: %s\n", result.Error)
				}
				if result.Details != "" {
					fmt.Printf("       Details: %s\n", result.Details)
				}
			}
		}
		fmt.Println()
	}

	overallStatus := "✅ PASS"
	if !summary.OverallPassed {
		overallStatus = "❌ FAIL"
	}

	fmt.Printf("=== OVERALL RESULT: %s ===\n", overallStatus)
	fmt.Printf("Sections: %d/%d passed\n", summary.PassedSections, summary.TotalSections)
	fmt.Printf("Specs: %d/%d passed\n", summary.PassedSpecs, summary.TotalSpecs)
}

// PrintCompactChecklist prints a compact checklist format for menu integration
func (sr *SpecRunner) PrintCompactChecklist(summary SpecSummary) {
	fmt.Print(summary.CompactChecklist())
}

// NewSpecResult creates a new passing SpecResult
func NewSpecResult(name, description string) SpecResult {
	return SpecResult{
		Name:        name,
		Description: description,
		Passed:      true,
		Timestamp:   time.Now().UTC(),
	}
}

// NewFailedSpecResult creates a new failing SpecResult
func NewFailedSpecResult(name, description, errorMsg string) SpecResult {
	return SpecResult{
		Name:        name,
		Description: description,
		Passed:      false,
		Error:       errorMsg,
		Timestamp:   time.Now().UTC(),
	}
}

// WithDetails adds additional details to a SpecResult
func (sr SpecResult) WithDetails(details string) SpecResult {
	sr.Details = details
	return sr
}

// String provides a human-readable representation of the SpecResult
func (sr SpecResult) String() string {
	status := "PASS"
	if !sr.Passed {
		status = "FAIL"
	}

	result := fmt.Sprintf("[%s] %s: %s", status, sr.Name, sr.Description)

	if sr.Error != "" {
		result += fmt.Sprintf(" - ERROR: %s", sr.Error)
	}

	if sr.Details != "" {
		result += fmt.Sprintf(" (%s)", sr.Details)
	}

	return result
}

// String provides a human-readable representation of the SpecSummary
func (ss SpecSummary) String() string {
	status := "PASS"
	if !ss.OverallPassed {
		status = "FAIL"
	}

	return fmt.Sprintf("Specification Suite [%s]: %d/%d sections passed, %d/%d specs passed (%.1fms)",
		status, ss.PassedSections, ss.TotalSections, ss.PassedSpecs, ss.TotalSpecs,
		float64(ss.ExecutionTime.Nanoseconds())/1e6)
}

// CompactChecklist returns a compact checklist format for menu display
func (ss SpecSummary) CompactChecklist() string {
	result := "Specification Compliance Checklist:\n"

	for _, section := range ss.Sections {
		status := "✅"
		if !section.Passed {
			status = "❌"
		}
		result += fmt.Sprintf("  %s %s (%d/%d specs)\n",
			status, section.Name, section.PassedCount, section.SpecCount)
	}

	overallStatus := "✅ PASS"
	if !ss.OverallPassed {
		overallStatus = "❌ FAIL"
	}
	result += fmt.Sprintf("\nOverall: %s (%d/%d sections passed)\n",
		overallStatus, ss.PassedSections, ss.TotalSections)

	return result
}
