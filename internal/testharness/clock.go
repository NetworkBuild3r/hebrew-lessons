package testharness

import (
	"sync"
	"time"
)

// Epoch is the instant a new Env's clock shows, before Advance.
func Epoch() time.Time {
	return time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
}

// FixedClock is a clock.Clock that tests move with Advance.
// Now and Advance may be called from several goroutines.
type FixedClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFixedClock returns a clock frozen at t, in UTC.
func NewFixedClock(t time.Time) *FixedClock {
	return &FixedClock{now: t.UTC()}
}

// Now is the time the clock currently shows.
func (c *FixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}
