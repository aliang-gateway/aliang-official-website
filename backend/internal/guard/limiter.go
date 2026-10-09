// Package guard provides the anti-abuse primitives guarding the public
// registration path: a sliding-window rate limiter, a disposable-email-domain
// checker, and a Cloudflare Turnstile siteverify client.
package guard

import (
	"sync"
	"time"
)

// maxTrackedKeys bounds the limiter's in-memory map. Attackers can mint
// unlimited unique keys (spoofed XFF entries), so once the map exceeds this
// size expired entries are swept and — if still over — the whole map is
// dropped, trading one burst window for bounded memory.
const maxTrackedKeys = 65536

// RateLimiter is an in-memory sliding-window limiter keyed by arbitrary
// strings (client IP, email, ...). Safe for concurrent use. One instance is
// scoped to one guard, so keys from different deployments never collide.
type RateLimiter struct {
	mu   sync.Mutex
	now  func() time.Time
	hits map[string][]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{now: time.Now, hits: make(map[string][]time.Time)}
}

// Allow records a hit for key and reports whether it is within limit over
// window. limit <= 0 always allows.
func (l *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	if limit <= 0 {
		return true
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.hits) > maxTrackedKeys {
		l.sweepLocked(now)
	}

	stamps := l.hits[key]
	fresh := stamps[:0]
	for _, ts := range stamps {
		if now.Sub(ts) < window {
			fresh = append(fresh, ts)
		}
	}
	if len(fresh) >= limit {
		l.hits[key] = fresh
		return false
	}
	l.hits[key] = append(fresh, now)
	return true
}

func (l *RateLimiter) sweepLocked(now time.Time) {
	for key, stamps := range l.hits {
		if len(stamps) == 0 {
			delete(l.hits, key)
			continue
		}
		last := stamps[len(stamps)-1]
		if now.Sub(last) >= time.Hour {
			delete(l.hits, key)
		}
	}
	if len(l.hits) > maxTrackedKeys {
		l.hits = make(map[string][]time.Time)
	}
}
