package sub2apiauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-api-portal/backend/internal/db"
)

func TestServiceUpsertAndGetBearerByUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	userID := createUser(t, ctx, database, "sub2api-auth@example.com", "Sub2API Auth", "user")
	upstreamUserID := int64(1001)
	refresh := "refresh-1"

	if err := service.UpsertToken(ctx, UpsertTokenInput{
		UserID:         userID,
		UpstreamUserID: &upstreamUserID,
		AccessToken:    "access-1",
		RefreshToken:   &refresh,
	}); err != nil {
		t.Fatalf("UpsertToken first call error = %v", err)
	}

	bearer, err := service.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetBearerTokenByUserID first call error = %v", err)
	}
	if bearer != "access-1" {
		t.Fatalf("expected bearer access-1, got %q", bearer)
	}

	if err := service.UpsertToken(ctx, UpsertTokenInput{UserID: userID, AccessToken: "access-2"}); err != nil {
		t.Fatalf("UpsertToken second call error = %v", err)
	}

	bearer, err = service.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetBearerTokenByUserID second call error = %v", err)
	}
	if bearer != "access-2" {
		t.Fatalf("expected bearer access-2, got %q", bearer)
	}

	var (
		storedAccess  string
		storedRefresh sql.NullString
		storedUpID    sql.NullInt64
	)
	err = database.QueryRowContext(ctx, db.Rebind(testDialect(), `
		SELECT access_token, refresh_token, upstream_user_id
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`), userID).Scan(&storedAccess, &storedRefresh, &storedUpID)
	if err != nil {
		t.Fatalf("query als_sub2api_auth_tokens row: %v", err)
	}
	if storedAccess != "access-2" {
		t.Fatalf("expected stored access token access-2, got %q", storedAccess)
	}
	if !storedRefresh.Valid || storedRefresh.String != "refresh-1" {
		t.Fatalf("expected stored refresh token refresh-1, got %+v", storedRefresh)
	}
	if !storedUpID.Valid || storedUpID.Int64 != upstreamUserID {
		t.Fatalf("expected stored upstream user id %d, got %+v", upstreamUserID, storedUpID)
	}
}

func TestServiceGetBearerTokenByUserIDNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	_, err := service.GetBearerTokenByUserID(ctx, 999)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dialect := testDialect()
	if dialect == "postgres" {
		dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
		if dsn == "" {
			t.Skip("DB_DSN is required when DB_DRIVER=postgres")
		}

		bootstrap, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			t.Fatalf("Open bootstrap postgres error = %v", err)
		}

		schema := testSchemaName(t)
		if _, err := bootstrap.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA "%s";`, schema)); err != nil {
			_ = bootstrap.Close()
			t.Fatalf("create schema %s error = %v", schema, err)
		}

		database, err := db.Open(ctx, "postgres", dsn)
		if err != nil {
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("Open postgres error = %v", err)
		}
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		if _, err := database.ExecContext(ctx, fmt.Sprintf(`SET search_path TO "%s";`, schema)); err != nil {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("set search_path error = %v", err)
		}
		if err := db.ApplyMigrations(ctx, database, "postgres"); err != nil {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
			t.Fatalf("ApplyMigrations() error = %v", err)
		}
		t.Cleanup(func() {
			_ = database.Close()
			_, _ = bootstrap.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schema))
			_ = bootstrap.Close()
		})
		return database
	}

	dbFile := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func createUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	id, err := db.InsertID(ctx, testDialect(), database, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, "id", email, name, role)
	if err != nil {
		t.Fatalf("insert user error = %v", err)
	}

	return id
}

func testDialect() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		return "postgres"
	}
	return "sqlite"
}

func testSchemaName(t *testing.T) string {
	name := strings.ToLower(t.Name())
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return fmt.Sprintf("test_%s_%d", builder.String(), time.Now().UnixNano())
}
