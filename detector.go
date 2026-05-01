package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type SwingPoint struct {
	Index int
	Value float64
}

type DebugInfo struct {
	AvgPrice           float64
	ATR                float64
	Vol                float64
	CalcATRLog         string
	SwingHighsCount    int
	ResistanceLevel    float64
	ResistanceTouches  int
	FirstTouchIdx      int
	HighAboveThreshold float64
	CrashThreshold     float64
	ValleysCount       int
	FirstVIdx          int
	MaxCrashRange      float64
	AllowedFlat        float64
	SupportSlope       float64
	SupportIntercept   float64
	MaxValleyDepth     float64
	ValleyDeviation    float64
	PatternStart       int
	PatternEnd         int
	XIntersect         float64
	LastX              float64
	CeilingTol         float64
	Ceiling            float64
	FloorTol           float64
	HeightAtStart      float64
	HeightAtEnd        float64
	LastResistanceIdx  int
	LastValleyIdx      int
	PEnd               int
	PatternWidth       float64
}

type AscendingTriangleResult struct {
	Found                 bool
	RejectReason          string
	ResistanceLevel       float64
	ResistanceTouches     int
	ResistanceTouchPoints []SwingPoint
	SupportSlope          float64
	SupportIntercept      float64
	SupportTouchPoints    []SwingPoint
	Debug                 DebugInfo

	// Trading signals — derived after geometric pattern is confirmed.
	// Populated only when Found == true.
	//
	// TargetPrice: classical projection target for an ascending triangle.
	// Pattern height at formation = resistance - first valley. The target
	// is that same height projected above the resistance line.
	TargetPrice float64

	// BreakoutDetected: last candle's close has crossed above the
	// resistance level by the breakout tolerance. The pattern is reported
	// even when this is true (so callers see the breakout signal); the
	// detection rules in findHorizontalResistance and rule 13 are tuned
	// to allow the *final* candle to be a fresh breakout candle.
	BreakoutDetected bool

	// BreakoutVolumeRatio: last candle's volume divided by the average
	// volume of the prior 19 candles. A value >= 1.5 is the textbook
	// confirmation that the breakout has real participation. Zero when
	// breakout is not detected or when there is not enough history.
	BreakoutVolumeRatio float64
}

func DetectAscendingTriangle(candles []Candle, rejectStats map[string]*int) AscendingTriangleResult {
	return detectAscendingTriangle(candles, rejectStats)
}

func reject(reason string, stats map[string]*int) AscendingTriangleResult {
	if _, ok := stats[reason]; !ok {
		v := 0
		stats[reason] = &v
	}
	*stats[reason]++

	return AscendingTriangleResult{RejectReason: reason}
}

func detectAscendingTriangle(candles []Candle, rejectStats map[string]*int) AscendingTriangleResult {
	var dbg DebugInfo

	avgPrice := 0.0
	for _, c := range candles {
		avgPrice += c.Close
	}
	avgPrice /= float64(len(candles))

	atr, atrLog := calcATRDebug(candles)
	vol := atr / avgPrice
	dbg.AvgPrice = avgPrice
	dbg.ATR = atr
	dbg.Vol = vol
	dbg.CalcATRLog = atrLog

	const swingRadius = 3
	swingHighs := findSwingHighs(candles, swingRadius)
	dbg.SwingHighsCount = len(swingHighs)
	if len(swingHighs) < 2 {
		return reject("01_few_swing_highs", rejectStats)
	}

	resistanceLevel, resistanceTouches, resistanceTouchPoints := findHorizontalResistance(candles, swingHighs, vol)
	dbg.ResistanceLevel = resistanceLevel
	dbg.ResistanceTouches = resistanceTouches
	if resistanceTouches < 3 {
		return reject("02_resistance_<3_touches", rejectStats)
	}

	firstTouchIdx := resistanceTouchPoints[0].Index
	highAboveThreshold := resistanceLevel * (1 + vol*0.5)
	crashThreshold := resistanceLevel * (1 - math.Max(0.05, vol*8))
	dbg.FirstTouchIdx = firstTouchIdx
	dbg.HighAboveThreshold = highAboveThreshold
	dbg.CrashThreshold = crashThreshold
	for i := 0; i < firstTouchIdx; i++ {
		if candles[i].High > highAboveThreshold {
			return reject("03_high_before_first_touch", rejectStats)
		}
		if candles[i].Low < crashThreshold {
			return reject("04_crash_before_first_touch", rejectStats)
		}
	}

	if firstTouchIdx > len(candles)*2/5 {
		return reject("05_first_touch_too_late", rejectStats)
	}

	// 24_preceding_trend_not_up — ascending triangles are continuation
	// patterns of an existing uptrend. The candles before the first
	// resistance touch should be moving up into that resistance, not
	// drifting sideways or falling. Rule 03 already forbids highs above
	// resistance and rule 04 forbids crashes, but neither requires a
	// positive trend. Without this check, the same flat-top + flat-bottom
	// box can trigger a "triangle" if the geometry happens to fit.
	//
	// Implementation: linear regression on the closes of all candles
	// before firstTouchIdx. We require slope > 0. We skip the check if
	// there is not enough pre-pattern data to fit a line meaningfully.
	if firstTouchIdx >= 5 {
		prePoints := make([]SwingPoint, 0, firstTouchIdx)
		for i := range firstTouchIdx {
			prePoints = append(prePoints, SwingPoint{Index: i, Value: candles[i].Close})
		}
		preSlope, _ := linearRegression(prePoints)
		if preSlope <= 0 {
			return reject("24_preceding_trend_not_up", rejectStats)
		}
	}

	valleys := findValleysBetweenTouches(candles, resistanceTouchPoints)
	dbg.ValleysCount = len(valleys)
	if len(valleys) < 2 {
		return reject("06_few_valleys", rejectStats)
	}

	firstVIdx := valleys[0].Index
	maxCrashRange := 0.0
	for k := firstVIdx - 2; k <= firstVIdx; k++ {
		if k >= 0 {
			r := (candles[k].High - candles[k].Low) / avgPrice
			if r > maxCrashRange {
				maxCrashRange = r
			}
		}
	}
	dbg.FirstVIdx = firstVIdx
	dbg.MaxCrashRange = maxCrashRange
	if maxCrashRange > math.Max(0.015, vol*4) {
		return reject("20_first_valley_crash", rejectStats)
	}

	allowedFlat := vol * 1.5
	dbg.AllowedFlat = allowedFlat
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[i-1].Value*(1-allowedFlat) {
			return reject("07_valley_not_rising", rejectStats)
		}
	}

	// 21_first_valley_not_floor — distinguishes ascending triangles from
	// flat consolidations.
	//
	// In a true ascending triangle the FIRST valley defines the lowest
	// point and every subsequent valley sits at or above it (with minor
	// noise). In a flat market lows oscillate around a horizontal support,
	// so the lowest valley can appear anywhere in the pattern.
	//
	// This is intentionally orthogonal to the existing rules:
	//   - 07 checks only CONSECUTIVE pairs, so a valley can still dip
	//     below valley[0] if no single step is steep.
	//   - 08/16/17 measure the regression line's slope/narrowing; that
	//     line can be skewed positive by a few well-placed valleys even
	//     when the raw sequence oscillates.
	//   - 09 bounds depth relative to the resistance level, not relative
	//     to the first valley.
	//
	// We compare the detected swing valleys (not raw candle lows) so
	// isolated tail-wick noise does not trigger this. The tolerance is
	// ratio-based so genuinely narrow but valid triangles are not
	// penalized — only oscillation below the established floor is.
	floorTolerance := math.Max(0.003, vol)
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[0].Value*(1-floorTolerance) {
			return reject("21_first_valley_not_floor", rejectStats)
		}
	}

	supportSlope, supportIntercept := linearRegression(valleys)
	dbg.SupportSlope = supportSlope
	dbg.SupportIntercept = supportIntercept
	if supportSlope <= 0 {
		return reject("08_negative_slope", rejectStats)
	}

	maxValleyDepth := math.Max(0.015, vol*5)
	dbg.MaxValleyDepth = maxValleyDepth
	for _, v := range valleys {
		if v.Value < resistanceLevel*(1-maxValleyDepth) {
			return reject("09_valley_too_deep", rejectStats)
		}
	}

	if len(valleys) >= 3 {
		if rSquared(valleys, supportSlope, supportIntercept) < 0.85 {
			return reject("10_low_r_squared", rejectStats)
		}
	}

	valleyDeviation := math.Max(0.0015, vol*1.0)
	dbg.ValleyDeviation = valleyDeviation
	for _, v := range valleys {
		expected := supportSlope*float64(v.Index) + supportIntercept
		if expected > 0 && math.Abs(v.Value-expected)/expected > valleyDeviation {
			return reject("11_valley_off_support_line", rejectStats)
		}
	}

	patternStart := resistanceTouchPoints[0].Index
	if valleys[0].Index < patternStart {
		patternStart = valleys[0].Index
	}
	patternEnd := len(candles) - 1
	dbg.PatternStart = patternStart
	dbg.PatternEnd = patternEnd

	xIntersect := (resistanceLevel - supportIntercept) / supportSlope
	lastX := float64(len(candles) - 1)
	dbg.XIntersect = xIntersect
	dbg.LastX = lastX
	if xIntersect <= lastX {
		return reject("12_no_convergence", rejectStats)
	}

	// 13_breaks_ceiling — no candle's high may pierce the resistance by
	// more than ceilingTol. We deliberately exclude the LAST candle:
	// when the pattern is captured at the moment of breakout, the final
	// candle's high will exceed the ceiling — that is the breakout signal,
	// not a violation. Mid-pattern ceiling breaks still reject normally.
	ceilingTol := math.Max(0.002, vol*0.7)
	ceiling := resistanceLevel * (1 + ceilingTol)
	dbg.CeilingTol = ceilingTol
	dbg.Ceiling = ceiling
	ceilingEnd := patternEnd
	if ceilingEnd == len(candles)-1 {
		ceilingEnd = patternEnd - 1
	}
	for i := patternStart; i <= ceilingEnd; i++ {
		if candles[i].High > ceiling {
			return reject("13_breaks_ceiling", rejectStats)
		}
	}

	floorTol := math.Max(0.0015, vol*0.5)
	dbg.FloorTol = floorTol
	for i := patternStart; i <= patternEnd; i++ {
		supportVal := supportSlope*float64(i) + supportIntercept
		if candles[i].Low < supportVal*(1-floorTol) {
			return reject("14_breaks_support_floor", rejectStats)
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		if resistanceLevel <= supportSlope*float64(i)+supportIntercept {
			return reject("15_support_above_resistance", rejectStats)
		}
	}

	heightAtStart := resistanceLevel - (supportSlope*float64(patternStart) + supportIntercept)
	heightAtEnd := resistanceLevel - (supportSlope*float64(patternEnd) + supportIntercept)
	dbg.HeightAtStart = heightAtStart
	dbg.HeightAtEnd = heightAtEnd
	if heightAtEnd <= 0 || heightAtEnd >= heightAtStart*0.7 {
		return reject("16_not_narrowing", rejectStats)
	}

	if heightAtStart < resistanceLevel*0.005 {
		return reject("17_too_flat", rejectStats)
	}

	lastResistanceIdx := resistanceTouchPoints[len(resistanceTouchPoints)-1].Index
	lastValleyIdx := valleys[len(valleys)-1].Index
	pEnd := lastResistanceIdx
	if lastValleyIdx > pEnd {
		pEnd = lastValleyIdx
	}
	dbg.LastResistanceIdx = lastResistanceIdx
	dbg.LastValleyIdx = lastValleyIdx
	dbg.PEnd = pEnd
	if pEnd-patternStart < 15 {
		return reject("18_too_narrow", rejectStats)
	}

	patternWidth := float64(pEnd - patternStart)
	dbg.PatternWidth = patternWidth
	if xIntersect > lastX+patternWidth*2 {
		return reject("19_apex_too_far", rejectStats)
	}

	// 25_volume_not_declining — during a healthy ascending triangle volume
	// dries up as price coils into the apex (fewer participants want to
	// trade in the narrowing range). A pattern formed under rising volume
	// is more likely to be an active accumulation/distribution zone than a
	// classic continuation triangle.
	//
	// We fit a line through volumes from patternStart..pEnd and normalize
	// the slope by the average volume to get a per-candle percentage rate.
	// Strict "slope < 0" is too aggressive given how noisy volume is, so
	// we only reject when volume is *materially* rising (> 1% per candle,
	// roughly +30% across a 30-candle window). Patterns with flat or
	// declining volume pass.
	if pEnd-patternStart >= 10 {
		volPoints := make([]SwingPoint, 0, pEnd-patternStart+1)
		volSum := 0.0
		for i := patternStart; i <= pEnd; i++ {
			volPoints = append(volPoints, SwingPoint{Index: i, Value: candles[i].Volume})
			volSum += candles[i].Volume
		}
		avgVol := volSum / float64(len(volPoints))
		volSlope, _ := linearRegression(volPoints)
		if avgVol > 0 && volSlope/avgVol > 0.01 {
			return reject("25_volume_not_declining", rejectStats)
		}
	}

	// Target price — classical projection: pattern height (resistance
	// minus the lowest support point, i.e. first valley) added on top of
	// the resistance level. This is the price that a textbook breakout
	// trade would aim for, derived purely from the geometry already
	// validated above.
	targetPrice := resistanceLevel + (resistanceLevel - valleys[0].Value)

	// Breakout detection on the LAST candle. The geometric rules above
	// have been tuned so that a breakout in the final candle does not
	// invalidate the pattern (see comments on rule 13 and on
	// findHorizontalResistance's grace period). If the last close is
	// above resistance by the standard 0.5% tolerance, we report a
	// breakout and compute the volume confirmation ratio against the
	// prior 19 candles' average volume. Ratio >= 1.5 is the canonical
	// confirmation threshold; the caller decides how to act on it.
	n := len(candles)
	breakoutDetected := candles[n-1].Close > resistanceLevel*1.005
	breakoutVolumeRatio := 0.0
	if breakoutDetected {
		volStart := max(n-20, 0)
		sum := 0.0
		count := 0
		for i := volStart; i < n-1; i++ {
			sum += candles[i].Volume
			count++
		}
		if count > 0 && sum > 0 {
			avgVol := sum / float64(count)
			breakoutVolumeRatio = candles[n-1].Volume / avgVol
		}
	}

	return AscendingTriangleResult{
		Found:                 true,
		ResistanceLevel:       resistanceLevel,
		ResistanceTouches:     resistanceTouches,
		ResistanceTouchPoints: resistanceTouchPoints,
		SupportSlope:          supportSlope,
		SupportIntercept:      supportIntercept,
		SupportTouchPoints:    valleys,
		Debug:                 dbg,
		TargetPrice:           targetPrice,
		BreakoutDetected:      breakoutDetected,
		BreakoutVolumeRatio:   breakoutVolumeRatio,
	}
}

func findSwingHighs(candles []Candle, radius int) []SwingPoint {
	var highs []SwingPoint
	for i := radius; i < len(candles)-radius; i++ {
		isHigh := true
		for j := i - radius; j <= i+radius; j++ {
			if j != i && candles[j].High >= candles[i].High {
				isHigh = false
				break
			}
		}
		if isHigh {
			highs = append(highs, SwingPoint{Index: i, Value: candles[i].High})
		}
	}
	return highs
}

func atrFmt(x float64) string {
	const scale = 1e8
	r := math.Round(x*scale) / scale
	if r == 0 || math.Abs(r) < 1e-12 {
		return "0"
	}
	s := strconv.FormatFloat(r, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func calcATRDebug(candles []Candle) (float64, string) {
	var b strings.Builder
	fmt.Fprintf(&b, "calcATR step-by-step True Range trace\n")
	fmt.Fprintf(&b, "n=%d candles\n\n", len(candles))

	sum := candles[0].High - candles[0].Low
	c0 := candles[0]
	fmt.Fprintf(&b, "i=0 (first bar: TR = High-Low only, no prevClose)\n")
	fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s\n", atrFmt(c0.Open), atrFmt(c0.High), atrFmt(c0.Low), atrFmt(c0.Close))
	fmt.Fprintf(&b, "  tr = H-L = %s\n", atrFmt(sum))
	fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(sum))

	if len(candles) < 2 {
		atr := sum / float64(len(candles))
		fmt.Fprintf(&b, "---\n")
		fmt.Fprintf(&b, "sum(TR) = %s\n", atrFmt(sum))
		fmt.Fprintf(&b, "ATR = sum / n = %s / %d = %s\n", atrFmt(sum), len(candles), atrFmt(atr))
		return atr, b.String()
	}

	for i := 1; i < len(candles); i++ {
		c := candles[i]
		prevC := candles[i-1].Close
		tr := c.High - c.Low
		d1 := math.Abs(c.High - prevC)
		d2 := math.Abs(c.Low - prevC)

		fmt.Fprintf(&b, "i=%d\n", i)
		fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s  prevClose=%s\n",
			atrFmt(c.Open), atrFmt(c.High), atrFmt(c.Low), atrFmt(c.Close), atrFmt(prevC))
		fmt.Fprintf(&b, "  tr_initial (H-L) = %s\n", atrFmt(tr))
		fmt.Fprintf(&b, "  d1 = |H - prevClose| = %s\n", atrFmt(d1))
		fmt.Fprintf(&b, "  d2 = |L - prevClose| = %s\n", atrFmt(d2))

		cond1 := d1 > tr
		if cond1 {
			fmt.Fprintf(&b, "  condition (d1 > tr): true -> tr := d1 = %s\n", atrFmt(d1))
			tr = d1
		} else {
			fmt.Fprintf(&b, "  condition (d1 > tr): false (tr unchanged at %s)\n", atrFmt(tr))
		}

		cond2 := d2 > tr
		if cond2 {
			fmt.Fprintf(&b, "  condition (d2 > tr): true -> tr := d2 = %s\n", atrFmt(d2))
			tr = d2
		} else {
			fmt.Fprintf(&b, "  condition (d2 > tr): false (final tr = %s)\n", atrFmt(tr))
		}

		fmt.Fprintf(&b, "  final TR for bar i=%d: %s\n", i, atrFmt(tr))
		sum += tr
		fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(sum))
	}

	atr := sum / float64(len(candles))
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "sum(TR) = %s\n", atrFmt(sum))
	fmt.Fprintf(&b, "ATR = sum / n = %s / %d = %s\n", atrFmt(sum), len(candles), atrFmt(atr))
	return atr, b.String()
}

func calcATR(candles []Candle) float64 {
	v, _ := calcATRDebug(candles)
	return v
}

func findValleysBetweenTouches(candles []Candle, touches []SwingPoint) []SwingPoint {
	var valleys []SwingPoint

	for i := 0; i < len(touches)-1; i++ {
		start := touches[i].Index + 1
		end := touches[i+1].Index
		if end-start < 2 {
			continue
		}
		valleys = append(valleys, findLowestLow(candles, start, end))
	}

	lastTouch := touches[len(touches)-1].Index
	if len(candles)-1-lastTouch >= 5 {
		valleys = append(valleys, findLowestLow(candles, lastTouch+1, len(candles)))
	}

	return valleys
}

func findLowestLow(candles []Candle, from, to int) SwingPoint {
	minIdx := from
	minVal := candles[from].Low
	for i := from + 1; i < to; i++ {
		if candles[i].Low < minVal {
			minVal = candles[i].Low
			minIdx = i
		}
	}
	return SwingPoint{Index: minIdx, Value: minVal}
}

func rSquared(points []SwingPoint, slope, intercept float64) float64 {
	n := float64(len(points))
	sumY := 0.0
	for _, p := range points {
		sumY += p.Value
	}
	meanY := sumY / n
	ssTot, ssRes := 0.0, 0.0
	for _, p := range points {
		predicted := slope*float64(p.Index) + intercept
		ssTot += (p.Value - meanY) * (p.Value - meanY)
		ssRes += (p.Value - predicted) * (p.Value - predicted)
	}
	if ssTot == 0 {
		return 1.0
	}
	return 1.0 - ssRes/ssTot
}

func findHorizontalResistance(candles []Candle, highs []SwingPoint, vol float64) (level float64, touches int, touchPoints []SwingPoint) {
	if len(highs) < 2 {
		return 0, 0, nil
	}

	tolerance := math.Max(0.002, vol*0.8)
	const breakout = 0.005
	const minSpacing = 5

	type levelGroup struct {
		points []SwingPoint
		sum    float64
	}

	var groups []levelGroup

	for _, h := range highs {
		matched := false
		for i := range groups {
			avg := groups[i].sum / float64(len(groups[i].points))
			if math.Abs(h.Value-avg)/avg <= tolerance {
				groups[i].points = append(groups[i].points, h)
				groups[i].sum += h.Value
				matched = true
				break
			}
		}
		if !matched {
			groups = append(groups, levelGroup{points: []SwingPoint{h}, sum: h.Value})
		}
	}

	bestLevel := 0.0
	maxTouches := 0
	var bestTouchPoints []SwingPoint

	for _, g := range groups {
		valid := []SwingPoint{g.points[0]}
		for i := 1; i < len(g.points); i++ {
			if g.points[i].Index-valid[len(valid)-1].Index >= minSpacing {
				valid = append(valid, g.points[i])
			}
		}
		if len(valid) < 2 {
			continue
		}
		if len(valid) > maxTouches {
			maxTouches = len(valid)
			avg := g.sum / float64(len(g.points))
			bestLevel = avg
			bestTouchPoints = valid
		}
	}

	if maxTouches < 2 {
		return 0, 0, nil
	}

	for i := 0; i < len(bestTouchPoints)-1; i++ {
		start := bestTouchPoints[i].Index
		end := bestTouchPoints[i+1].Index
		for j := start; j <= end && j < len(candles); j++ {
			if candles[j].Close > bestLevel*(1+breakout) {
				return 0, 0, nil
			}
		}
	}

	// After-touch close check — any close above resistance between the
	// last touch and the end of the window invalidates the pattern,
	// EXCEPT for the very last candle. We treat the final candle as a
	// potential fresh breakout candle: allowing it to close above the
	// resistance is what makes downstream breakout detection possible.
	// Mid-period breakouts (anywhere except the last index) still reject.
	lastIdx := bestTouchPoints[len(bestTouchPoints)-1].Index
	for j := lastIdx; j < len(candles)-1; j++ {
		if candles[j].Close > bestLevel*(1+breakout) {
			return 0, 0, nil
		}
	}

	return bestLevel, maxTouches, bestTouchPoints
}

func linearRegression(points []SwingPoint) (slope, intercept float64) {
	n := float64(len(points))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for _, p := range points {
		x := float64(p.Index)
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}
