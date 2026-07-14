package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RequiresSub2APIBaseURL(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")
	t.Setenv("SUB2API_ADMIN_KEY", "")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error when sub2api_base_url is missing")
	}
	if !strings.Contains(err.Error(), "sub2api_base_url is required") {
		t.Fatalf("expected sub2api_base_url required error, got %v", err)
	}
}

func TestLoad_UsesYamlSub2APIBaseURLWithoutEnv(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")
	t.Setenv("SUB2API_ADMIN_KEY", "")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
sub2api_base_url: "https://sub2api.yaml.example.com///"
sub2api_admin_key: "yaml-admin-key"
stripe:
  secret_key: "sk_yaml"
  webhook_secret: "whsec_yaml"
  success_url: "https://portal.example.com/success"
  cancel_url: "https://portal.example.com/cancel"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Sub2APIBaseURL != "https://sub2api.yaml.example.com" {
		t.Fatalf("expected yaml sub2api base url, got %q", cfg.Sub2APIBaseURL)
	}
	if cfg.Sub2APIAdminKey != "yaml-admin-key" {
		t.Fatalf("expected yaml sub2api admin key, got %q", cfg.Sub2APIAdminKey)
	}
	if cfg.Stripe.SecretKey != "sk_yaml" {
		t.Fatalf("expected yaml stripe secret key, got %q", cfg.Stripe.SecretKey)
	}
}

func TestLoad_RequiresValidSub2APITokenEncryptionKey(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")
	t.Setenv("SUB2API_TOKEN_ENCRYPTION_KEY", "")
	for _, tc := range []struct {
		name string
		key  string
		want string
	}{
		{name: "missing", want: "sub2api_token_encryption_key is required"},
		{name: "invalid", key: "too-short", want: "must encode exactly 32 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := "server:\n  port: \"8081\"\ndatabase:\n  path: \"./test.db\"\nsub2api_base_url: \"https://sub2api.example.com\"\n"
			if tc.key != "" {
				body += "sub2api_token_encryption_key: \"" + tc.key + "\"\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoad_UsesEnvSub2APIBaseURLAndTrimsTrailingSlash(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com///")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
sub2api_base_url: "https://yaml.example.com/"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Sub2APIBaseURL != "https://sub2api.internal.example.com" {
		t.Fatalf("expected normalized env sub2api base url, got %q", cfg.Sub2APIBaseURL)
	}
}

func TestLoad_UsesEnvSub2APIAdminKey(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")
	t.Setenv("SUB2API_ADMIN_KEY", "  admin-secret-key  ")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
sub2api_admin_key: "yaml-key"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Sub2APIAdminKey != "admin-secret-key" {
		t.Fatalf("expected env sub2api admin key override, got %q", cfg.Sub2APIAdminKey)
	}
}

func TestLoad_UsesEnvStripeSettings(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_SUCCESS_URL", "https://portal.example.com/dashboard?checkout=success")
	t.Setenv("STRIPE_CANCEL_URL", "https://portal.example.com/dashboard?checkout=cancelled")
	t.Setenv("STRIPE_CURRENCY", "USD")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Stripe.SecretKey != "sk_test_123" {
		t.Fatalf("expected stripe secret key override, got %q", cfg.Stripe.SecretKey)
	}
	if cfg.Stripe.WebhookSecret != "whsec_123" {
		t.Fatalf("expected stripe webhook secret override, got %q", cfg.Stripe.WebhookSecret)
	}
	if cfg.Stripe.Currency != "usd" {
		t.Fatalf("expected normalized stripe currency usd, got %q", cfg.Stripe.Currency)
	}
}

func TestLoad_RejectsPartialStripeConfig(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
stripe:
  secret_key: "sk_test_only"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error for partial stripe config")
	}
	if !strings.Contains(err.Error(), "stripe secret_key, webhook_secret, success_url, and cancel_url must all be set together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_DefaultsToSQLitePath(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")

	path := writeConfigFile(t, `
server:
  port: "8081"
database: {}
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("expected database.driver=sqlite default, got %q", cfg.Database.Driver)
	}
	if cfg.Database.Path != "./data.db" {
		t.Fatalf("expected database.path default ./data.db, got %q", cfg.Database.Path)
	}
	if cfg.Database.EffectiveDSN() != "./data.db" {
		t.Fatalf("expected effective sqlite dsn ./data.db, got %q", cfg.Database.EffectiveDSN())
	}
}

func TestLoad_PostgresRequiresDSN(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  driver: "postgres"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error when postgres dsn is missing")
	}
	if !strings.Contains(err.Error(), "database.dsn is required for postgres") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PostgresUsesDSN(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  driver: "POSTGRES"
  dsn: "postgres://app:secret@localhost:5432/app?sslmode=disable"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Driver != "postgres" {
		t.Fatalf("expected normalized driver postgres, got %q", cfg.Database.Driver)
	}
	if cfg.Database.EffectiveDSN() != "postgres://app:secret@localhost:5432/app?sslmode=disable" {
		t.Fatalf("unexpected effective dsn: %q", cfg.Database.EffectiveDSN())
	}
}

func TestLoad_SQLitePrefersDSNOverPath(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "https://sub2api.internal.example.com")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  driver: "sqlite"
  path: "./ignored.db"
  dsn: "file::memory:?cache=shared"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.EffectiveDSN() != "file::memory:?cache=shared" {
		t.Fatalf("expected sqlite dsn override, got %q", cfg.Database.EffectiveDSN())
	}
}

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	const testTokenEncryptionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	body = strings.TrimSpace(body) + "\nsub2api_token_encryption_key: \"" + testTokenEncryptionKey + "\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}
