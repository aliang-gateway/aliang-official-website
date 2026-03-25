package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RequiresSub2APIBaseURL(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")

	path := writeConfigFile(t, `
server:
  port: "8081"
database:
  path: "./test.db"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error when SUB2API_BASE_URL is missing")
	}
	if !strings.Contains(err.Error(), "SUB2API_BASE_URL is required") {
		t.Fatalf("expected SUB2API_BASE_URL required error, got %v", err)
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

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}
