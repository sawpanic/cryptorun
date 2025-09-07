package factors

// Gram-Schmidt orthonormalization scaffold. MomentumCore is protected and not residualized here.

func GramSchmidt(X [][]float64) [][]float64 {
	if len(X) == 0 {
		return nil
	}
	n := len(X)
	m := len(X[0])
	Q := make([][]float64, n)
	for i := range Q {
		Q[i] = make([]float64, m)
	}
	copy(Q[0], X[0])
	normalize(Q[0])
	for k := 1; k < n; k++ {
		// subtract projections on previous vectors
		for j := 0; j < k; j++ {
			coeff := dot(X[k], Q[j])
			for t := 0; t < m; t++ {
				X[k][t] -= coeff * Q[j][t]
			}
		}
		copy(Q[k], X[k])
		normalize(Q[k])
	}
	return Q
}

func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
func normalize(a []float64) {
	s := 0.0
	for _, v := range a {
		s += v * v
	}
	if s == 0 {
		return
	}
	s = 1.0 / sqrt(s)
	for i := range a {
		a[i] *= s
	}
}
func sqrt(x float64) float64 {
	z := x
	for i := 0; i < 12; i++ {
		z -= (z*z - x) / (2 * z)
	}
	if z < 0 {
		return 0
	}
	return z
}
