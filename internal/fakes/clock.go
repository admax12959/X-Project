package fakes

import (
	"sync"
	"time"
)

// Clock is the fake of the application's clock port (domain.Options.Now): it starts at the real
// time of its creation, stands still, and only moves when a test advances it. Token expiry,
// rate-limit windows and session expiry all follow it, so a test can wait out an hour in no time.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func NewClock() *Clock { return &Clock{now: time.Now()} }

// Now is the clock the application is given (app.Deps.Now).
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
