package ports

import "context"

// CVDPoint represents a single CVD residual calculation
type CVDPoint struct {
	Residual float64
	R2       float64
	Beta     float64
	IsValid  bool
}

type CVDResiduals interface {
	// Inputs: paired time series of signed dollar flow (cvd_norm) and dollar volume (vol_norm).
	// Returns residuals = cvd_norm - β*vol_norm, and R2 of the robust fit.
	Residualize(cvdNorm, volNorm []float64) (residuals []float64, r2 float64, ok bool)

	// CalculateResiduals computes residuals with robust regression, fallback if len<200 or R²<0.30
	CalculateResiduals(ctx context.Context, cvdNorm, volNorm []float64) ([]CVDPoint, error)
}
