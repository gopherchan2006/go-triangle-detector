package detect

// SwingPoint is an index + value pair representing a local high or low.
type SwingPoint struct {
	Index int
	Value float64
}

// ATRDebug holds ATR calculation diagnostics.
type ATRDebug struct {
	AvgPrice   float64
	ATRValue   float64
	Vol        float64
	CalcATRLog string
}

// SwingDebug holds swing high detection diagnostics.
type SwingDebug struct {
	SwingHighsCount   int
	FindSwingHighsLog string
}

// ResistanceDebug holds horizontal resistance diagnostics.
type ResistanceDebug struct {
	ResistanceLevel             float64
	ResistanceTouches           int
	FirstTouchIdx               int
	HighAboveThreshold          float64
	CrashThreshold              float64
	FindHorizontalResistanceLog string
}

// SupportDebug holds valley/support line diagnostics.
type SupportDebug struct {
	ValleysCount     int
	FirstVIdx        int
	MaxCrashRange    float64
	AllowedFlat      float64
	SupportSlope     float64
	SupportIntercept float64
	MaxValleyDepth   float64
	ValleyDeviation  float64
}

// GeometryDebug holds triangle geometry diagnostics.
type GeometryDebug struct {
	PatternStart      int
	PatternEnd        int
	XIntersect        float64
	LastX             float64
	CeilingTol        float64
	Ceiling           float64
	FloorTol          float64
	HeightAtStart     float64
	HeightAtEnd       float64
	LastResistanceIdx int
	LastValleyIdx     int
	PEnd              int
	PatternWidth      float64
}

// DebugInfo contains diagnostic values collected during a single detection run.
type DebugInfo struct {
	ATR        ATRDebug
	Swing      SwingDebug
	Resistance ResistanceDebug
	Support    SupportDebug
	Geometry   GeometryDebug
}

// Result is the output of DetectAscendingTriangle.
type Result struct {
	Found                 bool
	RejectReason          RejectReason
	ResistanceLevel       float64
	ResistanceTouches     int
	ResistanceTouchPoints []SwingPoint
	SupportSlope          float64
	SupportIntercept      float64
	SupportTouchPoints    []SwingPoint
	Debug                 DebugInfo
	TargetPrice           float64
	BreakoutDetected      bool
	BreakoutVolumeRatio   float64
}

// CalcATRBarTrace holds per-bar trace data for the ATR calculation.
type CalcATRBarTrace struct {
	Index                 int
	FirstBar              bool
	O, H, L, C, PrevClose float64

	HighLow  float64
	D1, D2   float64
	D1TookTR bool
	D2TookTR bool
	TR       float64
	SumTR    float64
}

// CalcATRDebugSnapshot is a full trace snapshot for calcATR.
type CalcATRDebugSnapshot struct {
	BarCount int
	Bars     []CalcATRBarTrace
	SumTR    float64
	ATR      float64
}

// SwingHighScanRow is a per-bar row produced during swing-high scanning.
type SwingHighScanRow struct {
	Index       int
	High        float64
	IsSwingHigh bool
	BlockIndex  int
	BlockHigh   float64
}

// FindSwingHighsDebugSnapshot is the full snapshot for findSwingHighs.
type FindSwingHighsDebugSnapshot struct {
	Radius     int
	N          int
	Rows       []SwingHighScanRow
	SwingHighs []SwingPoint
}

// HorizontalResistanceGroupDebug holds details about one resistance cluster.
type HorizontalResistanceGroupDebug struct {
	Points      []SwingPoint
	Sum         float64
	AvgAll      float64
	SpacedValid []SwingPoint
}

// FindHorizontalResistanceDebugSnapshot is the full snapshot for findHorizontalResistance.
type FindHorizontalResistanceDebugSnapshot struct {
	Vol             float64
	Tolerance       float64
	Breakout        float64
	MinSpacing      int
	HighsIn         []SwingPoint
	Groups          []HorizontalResistanceGroupDebug
	BestGroupIdx    int
	BestLevel       float64
	BestTouchPoints []SwingPoint
	FailReason      string
	FailPairIdx     int
	FailBar         int
	FailClose       float64
	FailLimit       float64
	Level           float64
	Touches         int
	TouchPoints     []SwingPoint
}
