package social

import (
	"context"
	"fmt"
	"math"
	"time"
)

// SocialInputsEngine orchestrates fetching and normalizing social data from multiple sources
type SocialInputsEngine struct {
	sources    []SocialDataSource
	normalizer *Normalizer
	config     *EngineConfig
}

// NewSocialInputsEngine creates a new social inputs engine with configured sources
func NewSocialInputsEngine(sources []SocialDataSource, normalizer *Normalizer, config *EngineConfig) *SocialInputsEngine {
	if config == nil {
		config = DefaultEngineConfig()
	}
	return &SocialInputsEngine{
		sources:    sources,
		normalizer: normalizer,
		config:     config,
	}
}

// EngineConfig contains configuration for the social inputs engine
type EngineConfig struct {
	// Source aggregation
	MaxConcurrentSources int           `yaml:"max_concurrent_sources"` // 3 - max sources to query in parallel
	SourceTimeout        time.Duration `yaml:"source_timeout"`         // 10s - timeout per source
	RequireMinSources    int           `yaml:"require_min_sources"`    // 1 - minimum sources required for valid result

	// Data quality
	RequireMinMetrics   int    `yaml:"require_min_metrics"`   // 2 - minimum metrics required
	DefaultQualityGrade string `yaml:"default_quality_grade"` // "C" - quality grade when no source data

	// Performance limits
	MaxProcessingTime   time.Duration `yaml:"max_processing_time"`   // 30s - max total processing time
	EnableParallelFetch bool          `yaml:"enable_parallel_fetch"` // true - fetch sources in parallel

	// Output options
	IncludeProvenance bool `yaml:"include_provenance"` // true - include source provenance in output
	IncludeDebugInfo  bool `yaml:"include_debug_info"` // false - include debug/timing information
}

// DefaultEngineConfig returns production configuration for social inputs engine
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		MaxConcurrentSources: 3,
		SourceTimeout:        10 * time.Second,
		RequireMinSources:    1,
		RequireMinMetrics:    2,
		DefaultQualityGrade:  "C",
		MaxProcessingTime:    30 * time.Second,
		EnableParallelFetch:  true,
		IncludeProvenance:    true,
		IncludeDebugInfo:     false,
	}
}

// SocialInputs contains the complete normalized social data for an asset
type SocialInputs struct {
	Asset             string              `json:"asset"`                // Asset symbol
	Timestamp         time.Time           `json:"timestamp"`            // When data was processed
	DeveloperActivity float64             `json:"developer_activity"`   // Normalized 0-1 developer activity
	CommunityGrowth   float64             `json:"community_growth"`     // Normalized 0-1 community growth
	BrandMentions     float64             `json:"brand_mentions"`       // Normalized 0-1 brand mentions
	SocialSentiment   float64             `json:"social_sentiment"`     // Normalized 0-1 sentiment (0.5 = neutral)
	OverallSocial     float64             `json:"overall_social"`       // Weighted average of all components
	Components        []*SocialComponent  `json:"components"`           // Individual component breakdown
	Provenance        []*SourceProvenance `json:"provenance,omitempty"` // Source attribution
	DataQuality       *SocialDataQuality  `json:"data_quality"`         // Quality assessment
	ProcessingTimeMs  int64               `json:"processing_time_ms"`   // Total processing time
	Warnings          []string            `json:"warnings,omitempty"`   // Data quality warnings
}

// SocialComponent represents a single normalized social factor
type SocialComponent struct {
	Name         string        `json:"name"`         // Component name (e.g., "commit_frequency")
	Category     string        `json:"category"`     // Category (developer/community/news)
	Value        float64       `json:"value"`        // Normalized 0-1 value
	Weight       float64       `json:"weight"`       // Weight in overall calculation
	Contribution float64       `json:"contribution"` // Weighted contribution to overall score
	Quality      string        `json:"quality"`      // Data quality grade (A/B/C/D)
	LastUpdated  time.Time     `json:"last_updated"` // When this component was last updated
	TTL          time.Duration `json:"ttl"`          // How long this data remains valid
}

// SourceProvenance tracks where each data point came from
type SourceProvenance struct {
	SourceID         string    `json:"source_id"`         // Source identifier
	SourceName       string    `json:"source_name"`       // Human-readable source name
	ReliabilityGrade string    `json:"reliability_grade"` // Source reliability grade
	MetricsProvided  []string  `json:"metrics_provided"`  // List of metrics from this source
	LastFetch        time.Time `json:"last_fetch"`        // When data was fetched
	FetchDurationMs  int64     `json:"fetch_duration_ms"` // How long fetch took
	CacheHit         bool      `json:"cache_hit"`         // Whether data came from cache
}

// SocialDataQuality summarizes the quality of social data
type SocialDataQuality struct {
	OverallGrade       string  `json:"overall_grade"`        // Overall quality grade A-F
	SourcesAvailable   int     `json:"sources_available"`    // Number of sources with data
	SourcesTotal       int     `json:"sources_total"`        // Total number of configured sources
	MetricsPopulated   int     `json:"metrics_populated"`    // Number of metrics with data
	MetricsTotal       int     `json:"metrics_total"`        // Total number of possible metrics
	DataFreshnessScore float64 `json:"data_freshness_score"` // 0-1 score for data freshness
	SourceDiversity    float64 `json:"source_diversity"`     // 0-1 score for source diversity
}

// FetchSocialInputs retrieves and normalizes social data for an asset
func (sie *SocialInputsEngine) FetchSocialInputs(ctx context.Context, asset string) (*SocialInputs, error) {
	startTime := time.Now()

	// Create context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, sie.config.MaxProcessingTime)
	defer cancel()

	result := &SocialInputs{
		Asset:      asset,
		Timestamp:  time.Now(),
		Components: []*SocialComponent{},
		Provenance: []*SourceProvenance{},
		Warnings:   []string{},
	}

	// Fetch raw data from all sources
	rawData, sourceResults, err := sie.fetchFromAllSources(fetchCtx, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw social data: %w", err)
	}

	// Check minimum source requirement
	if len(sourceResults) < sie.config.RequireMinSources {
		return nil, fmt.Errorf("insufficient data sources: got %d, require minimum %d",
			len(sourceResults), sie.config.RequireMinSources)
	}

	// Normalize the raw data
	normalized, err := sie.normalizer.NormalizeMetrics(fetchCtx, asset, rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize social metrics: %w", err)
	}

	// Build components and calculate overall social score
	sie.buildSocialComponents(result, normalized)

	// Calculate overall social score (weighted average)
	result.OverallSocial = sie.calculateOverallSocialScore(result.Components)

	// Build provenance information
	if sie.config.IncludeProvenance {
		result.Provenance = sie.buildProvenance(sourceResults)
	}

	// Assess data quality
	result.DataQuality = sie.assessDataQuality(result, sourceResults)

	// Add warnings for data quality issues
	sie.addDataQualityWarnings(result)

	// Record processing time
	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()

	// Performance warning
	if result.ProcessingTimeMs > sie.config.MaxProcessingTime.Milliseconds() {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Processing took %dms (>%dms limit)",
				result.ProcessingTimeMs, sie.config.MaxProcessingTime.Milliseconds()))
	}

	return result, nil
}

// fetchFromAllSources retrieves raw data from all configured sources
func (sie *SocialInputsEngine) fetchFromAllSources(ctx context.Context, asset string) (*RawSocialMetrics, []*SourceResult, error) {
	sourceResults := make([]*SourceResult, 0, len(sie.sources))

	// If parallel fetch is enabled, use concurrent fetching
	if sie.config.EnableParallelFetch {
		return sie.fetchParallel(ctx, asset, &sourceResults)
	}

	// Sequential fetching
	return sie.fetchSequential(ctx, asset, &sourceResults)
}

// fetchParallel fetches data from sources concurrently
func (sie *SocialInputsEngine) fetchParallel(ctx context.Context, asset string, sourceResults *[]*SourceResult) (*RawSocialMetrics, []*SourceResult, error) {
	type fetchResult struct {
		result *SourceResult
		err    error
	}

	resultChan := make(chan fetchResult, len(sie.sources))

	// Start concurrent fetches
	for _, source := range sie.sources {
		go func(s SocialDataSource) {
			sourceResult := sie.fetchFromSingleSource(ctx, asset, s)
			resultChan <- fetchResult{result: sourceResult, err: nil}
		}(source)
	}

	// Collect results
	for i := 0; i < len(sie.sources); i++ {
		select {
		case res := <-resultChan:
			if res.result != nil {
				*sourceResults = append(*sourceResults, res.result)
			}
		case <-ctx.Done():
			return nil, *sourceResults, ctx.Err()
		}
	}

	// Merge all source data
	merged := sie.mergeSourceData(*sourceResults)
	return merged, *sourceResults, nil
}

// fetchSequential fetches data from sources one by one
func (sie *SocialInputsEngine) fetchSequential(ctx context.Context, asset string, sourceResults *[]*SourceResult) (*RawSocialMetrics, []*SourceResult, error) {
	for _, source := range sie.sources {
		select {
		case <-ctx.Done():
			return nil, *sourceResults, ctx.Err()
		default:
		}

		result := sie.fetchFromSingleSource(ctx, asset, source)
		if result != nil {
			*sourceResults = append(*sourceResults, result)
		}
	}

	merged := sie.mergeSourceData(*sourceResults)
	return merged, *sourceResults, nil
}

// SourceResult contains the result of fetching from a single source
type SourceResult struct {
	SourceInfo      *SourceInfo       `json:"source_info"`
	Developer       *DeveloperMetrics `json:"developer,omitempty"`
	Community       *CommunityMetrics `json:"community,omitempty"`
	News            *NewsMetrics      `json:"news,omitempty"`
	FetchDurationMs int64             `json:"fetch_duration_ms"`
	Success         bool              `json:"success"`
	Error           string            `json:"error,omitempty"`
	CacheHit        bool              `json:"cache_hit"`
}

// fetchFromSingleSource fetches data from one source with timeout and error handling
func (sie *SocialInputsEngine) fetchFromSingleSource(ctx context.Context, asset string, source SocialDataSource) *SourceResult {
	startTime := time.Now()

	// Create source-specific timeout context
	sourceCtx, cancel := context.WithTimeout(ctx, sie.config.SourceTimeout)
	defer cancel()

	result := &SourceResult{
		SourceInfo: source.GetSourceInfo(),
	}

	// Fetch developer metrics
	if dev, err := source.GetDeveloperActivity(sourceCtx, asset); err == nil {
		result.Developer = dev
	}

	// Fetch community metrics
	if comm, err := source.GetCommunityMetrics(sourceCtx, asset); err == nil {
		result.Community = comm
	}

	// Fetch news metrics
	if news, err := source.GetNewsMetrics(sourceCtx, asset); err == nil {
		result.News = news
	}

	result.FetchDurationMs = time.Since(startTime).Milliseconds()
	result.Success = (result.Developer != nil || result.Community != nil || result.News != nil)

	return result
}

// mergeSourceData combines data from multiple sources into a single raw metrics object
func (sie *SocialInputsEngine) mergeSourceData(sourceResults []*SourceResult) *RawSocialMetrics {
	merged := &RawSocialMetrics{
		Timestamp: time.Now(),
	}

	for _, sourceResult := range sourceResults {
		if !sourceResult.Success {
			continue
		}

		// Merge developer metrics (prefer higher quality sources)
		if sourceResult.Developer != nil {
			if merged.Developer == nil {
				merged.Developer = sourceResult.Developer
			} else {
				sie.mergeDeveloperMetrics(merged.Developer, sourceResult.Developer)
			}
		}

		// Merge community metrics
		if sourceResult.Community != nil {
			if merged.Community == nil {
				merged.Community = sourceResult.Community
			} else {
				sie.mergeCommunityMetrics(merged.Community, sourceResult.Community)
			}
		}

		// Merge news metrics
		if sourceResult.News != nil {
			if merged.News == nil {
				merged.News = sourceResult.News
			} else {
				sie.mergeNewsMetrics(merged.News, sourceResult.News)
			}
		}
	}

	return merged
}

// Helper functions for merging metrics (prefer newer, higher quality data)

func (sie *SocialInputsEngine) mergeDeveloperMetrics(existing *DeveloperMetrics, new *DeveloperMetrics) {
	if new.CommitFrequency != nil && (existing.CommitFrequency == nil || sie.isPreferred(new.CommitFrequency, existing.CommitFrequency)) {
		existing.CommitFrequency = new.CommitFrequency
	}
	if new.ActiveContributors != nil && (existing.ActiveContributors == nil || sie.isPreferred(new.ActiveContributors, existing.ActiveContributors)) {
		existing.ActiveContributors = new.ActiveContributors
	}
	if new.CodeQuality != nil && (existing.CodeQuality == nil || sie.isPreferred(new.CodeQuality, existing.CodeQuality)) {
		existing.CodeQuality = new.CodeQuality
	}
	if new.ReleaseFrequency != nil && (existing.ReleaseFrequency == nil || sie.isPreferred(new.ReleaseFrequency, existing.ReleaseFrequency)) {
		existing.ReleaseFrequency = new.ReleaseFrequency
	}
	if new.IssueResolution != nil && (existing.IssueResolution == nil || sie.isPreferred(new.IssueResolution, existing.IssueResolution)) {
		existing.IssueResolution = new.IssueResolution
	}
}

func (sie *SocialInputsEngine) mergeCommunityMetrics(existing *CommunityMetrics, new *CommunityMetrics) {
	if new.StarGrowth != nil && (existing.StarGrowth == nil || sie.isPreferred(new.StarGrowth, existing.StarGrowth)) {
		existing.StarGrowth = new.StarGrowth
	}
	if new.ForkRatio != nil && (existing.ForkRatio == nil || sie.isPreferred(new.ForkRatio, existing.ForkRatio)) {
		existing.ForkRatio = new.ForkRatio
	}
	if new.CommunitySize != nil && (existing.CommunitySize == nil || sie.isPreferred(new.CommunitySize, existing.CommunitySize)) {
		existing.CommunitySize = new.CommunitySize
	}
	if new.EngagementRate != nil && (existing.EngagementRate == nil || sie.isPreferred(new.EngagementRate, existing.EngagementRate)) {
		existing.EngagementRate = new.EngagementRate
	}
	if new.SocialMentions != nil && (existing.SocialMentions == nil || sie.isPreferred(new.SocialMentions, existing.SocialMentions)) {
		existing.SocialMentions = new.SocialMentions
	}
}

func (sie *SocialInputsEngine) mergeNewsMetrics(existing *NewsMetrics, new *NewsMetrics) {
	if new.MentionFrequency != nil && (existing.MentionFrequency == nil || sie.isPreferred(new.MentionFrequency, existing.MentionFrequency)) {
		existing.MentionFrequency = new.MentionFrequency
	}
	if new.SentimentScore != nil && (existing.SentimentScore == nil || sie.isPreferred(new.SentimentScore, existing.SentimentScore)) {
		existing.SentimentScore = new.SentimentScore
	}
	if new.AuthorityScore != nil && (existing.AuthorityScore == nil || sie.isPreferred(new.AuthorityScore, existing.AuthorityScore)) {
		existing.AuthorityScore = new.AuthorityScore
	}
	if new.TrendingScore != nil && (existing.TrendingScore == nil || sie.isPreferred(new.TrendingScore, existing.TrendingScore)) {
		existing.TrendingScore = new.TrendingScore
	}
	if new.CategoryRelevance != nil && (existing.CategoryRelevance == nil || sie.isPreferred(new.CategoryRelevance, existing.CategoryRelevance)) {
		existing.CategoryRelevance = new.CategoryRelevance
	}
}

// isPreferred determines if new data point is preferred over existing (newer + higher quality wins)
func (sie *SocialInputsEngine) isPreferred(new, existing *RawDataPoint) bool {
	// Prefer higher quality data
	qualityRank := map[string]int{"A": 4, "B": 3, "C": 2, "D": 1}
	newRank, newExists := qualityRank[new.Quality]
	existingRank, existingExists := qualityRank[existing.Quality]

	if newExists && existingExists {
		if newRank != existingRank {
			return newRank > existingRank
		}
	}

	// If quality is equal, prefer newer data
	return new.Timestamp.After(existing.Timestamp)
}

// buildSocialComponents converts normalized metrics into social components
func (sie *SocialInputsEngine) buildSocialComponents(result *SocialInputs, normalized *NormalizedSocialMetrics) {
	// Developer components
	if normalized.Developer != nil {
		sie.addComponent(result, "commit_frequency", "developer", normalized.Developer.CommitFrequency, 0.3)
		sie.addComponent(result, "active_contributors", "developer", normalized.Developer.ActiveContributors, 0.25)
		sie.addComponent(result, "code_quality", "developer", normalized.Developer.CodeQuality, 0.2)
		sie.addComponent(result, "release_frequency", "developer", normalized.Developer.ReleaseFrequency, 0.15)
		sie.addComponent(result, "issue_resolution", "developer", normalized.Developer.IssueResolution, 0.1)
	}

	// Community components
	if normalized.Community != nil {
		sie.addComponent(result, "star_growth", "community", normalized.Community.StarGrowth, 0.3)
		sie.addComponent(result, "community_size", "community", normalized.Community.CommunitySize, 0.25)
		sie.addComponent(result, "fork_ratio", "community", normalized.Community.ForkRatio, 0.2)
		sie.addComponent(result, "engagement_rate", "community", normalized.Community.EngagementRate, 0.15)
		sie.addComponent(result, "social_mentions", "community", normalized.Community.SocialMentions, 0.1)
	}

	// News components
	if normalized.News != nil {
		sie.addComponent(result, "mention_frequency", "news", normalized.News.MentionFrequency, 0.25)
		sie.addComponent(result, "sentiment_score", "news", normalized.News.SentimentScore, 0.3)
		sie.addComponent(result, "authority_score", "news", normalized.News.AuthorityScore, 0.2)
		sie.addComponent(result, "trending_score", "news", normalized.News.TrendingScore, 0.15)
		sie.addComponent(result, "category_relevance", "news", normalized.News.CategoryRelevance, 0.1)
	}

	// Calculate category-level aggregates
	result.DeveloperActivity = sie.calculateCategoryScore(result.Components, "developer")
	result.CommunityGrowth = sie.calculateCategoryScore(result.Components, "community")
	result.BrandMentions = sie.calculateCategoryScore(result.Components, "news")

	// Special handling for sentiment (already 0-1 normalized)
	if normalized.News != nil && normalized.News.SentimentScore != nil {
		result.SocialSentiment = normalized.News.SentimentScore.NormalizedValue
	} else {
		result.SocialSentiment = 0.5 // Neutral if no sentiment data
	}
}

// addComponent adds a social component to the result
func (sie *SocialInputsEngine) addComponent(result *SocialInputs, name, category string, dataPoint *NormalizedDataPoint, weight float64) {
	if dataPoint == nil {
		return
	}

	component := &SocialComponent{
		Name:         name,
		Category:     category,
		Value:        dataPoint.NormalizedValue,
		Weight:       weight,
		Contribution: dataPoint.NormalizedValue * weight,
		Quality:      dataPoint.Quality,
		LastUpdated:  dataPoint.Timestamp,
		TTL:          dataPoint.TTL,
	}

	result.Components = append(result.Components, component)
}

// calculateCategoryScore computes weighted average for a category of components
func (sie *SocialInputsEngine) calculateCategoryScore(components []*SocialComponent, category string) float64 {
	var weightedSum, totalWeight float64

	for _, comp := range components {
		if comp.Category == category {
			weightedSum += comp.Contribution
			totalWeight += comp.Weight
		}
	}

	if totalWeight > 0 {
		return weightedSum / totalWeight
	}
	return 0.0
}

// calculateOverallSocialScore computes the overall social score as weighted average
func (sie *SocialInputsEngine) calculateOverallSocialScore(components []*SocialComponent) float64 {
	if len(components) == 0 {
		return 0.0
	}

	var weightedSum, totalWeight float64

	for _, comp := range components {
		weightedSum += comp.Contribution
		totalWeight += comp.Weight
	}

	if totalWeight > 0 {
		return weightedSum / totalWeight
	}
	return 0.0
}

// buildProvenance creates source attribution information
func (sie *SocialInputsEngine) buildProvenance(sourceResults []*SourceResult) []*SourceProvenance {
	provenance := make([]*SourceProvenance, 0, len(sourceResults))

	for _, sourceResult := range sourceResults {
		if !sourceResult.Success {
			continue
		}

		metricsProvided := []string{}
		if sourceResult.Developer != nil {
			metricsProvided = append(metricsProvided, "developer_activity")
		}
		if sourceResult.Community != nil {
			metricsProvided = append(metricsProvided, "community_growth")
		}
		if sourceResult.News != nil {
			metricsProvided = append(metricsProvided, "news_mentions")
		}

		prov := &SourceProvenance{
			SourceID:         sourceResult.SourceInfo.ID,
			SourceName:       sourceResult.SourceInfo.Name,
			ReliabilityGrade: sourceResult.SourceInfo.ReliabilityGrade,
			MetricsProvided:  metricsProvided,
			LastFetch:        time.Now(),
			FetchDurationMs:  sourceResult.FetchDurationMs,
			CacheHit:         sourceResult.CacheHit,
		}

		provenance = append(provenance, prov)
	}

	return provenance
}

// assessDataQuality evaluates the quality of social data
func (sie *SocialInputsEngine) assessDataQuality(result *SocialInputs, sourceResults []*SourceResult) *SocialDataQuality {
	successfulSources := 0
	for _, sr := range sourceResults {
		if sr.Success {
			successfulSources++
		}
	}

	quality := &SocialDataQuality{
		SourcesAvailable:   successfulSources,
		SourcesTotal:       len(sie.sources),
		MetricsPopulated:   len(result.Components),
		MetricsTotal:       15, // Total possible metrics across all categories
		DataFreshnessScore: sie.calculateFreshnessScore(result.Components),
		SourceDiversity:    float64(successfulSources) / float64(len(sie.sources)),
	}

	// Assign overall grade based on various factors
	gradeScore := (quality.SourceDiversity * 0.4) +
		(quality.DataFreshnessScore * 0.3) +
		(float64(quality.MetricsPopulated) / float64(quality.MetricsTotal) * 0.3)

	if gradeScore >= 0.9 {
		quality.OverallGrade = "A"
	} else if gradeScore >= 0.8 {
		quality.OverallGrade = "B"
	} else if gradeScore >= 0.7 {
		quality.OverallGrade = "C"
	} else if gradeScore >= 0.6 {
		quality.OverallGrade = "D"
	} else {
		quality.OverallGrade = "F"
	}

	return quality
}

// calculateFreshnessScore computes a 0-1 freshness score based on data ages
func (sie *SocialInputsEngine) calculateFreshnessScore(components []*SocialComponent) float64 {
	if len(components) == 0 {
		return 0.0
	}

	now := time.Now()
	var totalFreshness float64

	for _, comp := range components {
		age := now.Sub(comp.LastUpdated)
		// Fresher data gets higher score, with exponential decay
		freshness := math.Exp(-float64(age.Hours()) / 24.0) // 24h half-life
		totalFreshness += freshness
	}

	return totalFreshness / float64(len(components))
}

// addDataQualityWarnings adds warnings based on data quality assessment
func (sie *SocialInputsEngine) addDataQualityWarnings(result *SocialInputs) {
	quality := result.DataQuality

	if quality.OverallGrade == "D" || quality.OverallGrade == "F" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Low data quality grade: %s", quality.OverallGrade))
	}

	if quality.SourcesAvailable < sie.config.RequireMinSources {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Insufficient data sources: %d available, %d required",
				quality.SourcesAvailable, sie.config.RequireMinSources))
	}

	if quality.MetricsPopulated < sie.config.RequireMinMetrics {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Insufficient metrics: %d populated, %d required",
				quality.MetricsPopulated, sie.config.RequireMinMetrics))
	}

	if quality.DataFreshnessScore < 0.5 {
		result.Warnings = append(result.Warnings, "Data freshness is below acceptable threshold")
	}

	if quality.SourceDiversity < 0.5 {
		result.Warnings = append(result.Warnings, "Limited source diversity may affect data reliability")
	}
}

// GetSocialInputsSummary returns a concise summary of social inputs
func (si *SocialInputs) GetSocialInputsSummary() string {
	return fmt.Sprintf("Social: %s overall=%.3f dev=%.3f comm=%.3f news=%.3f sent=%.3f (grade: %s, %dms)",
		si.Asset, si.OverallSocial, si.DeveloperActivity, si.CommunityGrowth,
		si.BrandMentions, si.SocialSentiment, si.DataQuality.OverallGrade, si.ProcessingTimeMs)
}

// GetDetailedSocialReport returns comprehensive social inputs analysis
func (si *SocialInputs) GetDetailedSocialReport() string {
	report := fmt.Sprintf("Social Inputs Analysis: %s\n", si.Asset)
	report += fmt.Sprintf("Overall Social Score: %.3f | Quality Grade: %s | Processing: %dms\n\n",
		si.OverallSocial, si.DataQuality.OverallGrade, si.ProcessingTimeMs)

	// Category breakdown
	report += fmt.Sprintf("Category Scores:\n")
	report += fmt.Sprintf("  Developer Activity: %.3f\n", si.DeveloperActivity)
	report += fmt.Sprintf("  Community Growth: %.3f\n", si.CommunityGrowth)
	report += fmt.Sprintf("  Brand Mentions: %.3f\n", si.BrandMentions)
	report += fmt.Sprintf("  Social Sentiment: %.3f\n\n", si.SocialSentiment)

	// Component details
	if len(si.Components) > 0 {
		report += fmt.Sprintf("Component Details:\n")
		for _, comp := range si.Components {
			report += fmt.Sprintf("  %s (%s): %.3f (weight: %.2f, quality: %s)\n",
				comp.Name, comp.Category, comp.Value, comp.Weight, comp.Quality)
		}
		report += "\n"
	}

	// Data quality
	report += fmt.Sprintf("Data Quality Assessment:\n")
	report += fmt.Sprintf("  Sources: %d/%d available\n", si.DataQuality.SourcesAvailable, si.DataQuality.SourcesTotal)
	report += fmt.Sprintf("  Metrics: %d/%d populated\n", si.DataQuality.MetricsPopulated, si.DataQuality.MetricsTotal)
	report += fmt.Sprintf("  Freshness Score: %.3f\n", si.DataQuality.DataFreshnessScore)
	report += fmt.Sprintf("  Source Diversity: %.3f\n", si.DataQuality.SourceDiversity)

	// Warnings
	if len(si.Warnings) > 0 {
		report += fmt.Sprintf("\nWarnings:\n")
		for i, warning := range si.Warnings {
			report += fmt.Sprintf("  %d. %s\n", i+1, warning)
		}
	}

	return report
}
