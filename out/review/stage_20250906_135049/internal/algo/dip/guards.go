package dip

import (
	"context"
	"fmt"
	"math"
	"time"
)

// GuardsConfig contains all guard parameters
type GuardsConfig struct {
	NewsShock NewsShockConfig `yaml:"news_shock"`
	StairStep StairStepConfig `yaml:"stair_step"`
	TimeDecay TimeDecayConfig `yaml:"time_decay"`
}

// NewsShockConfig contains news shock guard parameters
type NewsShockConfig struct {
	Return24hMin     float64 `yaml:"return_24h_min"`
	AccelRebound     float64 `yaml:"accel_rebound"`
	ReboundBars      int     `yaml:"rebound_bars"`
}

// StairStepConfig contains stair-step pattern guard parameters
type StairStepConfig struct {
	MaxAttempts      int     `yaml:"max_attempts"`
	LowerHighWindow  int     `yaml:"lower_high_window"`
}

// TimeDecayConfig contains time decay guard parameters
type TimeDecayConfig struct {
	BarsToLive int `yaml:"bars_to_live"`
}

// GuardResult contains guard evaluation results
type GuardResult struct {
	Passed      bool                    `json:"passed"`
	VetoReason  string                  `json:"veto_reason,omitempty"`
	GuardChecks map[string]GuardCheck   `json:"guard_checks"`
}

// GuardCheck contains individual guard check results
type GuardCheck struct {
	Name       string  `json:"name"`
	Passed     bool    `json:"passed"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	Reason     string  `json:"reason,omitempty"`
}

// DipGuards implements false-positive reduction guards
type DipGuards struct {
	config GuardsConfig
}

// NewDipGuards creates a new dip guards validator
func NewDipGuards(config GuardsConfig) *DipGuards {
	return &DipGuards{
		config: config,
	}
}

// ValidateEntry runs all guard checks on a dip candidate
func (dg *DipGuards) ValidateEntry(ctx context.Context, dipPoint *DipPoint, data []MarketData, 
	currentTime time.Time) (*GuardResult, error) {
	
	if dipPoint == nil || len(data) == 0 {
		return &GuardResult{
			Passed:     false,
			VetoReason: "invalid input data",
			GuardChecks: make(map[string]GuardCheck),
		}, nil
	}
	
	guardChecks := make(map[string]GuardCheck)
	
	// News shock guard
	newsShockCheck := dg.checkNewsShock(data, dipPoint.Index)
	guardChecks["news_shock"] = newsShockCheck
	
	// Stair-step pattern guard
	stairStepCheck := dg.checkStairStep(data, dipPoint.Index)
	guardChecks["stair_step"] = stairStepCheck
	
	// Time decay guard
	timeDecayCheck := dg.checkTimeDecay(dipPoint, currentTime)
	guardChecks["time_decay"] = timeDecayCheck
	
	// Determine overall result
	allPassed := true
	vetoReason := ""
	
	for name, check := range guardChecks {
		if !check.Passed {
			allPassed = false
			if vetoReason == "" {
				vetoReason = fmt.Sprintf("%s guard failed: %s", name, check.Reason)
			}
			break
		}
	}
	
	return &GuardResult{
		Passed:      allPassed,
		VetoReason:  vetoReason,
		GuardChecks: guardChecks,
	}, nil
}

// checkNewsShock validates there's no severe shock without rebound
func (dg *DipGuards) checkNewsShock(data []MarketData, dipIndex int) GuardCheck {
	// Need at least 24 bars for 24h return + rebound bars
	requiredBars := 24 + dg.config.NewsShock.ReboundBars
	if dipIndex < requiredBars || len(data) <= dipIndex+dg.config.NewsShock.ReboundBars {
		return GuardCheck{
			Name:   "news_shock",
			Passed: true, // Insufficient data - pass by default
			Reason: "insufficient data for news shock analysis",
		}
	}
	
	// Calculate 24h return at dip point
	price24hAgo := data[dipIndex-24].Close
	dipPrice := data[dipIndex].Close
	return24h := (dipPrice - price24hAgo) / price24hAgo * 100
	
	// If return is not severely negative, pass
	if return24h >= dg.config.NewsShock.Return24hMin {
		return GuardCheck{
			Name:      "news_shock",
			Passed:    true,
			Value:     return24h,
			Threshold: dg.config.NewsShock.Return24hMin,
			Reason:    "return not severely negative",
		}
	}
	
	// Check for acceleration rebound in subsequent bars
	hasRebound := dg.checkAccelerationRebound(data, dipIndex, dg.config.NewsShock.ReboundBars, dg.config.NewsShock.AccelRebound)
	
	if hasRebound {
		return GuardCheck{
			Name:      "news_shock",
			Passed:    true,
			Value:     return24h,
			Threshold: dg.config.NewsShock.Return24hMin,
			Reason:    "acceleration rebound detected",
		}
	}
	
	return GuardCheck{
		Name:      "news_shock",
		Passed:    false,
		Value:     return24h,
		Threshold: dg.config.NewsShock.Return24hMin,
		Reason:    fmt.Sprintf("severe shock %.1f%% without rebound", return24h),
	}
}

// checkAccelerationRebound looks for price acceleration after severe drop
func (dg *DipGuards) checkAccelerationRebound(data []MarketData, dipIndex, reboundBars int, minRebound float64) bool {
	dipPrice := data[dipIndex].Low
	
	for i := dipIndex + 1; i <= dipIndex+reboundBars && i < len(data); i++ {
		currentPrice := data[i].Close
		rebound := (currentPrice - dipPrice) / dipPrice * 100
		
		if rebound >= minRebound {
			return true
		}
	}
	
	return false
}

// checkStairStep rejects patterns with persistent lower highs
func (dg *DipGuards) checkStairStep(data []MarketData, dipIndex int) GuardCheck {
	window := dg.config.StairStep.LowerHighWindow
	maxAttempts := dg.config.StairStep.MaxAttempts
	
	if dipIndex < window*maxAttempts {
		return GuardCheck{
			Name:   "stair_step",
			Passed: true,
			Reason: "insufficient data for stair-step analysis",
		}
	}
	
	// Look for pattern of lower highs in preceding periods
	attempts := dg.countStairStepAttempts(data, dipIndex, window)
	
	passed := attempts < maxAttempts
	reason := ""
	if !passed {
		reason = fmt.Sprintf("detected %d stair-step attempts (max %d)", attempts, maxAttempts)
	}
	
	return GuardCheck{
		Name:      "stair_step",
		Passed:    passed,
		Value:     float64(attempts),
		Threshold: float64(maxAttempts),
		Reason:    reason,
	}
}

// countStairStepAttempts counts lower high patterns before dip
func (dg *DipGuards) countStairStepAttempts(data []MarketData, dipIndex, window int) int {
	attempts := 0
	lastHigh := 0.0
	
	// Scan backwards in windows
	for windowStart := dipIndex - window; windowStart >= 0; windowStart -= window {
		windowEnd := windowStart + window - 1
		if windowEnd >= dipIndex {
			windowEnd = dipIndex - 1
		}
		
		// Find highest point in this window
		windowHigh := 0.0
		for i := windowStart; i <= windowEnd && i >= 0; i++ {
			if data[i].High > windowHigh {
				windowHigh = data[i].High
			}
		}
		
		if windowHigh == 0 {
			continue
		}
		
		// Check if this high is lower than the last
		if lastHigh > 0 && windowHigh < lastHigh {
			attempts++
		}
		
		lastHigh = windowHigh
		
		// Stop if we've seen enough attempts
		if attempts >= dg.config.StairStep.MaxAttempts {
			break
		}
	}
	
	return attempts
}

// checkTimeDecay validates signal hasn't expired
func (dg *DipGuards) checkTimeDecay(dipPoint *DipPoint, currentTime time.Time) GuardCheck {
	barsToLive := dg.config.TimeDecay.BarsToLive
	
	// Calculate bars elapsed since dip (assuming 1h bars)
	elapsed := currentTime.Sub(dipPoint.Timestamp)
	barsElapsed := int(elapsed.Hours())
	
	passed := barsElapsed <= barsToLive
	reason := ""
	if !passed {
		reason = fmt.Sprintf("signal expired: %d bars elapsed (max %d)", barsElapsed, barsToLive)
	}
	
	return GuardCheck{
		Name:      "time_decay",
		Passed:    passed,
		Value:     float64(barsElapsed),
		Threshold: float64(barsToLive),
		Reason:    reason,
	}
}

// ResetTimeDecay updates a dip point's timestamp for time decay calculation
func (dg *DipGuards) ResetTimeDecay(dipPoint *DipPoint, newTime time.Time) {
	dipPoint.Timestamp = newTime
}

// ValidateEntryTiming checks if entry conditions are still valid at execution time
func (dg *DipGuards) ValidateEntryTiming(ctx context.Context, dipPoint *DipPoint, 
	currentData []MarketData, currentTime time.Time) (*GuardResult, error) {
	
	// Run standard guards
	guardResult, err := dg.ValidateEntry(ctx, dipPoint, currentData, currentTime)
	if err != nil {
		return nil, err
	}
	
	// Additional timing-specific checks
	timingChecks := dg.checkEntryTiming(dipPoint, currentData, currentTime)
	
	// Merge timing checks with guard checks
	for name, check := range timingChecks {
		guardResult.GuardChecks[name] = check
		if !check.Passed && guardResult.Passed {
			guardResult.Passed = false
			guardResult.VetoReason = fmt.Sprintf("%s timing check failed: %s", name, check.Reason)
		}
	}
	
	return guardResult, nil
}

// checkEntryTiming performs execution-time validation
func (dg *DipGuards) checkEntryTiming(dipPoint *DipPoint, currentData []MarketData, currentTime time.Time) map[string]GuardCheck {
	checks := make(map[string]GuardCheck)
	
	// Price movement check - ensure we haven't moved too far from dip
	if len(currentData) > 0 {
		currentPrice := currentData[len(currentData)-1].Close
		priceMove := math.Abs(currentPrice - dipPoint.Price) / dipPoint.Price * 100
		
		// Allow up to 5% movement from dip point
		maxPriceMove := 5.0
		passed := priceMove <= maxPriceMove
		
		checks["price_movement"] = GuardCheck{
			Name:      "price_movement",
			Passed:    passed,
			Value:     priceMove,
			Threshold: maxPriceMove,
			Reason:    fmt.Sprintf("%.1f%% price movement from dip", priceMove),
		}
	}
	
	// Volume confirmation - ensure volume is still adequate
	if len(currentData) > 0 {
		currentVolume := currentData[len(currentData)-1].Volume
		dipVolume := 0.0
		
		// Find volume at dip time (approximate)
		for i := len(currentData) - 1; i >= 0; i-- {
			if currentData[i].Timestamp.Before(dipPoint.Timestamp.Add(time.Hour)) &&
			   currentData[i].Timestamp.After(dipPoint.Timestamp.Add(-time.Hour)) {
				dipVolume = currentData[i].Volume
				break
			}
		}
		
		volumeRatio := 1.0
		if dipVolume > 0 {
			volumeRatio = currentVolume / dipVolume
		}
		
		// Require at least 50% of dip volume
		minVolumeRatio := 0.5
		passed := volumeRatio >= minVolumeRatio
		
		checks["volume_continuation"] = GuardCheck{
			Name:      "volume_continuation",
			Passed:    passed,
			Value:     volumeRatio,
			Threshold: minVolumeRatio,
			Reason:    fmt.Sprintf("%.1fx volume ratio vs dip", volumeRatio),
		}
	}
	
	return checks
}