package detect_test

import (
	"testing"

	"github.com/gopherchan2006/go-triangle-detector/internal/detect"
	"github.com/gopherchan2006/go-triangle-detector/internal/domain"
)

func makeCandle(close float64) domain.Candle {
	return domain.Candle{
		Open:   close,
		High:   close * 1.01,
		Low:    close * 0.99,
		Close:  close,
		Volume: 1000,
	}
}

func TestDetectAscendingTriangle_TooFewCandles(t *testing.T) {
	candles := make([]domain.Candle, 5)
	result := detect.DetectAscendingTriangle(candles)
	if result.Found {
		t.Error("expected not found with too few candles")
	}
}

func TestDetectAscendingTriangle_EmptyCandles(t *testing.T) {
	result := detect.DetectAscendingTriangle(nil)
	if result.Found {
		t.Error("expected not found with nil candles")
	}
}

func TestDetectAscendingTriangle_RejectReasonSet(t *testing.T) {

	candles := make([]domain.Candle, 50)
	for i := range candles {
		candles[i] = makeCandle(100.0)
	}
	result := detect.DetectAscendingTriangle(candles)
	if result.Found {
		t.Error("expected not found for flat candles")
	}
	if result.RejectReason == "" {
		t.Error("expected a reject reason to be set")
	}
}
