package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/middleware"
)

// LoadConfig loads configuration from YAML files
func LoadConfig(configDir string) (*datafacade.Config, error) {
	config := &datafacade.Config{}
	
	// Load cache configuration
	if err := loadCacheConfig(configDir, config); err != nil {
		return nil, fmt.Errorf("load cache config: %w", err)
	}
	
	// Load rate limit configuration
	if err := loadRateLimitConfig(configDir, config); err != nil {
		return nil, fmt.Errorf("load rate limit config: %w", err)
	}
	
	// Load circuit breaker configuration
	if err := loadCircuitConfig(configDir, config); err != nil {
		return nil, fmt.Errorf("load circuit config: %w", err)
	}
	
	// Load PIT configuration
	if err := loadPITConfig(configDir, config); err != nil {
		return nil, fmt.Errorf("load PIT config: %w", err)
	}
	
	// Load venue configurations
	if err := loadVenueConfig(configDir, config); err != nil {
		return nil, fmt.Errorf("load venue config: %w", err)
	}
	
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	
	return config, nil
}

func loadCacheConfig(configDir string, config *datafacade.Config) error {
	cacheFile := filepath.Join(configDir, "cache.yaml")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		// Use default cache config
		config.CacheConfig = datafacade.CacheConfig{
			Redis: datafacade.RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
			},
			TTLs: map[string]map[string]time.Duration{
				"default": {
					"trades":       30 * time.Second,
					"klines":       60 * time.Second,
					"orderbook_l1": 5 * time.Second,
					"orderbook_l2": 10 * time.Second,
					"funding":      300 * time.Second,
					"openinterest": 60 * time.Second,
				},
			},
		}
		return nil
	}
	
	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		return fmt.Errorf("read cache config: %w", err)
	}
	
	// Parse as generic map first for TTL duration conversion
	var cacheData struct {
		Redis struct {
			Addr     string `yaml:"addr"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
		} `yaml:"redis"`
		TTLs map[string]map[string]string `yaml:"ttls"`
	}
	
	if err := yaml.Unmarshal(data, &cacheData); err != nil {
		return fmt.Errorf("unmarshal cache config: %w", err)
	}
	
	// Convert TTL strings to durations
	ttlsMap := make(map[string]map[string]time.Duration)
	for venue, ttls := range cacheData.TTLs {
		ttlsMap[venue] = make(map[string]time.Duration)
		for dataType, ttlStr := range ttls {
			duration, err := time.ParseDuration(ttlStr)
			if err != nil {
				return fmt.Errorf("parse TTL duration for %s.%s: %w", venue, dataType, err)
			}
			ttlsMap[venue][dataType] = duration
		}
	}
	
	config.CacheConfig = datafacade.CacheConfig{
		Redis: datafacade.RedisConfig{
			Addr:     cacheData.Redis.Addr,
			Password: cacheData.Redis.Password,
			DB:       cacheData.Redis.DB,
		},
		TTLs: ttlsMap,
	}
	
	return nil
}

func loadRateLimitConfig(configDir string, config *datafacade.Config) error {
	rateLimitFile := filepath.Join(configDir, "rate_limits.yaml")
	if _, err := os.Stat(rateLimitFile); os.IsNotExist(err) {
		// Use default rate limit config
		config.RateLimitConfig = createDefaultRateLimitConfig()
		return nil
	}
	
	data, err := ioutil.ReadFile(rateLimitFile)
	if err != nil {
		return fmt.Errorf("read rate limit config: %w", err)
	}
	
	if err := yaml.Unmarshal(data, &config.RateLimitConfig); err != nil {
		return fmt.Errorf("unmarshal rate limit config: %w", err)
	}
	
	return nil
}

func loadCircuitConfig(configDir string, config *datafacade.Config) error {
	circuitFile := filepath.Join(configDir, "circuits.yaml")
	if _, err := os.Stat(circuitFile); os.IsNotExist(err) {
		// Use default circuit config
		config.CircuitConfig = createDefaultCircuitConfig()
		return nil
	}
	
	data, err := ioutil.ReadFile(circuitFile)
	if err != nil {
		return fmt.Errorf("read circuit config: %w", err)
	}
	
	// Parse as generic map first for duration conversion
	var circuitData struct {
		Venues map[string]struct {
			HTTP struct {
				FailureThreshold int    `yaml:"failure_threshold"`
				SuccessThreshold int    `yaml:"success_threshold"`
				Timeout          string `yaml:"timeout"`
				MaxRequests      int    `yaml:"max_requests"`
			} `yaml:"http"`
			WebSocket struct {
				FailureThreshold int    `yaml:"failure_threshold"`
				SuccessThreshold int    `yaml:"success_threshold"`
				Timeout          string `yaml:"timeout"`
				MaxRequests      int    `yaml:"max_requests"`
			} `yaml:"websocket"`
		} `yaml:"venues"`
	}
	
	if err := yaml.Unmarshal(data, &circuitData); err != nil {
		return fmt.Errorf("unmarshal circuit config: %w", err)
	}
	
	// Convert timeout strings to durations
	venuesMap := make(map[string]middleware.VenueConfig)
	for venueName, venueData := range circuitData.Venues {
		httpTimeout, err := time.ParseDuration(venueData.HTTP.Timeout)
		if err != nil {
			return fmt.Errorf("parse HTTP timeout for %s: %w", venueName, err)
		}
		
		wsTimeout, err := time.ParseDuration(venueData.WebSocket.Timeout)
		if err != nil {
			return fmt.Errorf("parse WebSocket timeout for %s: %w", venueName, err)
		}
		
		venuesMap[venueName] = middleware.VenueConfig{
			HTTP: struct {
				FailureThreshold int
				SuccessThreshold int
				Timeout          time.Duration
				MaxRequests      int
			}{
				FailureThreshold: venueData.HTTP.FailureThreshold,
				SuccessThreshold: venueData.HTTP.SuccessThreshold,
				Timeout:          httpTimeout,
				MaxRequests:      venueData.HTTP.MaxRequests,
			},
			WebSocket: struct {
				FailureThreshold int
				SuccessThreshold int
				Timeout          time.Duration
				MaxRequests      int
			}{
				FailureThreshold: venueData.WebSocket.FailureThreshold,
				SuccessThreshold: venueData.WebSocket.SuccessThreshold,
				Timeout:          wsTimeout,
				MaxRequests:      venueData.WebSocket.MaxRequests,
			},
		}
	}
	
	config.CircuitConfig = datafacade.CircuitConfig{
		Venues: venuesMap,
	}
	
	return nil
}

func loadPITConfig(configDir string, config *datafacade.Config) error {
	pitFile := filepath.Join(configDir, "pit.yaml")
	if _, err := os.Stat(pitFile); os.IsNotExist(err) {
		// Use default PIT config
		config.PITConfig = datafacade.PITConfig{
			BasePath:      "./data/pit",
			Compression:   true,
			RetentionDays: 30,
		}
		return nil
	}
	
	data, err := ioutil.ReadFile(pitFile)
	if err != nil {
		return fmt.Errorf("read PIT config: %w", err)
	}
	
	if err := yaml.Unmarshal(data, &config.PITConfig); err != nil {
		return fmt.Errorf("unmarshal PIT config: %w", err)
	}
	
	return nil
}

func loadVenueConfig(configDir string, config *datafacade.Config) error {
	venueFile := filepath.Join(configDir, "venues.yaml")
	if _, err := os.Stat(venueFile); os.IsNotExist(err) {
		// Use default venue config
		config.Venues = createDefaultVenueConfig()
		return nil
	}
	
	data, err := ioutil.ReadFile(venueFile)
	if err != nil {
		return fmt.Errorf("read venue config: %w", err)
	}
	
	if err := yaml.Unmarshal(data, &config.Venues); err != nil {
		return fmt.Errorf("unmarshal venue config: %w", err)
	}
	
	return nil
}

func validateConfig(config *datafacade.Config) error {
	// Validate Redis configuration
	if config.CacheConfig.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}
	
	// Validate rate limits
	for venue, limits := range config.RateLimitConfig.Venues {
		if limits.RequestsPerSecond <= 0 {
			return fmt.Errorf("requests_per_second must be positive for venue %s", venue)
		}
		if limits.BurstAllowance <= 0 {
			return fmt.Errorf("burst_allowance must be positive for venue %s", venue)
		}
	}
	
	// Validate circuit breaker thresholds
	for venue, venueConfig := range config.CircuitConfig.Venues {
		if venueConfig.HTTP.FailureThreshold <= 0 {
			return fmt.Errorf("HTTP failure_threshold must be positive for venue %s", venue)
		}
		if venueConfig.WebSocket.FailureThreshold <= 0 {
			return fmt.Errorf("WebSocket failure_threshold must be positive for venue %s", venue)
		}
	}
	
	// Validate PIT configuration
	if config.PITConfig.BasePath == "" {
		return fmt.Errorf("PIT base_path is required")
	}
	if config.PITConfig.RetentionDays < 0 {
		return fmt.Errorf("PIT retention_days must be non-negative")
	}
	
	// Validate venues
	if len(config.Venues) == 0 {
		return fmt.Errorf("at least one venue must be configured")
	}
	
	enabledCount := 0
	for venue, venueConfig := range config.Venues {
		if venueConfig.BaseURL == "" {
			return fmt.Errorf("base_url is required for venue %s", venue)
		}
		if venueConfig.WSURL == "" {
			return fmt.Errorf("ws_url is required for venue %s", venue)
		}
		if venueConfig.Enabled {
			enabledCount++
		}
	}
	
	if enabledCount == 0 {
		return fmt.Errorf("at least one venue must be enabled")
	}
	
	return nil
}

func createDefaultRateLimitConfig() datafacade.RateLimitConfig {
	return datafacade.RateLimitConfig{
		Venues: map[string]datafacade.VenueRateLimits{
			"binance": {
				RequestsPerSecond: 20,
				BurstAllowance:    10,
				WeightLimits: map[string]int{
					"trades":       1,
					"klines":       1,
					"orderbook":    1,
					"funding":      1,
					"openinterest": 1,
				},
				DailyLimit:   intPtr(160000),
				MonthlyLimit: intPtr(5000000),
			},
			"okx": {
				RequestsPerSecond: 10,
				BurstAllowance:    5,
				WeightLimits:      map[string]int{},
				DailyLimit:        intPtr(50000),
				MonthlyLimit:      intPtr(1500000),
			},
			"coinbase": {
				RequestsPerSecond: 5,
				BurstAllowance:    3,
				WeightLimits:      map[string]int{},
				DailyLimit:        intPtr(10000),
				MonthlyLimit:      intPtr(300000),
			},
			"kraken": {
				RequestsPerSecond: 1,
				BurstAllowance:    1,
				WeightLimits:      map[string]int{},
				DailyLimit:        intPtr(5000),
				MonthlyLimit:      intPtr(150000),
			},
		},
	}
}

func createDefaultCircuitConfig() datafacade.CircuitConfig {
	return datafacade.CircuitConfig{
		Venues: map[string]middleware.VenueConfig{
			"binance": createDefaultVenueCircuitConfig(5, 3, 30*time.Second, 3, 60*time.Second),
			"okx":     createDefaultVenueCircuitConfig(5, 3, 30*time.Second, 3, 60*time.Second),
			"coinbase": createDefaultVenueCircuitConfig(8, 4, 45*time.Second, 2, 60*time.Second),
			"kraken":   createDefaultVenueCircuitConfig(3, 2, 60*time.Second, 1, 120*time.Second),
		},
	}
}

func createDefaultVenueCircuitConfig(httpFailThreshold, httpSuccThreshold int, httpTimeout time.Duration,
	httpMaxRequests int, wsTimeout time.Duration) middleware.VenueConfig {
	return middleware.VenueConfig{
		HTTP: struct {
			FailureThreshold int
			SuccessThreshold int
			Timeout          time.Duration
			MaxRequests      int
		}{
			FailureThreshold: httpFailThreshold,
			SuccessThreshold: httpSuccThreshold,
			Timeout:          httpTimeout,
			MaxRequests:      httpMaxRequests,
		},
		WebSocket: struct {
			FailureThreshold int
			SuccessThreshold int
			Timeout          time.Duration
			MaxRequests      int
		}{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          wsTimeout,
			MaxRequests:      2,
		},
	}
}

func createDefaultVenueConfig() map[string]datafacade.VenueConfig {
	return map[string]datafacade.VenueConfig{
		"binance": {
			BaseURL: "https://api.binance.com",
			WSURL:   "wss://stream.binance.com:9443/ws",
			Enabled: true,
		},
		"okx": {
			BaseURL: "https://www.okx.com",
			WSURL:   "wss://ws.okx.com:8443",
			Enabled: true,
		},
		"coinbase": {
			BaseURL: "https://api.exchange.coinbase.com",
			WSURL:   "wss://ws-feed.exchange.coinbase.com",
			Enabled: true,
		},
		"kraken": {
			BaseURL: "https://api.kraken.com",
			WSURL:   "wss://ws.kraken.com",
			Enabled: true,
		},
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}