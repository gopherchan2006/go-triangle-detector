package detect

import (
	"triangle-detector/internal/domain"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func collectCalcATRDebug(candles []domain.Candle) CalcATRDebugSnapshot {
	n := len(candles)
	out := CalcATRDebugSnapshot{BarCount: n, Bars: make([]CalcATRBarTrace, 0, n)}
	if n == 0 {
		return out
	}

	sum := candles[0].High - candles[0].Low
	c0 := candles[0]
	out.Bars = append(out.Bars, CalcATRBarTrace{
		Index:    0,
		FirstBar: true,
		O:        c0.Open, H: c0.High, L: c0.Low, C: c0.Close,
		HighLow: sum,
		TR:      sum,
		SumTR:   sum,
	})

	if n < 2 {
		out.SumTR = sum
		out.ATR = sum / float64(n)
		return out
	}

	for i := 1; i < n; i++ {
		c := candles[i]
		prevC := candles[i-1].Close
		hl := c.High - c.Low
		d1 := math.Abs(c.High - prevC)
		d2 := math.Abs(c.Low - prevC)

		tr := hl
		d1Took := d1 > tr
		if d1Took {
			tr = d1
		}
		d2Took := d2 > tr
		if d2Took {
			tr = d2
		}

		sum += tr
		out.Bars = append(out.Bars, CalcATRBarTrace{
			Index:    i,
			FirstBar: false,
			O:        c.Open, H: c.High, L: c.Low, C: c.Close,
			PrevClose: prevC,
			HighLow:   hl,
			D1:        d1,
			D2:        d2,
			D1TookTR:  d1Took,
			D2TookTR:  d2Took,
			TR:        tr,
			SumTR:     sum,
		})
	}

	out.SumTR = sum
	out.ATR = sum / float64(n)
	return out
}

func formatCalcATRDebug(s CalcATRDebugSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "calcATR step-by-step True Range trace\n")
	fmt.Fprintf(&b, "n=%d candles\n\n", s.BarCount)

	for _, row := range s.Bars {
		if row.FirstBar {
			fmt.Fprintf(&b, "i=0 (first bar: TR = High-Low only, no prevClose)\n")
			fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s\n", atrFmt(row.O), atrFmt(row.H), atrFmt(row.L), atrFmt(row.C))
			fmt.Fprintf(&b, "  tr = H-L = %s\n", atrFmt(row.HighLow))
			fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(row.SumTR))
			continue
		}

		fmt.Fprintf(&b, "i=%d\n", row.Index)
		fmt.Fprintf(&b, "  O=%s H=%s L=%s C=%s  prevClose=%s\n",
			atrFmt(row.O), atrFmt(row.H), atrFmt(row.L), atrFmt(row.C), atrFmt(row.PrevClose))
		fmt.Fprintf(&b, "  tr_initial (H-L) = %s\n", atrFmt(row.HighLow))
		fmt.Fprintf(&b, "  d1 = |H - prevClose| = %s\n", atrFmt(row.D1))
		fmt.Fprintf(&b, "  d2 = |L - prevClose| = %s\n", atrFmt(row.D2))

		if row.D1TookTR {
			fmt.Fprintf(&b, "  condition (d1 > tr): true -> tr := d1 = %s\n", atrFmt(row.D1))
		} else {
			fmt.Fprintf(&b, "  condition (d1 > tr): false (tr unchanged at %s)\n", atrFmt(row.HighLow))
		}

		trAfterD1 := row.HighLow
		if row.D1TookTR {
			trAfterD1 = row.D1
		}
		if row.D2TookTR {
			fmt.Fprintf(&b, "  condition (d2 > tr): true -> tr := d2 = %s\n", atrFmt(row.D2))
		} else {
			fmt.Fprintf(&b, "  condition (d2 > tr): false (final tr = %s)\n", atrFmt(trAfterD1))
		}

		fmt.Fprintf(&b, "  final TR for bar i=%d: %s\n", row.Index, atrFmt(row.TR))
		fmt.Fprintf(&b, "  running sum after this bar = %s\n\n", atrFmt(row.SumTR))
	}

	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "sum(TR) = %s\n", atrFmt(s.SumTR))
	fmt.Fprintf(&b, "ATR = sum / n = %s / %d = %s\n", atrFmt(s.SumTR), s.BarCount, atrFmt(s.ATR))
	return b.String()
}

func atrFmt(x float64) string {
	const scale = 1e8
	r := math.Round(x*scale) / scale
	if r == 0 || math.Abs(r) < 1e-12 {
		return "0"
	}
	s := strconv.FormatFloat(r, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func calcATR(candles []domain.Candle) float64 {
	return collectCalcATRDebug(candles).ATR
}



