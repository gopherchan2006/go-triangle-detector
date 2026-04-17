package main

import (
	"fmt"
	"math"
)

type SwingPoint struct {
	Index int
	Value float64
}

type AscendingTriangleResult struct {
	Found            bool
	ResistanceLevel  float64
	SupportSlope     float64
	SupportIntercept float64
}

func DetectAscendingTriangle(candles []Candle) AscendingTriangleResult {
	swingHighs := findSwingHighs(candles, 3)
	swingLows := findSwingLows(candles, 3)

	if len(swingHighs) < 2 || len(swingLows) < 2 {
		return AscendingTriangleResult{}
	}

	resistanceLevel, resistanceTouches := findHorizontalResistance(candles, swingHighs)
	if resistanceTouches < 2 {
		return AscendingTriangleResult{}
	}

	supportSlope, supportIntercept, supportTouches := findAscendingSupport(swingLows)
	if supportTouches < 2 {
		return AscendingTriangleResult{}
	}

	return AscendingTriangleResult{
		Found:            true,
		ResistanceLevel:  resistanceLevel,
		SupportSlope:     supportSlope,
		SupportIntercept: supportIntercept,
	}
}

func findSwingHighs(candles []Candle, minDistance int) []SwingPoint {
	var highs []SwingPoint
	for i := minDistance; i < len(candles)-minDistance; i++ {
		isHigh := true
		for j := i - minDistance; j <= i+minDistance; j++ {
			if j != i && candles[j].High >= candles[i].High {
				isHigh = false
				break
			}
		}
		if isHigh {
			highs = append(highs, SwingPoint{Index: i, Value: candles[i].High})
		}
	}
	return highs
}

func findSwingLows(candles []Candle, minDistance int) []SwingPoint {
	var lows []SwingPoint
	for i := minDistance; i < len(candles)-minDistance; i++ {
		isLow := true
		for j := i - minDistance; j <= i+minDistance; j++ {
			if j != i && candles[j].Low <= candles[i].Low {
				isLow = false
				break
			}
		}
		if isLow {
			lows = append(lows, SwingPoint{Index: i, Value: candles[i].Low})
		}
	}
	return lows
}

func findHorizontalResistance(candles []Candle, highs []SwingPoint) (level float64, touches int) {
	if len(highs) < 2 {
		return 0, 0
	}

	const tolerance = 0.025
	levelGroups := make(map[float64][]SwingPoint)

	for _, h := range highs {
		matched := false
		for lvl := range levelGroups {
			if math.Abs(h.Value-lvl)/lvl <= tolerance {
				fmt.Println(lvl)
				levelGroups[lvl] = append(levelGroups[lvl], h)
				matched = true
				break
			}
		}
		if !matched {
			levelGroups[h.Value] = []SwingPoint{h}
		}
	}

	maxTouches := 0
	bestLevel := 0.0
	for lvl, points := range levelGroups {
		if len(points) > maxTouches {
			maxTouches = len(points)
			bestLevel = lvl
		}
	}

	for _, c := range candles {
		if bestLevel <= c.Close {
			bestLevel = 0
			maxTouches = 0
		}
	}

	return bestLevel, maxTouches
}

func findAscendingSupport(lows []SwingPoint) (slope, intercept float64, touches int) {
	if len(lows) < 2 {
		return 0, 0, 0
	}

	n := float64(len(lows))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for _, low := range lows {
		x := float64(low.Index)
		y := low.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	if slope <= 0 {
		return 0, 0, 0
	}

	validTouches := 0
	for i := 1; i < len(lows); i++ {
		if lows[i].Value >= lows[i-1].Value*0.985 {
			validTouches++
		}
	}

	if validTouches >= len(lows)-1 {
		return slope, intercept, len(lows)
	}
	return 0, 0, 0
}
