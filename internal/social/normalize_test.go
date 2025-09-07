package social

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizer_NormalizeDataPoint_WithStatistics(t *testing.T) {
	config := DefaultNormalizationConfig()
	normalizer := NewNormalizer(config, nil)

	// Create mock rolling statistics
	stats := &RollingStats{
		Mean:              50.0,
		StandardDeviation: 15.0,
		Minimum:           10.0,
		Maximum:           90.0,
		Count:             50,
		WindowStart:       time.Now().AddDate(0, 0, -30),
		WindowEnd:         time.Now(),
	}

	// Test data point with 1 standard deviation above mean
	rawPoint := &RawDataPoint{
		Value:     65.0, // 1σ above mean
		Timestamp: time.Now(),
		SourceID:  "test",
		TTL:       1 * time.Hour,
		Quality:   "A",
	}

	normalized := normalizer.normalizeDataPoint(rawPoint, stats, "test_metric")
	require.NotNil(t, normalized)

	// Verify basic fields
	assert.Equal(t, 65.0, normalized.RawValue)
	assert.Equal(t, "test", normalized.SourceID)
	assert.Equal(t, "A", normalized.Quality)
	assert.Equal(t, "zscore_minmax", normalized.TransformMethod)
	assert.Equal(t, stats, normalized.StatisticsUsed)

	// Verify z-score calculation
	expectedZScore := (65.0 - 50.0) / 15.0 // = 1.0
	assert.InDelta(t, expectedZScore, normalized.ZScore, 0.01, "Z-score should be calculated correctly")

	// Verify percentile calculation (rough check)
	assert.Greater(t, normalized.Percentile, 50.0, "Value above mean should have >50th percentile")
	assert.Less(t, normalized.Percentile, 100.0, "Percentile should be <100")

	// Verify normalized value is in 0-1 range
	assert.GreaterOrEqual(t, normalized.NormalizedValue, 0.0, "Normalized value should be >= 0")
	assert.LessOrEqual(t, normalized.NormalizedValue, 1.0, "Normalized value should be <= 1")

	// For 1σ above mean, normalized value should be > 0.5
	assert.Greater(t, normalized.NormalizedValue, 0.5, "Above-mean values should normalize to >0.5")
}

func TestNormalizer_NormalizeDataPoint_ExtremezScoreClipping(t *testing.T) {
	config := DefaultNormalizationConfig()
	config.ZScoreClipEnabled = true
	config.ZScoreClipThreshold = 3.0

	normalizer := NewNormalizer(config, nil)

	stats := &RollingStats{
		Mean:              50.0,
		StandardDeviation: 10.0,
		Count:             30,
	}

	// Test extreme value (5σ above mean, should be clipped to 3σ)
	extremePoint := &RawDataPoint{
		Value:     100.0, // 5σ above mean
		Timestamp: time.Now(),
		SourceID:  "test",
		Quality:   "B",
	}

	normalized := normalizer.normalizeDataPoint(extremePoint, stats, "extreme_test")
	require.NotNil(t, normalized)

	// Z-score should be clipped to threshold
	assert.Equal(t, 3.0, normalized.ZScore, "Z-score should be clipped to threshold")

	// Normalized value should still be in bounds
	assert.GreaterOrEqual(t, normalized.NormalizedValue, 0.0)
	assert.LessOrEqual(t, normalized.NormalizedValue, 1.0)
	assert.Greater(t, normalized.NormalizedValue, 0.8, "Extreme positive value should normalize near 1.0")
}

func TestNormalizer_NormalizeDataPoint_NoStatistics(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	rawPoint := &RawDataPoint{
		Value:     0.75, // Already in 0-1 range
		Timestamp: time.Now(),
		SourceID:  "test",
		Quality:   "C",
	}

	normalized := normalizer.normalizeDataPoint(rawPoint, nil, "no_stats_test")
	require.NotNil(t, normalized)

	// Should fall back to raw clamping
	assert.Equal(t, 0.75, normalized.NormalizedValue, "Should use raw value when no statistics")
	assert.Equal(t, 0.0, normalized.ZScore, "Z-score should be 0 when no statistics")
	assert.Equal(t, 50.0, normalized.Percentile, "Should assume median percentile")
	assert.Equal(t, "raw_clamp", normalized.TransformMethod, "Should use raw_clamp method")
}

func TestNormalizer_NormalizeSentimentScore(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	testCases := []struct {
		name               string
		sentimentValue     float64
		expectedNormalized float64
		expectedPercentile float64
	}{
		{
			name:               "strongly_negative",
			sentimentValue:     -0.8,
			expectedNormalized: 0.1, // (-0.8 + 1) / 2 = 0.1
			expectedPercentile: 10.0,
		},
		{
			name:               "neutral",
			sentimentValue:     0.0,
			expectedNormalized: 0.5, // (0 + 1) / 2 = 0.5
			expectedPercentile: 50.0,
		},
		{
			name:               "strongly_positive",
			sentimentValue:     0.6,
			expectedNormalized: 0.8, // (0.6 + 1) / 2 = 0.8
			expectedPercentile: 80.0,
		},
		{
			name:               "extreme_negative",
			sentimentValue:     -1.0,
			expectedNormalized: 0.0, // (-1 + 1) / 2 = 0
			expectedPercentile: 0.0,
		},
		{
			name:               "extreme_positive",
			sentimentValue:     1.0,
			expectedNormalized: 1.0, // (1 + 1) / 2 = 1
			expectedPercentile: 100.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawSentiment := &RawDataPoint{
				Value:     tc.sentimentValue,
				Timestamp: time.Now(),
				SourceID:  "sentiment_test",
				Quality:   "B",
			}

			normalized := normalizer.normalizeSentimentScore(rawSentiment)
			require.NotNil(t, normalized)

			assert.InDelta(t, tc.expectedNormalized, normalized.NormalizedValue, 0.01,
				"Normalized sentiment value should match expected")
			assert.InDelta(t, tc.expectedPercentile, normalized.Percentile, 0.01,
				"Percentile should match expected")
			assert.Equal(t, "sentiment_linear", normalized.TransformMethod,
				"Should use sentiment_linear transform method")
			assert.Nil(t, normalized.StatisticsUsed, "Sentiment doesn't use rolling statistics")
		})
	}
}

func TestNormalizer_NormalizeMetrics_FullPipeline(t *testing.T) {
	normalizer := NewNormalizer(nil, nil) // Use defaults

	// Create comprehensive raw metrics
	rawMetrics := &RawSocialMetrics{
		Asset:     "BTC-USD",
		Timestamp: time.Now(),
		Developer: &DeveloperMetrics{
			CommitFrequency: &RawDataPoint{
				Value: 25.0, Timestamp: time.Now(), SourceID: "github", Quality: "A",
			},
			ActiveContributors: &RawDataPoint{
				Value: 8.0, Timestamp: time.Now(), SourceID: "github", Quality: "A",
			},
			CodeQuality: &RawDataPoint{
				Value: 0.85, Timestamp: time.Now(), SourceID: "github", Quality: "B",
			},
		},
		Community: &CommunityMetrics{
			StarGrowth: &RawDataPoint{
				Value: 40.0, Timestamp: time.Now(), SourceID: "github", Quality: "A",
			},
			CommunitySize: &RawDataPoint{
				Value: 5000.0, Timestamp: time.Now(), SourceID: "coingecko", Quality: "B",
			},
		},
		News: &NewsMetrics{
			MentionFrequency: &RawDataPoint{
				Value: 120.0, Timestamp: time.Now(), SourceID: "news_api", Quality: "C",
			},
			SentimentScore: &RawDataPoint{
				Value: 0.4, Timestamp: time.Now(), SourceID: "news_api", Quality: "C",
			},
		},
	}

	normalized, err := normalizer.NormalizeMetrics(context.Background(), "BTC-USD", rawMetrics)
	require.NoError(t, err)
	require.NotNil(t, normalized)

	// Verify top-level structure
	assert.Equal(t, "BTC-USD", normalized.Asset)
	assert.WithinDuration(t, time.Now(), normalized.Timestamp, 5*time.Second)

	// Verify developer metrics normalization
	require.NotNil(t, normalized.Developer)
	assert.NotNil(t, normalized.Developer.CommitFrequency)
	assert.NotNil(t, normalized.Developer.ActiveContributors)
	assert.NotNil(t, normalized.Developer.CodeQuality)

	// Check that normalized values are in correct range
	devMetrics := []*NormalizedDataPoint{
		normalized.Developer.CommitFrequency,
		normalized.Developer.ActiveContributors,
		normalized.Developer.CodeQuality,
	}

	for i, metric := range devMetrics {
		assert.GreaterOrEqual(t, metric.NormalizedValue, 0.0, "Developer metric %d should be >= 0", i)
		assert.LessOrEqual(t, metric.NormalizedValue, 1.0, "Developer metric %d should be <= 1", i)
		assert.Equal(t, "github", metric.SourceID, "Should preserve source ID")
	}

	// Verify community metrics normalization
	require.NotNil(t, normalized.Community)
	assert.NotNil(t, normalized.Community.StarGrowth)
	assert.NotNil(t, normalized.Community.CommunitySize)

	// Verify news metrics normalization
	require.NotNil(t, normalized.News)
	assert.NotNil(t, normalized.News.MentionFrequency)
	assert.NotNil(t, normalized.News.SentimentScore)

	// Sentiment should be specifically transformed from [-1,1] to [0,1]
	sentiment := normalized.News.SentimentScore
	assert.Equal(t, 0.7, sentiment.NormalizedValue, "Sentiment 0.4 should normalize to 0.7")
	assert.Equal(t, "sentiment_linear", sentiment.TransformMethod)
}

func TestNormalizer_WinsorizeValues(t *testing.T) {
	config := DefaultNormalizationConfig()
	normalizer := NewNormalizer(config, nil)

	// Create dataset with outliers
	values := []float64{
		1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, // Normal values
		50.0, 100.0, // Outliers
	}

	winsorized := normalizer.winsorizeValues(values)

	// Should remove outliers but keep normal values
	assert.Less(t, len(winsorized), len(values), "Should remove some outliers")
	assert.GreaterOrEqual(t, len(winsorized), 10, "Should keep most normal values")

	// Check that extreme outliers are removed
	for _, val := range winsorized {
		assert.Less(t, val, 50.0, "Extreme outliers should be removed")
	}
}

func TestNormalizer_WinsorizeValues_SmallDataset(t *testing.T) {
	config := DefaultNormalizationConfig()
	normalizer := NewNormalizer(config, nil)

	// Small dataset should not be winsorized
	smallValues := []float64{1.0, 2.0, 3.0, 100.0} // Include outlier

	winsorized := normalizer.winsorizeValues(smallValues)

	// Should return original dataset unchanged
	assert.Equal(t, len(smallValues), len(winsorized), "Small datasets should not be winsorized")
	assert.ElementsMatch(t, smallValues, winsorized, "Values should be unchanged")
}

func TestNormalizer_ClampToRange(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	testCases := []struct {
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{5.0, 0.0, 10.0, 5.0},   // Within range
		{-2.0, 0.0, 10.0, 0.0},  // Below range
		{15.0, 0.0, 10.0, 10.0}, // Above range
		{0.5, 0.0, 1.0, 0.5},    // Exact bounds check
		{1.1, 0.0, 1.0, 1.0},    // Slightly above
	}

	for i, tc := range testCases {
		result := normalizer.clampToRange(tc.value, tc.min, tc.max)
		assert.Equal(t, tc.expected, result, "Test case %d: clamp(%.2f, %.2f, %.2f)", i, tc.value, tc.min, tc.max)
	}
}

func TestNormalizer_CalculatePercentileRank(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	stats := &RollingStats{
		Minimum: 10.0,
		Maximum: 90.0,
	}

	testCases := []struct {
		value        float64
		expectedRank float64
		description  string
	}{
		{10.0, 0.0, "minimum value should be 0th percentile"},
		{90.0, 100.0, "maximum value should be 100th percentile"},
		{50.0, 50.0, "middle value should be 50th percentile"},
		{30.0, 25.0, "25% between min and max"},
		{70.0, 75.0, "75% between min and max"},
	}

	for _, tc := range testCases {
		rank := normalizer.calculatePercentileRank(tc.value, stats)
		assert.Equal(t, tc.expectedRank, rank, tc.description)
	}
}

func TestNormalizer_CalculatePercentileRank_NoRange(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	// Stats with no range (min == max)
	statsNoRange := &RollingStats{
		Minimum: 50.0,
		Maximum: 50.0,
	}

	rank := normalizer.calculatePercentileRank(42.0, statsNoRange)
	assert.Equal(t, 50.0, rank, "Should return median when no range available")

	// Nil stats
	rankNil := normalizer.calculatePercentileRank(42.0, nil)
	assert.Equal(t, 50.0, rankNil, "Should return median when stats are nil")
}

func TestDefaultNormalizationConfig(t *testing.T) {
	config := DefaultNormalizationConfig()
	require.NotNil(t, config)

	// Verify rolling window parameters
	assert.Equal(t, 30, config.WindowSizeDays, "Should use 30-day rolling window")
	assert.Equal(t, 10, config.MinDataPoints, "Should require minimum 10 data points")
	assert.Equal(t, 100, config.MaxDataPointsForStats, "Should limit to 100 data points for stats")

	// Verify winsorization parameters
	assert.True(t, config.WinsorizeEnabled, "Winsorization should be enabled")
	assert.Equal(t, 5.0, config.WinsorizeLowerPct, "Should use 5% lower bound")
	assert.Equal(t, 95.0, config.WinsorizeUpperPct, "Should use 95% upper bound")

	// Verify z-score parameters
	assert.True(t, config.ZScoreClipEnabled, "Z-score clipping should be enabled")
	assert.Equal(t, 3.0, config.ZScoreClipThreshold, "Should clip at 3 standard deviations")

	// Verify min-max normalization bounds
	assert.Equal(t, 0.0, config.MinMaxFloor, "Floor should be 0.0")
	assert.Equal(t, 1.0, config.MinMaxCeiling, "Ceiling should be 1.0")

	// Verify cache parameters
	assert.Equal(t, 6*time.Hour, config.StatsCacheTTL, "Stats should cache for 6 hours")
	assert.True(t, config.UseStaleStatsOnError, "Should use stale stats on error")
}

func TestNormalizer_ComputeRollingStatistics(t *testing.T) {
	normalizer := NewNormalizer(nil, nil)

	stats, err := normalizer.computeRollingStatistics(context.Background(), "TEST-USD")
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, "TEST-USD", stats.Asset, "Should set correct asset")
	assert.WithinDuration(t, time.Now(), stats.LastUpdated, 5*time.Second, "Should have recent update time")

	// Verify all metric categories have statistics
	assert.NotNil(t, stats.DeveloperStats, "Should have developer statistics")
	assert.NotNil(t, stats.CommunityStats, "Should have community statistics")
	assert.NotNil(t, stats.NewsStats, "Should have news statistics")

	// Check that expected metrics are present
	expectedDevMetrics := []string{"commit_frequency", "active_contributors", "code_quality", "release_frequency", "issue_resolution"}
	for _, metric := range expectedDevMetrics {
		assert.Contains(t, stats.DeveloperStats, metric, "Should have %s in developer stats", metric)
		stat := stats.DeveloperStats[metric]
		assert.GreaterOrEqual(t, stat.Count, 10, "Should meet minimum data point requirement")
		assert.Greater(t, stat.StandardDeviation, 0.0, "Should have positive standard deviation")
	}

	expectedCommunityMetrics := []string{"star_growth", "fork_ratio", "community_size", "engagement_rate", "social_mentions"}
	for _, metric := range expectedCommunityMetrics {
		assert.Contains(t, stats.CommunityStats, metric, "Should have %s in community stats", metric)
	}

	expectedNewsMetrics := []string{"mention_frequency", "authority_score", "trending_score", "category_relevance"}
	for _, metric := range expectedNewsMetrics {
		assert.Contains(t, stats.NewsStats, metric, "Should have %s in news stats", metric)
	}
}

func TestRollingStats_WindowValidation(t *testing.T) {
	stats := &RollingStats{
		Mean:              42.5,
		StandardDeviation: 12.3,
		Minimum:           5.0,
		Maximum:           98.7,
		Count:             45,
		WindowStart:       time.Now().AddDate(0, 0, -30),
		WindowEnd:         time.Now(),
		WinsorizedCount:   3,
	}

	// Verify window duration makes sense
	windowDuration := stats.WindowEnd.Sub(stats.WindowStart)
	assert.Greater(t, windowDuration, 25*24*time.Hour, "Window should be at least 25 days")
	assert.Less(t, windowDuration, 35*24*time.Hour, "Window should be at most 35 days")

	// Verify statistical consistency
	assert.Greater(t, stats.Maximum, stats.Minimum, "Maximum should be greater than minimum")
	assert.Greater(t, stats.StandardDeviation, 0.0, "Standard deviation should be positive")
	assert.Greater(t, stats.Count, 0, "Count should be positive")
	assert.LessOrEqual(t, stats.WinsorizedCount, stats.Count, "Winsorized count should not exceed total count")
}

func TestNormalizedDataPoint_CompleteStructure(t *testing.T) {
	point := &NormalizedDataPoint{
		NormalizedValue: 0.75,
		RawValue:        123.45,
		ZScore:          1.5,
		Percentile:      85.2,
		Timestamp:       time.Now(),
		SourceID:        "test_source",
		TTL:             2 * time.Hour,
		Quality:         "B",
		TransformMethod: "zscore_minmax",
		StatisticsUsed: &RollingStats{
			Mean:              100.0,
			StandardDeviation: 15.0,
			Count:             50,
		},
	}

	// Verify all fields are accessible and have expected types
	assert.Equal(t, 0.75, point.NormalizedValue)
	assert.Equal(t, 123.45, point.RawValue)
	assert.Equal(t, 1.5, point.ZScore)
	assert.Equal(t, 85.2, point.Percentile)
	assert.Equal(t, "test_source", point.SourceID)
	assert.Equal(t, 2*time.Hour, point.TTL)
	assert.Equal(t, "B", point.Quality)
	assert.Equal(t, "zscore_minmax", point.TransformMethod)
	assert.NotNil(t, point.StatisticsUsed)
	assert.Equal(t, 100.0, point.StatisticsUsed.Mean)
}

func TestNormalizedSocialMetrics_StructureValidation(t *testing.T) {
	normalized := &NormalizedSocialMetrics{
		Asset:     "ETH-USD",
		Timestamp: time.Now(),
		Developer: &NormalizedDeveloperMetrics{
			CommitFrequency: &NormalizedDataPoint{NormalizedValue: 0.6},
		},
		Community: &NormalizedCommunityMetrics{
			StarGrowth: &NormalizedDataPoint{NormalizedValue: 0.8},
		},
		News: &NormalizedNewsMetrics{
			SentimentScore: &NormalizedDataPoint{NormalizedValue: 0.7},
		},
	}

	// Verify structure integrity
	assert.Equal(t, "ETH-USD", normalized.Asset)
	require.NotNil(t, normalized.Developer)
	require.NotNil(t, normalized.Developer.CommitFrequency)
	assert.Equal(t, 0.6, normalized.Developer.CommitFrequency.NormalizedValue)

	require.NotNil(t, normalized.Community)
	require.NotNil(t, normalized.Community.StarGrowth)
	assert.Equal(t, 0.8, normalized.Community.StarGrowth.NormalizedValue)

	require.NotNil(t, normalized.News)
	require.NotNil(t, normalized.News.SentimentScore)
	assert.Equal(t, 0.7, normalized.News.SentimentScore.NormalizedValue)
}
