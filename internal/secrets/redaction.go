package secrets

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Redactor provides secure redaction of sensitive data in logs and outputs
type Redactor struct {
	patterns    []*regexp.Regexp
	replacement string
}

// NewRedactor creates a new redactor with default sensitive patterns
func NewRedactor() *Redactor {
	// Default patterns for common sensitive data (case-insensitive)
	defaultPatterns := []string{
		// Database connection strings
		`postgres://[^:]+:[^@]+@[^/]+/[^\s?"']+`,
		`mysql://[^:]+:[^@]+@[^/]+/[^\s?"']+`,
		`mongodb://[^:]+:[^@]+@[^/]+/[^\s?"']+`,
		
		// API keys and tokens
		`(?i)\b[a-z0-9]{20,}\b`, // Generic long alphanumeric strings
		`(?i)(?:api[_-]?key|token|secret|password|pwd)["\s]*[:=]["\s]*[^\s"',}]+`,
		`(?i)bearer\s+[a-zA-Z0-9\-\._~\+/]+=*`,
		`(?i)basic\s+[a-zA-Z0-9\+/]+=*`,
		
		// JWT tokens
		`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`,
		
		// Common cloud provider patterns
		`(?i)AKIA[0-9A-Z]{16}`, // AWS Access Key ID
		`(?i)[0-9a-zA-Z/\+]{40}`, // AWS Secret Access Key pattern
		`(?i)AIza[0-9A-Za-z\\-_]{35}`, // Google API Key
		`(?i)sk-[a-zA-Z0-9]{48}`, // OpenAI API Key
		
		// Private keys
		`-----BEGIN[A-Z\s]+PRIVATE KEY-----[\s\S]*?-----END[A-Z\s]+PRIVATE KEY-----`,
		
		// Credit card numbers (PCI compliance)
		`\b(?:\d{4}[-\s]?){3}\d{4}\b`,
		
		// Social security numbers
		`\b\d{3}-?\d{2}-?\d{4}\b`,
		
		// Phone numbers (basic patterns)
		`\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`,
		
		// Email addresses (when used as usernames in URLs)
		`(?i)[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
	}
	
	patterns := make([]*regexp.Regexp, len(defaultPatterns))
	for i, pattern := range defaultPatterns {
		patterns[i] = regexp.MustCompile(pattern)
	}
	
	return &Redactor{
		patterns:    patterns,
		replacement: "[REDACTED]",
	}
}

// AddPattern adds a custom redaction pattern
func (r *Redactor) AddPattern(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	r.patterns = append(r.patterns, compiled)
	return nil
}

// SetReplacement sets the replacement text for redacted content
func (r *Redactor) SetReplacement(replacement string) {
	r.replacement = replacement
}

// RedactString redacts sensitive data from a string
func (r *Redactor) RedactString(input string) string {
	result := input
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllString(result, r.replacement)
	}
	return result
}

// RedactBytes redacts sensitive data from bytes
func (r *Redactor) RedactBytes(input []byte) []byte {
	return []byte(r.RedactString(string(input)))
}

// RedactMap redacts sensitive data from a map[string]interface{}
func (r *Redactor) RedactMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range input {
		result[k] = r.redactValue(v)
	}
	return result
}

// RedactJSON redacts sensitive data from JSON
func (r *Redactor) RedactJSON(input []byte) ([]byte, error) {
	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		// If it's not valid JSON, treat as string
		return r.RedactBytes(input), nil
	}
	
	redacted := r.redactValue(data)
	return json.Marshal(redacted)
}

// redactValue recursively redacts values in nested structures
func (r *Redactor) redactValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return r.RedactString(v)
	case []byte:
		return r.RedactBytes(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			// Check if the key itself suggests sensitive content
			if r.isSensitiveKey(k) {
				result[k] = r.replacement
			} else {
				result[k] = r.redactValue(val)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = r.redactValue(val)
		}
		return result
	default:
		return value
	}
}

// isSensitiveKey checks if a key name suggests sensitive content
func (r *Redactor) isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"password", "pwd", "pass", "secret", "token", "key", "auth",
		"credential", "dsn", "connection_string", "private_key",
		"access_key", "secret_key", "api_key", "bearer", "authorization",
	}
	
	lowerKey := strings.ToLower(key)
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitiveKey) {
			return true
		}
	}
	return false
}

// SecureLogger wraps a logger to automatically redact sensitive data
type SecureLogger struct {
	redactor *Redactor
}

// NewSecureLogger creates a new secure logger with automatic redaction
func NewSecureLogger() *SecureLogger {
	return &SecureLogger{
		redactor: NewRedactor(),
	}
}

// RedactLogMessage redacts sensitive data from log messages
func (sl *SecureLogger) RedactLogMessage(message string, fields map[string]interface{}) (string, map[string]interface{}) {
	redactedMessage := sl.redactor.RedactString(message)
	redactedFields := sl.redactor.RedactMap(fields)
	return redactedMessage, redactedFields
}

// LogSecretAccess logs secret access attempts safely
func (sl *SecureLogger) LogSecretAccess(key string, provider string, success bool, duration int64) map[string]interface{} {
	return map[string]interface{}{
		"action":          "secret_access",
		"key":             sl.redactor.RedactString(key), // Redact the key name too
		"provider":        provider,
		"success":         success,
		"duration_ms":     duration,
		"timestamp":       time.Now().Format(time.RFC3339), // Timestamp for audit trail
	}
}

// ValidateSecretSafety checks if a string contains patterns that might be secrets
// This is useful for preventing accidental logging of secrets
func ValidateSecretSafety(input string) []string {
	r := NewRedactor()
	var violations []string
	
	for i, pattern := range r.patterns {
		if pattern.MatchString(input) {
			violations = append(violations, fmt.Sprintf("pattern_%d_matched", i))
		}
	}
	
	return violations
}

// SecretSafeString creates a string wrapper that prevents accidental logging
type SecretSafeString struct {
	value    string
	redactor *Redactor
}

// NewSecretSafeString creates a new secret-safe string
func NewSecretSafeString(value string) *SecretSafeString {
	return &SecretSafeString{
		value:    value,
		redactor: NewRedactor(),
	}
}

// String implements the Stringer interface with automatic redaction
func (sss *SecretSafeString) String() string {
	return sss.redactor.replacement
}

// Value returns the actual value (use carefully)
func (sss *SecretSafeString) Value() string {
	return sss.value
}

// MarshalJSON implements JSON marshaling with redaction
func (sss *SecretSafeString) MarshalJSON() ([]byte, error) {
	return json.Marshal(sss.redactor.replacement)
}

// GoString implements GoStringer interface with redaction
func (sss *SecretSafeString) GoString() string {
	return fmt.Sprintf("SecretSafeString{[REDACTED]}")
}