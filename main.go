package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	symbol := flag.String("symbol", "", "Trading pair symbol, e.g. BTCUSDT")
	interval := flag.String("interval", "", "Candle interval, e.g. 15m")
	startDate := flag.String("start", "", "Start time in RFC3339 or YYYY-MM-DD (default: 2026-01-01)")
	endDate := flag.String("end", "", "End time in RFC3339 or YYYY-MM-DD (default: 2026-04-18)")
	force := flag.Bool("force", false, "Overwrite candles.json when fetching from Binance")
	flag.Parse()

	_ = loadEnvFile(".env")

	dataDir := os.Getenv("DATA_DIR")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	// Create chart directory under tmp and clean old charts
	chartDir := filepath.Join("tmp", "chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		log.Fatalf("failed to create chart dir: %v", err)
	}

	// Remove old chart files
	entries, err := os.ReadDir(chartDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
				oldFile := filepath.Join(chartDir, entry.Name())
				if err := os.Remove(oldFile); err != nil {
					log.Printf("warning: could not remove %s: %v\n", oldFile, err)
				}
			}
		}
	}

	// Default date range: 2026-01-01 to 2026-04-18
	if *startDate == "" {
		*startDate = "2026-01-01"
	}
	if *endDate == "" {
		*endDate = "2026-04-18"
	}

	candles, err := LoadCandles(CandleRequestParams{
		Symbol:    *symbol,
		Interval:  *interval,
		StartTime: *startDate,
		EndTime:   *endDate,
		Overwrite: *force,
	}, filepath.Join(dataDir, func() string {
		f := os.Getenv("CANDLES_FILE")
		return f
	}()))
	if err != nil {
		log.Fatalf("Failed to load candles: %v", err)
	}

	fmt.Printf("Loaded %d candles\n", len(candles))

	if len(candles) < 50 {
		fmt.Println("Not enough candles (need at least 50)")
		return
	}

	// Sliding window analysis: 50 candles per window, shift by 1
	windowSize := 50
	patterns := 0

	fmt.Printf("Starting analysis: %d candles, window size %d, shift 1\n", len(candles), windowSize)

	for i := 0; i <= len(candles)-windowSize; i++ {
		window := candles[i : i+windowSize]
		result := DetectAscendingTriangle(window)

		if result.Found {
			patterns++
			timestamp := window[0].Timestamp
			dateStr := timestamp.Format("2006-01-02")

			chartName := fmt.Sprintf("chart_%s.html", dateStr)
			outputFile := filepath.Join(chartDir, chartName)

			renderer := NewEChartsRenderer()
			if err := RenderTriangleDetection(window, result, renderer, outputFile); err != nil {
				log.Printf("Error rendering chart for %s: %v\n", dateStr, err)
				continue
			}

			fmt.Printf("[Pattern #%d] %s | Resistance: %.2f | Support slope: %.4f\n",
				patterns, dateStr, result.ResistanceLevel, result.SupportSlope)
		}
	}

	fmt.Printf("\nAnalysis complete. Found %d ascending triangle pattern(s).\nCharts saved to: %s\n", patterns, chartDir)
}
