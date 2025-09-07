package momentum

import (
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/src/domain/momentum"
)

// OHLCVBar represents a single OHLCV data point
type OHLCVBar struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// TimeframeData holds OHLCV bars for a specific timeframe
type TimeframeData struct {
	Timeframe string     `json:"timeframe"` // "1h", "4h", "12h", "24h", "7d"
	Bars      []OHLCVBar `json:"bars"`
}

// MultiTimeframeData holds all required timeframes for momentum calculation
type MultiTimeframeData struct {
	Symbol    string                   `json:"symbol"`
	Data      map[string]TimeframeData `json:"data"` // keyed by timeframe
	UpdatedAt time.Time                `json:"updated_at"`
}

// MinBarsConfig defines minimum bars required per timeframe to avoid cold starts
type MinBarsConfig struct {
	H1  int `yaml:"1h" json:"1h"`
	H4  int `yaml:"4h" json:"4h"`
	H12 int `yaml:"12h" json:"12h"`
	H24 int `yaml:"24h" json:"24h"`
	D7  int `yaml:"7d" json:"7d"`
}

// DefaultMinBars returns the default minimum bars configuration
func DefaultMinBars() MinBarsConfig {
	return MinBarsConfig{
		H1:  60, // 60 1-hour bars = 60 hours = 2.5 days
		H4:  60, // 60 4-hour bars = 240 hours = 10 days
		H12: 60, // 60 12-hour bars = 720 hours = 30 days
		H24: 30, // 30 24-hour bars = 30 days
		D7:  30, // 30 7-day bars = 210 days (~7 months)
	}
}

// InputsBuilder transforms OHLCV bars into MomentumCore inputs
type InputsBuilder struct {
	minBars MinBarsConfig
}

// NewInputsBuilder creates a new inputs builder with specified minimum bars
func NewInputsBuilder(minBars MinBarsConfig) *InputsBuilder {
	return &InputsBuilder{
		minBars: minBars,
	}
}

// BuildCoreInputs transforms multi-timeframe OHLCV data into CoreInputs
func (ib *InputsBuilder) BuildCoreInputs(data MultiTimeframeData) (momentum.CoreInputs, error) {
	var inputs momentum.CoreInputs

	// Validate we have all required timeframes
	requiredTFs := []string{"1h", "4h", "12h", "24h", "7d"}
	for _, tf := range requiredTFs {
		if _, exists := data.Data[tf]; !exists {
			return inputs, fmt.Errorf("missing timeframe data: %s", tf)
		}
	}

	// Check minimum bars requirement for each timeframe
	if err := ib.validateMinBars(data); err != nil {
		return inputs, fmt.Errorf("insufficient bars: %w", err)
	}

	// Calculate returns for each timeframe
	h1Data := data.Data["1h"]
	h4Data := data.Data["4h"]
	h12Data := data.Data["12h"]
	h24Data := data.Data["24h"]
	d7Data := data.Data["7d"]

	// 1-hour return (most recent bar vs 1 bar ago)
	if len(h1Data.Bars) >= 2 {
		inputs.R1h = calculateLogReturn(h1Data.Bars[len(h1Data.Bars)-2].Close,
			h1Data.Bars[len(h1Data.Bars)-1].Close)
	}

	// 4-hour return (most recent bar vs 1 bar ago)
	if len(h4Data.Bars) >= 2 {
		inputs.R4h = calculateLogReturn(h4Data.Bars[len(h4Data.Bars)-2].Close,
			h4Data.Bars[len(h4Data.Bars)-1].Close)
	}

	// 12-hour return (most recent bar vs 1 bar ago)
	if len(h12Data.Bars) >= 2 {
		inputs.R12h = calculateLogReturn(h12Data.Bars[len(h12Data.Bars)-2].Close,
			h12Data.Bars[len(h12Data.Bars)-1].Close)
	}

	// 24-hour return (most recent bar vs 1 bar ago)
	if len(h24Data.Bars) >= 2 {
		inputs.R24h = calculateLogReturn(h24Data.Bars[len(h24Data.Bars)-2].Close,
			h24Data.Bars[len(h24Data.Bars)-1].Close)
	}

	// 7-day return (most recent bar vs 1 bar ago)
	if len(d7Data.Bars) >= 2 {
		inputs.R7d = calculateLogReturn(d7Data.Bars[len(d7Data.Bars)-2].Close,
			d7Data.Bars[len(d7Data.Bars)-1].Close)
	}

	// Calculate ATR for normalization
	if len(h1Data.Bars) >= 14 {
		inputs.ATR1h = calculateATR(h1Data.Bars, 14) // 14-period ATR
	}
	if len(h4Data.Bars) >= 14 {
		inputs.ATR4h = calculateATR(h4Data.Bars, 14) // 14-period ATR
	}

	// Calculate 4-hour acceleration (d/dt of R4h over last 2-3 bars)
	if len(h4Data.Bars) >= 4 {
		inputs.Accel4h = calculate4hAcceleration(h4Data.Bars)
	}

	return inputs, nil
}

// validateMinBars ensures each timeframe has sufficient bars
func (ib *InputsBuilder) validateMinBars(data MultiTimeframeData) error {
	checks := []struct {
		tf  string
		min int
	}{
		{"1h", ib.minBars.H1},
		{"4h", ib.minBars.H4},
		{"12h", ib.minBars.H12},
		{"24h", ib.minBars.H24},
		{"7d", ib.minBars.D7},
	}

	for _, check := range checks {
		tfData, exists := data.Data[check.tf]
		if !exists {
			return fmt.Errorf("missing timeframe: %s", check.tf)
		}
		if len(tfData.Bars) < check.min {
			return fmt.Errorf("insufficient bars for %s: got %d, need %d",
				check.tf, len(tfData.Bars), check.min)
		}
	}

	return nil
}

// calculateLogReturn computes log return between two prices
func calculateLogReturn(prevClose, currentClose float64) float64 {
	if prevClose <= 0 || currentClose <= 0 {
		return 0.0
	}
	return math.Log(currentClose / prevClose)
}

// calculateATR computes Average True Range over specified periods
func calculateATR(bars []OHLCVBar, periods int) float64 {
	if len(bars) < periods+1 {
		return 0.0
	}

	trueRanges := make([]float64, 0, periods)

	// Calculate True Range for each period
	for i := len(bars) - periods; i < len(bars); i++ {
		if i == 0 {
			continue // Skip first bar (no previous close)
		}

		prevClose := bars[i-1].Close
		high := bars[i].High
		low := bars[i].Low

		// True Range = max(high-low, abs(high-prevClose), abs(low-prevClose))
		tr := math.Max(high-low,
			math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

		trueRanges = append(trueRanges, tr)
	}

	// Calculate average of true ranges
	if len(trueRanges) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, tr := range trueRanges {
		sum += tr
	}

	return sum / float64(len(trueRanges))
}

// calculate4hAcceleration computes the acceleration of 4h returns over last 2-3 bars
func calculate4hAcceleration(bars []OHLCVBar) float64 {
	if len(bars) < 4 {
		return 0.0
	}

	// Get the last 3 bars to calculate acceleration
	n := len(bars)

	// Calculate returns for the last 3 periods
	r1 := calculateLogReturn(bars[n-4].Close, bars[n-3].Close) // t-2 to t-1
	r2 := calculateLogReturn(bars[n-3].Close, bars[n-2].Close) // t-1 to t
	r3 := calculateLogReturn(bars[n-2].Close, bars[n-1].Close) // t to t+1

	// Simple acceleration: (r3 - r2) - (r2 - r1) = r3 - 2*r2 + r1
	accel := r3 - 2*r2 + r1

	return accel
}

// GetDataFreshness returns the age of the most recent data
func GetDataFreshness(data MultiTimeframeData) map[string]time.Duration {
	freshness := make(map[string]time.Duration)
	now := time.Now()

	for tf, tfData := range data.Data {
		if len(tfData.Bars) > 0 {
			lastBar := tfData.Bars[len(tfData.Bars)-1]
			freshness[tf] = now.Sub(lastBar.Timestamp)
		}
	}

	return freshness
}

// IsFresh checks if data is fresh enough for momentum calculation
func IsFresh(data MultiTimeframeData, maxAge map[string]time.Duration) bool {
	freshness := GetDataFreshness(data)

	for tf, maxAllowed := range maxAge {
		if age, exists := freshness[tf]; exists {
			if age > maxAllowed {
				return false
			}
		} else {
			return false // Missing timeframe data
		}
	}

	return true
}

// ValidateOHLCVBar performs basic validation on OHLCV data
func ValidateOHLCVBar(bar OHLCVBar) error {
	// Check for invalid prices
	if bar.Open <= 0 || bar.High <= 0 || bar.Low <= 0 || bar.Close <= 0 {
		return fmt.Errorf("invalid OHLCV prices: O=%f H=%f L=%f C=%f",
			bar.Open, bar.High, bar.Low, bar.Close)
	}

	// Check OHLC relationships
	if bar.High < bar.Low {
		return fmt.Errorf("high %.6f less than low %.6f", bar.High, bar.Low)
	}
	if bar.High < bar.Open || bar.High < bar.Close {
		return fmt.Errorf("high %.6f less than open %.6f or close %.6f",
			bar.High, bar.Open, bar.Close)
	}
	if bar.Low > bar.Open || bar.Low > bar.Close {
		return fmt.Errorf("low %.6f greater than open %.6f or close %.6f",
			bar.Low, bar.Open, bar.Close)
	}

	// Check for reasonable volume (can be zero but not negative)
	if bar.Volume < 0 {
		return fmt.Errorf("negative volume: %f", bar.Volume)
	}

	// Check timestamp is not zero
	if bar.Timestamp.IsZero() {
		return fmt.Errorf("zero timestamp")
	}

	return nil
}
