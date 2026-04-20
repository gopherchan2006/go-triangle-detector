package main

import (
	"math"
)

type SwingPoint struct {
	Index int
	Value float64
}

type AscendingTriangleResult struct {
	Found                 bool
	ResistanceLevel       float64
	ResistanceTouches     int
	ResistanceTouchPoints []SwingPoint
	SupportSlope          float64
	SupportIntercept      float64
	SupportTouchPoints    []SwingPoint
}

func DetectAscendingTriangle(candles []Candle) AscendingTriangleResult {
	const swingRadius = 3

	swingHighs := findSwingHighs(candles, swingRadius)
	if len(swingHighs) < 2 {
		return AscendingTriangleResult{}
	}

	resistanceLevel, resistanceTouches, resistanceTouchPoints := findHorizontalResistance(candles, swingHighs)
	if resistanceTouches < 3 {
		return AscendingTriangleResult{}
	}

	firstTouchIdx := resistanceTouchPoints[0].Index
	for i := 0; i < firstTouchIdx; i++ {
		if candles[i].High > resistanceLevel*1.0015 {
			return AscendingTriangleResult{}
		}

		if candles[i].Low < resistanceLevel*0.985 {
			return AscendingTriangleResult{}
		}
	}

	if firstTouchIdx > len(candles)*2/5 {
		return AscendingTriangleResult{}
	}

	valleys := findValleysBetweenTouches(candles, resistanceTouchPoints)
	if len(valleys) < 2 {
		return AscendingTriangleResult{}
	}

	for i := 1; i < len(valleys); i++ {
		if valleys[i].Value <= valleys[i-1].Value {
			return AscendingTriangleResult{}
		}
	}

	supportSlope, supportIntercept := linearRegression(valleys)
	if supportSlope <= 0 {
		return AscendingTriangleResult{}
	}

	for _, v := range valleys {
		if v.Value < resistanceLevel*0.985 {
			return AscendingTriangleResult{}
		}
	}

	if len(valleys) >= 3 {
		if rSquared(valleys, supportSlope, supportIntercept) < 0.85 {
			return AscendingTriangleResult{}
		}
	}

	for _, v := range valleys {
		expected := supportSlope*float64(v.Index) + supportIntercept
		if expected > 0 && math.Abs(v.Value-expected)/expected > 0.0015 {
			return AscendingTriangleResult{}
		}
	}

	patternStart := resistanceTouchPoints[0].Index
	if valleys[0].Index < patternStart {
		patternStart = valleys[0].Index
	}
	patternEnd := len(candles) - 1

	xIntersect := (resistanceLevel - supportIntercept) / supportSlope
	lastX := float64(len(candles) - 1)
	if xIntersect <= lastX {
		return AscendingTriangleResult{}
	}

	ceiling := resistanceLevel * 1.002
	for i := patternStart; i <= patternEnd; i++ {
		if candles[i].High > ceiling {
			return AscendingTriangleResult{}
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		supportVal := supportSlope*float64(i) + supportIntercept
		if candles[i].Low < supportVal*(1-0.0015) {
			return AscendingTriangleResult{}
		}
	}

	for i := patternStart; i <= patternEnd; i++ {
		if resistanceLevel <= supportSlope*float64(i)+supportIntercept {
			return AscendingTriangleResult{}
		}
	}

	heightAtStart := resistanceLevel - (supportSlope*float64(patternStart) + supportIntercept)
	heightAtEnd := resistanceLevel - (supportSlope*float64(patternEnd) + supportIntercept)
	if heightAtEnd <= 0 || heightAtEnd >= heightAtStart*0.7 {
		return AscendingTriangleResult{}
	}

	if heightAtStart < resistanceLevel*0.005 {
		return AscendingTriangleResult{}
	}

	lastResistanceIdx := resistanceTouchPoints[len(resistanceTouchPoints)-1].Index
	lastValleyIdx := valleys[len(valleys)-1].Index
	pEnd := lastResistanceIdx
	if lastValleyIdx > pEnd {
		pEnd = lastValleyIdx
	}
	if pEnd-patternStart < 15 {
		return AscendingTriangleResult{}
	}

	patternWidth := float64(pEnd - patternStart)
	if xIntersect > lastX+patternWidth*2 {
		return AscendingTriangleResult{}
	}

	return AscendingTriangleResult{
		Found:                 true,
		ResistanceLevel:       resistanceLevel,
		ResistanceTouches:     resistanceTouches,
		ResistanceTouchPoints: resistanceTouchPoints,
		SupportSlope:          supportSlope,
		SupportIntercept:      supportIntercept,
		SupportTouchPoints:    valleys,
	}
}

func findSwingHighs(candles []Candle, radius int) []SwingPoint {
	var highs []SwingPoint
	for i := radius; i < len(candles)-radius; i++ {
		isHigh := true
		for j := i - radius; j <= i+radius; j++ {
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

func findValleysBetweenTouches(candles []Candle, touches []SwingPoint) []SwingPoint {
	var valleys []SwingPoint

	for i := 0; i < len(touches)-1; i++ {
		start := touches[i].Index + 1
		end := touches[i+1].Index
		if end-start < 2 {
			continue
		}
		valleys = append(valleys, findLowestLow(candles, start, end))
	}

	lastTouch := touches[len(touches)-1].Index
	if len(candles)-1-lastTouch >= 5 {
		valleys = append(valleys, findLowestLow(candles, lastTouch+1, len(candles)))
	}

	return valleys
}

func findLowestLow(candles []Candle, from, to int) SwingPoint {
	minIdx := from
	minVal := candles[from].Low
	for i := from + 1; i < to; i++ {
		if candles[i].Low < minVal {
			minVal = candles[i].Low
			minIdx = i
		}
	}
	return SwingPoint{Index: minIdx, Value: minVal}
}

func rSquared(points []SwingPoint, slope, intercept float64) float64 {
	n := float64(len(points))
	sumY := 0.0
	for _, p := range points {
		sumY += p.Value
	}
	meanY := sumY / n
	ssTot, ssRes := 0.0, 0.0
	for _, p := range points {
		predicted := slope*float64(p.Index) + intercept
		ssTot += (p.Value - meanY) * (p.Value - meanY)
		ssRes += (p.Value - predicted) * (p.Value - predicted)
	}
	if ssTot == 0 {
		return 1.0
	}
	return 1.0 - ssRes/ssTot
}

func findHorizontalResistance(candles []Candle, highs []SwingPoint) (level float64, touches int, touchPoints []SwingPoint) {
	if len(highs) < 2 {
		return 0, 0, nil
	}

	const tolerance = 0.002
	const breakout = 0.005
	const minSpacing = 5

	type levelGroup struct {
		points []SwingPoint
		sum    float64
	}

	var groups []levelGroup

	for _, h := range highs {
		matched := false
		for i := range groups {
			avg := groups[i].sum / float64(len(groups[i].points))
			if math.Abs(h.Value-avg)/avg <= tolerance {
				groups[i].points = append(groups[i].points, h)
				groups[i].sum += h.Value
				matched = true
				break
			}
		}
		if !matched {
			groups = append(groups, levelGroup{points: []SwingPoint{h}, sum: h.Value})
		}
	}

	bestLevel := 0.0
	maxTouches := 0
	var bestTouchPoints []SwingPoint

	for _, g := range groups {
		valid := []SwingPoint{g.points[0]}
		for i := 1; i < len(g.points); i++ {
			if g.points[i].Index-valid[len(valid)-1].Index >= minSpacing {
				valid = append(valid, g.points[i])
			}
		}
		if len(valid) < 2 {
			continue
		}
		if len(valid) > maxTouches {
			maxTouches = len(valid)
			avg := g.sum / float64(len(g.points))
			bestLevel = avg
			bestTouchPoints = valid
		}
	}

	if maxTouches < 2 {
		return 0, 0, nil
	}

	for i := 0; i < len(bestTouchPoints)-1; i++ {
		start := bestTouchPoints[i].Index
		end := bestTouchPoints[i+1].Index
		for j := start; j <= end && j < len(candles); j++ {
			if candles[j].Close > bestLevel*(1+breakout) {
				return 0, 0, nil
			}
		}
	}

	lastIdx := bestTouchPoints[len(bestTouchPoints)-1].Index
	for j := lastIdx; j < len(candles); j++ {
		if candles[j].Close > bestLevel*(1+breakout) {
			return 0, 0, nil
		}
	}

	return bestLevel, maxTouches, bestTouchPoints
}

func linearRegression(points []SwingPoint) (slope, intercept float64) {
	n := float64(len(points))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for _, p := range points {
		x := float64(p.Index)
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}
