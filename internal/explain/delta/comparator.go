package delta

import (
	"fmt"
	"math"
	"sort"

	"github.com/rs/zerolog/log"
)

// Comparator performs delta analysis between baseline and current factors
type Comparator struct{}

// NewComparator creates a new delta comparator
func NewComparator() *Comparator {
	return &Comparator{}
}

// Compare performs comprehensive delta analysis
func (c *Comparator) Compare(baseline *BaselineSnapshot, current map[string]*AssetFactors, regime string, tolerance *ToleranceConfig) (*Results, error) {
	log.Info().
		Int("baseline_assets", baseline.AssetCount).
		Int("current_assets", len(current)).
		Str("regime", regime).
		Msg("Starting delta comparison")

	results := &Results{
		TotalAssets:    0,
		FailCount:      0,
		WarnCount:      0,
		OKCount:        0,
		Assets:         make([]*AssetDelta, 0),
		WorstOffenders: make([]*WorstOffender, 0),
	}

	// Get regime tolerance settings
	regimeTolerance, exists := tolerance.Regimes[regime]
	if !exists {
		return nil, fmt.Errorf("no tolerance configuration for regime: %s", regime)
	}

	// Process each asset in current dataset
	for symbol, currentFactors := range current {
		results.TotalAssets++

		assetDelta := c.compareAsset(symbol, baseline, currentFactors, regimeTolerance)
		results.Assets = append(results.Assets, assetDelta)

		// Update status counts
		switch assetDelta.Status {
		case "FAIL":
			results.FailCount++
		case "WARN":
			results.WarnCount++
		case "OK":
			results.OKCount++
		}

		// Collect worst violations for summary
		if assetDelta.WorstViolation != nil {
			results.WorstOffenders = append(results.WorstOffenders, assetDelta.WorstViolation)
		}
	}

	// Sort worst offenders by severity and delta magnitude
	c.sortWorstOffenders(results.WorstOffenders)

	// Limit to top 10 worst offenders for reporting
	if len(results.WorstOffenders) > 10 {
		results.WorstOffenders = results.WorstOffenders[:10]
	}

	log.Info().
		Int("total", results.TotalAssets).
		Int("fail", results.FailCount).
		Int("warn", results.WarnCount).
		Int("ok", results.OKCount).
		Int("worst_offenders", len(results.WorstOffenders)).
		Msg("Delta comparison completed")

	return results, nil
}

// compareAsset compares a single asset between baseline and current
func (c *Comparator) compareAsset(symbol string, baseline *BaselineSnapshot, current *AssetFactors, regimeTolerance *RegimeTolerance) *AssetDelta {
	assetDelta := &AssetDelta{
		Symbol:          symbol,
		Regime:          current.Regime,
		Status:          "OK",
		BaselineFactors: make(map[string]float64),
		CurrentFactors:  make(map[string]float64),
		Deltas:          make(map[string]float64),
		ToleranceCheck:  make(map[string]*ToleranceCheck),
	}

	// Get baseline factors for this asset (or use zeros if not found)
	var baselineFactors *AssetFactors
	if baseline.Factors != nil {
		baselineFactors = baseline.Factors[symbol]
	}
	if baselineFactors == nil {
		// Asset not in baseline, use zeros
		baselineFactors = &AssetFactors{
			Symbol:         symbol,
			Regime:         baseline.Regime,
			MomentumCore:   0.0,
			TechnicalResid: 0.0,
			VolumeResid:    0.0,
			QualityResid:   0.0,
			SocialResid:    0.0,
			CompositeScore: 0.0,
			Gates:          make(map[string]bool),
		}
	}

	// Extract factor values for comparison
	factorPairs := map[string]struct{ baseline, current float64 }{
		"momentum_core":   {baselineFactors.MomentumCore, current.MomentumCore},
		"technical_resid": {baselineFactors.TechnicalResid, current.TechnicalResid},
		"volume_resid":    {baselineFactors.VolumeResid, current.VolumeResid},
		"quality_resid":   {baselineFactors.QualityResid, current.QualityResid},
		"social_resid":    {baselineFactors.SocialResid, current.SocialResid},
		"composite_score": {baselineFactors.CompositeScore, current.CompositeScore},
	}

	worstViolation := &WorstOffender{Symbol: symbol, Delta: 0.0}

	// Compare each factor
	for factorName, values := range factorPairs {
		baseline := values.baseline
		current := values.current
		delta := current - baseline

		// Store values
		assetDelta.BaselineFactors[factorName] = baseline
		assetDelta.CurrentFactors[factorName] = current
		assetDelta.Deltas[factorName] = delta

		// Get tolerance settings for this factor
		factorTolerance, exists := regimeTolerance.FactorTolerances[factorName]
		if !exists {
			// No tolerance configured, mark as OK
			assetDelta.ToleranceCheck[factorName] = &ToleranceCheck{
				Factor:    factorName,
				Delta:     delta,
				Tolerance: 0.0,
				Exceeded:  false,
				Severity:  "OK",
			}
			continue
		}

		// Check tolerance violation
		tolerance := c.checkTolerance(delta, factorTolerance)
		assetDelta.ToleranceCheck[factorName] = tolerance

		// Update asset status based on worst violation
		if tolerance.Severity == "FAIL" && assetDelta.Status != "FAIL" {
			assetDelta.Status = "FAIL"
		} else if tolerance.Severity == "WARN" && assetDelta.Status == "OK" {
			assetDelta.Status = "WARN"
		}

		// Track worst violation for this asset
		if tolerance.Exceeded && math.Abs(delta) > math.Abs(worstViolation.Delta) {
			worstViolation.Factor = factorName
			worstViolation.Delta = delta
			worstViolation.Tolerance = tolerance.Tolerance
			worstViolation.Severity = tolerance.Severity
			worstViolation.Hint = c.generateHint(factorName, delta, tolerance.Severity)
		}
	}

	// Set worst violation if any
	if math.Abs(worstViolation.Delta) > 0 {
		assetDelta.WorstViolation = worstViolation
	}

	return assetDelta
}

// checkTolerance validates a delta against tolerance settings
func (c *Comparator) checkTolerance(delta float64, tolerance *FactorTolerance) *ToleranceCheck {
	absDelta := math.Abs(delta)

	check := &ToleranceCheck{
		Factor:    tolerance.Factor,
		Delta:     delta,
		Tolerance: tolerance.FailAt, // Use fail threshold as primary tolerance
		Exceeded:  false,
		Severity:  "OK",
	}

	// Check direction constraints
	switch tolerance.Direction {
	case "positive":
		if delta < 0 {
			// Negative delta is not a concern for positive-only tolerance
			return check
		}
	case "negative":
		if delta > 0 {
			// Positive delta is not a concern for negative-only tolerance
			return check
		}
	case "both":
		// Both directions are checked (default behavior)
	}

	// Check failure threshold
	if absDelta >= tolerance.FailAt {
		check.Exceeded = true
		check.Severity = "FAIL"
		check.Tolerance = tolerance.FailAt
		return check
	}

	// Check warning threshold
	if absDelta >= tolerance.WarnAt {
		check.Exceeded = true
		check.Severity = "WARN"
		check.Tolerance = tolerance.WarnAt
		return check
	}

	// Within acceptable range
	return check
}

// generateHint creates a human-readable hint for violations
func (c *Comparator) generateHint(factor string, delta float64, severity string) string {
	direction := "increased"
	if delta < 0 {
		direction = "decreased"
	}

	switch factor {
	case "momentum_core":
		return fmt.Sprintf("momentum strength %s significantly", direction)
	case "technical_resid":
		return fmt.Sprintf("technical indicators %s beyond normal range", direction)
	case "volume_resid":
		return fmt.Sprintf("volume patterns %s substantially", direction)
	case "quality_resid":
		return fmt.Sprintf("quality metrics %s unexpectedly", direction)
	case "social_resid":
		return fmt.Sprintf("social sentiment %s dramatically", direction)
	case "composite_score":
		return fmt.Sprintf("overall score %s beyond expectations", direction)
	default:
		return fmt.Sprintf("%s %s significantly", factor, direction)
	}
}

// sortWorstOffenders sorts violations by severity then absolute delta
func (c *Comparator) sortWorstOffenders(offenders []*WorstOffender) {
	sort.Slice(offenders, func(i, j int) bool {
		// First sort by severity (FAIL > WARN > OK)
		severityOrder := map[string]int{"FAIL": 3, "WARN": 2, "OK": 1}

		severityI := severityOrder[offenders[i].Severity]
		severityJ := severityOrder[offenders[j].Severity]

		if severityI != severityJ {
			return severityI > severityJ // Higher severity first
		}

		// Then by absolute delta magnitude (larger violations first)
		return math.Abs(offenders[i].Delta) > math.Abs(offenders[j].Delta)
	})
}
