package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimits configures login throttling. Per-IP buckets stop a single client from
// guessing; the global bucket caps a distributed attack.
type RateLimits struct {
	PerIPEvery  time.Duration // one token per interval, per client IP
	PerIPBurst  int
	GlobalEvery time.Duration
	GlobalBurst int
}

// DefaultRateLimits allow a few typos, then about one attempt every 10 s per IP.
func DefaultRateLimits() RateLimits {
	return RateLimits{PerIPEvery: 10 * time.Second, PerIPBurst: 10, GlobalEvery: time.Second, GlobalBurst: 30}
}

// maxTrackedKeys bounds memory if many clients hit a limited endpoint.
const maxTrackedKeys = 10_000

// Limiter is an in-memory token-bucket limiter with a bucket per key (a client IP, a
// user id) and optionally one shared by every key.
type Limiter struct {
	every time.Duration
	burst int
	now   Clock

	mu      sync.Mutex
	global  *rate.Limiter // nil for none
	buckets map[string]*keyBucket
}

type keyBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLoginLimiter returns a limiter keyed by client IP, with a global bucket, using
// limits and the given clock.
func NewLoginLimiter(limits RateLimits, now Clock) *Limiter {
	l := NewLimiter(limits.PerIPEvery, limits.PerIPBurst, now)
	l.global = rate.NewLimiter(rate.Every(limits.GlobalEvery), limits.GlobalBurst)
	return l
}

// NewLimiter returns a limiter giving each key one token every interval, up to burst.
func NewLimiter(every time.Duration, burst int, now Clock) *Limiter {
	return &Limiter{every: every, burst: burst, now: now, buckets: make(map[string]*keyBucket)}
}

// Allow consumes one attempt for key and reports whether it may proceed.
func (l *Limiter) Allow(key string) bool {
	ok, _ := l.Take(key)
	return ok
}

// Take consumes one attempt for key if one is available. When none is, it reports how
// long until one is (at least a second, for Retry-After) and consumes nothing.
func (l *Limiter) Take(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxTrackedKeys {
			l.pruneLocked(now)
		}
		b = &keyBucket{limiter: rate.NewLimiter(rate.Every(l.every), l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = now
	// Check the key's bucket first so one noisy client does not drain the global budget.
	if wait := take(b.limiter, now); wait > 0 {
		return false, wait
	}
	if l.global != nil {
		if wait := take(l.global, now); wait > 0 {
			return false, wait
		}
	}
	return true, 0
}

// take consumes a token from lim, or reports how long until there is one.
func take(lim *rate.Limiter, now time.Time) time.Duration {
	r := lim.ReserveN(now, 1)
	wait := r.DelayFrom(now)
	if wait == 0 {
		return 0
	}
	r.CancelAt(now)
	return max(wait, time.Second)
}

// pruneLocked drops buckets idle long enough to have refilled completely; forgetting
// them changes nothing. If that frees nothing, it starts over rather than grow unbounded.
func (l *Limiter) pruneLocked(now time.Time) {
	idle := l.every * time.Duration(l.burst)
	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > idle {
			delete(l.buckets, key)
		}
	}
	if len(l.buckets) >= maxTrackedKeys {
		clear(l.buckets)
	}
}
