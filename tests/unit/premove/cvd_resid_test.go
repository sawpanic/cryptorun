package premove

import (
	"context"
	"math"
	"testing"

	"github.com/sawpanic/cryptorun/src/domain/premove/cvd"
)

func TestCVDResiduals_KnownBeta(t *testing.T) {
	residualizer := cvd.NewCVDResiduals()

	// Synthetic data with known β = 0.5
	n := 300
	cvdNorm := make([]float64, n)
	volNorm := make([]float64, n)

	// Generate y = 0.5*x + noise
	for i := 0; i < n; i++ {
		x := float64(i + 1)
		volNorm[i] = x
		cvdNorm[i] = 0.5*x + 0.1*float64(i%10-5) // Small noise
	}

	residuals, r2, ok := residualizer.Residualize(cvdNorm, volNorm)

	if !ok {
		t.Fatal("Residualization should succeed with known data")
	}

	if r2 < 0.3 {
		t.Errorf("R² should be >= 0.3, got %.3f", r2)
	}

	if len(residuals) != len(cvdNorm) {
		t.Errorf("Residuals length mismatch: expected %d, got %d", len(cvdNorm), len(residuals))
	}

	// Check residual mean ≈ 0
	var mean float64
	for _, r := range residuals {
		mean += r
	}
	mean /= float64(len(residuals))

	if math.Abs(mean) > 0.5 {
		t.Errorf("Residual mean should be ≈0, got %.3f", mean)
	}
}

func TestCVDResiduals_ShortSeries(t *testing.T) {
	residualizer := cvd.NewCVDResiduals()

	// Short series < 200 samples
	cvdNorm := make([]float64, 150)
	volNorm := make([]float64, 150)

	for i := 0; i < 150; i++ {
		cvdNorm[i] = float64(i)
		volNorm[i] = float64(i) * 2
	}

	residuals, r2, ok := residualizer.Residualize(cvdNorm, volNorm)

	if ok {
		t.Error("Should fail with insufficient samples (<200)")
	}

	// Should return original cvdNorm as fallback
	if len(residuals) != len(cvdNorm) {
		t.Errorf("Fallback residuals length mismatch")
	}

	if r2 != 0.0 {
		t.Errorf("R² should be 0.0 for fallback, got %.3f", r2)
	}
}

func TestCVDResiduals_LowR2Fallback(t *testing.T) {
	residualizer := cvd.NewCVDResiduals()

	// Random uncorrelated data (R² < 0.3)
	n := 300
	cvdNorm := make([]float64, n)
	volNorm := make([]float64, n)

	for i := 0; i < n; i++ {
		cvdNorm[i] = float64((i*7 + 13) % 100) // Pseudo-random
		volNorm[i] = float64((i*11 + 17) % 100)
	}

	residuals, r2, ok := residualizer.Residualize(cvdNorm, volNorm)

	if ok && r2 < 0.3 {
		t.Error("Should fail when R² < 0.3")
	}

	// Should return original cvdNorm as fallback if ok=false
	if !ok && len(residuals) != len(cvdNorm) {
		t.Errorf("Fallback residuals length mismatch")
	}
}

func TestCVDResiduals_MismatchedLengths(t *testing.T) {
	residualizer := cvd.NewCVDResiduals()

	cvdNorm := make([]float64, 300)
	volNorm := make([]float64, 250) // Different length

	residuals, r2, ok := residualizer.Residualize(cvdNorm, volNorm)

	if ok {
		t.Error("Should fail with mismatched input lengths")
	}

	if r2 != 0.0 {
		t.Errorf("R² should be 0.0 for failed case, got %.3f", r2)
	}

	if len(residuals) != len(cvdNorm) {
		t.Errorf("Should return cvdNorm as fallback")
	}
}

func TestCVDResiduals_CalculateResiduals(t *testing.T) {
	calc := cvd.NewCVDResiduals()

	// Generate sufficient test data
	n := 250
	cvdNorm := make([]float64, n)
	volNorm := make([]float64, n)

	for i := 0; i < n; i++ {
		volNorm[i] = float64(i + 1)
		cvdNorm[i] = 1.5*volNorm[i] + 0.05*float64(i%5)
	}

	ctx := context.Background()
	results, err := calc.CalculateResiduals(ctx, cvdNorm, volNorm)

	if err != nil {
		t.Fatalf("CalculateResiduals failed: %v", err)
	}

	// Should have results for each rolling window starting at minSamples-1
	expectedResults := n - 199 // minSamples is 200, so skip first 199
	if len(results) != expectedResults {
		t.Errorf("Expected %d results, got %d", expectedResults, len(results))
	}

	// Check results are reasonable
	validCount := 0
	for _, result := range results {
		if result.IsValid {
			validCount++
			if result.R2 < 0.0 || result.R2 > 1.0 {
				t.Errorf("Invalid R² value: %f", result.R2)
			}
		}
	}

	if validCount == 0 {
		t.Error("Expected at least some valid results")
	}
}
