package api

// Minimal API client stubs for live scanning pipeline.

type Ticker struct {
	Price     float64
	Change24h float64
}

type PairInfo struct {
	Base  string
	Quote string
}

type CombinedPairData struct{}

type BasicClient struct{}

func (b *BasicClient) GetTicker(_ string) (Ticker, error) {
	return Ticker{Price: 0, Change24h: 0}, nil
}

func (b *BasicClient) GetTradingPairs() (map[string]PairInfo, error) {
	return map[string]PairInfo{}, nil
}

type ParallelClient struct{}

func NewParallelClient() *ParallelClient { return &ParallelClient{} }

func (p *ParallelClient) GetBasicClient() *BasicClient { return &BasicClient{} }

// ProcessInBatches invokes the callback once with an empty dataset so downstream
// code can proceed and rely on synthetic/generator paths for opportunities.
func (p *ParallelClient) ProcessInBatches(_ []string, _ int, _ int, fn func(map[string]CombinedPairData) error) error {
	return fn(map[string]CombinedPairData{})
}
