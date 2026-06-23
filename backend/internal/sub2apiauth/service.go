package sub2apiauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-api-portal/backend/internal/db"
)

var ErrTokenNotFound = errors.New("sub2api token not found")

type Service struct {
	db         *sql.DB
	sqlDialect string
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
	return &Service{db: database, sqlDialect: sqlDialect}
}

func (s *Service) UpsertToken(ctx context.Context, input UpsertTokenInput) error {
	if input.UserID <= 0 {
		return errors.New("user id must be positive")
	}
	accessToken := strings.TrimSpace(input.AccessToken)
	if accessToken == "" {
		return errors.New("access token is required")
	}

	var refreshToken any
	if input.RefreshToken != nil {
		refreshToken = strings.TrimSpace(*input.RefreshToken)
		if refreshToken == "" {
			refreshToken = nil
		}
	}

	var accessExpires any
	if input.AccessExpiresAt != nil {
		accessExpires = input.AccessExpiresAt.UTC()
	}

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		INSERT INTO als_sub2api_auth_tokens(
			user_id,
			upstream_user_id,
			access_token,
			refresh_token,
			access_expires_at,
			prev_refresh_token,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
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

	return bearer, nil
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

	return refresh.String, nil
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
// RefreshToken is the current value clients should converge on; PrevRefreshToken
// is the immediately-previous value accepted as a one-generation grace window.
type VaultTokens struct {
	UserID           int64
	AccessToken      string
	RefreshToken     string // current
	HasRefresh       bool
	PrevRefreshToken string // grace-window predecessor
	HasPrevRefresh   bool
	AccessExpiresAt  time.Time
	HasAccessExpires bool
}

// LoadVault returns the current stored token pair for the user, or nil when no
// row exists. Read under the arbiter's per-user lock to make freshness decisions.
func (s *Service) LoadVault(ctx context.Context, userID int64) (*VaultTokens, error) {
	if userID <= 0 {
		return nil, errors.New("user id must be positive")
	}

	var (
		access      string
		refresh     sql.NullString
		prevRefresh sql.NullString
		expires     sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT access_token, refresh_token, prev_refresh_token, access_expires_at
		FROM als_sub2api_auth_tokens
		WHERE user_id = ?;
	`), userID).Scan(&access, &refresh, &prevRefresh, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load sub2api token vault: %w", err)
	}

	return &VaultTokens{
		UserID:           userID,
		AccessToken:      access,
		HasRefresh:       refresh.Valid && strings.TrimSpace(refresh.String) != "",
		RefreshToken:     refresh.String,
		HasPrevRefresh:   prevRefresh.Valid && strings.TrimSpace(prevRefresh.String) != "",
		PrevRefreshToken: prevRefresh.String,
		HasAccessExpires: expires.Valid,
		AccessExpiresAt:  expires.Time,
	}, nil
}

// FindUserIDByRefreshOrPrev resolves a local user from a refresh_token that is
// either the current or the immediately-previous (grace-window) stored value.
// A token older than that (or unknown) returns found=false so the arbiter forces
// re-authentication rather than forwarding a stale token upstream and tripping
// sub2api's refresh-token replay detection.
func (s *Service) FindUserIDByRefreshOrPrev(ctx context.Context, refreshToken string) (int64, bool, error) {
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return 0, false, nil
	}

	var userID int64
	err := s.db.QueryRowContext(ctx, db.Rebind(s.sqlDialect, `
		SELECT user_id
		FROM als_sub2api_auth_tokens
		WHERE refresh_token = ? OR prev_refresh_token = ?
		LIMIT 1;
	`), trimmed, trimmed).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find user by refresh token: %w", err)
	}

	return userID, true, nil
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
