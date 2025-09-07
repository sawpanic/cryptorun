package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with additional features for exchange APIs
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	timeout    time.Duration
}

// NewClient creates a new HTTP client for exchange APIs
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     60 * time.Second,
			},
		},
		baseURL:   baseURL,
		userAgent: "CryptoRun/3.2.1 DataFacade",
		timeout:   timeout,
	}
}

// Request represents an HTTP request
type Request struct {
	Method   string
	Endpoint string
	Headers  map[string]string
	Body     interface{}
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Do executes an HTTP request
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	url := c.baseURL + req.Endpoint
	
	var body io.Reader
	if req.Body != nil {
		jsonBody, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	
	// Set default headers
	httpReq.Header.Set("User-Agent", c.userAgent)
	httpReq.Header.Set("Accept", "application/json")
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	
	// Set custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	
	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer httpResp.Body.Close()
	
	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	
	// Collect response headers
	headers := make(map[string]string)
	for key, values := range httpResp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	
	response := &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    headers,
		Body:       respBody,
	}
	
	return response, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, endpoint string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, Request{
		Method:   "GET",
		Endpoint: endpoint,
		Headers:  headers,
	})
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, endpoint string, body interface{}, headers map[string]string) (*Response, error) {
	return c.Do(ctx, Request{
		Method:   "POST",
		Endpoint: endpoint,
		Headers:  headers,
		Body:     body,
	})
}

// GetJSON performs a GET request and unmarshals the JSON response
func (c *Client) GetJSON(ctx context.Context, endpoint string, result interface{}) error {
	resp, err := c.Get(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(resp.Body))
	}
	
	if err := json.Unmarshal(resp.Body, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	
	return nil
}

// PostJSON performs a POST request and unmarshals the JSON response
func (c *Client) PostJSON(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	resp, err := c.Post(ctx, endpoint, body, nil)
	if err != nil {
		return fmt.Errorf("post request: %w", err)
	}
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(resp.Body))
	}
	
	if err := json.Unmarshal(resp.Body, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	
	return nil
}

// WithRateLimitHeaders processes rate limit headers and returns them
func (c *Client) GetWithRateLimitHeaders(ctx context.Context, endpoint string) (*Response, map[string]string, error) {
	resp, err := c.Get(ctx, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("get request: %w", err)
	}
	
	// Extract rate limit related headers
	rateLimitHeaders := make(map[string]string)
	for key, value := range resp.Headers {
		switch key {
		case "X-MBX-USED-WEIGHT", "X-MBX-USED-WEIGHT-1M", "X-MBX-ORDER-COUNT-10S", "X-MBX-ORDER-COUNT-1M":
			rateLimitHeaders[key] = value
		case "ratelimit-limit", "ratelimit-remaining", "ratelimit-reset":
			rateLimitHeaders[key] = value
		case "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset":
			rateLimitHeaders[key] = value
		case "Retry-After":
			rateLimitHeaders[key] = value
		}
	}
	
	return resp, rateLimitHeaders, nil
}