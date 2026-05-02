package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"triangle-detector/internal/domain"
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
	FetchKlines(ctx context.Context, symbol, interval string, startMs, endMs int64, limit int) ([]byte, error)
	FetchLastKlines(ctx context.Context, symbol, interval string, n int) ([]byte, error)
}

// Reader implements KlineReader against the Binance REST API.
type Reader struct {
	client  *http.Client
	baseURL string
}

// NewReader creates a Reader with a 15-second timeout.
func NewReader() *Reader {
	return &Reader{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://api.binance.com",
	}
}

func (r *Reader) FetchKlines(ctx context.Context, symbol, interval string, startMs, endMs int64, limit int) ([]byte, error) {
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

func (r *Reader) FetchLastKlines(ctx context.Context, symbol, interval string, n int) ([]byte, error) {
	q := url.Values{
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(n)},
	}
	return r.get(ctx, "/api/v3/klines?"+q.Encode())
}

func (r *Reader) get(ctx context.Context, path string) ([]byte, error) {
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

// CandleRequestParams holds parameters for a candle load request.
type CandleRequestParams struct {
	Symbol    string
	Interval  string
	StartTime string
	EndTime   string
}

// LoadCandles fetches candles for the given params using direct HTTP.
func LoadCandles(params CandleRequestParams) ([]domain.Candle, error) {
	if params.Interval == "" {
		params.Interval = "15m"
	}
	return fetchCandles(params.Symbol, params.Interval, params.StartTime, params.EndTime, 0)
}

func fetchCandles(symbol, interval, startStr, endStr string, limit int) ([]domain.Candle, error) {
	if symbol == "" {
		return nil, fmt.Errorf("binance symbol cannot be empty")
	}
	if interval == "" {
		interval = "15m"
	}

	var startMs, endMs int64
	if startStr != "" {
		t, err := parseTime(startStr)
		if err != nil {
			return nil, err
		}
		startMs = t.UnixMilli()
	}
	if endStr != "" {
		t, err := parseTime(endStr)
		if err != nil {
			return nil, err
		}
		endMs = t.UnixMilli()
	}
	if startMs > 0 && endMs > 0 && startMs >= endMs {
		return nil, fmt.Errorf("start time must be before end time")
	}

	intervalMs, err := IntervalToMilliseconds(interval)
	if err != nil {
		return nil, err
	}

	var allCandles []domain.Candle
	currentStart := startMs

	for {
		currentEnd := endMs
		if currentEnd == 0 || currentEnd-currentStart > intervalMs*1000 {
			currentEnd = currentStart + intervalMs*1000
		}

		query := url.Values{
			"symbol":   {symbol},
			"interval": {interval},
			"limit":    {"1000"},
		}
		if currentStart > 0 {
			query.Set("startTime", strconv.FormatInt(currentStart, 10))
		}
		if currentEnd > 0 {
			query.Set("endTime", strconv.FormatInt(currentEnd, 10))
		}
		endpoint := "https://api.binance.com/api/v3/klines?" + query.Encode()

		resp, err := http.Get(endpoint)
		if err != nil {
			return nil, fmt.Errorf("binance request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("binance returned %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read binance response: %w", err)
		}

		candles, err := ParseKlines(body)
		if err != nil {
			return nil, err
		}

		if len(candles) == 0 {
			break
		}

		allCandles = append(allCandles, candles...)

		lastCandle := candles[len(candles)-1]
		currentStart = lastCandle.Timestamp.UnixMilli() + intervalMs

		if (endMs > 0 && currentStart >= endMs) || len(candles) < 1000 {
			break
		}
	}

	return allCandles, nil
}

// LoadLastNCandles fetches the n most recent completed candles.
func LoadLastNCandles(symbol, interval string, n int) ([]domain.Candle, error) {
	if n <= 0 {
		n = 50
	}
	if n > 999 {
		n = 999
	}

	query := url.Values{
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(n + 1)},
	}
	endpoint := "https://api.binance.com/api/v3/klines?" + query.Encode()

	const maxRetries = 5
	retryDelays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelays[attempt-1])
		}

		resp, err := http.Get(endpoint)
		if err != nil {
			lastErr = fmt.Errorf("binance request failed: %w", err)
			if isNetworkError(err) {
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("binance returned %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode == 429 || resp.StatusCode >= 500 {
				continue
			}
			return nil, lastErr
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		candles, err := ParseKlines(body)
		if err != nil {
			return nil, err
		}

		if len(candles) > 0 {
			candles = candles[:len(candles)-1]
		}
		return candles, nil
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// ParseKlines parses raw Binance kline JSON into a slice of Candles.
func ParseKlines(body []byte) ([]domain.Candle, error) {
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse binance response: %w", err)
	}

	candles := make([]domain.Candle, 0, len(raw))
	for _, item := range raw {
		if len(item) < 6 {
			continue
		}
		openTime, ok := parseInt64(item[0])
		openPrice, ok1 := parseFloat(item[1])
		highPrice, ok2 := parseFloat(item[2])
		lowPrice, ok3 := parseFloat(item[3])
		closePrice, ok4 := parseFloat(item[4])
		volume, ok5 := parseFloat(item[5])
		if !ok || !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
			continue
		}
		candles = append(candles, domain.Candle{
			Open:      openPrice,
			High:      highPrice,
			Low:       lowPrice,
			Close:     closePrice,
			Volume:    volume,
			Timestamp: time.UnixMilli(openTime),
		})
	}
	return candles, nil
}

// FetchAllUSDTSymbols returns all actively-trading USDT spot pairs on Binance.
func FetchAllUSDTSymbols() ([]string, error) {
	resp, err := http.Get("https://api.binance.com/api/v3/exchangeInfo?permissions=SPOT")
	if err != nil {
		return nil, fmt.Errorf("exchangeInfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exchangeInfo returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading exchangeInfo response: %w", err)
	}

	var info struct {
		Symbols []struct {
			Symbol     string `json:"symbol"`
			Status     string `json:"status"`
			QuoteAsset string `json:"quoteAsset"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing exchangeInfo: %w", err)
	}

	var symbols []string
	for _, s := range info.Symbols {
		if s.Status == "TRADING" && s.QuoteAsset == "USDT" {
			symbols = append(symbols, s.Symbol)
		}
	}
	return symbols, nil
}
