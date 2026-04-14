package main

import "math"

type SwingPoint struct {
	Index int
	Value float64
}

type AscendingTriangleResult struct {
	Found             bool
	ResistanceLevel   float64
	SupportSlope      float64
	SupportIntercept  float64
	BreakoutConfirmed bool
	BreakoutIndex     int
	TargetPrice       float64
	Score             float64
}

func DetectAscendingTriangle(candles []Candle) AscendingTriangleResult {
	swingHighs := findSwingHighs(candles, 3)
	swingLows := findSwingLows(candles, 3)

	if len(swingHighs) < 2 || len(swingLows) < 2 {
		return AscendingTriangleResult{}
	}

	resistanceLevel, resistanceTouches := findHorizontalResistance(swingHighs)
	if resistanceTouches < 2 {
		return AscendingTriangleResult{}
	}

	supportSlope, supportIntercept, supportTouches := findAscendingSupport(swingLows)
	if supportTouches < 2 {
		return AscendingTriangleResult{}
	}

	if !validateCorrectionMinima(swingHighs, swingLows, resistanceLevel) {
		return AscendingTriangleResult{}
	}

	breakoutConfirmed, breakoutIndex := checkBreakout(candles, resistanceLevel)

	height := calculateTriangleHeight(swingHighs, swingLows)
	targetPrice := resistanceLevel + height

	score := calculateScore(resistanceTouches, supportTouches, breakoutConfirmed, height)

	return AscendingTriangleResult{
		Found:             true,
		ResistanceLevel:   resistanceLevel,
		SupportSlope:      supportSlope,
		SupportIntercept:  supportIntercept,
		BreakoutConfirmed: breakoutConfirmed,
		BreakoutIndex:     breakoutIndex,
		TargetPrice:       targetPrice,
		Score:             score,
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

func findHorizontalResistance(highs []SwingPoint) (level float64, touches int) {
	if len(highs) < 2 {
		return 0, 0
	}

	const tolerance = 0.025
	levelGroups := make(map[float64][]SwingPoint)

	for _, h := range highs {
		matched := false
		for lvl := range levelGroups {
			if math.Abs(h.Value-lvl)/lvl <= tolerance {
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

func validateCorrectionMinima(highs []SwingPoint, lows []SwingPoint, resistance float64) bool {
	const tolerance = 0.025

	var resistanceIndices []int
	for _, h := range highs {
		if math.Abs(h.Value-resistance)/resistance <= tolerance {
			resistanceIndices = append(resistanceIndices, h.Index)
		}
	}

	if len(resistanceIndices) < 2 {
		return false
	}

	for i := 0; i < len(resistanceIndices)-1; i++ {
		start := resistanceIndices[i]
		end := resistanceIndices[i+1]

		minBetween := math.MaxFloat64
		for _, low := range lows {
			if low.Index > start && low.Index < end && low.Value < minBetween {
				minBetween = low.Value
			}
		}

		minBefore := math.MaxFloat64
		for _, low := range lows {
			if low.Index < start && low.Value < minBefore {
				minBefore = low.Value
			}
		}

		if minBetween == math.MaxFloat64 || minBefore == math.MaxFloat64 {
			continue
		}

		if minBetween <= minBefore {
			return false
		}
	}
	return true
}

func checkBreakout(candles []Candle, resistance float64) (confirmed bool, index int) {
	for i := len(candles) - 3; i < len(candles); i++ {
		if i >= 0 && candles[i].Close > resistance {
			return true, i
		}
	}
	return false, -1
}

func calculateTriangleHeight(highs []SwingPoint, lows []SwingPoint) float64 {
	if len(highs) == 0 || len(lows) == 0 {
		return 0
	}

	maxHigh := highs[0].Value
	for _, h := range highs {
		if h.Value > maxHigh {
			maxHigh = h.Value
		}
	}

	minLow := lows[0].Value
	for _, l := range lows {
		if l.Value < minLow {
			minLow = l.Value
		}
	}

	return maxHigh - minLow
}

func calculateScore(resTouches, supTouches int, breakout bool, height float64) float64 {
	score := 0.0

	score += float64(resTouches) * 0.2
	score += float64(supTouches) * 0.2

	if breakout {
		score += 0.3
	}

	if height > 0 {
		score += 0.3 * math.Min(height/100, 1)
	}

	return math.Min(score, 1.0)
}
