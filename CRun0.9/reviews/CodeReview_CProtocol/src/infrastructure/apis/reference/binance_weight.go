package reference

import (
	"context"
	"net/http"
	"strconv"
)

type BinanceClient struct{ HTTP *http.Client }

func (b *BinanceClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	resp, err := b.HTTP.Do(req)
	if err != nil { return nil, err }
	if w := resp.Header.Get("X-MBX-USED-WEIGHT"); w != "" {
		if _, err := strconv.Atoi(w); err == nil {
			// reference-only: could adapt pacing
		}
	}
	return resp, nil
}
