package user

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/db"
)

func TestHashPasswordAndCheckPassword(t *testing.T) {
	t.Parallel()

	password := "CorrectHorseBatteryStaple!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if hash == password {
		t.Fatalf("password appears to be stored plaintext")
	}

	if !CheckPassword(password, hash) {
		t.Fatalf("expected CheckPassword() to accept valid password")
	}
	if CheckPassword("wrong-password", hash) {
		t.Fatalf("expected CheckPassword() to reject invalid password")
	}
}

func TestLoginReturnsUserAndCreatesSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	password := "Password#123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	userID := createUserWithPassword(t, ctx, database, "login@example.com", "Login User", "user", hash)

	authUser, err := service.Login(ctx, "login@example.com", password)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if authUser.ID != userID {
		t.Fatalf("unexpected user id: got %d want %d", authUser.ID, userID)
	}
	if authUser.Email != "login@example.com" || authUser.Name != "Login User" || authUser.Role != "user" {
		t.Fatalf("unexpected auth user payload: %+v", authUser)
	}
	if authUser.SessionToken == "" {
		t.Fatalf("expected non-empty session token")
	}

	var (
		storedHash string
		expiresAt  time.Time
		revokedAt  sql.NullTime
	)
	err = database.QueryRowContext(ctx, `
		SELECT token_hash, expires_at, revoked_at
		FROM sessions
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT 1;
	`, userID).Scan(&storedHash, &expiresAt, &revokedAt)
	if err != nil {
		t.Fatalf("query created session error = %v", err)
	}

	if storedHash != auth.HashSessionToken(authUser.SessionToken) {
		t.Fatalf("stored session hash mismatch")
	}
	if revokedAt.Valid {
		t.Fatalf("expected new session to be active")
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected session expiration in the future")
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	hash, err := HashPassword("Password#123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	createUserWithPassword(t, ctx, database, "invalid@example.com", "Invalid User", "user", hash)

	_, err = service.Login(ctx, "invalid@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	_, err = service.Login(ctx, "missing@example.com", "Password#123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for missing user, got %v", err)
	}
}

func TestGetAndUpdateProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	hash, err := HashPassword("Password#123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	userID := createUserWithPassword(t, ctx, database, "profile@example.com", "Profile User", "user", hash)

	profile, err := service.GetProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.Email != "profile@example.com" || profile.Name != "Profile User" || profile.Role != "user" {
		t.Fatalf("unexpected profile before update: %+v", profile)
	}

	err = service.UpdateProfile(ctx, userID, "Updated Name", "updated@example.com")
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	updated, err := service.GetProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetProfile() after update error = %v", err)
	}
	if updated.Name != "Updated Name" || updated.Email != "updated@example.com" {
		t.Fatalf("unexpected profile after update: %+v", updated)
	}
}

func TestUpdateProfileEmailTaken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	hash, err := HashPassword("Password#123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	userOneID := createUserWithPassword(t, ctx, database, "one@example.com", "One", "user", hash)
	createUserWithPassword(t, ctx, database, "two@example.com", "Two", "user", hash)

	err = service.UpdateProfile(ctx, userOneID, "One", "two@example.com")
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	oldPassword := "OldPassword#123"
	hash, err := HashPassword(oldPassword)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	userID := createUserWithPassword(t, ctx, database, "pw@example.com", "Password User", "user", hash)

	err = service.ChangePassword(ctx, userID, oldPassword, "NewPassword#789")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	_, err = service.Login(ctx, "pw@example.com", oldPassword)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail login, got %v", err)
	}

	_, err = service.Login(ctx, "pw@example.com", "NewPassword#789")
	if err != nil {
		t.Fatalf("expected new password login success, got %v", err)
	}
}

func TestChangePasswordWrongCurrentPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	hash, err := HashPassword("OldPassword#123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	userID := createUserWithPassword(t, ctx, database, "wrongcurrent@example.com", "Wrong Current", "user", hash)

	err = service.ChangePassword(ctx, userID, "not-correct", "NewPassword#789")
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestListSessionsAndLogout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	hash, err := HashPassword("Password#123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	userID := createUserWithPassword(t, ctx, database, "sessions@example.com", "Sessions User", "user", hash)

	loginOne, err := service.Login(ctx, "sessions@example.com", "Password#123")
	if err != nil {
		t.Fatalf("first login error = %v", err)
	}
	_, err = service.Login(ctx, "sessions@example.com", "Password#123")
	if err != nil {
		t.Fatalf("second login error = %v", err)
	}

	sessions, err := service.ListSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	err = service.Logout(ctx, userID, loginOne.SessionToken)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	sessions, err = service.ListSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListSessions() after logout error = %v", err)
	}

	var revokedCount int
	for _, session := range sessions {
		if session.RevokedAt != nil {
			revokedCount++
		}
	}
	if revokedCount != 1 {
		t.Fatalf("expected exactly one revoked session, got %d", revokedCount)
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

func createUserWithPassword(t *testing.T, ctx context.Context, database *sql.DB, email, name, role, passwordHash string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `
		INSERT INTO users(email, name, role, password_hash)
		VALUES (?, ?, ?, ?);
	`, email, name, role, passwordHash)
	if err != nil {
		t.Fatalf("createUserWithPassword insert error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("createUserWithPassword LastInsertId error = %v", err)
	}

	return id
}
