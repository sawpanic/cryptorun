package artifacts

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var (
	artifactsDir = "artifacts/ledger"
)

func WriteJSON(name string, v interface{}) error {
	if err := ensureDir(); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.json", timestamp, name)
	path := filepath.Join(artifactsDir, filename)

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	return nil
}

func WriteCSV(name string, rows [][]string) error {
	if err := ensureDir(); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.csv", timestamp, name)
	path := filepath.Join(artifactsDir, filename)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	return nil
}

func ensureDir() error {
	return os.MkdirAll(artifactsDir, 0755)
}
