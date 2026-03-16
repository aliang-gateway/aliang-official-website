package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "authenticated_user"

type AuthenticatedUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(userContextKey).(AuthenticatedUser)
	return user, ok
}

func RequireUser(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			tokenHash := HashSessionToken(token)

			const query = `
				SELECT u.id, u.email, u.name, u.role
				FROM sessions s
				JOIN users u ON u.id = s.user_id
				WHERE s.token_hash = ?
					AND s.revoked_at IS NULL
					AND datetime(s.expires_at) > datetime('now')
				LIMIT 1;
			`
			var user AuthenticatedUser
			err := database.QueryRowContext(r.Context(), query, tokenHash).Scan(&user.ID, &user.Email, &user.Name, &user.Role)
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to authenticate user")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: message})
}
