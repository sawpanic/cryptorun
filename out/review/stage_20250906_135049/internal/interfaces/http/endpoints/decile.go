package endpoints

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cryptorun/internal/metrics"
)

// DecileResponse represents the decile analysis endpoint response
type DecileResponse struct {
	Timestamp     time.Time                 `json:"timestamp"`
	Analysis      *metrics.DecileAnalysis   `json:"analysis"`
	Summary       DecileSummary             `json:"summary"`
	Insights      []string                  `json:"insights"`
	ModelQuality  ModelQuality              `json:"model_quality"`
}

// DecileSummary provides high-level decile performance overview
type DecileSummary struct {
	TopDecileReturn    float64 `json:"top_decile_return"`     // Decile 10 avg return
	BottomDecileReturn float64 `json:"bottom_decile_return"`  // Decile 1 avg return
	Spread             float64 `json:"spread"`                // Top - Bottom
	Monotonicity       float64 `json:"monotonicity"`          // How well ordered deciles are
	PredictivePower    string  `json:"predictive_power"`      // "strong", "moderate", "weak"
	SignalQuality      string  `json:"signal_quality"`        // "excellent", "good", "poor"
}

// ModelQuality provides model validation metrics
type ModelQuality struct {
	CorrelationStrength  string  `json:"correlation_strength"`   // "strong", "moderate", "weak"
	SharpeRating         string  `json:"sharpe_rating"`          // "excellent", "good", "poor"  
	DrawdownRisk         string  `json:"drawdown_risk"`          // "low", "moderate", "high"
	SampleSufficiency    bool    `json:"sample_sufficiency"`     // true if sample size adequate
	RecommendedAction    string  `json:"recommended_action"`     // "deploy", "caution", "retrain"
}

// DecileHandler returns the decile analysis endpoint
func DecileHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get decile analysis from collector
		analysis := collector.GetDecileAnalysis()
		
		// Handle query parameters
		query := r.URL.Query()
		if horizon := query.Get("horizon"); horizon != "" {
			// Could filter by horizon if needed
			_ = horizon
		}
		
		if limitStr := query.Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(analysis.Deciles) {
				analysis.Deciles = analysis.Deciles[:limit]
			}
		}

		// Calculate summary metrics
		summary := calculateDecileSummary(analysis)
		
		// Generate insights
		insights := generateDecileInsights(analysis, summary)
		
		// Assess model quality
		modelQuality := assessModelQuality(analysis)

		response := DecileResponse{
			Timestamp:    time.Now(),
			Analysis:     analysis,
			Summary:      summary,
			Insights:     insights,
			ModelQuality: modelQuality,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=300") // 5 minute cache

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// calculateDecileSummary computes summary metrics from decile analysis
func calculateDecileSummary(analysis *metrics.DecileAnalysis) DecileSummary {
	if len(analysis.Deciles) == 0 {
		return DecileSummary{}
	}

	topDecile := analysis.Deciles[len(analysis.Deciles)-1]
	bottomDecile := analysis.Deciles[0]
	
	spread := topDecile.AvgForwardReturn - bottomDecile.AvgForwardReturn
	
	// Calculate monotonicity (how well-ordered the deciles are)
	monotonicity := calculateMonotonicity(analysis.Deciles)
	
	// Determine predictive power based on spread and correlation
	predictivePower := "weak"
	if spread > 8.0 && analysis.Correlation > 0.6 {
		predictivePower = "strong"
	} else if spread > 4.0 && analysis.Correlation > 0.4 {
		predictivePower = "moderate"
	}

	// Determine signal quality based on multiple factors
	signalQuality := "poor"
	if monotonicity > 0.8 && analysis.Correlation > 0.7 && spread > 6.0 {
		signalQuality = "excellent"
	} else if monotonicity > 0.6 && analysis.Correlation > 0.5 && spread > 3.0 {
		signalQuality = "good"
	}

	return DecileSummary{
		TopDecileReturn:    topDecile.AvgForwardReturn,
		BottomDecileReturn: bottomDecile.AvgForwardReturn,
		Spread:             spread,
		Monotonicity:       monotonicity,
		PredictivePower:    predictivePower,
		SignalQuality:      signalQuality,
	}
}

// calculateMonotonicity measures how well-ordered the deciles are (0.0 to 1.0)
func calculateMonotonicity(deciles []metrics.DecileBucket) float64 {
	if len(deciles) <= 1 {
		return 1.0
	}

	correctOrdering := 0
	totalPairs := 0

	// Check all pairs of deciles
	for i := 0; i < len(deciles); i++ {
		for j := i + 1; j < len(deciles); j++ {
			totalPairs++
			if deciles[j].AvgForwardReturn >= deciles[i].AvgForwardReturn {
				correctOrdering++
			}
		}
	}

	return float64(correctOrdering) / float64(totalPairs)
}

// generateDecileInsights creates actionable insights from decile analysis
func generateDecileInsights(analysis *metrics.DecileAnalysis, summary DecileSummary) []string {
	var insights []string

	// Correlation insights
	if analysis.Correlation > 0.7 {
		insights = append(insights, "Strong positive correlation indicates excellent predictive model performance")
	} else if analysis.Correlation > 0.4 {
		insights = append(insights, "Moderate correlation suggests model has predictive value but can be improved")
	} else {
		insights = append(insights, "Low correlation indicates model may need retraining or feature engineering")
	}

	// Spread insights
	if summary.Spread > 10.0 {
		insights = append(insights, "Large return spread between deciles shows strong signal differentiation")
	} else if summary.Spread > 5.0 {
		insights = append(insights, "Moderate return spread indicates decent signal quality")
	} else {
		insights = append(insights, "Small return spread suggests weak signal - consider factor optimization")
	}

	// Monotonicity insights
	if summary.Monotonicity > 0.9 {
		insights = append(insights, "Excellent monotonicity - deciles are well-ordered by performance")
	} else if summary.Monotonicity < 0.7 {
		insights = append(insights, "Poor monotonicity detected - some lower deciles outperform higher ones")
	}

	// Sharpe ratio insights
	if analysis.SharpeRatio > 1.0 {
		insights = append(insights, "Strong risk-adjusted returns indicate robust model performance")
	} else if analysis.SharpeRatio < 0.5 {
		insights = append(insights, "Low Sharpe ratio suggests high volatility relative to returns")
	}

	// Sample size insights
	if analysis.SampleSize < 100 {
		insights = append(insights, "Small sample size - results may not be statistically significant")
	} else if analysis.SampleSize > 1000 {
		insights = append(insights, "Large sample provides high statistical confidence in results")
	}

	// Top decile performance insights
	topDecile := analysis.Deciles[len(analysis.Deciles)-1]
	if topDecile.WinRate > 0.7 {
		insights = append(insights, "High win rate in top decile indicates consistent outperformance")
	}

	return insights
}

// assessModelQuality provides overall model validation assessment
func assessModelQuality(analysis *metrics.DecileAnalysis) ModelQuality {
	// Assess correlation strength
	correlationStrength := "weak"
	if analysis.Correlation > 0.7 {
		correlationStrength = "strong"
	} else if analysis.Correlation > 0.4 {
		correlationStrength = "moderate"
	}

	// Assess Sharpe ratio
	sharpeRating := "poor"
	if analysis.SharpeRatio > 1.0 {
		sharpeRating = "excellent"
	} else if analysis.SharpeRatio > 0.5 {
		sharpeRating = "good"
	}

	// Assess drawdown risk
	drawdownRisk := "moderate"
	if analysis.MaxDrawdown > -5.0 {
		drawdownRisk = "low"
	} else if analysis.MaxDrawdown < -15.0 {
		drawdownRisk = "high"
	}

	// Check sample sufficiency (need at least 50 samples per decile minimum)
	sampleSufficiency := analysis.SampleSize >= 500

	// Determine recommended action
	recommendedAction := "retrain"
	if analysis.Correlation > 0.6 && analysis.SharpeRatio > 0.7 && sampleSufficiency {
		recommendedAction = "deploy"
	} else if analysis.Correlation > 0.4 && analysis.SharpeRatio > 0.5 {
		recommendedAction = "caution"
	}

	return ModelQuality{
		CorrelationStrength: correlationStrength,
		SharpeRating:        sharpeRating,
		DrawdownRisk:        drawdownRisk,
		SampleSufficiency:   sampleSufficiency,
		RecommendedAction:   recommendedAction,
	}
}