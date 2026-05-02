package detect

import (
	"triangle-detector/internal/domain"
"fmt"
"strings"
)

func collectFindSwingHighsDebug(candles []domain.Candle, radius int) FindSwingHighsDebugSnapshot {
n := len(candles)
snap := FindSwingHighsDebugSnapshot{
Radius:     radius,
N:          n,
Rows:       nil,
SwingHighs: nil,
}
if n < radius*2+1 {
return snap
}
for i := radius; i < n-radius; i++ {
centerH := candles[i].High
isHigh := true
blockIdx := -1
blockH := 0.0
for j := i - radius; j <= i+radius; j++ {
if j != i && candles[j].High >= centerH {
isHigh = false
blockIdx = j
blockH = candles[j].High
break
}
}
snap.Rows = append(snap.Rows, SwingHighScanRow{
Index:       i,
High:        centerH,
IsSwingHigh: isHigh,
BlockIndex:  blockIdx,
BlockHigh:   blockH,
})
if isHigh {
snap.SwingHighs = append(snap.SwingHighs, SwingPoint{Index: i, Value: centerH})
}
}
return snap
}

func formatFindSwingHighsDebug(s FindSwingHighsDebugSnapshot) string {
var b strings.Builder
fmt.Fprintf(&b, "findSwingHighs step-by-step trace\n")
fmt.Fprintf(&b, "radius=%d  n=%d candles  scanned indices [%d .. %d)\n\n",
s.Radius, s.N, s.Radius, max(s.N-s.Radius, 0))
if len(s.Rows) == 0 {
fmt.Fprintf(&b, "(no indices scanned: need n >= 2*radius+1)\n")
return b.String()
}
for _, row := range s.Rows {
fmt.Fprintf(&b, "i=%d  High=%s\n", row.Index, atrFmt(row.High))
if row.IsSwingHigh {
fmt.Fprintf(&b, "  swing high: yes (strict max High on [%d .. %d] inclusive)\n",
row.Index-s.Radius, row.Index+s.Radius)
} else {
fmt.Fprintf(&b, "  swing high: no -- bar j=%d has High=%s >= center (first such j in window)\n",
row.BlockIndex, atrFmt(row.BlockHigh))
}
fmt.Fprintf(&b, "\n")
}
fmt.Fprintf(&b, "---\n")
fmt.Fprintf(&b, "swing highs count: %d\n", len(s.SwingHighs))
for k, p := range s.SwingHighs {
fmt.Fprintf(&b, "  [%d] index=%d value=%s\n", k, p.Index, atrFmt(p.Value))
}
return b.String()
}

func findSwingHighs(candles []domain.Candle, radius int) []SwingPoint {
return collectFindSwingHighsDebug(candles, radius).SwingHighs
}



