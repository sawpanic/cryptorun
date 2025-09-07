package catalyst

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// EventSource defines interface for catalyst event providers
type EventSource interface {
	GetEvents(ctx context.Context, symbols []string) ([]RawEvent, error)
	GetName() string
	GetCacheTTL() time.Duration
	RespectRobotsTxt() bool
}

// RawEvent represents an event from external sources before normalization
type RawEvent struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	Source      string    `json:"source"`
	Tier        int       `json:"tier"`     // 1=Major, 2=Minor, 3=Info
	Polarity    int       `json:"polarity"` // +1=Positive, -1=Negative, 0=Neutral
	Categories  []string  `json:"categories"`
}

// CatalystClient manages multiple event sources with caching
type CatalystClient struct {
	sources []EventSource
	cache   *redis.Client
	config  SourceConfig
}

// SourceConfig configures catalyst source behavior
type SourceConfig struct {
	PollingCadence   time.Duration `yaml:"polling_cadence"`
	CacheTTL         time.Duration `yaml:"cache_ttl"`
	RespectRobotsTxt bool          `yaml:"respect_robots_txt"`
	UserAgent        string        `yaml:"user_agent"`
	RequestTimeout   time.Duration `yaml:"request_timeout"`
	MaxRetries       int           `yaml:"max_retries"`
}

// NewCatalystClient creates client with configured sources
func NewCatalystClient(cache *redis.Client, config SourceConfig) *CatalystClient {
	client := &CatalystClient{
		cache:  cache,
		config: config,
	}

	// Initialize available sources
	client.sources = []EventSource{
		NewCoinMarketCalSource(config),
		NewExchangeAnnouncementSource(config),
	}

	log.Info().
		Int("sources", len(client.sources)).
		Dur("polling_cadence", config.PollingCadence).
		Dur("cache_ttl", config.CacheTTL).
		Msg("Catalyst client initialized")

	return client
}

// GetEvents fetches and merges events from all sources with caching
func (c *CatalystClient) GetEvents(ctx context.Context, symbols []string) ([]RawEvent, error) {
	allEvents := []RawEvent{}

	for _, source := range c.sources {
		cacheKey := fmt.Sprintf("catalyst:%s:%s", source.GetName(), strings.Join(symbols, ","))

		// Try cache first
		cached, err := c.getCachedEvents(ctx, cacheKey)
		if err == nil && len(cached) > 0 {
			log.Debug().
				Str("source", source.GetName()).
				Int("events", len(cached)).
				Msg("Using cached catalyst events")
			allEvents = append(allEvents, cached...)
			continue
		}

		// Fetch from source
		events, err := source.GetEvents(ctx, symbols)
		if err != nil {
			log.Warn().
				Err(err).
				Str("source", source.GetName()).
				Msg("Failed to fetch catalyst events")
			continue
		}

		// Cache results
		if err := c.cacheEvents(ctx, cacheKey, events, source.GetCacheTTL()); err != nil {
			log.Warn().
				Err(err).
				Str("cache_key", cacheKey).
				Msg("Failed to cache catalyst events")
		}

		log.Debug().
			Str("source", source.GetName()).
			Int("events", len(events)).
			Msg("Fetched fresh catalyst events")

		allEvents = append(allEvents, events...)
	}

	// Deduplicate events by ID and symbol
	deduped := c.deduplicateEvents(allEvents)

	log.Info().
		Int("total_events", len(allEvents)).
		Int("unique_events", len(deduped)).
		Strs("symbols", symbols).
		Msg("Catalyst events merged from all sources")

	return deduped, nil
}

// getCachedEvents retrieves events from Redis cache
func (c *CatalystClient) getCachedEvents(ctx context.Context, key string) ([]RawEvent, error) {
	if c.cache == nil {
		return nil, fmt.Errorf("no cache configured")
	}

	data, err := c.cache.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var events []RawEvent
	if err := json.Unmarshal([]byte(data), &events); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached events: %w", err)
	}

	return events, nil
}

// cacheEvents stores events in Redis with TTL
func (c *CatalystClient) cacheEvents(ctx context.Context, key string, events []RawEvent, ttl time.Duration) error {
	if c.cache == nil {
		return nil // No caching configured
	}

	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	return c.cache.Set(ctx, key, data, ttl).Err()
}

// deduplicateEvents removes duplicate events by ID+symbol combination
func (c *CatalystClient) deduplicateEvents(events []RawEvent) []RawEvent {
	seen := make(map[string]bool)
	unique := []RawEvent{}

	for _, event := range events {
		key := fmt.Sprintf("%s:%s:%s", event.Source, event.ID, event.Symbol)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, event)
		}
	}

	return unique
}

// NormalizeSymbol converts exchange-specific symbols to CryptoRun format
func (c *CatalystClient) NormalizeSymbol(symbol, source string) string {
	// Common normalization patterns
	switch source {
	case "coinmarketcal":
		// CMC uses full names, map to symbols
		mappings := map[string]string{
			"Bitcoin":  "BTCUSD",
			"Ethereum": "ETHUSD",
			"Cardano":  "ADAUSD",
			"Solana":   "SOLUSD",
		}
		if normalized, exists := mappings[symbol]; exists {
			return normalized
		}
	case "kraken":
		// Kraken uses XXBTZUSD format, normalize to BTCUSD
		if strings.HasPrefix(symbol, "XX") && strings.HasSuffix(symbol, "ZUSD") {
			base := strings.TrimPrefix(strings.TrimSuffix(symbol, "ZUSD"), "XX")
			return base + "USD"
		}
	}

	// Default: ensure USD suffix for consistency
	if !strings.HasSuffix(symbol, "USD") && !strings.Contains(symbol, "USD") {
		return symbol + "USD"
	}

	return strings.ToUpper(symbol)
}

// CoinMarketCalSource implements free tier CoinMarketCal integration
type CoinMarketCalSource struct {
	config SourceConfig
	client *http.Client
}

// NewCoinMarketCalSource creates CoinMarketCal source
func NewCoinMarketCalSource(config SourceConfig) *CoinMarketCalSource {
	return &CoinMarketCalSource{
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}
}

// GetEvents fetches events from CoinMarketCal free API
func (c *CoinMarketCalSource) GetEvents(ctx context.Context, symbols []string) ([]RawEvent, error) {
	// Mock implementation - in production would call real CoinMarketCal API
	// Free tier typically allows limited requests per month

	events := []RawEvent{
		{
			ID:          "cmc_001",
			Symbol:      "BTCUSD",
			Title:       "Bitcoin ETF Decision",
			Description: "SEC decision on Bitcoin ETF approval",
			Date:        time.Now().Add(2 * 7 * 24 * time.Hour), // 2 weeks
			Source:      "coinmarketcal",
			Tier:        1, // Major event
			Polarity:    1, // Positive
			Categories:  []string{"regulatory", "etf"},
		},
		{
			ID:          "cmc_002",
			Symbol:      "ETHUSD",
			Title:       "Ethereum Upgrade Delay",
			Description: "Shanghai upgrade delayed by 2 weeks",
			Date:        time.Now().Add(6 * 7 * 24 * time.Hour), // 6 weeks
			Source:      "coinmarketcal",
			Tier:        2,  // Minor event
			Polarity:    -1, // Negative (delay)
			Categories:  []string{"upgrade", "technical"},
		},
	}

	log.Debug().
		Int("events", len(events)).
		Strs("symbols", symbols).
		Msg("CoinMarketCal events fetched (mock)")

	return events, nil
}

// GetName returns source identifier
func (c *CoinMarketCalSource) GetName() string {
	return "coinmarketcal"
}

// GetCacheTTL returns cache duration for this source
func (c *CoinMarketCalSource) GetCacheTTL() time.Duration {
	return 30 * time.Minute // CMC data changes slowly
}

// RespectRobotsTxt returns whether to check robots.txt
func (c *CoinMarketCalSource) RespectRobotsTxt() bool {
	return c.config.RespectRobotsTxt
}

// ExchangeAnnouncementSource scrapes exchange announcement pages
type ExchangeAnnouncementSource struct {
	config SourceConfig
	client *http.Client
}

// NewExchangeAnnouncementSource creates exchange announcement source
func NewExchangeAnnouncementSource(config SourceConfig) *ExchangeAnnouncementSource {
	return &ExchangeAnnouncementSource{
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}
}

// GetEvents scrapes exchange announcements with robots.txt respect
func (e *ExchangeAnnouncementSource) GetEvents(ctx context.Context, symbols []string) ([]RawEvent, error) {
	// Mock implementation - in production would:
	// 1. Check robots.txt for each exchange
	// 2. Scrape announcement pages with polite delays
	// 3. Parse HTML for relevant events

	if e.config.RespectRobotsTxt {
		// Check robots.txt compliance (mock)
		if allowed, err := e.checkRobotsTxt("https://blog.kraken.com"); err != nil || !allowed {
			log.Warn().
				Err(err).
				Bool("allowed", allowed).
				Msg("Robots.txt check failed or disallowed")
			return []RawEvent{}, nil
		}
	}

	events := []RawEvent{
		{
			ID:          "kraken_001",
			Symbol:      "SOLUSD",
			Title:       "Solana Staking Launch",
			Description: "Kraken announces Solana staking with 6% APY",
			Date:        time.Now().Add(1 * 7 * 24 * time.Hour), // 1 week
			Source:      "kraken",
			Tier:        2, // Minor event
			Polarity:    1, // Positive
			Categories:  []string{"staking", "exchange"},
		},
		{
			ID:          "kraken_002",
			Symbol:      "ADAUSD",
			Title:       "Maintenance Window",
			Description: "Cardano trading halted for maintenance",
			Date:        time.Now().Add(3 * 24 * time.Hour), // 3 days
			Source:      "kraken",
			Tier:        3,  // Info event
			Polarity:    -1, // Negative (trading halt)
			Categories:  []string{"maintenance", "trading"},
		},
	}

	log.Debug().
		Int("events", len(events)).
		Strs("symbols", symbols).
		Msg("Exchange announcements fetched (mock)")

	return events, nil
}

// GetName returns source identifier
func (e *ExchangeAnnouncementSource) GetName() string {
	return "exchange_announcements"
}

// GetCacheTTL returns cache duration for this source
func (e *ExchangeAnnouncementSource) GetCacheTTL() time.Duration {
	return 5 * time.Minute // Announcements can be time-sensitive
}

// RespectRobotsTxt returns whether to check robots.txt
func (e *ExchangeAnnouncementSource) RespectRobotsTxt() bool {
	return e.config.RespectRobotsTxt
}

// checkRobotsTxt verifies if scraping is allowed (mock implementation)
func (e *ExchangeAnnouncementSource) checkRobotsTxt(baseURL string) (bool, error) {
	// Mock implementation - in production would:
	// 1. Fetch /robots.txt from the domain
	// 2. Parse robots.txt file
	// 3. Check if User-Agent is allowed for the path

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return false, fmt.Errorf("invalid URL: %w", err)
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedURL.Scheme, parsedURL.Host)

	log.Debug().
		Str("robots_url", robotsURL).
		Str("user_agent", e.config.UserAgent).
		Msg("Checking robots.txt compliance (mock)")

	// Mock: always allow for demo, but log the check
	return true, nil
}
