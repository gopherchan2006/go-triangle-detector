package main

import "time"

type ChartRenderer interface {
	RenderCandles(candles []Candle)

	DrawHorizontalLine(level float64, fromIndex, toIndex int, label string)

	DrawTrendLine(slope, intercept float64, fromIndex, toIndex int, label string)

	DrawScatterMarkers(points []SwingPoint, label string, color string)

	AddStat(key, value string)

	SetCaption(symbol string, capturedAt time.Time)

	Export(filename string) error
}
