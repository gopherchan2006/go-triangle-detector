package main

// DetectorParams holds all tunable thresholds for the ascending triangle detector.
// Default values match the original hard-coded constants exactly.
type DetectorParams struct {
	// SwingRadius is the lookback/lookahead window for swing high detection.
	SwingRadius int
	// VolTolerance is the floor for resistance-level grouping tolerance.
	// Actual tolerance = max(VolTolerance, vol*0.8).
	VolTolerance float64
	// BreakoutTolerance: a Close > resistance*(1+BreakoutTolerance) between touches fails the pattern.
	BreakoutTolerance float64
	// MinResistanceSpacing is the minimum bar gap between two counted resistance touches.
	MinResistanceSpacing int
	// FirstTouchMaxRatio: first touch must be at index <= len(candles)*FirstTouchMaxRatio.
	FirstTouchMaxRatio float64
	// HighAboveVolMult: spike filter — High > resistance*(1 + vol*HighAboveVolMult) before first touch rejects.
	HighAboveVolMult float64
	// CrashVolMin: crash filter — Low < resistance*(1 - max(CrashVolMin, vol*8)) before first touch rejects.
	CrashVolMin float64
	// MaxFirstValleyCrash: candle range / avgPrice in first valley area must not exceed max(MaxFirstValleyCrash, vol*4).
	MaxFirstValleyCrash float64
	// AllowedFlatVolMult: valley[i] >= valley[i-1]*(1 - vol*AllowedFlatVolMult) — valleys must not drop by more.
	AllowedFlatVolMult float64
	// FloorTolerance: floor for first-valley-not-floor check: max(FloorTolerance, vol).
	FloorTolerance float64
	// MinRSquared is the minimum R² required when 3+ valleys are present.
	MinRSquared float64
	// MaxValleyDepthMin: valley Low must not be below resistance*(1 - max(MaxValleyDepthMin, vol*5)).
	MaxValleyDepthMin float64
	// ValleyDeviationMin: per-valley deviation from support line must not exceed max(ValleyDeviationMin, vol).
	ValleyDeviationMin float64
	// CeilingTolMin: ceiling = resistance*(1 + max(CeilingTolMin, vol*0.7)).
	CeilingTolMin float64
	// FloorTolMin: support floor tolerance = max(FloorTolMin, vol*0.5).
	FloorTolMin float64
	// MaxNarrowingRatio: heightAtEnd must be < heightAtStart*MaxNarrowingRatio to confirm narrowing.
	MaxNarrowingRatio float64
	// MinPatternHeight: heightAtStart must be >= resistance*MinPatternHeight.
	MinPatternHeight float64
	// MinPatternWidth is the minimum number of bars for a valid pattern.
	MinPatternWidth int
	// MaxApexFactor: apex (xIntersect) must be within lastX + patternWidth*MaxApexFactor.
	MaxApexFactor float64
	// VolDeclSlopeMax: volume slope / avgVol must not exceed this for the volume-declining check.
	VolDeclSlopeMax float64
	// VolDeclMinWidth: minimum pattern width (bars) to run the volume-declining check.
	VolDeclMinWidth int
	// BreakoutConfirm: last candle Close > resistance*(1+BreakoutConfirm) → BreakoutDetected.
	BreakoutConfirm float64
	// VolAvgWindow: how many bars to average volume for BreakoutVolumeRatio.
	VolAvgWindow int
}

// DefaultDetectorParams returns the original hard-coded thresholds.
func DefaultDetectorParams() DetectorParams {
	return DetectorParams{
		SwingRadius:          3,
		VolTolerance:         0.002,
		BreakoutTolerance:    0.005,
		MinResistanceSpacing: 5,
		FirstTouchMaxRatio:   2.0 / 5.0, // 0.4 — original: len(candles)*2/5
		HighAboveVolMult:     0.5,
		CrashVolMin:          0.05,
		MaxFirstValleyCrash:  0.015,
		AllowedFlatVolMult:   1.5,
		FloorTolerance:       0.003,
		MinRSquared:          0.85,
		MaxValleyDepthMin:    0.015,
		ValleyDeviationMin:   0.0015,
		CeilingTolMin:        0.002,
		FloorTolMin:          0.0015,
		MaxNarrowingRatio:    0.7,
		MinPatternHeight:     0.005,
		MinPatternWidth:      15,
		MaxApexFactor:        2.0,
		VolDeclSlopeMax:      0.01,
		VolDeclMinWidth:      10,
		BreakoutConfirm:      0.005,
		VolAvgWindow:         20,
	}
}
