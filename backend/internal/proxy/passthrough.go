package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RequestTimeout = 10 * time.Second
	maxRetries     = 1
)

var (
	requestHeaderAllowlist = []string{
		http.CanonicalHeaderKey("Authorization"),
		http.CanonicalHeaderKey("Content-Type"),
		http.CanonicalHeaderKey("Accept"),
		http.CanonicalHeaderKey("User-Agent"),
		http.CanonicalHeaderKey("X-Request-Id"),
	}
	hopByHopHeaders = map[string]struct{}{
		http.CanonicalHeaderKey("Connection"):          {},
		http.CanonicalHeaderKey("Keep-Alive"):          {},
		http.CanonicalHeaderKey("Proxy-Authenticate"):  {},
		http.CanonicalHeaderKey("Proxy-Authorization"): {},
		http.CanonicalHeaderKey("Te"):                  {},
		http.CanonicalHeaderKey("Trailer"):             {},
		http.CanonicalHeaderKey("Transfer-Encoding"):   {},
		http.CanonicalHeaderKey("Upgrade"):             {},
	}
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(baseURL string) (*Client, error) {
	return NewClientWithHTTPClient(baseURL, &http.Client{Timeout: RequestTimeout})
}

func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("http client is required")
	}

	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("base url must be absolute")
	}

	return &Client{baseURL: parsedBaseURL, httpClient: httpClient}, nil
}

func (c *Client) Do(ctx context.Context, incoming *http.Request, upstreamPath string) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("proxy client is nil")
	}
	if incoming == nil {
		return nil, errors.New("incoming request is nil")
	}

	upstreamURL, err := BuildUpstreamURL(c.baseURL, upstreamPath, incoming.URL.RawQuery)
	if err != nil {
		return nil, err
	}

	bodyBytes := []byte{}
	if incoming.Body != nil {
		readBody, readErr := io.ReadAll(incoming.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read request body: %w", readErr)
		}
		bodyBytes = readBody
		_ = incoming.Body.Close()
	}

	retryableMethod := incoming.Method == http.MethodGet || incoming.Method == http.MethodHead

	for attempt := 0; attempt <= maxRetries; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
		upstreamReq, buildErr := http.NewRequestWithContext(reqCtx, incoming.Method, upstreamURL.String(), bytes.NewReader(bodyBytes))
		if buildErr != nil {
			cancel()
			return nil, fmt.Errorf("build upstream request: %w", buildErr)
		}
		if len(bodyBytes) > 0 {
			upstreamReq.ContentLength = int64(len(bodyBytes))
		}

		copyAllowedRequestHeaders(upstreamReq.Header, incoming)
		setForwardedHeaders(upstreamReq.Header, incoming)

		resp, doErr := c.httpClient.Do(upstreamReq)
		if doErr != nil {
			cancel()
			if retryableMethod && attempt < maxRetries && isRetryableNetworkError(doErr) {
				continue
			}
			return nil, doErr
		}

		stripHopByHopHeaders(resp.Header)
		if retryableMethod && attempt < maxRetries && isRetryableStatus(resp.StatusCode) {
			cancel()
			drainAndClose(resp.Body)
			continue
		}
		resp.Body = &cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}

		return resp, nil
	}

	return nil, errors.New("upstream request failed")
}

func BuildUpstreamURL(baseURL *url.URL, upstreamPath, rawQuery string) (*url.URL, error) {
	if baseURL == nil {
		return nil, errors.New("base url is required")
	}

	relativeURL, err := url.Parse(strings.TrimSpace(upstreamPath))
	if err != nil {
		return nil, fmt.Errorf("parse upstream path: %w", err)
	}
	if relativeURL.IsAbs() || relativeURL.Host != "" || strings.HasPrefix(strings.TrimSpace(upstreamPath), "//") {
		return nil, errors.New("upstream path must be relative")
	}

	pathPart := strings.TrimLeft(relativeURL.Path, "/")
	joinedPath := strings.TrimRight(baseURL.Path, "/")
	if pathPart != "" {
		joinedPath = joinedPath + "/" + pathPart
	} else if joinedPath == "" {
		joinedPath = "/"
	}

	queryToUse := relativeURL.RawQuery
	if rawQuery != "" {
		queryToUse = rawQuery
	}

	queryValues := url.Values{}
	if queryToUse != "" {
		parsedQuery, parseErr := url.ParseQuery(queryToUse)
		if parseErr != nil {
			return nil, fmt.Errorf("parse query: %w", parseErr)
		}
		for key, values := range parsedQuery {
			for _, value := range values {
				queryValues.Add(key, value)
			}
		}
	}

	finalURL := *baseURL
	finalURL.Path = joinedPath
	finalURL.RawQuery = queryValues.Encode()
	return &finalURL, nil
}

func CopyResponse(w http.ResponseWriter, resp *http.Response) error {
	if w == nil {
		return errors.New("response writer is nil")
	}
	if resp == nil {
		return errors.New("upstream response is nil")
	}

	defer resp.Body.Close()

	headers := cloneHeader(resp.Header)
	stripHopByHopHeaders(headers)
	for name, values := range headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, err := io.Copy(w, resp.Body)
	return err
}

func copyAllowedRequestHeaders(dst http.Header, incoming *http.Request) {
	connectionTokens := connectionHeaderTokens(incoming.Header)
	for _, name := range requestHeaderAllowlist {
		if _, blocked := connectionTokens[name]; blocked {
			continue
		}
		for _, value := range incoming.Header.Values(name) {
			dst.Add(name, value)
		}
	}
	stripHopByHopHeaders(dst)
}

func setForwardedHeaders(dst http.Header, incoming *http.Request) {
	dst.Del("X-Forwarded-For")
	dst.Del("X-Forwarded-Proto")
	dst.Del("X-Forwarded-Host")

	if ip := remoteIP(incoming.RemoteAddr); ip != "" {
		dst.Set("X-Forwarded-For", ip)
	}

	if incoming.TLS == nil {
		dst.Set("X-Forwarded-Proto", "http")
	} else {
		dst.Set("X-Forwarded-Proto", "https")
	}

	if host := strings.TrimSpace(incoming.Host); host != "" {
		dst.Set("X-Forwarded-Host", host)
	}
}

func stripHopByHopHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	for name := range hopByHopHeaders {
		headers.Del(name)
	}

	for _, value := range headers.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(token)
			if trimmed == "" {
				continue
			}
			headers.Del(trimmed)
		}
	}
	headers.Del("Connection")
}

func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		if errors.As(urlErr.Err, &netErr) {
			return true
		}
	}

	return false
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	if r.cancel != nil {
		r.cancel()
	}
	return err
}

func cloneHeader(headers http.Header) http.Header {
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}

func remoteIP(remoteAddr string) string {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return trimmed
	}

	return strings.Trim(host, "[]")
}

func connectionHeaderTokens(headers http.Header) map[string]struct{} {
	blocked := map[string]struct{}{}
	for _, value := range headers.Values(http.CanonicalHeaderKey("Connection")) {
		for _, token := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(token)
			if trimmed == "" {
				continue
			}
			blocked[http.CanonicalHeaderKey(trimmed)] = struct{}{}
		}
	}
	return blocked
}
