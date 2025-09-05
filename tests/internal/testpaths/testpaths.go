package testpaths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RepoRoot walks up from runtime.Caller until go.mod found
func RepoRoot() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("unable to determine caller location")
	}
	
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

// PathFromRoot returns absolute path from repo root
func PathFromRoot(rel string) string {
	root := RepoRoot()
	path := filepath.Join(root, filepath.FromSlash(rel))
	return filepath.Clean(path)
}

// MustRead reads file content or panics
func MustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read %s: %v", path, err))
	}
	return data
}

// MustWriteFileAtomic writes file atomically or panics
func MustWriteFileAtomic(path string, data []byte) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create dir %s: %v", dir, err))
	}
	
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		panic(fmt.Sprintf("failed to write %s: %v", tmpPath, err))
	}
	
	if err := os.Rename(tmpPath, path); err != nil {
		panic(fmt.Sprintf("failed to rename %s to %s: %v", tmpPath, path, err))
	}
}

// CleanPath converts filepath to use forward slashes for cross-platform compatibility
func CleanPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// NormalizePath converts relative paths to absolute and normalizes separators
func NormalizePath(path string) string {
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			panic(fmt.Sprintf("failed to get absolute path for %s: %v", path, err))
		}
		path = abs
	}
	return filepath.Clean(path)
}

// TempConfigDir creates a temporary directory for config files during tests
func TempConfigDir(prefix string) string {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		panic(fmt.Sprintf("failed to create temp dir: %v", err))
	}
	return dir
}