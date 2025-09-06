package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sawpanic/CProtocol/data/cache"
)

type Prices struct{ c cache.Cache }

func NewPrices() *Prices { return &Prices{c: cache.NewAuto()} }

// Klines returns close and volume arrays for a venue/symbol/window. Uses Binance REST.
func (p *Prices) Klines(ctx context.Context, venue, symbol, window string, limit int) ([]float64, []float64, error) {
	if strings.ToLower(venue) != "binance" {
		return nil, nil, fmt.Errorf("only binance supported in slice")
	}
	inter := map[string]string{"1h": "1h", "4h": "4h", "12h": "12h", "24h": "1d"}[window]
	if inter == "" {
		inter = "4h"
	}
	if limit <= 0 {
		limit = 200
	}
	key := fmt.Sprintf("kl:%s:%s:%s:%d", venue, symbol, inter, limit)
	if b, ok := p.c.Get(key); ok {
		var out struct {
			C []float64
			V []float64
		}
		if json.Unmarshal(b, &out) == nil {
			return out.C, out.V, nil
		}
	}
	api := "https://api.binance.com/api/v3/klines"
	q := url.Values{}
	q.Set("symbol", strings.ToUpper(symbol))
	q.Set("interval", inter)
	q.Set("limit", fmt.Sprintf("%d", limit))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, api+"?"+q.Encode(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	var rows [][]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, nil, err
	}
	close := make([]float64, 0, len(rows))
	vols := make([]float64, 0, len(rows))
	for _, r := range rows {
		if len(r) < 7 {
			continue
		}
		var c, v float64
		fmt.Sscan(fmt.Sprintf("%v", r[4]), &c)
		fmt.Sscan(fmt.Sprintf("%v", r[5]), &v)
		close = append(close, c)
		vols = append(vols, v)
	}
	b, _ := json.Marshal(struct {
		C []float64
		V []float64
	}{C: close, V: vols})
	p.c.Set(key, b, 30*time.Second)
	return close, vols, nil
}
