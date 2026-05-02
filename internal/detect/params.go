package detect

// Params holds all tunable thresholds for the ascending triangle detector.
// Default values match the original hard-coded constants exactly.
type Params struct {
	SwingRadius          int
	VolTolerance         float64
	BreakoutTolerance    float64
	MinResistanceSpacing int
	FirstTouchMaxRatio   float64
	HighAboveVolMult     float64
	CrashVolMin          float64
	MaxFirstValleyCrash  float64
	AllowedFlatVolMult   float64
	FloorTolerance       float64
	MinRSquared          float64
	MaxValleyDepthMin    float64
	ValleyDeviationMin   float64
	CeilingTolMin        float64
	FloorTolMin          float64
	MaxNarrowingRatio    float64
	MinPatternHeight     float64
	MinPatternWidth      int
	MaxApexFactor        float64
	VolDeclSlopeMax      float64
	VolDeclMinWidth      int
	BreakoutConfirm      float64
	VolAvgWindow         int
}

// DefaultParams returns the original hard-coded thresholds.
func DefaultParams() Params {
	return Params{
		SwingRadius:          3,
		VolTolerance:         0.002,
		BreakoutTolerance:    0.005,
		MinResistanceSpacing: 5,
		FirstTouchMaxRatio:   2.0 / 5.0,
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
