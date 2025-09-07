package momentum

import (
	"math"
)

// FactorMatrix represents a matrix of factor values [symbols x factors]
type FactorMatrix struct {
	Symbols []string    `json:"symbols"`
	Factors []string    `json:"factors"`
	Data    [][]float64 `json:"data"` // [symbol][factor]
}

// OrthogonalResult contains orthogonalization results
type OrthogonalResult struct {
	OriginalMatrix    FactorMatrix `json:"original_matrix"`
	OrthogonalMatrix  FactorMatrix `json:"orthogonal_matrix"`
	ProtectedFactors  []string     `json:"protected_factors"`
	Correlations      [][]float64  `json:"correlations"`
	ExplainedVariance []float64    `json:"explained_variance"`
}

// GramSchmidtOrthogonalizer implements Gram-Schmidt orthogonalization with MomentumCore protection
type GramSchmidtOrthogonalizer struct {
	protectedFactors []string
}

// NewGramSchmidtOrthogonalizer creates a new orthogonalizer with protected factors
func NewGramSchmidtOrthogonalizer(protectedFactors []string) *GramSchmidtOrthogonalizer {
	return &GramSchmidtOrthogonalizer{
		protectedFactors: protectedFactors,
	}
}

// Orthogonalize applies Gram-Schmidt orthogonalization while protecting MomentumCore
func (gso *GramSchmidtOrthogonalizer) Orthogonalize(matrix FactorMatrix) (*OrthogonalResult, error) {
	if len(matrix.Data) == 0 || len(matrix.Data[0]) == 0 {
		return nil, &OrthogonalError{Message: "empty factor matrix"}
	}

	numSymbols := len(matrix.Data)
	numFactors := len(matrix.Factors)

	// Create result structure
	result := &OrthogonalResult{
		OriginalMatrix:    matrix,
		ProtectedFactors:  gso.protectedFactors,
		Correlations:      make([][]float64, numFactors),
		ExplainedVariance: make([]float64, numFactors),
	}

	// Initialize correlation matrix
	for i := 0; i < numFactors; i++ {
		result.Correlations[i] = make([]float64, numFactors)
	}

	// Copy original matrix for orthogonalization
	orthogonalData := make([][]float64, numSymbols)
	for i := 0; i < numSymbols; i++ {
		orthogonalData[i] = make([]float64, numFactors)
		copy(orthogonalData[i], matrix.Data[i])
	}

	// Calculate original correlations
	gso.calculateCorrelations(matrix.Data, result.Correlations)

	// Apply Gram-Schmidt with protection for MomentumCore
	err := gso.gramSchmidtWithProtection(orthogonalData, matrix.Factors, result)
	if err != nil {
		return nil, err
	}

	// Create orthogonal matrix
	result.OrthogonalMatrix = FactorMatrix{
		Symbols: matrix.Symbols,
		Factors: matrix.Factors,
		Data:    orthogonalData,
	}

	// Calculate explained variance
	gso.calculateExplainedVariance(orthogonalData, result.ExplainedVariance)

	return result, nil
}

// gramSchmidtWithProtection applies Gram-Schmidt while protecting specified factors
func (gso *GramSchmidtOrthogonalizer) gramSchmidtWithProtection(data [][]float64, factors []string, result *OrthogonalResult) error {
	numSymbols := len(data)
	numFactors := len(factors)

	// Create working vectors for each factor
	vectors := make([][]float64, numFactors)
	for i := 0; i < numFactors; i++ {
		vectors[i] = make([]float64, numSymbols)
		for j := 0; j < numSymbols; j++ {
			vectors[i][j] = data[j][i]
		}
	}

	// Identify protected factor indices
	protectedIndices := gso.getProtectedIndices(factors)

	// Apply Gram-Schmidt process
	for i := 0; i < numFactors; i++ {
		// Skip orthogonalization for protected factors (keep them as-is)
		if gso.isProtected(i, protectedIndices) {
			continue
		}

		// Orthogonalize against all previous vectors (including protected ones)
		for j := 0; j < i; j++ {
			projection := gso.project(vectors[i], vectors[j])
			for k := 0; k < numSymbols; k++ {
				vectors[i][k] -= projection[k]
			}
		}

		// Normalize the vector
		norm := gso.norm(vectors[i])
		if norm > 0 {
			for k := 0; k < numSymbols; k++ {
				vectors[i][k] /= norm
			}
		}
	}

	// Copy orthogonalized vectors back to data matrix
	for i := 0; i < numFactors; i++ {
		for j := 0; j < numSymbols; j++ {
			data[j][i] = vectors[i][j]
		}
	}

	return nil
}

// getProtectedIndices returns indices of protected factors
func (gso *GramSchmidtOrthogonalizer) getProtectedIndices(factors []string) map[int]bool {
	indices := make(map[int]bool)

	for i, factor := range factors {
		for _, protected := range gso.protectedFactors {
			if factor == protected || factor == "MomentumCore" {
				indices[i] = true
				break
			}
		}
	}

	return indices
}

// isProtected checks if a factor index is protected
func (gso *GramSchmidtOrthogonalizer) isProtected(index int, protectedIndices map[int]bool) bool {
	return protectedIndices[index]
}

// project calculates vector projection of u onto v
func (gso *GramSchmidtOrthogonalizer) project(u, v []float64) []float64 {
	if len(u) != len(v) {
		return make([]float64, len(u))
	}

	dotProduct := gso.dotProduct(u, v)
	vNormSquared := gso.dotProduct(v, v)

	if vNormSquared == 0 {
		return make([]float64, len(u))
	}

	scalar := dotProduct / vNormSquared
	projection := make([]float64, len(u))

	for i := 0; i < len(u); i++ {
		projection[i] = scalar * v[i]
	}

	return projection
}

// dotProduct calculates dot product of two vectors
func (gso *GramSchmidtOrthogonalizer) dotProduct(u, v []float64) float64 {
	if len(u) != len(v) {
		return 0
	}

	sum := 0.0
	for i := 0; i < len(u); i++ {
		sum += u[i] * v[i]
	}
	return sum
}

// norm calculates Euclidean norm of a vector
func (gso *GramSchmidtOrthogonalizer) norm(v []float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}

// calculateCorrelations calculates correlation matrix between factors
func (gso *GramSchmidtOrthogonalizer) calculateCorrelations(data [][]float64, correlations [][]float64) {
	numSymbols := len(data)
	numFactors := len(data[0])

	// Calculate means
	means := make([]float64, numFactors)
	for j := 0; j < numFactors; j++ {
		sum := 0.0
		for i := 0; i < numSymbols; i++ {
			sum += data[i][j]
		}
		means[j] = sum / float64(numSymbols)
	}

	// Calculate correlations
	for i := 0; i < numFactors; i++ {
		for j := 0; j < numFactors; j++ {
			if i == j {
				correlations[i][j] = 1.0
				continue
			}

			// Calculate covariance
			covariance := 0.0
			varianceI := 0.0
			varianceJ := 0.0

			for k := 0; k < numSymbols; k++ {
				deviationI := data[k][i] - means[i]
				deviationJ := data[k][j] - means[j]

				covariance += deviationI * deviationJ
				varianceI += deviationI * deviationI
				varianceJ += deviationJ * deviationJ
			}

			// Calculate correlation coefficient
			if varianceI > 0 && varianceJ > 0 {
				correlations[i][j] = covariance / math.Sqrt(varianceI*varianceJ)
			} else {
				correlations[i][j] = 0.0
			}
		}
	}
}

// calculateExplainedVariance calculates explained variance for each factor
func (gso *GramSchmidtOrthogonalizer) calculateExplainedVariance(data [][]float64, variance []float64) {
	numSymbols := len(data)
	numFactors := len(data[0])

	totalVariance := 0.0

	// Calculate variance for each factor
	for j := 0; j < numFactors; j++ {
		// Calculate mean
		mean := 0.0
		for i := 0; i < numSymbols; i++ {
			mean += data[i][j]
		}
		mean /= float64(numSymbols)

		// Calculate variance
		factorVariance := 0.0
		for i := 0; i < numSymbols; i++ {
			deviation := data[i][j] - mean
			factorVariance += deviation * deviation
		}
		factorVariance /= float64(numSymbols)

		variance[j] = factorVariance
		totalVariance += factorVariance
	}

	// Convert to explained variance percentages
	if totalVariance > 0 {
		for j := 0; j < numFactors; j++ {
			variance[j] = (variance[j] / totalVariance) * 100.0
		}
	}
}

// OrthogonalError represents orthogonalization errors
type OrthogonalError struct {
	Message string
}

func (e *OrthogonalError) Error() string {
	return "orthogonalization error: " + e.Message
}
