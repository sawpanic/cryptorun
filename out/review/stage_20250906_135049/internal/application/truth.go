package application

import "time"

// LedgerEntry represents a single row in the tracking ledger
type LedgerEntry struct {
	TsScan       time.Time                `json:"ts_scan"`
	Symbol       string                   `json:"symbol"`
	Composite    float64                  `json:"composite"`
	GatesAllPass bool                     `json:"gates_all_pass"`
	Horizons     map[string]time.Time     `json:"horizons"`
	Realized     map[string]*float64      `json:"realized"`
	Pass         map[string]*bool         `json:"pass"`
}