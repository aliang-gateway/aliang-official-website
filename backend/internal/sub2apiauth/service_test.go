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

	resolvedUpstreamUserID, found, err := service.GetUpstreamUserIDByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetUpstreamUserIDByUserID error = %v", err)
	}
	if !found || resolvedUpstreamUserID != upstreamUserID {
		t.Fatalf("expected upstream user id %d, found=%v got=%d", upstreamUserID, found, resolvedUpstreamUserID)
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

func TestServiceGetRefreshTokenByUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())

	// 有 refresh_token：正常返回。
	userID := createUser(t, ctx, database, "refresh-ok@example.com", "Refresh OK", "user")
	refresh := "upstream-refresh-1"
	if err := service.UpsertToken(ctx, UpsertTokenInput{
		UserID:       userID,
		AccessToken:  "access-1",
		RefreshToken: &refresh,
	}); err != nil {
		t.Fatalf("UpsertToken error = %v", err)
	}
	got, err := service.GetRefreshTokenByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetRefreshTokenByUserID error = %v", err)
	}
	if got != refresh {
		t.Fatalf("expected refresh %q, got %q", refresh, got)
	}

	// 仅 access、无 refresh：归为 ErrTokenNotFound（列可空）。
	accessOnly := createUser(t, ctx, database, "refresh-absent@example.com", "No Refresh", "user")
	if err := service.UpsertToken(ctx, UpsertTokenInput{UserID: accessOnly, AccessToken: "access-2"}); err != nil {
		t.Fatalf("UpsertToken access-only error = %v", err)
	}
	if _, err := service.GetRefreshTokenByUserID(ctx, accessOnly); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound for missing refresh, got %v", err)
	}

	// 不存在的用户：ErrTokenNotFound。
	if _, err := service.GetRefreshTokenByUserID(ctx, 99999); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound for unknown user, got %v", err)
	}
}

func TestServiceRotationStateMachine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "rotation@example.com", "Rotation", "user")
	refresh0 := "upstream-r0"
	expires0 := time.Now().UTC().Add(-time.Minute)
	if err := service.UpsertToken(ctx, UpsertTokenInput{UserID: userID, AccessToken: "upstream-a0", RefreshToken: &refresh0, AccessExpiresAt: &expires0}); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	if err := service.BeginRotation(ctx, userID, refresh0, 0); err != nil {
		t.Fatalf("BeginRotation() error = %v", err)
	}
	rotating, err := service.LoadVault(ctx, userID)
	if err != nil {
		t.Fatalf("LoadVault(rotating) error = %v", err)
	}
	if rotating.RotationState != "rotating" || !rotating.HasRotationStarted {
		t.Fatalf("rotating vault state = %+v", rotating)
	}
	if err := service.BeginRotation(ctx, userID, refresh0, 0); !errors.Is(err, ErrRotationConflict) {
		t.Fatalf("second BeginRotation() error = %v, want ErrRotationConflict", err)
	}

	refresh1 := "upstream-r1"
	expires1 := time.Now().UTC().Add(time.Hour)
	if err := service.CompleteRotation(ctx, refresh0, 0, UpsertTokenInput{UserID: userID, AccessToken: "upstream-a1", RefreshToken: &refresh1, AccessExpiresAt: &expires1}); err != nil {
		t.Fatalf("CompleteRotation() error = %v", err)
	}
	stable, err := service.LoadVault(ctx, userID)
	if err != nil {
		t.Fatalf("LoadVault(stable) error = %v", err)
	}
	if stable.RotationState != "stable" || stable.RefreshToken != refresh1 || stable.PrevRefreshToken != refresh0 || stable.AccessToken != "upstream-a1" || stable.Version != 1 {
		t.Fatalf("completed vault = %+v", stable)
	}
	if err := service.BeginRotation(ctx, userID, refresh0, 1); !errors.Is(err, ErrRotationConflict) {
		t.Fatalf("BeginRotation(old token) error = %v, want ErrRotationConflict", err)
	}
	if err := service.BeginRotation(ctx, userID, refresh1, 1); err != nil {
		t.Fatalf("BeginRotation(current token) error = %v", err)
	}
	if err := service.ResetRotation(ctx, userID, refresh1, 1); err != nil {
		t.Fatalf("ResetRotation() error = %v", err)
	}
	reset, err := service.LoadVault(ctx, userID)
	if err != nil || reset.RotationState != "stable" || reset.RefreshToken != refresh1 {
		t.Fatalf("reset vault = %+v err=%v", reset, err)
	}
}

func TestServiceEncryptsUpstreamCredentialsAtRest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	const key = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	service := NewServiceWithDialectAndKey(database, testDialect(), key)
	userID := createUser(t, ctx, database, "encrypted@example.com", "Encrypted", "user")
	refresh := "plain-refresh-secret"
	if err := service.UpsertToken(ctx, UpsertTokenInput{UserID: userID, AccessToken: "plain-access-secret", RefreshToken: &refresh}); err != nil {
		t.Fatalf("UpsertToken() error = %v", err)
	}
	var storedAccess, storedRefresh string
	if err := database.QueryRowContext(ctx, db.Rebind(testDialect(), `SELECT access_token, refresh_token FROM als_sub2api_auth_tokens WHERE user_id = ?`), userID).Scan(&storedAccess, &storedRefresh); err != nil {
		t.Fatalf("query encrypted tokens: %v", err)
	}
	if storedAccess == "plain-access-secret" || storedRefresh == refresh || !strings.HasPrefix(storedAccess, encryptedTokenPrefix) || !strings.HasPrefix(storedRefresh, encryptedTokenPrefix) {
		t.Fatalf("credentials were not encrypted at rest: access=%q refresh=%q", storedAccess, storedRefresh)
	}
	if got, err := service.GetBearerTokenByUserID(ctx, userID); err != nil || got != "plain-access-secret" {
		t.Fatalf("GetBearerTokenByUserID() = %q, %v", got, err)
	}
	vault, err := service.LoadVault(ctx, userID)
	if err != nil || vault.AccessToken != "plain-access-secret" || vault.RefreshToken != refresh {
		t.Fatalf("LoadVault() = %+v, %v", vault, err)
	}
}

func TestServiceCompleteRotationRejectsStaleVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	service := NewServiceWithDialect(database, testDialect())
	userID := createUser(t, ctx, database, "stale-version@example.com", "Stale Version", "user")
	refresh0 := "r0"
	if err := service.UpsertToken(ctx, UpsertTokenInput{UserID: userID, AccessToken: "a0", RefreshToken: &refresh0}); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	if err := service.BeginRotation(ctx, userID, refresh0, 0); err != nil {
		t.Fatalf("BeginRotation() error = %v", err)
	}
	if _, err := database.ExecContext(ctx, db.Rebind(testDialect(), `UPDATE als_sub2api_auth_tokens SET version = version + 1 WHERE user_id = ?`), userID); err != nil {
		t.Fatalf("simulate competing writer: %v", err)
	}
	refresh1 := "r1"
	err := service.CompleteRotation(ctx, refresh0, 0, UpsertTokenInput{UserID: userID, AccessToken: "a1", RefreshToken: &refresh1})
	if !errors.Is(err, ErrRotationConflict) {
		t.Fatalf("CompleteRotation() error = %v, want ErrRotationConflict", err)
	}
}

func TestServiceReencryptsLegacyPlaintextCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := setupTestDB(t)
	userID := createUser(t, ctx, database, "legacy-encryption@example.com", "Legacy Encryption", "user")
	if _, err := database.ExecContext(ctx, db.Rebind(testDialect(), `
		INSERT INTO als_sub2api_auth_tokens(user_id, access_token, refresh_token, prev_refresh_token)
		VALUES (?, ?, ?, ?)
	`), userID, "legacy-access", "legacy-refresh", "legacy-prev"); err != nil {
		t.Fatalf("seed legacy plaintext credentials: %v", err)
	}
	const key = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	service := NewServiceWithDialectAndKey(database, testDialect(), key)
	if err := service.ReencryptLegacyCredentials(ctx); err != nil {
		t.Fatalf("ReencryptLegacyCredentials() error = %v", err)
	}
	var storedAccess, storedRefresh, storedPrev string
	if err := database.QueryRowContext(ctx, db.Rebind(testDialect(), `SELECT access_token, refresh_token, prev_refresh_token FROM als_sub2api_auth_tokens WHERE user_id = ?`), userID).Scan(&storedAccess, &storedRefresh, &storedPrev); err != nil {
		t.Fatalf("query migrated credentials: %v", err)
	}
	for name, value := range map[string]string{"access": storedAccess, "refresh": storedRefresh, "previous": storedPrev} {
		if !strings.HasPrefix(value, encryptedTokenPrefix) {
			t.Fatalf("%s credential was not migrated: %q", name, value)
		}
	}
	vault, err := service.LoadVault(ctx, userID)
	if err != nil || vault.AccessToken != "legacy-access" || vault.RefreshToken != "legacy-refresh" || vault.PrevRefreshToken != "legacy-prev" {
		t.Fatalf("LoadVault() after migration = %+v, %v", vault, err)
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
