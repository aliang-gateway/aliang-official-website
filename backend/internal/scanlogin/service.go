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
