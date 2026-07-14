package tools

// ratelimit.go — per-user fixed-window limiter, ported from lib/rate-limit.ts.
// On the single long-lived VPS process this is finally a real global ceiling
// (the serverless TS server counted per-lambda). footics-api has its own limiter
// + Cloudflare WAF in front, so this is a courtesy cap, not the security control.

import (
	"sync"
	"time"
)

const (
	rlWindow     = time.Minute
	rlMaxEntries = 10_000
)

type rlEntry struct {
	startedAt time.Time
	count     int
}

type rateLimiter struct {
	perMin int
	mu     sync.Mutex
	// ponytail: one global map + mutex. Fine at Footics' scale (a handful of
	// calls/sec); shard by user hash only if this mutex ever shows up in a profile.
	windows map[string]*rlEntry
}

func newRateLimiter(perMin int) *rateLimiter {
	return &rateLimiter{perMin: perMin, windows: make(map[string]*rlEntry)}
}

// retryAfter returns 0 when the call is allowed, or the seconds to wait when the
// user's window is full (mirrors rateLimit() in the TS server).
func (rl *rateLimiter) retryAfter(userID string, now time.Time) int {
	if rl.perMin <= 0 {
		return 0
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	w := rl.windows[userID]
	if w == nil || now.Sub(w.startedAt) >= rlWindow {
		if len(rl.windows) >= rlMaxEntries {
			rl.prune(now)
		}
		rl.windows[userID] = &rlEntry{startedAt: now, count: 1}
		return 0
	}
	if w.count < rl.perMin {
		w.count++
		return 0
	}
	remaining := w.startedAt.Add(rlWindow).Sub(now).Seconds()
	if remaining < 1 {
		return 1
	}
	return int(remaining) + 1 // ceil, matching Math.ceil in the TS server
}

func (rl *rateLimiter) prune(now time.Time) {
	for k, w := range rl.windows {
		if now.Sub(w.startedAt) >= rlWindow {
			delete(rl.windows, k)
		}
	}
	if len(rl.windows) >= rlMaxEntries {
		rl.windows = make(map[string]*rlEntry)
	}
}
