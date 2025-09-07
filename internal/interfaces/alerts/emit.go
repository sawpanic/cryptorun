package alerts

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sawpanic/cryptorun/internal/domain/premove"
)

type Emitter struct{}

func NewEmitter() *Emitter {
	return &Emitter{}
}

func (e *Emitter) EmitAlertsJSON(filePath string, results *premove.DetectionResults) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create alerts JSON file: %w", err)
	}
	defer file.Close()

	// Create alerts structure optimized for action
	alertsData := map[string]interface{}{
		"timestamp": results.Timestamp,
		"alert_summary": map[string]interface{}{
			"total_alerts":        len(results.Candidates),
			"high_priority":       e.countByPriority(results.Candidates, "HIGH"),
			"medium_priority":     e.countByPriority(results.Candidates, "MEDIUM"),
			"low_priority":        e.countByPriority(results.Candidates, "LOW"),
			"avg_score":           results.Summary.AvgScore,
			"dominant_regime":     results.Summary.TopRegime,
		},
		"alerts": e.enrichCandidatesWithAlerts(results.Candidates),
		"gate_analysis": map[string]interface{}{
			"funding_divergence_rate": float64(results.Summary.GateAPassed) / float64(results.Summary.TotalCandidates) * 100,
			"supply_squeeze_rate":     float64(results.Summary.GateBPassed) / float64(results.Summary.TotalCandidates) * 100,
			"whale_accumulation_rate": float64(results.Summary.GateCPassed) / float64(results.Summary.TotalCandidates) * 100,
			"two_of_three_rate":       float64(results.Summary.TwoOfThreePassed) / float64(results.Summary.TotalCandidates) * 100,
		},
		"system_info": map[string]interface{}{
			"version":        "v3.3",
			"detection_type": "2-of-3 Pre-Movement",
			"universe_size":  results.Universe,
		},
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(alertsData); err != nil {
		return fmt.Errorf("failed to encode alerts JSON: %w", err)
	}

	return nil
}

func (e *Emitter) EmitExplainJSON(filePath string, results *premove.DetectionResults) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create explain JSON file: %w", err)
	}
	defer file.Close()

	// Create detailed explanation structure  
	explainData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"timestamp":       results.Timestamp,
			"universe_size":   results.Universe,
			"candidates":      len(results.Candidates),
			"version":         "v3.3",
			"detection_model": "2-of-3 Pre-Movement Detector",
		},
		"gate_system": map[string]interface{}{
			"requirement":   "2 of 3 gates must pass",
			"gate_a":        "Funding Divergence (funding_z < -1.5 AND spot ≥ VWAP)",
			"gate_b":        "Supply Squeeze (reserves_7d ≤ -5% across ≥3 venues OR 2-of-4 proxy)",
			"gate_c":        "Whale Accumulation (2-of-3: large prints, CVD residual, hotwallet decline)",
			"microstructure": "Tiered gates by ADV with VADR precedence: max(p80 24h, tier_min)",
		},
		"scoring_system": map[string]interface{}{
			"structural":   "45 points - Funding divergence + supply dynamics", 
			"behavioral":   "30 points - Large print clustering + CVD analysis",
			"catalyst":     "25 points - Gate combination multiplier",
			"penalties":    "Freshness penalty (-15%), venue health modifier (-10%)",
		},
		"candidates": e.enrichCandidatesWithExplanation(results.Candidates),
		"summary_stats": results.Summary,
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(explainData); err != nil {
		return fmt.Errorf("failed to encode explain JSON: %w", err)
	}

	return nil
}

func (e *Emitter) enrichCandidatesWithAlerts(candidates []premove.Candidate) []map[string]interface{} {
	enriched := make([]map[string]interface{}, len(candidates))

	for i, candidate := range candidates {
		priority := e.calculatePriority(candidate)
		
		enriched[i] = map[string]interface{}{
			"symbol":   candidate.Symbol,
			"score":    candidate.Score,
			"priority": priority,
			"action":   e.determineAction(candidate, priority),
			"gates": map[string]interface{}{
				"a_funding":     candidate.Gates.FundingDivergence,
				"b_supply":      candidate.Gates.SupplySqueeze,
				"c_whale":       candidate.Gates.WhaleAccumulation,
				"total_passed":  candidate.Gates.GatesPassed,
				"micro_tier":    candidate.Gates.MicrostructureTier,
			},
			"key_metrics": map[string]interface{}{
				"funding_z":      candidate.Metrics.FundingZ,
				"supply_change":  candidate.Metrics.SupplyChangeWeek,
				"large_prints":   candidate.Metrics.LargePrintCount,
				"vadr":           candidate.VADR,
			},
			"regime":    candidate.Regime,
			"timestamp": candidate.Symbol, // Would be actual data timestamp
		}
	}

	return enriched
}

func (e *Emitter) enrichCandidatesWithExplanation(candidates []premove.Candidate) []map[string]interface{} {
	enriched := make([]map[string]interface{}, len(candidates))

	for i, candidate := range candidates {
		enriched[i] = map[string]interface{}{
			"symbol": candidate.Symbol,
			"score":  candidate.Score,
			"gate_analysis": map[string]interface{}{
				"funding_divergence": map[string]interface{}{
					"passed":           candidate.Gates.FundingDivergence,
					"funding_z_score":  candidate.Metrics.FundingZ,
					"spot_vwap_ratio":  candidate.Metrics.SpotVWAPRatio,
					"spot_cvd":         candidate.Metrics.SpotCVD,
					"perp_cvd":         candidate.Metrics.PerpCVD,
					"explanation":      e.explainGateA(candidate),
				},
				"supply_squeeze": map[string]interface{}{
					"passed":             candidate.Gates.SupplySqueeze,
					"weekly_change_pct":  candidate.Metrics.SupplyChangeWeek,
					"venue_count":        candidate.Metrics.VenueCount,
					"explanation":        e.explainGateB(candidate),
				},
				"whale_accumulation": map[string]interface{}{
					"passed":             candidate.Gates.WhaleAccumulation,
					"large_prints":       candidate.Metrics.LargePrintCount,
					"cvd_residual":       candidate.Metrics.CVDResidual,
					"price_drift":        candidate.Metrics.PriceDrift,
					"hotwallet_decline":  candidate.Metrics.HotwalletDecline,
					"explanation":        e.explainGateC(candidate),
				},
			},
			"microstructure": map[string]interface{}{
				"tier":        candidate.Gates.MicrostructureTier,
				"vadr":        candidate.VADR,
				"explanation": e.explainMicrostructure(candidate),
			},
			"regime_context": map[string]interface{}{
				"current_regime": candidate.Regime,
				"explanation":    e.explainRegimeContext(candidate),
			},
		}
	}

	return enriched
}

func (e *Emitter) calculatePriority(candidate premove.Candidate) string {
	switch {
	case candidate.Score >= 85 && candidate.Gates.GatesPassed >= 3:
		return "HIGH"
	case candidate.Score >= 75 && candidate.Gates.GatesPassed >= 2:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func (e *Emitter) countByPriority(candidates []premove.Candidate, priority string) int {
	count := 0
	for _, candidate := range candidates {
		if e.calculatePriority(candidate) == priority {
			count++
		}
	}
	return count
}

func (e *Emitter) determineAction(candidate premove.Candidate, priority string) string {
	switch priority {
	case "HIGH":
		return "IMMEDIATE_ALERT"
	case "MEDIUM":
		return "WATCHLIST"
	default:
		return "MONITOR"
	}
}

func (e *Emitter) explainGateA(candidate premove.Candidate) string {
	if candidate.Gates.FundingDivergence {
		return fmt.Sprintf("PASSED: Funding z-score %.2f < -1.5 AND spot/VWAP %.3f ≥ 1.0 with CVD confirmation", 
			candidate.Metrics.FundingZ, candidate.Metrics.SpotVWAPRatio)
	}
	return fmt.Sprintf("FAILED: Funding z-score %.2f or spot/VWAP %.3f criteria not met", 
		candidate.Metrics.FundingZ, candidate.Metrics.SpotVWAPRatio)
}

func (e *Emitter) explainGateB(candidate premove.Candidate) string {
	if candidate.Gates.SupplySqueeze {
		return fmt.Sprintf("PASSED: Supply change %.1f%% with %d venues indicates squeeze", 
			candidate.Metrics.SupplyChangeWeek, candidate.Metrics.VenueCount)
	}
	return fmt.Sprintf("FAILED: Supply change %.1f%% insufficient or venue count %d < 3", 
		candidate.Metrics.SupplyChangeWeek, candidate.Metrics.VenueCount)
}

func (e *Emitter) explainGateC(candidate premove.Candidate) string {
	if candidate.Gates.WhaleAccumulation {
		return fmt.Sprintf("PASSED: Large prints: %d, CVD residual: %.0f, hotwallet decline: %.1f%%", 
			candidate.Metrics.LargePrintCount, candidate.Metrics.CVDResidual, candidate.Metrics.HotwalletDecline)
	}
	return fmt.Sprintf("FAILED: Insufficient whale activity signals (prints: %d, CVD: %.0f)", 
		candidate.Metrics.LargePrintCount, candidate.Metrics.CVDResidual)
}

func (e *Emitter) explainMicrostructure(candidate premove.Candidate) string {
	tierDesc := map[int]string{
		1: "Tier 1 (Best liquidity)",
		2: "Tier 2 (Good liquidity)",
		3: "Tier 3 (Lower liquidity)",
	}
	return fmt.Sprintf("%s with VADR %.1f", tierDesc[candidate.Gates.MicrostructureTier], candidate.VADR)
}

func (e *Emitter) explainRegimeContext(candidate premove.Candidate) string {
	switch candidate.Regime {
	case "TRENDING":
		return "Trending market - momentum favored, relaxed gates"
	case "CHOPPY":
		return "Choppy market - mean reversion focus, standard gates"
	case "HIGH_VOL":
		return "High volatility - quality emphasis, tightened gates"
	default:
		return "Normal market conditions"
	}
}