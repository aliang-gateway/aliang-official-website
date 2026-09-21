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

func TestStatusAuthorizedReturnsOnlyLocalSessionCredential(t *testing.T) {
	_, db := newTestService(t)
	harness := newResolverHarness(t, db)

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
	if got.RefreshToken != "st_xyz" {
		t.Fatalf("want local refresh credential st_xyz, got %q", got.RefreshToken)
	}
}

func TestStatusAuthorizedDoesNotDependOnUpstreamRefreshToken(t *testing.T) {
	_, db := newTestService(t)
	harness := newResolverHarness(t, db)
	got, _ := harness.Status(context.Background(), harness.deviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.RefreshToken != "st_xyz" {
		t.Fatalf("refresh_token should be the local session, got %q", got.RefreshToken)
	}
	if got.SessionToken != "st_xyz" {
		t.Fatalf("session token still delivered: %q", got.SessionToken)
	}
}

func TestStatusAuthorizedReturnsLocalRefreshWithoutResolver(t *testing.T) {
	_, db := newTestService(t)
	harness := newResolverHarness(t, db)
	got, _ := harness.Status(context.Background(), harness.deviceCode)
	if got.Status != scanlogin.StatusAuthorized {
		t.Fatalf("want authorized, got %s", got.Status)
	}
	if got.RefreshToken != "st_xyz" {
		t.Fatalf("refresh_token should be the local session without resolver, got %q", got.RefreshToken)
	}
}

// newResolverHarness 把一条 scan code 推进到 authorized（含 session_token + user_id）并 seed 用户。
type resolverHarness struct {
	*scanlogin.Service
	deviceCode string
}

func newResolverHarness(t *testing.T, db *sql.DB) *resolverHarness {
	t.Helper()
	frozen := time.Now()
	opts := scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen }}
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
	svc, db := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Scan(context.Background(), init.ScanCode, 42); err != nil {
		t.Fatalf("scan: %v", err)
	}
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusScanned {
		t.Fatalf("want scanned, got %s", got.Status)
	}
	// 同一用户的重复扫码（扫码帧连发/网络重试）必须幂等成功：
	// 2026-09-21 生产事故——iOS 扫码帧双发，第二发 409 后到覆盖成功 UI，
	// 而 scanned 行永远无法重扫，用户被卡死到网页 5 分钟后换码。
	if err := svc.Scan(context.Background(), init.ScanCode, 42); err != nil {
		t.Fatalf("want idempotent success on same-user rescan, got %v", err)
	}
	// 其他用户扫走已绑定的码仍是真冲突。
	if err := svc.Scan(context.Background(), init.ScanCode, 43); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for another user, got %v", err)
	}
	if err := svc.Scan(context.Background(), "sc_bogus", 42); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// 幂等重扫不得改写首次绑定的 user_id。
	if uid := dbUserIDOf(t, db, init.ScanCode); uid != 42 {
		t.Fatalf("user_id should stay 42, got %d", uid)
	}
}

// dbUserIDOf 取 scan code 行当前绑定的 user_id（0 = 未绑定）。
func dbUserIDOf(t *testing.T, db *sql.DB, scanCode string) int64 {
	t.Helper()
	var uid sql.NullInt64
	if err := db.QueryRow(`SELECT user_id FROM als_scan_codes WHERE scan_code_hash=?`, scanlogin.Hash(scanCode)).Scan(&uid); err != nil {
		t.Fatalf("query user_id: %v", err)
	}
	if !uid.Valid {
		return 0
	}
	return uid.Int64
}

// 已 scanned 的行一旦过期，即使同一用户也不能幂等重扫——TTL 是配对窗口硬边界。
func TestScanIdempotentStopsAtExpiry(t *testing.T) {
	_, db := newTestService(t)
	frozen := time.Now()
	svc := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen }})
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Scan(context.Background(), init.ScanCode, 42); err != nil {
		t.Fatalf("scan: %v", err)
	}
	later := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{db: db}, Now: func() time.Time { return frozen.Add(scanlogin.DefaultTTL + time.Second) }})
	if err := later.Scan(context.Background(), init.ScanCode, 42); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for expired rescan, got %v", err)
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
	// 同一用户的重复确认（按钮连点/请求重试）幂等成功，且不得重复签发 session。
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); err != nil {
		t.Fatalf("want idempotent success on same-user reconfirm, got %v", err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_sessions WHERE user_id=9`).Scan(&n)
	if n != 1 {
		t.Fatalf("reconfirm must not mint another session, got %d rows", n)
	}
	// 其他用户的确认仍是真冲突。
	if err := svc.Confirm(context.Background(), init.ScanCode, 99); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for another confirmer, got %v", err)
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

// 无效码的 Confirm 不得 mint session：mint 只允许发生在「scanned+同人」这一
// 可推进窗口内（先 mint 后转移的崩溃安全设计仅为此保留）。否则任一登录账号
// 用随机 scan_code 刷 confirm，每次都能向无清理机制的 als_sessions 烧一行。
func TestConfirmInvalidCodeDoesNotMintSession(t *testing.T) {
	svc, db := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	// 不存在
	if err := svc.Confirm(context.Background(), "sc_bogus", 9); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// 仍 pending（没人扫过）
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for pending, got %v", err)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_sessions`).Scan(&n)
	if n != 0 {
		t.Fatalf("invalid confirms must not mint sessions, got %d rows", n)
	}
}

// Scan 幂等只认 scanned；已 authorized 的行是已消费的登录，重扫必须诚实 409，
// 否则成功页残留的旧二维码可被重放出一次「再次登录成功」。
func TestScanAuthorizedRowIsNotIdempotent(t *testing.T) {
	svc, db := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(9,'c@x.com','C','user')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.Scan(context.Background(), init.ScanCode, 9); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := svc.Scan(context.Background(), init.ScanCode, 9); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on rescan of authorized row, got %v", err)
	}
}
