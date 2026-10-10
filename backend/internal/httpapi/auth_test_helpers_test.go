package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"ai-api-portal/backend/internal/auth"
)

func setBearerAuth(req *http.Request, sessionToken string) {
	req.Header.Set("Authorization", "Bearer "+sessionToken)
}

// createTestUser is the shared user fixture: inserts the user directly with
// the requested role and mints a live session for it. It used to drive
// POST /users, but that endpoint now requires an authenticated admin
// (register-abuse hardening), and seeding keeps auto-increment ids and
// admin-count invariants (e.g. the last-admin guard) deterministic. The
// mux and adminBootstrapSecret parameters are kept for call-site
// compatibility and are unused.
func createTestUser(t *testing.T, database *sql.DB, mux *http.ServeMux, email, name, role, adminBootstrapSecret string) (int64, string) {
	t.Helper()
	_, _ = mux, adminBootstrapSecret

	ctx := context.Background()
	result, err := database.ExecContext(ctx,
		`INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, email, name, role)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("seed user id: %v", err)
	}

	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint session: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`INSERT INTO als_sessions(user_id, token_hash, expires_at) VALUES (?, ?, ?);`,
		userID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return userID, plaintext
}

// seedUserWithSession inserts a local user with the given role and mints a
// session for it, returning the plaintext bearer token.
func seedUserWithSession(t *testing.T, database *sql.DB, email, role string) string {
	t.Helper()

	ctx := context.Background()
	result, err := database.ExecContext(ctx,
		`INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, email, "Seedy "+role, role)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("seed user id: %v", err)
	}

	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint session: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`INSERT INTO als_sessions(user_id, token_hash, expires_at) VALUES (?, ?, ?);`,
		userID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return plaintext
}
