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
			session_token TEXT,
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

// stubRefreshResolver 模拟 sub2api Gateway.UpstreamRefreshToken，可控地返回 refresh_token / found。
type stubRefreshResolver struct {
	token string
	found bool
	err   error
}

func (s stubRefreshResolver) UpstreamRefreshToken(ctx context.Context, userID int64) (string, bool, error) {
	return s.token, s.found, s.err
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
	var (
		devHash, scanHash, status string
		token                     sql.NullString
	)
	err = db.QueryRow(`SELECT device_code_hash, scan_code_hash, status, session_token FROM als_scan_codes`).Scan(&devHash, &scanHash, &status, &token)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status=%s", status)
	}
	if token.Valid {
		t.Fatalf("init must not store a session token, got %q", token.String)
	}
	if devHash == res.DeviceCode || scanHash == res.ScanCode {
		t.Fatalf("must store hashes, not plaintext")
	}
}

func TestStatusLifecycle(t *testing.T) {
	_, db := newTestService(t)
	frozen := time.Now()
	svc2 := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen }})

	init, err := svc2.Init(context.Background(), "")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := svc2.Status(context.Background(), init.DeviceCode)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Status != scanlogin.StatusPending {
		t.Fatalf("want pending, got %s", got.Status)
	}
	if got.SessionToken != "" {
		t.Fatalf("pending must not leak token")
	}
	if _, err := svc2.Status(context.Background(), "dc_bogus"); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := db.Exec(`UPDATE als_scan_codes SET status='scanned', user_id=7 WHERE scan_code_hash=?`, scanlogin.Hash(init.ScanCode)); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = svc2.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusScanned {
		t.Fatalf("want scanned, got %s", got.Status)
	}
	if _, err := db.Exec(`UPDATE als_scan_codes SET status='authorized', session_token='st_xyz', user_id=7`); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(7,'u@x.com','U','user')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	got, _ = svc2.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.SessionToken != "st_xyz" {
		t.Fatalf("want token st_xyz, got %q", got.SessionToken)
	}
	if got.User == nil || got.User.ID != 7 || got.User.Email != "u@x.com" {
		t.Fatalf("bad user: %+v", got.User)
	}
	svc3 := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen.Add(scanlogin.DefaultTTL + time.Second) }})
	got, _ = svc3.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusExpired {
		t.Fatalf("want expired, got %s", got.Status)
	}
}

func TestStatusAuthorizedExposesRefreshToken(t *testing.T) {
	_, db := newTestService(t)
	harness := newResolverHarness(t, db, stubRefreshResolver{token: "rt_secret", found: true})

	got, err := harness.Status(context.Background(), harness.deviceCode)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.SessionToken != "st_xyz" {
		t.Fatalf("session token: %q", got.SessionToken)
	}
	if got.RefreshToken != "rt_secret" {
		t.Fatalf("want refresh_token rt_secret, got %q", got.RefreshToken)
	}
}

func TestStatusAuthorizedOmitsRefreshTokenWhenAbsent(t *testing.T) {
	_, db := newTestService(t)
	// resolver 报告 not-found（用户无 upstream 令牌）：refresh_token 应缺席，st_ 仍下发。
	harness := newResolverHarness(t, db, stubRefreshResolver{found: false})
	got, _ := harness.Status(context.Background(), harness.deviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.RefreshToken != "" {
		t.Fatalf("refresh_token should be omitted when not found, got %q", got.RefreshToken)
	}
	if got.SessionToken != "st_xyz" {
		t.Fatalf("session token still delivered: %q", got.SessionToken)
	}
}

func TestStatusAuthorizedOmitsRefreshTokenWithoutResolver(t *testing.T) {
	_, db := newTestService(t)
	// 不注入 resolver（nil）：authorized 仍正常，refresh_token 缺席。
	harness := newResolverHarness(t, db, nil)
	got, _ := harness.Status(context.Background(), harness.deviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.RefreshToken != "" {
		t.Fatalf("refresh_token should be omitted without resolver, got %q", got.RefreshToken)
	}
}

// newResolverHarness 把一条 scan code 推进到 authorized（含 session_token + user_id）并 seed 用户，
// 返回一个带指定 resolver 的 Service 及其 device_code。resolver 为 nil 时不注入。
type resolverHarness struct {
	*scanlogin.Service
	deviceCode string
}

func newResolverHarness(t *testing.T, db *sql.DB, resolver scanlogin.RefreshTokenResolver) *resolverHarness {
	t.Helper()
	frozen := time.Now()
	opts := scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen }}
	if resolver != nil {
		opts.RefreshTokenResolver = resolver
	}
	svc := scanlogin.NewService(db, opts)
	init, err := svc.Init(context.Background(), "")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(7,'u@x.com','U','user')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.Exec(`UPDATE als_scan_codes SET status='authorized', session_token='st_xyz', user_id=7 WHERE device_code_hash=?`, scanlogin.Hash(init.DeviceCode)); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	return &resolverHarness{Service: svc, deviceCode: init.DeviceCode}
}

func TestScanTransitionsAndGuards(t *testing.T) {
	svc, _ := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Scan(context.Background(), init.ScanCode, 42); err != nil {
		t.Fatalf("scan: %v", err)
	}
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusScanned {
		t.Fatalf("want scanned, got %s", got.Status)
	}
	if err := svc.Scan(context.Background(), init.ScanCode, 42); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on rescan, got %v", err)
	}
	if err := svc.Scan(context.Background(), "sc_bogus", 42); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestConfirmBindsUserAndMintsToken(t *testing.T) {
	svc, db := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Scan(context.Background(), init.ScanCode, 9); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(9,'c@x.com','C','user')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.Confirm(context.Background(), init.ScanCode, 99); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for mismatched confirmer, got %v", err)
	}
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.SessionToken == "" || got.User == nil || got.User.ID != 9 {
		t.Fatalf("bad authorized result: %+v", got)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_sessions WHERE user_id=9`).Scan(&n)
	if n != 1 {
		t.Fatalf("als_sessions should have 1 row, got %d", n)
	}
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on reconfirm, got %v", err)
	}
}

func TestDenyFromScannedOrPending(t *testing.T) {
	svc, _ := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Deny(context.Background(), init.ScanCode); err != nil {
		t.Fatalf("deny pending: %v", err)
	}
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusDenied {
		t.Fatalf("want denied, got %s", got.Status)
	}
	if err := svc.Deny(context.Background(), init.ScanCode); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on re-deny, got %v", err)
	}
}

func TestCleanupExpiredDeletesOldRows(t *testing.T) {
	svc, db := newTestService(t)
	_, _ = svc.Init(context.Background(), "")
	if _, err := db.Exec(`UPDATE als_scan_codes SET expires_at = ?`, time.Now().Add(-time.Hour).UTC()); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := svc.CleanupExpired(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_scan_codes`).Scan(&n)
	if n != 0 {
		t.Fatalf("want 0 rows after cleanup, got %d", n)
	}
}
