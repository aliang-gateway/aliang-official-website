package guard

import (
	"image"
	"testing"
	"time"
)

func TestChallengeStorePutThenVerifyOnce(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	s := NewChallengeStore(time.Minute)
	s.now = func() time.Time { return now }

	key := s.Put(100, 80)
	if key == "" {
		t.Fatal("expected non-empty key")
	}

	if !s.Verify(key, 102, 79) {
		t.Fatal("expected the exact gap position to verify")
	}

	// One-time: the same key must not verify twice (replay protection).
	if s.Verify(key, 102, 79) {
		t.Fatal("challenge key must be consumed by the first verification")
	}
}

func TestChallengeStoreRejectsWrongOffset(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	s := NewChallengeStore(time.Minute)
	s.now = func() time.Time { return now }

	// Verify consumes the key one-time, so every attempt gets a fresh Put.
	if s.Verify(s.Put(100, 80), 150, 80) {
		t.Fatal("expected a far-off x to be rejected")
	}
	if s.Verify(s.Put(100, 80), 100, 120) {
		t.Fatal("expected a far-off y to be rejected")
	}
	// Within tolerance still passes (padding is 2*padding box centered on gap).
	if !s.Verify(s.Put(100, 80), 96, 83) {
		t.Fatal("expected a slightly-off position within tolerance to verify")
	}
}

func TestChallengeStoreExpiry(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	s := NewChallengeStore(time.Minute)
	s.now = func() time.Time { return now }

	key := s.Put(50, 50)
	now = now.Add(2 * time.Minute)

	if s.Verify(key, 50, 50) {
		t.Fatal("expected an expired challenge to be rejected")
	}
}

func TestChallengeStoreUnknownKeyRejected(t *testing.T) {
	t.Parallel()

	s := NewChallengeStore(time.Minute)
	if s.Verify("nope", 1, 1) {
		t.Fatal("expected unknown key to be rejected")
	}
}

func TestChallengeStoreBoundedMemory(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	s := NewChallengeStore(time.Minute)
	s.now = func() time.Time { return now }

	for i := 0; i < maxChallengeEntries+10; i++ {
		s.Put(i, i)
	}

	s.mu.Lock()
	size := len(s.entries)
	s.mu.Unlock()
	if size > maxChallengeEntries {
		t.Fatalf("expected store bounded under %d entries, got %d", maxChallengeEntries, size)
	}
}

// 模板匹配加固：每次出题的背景必须不同（固定背景池可被 bot 预先建模）。
func TestSlideChallengeBackgroundsAreRandomized(t *testing.T) {
	t.Parallel()

	s := NewChallengeStore(time.Minute)
	c := NewSlideCaptcha(s)

	first, err := c.NewChallenge()
	if err != nil {
		t.Fatalf("first challenge: %v", err)
	}
	second, err := c.NewChallenge()
	if err != nil {
		t.Fatalf("second challenge: %v", err)
	}
	if first.MasterImage == second.MasterImage {
		t.Fatal("consecutive challenges must not share a background (template-match hardening)")
	}
}

func TestSynthRandomBackgroundDeterministicBySeed(t *testing.T) {
	t.Parallel()

	a1 := synthRandomBackground(1)
	a2 := synthRandomBackground(1)
	b := synthRandomBackground(2)

	if !imagesEqual(a1, a2) {
		t.Fatal("same seed must produce an identical background (test determinism)")
	}
	if imagesEqual(a1, b) {
		t.Fatal("different seeds must produce different backgrounds")
	}
	if bounds := a1.Bounds(); bounds.Dx() != slideMasterWidth || bounds.Dy() != slideMasterHeight {
		t.Fatalf("background size = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), slideMasterWidth, slideMasterHeight)
	}
}

func imagesEqual(a, b image.Image) bool {
	ba, bb := a.Bounds(), b.Bounds()
	if ba != bb {
		return false
	}
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}
