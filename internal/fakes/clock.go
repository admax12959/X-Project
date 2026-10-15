package fakes

import (
	"sync"
	"time"
)

// Clock is the application's clock in tests. By default it is the real clock, so everything
// that depends on real time (the mail worker's retry waits, for example) behaves as in
// production. A test that needs to control time calls Freeze, then moves it with Advance or
// Set; Unfreeze goes back to the real clock, continuing from where the fake one stopped.
type Clock struct {
	mu     sync.Mutex
	frozen bool
	at     time.Time     // the time while frozen
	offset time.Duration // fake time minus real time while running
}

func NewClock() *Clock { return &Clock{} }

// Now is the function the application is given as its clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frozen {
		return c.at
	}
	return time.Now().Add(c.offset)
}

// Freeze stops the clock at t.
func (c *Clock) Freeze(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frozen, c.at = true, t
}

// Advance moves the clock forward (a frozen clock stays frozen at the new time; a running
// one jumps ahead and keeps running).
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frozen {
		c.at = c.at.Add(d)
		return
	}
	c.offset += d
}

// Unfreeze lets the clock run again, from the time it stopped at.
func (c *Clock) Unfreeze() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frozen {
		c.frozen, c.offset = false, time.Until(c.at)
	}
}
