package scanlogin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-api-portal/backend/internal/db"
)

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
	return &Service{
		db:      database,
		dialect: opts.Dialect,
		minter:  opts.Minter,
		now:     now,
	}
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

type StatusResult struct {
	Status       Status      `json:"status"`
	ExpiresIn    int         `json:"expires_in"`
	Interval     int         `json:"interval"`
	SessionToken string      `json:"session_token,omitempty"`
	RefreshToken string      `json:"refresh_token,omitempty"`
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
		res.SessionToken = sessionToken.String
		// The public refresh credential is the device-specific local session.
		// Upstream credentials remain private to the gateway token broker.
		res.RefreshToken = sessionToken.String
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Hash 导出哈希函数供测试/审计使用。
func Hash(code string) string { return hashCode(code) }

// Scan 由 App（已登录）调用：把 pending 行原子置为 scanned 并绑定 App 用户。
// 幂等：同一用户对已绑定自己的行重复调用（扫码帧连发/网络重试）直接成功。
// 2026-09-21 生产事故：iOS 扫码帧双发，第二发 409 后到覆盖成功 UI，而 scanned
// 行永远无法重扫，用户被卡死到网页 TTL 换码。过期行不参与幂等（TTL 是配对窗口硬边界）。
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
		idempotent, err := s.repeatOutcomeForUser(ctx, scanCode, userID, false)
		if err != nil {
			return err
		}
		if idempotent {
			return nil
		}
		return ErrInvalidState
	}
	return nil
}

// lookupScanCodeRow 按 scan_code 哈希取行的 (status, user_id, expires_at)。
func (s *Service) lookupScanCodeRow(ctx context.Context, scanCode string) (Status, sql.NullInt64, time.Time, error) {
	var (
		status    Status
		rowUser   sql.NullInt64
		expiresAt time.Time
	)
	err := s.db.QueryRowContext(ctx, db.Rebind(s.dialect, `
		SELECT status, user_id, expires_at FROM als_scan_codes WHERE scan_code_hash = ?;
	`), hashCode(scanCode)).Scan(&status, &rowUser, &expiresAt)
	return status, rowUser, expiresAt, err
}

// repeatOutcomeForUser 判定「同一用户对已推进状态的行重复调用」是否幂等成功。
// 返回 (true, nil) = 幂等成功；(false, nil) = 真状态冲突；(非nil err) = NotFound/DB 错误。
// expectAuthorized（Confirm 路径）只认 authorized；Scan 路径只认 scanned——
// authorized 行是已消费的登录，对 Scan 幂等会允许旧二维码重放出一次「再次登录成功」。
// denied 行与过期行一律不算幂等。
func (s *Service) repeatOutcomeForUser(ctx context.Context, scanCode string, userID int64, expectAuthorized bool) (bool, error) {
	status, rowUser, expiresAt, err := s.lookupScanCodeRow(ctx, scanCode)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if !rowUser.Valid || rowUser.Int64 != userID || !expiresAt.After(s.now()) {
		return false, ErrInvalidState
	}
	switch status {
	case StatusAuthorized:
		// Scan 路径不认可已消费的 authorized 行（防旧码重放），只有 Confirm 自己认。
		return expectAuthorized, nil
	case StatusScanned:
		return !expectAuthorized, nil
	default:
		return false, ErrInvalidState
	}
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

// Confirm 由 App 调用：先签发 session，再用单条原子 UPDATE 完成 scanned→authorized 并写入明文 token。
// mint 前硬闸：状态预检不通（不存在/pending/denied/过期/他人行）直接返回错误，
// 不 mint——否则每个无效 confirm 都会向无清理机制的 als_sessions 烧一条孤儿行。
// 唯一允许 mint 的窗口是「scanned+同人+未过期」。该窗口内先 mint 后转移：即使进程在
// 两步之间崩溃，scan_codes 仍停留在 scanned（PC 继续轮询、可重试确认），不会卡在
// authorized 却无 token 的状态。并发双确认的输家经下方幂等回查返回成功；
// 多 mint 的那行 als_sessions 无人持有明文，无害。confirmer 必须等于扫码者。
func (s *Service) Confirm(ctx context.Context, scanCode string, confirmerID int64) error {
	if s.minter == nil {
		return errors.New("session minter not configured")
	}
	status, rowUser, expiresAt, err := s.lookupScanCodeRow(ctx, scanCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	sameUser := rowUser.Valid && rowUser.Int64 == confirmerID && expiresAt.After(s.now())
	if status == StatusAuthorized && sameUser {
		return nil // 幂等：已由本确认者确认过（连点/重试）
	}
	if status != StatusScanned || !sameUser {
		return ErrInvalidState
	}
	plaintext, tokenHash, err := s.minter.MintSessionForUser(ctx, confirmerID)
	if err != nil {
		return fmt.Errorf("mint session: %w", err)
	}
	now := s.now().UTC()
	res, err := s.db.ExecContext(ctx, db.Rebind(s.dialect, `
		UPDATE als_scan_codes
		SET status = 'authorized', authorized_at = ?, session_token = ?, session_token_hash = ?
		WHERE scan_code_hash = ? AND status = 'scanned' AND user_id = ? AND expires_at > ?;
	`), now, plaintext, tokenHash, hashCode(scanCode), confirmerID, now)
	if err != nil {
		return fmt.Errorf("confirm transition: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		idempotent, err := s.repeatOutcomeForUser(ctx, scanCode, confirmerID, true)
		if err != nil {
			return err
		}
		if idempotent {
			return nil
		}
		return ErrInvalidState
	}
	return nil
}

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
