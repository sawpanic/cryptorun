package domain

// RegimeDetector basic placeholder based on thresholds
func RegimeDetector(vol7d, pctAboveMA, breadth float64) string {
	if vol7d < 0.3 && pctAboveMA > 0.6 && breadth > 0.4 { return "trending_bull" }
	if vol7d > 0.5 { return "high_volatility" }
	return "choppy"
}
