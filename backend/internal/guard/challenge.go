package guard

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"image/color"
	"math"
	mrand "math/rand"
	"sync"
	"time"

	goslide "github.com/wenlng/go-captcha/v2/slide"
)

// maxChallengeEntries bounds the in-memory challenge map (each entry is a
// few dozen bytes; unbounded growth would be an attacker's cheap DoS).
const maxChallengeEntries = 8192

// slideCaptchaPadding is the ± pixel tolerance around the true gap position
// accepted as a solve. Wide enough for touch imprecision, tight enough that
// a blind guess (~300x220 px space) almost never lands inside it.
const slideCaptchaPadding = 6

// ChallengeStore holds outstanding slider-captcha challenges: a random
// one-time key maps to the true gap position. Entries expire after ttl and
// are consumed by the first Verify, so captured responses cannot be replayed.
type ChallengeStore struct {
	mu      sync.Mutex
	now     func() time.Time
	ttl     time.Duration
	entries map[string]challengeEntry
}

type challengeEntry struct {
	x, y      int
	expiresAt time.Time
}

func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	return &ChallengeStore{
		now:     time.Now,
		ttl:     ttl,
		entries: make(map[string]challengeEntry),
	}
}

// Put stores a gap position and returns its one-time key.
func (s *ChallengeStore) Put(x, y int) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is effectively impossible; degrade to a single
		// shared key rather than panicking the request path.
		buf = []byte("fallback-challenge-key")
	}
	key := hex.EncodeToString(buf)

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) >= maxChallengeEntries {
		s.sweepLocked(now)
	}
	s.entries[key] = challengeEntry{x: x, y: y, expiresAt: now.Add(s.ttl)}
	return key
}

// Verify checks a submitted tile position against a live challenge and
// consumes it (one-time), whether or not it matches.
func (s *ChallengeStore) Verify(key string, x, y int) bool {
	now := s.now()
	s.mu.Lock()
	entry, ok := s.entries[key]
	delete(s.entries, key)
	s.mu.Unlock()
	if !ok || now.After(entry.expiresAt) {
		return false
	}
	return goslide.Validate(x, y, entry.x, entry.y, slideCaptchaPadding)
}

func (s *ChallengeStore) sweepLocked(now time.Time) {
	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
	if len(s.entries) >= maxChallengeEntries {
		s.entries = make(map[string]challengeEntry)
	}
}

// --- go-captcha resource synthesis ---
//
// go-captcha ships no embedded assets: Generate() refuses to run without
// backgrounds and graph images. Backgrounds are synthesized fresh for every
// challenge (random palette, gradient axis, noise and decoy shapes) so an
// attacker cannot pre-build templates for a fixed image pool; tiles come
// from a small static shape pool. The repo stays free of binary blobs.

var (
	slideTilesOnce sync.Once
	slideTilePool  []*goslide.GraphImage
)

const (
	// slideMasterSize matches the library's default image size.
	slideMasterWidth  = 300
	slideMasterHeight = 220
	// slideTileSize matches the library's default graph range (60-70px).
	slideTileSize = 66
)

// SlideChallenge is the wire format handed to the browser.
type SlideChallenge struct {
	Key         string `json:"captcha_key"`
	MasterImage string `json:"master_image"` // data:image/jpeg;base64,...
	TileImage   string `json:"tile_image"`   // data:image/png;base64,...
	TileX       int    `json:"tile_x"`       // initial tile display position
	TileY       int    `json:"tile_y"`
	TileSize    int    `json:"tile_size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// SlideCaptcha generates fresh slider challenges and records their answers.
type SlideCaptcha struct {
	store *ChallengeStore
}

func NewSlideCaptcha(store *ChallengeStore) *SlideCaptcha {
	return &SlideCaptcha{store: store}
}

// Verify checks and consumes a submitted solve against its challenge.
func (c *SlideCaptcha) Verify(key string, x, y int) bool {
	return c.store.Verify(key, x, y)
}

// NewChallenge renders a fresh slider challenge and registers its answer.
// The background is synthesized per challenge from a crypto-random seed —
// fixed pools would let a bot precompute template matches.
func (c *SlideCaptcha) NewChallenge() (*SlideChallenge, error) {
	slideTilesOnce.Do(initSlideTiles)

	seed, err := randomSeed()
	if err != nil {
		return nil, err
	}
	builder := goslide.NewBuilder()
	builder.SetResources(
		goslide.WithBackgrounds([]image.Image{synthRandomBackground(seed)}),
		goslide.WithGraphImages(slideTilePool),
	)
	data, err := builder.Make().Generate()
	if err != nil {
		return nil, err
	}

	block := data.GetData()
	masterB64, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	tileB64, err := data.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}

	key := c.store.Put(block.X, block.Y)
	return &SlideChallenge{
		Key:         key,
		MasterImage: masterB64,
		TileImage:   tileB64,
		TileX:       block.DX,
		TileY:       block.DY,
		TileSize:    slideTileSize,
		Width:       slideMasterWidth,
		Height:      slideMasterHeight,
	}, nil
}

func initSlideTiles() {
	slideTilePool = []*goslide.GraphImage{
		synthTileShape(11, 10),              // squarish
		synthTileShape(12, 22),              // soft corners
		synthTileShape(13, slideTileSize/2), // circle
	}
}

func randomSeed() (int64, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return 0, err
	}
	var seed int64
	for _, b := range buf {
		seed = seed<<8 | int64(b)
	}
	if seed < 0 {
		seed = -seed
	}
	return seed, nil
}

// synthRandomBackground paints a seed-determined one-off background: random
// palette, random gradient axis, translucent decoy geometry and speckle
// noise. A fresh background per challenge is the template-match defence —
// a fixed pool would let a bot precompute gap positions offline.
func synthRandomBackground(seed int64) image.Image {
	rng := mrand.New(mrand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, slideMasterWidth, slideMasterHeight))

	top := color.RGBA{
		R: uint8(24 + rng.Intn(90)),
		G: uint8(34 + rng.Intn(90)),
		B: uint8(48 + rng.Intn(110)),
		A: 255,
	}
	bottom := color.RGBA{
		R: uint8(10 + rng.Intn(40)),
		G: uint8(14 + rng.Intn(50)),
		B: uint8(24 + rng.Intn(70)),
		A: 255,
	}
	diagonal := rng.Intn(2) == 0
	for y := 0; y < slideMasterHeight; y++ {
		for x := 0; x < slideMasterWidth; x++ {
			var t float64
			if diagonal {
				t = float64(x+y) / float64(slideMasterWidth+slideMasterHeight)
			} else {
				t = float64(y) / float64(slideMasterHeight)
			}
			img.Set(x, y, lerpColor(top, bottom, t))
		}
	}

	// Decoy geometry: translucent circles and lines that break up uniform
	// gradients and waste edge-detection cycles.
	for n := 0; n < 3+rng.Intn(3); n++ {
		decoy := color.RGBA{
			R: uint8(120 + rng.Intn(120)),
			G: uint8(130 + rng.Intn(110)),
			B: uint8(150 + rng.Intn(100)),
			A: uint8(14 + rng.Intn(26)),
		}
		cx, cy := rng.Intn(slideMasterWidth), rng.Intn(slideMasterHeight)
		radius := 20 + rng.Intn(60)
		if rng.Intn(2) == 0 {
			for y := cy - radius; y <= cy+radius; y++ {
				for x := cx - radius; x <= cx+radius; x++ {
					dx, dy := x-cx, y-cy
					if dx*dx+dy*dy <= radius*radius {
						img.Set(x, y, blendPixel(img.At(x, y), decoy))
					}
				}
			}
		} else {
			x0, y0 := rng.Intn(slideMasterWidth), rng.Intn(slideMasterHeight)
			x1, y1 := rng.Intn(slideMasterWidth), rng.Intn(slideMasterHeight)
			steps := 2 * maxInt(absInt(x1-x0), absInt(y1-y0), 1)
			for i := 0; i <= steps; i++ {
				x := clampInt(x0+(x1-x0)*i/steps, 0, slideMasterWidth-1)
				y := clampInt(y0+(y1-y0)*i/steps, 0, slideMasterHeight-1)
				img.Set(x, y, blendPixel(img.At(x, y), decoy))
				img.Set(clampInt(x+1, 0, slideMasterWidth-1), y, blendPixel(img.At(x, y), decoy))
			}
		}
	}

	// Speckle noise on top.
	for n := 0; n < 700+rng.Intn(500); n++ {
		x, y := rng.Intn(slideMasterWidth), rng.Intn(slideMasterHeight)
		shade := uint8(rng.Intn(70))
		img.Set(x, y, color.RGBA{R: 190 + shade, G: 200 + shade, B: 225, A: uint8(16 + rng.Intn(46))})
	}
	return img
}

func blendPixel(base color.Color, overlay color.RGBA) color.RGBA {
	br, bg, bb, ba := base.RGBA()
	a := float64(overlay.A) / 255
	return color.RGBA{
		R: uint8(float64(overlay.R)*a + float64(br>>8)*(1-a)),
		G: uint8(float64(overlay.G)*a + float64(bg>>8)*(1-a)),
		B: uint8(float64(overlay.B)*a + float64(bb>>8)*(1-a)),
		A: uint8(ba >> 8),
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// synthTileShape draws a translucent tile with a bright rim: overlay is the
// draggable piece, shadow sits behind it, mask carves the shape. The corner
// radius varies per pool entry (squarish → soft → circle) so the draggable
// silhouette is not always the same.
func synthTileShape(seed int64, radius int) *goslide.GraphImage {
	rng := mrand.New(mrand.NewSource(seed))
	bounds := image.Rect(0, 0, slideTileSize, slideTileSize)

	overlay := image.NewRGBA(bounds)
	shadow := image.NewRGBA(bounds)
	mask := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !roundedRectContains(x, y, bounds, radius) {
				continue
			}
			mask.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			if roundedRectEdgeDistance(x, y, bounds, radius) <= 2 {
				// bright rim so the piece reads on any background
				overlay.Set(x, y, color.RGBA{R: 240, G: 244, B: 255, A: 235})
				continue
			}
			overlay.Set(x, y, color.RGBA{
				R: uint8(150 + rng.Intn(40)),
				G: uint8(170 + rng.Intn(40)),
				B: uint8(200 + rng.Intn(40)),
				A: 140,
			})
			shadow.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 90})
		}
	}
	return &goslide.GraphImage{OverlayImage: overlay, ShadowImage: shadow, MaskImage: mask}
}

// roundedRectContains reports whether the pixel lies inside the rounded
// rectangle that fills bounds with the given corner radius.
func roundedRectContains(x, y int, bounds image.Rectangle, radius int) bool {
	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X-1, bounds.Max.Y-1
	if x < minX || x > maxX || y < minY || y > maxY {
		return false
	}
	cx := clampInt(x, minX+radius, maxX-radius)
	cy := clampInt(y, minY+radius, maxY-radius)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

// roundedRectEdgeDistance is the pixel's distance to the shape border in
// pixels (0 on the rim), good enough for rim highlighting.
func roundedRectEdgeDistance(x, y int, bounds image.Rectangle, radius int) int {
	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X-1, bounds.Max.Y-1
	edge := minInt(x-minX, maxX-x, y-minY, maxY-y)
	cx := clampInt(x, minX+radius, maxX-radius)
	cy := clampInt(y, minY+radius, maxY-radius)
	if cx != x || cy != y {
		// corner arc: distance from the arc border inward
		dx, dy := x-cx, y-cy
		return radius - int(math.Sqrt(float64(dx*dx+dy*dy)))
	}
	return edge
}

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-t) + float64(y)*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
