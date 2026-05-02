package main

import "math"


func DetectAscendingTriangle(candles []Candle, opts ...DetectOption) AscendingTriangleResult {
	o := newDetectOpts(opts)
	return detectAscendingTriangle(candles, o)
}

func reject(reason RejectReason, o detectOpts) AscendingTriangleResult {
	o.counter.Inc(reason)
	return AscendingTriangleResult{RejectReason: reason}
}

func detectAscendingTriangle(candles []Candle, o detectOpts) AscendingTriangleResult {
	p := o.params
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
	if o.trace {
		dbg.CalcATRLog = formatCalcATRDebug(atrSnap)
	}

	swingSnap := collectFindSwingHighsDebug(candles, p.SwingRadius)
	swingHighs := swingSnap.SwingHighs
	if o.trace {
		dbg.FindSwingHighsLog = formatFindSwingHighsDebug(swingSnap)
	}
	dbg.SwingHighsCount = len(swingHighs)
	if len(swingHighs) < 2 {
		return reject(ReasonFewSwingHighs, o)
	}

	rhSnap := collectFindHorizontalResistanceDebug(candles, swingHighs, vol, p)
	if o.trace {
		dbg.FindHorizontalResistanceLog = formatFindHorizontalResistanceDebug(rhSnap)
	}
	resistanceLevel, resistanceTouches, resistanceTouchPoints := rhSnap.Level, rhSnap.Touches, rhSnap.TouchPoints
	dbg.ResistanceLevel = resistanceLevel
	dbg.ResistanceTouches = resistanceTouches
	if resistanceTouches < 3 {
		return reject(ReasonResistanceLt3Touches, o)
	}

	firstTouchIdx := resistanceTouchPoints[0].Index
	highAboveThreshold := resistanceLevel * (1 + vol*p.HighAboveVolMult)
	crashThreshold := resistanceLevel * (1 - math.Max(p.CrashVolMin, vol*8))
	dbg.FirstTouchIdx = firstTouchIdx
	dbg.HighAboveThreshold = highAboveThreshold
	dbg.CrashThreshold = crashThreshold
	for i := 0; i < firstTouchIdx; i++ {
		if candles[i].High > highAboveThreshold {
			return reject(ReasonHighBeforeFirstTouch, o)
		}
		if candles[i].Low < crashThreshold {
			return reject(ReasonCrashBeforeFirstTouch, o)
		}
	}

	if float64(firstTouchIdx) > float64(len(candles))*p.FirstTouchMaxRatio {
		return reject(ReasonFirstTouchTooLate, o)
	}

	if firstTouchIdx >= 5 {
		prePoints := make([]SwingPoint, 0, firstTouchIdx)
		for i := range firstTouchIdx {
			prePoints = append(prePoints, SwingPoint{Index: i, Value: candles[i].Close})
		}
		preSlope, _ := linearRegression(prePoints)
		if preSlope <= 0 {
			return reject(ReasonPrecedingTrendNotUp, o)
		}
	}

	valleys := findValleysBetweenTouches(candles, resistanceTouchPoints)
	dbg.ValleysCount = len(valleys)
	if len(valleys) < 2 {
		return reject(ReasonFewValleys, o)
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
		return reject(ReasonFirstValleyCrash, o)
	}

	allowedFlat := vol * p.AllowedFlatVolMult
	dbg.AllowedFlat = allowedFlat
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[i-1].Value*(1-allowedFlat) {
			return reject(ReasonValleyNotRising, o)
		}
	}

	floorTolerance := math.Max(p.FloorTolerance, vol)
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[0].Value*(1-floorTolerance) {
			return reject(ReasonFirstValleyNotFloor, o)
		}
	}

	supportSlope, supportIntercept := linearRegression(valleys)
	dbg.SupportSlope = supportSlope
	dbg.SupportIntercept = supportIntercept
	if supportSlope <= 0 {
		return reject(ReasonNegativeSlope, o)
	}

	maxValleyDepth := math.Max(p.MaxValleyDepthMin, vol*5)
	dbg.MaxValleyDepth = maxValleyDepth
	for _, v := range valleys {
		if v.Value < resistanceLevel*(1-maxValleyDepth) {
			return reject(ReasonValleyTooDeep, o)
		}
	}

	if len(valleys) >= 3 {
		if rSquared(valleys, supportSlope, supportIntercept) < p.MinRSquared {
			return reject(ReasonLowRSquared, o)
		}
	}

	valleyDeviation := math.Max(p.ValleyDeviationMin, vol*1.0)
	dbg.ValleyDeviation = valleyDeviation
	for _, v := range valleys {
		expected := supportSlope*float64(v.Index) + supportIntercept
		if expected > 0 && math.Abs(v.Value-expected)/expected > valleyDeviation {
			return reject(ReasonValleyOffSupportLine, o)
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
		return reject(ReasonNoConvergence, o)
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
			return reject(ReasonBreaksCeiling, o)
		}
	}

	floorTol := math.Max(p.FloorTolMin, vol*0.5)
	dbg.FloorTol = floorTol
	for i := patternStart; i <= patternEnd; i++ {
		supportVal := supportSlope*float64(i) + supportIntercept
		if candles[i].Low < supportVal*(1-floorTol) {
			return reject(ReasonBreaksSupportFloor, o)
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		if resistanceLevel <= supportSlope*float64(i)+supportIntercept {
			return reject(ReasonSupportAboveResistance, o)
		}
	}

	heightAtStart := resistanceLevel - (supportSlope*float64(patternStart) + supportIntercept)
	heightAtEnd := resistanceLevel - (supportSlope*float64(patternEnd) + supportIntercept)
	dbg.HeightAtStart = heightAtStart
	dbg.HeightAtEnd = heightAtEnd
	if heightAtEnd <= 0 || heightAtEnd >= heightAtStart*p.MaxNarrowingRatio {
		return reject(ReasonNotNarrowing, o)
	}

	if heightAtStart < resistanceLevel*p.MinPatternHeight {
		return reject(ReasonTooFlat, o)
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
		return reject(ReasonTooNarrow, o)
	}

	patternWidth := float64(pEnd - patternStart)
	dbg.PatternWidth = patternWidth
	if xIntersect > lastX+patternWidth*p.MaxApexFactor {
		return reject(ReasonApexTooFar, o)
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
			return reject(ReasonVolumeNotDeclining, o)
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

