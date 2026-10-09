package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"ai-api-portal/backend/internal/guard"
)

// registerGuard enforces the anti-abuse checks on the public registration
// path: per-IP / per-email rate limits, disposable-email-domain blocking, an
// optional email-domain allowlist, and server-side Turnstile verification.
//
// A zero-value RoutesOptions builds a nil guard (every check off) so tests
// and local development keep the raw passthrough behaviour; production wires
// resolved values from config. Turnstile activates only when a secret is
// configured, so rollout can stage: deploy code → set TURNSTILE_SECRET_KEY.
type registerGuard struct {
	turnstile         *guard.TurnstileVerifier // nil → turnstile off
	limiter           *guard.RateLimiter
	maxPerIPPerHour   int
	maxPerEmailPerDay int
	disposable        *guard.DisposableChecker // nil → disposable blocking off
	allowedDomains    []string                 // empty → no allowlist
}

func newRegisterGuardFromOptions(opts RoutesOptions) *registerGuard {
	g := &registerGuard{
		limiter:           guard.NewRateLimiter(),
		maxPerIPPerHour:   opts.RegisterMaxPerIPPerHour,
		maxPerEmailPerDay: opts.RegisterMaxPerEmailPerDay,
		allowedDomains:    lowercaseDomains(opts.RegisterAllowedEmailDomains),
	}
	if strings.TrimSpace(opts.TurnstileSecretKey) != "" {
		g.turnstile = guard.NewTurnstileVerifier(opts.TurnstileSecretKey, opts.TurnstileEndpoint)
	}
	if opts.RegisterBlockDisposableDomains {
		g.disposable = guard.NewDefaultDisposableChecker()
	}

	if g.turnstile == nil && g.maxPerIPPerHour <= 0 && g.maxPerEmailPerDay <= 0 &&
		g.disposable == nil && len(g.allowedDomains) == 0 {
		return nil
	}
	return g
}

// check runs the guard in escalating order and returns the HTTP status to
// reject with (0 means allow). Malformed bodies are allowed through —
// validating them is the upstream's job, and it already does.
func (g *registerGuard) check(req *http.Request, body []byte) (int, string) {
	var payload struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return 0, ""
		}
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	clientIP := registerClientIP(req)

	if g.limiter != nil {
		if !g.limiter.Allow("ip:"+clientIP, g.maxPerIPPerHour, time.Hour) {
			return http.StatusTooManyRequests, "too many registration attempts from this address, please try again later"
		}
		if !g.limiter.Allow("email:"+email, g.maxPerEmailPerDay, 24*time.Hour) {
			return http.StatusTooManyRequests, "too many registration attempts for this email, please try again later"
		}
	}

	if email != "" {
		domain := emailDomain(email)
		if g.disposable != nil && g.disposable.IsBlocked(domain) {
			return http.StatusBadRequest, "email domain not allowed"
		}
		if len(g.allowedDomains) > 0 && !containsDomain(g.allowedDomains, domain) {
			return http.StatusBadRequest, "email domain not allowed"
		}
	}

	if g.turnstile != nil {
		if err := g.turnstile.Verify(req.Context(), payload.TurnstileToken, clientIP); err != nil {
			return http.StatusForbidden, "captcha verification failed, please retry"
		}
	}

	return 0, ""
}

// registerClientIP resolves the best-effort client IP for abuse controls:
// the left-most X-Forwarded-For entry when present (set by the ingress and
// forwarded by the Next.js proxy), otherwise the socket peer.
func registerClientIP(req *http.Request) string {
	if xff := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			xff = xff[:idx]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(req.RemoteAddr)
	}
	return host
}

func emailDomain(email string) string {
	if idx := strings.LastIndex(email, "@"); idx >= 0 {
		email = email[idx+1:]
	}
	return strings.TrimSuffix(email, ".")
}

func lowercaseDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" {
			out = append(out, domain)
		}
	}
	return out
}

func containsDomain(allowed []string, domain string) bool {
	for _, candidate := range allowed {
		if candidate == domain {
			return true
		}
	}
	return false
}
