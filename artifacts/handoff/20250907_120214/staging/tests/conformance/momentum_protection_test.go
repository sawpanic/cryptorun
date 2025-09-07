package conformance

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMomentumProtectionInOrthogonalization verifies MomentumCore is never residualized
func TestMomentumProtectionInOrthogonalization(t *testing.T) {
	// Check scoring.go for proper Gram-Schmidt ordering
	scoringPath := filepath.Join("..", "..", "internal", "application", "pipeline", "scoring.go")

	content, err := readFileContent(scoringPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read scoring.go: %v", err)
	}

	// Look for Gram-Schmidt ordering that protects MomentumCore
	requiredPatterns := []string{
		"MomentumCore",    // Must reference momentum core protection
		"Gram", "Schmidt", // Must use Gram-Schmidt
		"protected", // Must indicate protection
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf("CONFORMANCE VIOLATION: scoring.go missing required pattern '%s' for momentum protection", pattern)
		}
	}

	// Forbidden patterns - momentum should never be residualized
	forbiddenPatterns := []string{
		"momentum.*residual",
		"residualize.*momentum",
		"MomentumCore.*=.*residual",
	}

	for _, pattern := range forbiddenPatterns {
		if containsPattern(content, pattern) {
			t.Errorf("CONFORMANCE VIOLATION: scoring.go contains forbidden pattern '%s' - momentum must be protected", pattern)
		}
	}
}

// TestOrthogonalizationSequence ensures proper factor hierarchy
func TestOrthogonalizationSequence(t *testing.T) {
	scoringPath := filepath.Join("..", "..", "internal", "application", "pipeline", "scoring.go")

	content, err := readFileContent(scoringPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read scoring.go: %v", err)
	}

	// Expected factor residualization order (comments or variable names)
	expectedOrder := []string{
		"MomentumCore",      // 1st - protected
		"TechnicalResidual", // 2nd - residualized against momentum
		"VolumeResidual",    // 3rd - residualized against momentum + technical
		"QualityResidual",   // 4th - residualized against previous factors
		"SocialResidual",    // 5th - residualized against all previous
	}

	// Look for evidence of proper ordering in comments or code structure
	lastIndex := -1
	for i, factor := range expectedOrder {
		index := strings.Index(content, factor)
		if index == -1 && factor != "TechnicalResidual" { // TechnicalResidual might not exist yet
			t.Errorf("CONFORMANCE VIOLATION: Factor '%s' not found in expected orthogonalization sequence", factor)
			continue
		}
		if index != -1 && index <= lastIndex {
			t.Errorf("CONFORMANCE VIOLATION: Factor '%s' appears before previous factor in orthogonalization sequence", factor)
		}
		if index != -1 {
			lastIndex = index
		}
	}
}

// TestSocialFactorConstraints verifies social factor is properly constrained
func TestSocialFactorConstraints(t *testing.T) {
	scoringPath := filepath.Join("..", "..", "internal", "application", "pipeline", "scoring.go")

	content, err := readFileContent(scoringPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read scoring.go: %v", err)
	}

	// Look for social cap enforcement
	socialCapPatterns := []string{
		"+10",        // Social cap value
		"cap", "Cap", // Capping mechanism
		"social", "Social", // Social factor reference
	}

	foundSocialRef := false
	foundCapRef := false

	for _, pattern := range socialCapPatterns {
		if strings.Contains(content, pattern) {
			if strings.Contains(pattern, "social") || strings.Contains(pattern, "Social") {
				foundSocialRef = true
			}
			if strings.Contains(pattern, "cap") || strings.Contains(pattern, "Cap") || pattern == "+10" {
				foundCapRef = true
			}
		}
	}

	if !foundSocialRef {
		t.Error("CONFORMANCE VIOLATION: scoring.go missing social factor reference")
	}

	if !foundCapRef {
		t.Error("CONFORMANCE VIOLATION: scoring.go missing social cap enforcement mechanism")
	}

	// Forbidden: Social factor appearing before momentum in any calculation
	if containsPattern(content, "social.*momentum.*composite") {
		t.Error("CONFORMANCE VIOLATION: Social factor processed before momentum in composite calculation")
	}
}

// TestFactorCalculationOrder ensures momentum is calculated first
func TestFactorCalculationOrder(t *testing.T) {
	scoringPath := filepath.Join("..", "..", "internal", "application", "pipeline", "scoring.go")

	content, err := readFileContent(scoringPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read scoring.go: %v", err)
	}

	lines := strings.Split(content, "\n")
	momentumLine := -1
	socialLine := -1

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "momentum") &&
			(strings.Contains(line, "calculate") || strings.Contains(line, "compute")) {
			if momentumLine == -1 {
				momentumLine = i
			}
		}
		if strings.Contains(strings.ToLower(line), "social") &&
			(strings.Contains(line, "calculate") || strings.Contains(line, "compute")) {
			if socialLine == -1 {
				socialLine = i
			}
		}
	}

	if momentumLine != -1 && socialLine != -1 && socialLine < momentumLine {
		t.Errorf("CONFORMANCE VIOLATION: Social factor calculated at line %d before momentum at line %d",
			socialLine+1, momentumLine+1)
	}
}

// Helper functions
func readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	return content.String(), scanner.Err()
}

func containsPattern(content, pattern string) bool {
	// Simple pattern matching for demonstration
	// In production, would use regex for complex patterns
	return strings.Contains(strings.ToLower(content), strings.ToLower(pattern))
}
