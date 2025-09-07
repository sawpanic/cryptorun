package premove

import (
	"math/rand"
	"time"
)

// Gate A: Funding Divergence Analyzer
type FundingAnalyzer struct{}

type FundingResult struct {
	Passed        bool    `json:"passed"`
	ZScore        float64 `json:"z_score"`
	SpotVWAPRatio float64 `json:"spot_vwap_ratio"`
	SpotCVD       float64 `json:"spot_cvd"`
	PerpCVD       float64 `json:"perp_cvd"`
}

func NewFundingAnalyzer() *FundingAnalyzer {
	return &FundingAnalyzer{}
}

func (fa *FundingAnalyzer) AnalyzeFunding(symbol string) *FundingResult {
	// Mock funding analysis - in production would analyze real funding rates
	rand.Seed(time.Now().UnixNano() + int64(len(symbol)))
	
	// Simulate venue-median funding z-score
	fundingZ := (rand.Float64() - 0.5) * 6.0 // Range: -3 to +3
	
	// Simulate spot vs VWAP ratio
	spotVWAPRatio := 0.98 + (rand.Float64() * 0.04) // Range: 0.98 to 1.02
	
	// Simulate CVD values
	spotCVD := (rand.Float64() - 0.5) * 2000 // Range: -1000 to +1000
	perpCVD := (rand.Float64() - 0.5) * 2000 // Range: -1000 to +1000
	
	// Gate A criteria:
	// 1. funding_z < -1.5 AND spot >= VWAP(24h)
	// 2. Confirm: spot_cvd >= 0 OR perp_cvd <= 0
	fundingCriteria := fundingZ < -1.5 && spotVWAPRatio >= 1.0
	cvdConfirmation := spotCVD >= 0 || perpCVD <= 0
	
	passed := fundingCriteria && cvdConfirmation
	
	return &FundingResult{
		Passed:        passed,
		ZScore:        fundingZ,
		SpotVWAPRatio: spotVWAPRatio,
		SpotCVD:       spotCVD,
		PerpCVD:       perpCVD,
	}
}

// Gate B: Supply Squeeze Analyzer
type SupplyAnalyzer struct{}

type SupplyResult struct {
	Passed       bool    `json:"passed"`
	WeeklyChange float64 `json:"weekly_change"`
	VenueCount   int     `json:"venue_count"`
	ProxyCount   int     `json:"proxy_count"`
	ProxyPassed  int     `json:"proxy_passed"`
}

func NewSupplyAnalyzer() *SupplyAnalyzer {
	return &SupplyAnalyzer{}
}

func (sa *SupplyAnalyzer) AnalyzeSupply(symbol string) *SupplyResult {
	// Mock supply analysis - in production would check exchange reserves
	rand.Seed(time.Now().UnixNano() + int64(len(symbol)*2))
	
	venueCount := 2 + rand.Intn(4) // 2-5 venues
	weeklyChange := (rand.Float64() - 0.7) * 20 // Bias toward negative change
	
	// Primary method: reserves_7d <= -5% across ≥3 venues
	primaryPassed := weeklyChange <= -5.0 && venueCount >= 3
	
	// Fallback: Supply squeeze proxy 2-of-4 if primary unavailable
	proxyChecks := []bool{
		rand.Float64() < 0.6, // Exchange outflow > inflow
		rand.Float64() < 0.5, // Stablecoin premium
		rand.Float64() < 0.4, // Futures basis widening  
		rand.Float64() < 0.3, // Options put/call skew
	}
	
	proxyPassed := 0
	for _, passed := range proxyChecks {
		if passed {
			proxyPassed++
		}
	}
	
	// Gate B passes if either primary OR 2-of-4 proxy
	passed := primaryPassed || proxyPassed >= 2
	
	return &SupplyResult{
		Passed:       passed,
		WeeklyChange: weeklyChange,
		VenueCount:   venueCount,
		ProxyCount:   len(proxyChecks),
		ProxyPassed:  proxyPassed,
	}
}

// Gate C: Whale Accumulation Analyzer  
type WhaleAnalyzer struct{}

type WhaleResult struct {
	Passed           bool    `json:"passed"`
	LargePrints      int     `json:"large_prints"`
	CVDResidual      float64 `json:"cvd_residual"`
	PriceDrift       float64 `json:"price_drift"`
	MakerPull        bool    `json:"maker_pull"`
	HotwalletDecline float64 `json:"hotwallet_decline"`
}

func NewWhaleAnalyzer() *WhaleAnalyzer {
	return &WhaleAnalyzer{}
}

func (wa *WhaleAnalyzer) AnalyzeWhale(symbol string) *WhaleResult {
	// Mock whale analysis - in production would analyze large trades and flows
	rand.Seed(time.Now().UnixNano() + int64(len(symbol)*3))
	
	largePrints := rand.Intn(8) // 0-7 large prints detected
	cvdResidual := (rand.Float64() - 0.3) * 1000 // Bias toward positive
	priceDrift := rand.Float64() * 1.0 // 0-1.0 ATR drift
	makerPull := rand.Float64() < 0.4 // 40% chance
	hotwalletDecline := rand.Float64() * 25 // 0-25% decline
	
	// Gate C: Whale 2-of-3 criteria
	checks := []bool{
		largePrints >= 3, // Large print clustering
		cvdResidual > 0 && priceDrift < 0.5, // CVD residual > 0 with price drift < 0.5×ATR
		makerPull || hotwalletDecline > 10, // Maker pull OR hotwallet decline > 10%
	}
	
	whaleScore := 0
	for _, passed := range checks {
		if passed {
			whaleScore++
		}
	}
	
	passed := whaleScore >= 2
	
	return &WhaleResult{
		Passed:           passed,
		LargePrints:      largePrints,
		CVDResidual:      cvdResidual,
		PriceDrift:       priceDrift,
		MakerPull:        makerPull,
		HotwalletDecline: hotwalletDecline,
	}
}

// Microstructure Gates
type MicrostructureGates struct{}

func NewMicrostructureGates() *MicrostructureGates {
	return &MicrostructureGates{}
}

func (mg *MicrostructureGates) EvaluateTier(symbol string) int {
	// Mock microstructure tier evaluation - in production would check ADV, VADR
	rand.Seed(time.Now().UnixNano() + int64(len(symbol)*4))
	
	// Simulate ADV (Average Daily Volume) in millions
	adv := 50 + rand.Float64()*200 // $50M - $250M ADV
	
	// Simulate VADR (Volume-Adjusted Daily Range)  
	vadr := 1.0 + rand.Float64()*2.0 // 1.0 - 3.0 VADR
	
	// Tier assignment based on liquidity metrics
	switch {
	case adv >= 100 && vadr >= 2.0:
		return 1 // Tier 1: Best liquidity
	case adv >= 50 && vadr >= 1.75:
		return 2 // Tier 2: Good liquidity  
	default:
		return 3 // Tier 3: Lower liquidity
	}
}