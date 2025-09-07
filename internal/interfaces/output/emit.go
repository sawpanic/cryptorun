package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/sawpanic/cryptorun/internal/application/signals"
	"github.com/sawpanic/cryptorun/internal/regime"
)

type Emitter struct{}

func NewEmitter() *Emitter {
	return &Emitter{}
}

func (e *Emitter) EmitSignalsCSV(filePath string, results *signals.ScanResults) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header with attribution fields
	header := []string{
		"Symbol", "Score", "Fresh", "Depth", "Venue", "Sources", "LatencyMs",
		"ScoreGate", "VADRGate", "FundingGate", "FreshnessGate", "LateFillGate", "FatigueGate",
		"MomentumCore", "TechnicalResidual", "VolumeResidual", "QualityResidual", "SocialCapped",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write candidates
	for _, candidate := range results.Candidates {
		record := []string{
			candidate.Symbol,
			fmt.Sprintf("%.2f", candidate.Score),
			formatCSVBool(candidate.Attribution.Fresh, "●", "○"),
			formatCSVBool(candidate.Attribution.DepthOK, "✓", "✗"),
			candidate.Attribution.Venue,
			strconv.Itoa(candidate.Attribution.SourceCount),
			strconv.Itoa(candidate.Attribution.LatencyMs),
			formatCSVBool(candidate.Gates.ScoreGate, "PASS", "FAIL"),
			formatCSVBool(candidate.Gates.VADRGate, "PASS", "FAIL"),
			formatCSVBool(candidate.Gates.FundingGate, "PASS", "FAIL"),
			formatCSVBool(candidate.Gates.FreshnessGate, "PASS", "FAIL"),
			formatCSVBool(candidate.Gates.LateFillGate, "PASS", "FAIL"),
			formatCSVBool(candidate.Gates.FatigueGate, "PASS", "FAIL"),
			fmt.Sprintf("%.3f", candidate.Factors.MomentumCore),
			fmt.Sprintf("%.3f", candidate.Factors.Technical),
			fmt.Sprintf("%.3f", candidate.Factors.Volume),
			fmt.Sprintf("%.3f", candidate.Factors.Quality),
			fmt.Sprintf("%.3f", candidate.Factors.Social),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

func (e *Emitter) EmitExplainJSON(filePath string, results *signals.ScanResults) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	// Create explain structure with detailed attribution
	explainData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"timestamp":      results.Timestamp,
			"scan_type":      results.ScanType,
			"universe_size":  results.Universe,
			"candidates":     len(results.Candidates),
			"version":        "v3.2.1",
		},
		"scoring_system": map[string]interface{}{
			"protected_momentum": "MomentumCore never orthogonalized",
			"orthogonalization":  "Technical → Volume → Quality → Social (Gram-Schmidt)",
			"social_cap":         "+10 points maximum, applied outside weight allocation",
			"regime_adaptive":    "Weight profiles switch on 4h cadence",
		},
		"gates": map[string]interface{}{
			"core_requirement": "2 of 3: Score≥75 + VADR≥1.8 + FundingDivergence≥2σ",
			"guard_requirement": "ALL: Freshness≤2bars + LateFill<30s + Fatigue protection",
		},
		"candidates": e.enrichCandidatesWithExplanation(results.Candidates),
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(explainData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func (e *Emitter) EmitRegimeJSON(filePath string, regimeState *regime.State) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create regime JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(regimeState); err != nil {
		return fmt.Errorf("failed to encode regime JSON: %w", err)
	}

	return nil
}

func (e *Emitter) enrichCandidatesWithExplanation(candidates []signals.Candidate) []map[string]interface{} {
	enriched := make([]map[string]interface{}, len(candidates))

	for i, candidate := range candidates {
		enriched[i] = map[string]interface{}{
			"symbol": candidate.Symbol,
			"score":  candidate.Score,
			"attribution": map[string]interface{}{
				"data_freshness": map[string]interface{}{
					"fresh":       candidate.Attribution.Fresh,
					"description": "Data ≤2 bars old and within 1.2×ATR(1h)",
				},
				"liquidity_validation": map[string]interface{}{
					"depth_ok":    candidate.Attribution.DepthOK,
					"venue":       candidate.Attribution.Venue,
					"description": "Exchange-native depth ≥$100k within ±2%",
				},
				"data_sources": map[string]interface{}{
					"count":       candidate.Attribution.SourceCount,
					"latency_ms":  candidate.Attribution.LatencyMs,
					"description": "Number of independent data sources used",
				},
			},
			"gates": map[string]interface{}{
				"core_gates": map[string]interface{}{
					"score":   candidate.Gates.ScoreGate,
					"vadr":    candidate.Gates.VADRGate,
					"funding": candidate.Gates.FundingGate,
					"requirement": "Pass 2 of 3 core gates",
				},
				"guard_gates": map[string]interface{}{
					"freshness": candidate.Gates.FreshnessGate,
					"late_fill": candidate.Gates.LateFillGate,
					"fatigue":   candidate.Gates.FatigueGate,
					"requirement": "Pass ALL guard gates",
				},
			},
			"factor_breakdown": map[string]interface{}{
				"momentum_core": map[string]interface{}{
					"value":       candidate.Factors.MomentumCore,
					"protected":   true,
					"description": "Multi-timeframe momentum, never orthogonalized",
				},
				"residuals": map[string]interface{}{
					"technical": map[string]interface{}{
						"value":       candidate.Factors.Technical,
						"description": "Technical indicators orthogonalized against momentum",
					},
					"volume": map[string]interface{}{
						"value":       candidate.Factors.Volume,
						"description": "Volume metrics orthogonalized against momentum + technical",
					},
					"quality": map[string]interface{}{
						"value":       candidate.Factors.Quality,
						"description": "Quality metrics orthogonalized against prior factors",
					},
					"social": map[string]interface{}{
						"value":       candidate.Factors.Social,
						"capped":      true,
						"description": "Social sentiment, capped at +10 points, applied outside allocation",
					},
				},
			},
		}
	}

	return enriched
}

func formatCSVBool(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}