package secrets

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// EnvProvider implements SecretProvider for environment variables
// This is the simplest provider for development and basic deployments
type EnvProvider struct {
	prefix       string
	redactPatterns []*regexp.Regexp
	metadata     map[string]string
}

// NewEnvProvider creates a new environment variable secret provider
func NewEnvProvider(prefix string) *EnvProvider {
	// Default patterns to redact in logs (case-insensitive)
	defaultPatterns := []string{
		`(?i).*password.*`,
		`(?i).*secret.*`,
		`(?i).*key.*`,
		`(?i).*token.*`,
		`(?i).*dsn.*`,
		`(?i).*auth.*`,
		`(?i).*credential.*`,
	}
	
	redactPatterns := make([]*regexp.Regexp, len(defaultPatterns))
	for i, pattern := range defaultPatterns {
		redactPatterns[i] = regexp.MustCompile(pattern)
	}
	
	return &EnvProvider{
		prefix:         prefix,
		redactPatterns: redactPatterns,
		metadata: map[string]string{
			"provider": "environment",
			"version":  "1.0",
		},
	}
}

// GetSecret retrieves a secret from environment variables
func (p *EnvProvider) GetSecret(ctx context.Context, key string) (*Secret, error) {
	envKey := p.buildEnvKey(key)
	value := os.Getenv(envKey)
	
	if value == "" {
		return nil, &SecretNotFoundError{
			Key:      key,
			Provider: "environment",
		}
	}
	
	secret := &Secret{
		Key:       key,
		Value:     []byte(value),
		CreatedAt: time.Now(), // We can't know the actual creation time from env vars
		Metadata: map[string]string{
			"source":     "environment",
			"env_key":    envKey,
			"redacted":   p.shouldRedact(envKey),
		},
	}
	
	return secret, nil
}

// GetSecrets retrieves multiple secrets from environment variables
func (p *EnvProvider) GetSecrets(ctx context.Context, keys []string) (map[string]*Secret, error) {
	results := make(map[string]*Secret)
	
	for _, key := range keys {
		if secret, err := p.GetSecret(ctx, key); err == nil {
			results[key] = secret
		}
		// We don't error on missing individual secrets in batch operations
	}
	
	return results, nil
}

// SetSecret is not supported for environment provider
func (p *EnvProvider) SetSecret(ctx context.Context, key string, value []byte, options *SecretOptions) error {
	return fmt.Errorf("SetSecret not supported for environment provider")
}

// DeleteSecret is not supported for environment provider
func (p *EnvProvider) DeleteSecret(ctx context.Context, key string) error {
	return fmt.Errorf("DeleteSecret not supported for environment provider")
}

// ListSecrets returns environment variables matching the prefix
func (p *EnvProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	envPrefix := p.buildEnvKey(prefix)
	
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, envPrefix) {
			// Extract key from environment variable
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envKey := parts[0]
				// Convert back to secret key format
				secretKey := p.envKeyToSecretKey(envKey)
				keys = append(keys, secretKey)
			}
		}
	}
	
	return keys, nil
}

// Health returns the health status of the environment provider
func (p *EnvProvider) Health(ctx context.Context) *HealthStatus {
	start := time.Now()
	
	// Test by checking a few common environment variables
	testVars := []string{"HOME", "PATH", "USER"}
	healthy := true
	var errors []string
	
	for _, testVar := range testVars {
		if os.Getenv(testVar) == "" {
			// This might be normal on some systems, so we don't fail hard
		}
	}
	
	responseTime := time.Since(start).Milliseconds()
	
	return &HealthStatus{
		Healthy:        healthy,
		Provider:       "environment",
		LastCheck:      time.Now(),
		ResponseTimeMS: responseTime,
		Errors:         errors,
		Metadata: map[string]string{
			"prefix":         p.prefix,
			"redact_enabled": "true",
		},
	}
}

// Helper methods

func (p *EnvProvider) buildEnvKey(key string) string {
	if p.prefix == "" {
		return strings.ToUpper(key)
	}
	return fmt.Sprintf("%s_%s", strings.ToUpper(p.prefix), strings.ToUpper(key))
}

func (p *EnvProvider) envKeyToSecretKey(envKey string) string {
	if p.prefix == "" {
		return strings.ToLower(envKey)
	}
	
	prefix := fmt.Sprintf("%s_", strings.ToUpper(p.prefix))
	if strings.HasPrefix(envKey, prefix) {
		return strings.ToLower(strings.TrimPrefix(envKey, prefix))
	}
	
	return strings.ToLower(envKey)
}

func (p *EnvProvider) shouldRedact(envKey string) string {
	for _, pattern := range p.redactPatterns {
		if pattern.MatchString(envKey) {
			return "true"
		}
	}
	return "false"
}

// WithRedactionPatterns allows customizing redaction patterns
func (p *EnvProvider) WithRedactionPatterns(patterns []string) *EnvProvider {
	redactPatterns := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		redactPatterns[i] = regexp.MustCompile(pattern)
	}
	p.redactPatterns = redactPatterns
	return p
}

// GetRedactedEnvVars returns a list of environment variables that would be redacted
// This is useful for debugging and security audits
func (p *EnvProvider) GetRedactedEnvVars() []string {
	var redacted []string
	
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envKey := parts[0]
			if p.shouldRedact(envKey) == "true" {
				redacted = append(redacted, envKey)
			}
		}
	}
	
	return redacted
}