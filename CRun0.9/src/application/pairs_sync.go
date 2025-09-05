package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type PairsSyncConfig struct {
	Venue    string
	Quote    string
	MinADV   int64
}

type UniverseConfig struct {
	Venue     string    `json:"venue"`
	USDPairs  []string  `json:"usd_pairs"`
	DoNotTrade []string  `json:"do_not_trade"`
	SyncedAt  string    `json:"_synced_at"`
	Source    string    `json:"_source"`
	Note      string    `json:"_note"`
	Criteria  Criteria  `json:"_criteria"`
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
	Error  []string           `json:"error"`
	Result map[string]KrakenAsset `json:"result"`
}

type KrakenAsset struct {
	Altname        string `json:"altname"`
	AssetClass     string `json:"aclass"`
	DisplayDecimals int   `json:"display_decimals"`
}

type KrakenTradablePairsResponse struct {
	Error  []string                       `json:"error"`
	Result map[string]KrakenTradablePair  `json:"result"`
}

type KrakenTradablePair struct {
	Altname      string   `json:"altname"`
	WSName       string   `json:"wsname"`
	AssetClassBase string `json:"aclass_base"`
	Base         string   `json:"base"`
	AssetClassQuote string `json:"aclass_quote"`
	Quote        string   `json:"quote"`
	Status       string   `json:"status"`
	OrderMin     string   `json:"ordermin"`
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
	client *http.Client
	config PairsSyncConfig
}

func NewPairsSync(config PairsSyncConfig) *PairsSync {
	return &PairsSync{
		client: &http.Client{Timeout: 30 * time.Second},
		config: config,
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

	advResults := ps.calculateADVs(tickers, normalizedPairs)
	
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
	if pair.Quote != "ZUSD" && pair.Quote != "USD" {
		return false
	}

	if pair.Status != "online" {
		return false
	}

	if strings.Contains(strings.ToLower(pair.Altname), "test") ||
	   strings.Contains(strings.ToLower(pair.Altname), ".d") ||
	   strings.Contains(strings.ToLower(pair.Altname), "dark") {
		return false
	}

	if pair.AssetClassBase != "currency" || pair.AssetClassQuote != "currency" {
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
		
		if mappedBase, exists := symbolsMap["kraken"][baseCurrency]; exists {
			normalized[pair] = mappedBase + "USD"
		} else if baseCurrency == "XBT" {
			symbolsMap["kraken"]["XBT"] = "BTC"
			normalized[pair] = "BTCUSD"
		} else {
			normalized[pair] = baseCurrency + "USD"
		}
	}

	err = ps.saveSymbolsMap(symbolsMap)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

func (ps *PairsSync) extractBaseCurrency(pair string) string {
	if strings.HasSuffix(pair, "USD") {
		return strings.TrimSuffix(pair, "USD")
	}
	if strings.HasSuffix(pair, "ZUSD") {
		return strings.TrimSuffix(pair, "ZUSD")
	}
	return pair
}

func (ps *PairsSync) loadSymbolsMap() (map[string]map[string]string, error) {
	data, err := os.ReadFile("config/symbols_map.json")
	if err != nil {
		return nil, err
	}

	var symbolsMap map[string]map[string]string
	if err := json.Unmarshal(data, &symbolsMap); err != nil {
		return nil, err
	}

	if symbolsMap["kraken"] == nil {
		symbolsMap["kraken"] = make(map[string]string)
	}

	return symbolsMap, nil
}

func (ps *PairsSync) saveSymbolsMap(symbolsMap map[string]map[string]string) error {
	data, err := json.MarshalIndent(symbolsMap, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := "config/symbols_map.json.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, "config/symbols_map.json")
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
		Symbol:         symbol,
		Volume24hBase:  volume24h,
		LastPrice:      lastPrice,
		QuoteCurrency:  "USD",
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
	config := UniverseConfig{
		Venue:      "KRAKEN",
		USDPairs:   pairs,
		DoNotTrade: []string{},
		SyncedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:     "kraken",
		Note:       "auto-generated; do not hand-edit",
		Criteria: Criteria{
			Quote:     ps.config.Quote,
			MinADVUSD: ps.config.MinADV,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := "config/universe.json.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, "config/universe.json")
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