package guards

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache provides TTL-based caching with optional file backing
type Cache struct {
	entries    map[string]CacheEntry
	mutex      sync.RWMutex
	ttl        time.Duration
	fileBacked bool
	cachePath  string
	config     ProviderConfig
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Data         []byte      `json:"data"`
	StatusCode   int         `json:"status_code"`
	Headers      http.Header `json:"headers"`
	Timestamp    time.Time   `json:"timestamp"`
	ETag         string      `json:"etag,omitempty"`
	LastModified string      `json:"last_modified,omitempty"`
}

// NewCache creates a new cache with TTL and optional file backing
func NewCache(config ProviderConfig) *Cache {
	ttl := time.Duration(config.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 300 * time.Second // Default 5 minutes
	}

	cache := &Cache{
		entries:    make(map[string]CacheEntry),
		ttl:        ttl,
		fileBacked: config.EnableFileCache,
		cachePath:  config.CachePath,
		config:     config,
	}

	if cache.fileBacked {
		cache.loadFromFile()
		// Start background cleanup
		go cache.backgroundCleanup()
	}

	return cache
}

// GenerateCacheKey creates a deterministic cache key from request components
func (c *Cache) GenerateCacheKey(method, url string, headers map[string]string, body []byte) string {
	// Create fingerprint from all request components
	keyComponents := fmt.Sprintf("%s|%s", method, url)

	// Add relevant headers (excluding auth-related ones for security)
	relevantHeaders := []string{"Accept", "Content-Type", "If-None-Match", "If-Modified-Since"}
	for _, header := range relevantHeaders {
		if value, exists := headers[header]; exists {
			keyComponents += fmt.Sprintf("|%s:%s", header, value)
		}
	}

	// Add body hash if present
	if len(body) > 0 {
		bodyHash := md5.Sum(body)
		keyComponents += fmt.Sprintf("|body:%x", bodyHash)
	}

	// Add provider name for namespacing
	keyComponents += fmt.Sprintf("|provider:%s", c.config.Name)

	// Create MD5 hash of the key for fixed length
	hash := md5.Sum([]byte(keyComponents))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves an entry from cache if not expired
func (c *Cache) Get(key string) (CacheEntry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return CacheEntry{}, false
	}

	// Check TTL expiration
	if time.Since(entry.Timestamp) > c.ttl {
		// Entry expired - clean it up asynchronously
		go c.delete(key)
		return CacheEntry{}, false
	}

	return entry, true
}

// Set stores an entry in cache
func (c *Cache) Set(key string, entry CacheEntry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Extract ETag and Last-Modified for PIT integrity
	if entry.Headers != nil {
		entry.ETag = entry.Headers.Get("ETag")
		entry.LastModified = entry.Headers.Get("Last-Modified")
	}

	c.entries[key] = entry

	if c.fileBacked {
		go c.persistToFile()
	}
}

// delete removes an entry (internal method)
func (c *Cache) delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.entries = make(map[string]CacheEntry)

	if c.fileBacked {
		go c.persistToFile()
	}
}

// Size returns the number of cached entries
func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var totalSize int64
	var expiredCount int
	now := time.Now()

	for _, entry := range c.entries {
		totalSize += int64(len(entry.Data))
		if now.Sub(entry.Timestamp) > c.ttl {
			expiredCount++
		}
	}

	return CacheStats{
		EntryCount:     len(c.entries),
		ExpiredCount:   expiredCount,
		TotalSizeBytes: totalSize,
		TTL:            c.ttl,
		FileBacked:     c.fileBacked,
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	EntryCount     int           `json:"entry_count"`
	ExpiredCount   int           `json:"expired_count"`
	TotalSizeBytes int64         `json:"total_size_bytes"`
	TTL            time.Duration `json:"ttl"`
	FileBacked     bool          `json:"file_backed"`
}

// loadFromFile loads cache from disk if file-backed caching is enabled
func (c *Cache) loadFromFile() {
	if !c.fileBacked || c.cachePath == "" {
		return
	}

	// Ensure cache directory exists
	dir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return // Fail silently - file caching is optional
	}

	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return // File might not exist yet
	}

	var fileEntries map[string]CacheEntry
	if err := json.Unmarshal(data, &fileEntries); err != nil {
		return // Corrupted cache file - start fresh
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Load only non-expired entries
	now := time.Now()
	for key, entry := range fileEntries {
		if now.Sub(entry.Timestamp) <= c.ttl {
			c.entries[key] = entry
		}
	}
}

// persistToFile saves cache to disk if file-backed caching is enabled
func (c *Cache) persistToFile() {
	if !c.fileBacked || c.cachePath == "" {
		return
	}

	c.mutex.RLock()
	entriesCopy := make(map[string]CacheEntry, len(c.entries))
	for k, v := range c.entries {
		entriesCopy[k] = v
	}
	c.mutex.RUnlock()

	data, err := json.Marshal(entriesCopy)
	if err != nil {
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	// Write atomically via temp file
	tempPath := c.cachePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return
	}

	os.Rename(tempPath, c.cachePath)
}

// backgroundCleanup periodically removes expired entries
func (c *Cache) backgroundCleanup() {
	ticker := time.NewTicker(c.ttl / 4) // Clean every quarter TTL period
	defer ticker.Stop()

	for range ticker.C {
		c.cleanupExpired()
	}
}

// cleanupExpired removes expired entries from memory
func (c *Cache) cleanupExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	var expired []string

	for key, entry := range c.entries {
		if now.Sub(entry.Timestamp) > c.ttl {
			expired = append(expired, key)
		}
	}

	for _, key := range expired {
		delete(c.entries, key)
	}

	// Persist changes if file-backed
	if c.fileBacked && len(expired) > 0 {
		go c.persistToFile()
	}
}

// AddPITHeaders adds point-in-time headers to a request if cache entry exists
func (c *Cache) AddPITHeaders(key string, headers map[string]string) {
	entry, exists := c.Get(key)
	if !exists {
		return
	}

	// Add If-None-Match header with ETag
	if entry.ETag != "" {
		headers["If-None-Match"] = entry.ETag
	}

	// Add If-Modified-Since header
	if entry.LastModified != "" {
		headers["If-Modified-Since"] = entry.LastModified
	}
}
