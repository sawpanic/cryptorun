package domain

import "math"

func CorrelationMatrix(series [][]float64) [][]float64 {
	n := len(series)
	M := make([][]float64, n)
	for i := range M { M[i] = make([]float64, n) }
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			c := corr(series[i], series[j])
			M[i][j], M[j][i] = c, c
		}
	}
	return M
}

func corr(a, b []float64) float64 {
	n := min(len(a), len(b))
	if n == 0 { return 0 }
	var ma, mb float64
	for i := 0; i < n; i++ { ma += a[i]; mb += b[i] }
	ma /= float64(n); mb /= float64(n)
	var num, da, db float64
	for i := 0; i < n; i++ {
		x := a[i]-ma; y := b[i]-mb
		num += x*y; da += x*x; db += y*y
	}
	den := math.Sqrt(da*db)
	if den == 0 { return 0 }
	return num/den
}

func min(a,b int) int { if a<b {return a}; return b }
