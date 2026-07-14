package sub2api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"ai-api-portal/backend/internal/proxy"
	"ai-api-portal/backend/internal/sub2apiauth"
)

// UserResolver resolves local session tokens to user IDs and roles.
// routes implements this interface so Gateway does not depend on routes.
type UserResolver interface {
	FindUserIDBySession(ctx context.Context, sessionToken string) (int64, bool, error)
	FindUserRoleByID(ctx context.Context, userID int64) (string, bool, error)
	// EnsureFreshUpstreamAccessToken rotates the user's cached sub2api
	// access_token via sub2api if it has expired, so passthroughs keep working.
	// Best-effort: returning an error does not abort the caller, which falls back
	// to the stored token (or surfaces ErrTokenNotFound).
	EnsureFreshUpstreamAccessToken(ctx context.Context, userID int64) error
}

// Gateway combines the proxy client and auth service into a single entry point
// for upstream sub2api interactions.
type Gateway struct {
	proxy *proxy.Client
	auth  *sub2apiauth.Service
}

// NewGateway creates a Gateway from its two dependencies.
func NewGateway(proxyClient *proxy.Client, authSvc *sub2apiauth.Service) *Gateway {
	return &Gateway{proxy: proxyClient, auth: authSvc}
}

// IsConfigured returns true when both the proxy client and auth service are present.
func (g *Gateway) IsConfigured() bool {
	return g != nil && g.proxy != nil && g.auth != nil
}

// HasUpstreamToken checks whether a user has a stored upstream bearer token.
func (g *Gateway) HasUpstreamToken(ctx context.Context, userID int64) (bool, error) {
	if g == nil || g.auth == nil {
		return false, nil
	}
	_, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
		return false, nil
	}
	return false, err
}

func (g *Gateway) UpstreamUserID(ctx context.Context, userID int64) (int64, bool, error) {
	if g == nil || g.auth == nil {
		return 0, false, nil
	}
	return g.auth.GetUpstreamUserIDByUserID(ctx, userID)
}

// LoadVault returns the user's stored upstream token pair for the refresh
// arbiter (current + previous refresh, and access-token expiry). nil when absent.
func (g *Gateway) LoadVault(ctx context.Context, userID int64) (*sub2apiauth.VaultTokens, error) {
	if g == nil || g.auth == nil {
		return nil, nil
	}
	return g.auth.LoadVault(ctx, userID)
}

// ClearVault drops the user's stored upstream tokens (family invalidated).
func (g *Gateway) ClearVault(ctx context.Context, userID int64) error {
	if g == nil || g.auth == nil {
		return nil
	}
	return g.auth.ClearVault(ctx, userID)
}

func (g *Gateway) BeginRotation(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	if g == nil || g.auth == nil {
		return sub2apiauth.ErrTokenNotFound
	}
	return g.auth.BeginRotation(ctx, userID, expectedRefresh, expectedVersion)
}

func (g *Gateway) ResetRotation(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	if g == nil || g.auth == nil {
		return sub2apiauth.ErrTokenNotFound
	}
	return g.auth.ResetRotation(ctx, userID, expectedRefresh, expectedVersion)
}

func (g *Gateway) MarkRotationUncertain(ctx context.Context, userID int64, expectedRefresh string, expectedVersion int64) error {
	if g == nil || g.auth == nil {
		return sub2apiauth.ErrTokenNotFound
	}
	return g.auth.MarkRotationUncertain(ctx, userID, expectedRefresh, expectedVersion)
}

func (g *Gateway) CompleteRotation(ctx context.Context, expectedRefresh string, expectedVersion int64, input sub2apiauth.UpsertTokenInput) error {
	if g == nil || g.auth == nil {
		return sub2apiauth.ErrTokenNotFound
	}
	return g.auth.CompleteRotation(ctx, expectedRefresh, expectedVersion, input)
}

func (g *Gateway) ProtectCredential(value string) (string, error) {
	if g == nil || g.auth == nil {
		return "", sub2apiauth.ErrTokenNotFound
	}
	return g.auth.ProtectCredential(value)
}

// CreateUserAPIKeyForUser resolves the user's upstream token and creates an API key.
func (g *Gateway) CreateUserAPIKeyForUser(ctx context.Context, userID int64, req proxy.CreateUserAPIKeyRequest, idempotencyKey string) (*proxy.ResponseEnvelope[proxy.APIKey], error) {
	bearerToken, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get bearer token for user %d: %w", userID, err)
	}
	return g.proxy.CreateUserAPIKey(ctx, bearerToken, req, idempotencyKey)
}

// EnsureUserKeyInGroup ensures the user has an auto-key in the specified group,
// tolerating 409 (already exists).
func (g *Gateway) EnsureUserKeyInGroup(ctx context.Context, userID int64, groupID int64, parentIdempotencyKey string) error {
	if g == nil || g.proxy == nil || g.auth == nil {
		return nil
	}
	bearerToken, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get bearer token for user %d: %w", userID, err)
	}
	keys, err := g.proxy.ListUserAPIKeys(ctx, bearerToken, groupID, "auto-key")
	if err != nil {
		return err
	}
	for _, key := range keys.Data {
		if key.GroupID == groupID && strings.TrimSpace(key.Name) == "auto-key" {
			return nil
		}
	}
	childKey := parentIdempotencyKey + ":ensure-key:" + strconv.FormatInt(groupID, 10)
	_, createErr := g.proxy.CreateUserAPIKey(ctx, bearerToken, proxy.CreateUserAPIKeyRequest{
		Name:    "auto-key",
		GroupID: groupID,
	}, childKey)
	if createErr == nil {
		return nil
	}
	var apiErr *proxy.APIError
	if errors.As(createErr, &apiErr) && apiErr.IsConflict() {
		return nil // key already exists
	}
	return createErr
}

// ReplaceAuthHeader resolves a local session token in the Authorization header
// to the user's upstream bearer token. Admin users without an upstream token
// are left unchanged.
func (g *Gateway) ReplaceAuthHeader(ctx context.Context, headers http.Header, resolver UserResolver) error {
	if headers == nil || g == nil || g.auth == nil {
		return nil
	}

	authHeader := strings.TrimSpace(headers.Get("Authorization"))
	if authHeader == "" {
		return nil
	}

	localSessionToken, err := extractBearerToken(authHeader)
	if err != nil {
		return nil
	}

	userID, found, err := resolver.FindUserIDBySession(ctx, localSessionToken)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	// Keep the cached sub2api access_token fresh: rotate it via sub2api if it has
	// expired, so data passthroughs don't fail with INVALID_TOKEN just because the
	// credential aged out. Best-effort — a failure is logged but does not abort
	// (the stored token may still work, or the caller surfaces ErrTokenNotFound);
	// a dead token family is cleared inside EnsureFresh to force re-authentication.
	if refreshErr := resolver.EnsureFreshUpstreamAccessToken(ctx, userID); refreshErr != nil {
		slog.Warn("replace auth header: ensure fresh upstream token failed", "user_id", userID, "error", refreshErr)
	}

	upstreamAccessToken, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sub2apiauth.ErrTokenNotFound) {
			role, foundRole, roleErr := resolver.FindUserRoleByID(ctx, userID)
			if roleErr != nil {
				return roleErr
			}
			if foundRole && role == "admin" {
				return nil
			}
		}
		return err
	}

	headers.Set("Authorization", "Bearer "+upstreamAccessToken)
	return nil
}

// CaptureTokens delegates to the auth service's UpsertToken.
func (g *Gateway) CaptureTokens(ctx context.Context, input sub2apiauth.UpsertTokenInput) error {
	if g == nil || g.auth == nil {
		return nil
	}
	return g.auth.UpsertToken(ctx, input)
}

// extractBearerToken extracts the token portion from a "Bearer <token>" header.
func extractBearerToken(rawAuthHeader string) (string, error) {
	authHeader := strings.TrimSpace(rawAuthHeader)
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", errors.New("missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if token == "" {
		return "", errors.New("missing bearer token")
	}

	return token, nil
}
