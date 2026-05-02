package main

import "math"

// pipeCtx carries mutable state threaded through all pipeline steps.
type pipeCtx struct {
	candles []Candle
	opts    detectOpts
	p       DetectorParams
	dbg     DebugInfo

	avgPrice              float64
	vol                   float64
	swingHighs            []SwingPoint
	resistanceLevel       float64
	resistanceTouches     int
	resistanceTouchPoints []SwingPoint
	valleys               []SwingPoint
	supportSlope          float64
	supportIntercept      float64
	patternStart          int
	patternEnd            int
	xIntersect            float64
	lastX                 float64

	rejected *AscendingTriangleResult
}

// pipeStep processes one stage of the detection pipeline.
type pipeStep func(ctx *pipeCtx)

// rejectCtx records a rejection and stops further pipeline steps.
func rejectCtx(ctx *pipeCtx, reason RejectReason) {
	ctx.opts.counter.Inc(reason)
	res := AscendingTriangleResult{RejectReason: reason}
	ctx.rejected = &res
}

func DetectAscendingTriangle(candles []Candle, opts ...DetectOption) AscendingTriangleResult {
	o := newDetectOpts(opts)
	return detectAscendingTriangle(candles, o)
}

func detectAscendingTriangle(candles []Candle, o detectOpts) AscendingTriangleResult {
	ctx := &pipeCtx{candles: candles, opts: o, p: o.params}

	for _, step := range []pipeStep{
		stepCalcATR,
		stepFindSwingHighs,
		stepFindResistance,
		stepCheckTimingAndHighs,
		stepFindValleys,
		stepValidateValleys,
		stepFitSupportLine,
		stepCheckGeometry,
		stepCheckVolume,
	} {
		step(ctx)
		if ctx.rejected != nil {
			return *ctx.rejected
		}
	}

	return buildDetectResult(ctx)
}

func stepCalcATR(ctx *pipeCtx) {
	sum := 0.0
	for _, c := range ctx.candles {
		sum += c.Close
	}
	ctx.avgPrice = sum / float64(len(ctx.candles))

	atrSnap := collectCalcATRDebug(ctx.candles)
	ctx.vol = atrSnap.ATR / ctx.avgPrice
	ctx.dbg.AvgPrice = ctx.avgPrice
	ctx.dbg.ATR = atrSnap.ATR
	ctx.dbg.Vol = ctx.vol
	if ctx.opts.trace {
		ctx.dbg.CalcATRLog = formatCalcATRDebug(atrSnap)
	}
}

func stepFindSwingHighs(ctx *pipeCtx) {
	snap := collectFindSwingHighsDebug(ctx.candles, ctx.p.SwingRadius)
	ctx.swingHighs = snap.SwingHighs
	if ctx.opts.trace {
		ctx.dbg.FindSwingHighsLog = formatFindSwingHighsDebug(snap)
	}
	ctx.dbg.SwingHighsCount = len(ctx.swingHighs)
	if len(ctx.swingHighs) < 2 {
		rejectCtx(ctx, ReasonFewSwingHighs)
	}
}

func stepFindResistance(ctx *pipeCtx) {
	snap := collectFindHorizontalResistanceDebug(ctx.candles, ctx.swingHighs, ctx.vol, ctx.p)
	if ctx.opts.trace {
		ctx.dbg.FindHorizontalResistanceLog = formatFindHorizontalResistanceDebug(snap)
	}
	ctx.resistanceLevel = snap.Level
	ctx.resistanceTouches = snap.Touches
	ctx.resistanceTouchPoints = snap.TouchPoints
	ctx.dbg.ResistanceLevel = snap.Level
	ctx.dbg.ResistanceTouches = snap.Touches
	if snap.Touches < 3 {
		rejectCtx(ctx, ReasonResistanceLt3Touches)
	}
}

func stepCheckTimingAndHighs(ctx *pipeCtx) {
	p := ctx.p
	firstTouchIdx := ctx.resistanceTouchPoints[0].Index
	highAboveThreshold := ctx.resistanceLevel * (1 + ctx.vol*p.HighAboveVolMult)
	crashThreshold := ctx.resistanceLevel * (1 - math.Max(p.CrashVolMin, ctx.vol*8))
	ctx.dbg.FirstTouchIdx = firstTouchIdx
	ctx.dbg.HighAboveThreshold = highAboveThreshold
	ctx.dbg.CrashThreshold = crashThreshold

	for i := 0; i < firstTouchIdx; i++ {
		if ctx.candles[i].High > highAboveThreshold {
			rejectCtx(ctx, ReasonHighBeforeFirstTouch)
			return
		}
		if ctx.candles[i].Low < crashThreshold {
			rejectCtx(ctx, ReasonCrashBeforeFirstTouch)
			return
		}
	}

	if float64(firstTouchIdx) > float64(len(ctx.candles))*p.FirstTouchMaxRatio {
		rejectCtx(ctx, ReasonFirstTouchTooLate)
		return
	}

	if firstTouchIdx >= 5 {
		prePoints := make([]SwingPoint, 0, firstTouchIdx)
		for i := range firstTouchIdx {
			prePoints = append(prePoints, SwingPoint{Index: i, Value: ctx.candles[i].Close})
		}
		preSlope, _ := linearRegression(prePoints)
		if preSlope <= 0 {
			rejectCtx(ctx, ReasonPrecedingTrendNotUp)
		}
	}
}

func stepFindValleys(ctx *pipeCtx) {
	ctx.valleys = findValleysBetweenTouches(ctx.candles, ctx.resistanceTouchPoints)
	ctx.dbg.ValleysCount = len(ctx.valleys)
	if len(ctx.valleys) < 2 {
		rejectCtx(ctx, ReasonFewValleys)
	}
}

func stepValidateValleys(ctx *pipeCtx) {
	p := ctx.p
	candles := ctx.candles
	valleys := ctx.valleys

	firstVIdx := valleys[0].Index
	maxCrashRange := 0.0
	for k := firstVIdx - 2; k <= firstVIdx; k++ {
		if k >= 0 {
			r := (candles[k].High - candles[k].Low) / ctx.avgPrice
			if r > maxCrashRange {
				maxCrashRange = r
			}
		}
	}
	ctx.dbg.FirstVIdx = firstVIdx
	ctx.dbg.MaxCrashRange = maxCrashRange
	if maxCrashRange > math.Max(p.MaxFirstValleyCrash, ctx.vol*4) {
		rejectCtx(ctx, ReasonFirstValleyCrash)
		return
	}

	allowedFlat := ctx.vol * p.AllowedFlatVolMult
	ctx.dbg.AllowedFlat = allowedFlat
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[i-1].Value*(1-allowedFlat) {
			rejectCtx(ctx, ReasonValleyNotRising)
			return
		}
	}

	floorTolerance := math.Max(p.FloorTolerance, ctx.vol)
	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value < valleys[0].Value*(1-floorTolerance) {
			rejectCtx(ctx, ReasonFirstValleyNotFloor)
			return
		}
	}

	maxValleyDepth := math.Max(p.MaxValleyDepthMin, ctx.vol*5)
	ctx.dbg.MaxValleyDepth = maxValleyDepth
	for _, v := range valleys {
		if v.Value < ctx.resistanceLevel*(1-maxValleyDepth) {
			rejectCtx(ctx, ReasonValleyTooDeep)
			return
		}
	}
}

func stepFitSupportLine(ctx *pipeCtx) {
	p := ctx.p
	valleys := ctx.valleys

	slope, intercept := linearRegression(valleys)
	ctx.supportSlope = slope
	ctx.supportIntercept = intercept
	ctx.dbg.SupportSlope = slope
	ctx.dbg.SupportIntercept = intercept

	if slope <= 0 {
		rejectCtx(ctx, ReasonNegativeSlope)
		return
	}

	if len(valleys) >= 3 {
		if rSquared(valleys, slope, intercept) < p.MinRSquared {
			rejectCtx(ctx, ReasonLowRSquared)
			return
		}
	}

	valleyDeviation := math.Max(p.ValleyDeviationMin, ctx.vol*1.0)
	ctx.dbg.ValleyDeviation = valleyDeviation
	for _, v := range valleys {
		expected := slope*float64(v.Index) + intercept
		if expected > 0 && math.Abs(v.Value-expected)/expected > valleyDeviation {
			rejectCtx(ctx, ReasonValleyOffSupportLine)
			return
		}
	}
}

func stepCheckGeometry(ctx *pipeCtx) {
	p := ctx.p
	candles := ctx.candles
	valleys := ctx.valleys

	patternStart := ctx.resistanceTouchPoints[0].Index
	if valleys[0].Index < patternStart {
		patternStart = valleys[0].Index
	}
	patternEnd := len(candles) - 1
	ctx.patternStart = patternStart
	ctx.patternEnd = patternEnd
	ctx.dbg.PatternStart = patternStart
	ctx.dbg.PatternEnd = patternEnd

	xIntersect := (ctx.resistanceLevel - ctx.supportIntercept) / ctx.supportSlope
	lastX := float64(len(candles) - 1)
	ctx.xIntersect = xIntersect
	ctx.lastX = lastX
	ctx.dbg.XIntersect = xIntersect
	ctx.dbg.LastX = lastX
	if xIntersect <= lastX {
		rejectCtx(ctx, ReasonNoConvergence)
		return
	}

	ceilingTol := math.Max(p.CeilingTolMin, ctx.vol*0.7)
	ceiling := ctx.resistanceLevel * (1 + ceilingTol)
	ctx.dbg.CeilingTol = ceilingTol
	ctx.dbg.Ceiling = ceiling
	ceilingEnd := patternEnd
	if ceilingEnd == len(candles)-1 {
		ceilingEnd = patternEnd - 1
	}
	for i := patternStart; i <= ceilingEnd; i++ {
		if candles[i].High > ceiling {
			rejectCtx(ctx, ReasonBreaksCeiling)
			return
		}
	}

	floorTol := math.Max(p.FloorTolMin, ctx.vol*0.5)
	ctx.dbg.FloorTol = floorTol
	for i := patternStart; i <= patternEnd; i++ {
		supportVal := ctx.supportSlope*float64(i) + ctx.supportIntercept
		if candles[i].Low < supportVal*(1-floorTol) {
			rejectCtx(ctx, ReasonBreaksSupportFloor)
			return
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		if ctx.resistanceLevel <= ctx.supportSlope*float64(i)+ctx.supportIntercept {
			rejectCtx(ctx, ReasonSupportAboveResistance)
			return
		}
	}

	heightAtStart := ctx.resistanceLevel - (ctx.supportSlope*float64(patternStart) + ctx.supportIntercept)
	heightAtEnd := ctx.resistanceLevel - (ctx.supportSlope*float64(patternEnd) + ctx.supportIntercept)
	ctx.dbg.HeightAtStart = heightAtStart
	ctx.dbg.HeightAtEnd = heightAtEnd
	if heightAtEnd <= 0 || heightAtEnd >= heightAtStart*p.MaxNarrowingRatio {
		rejectCtx(ctx, ReasonNotNarrowing)
		return
	}

	if heightAtStart < ctx.resistanceLevel*p.MinPatternHeight {
		rejectCtx(ctx, ReasonTooFlat)
		return
	}

	lastResistanceIdx := ctx.resistanceTouchPoints[len(ctx.resistanceTouchPoints)-1].Index
	lastValleyIdx := valleys[len(valleys)-1].Index
	pEnd := lastResistanceIdx
	if lastValleyIdx > pEnd {
		pEnd = lastValleyIdx
	}
	ctx.dbg.LastResistanceIdx = lastResistanceIdx
	ctx.dbg.LastValleyIdx = lastValleyIdx
	ctx.dbg.PEnd = pEnd
	if pEnd-patternStart < p.MinPatternWidth {
		rejectCtx(ctx, ReasonTooNarrow)
		return
	}

	patternWidth := float64(pEnd - patternStart)
	ctx.dbg.PatternWidth = patternWidth
	if xIntersect > lastX+patternWidth*p.MaxApexFactor {
		rejectCtx(ctx, ReasonApexTooFar)
	}
}

func stepCheckVolume(ctx *pipeCtx) {
	p := ctx.p
	patternStart := ctx.patternStart
	pEnd := ctx.dbg.PEnd
	if pEnd-patternStart < p.VolDeclMinWidth {
		return
	}

	volPoints := make([]SwingPoint, 0, pEnd-patternStart+1)
	volSum := 0.0
	for i := patternStart; i <= pEnd; i++ {
		volPoints = append(volPoints, SwingPoint{Index: i, Value: ctx.candles[i].Volume})
		volSum += ctx.candles[i].Volume
	}
	avgVol := volSum / float64(len(volPoints))
	volSlope, _ := linearRegression(volPoints)
	if avgVol > 0 && volSlope/avgVol > p.VolDeclSlopeMax {
		rejectCtx(ctx, ReasonVolumeNotDeclining)
	}
}

func buildDetectResult(ctx *pipeCtx) AscendingTriangleResult {
	p := ctx.p
	candles := ctx.candles
	n := len(candles)

	targetPrice := ctx.resistanceLevel + (ctx.resistanceLevel - ctx.valleys[0].Value)

	breakoutDetected := candles[n-1].Close > ctx.resistanceLevel*(1+p.BreakoutConfirm)
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
		ResistanceLevel:       ctx.resistanceLevel,
		ResistanceTouches:     ctx.resistanceTouches,
		ResistanceTouchPoints: ctx.resistanceTouchPoints,
		SupportSlope:          ctx.supportSlope,
		SupportIntercept:      ctx.supportIntercept,
		SupportTouchPoints:    ctx.valleys,
		Debug:                 ctx.dbg,
		TargetPrice:           targetPrice,
		BreakoutDetected:      breakoutDetected,
		BreakoutVolumeRatio:   breakoutVolumeRatio,
	}
}
