package render

import (
	"time"

	"github.com/gopherchan2006/go-triangle-detector/internal/detect"
	"github.com/gopherchan2006/go-triangle-detector/internal/domain"
)

type ChartRenderer interface {
	RenderCandles(candles []domain.Candle)
	DrawHorizontalLine(level float64, fromIndex, toIndex int, label string)
	DrawTrendLine(slope, intercept float64, fromIndex, toIndex int, label string)
	DrawScatterMarkers(points []detect.SwingPoint, label string, color string)
	AddStat(key, value string)
	SetCaption(symbol string, capturedAt time.Time)
	Export(filename string) error
}
