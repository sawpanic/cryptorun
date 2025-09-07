package social

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocialInputsEngine_FetchSocialInputs_SingleSource(t *testing.T) {
	// Create fake source with known data
	fakeSource := NewFakeSocialSource("test_source", "A")

	normalizer := NewNormalizer(nil, nil)
	engine := NewSocialInputsEngine([]SocialDataSource{fakeSource}, normalizer, nil)

	result, err := engine.FetchSocialInputs(context.Background(), "BTC-USD")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify basic structure
	assert.Equal(t, "BTC-USD", result.Asset)
	assert.WithinDuration(t, time.Now(), result.Timestamp, 5*time.Second)

	// Should have some components from fake data
	assert.Greater(t, len(result.Components), 0, "Should have social components")

	// All normalized scores should be 0-1
	assert.GreaterOrEqual(t, result.DeveloperActivity, 0.0)
	assert.LessOrEqual(t, result.DeveloperActivity, 1.0)
	assert.GreaterOrEqual(t, result.CommunityGrowth, 0.0)
	assert.LessOrEqual(t, result.CommunityGrowth, 1.0)
	assert.GreaterOrEqual(t, result.BrandMentions, 0.0)
	assert.LessOrEqual(t, result.BrandMentions, 1.0)
	assert.GreaterOrEqual(t, result.SocialSentiment, 0.0)
	assert.LessOrEqual(t, result.SocialSentiment, 1.0)
	assert.GreaterOrEqual(t, result.OverallSocial, 0.0)
	assert.LessOrEqual(t, result.OverallSocial, 1.0)

	// Should have data quality assessment
	require.NotNil(t, result.DataQuality)
	assert.Equal(t, 1, result.DataQuality.SourcesAvailable, "Should have 1 source available")
	assert.Equal(t, 1, result.DataQuality.SourcesTotal, "Should have 1 total source")

	// Should have provenance information
	assert.Greater(t, len(result.Provenance), 0, "Should have provenance information")
	prov := result.Provenance[0]
	assert.Equal(t, "test_source", prov.SourceID)
	assert.Equal(t, "A", prov.ReliabilityGrade)

	// Should track processing time
	assert.Greater(t, result.ProcessingTimeMs, int64(0), "Should report processing time")
}

func TestSocialInputsEngine_FetchSocialInputs_MultipleSources(t *testing.T) {
	// Create multiple fake sources with different grades
	sources := []SocialDataSource{
		NewFakeSocialSource("github", "A"),
		NewFakeSocialSource("coingecko", "B"),
		NewFakeSocialSource("news_api", "C"),
	}

	normalizer := NewNormalizer(nil, nil)
	engine := NewSocialInputsEngine(sources, normalizer, nil)

	result, err := engine.FetchSocialInputs(context.Background(), "ETH-USD")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have data from multiple sources
	assert.Equal(t, 3, result.DataQuality.SourcesAvailable, "Should have all 3 sources")
	assert.Equal(t, 3, result.DataQuality.SourcesTotal)

	// Should have multiple provenance entries
	assert.Len(t, result.Provenance, 3, "Should have provenance for all sources")

	// Verify source diversity
	sourceIDs := make(map[string]bool)
	for _, prov := range result.Provenance {
		sourceIDs[prov.SourceID] = true
	}
	assert.Contains(t, sourceIDs, "github")
	assert.Contains(t, sourceIDs, "coingecko")
	assert.Contains(t, sourceIDs, "news_api")

	// Should have good data quality with multiple sources
	assert.Equal(t, 1.0, result.DataQuality.SourceDiversity, "Should have perfect source diversity")
	assert.Contains(t, []string{"A", "B", "C"}, result.DataQuality.OverallGrade, "Should have reasonable quality grade")
}

func TestSocialInputsEngine_FetchSocialInputs_FailedSources(t *testing.T) {
	// Create sources where some fail
	sources := []SocialDataSource{
		NewFakeSocialSource("working", "A"),
		func() *FakeSocialSource {
			failing := NewFakeSocialSource("failing", "B")
			failing.SetFailure(true)
			return failing
		}(),
	}

	config := DefaultEngineConfig()
	config.RequireMinSources = 1 // Only require 1 source minimum

	normalizer := NewNormalizer(nil, nil)
	engine := NewSocialInputsEngine(sources, normalizer, config)

	result, err := engine.FetchSocialInputs(context.Background(), "SOL-USD")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should succeed with 1 working source
	assert.Equal(t, 1, result.DataQuality.SourcesAvailable)
	assert.Equal(t, 2, result.DataQuality.SourcesTotal)

	// Should have provenance only for working source
	assert.Len(t, result.Provenance, 1)
	assert.Equal(t, "working", result.Provenance[0].SourceID)

	// Source diversity should reflect failure
	assert.Equal(t, 0.5, result.DataQuality.SourceDiversity, "Should show 50% source success")
}

func TestSocialInputsEngine_FetchSocialInputs_InsufficientSources(t *testing.T) {
	// Create engine that requires 2 sources but only provide 1 failing source
	failingSource := NewFakeSocialSource("failing", "A")
	failingSource.SetFailure(true)

	config := DefaultEngineConfig()
	config.RequireMinSources = 2 // Require 2 sources minimum

	normalizer := NewNormalizer(nil, nil)
	engine := NewSocialInputsEngine([]SocialDataSource{failingSource}, normalizer, config)

	result, err := engine.FetchSocialInputs(context.Background(), "FAIL-USD")
	assert.Error(t, err, "Should fail when insufficient sources available")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "insufficient data sources")
}

func TestSocialInputsEngine_BuildSocialComponents(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	// Create normalized metrics with known values
	normalized := &NormalizedSocialMetrics{
		Developer: &NormalizedDeveloperMetrics{
			CommitFrequency: &NormalizedDataPoint{
				NormalizedValue: 0.8,
				Timestamp:       time.Now(),
				SourceID:        "github",
				Quality:         "A",
				TTL:             6 * time.Hour,
			},
			ActiveContributors: &NormalizedDataPoint{
				NormalizedValue: 0.6,
				Timestamp:       time.Now(),
				SourceID:        "github",
				Quality:         "A",
				TTL:             6 * time.Hour,
			},
		},
		Community: &NormalizedCommunityMetrics{
			StarGrowth: &NormalizedDataPoint{
				NormalizedValue: 0.9,
				Timestamp:       time.Now(),
				SourceID:        "github",
				Quality:         "A",
				TTL:             6 * time.Hour,
			},
			CommunitySize: &NormalizedDataPoint{
				NormalizedValue: 0.7,
				Timestamp:       time.Now(),
				SourceID:        "coingecko",
				Quality:         "B",
				TTL:             12 * time.Hour,
			},
		},
		News: &NormalizedNewsMetrics{
			MentionFrequency: &NormalizedDataPoint{
				NormalizedValue: 0.5,
				Timestamp:       time.Now(),
				SourceID:        "news_api",
				Quality:         "C",
				TTL:             2 * time.Hour,
			},
			SentimentScore: &NormalizedDataPoint{
				NormalizedValue: 0.75, // Positive sentiment
				Timestamp:       time.Now(),
				SourceID:        "news_api",
				Quality:         "C",
			},
		},
	}

	result := &SocialInputs{
		Components: []*SocialComponent{},
	}

	engine.buildSocialComponents(result, normalized)

	// Should have created components for all provided metrics
	assert.Greater(t, len(result.Components), 4, "Should have multiple components")

	// Verify developer activity calculation
	assert.Greater(t, result.DeveloperActivity, 0.0, "Should calculate developer activity")
	assert.LessOrEqual(t, result.DeveloperActivity, 1.0, "Developer activity should be normalized")

	// Verify community growth calculation
	assert.Greater(t, result.CommunityGrowth, 0.0, "Should calculate community growth")

	// Verify brand mentions calculation
	assert.Greater(t, result.BrandMentions, 0.0, "Should calculate brand mentions")

	// Verify sentiment handling
	assert.Equal(t, 0.75, result.SocialSentiment, "Should use sentiment score directly")

	// Check individual components have correct structure
	foundCommitFreq := false
	for _, comp := range result.Components {
		if comp.Name == "commit_frequency" {
			foundCommitFreq = true
			assert.Equal(t, "developer", comp.Category)
			assert.Equal(t, 0.8, comp.Value)
			assert.Equal(t, 0.3, comp.Weight, "Commit frequency should have 30% weight")
			assert.Equal(t, 0.24, comp.Contribution, "Contribution should be value × weight")
			assert.Equal(t, "A", comp.Quality)
			assert.Equal(t, "github", comp.SourceID)
		}
	}
	assert.True(t, foundCommitFreq, "Should have commit frequency component")
}

func TestSocialInputsEngine_CalculateOverallSocialScore(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	components := []*SocialComponent{
		{
			Name:         "metric1",
			Value:        0.8,
			Weight:       0.3,
			Contribution: 0.24, // 0.8 × 0.3
		},
		{
			Name:         "metric2",
			Value:        0.6,
			Weight:       0.4,
			Contribution: 0.24, // 0.6 × 0.4
		},
		{
			Name:         "metric3",
			Value:        0.5,
			Weight:       0.3,
			Contribution: 0.15, // 0.5 × 0.3
		},
	}

	overallScore := engine.calculateOverallSocialScore(components)

	// Should be weighted average: (0.24 + 0.24 + 0.15) / (0.3 + 0.4 + 0.3) = 0.63
	expectedScore := 0.63
	assert.InDelta(t, expectedScore, overallScore, 0.01, "Should calculate weighted average correctly")
}

func TestSocialInputsEngine_CalculateOverallSocialScore_NoComponents(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	overallScore := engine.calculateOverallSocialScore([]*SocialComponent{})
	assert.Equal(t, 0.0, overallScore, "Should return 0 for no components")
}

func TestSocialInputsEngine_MergeSourceData(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	// Create source results with different data
	sourceResults := []*SourceResult{
		{
			Success: true,
			Developer: &DeveloperMetrics{
				CommitFrequency: &RawDataPoint{
					Value: 25.0, SourceID: "github", Quality: "A", Timestamp: time.Now(),
				},
			},
			Community: &CommunityMetrics{
				StarGrowth: &RawDataPoint{
					Value: 40.0, SourceID: "github", Quality: "A", Timestamp: time.Now(),
				},
			},
		},
		{
			Success: true,
			Community: &CommunityMetrics{
				CommunitySize: &RawDataPoint{
					Value: 5000.0, SourceID: "coingecko", Quality: "B", Timestamp: time.Now(),
				},
			},
			News: &NewsMetrics{
				SentimentScore: &RawDataPoint{
					Value: 0.3, SourceID: "news", Quality: "C", Timestamp: time.Now(),
				},
			},
		},
		{
			Success: false, // This should be ignored
			Developer: &DeveloperMetrics{
				CommitFrequency: &RawDataPoint{Value: 999.0}, // Should not be used
			},
		},
	}

	merged := engine.mergeSourceData(sourceResults)
	require.NotNil(t, merged)

	// Should have data from successful sources
	require.NotNil(t, merged.Developer)
	assert.NotNil(t, merged.Developer.CommitFrequency)
	assert.Equal(t, 25.0, merged.Developer.CommitFrequency.Value, "Should use data from successful source")

	require.NotNil(t, merged.Community)
	assert.NotNil(t, merged.Community.StarGrowth)
	assert.NotNil(t, merged.Community.CommunitySize)
	assert.Equal(t, 40.0, merged.Community.StarGrowth.Value)
	assert.Equal(t, 5000.0, merged.Community.CommunitySize.Value)

	require.NotNil(t, merged.News)
	assert.NotNil(t, merged.News.SentimentScore)
	assert.Equal(t, 0.3, merged.News.SentimentScore.Value)

	// Should not include data from failed source
	assert.NotEqual(t, 999.0, merged.Developer.CommitFrequency.Value, "Should not use data from failed source")
}

func TestSocialInputsEngine_IsPreferred(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	now := time.Now()
	older := now.Add(-1 * time.Hour)

	testCases := []struct {
		name        string
		new         *RawDataPoint
		existing    *RawDataPoint
		expected    bool
		description string
	}{
		{
			name:        "higher_quality_wins",
			new:         &RawDataPoint{Quality: "A", Timestamp: older},
			existing:    &RawDataPoint{Quality: "B", Timestamp: now},
			expected:    true,
			description: "A-grade should be preferred over B-grade even if older",
		},
		{
			name:        "newer_wins_same_quality",
			new:         &RawDataPoint{Quality: "B", Timestamp: now},
			existing:    &RawDataPoint{Quality: "B", Timestamp: older},
			expected:    true,
			description: "Newer data should win when quality is equal",
		},
		{
			name:        "existing_wins",
			new:         &RawDataPoint{Quality: "C", Timestamp: now},
			existing:    &RawDataPoint{Quality: "A", Timestamp: older},
			expected:    false,
			description: "Existing higher quality should be kept",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.isPreferred(tc.new, tc.existing)
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}

func TestSocialInputsEngine_AssessDataQuality(t *testing.T) {
	engine := NewSocialInputsEngine(nil, nil, nil)

	// Create result with some components
	result := &SocialInputs{
		Components: []*SocialComponent{
			{Name: "comp1", LastUpdated: time.Now()},
			{Name: "comp2", LastUpdated: time.Now().Add(-2 * time.Hour)},
			{Name: "comp3", LastUpdated: time.Now().Add(-1 * time.Hour)},
		},
	}

	// Create source results
	sourceResults := []*SourceResult{
		{Success: true},
		{Success: true},
		{Success: false},
	}

	quality := engine.assessDataQuality(result, sourceResults)
	require.NotNil(t, quality)

	assert.Equal(t, 2, quality.SourcesAvailable, "Should count successful sources")
	assert.Equal(t, 3, quality.SourcesTotal, "Should count total sources")
	assert.Equal(t, 3, quality.MetricsPopulated, "Should count populated metrics")
	assert.Equal(t, 15, quality.MetricsTotal, "Should have fixed total of 15 possible metrics")

	// Source diversity should be 2/3
	assert.InDelta(t, 2.0/3.0, quality.SourceDiversity, 0.01, "Source diversity should be 2/3")

	// Should have freshness score
	assert.Greater(t, quality.DataFreshnessScore, 0.0, "Should calculate freshness score")
	assert.LessOrEqual(t, quality.DataFreshnessScore, 1.0, "Freshness score should be normalized")

	// Should assign overall grade
	assert.Contains(t, []string{"A", "B", "C", "D", "F"}, quality.OverallGrade, "Should assign valid grade")
}

func TestSocialInputsEngine_AddDataQualityWarnings(t *testing.T) {
	config := DefaultEngineConfig()
	config.RequireMinSources = 2
	config.RequireMinMetrics = 5

	engine := NewSocialInputsEngine(nil, nil, config)

	result := &SocialInputs{
		Warnings: []string{},
		DataQuality: &SocialDataQuality{
			OverallGrade:       "D", // Poor grade should trigger warning
			SourcesAvailable:   1,   // Below minimum
			MetricsPopulated:   3,   // Below minimum
			DataFreshnessScore: 0.3, // Below threshold
			SourceDiversity:    0.4, // Below threshold
		},
	}

	engine.addDataQualityWarnings(result)

	// Should have multiple warnings
	assert.Greater(t, len(result.Warnings), 0, "Should have data quality warnings")

	// Check for specific warning patterns
	warningsText := fmt.Sprintf("%v", result.Warnings)
	assert.Contains(t, warningsText, "Low data quality grade", "Should warn about poor grade")
	assert.Contains(t, warningsText, "Insufficient data sources", "Should warn about insufficient sources")
	assert.Contains(t, warningsText, "Insufficient metrics", "Should warn about insufficient metrics")
	assert.Contains(t, warningsText, "freshness", "Should warn about data freshness")
	assert.Contains(t, warningsText, "diversity", "Should warn about source diversity")
}

func TestSocialInputs_GetSocialInputsSummary(t *testing.T) {
	inputs := &SocialInputs{
		Asset:             "BTC-USD",
		OverallSocial:     0.75,
		DeveloperActivity: 0.8,
		CommunityGrowth:   0.7,
		BrandMentions:     0.6,
		SocialSentiment:   0.65,
		ProcessingTimeMs:  125,
		DataQuality:       &SocialDataQuality{OverallGrade: "B"},
	}

	summary := inputs.GetSocialInputsSummary()
	assert.Contains(t, summary, "BTC-USD", "Should include asset symbol")
	assert.Contains(t, summary, "0.750", "Should include overall social score")
	assert.Contains(t, summary, "0.800", "Should include developer activity")
	assert.Contains(t, summary, "0.700", "Should include community growth")
	assert.Contains(t, summary, "0.600", "Should include brand mentions")
	assert.Contains(t, summary, "0.650", "Should include social sentiment")
	assert.Contains(t, summary, "grade: B", "Should include quality grade")
	assert.Contains(t, summary, "125ms", "Should include processing time")
}

func TestSocialInputs_GetDetailedSocialReport(t *testing.T) {
	inputs := &SocialInputs{
		Asset:             "ETH-USD",
		OverallSocial:     0.72,
		DeveloperActivity: 0.85,
		CommunityGrowth:   0.68,
		BrandMentions:     0.55,
		SocialSentiment:   0.62,
		ProcessingTimeMs:  89,
		Components: []*SocialComponent{
			{
				Name:     "commit_frequency",
				Category: "developer",
				Value:    0.8,
				Weight:   0.3,
				Quality:  "A",
			},
			{
				Name:     "star_growth",
				Category: "community",
				Value:    0.7,
				Weight:   0.3,
				Quality:  "B",
			},
		},
		DataQuality: &SocialDataQuality{
			OverallGrade:       "B",
			SourcesAvailable:   2,
			SourcesTotal:       3,
			MetricsPopulated:   8,
			MetricsTotal:       15,
			DataFreshnessScore: 0.82,
			SourceDiversity:    0.67,
		},
		Warnings: []string{"Limited source diversity"},
	}

	report := inputs.GetDetailedSocialReport()

	// Should include all major sections
	assert.Contains(t, report, "ETH-USD", "Should include asset")
	assert.Contains(t, report, "Overall Social Score: 0.720", "Should include overall score")
	assert.Contains(t, report, "Quality Grade: B", "Should include quality grade")

	// Category scores
	assert.Contains(t, report, "Developer Activity: 0.850", "Should include category scores")
	assert.Contains(t, report, "Community Growth: 0.680")
	assert.Contains(t, report, "Brand Mentions: 0.550")
	assert.Contains(t, report, "Social Sentiment: 0.620")

	// Component details
	assert.Contains(t, report, "commit_frequency (developer): 0.800", "Should include component details")
	assert.Contains(t, report, "star_growth (community): 0.700")

	// Data quality section
	assert.Contains(t, report, "Sources: 2/3 available", "Should include source availability")
	assert.Contains(t, report, "Metrics: 8/15 populated", "Should include metrics count")
	assert.Contains(t, report, "Freshness Score: 0.820", "Should include freshness")
	assert.Contains(t, report, "Source Diversity: 0.670", "Should include diversity")

	// Warnings
	assert.Contains(t, report, "Limited source diversity", "Should include warnings")
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()
	require.NotNil(t, config)

	// Source aggregation
	assert.Equal(t, 3, config.MaxConcurrentSources)
	assert.Equal(t, 10*time.Second, config.SourceTimeout)
	assert.Equal(t, 1, config.RequireMinSources)

	// Data quality
	assert.Equal(t, 2, config.RequireMinMetrics)
	assert.Equal(t, "C", config.DefaultQualityGrade)

	// Performance
	assert.Equal(t, 30*time.Second, config.MaxProcessingTime)
	assert.True(t, config.EnableParallelFetch)

	// Output options
	assert.True(t, config.IncludeProvenance)
	assert.False(t, config.IncludeDebugInfo)
}

func TestSocialComponent_Structure(t *testing.T) {
	component := &SocialComponent{
		Name:         "test_metric",
		Category:     "developer",
		Value:        0.75,
		Weight:       0.3,
		Contribution: 0.225, // 0.75 × 0.3
		Quality:      "A",
		LastUpdated:  time.Now(),
		TTL:          6 * time.Hour,
	}

	// Verify all fields are accessible
	assert.Equal(t, "test_metric", component.Name)
	assert.Equal(t, "developer", component.Category)
	assert.Equal(t, 0.75, component.Value)
	assert.Equal(t, 0.3, component.Weight)
	assert.Equal(t, 0.225, component.Contribution)
	assert.Equal(t, "A", component.Quality)
	assert.Equal(t, 6*time.Hour, component.TTL)

	// Verify contribution calculation
	expectedContribution := component.Value * component.Weight
	assert.Equal(t, expectedContribution, component.Contribution, "Contribution should equal value × weight")
}

func TestSourceProvenance_Structure(t *testing.T) {
	provenance := &SourceProvenance{
		SourceID:         "github",
		SourceName:       "GitHub Developer Activity",
		ReliabilityGrade: "A",
		MetricsProvided:  []string{"developer_activity", "community_growth"},
		LastFetch:        time.Now(),
		FetchDurationMs:  45,
		CacheHit:         false,
	}

	// Verify all fields are accessible and have correct types
	assert.Equal(t, "github", provenance.SourceID)
	assert.Equal(t, "GitHub Developer Activity", provenance.SourceName)
	assert.Equal(t, "A", provenance.ReliabilityGrade)
	assert.Len(t, provenance.MetricsProvided, 2)
	assert.Contains(t, provenance.MetricsProvided, "developer_activity")
	assert.Contains(t, provenance.MetricsProvided, "community_growth")
	assert.Equal(t, int64(45), provenance.FetchDurationMs)
	assert.False(t, provenance.CacheHit)
}
