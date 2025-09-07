package social

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// Normalizer handles transformation of raw social data to 0-1 normalized values
type Normalizer struct {
	config *NormalizationConfig
	cache  Cache
}

// NewNormalizer creates a new social data normalizer
func NewNormalizer(config *NormalizationConfig, cache Cache) *Normalizer {
	if config == nil {
		config = DefaultNormalizationConfig()
	}
	return &Normalizer{
		config: config,
		cache:  cache,
	}
}

// NormalizationConfig contains parameters for social data normalization
type NormalizationConfig struct {
	// Rolling window parameters
	WindowSizeDays        int `yaml:"window_size_days"`          // 30-day rolling window
	MinDataPoints         int `yaml:"min_data_points"`           // 10 minimum data points
	MaxDataPointsForStats int `yaml:"max_data_points_for_stats"` // 100 max points for statistics

	// Winsorization parameters (remove extreme outliers)
	WinsorizeEnabled  bool    `yaml:"winsorize_enabled"`   // true - enable winsorization
	WinsorizeLowerPct float64 `yaml:"winsorize_lower_pct"` // 5.0% lower bound
	WinsorizeUpperPct float64 `yaml:"winsorize_upper_pct"` // 95.0% upper bound

	// Z-score parameters
	ZScoreClipEnabled   bool    `yaml:"zscore_clip_enabled"`   // true - clip extreme z-scores
	ZScoreClipThreshold float64 `yaml:"zscore_clip_threshold"` // 3.0 standard deviations

	// Min-max normalization parameters
	MinMaxFloor   float64 `yaml:"min_max_floor"`   // 0.0 - minimum normalized value
	MinMaxCeiling float64 `yaml:"min_max_ceiling"` // 1.0 - maximum normalized value

	// Cache parameters for rolling statistics
	StatsCacheTTL        time.Duration `yaml:"stats_cache_ttl"`          // 6h cache for statistics
	UseStaleStatsOnError bool          `yaml:"use_stale_stats_on_error"` // true - use stale stats if fresh fetch fails
}

// DefaultNormalizationConfig returns production normalization parameters
func DefaultNormalizationConfig() *NormalizationConfig {
	return &NormalizationConfig{
		// Rolling window
		WindowSizeDays:        30,
		MinDataPoints:         10,
		MaxDataPointsForStats: 100,

		// Winsorization
		WinsorizeEnabled:  true,
		WinsorizeLowerPct: 5.0,
		WinsorizeUpperPct: 95.0,

		// Z-score clipping
		ZScoreClipEnabled:   true,
		ZScoreClipThreshold: 3.0,

		// Min-max bounds
		MinMaxFloor:   0.0,
		MinMaxCeiling: 1.0,

		// Cache settings
		StatsCacheTTL:        6 * time.Hour,
		UseStaleStatsOnError: true,
	}
}

// NormalizedDataPoint represents a normalized social metric
type NormalizedDataPoint struct {
	NormalizedValue float64       `json:"normalized_value"` // Final 0-1 normalized value
	RawValue        float64       `json:"raw_value"`        // Original raw value
	ZScore          float64       `json:"z_score"`          // Z-score transformation
	Percentile      float64       `json:"percentile"`       // Percentile rank (0-100)
	Timestamp       time.Time     `json:"timestamp"`        // Data timestamp
	SourceID        string        `json:"source_id"`        // Source identifier
	TTL             time.Duration `json:"ttl"`              // Data validity period
	Quality         string        `json:"quality"`          // Data quality grade
	TransformMethod string        `json:"transform_method"` // Normalization method used
	StatisticsUsed  *RollingStats `json:"statistics_used"`  // Statistics used for normalization
}

// RollingStats contains statistics for a rolling window of data
type RollingStats struct {
	Mean              float64   `json:"mean"`               // Rolling window mean
	StandardDeviation float64   `json:"standard_deviation"` // Rolling window std dev
	Minimum           float64   `json:"minimum"`            // Rolling window minimum
	Maximum           float64   `json:"maximum"`            // Rolling window maximum
	Count             int       `json:"count"`              // Number of data points
	WindowStart       time.Time `json:"window_start"`       // Start of rolling window
	WindowEnd         time.Time `json:"window_end"`         // End of rolling window
	WinsorizedCount   int       `json:"winsorized_count"`   // Number of winsorized outliers
}

// MetricStatisticsCollection holds rolling statistics for different metric types
type MetricStatisticsCollection struct {
	DeveloperStats map[string]*RollingStats `json:"developer_stats"` // Statistics for developer metrics
	CommunityStats map[string]*RollingStats `json:"community_stats"` // Statistics for community metrics
	NewsStats      map[string]*RollingStats `json:"news_stats"`      // Statistics for news metrics
	LastUpdated    time.Time                `json:"last_updated"`    // When statistics were last computed
	Asset          string                   `json:"asset"`           // Asset these statistics apply to
}

// NormalizeMetrics transforms raw social metrics to normalized 0-1 values
func (n *Normalizer) NormalizeMetrics(ctx context.Context, asset string, rawMetrics *RawSocialMetrics) (*NormalizedSocialMetrics, error) {
	// Get rolling statistics for this asset
	stats, err := n.getRollingStatistics(ctx, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to get rolling statistics for %s: %w", asset, err)
	}

	normalized := &NormalizedSocialMetrics{
		Asset:     asset,
		Timestamp: time.Now(),
	}

	// Normalize developer metrics
	if rawMetrics.Developer != nil {
		normalized.Developer = &NormalizedDeveloperMetrics{}

		if rawMetrics.Developer.CommitFrequency != nil {
			normalized.Developer.CommitFrequency = n.normalizeDataPoint(
				rawMetrics.Developer.CommitFrequency,
				stats.DeveloperStats["commit_frequency"],
				"commit_frequency",
			)
		}

		if rawMetrics.Developer.ActiveContributors != nil {
			normalized.Developer.ActiveContributors = n.normalizeDataPoint(
				rawMetrics.Developer.ActiveContributors,
				stats.DeveloperStats["active_contributors"],
				"active_contributors",
			)
		}

		if rawMetrics.Developer.CodeQuality != nil {
			normalized.Developer.CodeQuality = n.normalizeDataPoint(
				rawMetrics.Developer.CodeQuality,
				stats.DeveloperStats["code_quality"],
				"code_quality",
			)
		}

		if rawMetrics.Developer.ReleaseFrequency != nil {
			normalized.Developer.ReleaseFrequency = n.normalizeDataPoint(
				rawMetrics.Developer.ReleaseFrequency,
				stats.DeveloperStats["release_frequency"],
				"release_frequency",
			)
		}

		if rawMetrics.Developer.IssueResolution != nil {
			normalized.Developer.IssueResolution = n.normalizeDataPoint(
				rawMetrics.Developer.IssueResolution,
				stats.DeveloperStats["issue_resolution"],
				"issue_resolution",
			)
		}
	}

	// Normalize community metrics
	if rawMetrics.Community != nil {
		normalized.Community = &NormalizedCommunityMetrics{}

		if rawMetrics.Community.StarGrowth != nil {
			normalized.Community.StarGrowth = n.normalizeDataPoint(
				rawMetrics.Community.StarGrowth,
				stats.CommunityStats["star_growth"],
				"star_growth",
			)
		}

		if rawMetrics.Community.ForkRatio != nil {
			normalized.Community.ForkRatio = n.normalizeDataPoint(
				rawMetrics.Community.ForkRatio,
				stats.CommunityStats["fork_ratio"],
				"fork_ratio",
			)
		}

		if rawMetrics.Community.CommunitySize != nil {
			normalized.Community.CommunitySize = n.normalizeDataPoint(
				rawMetrics.Community.CommunitySize,
				stats.CommunityStats["community_size"],
				"community_size",
			)
		}

		if rawMetrics.Community.EngagementRate != nil {
			normalized.Community.EngagementRate = n.normalizeDataPoint(
				rawMetrics.Community.EngagementRate,
				stats.CommunityStats["engagement_rate"],
				"engagement_rate",
			)
		}

		if rawMetrics.Community.SocialMentions != nil {
			normalized.Community.SocialMentions = n.normalizeDataPoint(
				rawMetrics.Community.SocialMentions,
				stats.CommunityStats["social_mentions"],
				"social_mentions",
			)
		}
	}

	// Normalize news metrics
	if rawMetrics.News != nil {
		normalized.News = &NormalizedNewsMetrics{}

		if rawMetrics.News.MentionFrequency != nil {
			normalized.News.MentionFrequency = n.normalizeDataPoint(
				rawMetrics.News.MentionFrequency,
				stats.NewsStats["mention_frequency"],
				"mention_frequency",
			)
		}

		if rawMetrics.News.SentimentScore != nil {
			// Sentiment is already -1 to +1, transform to 0-1
			normalized.News.SentimentScore = n.normalizeSentimentScore(rawMetrics.News.SentimentScore)
		}

		if rawMetrics.News.AuthorityScore != nil {
			normalized.News.AuthorityScore = n.normalizeDataPoint(
				rawMetrics.News.AuthorityScore,
				stats.NewsStats["authority_score"],
				"authority_score",
			)
		}

		if rawMetrics.News.TrendingScore != nil {
			normalized.News.TrendingScore = n.normalizeDataPoint(
				rawMetrics.News.TrendingScore,
				stats.NewsStats["trending_score"],
				"trending_score",
			)
		}

		if rawMetrics.News.CategoryRelevance != nil {
			normalized.News.CategoryRelevance = n.normalizeDataPoint(
				rawMetrics.News.CategoryRelevance,
				stats.NewsStats["category_relevance"],
				"category_relevance",
			)
		}
	}

	return normalized, nil
}

// normalizeDataPoint transforms a single raw data point to normalized 0-1 value
func (n *Normalizer) normalizeDataPoint(raw *RawDataPoint, stats *RollingStats, metricName string) *NormalizedDataPoint {
	if raw == nil {
		return nil
	}

	normalized := &NormalizedDataPoint{
		RawValue:        raw.Value,
		Timestamp:       raw.Timestamp,
		SourceID:        raw.SourceID,
		TTL:             raw.TTL,
		Quality:         raw.Quality,
		TransformMethod: "zscore_minmax",
		StatisticsUsed:  stats,
	}

	// If no statistics available, return raw value clamped to 0-1
	if stats == nil || stats.Count < n.config.MinDataPoints {
		normalized.NormalizedValue = n.clampToRange(raw.Value, 0.0, 1.0)
		normalized.ZScore = 0.0
		normalized.Percentile = 50.0 // Assume median if no stats
		normalized.TransformMethod = "raw_clamp"
		return normalized
	}

	// Calculate z-score
	if stats.StandardDeviation > 0 {
		normalized.ZScore = (raw.Value - stats.Mean) / stats.StandardDeviation
	} else {
		normalized.ZScore = 0.0
	}

	// Clip extreme z-scores if enabled
	if n.config.ZScoreClipEnabled {
		if normalized.ZScore > n.config.ZScoreClipThreshold {
			normalized.ZScore = n.config.ZScoreClipThreshold
		} else if normalized.ZScore < -n.config.ZScoreClipThreshold {
			normalized.ZScore = -n.config.ZScoreClipThreshold
		}
	}

	// Calculate percentile rank
	normalized.Percentile = n.calculatePercentileRank(raw.Value, stats)

	// Transform z-score to 0-1 using sigmoid-like function
	// This handles both positive and negative z-scores gracefully
	if n.config.ZScoreClipEnabled && n.config.ZScoreClipThreshold > 0 {
		// Map clipped z-score [-threshold, +threshold] to [0, 1]
		normalized.NormalizedValue = (normalized.ZScore + n.config.ZScoreClipThreshold) / (2 * n.config.ZScoreClipThreshold)
	} else {
		// Use sigmoid function for unbounded z-scores: 1 / (1 + exp(-z))
		normalized.NormalizedValue = 1.0 / (1.0 + math.Exp(-normalized.ZScore))
	}

	// Ensure final value is within configured bounds
	normalized.NormalizedValue = n.clampToRange(normalized.NormalizedValue, n.config.MinMaxFloor, n.config.MinMaxCeiling)

	return normalized
}

// normalizeSentimentScore transforms sentiment from [-1, +1] to [0, 1]
func (n *Normalizer) normalizeSentimentScore(raw *RawDataPoint) *NormalizedDataPoint {
	if raw == nil {
		return nil
	}

	// Sentiment is already normalized to [-1, +1], transform to [0, 1]
	normalizedValue := (raw.Value + 1.0) / 2.0
	normalizedValue = n.clampToRange(normalizedValue, 0.0, 1.0)

	return &NormalizedDataPoint{
		NormalizedValue: normalizedValue,
		RawValue:        raw.Value,
		ZScore:          raw.Value,               // Z-score same as raw for sentiment
		Percentile:      normalizedValue * 100.0, // Convert to percentile
		Timestamp:       raw.Timestamp,
		SourceID:        raw.SourceID,
		TTL:             raw.TTL,
		Quality:         raw.Quality,
		TransformMethod: "sentiment_linear",
		StatisticsUsed:  nil, // No rolling stats for sentiment
	}
}

// getRollingStatistics retrieves or computes rolling statistics for an asset
func (n *Normalizer) getRollingStatistics(ctx context.Context, asset string) (*MetricStatisticsCollection, error) {
	cacheKey := fmt.Sprintf("social_stats:%s", asset)

	// Try to get cached statistics first
	if n.cache != nil {
		if cached, err := n.cache.Get(ctx, cacheKey); err == nil {
			var stats MetricStatisticsCollection
			if err := n.unmarshalStats(cached, &stats); err == nil {
				// Check if statistics are fresh enough
				if time.Since(stats.LastUpdated) < n.config.StatsCacheTTL {
					return &stats, nil
				}
			}
		}
	}

	// Compute fresh statistics
	stats, err := n.computeRollingStatistics(ctx, asset)
	if err != nil {
		// If configured to use stale stats on error, try to return cached stats
		if n.config.UseStaleStatsOnError && n.cache != nil {
			if cached, cacheErr := n.cache.Get(ctx, cacheKey); cacheErr == nil {
				var staleStats MetricStatisticsCollection
				if unmarshalErr := n.unmarshalStats(cached, &staleStats); unmarshalErr == nil {
					return &staleStats, nil // Return stale stats as fallback
				}
			}
		}
		return nil, err
	}

	// Cache fresh statistics
	if n.cache != nil {
		if serialized, err := n.marshalStats(stats); err == nil {
			n.cache.Set(ctx, cacheKey, serialized, n.config.StatsCacheTTL)
		}
	}

	return stats, nil
}

// computeRollingStatistics computes statistics over a rolling window
func (n *Normalizer) computeRollingStatistics(ctx context.Context, asset string) (*MetricStatisticsCollection, error) {
	// In a real implementation, this would fetch historical data from a data store
	// For now, return minimal default statistics to make the interface work

	windowStart := time.Now().AddDate(0, 0, -n.config.WindowSizeDays)
	windowEnd := time.Now()

	// Create default statistics for common metrics
	defaultStats := &RollingStats{
		Mean:              50.0,  // Default mean
		StandardDeviation: 15.0,  // Default std dev
		Minimum:           0.0,   // Default min
		Maximum:           100.0, // Default max
		Count:             n.config.MinDataPoints,
		WindowStart:       windowStart,
		WindowEnd:         windowEnd,
		WinsorizedCount:   0,
	}

	return &MetricStatisticsCollection{
		DeveloperStats: map[string]*RollingStats{
			"commit_frequency":    defaultStats,
			"active_contributors": defaultStats,
			"code_quality":        defaultStats,
			"release_frequency":   defaultStats,
			"issue_resolution":    defaultStats,
		},
		CommunityStats: map[string]*RollingStats{
			"star_growth":     defaultStats,
			"fork_ratio":      defaultStats,
			"community_size":  defaultStats,
			"engagement_rate": defaultStats,
			"social_mentions": defaultStats,
		},
		NewsStats: map[string]*RollingStats{
			"mention_frequency":  defaultStats,
			"authority_score":    defaultStats,
			"trending_score":     defaultStats,
			"category_relevance": defaultStats,
		},
		LastUpdated: time.Now(),
		Asset:       asset,
	}, nil
}

// calculatePercentileRank computes the percentile rank of a value within rolling statistics
func (n *Normalizer) calculatePercentileRank(value float64, stats *RollingStats) float64 {
	if stats == nil {
		return 50.0 // Default to median
	}

	// Simple linear interpolation between min and max for percentile estimation
	if stats.Maximum > stats.Minimum {
		rank := (value - stats.Minimum) / (stats.Maximum - stats.Minimum) * 100.0
		return n.clampToRange(rank, 0.0, 100.0)
	}

	return 50.0 // If no range, assume median
}

// winsorizeValues removes extreme outliers from a dataset
func (n *Normalizer) winsorizeValues(values []float64) []float64 {
	if !n.config.WinsorizeEnabled || len(values) < n.config.MinDataPoints {
		return values
	}

	// Sort values to find percentile bounds
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate percentile indices
	lowerIdx := int(float64(len(sorted)) * n.config.WinsorizeLowerPct / 100.0)
	upperIdx := int(float64(len(sorted)) * n.config.WinsorizeUpperPct / 100.0)

	if lowerIdx >= len(sorted) {
		lowerIdx = len(sorted) - 1
	}
	if upperIdx >= len(sorted) {
		upperIdx = len(sorted) - 1
	}

	lowerBound := sorted[lowerIdx]
	upperBound := sorted[upperIdx]

	// Filter values within bounds
	winsorized := make([]float64, 0, len(values))
	for _, value := range values {
		if value >= lowerBound && value <= upperBound {
			winsorized = append(winsorized, value)
		}
	}

	return winsorized
}

// clampToRange ensures a value is within specified bounds
func (n *Normalizer) clampToRange(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Helper functions for statistics serialization (simplified for interface)
func (n *Normalizer) marshalStats(stats *MetricStatisticsCollection) ([]byte, error) {
	// In a real implementation, this would use proper JSON/protobuf serialization
	return []byte(fmt.Sprintf("stats:%s:%d", stats.Asset, stats.LastUpdated.Unix())), nil
}

func (n *Normalizer) unmarshalStats(data []byte, stats *MetricStatisticsCollection) error {
	// In a real implementation, this would deserialize from JSON/protobuf
	// For now, just return an error to force fresh computation
	return fmt.Errorf("serialized stats not supported in this implementation")
}

// Raw and normalized metric container types

// RawSocialMetrics contains unnormalized social data from multiple sources
type RawSocialMetrics struct {
	Asset     string            `json:"asset"`
	Timestamp time.Time         `json:"timestamp"`
	Developer *DeveloperMetrics `json:"developer,omitempty"`
	Community *CommunityMetrics `json:"community,omitempty"`
	News      *NewsMetrics      `json:"news,omitempty"`
}

// NormalizedSocialMetrics contains 0-1 normalized social data
type NormalizedSocialMetrics struct {
	Asset     string                      `json:"asset"`
	Timestamp time.Time                   `json:"timestamp"`
	Developer *NormalizedDeveloperMetrics `json:"developer,omitempty"`
	Community *NormalizedCommunityMetrics `json:"community,omitempty"`
	News      *NormalizedNewsMetrics      `json:"news,omitempty"`
}

// NormalizedDeveloperMetrics contains normalized developer activity metrics
type NormalizedDeveloperMetrics struct {
	CommitFrequency    *NormalizedDataPoint `json:"commit_frequency,omitempty"`
	ActiveContributors *NormalizedDataPoint `json:"active_contributors,omitempty"`
	CodeQuality        *NormalizedDataPoint `json:"code_quality,omitempty"`
	ReleaseFrequency   *NormalizedDataPoint `json:"release_frequency,omitempty"`
	IssueResolution    *NormalizedDataPoint `json:"issue_resolution,omitempty"`
}

// NormalizedCommunityMetrics contains normalized community engagement metrics
type NormalizedCommunityMetrics struct {
	StarGrowth     *NormalizedDataPoint `json:"star_growth,omitempty"`
	ForkRatio      *NormalizedDataPoint `json:"fork_ratio,omitempty"`
	CommunitySize  *NormalizedDataPoint `json:"community_size,omitempty"`
	EngagementRate *NormalizedDataPoint `json:"engagement_rate,omitempty"`
	SocialMentions *NormalizedDataPoint `json:"social_mentions,omitempty"`
}

// NormalizedNewsMetrics contains normalized news and media metrics
type NormalizedNewsMetrics struct {
	MentionFrequency  *NormalizedDataPoint `json:"mention_frequency,omitempty"`
	SentimentScore    *NormalizedDataPoint `json:"sentiment_score,omitempty"`
	AuthorityScore    *NormalizedDataPoint `json:"authority_score,omitempty"`
	TrendingScore     *NormalizedDataPoint `json:"trending_score,omitempty"`
	CategoryRelevance *NormalizedDataPoint `json:"category_relevance,omitempty"`
}
