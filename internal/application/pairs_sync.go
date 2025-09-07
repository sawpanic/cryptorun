package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	atomicio "cryptorun/internal/io"
)

type PairsSyncConfig struct {
	Venue  string
	Quote  string
	MinADV int64
}

type UniverseConfig struct {
	Venue      string   `json:"venue"`
	USDPairs   []string `json:"usd_pairs"`
	DoNotTrade []string `json:"do_not_trade"`
	SyncedAt   string   `json:"_synced_at"`
	Source     string   `json:"_source"`
	Note       string   `json:"_note"`
	Criteria   Criteria `json:"_criteria"`
	Hash       string   `json:"_hash"`
}

type Criteria struct {
	Quote     string `json:"quote"`
	MinADVUSD int64  `json:"min_adv_usd"`
}

type SyncReport struct {
	Found     int      `json:"found"`
	Kept      int      `json:"kept"`
	Dropped   int      `json:"dropped"`
	MinADVUSD int64    `json:"min_adv_usd"`
	Sample    []string `json:"sample"`
}

type KrakenAssetsResponse struct {
	Error  []string               `json:"error"`
	Result map[string]KrakenAsset `json:"result"`
}

type KrakenAsset struct {
	Altname         string `json:"altname"`
	AssetClass      string `json:"aclass"`
	DisplayDecimals int    `json:"display_decimals"`
}

type KrakenTradablePairsResponse struct {
	Error  []string                      `json:"error"`
	Result map[string]KrakenTradablePair `json:"result"`
}

type KrakenTradablePair struct {
	Altname         string `json:"altname"`
	WSName          string `json:"wsname"`
	AssetClassBase  string `json:"aclass_base"`
	Base            string `json:"base"`
	AssetClassQuote string `json:"aclass_quote"`
	Quote           string `json:"quote"`
	Status          string `json:"status"`
	OrderMin        string `json:"ordermin"`
}

type KrakenTickerResponse struct {
	Error  []string                    `json:"error"`
	Result map[string]KrakenTickerInfo `json:"result"`
}

type KrakenTickerInfo struct {
	Ask    []string `json:"a"` // [price, whole lot volume, lot volume]
	Bid    []string `json:"b"` // [price, whole lot volume, lot volume]
	Last   []string `json:"c"` // [price, lot volume]
	Volume []string `json:"v"` // [today, 24h]
	VWAP   []string `json:"p"` // [today, 24h]
	Trades []int    `json:"t"` // [today, 24h]
	Low    []string `json:"l"` // [today, 24h]
	High   []string `json:"h"` // [today, 24h]
	Open   string   `json:"o"` // today's opening price
}

type PairsSync struct {
	client      *http.Client
	config      PairsSyncConfig
	symbolRegex *regexp.Regexp
}

func NewPairsSync(config PairsSyncConfig) *PairsSync {
	// Regex for valid USD symbols: uppercase letters/numbers + USD suffix
	symbolRegex := regexp.MustCompile(`^[A-Z0-9]+USD$`)

	return &PairsSync{
		client:      &http.Client{Timeout: 30 * time.Second},
		config:      config,
		symbolRegex: symbolRegex,
	}
}

func (ps *PairsSync) SyncPairs(ctx context.Context) (*SyncReport, error) {
	if ps.config.Venue != "kraken" {
		return nil, fmt.Errorf("unsupported venue: %s", ps.config.Venue)
	}

	pairs, err := ps.fetchKrakenUSDPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pairs: %w", err)
	}

	tickers, err := ps.fetchKrakenTickers(ctx, pairs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tickers: %w", err)
	}

	normalizedPairs, err := ps.normalizePairs(pairs)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize pairs: %w", err)
	}

	// Validate normalized pairs using strict criteria
	validatedPairs := ps.validateNormalizedPairs(normalizedPairs)

	advResults := ps.calculateADVs(tickers, validatedPairs)

	validPairs := ps.FilterByADV(advResults, ps.config.MinADV)

	sort.Strings(validPairs)

	err = ps.WriteUniverseConfig(validPairs)
	if err != nil {
		return nil, fmt.Errorf("failed to write universe config: %w", err)
	}

	report := &SyncReport{
		Found:     len(pairs),
		Kept:      len(validPairs),
		Dropped:   len(pairs) - len(validPairs),
		MinADVUSD: ps.config.MinADV,
		Sample:    validPairs[:minInt(len(validPairs), 5)],
	}

	err = ps.writeReport(report)
	if err != nil {
		return nil, fmt.Errorf("failed to write report: %w", err)
	}

	return report, nil
}

func (ps *PairsSync) fetchKrakenUSDPairs(ctx context.Context) ([]string, error) {
	url := "https://api.kraken.com/0/public/AssetPairs"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := ps.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var krakenResp KrakenTradablePairsResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, err
	}

	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("kraken API error: %v", krakenResp.Error)
	}

	var usdPairs []string
	for pairID, pair := range krakenResp.Result {
		if ps.isValidUSDPair(pair) {
			usdPairs = append(usdPairs, pairID)
		}
	}

	return usdPairs, nil
}

func (ps *PairsSync) isValidUSDPair(pair KrakenTradablePair) bool {
	// Enforce Kraken USD cash spot pairs only
	if pair.Quote != "ZUSD" && pair.Quote != "USD" {
		return false
	}

	// Only online pairs
	if pair.Status != "online" {
		return false
	}

	// Reject test, dark pool, or derivative patterns
	lowerAltname := strings.ToLower(pair.Altname)
	if strings.Contains(lowerAltname, "test") ||
		strings.Contains(lowerAltname, ".d") ||
		strings.Contains(lowerAltname, "dark") ||
		strings.Contains(lowerAltname, "perp") ||
		strings.Contains(lowerAltname, "fut") {
		return false
	}

	// Only currency asset classes (no derivatives)
	if pair.AssetClassBase != "currency" || pair.AssetClassQuote != "currency" {
		return false
	}

	// Reject pairs that don't fit standard naming
	if len(pair.Altname) < 3 || len(pair.Altname) > 12 {
		return false
	}

	return true
}

func (ps *PairsSync) fetchKrakenTickers(ctx context.Context, pairs []string) (map[string]KrakenTickerInfo, error) {
	if len(pairs) == 0 {
		return make(map[string]KrakenTickerInfo), nil
	}

	url := "https://api.kraken.com/0/public/Ticker?pair=" + strings.Join(pairs, ",")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	resp, err := ps.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tickerResp KrakenTickerResponse
	if err := json.Unmarshal(body, &tickerResp); err != nil {
		return nil, err
	}

	if len(tickerResp.Error) > 0 {
		return nil, fmt.Errorf("kraken ticker API error: %v", tickerResp.Error)
	}

	return tickerResp.Result, nil
}

func (ps *PairsSync) normalizePairs(pairs []string) (map[string]string, error) {
	symbolsMap, err := ps.loadSymbolsMap()
	if err != nil {
		return nil, err
	}

	normalized := make(map[string]string)

	for _, pair := range pairs {
		baseCurrency := ps.extractBaseCurrency(pair)

		// Apply symbol normalization via symbol_map.json
		if mappedBase, exists := symbolsMap[baseCurrency]; exists {
			// Prevent USD duplication: only append if not already present
			if strings.HasSuffix(mappedBase, "USD") {
				normalized[pair] = mappedBase
			} else {
				normalized[pair] = mappedBase + "USD"
			}
		} else {
			// Default: use base currency as-is
			if strings.HasSuffix(baseCurrency, "USD") {
				normalized[pair] = baseCurrency
			} else {
				normalized[pair] = baseCurrency + "USD"
			}
		}
	}

	return normalized, nil
}

// validateNormalizedPairs applies strict validation to normalized pairs
func (ps *PairsSync) validateNormalizedPairs(normalizedPairs map[string]string) map[string]string {
	validated := make(map[string]string)

	for krakenPair, normalizedSymbol := range normalizedPairs {
		// Apply regex validation: must match ^[A-Z0-9]+USD$
		if !ps.symbolRegex.MatchString(normalizedSymbol) {
			continue // Skip malformed tickers
		}

		// Additional validation checks
		if ps.isValidNormalizedSymbol(normalizedSymbol) {
			validated[krakenPair] = normalizedSymbol
		}
	}

	return validated
}

// isValidNormalizedSymbol performs additional symbol validation
func (ps *PairsSync) isValidNormalizedSymbol(symbol string) bool {
	// Check minimum length (at least 4 chars: X + USD)
	if len(symbol) < 4 {
		return false
	}

	// Check for prohibited patterns
	lowerSymbol := strings.ToLower(symbol)
	prohibited := []string{"test", ".d", "dark", "perp", "fut"}
	for _, pattern := range prohibited {
		if strings.Contains(lowerSymbol, pattern) {
			return false
		}
	}

	return true
}

func (ps *PairsSync) extractBaseCurrency(pair string) string {
	// Handle Kraken's internal format first
	if strings.HasSuffix(pair, "ZUSD") {
		return strings.TrimSuffix(pair, "ZUSD")
	}
	if strings.HasSuffix(pair, "USD") {
		return strings.TrimSuffix(pair, "USD")
	}

	// For pairs like XXBTZUSD, XETHZUSD, handle the Z prefix
	basePart := pair
	if strings.Contains(pair, "Z") {
		parts := strings.Split(pair, "Z")
		if len(parts) >= 2 {
			basePart = parts[0] // Take everything before the first Z
		}
	}

	return basePart
}

func (ps *PairsSync) loadSymbolsMap() (map[string]string, error) {
	data, err := os.ReadFile("config/symbol_map.json")
	if err != nil {
		return nil, err
	}

	var symbolsMap map[string]string
	if err := json.Unmarshal(data, &symbolsMap); err != nil {
		return nil, err
	}

	return symbolsMap, nil
}

func (ps *PairsSync) calculateADVs(tickers map[string]KrakenTickerInfo, normalizedPairs map[string]string) []ADVResult {
	var results []ADVResult

	for pair, normalizedSymbol := range normalizedPairs {
		ticker, exists := tickers[pair]
		if !exists {
			continue
		}

		tickerData := ps.krakenTickerToTickerData(ticker, normalizedSymbol)
		result := CalculateADV(tickerData)
		if result.Valid {
			results = append(results, result)
		}
	}

	return results
}

func (ps *PairsSync) krakenTickerToTickerData(ticker KrakenTickerInfo, symbol string) TickerData {
	var volume24h, lastPrice float64

	if len(ticker.Volume) >= 2 {
		fmt.Sscanf(ticker.Volume[1], "%f", &volume24h)
	}

	if len(ticker.Last) >= 1 {
		fmt.Sscanf(ticker.Last[0], "%f", &lastPrice)
	}

	return TickerData{
		Symbol:        symbol,
		Volume24hBase: volume24h,
		LastPrice:     lastPrice,
		QuoteCurrency: "USD",
	}
}

func (ps *PairsSync) FilterByADV(advResults []ADVResult, minADV int64) []string {
	var validPairs []string

	for _, result := range advResults {
		if result.Valid && result.ADVUSD >= minADV {
			validPairs = append(validPairs, result.Symbol)
		}
	}

	return validPairs
}

func (ps *PairsSync) WriteUniverseConfig(pairs []string) error {
	// Sort pairs deterministically
	sortedPairs := make([]string, len(pairs))
	copy(sortedPairs, pairs)
	sort.Strings(sortedPairs)

	config := UniverseConfig{
		Venue:      "KRAKEN",
		USDPairs:   sortedPairs,
		DoNotTrade: []string{},
		SyncedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:     "kraken",
		Note:       "auto-generated; do not hand-edit",
		Criteria: Criteria{
			Quote:     ps.config.Quote,
			MinADVUSD: ps.config.MinADV,
		},
	}

	// Calculate hash of content (excluding _hash field)
	config.Hash = ps.calculateConfigHash(config)

	// Use atomic write helper
	return atomicio.WriteJSONAtomic("config/universe.json", config)
}

// calculateConfigHash computes SHA256 hash of config content (symbols + criteria only)
func (ps *PairsSync) calculateConfigHash(config UniverseConfig) string {
	// Create struct for hashing (only symbols and criteria)
	hashConfig := struct {
		Symbols  []string `json:"symbols"`
		Criteria Criteria `json:"criteria"`
	}{
		Symbols:  config.USDPairs,
		Criteria: config.Criteria,
	}

	data, err := json.Marshal(hashConfig)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (ps *PairsSync) writeReport(report *SyncReport) error {
	if err := os.MkdirAll("out/universe", 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("out/universe/report.json", data, 0644)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
