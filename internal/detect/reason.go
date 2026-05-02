package detect

// RejectReason is a typed string for detector rejection codes.
// Values are frozen — they match directory names under tmp/rejects/.
type RejectReason string

const (
	ReasonFewSwingHighs          RejectReason = "01_few_swing_highs"
	ReasonResistanceLt3Touches   RejectReason = "02_resistance_<3_touches"
	ReasonHighBeforeFirstTouch   RejectReason = "03_high_before_first_touch"
	ReasonCrashBeforeFirstTouch  RejectReason = "04_crash_before_first_touch"
	ReasonFirstTouchTooLate      RejectReason = "05_first_touch_too_late"
	ReasonFewValleys             RejectReason = "06_few_valleys"
	ReasonValleyNotRising        RejectReason = "07_valley_not_rising"
	ReasonNegativeSlope          RejectReason = "08_negative_slope"
	ReasonValleyTooDeep          RejectReason = "09_valley_too_deep"
	ReasonLowRSquared            RejectReason = "10_low_r_squared"
	ReasonValleyOffSupportLine   RejectReason = "11_valley_off_support_line"
	ReasonNoConvergence          RejectReason = "12_no_convergence"
	ReasonBreaksCeiling          RejectReason = "13_breaks_ceiling"
	ReasonBreaksSupportFloor     RejectReason = "14_breaks_support_floor"
	ReasonSupportAboveResistance RejectReason = "15_support_above_resistance"
	ReasonNotNarrowing           RejectReason = "16_not_narrowing"
	ReasonTooFlat                RejectReason = "17_too_flat"
	ReasonTooNarrow              RejectReason = "18_too_narrow"
	ReasonApexTooFar             RejectReason = "19_apex_too_far"
	ReasonFirstValleyCrash       RejectReason = "20_first_valley_crash"
	ReasonFirstValleyNotFloor    RejectReason = "21_first_valley_not_floor"
	ReasonPrecedingTrendNotUp    RejectReason = "24_preceding_trend_not_up"
	ReasonVolumeNotDeclining     RejectReason = "25_volume_not_declining"
)
