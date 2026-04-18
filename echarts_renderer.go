package main

import (
	"fmt"
	"os"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type overlayKind string

const (
	kindHorizontal overlayKind = "horizontal"
	kindTrend      overlayKind = "trend"
)

type overlay struct {
	kind      overlayKind
	level     float64
	slope     float64
	intercept float64
	fromIdx   int
	toIdx     int
	label     string
	color     string
}

type EChartsRenderer struct {
	candles    []Candle
	timestamps []string
	overlays   []overlay
}

func NewEChartsRenderer() *EChartsRenderer {
	return &EChartsRenderer{}
}

func (r *EChartsRenderer) RenderCandles(candles []Candle) {
	r.candles = candles
	r.timestamps = make([]string, len(candles))
	for i, c := range candles {
		if !c.Timestamp.IsZero() {
			r.timestamps[i] = c.Timestamp.Format("01/02 15:04")
		} else {
			r.timestamps[i] = fmt.Sprintf("#%d", i)
		}
	}
}

func (r *EChartsRenderer) DrawHorizontalLine(level float64, fromIndex, toIndex int, label string) {
	r.overlays = append(r.overlays, overlay{
		kind:    kindHorizontal,
		level:   level,
		fromIdx: fromIndex,
		toIdx:   toIndex,
		label:   label,
		color:   "#ff4444",
	})
}

func (r *EChartsRenderer) DrawTrendLine(slope, intercept float64, fromIndex, toIndex int, label string) {
	r.overlays = append(r.overlays, overlay{
		kind:      kindTrend,
		slope:     slope,
		intercept: intercept,
		fromIdx:   fromIndex,
		toIdx:     toIndex,
		label:     label,
		color:     "#44dd44",
	})
}

func (r *EChartsRenderer) Export(filename string) error {
	if len(r.candles) == 0 {
		return fmt.Errorf("no candles to render")
	}

	kline := r.buildKlineChart()

	for _, ov := range r.overlays {
		switch ov.kind {
		case kindHorizontal, kindTrend:
			line := r.buildLineOverlay(ov)
			kline.Overlap(line)
		}
	}

	page := components.NewPage()
	page.AddCharts(kline)

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer f.Close()

	return page.Render(f)
}

func (r *EChartsRenderer) buildKlineChart() *charts.Kline {
	kline := charts.NewKLine()

	kline.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1400px",
			Height: "700px",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Horizontal Resistance Detector",
			Subtitle: fmt.Sprintf("Analysis of %d candles", len(r.candles)),
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: true,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "inside",
			XAxisIndex: []int{0},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "slider",
			XAxisIndex: []int{0},
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: true,
		}),
	)

	klineData := make([]opts.KlineData, len(r.candles))
	for i, c := range r.candles {
		klineData[i] = opts.KlineData{
			Value: [4]float32{
				float32(c.Open),
				float32(c.Close),
				float32(c.Low),
				float32(c.High),
			},
		}
	}

	kline.SetXAxis(r.timestamps)
	kline.AddSeries("Candles", klineData)

	return kline
}

func (r *EChartsRenderer) buildLineOverlay(ov overlay) *charts.Line {
	line := charts.NewLine()

	var data []opts.LineData
	switch ov.kind {
	case kindHorizontal:
		for i := ov.fromIdx; i <= ov.toIdx && i < len(r.candles); i++ {
			data = append(data, opts.LineData{Value: ov.level})
		}
	case kindTrend:
		for i := ov.fromIdx; i <= ov.toIdx && i < len(r.candles); i++ {
			y := ov.slope*float64(i) + ov.intercept
			data = append(data, opts.LineData{Value: y})
		}
	}

	line.AddSeries(ov.label, data).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: true,
			}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: ov.color,
			}),
		)

	return line
}
