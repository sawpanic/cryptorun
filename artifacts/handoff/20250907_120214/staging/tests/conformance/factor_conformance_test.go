package conformance_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// PipelineConfig represents factor pipeline configuration
type PipelineConfig struct {
	Pipeline struct {
		ProtectedFactors []string `yaml:"protected_factors"`
		MaxSymbols       int      `yaml:"max_symbols"`
	} `yaml:"pipeline"`

	Social struct {
		MaxContribution float64 `yaml:"max_contribution"`
		Cap             float64 `yaml:"cap"`
	} `yaml:"social,omitempty"`

	Brand struct {
		MaxContribution float64 `yaml:"max_contribution"`
		Cap             float64 `yaml:"cap"`
	} `yaml:"brand,omitempty"`
}

// TestMomentumProtectionConformance verifies MomentumCore is protected from residualization
func TestMomentumProtectionConformance(t *testing.T) {
	// Load momentum configuration
	configData, err := os.ReadFile("config/momentum.yaml")
	if err != nil {
		t.Fatalf("Failed to read momentum config: %v", err)
	}

	var config PipelineConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse momentum config: %v", err)
	}

	// Verify MomentumCore is in protected factors
	t.Run("MomentumCoreProtected", func(t *testing.T) {
		found := false
		for _, factor := range config.Pipeline.ProtectedFactors {
			if factor == "MomentumCore" {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("CONFORMANCE VIOLATION: MomentumCore must be in protected_factors list")
		}
	})

	// Verify protected factors are not empty
	t.Run("ProtectedFactorsNotEmpty", func(t *testing.T) {
		if len(config.Pipeline.ProtectedFactors) == 0 {
			t.Errorf("CONFORMANCE VIOLATION: protected_factors cannot be empty")
		}
	})
}

// TestSocialCapConformance verifies social/brand factor cap ≤ +10
func TestSocialCapConformance(t *testing.T) {
	configs := []string{"config/momentum.yaml", "config/dip.yaml"}

	for _, configPath := range configs {
		t.Run(strings.ReplaceAll(configPath, "/", "_"), func(t *testing.T) {
			configData, err := os.ReadFile(configPath)
			if err != nil {
				// Skip if config doesn't exist
				t.Skipf("Config file %s not found", configPath)
			}

			var config PipelineConfig
			if err := yaml.Unmarshal(configData, &config); err != nil {
				t.Fatalf("Failed to parse config %s: %v", configPath, err)
			}

			// Test social cap
			if config.Social.Cap > 0 {
				if config.Social.Cap > 10.0 {
					t.Errorf("CONFORMANCE VIOLATION: Social cap = %.1f, must be ≤ +10", config.Social.Cap)
				}
			}

			if config.Social.MaxContribution > 0 {
				if config.Social.MaxContribution > 10.0 {
					t.Errorf("CONFORMANCE VIOLATION: Social max_contribution = %.1f, must be ≤ +10",
						config.Social.MaxContribution)
				}
			}

			// Test brand cap
			if config.Brand.Cap > 0 {
				if config.Brand.Cap > 10.0 {
					t.Errorf("CONFORMANCE VIOLATION: Brand cap = %.1f, must be ≤ +10", config.Brand.Cap)
				}
			}

			if config.Brand.MaxContribution > 0 {
				if config.Brand.MaxContribution > 10.0 {
					t.Errorf("CONFORMANCE VIOLATION: Brand max_contribution = %.1f, must be ≤ +10",
						config.Brand.MaxContribution)
				}
			}
		})
	}
}

// TestOrthogonalizationConformance verifies Gram-Schmidt protection
func TestOrthogonalizationConformance(t *testing.T) {
	// Check source code for proper orthogonalization implementation
	sourceFiles := []string{
		"internal/domain/scoring/orthogonal.go",
		"src/application/pipeline/orthogonalization.go",
	}

	for _, filePath := range sourceFiles {
		t.Run(strings.ReplaceAll(filePath, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				// Skip if file doesn't exist
				t.Skipf("Source file %s not found", filePath)
			}

			content := string(data)

			// Verify protected factors are not residualized
			if strings.Contains(content, "residualize") || strings.Contains(content, "Residualize") {
				// Should not residualize protected factors
				if !strings.Contains(content, "protected") && !strings.Contains(content, "Protected") {
					t.Errorf("CONFORMANCE VIOLATION: File %s contains residualization without protection checks",
						filePath)
				}
			}

			// Verify Gram-Schmidt implementation mentions protection
			if strings.Contains(content, "GramSchmidt") || strings.Contains(content, "gram_schmidt") {
				if !strings.Contains(content, "protect") && !strings.Contains(content, "Protected") {
					t.Errorf("CONFORMANCE VIOLATION: Gram-Schmidt in %s lacks protected factor handling",
						filePath)
				}
			}
		})
	}
}
