package premove

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/microstructure"
)

func TestPreMovementEngine_ListCandidates_FullPipeline(t *testing.T) {
	// Create mock microstructure evaluator
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			SpreadBps: 20.0,
			DepthUSD:  300000.0,
			VADR:      2.2,
			Healthy:   true,
		},
	}

	engine := NewPreMovementEngine(mockMicro, nil)

	// Create strong candidate inputs
	inputs := []*CandidateInput{
		{
			Symbol:    "BTC-USD",
			Timestamp: time.Now(),
			PreMovementData: &PreMovementData{
				Symbol:    "BTC-USD",
				Timestamp: time.Now(),

				// Strong structural signals (targeting ~35 points)
				FundingZScore:   3.2,   // Strong funding divergence
				OIResidual:      1.2e6, // $1.2M OI residual
				ETFFlowTint:     0.8,   // 80% bullish flows
				ReserveChange7d: -12.0, // -12% exchange reserves
				WhaleComposite:  0.85,  // 85% whale activity
				MicroDynamics:   0.7,   // 70% L1/L2 stress

				// Strong behavioral signals (targeting ~30 points)
				SmartMoneyFlow: 0.8, // 80% institutional flow
				CVDResidual:    0.6, // 60% CVD residual

				// Strong catalyst & compression (targeting ~20 points)
				CatalystHeat:       0.8, // 80% catalyst significance
				VolCompressionRank: 0.9, // 90th percentile compression

				OldestFeedHours: 0.5, // Fresh data
			},
			ConfirmationData: &ConfirmationData{
				Symbol:    "BTC-USD",
				Timestamp: time.Now(),

				// Strong 2-of-3 confirmations
				FundingZScore:    3.2,  // Strong funding
				WhaleComposite:   0.85, // Strong whale activity
				SupplyProxyScore: 0.7,  // Strong supply squeeze

				// Strong supply squeeze components
				ReserveChange7d:     -12.0,
				LargeWithdrawals24h: 80e6,
				StakingInflow24h:    15e6,
				DerivativesOIChange: 20.0,

				// Volume confirmation in supportive regime
				VolumeRatio24h: 3.5,
				CurrentRegime:  "risk_off",

				SpreadBps: 20.0,
				DepthUSD:  300000.0,
				VADR:      2.2,
			},
			CVDDataPoints: generateSyntheticCVDData(80, 0.8), // Good regression data
		},
		{
			Symbol:    "ETH-USD",
			Timestamp: time.Now(),
			PreMovementData: &PreMovementData{
				Symbol:    "ETH-USD",
				Timestamp: time.Now(),

				// Moderate signals (targeting ~60 points)
				FundingZScore:      2.1,
				OIResidual:         600000,
				ETFFlowTint:        0.5,
				ReserveChange7d:    -6.0,
				WhaleComposite:     0.6,
				MicroDynamics:      0.4,
				SmartMoneyFlow:     0.5,
				CVDResidual:        0.3,
				CatalystHeat:       0.4,
				VolCompressionRank: 0.6,
				OldestFeedHours:    1.2,
			},
			ConfirmationData: &ConfirmationData{
				Symbol:    "ETH-USD",
				Timestamp: time.Now(),

				// Moderate confirmations (only 2-of-3)
				FundingZScore:       2.1,  // Pass
				WhaleComposite:      0.75, // Pass
				SupplyProxyScore:    0.4,  // Fail
				ReserveChange7d:     -3.0,
				LargeWithdrawals24h: 30e6,
				StakingInflow24h:    8e6,
				DerivativesOIChange: 10.0,
				VolumeRatio24h:      1.8,
				CurrentRegime:       "normal",
			},
			CVDDataPoints: generateSyntheticCVDData(60, 0.6), // Moderate regression
		},
		{
			Symbol:    "SOL-USD",
			Timestamp: time.Now(),
			PreMovementData: &PreMovementData{
				Symbol:    "SOL-USD",
				Timestamp: time.Now(),

				// Weak signals (targeting ~30 points)
				FundingZScore:      1.2,
				OIResidual:         200000,
				ETFFlowTint:        0.2,
				ReserveChange7d:    -2.0,
				WhaleComposite:     0.3,
				MicroDynamics:      0.2,
				SmartMoneyFlow:     0.2,
				CVDResidual:        0.1,
				CatalystHeat:       0.2,
				VolCompressionRank: 0.3,
				OldestFeedHours:    0.8,
			},
			ConfirmationData: &ConfirmationData{
				Symbol:    "SOL-USD",
				Timestamp: time.Now(),

				// Weak confirmations (only 1-of-3)
				FundingZScore:       1.2, // Fail
				WhaleComposite:      0.5, // Fail
				SupplyProxyScore:    0.3, // Fail
				ReserveChange7d:     -2.0,
				LargeWithdrawals24h: 20e6,
				StakingInflow24h:    5e6,
				DerivativesOIChange: 8.0,
				VolumeRatio24h:      1.2,
				CurrentRegime:       "normal",
			},
			CVDDataPoints: generateNoisyCVDData(40), // Should trigger fallback
		},
	}

	result, err := engine.ListCandidates(context.Background(), inputs, 10)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should process all candidates
	assert.Equal(t, 3, result.TotalCandidates)

	// BTC and ETH should pass (strong and moderate), SOL should be filtered out
	assert.Equal(t, 2, result.ValidCandidates, "Should have 2 valid candidates")
	assert.Equal(t, 1, result.StrongCandidates, "Should have 1 strong candidate (BTC)")

	// Check ranking - BTC should be first (strongest)
	require.Len(t, result.Candidates, 2)
	assert.Equal(t, "BTC-USD", result.Candidates[0].Symbol, "BTC should rank first")
	assert.Equal(t, "STRONG", result.Candidates[0].OverallStatus)
	assert.Equal(t, 1, result.Candidates[0].Rank)

	assert.Equal(t, "ETH-USD", result.Candidates[1].Symbol, "ETH should rank second")
	assert.Equal(t, "MODERATE", result.Candidates[1].OverallStatus)
	assert.Equal(t, 2, result.Candidates[1].Rank)

	// Check data freshness assessment
	assert.NotNil(t, result.DataFreshness)
	assert.Greater(t, result.ProcessTimeMs, int64(0), "Should report processing time")
}

func TestPreMovementEngine_AnalyzeCandidate_ScoreBreakdown(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			SpreadBps: 30.0,
			DepthUSD:  150000.0,
			VADR:      1.9,
			Healthy:   true,
		},
	}

	engine := NewPreMovementEngine(mockMicro, nil)

	input := &CandidateInput{
		Symbol:    "TEST-USD",
		Timestamp: time.Now(),
		PreMovementData: &PreMovementData{
			Symbol:             "TEST-USD",
			Timestamp:          time.Now(),
			FundingZScore:      2.5,
			OIResidual:         800000,
			ETFFlowTint:        0.6,
			ReserveChange7d:    -8.0,
			WhaleComposite:     0.7,
			MicroDynamics:      0.5,
			SmartMoneyFlow:     0.6,
			CVDResidual:        0.4,
			CatalystHeat:       0.6,
			VolCompressionRank: 0.7,
			OldestFeedHours:    1.0,
		},
		ConfirmationData: &ConfirmationData{
			Symbol:              "TEST-USD",
			Timestamp:           time.Now(),
			FundingZScore:       2.5,
			WhaleComposite:      0.7,
			SupplyProxyScore:    0.4, // Will calculate from components
			ReserveChange7d:     -8.0,
			LargeWithdrawals24h: 60e6,
			StakingInflow24h:    12e6,
			DerivativesOIChange: 18.0,
			VolumeRatio24h:      2.0,
			CurrentRegime:       "normal",
		},
		CVDDataPoints: generateSyntheticCVDData(70, 0.7),
	}

	candidate, err := engine.analyzeCandidate(context.Background(), input)
	require.NoError(t, err)
	assert.NotNil(t, candidate)

	// Should have complete analysis
	assert.NotNil(t, candidate.ScoreBreakdown, "Should have score breakdown")
	assert.NotNil(t, candidate.GatesResult, "Should have gates result")
	assert.NotNil(t, candidate.CVDResult, "Should have CVD result")

	// Score should be reasonable for moderate signals
	assert.Greater(t, candidate.TotalScore, 50.0, "Moderate signals should yield >50 score")
	assert.Less(t, candidate.TotalScore, 85.0, "Moderate signals should yield <85 score")

	// Should have reasons for recommendation
	assert.Greater(t, len(candidate.Reasons), 0, "Should have recommendation reasons")

	// Should determine appropriate overall status
	assert.Contains(t, []string{"MODERATE", "WEAK"}, candidate.OverallStatus)

	assert.Greater(t, candidate.ProcessTimeMs, int64(0), "Should report processing time")
}

func TestPreMovementEngine_DetermineOverallStatus(t *testing.T) {
	engine := NewPreMovementEngine(nil, nil)

	// Test STRONG status (high score + confirmation + significant CVD)
	strongCandidate := &PreMovementCandidate{
		TotalScore:  88.0,
		GatesStatus: "CONFIRMED",
		CVDResult:   &CVDResidualResult{IsSignificant: true},
	}
	assert.Equal(t, "STRONG", engine.determineOverallStatus(strongCandidate))

	// Test MODERATE status (good score + confirmation)
	moderateCandidate := &PreMovementCandidate{
		TotalScore:  78.0,
		GatesStatus: "CONFIRMED",
		CVDResult:   &CVDResidualResult{IsSignificant: false},
	}
	assert.Equal(t, "MODERATE", engine.determineOverallStatus(moderateCandidate))

	// Test MODERATE status (very high score alone)
	highScoreCandidate := &PreMovementCandidate{
		TotalScore:  92.0,
		GatesStatus: "BLOCKED",
		CVDResult:   nil,
	}
	assert.Equal(t, "MODERATE", engine.determineOverallStatus(highScoreCandidate))

	// Test BLOCKED status
	blockedCandidate := &PreMovementCandidate{
		TotalScore:  85.0,
		GatesStatus: "BLOCKED",
		CVDResult:   &CVDResidualResult{IsSignificant: true},
	}
	assert.Equal(t, "BLOCKED", engine.determineOverallStatus(blockedCandidate))

	// Test WEAK status
	weakCandidate := &PreMovementCandidate{
		TotalScore:  65.0,
		GatesStatus: "CONFIRMED",
		CVDResult:   &CVDResidualResult{IsSignificant: false},
	}
	assert.Equal(t, "WEAK", engine.determineOverallStatus(weakCandidate))
}

func TestPreMovementEngine_RankCandidates(t *testing.T) {
	engine := NewPreMovementEngine(nil, nil)

	candidates := []*PreMovementCandidate{
		{
			Symbol:        "WEAK",
			TotalScore:    60.0,
			OverallStatus: "WEAK",
			GatesResult:   &ConfirmationResult{PrecedenceScore: 2.0},
			CVDResult:     &CVDResidualResult{SignificanceScore: 0.3},
		},
		{
			Symbol:        "STRONG",
			TotalScore:    90.0,
			OverallStatus: "STRONG",
			GatesResult:   &ConfirmationResult{PrecedenceScore: 6.0},
			CVDResult:     &CVDResidualResult{SignificanceScore: 0.8},
		},
		{
			Symbol:        "MODERATE",
			TotalScore:    75.0,
			OverallStatus: "MODERATE",
			GatesResult:   &ConfirmationResult{PrecedenceScore: 5.0},
			CVDResult:     &CVDResidualResult{SignificanceScore: 0.5},
		},
		{
			Symbol:        "BLOCKED",
			TotalScore:    85.0,
			OverallStatus: "BLOCKED",
			GatesResult:   &ConfirmationResult{PrecedenceScore: 3.0},
			CVDResult:     nil,
		},
	}

	engine.rankCandidates(candidates)

	// Check ranking order
	assert.Equal(t, "STRONG", candidates[0].Symbol, "STRONG should rank first")
	assert.Equal(t, 1, candidates[0].Rank)

	assert.Equal(t, "MODERATE", candidates[1].Symbol, "MODERATE should rank second")
	assert.Equal(t, 2, candidates[1].Rank)

	assert.Equal(t, "WEAK", candidates[2].Symbol, "WEAK should rank third")
	assert.Equal(t, 3, candidates[2].Rank)

	assert.Equal(t, "BLOCKED", candidates[3].Symbol, "BLOCKED should rank last")
	assert.Equal(t, 4, candidates[3].Rank)
}

func TestPreMovementEngine_AssessDataFreshness(t *testing.T) {
	engine := NewPreMovementEngine(nil, nil)

	candidates := []*PreMovementCandidate{
		{
			Symbol: "FRESH",
			ScoreBreakdown: &ScoreResult{
				DataFreshness: &FreshnessInfo{OldestFeedHours: 0.2}, // 12 minutes
			},
		},
		{
			Symbol: "MODERATE",
			ScoreBreakdown: &ScoreResult{
				DataFreshness: &FreshnessInfo{OldestFeedHours: 0.8}, // 48 minutes
			},
		},
		{
			Symbol: "STALE",
			ScoreBreakdown: &ScoreResult{
				DataFreshness: &FreshnessInfo{OldestFeedHours: 1.2}, // 72 minutes
			},
		},
	}

	report := engine.assessDataFreshness(candidates)
	assert.NotNil(t, report)

	// Should calculate average age
	expectedAvgSeconds := int64((0.2 + 0.8 + 1.2) / 3.0 * 3600) // Average in seconds
	assert.Equal(t, expectedAvgSeconds, report.AverageAgeSeconds)

	// Should find oldest data
	assert.Equal(t, int64(1.2*3600), report.OldestDataSeconds)

	// Should count stale candidates (>10 min warning threshold)
	assert.Equal(t, 2, report.StaleCandidatesCount, "Should count 2 stale candidates")
	assert.InDelta(t, 66.7, report.StaleCandidatesPct, 1.0, "Should calculate stale percentage")

	// Should assign reasonable freshness grade
	assert.Contains(t, []string{"A", "B", "C", "D", "F"}, report.FreshnessGrade)
}

func TestAnalysisResult_GetAnalysisSummary(t *testing.T) {
	result := &AnalysisResult{
		TotalCandidates:  5,
		ValidCandidates:  3,
		StrongCandidates: 1,
		ProcessTimeMs:    234,
		DataFreshness:    &DataFreshnessReport{FreshnessGrade: "B"},
	}

	summary := result.GetAnalysisSummary()
	assert.Contains(t, summary, "Pre-Movement v3.3 Analysis")
	assert.Contains(t, summary, "5 candidates")
	assert.Contains(t, summary, "3 valid")
	assert.Contains(t, summary, "1 strong")
	assert.Contains(t, summary, "freshness: B")
	assert.Contains(t, summary, "234ms")
}

func TestAnalysisResult_GetTopCandidatesSummary(t *testing.T) {
	result := &AnalysisResult{
		Candidates: []*PreMovementCandidate{
			{
				Rank:          1,
				Symbol:        "BTC-USD",
				OverallStatus: "STRONG",
				TotalScore:    88.5,
				GatesStatus:   "CONFIRMED",
			},
			{
				Rank:          2,
				Symbol:        "ETH-USD",
				OverallStatus: "MODERATE",
				TotalScore:    76.2,
				GatesStatus:   "CONFIRMED",
			},
			{
				Rank:          3,
				Symbol:        "SOL-USD",
				OverallStatus: "WEAK",
				TotalScore:    58.1,
				GatesStatus:   "BLOCKED",
			},
		},
	}

	summary := result.GetTopCandidatesSummary(2)
	assert.Contains(t, summary, "Top 2 Pre-Movement Candidates")
	assert.Contains(t, summary, "1. 🔥 BTC-USD STRONG | Score: 88.5 | Gates: ✅")
	assert.Contains(t, summary, "2. 📈 ETH-USD MODERATE | Score: 76.2 | Gates: ✅")
	assert.NotContains(t, summary, "SOL-USD") // Should not include 3rd candidate
}

func TestAnalysisResult_GetCandidateDetails(t *testing.T) {
	result := &AnalysisResult{
		Candidates: []*PreMovementCandidate{
			{
				Rank:          1,
				Symbol:        "BTC-USD",
				OverallStatus: "STRONG",
				TotalScore:    88.5,
				GatesStatus:   "CONFIRMED",
				ProcessTimeMs: 145,
				Reasons:       []string{"High Pre-Movement score (88.5)", "Strong confirmations"},
				ScoreBreakdown: &ScoreResult{
					Symbol:     "BTC-USD",
					TotalScore: 88.5,
					ComponentScores: map[string]float64{
						"derivatives": 12.5,
						"smart_money": 18.2,
					},
				},
			},
		},
	}

	details := result.GetCandidateDetails("BTC-USD")
	assert.Contains(t, details, "BTC-USD (Rank #1)")
	assert.Contains(t, details, "Status: STRONG")
	assert.Contains(t, details, "Score: 88.5")
	assert.Contains(t, details, "Gates: CONFIRMED")
	assert.Contains(t, details, "Time: 145ms")
	assert.Contains(t, details, "High Pre-Movement score (88.5)")

	// Test non-existent candidate
	notFound := result.GetCandidateDetails("UNKNOWN")
	assert.Contains(t, notFound, "Candidate UNKNOWN not found")
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()
	require.NotNil(t, config)

	// Should have all sub-configs
	assert.NotNil(t, config.ScoreConfig)
	assert.NotNil(t, config.GateConfig)
	assert.NotNil(t, config.CVDConfig)

	// Check API limits
	assert.Equal(t, 50, config.MaxCandidates)
	assert.Equal(t, int64(2000), config.MaxProcessTimeMs)
	assert.True(t, config.RequireScore)
	assert.True(t, config.RequireGates)

	// Check data freshness requirements
	assert.Equal(t, int64(1800), config.MaxDataStaleness) // 30 minutes
	assert.Equal(t, int64(600), config.StaleDataWarning)  // 10 minutes
}

func TestPreMovementEngine_PerformanceRequirements(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			SpreadBps: 25.0,
			DepthUSD:  200000.0,
			VADR:      2.0,
			Healthy:   true,
		},
	}

	engine := NewPreMovementEngine(mockMicro, nil)

	// Create multiple candidate inputs for performance test
	inputs := make([]*CandidateInput, 10)
	for i := 0; i < 10; i++ {
		inputs[i] = &CandidateInput{
			Symbol:    fmt.Sprintf("TEST-%d", i),
			Timestamp: time.Now(),
			PreMovementData: &PreMovementData{
				Symbol:             fmt.Sprintf("TEST-%d", i),
				Timestamp:          time.Now(),
				FundingZScore:      2.0 + float64(i)*0.1,
				OIResidual:         500000,
				ETFFlowTint:        0.5,
				ReserveChange7d:    -5.0,
				WhaleComposite:     0.6,
				MicroDynamics:      0.4,
				SmartMoneyFlow:     0.5,
				CVDResidual:        0.3,
				CatalystHeat:       0.4,
				VolCompressionRank: 0.5,
				OldestFeedHours:    1.0,
			},
			ConfirmationData: &ConfirmationData{
				Symbol:           fmt.Sprintf("TEST-%d", i),
				Timestamp:        time.Now(),
				FundingZScore:    2.0 + float64(i)*0.1,
				WhaleComposite:   0.6 + float64(i)*0.01,
				SupplyProxyScore: 0.5,
				VolumeRatio24h:   2.0,
				CurrentRegime:    "normal",
			},
			CVDDataPoints: generateSyntheticCVDData(50, 0.6),
		}
	}

	start := time.Now()
	result, err := engine.ListCandidates(context.Background(), inputs, 10)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Less(t, duration.Milliseconds(), int64(3000), "Should complete 10 candidates in <3s")
	assert.Greater(t, result.ProcessTimeMs, int64(0), "Should report processing time")
	assert.LessOrEqual(t, len(result.Candidates), 10, "Should respect candidate limit")
}
