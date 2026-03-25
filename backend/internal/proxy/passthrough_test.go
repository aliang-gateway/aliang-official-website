package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildUpstreamURL_SafelyJoinsPathAndUsesIncomingQuery(t *testing.T) {
	t.Parallel()

	baseURL, err := url.Parse("https://sub2.example.com/api/v1")
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	finalURL, err := BuildUpstreamURL(baseURL, "/auth/me?unused=1", "start=1&end=2")
	if err != nil {
		t.Fatalf("build upstream url: %v", err)
	}

	if finalURL.String() != "https://sub2.example.com/api/v1/auth/me?end=2&start=1" {
		t.Fatalf("unexpected upstream url: %q", finalURL.String())
	}
}

func TestBuildUpstreamURL_RejectsAbsoluteOrProtocolRelativeUpstreamPath(t *testing.T) {
	t.Parallel()

	baseURL, err := url.Parse("https://sub2.example.com/api/v1")
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	if _, err := BuildUpstreamURL(baseURL, "https://evil.example.com/x", ""); err == nil {
		t.Fatalf("expected absolute upstream path error")
	}
	if _, err := BuildUpstreamURL(baseURL, "//evil.example.com/x", ""); err == nil {
		t.Fatalf("expected protocol-relative upstream path error")
	}
}

func TestDo_StripsHopByHopAndBuildsForwardedHeaders(t *testing.T) {
	t.Parallel()

	var seenAuth string
	var seenContentType string
	var seenAccept string
	var seenUserAgent string
	var seenRequestID string
	var seenXForwardedFor string
	var seenXForwardedProto string
	var seenXForwardedHost string
	var sawConnection bool
	var sawKeepAlive bool
	var sawProxyAuthorization bool

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenContentType = r.Header.Get("Content-Type")
		seenAccept = r.Header.Get("Accept")
		seenUserAgent = r.Header.Get("User-Agent")
		seenRequestID = r.Header.Get("X-Request-Id")
		seenXForwardedFor = r.Header.Get("X-Forwarded-For")
		seenXForwardedProto = r.Header.Get("X-Forwarded-Proto")
		seenXForwardedHost = r.Header.Get("X-Forwarded-Host")
		sawConnection = r.Header.Get("Connection") != ""
		sawKeepAlive = r.Header.Get("Keep-Alive") != ""
		sawProxyAuthorization = r.Header.Get("Proxy-Authorization") != ""

		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Keep-Alive", "timeout=10")
		w.Header().Set("WWW-Authenticate", `Bearer realm="upstream"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer upstream.Close()

	client, err := NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	incoming := httptest.NewRequest(http.MethodGet, "http://backend.local/auth/me?from=client", nil)
	incoming.RemoteAddr = "203.0.113.9:41234"
	incoming.Host = "backend.local:8081"
	incoming.Header.Set("Authorization", "Bearer test-token")
	incoming.Header.Set("Content-Type", "application/json")
	incoming.Header.Set("Accept", "application/json")
	incoming.Header.Set("User-Agent", "portal-ui/1.0")
	incoming.Header.Set("X-Request-Id", "req-123")
	incoming.Header.Set("Connection", "keep-alive, Proxy-Authorization")
	incoming.Header.Set("Keep-Alive", "timeout=5")
	incoming.Header.Set("Proxy-Authorization", "Basic xxx")

	resp, err := client.Do(context.Background(), incoming, "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("proxy do: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
	if string(body) != `{"error":"unauthorized"}` {
		t.Fatalf("unexpected response body: %q", string(body))
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Bearer realm="upstream"` {
		t.Fatalf("expected WWW-Authenticate passthrough, got %q", got)
	}
	if got := resp.Header.Get("Connection"); got != "" {
		t.Fatalf("expected response connection header stripped, got %q", got)
	}
	if got := resp.Header.Get("Keep-Alive"); got != "" {
		t.Fatalf("expected response keep-alive header stripped, got %q", got)
	}

	if seenAuth != "Bearer test-token" {
		t.Fatalf("expected auth passthrough, got %q", seenAuth)
	}
	if seenContentType != "application/json" {
		t.Fatalf("expected content-type passthrough, got %q", seenContentType)
	}
	if seenAccept != "application/json" {
		t.Fatalf("expected accept passthrough, got %q", seenAccept)
	}
	if seenUserAgent != "portal-ui/1.0" {
		t.Fatalf("expected user-agent passthrough, got %q", seenUserAgent)
	}
	if seenRequestID != "req-123" {
		t.Fatalf("expected x-request-id passthrough, got %q", seenRequestID)
	}
	if sawConnection || sawKeepAlive || sawProxyAuthorization {
		t.Fatalf("expected hop-by-hop request headers stripped")
	}
	if seenXForwardedFor != "203.0.113.9" {
		t.Fatalf("expected x-forwarded-for rebuilt from remote addr, got %q", seenXForwardedFor)
	}
	if seenXForwardedProto != "http" {
		t.Fatalf("expected x-forwarded-proto http, got %q", seenXForwardedProto)
	}
	if seenXForwardedHost != "backend.local:8081" {
		t.Fatalf("expected x-forwarded-host from incoming host, got %q", seenXForwardedHost)
	}

	incomingHTTPS := httptest.NewRequest(http.MethodGet, "https://backend.local/auth/me", nil)
	incomingHTTPS.RemoteAddr = "198.51.100.7:443"
	incomingHTTPS.Host = "secure.backend.local"
	incomingHTTPS.TLS = &tls.ConnectionState{}

	respHTTPS, err := client.Do(context.Background(), incomingHTTPS, "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("proxy do https: %v", err)
	}
	_ = respHTTPS.Body.Close()
	if seenXForwardedProto != "https" {
		t.Fatalf("expected x-forwarded-proto https, got %q", seenXForwardedProto)
	}
}

func TestDo_RetryPolicyByMethodAndStatus(t *testing.T) {
	t.Parallel()

	var attempts int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	client, err := NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "http://backend.local/dashboard/home", nil)
	resp, err := client.Do(context.Background(), getReq, "/api/v1/usage/dashboard/stats")
	if err != nil {
		t.Fatalf("get proxy do: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read get response body: %v", err)
	}
	_ = resp.Body.Close()

	if attempts != 2 {
		t.Fatalf("expected 2 attempts for GET on 502, got %d", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 after retry, got %d", resp.StatusCode)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected get response body: %q", string(body))
	}

	attempts = 0
	postReq := httptest.NewRequest(http.MethodPost, "http://backend.local/auth/login", strings.NewReader(`{"email":"a"}`))
	postResp, err := client.Do(context.Background(), postReq, "/api/v1/auth/login")
	if err != nil {
		t.Fatalf("post proxy do: %v", err)
	}
	_, _ = io.Copy(io.Discard, postResp.Body)
	_ = postResp.Body.Close()

	if attempts != 1 {
		t.Fatalf("expected 1 attempt for POST on 502, got %d", attempts)
	}
	if postResp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status 502 passthrough for POST without retry, got %d", postResp.StatusCode)
	}
}

func TestDo_MethodRetryMatrixOn503(t *testing.T) {
	t.Parallel()

	attemptsByMethod := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptsByMethod[r.Method]++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer upstream.Close()

	client, err := NewClient(upstream.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	testCases := []struct {
		method           string
		expectedAttempts int
	}{
		{method: http.MethodGet, expectedAttempts: 2},
		{method: http.MethodHead, expectedAttempts: 2},
		{method: http.MethodPost, expectedAttempts: 1},
		{method: http.MethodPut, expectedAttempts: 1},
		{method: http.MethodPatch, expectedAttempts: 1},
		{method: http.MethodDelete, expectedAttempts: 1},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, "http://backend.local/retry-matrix", nil)
		resp, doErr := client.Do(context.Background(), req, "/api/v1/retry-matrix")
		if doErr != nil {
			t.Fatalf("%s proxy do: %v", tc.method, doErr)
		}
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s expected status 503 passthrough, got %d", tc.method, resp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if attemptsByMethod[tc.method] != tc.expectedAttempts {
			t.Fatalf("%s expected %d attempts, got %d", tc.method, tc.expectedAttempts, attemptsByMethod[tc.method])
		}
	}
}

func TestDo_RetryOnlyNetworkErrorsForGetHead(t *testing.T) {
	t.Parallel()

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/head") {
			return nil, &url.Error{Op: "Head", URL: req.URL.String(), Err: errors.New("dns failure")}
		}
		if strings.Contains(req.URL.Path, "/get") {
			return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: timeoutNetError{}}
		}
		return nil, errors.New("boom")
	})

	httpClient := &http.Client{Transport: transport}
	client, err := NewClientWithHTTPClient("https://sub2.example.com", httpClient)
	if err != nil {
		t.Fatalf("new client with transport: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "http://backend.local/get", nil)
	_, err = client.Do(context.Background(), getReq, "/get")
	if err == nil {
		t.Fatalf("expected network error after retry")
	}

	headReq := httptest.NewRequest(http.MethodHead, "http://backend.local/head", nil)
	_, err = client.Do(context.Background(), headReq, "/head")
	if err == nil {
		t.Fatalf("expected head network error after retry")
	}

	postReq := httptest.NewRequest(http.MethodPost, "http://backend.local/post", nil)
	_, err = client.Do(context.Background(), postReq, "/post")
	if err == nil {
		t.Fatalf("expected post error")
	}
}

func TestDo_AppliesRequestTimeoutViaContext(t *testing.T) {
	t.Parallel()

	timedOut := false
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if deadline, ok := req.Context().Deadline(); !ok {
			return nil, errors.New("request context missing deadline")
		} else {
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > RequestTimeout+time.Second {
				return nil, errors.New("unexpected deadline range")
			}
		}

		<-req.Context().Done()
		if errors.Is(req.Context().Err(), context.DeadlineExceeded) {
			timedOut = true
		}
		return nil, req.Context().Err()
	})

	httpClient := &http.Client{Transport: transport}
	client, err := NewClientWithHTTPClient("https://sub2.example.com", httpClient)
	if err != nil {
		t.Fatalf("new client with transport: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://backend.local/dashboard/home", nil)
	_, err = client.Do(context.Background(), req, "/api/v1/usage/dashboard/stats")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !timedOut {
		t.Fatalf("expected context deadline exceeded path")
	}
}

func TestCopyResponse_PreservesStatusBodyAndWWWAuthenticate(t *testing.T) {
	t.Parallel()

	upstream := &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"WWW-Authenticate": []string{`Bearer realm="upstream"`},
			"Connection":       []string{"close"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":"forbidden"}`)),
	}

	rec := httptest.NewRecorder()
	if err := CopyResponse(rec, upstream); err != nil {
		t.Fatalf("copy response: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 passthrough, got %d", rec.Code)
	}
	if rec.Body.String() != `{"error":"forbidden"}` {
		t.Fatalf("expected raw body passthrough, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="upstream"` {
		t.Fatalf("expected WWW-Authenticate passthrough, got %q", got)
	}
	if got := rec.Header().Get("Connection"); got != "" {
		t.Fatalf("expected hop-by-hop response header stripped, got %q", got)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }
