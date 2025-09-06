package regime

// Return fixed weights per PRD ranges (approx midpoints)
func Weights(reg string) []float64 {
	switch reg {
	case "trending_bull":
		return []float64{0.43, 0.20, 0.18, 0.11, 0.08}
	case "high_volatility":
		return []float64{0.33, 0.22, 0.20, 0.15, 0.10}
	default: // choppy
		return []float64{0.28, 0.25, 0.20, 0.17, 0.10}
	}
}
