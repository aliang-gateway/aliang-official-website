package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server                    ServerConfig   `yaml:"server"`
	LogLevel                  string         `yaml:"log_level"`
	Database                  DatabaseConfig `yaml:"database"`
	Auth                      AuthConfig     `yaml:"auth"`
	Register                  RegisterConfig `yaml:"register"`
	SMTP                      SMTPConfig     `yaml:"smtp"`
	Redis                     RedisConfig    `yaml:"redis"`
	Stripe                    StripeConfig   `yaml:"stripe"`
	Sub2APIBaseURL            string         `yaml:"sub2api_base_url"`
	Sub2APIAdminKey           string         `yaml:"sub2api_admin_key"`
	Sub2APITokenEncryptionKey string         `yaml:"sub2api_token_encryption_key"`
	// Companion phone platform server admin API, used only for the best-effort
	// mirror purge during self-service account deletion. Optional: when either
	// value is empty the mirror purge is skipped entirely.
	PhoneServerAdminURL      string `yaml:"phone_server_admin_url"`
	PhoneServerAdminPassword string `yaml:"phone_server_admin_password"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
	DSN    string `yaml:"dsn"`
}

func (c DatabaseConfig) EffectiveDSN() string {
	if strings.TrimSpace(c.DSN) != "" {
		return strings.TrimSpace(c.DSN)
	}

	if strings.EqualFold(strings.TrimSpace(c.Driver), "sqlite") {
		return strings.TrimSpace(c.Path)
	}

	return ""
}

type AuthConfig struct {
	AdminBootstrapSecret string `yaml:"admin_bootstrap_secret"`
}

type RegisterConfig struct {
	AllowedEmailDomains      []string `yaml:"allowed_email_domains"`
	RequireEmailVerification *bool    `yaml:"require_email_verification"`
	// Anti-abuse guard. Turnstile activates only when the secret is set
	// (config file or TURNSTILE_SECRET_KEY env), so the rollout can stage:
	// deploy code first, then add the key to enforce the captcha.
	TurnstileSecretKey string `yaml:"turnstile_secret_key"`
	// Registration rate limits. 0 → built-in default, negative → disabled.
	MaxPerIPPerHour   int `yaml:"max_per_ip_per_hour"`
	MaxPerEmailPerDay int `yaml:"max_per_email_per_day"`
	// BlockDisposableDomains nil → true (block the built-in disposable list).
	BlockDisposableDomains *bool `yaml:"block_disposable_domains"`
	// SliderCaptcha nil → true (self-hosted go-captcha slider on register).
	SliderCaptcha *bool `yaml:"slider_captcha"`
}

// ShouldBlockDisposableDomains reports whether well-known disposable email
// providers should be rejected at registration (default true).
func (c RegisterConfig) ShouldBlockDisposableDomains() bool {
	return c.BlockDisposableDomains == nil || *c.BlockDisposableDomains
}

// SliderCaptchaEnabled reports whether the self-hosted slider captcha gates
// the public registration path (default true).
func (c RegisterConfig) SliderCaptchaEnabled() bool {
	return c.SliderCaptcha == nil || *c.SliderCaptcha
}

// EmailVerificationEnabled reports whether registration requires a pre-verified
// email code (portal-side, independent of upstream sub2api). nil → false:
// the admin toggles it at runtime; this config only seeds the default.
func (c RegisterConfig) EmailVerificationEnabled() bool {
	return c.RequireEmailVerification != nil && *c.RequireEmailVerification
}

type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type StripeConfig struct {
	SecretKey      string `yaml:"secret_key"`
	PublishableKey string `yaml:"publishable_key"`
	WebhookSecret  string `yaml:"webhook_secret"`
	Currency       string `yaml:"currency"`
	SuccessURL     string `yaml:"success_url"`
	CancelURL      string `yaml:"cancel_url"`
}

func Load(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config: %w", err)
	}

	applyEnvString(&cfg.Sub2APIBaseURL, "SUB2API_BASE_URL")
	applyEnvString(&cfg.Sub2APIAdminKey, "SUB2API_ADMIN_KEY")
	applyEnvString(&cfg.Sub2APITokenEncryptionKey, "SUB2API_TOKEN_ENCRYPTION_KEY")
	applyEnvString(&cfg.PhoneServerAdminURL, "PHONE_SERVER_ADMIN_URL")
	applyEnvString(&cfg.PhoneServerAdminPassword, "PHONE_SERVER_ADMIN_PASSWORD")
	applyEnvString(&cfg.Stripe.SecretKey, "STRIPE_SECRET_KEY")
	applyEnvString(&cfg.Stripe.PublishableKey, "STRIPE_PUBLISHABLE_KEY")
	applyEnvString(&cfg.Stripe.WebhookSecret, "STRIPE_WEBHOOK_SECRET")
	applyEnvString(&cfg.Stripe.Currency, "STRIPE_CURRENCY")
	applyEnvString(&cfg.Stripe.SuccessURL, "STRIPE_SUCCESS_URL")
	applyEnvString(&cfg.Stripe.CancelURL, "STRIPE_CANCEL_URL")
	applyEnvString(&cfg.LogLevel, "LOG_LEVEL")
	applyEnvString(&cfg.Register.TurnstileSecretKey, "TURNSTILE_SECRET_KEY")
	applyEnvBool(&cfg.Register.RequireEmailVerification, "REGISTER_REQUIRE_EMAIL_VERIFICATION")

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	c.Sub2APIBaseURL = strings.TrimRight(strings.TrimSpace(c.Sub2APIBaseURL), "/")
	c.Sub2APIAdminKey = strings.TrimSpace(c.Sub2APIAdminKey)
	c.Sub2APITokenEncryptionKey = strings.TrimSpace(c.Sub2APITokenEncryptionKey)
	c.PhoneServerAdminURL = strings.TrimRight(strings.TrimSpace(c.PhoneServerAdminURL), "/")
	c.PhoneServerAdminPassword = strings.TrimSpace(c.PhoneServerAdminPassword)
	c.Stripe.SecretKey = strings.TrimSpace(c.Stripe.SecretKey)
	c.Stripe.PublishableKey = strings.TrimSpace(c.Stripe.PublishableKey)
	c.Stripe.WebhookSecret = strings.TrimSpace(c.Stripe.WebhookSecret)
	c.Stripe.Currency = strings.ToLower(strings.TrimSpace(c.Stripe.Currency))
	c.Stripe.SuccessURL = strings.TrimSpace(c.Stripe.SuccessURL)
	c.Stripe.CancelURL = strings.TrimSpace(c.Stripe.CancelURL)
	c.Database.Driver = strings.ToLower(strings.TrimSpace(c.Database.Driver))
	c.Database.Path = strings.TrimSpace(c.Database.Path)
	c.Database.DSN = strings.TrimSpace(c.Database.DSN)

	if strings.TrimSpace(c.Server.Port) == "" {
		c.Server.Port = "8080"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.Driver == "sqlite" && c.Database.EffectiveDSN() == "" {
		c.Database.Path = "./data.db"
	}
	if c.Stripe.Currency == "" {
		c.Stripe.Currency = "cny"
	}
	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	c.Register.TurnstileSecretKey = strings.TrimSpace(c.Register.TurnstileSecretKey)
	if c.Register.MaxPerIPPerHour == 0 {
		c.Register.MaxPerIPPerHour = 10
	}
	if c.Register.MaxPerEmailPerDay == 0 {
		c.Register.MaxPerEmailPerDay = 5
	}
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("server.port is required")
	}
	if c.Database.Driver != "sqlite" && c.Database.Driver != "postgres" {
		return fmt.Errorf("database.driver must be one of: sqlite, postgres")
	}
	if c.Database.Driver == "sqlite" && c.Database.EffectiveDSN() == "" {
		return fmt.Errorf("database.path or database.dsn is required for sqlite")
	}
	if c.Database.Driver == "postgres" && strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("database.dsn is required for postgres")
	}
	if c.Sub2APIBaseURL == "" {
		return fmt.Errorf("sub2api_base_url is required (set in config file or SUB2API_BASE_URL env)")
	}
	if c.Sub2APITokenEncryptionKey == "" {
		return fmt.Errorf("sub2api_token_encryption_key is required (set in config file or SUB2API_TOKEN_ENCRYPTION_KEY env)")
	}
	if !isEncoded32ByteKey(c.Sub2APITokenEncryptionKey) {
		return fmt.Errorf("sub2api_token_encryption_key must encode exactly 32 bytes (base64 or hex)")
	}
	if c.Stripe.SecretKey != "" || c.Stripe.WebhookSecret != "" || c.Stripe.SuccessURL != "" || c.Stripe.CancelURL != "" {
		if c.Stripe.SecretKey == "" || c.Stripe.WebhookSecret == "" || c.Stripe.SuccessURL == "" || c.Stripe.CancelURL == "" {
			return fmt.Errorf("stripe secret_key, webhook_secret, success_url, and cancel_url must all be set together")
		}
	}

	if c.SMTP.Enabled {
		if strings.TrimSpace(c.SMTP.Host) == "" || c.SMTP.Port <= 0 || strings.TrimSpace(c.SMTP.Username) == "" || strings.TrimSpace(c.SMTP.Password) == "" || strings.TrimSpace(c.SMTP.From) == "" {
			return fmt.Errorf("smtp enabled requires host, port, username, password, from")
		}
	}

	if c.Register.EmailVerificationEnabled() && !c.SMTP.Enabled {
		return fmt.Errorf("register.require_email_verification requires smtp.enabled=true (verification codes are sent by this portal)")
	}

	return nil
}

func isEncoded32ByteKey(value string) bool {
	for _, decode := range []func(string) ([]byte, error){base64.StdEncoding.DecodeString, base64.RawStdEncoding.DecodeString, hex.DecodeString} {
		if key, err := decode(strings.TrimSpace(value)); err == nil && len(key) == 32 {
			return true
		}
	}
	return false
}

func applyEnvBool(target **bool, key string) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	enabled := strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
	*target = &enabled
}

func applyEnvString(target *string, key string) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	*target = value
}
