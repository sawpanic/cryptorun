package social

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubSource_GetDeveloperActivity(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewGitHubSource(config, nil, nil)

	result, err := source.GetDeveloperActivity(context.Background(), "BTC-USD")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check that all expected metrics are present
	assert.NotNil(t, result.CommitFrequency, "Should have commit frequency data")
	assert.NotNil(t, result.ActiveContributors, "Should have active contributors data")
	assert.NotNil(t, result.CodeQuality, "Should have code quality data")
	assert.NotNil(t, result.ReleaseFrequency, "Should have release frequency data")
	assert.NotNil(t, result.IssueResolution, "Should have issue resolution data")

	// Verify data point structure
	commit := result.CommitFrequency
	assert.Greater(t, commit.Value, 0.0, "Commit frequency should be positive")
	assert.Equal(t, "github", commit.SourceID, "Should have correct source ID")
	assert.Equal(t, "A", commit.Quality, "GitHub should provide A-grade data")
	assert.Equal(t, 6*time.Hour, commit.TTL, "Should have 6-hour TTL")
	assert.WithinDuration(t, time.Now(), commit.Timestamp, 5*time.Second, "Timestamp should be recent")
}

func TestGitHubSource_GetCommunityMetrics(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewGitHubSource(config, nil, nil)

	result, err := source.GetCommunityMetrics(context.Background(), "ETH-USD")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check expected metrics
	assert.NotNil(t, result.StarGrowth, "Should have star growth data")
	assert.NotNil(t, result.ForkRatio, "Should have fork ratio data")
	assert.NotNil(t, result.CommunitySize, "Should have community size data")

	// Verify data quality
	star := result.StarGrowth
	assert.Greater(t, star.Value, 0.0, "Star growth should be positive")
	assert.Equal(t, "github", star.SourceID)
	assert.Equal(t, "A", star.Quality)
}

func TestGitHubSource_GetNewsMetrics(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewGitHubSource(config, nil, nil)

	result, err := source.GetNewsMetrics(context.Background(), "SOL-USD")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// GitHub doesn't provide news metrics, should be empty
	assert.Nil(t, result.MentionFrequency)
	assert.Nil(t, result.SentimentScore)
	assert.Nil(t, result.AuthorityScore)
}

func TestGitHubSource_GetSourceInfo(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewGitHubSource(config, nil, nil)

	info := source.GetSourceInfo()
	require.NotNil(t, info)

	assert.Equal(t, "github", info.ID)
	assert.Equal(t, "GitHub Developer Activity", info.Name)
	assert.Equal(t, "A", info.ReliabilityGrade)
	assert.Equal(t, 6*time.Hour, info.TTL)
	assert.True(t, info.IsKeyless)
	assert.True(t, info.RespectRobotsTxt)

	// Check rate limiting configuration
	require.NotNil(t, info.RateLimit)
	assert.Equal(t, 1.0, info.RateLimit.RequestsPerSecond)
	assert.Equal(t, 5, info.RateLimit.BurstSize)
	assert.Equal(t, 2.0, info.RateLimit.BackoffMultiplier)
	assert.Equal(t, 5*time.Minute, info.RateLimit.MaxBackoff)
}

func TestCoinGeckoSource_GetDeveloperActivity(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewCoinGeckoSource(config, nil, nil)

	result, err := source.GetDeveloperActivity(context.Background(), "ADA-USD")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// CoinGecko provides limited developer data
	assert.NotNil(t, result.CommitFrequency, "Should have commit frequency data")
	assert.Nil(t, result.ActiveContributors, "CoinGecko doesn't track contributors directly")

	commit := result.CommitFrequency
	assert.Greater(t, commit.Value, 0.0, "Should have positive commit frequency")
	assert.Equal(t, "coingecko", commit.SourceID)
	assert.Equal(t, "B", commit.Quality, "CoinGecko provides B-grade data")
	assert.Equal(t, 24*time.Hour, commit.TTL, "Should have longer TTL than GitHub")
}

func TestCoinGeckoSource_GetCommunityMetrics(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewCoinGeckoSource(config, nil, nil)

	result, err := source.GetCommunityMetrics(context.Background(), "LINK-USD")
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.NotNil(t, result.CommunitySize, "Should have community size data")
	assert.NotNil(t, result.SocialMentions, "Should have social mentions data")

	// Verify community size data
	community := result.CommunitySize
	assert.Greater(t, community.Value, 0.0, "Community size should be positive")
	assert.Equal(t, "coingecko", community.SourceID)
	assert.Equal(t, "B", community.Quality)

	// Verify social mentions (lower quality)
	mentions := result.SocialMentions
	assert.Greater(t, mentions.Value, 0.0, "Social mentions should be positive")
	assert.Equal(t, "C", mentions.Quality, "Social mentions are C-grade data")
	assert.Equal(t, 2*time.Hour, mentions.TTL, "Should have short TTL for social data")
}

func TestCoinGeckoSource_GetSourceInfo(t *testing.T) {
	config := DefaultSourceConfig()
	source := NewCoinGeckoSource(config, nil, nil)

	info := source.GetSourceInfo()
	require.NotNil(t, info)

	assert.Equal(t, "coingecko", info.ID)
	assert.Equal(t, "CoinGecko Community Data", info.Name)
	assert.Equal(t, "B", info.ReliabilityGrade)
	assert.Equal(t, 12*time.Hour, info.TTL)
	assert.True(t, info.IsKeyless)
	assert.True(t, info.RespectRobotsTxt)

	// Check conservative rate limiting for free tier
	require.NotNil(t, info.RateLimit)
	assert.Equal(t, 0.1, info.RateLimit.RequestsPerSecond, "Should have very conservative rate limit")
	assert.Equal(t, 3, info.RateLimit.BurstSize)
	assert.Equal(t, 10*time.Minute, info.RateLimit.MaxBackoff)
}

func TestFakeSocialSource_DefaultBehavior(t *testing.T) {
	source := NewFakeSocialSource("test", "A")

	// Test developer metrics
	dev, err := source.GetDeveloperActivity(context.Background(), "TEST-USD")
	require.NoError(t, err)
	assert.NotNil(t, dev)

	assert.NotNil(t, dev.CommitFrequency)
	assert.Equal(t, 10.0, dev.CommitFrequency.Value)
	assert.Equal(t, "test", dev.CommitFrequency.SourceID)
	assert.Equal(t, "A", dev.CommitFrequency.Quality)

	assert.NotNil(t, dev.ActiveContributors)
	assert.Equal(t, 5.0, dev.ActiveContributors.Value)

	// Test community metrics
	comm, err := source.GetCommunityMetrics(context.Background(), "TEST-USD")
	require.NoError(t, err)
	assert.NotNil(t, comm)

	assert.NotNil(t, comm.StarGrowth)
	assert.Equal(t, 20.0, comm.StarGrowth.Value)

	assert.NotNil(t, comm.CommunitySize)
	assert.Equal(t, 1000.0, comm.CommunitySize.Value)

	// Test news metrics
	news, err := source.GetNewsMetrics(context.Background(), "TEST-USD")
	require.NoError(t, err)
	assert.NotNil(t, news)

	assert.NotNil(t, news.MentionFrequency)
	assert.Equal(t, 15.0, news.MentionFrequency.Value)

	assert.NotNil(t, news.SentimentScore)
	assert.Equal(t, 0.3, news.SentimentScore.Value, "Should have slightly positive sentiment")
}

func TestFakeSocialSource_FailureMode(t *testing.T) {
	source := NewFakeSocialSource("failing", "C")
	source.SetFailure(true)

	// All methods should return errors when configured to fail
	dev, err := source.GetDeveloperActivity(context.Background(), "FAIL-USD")
	assert.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "fake source configured to fail")

	comm, err := source.GetCommunityMetrics(context.Background(), "FAIL-USD")
	assert.Error(t, err)
	assert.Nil(t, comm)

	news, err := source.GetNewsMetrics(context.Background(), "FAIL-USD")
	assert.Error(t, err)
	assert.Nil(t, news)

	// Source info should still work
	info := source.GetSourceInfo()
	assert.NotNil(t, info)
	assert.Equal(t, "failing", info.ID)
	assert.Equal(t, "C", info.ReliabilityGrade)
}

func TestFakeSocialSource_QualityGrades(t *testing.T) {
	grades := []string{"A", "B", "C", "D"}

	for _, grade := range grades {
		source := NewFakeSocialSource("test_"+grade, grade)

		dev, err := source.GetDeveloperActivity(context.Background(), "TEST-USD")
		require.NoError(t, err)

		assert.Equal(t, grade, dev.CommitFrequency.Quality, "Should use configured quality grade")
		assert.Equal(t, grade, source.GetSourceInfo().ReliabilityGrade, "Source info should match")
	}
}

func TestDefaultSourceConfig(t *testing.T) {
	config := DefaultSourceConfig()
	require.NotNil(t, config)

	assert.True(t, config.Enabled)
	assert.Equal(t, "", config.BaseURL) // Empty by default
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, "social:", config.CacheKeyPrefix)

	// GitHub-specific defaults
	assert.Equal(t, "", config.GitHubOwner)
	assert.Equal(t, "", config.GitHubRepo)

	// CoinGecko-specific defaults
	assert.Equal(t, "", config.CoinGeckoCoinID)
}

func TestRawDataPoint_Validation(t *testing.T) {
	dataPoint := &RawDataPoint{
		Value:     42.5,
		Timestamp: time.Now(),
		SourceID:  "test",
		TTL:       1 * time.Hour,
		Quality:   "A",
	}

	// Verify all fields are set correctly
	assert.Equal(t, 42.5, dataPoint.Value)
	assert.Equal(t, "test", dataPoint.SourceID)
	assert.Equal(t, 1*time.Hour, dataPoint.TTL)
	assert.Equal(t, "A", dataPoint.Quality)
	assert.WithinDuration(t, time.Now(), dataPoint.Timestamp, 5*time.Second)
}

func TestSourceInfo_RateLimitValidation(t *testing.T) {
	rateLimit := &RateLimit{
		RequestsPerSecond: 2.5,
		BurstSize:         10,
		BackoffMultiplier: 1.5,
		MaxBackoff:        2 * time.Minute,
	}

	sourceInfo := &SourceInfo{
		ID:               "test",
		Name:             "Test Source",
		ReliabilityGrade: "B",
		TTL:              4 * time.Hour,
		RateLimit:        rateLimit,
		IsKeyless:        true,
		RespectRobotsTxt: true,
	}

	// Validate rate limit configuration
	assert.Equal(t, 2.5, sourceInfo.RateLimit.RequestsPerSecond)
	assert.Equal(t, 10, sourceInfo.RateLimit.BurstSize)
	assert.Equal(t, 1.5, sourceInfo.RateLimit.BackoffMultiplier)
	assert.Equal(t, 2*time.Minute, sourceInfo.RateLimit.MaxBackoff)

	// Validate source info
	assert.Equal(t, "test", sourceInfo.ID)
	assert.Equal(t, "Test Source", sourceInfo.Name)
	assert.Equal(t, "B", sourceInfo.ReliabilityGrade)
	assert.True(t, sourceInfo.IsKeyless)
	assert.True(t, sourceInfo.RespectRobotsTxt)
}

func TestDeveloperMetrics_AllFields(t *testing.T) {
	metrics := &DeveloperMetrics{
		CommitFrequency: &RawDataPoint{
			Value:     25.0,
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "A",
		},
		ActiveContributors: &RawDataPoint{
			Value:     12.0,
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "A",
		},
		CodeQuality: &RawDataPoint{
			Value:     0.85,
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       12 * time.Hour,
			Quality:   "B",
		},
		ReleaseFrequency: &RawDataPoint{
			Value:     3.5,
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       24 * time.Hour,
			Quality:   "A",
		},
		IssueResolution: &RawDataPoint{
			Value:     0.72,
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "B",
		},
	}

	// Verify all fields are properly structured
	assert.NotNil(t, metrics.CommitFrequency)
	assert.NotNil(t, metrics.ActiveContributors)
	assert.NotNil(t, metrics.CodeQuality)
	assert.NotNil(t, metrics.ReleaseFrequency)
	assert.NotNil(t, metrics.IssueResolution)

	// Verify data ranges make sense
	assert.Greater(t, metrics.CommitFrequency.Value, 0.0)
	assert.Greater(t, metrics.ActiveContributors.Value, 0.0)
	assert.GreaterOrEqual(t, metrics.CodeQuality.Value, 0.0)
	assert.LessOrEqual(t, metrics.CodeQuality.Value, 1.0, "Code quality should be 0-1")
	assert.GreaterOrEqual(t, metrics.IssueResolution.Value, 0.0)
	assert.LessOrEqual(t, metrics.IssueResolution.Value, 1.0, "Issue resolution should be 0-1")
}

func TestCommunityMetrics_AllFields(t *testing.T) {
	metrics := &CommunityMetrics{
		StarGrowth:     &RawDataPoint{Value: 50.0, SourceID: "github", Quality: "A"},
		ForkRatio:      &RawDataPoint{Value: 0.15, SourceID: "github", Quality: "A"},
		CommunitySize:  &RawDataPoint{Value: 2500.0, SourceID: "coingecko", Quality: "B"},
		EngagementRate: &RawDataPoint{Value: 0.08, SourceID: "github", Quality: "B"},
		SocialMentions: &RawDataPoint{Value: 125.0, SourceID: "coingecko", Quality: "C"},
	}

	// Verify reasonable value ranges
	assert.Greater(t, metrics.StarGrowth.Value, 0.0, "Star growth should be positive")
	assert.GreaterOrEqual(t, metrics.ForkRatio.Value, 0.0, "Fork ratio should be non-negative")
	assert.LessOrEqual(t, metrics.ForkRatio.Value, 1.0, "Fork ratio should not exceed 1.0 typically")
	assert.Greater(t, metrics.CommunitySize.Value, 0.0, "Community size should be positive")
	assert.GreaterOrEqual(t, metrics.EngagementRate.Value, 0.0, "Engagement rate should be non-negative")
	assert.LessOrEqual(t, metrics.EngagementRate.Value, 1.0, "Engagement rate should not exceed 1.0")
	assert.Greater(t, metrics.SocialMentions.Value, 0.0, "Social mentions should be positive")
}

func TestNewsMetrics_SentimentRange(t *testing.T) {
	metrics := &NewsMetrics{
		MentionFrequency:  &RawDataPoint{Value: 45.0, Quality: "B"},
		SentimentScore:    &RawDataPoint{Value: 0.2, Quality: "C"}, // Slightly positive
		AuthorityScore:    &RawDataPoint{Value: 0.75, Quality: "B"},
		TrendingScore:     &RawDataPoint{Value: 0.65, Quality: "C"},
		CategoryRelevance: &RawDataPoint{Value: 0.90, Quality: "A"},
	}

	// Verify sentiment is in expected range
	assert.GreaterOrEqual(t, metrics.SentimentScore.Value, -1.0, "Sentiment should be >= -1")
	assert.LessOrEqual(t, metrics.SentimentScore.Value, 1.0, "Sentiment should be <= 1")

	// Verify other scores are 0-1 normalized
	for _, metric := range []*RawDataPoint{metrics.AuthorityScore, metrics.TrendingScore, metrics.CategoryRelevance} {
		assert.GreaterOrEqual(t, metric.Value, 0.0, "Score should be >= 0")
		assert.LessOrEqual(t, metric.Value, 1.0, "Score should be <= 1")
	}

	assert.Greater(t, metrics.MentionFrequency.Value, 0.0, "Mention frequency should be positive")
}
