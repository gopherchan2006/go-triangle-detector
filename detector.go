package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)


func DetectAscendingTriangle(candles []Candle, params DetectorParams, rejectStats map[RejectReason]*int) AscendingTriangleResult {
	return detectAscendingTriangle(candles, params, rejectStats)
}

func reject(reason RejectReason, stats map[RejectReason]*int) AscendingTriangleResult {
	if _, ok := stats[reason]; !ok {
		v := 0
		stats[reason] = &v
	}
	*stats[reason]++

	return AscendingTriangleResult{RejectReason: reason}
}

func detectAscendingTriangle(candles []Candle, p DetectorParams, rejectStats map[RejectReason]*int) AscendingTriangleResult {
	var dbg DebugInfo

	avgPrice := 0.0
	for _, c := range candles {
		avgPrice += c.Close
	}
	avgPrice /= float64(len(candles))

	atrSnap := collectCalcATRDebug(candles)
	vol := atrSnap.ATR / avgPrice
	dbg.AvgPrice = avgPrice
	dbg.ATR = atrSnap.ATR
	dbg.Vol = vol
	dbg.CalcATRLog = formatCalcATRDebug(atrSnap)

	swingSnap := collectFindSwingHighsDebug(candles, p.SwingRadius)
	swingHighs := swingSnap.SwingHighs
	dbg.FindSwingHighsLog = formatFindSwingHighsDebug(swingSnap)
	dbg.SwingHighsCount = len(swingHighs)
	if len(swingHighs) < 2 {
		return reject(ReasonFewSwingHighs, rejectStats)
	}

	rhSnap := collectFindHorizontalResistanceDebug(candles, swingHighs, vol, p)
	dbg.FindHorizontalResistanceLog = formatFindHorizontalResistanceDebug(rhSnap)
	resistanceLevel, resistanceTouches, resistanceTouchPoints := rhSnap.Level, rhSnap.Touches, rhSnap.TouchPoints
	dbg.ResistanceLevel = resistanceLevel
	dbg.ResistanceTouches = resistanceTouches
	if resistanceTouches < 3 {
		return reject(ReasonResistanceLt3Touches, rejectStats)
	}

	firstTouchIdx := resistanceTouchPoints[0].Index
	highAboveThreshold := resistanceLevel * (1 + vol*p.HighAboveVolMult)
	crashThreshold := resistanceLevel * (1 - math.Max(p.CrashVolMin, vol*8))
	dbg.FirstTouchIdx = firstTouchIdx
	dbg.HighAboveThreshold = highAboveThreshold
	dbg.CrashThreshold = crashThreshold
	for i := 0; i < firstTouchIdx; i++ {
		if candles[i].High > highAboveThreshold {
			return reject(ReasonHighBeforeFirstTouch, rejectStats)
		}
		if candles[i].Low < crashThreshold {
			return reject(ReasonCrashBeforeFirstTouch, rejectStats)
		}
	}

	if float64(firstTouchIdx) > float64(len(candles))*p.FirstTouchMaxRatio {
		return reject(ReasonFirstTouchTooLate, rejectStats)
	}

	if firstTouchIdx >= 5 {
		prePoints := make([]SwingPoint, 0, firstTouchIdx)
		for i := range firstTouchIdx {
			prePoints = append(prePoints, SwingPoint{Index: i, Value: candles[i].Close})
		}
		preSlope, _ := linearRegression(prePoints)
		if preSlope <= 0 {
			return reject(ReasonPrecedingTrendNotUp, rejectStats)
		}
	}

	valleys := findValleysBetweenTouches(candles, resistanceTouchPoints)
	dbg.ValleysCount = len(valleys)
	if len(valleys) < 2 {
		return reject(ReasonFewValleys, rejectStats)
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
	if maxCrashRange > math.Max(p.MaxFirstValleyCrash, vol*4) {
		return reject(ReasonFirstValleyCrash, rejectStats)
	}

	allowedFlat := vol * p.AllowedFlatVolMult
	dbg.AllowedFlat = allowedFlat
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[i-1].Value*(1-allowedFlat) {
			return reject(ReasonValleyNotRising, rejectStats)
		}
	}

	floorTolerance := math.Max(p.FloorTolerance, vol)
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[0].Value*(1-floorTolerance) {
			return reject(ReasonFirstValleyNotFloor, rejectStats)
		}
	}

	supportSlope, supportIntercept := linearRegression(valleys)
	dbg.SupportSlope = supportSlope
	dbg.SupportIntercept = supportIntercept
	if supportSlope <= 0 {
		return reject(ReasonNegativeSlope, rejectStats)
	}

	maxValleyDepth := math.Max(p.MaxValleyDepthMin, vol*5)
	dbg.MaxValleyDepth = maxValleyDepth
	for _, v := range valleys {
		if v.Value < resistanceLevel*(1-maxValleyDepth) {
			return reject(ReasonValleyTooDeep, rejectStats)
		}
	}

	if len(valleys) >= 3 {
		if rSquared(valleys, supportSlope, supportIntercept) < p.MinRSquared {
			return reject(ReasonLowRSquared, rejectStats)
		}
	}

	valleyDeviation := math.Max(p.ValleyDeviationMin, vol*1.0)
	dbg.ValleyDeviation = valleyDeviation
	for _, v := range valleys {
		expected := supportSlope*float64(v.Index) + supportIntercept
		if expected > 0 && math.Abs(v.Value-expected)/expected > valleyDeviation {
			return reject(ReasonValleyOffSupportLine, rejectStats)
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
		return reject(ReasonNoConvergence, rejectStats)
	}

	ceilingTol := math.Max(p.CeilingTolMin, vol*0.7)
	ceiling := resistanceLevel * (1 + ceilingTol)
	dbg.CeilingTol = ceilingTol
	dbg.Ceiling = ceiling
	ceilingEnd := patternEnd
	if ceilingEnd == len(candles)-1 {
		ceilingEnd = patternEnd - 1
	}
	for i := patternStart; i <= ceilingEnd; i++ {
		if candles[i].High > ceiling {
			return reject(ReasonBreaksCeiling, rejectStats)
		}
	}

	floorTol := math.Max(p.FloorTolMin, vol*0.5)
	dbg.FloorTol = floorTol
	for i := patternStart; i <= patternEnd; i++ {
		supportVal := supportSlope*float64(i) + supportIntercept
		if candles[i].Low < supportVal*(1-floorTol) {
			return reject(ReasonBreaksSupportFloor, rejectStats)
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		if resistanceLevel <= supportSlope*float64(i)+supportIntercept {
			return reject(ReasonSupportAboveResistance, rejectStats)
		}
	}

	heightAtStart := resistanceLevel - (supportSlope*float64(patternStart) + supportIntercept)
	heightAtEnd := resistanceLevel - (supportSlope*float64(patternEnd) + supportIntercept)
	dbg.HeightAtStart = heightAtStart
	dbg.HeightAtEnd = heightAtEnd
	if heightAtEnd <= 0 || heightAtEnd >= heightAtStart*p.MaxNarrowingRatio {
		return reject(ReasonNotNarrowing, rejectStats)
	}

	if heightAtStart < resistanceLevel*p.MinPatternHeight {
		return reject(ReasonTooFlat, rejectStats)
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
	if pEnd-patternStart < p.MinPatternWidth {
		return reject(ReasonTooNarrow, rejectStats)
	}

	patternWidth := float64(pEnd - patternStart)
	dbg.PatternWidth = patternWidth
	if xIntersect > lastX+patternWidth*p.MaxApexFactor {
		return reject(ReasonApexTooFar, rejectStats)
	}

	if pEnd-patternStart >= p.VolDeclMinWidth {
		volPoints := make([]SwingPoint, 0, pEnd-patternStart+1)
		volSum := 0.0
		for i := patternStart; i <= pEnd; i++ {
			volPoints = append(volPoints, SwingPoint{Index: i, Value: candles[i].Volume})
			volSum += candles[i].Volume
		}
		avgVol := volSum / float64(len(volPoints))
		volSlope, _ := linearRegression(volPoints)
		if avgVol > 0 && volSlope/avgVol > p.VolDeclSlopeMax {
			return reject(ReasonVolumeNotDeclining, rejectStats)
		}
	}

	targetPrice := resistanceLevel + (resistanceLevel - valleys[0].Value)

	n := len(candles)
	breakoutDetected := candles[n-1].Close > resistanceLevel*(1+p.BreakoutConfirm)
	breakoutVolumeRatio := 0.0
	if breakoutDetected {
		volStart := max(n-p.VolAvgWindow, 0)
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

func collectFindSwingHighsDebug(candles []Candle, radius int) FindSwingHighsDebugSnapshot {
	n := len(candles)
	snap := FindSwingHighsDebugSnapshot{
		Radius:     radius,
		N:          n,
		Rows:       nil,
		SwingHighs: nil,
	}
	if n < radius*2+1 {
		return snap
	}
	for i := radius; i < n-radius; i++ {
		centerH := candles[i].High
		isHigh := true
		blockIdx := -1
		blockH := 0.0
		for j := i - radius; j <= i+radius; j++ {
			if j != i && candles[j].High >= centerH {
				isHigh = false
				blockIdx = j
				blockH = candles[j].High
				break
			}
		}

		snap.Rows = append(snap.Rows, SwingHighScanRow{
			Index:       i,
			High:        centerH,
			IsSwingHigh: isHigh,
			BlockIndex:  blockIdx,
			BlockHigh:   blockH,
		})

		if isHigh {
			snap.SwingHighs = append(snap.SwingHighs, SwingPoint{Index: i, Value: centerH})
		}
	}

	return snap
}

func formatFindSwingHighsDebug(s FindSwingHighsDebugSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "findSwingHighs step-by-step trace\n")
	fmt.Fprintf(&b, "radius=%d  n=%d candles  scanned indices [%d .. %d)\n\n",
		s.Radius, s.N, s.Radius, max(s.N-s.Radius, 0))

	if len(s.Rows) == 0 {
		fmt.Fprintf(&b, "(no indices scanned: need n >= 2*radius+1)\n")
		return b.String()
	}

	for _, row := range s.Rows {
		fmt.Fprintf(&b, "i=%d  High=%s\n", row.Index, atrFmt(row.High))
		if row.IsSwingHigh {
			fmt.Fprintf(&b, "  swing high: yes (strict max High on [%d .. %d] inclusive)\n",
				row.Index-s.Radius, row.Index+s.Radius)
		} else {
			fmt.Fprintf(&b, "  swing high: no — bar j=%d has High=%s >= center (first such j in window)\n",
				row.BlockIndex, atrFmt(row.BlockHigh))
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "swing highs count: %d\n", len(s.SwingHighs))
	for k, p := range s.SwingHighs {
		fmt.Fprintf(&b, "  [%d] index=%d value=%s\n", k, p.Index, atrFmt(p.Value))
	}
	return b.String()
}

func findSwingHighs(candles []Candle, radius int) []SwingPoint {
	return collectFindSwingHighsDebug(candles, radius).SwingHighs
}

func collectCalcATRDebug(candles []Candle) CalcATRDebugSnapshot {
	n := len(candles)
	out := CalcATRDebugSnapshot{BarCount: n, Bars: make([]CalcATRBarTrace, 0, n)}
	if n == 0 {
		return out
	}

	sum := candles[0].High - candles[0].Low
	c0 := candles[0]
	out.Bars = append(out.Bars, CalcATRBarTrace{
		Index:    0,
		FirstBar: true,
		O:        c0.Open, H: c0.High, L: c0.Low, C: c0.Close,
		HighLow: sum,
		TR:      sum,
		SumTR:   sum,
	})

	if n < 2 {
		out.SumTR = sum
		out.ATR = sum / float64(n)
		return out
	}

	for i := 1; i < n; i++ {
		c := candles[i]
		prevC := candles[i-1].Close
		hl := c.High - c.Low
		d1 := math.Abs(c.High - prevC)
		d2 := math.Abs(c.Low - prevC)

		tr := hl
		d1Took := d1 > tr
		if d1Took {
			tr = d1
		}
		d2Took := d2 > tr
		if d2Took {
			tr = d2
		}

		sum += tr
		out.Bars = append(out.Bars, CalcATRBarTrace{
			Index:    i,
			FirstBar: false,
			O:        c.Open, H: c.High, L: c.Low, C: c.Close,
			PrevClose: prevC,
			HighLow:   hl,
			D1:        d1,
			D2:        d2,
			D1TookTR:  d1Took,
			D2TookTR:  d2Took,
			TR:        tr,
			SumTR:     sum,
		})
	}

	out.SumTR = sum
	out.ATR = sum / float64(n)
	return out
}

func formatCalcATRDebug(s CalcATRDebugSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "calcATR step-by-step True Range trace\n")
	fmt.Fprintf(&b, "n=%d candles\n\n", s.BarCount)

	for _, row := range s.Bars {
		if row.FirstBar {
			fmt.Fprintf(&b, "i=0 (first bar: TR = High-Low only, no prevClose)\n")
			fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s\n", atrFmt(row.O), atrFmt(row.H), atrFmt(row.L), atrFmt(row.C))
			fmt.Fprintf(&b, "  tr = H-L = %s\n", atrFmt(row.HighLow))
			fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(row.SumTR))
			continue
		}

		fmt.Fprintf(&b, "i=%d\n", row.Index)
		fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s  prevClose=%s\n",
			atrFmt(row.O), atrFmt(row.H), atrFmt(row.L), atrFmt(row.C), atrFmt(row.PrevClose))
		fmt.Fprintf(&b, "  tr_initial (H-L) = %s\n", atrFmt(row.HighLow))
		fmt.Fprintf(&b, "  d1 = |H - prevClose| = %s\n", atrFmt(row.D1))
		fmt.Fprintf(&b, "  d2 = |L - prevClose| = %s\n", atrFmt(row.D2))

		if row.D1TookTR {
			fmt.Fprintf(&b, "  condition (d1 > tr): true -> tr := d1 = %s\n", atrFmt(row.D1))
		} else {
			fmt.Fprintf(&b, "  condition (d1 > tr): false (tr unchanged at %s)\n", atrFmt(row.HighLow))
		}

		trAfterD1 := row.HighLow
		if row.D1TookTR {
			trAfterD1 = row.D1
		}
		if row.D2TookTR {
			fmt.Fprintf(&b, "  condition (d2 > tr): true -> tr := d2 = %s\n", atrFmt(row.D2))
		} else {
			fmt.Fprintf(&b, "  condition (d2 > tr): false (final tr = %s)\n", atrFmt(trAfterD1))
		}

		fmt.Fprintf(&b, "  final TR for bar i=%d: %s\n", row.Index, atrFmt(row.TR))
		fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(row.SumTR))
	}

	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "sum(TR) = %s\n", atrFmt(s.SumTR))
	fmt.Fprintf(&b, "ATR = sum / n = %s / %d = %s\n", atrFmt(s.SumTR), s.BarCount, atrFmt(s.ATR))
	return b.String()
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

func calcATR(candles []Candle) float64 {
	return collectCalcATRDebug(candles).ATR
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

func collectFindHorizontalResistanceDebug(candles []Candle, highs []SwingPoint, vol float64, p DetectorParams) FindHorizontalResistanceDebugSnapshot {
	snap := FindHorizontalResistanceDebugSnapshot{
		Vol:     vol,
		HighsIn: append([]SwingPoint(nil), highs...),
	}
	if len(highs) < 2 {
		snap.FailReason = "few_input_highs (need >= 2 swing highs)"
		return snap
	}

	tolerance := math.Max(p.VolTolerance, vol*0.8)
	breakout := p.BreakoutTolerance
	minSpacing := p.MinResistanceSpacing
	snap.Tolerance = tolerance
	snap.Breakout = breakout
	snap.MinSpacing = minSpacing

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

	for _, g := range groups {
		avgAll := g.sum / float64(len(g.points))
		valid := []SwingPoint{g.points[0]}
		for i := 1; i < len(g.points); i++ {
			if g.points[i].Index-valid[len(valid)-1].Index >= minSpacing {
				valid = append(valid, g.points[i])
			}
		}
		snap.Groups = append(snap.Groups, HorizontalResistanceGroupDebug{
			Points:      append([]SwingPoint(nil), g.points...),
			Sum:         g.sum,
			AvgAll:      avgAll,
			SpacedValid: append([]SwingPoint(nil), valid...),
		})
	}

	bestLevel := 0.0
	maxTouches := 0
	bestGroupIdx := -1
	var bestTouchPoints []SwingPoint

	for gi, g := range groups {
		valid := snap.Groups[gi].SpacedValid
		if len(valid) < 2 {
			continue
		}
		if len(valid) > maxTouches {
			maxTouches = len(valid)
			avg := g.sum / float64(len(g.points))
			bestLevel = avg
			bestTouchPoints = valid
			bestGroupIdx = gi
		}
	}

	snap.BestGroupIdx = bestGroupIdx
	snap.BestLevel = bestLevel
	snap.BestTouchPoints = append([]SwingPoint(nil), bestTouchPoints...)

	if maxTouches < 2 {
		snap.FailReason = "no_group_with_>=2_spaced_touches (minSpacing between swing highs)"
		return snap
	}

	limit := bestLevel * (1 + breakout)
	snap.FailLimit = limit

	for i := 0; i < len(bestTouchPoints)-1; i++ {
		start := bestTouchPoints[i].Index
		end := bestTouchPoints[i+1].Index
		for j := start; j <= end && j < len(candles); j++ {
			if candles[j].Close > limit {
				snap.FailReason = "close_breakout_between_touch_indices"
				snap.FailPairIdx = i
				snap.FailBar = j
				snap.FailClose = candles[j].Close
				return snap
			}
		}
	}

	lastIdx := bestTouchPoints[len(bestTouchPoints)-1].Index
	for j := lastIdx; j < len(candles)-1; j++ {
		if candles[j].Close > limit {
			snap.FailReason = "close_breakout_after_last_touch"
			snap.FailPairIdx = -1
			snap.FailBar = j
			snap.FailClose = candles[j].Close
			return snap
		}
	}

	snap.Level = bestLevel
	snap.Touches = maxTouches
	snap.TouchPoints = append([]SwingPoint(nil), bestTouchPoints...)
	return snap
}

func formatFindHorizontalResistanceDebug(s FindHorizontalResistanceDebugSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "findHorizontalResistance step-by-step trace\n")
	fmt.Fprintf(&b, "input swing highs: %d  vol=%s\n\n", len(s.HighsIn), atrFmt(s.Vol))

	if s.FailReason != "" && s.Level == 0 && len(s.TouchPoints) == 0 {
		if strings.HasPrefix(s.FailReason, "few_input_highs") {
			fmt.Fprintf(&b, "%s\n", s.FailReason)
			return b.String()
		}
	}

	fmt.Fprintf(&b, "tolerance = max(0.002, vol*0.8) = max(0.002, %s) = %s\n",
		atrFmt(s.Vol*0.8), atrFmt(s.Tolerance))
	fmt.Fprintf(&b, "breakout threshold on Close: level * (1 + %.5f)\n", s.Breakout)
	fmt.Fprintf(&b, "min spacing between counted touches (bar index delta): %d\n\n", s.MinSpacing)

	for hi, p := range s.HighsIn {
		fmt.Fprintf(&b, "swing high [%d]: index=%d value=%s\n", hi, p.Index, atrFmt(p.Value))
	}
	fmt.Fprintf(&b, "\n--- clustering (each new high joins first group whose relative avg is within tolerance) ---\n")
	for gi, g := range s.Groups {
		fmt.Fprintf(&b, "group %d: avg(all points in cluster)=%s  sum=%s  raw points=%d\n",
			gi, atrFmt(g.AvgAll), atrFmt(g.Sum), len(g.Points))
		for _, p := range g.Points {
			fmt.Fprintf(&b, "    index=%d value=%s\n", p.Index, atrFmt(p.Value))
		}
		fmt.Fprintf(&b, "  after minSpacing filter (>= %d bars since previous kept touch):\n", s.MinSpacing)
		if len(g.SpacedValid) == 0 {
			fmt.Fprintf(&b, "    (none)\n")
		}
		for _, p := range g.SpacedValid {
			fmt.Fprintf(&b, "    index=%d value=%s\n", p.Index, atrFmt(p.Value))
		}
		fmt.Fprintf(&b, "  spaced touch count: %d\n\n", len(g.SpacedValid))
	}

	if s.BestGroupIdx >= 0 {
		fmt.Fprintf(&b, "--- best group (max spaced touch count) ---\n")
		fmt.Fprintf(&b, "bestGroupIdx=%d  resistance level (avg of all points in that cluster)=%s\n",
			s.BestGroupIdx, atrFmt(s.BestLevel))
		fmt.Fprintf(&b, "spaced touch points used for pattern:\n")
		for _, p := range s.BestTouchPoints {
			fmt.Fprintf(&b, "  index=%d value=%s\n", p.Index, atrFmt(p.Value))
		}
		fmt.Fprintf(&b, "\n")
	}

	if s.FailReason != "" {
		fmt.Fprintf(&b, "--- result: rejected ---\n")
		fmt.Fprintf(&b, "reason: %s\n", s.FailReason)
		if strings.HasPrefix(s.FailReason, "no_group") {
			return b.String()
		}
		fmt.Fprintf(&b, "max allowed Close (level * (1+breakout)) = %s\n", atrFmt(s.FailLimit))
		if s.FailPairIdx >= 0 {
			fmt.Fprintf(&b, "between touch pair index %d and %d in bestTouchPoints\n",
				s.FailPairIdx, s.FailPairIdx+1)
		} else if s.FailReason == "close_breakout_after_last_touch" {
			fmt.Fprintf(&b, "segment: from last touch index to end of window (excluding final bar in loop bound)\n")
		}
		fmt.Fprintf(&b, "first offending bar: j=%d  Close=%s\n", s.FailBar, atrFmt(s.FailClose))
		return b.String()
	}

	fmt.Fprintf(&b, "--- breakout check passed ---\n")
	fmt.Fprintf(&b, "no Close > %s between consecutive spaced touches (inclusive)\n", atrFmt(s.FailLimit))
	fmt.Fprintf(&b, "and no Close > %s from last touch through bar len(candles)-2\n\n", atrFmt(s.FailLimit))
	fmt.Fprintf(&b, "--- return ---\n")
	fmt.Fprintf(&b, "level=%s  touches=%d\n", atrFmt(s.Level), s.Touches)
	for i, p := range s.TouchPoints {
		fmt.Fprintf(&b, "  touch[%d] index=%d value=%s\n", i, p.Index, atrFmt(p.Value))
	}
	return b.String()
}

func findHorizontalResistance(candles []Candle, highs []SwingPoint, vol float64, p DetectorParams) (level float64, touches int, touchPoints []SwingPoint) {
	s := collectFindHorizontalResistanceDebug(candles, highs, vol, p)
	return s.Level, s.Touches, s.TouchPoints
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
