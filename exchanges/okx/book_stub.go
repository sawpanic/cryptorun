package okx

import "context"

type OrderbookMetrics struct{ SpreadBps float64; DepthUSD2pc float64; LatencyP99Ms int64; Source string }
type BookProvider interface{ Metrics(context.Context, string) (OrderbookMetrics, error) }

// TODO: Implement OKX WS orderbook provider
type provider struct{}
func NewBookProvider() *provider { return &provider{} }
func (p *provider) Metrics(ctx context.Context, symbol string) (OrderbookMetrics, error) {
    return OrderbookMetrics{Source: "okx"}, nil
}

