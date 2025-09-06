package cvd

import (
	"context"
	"fmt"
	"math"

	"cryptorun/src/domain/premove/ports"
)

// Calculator implements CVDResiduals with robust regression
type Calculator struct {
	minSamples int
	minR2      float64
	maxIter    int
}

// NewCVDResiduals creates a new CVD residuals calculator
func NewCVDResiduals() *Calculator {
	return &Calculator{
		minSamples: 200,  // Minimum samples required for robust regression
		minR2:      0.30, // Minimum R² threshold for valid regression
		maxIter:    3,    // Maximum iterations for robust regression
	}
}

// Residualize implements the legacy interface for backward compatibility
func (c *Calculator) Residualize(cvdNorm, volNorm []float64) (residuals []float64, r2 float64, ok bool) {
	if len(cvdNorm) != len(volNorm) || len(cvdNorm) < c.minSamples {
		return cvdNorm, 0.0, false
	}

	// Winsorize both series at ±3σ
	cvdWin := winsorize3Sigma(cvdNorm)
	volWin := winsorize3Sigma(volNorm)

	if len(cvdWin) != len(volWin) || len(cvdWin) < c.minSamples {
		return cvdNorm, 0.0, false
	}

	// Robust regression using iterative reweighted least squares (IRLS)
	beta, r2Val := robustRegression(cvdWin, volWin)
	if r2Val < c.minR2 {
		return cvdNorm, r2Val, false
	}

	// Compute residuals: cvd_norm - β*vol_norm
	residuals = make([]float64, len(cvdNorm))
	for i := range cvdNorm {
		residuals[i] = cvdNorm[i] - beta*volNorm[i]
	}

	return residuals, r2Val, true
}

// CalculateResiduals computes residuals with robust regression
func (c *Calculator) CalculateResiduals(ctx context.Context, cvdNorm, volNorm []float64) ([]ports.CVDPoint, error) {
	if len(cvdNorm) != len(volNorm) {
		return nil, fmt.Errorf("cvdNorm and volNorm must have same length: %d vs %d", len(cvdNorm), len(volNorm))
	}

	if len(cvdNorm) == 0 {
		return []ports.CVDPoint{}, nil
	}

	var results []ports.CVDPoint

	// Process in rolling windows for time-varying residuals
	windowSize := c.minSamples
	for i := windowSize - 1; i < len(cvdNorm); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		start := i - windowSize + 1
		windowCVD := cvdNorm[start : i+1]
		windowVol := volNorm[start : i+1]

		point := ports.CVDPoint{
			IsValid: len(windowCVD) >= c.minSamples,
		}

		if point.IsValid {
			// Winsorize both series
			cvdWin := winsorize3Sigma(windowCVD)
			volWin := winsorize3Sigma(windowVol)

			if len(cvdWin) >= c.minSamples && len(volWin) >= c.minSamples {
				beta, r2 := robustRegression(cvdWin, volWin)
				if r2 >= c.minR2 {
					point.Beta = beta
					point.R2 = r2
					point.Residual = cvdNorm[i] - beta*volNorm[i]
				} else {
					point.IsValid = false
					point.Residual = cvdNorm[i] // Fallback to raw CVD
				}
			} else {
				point.IsValid = false
				point.Residual = cvdNorm[i] // Fallback to raw CVD
			}
		} else {
			point.Residual = cvdNorm[i] // Fallback to raw CVD
		}

		results = append(results, point)
	}

	return results, nil
}

func winsorize3Sigma(values []float64) []float64 {
	if len(values) < 3 {
		return append([]float64(nil), values...)
	}

	// Calculate mean and std dev
	var sum, sumSq float64
	validCount := 0
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			sum += v
			sumSq += v * v
			validCount++
		}
	}

	if validCount < 3 {
		return filterValidValues(values)
	}

	mean := sum / float64(validCount)
	variance := (sumSq - sum*sum/float64(validCount)) / float64(validCount-1)
	if variance <= 0 {
		return filterValidValues(values)
	}

	stdDev := math.Sqrt(variance)
	lowerBound := mean - 3*stdDev
	upperBound := mean + 3*stdDev

	// Winsorize outliers
	result := make([]float64, 0, len(values))
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if v < lowerBound {
			result = append(result, lowerBound)
		} else if v > upperBound {
			result = append(result, upperBound)
		} else {
			result = append(result, v)
		}
	}

	return result
}

func filterValidValues(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			result = append(result, v)
		}
	}
	return result
}

func robustRegression(y, x []float64) (beta, r2 float64) {
	if len(y) != len(x) || len(y) < 2 {
		return 0.0, 0.0
	}

	n := len(y)

	// Initial OLS estimate
	var sumX, sumY, sumXY, sumXX, sumYY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumXX += x[i] * x[i]
		sumYY += y[i] * y[i]
	}

	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	// Calculate beta
	numerator := sumXY - float64(n)*meanX*meanY
	denominator := sumXX - float64(n)*meanX*meanX

	if math.Abs(denominator) < 1e-10 {
		return 0.0, 0.0
	}

	beta = numerator / denominator

	// Simple IRLS with Huber weights (3 iterations)
	for iter := 0; iter < 3; iter++ {
		// Calculate residuals and weights
		weights := make([]float64, n)
		var sumWeights, sumWX, sumWY, sumWXY, sumWXX float64

		for i := 0; i < n; i++ {
			resid := y[i] - beta*x[i]
			absResid := math.Abs(resid)

			// Huber weight function (k=1.345)
			if absResid <= 1.345 {
				weights[i] = 1.0
			} else {
				weights[i] = 1.345 / absResid
			}

			sumWeights += weights[i]
			sumWX += weights[i] * x[i]
			sumWY += weights[i] * y[i]
			sumWXY += weights[i] * x[i] * y[i]
			sumWXX += weights[i] * x[i] * x[i]
		}

		if sumWeights < 1e-10 {
			break
		}

		// Weighted means
		wMeanX := sumWX / sumWeights
		wMeanY := sumWY / sumWeights

		// Update beta
		wNumerator := sumWXY - sumWeights*wMeanX*wMeanY
		wDenominator := sumWXX - sumWeights*wMeanX*wMeanX

		if math.Abs(wDenominator) < 1e-10 {
			break
		}

		beta = wNumerator / wDenominator
	}

	// Calculate R²
	var ssRes, ssTot float64
	for i := 0; i < n; i++ {
		predicted := beta * x[i]
		ssRes += (y[i] - predicted) * (y[i] - predicted)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}

	if ssTot < 1e-10 {
		return beta, 0.0
	}

	r2 = 1.0 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0.0
	}

	return beta, r2
}
