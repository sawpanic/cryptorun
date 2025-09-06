package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ProvidersConfig represents the complete provider operations configuration
type ProvidersConfig struct {
	Providers map[string]ProviderConfig `yaml:"providers"`
	Budget    BudgetConfig              `yaml:"budget"`
	Global    GlobalConfig              `yaml:"global"`
}

// ProviderConfig represents configuration for a single provider
type ProviderConfig struct {
	Host        string        `yaml:"host"`
	RPS         int           `yaml:"rps"`          // Requests per second
	Burst       int           `yaml:"burst"`        // Burst capacity
	DailyBudget int           `yaml:"daily_budget"` // Max requests per UTC day
	TTLSecs     int           `yaml:"ttl_secs"`     // Cache TTL in seconds
	BackoffMS   BackoffConfig `yaml:"backoff_ms"`   // Backoff configuration
	Circuit     CircuitConfig `yaml:"circuit"`      // Circuit breaker config
	Enabled     bool          `yaml:"enabled"`      // Provider enabled flag
	BaseURL     string        `yaml:"base_url"`     // Base URL for API calls
	Constraints interface{}   `yaml:"constraints"`  // Provider-specific constraints
}

// BackoffConfig represents exponential backoff configuration
type BackoffConfig struct {
	Base   int  `yaml:"base"`   // Base backoff in milliseconds
	Max    int  `yaml:"max"`    // Maximum backoff in milliseconds
	Jitter bool `yaml:"jitter"` // Enable jitter to prevent thundering herd
}

// CircuitConfig represents circuit breaker configuration
type CircuitConfig struct {
	FailureThreshold int `yaml:"failure_threshold"` // Consecutive failures to open circuit
	SuccessThreshold int `yaml:"success_threshold"` // Successes needed to close circuit
	TimeoutMS        int `yaml:"timeout_ms"`        // Request timeout in milliseconds
}

// BudgetConfig represents budget management configuration
type BudgetConfig struct {
	WarnThreshold float64 `yaml:"warn_threshold"` // Warn at this fraction of daily budget
	ResetHour     int     `yaml:"reset_hour"`     // UTC hour to reset budgets (0-23)
}

// GlobalConfig represents global provider settings
type GlobalConfig struct {
	MaxConcurrentPerHost int    `yaml:"max_concurrent_per_host"` // Max concurrent requests per provider
	UserAgent            string `yaml:"user_agent"`              // User agent for all requests
}

// LoadProvidersConfig loads provider configuration from YAML file
func LoadProvidersConfig(configPath string) (*ProvidersConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read providers config: %w", err)
	}

	var config ProvidersConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse providers config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid providers config: %w", err)
	}

	return &config, nil
}

// Validate ensures the configuration is valid and consistent
func (c *ProvidersConfig) Validate() error {
	// Validate budget config
	if c.Budget.WarnThreshold <= 0 || c.Budget.WarnThreshold > 1 {
		return fmt.Errorf("budget warn_threshold must be between 0 and 1, got %f", c.Budget.WarnThreshold)
	}
	if c.Budget.ResetHour < 0 || c.Budget.ResetHour > 23 {
		return fmt.Errorf("budget reset_hour must be between 0 and 23, got %d", c.Budget.ResetHour)
	}

	// Validate global config
	if c.Global.MaxConcurrentPerHost <= 0 {
		return fmt.Errorf("global max_concurrent_per_host must be positive, got %d", c.Global.MaxConcurrentPerHost)
	}
	if c.Global.UserAgent == "" {
		return fmt.Errorf("global user_agent cannot be empty")
	}

	// Validate each provider
	for name, provider := range c.Providers {
		if err := provider.Validate(name); err != nil {
			return fmt.Errorf("provider %s: %w", name, err)
		}
	}

	return nil
}

// Validate ensures a provider configuration is valid
func (p *ProviderConfig) Validate(name string) error {
	if p.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if p.RPS <= 0 {
		return fmt.Errorf("rps must be positive, got %d", p.RPS)
	}
	if p.Burst < p.RPS {
		return fmt.Errorf("burst (%d) must be >= rps (%d)", p.Burst, p.RPS)
	}
	if p.DailyBudget <= 0 {
		return fmt.Errorf("daily_budget must be positive, got %d", p.DailyBudget)
	}
	if p.TTLSecs < 0 {
		return fmt.Errorf("ttl_secs cannot be negative, got %d", p.TTLSecs)
	}
	if p.BaseURL == "" {
		return fmt.Errorf("base_url cannot be empty")
	}

	// Validate backoff config
	if err := p.BackoffMS.Validate(); err != nil {
		return fmt.Errorf("backoff_ms: %w", err)
	}

	// Validate circuit config
	if err := p.Circuit.Validate(); err != nil {
		return fmt.Errorf("circuit: %w", err)
	}

	return nil
}

// Validate ensures backoff configuration is valid
func (b *BackoffConfig) Validate() error {
	if b.Base <= 0 {
		return fmt.Errorf("base must be positive, got %d", b.Base)
	}
	if b.Max <= b.Base {
		return fmt.Errorf("max (%d) must be > base (%d)", b.Max, b.Base)
	}
	return nil
}

// Validate ensures circuit breaker configuration is valid
func (c *CircuitConfig) Validate() error {
	if c.FailureThreshold <= 0 {
		return fmt.Errorf("failure_threshold must be positive, got %d", c.FailureThreshold)
	}
	if c.SuccessThreshold <= 0 {
		return fmt.Errorf("success_threshold must be positive, got %d", c.SuccessThreshold)
	}
	if c.TimeoutMS <= 0 {
		return fmt.Errorf("timeout_ms must be positive, got %d", c.TimeoutMS)
	}
	return nil
}

// GetCacheTTL returns the cache TTL as a time.Duration
func (p *ProviderConfig) GetCacheTTL() time.Duration {
	return time.Duration(p.TTLSecs) * time.Second
}

// GetRequestTimeout returns the request timeout as a time.Duration
func (p *ProviderConfig) GetRequestTimeout() time.Duration {
	return time.Duration(p.Circuit.TimeoutMS) * time.Millisecond
}

// GetBaseBackoff returns the base backoff as a time.Duration
func (p *ProviderConfig) GetBaseBackoff() time.Duration {
	return time.Duration(p.BackoffMS.Base) * time.Millisecond
}

// GetMaxBackoff returns the maximum backoff as a time.Duration
func (p *ProviderConfig) GetMaxBackoff() time.Duration {
	return time.Duration(p.BackoffMS.Max) * time.Millisecond
}

// GetProvider returns configuration for a specific provider
func (c *ProvidersConfig) GetProvider(name string) (*ProviderConfig, bool) {
	config, exists := c.Providers[name]
	return &config, exists
}

// IsProviderEnabled checks if a provider is enabled
func (c *ProvidersConfig) IsProviderEnabled(name string) bool {
	if config, exists := c.Providers[name]; exists {
		return config.Enabled
	}
	return false
}
