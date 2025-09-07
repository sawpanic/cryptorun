# Social Inputs & Brand Signals System

## UX MUST — Live Progress & Explainability

The Social Inputs system provides **transparent, normalized 0-1 brand/social signals** from policy-compliant sources with complete provenance tracking. Every signal includes source attribution, quality grades, and data freshness indicators to support confident decision-making.

**Live Progress Indicators:**
- 🟢 **Grade A**: High-quality, direct API sources (GitHub, verified news)
- 🟡 **Grade B**: Reliable aggregated sources (CoinGecko community data)  
- 🟠 **Grade C**: Lower-quality but useful signals (social mentions, sentiment)
- 🔴 **Grade D/F**: Unreliable data requiring caution or exclusion

**Explainability Features:**
- Per-source reliability grading and data quality assessment
- Complete normalization methodology with rolling statistics and winsorization
- Component-level breakdown showing individual metric contributions
- Source provenance tracking with fetch timing and cache status
- Data freshness scoring with "staleness" warnings and TTL management

---

## Architecture Overview

The Social Inputs system aggregates brand and social signals from multiple free, policy-compliant sources and normalizes them to consistent 0-1 values for downstream scoring systems.

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Social Inputs Engine                    │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ Data Sources  │  │ Normalizer   │  │ API Engine      │  │
│  │ (Multi-source)│  │ (Z-score +   │  │ (Aggregation)   │  │
│  │               │  │  Winsorize)  │  │                 │  │
│  └───────────────┘  └──────────────┘  └─────────────────┘  │
│           │                    │                   │        │
│  ┌────────┴────────────────────┴───────────────────┴────┐   │
│  │              FetchSocialInputs API                  │   │
│  │         (0-1 normalized signals + provenance)       │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Integration Points

- **Normalized Output**: All signals normalized to 0-1 range with transparent methodology
- **Multi-Source Aggregation**: Combines GitHub, CoinGecko, and news APIs with quality preferences
- **Policy Compliance**: Respects robots.txt, uses keyless/free APIs, implements rate limiting
- **Cache-Friendly**: Configurable TTLs per source with stale data fallback options
- **Quality Grading**: A/B/C/D reliability grades for source assessment

---

## Data Sources & Reliability Grades

### Primary Sources (Grade A)

#### GitHub API
- **Scope**: Developer activity metrics from public repositories
- **Signals**: Commit frequency, active contributors, code quality, release frequency, issue resolution
- **TTL**: 6-12 hours depending on metric volatility
- **Rate Limits**: 60 requests/hour (1 req/min), burst size 5
- **Compliance**: Keyless public API, respects GitHub ToS
- **Quality**: Grade A - Direct, authoritative developer data

**Metrics Provided:**
```yaml
commit_frequency:    # commits/week rolling average
  range: 0-∞
  normalization: z-score with 30-day rolling window
  weight: 30% of developer activity
  
active_contributors: # unique contributors/month
  range: 0-∞  
  normalization: z-score with 30-day rolling window
  weight: 25% of developer activity
  
code_quality:       # composite quality score
  range: 0-1
  normalization: direct (already 0-1)
  weight: 20% of developer activity
  
release_frequency:  # releases/quarter
  range: 0-∞
  normalization: z-score with 90-day rolling window
  weight: 15% of developer activity
  
issue_resolution:   # resolution rate 
  range: 0-1
  normalization: direct (already 0-1)  
  weight: 10% of developer activity
```

### Secondary Sources (Grade B)

#### CoinGecko Free API  
- **Scope**: Community metrics and social aggregation
- **Signals**: Community size, social mentions (limited precision)
- **TTL**: 12-24 hours for less volatile metrics
- **Rate Limits**: 0.1 req/sec (very conservative), burst size 3
- **Compliance**: Free tier, keyless, respects rate limits
- **Quality**: Grade B - Aggregated but reliable community data

**Metrics Provided:**
```yaml
community_size:     # aggregate community score
  range: 0-∞ (typically 1k-100k)
  normalization: z-score with 30-day rolling window
  weight: 25% of community growth
  
social_mentions:    # mentions/day across platforms  
  range: 0-∞
  normalization: z-score with 7-day rolling window (volatile)
  weight: 10% of community growth
  quality: Grade C (lower precision)
```

### Tertiary Sources (Grade C)

#### News/Media APIs (Free Tiers)
- **Scope**: Brand mentions and sentiment analysis
- **Signals**: Mention frequency, sentiment scores, authority weighting
- **TTL**: 2-6 hours for news data
- **Rate Limits**: Variable by provider, typically 100-500 req/day
- **Compliance**: Free tier APIs with attribution requirements
- **Quality**: Grade C - Useful but requires validation

**Metrics Provided:**
```yaml
mention_frequency:   # news mentions/day
  range: 0-∞
  normalization: z-score with 7-day rolling window
  weight: 25% of brand mentions
  
sentiment_score:     # aggregate sentiment
  range: -1 to +1
  normalization: linear transform to 0-1: (sentiment + 1) / 2
  weight: 30% of brand mentions (special handling)
  
authority_score:     # source authority weighting
  range: 0-1  
  normalization: direct (already 0-1)
  weight: 20% of brand mentions
  
trending_score:      # trending topic relevance
  range: 0-1
  normalization: direct (already 0-1) 
  weight: 15% of brand mentions
  
category_relevance:  # crypto/tech category fit
  range: 0-1
  normalization: direct (already 0-1)
  weight: 10% of brand mentions
```

---

## Normalization Methodology

### Z-Score Normalization with Winsorization

**Primary Method** for continuous metrics:

1. **Data Collection**: Gather 30-day rolling window (minimum 10 points, maximum 100 for stats)
2. **Winsorization**: Remove extreme outliers using 5th-95th percentile bounds
3. **Statistics Calculation**: Compute rolling mean and standard deviation
4. **Z-Score Transformation**: `z = (value - mean) / std_dev`
5. **Clipping**: Limit z-scores to ±3σ range to prevent extreme values
6. **Min-Max Mapping**: Transform clipped z-scores to 0-1 range

**Mathematical Formula:**
```
winsorized_data = remove_outliers(raw_data, 5th_percentile, 95th_percentile)
rolling_stats = calculate_stats(winsorized_data, 30_days)
z_score = (current_value - rolling_stats.mean) / rolling_stats.std_dev
clipped_z = clamp(z_score, -3.0, +3.0)
normalized = (clipped_z + 3.0) / 6.0  # Maps [-3,+3] to [0,1]
```

### Special Cases

#### Sentiment Score Transformation
Sentiment data comes pre-normalized in [-1, +1] range:
```
normalized_sentiment = (raw_sentiment + 1.0) / 2.0
```
This maps -1 (very negative) → 0, 0 (neutral) → 0.5, +1 (very positive) → 1.

#### Ratio/Percentage Metrics
Already-normalized metrics (0-1 range) are used directly with validation:
```
normalized = clamp(raw_value, 0.0, 1.0)
```

#### Insufficient Data Fallback
When rolling statistics are unavailable:
```
normalized = clamp(raw_value, 0.0, 1.0)  # Assume reasonable bounds
z_score = 0.0                           # No z-score available
percentile = 50.0                       # Assume median
transform_method = "raw_clamp"
```

---

## Configuration & TTL Management

### Per-Source TTL Configuration

**GitHub Sources:**
```yaml
developer_metrics: 6h   # Moderate volatility
community_metrics: 12h  # Lower volatility  
code_quality: 24h       # Very stable metric
```

**CoinGecko Sources:**
```yaml
community_size: 12h     # Daily updates sufficient
social_mentions: 2h     # Higher volatility
price_correlations: 6h  # Moderate changes
```

**News/Media Sources:**
```yaml
mention_frequency: 2h   # Breaking news sensitivity
sentiment_score: 1h     # Rapid sentiment shifts
authority_score: 24h    # Stable source rankings
trending_score: 4h      # Topic trend changes
```

### Rate Limiting & Backoff

**GitHub API:**
```yaml
requests_per_second: 1.0      # 60/hour limit
burst_size: 5                 # Allow small bursts
backoff_multiplier: 2.0       # Exponential backoff
max_backoff: 5m               # Cap backoff duration
```

**CoinGecko API:**
```yaml
requests_per_second: 0.1      # Very conservative 
burst_size: 3                 # Minimal bursting
backoff_multiplier: 2.5       # Aggressive backoff
max_backoff: 10m              # Longer cooling period
```

**News APIs:**
```yaml
requests_per_second: 0.5      # Varies by provider
burst_size: 10                # News queries benefit from bursts
backoff_multiplier: 1.5       # Moderate backoff
max_backoff: 2m               # Quick recovery
```

---

## Output Specification

### SocialInputs Structure

**Complete API Response:**
```json
{
  "asset": "BTC-USD",
  "timestamp": "2025-09-06T15:30:00Z",
  
  "developer_activity": 0.72,    // 0-1 normalized developer signals
  "community_growth": 0.68,      // 0-1 normalized community signals  
  "brand_mentions": 0.55,        // 0-1 normalized news/media signals
  "social_sentiment": 0.62,      // 0-1 normalized sentiment (0.5 = neutral)
  "overall_social": 0.64,        // Weighted average of all components
  
  "components": [
    {
      "name": "commit_frequency",
      "category": "developer",
      "value": 0.75,
      "weight": 0.30,
      "contribution": 0.225,      // value × weight
      "quality": "A",
      "last_updated": "2025-09-06T15:25:00Z",
      "ttl": "6h"
    },
    // ... additional components
  ],
  
  "provenance": [
    {
      "source_id": "github",
      "source_name": "GitHub Developer Activity",
      "reliability_grade": "A",
      "metrics_provided": ["developer_activity", "community_growth"],
      "last_fetch": "2025-09-06T15:25:00Z", 
      "fetch_duration_ms": 247,
      "cache_hit": false
    },
    // ... additional sources
  ],
  
  "data_quality": {
    "overall_grade": "B",          // A-F composite quality grade
    "sources_available": 2,        // Successful source count
    "sources_total": 3,            // Total configured sources
    "metrics_populated": 8,        // Metrics with data
    "metrics_total": 15,           // Total possible metrics  
    "data_freshness_score": 0.82,  // 0-1 freshness assessment
    "source_diversity": 0.67       // 0-1 source coverage
  },
  
  "processing_time_ms": 1247,
  "warnings": [
    "Limited source diversity may affect data reliability",
    "News sentiment data is 3.2 hours old"
  ]
}
```

### Component Categories & Weights

**Developer Activity (0-1 normalized):**
- `commit_frequency`: 30% weight
- `active_contributors`: 25% weight
- `code_quality`: 20% weight
- `release_frequency`: 15% weight  
- `issue_resolution`: 10% weight

**Community Growth (0-1 normalized):**
- `star_growth`: 30% weight
- `community_size`: 25% weight
- `fork_ratio`: 20% weight
- `engagement_rate`: 15% weight
- `social_mentions`: 10% weight

**Brand Mentions (0-1 normalized):**
- `sentiment_score`: 30% weight (special: sentiment transform)
- `mention_frequency`: 25% weight
- `authority_score`: 20% weight  
- `trending_score`: 15% weight
- `category_relevance`: 10% weight

---

## Data Quality Assessment

### Quality Grade Calculation

**Overall Grade** computed from multiple factors:
```
grade_score = (source_diversity × 0.4) + 
              (data_freshness × 0.3) + 
              (metric_coverage × 0.3)

grade_mapping:
  ≥0.9 → "A"    # Excellent quality
  ≥0.8 → "B"    # Good quality  
  ≥0.7 → "C"    # Acceptable quality
  ≥0.6 → "D"    # Poor quality
  <0.6 → "F"    # Unacceptable quality
```

### Freshness Scoring

**Data Freshness Score** based on component ages:
```
freshness_per_component = exp(-age_hours / 24)  # 24h half-life
overall_freshness = average(component_freshness_scores)
```

**Freshness Warnings:**
- Grade D/F: Any data >24 hours old
- Grade C: Majority of data >12 hours old
- Grade B: Some data >6 hours old
- Grade A: All data <6 hours old

### Source Diversity Assessment

**Source Diversity Score:**
```
diversity = successful_sources / total_configured_sources
```

**Diversity Warnings:**
- <0.5: "Limited source diversity may affect data reliability"
- <0.3: "Very limited sources - consider data reliability"
- =0.0: "No sources available - cannot provide social signals"

---

## Performance Requirements

### API Performance Targets

- **Single Asset Fetch**: <500ms P95 latency
- **Batch Processing**: <30s for 50 assets  
- **Memory Usage**: <50MB working set
- **Cache Hit Rate**: >70% target for production

### Error Handling & Fallbacks

**Source Failure Handling:**
1. **Graceful Degradation**: Continue with available sources if minimum threshold met
2. **Stale Data Fallback**: Use cached data if fresh fetch fails (configurable)
3. **Quality Downgrade**: Lower quality grade when sources fail
4. **Transparent Warnings**: Alert users to data quality issues

**Minimum Requirements:**
- At least 1 successful source required
- At least 2 populated metrics required
- Maximum 30-second total processing time
- Grade F data rejected (unless explicitly allowed)

### Monitoring & Alerting

**System Health Metrics:**
- Source success rate per provider
- Average processing latency per asset
- Cache hit/miss ratios by source
- Quality grade distribution over time

**Alert Conditions:**
- Source success rate <80% over 1 hour
- Average latency >1s over 15 minutes  
- Cache hit rate <50% over 30 minutes
- >20% of responses graded D/F over 1 hour

---

## Implementation Examples

### Basic Usage

```go
// Initialize sources
githubSource := social.NewGitHubSource(githubConfig, httpClient, cache)
coinGeckoSource := social.NewCoinGeckoSource(coinGeckoConfig, httpClient, cache)
sources := []social.SocialDataSource{githubSource, coinGeckoSource}

// Initialize normalizer and engine
normalizer := social.NewNormalizer(normalizationConfig, cache)
engine := social.NewSocialInputsEngine(sources, normalizer, engineConfig)

// Fetch social inputs for an asset
inputs, err := engine.FetchSocialInputs(ctx, "BTC-USD")
if err != nil {
    log.Printf("Failed to fetch social inputs: %v", err)
    return
}

// Use normalized signals (all 0-1 range)
fmt.Printf("Overall Social Score: %.3f\n", inputs.OverallSocial)
fmt.Printf("Developer Activity: %.3f\n", inputs.DeveloperActivity) 
fmt.Printf("Community Growth: %.3f\n", inputs.CommunityGrowth)
fmt.Printf("Social Sentiment: %.3f\n", inputs.SocialSentiment) // 0.5 = neutral
fmt.Printf("Data Quality: %s\n", inputs.DataQuality.OverallGrade)
```

### Quality Assessment

```go
// Check data quality before using signals
if inputs.DataQuality.OverallGrade == "F" {
    log.Println("Social data quality too poor - skipping social signals")
    return
}

if inputs.DataQuality.SourceDiversity < 0.5 {
    log.Printf("Warning: Limited source diversity (%.1f%%)\n", 
               inputs.DataQuality.SourceDiversity*100)
}

// Use quality-adjusted weighting
qualityMultiplier := map[string]float64{
    "A": 1.0,
    "B": 0.8, 
    "C": 0.6,
    "D": 0.3,
    "F": 0.0,
}[inputs.DataQuality.OverallGrade]

adjustedSocialScore := inputs.OverallSocial * qualityMultiplier
```

### Component Analysis

```go
// Analyze individual components
for _, component := range inputs.Components {
    fmt.Printf("%s (%s): %.3f (quality: %s, weight: %.2f)\n",
        component.Name, 
        component.Category,
        component.Value,
        component.Quality, 
        component.Weight)
    
    // Check data freshness
    age := time.Since(component.LastUpdated)
    if age > component.TTL {
        fmt.Printf("  WARNING: %s data is stale (%.1fh old)\n", 
                   component.Name, age.Hours())
    }
}
```

### Source Provenance

```go
// Review data sources and fetch performance
for _, prov := range inputs.Provenance {
    fmt.Printf("Source: %s (grade %s)\n", prov.SourceName, prov.ReliabilityGrade)
    fmt.Printf("  Metrics: %v\n", prov.MetricsProvided)  
    fmt.Printf("  Fetch: %dms %s\n", prov.FetchDurationMs,
               map[bool]string{true: "(cached)", false: "(fresh)"}[prov.CacheHit])
}
```

---

## Integration with CryptoRun

### Scoring System Integration

Social inputs integrate with the main CryptoRun scoring system as **capped auxiliary signals** (maximum +10 points):

```go
// In main scoring logic
socialInputs, err := socialEngine.FetchSocialInputs(ctx, asset)
if err != nil || socialInputs.DataQuality.OverallGrade == "F" {
    // Skip social signals if unavailable or poor quality
    socialBonus = 0.0
} else {
    // Apply quality-adjusted social bonus (max +10 points)
    qualityMultiplier := getQualityMultiplier(socialInputs.DataQuality.OverallGrade)
    socialBonus = socialInputs.OverallSocial * 10.0 * qualityMultiplier
}

// Social bonus is applied OUTSIDE the main 100-point allocation
finalScore = baseScore + min(socialBonus, 10.0)
```

### Configuration Integration

```yaml
# config/social.yaml
social_inputs:
  enabled: true
  max_processing_time: 30s
  require_min_sources: 1
  require_min_metrics: 2
  
  sources:
    github:
      enabled: true
      timeout: 10s
      cache_ttl: 6h
      
    coingecko:
      enabled: true  
      timeout: 15s
      cache_ttl: 12h
      
    news_apis:
      enabled: false  # Disabled by default
      timeout: 8s
      cache_ttl: 2h
```

### Menu System Integration

```
CryptoRun Menu > Data Sources > Social Inputs
┌─────────────────────────────────────────────┐
│  Social Inputs & Brand Signals             │
├─────────────────────────────────────────────┤  
│  [1] View Current Social Scores             │
│  [2] Source Health & Performance            │
│  [3] Configure Source Priorities            │
│  [4] Data Quality Assessment                │
│  [5] Cache Management                       │
│  [0] Back to Data Sources Menu              │
└─────────────────────────────────────────────┘
```

---

*Last updated: September 2024*  
*Version: Social Inputs v1.0.0*  
*Status: Production Ready*