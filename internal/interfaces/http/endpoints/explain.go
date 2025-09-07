package endpoints

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	httpContracts "github.com/sawpanic/cryptorun/internal/interfaces/http"
	"github.com/rs/zerolog/log"
)

// ExplainHandler returns explainability information for a specific symbol
func ExplainHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract symbol from URL path - expects /explain/{symbol}
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 2 || pathParts[0] != "explain" {
			errorResp := httpContracts.ErrorResponse{
				Error:     "invalid_path",
				Message:   "Path should be /explain/{symbol}",
				Details:   "path=" + r.URL.Path,
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		symbol := strings.ToUpper(pathParts[1])

		// Validate symbol format
		if !isValidSymbol(symbol) {
			errorResp := httpContracts.ErrorResponse{
				Error:     "invalid_symbol",
				Message:   "Symbol must be in format XXX-USD (e.g., BTC-USD)",
				Details:   "symbol=" + symbol,
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		// Try to get explanation from artifacts first, then fall back to live data
		explanation, dataSource := getExplanation(symbol)
		if explanation == nil {
			errorResp := httpContracts.ErrorResponse{
				Error:     "symbol_not_found",
				Message:   "No explanation data available for symbol",
				Details:   "symbol=" + symbol,
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=60") // Cache for 1 minute

		// Add data source header
		w.Header().Set("X-Data-Source", dataSource)

		// Log performance
		duration := time.Since(requestStart)
		log.Debug().
			Dur("duration", duration).
			Str("symbol", symbol).
			Str("data_source", dataSource).
			Msg("Explain endpoint served")

		if err := json.NewEncoder(w).Encode(explanation); err != nil {
			log.Error().Err(err).Str("symbol", symbol).Msg("Failed to encode explanation response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// isValidSymbol validates symbol format (XXX-USD)
func isValidSymbol(symbol string) bool {
	parts := strings.Split(symbol, "-")
	return len(parts) == 2 && parts[1] == "USD" && len(parts[0]) >= 2 && len(parts[0]) <= 10
}

// getExplanation retrieves explanation data from artifacts or generates live data
func getExplanation(symbol string) (*httpContracts.ExplainResponse, string) {
	// First try to load from artifacts
	if explanation := loadFromArtifacts(symbol); explanation != nil {
		return explanation, "artifacts"
	}

	// Fall back to generating live explanation
	if explanation := generateLiveExplanation(symbol); explanation != nil {
		return explanation, "live"
	}

	return nil, ""
}

// loadFromArtifacts attempts to load explanation from artifact files
func loadFromArtifacts(symbol string) *httpContracts.ExplainResponse {
	// In production, this would look in C:\wallet\artifacts\ for JSON files
	// For now, return nil to always use live data
	return nil
}

// generateLiveExplanation creates explanation data for the symbol
func generateLiveExplanation(symbol string) *httpContracts.ExplainResponse {
	// Generate mock explanation - in production this would come from the scan pipeline
	timestamp := time.Now()

	// Generate base score and components
	baseScore := 65.0 + (float64(len(symbol)%10) * 3.5) // Vary by symbol
	regime := getCurrentRegimeForExplain()

	// Create detailed explanation
	explanation := &httpContracts.ExplainResponse{
		Symbol:      symbol,
		Exchange:    "kraken",
		Timestamp:   timestamp,
		DataSource:  "live",
		Score:       generateScoreExplanation(baseScore, regime),
		Gates:       generateGateExplanation(baseScore, symbol),
		Factors:     generateFactorExplanation(baseScore, regime),
		Regime:      generateRegimeExplanation(regime),
		Attribution: generateAttributionExplanation(baseScore, regime),
	}

	return explanation
}

// getCurrentRegimeForExplain gets current regime for explanation
func getCurrentRegimeForExplain() string {
	// Use same logic as candidates endpoint
	hour := time.Now().Hour()
	switch {
	case hour >= 9 && hour < 16:
		return "trending_bull"
	case hour >= 16 && hour < 21:
		return "choppy"
	default:
		return "high_vol"
	}
}

// generateScoreExplanation creates detailed score breakdown
func generateScoreExplanation(baseScore float64, regime string) httpContracts.ScoreExplanation {
	weights := getRegimeWeights(regime)

	// Generate pre-orthogonal scores
	preOrth := map[string]float64{
		"momentum":  baseScore * 0.4,
		"technical": baseScore * 0.35,
		"volume":    baseScore * 0.25,
		"quality":   baseScore * 0.3,
		"social":    8.5,
	}

	// Simulate Gram-Schmidt orthogonalization (except momentum)
	postOrth := map[string]float64{
		"momentum":  preOrth["momentum"], // Protected
		"technical": preOrth["technical"] * 0.85,
		"volume":    preOrth["volume"] * 0.72,
		"quality":   preOrth["quality"] * 0.68,
		"social":    min(preOrth["social"], 10.0), // Capped
	}

	// Apply weights
	weightedScores := map[string]float64{
		"momentum":  postOrth["momentum"] * weights["momentum"] / 100.0,
		"technical": postOrth["technical"] * weights["technical"] / 100.0,
		"volume":    postOrth["volume"] * weights["volume"] / 100.0,
		"quality":   postOrth["quality"] * weights["quality"] / 100.0,
	}

	finalScore := weightedScores["momentum"] + weightedScores["technical"] +
		weightedScores["volume"] + weightedScores["quality"] + postOrth["social"]

	// Generate calculation steps
	steps := []httpContracts.CalculationStep{
		{
			Step:        "raw_factors",
			Description: "Initial factor calculations",
			Input:       0.0,
			Output:      baseScore,
			Applied:     "market_data",
		},
		{
			Step:        "orthogonalization",
			Description: "Gram-Schmidt orthogonalization (momentum protected)",
			Input:       baseScore,
			Output:      baseScore * 0.92,
			Applied:     "gram_schmidt",
		},
		{
			Step:        "regime_weights",
			Description: "Apply regime-specific weights",
			Input:       baseScore * 0.92,
			Output:      finalScore - postOrth["social"],
			Applied:     regime + "_weights",
		},
		{
			Step:        "social_cap",
			Description: "Add social bonus (capped at +10)",
			Input:       finalScore - postOrth["social"],
			Output:      finalScore,
			Applied:     "+10_cap",
		},
	}

	return httpContracts.ScoreExplanation{
		FinalScore:       finalScore,
		PreOrthogonal:    preOrth,
		PostOrthogonal:   postOrth,
		WeightedScores:   weightedScores,
		SocialBonus:      postOrth["social"],
		CalculationSteps: steps,
	}
}

// generateGateExplanation creates detailed gate evaluation
func generateGateExplanation(score float64, symbol string) httpContracts.GateExplanation {
	timestamp := time.Now()

	// Generate mock microstructure values
	spread := 0.25 + (float64(len(symbol)%5) * 0.05) // 25-45bps
	vadr := 1.6 + (float64(len(symbol)%7) * 0.3)     // 1.6-3.4
	depth := 120000.0 + float64(len(symbol)*5000)    // $120k-$270k

	scoreGate := httpContracts.GateDetail{
		Passed:      score >= 75.0,
		Threshold:   75.0,
		ActualValue: score,
		Margin:      score - 75.0,
		Description: "Composite score must be >= 75",
		LastChecked: timestamp,
	}

	vadrGate := httpContracts.GateDetail{
		Passed:      vadr >= 1.8,
		Threshold:   1.8,
		ActualValue: vadr,
		Margin:      vadr - 1.8,
		Description: "VADR (Volume-Adjusted Daily Range) >= 1.8x",
		LastChecked: timestamp,
	}

	spreadGate := httpContracts.GateDetail{
		Passed:      spread < 0.50,
		Threshold:   0.50,
		ActualValue: spread,
		Margin:      0.50 - spread,
		Description: "Spread must be < 50bps",
		LastChecked: timestamp,
	}

	depthGate := httpContracts.GateDetail{
		Passed:      depth >= 100000.0,
		Threshold:   100000.0,
		ActualValue: depth,
		Margin:      depth - 100000.0,
		Description: "Depth >= $100k within ±2%",
		LastChecked: timestamp,
	}

	// Mock other gates as passing
	fundingGate := httpContracts.GateDetail{
		Passed:      true,
		Threshold:   2.0,
		ActualValue: 2.3,
		Margin:      0.3,
		Description: "Funding divergence >= 2σ",
		LastChecked: timestamp,
	}

	fatigueGate := httpContracts.GateDetail{
		Passed:      true,
		Threshold:   70.0,
		ActualValue: 45.0,
		Margin:      25.0,
		Description: "RSI4h < 70 or acceleration increasing",
		LastChecked: timestamp,
	}

	freshnessGate := httpContracts.GateDetail{
		Passed:      true,
		Threshold:   2.0,
		ActualValue: 1.0,
		Margin:      1.0,
		Description: "Signal <= 2 bars old and within 1.2x ATR(1h)",
		LastChecked: timestamp,
	}

	overall := scoreGate.Passed && vadrGate.Passed && fundingGate.Passed &&
		spreadGate.Passed && depthGate.Passed && fatigueGate.Passed && freshnessGate.Passed

	return httpContracts.GateExplanation{
		Overall:        overall,
		ScoreGate:      scoreGate,
		VADRGate:       vadrGate,
		FundingGate:    fundingGate,
		SpreadGate:     spreadGate,
		DepthGate:      depthGate,
		FatigueGate:    fatigueGate,
		FreshnessGate:  freshnessGate,
		EvaluationTime: timestamp,
	}
}

// generateFactorExplanation creates detailed factor breakdown
func generateFactorExplanation(baseScore float64, regime string) httpContracts.FactorExplanation {
	dataAge := time.Duration(30) * time.Second

	// Momentum (protected from orthogonalization)
	momentumCore := httpContracts.MomentumFactorDetail{
		RawScore:      baseScore * 0.4,
		Weight:        getRegimeWeights(regime)["momentum"],
		WeightedScore: baseScore * 0.4 * getRegimeWeights(regime)["momentum"] / 100.0,
		Timeframes: map[string]float64{
			"1h":  15.2,
			"4h":  22.8,
			"12h": 18.5,
			"24h": 25.1,
		},
		Protected:  true,
		DataAge:    dataAge,
		Confidence: 0.87,
	}

	// Technical (orthogonalized)
	technical := httpContracts.FactorDetail{
		RawScore:        baseScore * 0.35,
		OrthogonalScore: baseScore * 0.35 * 0.85, // Reduced by orthogonalization
		Weight:          getRegimeWeights(regime)["technical"],
		WeightedScore:   baseScore * 0.35 * 0.85 * getRegimeWeights(regime)["technical"] / 100.0,
		Components: map[string]float64{
			"rsi_4h":      12.5,
			"macd_signal": 8.7,
			"bb_position": 6.2,
			"adx":         11.1,
		},
		DataAge:    dataAge,
		Confidence: 0.82,
	}

	// Volume (orthogonalized)
	volume := httpContracts.FactorDetail{
		RawScore:        baseScore * 0.25,
		OrthogonalScore: baseScore * 0.25 * 0.72, // Further reduced
		Weight:          getRegimeWeights(regime)["volume"],
		WeightedScore:   baseScore * 0.25 * 0.72 * getRegimeWeights(regime)["volume"] / 100.0,
		Components: map[string]float64{
			"vol_surge":   9.5,
			"vol_profile": 7.2,
			"oi_change":   5.8,
		},
		DataAge:    dataAge,
		Confidence: 0.79,
	}

	// Quality (orthogonalized)
	quality := httpContracts.FactorDetail{
		RawScore:        baseScore * 0.3,
		OrthogonalScore: baseScore * 0.3 * 0.68, // Most reduced
		Weight:          getRegimeWeights(regime)["quality"],
		WeightedScore:   baseScore * 0.3 * 0.68 * getRegimeWeights(regime)["quality"] / 100.0,
		Components: map[string]float64{
			"liquidity":   8.1,
			"stability":   6.7,
			"correlation": 4.9,
		},
		DataAge:    dataAge,
		Confidence: 0.76,
	}

	// Social (capped)
	social := httpContracts.SocialFactorDetail{
		RawScore:    12.5,
		CappedScore: 10.0,
		Bonus:       10.0,
		Sources: map[string]float64{
			"social_momentum": 6.2,
			"news_sentiment":  3.8,
			"dev_activity":    2.5,
		},
		WasCapped: true,
		DataAge:   dataAge,
	}

	return httpContracts.FactorExplanation{
		MomentumCore: momentumCore,
		Technical:    technical,
		Volume:       volume,
		Quality:      quality,
		Social:       social,
	}
}

// generateRegimeExplanation creates regime detection explanation
func generateRegimeExplanation(regime string) httpContracts.RegimeExplanation {
	indicators := map[string]float64{
		"volatility_7d":  0.42,
		"above_ma_pct":   0.68,
		"breadth_thrust": 0.24,
	}

	return httpContracts.RegimeExplanation{
		CurrentRegime: regime,
		RegimeWeights: getRegimeWeights(regime),
		Indicators:    indicators,
		Confidence:    0.84,
		LastSwitch:    time.Now().Add(-4 * time.Hour),
		SwitchReason:  "volatility_threshold",
	}
}

// generateAttributionExplanation creates comprehensive attribution
func generateAttributionExplanation(baseScore float64, regime string) httpContracts.AttributionExplanation {
	weights := getRegimeWeights(regime)

	contributions := map[string]float64{
		"momentum":  baseScore * weights["momentum"] / 100.0,
		"technical": baseScore * weights["technical"] / 100.0 * 0.85,
		"volume":    baseScore * weights["volume"] / 100.0 * 0.72,
		"quality":   baseScore * weights["quality"] / 100.0 * 0.68,
		"social":    10.0,
	}

	steps := []httpContracts.AttributionStep{
		{
			Step:         "data_fetch",
			RunningTotal: 0.0,
			Contribution: baseScore,
			Duration:     125 * time.Millisecond,
		},
		{
			Step:         "orthogonalization",
			RunningTotal: baseScore,
			Contribution: -baseScore * 0.15,
			Duration:     45 * time.Millisecond,
		},
		{
			Step:         "regime_weighting",
			RunningTotal: baseScore * 0.85,
			Contribution: 8.5,
			Duration:     25 * time.Millisecond,
		},
		{
			Step:         "social_capping",
			RunningTotal: baseScore*0.85 + 8.5,
			Contribution: 10.0,
			Duration:     15 * time.Millisecond,
		},
	}

	dataSources := map[string]string{
		"price":     "kraken_ws",
		"volume":    "kraken_rest",
		"orderbook": "kraken_l2",
		"social":    "free_apis",
	}

	cacheStatus := map[string]bool{
		"price_data":  true,
		"volume_data": false,
		"social_data": true,
		"regime_data": true,
	}

	performance := httpContracts.PerformanceMetrics{
		TotalDuration: 210 * time.Millisecond,
		CacheHits:     3,
		CacheMisses:   1,
		APICallsMade:  2,
		DataFreshness: 30 * time.Second,
	}

	return httpContracts.AttributionExplanation{
		TotalContributions: contributions,
		StepByStep:         steps,
		DataSources:        dataSources,
		CacheStatus:        cacheStatus,
		PerformanceMetrics: performance,
	}
}
