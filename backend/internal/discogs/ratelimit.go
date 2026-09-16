package discogs

import (
	"context"
	"sync"
	"time"
)

// RateLimiter is a simple token-bucket limiter that serialises callers to at
// most ratePerSec requests per second, shared across every game and lookup.
type RateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

// NewRateLimiter creates a limiter allowing ratePerSec requests per second.
func NewRateLimiter(ratePerSec float64) *RateLimiter {
	if ratePerSec <= 0 {
		ratePerSec = 1
	}
	return &RateLimiter{interval: time.Duration(float64(time.Second) / ratePerSec)}
}

// Wait blocks until it is this caller's turn, or the context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	now := time.Now()
	next := r.last.Add(r.interval)
	var wait time.Duration
	if next.After(now) {
		wait = next.Sub(now)
	}
	r.last = now.Add(wait)
	r.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
