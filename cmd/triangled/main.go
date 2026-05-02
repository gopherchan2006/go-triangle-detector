package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"triangle-detector/internal/app"
	"triangle-detector/internal/artifact"
	"triangle-detector/internal/config"
	"triangle-detector/internal/detect"
	"triangle-detector/internal/domain"
	"triangle-detector/internal/marketdata/binance"
	"triangle-detector/internal/render/echarts"
	"triangle-detector/internal/screenshot"
)

func sanitizeReason(reason detect.RejectReason) string {
	r := strings.ReplaceAll(string(reason), "<", "lt")
	r = strings.ReplaceAll(r, ">", "gt")
	r = strings.ReplaceAll(r, ":", "_")
	r = strings.ReplaceAll(r, "/", "_")
	r = strings.ReplaceAll(r, "\\", "_")
	r = strings.ReplaceAll(r, "*", "_")
	r = strings.ReplaceAll(r, "?", "_")
	r = strings.ReplaceAll(r, "\"", "_")
	r = strings.ReplaceAll(r, "|", "_")
	return r
}

func writeDebugTxt(txtPath string, result detect.Result) {
	d := result.Debug
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("avgPrice            = %.6f\n", d.ATR.AvgPrice))
	sb.WriteString(fmt.Sprintf("atr                 = %.6f\n", d.ATR.ATRValue))
	sb.WriteString(fmt.Sprintf("vol                 = %.8f\n", d.ATR.Vol))
	sb.WriteString(fmt.Sprintf("swingHighsCount     = %d\n", d.Swing.SwingHighsCount))
	sb.WriteString(fmt.Sprintf("resistanceLevel     = %.6f\n", d.Resistance.ResistanceLevel))
	sb.WriteString(fmt.Sprintf("resistanceTouches   = %d\n", d.Resistance.ResistanceTouches))
	sb.WriteString(fmt.Sprintf("firstTouchIdx       = %d\n", d.Resistance.FirstTouchIdx))
	sb.WriteString(fmt.Sprintf("highAboveThreshold  = %.6f\n", d.Resistance.HighAboveThreshold))
	sb.WriteString(fmt.Sprintf("crashThreshold      = %.6f\n", d.Resistance.CrashThreshold))
	sb.WriteString(fmt.Sprintf("valleysCount        = %d\n", d.Support.ValleysCount))
	sb.WriteString(fmt.Sprintf("firstVIdx           = %d\n", d.Support.FirstVIdx))
	sb.WriteString(fmt.Sprintf("maxCrashRange       = %.8f\n", d.Support.MaxCrashRange))
	sb.WriteString(fmt.Sprintf("allowedFlat         = %.8f\n", d.Support.AllowedFlat))
	sb.WriteString(fmt.Sprintf("supportSlope        = %.8f\n", d.Support.SupportSlope))
	sb.WriteString(fmt.Sprintf("supportIntercept    = %.6f\n", d.Support.SupportIntercept))
	sb.WriteString(fmt.Sprintf("maxValleyDepth      = %.8f\n", d.Support.MaxValleyDepth))
	sb.WriteString(fmt.Sprintf("valleyDeviation     = %.8f\n", d.Support.ValleyDeviation))
	sb.WriteString(fmt.Sprintf("patternStart        = %d\n", d.Geometry.PatternStart))
	sb.WriteString(fmt.Sprintf("patternEnd          = %d\n", d.Geometry.PatternEnd))
	sb.WriteString(fmt.Sprintf("xIntersect          = %.4f\n", d.Geometry.XIntersect))
	sb.WriteString(fmt.Sprintf("lastX               = %.4f\n", d.Geometry.LastX))
	sb.WriteString(fmt.Sprintf("ceilingTol          = %.8f\n", d.Geometry.CeilingTol))
	sb.WriteString(fmt.Sprintf("ceiling             = %.6f\n", d.Geometry.Ceiling))
	sb.WriteString(fmt.Sprintf("floorTol            = %.8f\n", d.Geometry.FloorTol))
	sb.WriteString(fmt.Sprintf("heightAtStart       = %.6f\n", d.Geometry.HeightAtStart))
	sb.WriteString(fmt.Sprintf("heightAtEnd         = %.6f\n", d.Geometry.HeightAtEnd))
	sb.WriteString(fmt.Sprintf("lastResistanceIdx   = %d\n", d.Geometry.LastResistanceIdx))
	sb.WriteString(fmt.Sprintf("lastValleyIdx       = %d\n", d.Geometry.LastValleyIdx))
	sb.WriteString(fmt.Sprintf("pEnd                = %d\n", d.Geometry.PEnd))
	sb.WriteString(fmt.Sprintf("patternWidth        = %.4f\n", d.Geometry.PatternWidth))

	if err := os.WriteFile(txtPath, []byte(sb.String()), 0o644); err != nil {
		log.Printf("writeDebugTxt: %v", err)
	}
}

func analyzeSymbol(ctx context.Context, sym, interval, startDate, endDate string, dataDir string, ss *screenshot.Screenshotter, rejectLimit int) {
	chartDir := filepath.Join("tmp", sym+"_chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		log.Printf("[%s] failed to create chart dir: %v", sym, err)
		return
	}

	if entries, err := os.ReadDir(chartDir); err == nil {
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(chartDir, entry.Name()))
		}
	}

	candles, err := binance.LoadCandles(binance.CandleRequestParams{
		Symbol:    sym,
		Interval:  interval,
		StartTime: startDate,
		EndTime:   endDate,
	})
	if err != nil {
		log.Printf("[%s] failed to load candles: %v", sym, err)
		return
	}

	fmt.Printf("\n[%s] Loaded %d candles\n", sym, len(candles))

	if len(candles) < 50 {
		fmt.Printf("[%s] not enough candles (need at least 50)\n", sym)
		return
	}

	windowSize := 50
	patterns := 0
	counter := detect.NewMapCounter()
	rejectChartCounts := make(map[detect.RejectReason]int)

	for i := 0; i <= len(candles)-windowSize; i++ {
		select {
		case <-ctx.Done():
			fmt.Printf("[%s] interrupted\n", sym)
			return
		default:
		}
		window := candles[i : i+windowSize]
		result := detect.DetectAscendingTriangle(window, detect.WithCounter(counter))

		if result.Found {
			patterns++
			timestamp := window[0].Timestamp
			fileDate := timestamp.Format("2006-01-02")
			labelDate := timestamp.Format("2006-01-02 15:04:05")

			stem := fmt.Sprintf("%s_%s", sym, fileDate)
			artNames := artifact.NewNames(chartDir, stem)

			if err := os.MkdirAll(artNames.GroupDir, 0o755); err != nil {
				log.Printf("[%s] failed to create group dir %s: %v", sym, artNames.GroupDir, err)
				continue
			}
			renderer := echarts.NewEChartsRenderer()
			renderer.SetCaption(sym, time.Now().UTC())
			if err := app.RenderTriangleDetection(window, result, renderer, artNames.HTMLTmp); err != nil {
				log.Printf("[%s] error rendering chart for %s: %v", sym, fileDate, err)
				_ = os.Remove(artNames.HTMLTmp)
				continue
			}
			if ss != nil {
				if err := ss.Screenshot(artNames.HTMLTmp, artNames.PNG); err != nil {
					log.Printf("[%s] error taking screenshot for %s: %v", sym, fileDate, err)
				}
			}
			artifact.WriteTexts(artNames, result, writeDebugTxt)
			_ = os.Remove(artNames.HTMLTmp)

			fmt.Printf("[%s] [Pattern #%d] %s | Resistance: %.2f | Support slope: %.4f\n",
				sym, patterns, labelDate, result.ResistanceLevel, result.SupportSlope)

		} else if rejectLimit > 0 && result.RejectReason != "" {
			reason := result.RejectReason
			if rejectChartCounts[reason] >= rejectLimit {
				continue
			}

			timestamp := window[0].Timestamp
			fileDate := timestamp.Format("2006-01-02")

			safeReason := sanitizeReason(reason)
			rejectDir := filepath.Join("tmp", "rejects", safeReason, sym)
			if err := os.MkdirAll(rejectDir, 0o755); err != nil {
				log.Printf("[%s] failed to create reject dir: %v", sym, err)
				continue
			}

			stem := fmt.Sprintf("%s_%s", sym, fileDate)
			groupDir := filepath.Join(rejectDir, stem)
			if err := os.MkdirAll(groupDir, 0o755); err != nil {
				log.Printf("[%s] failed to create reject group dir: %v", sym, err)
				continue
			}
			htmlTmp := filepath.Join(rejectDir, stem+"_render.tmp.html")
			pngFile := filepath.Join(groupDir, fmt.Sprintf("1_%s_1.png", stem))

			if _, statErr := os.Stat(pngFile); statErr == nil {
				continue
			}

			renderer := echarts.NewEChartsRenderer()
			renderer.SetCaption(sym, time.Now().UTC())
			if err := app.RenderTriangleDetection(window, result, renderer, htmlTmp); err != nil {
				_ = os.Remove(htmlTmp)
				continue
			}
			if ss != nil {
				if err := ss.Screenshot(htmlTmp, pngFile); err != nil {
					log.Printf("[%s] reject chart error for %s/%s: %v", sym, reason, fileDate, err)
				}
			}
			_ = os.Remove(htmlTmp)

			rejectChartCounts[reason]++
		}
	}

	fmt.Printf("[%s] Analysis complete. Found %d pattern(s). Charts saved to: %s\n", sym, patterns, chartDir)

	fmt.Printf("[%s] --- Reject reasons ---\n", sym)
	for reason, count := range counter.Snapshot() {
		saved := rejectChartCounts[reason]
		fmt.Printf("[%s]   %-40s hits: %d  charts: %d\n", sym, reason, count, saved)
	}
}

type realtimeConfig struct {
	Interval        string
	IntervalDur     time.Duration
	Workers         int
	WindowSize      int
	OutputDir       string
	WithScreenshots bool
}

type scanResult struct {
	symbol  string
	candles []domain.Candle
	result  detect.Result
}

type alertState struct {
	resistance float64
	alertedAt  time.Time
}

func runRealtime(ctx context.Context, cfg realtimeConfig, ss *screenshot.Screenshotter) {
	fmt.Println("[realtime] Fetching all active USDT spot pairs from Binance...")
	symbols, err := binance.FetchAllUSDTSymbols()
	if err != nil {
		log.Fatalf("[realtime] failed to fetch symbols: %v", err)
	}
	fmt.Printf("[realtime] Found %d USDT symbols\n", len(symbols))

	intervalMs, err := binance.IntervalToMilliseconds(cfg.Interval)
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

	runCycle(ctx, 0, symbols, cfg, alerts, ss)

	for cycle := 1; ; cycle++ {
		select {
		case <-ctx.Done():
			fmt.Println("[realtime] shutdown signal received")
			return
		default:
		}
		if time.Since(symbolsFetchedAt) > 24*time.Hour {
			if fresh, err := binance.FetchAllUSDTSymbols(); err == nil {
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
		select {
		case <-time.After(waitDur):
		case <-ctx.Done():
			fmt.Println("[realtime] shutdown signal received")
			return
		}

		runCycle(ctx, cycle, symbols, cfg, alerts, ss)
	}
}

func runCycle(ctx context.Context, cycle int, symbols []string, cfg realtimeConfig, alerts map[string]alertState, ss *screenshot.Screenshotter) {
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

func scanAllSymbols(symbols []string, cfg realtimeConfig) []scanResult {
	jobs := make(chan string, len(symbols))
	resultCh := make(chan scanResult, len(symbols))

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				candles, err := binance.LoadLastNCandles(sym, cfg.Interval, cfg.WindowSize)
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
				det := detect.DetectAscendingTriangle(window, detect.WithTrace(false))
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

func takeRealtimeScreenshot(r scanResult, cfg realtimeConfig, ss *screenshot.Screenshotter) {
	ts := time.Now().UTC().Format("20060102_1504")
	pairDir := filepath.Join(cfg.OutputDir, r.symbol)
	if err := os.MkdirAll(pairDir, 0o755); err != nil {
		log.Printf("[realtime] [%s] failed to create dir: %v", r.symbol, err)
		return
	}
	stem := fmt.Sprintf("%s_%s", r.symbol, ts)
	artNames := artifact.NewNames(pairDir, stem)
	if err := os.MkdirAll(artNames.GroupDir, 0o755); err != nil {
		log.Printf("[realtime] [%s] failed to create group dir: %v", r.symbol, err)
		return
	}
	renderer := echarts.NewEChartsRenderer()
	renderer.SetCaption(r.symbol, time.Now().UTC())
	if err := app.RenderTriangleDetection(r.candles, r.result, renderer, artNames.HTMLTmp); err != nil {
		log.Printf("[realtime] [%s] render error: %v", r.symbol, err)
		_ = os.Remove(artNames.HTMLTmp)
		return
	}
	if err := ss.Screenshot(artNames.HTMLTmp, artNames.PNG); err != nil {
		log.Printf("[realtime] [%s] screenshot error: %v", r.symbol, err)
	}
	_ = os.Remove(artNames.HTMLTmp)
	artifact.WriteTexts(artNames, r.result, writeDebugTxt)
	fmt.Printf("[realtime] [%s] screenshot saved: %s\n", r.symbol, artNames.PNG)
}

func nextCandleClose(now time.Time, interval time.Duration) time.Time {
	next := now.Truncate(interval).Add(interval).Add(5 * time.Second)
	if !next.After(now) {
		next = next.Add(interval)
	}
	return next
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	symbol := flag.String("symbol", "", "Trading pair symbol, e.g. BTCUSDT")
	interval := flag.String("interval", "", "Candle interval, e.g. 15m")
	startDate := flag.String("start", "", "Start time in RFC3339 or YYYY-MM-DD (default: 2026-01-01)")
	endDate := flag.String("end", "", "End time in RFC3339 or YYYY-MM-DD (default: 2026-04-18)")
	realtimeMode := flag.Bool("realtime", false, "Run real-time scanning on all active USDT pairs")
	workers := flag.Int("workers", 20, "Concurrent workers for real-time mode")
	noScreenshots := flag.Bool("no-screenshots", false, "Disable screenshots in real-time mode")
	rejectLimit := flag.Int("reject-limit", 0, "Max reject charts to save per filter (0 = disabled)")
	flag.Parse()

	if err := os.RemoveAll("tmp"); err != nil {
		log.Printf("remove tmp: %v", err)
	}

	_ = config.LoadEnvFile(".env")
	appCfg := config.LoadAppConfig()

	if *interval == "" {
		*interval = "15m"
	}

	if *realtimeMode {
		needBrowser := !*noScreenshots
		var ss *screenshot.Screenshotter
		if needBrowser {
			var err error
			ss, err = screenshot.NewScreenshotter()
			if err != nil {
				log.Fatalf("failed to start browser: %v", err)
			}
			defer ss.Close()
		}

		cfg := realtimeConfig{
			Interval:        *interval,
			Workers:         *workers,
			WindowSize:      50,
			OutputDir:       filepath.Join("tmp", "realtime"),
			WithScreenshots: needBrowser,
		}
		runRealtime(ctx, cfg, ss)
		return
	}

	if *startDate == "" {
		*startDate = "2026-01-01"
	}
	if *endDate == "" {
		*endDate = "2026-04-18"
	}

	dataDir := appCfg.DataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	var symbols []string
	if *symbol != "" {
		symbols = []string{*symbol}
	} else {
		raw := appCfg.Symbols
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

	ss, err := screenshot.NewScreenshotter()
	if err != nil {
		log.Fatalf("failed to start browser: %v", err)
	}
	defer ss.Close()

	for _, sym := range symbols {
		analyzeSymbol(ctx, sym, *interval, *startDate, *endDate, dataDir, ss, *rejectLimit)
	}
}
