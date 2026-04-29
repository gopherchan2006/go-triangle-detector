package main

import "fmt"

func RenderTriangleDetection(
	candles []Candle,
	result AscendingTriangleResult,
	renderer ChartRenderer,
	outputPath string,
) error {
	renderer.RenderCandles(candles)

	if !result.Found {
		fmt.Println("Pattern not found. Saving clean chart.")
		return renderer.Export(outputPath)
	}

	renderer.DrawHorizontalLine(
		result.ResistanceLevel,
		0,
		len(candles)-1,
		fmt.Sprintf("Resistance %.2f", result.ResistanceLevel),
	)

	renderer.DrawScatterMarkers(result.ResistanceTouchPoints, "Resistance touches", "#ff4444")

	renderer.AddStat("Resistance", fmt.Sprintf("%.2f", result.ResistanceLevel))
	renderer.AddStat("Touches", fmt.Sprintf("%d", result.ResistanceTouches))

	supportFromIdx := 0
	if len(result.SupportTouchPoints) > 0 {
		supportFromIdx = result.SupportTouchPoints[0].Index
	}

	renderer.DrawTrendLine(
		result.SupportSlope,
		result.SupportIntercept,
		supportFromIdx,
		len(candles)-1,
		"Support",
	)

	renderer.DrawScatterMarkers(result.SupportTouchPoints, "Support touches", "#44dd44")

	fmt.Println("Pattern found!")
	fmt.Printf("  Resistance : %.2f\n", result.ResistanceLevel)
	fmt.Printf("  Support slope : %.4f\n", result.SupportSlope)

	return renderer.Export(outputPath)
}
