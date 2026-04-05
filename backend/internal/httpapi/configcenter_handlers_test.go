package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDefaultConfigRendersKnownGlobalVarsForAuthorizedUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 101)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	userID, sessionToken := createUserViaAPI(t, mux, "config-user@example.com", "Config User", "user", "")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")
	insertDefaultConfigFixture(t, ctx, database, "codex", "Codex", 101, "codex", "default", "json", `{"api_key":"{{apikey}}","base_url":"{{base_url}}"}`)
	insertGlobalTemplateVar(t, ctx, database, "base_url", "https://example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/configs/default?tag=codex", nil)
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		SoftwareCode string `json:"software_code"`
		SoftwareName string `json:"software_name"`
		TemplateName string `json:"template_name"`
		Format       string `json:"format"`
		Content      string `json:"content"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SoftwareCode != "codex" || payload.SoftwareName != "Codex" {
		t.Fatalf("unexpected software identity: %+v", payload)
	}
	if payload.TemplateName != "default" || payload.Format != "json" {
		t.Fatalf("unexpected template metadata: %+v", payload)
	}
	if payload.Content != `{"api_key":"{{apikey}}","base_url":"https://example.com"}` {
		t.Fatalf("expected rendered template content, got %q", payload.Content)
	}
}

func TestGetDefaultConfigRejectsUnauthorizedGroup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	tierID := insertTier(t, ctx, database, "starter", "Starter")
	insertTierGroupBinding(t, ctx, database, tierID, 101)

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	userID, sessionToken := createUserViaAPI(t, mux, "config-user-2@example.com", "Config User 2", "user", "")
	insertActiveSubscription(t, ctx, database, userID, tierID, "2026-01-01T00:00:00Z")
	insertDefaultConfigFixture(t, ctx, database, "claude", "Claude", 202, "claude", "default", "json", `{"api_key":"{{apikey}}"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/configs/default?tag=claude", nil)
	setBearerAuth(req, sessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func TestGetDefaultConfigAllowsAdminByTag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})

	_, adminSessionToken := createUserViaAPI(t, mux, "config-admin@example.com", "Config Admin", "admin", "test-admin-secret")
	insertDefaultConfigFixture(t, ctx, database, "opencode", "OpenCode", 303, "opencode", "default", "yaml", "api_key: {{apikey}}")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/configs/default?tag=opencode", nil)
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		SoftwareCode string `json:"software_code"`
		Format       string `json:"format"`
		Content      string `json:"content"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SoftwareCode != "opencode" || payload.Format != "yaml" || payload.Content != "api_key: {{apikey}}" {
		t.Fatalf("unexpected admin payload: %+v", payload)
	}
}

func insertDefaultConfigFixture(t *testing.T, ctx context.Context, database *sql.DB, softwareCode, softwareName string, groupID int64, tag, templateName, format, content string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `
		INSERT INTO als_software_configs(software_code, software_name, group_id, description, is_enabled)
		VALUES (?, ?, ?, '', 1);
	`, softwareCode, softwareName, groupID)
	if err != nil {
		t.Fatalf("insert software config error: %v", err)
	}

	softwareConfigID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert software config LastInsertId error: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_software_tags(software_config_id, tag)
		VALUES (?, ?);
	`, softwareConfigID, tag); err != nil {
		t.Fatalf("insert software tag error: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_config_templates(software_config_id, name, format, content, is_default)
		VALUES (?, ?, ?, ?, 1);
	`, softwareConfigID, templateName, format, content); err != nil {
		t.Fatalf("insert config template error: %v", err)
	}

	return softwareConfigID
}

func insertGlobalTemplateVar(t *testing.T, ctx context.Context, database *sql.DB, key, value string) {
	t.Helper()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO als_global_template_vars(var_key, var_value, description)
		VALUES (?, ?, '');
	`, key, value); err != nil {
		t.Fatalf("insert global template var error: %v", err)
	}
}
