package sub2apiauth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-api-portal/backend/internal/db"
)

var (
	ErrTokenNotFound     = errors.New("sub2api token not found")
	ErrRotationConflict  = errors.New("sub2api token rotation conflict")
	ErrRotationUncertain = errors.New("sub2api token rotation outcome is uncertain")
)

type Service struct {
	db          *sql.DB
	sqlDialect  string
	tokenCipher *tokenCipher
	initErr     error
}

type UpsertTokenInput struct {
	UserID         int64
	UpstreamUserID *int64
	AccessToken    string
	RefreshToken   *string
	// AccessExpiresAt is the upstream access_token's expiry. When nil the stored
	// value is left untouched (NULL on insert); the refresh arbiter treats a
	// NULL/past expiry as "needs rotation".
	AccessExpiresAt *time.Time
}

func NewService(database *sql.DB) *Service {
	return NewServiceWithDialect(database, "sqlite")
}

func NewServiceWithDialect(database *sql.DB, sqlDialect string) *Service {
	return NewServiceWithDialectAndKey(database, sqlDialect, "")
}

func NewServiceWithDialectAndKey(database *sql.DB, sqlDialect, encryptionKey string) *Service {
	cipher, err := newTokenCipher(encryptionKey)
	return &Service{db: database, sqlDialect: sqlDialect, tokenCipher: cipher, initErr: err}
}

func (s *Service) protectCredential(value string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	return s.tokenCipher.seal(value)
}

func (s *Service) revealCredential(value string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	return s.tokenCipher.open(value)
}

func (s *Service) ProtectCredential(value string) (string, error) {
	return s.protectCredential(strings.TrimSpace(value))
}

// ReencryptLegacyCredentials migrates plaintext rows before the HTTP server
// starts and validates existing ciphertext with the configured key.
func (s *Service) ReencryptLegacyCredentials(ctx context.Context) error {
	if s.initErr != nil {
		return s.initErr
	}
	if s.tokenCipher == nil {
		return errors.New("sub2api token encryption key is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, access_token, refresh_token, prev_refresh_token
		FROM als_sub2api_auth_tokens;
	`)
	if err != nil {
		return fmt.Errorf("query sub2api credentials for re-encryption: %w", err)
	}
	type row struct {
		userID  int64
		access  string
		refresh sql.NullString
		prev    sql.NullString
	}
	var credentials []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.userID, &item.access, &item.refresh, &item.prev); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan sub2api credential for re-encryption: %w", err)
		}
		credentials = append(credentials, item)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close sub2api credential rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sub2api credentials for re-encryption: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sub2api credential re-encryption: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range credentials {
		needsRewrite := !strings.HasPrefix(item.access, encryptedTokenPrefix) ||
			(item.refresh.Valid && !strings.HasPrefix(item.refresh.String, encryptedTokenPrefix)) ||
			(item.prev.Valid && !strings.HasPrefix(item.prev.String, encryptedTokenPrefix))
		plainAccess, err := s.revealCredential(item.access)
		if err != nil {
			return fmt.Errorf("decrypt access token for user %d: %w", item.userID, err)
		}
		protectedAccess, err := s.protectCredential(plainAccess)
		if err != nil {
			return err
		}
		var protectedRefresh, protectedPrev any
		if item.refresh.Valid {
			plain, err := s.revealCredential(item.refresh.String)
			if err != nil {
				return fmt.Errorf("decrypt refresh token for user %d: %w", item.userID, err)
			}
			protectedRefresh, err = s.protectCredential(plain)
			if err != nil {
				return err
			}
		}
		if item.prev.Valid {
			plain, err := s.revealCredential(item.prev.String)
			if err != nil {
				return fmt.Errorf("decrypt previous refresh token for user %d: %w", item.userID, err)
			}
			protectedPrev, err = s.protectCredential(plain)
			if err != nil {
				return err
			}
		}
		if !needsRewrite {
			continue
		}
		if _, err := tx.ExecContext(ctx, db.Rebind(s.sqlDialect, `
			UPDATE als_sub2api_auth_tokens
			SET access_token = ?, refresh_token = ?, prev_refresh_token = ?
			WHERE user_id = ?;
		`), protectedAccess, protectedRefresh, protectedPrev, item.userID); err != nil {
			return fmt.Errorf("rewrite encrypted credentials for user %d: %w", item.userID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sub2api credential re-encryption: %w", err)
	}
	return nil
}

func (s *Service) UpsertToken(ctx context.Context, input UpsertTokenInput) error {
	if input.UserID <= 0 {
		return errors.New("user id must be positive")
	}
	accessToken := strings.TrimSpace(input.AccessToken)
	if accessToken == "" {
		return errors.New("access token is required")
	}
	accessToken, err := s.protectCredential(accessToken)
	if err != nil {
		return err
	}

	var refreshToken any
	if input.RefreshToken != nil {
		plainRefresh := strings.TrimSpace(*input.RefreshToken)
		refreshToken = plainRefresh
		if refreshToken == "" {
			refreshToken = nil
		} else {
			protected, err := s.protectCredential(plainRefresh)
			if err != nil {
				return err
			}
			refreshToken = protected
		}
	}

	var accessExpires any
	if input.AccessExpiresAt != nil {
		accessExpires = input.AccessExpiresAt.UTC()
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		INSERT INTO als_sub2api_auth_tokens(
			user_id,
			upstream_user_id,
			access_token,
			refresh_token,
			access_expires_at,
			prev_refresh_token,
			rotation_state,
			rotation_started_at,
			version,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, NULL, 'stable', NULL, 0, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			upstream_user_id = COALESCE(excluded.upstream_user_id, als_sub2api_auth_tokens.upstream_user_id),
			access_token = excluded.access_token,
			prev_refresh_token = CASE
				WHEN excluded.refresh_token IS NOT NULL
					AND als_sub2api_auth_tokens.refresh_token IS NOT NULL
					AND excluded.refresh_token <> als_sub2api_auth_tokens.refresh_token
				THEN als_sub2api_auth_tokens.refresh_token
				ELSE als_sub2api_auth_tokens.prev_refresh_token
			END,
			refresh_token = COALESCE(excluded.refresh_token, als_sub2api_auth_tokens.refresh_token),
			access_expires_at = COALESCE(excluded.access_expires_at, als_sub2api_auth_tokens.access_expires_at),
			rotation_state = 'stable',
			rotation_started_at = NULL,
			version = als_sub2api_auth_tokens.version + 1,
			updated_at = excluded.updated_at;
	`), input.UserID, input.UpstreamUserID, accessToken, refreshToken, accessExpires, now, now)
	if err != nil {
		return fmt.Errorf("upsert sub2api auth token: %w", err)
	}

	return nil
}

func (s *Service) GetBearerTokenByUserID(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", errors.New("user id must be positive")
	}

	var bearer string
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT access_token
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?
		LIMIT 1;
	`), userID).Scan(&bearer)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query sub2api bearer token: %w", err)
	}

	if strings.TrimSpace(bearer) == "" {
		return "", ErrTokenNotFound
	}

	plain, err := s.revealCredential(bearer)
	if err != nil {
		return "", fmt.Errorf("decrypt sub2api bearer token: %w", err)
	}
	return plain, nil
}

// GetRefreshTokenByUserID 返回该用户的 sub2api refresh_token。列可空，故用 sql.NullString；
// 无行或空值统一归为 ErrTokenNotFound，便于调用方按「该用户无 upstream 令牌」降级处理。
func (s *Service) GetRefreshTokenByUserID(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", errors.New("user id must be positive")
	}

	var refresh sql.NullString
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT refresh_token
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?
		LIMIT 1;
	`), userID).Scan(&refresh)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query sub2api refresh token: %w", err)
	}

	if !refresh.Valid || strings.TrimSpace(refresh.String) == "" {
		return "", ErrTokenNotFound
	}

	plain, err := s.revealCredential(refresh.String)
	if err != nil {
		return "", fmt.Errorf("decrypt sub2api refresh token: %w", err)
	}
	return plain, nil
}

func (s *Service) GetUpstreamUserIDByUserID(ctx context.Context, userID int64) (int64, bool, error) {
	if userID <= 0 {
		return 0, false, errors.New("user id must be positive")
	}

	var upstreamUserID sql.NullInt64
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT upstream_user_id
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?
		LIMIT 1;
	`), userID).Scan(&upstreamUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query sub2api upstream user id: %w", err)
	}
	if !upstreamUserID.Valid || upstreamUserID.Int64 <= 0 {
		return 0, false, nil
	}

	return upstreamUserID.Int64, true, nil
}

// VaultTokens is the refresh arbiter's view of a user's stored upstream tokens.
// RefreshToken is the current server-owned upstream credential;
// PrevRefreshToken is retained only for audit/recovery diagnostics.
type VaultTokens struct {
	UserID             int64
	AccessToken        string
	RefreshToken       string // current
	HasRefresh         bool
	PrevRefreshToken   string // grace-window predecessor
	HasPrevRefresh     bool
	AccessExpiresAt    time.Time
	HasAccessExpires   bool
	RotationState      string
	RotationStartedAt  time.Time
	HasRotationStarted bool
	Version            int64
}

// LoadVault returns the current stored token pair for the user, or nil when no
// row exists. Read under the arbiter's per-user lock to make freshness decisions.
func (s *Service) LoadVault(ctx context.Context, userID int64) (*VaultTokens, error) {
	if userID <= 0 {
		return nil, errors.New("user id must be positive")
	}

	var (
		access          string
		refresh         sql.NullString
		prevRefresh     sql.NullString
		expires         sql.NullTime
		rotationStarted sql.NullTime
		rotationState   string
		version         int64
	)
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT access_token, refresh_token, prev_refresh_token, access_expires_at,
		       rotation_state, rotation_started_at, version
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`), userID).Scan(&access, &refresh, &prevRefresh, &expires, &rotationState, &rotationStarted, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load sub2api token vault: %w", err)
	}
	plainAccess, err := s.revealCredential(access)
	if err != nil {
		return nil, fmt.Errorf("decrypt vault access token: %w", err)
	}
	plainRefresh := refresh.String
	if refresh.Valid {
		plainRefresh, err = s.revealCredential(refresh.String)
		if err != nil {
			return nil, fmt.Errorf("decrypt vault refresh token: %w", err)
		}
	}
	plainPrev := prevRefresh.String
	if prevRefresh.Valid {
		plainPrev, err = s.revealCredential(prevRefresh.String)
		if err != nil {
			return nil, fmt.Errorf("decrypt vault previous refresh token: %w", err)
		}
	}

	return &VaultTokens{
		UserID:             userID,
		AccessToken:        plainAccess,
		HasRefresh:         refresh.Valid && strings.TrimSpace(refresh.String) != "",
		RefreshToken:       plainRefresh,
		HasPrevRefresh:     prevRefresh.Valid && strings.TrimSpace(prevRefresh.String) != "",
		PrevRefreshToken:   plainPrev,
		HasAccessExpires:   expires.Valid,
		AccessExpiresAt:    expires.Time,
		RotationState:      rotationState,
		HasRotationStarted: rotationStarted.Valid,
		RotationStartedAt:  rotationStarted.Time,
		Version:            version,
	}, nil
}

func (s *Service) refreshCredentialMatches(ctx context.Context, userID int64, expectedRefresh, requiredState string, expectedVersion int64) (bool, error) {
	var stored sql.NullString
	var state string
	var version int64
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT refresh_token, rotation_state, version
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`), userID).Scan(&stored, &state, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load refresh credential for rotation: %w", err)
	}
	if state != requiredState || version != expectedVersion {
		return false, nil
	}
	if !stored.Valid || strings.TrimSpace(stored.String) == "" {
		return false, nil
	}
	plain, err := s.revealCredential(stored.String)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(plain), []byte(strings.TrimSpace(expectedRefresh))) == 1, nil
}

// BeginRotation durably records that the current upstream refresh token is in
// flight. If the process dies after sub2api consumes the token, a later process
// will see the non-stable state and must not replay the old credential.
func (s *Service) BeginRotation(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	expectedRefresh = strings.TrimSpace(expectedRefresh)
	if userID <= 0 || expectedRefresh == "" {
		return ErrRotationConflict
	}
	matches, err := s.refreshCredentialMatches(ctx, userID, expectedRefresh, "stable", expectedVersion)
	if err != nil {
		return err
	}
	if !matches {
		return ErrRotationConflict
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		UPDATE als_sub2api_auth_tokens
		SET rotation_state = 'rotating', rotation_started_at = ?, updated_at = ?
		WHERE user_id = ? AND rotation_state = 'stable' AND version = ?;
	`), now, now, userID, expectedVersion)
	if err != nil {
		return fmt.Errorf("begin sub2api token rotation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("begin sub2api token rotation rows affected: %w", err)
	}
	if rows != 1 {
		return ErrRotationConflict
	}
	return nil
}

// ResetRotation marks a definitively non-consuming upstream response (for
// example 429/5xx) as retryable while preserving the current token family.
func (s *Service) ResetRotation(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	matches, err := s.refreshCredentialMatches(ctx, userID, expectedRefresh, "rotating", expectedVersion)
	if err != nil {
		return err
	}
	if !matches {
		return ErrRotationConflict
	}
	result, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		UPDATE als_sub2api_auth_tokens
		SET rotation_state = 'stable', rotation_started_at = NULL, updated_at = ?
		WHERE user_id = ? AND rotation_state = 'rotating' AND version = ?;
	`), time.Now().UTC(), userID, expectedVersion)
	if err != nil {
		return fmt.Errorf("reset sub2api token rotation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reset sub2api token rotation rows affected: %w", err)
	}
	if rows != 1 {
		return ErrRotationConflict
	}
	return nil
}

// MarkRotationUncertain prevents replay after an ambiguous transport or
// persistence failure. Recovery requires a fresh login, which is safer than
// forwarding a refresh token that sub2api may already have consumed.
func (s *Service) MarkRotationUncertain(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	matches, err := s.refreshCredentialMatches(ctx, userID, expectedRefresh, "rotating", expectedVersion)
	if err != nil {
		return err
	}
	if !matches {
		return ErrRotationConflict
	}
	_, err = s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		UPDATE als_sub2api_auth_tokens
		SET rotation_state = 'uncertain', updated_at = ?
		WHERE user_id = ? AND rotation_state = 'rotating' AND version = ?;
	`), time.Now().UTC(), userID, expectedVersion)
	if err != nil {
		return fmt.Errorf("mark sub2api token rotation uncertain: %w", err)
	}
	return nil
}

// CompleteRotation atomically installs the pair returned by sub2api only when
// the vault still contains the exact refresh token used for this rotation.
func (s *Service) CompleteRotation(ctx context.Context, expectedRefresh string, expectedVersion int64, input UpsertTokenInput) error {
	accessToken := strings.TrimSpace(input.AccessToken)
	if input.UserID <= 0 || strings.TrimSpace(expectedRefresh) == "" || accessToken == "" {
		return ErrRotationConflict
	}
	matches, err := s.refreshCredentialMatches(ctx, input.UserID, expectedRefresh, "rotating", expectedVersion)
	if err != nil {
		return err
	}
	if !matches {
		return ErrRotationConflict
	}
	accessToken, err = s.protectCredential(accessToken)
	if err != nil {
		return err
	}
	var nextRefresh any
	if input.RefreshToken != nil && strings.TrimSpace(*input.RefreshToken) != "" {
		nextRefresh, err = s.protectCredential(strings.TrimSpace(*input.RefreshToken))
		if err != nil {
			return err
		}
	}
	var expires any
	if input.AccessExpiresAt != nil {
		expires = input.AccessExpiresAt.UTC()
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		UPDATE als_sub2api_auth_tokens
		SET upstream_user_id = COALESCE(?, upstream_user_id),
		    access_token = ?,
		    prev_refresh_token = refresh_token,
		    refresh_token = COALESCE(?, refresh_token),
		    access_expires_at = COALESCE(?, access_expires_at),
		    rotation_state = 'stable',
		    rotation_started_at = NULL,
		    version = version + 1,
		    updated_at = ?
		WHERE user_id = ? AND rotation_state = 'rotating' AND version = ?;
	`), input.UpstreamUserID, accessToken, nextRefresh, expires, now, input.UserID, expectedVersion)
	if err != nil {
		return fmt.Errorf("complete sub2api token rotation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("complete sub2api token rotation rows affected: %w", err)
	}
	if rows != 1 {
		return ErrRotationConflict
	}
	return nil
}

// ClearVault removes the stored upstream tokens for a user. The arbiter calls
// this when sub2api rejects a rotation (the family is dead) so the user
// re-authenticates instead of the server repeatedly forwarding a doomed token.
func (s *Service) ClearVault(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("user id must be positive")
	}

	if _, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		DELETE FROM als_sub2api_auth_tokens WHERE user_id = ?;
	`), userID); err != nil {
		return fmt.Errorf("clear sub2api token vault: %w", err)
	}
	return nil
}
