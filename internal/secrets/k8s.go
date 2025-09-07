package secrets

import (
	"context"
	"fmt"
	"path/filepath"
	"os"
	"strings"
	"time"
)

// K8sProvider implements SecretProvider for Kubernetes secrets mounted as files
// This is the standard approach for production Kubernetes deployments
type K8sProvider struct {
	mountPath string
	namespace string
	metadata  map[string]string
}

// NewK8sProvider creates a new Kubernetes secret provider
func NewK8sProvider(mountPath, namespace string) *K8sProvider {
	return &K8sProvider{
		mountPath: mountPath,
		namespace: namespace,
		metadata: map[string]string{
			"provider":   "kubernetes",
			"version":    "1.0",
			"mount_path": mountPath,
			"namespace":  namespace,
		},
	}
}

// GetSecret retrieves a secret from Kubernetes secret mount
func (p *K8sProvider) GetSecret(ctx context.Context, key string) (*Secret, error) {
	secretPath := filepath.Join(p.mountPath, key)
	
	// Check if file exists
	info, err := os.Stat(secretPath)
	if os.IsNotExist(err) {
		return nil, &SecretNotFoundError{
			Key:      key,
			Provider: "kubernetes",
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat secret file %s: %w", secretPath, err)
	}
	
	// Read secret value
	value, err := os.ReadFile(secretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret file %s: %w", secretPath, err)
	}
	
	// Kubernetes secrets often have trailing newlines
	value = []byte(strings.TrimSpace(string(value)))
	
	secret := &Secret{
		Key:       key,
		Value:     value,
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
		Metadata: map[string]string{
			"source":     "kubernetes",
			"namespace":  p.namespace,
			"file_path":  secretPath,
			"file_size":  fmt.Sprintf("%d", len(value)),
			"file_mode":  info.Mode().String(),
		},
	}
	
	return secret, nil
}

// GetSecrets retrieves multiple secrets from Kubernetes secret mounts
func (p *K8sProvider) GetSecrets(ctx context.Context, keys []string) (map[string]*Secret, error) {
	results := make(map[string]*Secret)
	
	for _, key := range keys {
		if secret, err := p.GetSecret(ctx, key); err == nil {
			results[key] = secret
		}
		// We don't error on missing individual secrets in batch operations
	}
	
	return results, nil
}

// SetSecret is not supported for Kubernetes provider (secrets are managed externally)
func (p *K8sProvider) SetSecret(ctx context.Context, key string, value []byte, options *SecretOptions) error {
	return fmt.Errorf("SetSecret not supported for Kubernetes provider - secrets are managed externally")
}

// DeleteSecret is not supported for Kubernetes provider
func (p *K8sProvider) DeleteSecret(ctx context.Context, key string) error {
	return fmt.Errorf("DeleteSecret not supported for Kubernetes provider - secrets are managed externally")
}

// ListSecrets returns available secret files in the mount path
func (p *K8sProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	entries, err := os.ReadDir(p.mountPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read secrets directory %s: %w", p.mountPath, err)
	}
	
	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		
		// Skip Kubernetes metadata files
		if name == "..data" || strings.HasPrefix(name, "..") {
			continue
		}
		
		// Apply prefix filter if specified
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		
		keys = append(keys, name)
	}
	
	return keys, nil
}

// Health returns the health status of the Kubernetes provider
func (p *K8sProvider) Health(ctx context.Context) *HealthStatus {
	start := time.Now()
	healthy := true
	var errors []string
	
	// Check if mount path exists and is accessible
	info, err := os.Stat(p.mountPath)
	if err != nil {
		healthy = false
		errors = append(errors, fmt.Sprintf("mount path not accessible: %v", err))
	} else if !info.IsDir() {
		healthy = false
		errors = append(errors, "mount path is not a directory")
	}
	
	// Check if we can list secrets
	if healthy {
		_, err := p.ListSecrets(ctx, "")
		if err != nil {
			healthy = false
			errors = append(errors, fmt.Sprintf("cannot list secrets: %v", err))
		}
	}
	
	responseTime := time.Since(start).Milliseconds()
	
	return &HealthStatus{
		Healthy:        healthy,
		Provider:       "kubernetes",
		LastCheck:      time.Now(),
		ResponseTimeMS: responseTime,
		Errors:         errors,
		Metadata: map[string]string{
			"mount_path": p.mountPath,
			"namespace":  p.namespace,
		},
	}
}

// GetSecretMetadata returns metadata about a secret without reading its value
func (p *K8sProvider) GetSecretMetadata(ctx context.Context, key string) (map[string]string, error) {
	secretPath := filepath.Join(p.mountPath, key)
	
	info, err := os.Stat(secretPath)
	if os.IsNotExist(err) {
		return nil, &SecretNotFoundError{
			Key:      key,
			Provider: "kubernetes",
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat secret file %s: %w", secretPath, err)
	}
	
	return map[string]string{
		"file_path":    secretPath,
		"file_size":    fmt.Sprintf("%d", info.Size()),
		"file_mode":    info.Mode().String(),
		"modified_at":  info.ModTime().Format(time.RFC3339),
		"namespace":    p.namespace,
		"mount_path":   p.mountPath,
	}, nil
}

// WatchSecrets returns a channel that sends updates when secrets change
// This is useful for hot-reloading configuration
func (p *K8sProvider) WatchSecrets(ctx context.Context, keys []string) (<-chan SecretUpdate, error) {
	updates := make(chan SecretUpdate, 10)
	
	go func() {
		defer close(updates)
		
		// Simple polling-based implementation
		// In production, you might want to use inotify or similar
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		// Track last modification times
		lastMods := make(map[string]time.Time)
		
		// Initialize with current mod times
		for _, key := range keys {
			if metadata, err := p.GetSecretMetadata(ctx, key); err == nil {
				if modTime, err := time.Parse(time.RFC3339, metadata["modified_at"]); err == nil {
					lastMods[key] = modTime
				}
			}
		}
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Check for changes
				for _, key := range keys {
					if metadata, err := p.GetSecretMetadata(ctx, key); err == nil {
						if modTime, err := time.Parse(time.RFC3339, metadata["modified_at"]); err == nil {
							if lastMod, exists := lastMods[key]; !exists || modTime.After(lastMod) {
								lastMods[key] = modTime
								updates <- SecretUpdate{
									Key:       key,
									Action:    "updated",
									Timestamp: modTime,
								}
							}
						}
					}
				}
			}
		}
	}()
	
	return updates, nil
}

// SecretUpdate represents a change to a secret
type SecretUpdate struct {
	Key       string    `json:"key"`
	Action    string    `json:"action"` // "created", "updated", "deleted"
	Timestamp time.Time `json:"timestamp"`
}