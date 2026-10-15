package fakes

import (
	"sync"
	"time"
)

// Clock is the fake of the application's clock port (domain.Options.Now): the real time plus
// an offset that only moves when a test advances it. Token expiry, rate-limit windows and
// session expiry all follow it, so a test can wait out an hour in no time. It keeps running
// (rather than standing still) because retry backoff and "newest first" orderings rely on time
// passing between events.
type Clock struct {
	mu     sync.Mutex
	offset time.Duration
}

func NewClock() *Clock { return &Clock{} }

// Now is the clock the application is given (app.Deps.Now).
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Add(c.offset)
}

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset += d
}
