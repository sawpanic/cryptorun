package artifacts

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

var (
	artifactsDir = "artifacts/ledger"
)

type AtomicWriter struct {
	BaseDir string
}

func NewAtomicWriter(baseDir string) *AtomicWriter {
	if baseDir == "" {
		baseDir = "artifacts/explain"
	}
	return &AtomicWriter{BaseDir: baseDir}
}

func (w *AtomicWriter) WriteExplainReport(report interface{}, prefix string) error {
	if err := w.ensureDir(); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")

	jsonFile := fmt.Sprintf("%s-%s-explain.json", timestamp, prefix)
	if err := w.writeJSONAtomic(jsonFile, report); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}

	csvFile := fmt.Sprintf("%s-%s-explain.csv", timestamp, prefix)
	if err := w.writeCSVFromStruct(csvFile, report); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}

	return nil
}

func (w *AtomicWriter) writeJSONAtomic(filename string, v interface{}) error {
	finalPath := filepath.Join(w.BaseDir, filename)
	tempPath := finalPath + ".tmp"

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("rename temp to final: %w", err)
	}

	return nil
}

func (w *AtomicWriter) writeCSVFromStruct(filename string, report interface{}) error {
	finalPath := filepath.Join(w.BaseDir, filename)
	tempPath := finalPath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	rows, err := w.extractCSVRows(report)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("extract CSV rows: %w", err)
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("write CSV row: %w", err)
		}
	}

	writer.Flush()
	file.Close()

	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("rename temp to final: %w", err)
	}

	return nil
}

func (w *AtomicWriter) extractCSVRows(report interface{}) ([][]string, error) {
	val := reflect.ValueOf(report)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %T", report)
	}

	universeField := val.FieldByName("Universe")
	if !universeField.IsValid() || universeField.Kind() != reflect.Slice {
		return nil, fmt.Errorf("Universe field not found or not slice")
	}

	var rows [][]string

	if universeField.Len() > 0 {
		header := w.generateCSVHeader()
		rows = append(rows, header)

		for i := 0; i < universeField.Len(); i++ {
			asset := universeField.Index(i)
			if asset.Kind() == reflect.Ptr {
				asset = asset.Elem()
			}

			csvRowMethod := asset.MethodByName("ToCSVRow")
			if csvRowMethod.IsValid() {
				results := csvRowMethod.Call(nil)
				if len(results) > 0 {
					csvRow := results[0].Interface()
					row := w.structToStringSlice(csvRow)
					rows = append(rows, row)
				}
			}
		}
	}

	return rows, nil
}

func (w *AtomicWriter) generateCSVHeader() []string {
	return []string{
		"symbol", "decision", "score", "rank", "momentum", "technical",
		"volume", "quality", "social", "entry_gate", "spread_bps",
		"depth_usd", "vadr", "heat_score", "regime", "exchange",
		"top_reason", "cache_hit_rate",
	}
}

func (w *AtomicWriter) structToStringSlice(s interface{}) []string {
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var result []string
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		result = append(result, w.valueToString(field))
	}
	return result
}

func (w *AtomicWriter) valueToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', 6, 64)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

func (w *AtomicWriter) ensureDir() error {
	return os.MkdirAll(w.BaseDir, 0755)
}

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
