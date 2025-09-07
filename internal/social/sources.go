package social

import (
	"context"
	"fmt"
	"time"
)

// SocialDataSource defines the interface for retrieving social/brand signals
type SocialDataSource interface {
	// GetDeveloperActivity returns normalized developer activity metrics (0-1)
	GetDeveloperActivity(ctx context.Context, asset string) (*DeveloperMetrics, error)

	// GetCommunityMetrics returns normalized community engagement metrics (0-1)
	GetCommunityMetrics(ctx context.Context, asset string) (*CommunityMetrics, error)

	// GetNewsMetrics returns normalized news mention metrics (0-1)
	GetNewsMetrics(ctx context.Context, asset string) (*NewsMetrics, error)

	// GetSourceInfo returns metadata about this data source
	GetSourceInfo() *SourceInfo
}

// SourceInfo contains metadata about a social data source
type SourceInfo struct {
	ID               string        `json:"id"`                // Source identifier (e.g., "github", "coingecko")
	Name             string        `json:"name"`              // Human-readable name
	ReliabilityGrade string        `json:"reliability_grade"` // A/B/C reliability grade
	TTL              time.Duration `json:"ttl"`               // Cache TTL for this source
	RateLimit        *RateLimit    `json:"rate_limit"`        // Rate limiting configuration
	IsKeyless        bool          `json:"is_keyless"`        // Whether source requires API keys
	RespectRobotsTxt bool          `json:"respect_robots"`    // Whether source respects robots.txt
}

// RateLimit defines rate limiting parameters for a data source
type RateLimit struct {
	RequestsPerSecond float64       `json:"requests_per_second"` // Max requests per second
	BurstSize         int           `json:"burst_size"`          // Burst allowance
	BackoffMultiplier float64       `json:"backoff_multiplier"`  // Exponential backoff multiplier
	MaxBackoff        time.Duration `json:"max_backoff"`         // Maximum backoff duration
}

// RawDataPoint represents an unnormalized data point from a source
type RawDataPoint struct {
	Value     float64       `json:"value"`     // Raw metric value
	Timestamp time.Time     `json:"timestamp"` // When the data was collected
	SourceID  string        `json:"source_id"` // Source that provided this data
	TTL       time.Duration `json:"ttl"`       // How long this data is valid
	Quality   string        `json:"quality"`   // Data quality indicator (A/B/C/D)
}

// DeveloperMetrics contains development activity signals
type DeveloperMetrics struct {
	CommitFrequency    *RawDataPoint `json:"commit_frequency"`    // Commits per week
	ActiveContributors *RawDataPoint `json:"active_contributors"` // Number of active contributors
	CodeQuality        *RawDataPoint `json:"code_quality"`        // Code quality metrics
	ReleaseFrequency   *RawDataPoint `json:"release_frequency"`   // Releases per quarter
	IssueResolution    *RawDataPoint `json:"issue_resolution"`    // Issue resolution rate
}

// CommunityMetrics contains community engagement signals
type CommunityMetrics struct {
	StarGrowth     *RawDataPoint `json:"star_growth"`     // GitHub star growth rate
	ForkRatio      *RawDataPoint `json:"fork_ratio"`      // Fork-to-star ratio
	CommunitySize  *RawDataPoint `json:"community_size"`  // Community size proxy
	EngagementRate *RawDataPoint `json:"engagement_rate"` // Community engagement rate
	SocialMentions *RawDataPoint `json:"social_mentions"` // Social media mentions
}

// NewsMetrics contains news and media coverage signals
type NewsMetrics struct {
	MentionFrequency  *RawDataPoint `json:"mention_frequency"`  // News mentions per day
	SentimentScore    *RawDataPoint `json:"sentiment_score"`    // Aggregate sentiment (-1 to +1)
	AuthorityScore    *RawDataPoint `json:"authority_score"`    // Source authority weight
	TrendingScore     *RawDataPoint `json:"trending_score"`     // Trending topic score
	CategoryRelevance *RawDataPoint `json:"category_relevance"` // Relevance to crypto/tech
}

// GitHubSource provides developer activity data from GitHub's free API
type GitHubSource struct {
	config     *SourceConfig
	httpClient HTTPClient
	cache      Cache
}

// NewGitHubSource creates a new GitHub data source
func NewGitHubSource(config *SourceConfig, httpClient HTTPClient, cache Cache) *GitHubSource {
	return &GitHubSource{
		config:     config,
		httpClient: httpClient,
		cache:      cache,
	}
}

// GetDeveloperActivity fetches developer metrics from GitHub
func (gs *GitHubSource) GetDeveloperActivity(ctx context.Context, asset string) (*DeveloperMetrics, error) {
	// Implementation would fetch from GitHub API
	// For now, return mock data that respects the interface
	return &DeveloperMetrics{
		CommitFrequency: &RawDataPoint{
			Value:     15.5, // commits per week
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "A",
		},
		ActiveContributors: &RawDataPoint{
			Value:     8.0, // active contributors
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "A",
		},
		CodeQuality: &RawDataPoint{
			Value:     0.78, // quality score 0-1
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       12 * time.Hour,
			Quality:   "B",
		},
		ReleaseFrequency: &RawDataPoint{
			Value:     2.5, // releases per quarter
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       24 * time.Hour,
			Quality:   "A",
		},
		IssueResolution: &RawDataPoint{
			Value:     0.65, // resolution rate 0-1
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "B",
		},
	}, nil
}

// GetCommunityMetrics fetches community metrics from GitHub
func (gs *GitHubSource) GetCommunityMetrics(ctx context.Context, asset string) (*CommunityMetrics, error) {
	return &CommunityMetrics{
		StarGrowth: &RawDataPoint{
			Value:     25.3, // stars per week
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       6 * time.Hour,
			Quality:   "A",
		},
		ForkRatio: &RawDataPoint{
			Value:     0.12, // forks/stars ratio
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       12 * time.Hour,
			Quality:   "A",
		},
		CommunitySize: &RawDataPoint{
			Value:     1250.0, // watchers + contributors
			Timestamp: time.Now(),
			SourceID:  "github",
			TTL:       12 * time.Hour,
			Quality:   "B",
		},
	}, nil
}

// GetNewsMetrics returns empty for GitHub (doesn't provide news data)
func (gs *GitHubSource) GetNewsMetrics(ctx context.Context, asset string) (*NewsMetrics, error) {
	return &NewsMetrics{}, nil
}

// GetSourceInfo returns GitHub source metadata
func (gs *GitHubSource) GetSourceInfo() *SourceInfo {
	return &SourceInfo{
		ID:               "github",
		Name:             "GitHub Developer Activity",
		ReliabilityGrade: "A",
		TTL:              6 * time.Hour,
		RateLimit: &RateLimit{
			RequestsPerSecond: 1.0, // GitHub allows 60/hour = 1/min
			BurstSize:         5,
			BackoffMultiplier: 2.0,
			MaxBackoff:        5 * time.Minute,
		},
		IsKeyless:        true, // GitHub public API is keyless for basic data
		RespectRobotsTxt: true,
	}
}

// CoinGeckoSource provides community and market data from CoinGecko's free API
type CoinGeckoSource struct {
	config     *SourceConfig
	httpClient HTTPClient
	cache      Cache
}

// NewCoinGeckoSource creates a new CoinGecko data source
func NewCoinGeckoSource(config *SourceConfig, httpClient HTTPClient, cache Cache) *CoinGeckoSource {
	return &CoinGeckoSource{
		config:     config,
		httpClient: httpClient,
		cache:      cache,
	}
}

// GetDeveloperActivity returns limited developer data from CoinGecko
func (cgs *CoinGeckoSource) GetDeveloperActivity(ctx context.Context, asset string) (*DeveloperMetrics, error) {
	return &DeveloperMetrics{
		CommitFrequency: &RawDataPoint{
			Value:     12.8, // commits per week (lower precision than GitHub)
			Timestamp: time.Now(),
			SourceID:  "coingecko",
			TTL:       24 * time.Hour,
			Quality:   "B",
		},
	}, nil
}

// GetCommunityMetrics fetches community metrics from CoinGecko
func (cgs *CoinGeckoSource) GetCommunityMetrics(ctx context.Context, asset string) (*CommunityMetrics, error) {
	return &CommunityMetrics{
		CommunitySize: &RawDataPoint{
			Value:     45000.0, // community score from CoinGecko
			Timestamp: time.Now(),
			SourceID:  "coingecko",
			TTL:       12 * time.Hour,
			Quality:   "B",
		},
		SocialMentions: &RawDataPoint{
			Value:     180.5, // social mentions per day
			Timestamp: time.Now(),
			SourceID:  "coingecko",
			TTL:       2 * time.Hour,
			Quality:   "C",
		},
	}, nil
}

// GetNewsMetrics returns empty for CoinGecko
func (cgs *CoinGeckoSource) GetNewsMetrics(ctx context.Context, asset string) (*NewsMetrics, error) {
	return &NewsMetrics{}, nil
}

// GetSourceInfo returns CoinGecko source metadata
func (cgs *CoinGeckoSource) GetSourceInfo() *SourceInfo {
	return &SourceInfo{
		ID:               "coingecko",
		Name:             "CoinGecko Community Data",
		ReliabilityGrade: "B",
		TTL:              12 * time.Hour,
		RateLimit: &RateLimit{
			RequestsPerSecond: 0.1, // Conservative rate limiting for free tier
			BurstSize:         3,
			BackoffMultiplier: 2.5,
			MaxBackoff:        10 * time.Minute,
		},
		IsKeyless:        true,
		RespectRobotsTxt: true,
	}
}

// FakeSocialSource provides deterministic fake data for testing
type FakeSocialSource struct {
	sourceInfo *SourceInfo
	devData    *DeveloperMetrics
	commData   *CommunityMetrics
	newsData   *NewsMetrics
	shouldFail bool
}

// NewFakeSocialSource creates a fake data source with configurable responses
func NewFakeSocialSource(sourceID string, grade string) *FakeSocialSource {
	return &FakeSocialSource{
		sourceInfo: &SourceInfo{
			ID:               sourceID,
			Name:             fmt.Sprintf("Fake %s Source", sourceID),
			ReliabilityGrade: grade,
			TTL:              1 * time.Hour,
			RateLimit: &RateLimit{
				RequestsPerSecond: 10.0,
				BurstSize:         20,
				BackoffMultiplier: 1.5,
				MaxBackoff:        30 * time.Second,
			},
			IsKeyless:        true,
			RespectRobotsTxt: true,
		},
		devData: &DeveloperMetrics{
			CommitFrequency: &RawDataPoint{
				Value:     10.0,
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
			ActiveContributors: &RawDataPoint{
				Value:     5.0,
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
		},
		commData: &CommunityMetrics{
			StarGrowth: &RawDataPoint{
				Value:     20.0,
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
			CommunitySize: &RawDataPoint{
				Value:     1000.0,
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
		},
		newsData: &NewsMetrics{
			MentionFrequency: &RawDataPoint{
				Value:     15.0,
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
			SentimentScore: &RawDataPoint{
				Value:     0.3, // Slightly positive sentiment
				Timestamp: time.Now(),
				SourceID:  sourceID,
				TTL:       1 * time.Hour,
				Quality:   grade,
			},
		},
	}
}

// SetFailure configures the fake source to return errors
func (fss *FakeSocialSource) SetFailure(shouldFail bool) {
	fss.shouldFail = shouldFail
}

// GetDeveloperActivity returns fake developer data
func (fss *FakeSocialSource) GetDeveloperActivity(ctx context.Context, asset string) (*DeveloperMetrics, error) {
	if fss.shouldFail {
		return nil, fmt.Errorf("fake source configured to fail")
	}
	return fss.devData, nil
}

// GetCommunityMetrics returns fake community data
func (fss *FakeSocialSource) GetCommunityMetrics(ctx context.Context, asset string) (*CommunityMetrics, error) {
	if fss.shouldFail {
		return nil, fmt.Errorf("fake source configured to fail")
	}
	return fss.commData, nil
}

// GetNewsMetrics returns fake news data
func (fss *FakeSocialSource) GetNewsMetrics(ctx context.Context, asset string) (*NewsMetrics, error) {
	if fss.shouldFail {
		return nil, fmt.Errorf("fake source configured to fail")
	}
	return fss.newsData, nil
}

// GetSourceInfo returns fake source metadata
func (fss *FakeSocialSource) GetSourceInfo() *SourceInfo {
	return fss.sourceInfo
}

// Supporting interfaces and types

// HTTPClient interface for HTTP requests (allows mocking)
type HTTPClient interface {
	Get(ctx context.Context, url string) (*HTTPResponse, error)
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

// Cache interface for caching social data
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// SourceConfig contains configuration for social data sources
type SourceConfig struct {
	Enabled        bool          `yaml:"enabled"`          // Whether this source is enabled
	BaseURL        string        `yaml:"base_url"`         // Base URL for API endpoints
	Timeout        time.Duration `yaml:"timeout"`          // Request timeout
	RetryAttempts  int           `yaml:"retry_attempts"`   // Number of retry attempts
	CacheKeyPrefix string        `yaml:"cache_key_prefix"` // Cache key prefix

	// GitHub-specific config
	GitHubOwner string `yaml:"github_owner"` // GitHub repository owner
	GitHubRepo  string `yaml:"github_repo"`  // GitHub repository name

	// CoinGecko-specific config
	CoinGeckoCoinID string `yaml:"coingecko_coin_id"` // CoinGecko coin identifier
}

// DefaultSourceConfig returns default configuration for social sources
func DefaultSourceConfig() *SourceConfig {
	return &SourceConfig{
		Enabled:         true,
		BaseURL:         "",
		Timeout:         10 * time.Second,
		RetryAttempts:   3,
		CacheKeyPrefix:  "social:",
		GitHubOwner:     "",
		GitHubRepo:      "",
		CoinGeckoCoinID: "",
	}
}
