package endpoints

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	httpContracts "cryptorun/internal/interfaces/http"
	"cryptorun/internal/metrics"
	"github.com/rs/zerolog/log"
)

// CandidatesHandler returns the top composite candidates with gate status
func CandidatesHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse query parameters
		limitStr := r.URL.Query().Get("n")
		if limitStr == "" {
			limitStr = "50" // Default to 50 candidates
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 200 {
			errorResp := httpContracts.ErrorResponse{
				Error:     "invalid_parameter",
				Message:   "Parameter 'n' must be an integer between 1 and 200",
				Details:   "limit=" + limitStr,
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		// Get current regime from metrics
		currentRegime := getCurrentRegime(collector)

		// Generate mock candidates data - in production this would come from the scan pipeline
		candidates := generateMockCandidates(limit, currentRegime)

		// Calculate summary statistics
		summary := calculateCandidatesSummary(candidates)

		response := httpContracts.CandidatesResponse{
			Timestamp:  time.Now(),
			Regime:     currentRegime,
			TotalCount: len(candidates),
			Requested:  limit,
			Candidates: candidates,
			Summary:    summary,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		// Log performance
		duration := time.Since(requestStart)
		log.Debug().
			Dur("duration", duration).
			Int("candidates_count", len(candidates)).
			Str("regime", currentRegime).
			Msg("Candidates endpoint served")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error().Err(err).Msg("Failed to encode candidates response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// getCurrentRegime determines the current market regime
func getCurrentRegime(collector *metrics.Collector) string {
	// In production, this would come from the regime detector
	// For now, return a mock regime based on time of day for variability
	hour := time.Now().Hour()
	switch {
	case hour >= 9 && hour < 16: // Market hours
		return "trending_bull"
	case hour >= 16 && hour < 21: // Evening
		return "choppy"
	default: // Night/early morning
		return "high_vol"
	}
}

// generateMockCandidates creates mock candidate data for testing
func generateMockCandidates(limit int, regime string) []httpContracts.CandidateRecord {
	symbols := []string{
		"BTC-USD", "ETH-USD", "SOL-USD", "ADA-USD", "DOT-USD", "AVAX-USD",
		"MATIC-USD", "ALGO-USD", "ATOM-USD", "LINK-USD", "UNI-USD", "AAVE-USD",
		"FIL-USD", "LTC-USD", "BCH-USD", "XLM-USD", "ETC-USD", "MANA-USD",
		"SAND-USD", "CRV-USD", "BAL-USD", "COMP-USD", "YFI-USD", "SNX-USD",
		"GRT-USD", "ENJ-USD", "CHZ-USD", "BAT-USD", "ZRX-USD", "REP-USD",
	}

	candidates := make([]httpContracts.CandidateRecord, 0, limit)

	for i := 0; i < limit && i < len(symbols); i++ {
		symbol := symbols[i]

		// Generate scores based on regime
		baseScore := 45.0 + float64(limit-i)*0.8 + (float64(i%10) * 2.5)
		if regime == "trending_bull" {
			baseScore += 10.0 // Boost scores in trending markets
		}

		// Create attribution based on regime weights
		attribution := generateAttribution(baseScore, regime)

		// Create microstructure data
		microstructure := generateMicrostructure(symbol)

		// Evaluate gates based on score and microstructure
		gateStatus := evaluateGates(baseScore, microstructure, attribution)

		candidate := httpContracts.CandidateRecord{
			Symbol:         symbol,
			Exchange:       "kraken",
			Score:          baseScore,
			Rank:           i + 1,
			GateStatus:     gateStatus,
			Microstructure: microstructure,
			Attribution:    attribution,
			LastUpdated:    time.Now().Add(-time.Duration(i*30) * time.Second),
		}

		candidates = append(candidates, candidate)
	}

	return candidates
}

// generateAttribution creates mock attribution data based on regime
func generateAttribution(baseScore float64, regime string) httpContracts.Attribution {
	// Get regime weights
	weights := getRegimeWeights(regime)

	// Generate component scores that sum to baseScore
	momentumScore := (baseScore * weights["momentum"] / 100.0)
	technicalScore := (baseScore * weights["technical"] / 100.0)
	volumeScore := (baseScore * weights["volume"] / 100.0)
	qualityScore := (baseScore * weights["quality"] / 100.0)

	// Social bonus is added separately (capped at +10)
	socialBonus := min(weights["catalyst"]*0.3, 10.0)

	return httpContracts.Attribution{
		MomentumScore:  momentumScore,
		TechnicalScore: technicalScore,
		VolumeScore:    volumeScore,
		QualityScore:   qualityScore,
		SocialBonus:    socialBonus,
		WeightProfile:  regime,
	}
}

// getRegimeWeights returns the weight profile for a regime
func getRegimeWeights(regime string) map[string]float64 {
	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		return map[string]float64{
			"momentum": 50.0, "technical": 20.0, "volume": 15.0,
			"quality": 10.0, "catalyst": 5.0,
		}
	case "choppy", "chop", "ranging":
		return map[string]float64{
			"momentum": 35.0, "technical": 30.0, "volume": 15.0,
			"quality": 15.0, "catalyst": 5.0,
		}
	case "high_vol", "volatile", "high_volatility", "highvol":
		return map[string]float64{
			"momentum": 30.0, "technical": 25.0, "volume": 20.0,
			"quality": 20.0, "catalyst": 5.0,
		}
	default:
		return map[string]float64{
			"momentum": 35.0, "technical": 30.0, "volume": 15.0,
			"quality": 15.0, "catalyst": 5.0,
		}
	}
}

// generateMicrostructure creates mock microstructure data
func generateMicrostructure(symbol string) httpContracts.MicrostructureData {
	// Generate realistic microstructure data based on symbol
	priceLevel := 50000.0 // Base price
	if strings.Contains(symbol, "ETH") {
		priceLevel = 3000.0
	} else if strings.Contains(symbol, "SOL") {
		priceLevel = 150.0
	} else if !strings.Contains(symbol, "BTC") {
		priceLevel = 5.0
	}

	spread := 0.02 + (float64(len(symbol)%10) * 0.005) // 2-7bps
	vadr := 1.5 + (float64(len(symbol)%5) * 0.4)       // 1.5-3.1

	return httpContracts.MicrostructureData{
		SpreadBps: spread * 100, // Convert to basis points
		DepthUSD:  150000.0 + float64(len(symbol)*10000),
		VADR:      vadr,
		Volume24h: 1000000.0 + float64(len(symbol)*100000),
		LastPrice: priceLevel,
		BidPrice:  priceLevel * (1 - spread/2),
		AskPrice:  priceLevel * (1 + spread/2),
	}
}

// evaluateGates determines gate status for a candidate
func evaluateGates(score float64, micro httpContracts.MicrostructureData, attr httpContracts.Attribution) httpContracts.GateStatus {
	scoreGate := score >= 75.0
	vadrGate := micro.VADR >= 1.8
	fundingGate := true // Mock funding divergence check
	spreadGate := micro.SpreadBps < 50.0
	depthGate := micro.DepthUSD >= 100000.0
	fatigueGate := true   // Mock fatigue check
	freshnessGate := true // Mock freshness check

	overallPassed := scoreGate && vadrGate && fundingGate && spreadGate && depthGate && fatigueGate && freshnessGate

	failureReasons := []string{}
	if !scoreGate {
		failureReasons = append(failureReasons, "score below 75")
	}
	if !vadrGate {
		failureReasons = append(failureReasons, "VADR below 1.8")
	}
	if !spreadGate {
		failureReasons = append(failureReasons, "spread above 50bps")
	}
	if !depthGate {
		failureReasons = append(failureReasons, "depth below $100k")
	}

	return httpContracts.GateStatus{
		ScoreGate:      scoreGate,
		VADRGate:       vadrGate,
		FundingGate:    fundingGate,
		SpreadGate:     spreadGate,
		DepthGate:      depthGate,
		FatigueGate:    fatigueGate,
		FreshnessGate:  freshnessGate,
		OverallPassed:  overallPassed,
		FailureReasons: failureReasons,
	}
}

// calculateCandidatesSummary generates summary statistics
func calculateCandidatesSummary(candidates []httpContracts.CandidateRecord) httpContracts.CandidatesSummary {
	if len(candidates) == 0 {
		return httpContracts.CandidatesSummary{}
	}

	totalScore := 0.0
	passedAll := 0
	gateStats := make(map[string]int)

	for _, candidate := range candidates {
		totalScore += candidate.Score
		if candidate.GateStatus.OverallPassed {
			passedAll++
		}

		// Count gate passes
		if candidate.GateStatus.ScoreGate {
			gateStats["score"]++
		}
		if candidate.GateStatus.VADRGate {
			gateStats["vadr"]++
		}
		if candidate.GateStatus.FundingGate {
			gateStats["funding"]++
		}
		if candidate.GateStatus.SpreadGate {
			gateStats["spread"]++
		}
		if candidate.GateStatus.DepthGate {
			gateStats["depth"]++
		}
		if candidate.GateStatus.FatigueGate {
			gateStats["fatigue"]++
		}
		if candidate.GateStatus.FreshnessGate {
			gateStats["freshness"]++
		}
	}

	avgScore := totalScore / float64(len(candidates))

	// Calculate median (simplified)
	medianScore := avgScore // In production would properly calculate median

	// Top decile threshold (90th percentile)
	topDecileIdx := len(candidates) / 10
	topDecileThreshold := candidates[0].Score
	if topDecileIdx < len(candidates) {
		topDecileThreshold = candidates[topDecileIdx].Score
	}

	total := float64(len(candidates))

	return httpContracts.CandidatesSummary{
		PassedAllGates:     passedAll,
		AvgScore:           avgScore,
		MedianScore:        medianScore,
		TopDecileThreshold: topDecileThreshold,
		GatePassRates: httpContracts.GatePassRates{
			Score:     float64(gateStats["score"]) / total,
			VADR:      float64(gateStats["vadr"]) / total,
			Funding:   float64(gateStats["funding"]) / total,
			Spread:    float64(gateStats["spread"]) / total,
			Depth:     float64(gateStats["depth"]) / total,
			Fatigue:   float64(gateStats["fatigue"]) / total,
			Freshness: float64(gateStats["freshness"]) / total,
		},
	}
}

// min returns the smaller of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
