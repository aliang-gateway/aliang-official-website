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
// and local development keep the raw passthrough behaviour. In production
// the rate limits and domain filtering are active by default from config;
// Turnstile additionally activates once a secret is configured, staging the
// rollout: deploy code first → set TURNSTILE_SECRET_KEY to enforce the
// captcha.
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

	// The per-email budget is consumed only after the captcha passed, so a
	// griefing attacker cannot lock a victim's mailbox out of registration
	// without paying for Turnstile solves.
	if g.limiter != nil {
		if !g.limiter.Allow("email:"+email, g.maxPerEmailPerDay, 24*time.Hour) {
			return http.StatusTooManyRequests, "too many registration attempts for this email, please try again later"
		}
	}

	return 0, ""
}

// registerClientIP resolves the best-effort client IP for abuse controls.
// Every proxy on this deployment's path (edge nginx, the frp tunnel, and
// ingress-nginx with compute-full-forwarded-for) APPENDS to X-Forwarded-For
// rather than overwriting it, so the right-most entry is an internal hop and
// the second-to-last entry is the ingress's view of the real client. The
// left-most entries are fully attacker-controlled (a script can prepend any
// number of fake IPs), so they must never key a rate limit.
func registerClientIP(req *http.Request) string {
	if xff := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		idx := len(parts) - 2
		if idx < 0 {
			idx = 0
		}
		if ip := strings.TrimSpace(parts[idx]); ip != "" {
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
