package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/exchanges/kraken"
	"github.com/sawpanic/cryptorun/internal/data/interfaces"
	"github.com/sawpanic/cryptorun/internal/domain/guards"
	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/internal/domain/scoring"
)

// LiveScanResult represents a complete scan result with real market data
type LiveScanResult struct {
	Symbol           string            `json:"symbol"`
	Venue            string            `json:"venue"`
	Score            float64           `json:"score"`
	MomentumCore     float64           `json:"momentum_core"`
	TechnicalScore   float64           `json:"technical_score"`
	VolumeScore      float64           `json:"volume_score"`
	QualityScore     float64           `json:"quality_score"`
	SocialScore      float64           `json:"social_score"`
	Price            float64           `json:"price"`
	Volume24h        float64           `json:"volume_24h"`
	SpreadBps        float64           `json:"spread_bps"`
	DepthUSD         float64           `json:"depth_usd"`
	VADR             float64           `json:"vadr"`
	PassesGates      bool              `json:"passes_gates"`
	GateResults      map[string]bool   `json:"gate_results"`
	Timestamp        time.Time         `json:"timestamp"`
	DataAge          time.Duration     `json:"data_age"`
	Attribution      map[string]string `json:"attribution"`
}

// LiveScanSummary provides overview of the live scan
type LiveScanSummary struct {
	TotalSymbols     int                        `json:"total_symbols"`
	CandidatesFound  int                        `json:"candidates_found"`
	Regime           string                     `json:"regime"`
	ScanDuration     time.Duration              `json:"scan_duration"`
	Timestamp        time.Time                  `json:"timestamp"`
	VenueHealth      map[string]string          `json:"venue_health"`
	GatesPassed      map[string]int             `json:"gates_passed"`
	Errors           []string                   `json:"errors,omitempty"`
}

var krakenUSDPairs = []string{
	"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "LINKUSD",
	"DOTUSD", "MATICUSD", "AVAXUSD", "UNIUSD", "LTCUSD", "XRPUSD",
}

func main() {
	ctx := context.Background()
	
	fmt.Println("🔴 LIVE SCAN: CryptoRun D3 Entry Gates Integration")
	fmt.Println("⚠️  Using REAL Kraken APIs - respect rate limits!")
	fmt.Println()

	// Initialize Kraken provider
	exchange := kraken.NewAdapter("kraken-live")
	
	// Check venue health first
	health := exchange.Health()
	fmt.Printf("🏥 Venue Health: %s (%s)\n", health.Status, health.Recommendation)
	
	if health.Status == "unhealthy" {
		log.Fatal("❌ Kraken is unhealthy, aborting live scan")
	}

	// Initialize scoring system with trending_bull regime
	weightsConfig := regime.WeightsConfig{
		Trending: regime.RegimeWeights{
			Description:  "Trending Bull Market",
			MomentumCore: 0.40,
			Technical:    0.25, 
			Volume:       0.20,
			Quality:      0.10,
			Social:       0.05,
		},
	}
	
	scorer := scoring.NewCompositeScorer(weightsConfig)
	
	// Initialize microstructure checker and guards
	microChecker := microstructure.NewChecker()
	guardEvaluator := guards.NewEvaluator()
	
	results := make([]LiveScanResult, 0, len(krakenUSDPairs))
	var errors []string
	
	fmt.Printf("🔍 Scanning %d USD pairs with live Kraken data...\n\n", len(krakenUSDPairs))
	
	startTime := time.Now()
	
	for i, symbol := range krakenUSDPairs {
		fmt.Printf("[%d/%d] %s: ", i+1, len(krakenUSDPairs), symbol)
		
		result, err := scanSymbolLive(ctx, exchange, scorer, microChecker, guardEvaluator, symbol)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			errors = append(errors, fmt.Sprintf("%s: %v", symbol, err))
			continue
		}
		
		results = append(results, result)
		
		// Display result with gates status
		gateIcon := "❌"
		if result.PassesGates {
			gateIcon = "✅"
		}
		
		fmt.Printf("%s Score=%.1f, Spread=%.1fbps, VADR=%.2f %s\n", 
			gateIcon, result.Score, result.SpreadBps, result.VADR, result.Attribution["source"])
		
		// Rate limiting - be respectful to Kraken
		time.Sleep(2 * time.Second)
	}
	
	scanDuration := time.Since(startTime)
	
	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	// Count candidates that pass all gates
	candidates := 0
	gatesPassed := make(map[string]int)
	for _, result := range results {
		if result.PassesGates {
			candidates++
		}
		for gate, passed := range result.GateResults {
			if passed {
				gatesPassed[gate]++
			}
		}
	}
	
	// Create summary
	summary := LiveScanSummary{
		TotalSymbols:    len(krakenUSDPairs),
		CandidatesFound: candidates,
		Regime:          "trending_bull",
		ScanDuration:    scanDuration,
		Timestamp:       time.Now(),
		VenueHealth: map[string]string{
			"kraken": health.Status,
		},
		GatesPassed: gatesPassed,
		Errors:      errors,
	}
	
	// Ensure output directory exists
	outputDir := "out/live_scan"
	os.MkdirAll(outputDir, 0755)
	
	// Write candidates to JSONL
	candidatesFile := filepath.Join(outputDir, "live_candidates.jsonl")
	f, err := os.Create(candidatesFile)
	if err != nil {
		log.Fatal("Failed to create candidates file:", err)
	}
	defer f.Close()
	
	for _, result := range results {
		if result.PassesGates {
			data, _ := json.Marshal(result)
			f.Write(data)
			f.Write([]byte("\n"))
		}
	}
	
	// Write summary to JSON
	summaryFile := filepath.Join(outputDir, "live_scan_summary.json")
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(summaryFile, summaryData, 0644)
	
	// Display final results
	fmt.Printf("\n🏁 Live Scan Complete (%.1fs)\n", scanDuration.Seconds())
	fmt.Printf("📊 Results: %d candidates from %d symbols\n", candidates, len(results))
	fmt.Printf("📁 Output: %s\n", outputDir)
	
	if candidates > 0 {
		fmt.Printf("\n🎯 Top Candidates:\n")
		count := 0
		for _, result := range results {
			if result.PassesGates {
				fmt.Printf("  %s: %.1f (Spread=%.1fbps, VADR=%.2f)\n", 
					result.Symbol, result.Score, result.SpreadBps, result.VADR)
				count++
				if count >= 5 { // Show top 5
					break
				}
			}
		}
	}
}

func scanSymbolLive(ctx context.Context, exchange interfaces.Exchange, scorer *scoring.CompositeScorer, 
	microChecker *microstructure.Checker, guardEvaluator *guards.Evaluator, symbol string) (LiveScanResult, error) {
	
	result := LiveScanResult{
		Symbol:      symbol,
		Venue:       exchange.Name(),
		Timestamp:   time.Now(),
		Attribution: make(map[string]string),
		GateResults: make(map[string]bool),
	}
	
	// Get L2 order book for microstructure analysis
	book, err := exchange.GetBookL2(ctx, symbol)
	if err != nil {
		return result, fmt.Errorf("failed to get order book: %w", err)
	}
	
	result.DataAge = time.Since(book.Timestamp)
	result.Attribution["source"] = fmt.Sprintf("live_%s_%s", exchange.Name(), book.Timestamp.Format("15:04:05"))
	
	// Calculate microstructure metrics
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return result, fmt.Errorf("empty order book")
	}
	
	bestBid := book.Bids[0].Price
	bestAsk := book.Asks[0].Price
	midPrice := (bestBid + bestAsk) / 2.0
	result.Price = midPrice
	
	// Calculate spread in basis points
	spread := (bestAsk - bestBid) / midPrice
	result.SpreadBps = spread * 10000
	
	// Calculate depth within ±2%
	depthBid, depthAsk := calculateDepth(book, midPrice, 2.0)
	result.DepthUSD = (depthBid + depthAsk) / 2.0
	
	// Estimate VADR (simplified for live demo)
	result.VADR = estimateVADR(book, midPrice)
	
	// Generate mock scores for demonstration (in real system, would use klines data)
	result.Score = generateMockScore(symbol, "trending_bull")
	result.MomentumCore = result.Score * 0.40
	result.TechnicalScore = result.Score * 0.25  
	result.VolumeScore = result.Score * 0.20
	result.QualityScore = result.Score * 0.10
	result.SocialScore = result.Score * 0.05
	result.Volume24h = float64(len(symbol) * 1000000) // Mock volume
	
	// Evaluate entry gates
	result.GateResults["score_threshold"] = result.Score >= 75.0
	result.GateResults["spread_limit"] = result.SpreadBps < 50.0
	result.GateResults["depth_minimum"] = result.DepthUSD >= 100000.0
	result.GateResults["vadr_threshold"] = result.VADR >= 1.75
	
	// Overall gate pass status
	result.PassesGates = true
	for _, passed := range result.GateResults {
		if !passed {
			result.PassesGates = false
			break
		}
	}
	
	return result, nil
}

func calculateDepth(book *interfaces.BookL2, midPrice, percentRange float64) (bidDepth, askDepth float64) {
	lowerBound := midPrice * (1 - percentRange/100)
	upperBound := midPrice * (1 + percentRange/100)
	
	// Sum bid depth within range
	for _, bid := range book.Bids {
		if bid.Price >= lowerBound {
			bidDepth += bid.Price * bid.Size
		}
	}
	
	// Sum ask depth within range  
	for _, ask := range book.Asks {
		if ask.Price <= upperBound {
			askDepth += ask.Price * ask.Size
		}
	}
	
	return bidDepth, askDepth
}

func estimateVADR(book *interfaces.BookL2, midPrice float64) float64 {
	// Simplified VADR estimation based on order book depth
	// Real implementation would need historical volume data
	
	if len(book.Bids) < 5 || len(book.Asks) < 5 {
		return 0.5 // Low VADR for thin books
	}
	
	// Use depth as proxy for volume adequacy
	bidDepth, askDepth := calculateDepth(book, midPrice, 1.0)
	totalDepth := bidDepth + askDepth
	
	if totalDepth > 500000 { // >$500k depth = good VADR
		return 2.5
	} else if totalDepth > 100000 { // >$100k depth = moderate VADR
		return 1.8
	} else {
		return 1.0 // Thin depth = low VADR
	}
}

func generateMockScore(symbol, regime string) float64 {
	// Generate deterministic mock score based on symbol
	seed := int64(0)
	for _, char := range symbol {
		seed += int64(char)
	}
	
	// Use time component for some variation in live testing
	timeSeed := time.Now().Unix() % 100
	seed += timeSeed
	
	baseScore := 40.0 + float64(seed%40)
	
	if regime == "trending_bull" {
		baseScore += 15.0
	}
	
	return baseScore
}