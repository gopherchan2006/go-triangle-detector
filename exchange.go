package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type exchangeSymbol struct {
	Symbol     string `json:"symbol"`
	Status     string `json:"status"`
	QuoteAsset string `json:"quoteAsset"`
}

type exchangeInfoResp struct {
	Symbols []exchangeSymbol `json:"symbols"`
}

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

	var info exchangeInfoResp
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
