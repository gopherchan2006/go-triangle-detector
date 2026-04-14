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
	startDate := flag.String("start", "", "Start time in RFC3339 or YYYY-MM-DD")
	endDate := flag.String("end", "", "End time in RFC3339 or YYYY-MM-DD")
	force := flag.Bool("force", false, "Overwrite candles.json when fetching from Binance")
	flag.Parse()

	_ = loadEnvFile(".env")

	dataDir := os.Getenv("DATA_DIR")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
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

	if len(candles) > 50 {
		candles = candles[len(candles)-50:]
		fmt.Println("Working with the last 50 candles.")
	}

	result := DetectAscendingTriangle(candles)

	renderer := NewEChartsRenderer()

	chartName := os.Getenv("CHART_FILE")
	outputFile := filepath.Join(dataDir, chartName)
	if err := RenderTriangleDetection(candles, result, renderer, outputFile); err != nil {
		log.Fatalf("Rendering error: %v", err)
	}

	fmt.Printf("\nChart saved: %s\n", outputFile)
	fmt.Println("Open the file in your browser to view.")
}
