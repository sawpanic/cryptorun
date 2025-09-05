package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Snapshot struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"ts"`
	Bid         float64   `json:"bid"`
	Ask         float64   `json:"ask"`
	SpreadBps   float64   `json:"spread_bps"`
	Depth2PcUSD float64   `json:"depth2pc_usd"`
	VADR        float64   `json:"vadr"`
	ADVUSD      int64     `json:"adv_usd"`
}

type SnapshotWriter struct {
	baseDir string
}

func NewSnapshotWriter(baseDir string) *SnapshotWriter {
	if baseDir == "" {
		baseDir = "out/microstructure/snapshots"
	}
	return &SnapshotWriter{baseDir: baseDir}
}

func (sw *SnapshotWriter) SaveSnapshot(snapshot Snapshot) error {
	if err := os.MkdirAll(sw.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshots directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%d.json", 
		snapshot.Symbol,
		snapshot.Timestamp.Unix())
	
	filePath := filepath.Join(sw.baseDir, filename)
	tmpPath := filePath + ".tmp"

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary snapshot file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to finalize snapshot file: %w", err)
	}

	return nil
}

func NewSnapshot(symbol string, bid, ask, spreadBps, depth2PcUSD, vadr float64, advUSD int64) Snapshot {
	return Snapshot{
		Symbol:      symbol,
		Timestamp:   time.Now().UTC(),
		Bid:         roundToDecimals(bid, 4),
		Ask:         roundToDecimals(ask, 4),
		SpreadBps:   roundToDecimals(spreadBps, 2),
		Depth2PcUSD: roundToDecimals(depth2PcUSD, 0),
		VADR:        roundToDecimals(vadr, 3),
		ADVUSD:      advUSD,
	}
}

func roundToDecimals(value float64, decimals int) float64 {
	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10.0
	}
	return float64(int64(value*multiplier+0.5)) / multiplier
}

func LoadSnapshot(filePath string) (*Snapshot, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

func ListSnapshots(baseDir, symbol string) ([]string, error) {
	if baseDir == "" {
		baseDir = "out/microstructure/snapshots"
	}

	pattern := filepath.Join(baseDir, symbol+"-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	return matches, nil
}