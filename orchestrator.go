package main

import "fmt"

func RenderTriangleDetection(
	candles []Candle,
	result AscendingTriangleResult,
	renderer ChartRenderer,
) error {
	renderer.RenderCandles(candles)

	if !result.Found {
		fmt.Println("Pattern not found. Saving clean chart.")
		
		return renderer.Export("chart.html")
	}

	renderer.DrawHorizontalLine(
		result.ResistanceLevel,
		0,
		len(candles)-1,
		fmt.Sprintf("Resistance %.2f", result.ResistanceLevel),
	)

	renderer.DrawTrendLine(
		result.SupportSlope,
		result.SupportIntercept,
		0,
		len(candles)-1,
		"Support",
	)

	if result.BreakoutConfirmed && result.BreakoutIndex >= 0 {
		renderer.MarkBreakout(result.BreakoutIndex)
	}

	renderer.DrawTargetLine(result.TargetPrice)

	fmt.Println("Pattern found!")
	fmt.Printf("  Resistance : %.2f\n", result.ResistanceLevel)
	fmt.Printf("  Support slope : %.4f\n", result.SupportSlope)
	fmt.Printf("  Target : %.2f\n", result.TargetPrice)
	fmt.Printf("  Score : %.2f / 1.00\n", result.Score)
	if result.BreakoutConfirmed {
		fmt.Printf("  Breakout : candle #%d ✓\n", result.BreakoutIndex)
	} else {
		fmt.Println("  Breakout : pending")
	}

	return renderer.Export("chart.html")
}
