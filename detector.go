package main

import (
	"math"
)

type SwingPoint struct {
	Index int
	Value float64
}

type DebugInfo struct {
	AvgPrice           float64
	ATR                float64
	Vol                float64
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

	atr := calcATR(candles)
	vol := atr / avgPrice
	dbg.AvgPrice = avgPrice
	dbg.ATR = atr
	dbg.Vol = vol

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

	supportSlope, supportIntercept := linearRegression(valleys)
	dbg.SupportSlope = supportSlope
	dbg.SupportIntercept = supportIntercept
	if supportSlope <= 0 {
		return reject("08_negative_slope", rejectStats)
	}

	valleySpan := float64(valleys[len(valleys)-1].Index - valleys[0].Index)
	minSlopeRise := resistanceLevel * math.Max(0.004, vol*1.2)
	if supportSlope*valleySpan < minSlopeRise {
		return reject("21_slope_too_shallow", rejectStats)
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

	ceilingTol := math.Max(0.002, vol*0.7)
	ceiling := resistanceLevel * (1 + ceilingTol)
	dbg.CeilingTol = ceilingTol
	dbg.Ceiling = ceiling
	for i := patternStart; i <= patternEnd; i++ {
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

	firstValleyValue := valleys[0].Value
	lastValleyValue := valleys[len(valleys)-1].Value
	initialGap := resistanceLevel - firstValleyValue
	actualRise := lastValleyValue - firstValleyValue
	if initialGap <= 0 || actualRise < initialGap*0.4 {
		return reject("22_lows_not_rising_enough", rejectStats)
	}

	midIdx := (patternStart + patternEnd) / 2
	firstHalfMinLow := math.MaxFloat64
	secondHalfMinLow := math.MaxFloat64
	for i := patternStart; i <= midIdx; i++ {
		if candles[i].Low < firstHalfMinLow {
			firstHalfMinLow = candles[i].Low
		}
	}
	for i := midIdx + 1; i <= patternEnd; i++ {
		if candles[i].Low < secondHalfMinLow {
			secondHalfMinLow = candles[i].Low
		}
	}
	minLowRise := math.Max(0.005, vol*1.5)
	if firstHalfMinLow > 0 && secondHalfMinLow < firstHalfMinLow*(1+minLowRise) {
		return reject("23_second_half_lows_too_low", rejectStats)
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

	return AscendingTriangleResult{
		Found:                 true,
		ResistanceLevel:       resistanceLevel,
		ResistanceTouches:     resistanceTouches,
		ResistanceTouchPoints: resistanceTouchPoints,
		SupportSlope:          supportSlope,
		SupportIntercept:      supportIntercept,
		SupportTouchPoints:    valleys,
		Debug:                 dbg,
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

func calcATR(candles []Candle) float64 {
	sum := candles[0].High - candles[0].Low
	if len(candles) < 2 {
		return sum
	}

	for i := 1; i < len(candles); i++ {
		tr := candles[i].High - candles[i].Low
		d1 := math.Abs(candles[i].High - candles[i-1].Close)
		d2 := math.Abs(candles[i].Low - candles[i-1].Close)
		if d1 > tr {
			tr = d1
		}
		if d2 > tr {
			tr = d2
		}
		sum += tr
	}
	return sum / float64(len(candles))
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

	lastIdx := bestTouchPoints[len(bestTouchPoints)-1].Index
	for j := lastIdx; j < len(candles); j++ {
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
