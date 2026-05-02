package binance

import (
	"fmt"
	"strconv"
	"time"
)

func parseInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

func parseFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case float64:
		return v, true
	default:
		return 0, false
	}
}

// IntervalToMilliseconds converts a Binance interval string (e.g. "15m") to milliseconds.
func IntervalToMilliseconds(interval string) (int64, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("unsupported interval %q", interval)
	}
	unit := interval[len(interval)-1]
	value, err := strconv.Atoi(interval[:len(interval)-1])
	if err != nil {
		return 0, err
	}
	units := map[byte]time.Duration{
		'm': time.Minute,
		'h': time.Hour,
		'd': 24 * time.Hour,
		'w': 7 * 24 * time.Hour,
		'M': 30 * 24 * time.Hour,
	}
	duration, ok := units[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported interval %q", interval)
	}
	return int64(time.Duration(value) * duration / time.Millisecond), nil
}

func parseTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time format %q, expected RFC3339 or YYYY-MM-DD", value)
}
