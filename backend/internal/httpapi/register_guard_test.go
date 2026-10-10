package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"time"

	"ai-api-portal/backend/internal/guard"
	"ai-api-portal/backend/internal/proxy"
)

// newRegisterGuardTestRoutes wires a routes mux whose upstream counts how
// many registration requests actually reached it.
func newRegisterGuardTestRoutes(t *testing.T, opts RoutesOptions) (*http.ServeMux, *atomic.Int64) {
	t.Helper()

	var upstreamHits atomic.Int64 // counts only /api/v1/auth/register hits
	var userIDMu sync.Mutex
	upstreamUserIDs := map[string]int64{} // stable upstream id per email
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/auth/register" {
			// Async default-keys hook calls like groups/available land here.
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{}}`))
			return
		}
		hits := upstreamHits.Add(1)
		var body struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		userIDMu.Lock()
		id, seen := upstreamUserIDs[body.Email]
		if !seen {
			id = int64(71000 + hits)
			upstreamUserIDs[body.Email] = id
		}
		userIDMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "success",
			"data": map[string]any{
				"access_token":  fmt.Sprintf("up-at-%d", hits),
				"refresh_token": fmt.Sprintf("up-rt-%d", hits),
				"role":          "user",
				"user": map[string]any{
					"id":    id,
					"email": body.Email,
					"role":  "user",
				},
			},
		})
	}))
	t.Cleanup(upstream.Close)

	proxyClient, err := proxy.NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy client: %v", err)
	}
	opts.ProxyClient = proxyClient

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, setupTestDB(t), opts)
	return mux, &upstreamHits
}

func postRegister(t *testing.T, mux *http.ServeMux, xff, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterGuardBlocksDisposableEmailDomain(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{RegisterBlockDisposableDomains: true})

	rec := postRegister(t, mux, "", `{"email":"bot@mailinator.com","name":"bot","password":"pw123456"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for disposable domain, got %d", rec.Code)
	}
	if hits := upstreamHits.Load(); hits != 0 {
		t.Fatalf("disposable-domain registration must not reach upstream, got %d hits", hits)
	}
}

func TestRegisterGuardAllowsOrdinaryDomainWhenDisposableBlocked(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{RegisterBlockDisposableDomains: true})

	rec := postRegister(t, mux, "", `{"email":"someone@gmail.com","name":"ok","password":"pw123456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("expected 1 upstream hit, got %d", hits)
	}
}

func TestRegisterGuardRateLimitsPerIP(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{
		RegisterMaxPerIPPerHour: 2,
	})

	for i := 0; i < 2; i++ {
		if rec := postRegister(t, mux, "8.8.8.8", `{"email":"u`+strconv.Itoa(i)+`@gmail.com","name":"u","password":"pw"}`); rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	rec := postRegister(t, mux, "8.8.8.8", `{"email":"third@gmail.com","name":"u","password":"pw"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once per-IP limit is exceeded, got %d", rec.Code)
	}
	if hits := upstreamHits.Load(); hits != 2 {
		t.Fatalf("rate-limited request must not reach upstream, got %d hits", hits)
	}
	// A different IP is not affected.
	if rec := postRegister(t, mux, "9.9.9.9", `{"email":"other@gmail.com","name":"u","password":"pw"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected other IP unaffected, got %d", rec.Code)
	}
}

func TestRegisterGuardRateLimitsPerEmail(t *testing.T) {
	t.Parallel()

	mux, _ := newRegisterGuardTestRoutes(t, RoutesOptions{
		RegisterMaxPerEmailPerDay: 1,
	})

	if rec := postRegister(t, mux, "8.8.8.8", `{"email":"same@gmail.com","name":"u","password":"pw"}`); rec.Code != http.StatusOK {
		t.Fatalf("first attempt: expected 200, got %d", rec.Code)
	}
	if rec := postRegister(t, mux, "9.9.9.9", `{"email":"same@gmail.com","name":"u","password":"pw"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second attempt from another IP: expected 429, got %d", rec.Code)
	}
}

func TestRegisterGuardRequiresTurnstileWhenSecretSet(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{
		TurnstileSecretKey: "guard-secret",
		TurnstileEndpoint:  newTurnstileTestEndpoint(t, false),
	})

	rec := postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without a turnstile token, got %d", rec.Code)
	}
	if hits := upstreamHits.Load(); hits != 0 {
		t.Fatalf("unverified registration must not reach upstream, got %d hits", hits)
	}

	rec = postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw","turnstile_token":"forged"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a rejected token, got %d", rec.Code)
	}
}

func TestRegisterGuardForwardsWhenTurnstilePasses(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{
		TurnstileSecretKey: "guard-secret",
		TurnstileEndpoint:  newTurnstileTestEndpoint(t, true),
	})

	rec := postRegister(t, mux, "8.8.4.4", `{"email":"human@gmail.com","name":"u","password":"pw","turnstile_token":"real-token"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("expected registration forwarded upstream, got %d hits", hits)
	}
}

func TestRegisterGuardDisabledByDefault(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{})

	// Disposable domain, no token, more requests than any sane limit — with a
	// zero-value guard none of the checks may fire (backward compatibility).
	for i := 0; i < 15; i++ {
		rec := postRegister(t, mux, "", `{"email":"bot@mailinator.com","name":"b","password":"pw"}`)
		if rec.Code == http.StatusTooManyRequests || rec.Code == http.StatusForbidden || rec.Code == http.StatusBadRequest {
			t.Fatalf("request %d: guard unexpectedly fired with status %d", i+1, rec.Code)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected upstream 200, got %d", i+1, rec.Code)
		}
	}
	if hits := upstreamHits.Load(); hits != 15 {
		t.Fatalf("expected 15 upstream hits, got %d", hits)
	}
}

func TestRegisterGuardAllowedEmailDomains(t *testing.T) {
	t.Parallel()

	mux, _ := newRegisterGuardTestRoutes(t, RoutesOptions{
		RegisterAllowedEmailDomains: []string{"aliang.one"},
	})

	if rec := postRegister(t, mux, "", `{"email":"a@aliang.one","name":"u","password":"pw"}`); rec.Code != http.StatusOK {
		t.Fatalf("allowed domain: expected 200, got %d", rec.Code)
	}
	if rec := postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("domain outside allowlist: expected 400, got %d", rec.Code)
	}
}

func TestRegisterGuardSkipsMalformedBody(t *testing.T) {
	t.Parallel()

	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{
		TurnstileSecretKey:             "guard-secret",
		TurnstileEndpoint:              newTurnstileTestEndpoint(t, false),
		RegisterBlockDisposableDomains: true,
	})

	// Malformed JSON is upstream's business — the guard must not 4xx it
	// itself (upstream returns its own 400).
	rec := postRegister(t, mux, "", `not-json`)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusBadRequest {
		t.Fatalf("guard must not reject a malformed body itself, got %d", rec.Code)
	}
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("malformed body should be forwarded for upstream validation, got %d hits", hits)
	}
}

// newTurnstileTestEndpoint stands in for Cloudflare's siteverify endpoint.
func newTurnstileTestEndpoint(t *testing.T, success bool) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": success})
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestRegisterClientIPPrefersPenultimateXFFHop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		xff  string
		peer string
		want string
	}{
		{
			// 本部署链路（边缘 nginx→frp→ingress）对 XFF 一律追加：最右是内网跳，
			// 倒数第二跳恒为真实客户端。攻击者再怎么在前面塞伪造值，倒数第二跳不变。
			name: "multi hop keeps attacker real ip as penultimate",
			xff:  "6.6.6.6, 7.7.7.7, 8.8.8.8, 172.16.111.101",
			want: "8.8.8.8",
		},
		{
			name: "legit browser request real client penultimate",
			xff:  "1.2.3.4, 172.16.111.101",
			want: "1.2.3.4",
		},
		{
			name: "single entry used as is",
			xff:  "1.2.3.4",
			want: "1.2.3.4",
		},
		{
			name: "no xff falls back to peer addr",
			xff:  "",
			peer: "9.9.9.9:51234",
			want: "9.9.9.9",
		},
		{
			name: "entries are trimmed",
			xff:  " 5.5.5.5 , 172.16.111.101 ",
			want: "5.5.5.5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
			if tc.peer != "" {
				req.RemoteAddr = tc.peer
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := registerClientIP(req); got != tc.want {
				t.Fatalf("registerClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegisterGuardRequiresSliderCaptcha(t *testing.T) {
	t.Parallel()

	store := guard.NewChallengeStore(time.Minute)
	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{
		RegisterSliderCaptcha:  true,
		RegisterChallengeStore: store,
	})

	// No captcha fields at all → rejected before reaching upstream.
	rec := postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without slider captcha, got %d", rec.Code)
	}
	if hits := upstreamHits.Load(); hits != 0 {
		t.Fatalf("captcha-less registration must not reach upstream, got %d hits", hits)
	}

	// Wrong drop position → rejected, and the consumed key cannot be reused.
	key := store.Put(120, 90)
	rec = postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw","captcha_key":"`+key+`","captcha_x":1,"captcha_y":1}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a wrong slider position, got %d", rec.Code)
	}
	rec = postRegister(t, mux, "", `{"email":"a@gmail.com","name":"u","password":"pw","captcha_key":"`+key+`","captcha_x":120,"captcha_y":90}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a replayed key, got %d", rec.Code)
	}

	// Fresh key with the exact position registered in the store → forwarded.
	key = store.Put(120, 90)
	rec = postRegister(t, mux, "", `{"email":"human@gmail.com","name":"u","password":"pw","captcha_key":"`+key+`","captcha_x":118,"captcha_y":92}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a correct slider solve, got %d: %s", rec.Code, rec.Body.String())
	}
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("expected the solved registration forwarded upstream, got %d hits", hits)
	}
}

func TestRegisterCaptchaEndpointIssuesChallenge(t *testing.T) {
	t.Parallel()

	mux, _ := newRegisterGuardTestRoutes(t, RoutesOptions{
		RegisterSliderCaptcha: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/register-captcha", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from register-captcha, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		CaptchaKey  string `json:"captcha_key"`
		MasterImage string `json:"master_image"`
		TileImage   string `json:"tile_image"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		TileSize    int    `json:"tile_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode captcha payload: %v", err)
	}
	if payload.CaptchaKey == "" {
		t.Fatal("expected non-empty captcha_key")
	}
	if !strings.HasPrefix(payload.MasterImage, "data:image/jpeg;base64,") {
		t.Fatalf("master image should be a jpeg data URI, got %q", payload.MasterImage[:min(40, len(payload.MasterImage))])
	}
	if !strings.HasPrefix(payload.TileImage, "data:image/png;base64,") {
		t.Fatalf("tile image should be a png data URI, got %q", payload.TileImage[:min(40, len(payload.TileImage))])
	}
	if payload.Width == 0 || payload.Height == 0 || payload.TileSize == 0 {
		t.Fatalf("expected geometry hints, got %+v", payload)
	}
}

func TestRegisterGuardSkipsSliderWhenDisabled(t *testing.T) {
	t.Parallel()

	// Zero-value options: no slider, register flows through untouched.
	mux, upstreamHits := newRegisterGuardTestRoutes(t, RoutesOptions{})
	for i := 0; i < 3; i++ {
		rec := postRegister(t, mux, "", `{"email":"no-slider@gmail.com","name":"u","password":"pw"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 without slider captcha, got %d", i+1, rec.Code)
		}
	}
	if hits := upstreamHits.Load(); hits != 3 {
		t.Fatalf("expected 3 upstream hits, got %d", hits)
	}
}
