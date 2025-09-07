package application

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type APIsConfig struct {
	PrimaryExchange string `yaml:"primary_exchange"`
	Budgets struct { MonthlyLimitUSD int `yaml:"monthly_limit_usd"`; SwitchAtRemainingUSD int `yaml:"switch_at_remaining_usd"` } `yaml:"budgets"`
}

func LoadAPIsConfig(path string) (*APIsConfig, error) {
	b, err := os.ReadFile(path); if err != nil { return nil, err }
	var c APIsConfig; if err := yaml.Unmarshal(b, &c); err != nil { return nil, err }
	return &c, nil
}

type CacheConfig struct { Redis struct { Addr string; DB int; TLS bool; DefaultTTLSeconds int `yaml:"default_ttl_seconds"` } `yaml:"redis"` }

func LoadCacheConfig(path string) (*CacheConfig, error) {
	b, err := os.ReadFile(path); if err != nil { return nil, err }
	var c CacheConfig; if err := yaml.Unmarshal(b, &c); err != nil { return nil, err }
	return &c, nil
}

func (c *CacheConfig) DefaultTTL() time.Duration { return time.Duration(c.Redis.DefaultTTLSeconds) * time.Second }
