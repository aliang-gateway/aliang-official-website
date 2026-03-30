package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server          ServerConfig   `yaml:"server"`
	Database        DatabaseConfig `yaml:"database"`
	Auth            AuthConfig     `yaml:"auth"`
	Register        RegisterConfig `yaml:"register"`
	SMTP            SMTPConfig     `yaml:"smtp"`
	Redis           RedisConfig    `yaml:"redis"`
	Stripe          StripeConfig   `yaml:"stripe"`
	Sub2APIBaseURL  string         `yaml:"sub2api_base_url"`
	Sub2APIAdminKey string         `yaml:"sub2api_admin_key"`
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
	SecretKey     string `yaml:"secret_key"`
	PublishableKey string `yaml:"publishable_key"`
	WebhookSecret string `yaml:"webhook_secret"`
	Currency      string `yaml:"currency"`
	SuccessURL    string `yaml:"success_url"`
	CancelURL     string `yaml:"cancel_url"`
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

	if envBaseURL, ok := os.LookupEnv("SUB2API_BASE_URL"); ok {
		cfg.Sub2APIBaseURL = envBaseURL
	}
	if envAdminKey, ok := os.LookupEnv("SUB2API_ADMIN_KEY"); ok {
		cfg.Sub2APIAdminKey = envAdminKey
	}
	if value, ok := os.LookupEnv("STRIPE_SECRET_KEY"); ok {
		cfg.Stripe.SecretKey = value
	}
	if value, ok := os.LookupEnv("STRIPE_PUBLISHABLE_KEY"); ok {
		cfg.Stripe.PublishableKey = value
	}
	if value, ok := os.LookupEnv("STRIPE_WEBHOOK_SECRET"); ok {
		cfg.Stripe.WebhookSecret = value
	}
	if value, ok := os.LookupEnv("STRIPE_CURRENCY"); ok {
		cfg.Stripe.Currency = value
	}
	if value, ok := os.LookupEnv("STRIPE_SUCCESS_URL"); ok {
		cfg.Stripe.SuccessURL = value
	}
	if value, ok := os.LookupEnv("STRIPE_CANCEL_URL"); ok {
		cfg.Stripe.CancelURL = value
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	c.Sub2APIBaseURL = strings.TrimRight(strings.TrimSpace(c.Sub2APIBaseURL), "/")
	c.Sub2APIAdminKey = strings.TrimSpace(c.Sub2APIAdminKey)
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
		return fmt.Errorf("SUB2API_BASE_URL is required")
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

	return nil
}
