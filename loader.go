package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type CandleRequestParams struct {
	Symbol    string
	Interval  string
	StartTime string
	EndTime   string
	Overwrite bool
}

func LoadCandles(params CandleRequestParams, filePath string) ([]Candle, error) {
	isEmptyCandles, err := isEmptyFile(filePath)
	if err != nil {
		return nil, err
	}

	wantFetch := false
	if params.Symbol != "" && params.Interval != "" {
		wantFetch = true
	}

	if isEmptyCandles && params.Symbol == "" && params.Interval == "" {
		params.Symbol = "BTCUSDT"
		params.Interval = "15m"

		wantFetch = true
	}

	if wantFetch {
		candles, err := fetchBinanceCandles(
			params.Symbol,
			params.Interval,
			params.StartTime,
			params.EndTime,
			0,
		)
		if err != nil {
			return nil, err
		}

		// Save fetched candles only if the file was empty or user requested overwrite.
		if isEmptyCandles || params.Overwrite {
			if err := saveJSONFile[Candle](filePath, candles); err != nil {
				return nil, err
			}
		} else {
			fmt.Printf("note: fetched %d candles but %s already contains data; not overwriting\n", len(candles), filePath)
		}

		return candles, nil
	}

	return readJSONFile[Candle](filePath)
}

func fetchBinanceCandles(
	symbol,
	interval,
	startStr,
	endStr string,
	limit int,
) ([]Candle, error) {
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

	intervalMs, err := intervalToMilliseconds(interval)
	if err != nil {
		return nil, err
	}

	allCandles := []Candle{}
	currentStart := startMs

	for {
		// Fetch up to 1000 candles per request (Binance API limit)
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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("binance returned %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read binance response: %w", err)
		}

		candles, err := parseKlines(body)
		if err != nil {
			return nil, err
		}

		if len(candles) == 0 {
			break
		}

		allCandles = append(allCandles, candles...)

		// Move to next batch (start after last candle)
		lastCandle := candles[len(candles)-1]
		currentStart = lastCandle.Timestamp.UnixMilli() + intervalMs

		// Stop if we've reached the end date or got fewer than 1000 (meaning we're at the end)
		if (endMs > 0 && currentStart >= endMs) || len(candles) < 1000 {
			break
		}
	}

	return allCandles, nil
}

func parseKlines(body []byte) ([]Candle, error) {
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse binance response: %w", err)
	}

	candles := make([]Candle, 0, len(raw))
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
		candles = append(candles, Candle{
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
