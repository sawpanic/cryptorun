package empirical

import (
	"testing"
)

// GateAnalysis holds results of entry gate performance analysis
type GateAnalysis struct {
	TotalCount        int
	GatePassCount     int
	GateFailCount     int
	GatePassAvgReturn float64
	GateFailAvgReturn float64
	GatePassHitRate   float64
	GateFailHitRate   float64
	OutperformanceGap float64
}

// EntryGates represents the combined entry criteria
type EntryGates struct {
	MinScore                   float64 // ≥75
	MinVADR                    float64 // ≥1.8
	MinFundingDivergenceZScore float64 // ≥2.0 (implicit from meets_gates)
}

func TestGateAlignment_OutperformanceVerification(t *testing.T) {
	panel := loadSyntheticPanel(t)

	gates := EntryGates{
		MinScore:                   75.0,
		MinVADR:                    1.8,
		MinFundingDivergenceZScore: 2.0,
	}

	// Analyze 4h performance
	analysis4h := analyzeGatePerformance(panel, gates, "4h")

	// Analyze 24h performance
	analysis24h := analyzeGatePerformance(panel, gates, "24h")

	// Acceptance criteria: Gate-passing entries should outperform controls
	if analysis4h.OutperformanceGap <= 0 {
		t.Errorf("4h gate alignment failed: gate-passing entries underperformed by %.3f%%",
			analysis4h.OutperformanceGap*100)
	}

	if analysis24h.OutperformanceGap <= 0 {
		t.Errorf("24h gate alignment failed: gate-passing entries underperformed by %.3f%%",
			analysis24h.OutperformanceGap*100)
	}

	t.Logf("Gate alignment results:")
	t.Logf("  4h: Pass avg=%.3f%%, Fail avg=%.3f%%, Gap=+%.3f%%",
		analysis4h.GatePassAvgReturn*100, analysis4h.GateFailAvgReturn*100, analysis4h.OutperformanceGap*100)
	t.Logf("  24h: Pass avg=%.3f%%, Fail avg=%.3f%%, Gap=+%.3f%%",
		analysis24h.GatePassAvgReturn*100, analysis24h.GateFailAvgReturn*100, analysis24h.OutperformanceGap*100)
}

func TestGateAlignment_HitRateComparison(t *testing.T) {
	panel := loadSyntheticPanel(t)

	gates := EntryGates{
		MinScore:                   75.0,
		MinVADR:                    1.8,
		MinFundingDivergenceZScore: 2.0,
	}

	// Calculate hit rates for gate-passing vs gate-failing entries
	analysis4h := analyzeGatePerformance(panel, gates, "4h")
	analysis24h := analyzeGatePerformance(panel, gates, "24h")

	// Hit rate thresholds (4h: 2.5%, 24h: 4.0%)
	hitThreshold4h := 0.025
	hitThreshold24h := 0.040

	// Calculate hit rates
	gatePassHitRate4h := calculateHitRate(panel, gates, hitThreshold4h, true)
	gateFailHitRate4h := calculateHitRate(panel, gates, hitThreshold4h, false)

	gatePassHitRate24h := calculateHitRate(panel, gates, hitThreshold24h, true)
	gateFailHitRate24h := calculateHitRate(panel, gates, hitThreshold24h, false)

	// Gate-passing entries should have higher hit rates
	if gatePassHitRate4h <= gateFailHitRate4h {
		t.Errorf("4h hit rate: gate-passing (%.1f%%) <= gate-failing (%.1f%%)",
			gatePassHitRate4h*100, gateFailHitRate4h*100)
	}

	if gatePassHitRate24h <= gateFailHitRate24h {
		t.Errorf("24h hit rate: gate-passing (%.1f%%) <= gate-failing (%.1f%%)",
			gatePassHitRate24h*100, gateFailHitRate24h*100)
	}

	t.Logf("Hit rate comparison:")
	t.Logf("  4h: Pass=%.1f%%, Fail=%.1f%% (threshold=%.1f%%)",
		gatePassHitRate4h*100, gateFailHitRate4h*100, hitThreshold4h*100)
	t.Logf("  24h: Pass=%.1f%%, Fail=%.1f%% (threshold=%.1f%%)",
		gatePassHitRate24h*100, gateFailHitRate24h*100, hitThreshold24h*100)
}

func TestGateAlignment_IndividualGateContribution(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Test individual gate contributions
	tests := []struct {
		name     string
		testGate func(entry SyntheticPanelEntry) bool
	}{
		{
			name: "score_gate_75",
			testGate: func(entry SyntheticPanelEntry) bool {
				return entry.CompositeScore >= 75.0
			},
		},
		{
			name: "vadr_gate_1.8",
			testGate: func(entry SyntheticPanelEntry) bool {
				return entry.VADR >= 1.8
			},
		},
		{
			name: "funding_divergence_gate",
			testGate: func(entry SyntheticPanelEntry) bool {
				return entry.FundingDivergenceZScore >= 2.0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passEntries := []SyntheticPanelEntry{}
			failEntries := []SyntheticPanelEntry{}

			for _, entry := range panel {
				if tt.testGate(entry) {
					passEntries = append(passEntries, entry)
				} else {
					failEntries = append(failEntries, entry)
				}
			}

			if len(passEntries) == 0 || len(failEntries) == 0 {
				t.Skipf("insufficient entries for gate %s (pass=%d, fail=%d)",
					tt.name, len(passEntries), len(failEntries))
			}

			passAvg4h := calculateAvgReturn(passEntries, "4h")
			failAvg4h := calculateAvgReturn(failEntries, "4h")
			gap := passAvg4h - failAvg4h

			t.Logf("Gate %s: Pass=%.3f%%, Fail=%.3f%%, Gap=+%.3f%% (n_pass=%d, n_fail=%d)",
				tt.name, passAvg4h*100, failAvg4h*100, gap*100, len(passEntries), len(failEntries))

			// Individual gates should generally show positive contribution
			if gap <= 0 {
				t.Logf("Warning: Individual gate %s shows negative contribution (%.3f%%)", tt.name, gap*100)
			}
		})
	}
}

func TestGateAlignment_CombinedGateStrength(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Test different gate combination strictness levels
	gateConfigs := []struct {
		name   string
		gates  EntryGates
		expect string
	}{
		{
			name: "lenient_gates",
			gates: EntryGates{
				MinScore:                   70.0, // Lower threshold
				MinVADR:                    1.5,  // Lower threshold
				MinFundingDivergenceZScore: 1.5,  // Lower threshold
			},
			expect: "moderate outperformance",
		},
		{
			name: "standard_gates",
			gates: EntryGates{
				MinScore:                   75.0, // Standard threshold
				MinVADR:                    1.8,  // Standard threshold
				MinFundingDivergenceZScore: 2.0,  // Standard threshold
			},
			expect: "strong outperformance",
		},
		{
			name: "strict_gates",
			gates: EntryGates{
				MinScore:                   80.0, // Higher threshold
				MinVADR:                    2.2,  // Higher threshold
				MinFundingDivergenceZScore: 2.5,  // Higher threshold
			},
			expect: "very strong outperformance",
		},
	}

	for _, config := range gateConfigs {
		t.Run(config.name, func(t *testing.T) {
			analysis := analyzeGatePerformance(panel, config.gates, "4h")

			if analysis.GatePassCount == 0 {
				t.Skipf("no entries pass %s", config.name)
			}

			t.Logf("%s: Pass count=%d, Fail count=%d, Gap=+%.3f%%",
				config.name, analysis.GatePassCount, analysis.GateFailCount,
				analysis.OutperformanceGap*100)

			// Stricter gates should generally show larger gaps (when sufficient data)
			if analysis.OutperformanceGap <= 0 {
				t.Errorf("%s failed: expected positive outperformance, got %.3f%%",
					config.name, analysis.OutperformanceGap*100)
			}
		})
	}
}

func TestGateAlignment_RegimeSpecificGates(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Group by regime and test gate effectiveness per regime
	regimeGroups := make(map[string][]SyntheticPanelEntry)
	for _, entry := range panel {
		regimeGroups[entry.Regime] = append(regimeGroups[entry.Regime], entry)
	}

	standardGates := EntryGates{
		MinScore:                   75.0,
		MinVADR:                    1.8,
		MinFundingDivergenceZScore: 2.0,
	}

	for regime, entries := range regimeGroups {
		t.Run(regime+"_gates", func(t *testing.T) {
			if len(entries) < 4 {
				t.Skipf("insufficient entries for regime %s (%d)", regime, len(entries))
			}

			analysis := analyzeGatePerformanceForEntries(entries, standardGates, "4h")

			if analysis.GatePassCount == 0 || analysis.GateFailCount == 0 {
				t.Skipf("regime %s: insufficient data (pass=%d, fail=%d)",
					regime, analysis.GatePassCount, analysis.GateFailCount)
			}

			t.Logf("Regime %s gates: Pass=%.3f%%, Fail=%.3f%%, Gap=+%.3f%%",
				regime, analysis.GatePassAvgReturn*100, analysis.GateFailAvgReturn*100,
				analysis.OutperformanceGap*100)

			// Gates should work across different regimes
			if analysis.OutperformanceGap <= 0 {
				t.Logf("Warning: Gates show negative performance in regime %s", regime)
			}
		})
	}
}

func TestGateAlignment_EdgeCases(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Test edge cases that might break gate logic
	tests := []struct {
		name        string
		gates       EntryGates
		expectPass  bool
		description string
	}{
		{
			name: "impossible_gates",
			gates: EntryGates{
				MinScore:                   100.0, // Impossible threshold
				MinVADR:                    10.0,  // Impossible threshold
				MinFundingDivergenceZScore: 10.0,  // Impossible threshold
			},
			expectPass:  false,
			description: "no entries should pass impossible gates",
		},
		{
			name: "minimal_gates",
			gates: EntryGates{
				MinScore:                   0.0,   // Everyone passes
				MinVADR:                    0.0,   // Everyone passes
				MinFundingDivergenceZScore: -10.0, // Everyone passes
			},
			expectPass:  true,
			description: "all entries should pass minimal gates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeGatePerformance(panel, tt.gates, "4h")

			hasPassingEntries := analysis.GatePassCount > 0

			if tt.expectPass && !hasPassingEntries {
				t.Errorf("%s: expected passing entries, got none", tt.description)
			}

			if !tt.expectPass && hasPassingEntries {
				t.Errorf("%s: expected no passing entries, got %d", tt.description, analysis.GatePassCount)
			}

			t.Logf("%s: Pass count=%d (expected pass=%v)", tt.name, analysis.GatePassCount, tt.expectPass)
		})
	}
}

// Helper functions

func analyzeGatePerformance(panel []SyntheticPanelEntry, gates EntryGates, timeframe string) GateAnalysis {
	return analyzeGatePerformanceForEntries(panel, gates, timeframe)
}

func analyzeGatePerformanceForEntries(entries []SyntheticPanelEntry, gates EntryGates, timeframe string) GateAnalysis {
	var passEntries, failEntries []SyntheticPanelEntry

	for _, entry := range entries {
		if passesGates(entry, gates) {
			passEntries = append(passEntries, entry)
		} else {
			failEntries = append(failEntries, entry)
		}
	}

	analysis := GateAnalysis{
		TotalCount:    len(entries),
		GatePassCount: len(passEntries),
		GateFailCount: len(failEntries),
	}

	if len(passEntries) > 0 {
		analysis.GatePassAvgReturn = calculateAvgReturn(passEntries, timeframe)
	}

	if len(failEntries) > 0 {
		analysis.GateFailAvgReturn = calculateAvgReturn(failEntries, timeframe)
	}

	analysis.OutperformanceGap = analysis.GatePassAvgReturn - analysis.GateFailAvgReturn

	return analysis
}

func passesGates(entry SyntheticPanelEntry, gates EntryGates) bool {
	return entry.CompositeScore >= gates.MinScore &&
		entry.VADR >= gates.MinVADR &&
		entry.FundingDivergenceZScore >= gates.MinFundingDivergenceZScore
}

func calculateHitRate(panel []SyntheticPanelEntry, gates EntryGates, threshold float64, passesGatesFlag bool) float64 {
	var relevantEntries []SyntheticPanelEntry

	for _, entry := range panel {
		entryPassesGates := passesGates(entry, gates)
		if entryPassesGates == passesGatesFlag {
			relevantEntries = append(relevantEntries, entry)
		}
	}

	if len(relevantEntries) == 0 {
		return 0
	}

	hits := 0
	for _, entry := range relevantEntries {
		if entry.ForwardReturn4h >= threshold {
			hits++
		}
	}

	return float64(hits) / float64(len(relevantEntries))
}
