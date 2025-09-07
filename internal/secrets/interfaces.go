package secrets

import (
	"context"
	"fmt"
	"time"
)

// SecretProvider defines the interface for secret management systems
type SecretProvider interface {
	// GetSecret retrieves a secret by key
	GetSecret(ctx context.Context, key string) (*Secret, error)
	
	// GetSecrets retrieves multiple secrets by keys
	GetSecrets(ctx context.Context, keys []string) (map[string]*Secret, error)
	
	// SetSecret stores a secret (for providers that support write operations)
	SetSecret(ctx context.Context, key string, value []byte, options *SecretOptions) error
	
	// DeleteSecret removes a secret (for providers that support delete operations)
	DeleteSecret(ctx context.Context, key string) error
	
	// ListSecrets returns available secret keys (for providers that support listing)
	ListSecrets(ctx context.Context, prefix string) ([]string, error)
	
	// Health returns the health status of the secret provider
	Health(ctx context.Context) *HealthStatus
}

// Secret represents a secret with metadata
type Secret struct {
	Key       string            `json:"key"`
	Value     []byte            `json:"-"` // Never serialize the actual value
	Metadata  map[string]string `json:"metadata,omitempty"`
	Version   string            `json:"version,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// SecretOptions provides configuration for secret storage
type SecretOptions struct {
	TTL         time.Duration     `json:"ttl,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Encrypt     bool              `json:"encrypt,omitempty"`
}

// HealthStatus represents the health of a secret provider
type HealthStatus struct {
	Healthy        bool              `json:"healthy"`
	Provider       string            `json:"provider"`
	LastCheck      time.Time         `json:"last_check"`
	ResponseTimeMS int64             `json:"response_time_ms"`
	Errors         []string          `json:"errors,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Manager provides a unified interface for secret management with multiple providers
type Manager struct {
	providers map[string]SecretProvider
	primary   string
	fallback  []string
}

// NewManager creates a new secret manager with providers
func NewManager(primary string, providers map[string]SecretProvider) *Manager {
	return &Manager{
		providers: providers,
		primary:   primary,
		fallback:  make([]string, 0),
	}
}

// WithFallback configures fallback providers in order of preference
func (m *Manager) WithFallback(providers ...string) *Manager {
	m.fallback = providers
	return m
}

// GetSecret retrieves a secret with fallback support
func (m *Manager) GetSecret(ctx context.Context, key string) (*Secret, error) {
	// Try primary provider first
	if provider, exists := m.providers[m.primary]; exists {
		if secret, err := provider.GetSecret(ctx, key); err == nil {
			return secret, nil
		}
	}
	
	// Try fallback providers
	for _, fallbackName := range m.fallback {
		if provider, exists := m.providers[fallbackName]; exists {
			if secret, err := provider.GetSecret(ctx, key); err == nil {
				return secret, nil
			}
		}
	}
	
	return nil, fmt.Errorf("secret not found in any provider: %s", key)
}

// GetSecrets retrieves multiple secrets efficiently
func (m *Manager) GetSecrets(ctx context.Context, keys []string) (map[string]*Secret, error) {
	results := make(map[string]*Secret)
	remaining := make([]string, 0, len(keys))
	
	// Try primary provider first
	if provider, exists := m.providers[m.primary]; exists {
		if secrets, err := provider.GetSecrets(ctx, keys); err == nil {
			for k, v := range secrets {
				results[k] = v
			}
		}
	}
	
	// Identify missing secrets
	for _, key := range keys {
		if _, found := results[key]; !found {
			remaining = append(remaining, key)
		}
	}
	
	// Try fallback providers for remaining secrets
	for _, fallbackName := range m.fallback {
		if len(remaining) == 0 {
			break
		}
		
		if provider, exists := m.providers[fallbackName]; exists {
			if secrets, err := provider.GetSecrets(ctx, remaining); err == nil {
				newRemaining := make([]string, 0)
				for _, key := range remaining {
					if secret, found := secrets[key]; found {
						results[key] = secret
					} else {
						newRemaining = append(newRemaining, key)
					}
				}
				remaining = newRemaining
			}
		}
	}
	
	// Return partial results even if some secrets are missing
	return results, nil
}

// Health returns the health status of all providers
func (m *Manager) Health(ctx context.Context) map[string]*HealthStatus {
	health := make(map[string]*HealthStatus)
	
	for name, provider := range m.providers {
		health[name] = provider.Health(ctx)
	}
	
	return health
}

// String returns the secret value as a string
func (s *Secret) String() string {
	return string(s.Value)
}

// IsExpired checks if the secret has expired
func (s *Secret) IsExpired() bool {
	return s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt)
}

// Redact returns a redacted version of the secret for logging
func (s *Secret) Redact() *Secret {
	redacted := *s
	if len(redacted.Value) > 0 {
		redacted.Value = []byte("[REDACTED]")
	}
	return &redacted
}

// Error types for secret operations
var (
	ErrSecretNotFound     = fmt.Errorf("secret not found")
	ErrSecretExpired      = fmt.Errorf("secret expired")
	ErrProviderUnhealthy  = fmt.Errorf("provider unhealthy")
	ErrInvalidKey         = fmt.Errorf("invalid secret key")
	ErrAccessDenied       = fmt.Errorf("access denied")
	ErrSecretTooLarge     = fmt.Errorf("secret value too large")
)

// SecretNotFoundError wraps secret not found errors with context
type SecretNotFoundError struct {
	Key      string
	Provider string
}

func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("secret '%s' not found in provider '%s'", e.Key, e.Provider)
}

// ProviderHealthError wraps provider health errors
type ProviderHealthError struct {
	Provider string
	Errors   []string
}

func (e *ProviderHealthError) Error() string {
	return fmt.Sprintf("provider '%s' unhealthy: %v", e.Provider, e.Errors)
}