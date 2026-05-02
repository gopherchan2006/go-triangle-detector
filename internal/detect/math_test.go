package detect

import (
	"math"
	"testing"

	"github.com/gopherchan2006/go-triangle-detector/internal/domain"
)

func TestLinearRegression_PositiveSlope(t *testing.T) {

	points := []SwingPoint{
		{Index: 0, Value: 1},
		{Index: 1, Value: 3},
		{Index: 2, Value: 5},
		{Index: 3, Value: 7},
	}
	slope, intercept := linearRegression(points)
	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("expected slope ~2.0, got %f", slope)
	}
	if math.Abs(intercept-1.0) > 0.01 {
		t.Errorf("expected intercept ~1.0, got %f", intercept)
	}
}

func TestLinearRegression_ZeroSlope(t *testing.T) {

	points := []SwingPoint{
		{Index: 0, Value: 5},
		{Index: 1, Value: 5},
		{Index: 2, Value: 5},
	}
	slope, intercept := linearRegression(points)
	if math.Abs(slope) > 0.001 {
		t.Errorf("expected slope ~0.0, got %f", slope)
	}
	if math.Abs(intercept-5.0) > 0.01 {
		t.Errorf("expected intercept ~5.0, got %f", intercept)
	}
}

func TestRSquared_PerfectFit(t *testing.T) {
	points := []SwingPoint{
		{Index: 0, Value: 1},
		{Index: 1, Value: 2},
		{Index: 2, Value: 3},
	}
	r2 := rSquared(points, 1.0, 1.0)
	if r2 < 0.999 {
		t.Errorf("expected r2 ~1.0 for perfect linear fit, got %f", r2)
	}
}

func TestRSquared_BadPrediction(t *testing.T) {

	points := []SwingPoint{
		{Index: 0, Value: 1},
		{Index: 1, Value: 2},
		{Index: 2, Value: 3},
	}
	r2 := rSquared(points, 0.0, 100.0)
	if r2 >= 0 {
		t.Errorf("expected negative r2 for bad prediction, got %f", r2)
	}
}

func TestCalcATR_SingleBar(t *testing.T) {
	candles := []domain.Candle{
		{Open: 100, High: 110, Low: 90, Close: 105, Volume: 1000},
	}
	atr := calcATR(candles)
	expected := 110.0 - 90.0
	if math.Abs(atr-expected) > 0.001 {
		t.Errorf("expected ATR=%.2f for single bar, got %.2f", expected, atr)
	}
}

func TestCalcATR_TwoBars(t *testing.T) {

	candles := []domain.Candle{
		{Open: 95, High: 110, Low: 90, Close: 100, Volume: 1000},
		{Open: 100, High: 108, Low: 92, Close: 104, Volume: 1000},
	}
	atr := calcATR(candles)
	expected := 36.0 / 2.0
	if math.Abs(atr-expected) > 0.001 {
		t.Errorf("expected ATR=%.2f, got %.2f", expected, atr)
	}
}

func TestCalcATR_Empty(t *testing.T) {
	atr := calcATR(nil)
	if atr != 0 {
		t.Errorf("expected ATR=0 for empty candles, got %f", atr)
	}
}
