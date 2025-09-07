package premove

import (
	"fmt"
	"math"
	"time"
)

type Detector struct {
	fundingAnalyzer   *FundingAnalyzer
	supplyAnalyzer    *SupplyAnalyzer  
	whaleAnalyzer     *WhaleAnalyzer
	microstructure    *MicrostructureGates
}

type DetectionResults struct {
	Candidates []Candidate `json:"candidates"`
	Timestamp  time.Time   `json:"timestamp"`
	Universe   int         `json:"universe_size"`
	Summary    Summary     `json:"summary"`
}

type Candidate struct {
	Symbol  string     `json:"symbol"`
	Score   float64    `json:"score"`
	Gates   GateStatus `json:"gates"`
	Metrics Metrics    `json:"metrics"`
	Regime  string     `json:"regime"`
	VADR    float64    `json:"vadr"`
}

type GateStatus struct {
	FundingDivergence   bool `json:"funding_divergence"`   // Gate A
	SupplySqueeze       bool `json:"supply_squeeze"`       // Gate B
	WhaleAccumulation   bool `json:"whale_accumulation"`   // Gate C
	MicrostructureTier  int  `json:"microstructure_tier"`  // 1-3 (1=best)
	GatesPassed         int  `json:"gates_passed"`         // Count of A/B/C passed
}

type Metrics struct {
	FundingZ           float64 `json:"funding_z"`            // Funding rate z-score
	SpotVWAPRatio      float64 `json:"spot_vwap_ratio"`      // Spot price vs VWAP
	SpotCVD            float64 `json:"spot_cvd"`             // Spot cumulative delta
	PerpCVD            float64 `json:"perp_cvd"`             // Perpetual cumulative delta
	SupplyChangeWeek   float64 `json:"supply_change_week"`   // 7-day supply change %
	VenueCount         int     `json:"venue_count"`          // Venues with supply data
	LargePrintCount    int     `json:"large_print_count"`    // Large trades detected
	CVDResidual        float64 `json:"cvd_residual"`         // CVD vs price residual
	PriceDrift         float64 `json:"price_drift"`          // Price drift vs ATR
	HotwalletDecline   float64 `json:"hotwallet_decline"`    // Hot wallet decline %
}

type Summary struct {
	TotalCandidates    int     `json:"total_candidates"`
	GateAPassed        int     `json:"gate_a_passed"`
	GateBPassed        int     `json:"gate_b_passed"`
	GateCPassed        int     `json:"gate_c_passed"`
	TwoOfThreePassed   int     `json:"two_of_three_passed"`
	AvgScore           float64 `json:"avg_score"`
	TopRegime          string  `json:"top_regime"`
}

func NewDetector() *Detector {
	return &Detector{
		fundingAnalyzer:   NewFundingAnalyzer(),
		supplyAnalyzer:    NewSupplyAnalyzer(),
		whaleAnalyzer:     NewWhaleAnalyzer(),
		microstructure:    NewMicrostructureGates(),
	}
}

func (d *Detector) DetectPreMovement() (*DetectionResults, error) {
	startTime := time.Now()
	
	// Get universe for analysis
	universe := d.getAnalysisUniverse()
	
	results := &DetectionResults{
		Timestamp:  startTime,
		Universe:   len(universe),
		Candidates: make([]Candidate, 0),
	}
	
	fmt.Printf("🔍 Analyzing %d symbols for pre-movement signals...\n", len(universe))
	
	candidates := make([]Candidate, 0)
	summary := Summary{
		TotalCandidates: len(universe),
		TopRegime:       "NORMAL", // Would be determined from regime detector
	}
	
	for _, symbol := range universe {
		candidate := d.analyzeSymbol(symbol)
		candidates = append(candidates, candidate)
		
		// Update summary statistics
		if candidate.Gates.FundingDivergence {
			summary.GateAPassed++
		}
		if candidate.Gates.SupplySqueeze {
			summary.GateBPassed++
		}
		if candidate.Gates.WhaleAccumulation {
			summary.GateCPassed++
		}
		if candidate.Gates.GatesPassed >= 2 {
			summary.TwoOfThreePassed++
			results.Candidates = append(results.Candidates, candidate)
		}
	}
	
	// Calculate average score for qualifying candidates
	if len(results.Candidates) > 0 {
		var totalScore float64
		for _, c := range results.Candidates {
			totalScore += c.Score
		}
		summary.AvgScore = totalScore / float64(len(results.Candidates))
	}
	
	// Sort candidates by score descending
	d.sortCandidatesByScore(results.Candidates)
	
	results.Summary = summary
	
	fmt.Printf("✅ Analysis complete: %d/%d passed 2-of-3 gates\n", 
		summary.TwoOfThreePassed, summary.TotalCandidates)
	
	return results, nil
}

func (d *Detector) analyzeSymbol(symbol string) Candidate {
	// Gate A: Funding Divergence Analysis
	fundingResult := d.fundingAnalyzer.AnalyzeFunding(symbol)
	
	// Gate B: Supply Squeeze Analysis  
	supplyResult := d.supplyAnalyzer.AnalyzeSupply(symbol)
	
	// Gate C: Whale Accumulation Analysis
	whaleResult := d.whaleAnalyzer.AnalyzeWhale(symbol)
	
	// Microstructure tier evaluation
	microTier := d.microstructure.EvaluateTier(symbol)
	
	// Count gates passed
	gatesPassed := 0
	if fundingResult.Passed {
		gatesPassed++
	}
	if supplyResult.Passed {
		gatesPassed++
	}
	if whaleResult.Passed {
		gatesPassed++
	}
	
	// Calculate composite score (0-100)
	structuralScore := 45.0 * d.calculateStructuralScore(fundingResult, supplyResult)
	behavioralScore := 30.0 * d.calculateBehavioralScore(whaleResult)
	catalystScore := 25.0 * d.calculateCatalystScore(fundingResult, supplyResult, whaleResult)
	
	compositeScore := structuralScore + behavioralScore + catalystScore
	
	// Apply freshness penalty if data is stale
	if d.isDataStale(symbol) {
		compositeScore *= 0.85 // 15% penalty
	}
	
	// Apply venue health modifier
	if microTier > 2 {
		compositeScore *= 0.9 // 10% penalty for lower tier venues
	}
	
	return Candidate{
		Symbol: symbol,
		Score:  compositeScore,
		Gates: GateStatus{
			FundingDivergence:  fundingResult.Passed,
			SupplySqueeze:      supplyResult.Passed,
			WhaleAccumulation:  whaleResult.Passed,
			MicrostructureTier: microTier,
			GatesPassed:        gatesPassed,
		},
		Metrics: Metrics{
			FundingZ:           fundingResult.ZScore,
			SpotVWAPRatio:      fundingResult.SpotVWAPRatio,
			SpotCVD:            fundingResult.SpotCVD,
			PerpCVD:            fundingResult.PerpCVD,
			SupplyChangeWeek:   supplyResult.WeeklyChange,
			VenueCount:         supplyResult.VenueCount,
			LargePrintCount:    whaleResult.LargePrints,
			CVDResidual:        whaleResult.CVDResidual,
			PriceDrift:         whaleResult.PriceDrift,
			HotwalletDecline:   whaleResult.HotwalletDecline,
		},
		Regime: d.getCurrentRegime(), // Would integrate with regime detector
		VADR:   d.calculateVADR(symbol),
	}
}

func (d *Detector) calculateStructuralScore(funding *FundingResult, supply *SupplyResult) float64 {
	score := 0.0
	
	// Funding divergence component (60% of structural)
	if funding.Passed {
		score += 0.6 * math.Min(funding.ZScore/3.0, 1.0) // Normalize z-score
	}
	
	// Supply squeeze component (40% of structural)
	if supply.Passed {
		score += 0.4 * math.Min(math.Abs(supply.WeeklyChange)/10.0, 1.0) // Normalize %
	}
	
	return math.Min(score, 1.0)
}

func (d *Detector) calculateBehavioralScore(whale *WhaleResult) float64 {
	score := 0.0
	
	// Large print clustering (50% of behavioral)
	if whale.LargePrints > 0 {
		score += 0.5 * math.Min(float64(whale.LargePrints)/10.0, 1.0)
	}
	
	// CVD residual strength (30% of behavioral)
	if whale.CVDResidual > 0 && whale.PriceDrift < 0.5 {
		score += 0.3
	}
	
	// Hot wallet decline (20% of behavioral)
	if whale.HotwalletDecline > 0 {
		score += 0.2 * math.Min(whale.HotwalletDecline/20.0, 1.0)
	}
	
	return math.Min(score, 1.0)
}

func (d *Detector) calculateCatalystScore(funding *FundingResult, supply *SupplyResult, whale *WhaleResult) float64 {
	score := 0.0
	
	// Multiple gates passed bonus
	gateCount := 0
	if funding.Passed {
		gateCount++
	}
	if supply.Passed {
		gateCount++
	}
	if whale.Passed {
		gateCount++
	}
	
	switch gateCount {
	case 3:
		score = 1.0 // All gates passed - maximum catalyst
	case 2:
		score = 0.7 // Two gates passed - strong catalyst
	case 1:
		score = 0.3 // One gate passed - weak catalyst
	default:
		score = 0.0 // No gates passed - no catalyst
	}
	
	return score
}

func (d *Detector) getAnalysisUniverse() []string {
	// Mock universe - in production would come from config or market data
	return []string{
		"BTC-USD", "ETH-USD", "SOL-USD", "ADA-USD", "DOT-USD",
		"MATIC-USD", "AVAX-USD", "ATOM-USD", "NEAR-USD", "FTM-USD",
		"ALGO-USD", "XLM-USD", "VET-USD", "HBAR-USD", "ICP-USD",
	}
}

func (d *Detector) getCurrentRegime() string {
	// Mock regime - in production would integrate with regime detector
	regimes := []string{"TRENDING", "CHOPPY", "HIGH_VOL"}
	return regimes[int(time.Now().Unix())%len(regimes)]
}

func (d *Detector) calculateVADR(symbol string) float64 {
	// Mock VADR calculation - would integrate with microstructure system
	base := 1.5 + (float64(len(symbol)) * 0.1)
	return math.Min(base, 3.0)
}

func (d *Detector) isDataStale(symbol string) bool {
	// Mock staleness check - would check actual data timestamps
	return false
}

func (d *Detector) sortCandidatesByScore(candidates []Candidate) {
	// Simple bubble sort for small arrays
	n := len(candidates)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if candidates[j].Score < candidates[j+1].Score {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}
}