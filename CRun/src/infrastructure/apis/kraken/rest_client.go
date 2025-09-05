package kraken

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type RESTClient struct {
	HTTP *http.Client
	Limiter *rate.Limiter
}

func NewRESTClient() *RESTClient {
	return &RESTClient{
		HTTP: &http.Client{ Timeout: 10 * time.Second },
		Limiter: rate.NewLimiter(rate.Limit(15), 15), // 15 calls/sec
	}
}

func (c *RESTClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.Limiter.Wait(ctx); err != nil { return nil, err }
	return c.HTTP.Do(req)
}
