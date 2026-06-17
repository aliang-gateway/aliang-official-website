# 扫码登录（QR Scan-to-Login）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在密码登录之外新增「扫码登录」：PC 展示二维码，已登录 App 扫码确认后复用 `als_sessions` 给 PC 下发 Bearer token。

**Architecture:** 方案 A——密钥分离 + 两阶段确认。`device_code`（PC 密钥，永不进二维码）+ `scan_code`（进二维码）。状态机 `pending→scanned→authorized/denied`，全部 DB 原子转移，短轮询取状态。新增 `internal/scanlogin` 包封装状态机；HTTP handler 放独立文件，避免 routes.go 继续膨胀；session 签发抽 `user.Service.MintSessionForUser` 复用。

**Tech Stack:** Go 1.25（net/http ServeMux, database/sql, sqlite+postgres）、`internal/db.Rebind` 做方言占位符转换、modernc sqlite；前端 Next.js App Router + next-intl + `qrcode.react`。

**Spec:** `docs/superpowers/specs/2026-06-17-qr-scan-login-design.md`

**仓库根：** 所有相对路径以 `ai-api-portal/backend` 或 `frontend/` 为基准；根目录为 `/Users/mac/MyProgram/AiProgram/aliang-official-website`。

**跨方言要点（每个 SQL 都要遵守）：**
- 所有带参数的 SQL 用 `?` 占位符，外层包 `db.Rebind(s.dialect, query)`（postgres 自动转 `$1,$2…`）。见 `backend/internal/db/sql_dialect.go:16`。
- 迁移要同时写 `migrations/sqlite/` 和 `migrations/postgres/` 两份（sqlite 用 `INTEGER PRIMARY KEY AUTOINCREMENT`/`TIMESTAMP`，postgres 用 `BIGSERIAL`/`TIMESTAMPTZ`）。迁移按文件名排序自动发现，下一个编号是 **0023**。

**提交约定：** 每个 Task 末尾提交一次；commit message 用中文，与仓库历史风格一致（如 `新增扫码登录：xxx`）。不要提交不相关的预存改动（仓库里 `backend/internal/httpapi/routes.go` 可能有他人未提交改动，提交时只 `git add` 本任务相关文件）。

---

## File Structure

**Backend — 新建：**
- `backend/migrations/sqlite/0023_add_scan_login_codes.sql` — sqlite 建表
- `backend/migrations/postgres/0023_add_scan_login_codes.sql` — postgres 建表
- `backend/internal/scanlogin/service.go` — 状态机 + DB 操作（Init/Status/Scan/Confirm/Deny/Cleanup）
- `backend/internal/scanlogin/service_test.go` — 单测
- `backend/internal/httpapi/scan_login_handlers.go` — 5 个 HTTP handler（独立文件，不塞进 routes.go）

**Backend — 修改：**
- `backend/internal/user/service.go` — 新增 `MintSessionForUser`，重构 `Login` 复用之
- `backend/internal/httpapi/routes.go` — `routes` 结构体加 `scanLogin` 字段；`RegisterRoutesWithOptions` 里初始化 service + 注册 5 个路由

**Frontend — 新建：**
- `frontend/app/api/auth/scan/init/route.ts` — POST 透传
- `frontend/app/api/auth/scan/status/route.ts` — GET 透传
- `frontend/components/auth/ScanLoginPanel.tsx` — QR 渲染 + 轮询组件

**Frontend — 修改：**
- `frontend/package.json` — 加 `qrcode.react` 依赖
- `frontend/app/login/page.tsx` — 加「密码登录/扫码登录」Tab
- `frontend/messages/en.json`、`frontend/messages/zh.json` — 加 i18n key

---

## Task 1: 数据库迁移（sqlite + postgres）

**Files:**
- Create: `backend/migrations/sqlite/0023_add_scan_login_codes.sql`
- Create: `backend/migrations/postgres/0023_add_scan_login_codes.sql`
- Test: `backend/internal/db/migrate_test.go`（已有，验证迁移能被列举/执行）

- [ ] **Step 1: 写 sqlite 迁移**

Create `backend/migrations/sqlite/0023_add_scan_login_codes.sql`:
```sql
CREATE TABLE IF NOT EXISTS als_scan_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_code_hash TEXT NOT NULL UNIQUE,
    scan_code_hash   TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    user_id          INTEGER,
    session_token_hash TEXT,
    session_token    TEXT,
    init_ip          TEXT,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMP NOT NULL,
    scanned_at       TIMESTAMP,
    authorized_at    TIMESTAMP,
    denied_at        TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scan_codes_device_hash ON als_scan_codes(device_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_scan_hash   ON als_scan_codes(scan_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_expires_at  ON als_scan_codes(expires_at);
```

- [ ] **Step 2: 写 postgres 迁移**

Create `backend/migrations/postgres/0023_add_scan_login_codes.sql`:
```sql
CREATE TABLE IF NOT EXISTS als_scan_codes (
    id BIGSERIAL PRIMARY KEY,
    device_code_hash TEXT NOT NULL UNIQUE,
    scan_code_hash   TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    user_id          BIGINT,
    session_token_hash TEXT,
    session_token    TEXT,
    init_ip          TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMPTZ NOT NULL,
    scanned_at       TIMESTAMPTZ,
    authorized_at    TIMESTAMPTZ,
    denied_at        TIMESTAMPTZ,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scan_codes_device_hash ON als_scan_codes(device_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_scan_hash   ON als_scan_codes(scan_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_expires_at  ON als_scan_codes(expires_at);
```

- [ ] **Step 3: 验证迁移被发现且可执行**

Run: `cd backend && go test ./internal/db/ -run TestMigrationFilesExistForBothDialects -v`
Expected: PASS（`migrate_test.go` 断言 sqlite/postgres 迁移文件可被 `migrations.Filenames` 列举；新文件应被纳入，数量 +1）。

- [ ] **Step 4: 验证实际建表**

Run: `cd backend && go test ./internal/db/ -run TestApplyMigrationsCreatesRequiredTables -v`
Expected: PASS（对临时库跑全量迁移，确认 `als_scan_codes` 表与三个索引存在）。

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/sqlite/0023_add_scan_login_codes.sql backend/migrations/postgres/0023_add_scan_login_codes.sql
git commit -m "新增扫码登录 als_scan_codes 表迁移"
```

---

## Task 2: 抽取 user.Service.MintSessionForUser

把 `Login` 里「生成 session token + 写 als_sessions」的逻辑抽成可复用方法，供扫码确认调用。`Register` 用事务、暂不动（YAGNI）。

**Files:**
- Modify: `backend/internal/user/service.go`（`Login` 在 `:160-210`；session 生成在 `:194-206`）
- Test: `backend/internal/user/service_test.go`

- [ ] **Step 1: 写失败测试**

在 `service_test.go` 加（沿用该文件现有 helper `setupTestDB(t)` + `createUserWithPassword(...)`）：
```go
func TestMintSessionForUserCreatesValidSession(t *testing.T) {
	db := setupTestDB(t)
	svc := user.NewService(db)
	userID := createUserWithPassword(t, db, "scan@x.com", "Hunter2hunter!") // 对齐现有 helper 签名

	plaintext, tokenHash, err := svc.MintSessionForUser(context.Background(), userID)
	if err != nil { t.Fatalf("mint: %v", err) }
	if !strings.HasPrefix(plaintext, "st_") || plaintext == "" { t.Fatalf("bad plaintext %q", plaintext) }
	if tokenHash == "" || tokenHash == plaintext { t.Fatalf("bad tokenHash") }

	// 校验 als_sessions 里有一行，且 hash 可被 RequireUser 认出
	var got string
	if err := db.QueryRow(`SELECT token_hash FROM als_sessions WHERE user_id=?`, userID).Scan(&got); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if got != tokenHash { t.Fatalf("hash mismatch: %s != %s", got, tokenHash) }
}
```
（`setupTestDB`/`createUserWithPassword` 是 `service_test.go` 现有 helper；签名以源码为准，按需微调。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/user/ -run TestMintSessionForUser -v`
Expected: FAIL（`MintSessionForUser` undefined）。

- [ ] **Step 3: 加常量 + 方法**

在 `service.go` 顶部 const 区（如 `:12` 附近）加：
```go
const SessionLifetime = 24 * time.Hour
```
在 `Login` 之后（`:210` 后）加方法：
```go
// MintSessionForUser 为已有用户签发一个新 session，返回明文 token（下发用）与 token_hash（落库用）。
// 供扫码登录确认路径与（未来）其它无密码登录路径复用。
func (s *Service) MintSessionForUser(ctx context.Context, userID int64) (string, string, error) {
	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO als_sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, userID, tokenHash, time.Now().UTC().Add(SessionLifetime))
	if err != nil {
		return "", "", fmt.Errorf("insert session: %w", err)
	}
	return plaintext, tokenHash, nil
}
```

- [ ] **Step 4: 重构 Login 复用之**

把 `Login` 中 `:194-206`（`plaintext, tokenHash, err := auth.NewSessionToken()` ... `INSERT INTO als_sessions`）替换为：
```go
	plaintext, _, err := s.MintSessionForUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
```

- [ ] **Step 5: 跑 user 包全部测试，确认 Login 仍通过**

Run: `cd backend && go test ./internal/user/ -v`
Expected: PASS（含新测试 + 既有 Login/Register 测试）。

- [ ] **Step 6: Commit**

```bash
git add backend/internal/user/service.go backend/internal/user/service_test.go
git commit -m "抽取 MintSessionForUser 供扫码登录复用"
```

---

## Task 3: scanlogin.Service 骨架 + Init

**Files:**
- Create: `backend/internal/scanlogin/service.go`
- Create: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/scanlogin/service_test.go`。先放建表 helper（复用 sqlite 内存库 + 现有 `0001..0023` 迁移，或最小化只建 `als_scan_codes`/`als_users`/`als_sessions`）。最小化建表更稳：
```go
package scanlogin_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ai-api-portal/backend/internal/scanlogin"

	_ "modernc.org/sqlite"
)

func newTestService(t *testing.T) (*scanlogin.Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil { t.Fatalf("open: %v", err) }
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
		if _, err := db.Exec(q); err != nil { t.Fatalf("exec %q: %v", q, err) }
	}
	return scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{}}), db
}

type stubMinter struct{}
func (stubMinter) MintSessionForUser(ctx context.Context, userID int64) (string, string, error) {
	return "st_stub_" + fmt.Sprint(userID), "hash_" + fmt.Sprint(userID), nil
}

func TestInitCreatesRowAndReturnsCodes(t *testing.T) {
	svc, db := newTestService(t)
	res, err := svc.Init(context.Background(), "1.2.3.4")
	if err != nil { t.Fatalf("init: %v", err) }
	if !strings.HasPrefix(res.DeviceCode, "dc_") { t.Fatalf("device code: %q", res.DeviceCode) }
	if !strings.HasPrefix(res.ScanCode, "sc_") { t.Fatalf("scan code: %q", res.ScanCode) }
	if res.QRPayload != res.ScanCode { t.Fatalf("qr payload should equal scan code") }
	if res.ExpiresIn <= 0 || res.Interval <= 0 { t.Fatalf("bad expires/interval: %+v", res) }

	// 库里只存哈希，不存明文
	var devHash, scanHash, status, token string
	err = db.QueryRow(`SELECT device_code_hash, scan_code_hash, status, session_token FROM als_scan_codes`).Scan(&devHash, &scanHash, &status, &token)
	if err != nil { t.Fatalf("query: %v", err) }
	if status != "pending" { t.Fatalf("status=%s", status) }
	if token != "" { t.Fatalf("init must not store a session token") }
	if devHash == res.DeviceCode || scanHash == res.ScanCode { t.Fatalf("must store hashes, not plaintext") }
}
```
（补 `fmt`/`strings` import。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestInit -v`
Expected: FAIL（package 未创建/`NewService` undefined）。

- [ ] **Step 3: 写 service.go 骨架 + Init**

Create `backend/internal/scanlogin/service.go`:
```go
package scanlogin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"ai-api-portal/backend/internal/db"
)
```
> 注意：本 Task 暂不 import `strings`（`Init` 用不到，否则编译报 unused import）。`strings` 在 Task 4 的 `Status`/`Deny` 中用到时再加。同理 `database/sql`、`errors` 本 Task 也暂未使用——只保留实际用到的 import（`context`/`rand`/`sha256`/`hex`/`fmt`/`time`/`db`）。最终 import 以编译器为准。

const (
	DefaultTTL       = 5 * time.Minute
	DeviceCodeBytes  = 32
	ScanCodeBytes    = 24
	PollIntervalSec  = 2
	CleanupGrace     = 10 * time.Minute
	DeviceCodePrefix = "dc_"
	ScanCodePrefix   = "sc_"
)

// Status 枚举 als_scan_codes.status
type Status string

const (
	StatusPending    Status = "pending"
	StatusScanned    Status = "scanned"
	StatusAuthorized Status = "authorized"
	StatusDenied     Status = "denied"
	StatusExpired    Status = "expired" // 由 expires_at 计算，不入库
)

var (
	ErrNotFound     = errors.New("scan code not found")
	ErrInvalidState = errors.New("scan code is not in a valid state")
)

// SessionMinter 由 user.Service 实现，签发 session。
type SessionMinter interface {
	MintSessionForUser(ctx context.Context, userID int64) (plaintext, tokenHash string, err error)
}

type Options struct {
	Dialect string
	Minter  SessionMinter
	Now     func() time.Time // 测试注入时钟
}

type Service struct {
	db      *sql.DB
	dialect string
	minter  SessionMinter
	now     func() time.Time
}

func NewService(database *sql.DB, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: database, dialect: opts.Dialect, minter: opts.Minter, now: now}
}

type InitResult struct {
	DeviceCode string `json:"device_code"`
	ScanCode   string `json:"scan_code"`
	QRPayload  string `json:"qr_payload"`
	ExpiresIn  int    `json:"expires_in"`
	Interval   int    `json:"interval"`
}

func (s *Service) Init(ctx context.Context, initIP string) (*InitResult, error) {
	deviceCode, deviceHash, err := generateCode(DeviceCodePrefix, DeviceCodeBytes)
	if err != nil {
		return nil, fmt.Errorf("generate device code: %w", err)
	}
	scanCode, scanHash, err := generateCode(ScanCodePrefix, ScanCodeBytes)
	if err != nil {
		return nil, fmt.Errorf("generate scan code: %w", err)
	}
	now := s.now()
	expiresAt := now.Add(DefaultTTL)
	_, err = s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		INSERT INTO als_scan_codes(device_code_hash, scan_code_hash, status, init_ip, created_at, expires_at)
		VALUES (?, ?, 'pending', ?, ?, ?);
	`), deviceHash, scanHash, initIP, now.UTC(), expiresAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("insert scan code: %w", err)
	}
	return &InitResult{
		DeviceCode: deviceCode,
		ScanCode:   scanCode,
		QRPayload:  scanCode,
		ExpiresIn:  int(DefaultTTL / time.Second),
		Interval:   PollIntervalSec,
	}, nil
}

func generateCode(prefix string, nBytes int) (plaintext, hash string, err error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext = prefix + hex.EncodeToString(buf)
	return plaintext, hashCode(plaintext), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// 防止 "unused" 警告占位（Status 在后续 Task 使用；此处保留枚举）
var _ = strings.TrimSpace
```
（若 `strings` 暂未用到，删掉该 import 与占位行——后续 Task 会用到 `strings.TrimSpace`。）

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -run TestInit -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "新增 scanlogin 包与 Init（生成 device/scan 码）"
```

---

## Task 4: scanlogin.Service.Status（轮询）

**Files:**
- Modify: `backend/internal/scanlogin/service.go`
- Modify: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写失败测试（覆盖 pending/scanned/authorized/expired/notfound）**

```go
func TestStatusLifecycle(t *testing.T) {
	svc, db := newTestService(t)
	// 注入可控时钟，方便测过期
	frozen := time.Now()
	svc2 := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{}, Now: func() time.Time { return frozen }})

	init, err := svc2.Init(context.Background(), "")
	if err != nil { t.Fatalf("init: %v", err) }

	// pending
	got, err := svc2.Status(context.Background(), init.DeviceCode)
	if err != nil { t.Fatalf("status: %v", err) }
	if got.Status != scanlogin.StatusPending { t.Fatalf("want pending, got %s", got.Status) }
	if got.SessionToken != "" { t.Fatalf("pending must not leak token") }

	// not found
	if _, err := svc2.Status(context.Background(), "dc_bogus"); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// 直接造 scanned 行（模拟 App 已 scan）：拿 scan_code 明文→更新
	if _, err := db.Exec(`UPDATE als_scan_codes SET status='scanned', user_id=7 WHERE scan_code_hash=?`, hashOf(init.ScanCode)); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = svc2.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusScanned { t.Fatalf("want scanned, got %s", got.Status) }

	// authorized + 造 token
	if _, err := db.Exec(`UPDATE als_scan_codes SET status='authorized', session_token='st_xyz', user_id=7`); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(7,'u@x.com','U','user')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	got, _ = svc2.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusAuthorized { t.Fatalf("want authorized, got %s", got.Status) }
	if got.SessionToken != "st_xyz" { t.Fatalf("want token st_xyz, got %q", got.SessionToken) }
	if got.User == nil || got.User.ID != 7 || got.User.Email != "u@x.com" { t.Fatalf("bad user: %+v", got.User) }

	// expired：时钟推进到过期之后
	svc3 := scanlogin.NewService(db, scanlogin.Options{Minter: stubMinter{}, Now: func() time.Time { return frozen.Add(scanlogin.DefaultTTL + time.Second) }})
	got, _ = svc3.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusExpired { t.Fatalf("want expired, got %s", got.Status) }
}
```
（`hashOf` = 测试内对 scan_code 做 sha256 hex，与 service 内 `hashCode` 等价；可把 `hashCode` 导出为 `Hash` 供测试用，或在测试里复制实现。建议导出 `Hash`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestStatus -v`
Expected: FAIL（`Status` undefined）。

- [ ] **Step 3: 实现 Status + loadUser**

在 service.go 加：
```go
type StatusResult struct {
	Status       Status      `json:"status"`
	ExpiresIn    int         `json:"expires_in"`
	Interval     int         `json:"interval"`
	SessionToken string      `json:"session_token,omitempty"`
	User         *StatusUser `json:"user,omitempty"`
}

type StatusUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (s *Service) Status(ctx context.Context, deviceCode string) (*StatusResult, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil, ErrNotFound
	}
	now := s.now()
	var (
		status       Status
		expiresAt    time.Time
		sessionToken sql.NullString
		userID       sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, db.Rebind(s.dialect, `
		SELECT status, expires_at, session_token, user_id
		FROM als_scan_codes
		WHERE device_code_hash = ?;
	`), hashCode(deviceCode)).Scan(&status, &expiresAt, &sessionToken, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query scan code: %w", err)
	}
	if !expiresAt.After(now) {
		return &StatusResult{Status: StatusExpired, Interval: PollIntervalSec}, nil
	}
	res := &StatusResult{
		Status:    status,
		ExpiresIn: maxInt(int(time.Until(expiresAt)/time.Second), 0),
		Interval:  PollIntervalSec,
	}
	if status == StatusAuthorized && userID.Valid {
		if u, err := s.loadUser(ctx, userID.Int64); err == nil {
			res.User = u
		}
		res.SessionToken = sessionToken.String // 明文，短暂暂存
	}
	return res, nil
}

func (s *Service) loadUser(ctx context.Context, userID int64) (*StatusUser, error) {
	var u StatusUser
	err := s.db.QueryRowContext(ctx, db.Rebind(s.dialect, `
		SELECT id, email, name, role FROM als_users WHERE id = ?;
	`), userID).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func maxInt(a, b int) int { if a > b { return a }; return b }

// Hash 导出哈希函数供测试/审计使用。
func Hash(code string) string { return hashCode(code) }
```
（测试里 `hashOf(init.ScanCode)` 改用 `scanlogin.Hash(init.ScanCode)`。）

> 本 Task 起 `service.go` 需要 import `strings`（`Status` 用 `strings.TrimSpace`）、`database/sql`（`sql.NullString`/`sql.NullInt64`/`sql.ErrNoRows`）、`errors`（`errors.Is`）。把 Task 3 暂缺的这三个 import 补上。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -run TestStatus -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "scanlogin: 实现 Status 轮询与 user 关联"
```

---

## Task 5: scanlogin.Service.Scan（pending→scanned）

**Files:**
- Modify: `backend/internal/scanlogin/service.go`
- Modify: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestScanTransitionsAndGuards(t *testing.T) {
	svc, _ := newTestService(t)
	init, _ := svc.Init(context.Background(), "")

	// 正常 scan
	if err := svc.Scan(context.Background(), init.ScanCode, 42); err != nil { t.Fatalf("scan: %v", err) }
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusScanned { t.Fatalf("want scanned, got %s", got.Status) }

	// 重复 scan → ErrInvalidState
	if err := svc.Scan(context.Background(), init.ScanCode, 42); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on rescan, got %v", err)
	}
	// 不存在的码 → ErrNotFound
	if err := svc.Scan(context.Background(), "sc_bogus", 42); !errors.Is(err, scanlogin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestScan -v`
Expected: FAIL（`Scan` undefined）。

- [ ] **Step 3: 实现 Scan + scanCodeError**

```go
// Scan 由 App（已登录）调用：把 pending 行原子置为 scanned 并绑定 App 用户。
func (s *Service) Scan(ctx context.Context, scanCode string, userID int64) error {
	now := s.now().UTC()
	res, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		UPDATE als_scan_codes
		SET status = 'scanned', user_id = ?, scanned_at = ?
		WHERE scan_code_hash = ? AND status = 'pending' AND expires_at > ?;
	`), userID, now, hashCode(scanCode), now)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return s.scanCodeError(ctx, scanCode)
	}
	return nil
}

// scanCodeError 在转移失败时区分「不存在」与「状态不对」。
func (s *Service) scanCodeError(ctx context.Context, scanCode string) error {
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, db.Rebind(s.dialect, `
		SELECT expires_at FROM als_scan_codes WHERE scan_code_hash = ?;
	`), hashCode(scanCode)).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrInvalidState
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -run TestScan -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "scanlogin: 实现 Scan（pending→scanned）"
```

---

## Task 6: scanlogin.Service.Confirm（scanned→authorized + 签发）

**Files:**
- Modify: `backend/internal/scanlogin/service.go`
- Modify: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestConfirmBindsUserAndMintsToken(t *testing.T) {
	svc, db := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	if err := svc.Scan(context.Background(), init.ScanCode, 9); err != nil { t.Fatalf("scan: %v", err) }
	if _, err := db.Exec(`INSERT INTO als_users(id,email,name,role) VALUES(9,'c@x.com','C','user')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 串号：非扫码者确认 → ErrInvalidState
	if err := svc.Confirm(context.Background(), init.ScanCode, 99); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState for mismatched confirmer, got %v", err)
	}
	// 正确确认
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); err != nil { t.Fatalf("confirm: %v", err) }

	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusAuthorized { t.Fatalf("want authorized, got %s", got.Status) }
	if got.SessionToken == "" || got.User == nil || got.User.ID != 9 { t.Fatalf("bad authorized result: %+v", got) }

	// als_sessions 落了哈希行（非明文）
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_sessions WHERE user_id=9`).Scan(&n)
	if n != 1 { t.Fatalf("als_sessions should have 1 row, got %d", n) }

	// 重复确认 → ErrInvalidState
	if err := svc.Confirm(context.Background(), init.ScanCode, 9); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on reconfirm, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestConfirm -v`
Expected: FAIL（`Confirm` undefined）。

- [ ] **Step 3: 实现 Confirm**

```go
// Confirm 由 App 调用：原子 scanned→authorized（必须 confirmer==scanner），再为该用户签发 session。
// 明文 token 短暂暂存于 als_scan_codes.session_token 供 PC 幂等取用；als_sessions 仍只存哈希。
func (s *Service) Confirm(ctx context.Context, scanCode string, confirmerID int64) error {
	if s.minter == nil {
		return errors.New("session minter not configured")
	}
	now := s.now().UTC()
	// 1) 原子转移 scanned→authorized（同时校验 confirmer==scanner、未过期）
	res, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		UPDATE als_scan_codes
		SET status = 'authorized', authorized_at = ?
		WHERE scan_code_hash = ? AND status = 'scanned' AND user_id = ? AND expires_at > ?;
	`), now, hashCode(scanCode), confirmerID, now)
	if err != nil {
		return fmt.Errorf("confirm transition: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return s.scanCodeError(ctx, scanCode)
	}
	// 2) 签发 session（写 als_sessions 哈希行）
	plaintext, tokenHash, err := s.minter.MintSessionForUser(ctx, confirmerID)
	if err != nil {
		return fmt.Errorf("mint session: %w", err)
	}
	// 3) 暂存明文 + 哈希（同一行；authorized 窗口极短，PC 下一次轮询即取走）
	if _, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		UPDATE als_scan_codes
		SET session_token = ?, session_token_hash = ?
		WHERE scan_code_hash = ? AND status = 'authorized';
	`), plaintext, tokenHash, hashCode(scanCode)); err != nil {
		return fmt.Errorf("store session token: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -run TestConfirm -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "scanlogin: 实现 Confirm（scanned→authorized 并签发 session）"
```

---

## Task 7: scanlogin.Service.Deny

**Files:**
- Modify: `backend/internal/scanlogin/service.go`
- Modify: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestDenyFromScannedOrPending(t *testing.T) {
	svc, _ := newTestService(t)
	init, _ := svc.Init(context.Background(), "")
	// pending → denied
	if err := svc.Deny(context.Background(), init.ScanCode); err != nil { t.Fatalf("deny pending: %v", err) }
	got, _ := svc.Status(context.Background(), init.DeviceCode)
	if got.Status != scanlogin.StatusDenied { t.Fatalf("want denied, got %s", got.Status) }
	// 再 deny → ErrInvalidState
	if err := svc.Deny(context.Background(), init.ScanCode); !errors.Is(err, scanlogin.ErrInvalidState) {
		t.Fatalf("want ErrInvalidState on re-deny, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestDeny -v`
Expected: FAIL。

- [ ] **Step 3: 实现 Deny**

```go
// Deny 由 App 调用：把 pending/scanned 行置为 denied（取消登录）。
func (s *Service) Deny(ctx context.Context, scanCode string) error {
	now := s.now().UTC()
	res, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		UPDATE als_scan_codes
		SET status = 'denied', denied_at = ?
		WHERE scan_code_hash = ? AND status IN ('pending','scanned') AND expires_at > ?;
	`), now, hashCode(scanCode), now)
	if err != nil {
		return fmt.Errorf("deny: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return s.scanCodeError(ctx, scanCode)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -v`（跑全包确认无回归）
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "scanlogin: 实现 Deny（取消登录）"
```

---

## Task 8: 后台清理过期行

**Files:**
- Modify: `backend/internal/scanlogin/service.go`
- Modify: `backend/internal/scanlogin/service_test.go`

- [ ] **Step 1: 写测试**

```go
func TestCleanupExpiredDeletesOldRows(t *testing.T) {
	svc, db := newTestService(t)
	_, _ = svc.Init(context.Background(), "")
	// 把行改成很久以前过期
	if _, err := db.Exec(`UPDATE als_scan_codes SET expires_at = ?`, time.Now().Add(-time.Hour).UTC()); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := svc.CleanupExpired(context.Background()); err != nil { t.Fatalf("cleanup: %v", err) }
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM als_scan_codes`).Scan(&n)
	if n != 0 { t.Fatalf("want 0 rows after cleanup, got %d", n) }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/scanlogin/ -run TestCleanup -v`
Expected: FAIL（`CleanupExpired` undefined）。

- [ ] **Step 3: 实现 CleanupExpired + StartCleanup**

```go
// CleanupExpired 删除已过期且超过宽限期的行（含已取 token 的 authorized 行）。
func (s *Service) CleanupExpired(ctx context.Context) error {
	cutoff := s.now().Add(-CleanupGrace).UTC()
	_, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		DELETE FROM als_scan_codes WHERE expires_at < ?;
	`), cutoff)
	if err != nil {
		return fmt.Errorf("cleanup expired scan codes: %w", err)
	}
	return nil
}

// StartCleanup 启动每分钟清理一次的后台 goroutine，直到 ctx 取消。多实例各自幂等清扫。
func (s *Service) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.CleanupExpired(context.Background())
			}
		}
	}()
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/scanlogin/ -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scanlogin/
git commit -m "scanlogin: 后台清理过期扫码行"
```

---

## Task 9: HTTP handlers + 路由注册 + 集成测试

**Files:**
- Create: `backend/internal/httpapi/scan_login_handlers.go`
- Modify: `backend/internal/httpapi/routes.go`（`routes` struct `:38`；`RegisterRoutesWithOptions` `:733-761`）
- Test: `backend/internal/httpapi/scan_login_handlers_test.go`

- [ ] **Step 1: 写集成测试（先红）**

Create `backend/internal/httpapi/scan_login_handlers_test.go`，沿用 `internal/httpapi/*_test.go` 的 httptest 模式。现有 helper：`setupTestDB(t)`、`setupTestServer(t)`（返回 `*httptest.Server` + `*sql.DB`）、`createUserViaAPI(...)`、`setBearerAuth(req, token)`。全链路：
```go
func TestScanLoginFullFlow(t *testing.T) {
	db := setupTestDB(t)
	server, _ := setupTestServer(t, db) // 既有 helper，注册全部路由
	base := server.URL
	t.Cleanup(server.Close)
	// 用现有 helper 建一个 App 用户并拿到其 session token（App 凭证）。
	// 优先复用 createUserViaAPI + 登录拿 token；若名字不同，对齐现有 helper。
	appToken := createUserAndGetSessionToken(t, base, "app@x.com", "Hunter2hunter!")

	// init
	initResp := doJSON(t, base, "POST", "/auth/scan/init", nil, "")
	if initResp.status != 200 { t.Fatalf("init status %d", initResp.status) }
	deviceCode := initResp.body["device_code"].(string)
	scanCode := initResp.body["scan_code"].(string)

	// 初始 status = pending
	st := doJSON(t, base, "GET", "/auth/scan/status?device_code="+url.QueryEscape(deviceCode), nil, "")
	if st.body["status"] != "pending" { t.Fatalf("want pending") }

	// App 未带 token 调 scan → 401
	bad := doJSON(t, base, "POST", "/auth/scan/scan", map[string]any{"code": scanCode}, "")
	if bad.status != 401 { t.Fatalf("want 401, got %d", bad.status) }

	// App scan
	sc := doJSON(t, base, "POST", "/auth/scan/scan", map[string]any{"code": scanCode}, "Bearer "+appToken)
	if sc.status != 200 { t.Fatalf("scan: %d %v", sc.status, sc.body) }
	st = doJSON(t, base, "GET", "/auth/scan/status?device_code="+url.QueryEscape(deviceCode), nil, "")
	if st.body["status"] != "scanned" { t.Fatalf("want scanned") }

	// App confirm
	cf := doJSON(t, base, "POST", "/auth/scan/confirm", map[string]any{"code": scanCode}, "Bearer "+appToken)
	if cf.status != 200 { t.Fatalf("confirm: %d", cf.status) }

	// status 取到 token
	st = doJSON(t, base, "GET", "/auth/scan/status?device_code="+url.QueryEscape(deviceCode), nil, "")
	if st.body["status"] != "authorized" { t.Fatalf("want authorized") }
	pcToken := st.body["session_token"].(string)
	if !strings.HasPrefix(pcToken, "st_") { t.Fatalf("bad pc token") }

	// PC token 能通过 RequireUser（GET /auth/me）
	me := doJSON(t, base, "GET", "/auth/me", nil, "Bearer "+pcToken)
	if me.status != 200 { t.Fatalf("PC token not accepted: %d", me.status) }
}

// doJSON / createUserAndGetSessionToken 为本测试文件内的最小 helper（httpapi 包）。
// 复用现有 setBearerAuth(req, token) 设置 Bearer；GET 时 body 传 nil。
type httpResp struct {
	status int
	body   map[string]any
}

func doJSON(t *testing.T, base, method, path string, body any, bearer string) httpResp {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rd)
	if err != nil { t.Fatalf("new req: %v", err) }
	req.Header.Set("accept", "application/json")
	if body != nil { req.Header.Set("content-type", "application/json") }
	if bearer != "" { req.Header.Set("authorization", bearer) }
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatalf("do: %v", err) }
	defer resp.Body.Close()
	out := httpResp{status: resp.StatusCode}
	if resp.Header.Get("content-type") != "" {
		_ = json.NewDecoder(resp.Body).Decode(&out.body)
	}
	return out
}
```
（`setupTestDB`/`setupTestServer`/`setBearerAuth`/`createUserViaAPI` 是 `internal/httpapi/*_test.go` 现有 helper；`doJSON`/`createUserAndGetSessionToken` 在本测试文件内实现最小版。`io`/`bytes`/`net/http`/`encoding/json` import 按需加。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/httpapi/ -run TestScanLogin -v`
Expected: FAIL（路由/handler 未注册，404）。

- [ ] **Step 3: 写 handler 文件**

Create `backend/internal/httpapi/scan_login_handlers.go`（package httpapi，与 routes.go 同包）：
```go
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/scanlogin"
)
```

```go
type scanCodeRequest struct {
	Code string `json:"code"`
}

func (r *routes) handleScanInit(w http.ResponseWriter, req *http.Request) {
	res, err := r.scanLogin.Init(req.Context(), scanClientIP(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create scan code")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (r *routes) handleScanStatus(w http.ResponseWriter, req *http.Request) {
	res, err := r.scanLogin.Status(req.Context(), req.URL.Query().Get("device_code"))
	if err != nil {
		if errors.Is(err, scanlogin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "scan code not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query status")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (r *routes) handleScanScan(w http.ResponseWriter, req *http.Request) {
	u, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Scan(req.Context(), body.Code, u.ID); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "scanned"})
}

func (r *routes) handleScanConfirm(w http.ResponseWriter, req *http.Request) {
	u, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Confirm(req.Context(), body.Code, u.ID); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "authorized"})
}

func (r *routes) handleScanDeny(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.UserFromContext(req.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Deny(req.Context(), body.Code); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "denied"})
}

func decodeScanCodeBody(w http.ResponseWriter, req *http.Request) (*scanCodeRequest, bool) {
	var body scanCodeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	body.Code = strings.TrimSpace(body.Code)
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return nil, false
	}
	return &body, true
}

func writeScanStateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scanlogin.ErrNotFound):
		writeError(w, http.StatusNotFound, "scan code not found")
	case errors.Is(err, scanlogin.ErrInvalidState):
		writeError(w, http.StatusConflict, "scan code is not in a valid state")
	default:
		writeError(w, http.StatusInternalServerError, "scan operation failed")
	}
}

// scanClientIP 取发起端 IP（去端口），用于审计。
func scanClientIP(req *http.Request) string {
	host := req.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.TrimSpace(host)
}
```
（若 routes.go 已有 `clientIP`/`realIP` helper，改用它并删 `scanClientIP`。）

- [ ] **Step 4: routes.go 加字段 + 初始化 + 注册**

在 `routes` struct（`:38`）加字段：
```go
	scanLogin          *scanlogin.Service
```
在文件头 import 区加 `"ai-api-portal/backend/internal/scanlogin"`。

在 `RegisterRoutesWithOptions` 的 `r := &routes{...}` 字面量（`:738-753`）里加：
```go
		scanLogin:          scanlogin.NewService(database, scanlogin.Options{Dialect: strings.TrimSpace(opts.SQLDialect), Minter: userSvc}),
```
在 `authenticated := auth.RequireUserWithDialect(database, r.sqlDialect)`（`:754`）之后、`mux.HandleFunc("POST /users", ...)`（`:756`）之前加注册：
```go
	// 扫码登录（本地能力，非 upstream passthrough）
	mux.HandleFunc("POST /auth/scan/init", r.handleScanInit)
	mux.HandleFunc("GET /auth/scan/status", r.handleScanStatus)
	mux.Handle("POST /auth/scan/scan", authenticated(http.HandlerFunc(r.handleScanScan)))
	mux.Handle("POST /auth/scan/confirm", authenticated(http.HandlerFunc(r.handleScanConfirm)))
	mux.Handle("POST /auth/scan/deny", authenticated(http.HandlerFunc(r.handleScanDeny)))
```
在 `RegisterRoutesWithOptions` 末尾（return 前）启动清理：
```go
	r.scanLogin.StartCleanup(context.Background())
```
（文件头确保 `context` 已 import；routes.go 多半已 import `context`。）

- [ ] **Step 5: 编译 + 跑集成测试**

Run: `cd backend && go build ./... && go test ./internal/httpapi/ -run TestScanLogin -v`
Expected: PASS。

- [ ] **Step 6: 跑全量 backend 测试确认无回归**

Run: `cd backend && go test ./...`
Expected: PASS（注意：仓库既有 `TestAgentService...` 等与本功能无关的失败需与既有基线对比；只确认本任务不引入新失败）。

- [ ] **Step 7: Commit**

```bash
git add backend/internal/httpapi/scan_login_handlers.go backend/internal/httpapi/scan_login_handlers_test.go backend/internal/httpapi/routes.go
git commit -m "接入扫码登录 HTTP 路由与 handler"
```

---

## Task 10: 前端依赖 + 透传路由

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/app/api/auth/scan/init/route.ts`
- Create: `frontend/app/api/auth/scan/status/route.ts`

- [ ] **Step 1: 加 qrcode.react 依赖**

Run: `cd frontend && npm install qrcode.react`
（确认 `package.json` 出现 `"qrcode.react"`。）

- [ ] **Step 2: 写 init 透传路由**

Create `frontend/app/api/auth/scan/init/route.ts`（仿 `app/api/auth/login/route.ts`）：
```ts
import { NextResponse } from "next/server";
import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function POST(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }
  const upstream = await fetch(`${apiBaseUrl}/auth/scan/init`, {
    method: "POST",
    headers: { "content-type": "application/json", accept: "application/json" },
    cache: "no-store",
  });
  return new Response(upstream.body, { status: upstream.status, headers: upstream.headers });
}
```

- [ ] **Step 3: 写 status 透传路由（GET，带 query）**

Create `frontend/app/api/auth/scan/status/route.ts`：
```ts
import { NextResponse } from "next/server";
import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function GET(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }
  const qs = request.url.includes("?") ? request.url.slice(request.url.indexOf("?")) : "";
  const upstream = await fetch(`${apiBaseUrl}/auth/scan/status${qs}`, {
    method: "GET",
    headers: { accept: "application/json" },
    cache: "no-store",
  });
  return new Response(upstream.body, { status: upstream.status, headers: upstream.headers });
}
```

- [ ] **Step 4: 类型检查**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/app/api/auth/scan/
git commit -m "前端新增扫码登录透传路由与 qrcode.react 依赖"
```

---

## Task 11: 前端 ScanLoginPanel + 登录 Tab + i18n

**Files:**
- Create: `frontend/components/auth/ScanLoginPanel.tsx`
- Modify: `frontend/app/login/page.tsx`
- Modify: `frontend/messages/en.json`、`frontend/messages/zh.json`

- [ ] **Step 1: 加 i18n key**

在 `messages/en.json` 的 `"login": { ... }` 块内加：
```json
    "passwordTab": "Password",
    "scanTab": "Scan to Login",
    "scanWaiting": "Scan with the ALiang app to log in",
    "scanScanned": "Scanned — tap confirm on your phone",
    "scanSuccess": "Logged in",
    "scanDenied": "Login cancelled",
    "scanExpired": "QR code expired, refreshing..."
```
在 `messages/zh.json` 的 `"login": { ... }` 块内加对应中文：
```json
    "passwordTab": "密码登录",
    "scanTab": "扫码登录",
    "scanWaiting": "请使用 ALiang App 扫码登录",
    "scanScanned": "已扫描，请在手机上确认",
    "scanSuccess": "登录成功",
    "scanDenied": "已取消登录",
    "scanExpired": "二维码已过期，正在刷新..."
```

- [ ] **Step 2: 写 ScanLoginPanel 组件**

Create `frontend/components/auth/ScanLoginPanel.tsx`：
```tsx
"use client";

import { QRCodeSVG } from "qrcode.react";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";
import { useTranslations } from "next-intl";

type InitResp = { device_code?: string; scan_code?: string; qr_payload?: string; interval?: number };
type StatusResp = {
  status?: string;
  session_token?: string;
  user?: { role?: "user" | "admin" | "distributor" };
};

export function ScanLoginPanel({ nextPath }: { nextPath: string }) {
  const router = useRouter();
  const t = useTranslations("login");
  const [qrPayload, setQrPayload] = useState<string | null>(null);
  const [deviceCode, setDeviceCode] = useState<string>("");
  const [interval, setIntervalSec] = useState<number>(2);
  const [message, setMessage] = useState<string>(t("scanWaiting"));
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cancelledRef = useRef(false);

  async function init() {
    setMessage(t("scanWaiting"));
    setQrPayload(null);
    const res = await fetch("/api/auth/scan/init", { method: "POST", headers: { accept: "application/json" } });
    const payload = (await res.json()) as unknown;
    if (!res.ok) { setMessage(extractApiError(payload, "init failed")); scheduleReinit(2000); return; }
    const data = unwrapData<InitResp>(payload) ?? asRecord(payload);
    const dc = asString(data?.device_code);
    const qp = asString(data?.qr_payload) || asString(data?.scan_code);
    if (!dc || !qp) { setMessage("init failed"); scheduleReinit(2000); return; }
    setDeviceCode(dc);
    setQrPayload(qp);
    if (data?.interval && data.interval > 0) setIntervalSec(data.interval);
  }

  async function poll(dc: string) {
    const res = await fetch(`/api/auth/scan/status?device_code=${encodeURIComponent(dc)}`, { headers: { accept: "application/json" } });
    if (!res.ok) { if (!cancelledRef.current) schedulePoll(dc); return; }
    const payload = (await res.json()) as unknown;
    const data = unwrapData<StatusResp>(payload) ?? asRecord(payload);
    const status = asString(asRecord(payload)?.status) || asString((data as Record<string, unknown> | null)?.status as string);
    const token = asString(asRecord(payload)?.session_token) || asString((data as Record<string, unknown> | null)?.session_token as string);
    const role = (asRecord(asRecord(payload)?.user)?.role) ?? (asRecord((data as Record<string, unknown> | null)?.user)?.role);

    if (status === "authorized" && token) {
      localStorage.setItem("session_token", token);
      setMessage(t("scanSuccess"));
      const safeNext = nextPath.startsWith("/") && !nextPath.startsWith("//") ? nextPath : "";
      router.replace(safeNext || (role === "distributor" ? "/distributor" : role === "admin" ? "/admin/users" : "/dashboard"));
      return;
    }
    if (status === "scanned") setMessage(t("scanScanned"));
    else if (status === "denied") { setMessage(t("scanDenied")); scheduleReinit(2000); return; }
    else if (status === "expired") { setMessage(t("scanExpired")); scheduleReinit(500); return; }
    else setMessage(t("scanWaiting"));
    if (!cancelledRef.current) schedulePoll(dc);
  }

  function schedulePoll(dc: string) {
    timerRef.current = setTimeout(() => { void poll(dc); }, interval * 1000);
  }
  function scheduleReinit(ms: number) {
    timerRef.current = setTimeout(() => { void init(); }, ms);
  }

  useEffect(() => {
    cancelledRef.current = false;
    void init();
    return () => {
      cancelledRef.current = true;
      if (timerRef.current) clearTimeout(timerRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="flex flex-col items-center gap-4">
      <div className="rounded-lg border border-[var(--stitch-border)] bg-white p-3">
        {qrPayload ? (
          <QRCodeSVG value={qrPayload} size={200} />
        ) : (
          <div className="flex h-[200px] w-[200px] items-center justify-center text-sm text-[var(--stitch-text-muted)]">…</div>
        )}
      </div>
      <p className="text-sm text-[var(--stitch-text-muted)]">{message}</p>
    </div>
  );
}
```
> 注：`status`/`session_token` 的解析沿用了 `app/login/page.tsx` 既有「legacy + nested + data」三重兜底写法。如 `lib/api-response` 提供更顺手的方法可简化；保持与登录页一致即可。

- [ ] **Step 3: 登录页加 Tab**

Modify `frontend/app/login/page.tsx`：在 `LoginContent` 内加 tab 状态并在表单上方渲染切换；扫码 tab 渲染 `<ScanLoginPanel nextPath={nextPath} />`。骨架（插入到现有表单 JSX 之前/替换为条件渲染）：
```tsx
import { ScanLoginPanel } from "@/components/auth/ScanLoginPanel";
// ...
const [mode, setMode] = useState<"password" | "scan">("password");
// ...在 return 的卡片里，标题下方加：
<div className="mb-6 flex gap-2 rounded-lg bg-[var(--stitch-bg)] p-1">
  <button type="button" onClick={() => setMode("password")}
    className={`flex-1 rounded-md px-3 py-2 text-sm font-medium ${mode === "password" ? "bg-[var(--stitch-primary)] text-white" : "text-[var(--stitch-text-muted)]"}`}>
    {t("passwordTab")}
  </button>
  <button type="button" onClick={() => setMode("scan")}
    className={`flex-1 rounded-md px-3 py-2 text-sm font-medium ${mode === "scan" ? "bg-[var(--stitch-primary)] text-white" : "text-[var(--stitch-text-muted)]"}`}>
    {t("scanTab")}
  </button>
</div>
{mode === "scan" ? (
  <ScanLoginPanel nextPath={nextPath} />
) : (
  /* 原有 <form>...</form> 原样保留 */
)}
```
（保留原有 `<form>`、错误提示与底部「创建账号」链接。）

- [ ] **Step 4: 类型检查 + 构建**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 构建通过（lint/build 无错）。

- [ ] **Step 5: 手测**

启动 backend + frontend，打开 `/login` → 切到「扫码登录」→ 确认 QR 渲染、轮询 `pending`。用 App session 调 `POST /auth/scan/scan` → `confirm`，确认 PC 端自动拿到 token 跳转 dashboard。

- [ ] **Step 6: Commit**

```bash
git add frontend/components/auth/ScanLoginPanel.tsx frontend/app/login/page.tsx frontend/messages/en.json frontend/messages/zh.json
git commit -m "前端新增扫码登录面板与登录页 Tab"
```

---

## 验证总览（全部完成后）

- 后端：`cd backend && go vet ./... && go test ./...`，sqlite + 交叉编译 postgres 兼容（`GOOS` 不涉及；方言靠 `db.Rebind`）。
- 前端：`cd frontend && npx tsc --noEmit && npm run build`。
- 端到端：PC init→QR→App scan→confirm→PC 取 token→跳转；过期/取消/串号/重复各路径。
- 安全复核：`device_code` 不进日志/不进 QR；`als_sessions` 只存哈希；明文 token 仅 `als_scan_codes.session_token` 短暂暂存（≤TTL+宽限期）。

## 不在本次范围

- App 端扫码/确认 UI（接口已就绪）。
- 手机浏览器无 App 兜底确认页。
- `/auth/scan/init` 的 per-IP 限流（防刷表）——当前靠清理 goroutine 兜底；如需可在 handler/中间件层加，列为后续硬化。
