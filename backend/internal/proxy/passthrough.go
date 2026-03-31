package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type APIError struct {
	StatusCode int
	Code       int
	Message    string
	Reason     string
	RetryAfter time.Duration
	Body       []byte
}

const (
	ReasonIdempotencyKeyRequired  = "IDEMPOTENCY_KEY_REQUIRED"
	ReasonIdempotencyKeyInvalid   = "IDEMPOTENCY_KEY_INVALID"
	ReasonIdempotencyKeyConflict  = "IDEMPOTENCY_KEY_CONFLICT"
	ReasonIdempotencyInProgress   = "IDEMPOTENCY_IN_PROGRESS"
	ReasonIdempotencyRetryBackoff = "IDEMPOTENCY_RETRY_BACKOFF"
	ReasonIdempotencyStoreDown    = "IDEMPOTENCY_STORE_UNAVAILABLE"
	RedeemTypeSubscription        = "subscription"
	RedeemTypeBalance             = "balance"
)

func (e *APIError) Error() string {
	if e == nil {
		return "sub2api api error"
	}
	parts := []string{fmt.Sprintf("sub2api status %d", e.StatusCode)}
	if strings.TrimSpace(e.Reason) != "" {
		parts = append(parts, "reason="+strings.TrimSpace(e.Reason))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	return strings.Join(parts, " ")
}

func (e *APIError) IsRetryable() bool {
	if e == nil {
		return false
	}
	switch strings.TrimSpace(e.Reason) {
	case ReasonIdempotencyInProgress, ReasonIdempotencyRetryBackoff, ReasonIdempotencyStoreDown:
		return true
	default:
		return e.StatusCode >= 500
	}
}

func (e *APIError) IsConflict() bool {
	if e == nil {
		return false
	}
	return strings.TrimSpace(e.Reason) == ReasonIdempotencyKeyConflict || e.StatusCode == http.StatusConflict
}

type AdminRequest struct {
	Method         string
	Path           string
	Body           any
	IdempotencyKey string
	Headers        http.Header
}

type CreateAndRedeemRequest struct {
	Code         string `json:"code"`
	Type         string `json:"type,omitempty"`
	Value        float64 `json:"value"`
	UserID       int64  `json:"user_id"`
	GroupID      *int64 `json:"group_id,omitempty"`
	ValidityDays *int   `json:"validity_days,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type UpdateUserBalanceRequest struct {
	Balance   float64 `json:"balance"`
	Operation string  `json:"operation"`
	Notes     string  `json:"notes,omitempty"`
}

type AdminUser struct {
	ID      int64   `json:"id"`
	Balance float64 `json:"balance"`
	Email   string  `json:"email,omitempty"`
	Name    string  `json:"name,omitempty"`
}

type CreateUserAPIKeyRequest struct {
	Name          string   `json:"name"`
	GroupID       int64    `json:"group_id"`
	CustomKey     string   `json:"custom_key,omitempty"`
	IPWhitelist   []string `json:"ip_whitelist,omitempty"`
	IPBlacklist   []string `json:"ip_blacklist,omitempty"`
	Quota         int64    `json:"quota,omitempty"`
	ExpiresInDays int      `json:"expires_in_days,omitempty"`
	RateLimit5H   int64    `json:"rate_limit_5h,omitempty"`
	RateLimit1D   int64    `json:"rate_limit_1d,omitempty"`
	RateLimit7D   int64    `json:"rate_limit_7d,omitempty"`
}

type APIKey struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Key      string `json:"key,omitempty"`
	GroupID  int64  `json:"group_id"`
	UserID   int64  `json:"user_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Quota    int64  `json:"quota,omitempty"`
	Platform string `json:"platform,omitempty"`
}

type AdminGroup struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Code             string `json:"code,omitempty"`
	Platform         string `json:"platform,omitempty"`
	Type             string `json:"type,omitempty"`
	SubscriptionType string `json:"subscription_type,omitempty"`
}

type ResponseEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type CreateAndRedeemData struct {
	RedeemCode RedeemCode `json:"redeem_code"`
}

type RedeemCode struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Type         string `json:"type"`
	Value        float64 `json:"value"`
	Status       string `json:"status"`
	UsedBy       *int64 `json:"used_by,omitempty"`
	GroupID      *int64 `json:"group_id,omitempty"`
	ValidityDays *int   `json:"validity_days,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

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
		http.CanonicalHeaderKey("Idempotency-Key"),
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

type ClientOptions struct {
	AdminAPIKey string
}

type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	adminAPIKey string
}

func NewClient(baseURL string) (*Client, error) {
	return NewClientWithOptions(baseURL, &http.Client{Timeout: RequestTimeout}, ClientOptions{})
}

func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) (*Client, error) {
	return NewClientWithOptions(baseURL, httpClient, ClientOptions{})
}

func NewClientWithOptions(baseURL string, httpClient *http.Client, opts ClientOptions) (*Client, error) {
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

	return &Client{baseURL: parsedBaseURL, httpClient: httpClient, adminAPIKey: strings.TrimSpace(opts.AdminAPIKey)}, nil
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

func (c *Client) DoAdminJSON(ctx context.Context, req AdminRequest, out any) error {
	if c == nil {
		return errors.New("proxy client is nil")
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = http.MethodPost
	}
	if strings.TrimSpace(req.Path) == "" {
		return errors.New("admin request path is required")
	}
	if strings.TrimSpace(c.adminAPIKey) == "" {
		return errors.New("sub2api admin api key is required")
	}

	bodyBytes := []byte(nil)
	if req.Body != nil {
		encoded, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshal admin request body: %w", err)
		}
		bodyBytes = encoded
	}

	upstreamURL, err := BuildUpstreamURL(c.baseURL, req.Path, "")
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, method, upstreamURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build admin request: %w", err)
	}
	if len(bodyBytes) > 0 {
		httpReq.ContentLength = int64(len(bodyBytes))
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.adminAPIKey)
	if key := strings.TrimSpace(req.IdempotencyKey); key != "" {
		httpReq.Header.Set("Idempotency-Key", key)
	}
	for name, values := range req.Headers {
		canonicalName := http.CanonicalHeaderKey(name)
		if canonicalName == http.CanonicalHeaderKey("x-api-key") || canonicalName == http.CanonicalHeaderKey("Content-Length") {
			continue
		}
		httpReq.Header.Del(canonicalName)
		for _, value := range values {
			httpReq.Header.Add(canonicalName, value)
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read admin response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp, body)
	}
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode admin response: %w", err)
	}
	return nil
}

func (c *Client) CreateAndRedeem(ctx context.Context, req CreateAndRedeemRequest, idempotencyKey string) (*ResponseEnvelope[CreateAndRedeemData], error) {
	if err := validateCreateAndRedeemRequest(req, idempotencyKey); err != nil {
		return nil, err
	}
	var resp ResponseEnvelope[CreateAndRedeemData]
	if err := c.DoAdminJSON(ctx, AdminRequest{
		Method:         http.MethodPost,
		Path:           "/api/v1/admin/redeem-codes/create-and-redeem",
		Body:           req,
		IdempotencyKey: idempotencyKey,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateUserBalance(ctx context.Context, userID int64, req UpdateUserBalanceRequest, idempotencyKey string) (*ResponseEnvelope[AdminUser], error) {
	if err := validateUpdateUserBalanceRequest(userID, req, idempotencyKey); err != nil {
		return nil, err
	}
	var resp ResponseEnvelope[AdminUser]
	if err := c.DoAdminJSON(ctx, AdminRequest{
		Method:         http.MethodPost,
		Path:           fmt.Sprintf("/api/v1/admin/users/%d/balance", userID),
		Body:           req,
		IdempotencyKey: idempotencyKey,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateUserAPIKey(ctx context.Context, bearerToken string, req CreateUserAPIKeyRequest, idempotencyKey string) (*ResponseEnvelope[APIKey], error) {
	if err := validateCreateUserAPIKeyRequest(req, bearerToken, idempotencyKey); err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal api key request body: %w", err)
	}

	upstreamURL, err := BuildUpstreamURL(c.baseURL, "/api/v1/keys", "")
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, upstreamURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build api key request: %w", err)
	}
	httpReq.ContentLength = int64(len(bodyBytes))
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	httpReq.Header.Set("Idempotency-Key", strings.TrimSpace(idempotencyKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read api key response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp, body)
	}

	var decoded ResponseEnvelope[APIKey]
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode api key response: %w", err)
	}
	return &decoded, nil
}

func (c *Client) ListAdminGroups(ctx context.Context, platform string) (*ResponseEnvelope[[]AdminGroup], error) {
	path := "/api/v1/admin/groups/all"
	platform = strings.TrimSpace(platform)
	if platform != "" {
		path = path + "?platform=" + url.QueryEscape(platform)
	}
	var resp ResponseEnvelope[[]AdminGroup]
	if err := c.DoAdminJSON(ctx, AdminRequest{Method: http.MethodGet, Path: path}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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

func parseAPIError(resp *http.Response, body []byte) error {
	if resp == nil {
		return errors.New("upstream response is nil")
	}

	type apiErrorEnvelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		Body:       append([]byte(nil), body...),
	}
	var envelope apiErrorEnvelope
	if len(bytes.TrimSpace(body)) > 0 && json.Unmarshal(body, &envelope) == nil {
		apiErr.Code = envelope.Code
		apiErr.Message = strings.TrimSpace(envelope.Message)
		apiErr.Reason = strings.TrimSpace(envelope.Reason)
	}
	if apiErr.Message == "" && len(bytes.TrimSpace(body)) > 0 {
		apiErr.Message = string(bytes.TrimSpace(body))
	}
	return apiErr
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if retryAt, err := http.ParseTime(raw); err == nil {
		delta := time.Until(retryAt)
		if delta > 0 {
			return delta
		}
	}
	return 0
}

func validateCreateAndRedeemRequest(req CreateAndRedeemRequest, idempotencyKey string) error {
	if strings.TrimSpace(idempotencyKey) == "" {
		return errors.New("idempotency key is required")
	}
	if len(strings.TrimSpace(req.Code)) < 3 {
		return errors.New("code must be at least 3 characters")
	}
	if req.UserID <= 0 {
		return errors.New("user_id must be greater than 0")
	}
	redeemType := strings.TrimSpace(req.Type)
	if redeemType == "" {
		redeemType = RedeemTypeBalance
	}
	if redeemType == RedeemTypeSubscription {
		if req.Value < 0 {
			return errors.New("value must be zero or greater for subscription redeem")
		}
		if req.GroupID == nil || *req.GroupID <= 0 {
			return errors.New("group_id is required for subscription redeem")
		}
		if req.ValidityDays == nil || *req.ValidityDays <= 0 {
			return errors.New("validity_days must be greater than 0 for subscription redeem")
		}
		return nil
	}
	if req.Value <= 0 {
		return errors.New("value must be greater than 0")
	}
	return nil
}

func validateUpdateUserBalanceRequest(userID int64, req UpdateUserBalanceRequest, idempotencyKey string) error {
	if userID <= 0 {
		return errors.New("user id must be greater than 0")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return errors.New("idempotency key is required")
	}
	if req.Balance <= 0 {
		return errors.New("balance must be greater than 0")
	}
	switch strings.TrimSpace(req.Operation) {
	case "set", "add", "subtract":
		return nil
	default:
		return errors.New("operation must be one of: set, add, subtract")
	}
}

func validateCreateUserAPIKeyRequest(req CreateUserAPIKeyRequest, bearerToken, idempotencyKey string) error {
	if strings.TrimSpace(bearerToken) == "" {
		return errors.New("bearer token is required")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return errors.New("idempotency key is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if req.GroupID <= 0 {
		return errors.New("group_id must be greater than 0")
	}
	return nil
}
