package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// APIError is returned when Binance responds with a non-200 status code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance API error %d: %s", e.StatusCode, e.Body)
}

// KlineReader fetches raw kline data from a market data source.
type KlineReader interface {
	// FetchKlines returns raw kline JSON for the given parameters.
	FetchKlines(ctx context.Context, symbol, interval string, startMs, endMs int64, limit int) ([]byte, error)
	// FetchLastKlines returns the most recent n+1 raw klines.
	FetchLastKlines(ctx context.Context, symbol, interval string, n int) ([]byte, error)
}

// BinanceReader implements KlineReader against the Binance REST API.
type BinanceReader struct {
	client   *http.Client
	baseURL  string
}

// NewBinanceReader creates a BinanceReader with a 15-second timeout.
func NewBinanceReader() *BinanceReader {
	return &BinanceReader{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://api.binance.com",
	}
}

func (r *BinanceReader) FetchKlines(ctx context.Context, symbol, interval string, startMs, endMs int64, limit int) ([]byte, error) {
	q := url.Values{
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(limit)},
	}
	if startMs > 0 {
		q.Set("startTime", strconv.FormatInt(startMs, 10))
	}
	if endMs > 0 {
		q.Set("endTime", strconv.FormatInt(endMs, 10))
	}
	return r.get(ctx, "/api/v3/klines?"+q.Encode())
}

func (r *BinanceReader) FetchLastKlines(ctx context.Context, symbol, interval string, n int) ([]byte, error) {
	q := url.Values{
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(n)},
	}
	return r.get(ctx, "/api/v3/klines?"+q.Encode())
}

func (r *BinanceReader) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("binance request failed: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}
