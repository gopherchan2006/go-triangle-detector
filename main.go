package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func sanitizeReason(reason string) string {
	r := strings.ReplaceAll(reason, "<", "lt")
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

func writeDebugTxt(txtPath string, result AscendingTriangleResult) {
	d := result.Debug
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("avgPrice            = %.6f\n", d.AvgPrice))
	sb.WriteString(fmt.Sprintf("atr                 = %.6f\n", d.ATR))
	sb.WriteString(fmt.Sprintf("vol                 = %.8f\n", d.Vol))
	sb.WriteString(fmt.Sprintf("swingHighsCount     = %d\n", d.SwingHighsCount))
	sb.WriteString(fmt.Sprintf("resistanceLevel     = %.6f\n", d.ResistanceLevel))
	sb.WriteString(fmt.Sprintf("resistanceTouches   = %d\n", d.ResistanceTouches))
	sb.WriteString(fmt.Sprintf("firstTouchIdx       = %d\n", d.FirstTouchIdx))
	sb.WriteString(fmt.Sprintf("highAboveThreshold  = %.6f\n", d.HighAboveThreshold))
	sb.WriteString(fmt.Sprintf("crashThreshold      = %.6f\n", d.CrashThreshold))
	sb.WriteString(fmt.Sprintf("valleysCount        = %d\n", d.ValleysCount))
	sb.WriteString(fmt.Sprintf("firstVIdx           = %d\n", d.FirstVIdx))
	sb.WriteString(fmt.Sprintf("maxCrashRange       = %.8f\n", d.MaxCrashRange))
	sb.WriteString(fmt.Sprintf("allowedFlat         = %.8f\n", d.AllowedFlat))
	sb.WriteString(fmt.Sprintf("supportSlope        = %.8f\n", d.SupportSlope))
	sb.WriteString(fmt.Sprintf("supportIntercept    = %.6f\n", d.SupportIntercept))
	sb.WriteString(fmt.Sprintf("maxValleyDepth      = %.8f\n", d.MaxValleyDepth))
	sb.WriteString(fmt.Sprintf("valleyDeviation     = %.8f\n", d.ValleyDeviation))
	sb.WriteString(fmt.Sprintf("patternStart        = %d\n", d.PatternStart))
	sb.WriteString(fmt.Sprintf("patternEnd          = %d\n", d.PatternEnd))
	sb.WriteString(fmt.Sprintf("xIntersect          = %.4f\n", d.XIntersect))
	sb.WriteString(fmt.Sprintf("lastX               = %.4f\n", d.LastX))
	sb.WriteString(fmt.Sprintf("ceilingTol          = %.8f\n", d.CeilingTol))
	sb.WriteString(fmt.Sprintf("ceiling             = %.6f\n", d.Ceiling))
	sb.WriteString(fmt.Sprintf("floorTol            = %.8f\n", d.FloorTol))
	sb.WriteString(fmt.Sprintf("heightAtStart       = %.6f\n", d.HeightAtStart))
	sb.WriteString(fmt.Sprintf("heightAtEnd         = %.6f\n", d.HeightAtEnd))
	sb.WriteString(fmt.Sprintf("lastResistanceIdx   = %d\n", d.LastResistanceIdx))
	sb.WriteString(fmt.Sprintf("lastValleyIdx       = %d\n", d.LastValleyIdx))
	sb.WriteString(fmt.Sprintf("pEnd                = %d\n", d.PEnd))
	sb.WriteString(fmt.Sprintf("patternWidth        = %.4f\n", d.PatternWidth))

	if err := os.WriteFile(txtPath, []byte(sb.String()), 0o644); err != nil {
		log.Printf("writeDebugTxt: %v", err)
	}
}

func writeCalcATRDebugTxt(txtPath string, logContent string) {
	if strings.TrimSpace(logContent) == "" {
		return
	}
	if err := os.WriteFile(txtPath, []byte(logContent), 0o644); err != nil {
		log.Printf("writeCalcATRDebugTxt: %v", err)
	}
}

func writeFindSwingHighsDebugTxt(txtPath string, logContent string) {
	if strings.TrimSpace(logContent) == "" {
		return
	}
	if err := os.WriteFile(txtPath, []byte(logContent), 0o644); err != nil {
		log.Printf("writeFindSwingHighsDebugTxt: %v", err)
	}
}

func writeFindHorizontalResistanceDebugTxt(txtPath string, logContent string) {
	if strings.TrimSpace(logContent) == "" {
		return
	}
	if err := os.WriteFile(txtPath, []byte(logContent), 0o644); err != nil {
		log.Printf("writeFindHorizontalResistanceDebugTxt: %v", err)
	}
}

func main() {
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
		analyzeSymbol(sym, *interval, *startDate, *endDate, dataDir, ss, *rejectLimit)
	}
}

func analyzeSymbol(symbol, interval, startDate, endDate string, dataDir string, ss *Screenshotter, rejectLimit int) {
	chartDir := filepath.Join("tmp", symbol+"_chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		log.Printf("[%s] failed to create chart dir: %v", symbol, err)
		return
	}

	if entries, err := os.ReadDir(chartDir); err == nil {
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(chartDir, entry.Name()))
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
	rejectChartCounts := make(map[string]int)

	for i := 0; i <= len(candles)-windowSize; i++ {
		window := candles[i : i+windowSize]
		result := DetectAscendingTriangle(window, rejectStats)

		if result.Found {
			patterns++
			timestamp := window[0].Timestamp
			fileDate := timestamp.Format("2006-01-02")
			labelDate := timestamp.Format("2006-01-02 15:04:05")

			stem := fmt.Sprintf("%s_%s", symbol, fileDate)
			groupDir := filepath.Join(chartDir, stem)
			if err := os.MkdirAll(groupDir, 0o755); err != nil {
				log.Printf("[%s] failed to create group dir %s: %v", symbol, groupDir, err)
				continue
			}
			htmlTmp := filepath.Join(chartDir, stem+"_render.tmp.html")
			pngFile := filepath.Join(groupDir, fmt.Sprintf("1_%s_1.png", stem))
			debugTxt := filepath.Join(groupDir, fmt.Sprintf("2_%s_2.txt", stem))
			calcATRTxt := filepath.Join(groupDir, fmt.Sprintf("3_%s_calcATR_3.txt", stem))
			swingTxt := filepath.Join(groupDir, fmt.Sprintf("4_%s_findSwingHighs_4.txt", stem))
			horizTxt := filepath.Join(groupDir, fmt.Sprintf("5_%s_findHorizontalResistance_5.txt", stem))

			renderer := NewEChartsRenderer()
			renderer.SetCaption(symbol, time.Now().UTC())
			if err := RenderTriangleDetection(window, result, renderer, htmlTmp); err != nil {
				log.Printf("[%s] error rendering chart for %s: %v", symbol, fileDate, err)
				_ = os.Remove(htmlTmp)
				continue
			}
			if err := ss.Screenshot(htmlTmp, pngFile); err != nil {
				log.Printf("[%s] error taking screenshot for %s: %v", symbol, fileDate, err)
			}
			writeDebugTxt(debugTxt, result)
			writeCalcATRDebugTxt(calcATRTxt, result.Debug.CalcATRLog)
			writeFindSwingHighsDebugTxt(swingTxt, result.Debug.FindSwingHighsLog)
			writeFindHorizontalResistanceDebugTxt(horizTxt, result.Debug.FindHorizontalResistanceLog)
			_ = os.Remove(htmlTmp)

			fmt.Printf("[%s] [Pattern #%d] %s | Resistance: %.2f | Support slope: %.4f\n",
				symbol, patterns, labelDate, result.ResistanceLevel, result.SupportSlope)

		} else if rejectLimit > 0 && result.RejectReason != "" {
			reason := result.RejectReason
			if rejectChartCounts[reason] >= rejectLimit {
				continue
			}

			timestamp := window[0].Timestamp
			fileDate := timestamp.Format("2006-01-02")

			safeReason := sanitizeReason(reason)
			rejectDir := filepath.Join("tmp", "rejects", safeReason, symbol)
			if err := os.MkdirAll(rejectDir, 0o755); err != nil {
				log.Printf("[%s] failed to create reject dir: %v", symbol, err)
				continue
			}

			stem := fmt.Sprintf("%s_%s", symbol, fileDate)
			groupDir := filepath.Join(rejectDir, stem)
			if err := os.MkdirAll(groupDir, 0o755); err != nil {
				log.Printf("[%s] failed to create reject group dir: %v", symbol, err)
				continue
			}
			htmlTmp := filepath.Join(rejectDir, stem+"_render.tmp.html")
			pngFile := filepath.Join(groupDir, fmt.Sprintf("1_%s_1.png", stem))

			if _, statErr := os.Stat(pngFile); statErr == nil {
				continue
			}

			renderer := NewEChartsRenderer()
			renderer.SetCaption(symbol, time.Now().UTC())
			if err := RenderTriangleDetection(window, result, renderer, htmlTmp); err != nil {
				_ = os.Remove(htmlTmp)
				continue
			}
			if err := ss.Screenshot(htmlTmp, pngFile); err != nil {
				log.Printf("[%s] reject chart error for %s/%s: %v", symbol, reason, fileDate, err)
			}
			_ = os.Remove(htmlTmp)

			rejectChartCounts[reason]++
		}
	}

	fmt.Printf("[%s] Analysis complete. Found %d pattern(s). Charts saved to: %s\n", symbol, patterns, chartDir)

	fmt.Printf("[%s] --- Reject reasons ---\n", symbol)
	for reason, count := range rejectStats {
		saved := rejectChartCounts[reason]
		fmt.Printf("[%s]   %-40s hits: %d  charts: %d\n", symbol, reason, *count, saved)
	}
}
