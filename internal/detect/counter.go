package detect

import "sync"

// RejectCounter collects reject reason hits from the detector.
type RejectCounter interface {
	Inc(reason RejectReason)
}

// NoopCounter discards all increments.
type NoopCounter struct{}

func (NoopCounter) Inc(RejectReason) {}

// MapCounter is a thread-safe reject counter backed by a map.
type MapCounter struct {
	mu sync.Mutex
	m  map[RejectReason]int
}

// NewMapCounter creates an empty MapCounter.
func NewMapCounter() *MapCounter {
	return &MapCounter{m: make(map[RejectReason]int)}
}

func (c *MapCounter) Inc(r RejectReason) {
	c.mu.Lock()
	c.m[r]++
	c.mu.Unlock()
}

// Snapshot returns a copy of current counts.
func (c *MapCounter) Snapshot() map[RejectReason]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[RejectReason]int, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	return out
}

// SliceCounter appends each rejected reason to a slice. Useful in unit tests.
type SliceCounter struct {
	Reasons []RejectReason
}

func (c *SliceCounter) Inc(r RejectReason) {
	c.Reasons = append(c.Reasons, r)
}
