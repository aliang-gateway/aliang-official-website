package scanlogin_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ai-api-portal/backend/internal/scanlogin"

	_ "modernc.org/sqlite"
)

func newTestService(t *testing.T) (*scanlogin.Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`CREATE TABLE als_users(id INTEGER PRIMARY KEY, email TEXT, name TEXT, role TEXT)`,
		`CREATE TABLE als_sessions(id INTEGER PRIMARY KEY, user_id INTEGER, token_hash TEXT, expires_at TIMESTAMP)`,
		`CREATE TABLE als_scan_codes(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_code_hash TEXT UNIQUE,
			scan_code_hash TEXT UNIQUE,
			status TEXT DEFAULT 'pending',
			user_id INTEGER,
			session_token_hash TEXT,
			session_token TEXT DEFAULT '',
			init_ip TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			scanned_at TIMESTAMP,
			authorized_at TIMESTAMP,
			denied_at TIMESTAMP)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	return scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{db: db}}), db
}

// stubMinter 忠实模拟 user.Service.MintSessionForUser：返回 token 并写一行 als_sessions。
type stubMinter struct{ db *sql.DB }

func (m stubMinter) MintSessionForUser(ctx context.Context, userID int64) (string, string, error) {
	plaintext := "st_stub_" + fmt.Sprint(userID)
	tokenHash := "hash_" + fmt.Sprint(userID)
	_, err := m.db.ExecContext(ctx, `INSERT INTO als_sessions(user_id, token_hash, expires_at) VALUES (?,?,?)`, userID, tokenHash, time.Now().Add(time.Hour))
	return plaintext, tokenHash, err
}

func TestInitCreatesRowAndReturnsCodes(t *testing.T) {
	svc, db := newTestService(t)
	res, err := svc.Init(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.HasPrefix(res.DeviceCode, "dc_") {
		t.Fatalf("device code: %q", res.DeviceCode)
	}
	if !strings.HasPrefix(res.ScanCode, "sc_") {
		t.Fatalf("scan code: %q", res.ScanCode)
	}
	if res.QRPayload != res.ScanCode {
		t.Fatalf("qr payload should equal scan code")
	}
	if res.ExpiresIn <= 0 || res.Interval <= 0 {
		t.Fatalf("bad expires/interval: %+v", res)
	}
	var devHash, scanHash, status, token string
	err = db.QueryRow(`SELECT device_code_hash, scan_code_hash, status, session_token FROM als_scan_codes`).Scan(&devHash, &scanHash, &status, &token)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status=%s", status)
	}
	if token != "" {
		t.Fatalf("init must not store a session token")
	}
	if devHash == res.DeviceCode || scanHash == res.ScanCode {
		t.Fatalf("must store hashes, not plaintext")
	}
}

// keep errors import used even when later tests removed temporarily
var _ = errors.Is

