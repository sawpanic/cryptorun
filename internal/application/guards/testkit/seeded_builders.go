package testkit

import (
	"time"
)

// Clock provides deterministic time for testing
type Clock struct {
	now time.Time
}

// NewClock creates a test clock at fixed timestamp
func NewClock(timestamp string) *Clock {
	t, _ := time.Parse("2006-01-02T15:04:05Z", timestamp)
	return &Clock{now: t}
}

// Now returns the fixed test time
func (c *Clock) Now() time.Time {
	return c.now
}

// Advance moves the clock forward by duration
func (c *Clock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// CandidateFixture represents a test candidate with guard inputs
type CandidateFixture struct {
	Symbol         string  `json:"symbol"`
	CompositeScore float64 `json:"composite_score"`
	MomentumCore   float64 `json:"momentum_core"`
	RSI4h          float64 `json:"rsi_4h"`
	Volume24h      float64 `json:"volume_24h"`
	VolumeAvg      float64 `json:"volume_avg"`
	Spread         float64 `json:"spread_bps"`
	DepthUSD       float64 `json:"depth_usd"`
	VADR           float64 `json:"vadr"`
	CatalystHeat   float64 `json:"catalyst_heat"`
	SocialScore    float64 `json:"social_score"`
	BrandScore     float64 `json:"brand_score"`
	LastUpdate     string  `json:"last_update"`
	BarAge         int     `json:"bar_age"`
	ATRCurrent     float64 `json:"atr_current"`
	PriceMove      float64 `json:"price_move_atr"`
}

// GuardTestCase represents a complete test scenario
type GuardTestCase struct {
	Name       string             `json:"name"`
	Regime     string             `json:"regime"`
	Candidates []CandidateFixture `json:"candidates"`
	Expected   GuardTestExpected  `json:"expected"`
	Timestamp  string             `json:"timestamp"`
}

// GuardTestExpected defines expected test outcomes
type GuardTestExpected struct {
	PassCount    int                   `json:"pass_count"`
	FailCount    int                   `json:"fail_count"`
	ExitCode     int                   `json:"exit_code"`
	GuardResults []ExpectedGuardResult `json:"guard_results"`
}

// CreateFatigueTestCase creates test case for fatigue guard
func CreateFatigueTestCase(regime string) *GuardTestCase {
	var threshold24h float64
	switch regime {
	case "calm":
		threshold24h = 10.0 // Relaxed fatigue threshold
	case "normal":
		threshold24h = 12.0 // Baseline threshold
	case "volatile":
		threshold24h = 15.0 // Stricter threshold
	}

	return &GuardTestCase{
		Name:      "fatigue_guard_" + regime,
		Regime:    regime,
		Timestamp: "2025-01-15T12:00:00Z",
		Candidates: []CandidateFixture{
			{
				Symbol:         "BTCUSD",
				CompositeScore: 8.5,
				MomentumCore:   8.0,  // Below threshold - should pass
				RSI4h:          65.0, // Below 70 RSI limit
				LastUpdate:     "2025-01-15T11:58:00Z",
			},
			{
				Symbol:         "ETHUSD",
				CompositeScore: 9.2,
				MomentumCore:   threshold24h + 2.0, // Above threshold
				RSI4h:          75.0,               // Above 70 RSI limit - should fail fatigue
				LastUpdate:     "2025-01-15T11:59:00Z",
			},
			{
				Symbol:         "SOLUSD",
				CompositeScore: 7.8,
				MomentumCore:   threshold24h - 1.0, // Below threshold - should pass
				RSI4h:          60.0,               // Safe RSI level
				LastUpdate:     "2025-01-15T11:57:30Z",
			},
		},
		Expected: GuardTestExpected{
			PassCount: 2,
			FailCount: 1,
			ExitCode:  1, // Hard guard failure
			GuardResults: []ExpectedGuardResult{
				{
					Symbol: "BTCUSD",
					Status: "PASS",
				},
				{
					Symbol:      "ETHUSD",
					Status:      "FAIL",
					FailedGuard: "fatigue",
					Reason:      "24h momentum 17.0% > 15.0% + RSI4h 75.0 > 70.0",
					FixHint:     "Wait for momentum cooldown or RSI retreat",
				},
				{
					Symbol: "SOLUSD",
					Status: "PASS",
				},
			},
		},
	}
}

// CreateFreshnessTestCase creates test case for freshness guard
func CreateFreshnessTestCase(regime string) *GuardTestCase {
	var maxBarsAge int
	var atrFactor float64

	switch regime {
	case "calm":
		maxBarsAge = 3  // Relaxed bar age limit
		atrFactor = 1.5 // Relaxed price movement
	case "normal":
		maxBarsAge = 2  // Baseline bar age
		atrFactor = 1.2 // Baseline price movement
	case "volatile":
		maxBarsAge = 1  // Strict bar age in volatile markets
		atrFactor = 1.0 // Strict price movement
	}

	_ = atrFactor // Used in test data generation

	return &GuardTestCase{
		Name:      "freshness_guard_" + regime,
		Regime:    regime,
		Timestamp: "2025-01-15T12:00:00Z",
		Candidates: []CandidateFixture{
			{
				Symbol:         "BTCUSD",
				CompositeScore: 8.5,
				BarAge:         1, // Fresh data - should pass
				ATRCurrent:     100.0,
				PriceMove:      80.0, // 0.8 × ATR - within limit
				LastUpdate:     "2025-01-15T11:59:00Z",
			},
			{
				Symbol:         "ETHUSD",
				CompositeScore: 9.2,
				BarAge:         maxBarsAge + 1, // Stale data - should fail
				ATRCurrent:     50.0,
				PriceMove:      30.0, // 0.6 × ATR - would pass price check
				LastUpdate:     "2025-01-15T11:54:00Z",
			},
			{
				Symbol:         "ADAUSD",
				CompositeScore: 7.8,
				BarAge:         1, // Fresh data
				ATRCurrent:     25.0,
				PriceMove:      35.0, // 1.4 × ATR - exceeds limit
				LastUpdate:     "2025-01-15T11:59:30Z",
			},
		},
		Expected: GuardTestExpected{
			PassCount: 1,
			FailCount: 2,
			ExitCode:  1,
			GuardResults: []ExpectedGuardResult{
				{
					Symbol: "BTCUSD",
					Status: "PASS",
				},
				{
					Symbol:      "ETHUSD",
					Status:      "FAIL",
					FailedGuard: "freshness",
					Reason:      "Bar age 2 > 1 bars maximum",
					FixHint:     "Wait for fresh data or increase bar age tolerance",
				},
				{
					Symbol:      "ADAUSD",
					Status:      "FAIL",
					FailedGuard: "freshness",
					Reason:      "Price move 1.40×ATR > 1.00×ATR limit",
					FixHint:     "Wait for price stabilization or increase ATR tolerance",
				},
			},
		},
	}
}

// CreateSocialCapTestCase creates test case for social/brand cap guards
func CreateSocialCapTestCase(regime string) *GuardTestCase {
	var socialCap, brandCap float64

	switch regime {
	case "calm":
		socialCap = 12.0 // Relaxed social cap
		brandCap = 8.0   // Relaxed brand cap
	case "normal":
		socialCap = 10.0 // Standard caps
		brandCap = 6.0
	case "volatile":
		socialCap = 8.0 // Strict caps in volatile markets
		brandCap = 5.0
	}

	return &GuardTestCase{
		Name:      "social_cap_guard_" + regime,
		Regime:    regime,
		Timestamp: "2025-01-15T12:00:00Z",
		Candidates: []CandidateFixture{
			{
				Symbol:         "BTCUSD",
				CompositeScore: 8.5,
				SocialScore:    socialCap - 1.0, // Within cap
				BrandScore:     brandCap - 1.0,  // Within cap
			},
			{
				Symbol:         "ETHUSD",
				CompositeScore: 9.2,
				SocialScore:    socialCap + 2.0, // Exceeds social cap
				BrandScore:     brandCap - 0.5,  // Within brand cap
			},
			{
				Symbol:         "ADAUSD",
				CompositeScore: 7.8,
				SocialScore:    socialCap - 0.5, // Within social cap
				BrandScore:     brandCap + 1.5,  // Exceeds brand cap
			},
		},
		Expected: GuardTestExpected{
			PassCount: 1,
			FailCount: 2,
			ExitCode:  1,
			GuardResults: []ExpectedGuardResult{
				{
					Symbol: "BTCUSD",
					Status: "PASS",
				},
				{
					Symbol:      "ETHUSD",
					Status:      "FAIL",
					FailedGuard: "social_cap",
					Reason:      "Social score 12.0 exceeds 10.0 cap",
					FixHint:     "Reduce social factor weighting or wait for cooling",
				},
				{
					Symbol:      "ADAUSD",
					Status:      "FAIL",
					FailedGuard: "brand_cap",
					Reason:      "Brand score 7.5 exceeds 6.0 cap",
					FixHint:     "Reduce brand factor weighting or wait for normalization",
				},
			},
		},
	}
}

// CreateLiquidityTestCase creates test case for liquidity guards
func CreateLiquidityTestCase() *GuardTestCase {
	return &GuardTestCase{
		Name:      "liquidity_guards",
		Regime:    "normal",
		Timestamp: "2025-01-15T12:00:00Z",
		Candidates: []CandidateFixture{
			{
				Symbol:         "BTCUSD",
				CompositeScore: 8.5,
				Spread:         25.0,   // 25 bps - within 50 bps limit
				DepthUSD:       150000, // Above $100k minimum
				VADR:           2.0,    // Above 1.75× minimum
			},
			{
				Symbol:         "ALTCOIN",
				CompositeScore: 9.2,
				Spread:         75.0,   // 75 bps - exceeds 50 bps limit
				DepthUSD:       120000, // Above depth minimum
				VADR:           1.8,    // Above VADR minimum
			},
			{
				Symbol:         "ILLIQUID",
				CompositeScore: 7.8,
				Spread:         30.0,  // Within spread limit
				DepthUSD:       80000, // Below $100k minimum
				VADR:           1.5,   // Below 1.75× minimum
			},
		},
		Expected: GuardTestExpected{
			PassCount: 1,
			FailCount: 2,
			ExitCode:  1,
			GuardResults: []ExpectedGuardResult{
				{
					Symbol: "BTCUSD",
					Status: "PASS",
				},
				{
					Symbol:      "ALTCOIN",
					Status:      "FAIL",
					FailedGuard: "spread",
					Reason:      "Spread 75.0 bps > 50.0 bps limit",
					FixHint:     "Wait for tighter spread or increase spread tolerance",
				},
				{
					Symbol:      "ILLIQUID",
					Status:      "FAIL",
					FailedGuard: "depth",
					Reason:      "Depth $80k < $100k minimum, VADR 1.5× < 1.75× minimum",
					FixHint:     "Wait for improved liquidity or reduce depth requirements",
				},
			},
		},
	}
}

// CreateCatalystHeatTestCase creates test case for catalyst heat cap
func CreateCatalystHeatTestCase() *GuardTestCase {
	return &GuardTestCase{
		Name:      "catalyst_heat_cap",
		Regime:    "normal",
		Timestamp: "2025-01-15T12:00:00Z",
		Candidates: []CandidateFixture{
			{
				Symbol:         "BTCUSD",
				CompositeScore: 8.5,
				CatalystHeat:   8.0, // Within 10.0 cap
			},
			{
				Symbol:         "HYPE",
				CompositeScore: 9.2,
				CatalystHeat:   15.0, // Exceeds 10.0 cap
			},
			{
				Symbol:         "STABLE",
				CompositeScore: 7.8,
				CatalystHeat:   3.0, // Well within cap
			},
		},
		Expected: GuardTestExpected{
			PassCount: 2,
			FailCount: 1,
			ExitCode:  1,
			GuardResults: []ExpectedGuardResult{
				{
					Symbol: "BTCUSD",
					Status: "PASS",
				},
				{
					Symbol:      "HYPE",
					Status:      "FAIL",
					FailedGuard: "catalyst_heat",
					Reason:      "Catalyst heat 15.0 exceeds 10.0 cap",
					FixHint:     "Wait for event cooling or reduce catalyst sensitivity",
				},
				{
					Symbol: "STABLE",
					Status: "PASS",
				},
			},
		},
	}
}
