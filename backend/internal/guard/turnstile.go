package guard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TurnstileEndpoint is Cloudflare's official siteverify endpoint.
const TurnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrChallengeFailed means the presented token did not pass verification
// (missing, already used, wrong site, or rejected by Cloudflare). The
// registration gate treats every non-nil error as fail-closed.
var ErrChallengeFailed = errors.New("turnstile challenge failed")

// TurnstileVerifier validates Turnstile tokens server-side against the
// siteverify endpoint. Endpoint is injectable so tests can point it at an
// httptest server.
type TurnstileVerifier struct {
	secret   string
	endpoint string
	client   *http.Client
}

func NewTurnstileVerifier(secret, endpoint string) *TurnstileVerifier {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = TurnstileEndpoint
	}
	return &TurnstileVerifier{
		secret:   strings.TrimSpace(secret),
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// siteverifyResponse mirrors the documented siteverify JSON envelope.
type siteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify checks token against siteverify, optionally attributing it to
// remoteIP (the client IP that solved the challenge). Transport errors and
// non-2xx responses fail closed.
func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if v.secret == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: missing secret or token", ErrChallengeFailed)
	}

	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", strings.TrimSpace(token))
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrChallengeFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: siteverify unreachable: %v", ErrChallengeFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: siteverify status %d", ErrChallengeFailed, resp.StatusCode)
	}

	var payload siteverifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("%w: decode siteverify response: %v", ErrChallengeFailed, err)
	}
	if !payload.Success {
		return fmt.Errorf("%w: %v", ErrChallengeFailed, payload.ErrorCodes)
	}
	return nil
}
