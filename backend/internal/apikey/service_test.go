package apikey

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"ai-api-portal/backend/internal/db"
)

func TestGenerateAPIKeyFormatAndUniqueness(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() error = %v", err)
		}

		if !strings.HasPrefix(key, "ak_") {
			t.Fatalf("expected key prefix ak_, got %q", key)
		}

		if len(key) < 67 {
			t.Fatalf("expected key length >= 67, got %d", len(key))
		}

		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = struct{}{}
	}
}

func TestCreateKeyStoresHashAndReturnsPlaintext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)
	userID := createUser(t, ctx, database, "u1@example.com", "User One", "user")

	created, err := service.CreateKey(ctx, userID, "ci")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	if created.APIKey == "" {
		t.Fatalf("expected plaintext api key in create response")
	}

	if !strings.HasPrefix(created.APIKey, "ak_") {
		t.Fatalf("expected plaintext key prefix ak_, got %q", created.APIKey)
	}

	var storedHash string
	err = database.QueryRowContext(ctx, `SELECT key_hash FROM api_keys WHERE id = ?;`, created.ID).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query stored hash error = %v", err)
	}

	if storedHash == created.APIKey {
		t.Fatalf("api key stored in plaintext")
	}

	if storedHash != HashAPIKey(created.APIKey) {
		t.Fatalf("stored hash does not match plaintext api key")
	}
}

func TestRevokeKeyDisablesActiveState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)
	userID := createUser(t, ctx, database, "u2@example.com", "User Two", "user")

	created, err := service.CreateKey(ctx, userID, "runtime")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	active, err := service.IsKeyActive(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("IsKeyActive() before revoke error = %v", err)
	}
	if !active {
		t.Fatalf("expected key to be active before revoke")
	}

	revoked, err := service.RevokeKey(ctx, created.ID, userID, false)
	if err != nil {
		t.Fatalf("RevokeKey() error = %v", err)
	}
	if !revoked {
		t.Fatalf("expected revoke to succeed")
	}

	active, err = service.IsKeyActive(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("IsKeyActive() after revoke error = %v", err)
	}
	if active {
		t.Fatalf("expected key to be inactive after revoke")
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(ctx, dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func createUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO users(email, name, role) VALUES (?, ?, ?);`, email, name, role)
	if err != nil {
		t.Fatalf("createUser insert error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("createUser LastInsertId error = %v", err)
	}

	return id
}
