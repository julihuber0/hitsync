package ws

import (
	"sync"
	"time"
)

// connRateLimiter enforces the 30 messages/second cap per connection (§11.4)
// using a simple fixed-window counter.
type connRateLimiter struct {
	mu            sync.Mutex
	max           int
	windowStart   time.Time
	countInWindow int
}

func newRateLimiter(maxPerSec int) *connRateLimiter {
	return &connRateLimiter{max: maxPerSec, windowStart: time.Now()}
}

func (r *connRateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.windowStart) >= time.Second {
		r.windowStart = now
		r.countInWindow = 0
	}
	r.countInWindow++
	return r.countInWindow <= r.max
}
