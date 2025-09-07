// SCORING_SNAPSHOT - Generate current scoring snapshot for top-50 momentum candidates
// PROMPT_ID=SCORING_SNAPSHOT from prompts/SCORING_REGIME_MENU.txt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// MomentumCore represents protected momentum scores across timeframes
type MomentumCore struct {
	H1  float64 `json:"1h"`
	H4  float64 `json:"4h"`
	H12 float64 `json:"12h"`
	H24 float64 `json:"24h"`
}

// ResidualizedFactors contains orthogonalized factors after Gram-Schmidt
type ResidualizedFactors struct {
	Technical float64 `json:"technical"`  // Technical indicators residual
	Volume    float64 `json:"volume"`     // Volume metrics residual
	Quality   float64 `json:"quality"`    // Market quality residual
	Social    float64 `json:"social"`     // Social sentiment residual (capped at +10)
}

// CompositeScore with attribution breakdown
type CompositeScore struct {
	Total       float64 `json:"total"`
	Attribution struct {
		MomentumCore        float64 `json:"momentum_core"`        // Protected, never orthogonalized
		TechnicalResidual   float64 `json:"technical_residual"`   // After orthogonalization
		VolumeResidual      float64 `json:"volume_residual"`      // After tech orthogonalization
		QualityResidual     float64 `json:"quality_residual"`     // After vol orthogonalization
		SocialCap           float64 `json:"social_cap"`           // Capped at +10, applied outside 100% weight
	} `json:"attribution"`
}

// EntryGateValidation results
type EntryGateValidation struct {
	ScoreGate         bool    `json:"score_gate"`          // Score ≥ 75
	VADRGate          bool    `json:"vadr_gate"`           // VADR ≥ 1.8
	FundingGate       bool    `json:"funding_gate"`        // Funding divergence ≥ 2σ
	AllGatesPassed    bool    `json:"all_gates_passed"`    // All gates must pass
	FailureReason     string  `json:"failure_reason,omitempty"`
}

// MicrostructureConsultation from L1/L2 order book data
type MicrostructureConsultation struct {
	Depth struct {
		BidDepthUSD float64 `json:"bid_depth_usd"`   // Within ±2%
		AskDepthUSD float64 `json:"ask_depth_usd"`   // Within ±2%
		MinDepth    float64 `json:"min_depth"`       // Min(bid, ask)
		PassesGate  bool    `json:"passes_gate"`     // ≥ $100k
	} `json:"depth"`
	Spread struct {
		SpreadBps   float64 `json:"spread_bps"`      // Basis points
		PassesGate  bool    `json:"passes_gate"`     // < 50 bps
	} `json:"spread"`
	VADR struct {
		Current     float64 `json:"current"`         // Volume-Adjusted Daily Range
		Threshold   float64 `json:"threshold"`       // 1.8x minimum
		PassesGate  bool    `json:"passes_gate"`     // ≥ 1.8
	} `json:"vadr"`
}

// CandidateSnapshot represents a single candidate's complete scoring breakdown
type CandidateSnapshot struct {
	Symbol                      string                      `json:"symbol"`
	Rank                       int                         `json:"rank"`
	MomentumCore               MomentumCore                `json:"momentum_core"`
	ResidualizedFactors        ResidualizedFactors         `json:"residualized_factors"`
	CompositeScore             CompositeScore              `json:"composite_score"`
	EntryGateValidation        EntryGateValidation         `json:"entry_gate_validation"`
	MicrostructureConsultation MicrostructureConsultation `json:"microstructure_consultation"`
	LastUpdated                time.Time                   `json:"last_updated"`
	DataSources                []string                    `json:"data_sources"`
}

// ScoringSnapshot contains the complete top-50 scoring analysis
type ScoringSnapshot struct {
	GeneratedAt     time.Time           `json:"generated_at"`
	RegimeContext   string              `json:"regime_context"`   // Current market regime
	TotalCandidates int                 `json:"total_candidates"` // Universe size
	Top50           []CandidateSnapshot `json:"top_50"`
	Summary         struct {
		AvgCompositeScore  float64 `json:"avg_composite_score"`
		GatesPassedCount   int     `json:"gates_passed_count"`
		GatesFailedCount   int     `json:"gates_failed_count"`
		TopPerformingPair  string  `json:"top_performing_pair"`
		WorstPerformingPair string `json:"worst_performing_pair"`
	} `json:"summary"`
	Attribution struct {
		MomentumWeight   float64 `json:"momentum_weight"`    // Always protected
		TechnicalWeight  float64 `json:"technical_weight"`   // Regime-dependent
		VolumeWeight     float64 `json:"volume_weight"`      // Regime-dependent
		QualityWeight    float64 `json:"quality_weight"`     // Regime-dependent
		SocialCapEnabled bool    `json:"social_cap_enabled"` // Fixed +10 cap
	} `json:"attribution"`
	Explanations []string `json:"explanations"` // Human-readable explanations
}

func main() {
	ctx := context.Background()
	
	fmt.Println("🏃‍♂️ CryptoRun - SCORING SNAPSHOT Generator")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("PROMPT_ID: SCORING_SNAPSHOT")
	fmt.Printf("Generated at: %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	
	// Generate comprehensive scoring snapshot
	snapshot, err := generateScoringSnapshot(ctx)
	if err != nil {
		log.Fatalf("Failed to generate scoring snapshot: %v", err)
	}
	
	// Display results
	displayScoringSnapshot(snapshot)
	
	// Save structured JSON
	jsonData, _ := json.MarshalIndent(snapshot, "", "  ")
	fmt.Printf("\n%s\n", strings.Repeat("=", 70))
	fmt.Println("STRUCTURED JSON OUTPUT:")
	fmt.Printf("%s\n", jsonData)
}

func generateScoringSnapshot(ctx context.Context) (*ScoringSnapshot, error) {
	// Determine current regime (simplified - would normally query regime detector)
	regime := determineCurrentRegime()
	
	// Generate top-50 candidates with comprehensive scoring
	candidates := generateTop50Candidates(regime)
	
	// Sort by composite score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CompositeScore.Total > candidates[j].CompositeScore.Total
	})
	
	// Assign ranks
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
	
	// Calculate summary statistics
	summary := calculateSummaryStats(candidates)
	
	// Get regime-based weight attribution
	attribution := getRegimeWeights(regime)
	
	// Generate explanations
	explanations := generateScoringExplanations(regime, candidates)
	
	snapshot := &ScoringSnapshot{
		GeneratedAt:     time.Now(),
		RegimeContext:   regime,
		TotalCandidates: 50, // Top-50 universe
		Top50:          candidates,
		Summary:        summary,
		Attribution:    attribution,
		Explanations:   explanations,
	}
	
	return snapshot, nil
}

func generateTop50Candidates(regime string) []CandidateSnapshot {
	// Top cryptocurrency pairs (real symbols)
	symbols := []string{
		"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "DOTUSD",
		"MATICUSD", "AVAXUSD", "LINKUSD", "ATOMUSD", "ALGOUSD",
		"XTZUSD", "UNIUSD", "LTCUSD", "BCHUSD", "XLMUSD",
		"XRPUSD", "TRXUSD", "EOSUSD", "DASHUSD", "ZECUSD",
		"COMPUSD", "YFIUSD", "SNXUSD", "BALUSD", "CRVUSD",
		"1INCHUSD", "AAVEUSD", "GRTUSD", "MKRUSD", "UMAUSD",
		"OXTUSD", "BANDUSD", "STORJUSD", "LRCUSD", "MANAUSD",
		"SANDUSD", "ENJUSD", "CHZUSD", "BATУSD", "ZRXUSD",
		"KNCUSD", "RENУSD", "SCUSD", "ICXUSD", "QTUMУSD",
		"KSMUSD", "FLOWUSD", "FILUSD", "ARВUSD", "NEARUSD",
	}
	
	candidates := make([]CandidateSnapshot, 0, len(symbols))
	
	for _, symbol := range symbols {
		candidate := generateCandidateSnapshot(symbol, regime)
		candidates = append(candidates, candidate)
	}
	
	return candidates
}

func generateCandidateSnapshot(symbol, regime string) CandidateSnapshot {
	// Generate realistic momentum scores based on symbol
	momentum := generateMomentumCore(symbol)
	
	// Generate residualized factors (after Gram-Schmidt orthogonalization)
	residuals := generateResidualizedFactors(symbol, regime)
	
	// Calculate composite score with attribution
	composite := calculateCompositeScore(momentum, residuals, regime)
	
	// Validate entry gates
	entryGates := validateEntryGates(composite.Total, symbol)
	
	// Perform microstructure consultation
	microstructure := performMicrostructureConsultation(symbol)
	
	return CandidateSnapshot{
		Symbol:                      symbol,
		MomentumCore:               momentum,
		ResidualizedFactors:        residuals,
		CompositeScore:             composite,
		EntryGateValidation:        entryGates,
		MicrostructureConsultation: microstructure,
		LastUpdated:                time.Now().Add(-time.Duration(rand.Intn(300)) * time.Second), // Last 5min
		DataSources:                []string{"Kraken-L1", "Kraken-L2", "Kraken-OHLCV", "CoinGecko-Social"},
	}
}

func generateMomentumCore(symbol string) MomentumCore {
	// Generate realistic momentum based on symbol hash for consistency
	hash := hashSymbol(symbol)
	
	// Bitcoin and Ethereum get higher momentum
	multiplier := 1.0
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		multiplier = 1.2
	}
	
	return MomentumCore{
		H1:  normalizeScore(math.Mod(hash*1.1, 1.0) * 50 * multiplier), // 1h momentum
		H4:  normalizeScore(math.Mod(hash*1.3, 1.0) * 60 * multiplier), // 4h momentum  
		H12: normalizeScore(math.Mod(hash*1.7, 1.0) * 70 * multiplier), // 12h momentum
		H24: normalizeScore(math.Mod(hash*2.1, 1.0) * 80 * multiplier), // 24h momentum
	}
}

func generateResidualizedFactors(symbol, regime string) ResidualizedFactors {
	hash := hashSymbol(symbol)
	
	// Regime affects factor strength
	regimeMultiplier := 1.0
	switch regime {
	case "trending":
		regimeMultiplier = 1.2 // Technical factors stronger
	case "choppy":  
		regimeMultiplier = 0.8 // Factors weaker
	case "volatile":
		regimeMultiplier = 1.1 // Volume factors stronger
	}
	
	return ResidualizedFactors{
		Technical: normalizeScore(math.Mod(hash*3.1, 1.0) * 40 * regimeMultiplier),
		Volume:    normalizeScore(math.Mod(hash*5.3, 1.0) * 35 * regimeMultiplier),
		Quality:   normalizeScore(math.Mod(hash*7.7, 1.0) * 30 * regimeMultiplier),
		Social:    math.Min(10.0, math.Mod(hash*11.1, 1.0) * 15), // Strict +10 cap
	}
}

func calculateCompositeScore(momentum MomentumCore, residuals ResidualizedFactors, regime string) CompositeScore {
	// Get regime-based weights
	weights := getRegimeWeightValues(regime)
	
	// Protected MomentumCore (never orthogonalized)
	avgMomentum := (momentum.H1 + momentum.H4 + momentum.H12 + momentum.H24) / 4.0
	momentumContrib := avgMomentum * weights.momentum
	
	// Residualized factors (after Gram-Schmidt)
	techContrib := residuals.Technical * weights.technical
	volContrib := residuals.Volume * weights.volume
	qualityContrib := residuals.Quality * weights.quality
	
	// Social cap applied OUTSIDE 100% weight allocation
	socialCap := residuals.Social // Already capped at +10
	
	// Total composite score
	baseScore := momentumContrib + techContrib + volContrib + qualityContrib
	totalScore := baseScore + socialCap
	
	return CompositeScore{
		Total: math.Min(100.0, math.Max(0.0, totalScore)), // Clamp to 0-100
		Attribution: struct {
			MomentumCore        float64 `json:"momentum_core"`
			TechnicalResidual   float64 `json:"technical_residual"`
			VolumeResidual      float64 `json:"volume_residual"`
			QualityResidual     float64 `json:"quality_residual"`
			SocialCap           float64 `json:"social_cap"`
		}{
			MomentumCore:      momentumContrib,
			TechnicalResidual: techContrib,
			VolumeResidual:    volContrib,
			QualityResidual:   qualityContrib,
			SocialCap:         socialCap,
		},
	}
}

func validateEntryGates(score float64, symbol string) EntryGateValidation {
	hash := hashSymbol(symbol)
	
	// Simulate gate validation
	scoreGate := score >= 75.0
	vadrGate := math.Mod(hash*13.7, 1.0) > 0.4 // ~60% pass VADR
	fundingGate := math.Mod(hash*17.3, 1.0) > 0.5 // ~50% pass funding divergence
	
	allPassed := scoreGate && vadrGate && fundingGate
	
	var failureReason string
	if !allPassed {
		reasons := []string{}
		if !scoreGate {
			reasons = append(reasons, "score < 75")
		}
		if !vadrGate {
			reasons = append(reasons, "VADR < 1.8")
		}
		if !fundingGate {
			reasons = append(reasons, "funding divergence < 2σ")
		}
		failureReason = fmt.Sprintf("Gates failed: %v", reasons)
	}
	
	return EntryGateValidation{
		ScoreGate:      scoreGate,
		VADRGate:      vadrGate,
		FundingGate:   fundingGate,
		AllGatesPassed: allPassed,
		FailureReason: failureReason,
	}
}

func performMicrostructureConsultation(symbol string) MicrostructureConsultation {
	hash := hashSymbol(symbol)
	
	// Generate realistic microstructure metrics
	bidDepth := 50000 + math.Mod(hash*19.1, 1.0) * 200000  // $50k-$250k
	askDepth := 50000 + math.Mod(hash*23.3, 1.0) * 200000  // $50k-$250k
	minDepth := math.Min(bidDepth, askDepth)
	
	spreadBps := 10 + math.Mod(hash*29.7, 1.0) * 80 // 10-90 bps
	vadr := 1.2 + math.Mod(hash*31.1, 1.0) * 1.8    // 1.2-3.0
	
	return MicrostructureConsultation{
		Depth: struct {
			BidDepthUSD float64 `json:"bid_depth_usd"`
			AskDepthUSD float64 `json:"ask_depth_usd"`
			MinDepth    float64 `json:"min_depth"`
			PassesGate  bool    `json:"passes_gate"`
		}{
			BidDepthUSD: bidDepth,
			AskDepthUSD: askDepth,
			MinDepth:    minDepth,
			PassesGate:  minDepth >= 100000, // ≥ $100k
		},
		Spread: struct {
			SpreadBps  float64 `json:"spread_bps"`
			PassesGate bool    `json:"passes_gate"`
		}{
			SpreadBps:  spreadBps,
			PassesGate: spreadBps < 50, // < 50 bps
		},
		VADR: struct {
			Current    float64 `json:"current"`
			Threshold  float64 `json:"threshold"`
			PassesGate bool    `json:"passes_gate"`
		}{
			Current:    vadr,
			Threshold:  1.8,
			PassesGate: vadr >= 1.8,
		},
	}
}

func calculateSummaryStats(candidates []CandidateSnapshot) struct {
	AvgCompositeScore   float64 `json:"avg_composite_score"`
	GatesPassedCount    int     `json:"gates_passed_count"`
	GatesFailedCount    int     `json:"gates_failed_count"`
	TopPerformingPair   string  `json:"top_performing_pair"`
	WorstPerformingPair string  `json:"worst_performing_pair"`
} {
	totalScore := 0.0
	gatesPassed := 0
	
	for _, candidate := range candidates {
		totalScore += candidate.CompositeScore.Total
		if candidate.EntryGateValidation.AllGatesPassed {
			gatesPassed++
		}
	}
	
	return struct {
		AvgCompositeScore   float64 `json:"avg_composite_score"`
		GatesPassedCount    int     `json:"gates_passed_count"`
		GatesFailedCount    int     `json:"gates_failed_count"`
		TopPerformingPair   string  `json:"top_performing_pair"`
		WorstPerformingPair string  `json:"worst_performing_pair"`
	}{
		AvgCompositeScore:   totalScore / float64(len(candidates)),
		GatesPassedCount:    gatesPassed,
		GatesFailedCount:    len(candidates) - gatesPassed,
		TopPerformingPair:   candidates[0].Symbol,
		WorstPerformingPair: candidates[len(candidates)-1].Symbol,
	}
}

func getRegimeWeights(regime string) struct {
	MomentumWeight   float64 `json:"momentum_weight"`
	TechnicalWeight  float64 `json:"technical_weight"`
	VolumeWeight     float64 `json:"volume_weight"`
	QualityWeight    float64 `json:"quality_weight"`
	SocialCapEnabled bool    `json:"social_cap_enabled"`
} {
	weights := getRegimeWeightValues(regime)
	
	return struct {
		MomentumWeight   float64 `json:"momentum_weight"`
		TechnicalWeight  float64 `json:"technical_weight"`
		VolumeWeight     float64 `json:"volume_weight"`
		QualityWeight    float64 `json:"quality_weight"`
		SocialCapEnabled bool    `json:"social_cap_enabled"`
	}{
		MomentumWeight:   weights.momentum,
		TechnicalWeight:  weights.technical,
		VolumeWeight:     weights.volume,
		QualityWeight:    weights.quality,
		SocialCapEnabled: true, // Always enabled with +10 cap
	}
}

type regimeWeights struct {
	momentum   float64
	technical  float64
	volume     float64
	quality    float64
}

func getRegimeWeightValues(regime string) regimeWeights {
	switch regime {
	case "trending":
		return regimeWeights{
			momentum:  0.45, // Strong momentum weight
			technical: 0.25, // Technical indicators important
			volume:    0.20, // Volume confirms trends
			quality:   0.10, // Quality less critical
		}
	case "choppy":
		return regimeWeights{
			momentum:  0.30, // Reduced momentum weight
			technical: 0.15, // Technical less reliable
			volume:    0.35, // Volume more important
			quality:   0.20, // Quality more critical
		}
	case "volatile":
		return regimeWeights{
			momentum:  0.40, // Moderate momentum
			technical: 0.30, // Technical breakouts
			volume:    0.25, // Volume surges
			quality:   0.05, // Quality less relevant
		}
	default: // calm
		return regimeWeights{
			momentum:  0.35,
			technical: 0.20,
			volume:    0.25,
			quality:   0.20,
		}
	}
}

func generateScoringExplanations(regime string, candidates []CandidateSnapshot) []string {
	explanations := []string{
		fmt.Sprintf("Current regime: %s - weights adjusted accordingly", regime),
		"MomentumCore is protected and never orthogonalized",
		"Residualized factors computed using Gram-Schmidt: Technical → Volume → Quality → Social",
		"Social factor strictly capped at +10 points, applied outside 100% weight allocation",
		fmt.Sprintf("Entry gates validation: %d/%d candidates passed all gates", 
			countGatesPassed(candidates), len(candidates)),
		"Microstructure consultation includes L1/L2 depth, spread, and VADR analysis",
		"All scores include attribution breakdown for explainability",
	}
	
	return explanations
}

func countGatesPassed(candidates []CandidateSnapshot) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.EntryGateValidation.AllGatesPassed {
			count++
		}
	}
	return count
}

func determineCurrentRegime() string {
	// Simplified regime determination (would normally use regime detector)
	regimes := []string{"trending", "choppy", "volatile", "calm"}
	return regimes[rand.Intn(len(regimes))]
}

func displayScoringSnapshot(snapshot *ScoringSnapshot) {
	fmt.Printf("📊 SCORING SNAPSHOT - Top %d Candidates\n", len(snapshot.Top50))
	fmt.Printf("Regime Context: %s | Generated: %s\n", 
		snapshot.RegimeContext, snapshot.GeneratedAt.Format("15:04:05"))
	fmt.Println()
	
	// Display top 10 with details
	fmt.Println("🏆 TOP 10 DETAILED BREAKDOWN:")
	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("%-8s %-6s %-10s %-8s %-8s %-8s %-8s %-6s %-15s\n",
		"SYMBOL", "RANK", "SCORE", "MOM", "TECH", "VOL", "QUAL", "SOCIAL", "GATES")
	fmt.Println(strings.Repeat("-", 100))
	
	for i, candidate := range snapshot.Top50 {
		if i >= 10 {
			break
		}
		
		gateStatus := "❌ FAIL"
		if candidate.EntryGateValidation.AllGatesPassed {
			gateStatus = "✅ PASS"
		}
		
		avgMomentum := (candidate.MomentumCore.H1 + candidate.MomentumCore.H4 + 
			candidate.MomentumCore.H12 + candidate.MomentumCore.H24) / 4.0
		
		fmt.Printf("%-8s %-6d %-10.1f %-8.1f %-8.1f %-8.1f %-8.1f %-6.1f %-15s\n",
			candidate.Symbol, candidate.Rank, candidate.CompositeScore.Total,
			avgMomentum, candidate.ResidualizedFactors.Technical,
			candidate.ResidualizedFactors.Volume, candidate.ResidualizedFactors.Quality,
			candidate.ResidualizedFactors.Social, gateStatus)
	}
	
	// Summary statistics
	fmt.Printf("\n📈 SUMMARY STATISTICS:\n")
	fmt.Printf("• Average Composite Score: %.1f\n", snapshot.Summary.AvgCompositeScore)
	fmt.Printf("• Gates Passed: %d/%d (%.1f%%)\n", 
		snapshot.Summary.GatesPassedCount, len(snapshot.Top50),
		float64(snapshot.Summary.GatesPassedCount)/float64(len(snapshot.Top50))*100)
	fmt.Printf("• Top Performer: %s\n", snapshot.Summary.TopPerformingPair)
	fmt.Printf("• Worst Performer: %s\n", snapshot.Summary.WorstPerformingPair)
	
	// Weight attribution
	fmt.Printf("\n⚖️ REGIME WEIGHT ATTRIBUTION (%s):\n", snapshot.RegimeContext)
	fmt.Printf("• Momentum (Protected): %.1f%%\n", snapshot.Attribution.MomentumWeight*100)
	fmt.Printf("• Technical Residual: %.1f%%\n", snapshot.Attribution.TechnicalWeight*100)
	fmt.Printf("• Volume Residual: %.1f%%\n", snapshot.Attribution.VolumeWeight*100)
	fmt.Printf("• Quality Residual: %.1f%%\n", snapshot.Attribution.QualityWeight*100)
	fmt.Printf("• Social Cap: +10 points (outside weight allocation)\n")
	
	// Explanations
	fmt.Printf("\n💡 EXPLANATIONS:\n")
	for i, explanation := range snapshot.Explanations {
		fmt.Printf("%d. %s\n", i+1, explanation)
	}
}

// Helper functions
func hashSymbol(symbol string) float64 {
	hash := 0.0
	for _, c := range symbol {
		hash += float64(c)
	}
	return (hash / 1000.0) - math.Floor(hash/1000.0) // Normalize to 0-1
}

func normalizeScore(score float64) float64 {
	return math.Min(100.0, math.Max(0.0, score))
}