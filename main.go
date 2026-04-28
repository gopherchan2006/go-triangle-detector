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
	realtimeMode := flag.Bool("realtime", false, "Run real-time scanning on all active USDT pairs")
	workers := flag.Int("workers", 20, "Concurrent workers for real-time mode")
	noScreenshots := flag.Bool("no-screenshots", false, "Disable screenshots in real-time mode")
	flag.Parse()

	_ = loadEnvFile(".env")

	if *interval == "" {
		*interval = "15m"
	}

	if *realtimeMode {
		needBrowser := !*noScreenshots
		var ss *Screenshotter
		if needBrowser {
			var err error
			ss, err = NewScreenshotter()
			if err != nil {
				log.Fatalf("failed to start browser: %v", err)
			}
			defer ss.Close()
		}

		cfg := RealtimeConfig{
			Interval:        *interval,
			Workers:         *workers,
			WindowSize:      50,
			OutputDir:       filepath.Join("tmp", "realtime"),
			WithScreenshots: needBrowser,
		}
		RunRealtime(cfg, ss)
		return
	}

	if *startDate == "" {
		*startDate = "2026-01-01"
	}
	if *endDate == "" {
		*endDate = "2026-04-18"
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

	ss, err := NewScreenshotter()
	if err != nil {
		log.Fatalf("failed to start browser: %v", err)
	}
	defer ss.Close()

	for _, sym := range symbols {
		analyzeSymbol(sym, *interval, *startDate, *endDate, dataDir, ss)
	}
}

func analyzeSymbol(symbol, interval, startDate, endDate string, dataDir string, ss *Screenshotter) {
	chartDir := filepath.Join("tmp", symbol+"_chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		log.Printf("[%s] failed to create chart dir: %v", symbol, err)
		return
	}

	entries, err := os.ReadDir(chartDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
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
		result := DetectAscendingTriangle(window, rejectStats)

		if result.Found {
			patterns++
			timestamp := window[0].Timestamp
			dateStr := timestamp.Format("2006-01-02")

			htmlTmp := filepath.Join(chartDir, fmt.Sprintf("%s_%s.tmp.html", symbol, dateStr))
			pngFile := filepath.Join(chartDir, fmt.Sprintf("%s_%s.png", symbol, dateStr))

			renderer := NewEChartsRenderer()
			if err := RenderTriangleDetection(window, result, renderer, htmlTmp); err != nil {
				log.Printf("[%s] error rendering chart for %s: %v", symbol, dateStr, err)
				_ = os.Remove(htmlTmp)
				continue
			}
			if err := ss.Screenshot(htmlTmp, pngFile); err != nil {
				log.Printf("[%s] error taking screenshot for %s: %v", symbol, dateStr, err)
			}
			_ = os.Remove(htmlTmp)

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
