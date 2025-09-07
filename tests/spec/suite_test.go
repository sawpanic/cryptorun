package spec

import (
	"testing"

	"github.com/sawpanic/cryptorun/internal/spec"
)

// TestSpecificationCompliance runs the complete specification compliance suite
func TestSpecificationCompliance(t *testing.T) {
	// Create all specification sections
	sections := []spec.SpecSection{
		&spec.FactorHierarchySpec{},
		&spec.GuardsSpec{},
		&spec.MicrostructureSpec{},
		&spec.SocialCapSpec{},
		&spec.RegimeSwitchSpec{},
	}

	// Create and run spec runner
	runner := spec.NewSpecRunner(sections)
	summary := runner.RunAll()

	// Print detailed results for debugging
	if testing.Verbose() {
		runner.PrintResults(summary)
	}

	// Assert overall compliance
	if !summary.OverallPassed {
		t.Errorf("Specification compliance failed: %d/%d sections passed, %d/%d specs passed",
			summary.PassedSections, summary.TotalSections,
			summary.PassedSpecs, summary.TotalSpecs)

		// Print failed specs for diagnosis
		for _, section := range summary.Sections {
			if !section.Passed {
				t.Errorf("Section '%s' failed:", section.Name)
				for _, result := range section.Results {
					if !result.Passed {
						t.Errorf("  - %s: %s", result.Name, result.Error)
						if result.Details != "" {
							t.Errorf("    Details: %s", result.Details)
						}
					}
				}
			}
		}
	}

	// Log summary for successful runs
	t.Logf("Specification compliance summary: %s", summary.String())
}

// TestFactorHierarchySpec tests factor hierarchy compliance in isolation
func TestFactorHierarchySpec(t *testing.T) {
	spec := &spec.FactorHierarchySpec{}
	results := spec.RunSpecs()

	for _, result := range results {
		if !result.Passed {
			t.Errorf("Factor hierarchy spec failed - %s: %s", result.Name, result.Error)
			if result.Details != "" {
				t.Logf("Details: %s", result.Details)
			}
		} else {
			t.Logf("✅ %s: %s", result.Name, result.Description)
		}
	}
}

// TestGuardsSpec tests trading guards compliance in isolation
func TestGuardsSpec(t *testing.T) {
	spec := &spec.GuardsSpec{}
	results := spec.RunSpecs()

	for _, result := range results {
		if !result.Passed {
			t.Errorf("Guards spec failed - %s: %s", result.Name, result.Error)
			if result.Details != "" {
				t.Logf("Details: %s", result.Details)
			}
		} else {
			t.Logf("✅ %s: %s", result.Name, result.Description)
		}
	}
}

// TestMicrostructureSpec tests microstructure gates compliance in isolation
func TestMicrostructureSpec(t *testing.T) {
	spec := &spec.MicrostructureSpec{}
	results := spec.RunSpecs()

	for _, result := range results {
		if !result.Passed {
			t.Errorf("Microstructure spec failed - %s: %s", result.Name, result.Error)
			if result.Details != "" {
				t.Logf("Details: %s", result.Details)
			}
		} else {
			t.Logf("✅ %s: %s", result.Name, result.Description)
		}
	}
}

// TestSocialCapSpec tests social factor capping compliance in isolation
func TestSocialCapSpec(t *testing.T) {
	spec := &spec.SocialCapSpec{}
	results := spec.RunSpecs()

	for _, result := range results {
		if !result.Passed {
			t.Errorf("Social cap spec failed - %s: %s", result.Name, result.Error)
			if result.Details != "" {
				t.Logf("Details: %s", result.Details)
			}
		} else {
			t.Logf("✅ %s: %s", result.Name, result.Description)
		}
	}
}

// TestRegimeSwitchSpec tests regime switching compliance in isolation
func TestRegimeSwitchSpec(t *testing.T) {
	spec := &spec.RegimeSwitchSpec{}
	results := spec.RunSpecs()

	for _, result := range results {
		if !result.Passed {
			t.Errorf("Regime switch spec failed - %s: %s", result.Name, result.Error)
			if result.Details != "" {
				t.Logf("Details: %s", result.Details)
			}
		} else {
			t.Logf("✅ %s: %s", result.Name, result.Description)
		}
	}
}

// BenchmarkSpecificationSuite benchmarks the complete specification suite
func BenchmarkSpecificationSuite(b *testing.B) {
	// Create all specification sections
	sections := []spec.SpecSection{
		&spec.FactorHierarchySpec{},
		&spec.GuardsSpec{},
		&spec.MicrostructureSpec{},
		&spec.SocialCapSpec{},
		&spec.RegimeSwitchSpec{},
	}

	runner := spec.NewSpecRunner(sections)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		summary := runner.RunAll()
		if !summary.OverallPassed {
			b.Fatalf("Specification compliance failed during benchmark")
		}
	}
}
