package main

import (
	"fmt"
	"math/rand"
)

// Simple offline test for scanning capability
func main() {
	fmt.Println("🏃‍♂️ CryptoRun Offline Scan Test")
	fmt.Println("================================")

	// Fake universe symbols for testing
	symbols := []string{
		"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "LINKUSD",
		"DOTUSD", "MATICUSD", "AVAXUSD", "UNIUSD", "LTCUSD",
	}

	regime := "trending_bull"
	fmt.Printf("Regime: %s\n", regime)
	fmt.Println()

	var selectedCandidates []TestCandidate

	// Generate fake candidates
	for i, symbol := range symbols {
		if i >= 5 { // Test with 5 symbols
			break
		}

		candidate := generateTestCandidate(symbol, regime)
		if candidate.Selected {
			selectedCandidates = append(selectedCandidates, candidate)
		}

		fmt.Printf("%-8s Score: %5.1f Gates: %-4s Decision: %s\n",
			candidate.Symbol,
			candidate.Score.Total,
			gateStatus(candidate.Gates.AllPass),
			candidate.Decision)
	}

	fmt.Printf("\n✅ Offline scan completed: %d candidates processed, %d selected\n", 
		len(symbols[:5]), len(selectedCandidates))
	fmt.Println("Results would be saved to: out/scan/latest_candidates.jsonl")
}

type TestCandidate struct {
	Symbol   string    `json:"symbol"`
	Score    TestScore `json:"score"`
	Gates    TestGates `json:"gates"`
	Decision string    `json:"decision"`
	Selected bool      `json:"selected"`
}

type TestScore struct {
	Total     float64 `json:"total"`
	Momentum  float64 `json:"momentum"`
	Technical float64 `json:"technical"`
	Volume    float64 `json:"volume"`
	Quality   float64 `json:"quality"`
	Social    float64 `json:"social"`
}

type TestGates struct {
	AllPass bool `json:"all_pass"`
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
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
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

	// Most symbols should pass gates in test mode
	gatesPass := rng.Float64() < 0.8 // 80% pass rate
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		gatesPass = rng.Float64() < 0.95 // 95% for majors
	}

	gates := TestGates{
		AllPass: gatesPass,
	}

	selected := score.Total >= 75.0 && gates.AllPass
	decision := getTestDecision(selected, score.Total, gates.AllPass)

	return TestCandidate{
		Symbol:   symbol,
		Score:    score,
		Gates:    gates,
		Decision: decision,
		Selected: selected,
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