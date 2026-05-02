package detect

import (
	"fmt"
	"math"
	"strings"
	"triangle-detector/internal/domain"
)

func collectFindHorizontalResistanceDebug(candles []domain.Candle, highs []SwingPoint, vol float64, p Params) FindHorizontalResistanceDebugSnapshot {
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

func findHorizontalResistance(candles []domain.Candle, highs []SwingPoint, vol float64, p Params) (level float64, touches int, touchPoints []SwingPoint) {
	s := collectFindHorizontalResistanceDebug(candles, highs, vol, p)
	return s.Level, s.Touches, s.TouchPoints
}
