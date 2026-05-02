package detect

type SwingPoint struct {
	Index int
	Value float64
}

type ATRDebug struct {
	AvgPrice   float64
	ATRValue   float64
	Vol        float64
	CalcATRLog string
}

type SwingDebug struct {
	SwingHighsCount   int
	FindSwingHighsLog string
}

type ResistanceDebug struct {
	ResistanceLevel             float64
	ResistanceTouches           int
	FirstTouchIdx               int
	HighAboveThreshold          float64
	CrashThreshold              float64
	FindHorizontalResistanceLog string
}

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

type DebugInfo struct {
	ATR        ATRDebug
	Swing      SwingDebug
	Resistance ResistanceDebug
	Support    SupportDebug
	Geometry   GeometryDebug
}

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

type CalcATRDebugSnapshot struct {
	BarCount int
	Bars     []CalcATRBarTrace
	SumTR    float64
	ATR      float64
}

type SwingHighScanRow struct {
	Index       int
	High        float64
	IsSwingHigh bool
	BlockIndex  int
	BlockHigh   float64
}

type FindSwingHighsDebugSnapshot struct {
	Radius     int
	N          int
	Rows       []SwingHighScanRow
	SwingHighs []SwingPoint
}

type HorizontalResistanceGroupDebug struct {
	Points      []SwingPoint
	Sum         float64
	AvgAll      float64
	SpacedValid []SwingPoint
}

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
