package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type overlayKind string

const (
	kindHorizontal overlayKind = "horizontal"
	kindTrend      overlayKind = "trend"
	kindScatter    overlayKind = "scatter"
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
	points    []SwingPoint
}

type EChartsRenderer struct {
	candles    []Candle
	timestamps []string
	overlays   []overlay
	stats      []string
}

func NewEChartsRenderer() *EChartsRenderer {
	return &EChartsRenderer{}
}

func (r *EChartsRenderer) AddStat(key, value string) {
	r.stats = append(r.stats, key+": "+value)
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

func (r *EChartsRenderer) DrawScatterMarkers(points []SwingPoint, label string, color string) {
	r.overlays = append(r.overlays, overlay{
		kind:   kindScatter,
		label:  label,
		color:  color,
		points: points,
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
		case kindScatter:
			scatter := r.buildScatterOverlay(ov)
			kline.Overlap(scatter)
		}
	}

	page := components.NewPage()
	page.AddCharts(kline)

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return fmt.Errorf("failed to render page: %w", err)
	}

	html := strings.ReplaceAll(buf.String(), `"animation":true`, `"animation":false`)

	return os.WriteFile(filename, []byte(html), 0o644)
}

func (r *EChartsRenderer) buildSubtitle() string {
	base := fmt.Sprintf("Analysis of %d candles", len(r.candles))
	if len(r.stats) == 0 {
		return base
	}
	parts := []string{base}
	parts = append(parts, r.stats...)
	result := parts[0]
	for _, p := range parts[1:] {
		result += "  |  " + p
	}
	return result
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
			Subtitle: r.buildSubtitle(),
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

func (r *EChartsRenderer) buildScatterOverlay(ov overlay) *charts.Scatter {
	scatter := charts.NewScatter()

	data := make([]opts.ScatterData, len(r.candles))
	for _, p := range ov.points {
		if p.Index >= 0 && p.Index < len(r.candles) {
			data[p.Index] = opts.ScatterData{
				Value:      p.Value,
				Symbol:     "circle",
				SymbolSize: 12,
			}
		}
	}

	scatter.SetXAxis(r.timestamps)
	scatter.AddSeries(ov.label, data).
		SetSeriesOptions(
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: ov.color,
			}),
		)

	return scatter
}

func (r *EChartsRenderer) buildLineOverlay(ov overlay) *charts.Line {
	line := charts.NewLine()

	data := make([]opts.LineData, len(r.candles))
	for i := range data {
		data[i] = opts.LineData{Value: "-"}
	}

	switch ov.kind {
	case kindHorizontal:
		for i := ov.fromIdx; i <= ov.toIdx && i < len(r.candles); i++ {
			data[i] = opts.LineData{Value: ov.level}
		}
	case kindTrend:
		for i := ov.fromIdx; i <= ov.toIdx && i < len(r.candles); i++ {
			y := ov.slope*float64(i) + ov.intercept
			data[i] = opts.LineData{Value: y}
		}
	}

	line.SetXAxis(r.timestamps)
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
