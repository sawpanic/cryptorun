package signals

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/composite"
	"github.com/sawpanic/cryptorun/internal/gates"
	"github.com/sawpanic/cryptorun/internal/domain/guards"
	"github.com/sawpanic/cryptorun/internal/microstructure"
)

type Scanner struct {
	compositeScorer *composite.Scorer
	entryGates     *gates.EntryGateEvaluator
	guards         *guards.Manager
	microstructure microstructure.Evaluator
}

type ScanResults struct {
	Candidates []Candidate `json:"candidates"`
	Timestamp  time.Time   `json:"timestamp"`
	ScanType   string      `json:"scan_type"`
	Universe   int         `json:"universe_size"`
}

type Candidate struct {
	Symbol      string      `json:"symbol"`
	Score       float64     `json:"score"`
	Attribution Attribution `json:"attribution"`
	Gates       GateStatus  `json:"gates"`
	Factors     Factors     `json:"factors"`
}

type Attribution struct {
	Fresh       bool   `json:"fresh"`
	DepthOK     bool   `json:"depth_ok"`
	Venue       string `json:"venue"`
	SourceCount int    `json:"source_count"`
	LatencyMs   int    `json:"latency_ms"`
}

type GateStatus struct {
	ScoreGate      bool    `json:"score_gate"`       // Score >= 75
	VADRGate       bool    `json:"vadr_gate"`        // VADR >= 1.8
	FundingGate    bool    `json:"funding_gate"`     // Funding divergence >= 2σ
	FreshnessGate  bool    `json:"freshness_gate"`   // <= 2 bars old
	LateFillGate   bool    `json:"late_fill_gate"`   // < 30s after bar close
	FatigueGate    bool    `json:"fatigue_gate"`     // Not fatigued (24h>12% & RSI4h>70)
}

type Factors struct {
	MomentumCore float64 `json:"momentum_core"`
	Technical    float64 `json:"technical_residual"`
	Volume       float64 `json:"volume_residual"`
	Quality      float64 `json:"quality_residual"`
	Social       float64 `json:"social_capped"`
}

func NewScanner() *Scanner {
	return &Scanner{
		compositeScorer: composite.NewScorer(),
		entryGates:     gates.NewEntryGateEvaluator(nil, nil, nil, nil), // Mock dependencies
		guards:         guards.NewManager(),
		microstructure: microstructure.NewEvaluator(),
	}
}

func (s *Scanner) ScanUniverse(scanType string) (*ScanResults, error) {
	startTime := time.Now()
	
	// Get universe based on scan type
	universe := s.getUniverse(scanType)
	
	results := &ScanResults{
		Timestamp:  startTime,
		ScanType:   scanType,
		Universe:   len(universe),
		Candidates: make([]Candidate, 0, len(universe)),
	}
	
	fmt.Printf("🔍 Scanning %d symbols (%s mode)...\n", len(universe), scanType)
	
	processed := 0
	passed := 0
	
	for _, symbol := range universe {
		processed++
		
		// Mock guard evaluation
		guardResult := map[string]interface{}{
			"FreshnessOK": true,
			"LateFillOK": true,
		}
		
		// Mock microstructure evaluation
		microResult := map[string]interface{}{
			"VADR": 2.1,
			"DepthOK": true,
			"PrimaryVenue": "kraken",
			"Sources": []string{"kraken", "binance"},
		}
		
		// Mock composite score calculation
		scoreResult := map[string]interface{}{
			"CompositeScore": 80.0,
			"MomentumCore": 65.0,
		}
		
		// Mock fatigue check
		fatigueOK := true
		
		// Check entry gates
		gates := s.evaluateGates(scoreResult, microResult, guardResult, fatigueOK)
		
		// Require 2 of 3 core gates (score, VADR, funding) plus all guard gates
		coreGatesPassed := 0
		if gates.ScoreGate { coreGatesPassed++ }
		if gates.VADRGate { coreGatesPassed++ }  
		if gates.FundingGate { coreGatesPassed++ }
		
		allGuardsPass := gates.FreshnessGate && gates.LateFillGate && gates.FatigueGate
		
		if coreGatesPassed >= 2 && allGuardsPass {
			candidate := Candidate{
				Symbol: symbol,
				Score:  80.0, // Mock score
				Attribution: Attribution{
					Fresh:       true,
					DepthOK:     true,
					Venue:       "kraken",
					SourceCount: 2,
					LatencyMs:   int(time.Since(startTime).Milliseconds()),
				},
				Gates: gates,
				Factors: Factors{
					MomentumCore: 65.0,
					Technical:    8.0,
					Volume:       5.0,
					Quality:      2.0,
					Social:       0.0,
				},
			}
			
			results.Candidates = append(results.Candidates, candidate)
			passed++
		}
	}
	
	// Sort by composite score descending
	s.sortCandidatesByScore(results.Candidates)
	
	fmt.Printf("✅ Scan complete: %d processed, %d passed gates\n", processed, passed)
	
	return results, nil
}

func (s *Scanner) getUniverse(scanType string) []string {
	// Mock universe - in production would come from config/providers
	baseUniverse := []string{
		"BTC-USD", "ETH-USD", "SOL-USD", "ADA-USD", "DOT-USD",
		"MATIC-USD", "AVAX-USD", "ATOM-USD", "NEAR-USD", "FTM-USD",
		"ALGO-USD", "XLM-USD", "VET-USD", "HBAR-USD", "ICP-USD",
		"FLOW-USD", "EGLD-USD", "ONE-USD", "ROSE-USD", "SAND-USD",
	}
	
	switch scanType {
	case "hot":
		// Hot scan focuses on top 10 most liquid
		return baseUniverse[:10]
	case "warm":
		// Warm scan covers broader universe
		return baseUniverse
	default:
		return baseUniverse[:5] // Conservative default
	}
}

func (s *Scanner) evaluateGates(score interface{}, micro interface{}, guard interface{}, fatigueOK bool) GateStatus {
	return GateStatus{
		ScoreGate:     true,  // Mock gate result
		VADRGate:      true,  // Mock gate result
		FundingGate:   true,  // Mock gate result
		FreshnessGate: true,  // Mock gate result
		LateFillGate:  true,  // Mock gate result
		FatigueGate:   fatigueOK,
	}
}

func (s *Scanner) sortCandidatesByScore(candidates []Candidate) {
	// Simple bubble sort for small arrays - sufficient for top-N results
	n := len(candidates)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if candidates[j].Score < candidates[j+1].Score {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}
}