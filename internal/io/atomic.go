package io

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteJSONAtomic writes JSON to file atomically using temp file + rename
func WriteJSONAtomic(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// WriteLinesAtomic writes lines to file atomically
func WriteLinesAtomic(path string, lines [][]byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, line := range lines {
		if _, err := file.Write(line); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return err
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// WriteFileAtomic writes data to file atomically
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// FanoutWrite writes data to multiple paths atomically
func FanoutWrite(paths []string, data []byte) error {
	for _, path := range paths {
		if err := WriteFileAtomic(path, data); err != nil {
			return err
		}
	}
	return nil
}
