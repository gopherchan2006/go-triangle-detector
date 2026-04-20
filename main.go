package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	symbol := flag.String("symbol", "", "Trading pair symbol, e.g. BTCUSDT")
	interval := flag.String("interval", "", "Candle interval, e.g. 15m")
	startDate := flag.String("start", "", "Start time in RFC3339 or YYYY-MM-DD (default: 2026-01-01)")
	endDate := flag.String("end", "", "End time in RFC3339 or YYYY-MM-DD (default: 2026-04-18)")
	flag.Parse()

	_ = loadEnvFile(".env")

	if *startDate == "" {
		*startDate = "2026-01-01"
	}
	if *endDate == "" {
		*endDate = "2026-04-18"
	}
	if *interval == "" {
		*interval = "15m"
	}

	dataDir := os.Getenv("DATA_DIR")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	var symbols []string
	if *symbol != "" {
		symbols = []string{*symbol}
	} else {
		raw := os.Getenv("SYMBOLS")
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
		if len(symbols) == 0 {
			symbols = []string{"BTCUSDT"}
		}
	}

	for _, sym := range symbols {
		analyzeSymbol(sym, *interval, *startDate, *endDate, dataDir)
	}
}

func analyzeSymbol(symbol, interval, startDate, endDate string, dataDir string) {
	chartDir := filepath.Join("tmp", symbol+"_chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		log.Printf("[%s] failed to create chart dir: %v", symbol, err)
		return
	}

	entries, err := os.ReadDir(chartDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
				_ = os.Remove(filepath.Join(chartDir, entry.Name()))
			}
		}
	}

	candles, err := LoadCandles(CandleRequestParams{
		Symbol:    symbol,
		Interval:  interval,
		StartTime: startDate,
		EndTime:   endDate,
	})
	if err != nil {
		log.Printf("[%s] failed to load candles: %v", symbol, err)
		return
	}

	fmt.Printf("\n[%s] Loaded %d candles\n", symbol, len(candles))

	if len(candles) < 50 {
		fmt.Printf("[%s] not enough candles (need at least 50)\n", symbol)
		return
	}

	windowSize := 50
	patterns := 0
	rejectStats := make(map[string]*int)

	for i := 0; i <= len(candles)-windowSize; i++ {
		window := candles[i : i+windowSize]
		result := detectAscendingTriangleDiag(window, rejectStats)

		if result.Found {
			patterns++
			timestamp := window[0].Timestamp
			dateStr := timestamp.Format("2006-01-02")

			chartName := fmt.Sprintf("chart_%s.html", dateStr)
			outputFile := filepath.Join(chartDir, chartName)

			renderer := NewEChartsRenderer()
			if err := RenderTriangleDetection(window, result, renderer, outputFile); err != nil {
				log.Printf("[%s] error rendering chart for %s: %v", symbol, dateStr, err)
				continue
			}

			fmt.Printf("[%s] [Pattern #%d] %s | Resistance: %.2f | Support slope: %.4f\n",
				symbol, patterns, dateStr, result.ResistanceLevel, result.SupportSlope)
		}
	}

	fmt.Printf("[%s] Analysis complete. Found %d pattern(s). Charts saved to: %s\n", symbol, patterns, chartDir)

	fmt.Printf("[%s] --- Reject reasons ---\n", symbol)
	for reason, count := range rejectStats {
		fmt.Printf("[%s]   %-35s %d\n", symbol, reason, *count)
	}
}
