package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RealtimeConfig struct {
	Interval        string
	IntervalDur     time.Duration
	Workers         int
	WindowSize      int
	OutputDir       string
	WithScreenshots bool
}

type scanResult struct {
	symbol  string
	candles []Candle
	result  AscendingTriangleResult
}

type alertState struct {
	resistance float64
	alertedAt  time.Time
}

func RunRealtime(cfg RealtimeConfig, ss *Screenshotter) {
	fmt.Println("[realtime] Fetching all active USDT spot pairs from Binance...")
	symbols, err := FetchAllUSDTSymbols()
	if err != nil {
		log.Fatalf("[realtime] failed to fetch symbols: %v", err)
	}
	fmt.Printf("[realtime] Found %d USDT symbols\n", len(symbols))

	intervalMs, err := intervalToMilliseconds(cfg.Interval)
	if err != nil {
		log.Fatalf("[realtime] invalid interval %q: %v", cfg.Interval, err)
	}
	cfg.IntervalDur = time.Duration(intervalMs) * time.Millisecond

	if cfg.WithScreenshots && cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
			log.Fatalf("[realtime] failed to create output dir: %v", err)
		}
	}

	alerts := make(map[string]alertState)
	symbolsFetchedAt := time.Now()

	runCycle(0, symbols, cfg, alerts, ss)

	for cycle := 1; ; cycle++ {
		if time.Since(symbolsFetchedAt) > 24*time.Hour {
			if fresh, err := FetchAllUSDTSymbols(); err == nil {
				symbols = fresh
				symbolsFetchedAt = time.Now()
				fmt.Printf("[realtime] Symbol list refreshed: %d USDT pairs\n", len(symbols))
			} else {
				log.Printf("[realtime] symbol refresh failed (using old list): %v", err)
			}
		}

		next := nextCandleClose(time.Now().UTC(), cfg.IntervalDur)
		waitDur := time.Until(next)
		fmt.Printf("[realtime] Next scan at %s UTC (in %s)\n",
			next.Format("2006-01-02 15:04:05"), waitDur.Round(time.Second))
		time.Sleep(waitDur)

		runCycle(cycle, symbols, cfg, alerts, ss)
	}
}

func runCycle(cycle int, symbols []string, cfg RealtimeConfig, alerts map[string]alertState, ss *Screenshotter) {
	start := time.Now()
	label := "initial"
	if cycle > 0 {
		label = fmt.Sprintf("%d", cycle)
	}
	fmt.Printf("[realtime] === Cycle %s | %s UTC | scanning %d pairs ===\n",
		label, start.UTC().Format("15:04:05"), len(symbols))

	results := scanAllSymbols(symbols, cfg)

	newAlerts := 0
	totalFound := 0
	for _, r := range results {
		if !r.result.Found {
			continue
		}
		totalFound++

		last, seen := alerts[r.symbol]
		resistanceChanged := seen && last.resistance > 0 &&
			math.Abs(r.result.ResistanceLevel-last.resistance)/last.resistance > 0.01
		isNew := !seen || resistanceChanged || time.Since(last.alertedAt) > 4*cfg.IntervalDur

		if !isNew {
			continue
		}
		alerts[r.symbol] = alertState{
			resistance: r.result.ResistanceLevel,
			alertedAt:  time.Now(),
		}
		newAlerts++

		fmt.Printf("[%s] *** %s *** ASCENDING TRIANGLE | Resistance: %.4f | Support slope: %+.6f | Touches: %d\n",
			time.Now().UTC().Format("15:04:05"),
			r.symbol,
			r.result.ResistanceLevel,
			r.result.SupportSlope,
			r.result.ResistanceTouches,
		)

		if cfg.WithScreenshots && ss != nil {
			takeRealtimeScreenshot(r, cfg, ss)
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("[realtime] Cycle %s done | scanned: %d | patterns: %d | new alerts: %d | %.1fs\n",
		label, len(symbols), totalFound, newAlerts, elapsed.Seconds())
}

func scanAllSymbols(symbols []string, cfg RealtimeConfig) []scanResult {
	jobs := make(chan string, len(symbols))
	resultCh := make(chan scanResult, len(symbols))

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				candles, err := LoadLastNCandles(sym, cfg.Interval, cfg.WindowSize)
				if err != nil {
					log.Printf("[realtime] [%s] fetch error: %v", sym, err)
					resultCh <- scanResult{symbol: sym}
					continue
				}
				if len(candles) < cfg.WindowSize {
					resultCh <- scanResult{symbol: sym}
					continue
				}
				window := candles[len(candles)-cfg.WindowSize:]
				det := DetectAscendingTriangle(window, WithTrace(false))
				resultCh <- scanResult{symbol: sym, candles: window, result: det}
			}
		}()
	}

	for _, sym := range symbols {
		jobs <- sym
	}
	close(jobs)

	wg.Wait()
	close(resultCh)

	var out []scanResult
	for r := range resultCh {
		out = append(out, r)
	}
	return out
}

func takeRealtimeScreenshot(r scanResult, cfg RealtimeConfig, ss *Screenshotter) {
	ts := time.Now().UTC().Format("20060102_1504")
	pairDir := filepath.Join(cfg.OutputDir, r.symbol)
	if err := os.MkdirAll(pairDir, 0o755); err != nil {
		log.Printf("[realtime] [%s] failed to create dir: %v", r.symbol, err)
		return
	}
	stem := fmt.Sprintf("%s_%s", r.symbol, ts)
	artNames := NewArtifactNames(pairDir, stem)
	if err := os.MkdirAll(artNames.GroupDir, 0o755); err != nil {
		log.Printf("[realtime] [%s] failed to create group dir: %v", r.symbol, err)
		return
	}
	renderer := NewEChartsRenderer()
	renderer.SetCaption(r.symbol, time.Now().UTC())
	if err := RenderTriangleDetection(r.candles, r.result, renderer, artNames.HTMLTmp); err != nil {
		log.Printf("[realtime] [%s] render error: %v", r.symbol, err)
		_ = os.Remove(artNames.HTMLTmp)
		return
	}
	if err := ss.Screenshot(artNames.HTMLTmp, artNames.PNG); err != nil {
		log.Printf("[realtime] [%s] screenshot error: %v", r.symbol, err)
	}
	_ = os.Remove(artNames.HTMLTmp)
	writeArtifactTexts(artNames, r.result)
	fmt.Printf("[realtime] [%s] screenshot saved: %s\n", r.symbol, artNames.PNG)
}

func nextCandleClose(now time.Time, interval time.Duration) time.Time {
	next := now.Truncate(interval).Add(interval).Add(5 * time.Second)
	if !next.After(now) {
		next = next.Add(interval)
	}
	return next
}
