package unit

import (
	"math"
	"testing"

	"cryptorun/internal/algo/momentum"
)

func TestGramSchmidtOrthogonalization(t *testing.T) {
	// Create test factor matrix
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD", "ADAUSD"},
		Factors: []string{"MomentumCore", "TechnicalResidual", "VolumeResidual", "QualityResidual"},
		Data: [][]float64{
			{5.2, 4.8, 2.1, 50.0}, // BTCUSD
			{3.8, 3.0, 1.5, 45.0}, // ETHUSD
			{2.1, 1.8, 3.2, 55.0}, // ADAUSD
		},
	}

	protectedFactors := []string{"MomentumCore"}
	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer(protectedFactors)

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Verify result structure
	if len(result.OrthogonalMatrix.Symbols) != len(matrix.Symbols) {
		t.Error("Symbol count mismatch after orthogonalization")
	}

	if len(result.OrthogonalMatrix.Factors) != len(matrix.Factors) {
		t.Error("Factor count mismatch after orthogonalization")
	}

	if len(result.OrthogonalMatrix.Data) != len(matrix.Data) {
		t.Error("Data dimension mismatch after orthogonalization")
	}

	// Verify MomentumCore is protected (should remain unchanged)
	momentumCoreIndex := 0 // First factor is MomentumCore
	for i := 0; i < len(matrix.Symbols); i++ {
		originalValue := matrix.Data[i][momentumCoreIndex]
		orthogonalValue := result.OrthogonalMatrix.Data[i][momentumCoreIndex]

		if math.Abs(originalValue-orthogonalValue) > 1e-10 {
			t.Errorf("MomentumCore should be protected: original=%f, orthogonal=%f",
				originalValue, orthogonalValue)
		}
	}

	// Verify correlation matrix is computed
	if len(result.Correlations) != len(matrix.Factors) {
		t.Error("Correlation matrix dimension mismatch")
	}

	// Verify diagonal of correlation matrix is 1.0
	for i := 0; i < len(result.Correlations); i++ {
		if math.Abs(result.Correlations[i][i]-1.0) > 1e-10 {
			t.Errorf("Correlation matrix diagonal should be 1.0, got %f", result.Correlations[i][i])
		}
	}

	// Verify explained variance is computed
	if len(result.ExplainedVariance) != len(matrix.Factors) {
		t.Error("Explained variance dimension mismatch")
	}

	// Check that explained variance sums to approximately 100%
	totalVariance := 0.0
	for _, variance := range result.ExplainedVariance {
		totalVariance += variance
		if variance < 0 {
			t.Errorf("Explained variance should be non-negative, got %f", variance)
		}
	}

	if math.Abs(totalVariance-100.0) > 1e-6 {
		t.Errorf("Explained variance should sum to 100%%, got %f", totalVariance)
	}
}

func TestOrthogonalizationWithMultipleProtectedFactors(t *testing.T) {
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD"},
		Factors: []string{"MomentumCore", "VolumeCore", "TechnicalResidual", "QualityResidual"},
		Data: [][]float64{
			{5.2, 2.8, 1.5, 45.0}, // BTCUSD
			{3.8, 3.2, 2.1, 50.0}, // ETHUSD
		},
	}

	protectedFactors := []string{"MomentumCore", "VolumeCore"}
	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer(protectedFactors)

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Verify both protected factors remain unchanged
	for protectedIndex := 0; protectedIndex < 2; protectedIndex++ {
		for symbolIndex := 0; symbolIndex < len(matrix.Symbols); symbolIndex++ {
			originalValue := matrix.Data[symbolIndex][protectedIndex]
			orthogonalValue := result.OrthogonalMatrix.Data[symbolIndex][protectedIndex]

			if math.Abs(originalValue-orthogonalValue) > 1e-10 {
				t.Errorf("Protected factor %d should remain unchanged: original=%f, orthogonal=%f",
					protectedIndex, originalValue, orthogonalValue)
			}
		}
	}
}

func TestEmptyMatrixHandling(t *testing.T) {
	// Test empty matrix
	emptyMatrix := momentum.FactorMatrix{
		Symbols: []string{},
		Factors: []string{},
		Data:    [][]float64{},
	}

	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer([]string{})

	_, err := orthogonalizer.Orthogonalize(emptyMatrix)
	if err == nil {
		t.Error("Expected error for empty matrix")
	}

	// Check error type
	if orthoErr, ok := err.(*momentum.OrthogonalError); ok {
		if orthoErr.Message != "empty factor matrix" {
			t.Errorf("Expected 'empty factor matrix' error, got '%s'", orthoErr.Message)
		}
	} else {
		t.Error("Expected OrthogonalError type")
	}
}

func TestSingleFactorMatrix(t *testing.T) {
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD"},
		Factors: []string{"MomentumCore"},
		Data: [][]float64{
			{5.2}, // BTCUSD
			{3.8}, // ETHUSD
		},
	}

	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer([]string{"MomentumCore"})

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Single factor orthogonalization failed: %v", err)
	}

	// With single factor, it should remain unchanged
	for i := 0; i < len(matrix.Symbols); i++ {
		originalValue := matrix.Data[i][0]
		orthogonalValue := result.OrthogonalMatrix.Data[i][0]

		if math.Abs(originalValue-orthogonalValue) > 1e-10 {
			t.Errorf("Single factor should remain unchanged: original=%f, orthogonal=%f",
				originalValue, orthogonalValue)
		}
	}
}

func TestOrthogonalizationQuality(t *testing.T) {
	// Create correlated factors
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD"},
		Factors: []string{"MomentumCore", "TechnicalFactor", "VolumeFactor"},
		Data: [][]float64{
			{5.0, 4.5, 2.0}, // BTCUSD - correlated factors
			{4.0, 3.8, 1.8}, // ETHUSD - correlated factors
			{3.0, 2.7, 1.5}, // ADAUSD - correlated factors
			{2.0, 1.9, 1.0}, // SOLUSD - correlated factors
		},
	}

	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer([]string{"MomentumCore"})

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Check that factors are more orthogonal after transformation
	originalCorr := calculateCorrelation(extractColumn(matrix.Data, 0), extractColumn(matrix.Data, 1))
	orthogonalCorr := calculateCorrelation(
		extractColumn(result.OrthogonalMatrix.Data, 0),
		extractColumn(result.OrthogonalMatrix.Data, 1),
	)

	if math.Abs(orthogonalCorr) >= math.Abs(originalCorr) {
		t.Errorf("Orthogonalization should reduce correlation: original=%f, orthogonal=%f",
			originalCorr, orthogonalCorr)
	}
}

func TestProtectedFactorIdentification(t *testing.T) {
	// Test the protected factor functionality through the public Orthogonalize method
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD"},
		Factors: []string{"MomentumCore", "TechnicalResidual", "VolumeResidual"},
		Data: [][]float64{
			{5.2, 4.8, 2.1}, // BTCUSD
			{3.8, 3.0, 1.5}, // ETHUSD
		},
	}

	protectedFactors := []string{"MomentumCore"}
	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer(protectedFactors)

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Verify that MomentumCore (index 0) remains unchanged (protected)
	momentumCoreIndex := 0
	for i := 0; i < len(matrix.Symbols); i++ {
		originalValue := matrix.Data[i][momentumCoreIndex]
		orthogonalValue := result.OrthogonalMatrix.Data[i][momentumCoreIndex]

		if math.Abs(originalValue-orthogonalValue) > 1e-10 {
			t.Errorf("MomentumCore should be protected: original=%f, orthogonal=%f",
				originalValue, orthogonalValue)
		}
	}
}

func TestVectorOperations(t *testing.T) {
	// Test vector operations through the orthogonalization process
	// Create two vectors that we can verify are orthogonalized properly
	matrix := momentum.FactorMatrix{
		Symbols: []string{"BTCUSD", "ETHUSD", "ADAUSD"},
		Factors: []string{"FactorA", "FactorB"},
		Data: [][]float64{
			{1.0, 4.0}, // BTCUSD
			{2.0, 5.0}, // ETHUSD
			{3.0, 6.0}, // ADAUSD
		},
	}

	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer([]string{})

	result, err := orthogonalizer.Orthogonalize(matrix)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Verify that orthogonalization produces reasonable results
	if len(result.OrthogonalMatrix.Data) != len(matrix.Data) {
		t.Error("Orthogonal matrix should have same dimensions as input")
	}

	// Verify correlation matrix was computed
	if len(result.Correlations) != len(matrix.Factors) {
		t.Error("Correlation matrix should match factor count")
	}

	// Verify diagonal correlations are 1.0
	for i := 0; i < len(result.Correlations); i++ {
		if math.Abs(result.Correlations[i][i]-1.0) > 1e-10 {
			t.Errorf("Diagonal correlation should be 1.0, got %f", result.Correlations[i][i])
		}
	}
}

// Helper functions
func extractColumn(matrix [][]float64, col int) []float64 {
	column := make([]float64, len(matrix))
	for i := 0; i < len(matrix); i++ {
		column[i] = matrix[i][col]
	}
	return column
}

func calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0.0
	}

	// Calculate means
	meanX := 0.0
	meanY := 0.0
	for i := 0; i < len(x); i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(len(x))
	meanY /= float64(len(y))

	// Calculate correlation
	numerator := 0.0
	sumXX := 0.0
	sumYY := 0.0

	for i := 0; i < len(x); i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}
