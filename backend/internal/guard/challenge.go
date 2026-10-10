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
// backgrounds and graph images. We synthesize both at first use — gradient
// backgrounds with speckle noise, translucent rounded tiles — so the repo
// stays free of binary blobs while the captcha stays fully self-hosted.

var (
	slideResourcesOnce sync.Once
	slideResources     []goslide.Resource
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
func (c *SlideCaptcha) NewChallenge() (*SlideChallenge, error) {
	slideResourcesOnce.Do(initSlideResources)

	builder := goslide.NewBuilder()
	builder.SetResources(slideResources...)
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

func initSlideResources() {
	backgrounds := make([]image.Image, 0, 3)
	for seed := int64(1); seed <= 3; seed++ {
		backgrounds = append(backgrounds, synthBackground(seed))
	}
	tiles := []*goslide.GraphImage{
		synthTile(11),
		synthTile(12),
		synthTile(13),
	}
	slideResources = []goslide.Resource{
		goslide.WithBackgrounds(backgrounds),
		goslide.WithGraphImages(tiles),
	}
}

// synthBackground paints a deterministic vertical gradient with sparse noise
// speckles — enough texture for the slider visual, zero binary assets.
func synthBackground(seed int64) image.Image {
	rng := mrand.New(mrand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, slideMasterWidth, slideMasterHeight))
	top := color.RGBA{R: uint8(38 + seed*17), G: 52 + uint8(seed*9), B: 74, A: 255}
	bottom := color.RGBA{R: 16, G: 24, B: 40, A: 255}
	for y := 0; y < slideMasterHeight; y++ {
		row := lerpColor(top, bottom, float64(y)/float64(slideMasterHeight))
		for x := 0; x < slideMasterWidth; x++ {
			img.Set(x, y, row)
		}
	}
	for n := 0; n < 900; n++ {
		x, y := rng.Intn(slideMasterWidth), rng.Intn(slideMasterHeight)
		shade := uint8(rng.Intn(60))
		img.Set(x, y, color.RGBA{R: 200 + shade, G: 210 + shade, B: 230, A: uint8(20 + rng.Intn(50))})
	}
	return img
}

// synthTile draws a translucent rounded square with a bright rim: overlay is
// the draggable piece, shadow sits behind it, mask carves the rounded shape.
func synthTile(seed int64) *goslide.GraphImage {
	rng := mrand.New(mrand.NewSource(seed))
	bounds := image.Rect(0, 0, slideTileSize, slideTileSize)
	radius := 12

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
