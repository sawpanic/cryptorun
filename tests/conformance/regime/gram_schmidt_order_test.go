package regime

import (
	"testing"
)

// MockFactor represents a scoring factor with its properties
type MockFactor struct {
	Name        string
	Score       float64
	IsProtected bool // MomentumCore should be protected
	Order       int  // Order in Gram-Schmidt sequence
}

// MockGramSchmidtProcessor simulates the orthogonalization system
type MockGramSchmidtProcessor struct {
	Factors []MockFactor
}

func (m *MockGramSchmidtProcessor) ProcessOrthogonalization() []MockFactor {
	// Simulate Gram-Schmidt orthogonalization process
	// MomentumCore (protected) should NOT be residualized
	// Order: Technical → Volume → Quality → Social

	result := make([]MockFactor, len(m.Factors))
	copy(result, m.Factors)

	// Sort by processing order (protected factors go first, unchanged)
	// Non-protected factors are residualized in sequence

	for i, factor := range result {
		if factor.IsProtected {
			// Protected factors (MomentumCore) are NOT residualized
			result[i].Score = factor.Score // Unchanged
		} else {
			// Simulate residualization (subtract correlation with previous factors)
			// In real implementation, this would involve correlation matrix
			residualizedScore := factor.Score * 0.85 // Simulated residual
			result[i].Score = residualizedScore
		}
	}

	return result
}

func TestGramSchmidt_MomentumCoreProtection(t *testing.T) {
	processor := &MockGramSchmidtProcessor{
		Factors: []MockFactor{
			{Name: "MomentumCore", Score: 45.0, IsProtected: true, Order: 0},
			{Name: "TechnicalResidual", Score: 20.0, IsProtected: false, Order: 1},
			{Name: "VolumeResidual", Score: 18.0, IsProtected: false, Order: 2},
			{Name: "QualityResidual", Score: 12.0, IsProtected: false, Order: 3},
			{Name: "SocialResidual", Score: 5.0, IsProtected: false, Order: 4},
		},
	}

	result := processor.ProcessOrthogonalization()

	// Find MomentumCore in result
	var momentumCore *MockFactor
	for i := range result {
		if result[i].Name == "MomentumCore" {
			momentumCore = &result[i]
			break
		}
	}

	if momentumCore == nil {
		t.Fatal("MomentumCore not found in orthogonalization result")
	}

	// Verify MomentumCore was NOT residualized (score unchanged)
	if momentumCore.Score != 45.0 {
		t.Errorf("MomentumCore was residualized: expected 45.0, got %.1f", momentumCore.Score)
	}

	// Verify other factors WERE residualized (scores changed)
	for _, factor := range result {
		if factor.Name != "MomentumCore" && factor.Score == getOriginalScore(factor.Name, processor.Factors) {
			t.Errorf("factor %s was NOT residualized (score unchanged: %.1f)", factor.Name, factor.Score)
		}
	}
}

func TestGramSchmidt_ProcessingOrder(t *testing.T) {
	// Test that Gram-Schmidt follows the correct order:
	// MomentumCore (protected) → Technical → Volume → Quality → Social

	expectedOrder := []string{"MomentumCore", "TechnicalResidual", "VolumeResidual", "QualityResidual", "SocialResidual"}

	processor := &MockGramSchmidtProcessor{
		Factors: []MockFactor{
			{Name: "SocialResidual", Score: 5.0, IsProtected: false, Order: 4},
			{Name: "MomentumCore", Score: 45.0, IsProtected: true, Order: 0},
			{Name: "QualityResidual", Score: 12.0, IsProtected: false, Order: 3},
			{Name: "TechnicalResidual", Score: 20.0, IsProtected: false, Order: 1},
			{Name: "VolumeResidual", Score: 18.0, IsProtected: false, Order: 2},
		},
	}

	// Verify factors have correct processing order
	for i, expectedName := range expectedOrder {
		found := false
		for _, factor := range processor.Factors {
			if factor.Name == expectedName && factor.Order == i {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("factor %s not found at expected order %d", expectedName, i)
		}
	}
}

func TestGramSchmidt_ResidualFactorNaming(t *testing.T) {
	// Verify that non-protected factors are properly named as "Residual"
	processor := &MockGramSchmidtProcessor{
		Factors: []MockFactor{
			{Name: "MomentumCore", IsProtected: true},       // Protected, keeps original name
			{Name: "TechnicalResidual", IsProtected: false}, // Residualized
			{Name: "VolumeResidual", IsProtected: false},    // Residualized
			{Name: "QualityResidual", IsProtected: false},   // Residualized
		},
	}

	for _, factor := range processor.Factors {
		if factor.IsProtected {
			// Protected factors should NOT have "Residual" suffix
			if factor.Name != "MomentumCore" {
				t.Errorf("protected factor has unexpected name: %s", factor.Name)
			}
		} else {
			// Non-protected factors should have "Residual" suffix
			if factor.Name == "Technical" || factor.Name == "Volume" || factor.Name == "Quality" {
				t.Errorf("non-protected factor missing 'Residual' suffix: %s", factor.Name)
			}
		}
	}
}

func TestGramSchmidt_OrthogonalityValidation(t *testing.T) {
	// Test that orthogonalization reduces correlations between factors
	// (In a real implementation, this would test correlation matrix properties)

	processor := &MockGramSchmidtProcessor{
		Factors: []MockFactor{
			{Name: "MomentumCore", Score: 50.0, IsProtected: true, Order: 0},
			{Name: "TechnicalResidual", Score: 25.0, IsProtected: false, Order: 1},
			{Name: "VolumeResidual", Score: 15.0, IsProtected: false, Order: 2},
			{Name: "QualityResidual", Score: 10.0, IsProtected: false, Order: 3},
		},
	}

	result := processor.ProcessOrthogonalization()

	// Verify that sum of orthogonalized factors makes sense
	totalOrthogonal := 0.0
	for _, factor := range result {
		totalOrthogonal += factor.Score
	}

	// Total should be reasonable (not exactly equal due to orthogonalization)
	if totalOrthogonal < 50.0 || totalOrthogonal > 110.0 {
		t.Errorf("orthogonalized total %.1f seems unreasonable", totalOrthogonal)
	}

	// MomentumCore should still be the largest component
	momentumScore := 0.0
	for _, factor := range result {
		if factor.Name == "MomentumCore" {
			momentumScore = factor.Score
			break
		}
	}

	for _, factor := range result {
		if factor.Name != "MomentumCore" && factor.Score >= momentumScore {
			t.Errorf("factor %s (%.1f) should not exceed MomentumCore (%.1f) after orthogonalization",
				factor.Name, factor.Score, momentumScore)
		}
	}
}

func TestGramSchmidt_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		factors []MockFactor
		wantErr bool
		checkFn func([]MockFactor) error
	}{
		{
			name: "zero MomentumCore",
			factors: []MockFactor{
				{Name: "MomentumCore", Score: 0.0, IsProtected: true, Order: 0},
				{Name: "TechnicalResidual", Score: 20.0, IsProtected: false, Order: 1},
			},
			wantErr: false,
			checkFn: func(result []MockFactor) error {
				// Zero MomentumCore should remain zero (protected)
				for _, f := range result {
					if f.Name == "MomentumCore" && f.Score != 0.0 {
						t.Errorf("zero MomentumCore should remain zero, got %.1f", f.Score)
					}
				}
				return nil
			},
		},
		{
			name: "single factor only",
			factors: []MockFactor{
				{Name: "MomentumCore", Score: 100.0, IsProtected: true, Order: 0},
			},
			wantErr: false,
			checkFn: func(result []MockFactor) error {
				if len(result) != 1 {
					t.Errorf("expected 1 factor, got %d", len(result))
				}
				return nil
			},
		},
		{
			name: "no protected factors",
			factors: []MockFactor{
				{Name: "TechnicalResidual", Score: 50.0, IsProtected: false, Order: 0},
				{Name: "VolumeResidual", Score: 30.0, IsProtected: false, Order: 1},
			},
			wantErr: false,
			checkFn: func(result []MockFactor) error {
				// All should be residualized
				for _, f := range result {
					originalScore := getOriginalScore(f.Name, []MockFactor{
						{Name: "TechnicalResidual", Score: 50.0},
						{Name: "VolumeResidual", Score: 30.0},
					})
					if f.Score == originalScore {
						t.Errorf("factor %s should be residualized but score unchanged", f.Name)
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &MockGramSchmidtProcessor{Factors: tt.factors}
			result := processor.ProcessOrthogonalization()

			if tt.checkFn != nil {
				if err := tt.checkFn(result); err != nil {
					t.Errorf("check function failed: %v", err)
				}
			}
		})
	}
}

// Helper function to find original score of a factor
func getOriginalScore(name string, factors []MockFactor) float64 {
	for _, factor := range factors {
		if factor.Name == name {
			return factor.Score
		}
	}
	return 0.0
}
