package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ai-api-portal/backend/internal/user"
)

// 注册邮箱验证：portal 自有能力（发码/验码/开关），不依赖上游 sub2api。
// 开关是运行时值，存 als_portal_settings（admin 可切换）；DB 无记录时回退
// 到 config 默认（register.require_email_verification，默认关）。

const portalSettingEmailVerification = "register_email_verification"

func (r *routes) getEmailVerificationEnabled(ctx context.Context) bool {
	var value string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM als_portal_settings WHERE key = ?;`,
		portalSettingEmailVerification).Scan(&value)
	if err != nil {
		return r.emailVerificationDefault
	}
	return value == "true"
}

func (r *routes) setEmailVerificationEnabled(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO als_portal_settings(key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;
	`, portalSettingEmailVerification, value)
	return err
}

// handleRegisterSendCode issues a registration verification code. Public, but
// gated by the runtime switch and strictly rate-limited — an open relay here
// would turn the portal into a free mass-mailer.
func (r *routes) handleRegisterSendCode(w http.ResponseWriter, req *http.Request) {
	if !r.getEmailVerificationEnabled(req.Context()) {
		writeError(w, http.StatusNotFound, "email verification is not enabled")
		return
	}

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(payload.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	// 发码限流：per-IP 3/10min + per-email 2/10min。防邮件轰炸通道。
	if r.registerGuard != nil && r.registerGuard.limiter != nil {
		clientIP := registerClientIP(req)
		if !r.registerGuard.limiter.Allow("sendcode:ip:"+clientIP, 3, 10*time.Minute) {
			writeError(w, http.StatusTooManyRequests, "too many verification code requests, please retry later")
			return
		}
		if !r.registerGuard.limiter.Allow("sendcode:email:"+email, 2, 10*time.Minute) {
			writeError(w, http.StatusTooManyRequests, "too many verification code requests for this email, please retry later")
			return
		}
	}

	if err := r.userSvc.IssueRegisterEmailCode(req.Context(), email); err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidEmailDomain):
			// 域名不合法与成功同样应答，避免被用作邮箱探测面
			writeJSON(w, http.StatusOK, map[string]string{"message": "if the address is valid, a code has been sent"})
		case errors.Is(err, user.ErrMailSenderRequired):
			slog.Error("register send-code without mail sender")
			writeError(w, http.StatusInternalServerError, "mail delivery is not available")
		default:
			if strings.Contains(err.Error(), "invalid email format") {
				writeError(w, http.StatusBadRequest, "invalid email format")
				return
			}
			slog.Error("register send-code failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to send verification code")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "if the address is valid, a code has been sent"})
}

// handleRegisterOptions exposes runtime registration switches to the frontend
// (public; no auth) so the page renders only the controls that are enforced.
func (r *routes) handleRegisterOptions(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"email_verification": r.getEmailVerificationEnabled(req.Context()),
	})
}

// handleAdminGetRegisterSettings exposes the runtime switch state to admins.
func (r *routes) handleAdminGetRegisterSettings(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"email_verification_enabled": r.getEmailVerificationEnabled(req.Context()),
		"smtp_configured":            r.smtpConfigured,
	})
}

// handleAdminUpdateRegisterSettings toggles the runtime switch. Enabling
// without a configured SMTP would deadlock registration, so it is rejected
// outright instead of failing silently on the first send-code.
func (r *routes) handleAdminUpdateRegisterSettings(w http.ResponseWriter, req *http.Request) {
	var payload struct {
		EmailVerificationEnabled bool `json:"email_verification_enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if payload.EmailVerificationEnabled && !r.smtpConfigured {
		writeError(w, http.StatusBadRequest, "cannot enable email verification: smtp is not configured")
		return
	}
	if err := r.setEmailVerificationEnabled(req.Context(), payload.EmailVerificationEnabled); err != nil {
		slog.Error("update register settings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update register settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email_verification_enabled": payload.EmailVerificationEnabled,
		"smtp_configured":            r.smtpConfigured,
	})
}
