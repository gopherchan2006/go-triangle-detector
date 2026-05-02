package triangle

import (
	"github.com/gopherchan2006/go-triangle-detector/internal/detect"
	"github.com/gopherchan2006/go-triangle-detector/internal/domain"
)

type Candle = domain.Candle

type Result = detect.Result

type SwingPoint = detect.SwingPoint

type RejectReason = detect.RejectReason

type Params = detect.Params

type Option = detect.Option

const (
	ReasonFewSwingHighs          = detect.ReasonFewSwingHighs
	ReasonResistanceLt3Touches   = detect.ReasonResistanceLt3Touches
	ReasonHighBeforeFirstTouch   = detect.ReasonHighBeforeFirstTouch
	ReasonCrashBeforeFirstTouch  = detect.ReasonCrashBeforeFirstTouch
	ReasonFirstTouchTooLate      = detect.ReasonFirstTouchTooLate
	ReasonFewValleys             = detect.ReasonFewValleys
	ReasonValleyNotRising        = detect.ReasonValleyNotRising
	ReasonNegativeSlope          = detect.ReasonNegativeSlope
	ReasonValleyTooDeep          = detect.ReasonValleyTooDeep
	ReasonLowRSquared            = detect.ReasonLowRSquared
	ReasonValleyOffSupportLine   = detect.ReasonValleyOffSupportLine
	ReasonNoConvergence          = detect.ReasonNoConvergence
	ReasonBreaksCeiling          = detect.ReasonBreaksCeiling
	ReasonBreaksSupportFloor     = detect.ReasonBreaksSupportFloor
	ReasonSupportAboveResistance = detect.ReasonSupportAboveResistance
	ReasonNotNarrowing           = detect.ReasonNotNarrowing
	ReasonTooFlat                = detect.ReasonTooFlat
	ReasonTooNarrow              = detect.ReasonTooNarrow
	ReasonApexTooFar             = detect.ReasonApexTooFar
	ReasonFirstValleyCrash       = detect.ReasonFirstValleyCrash
	ReasonFirstValleyNotFloor    = detect.ReasonFirstValleyNotFloor
	ReasonPrecedingTrendNotUp    = detect.ReasonPrecedingTrendNotUp
	ReasonVolumeNotDeclining     = detect.ReasonVolumeNotDeclining
)

func Detect(candles []Candle, options ...Option) Result {
	return detect.DetectAscendingTriangle(candles, options...)
}

func WithTrace(on bool) Option {
	return detect.WithTrace(on)
}

func WithParams(p Params) Option {
	return detect.WithParams(p)
}

func DefaultParams() Params {
	return detect.DefaultParams()
}
