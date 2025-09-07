package db

import (
	"fmt"
	"os"
	"strconv"
	"time"
	
	"gopkg.in/yaml.v3"
)

// AppConfig represents the overall application configuration including database settings
type AppConfig struct {
	Database Config      `yaml:"database"`
	APIs     APIsSection  `yaml:"apis"`
	Cache    CacheSection `yaml:"cache"`
}

// APIsSection holds API-related configuration
type APIsSection struct {
	PrimaryExchange string `yaml:"primary_exchange"`
	Budgets         struct {
		MonthlyLimitUSD      int `yaml:"monthly_limit_usd"`
		SwitchAtRemainingUSD int `yaml:"switch_at_remaining_usd"`
	} `yaml:"budgets"`
}

// CacheSection holds cache-related configuration  
type CacheSection struct {
	Redis struct {
		Addr              string `yaml:"addr"`
		DB                int    `yaml:"db"`
		TLS               bool   `yaml:"tls"`
		DefaultTTLSeconds int    `yaml:"default_ttl_seconds"`
	} `yaml:"redis"`
}

// LoadAppConfig loads application configuration from YAML file with environment variable overrides
func LoadAppConfig(configPath string) (*AppConfig, error) {
	var config AppConfig

	// Load from YAML file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
			}

			if err := yaml.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		}
	}

	// Apply environment variable overrides for database config
	applyEnvOverrides(&config.Database)

	// Set defaults if not specified
	if config.Database.MaxOpenConns == 0 {
		config.Database.MaxOpenConns = 10
	}
	if config.Database.MaxIdleConns == 0 {
		config.Database.MaxIdleConns = 5
	}
	if config.Database.ConnMaxLifetime == 0 {
		config.Database.ConnMaxLifetime = 30 * time.Minute
	}
	if config.Database.ConnMaxIdleTime == 0 {
		config.Database.ConnMaxIdleTime = 5 * time.Minute
	}
	if config.Database.QueryTimeout == 0 {
		config.Database.QueryTimeout = 30 * time.Second
	}

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides to database config
func applyEnvOverrides(config *Config) {
	if dsn := os.Getenv("PG_DSN"); dsn != "" {
		config.DSN = dsn
	}
	
	if enabled := os.Getenv("PG_ENABLED"); enabled != "" {
		if val, err := strconv.ParseBool(enabled); err == nil {
			config.Enabled = val
		}
	}
	
	if maxOpen := os.Getenv("PG_MAX_OPEN_CONNS"); maxOpen != "" {
		if val, err := strconv.Atoi(maxOpen); err == nil {
			config.MaxOpenConns = val
		}
	}
	
	if maxIdle := os.Getenv("PG_MAX_IDLE_CONNS"); maxIdle != "" {
		if val, err := strconv.Atoi(maxIdle); err == nil {
			config.MaxIdleConns = val
		}
	}
	
	if maxLifetime := os.Getenv("PG_CONN_MAX_LIFETIME"); maxLifetime != "" {
		if val, err := time.ParseDuration(maxLifetime); err == nil {
			config.ConnMaxLifetime = val
		}
	}
	
	if maxIdleTime := os.Getenv("PG_CONN_MAX_IDLE_TIME"); maxIdleTime != "" {
		if val, err := time.ParseDuration(maxIdleTime); err == nil {
			config.ConnMaxIdleTime = val
		}
	}
	
	if queryTimeout := os.Getenv("PG_QUERY_TIMEOUT"); queryTimeout != "" {
		if val, err := time.ParseDuration(queryTimeout); err == nil {
			config.QueryTimeout = val
		}
	}
}

// DefaultAppConfig returns a default application configuration
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Database: DefaultConfig(),
		APIs: APIsSection{
			PrimaryExchange: "kraken",
		},
		Cache: CacheSection{},
	}
}

// SaveAppConfig saves the application configuration to a YAML file
func SaveAppConfig(config *AppConfig, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// Validate validates the application configuration
func (c *AppConfig) Validate() error {
	// Validate database config
	if c.Database.Enabled && c.Database.DSN == "" {
		return fmt.Errorf("database DSN is required when database is enabled")
	}

	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive")
	}

	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}

	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("max_idle_conns cannot exceed max_open_conns")
	}

	if c.Database.QueryTimeout <= 0 {
		return fmt.Errorf("query_timeout must be positive")
	}

	// Additional validation can be added for APIs and Cache sections

	return nil
}