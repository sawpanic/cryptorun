package endpoints

import (
	"encoding/json"
	"net/http"
	"time"

	httpContracts "cryptorun/internal/interfaces/http"
	"cryptorun/internal/metrics"
	"github.com/rs/zerolog/log"
)

// RegimeHandler returns current regime information and weights
func RegimeHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get current regime information
		regime := getCurrentRegimeInfo()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=30") // Cache for 30 seconds

		// Log performance
		duration := time.Since(requestStart)
		log.Debug().
			Dur("duration", duration).
			Str("regime", regime.CurrentRegime).
			Float64("stability", regime.Health.StabilityScore).
			Msg("Regime endpoint served")

		if err := json.NewEncoder(w).Encode(regime); err != nil {
			log.Error().Err(err).Msg("Failed to encode regime response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// getCurrentRegimeInfo generates current regime information
func getCurrentRegimeInfo() httpContracts.RegimeResponse {
	timestamp := time.Now()

	// Determine regime based on time of day and add some variability
	regime := getCurrentRegimeByTime()
	regimeNumeric := regimeToNumeric(regime)

	// Generate regime health indicators
	health := generateRegimeHealth(regime)

	// Get weights for current regime
	weights := getRegimeWeights(regime)

	// Generate regime switches for today (mock data)
	switchesToday := calculateSwitchesToday(regime)

	// Calculate average duration
	avgDuration := calculateAvgDuration(regime)

	// Next evaluation is every 4 hours
	nextEval := calculateNextEvaluation(timestamp)

	// Generate recent history
	history := generateRecentHistory(regime, timestamp)

	return httpContracts.RegimeResponse{
		Timestamp:        timestamp,
		CurrentRegime:    regime,
		RegimeNumeric:    regimeNumeric,
		Health:           health,
		Weights:          weights,
		SwitchesToday:    switchesToday,
		AvgDurationHours: avgDuration,
		NextEvaluation:   nextEval,
		History:          history,
	}
}

// getCurrentRegimeByTime determines regime based on time and adds variability
func getCurrentRegimeByTime() string {
	now := time.Now()
	hour := now.Hour()

	// Add some daily variation based on day of year
	dayOfYear := now.YearDay()
	variation := dayOfYear % 3

	switch {
	case hour >= 9 && hour < 16: // Market hours
		if variation == 0 {
			return "trending_bull"
		} else if variation == 1 {
			return "choppy"
		}
		return "trending_bull" // Default to trending during market hours
	case hour >= 16 && hour < 21: // Evening
		if variation == 2 {
			return "high_vol"
		}
		return "choppy"
	default: // Night/early morning
		if variation == 1 {
			return "choppy"
		}
		return "high_vol"
	}
}

// regimeToNumeric converts regime string to numeric representation
func regimeToNumeric(regime string) float64 {
	switch regime {
	case "choppy":
		return 0.0
	case "trending_bull":
		return 1.0
	case "high_vol":
		return 2.0
	default:
		return -1.0
	}
}

// generateRegimeHealth creates health indicators for the regime
func generateRegimeHealth(regime string) httpContracts.RegimeHealthData {
	now := time.Now()

	// Generate realistic health metrics based on regime
	var volatility7d, aboveMA, breadthThrust, stability float64

	switch regime {
	case "trending_bull":
		volatility7d = 0.35 + (float64(now.Minute()%10) * 0.02) // 0.35-0.53
		aboveMA = 0.65 + (float64(now.Second()%20) * 0.01)      // 0.65-0.84
		breadthThrust = 0.20 + (float64(now.Hour()%8) * 0.03)   // 0.20-0.41
		stability = 0.85 + (float64(now.Minute()%10) * 0.01)    // 0.85-0.94

	case "choppy":
		volatility7d = 0.25 + (float64(now.Minute()%15) * 0.02) // 0.25-0.53
		aboveMA = 0.45 + (float64(now.Second()%30) * 0.01)      // 0.45-0.74
		breadthThrust = 0.10 + (float64(now.Hour()%12) * 0.02)  // 0.10-0.32
		stability = 0.70 + (float64(now.Minute()%20) * 0.01)    // 0.70-0.89

	case "high_vol":
		volatility7d = 0.55 + (float64(now.Minute()%8) * 0.03) // 0.55-0.79
		aboveMA = 0.35 + (float64(now.Second()%25) * 0.02)     // 0.35-0.84
		breadthThrust = 0.30 + (float64(now.Hour()%6) * 0.05)  // 0.30-0.55
		stability = 0.60 + (float64(now.Minute()%15) * 0.02)   // 0.60-0.89

	default:
		volatility7d = 0.40
		aboveMA = 0.50
		breadthThrust = 0.20
		stability = 0.75
	}

	return httpContracts.RegimeHealthData{
		Volatility7d:   volatility7d,
		AboveMA_Pct:    aboveMA,
		BreadthThrust:  breadthThrust,
		StabilityScore: stability,
	}
}

// calculateSwitchesToday determines how many regime switches happened today
func calculateSwitchesToday(currentRegime string) int {
	now := time.Now()

	// Mock calculation based on current time and regime
	// More volatile regimes have had more switches
	hour := now.Hour()

	switch currentRegime {
	case "high_vol":
		return 2 + (hour / 8) // 2-4 switches
	case "choppy":
		return 1 + (hour / 12) // 1-3 switches
	case "trending_bull":
		return hour / 16 // 0-1 switches
	default:
		return 1
	}
}

// calculateAvgDuration calculates average regime duration in hours
func calculateAvgDuration(regime string) float64 {
	// Different regimes have different typical durations
	switch regime {
	case "trending_bull":
		return 18.5 + float64(time.Now().Minute()%10)*0.5 // 18.5-23.5h
	case "choppy":
		return 12.0 + float64(time.Now().Hour()%8)*0.8 // 12.0-18.4h
	case "high_vol":
		return 8.5 + float64(time.Now().Second()%20)*0.3 // 8.5-14.4h
	default:
		return 15.0
	}
}

// calculateNextEvaluation determines when the next 4h evaluation occurs
func calculateNextEvaluation(timestamp time.Time) time.Time {
	// Regime detector runs every 4 hours at 00:00, 04:00, 08:00, 12:00, 16:00, 20:00 UTC
	hour := timestamp.Hour()
	nextHour := ((hour / 4) + 1) * 4

	if nextHour >= 24 {
		// Next day
		next := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day()+1, 0, 0, 0, 0, timestamp.Location())
		return next
	}

	next := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), nextHour, 0, 0, 0, timestamp.Location())
	return next
}

// generateRecentHistory creates mock regime switch history
func generateRecentHistory(currentRegime string, timestamp time.Time) []httpContracts.RegimeSwitch {
	// Generate last few regime switches
	history := make([]httpContracts.RegimeSwitch, 0, 5)

	// Work backwards from current time
	regimes := []string{"trending_bull", "choppy", "high_vol"}
	triggers := []string{"volatility_threshold", "ma_crossover", "breadth_signal", "manual_override"}

	// Last switch (to current regime)
	lastSwitchTime := timestamp.Add(-4 * time.Hour)
	prevRegime := regimes[(getRegimeIndex(currentRegime)+1)%len(regimes)]

	history = append(history, httpContracts.RegimeSwitch{
		Timestamp:  lastSwitchTime,
		FromRegime: prevRegime,
		ToRegime:   currentRegime,
		Trigger:    triggers[timestamp.Hour()%len(triggers)],
		Confidence: 0.82 + float64(timestamp.Minute()%15)*0.01,
		Duration:   6*time.Hour + time.Duration(timestamp.Second()%3600)*time.Second,
	})

	// Previous switches
	switchTime := lastSwitchTime
	fromRegime := prevRegime

	for i := 1; i < 4; i++ {
		switchTime = switchTime.Add(-time.Duration(8+i*4) * time.Hour)
		toRegime := fromRegime
		fromRegime = regimes[(getRegimeIndex(toRegime)+2)%len(regimes)]

		history = append(history, httpContracts.RegimeSwitch{
			Timestamp:  switchTime,
			FromRegime: fromRegime,
			ToRegime:   toRegime,
			Trigger:    triggers[(timestamp.Hour()+i)%len(triggers)],
			Confidence: 0.75 + float64((timestamp.Minute()+i*10)%20)*0.01,
			Duration:   time.Duration(4+i*3)*time.Hour + time.Duration(timestamp.Second()%1800)*time.Second,
		})
	}

	return history
}

// getRegimeIndex returns the index of a regime in the regimes slice
func getRegimeIndex(regime string) int {
	switch regime {
	case "trending_bull":
		return 0
	case "choppy":
		return 1
	case "high_vol":
		return 2
	default:
		return 0
	}
}
