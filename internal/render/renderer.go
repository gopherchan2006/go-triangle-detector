package render

import (
	"time"

	"triangle-detector/internal/detect"
	"triangle-detector/internal/domain"
)

// ChartRenderer is the port for chart rendering backends.
type ChartRenderer interface {
	RenderCandles(candles []domain.Candle)
	DrawHorizontalLine(level float64, fromIndex, toIndex int, label string)
	DrawTrendLine(slope, intercept float64, fromIndex, toIndex int, label string)
	DrawScatterMarkers(points []detect.SwingPoint, label string, color string)
	AddStat(key, value string)
	SetCaption(symbol string, capturedAt time.Time)
	Export(filename string) error
}
