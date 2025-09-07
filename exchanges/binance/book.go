package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type OrderbookMetrics struct {
	SpreadBps    float64
	DepthUSD2pc  float64
	LatencyP99Ms int64
	Source       string
}

type BookProvider interface {
	Metrics(ctx context.Context, symbol string) (OrderbookMetrics, error)
}

type provider struct {
	mu    sync.Mutex
	books map[string]*book
}

func NewBookProvider() *provider { return &provider{books: make(map[string]*book)} }

func (p *provider) Metrics(ctx context.Context, symbol string) (OrderbookMetrics, error) {
	sym := strings.ToLower(symbol)
	p.mu.Lock()
	b, ok := p.books[sym]
	if !ok {
		b = newBook(sym)
		p.books[sym] = b
		go b.run()
	}
	p.mu.Unlock()
	return b.metrics(), nil
}

type level struct{ P, Q float64 }
type book struct {
	sym    string
	mu     sync.RWMutex
	bids   []level
	asks   []level
	lat    []int64 // ring buffer of ms latencies
	latIdx int
}

func newBook(sym string) *book { return &book{sym: sym, lat: make([]int64, 64)} }

func (b *book) run() {
	// initial snapshot
	b.refreshSnapshot()
	// ws diffs
	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@depth@100ms", b.sym)
	for {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		_ = c.SetReadDeadline(time.Now().Add(30 * time.Second))
		c.SetPongHandler(func(string) error { _ = c.SetReadDeadline(time.Now().Add(30 * time.Second)); return nil })
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				_ = c.Close()
				break
			}
			t0 := time.Now()
			var raw struct {
				Bids [][]string `json:"b"`
				Asks [][]string `json:"a"`
			}
			if err := json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			m := struct{ Bids, Asks [][]string }{Bids: raw.Bids, Asks: raw.Asks}
			b.applyDiff(m)
			b.recordLatency(time.Since(t0))
		}
		time.Sleep(1 * time.Second)
	}
}

func (b *book) refreshSnapshot() {
	api := fmt.Sprintf("https://api.binance.com/api/v3/depth?symbol=%s&limit=500", strings.ToUpper(b.sym))
	resp, err := http.Get(api)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var out struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bids = parseLevels(out.Bids)
	b.asks = parseLevels(out.Asks)
}

func parseLevels(x [][]string) []level {
	out := make([]level, 0, len(x))
	for _, e := range x {
		if len(e) < 2 {
			continue
		}
		var p, q float64
		fmt.Sscan(e[0], &p)
		fmt.Sscan(e[1], &q)
		if q <= 0 {
			continue
		}
		out = append(out, level{P: p, Q: q})
	}
	// sort: bids desc, asks asc handled by caller
	return out
}

func (b *book) applyDiff(m struct{ Bids, Asks [][]string }) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// naive replace for slice; acceptable for slice demo
	if len(m.Bids) > 0 {
		b.bids = parseLevels(m.Bids)
	}
	if len(m.Asks) > 0 {
		b.asks = parseLevels(m.Asks)
	}
}

func (b *book) recordLatency(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lat[b.latIdx%len(b.lat)] = d.Milliseconds()
	b.latIdx++
}

func (b *book) metrics() OrderbookMetrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.bids) == 0 || len(b.asks) == 0 {
		return OrderbookMetrics{Source: "binance"}
	}
	// best bid/ask
	bestBid := b.bids[0].P
	bestAsk := b.asks[0].P
	mid := (bestBid + bestAsk) / 2
	spreadBps := 0.0
	if mid > 0 {
		spreadBps = (bestAsk - bestBid) / mid * 10000
	}
	// depth within +/-2%
	low := mid * 0.98
	high := mid * 1.02
	depthUSD := 0.0
	// bids (<= mid, >= low)
	for _, lv := range b.bids {
		if lv.P < low {
			break
		}
		depthUSD += lv.P * lv.Q
	}
	// asks (>= mid, <= high)
	for _, lv := range b.asks {
		if lv.P > high {
			break
		}
		depthUSD += lv.P * lv.Q
	}
	// p99 latency
	lat := append([]int64(nil), b.lat...)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p99 := int64(0)
	if n := len(lat); n > 0 {
		p99 = lat[int(math.Ceil(float64(n)*0.99))-1]
	}
	return OrderbookMetrics{SpreadBps: spreadBps, DepthUSD2pc: depthUSD, LatencyP99Ms: p99, Source: "binance"}
}
