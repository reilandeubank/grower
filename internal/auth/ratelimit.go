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

// maxTrackedIPs bounds memory if many addresses hit the login endpoint.
const maxTrackedIPs = 10_000

// LoginLimiter is an in-memory token-bucket limiter keyed by client IP.
type LoginLimiter struct {
	limits RateLimits
	now    Clock

	mu     sync.Mutex
	global *rate.Limiter
	perIP  map[string]*ipBucket
}

type ipBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLoginLimiter returns a limiter using limits and the given clock.
func NewLoginLimiter(limits RateLimits, now Clock) *LoginLimiter {
	return &LoginLimiter{
		limits: limits,
		now:    now,
		global: rate.NewLimiter(rate.Every(limits.GlobalEvery), limits.GlobalBurst),
		perIP:  make(map[string]*ipBucket),
	}
}

// Allow consumes one attempt for ip and reports whether it may proceed.
func (l *LoginLimiter) Allow(ip string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.perIP[ip]
	if !ok {
		if len(l.perIP) >= maxTrackedIPs {
			l.pruneLocked(now)
		}
		b = &ipBucket{limiter: rate.NewLimiter(rate.Every(l.limits.PerIPEvery), l.limits.PerIPBurst)}
		l.perIP[ip] = b
	}
	b.lastSeen = now
	// Check the per-IP bucket first so one noisy client does not drain the global budget.
	if !b.limiter.AllowN(now, 1) {
		return false
	}
	return l.global.AllowN(now, 1)
}

// pruneLocked drops buckets idle long enough to have refilled completely; forgetting
// them changes nothing. If that frees nothing, it starts over rather than grow unbounded.
func (l *LoginLimiter) pruneLocked(now time.Time) {
	idle := l.limits.PerIPEvery * time.Duration(l.limits.PerIPBurst)
	for ip, b := range l.perIP {
		if now.Sub(b.lastSeen) > idle {
			delete(l.perIP, ip)
		}
	}
	if len(l.perIP) >= maxTrackedIPs {
		clear(l.perIP)
	}
}
