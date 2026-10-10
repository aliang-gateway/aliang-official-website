package guard

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiterAllowsUpToLimitThenBlocks(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	l := NewRateLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4", 3, time.Hour) {
			t.Fatalf("hit %d: expected allow within limit", i+1)
		}
	}
	if l.Allow("1.2.3.4", 3, time.Hour) {
		t.Fatal("expected block once limit is reached")
	}
}

func TestRateLimiterReallowsAfterWindowSlides(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	l := NewRateLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if !l.Allow("1.2.3.4", 2, time.Hour) {
			t.Fatalf("hit %d: expected allow", i+1)
		}
	}
	if l.Allow("1.2.3.4", 2, time.Hour) {
		t.Fatal("expected block at limit")
	}

	now = now.Add(time.Hour + time.Second)
	if !l.Allow("1.2.3.4", 2, time.Hour) {
		t.Fatal("expected allow after the window has fully slid past")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	l := NewRateLimiter()
	l.now = func() time.Time { return now }

	if !l.Allow("1.1.1.1", 1, time.Hour) {
		t.Fatal("first key: expected allow")
	}
	if l.Allow("1.1.1.1", 1, time.Hour) {
		t.Fatal("first key: expected block at limit")
	}
	if !l.Allow("2.2.2.2", 1, time.Hour) {
		t.Fatal("second key must not be affected by the first key")
	}
}

func TestRateLimiterExpiredEntriesArePruned(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	l := NewRateLimiter()
	l.now = func() time.Time { return now }

	// Fill far beyond the sweep threshold with unique keys.
	for i := 0; i < maxTrackedKeys+10; i++ {
		l.Allow(keyFor(i), 1, time.Minute)
	}
	// Advance past the window and trigger another Allow so the sweep runs.
	now = now.Add(2 * time.Minute)
	for i := 0; i < maxTrackedKeys+10; i++ {
		l.Allow(keyFor(i), 1, time.Minute)
	}

	l.mu.Lock()
	size := len(l.hits)
	l.mu.Unlock()
	if size > maxTrackedKeys {
		t.Fatalf("expected in-memory map to stay bounded under %d keys, got %d", maxTrackedKeys, size)
	}
}

func keyFor(i int) string {
	return fmt.Sprintf("10.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff)
}
