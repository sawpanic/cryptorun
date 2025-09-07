package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// Enhanced offline test with JSON output
func main() {
	fmt.Println("🏃‍♂️ CryptoRun Offline Scan Test with Output")
	fmt.Println("=============================================")

	// Fake universe symbols for testing
	symbols := []string{
		"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "LINKUSD",
		"DOTUSD", "MATICUSD", "AVAXUSD", "UNIUSD", "LTCUSD",
		"XRPUSD", "ALGOUSD", "ATOMUSD", "NEARUSD", "FTMUSD",
	}

	regime := "trending_bull"
	fmt.Printf("Regime: %s\n", regime)
	fmt.Println()

	var allCandidates []TestCandidate
	var selectedCandidates []TestCandidate

	// Generate fake candidates
	for i, symbol := range symbols {
		if i >= 10 { // Test with 10 symbols
			break
		}

		candidate := generateTestCandidate(symbol, regime)
		allCandidates = append(allCandidates, candidate)
		
		if candidate.Selected {
			selectedCandidates = append(selectedCandidates, candidate)
		}

		fmt.Printf("%-8s Score: %5.1f Gates: %-4s Decision: %s\n",
			candidate.Symbol,
			candidate.Score.Total,
			gateStatus(candidate.Gates.AllPass),
			candidate.Decision)
	}

	// Save to JSONL format
	outputPath := "out/scan/latest_candidates.jsonl"
	if err := writeJSONL(allCandidates, outputPath); err != nil {
		fmt.Printf("Error writing JSONL: %v\n", err)
	} else {
		fmt.Printf("\n✅ Results saved to: %s\n", outputPath)
	}

	// Generate summary report
	summaryPath := "out/scan/scan_summary.json"
	summary := ScanSummary{
		Timestamp:       time.Now(),
		Regime:          regime,
		TotalProcessed:  len(allCandidates),
		CandidatesFound: len(allCandidates),
		Selected:        len(selectedCandidates),
		ProcessingTime:  "0.05s",
		TopSelected:     selectedCandidates,
	}

	if err := writeSummary(summary, summaryPath); err != nil {
		fmt.Printf("Error writing summary: %v\n", err)
	} else {
		fmt.Printf("Summary saved to: %s\n", summaryPath)
	}

	fmt.Printf("\n📊 Scan Summary:\n")
	fmt.Printf("   • Processed: %d symbols\n", summary.TotalProcessed)
	fmt.Printf("   • Selected:  %d candidates\n", summary.Selected)
	fmt.Printf("   • Success:   %.1f%%\n", float64(summary.Selected)/float64(summary.TotalProcessed)*100)
}

type TestCandidate struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Score     TestScore `json:"score"`
	Factors   TestFactors `json:"factors"`
	Gates     TestGates `json:"gates"`
	Decision  string    `json:"decision"`
	Selected  bool      `json:"selected"`
}

type TestScore struct {
	Total     float64 `json:"total"`
	Momentum  float64 `json:"momentum"`
	Technical float64 `json:"technical"`
	Volume    float64 `json:"volume"`
	Quality   float64 `json:"quality"`
	Social    float64 `json:"social"`
}

type TestFactors struct {
	Momentum  float64 `json:"momentum"`
	Technical float64 `json:"technical"`
	Volume    float64 `json:"volume"`
	Quality   float64 `json:"quality"`
	Social    float64 `json:"social"`
}

type TestGates struct {
	Microstructure TestMicroGates `json:"microstructure"`
	Freshness      TestGateResult `json:"freshness"`
	LateFill       TestGateResult `json:"late_fill"`
	Fatigue        TestGateResult `json:"fatigue"`
	AllPass        bool           `json:"all_pass"`
}

type TestMicroGates struct {
	SpreadBps float64 `json:"spread_bps"`
	DepthUSD  float64 `json:"depth_usd"`
	VADR      float64 `json:"vadr"`
	AllPass   bool    `json:"all_pass"`
}

type TestGateResult struct {
	OK   bool   `json:"ok"`
	Name string `json:"name"`
}

type ScanSummary struct {
	Timestamp       time.Time       `json:"timestamp"`
	Regime          string          `json:"regime"`
	TotalProcessed  int             `json:"total_processed"`
	CandidatesFound int             `json:"candidates_found"`
	Selected        int             `json:"selected"`
	ProcessingTime  string          `json:"processing_time"`
	TopSelected     []TestCandidate `json:"top_selected"`
}

func generateTestCandidate(symbol, regime string) TestCandidate {
	// Deterministic seed based on symbol
	seed := int64(0)
	for _, char := range symbol {
		seed += int64(char)
	}
	rng := rand.New(rand.NewSource(seed))

	// Base score varies by symbol
	baseScore := 40.0 + rng.Float64()*50.0 // 40-90 range

	// Regime adjustments
	if regime == "trending_bull" {
		baseScore += 10.0 // Higher scores in bull markets
	}

	// Ensure some majors pass the threshold
	if symbol == "BTCUSD" || symbol == "ETHUSD" || symbol == "SOLUSD" {
		baseScore = 80.0 + rng.Float64()*15.0 // 80-95 for majors
	}

	score := TestScore{
		Total:     baseScore,
		Momentum:  baseScore * 0.4,  // 40% momentum
		Technical: baseScore * 0.25, // 25% technical
		Volume:    baseScore * 0.2,  // 20% volume
		Quality:   baseScore * 0.1,  // 10% quality
		Social:    rng.Float64() * 10.0, // 0-10 social cap
	}

	// Generate detailed factors
	factors := TestFactors{
		Momentum:  60.0 + rng.Float64()*30.0, // 60-90
		Technical: 50.0 + rng.Float64()*40.0, // 50-90
		Volume:    70.0 + rng.Float64()*25.0, // 70-95
		Quality:   65.0 + rng.Float64()*30.0, // 65-95
		Social:    rng.Float64() * 15.0,      // 0-15 (capped at 10 in composite)
	}

	// Generate realistic microstructure data
	spreadBps := 15.0 + rng.Float64()*35.0      // 15-50 bps
	depthUSD := 50000 + rng.Float64()*150000    // 50k-200k USD
	vadr := 1.2 + rng.Float64()*1.3            // 1.2-2.5

	microPass := spreadBps < 50.0 && depthUSD > 100000 && vadr > 1.75
	freshPass := rng.Float64() < 0.9  // 90% pass freshness
	latePass  := rng.Float64() < 0.95 // 95% pass late fill
	fatiguePass := rng.Float64() < 0.85 // 85% pass fatigue
	
	// Override for majors to have better microstructure
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		spreadBps = 10.0 + rng.Float64()*15.0 // 10-25 bps for majors
		depthUSD = 200000 + rng.Float64()*300000 // 200k-500k USD
		vadr = 1.8 + rng.Float64()*0.7 // 1.8-2.5
		microPass = true
		freshPass = true
		latePass = true
		fatiguePass = rng.Float64() < 0.95 // 95% pass fatigue for majors
	}

	allPass := microPass && freshPass && latePass && fatiguePass

	gates := TestGates{
		Microstructure: TestMicroGates{
			SpreadBps: spreadBps,
			DepthUSD:  depthUSD,
			VADR:      vadr,
			AllPass:   microPass,
		},
		Freshness: TestGateResult{
			OK:   freshPass,
			Name: "freshness_guard",
		},
		LateFill: TestGateResult{
			OK:   latePass,
			Name: "late_fill_guard",
		},
		Fatigue: TestGateResult{
			OK:   fatiguePass,
			Name: "fatigue_guard",
		},
		AllPass: allPass,
	}

	selected := score.Total >= 75.0 && gates.AllPass
	decision := getTestDecision(selected, score.Total, gates.AllPass)

	return TestCandidate{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Score:     score,
		Factors:   factors,
		Gates:     gates,
		Decision:  decision,
		Selected:  selected,
	}
}

func getTestDecision(selected bool, score float64, gatesPass bool) string {
	if selected {
		return fmt.Sprintf("SELECTED: score=%.1f ≥75.0, gates=PASS", score)
	}

	if score < 75.0 && !gatesPass {
		return fmt.Sprintf("REJECTED: score=%.1f <75.0, gates=FAIL", score)
	} else if score < 75.0 {
		return fmt.Sprintf("REJECTED: score=%.1f <75.0", score)
	} else {
		return fmt.Sprintf("REJECTED: gates=FAIL (score=%.1f ≥75.0)", score)
	}
}

func gateStatus(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func writeJSONL(candidates []TestCandidate, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, candidate := range candidates {
		if err := encoder.Encode(candidate); err != nil {
			return err
		}
	}

	return nil
}

func writeSummary(summary ScanSummary, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}