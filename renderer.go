package main

type ChartRenderer interface {
	RenderCandles(candles []Candle)

	DrawHorizontalLine(level float64, fromIndex, toIndex int, label string)

	DrawTrendLine(slope, intercept float64, fromIndex, toIndex int, label string)

	DrawScatterMarkers(points []SwingPoint, label string, color string)

	AddStat(key, value string)

	Export(filename string) error
}
