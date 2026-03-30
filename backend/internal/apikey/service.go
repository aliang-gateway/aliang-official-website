package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	db *sql.DB
}

type CreateResult struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResult struct {
	APIKeyID int64
	UserID   int64
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func GenerateAPIKey() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}

	return "ak_" + hex.EncodeToString(random), nil
}

func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func (s *Service) CreateKey(ctx context.Context, userID int64, label string) (CreateResult, error) {
	if userID <= 0 {
		return CreateResult{}, errors.New("invalid user id")
	}

	label = strings.TrimSpace(label)
	if label == "" {
		label = "default"
	}

	plaintext, err := GenerateAPIKey()
	if err != nil {
		return CreateResult{}, err
	}

	hash := HashAPIKey(plaintext)
	createdAt := time.Now().UTC()

	const query = `INSERT INTO als_api_keys(user_id, key_hash, label, created_at) VALUES (?, ?, ?, ?);`
	result, err := s.db.ExecContext(ctx, query, userID, hash, label, createdAt)
	if err != nil {
		return CreateResult{}, fmt.Errorf("insert api key: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return CreateResult{}, fmt.Errorf("read api key insert id: %w", err)
	}

	return CreateResult{ID: id, Label: label, APIKey: plaintext, CreatedAt: createdAt}, nil
}

func (s *Service) RevokeKey(ctx context.Context, keyID int64, requesterID int64, isAdmin bool) (bool, error) {
	if keyID <= 0 || requesterID <= 0 {
		return false, errors.New("invalid revoke request")
	}

	const query = `
UPDATE als_api_keys
SET revoked_at = ?
WHERE id = ?
  AND revoked_at IS NULL
  AND (user_id = ? OR ? = 1);`

	adminFlag := 0
	if isAdmin {
		adminFlag = 1
	}

	result, err := s.db.ExecContext(ctx, query, time.Now().UTC(), keyID, requesterID, adminFlag)
	if err != nil {
		return false, fmt.Errorf("revoke api key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("revoke rows affected: %w", err)
	}

	return rows > 0, nil
}

func (s *Service) IsKeyActive(ctx context.Context, plaintext string) (bool, error) {
	if strings.TrimSpace(plaintext) == "" {
		return false, nil
	}

	hash := HashAPIKey(plaintext)
	const query = `SELECT key_hash FROM als_api_keys WHERE key_hash = ? AND revoked_at IS NULL LIMIT 1;`

	var storedHash string
	err := s.db.QueryRowContext(ctx, query, hash).Scan(&storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query api key: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hash)) != 1 {
		return false, nil
	}

	return true, nil
}

func (s *Service) AuthenticateKey(ctx context.Context, plaintext string) (AuthResult, bool, error) {
	if strings.TrimSpace(plaintext) == "" {
		return AuthResult{}, false, nil
	}

	hash := HashAPIKey(plaintext)
	const query = `SELECT id, user_id, key_hash FROM als_api_keys WHERE key_hash = ? AND revoked_at IS NULL LIMIT 1;`

	var (
		result     AuthResult
		storedHash string
	)
	err := s.db.QueryRowContext(ctx, query, hash).Scan(&result.APIKeyID, &result.UserID, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthResult{}, false, nil
	}
	if err != nil {
		return AuthResult{}, false, fmt.Errorf("query api key auth: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hash)) != 1 {
		return AuthResult{}, false, nil
	}

	return result, true, nil
}
