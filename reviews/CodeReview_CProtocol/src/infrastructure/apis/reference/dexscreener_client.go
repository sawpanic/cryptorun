package reference

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type DEXScreenerClient struct {
	HTTP *http.Client
	Limiter *rate.Limiter // 60 rpm
}

func NewDEXScreener() *DEXScreenerClient {
	return &DEXScreenerClient{ HTTP: &http.Client{ Timeout: 10*time.Second }, Limiter: rate.NewLimiter(rate.Every(time.Minute/60), 1) }
}

func (c *DEXScreenerClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.Limiter.Wait(ctx); err != nil { return nil, err }
	return c.HTTP.Do(req)
}
