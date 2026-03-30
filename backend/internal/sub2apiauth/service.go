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

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, db.Rebind(s.sqlDialect, `
		INSERT INTO als_sub2api_auth_tokens(
			user_id,
			upstream_user_id,
			access_token,
			refresh_token,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			upstream_user_id = COALESCE(excluded.upstream_user_id, als_sub2api_auth_tokens.upstream_user_id),
			access_token = excluded.access_token,
			refresh_token = COALESCE(excluded.refresh_token, als_sub2api_auth_tokens.refresh_token),
			updated_at = excluded.updated_at;
	`), input.UserID, input.UpstreamUserID, accessToken, refreshToken, now, now)
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
