package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultWebhookTolerance = 5 * time.Minute

type Config struct {
	SecretKey     string
	WebhookSecret string
	Currency      string
	SuccessURL    string
	CancelURL     string
}

type Client struct {
	secretKey       string
	webhookSecret   string
	currency        string
	successURL      string
	cancelURL       string
	httpClient      *http.Client
	now             func() time.Time
	webhookTolerance time.Duration
}

type CheckoutSessionInput struct {
	PackageCode   string
	PackageName   string
	UserID        int64
	CustomerEmail string
	AmountMinor   int64
}

type CheckoutSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type CheckoutSessionCompleted struct {
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	PaymentStatus     string            `json:"payment_status"`
	ClientReferenceID string            `json:"client_reference_id"`
	CustomerEmail     string            `json:"customer_email"`
	Metadata          map[string]string `json:"metadata"`
	AmountTotal       int64             `json:"amount_total"`
	Currency          string            `json:"currency"`
}

func NewClient(cfg Config) (*Client, error) {
	return NewClientWithHTTPClient(cfg, newDefaultHTTPClient())
}

func NewClientWithHTTPClient(cfg Config, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("http client is required")
	}

	secretKey := strings.TrimSpace(cfg.SecretKey)
	webhookSecret := strings.TrimSpace(cfg.WebhookSecret)
	successURL := strings.TrimSpace(cfg.SuccessURL)
	cancelURL := strings.TrimSpace(cfg.CancelURL)
	currency := strings.ToLower(strings.TrimSpace(cfg.Currency))
	if currency == "" {
		currency = "cny"
	}

	if secretKey == "" {
		return nil, errors.New("stripe secret key is required")
	}
	if webhookSecret == "" {
		return nil, errors.New("stripe webhook secret is required")
	}
	if successURL == "" {
		return nil, errors.New("stripe success url is required")
	}
	if cancelURL == "" {
		return nil, errors.New("stripe cancel url is required")
	}

	return &Client{
		secretKey:        secretKey,
		webhookSecret:    webhookSecret,
		currency:         currency,
		successURL:       successURL,
		cancelURL:        cancelURL,
		httpClient:       httpClient,
		now:              func() time.Time { return time.Now().UTC() },
		webhookTolerance: defaultWebhookTolerance,
	}, nil
}

func (c *Client) CreateCheckoutSession(ctx context.Context, input CheckoutSessionInput) (*CheckoutSession, error) {
	if c == nil {
		return nil, errors.New("stripe client is nil")
	}
	if input.UserID <= 0 {
		return nil, errors.New("user id must be positive")
	}
	if strings.TrimSpace(input.PackageCode) == "" {
		return nil, errors.New("package code is required")
	}
	if strings.TrimSpace(input.PackageName) == "" {
		return nil, errors.New("package name is required")
	}
	if input.AmountMinor <= 0 {
		return nil, errors.New("amount_minor must be positive")
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", appendSessionIDParam(c.successURL))
	form.Set("cancel_url", c.cancelURL)
	form.Set("billing_address_collection", "auto")
	form.Set("client_reference_id", strconv.FormatInt(input.UserID, 10))
	form.Set("metadata[user_id]", strconv.FormatInt(input.UserID, 10))
	form.Set("metadata[tier_code]", strings.TrimSpace(input.PackageCode))
	form.Set("metadata[package_name]", strings.TrimSpace(input.PackageName))
	if strings.TrimSpace(input.CustomerEmail) != "" {
		form.Set("customer_email", strings.TrimSpace(input.CustomerEmail))
	}
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", c.currency)
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(input.AmountMinor, 10))
	form.Set("line_items[0][price_data][product_data][name]", strings.TrimSpace(input.PackageName))
	form.Set("line_items[0][price_data][product_data][metadata][tier_code]", strings.TrimSpace(input.PackageCode))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build stripe checkout session request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create stripe checkout session: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stripe checkout session response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("stripe checkout session status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var session CheckoutSession
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("decode stripe checkout session response: %w", err)
	}
	if strings.TrimSpace(session.ID) == "" || strings.TrimSpace(session.URL) == "" {
		return nil, errors.New("stripe checkout session response is missing id or url")
	}

	return &session, nil
}

func (c *Client) Currency() string {
	if c == nil || strings.TrimSpace(c.currency) == "" {
		return "cny"
	}
	return c.currency
}

func (c *Client) ConstructEvent(payload []byte, signatureHeader string) (*Event, error) {
	if c == nil {
		return nil, errors.New("stripe client is nil")
	}
	if len(payload) == 0 {
		return nil, errors.New("stripe webhook payload is required")
	}
	if err := c.verifyWebhookSignature(payload, signatureHeader); err != nil {
		return nil, err
	}

	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode stripe webhook event: %w", err)
	}
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Type) == "" {
		return nil, errors.New("stripe webhook event is missing id or type")
	}
	return &event, nil
}

func appendSessionIDParam(rawURL string) string {
	if strings.Contains(rawURL, "{CHECKOUT_SESSION_ID}") {
		return rawURL
	}
	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + "session_id={CHECKOUT_SESSION_ID}"
}

func newDefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

func (c *Client) verifyWebhookSignature(payload []byte, signatureHeader string) error {
	parts := strings.Split(strings.TrimSpace(signatureHeader), ",")
	if len(parts) == 0 {
		return errors.New("stripe signature header is required")
	}

	var (
		timestamp int64
		signature string
	)
	for _, part := range parts {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				timestamp = parsed
			}
		case "v1":
			signature = value
		}
	}

	if timestamp <= 0 || signature == "" {
		return errors.New("invalid stripe signature header")
	}

	now := c.now()
	signedAt := time.Unix(timestamp, 0).UTC()
	if now.Sub(signedAt) > c.webhookTolerance || signedAt.Sub(now) > c.webhookTolerance {
		return errors.New("stripe webhook signature is outside the allowed tolerance")
	}

	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	provided, err := hex.DecodeString(signature)
	if err != nil {
		return errors.New("invalid stripe webhook signature")
	}
	if !hmac.Equal(expected, provided) {
		return errors.New("stripe webhook signature verification failed")
	}
	return nil
}
