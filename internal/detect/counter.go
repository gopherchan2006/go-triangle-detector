package detect

import "sync"

type RejectCounter interface {
	Inc(reason RejectReason)
}

type NoopCounter struct{}

func (NoopCounter) Inc(RejectReason) {}

type MapCounter struct {
	mu sync.Mutex
	m  map[RejectReason]int
}

func NewMapCounter() *MapCounter {
	return &MapCounter{m: make(map[RejectReason]int)}
}

func (c *MapCounter) Inc(r RejectReason) {
	c.mu.Lock()
	c.m[r]++
	c.mu.Unlock()
}

func (c *MapCounter) Snapshot() map[RejectReason]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[RejectReason]int, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	return out
}

type SliceCounter struct {
	Reasons []RejectReason
}

func (c *SliceCounter) Inc(r RejectReason) {
	c.Reasons = append(c.Reasons, r)
}
