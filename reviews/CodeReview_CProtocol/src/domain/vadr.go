package domain

// VADR: Volume Adjusted Depth Ratio placeholder, requires 20 bars minimum.
func VADR(volumes []float64) (float64, bool) {
	if len(volumes) < 20 { return 0, false }
	var sum, mean float64
	for _, v := range volumes { sum += v }
	mean = sum / float64(len(volumes))
	if mean == 0 { return 0, false }
	return volumes[len(volumes)-1] / mean, true
}
