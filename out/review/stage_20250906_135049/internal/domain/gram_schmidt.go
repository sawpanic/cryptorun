package domain

import "math"

// GramSchmidt returns an orthogonal basis for the input vectors (columns of X)
func GramSchmidt(X [][]float64) [][]float64 {
	if len(X) == 0 { return X }
	m, n := len(X), len(X[0])
	Q := make([][]float64, m)
	for i := range Q { Q[i] = make([]float64, n) }
	for j := 0; j < n; j++ {
		for i := 0; i < m; i++ { Q[i][j] = X[i][j] }
		for k := 0; k < j; k++ {
			var dot, norm float64
			for i := 0; i < m; i++ { dot += X[i][j]*Q[i][k]; norm += Q[i][k]*Q[i][k] }
			if norm > 0 {
				r := dot / norm
				for i := 0; i < m; i++ { Q[i][j] -= r*Q[i][k] }
			}
		}
		// normalize
		var norm float64
		for i := 0; i < m; i++ { norm += Q[i][j]*Q[i][j] }
		norm = math.Sqrt(norm)
		if norm > 0 {
			for i := 0; i < m; i++ { Q[i][j] /= norm }
		}
	}
	return Q
}
