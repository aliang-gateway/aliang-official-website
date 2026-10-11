package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// 注册前邮箱验证码：portal 自有能力，不依赖上游 sub2api。与忘记密码的
// als_email_verification_tokens（绑定 user_id）不同，注册阶段本地还没有
// 用户，所以码以邮箱为键存放在 als_register_email_codes（migration 0031）。
var (
	// registerCodeTTL 验证码有效期：足够完成一次注册，短到不残留攻击面。
	registerCodeTTL = 10 * time.Minute
	// registerCodeMaxAttempts 连错上限：达到即作废，须重新发码（防拖库爆破）。
	registerCodeMaxAttempts = 5
)

var (
	ErrRegisterCodeInvalid = errors.New("email verification code is invalid")
	ErrRegisterCodeExpired = errors.New("email verification code has expired")
	ErrMailSenderRequired  = errors.New("mail sender is not configured")
)

// IssueRegisterEmailCode 生成新的注册验证码并经 SMTP 发送到邮箱。同一邮箱
// 重发会覆盖旧码（旧码立即失效），尝试计数一并重置。
func (s *Service) IssueRegisterEmailCode(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email format")
	}
	if !s.isAllowedEmailDomain(email) {
		return ErrInvalidEmailDomain
	}

	code, err := generateVerificationCode()
	if err != nil {
		return fmt.Errorf("generate register code: %w", err)
	}

	// UPSERT：每邮箱只保留最新一码，重发即覆盖并重置尝试计数。
	query := `
		INSERT INTO als_register_email_codes(email, code, expires_at, attempts)
		VALUES (?, ?, ?, 0)
		ON CONFLICT(email) DO UPDATE SET
			code = excluded.code,
			expires_at = excluded.expires_at,
			used_at = NULL,
			attempts = 0,
			created_at = CURRENT_TIMESTAMP;
	`
	if _, err := s.db.ExecContext(ctx, s.rebind(query), email, code, time.Now().UTC().Add(registerCodeTTL)); err != nil {
		return fmt.Errorf("store register code: %w", err)
	}

	subject := "Your registration verification code"
	body := "Your verification code is: " + code
	if err := s.mailSender.Send(ctx, email, subject, body); err != nil {
		return fmt.Errorf("send register code email: %w", err)
	}
	return nil
}

// VerifyRegisterEmailCode 校验并一次性消费注册验证码。无论对错都推进防
// 爆破状态：错误累计 attempts，达到上限后作废（须重新发码）。
func (s *Service) VerifyRegisterEmailCode(ctx context.Context, email, code string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(strings.ToUpper(code))
	if email == "" || code == "" {
		return ErrRegisterCodeInvalid
	}

	var (
		storedCode string
		expiresAt  time.Time
		attempts   int
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT code, expires_at, attempts
		FROM als_register_email_codes
		WHERE email = ?;
	`, email).Scan(&storedCode, &expiresAt, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRegisterCodeInvalid
	}
	if err != nil {
		return fmt.Errorf("query register code: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM als_register_email_codes WHERE email = ?;`, email)
		return ErrRegisterCodeExpired
	}
	if attempts >= registerCodeMaxAttempts {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM als_register_email_codes WHERE email = ?;`, email)
		return ErrRegisterCodeExpired
	}

	if storedCode != code {
		newAttempts := attempts + 1
		if newAttempts >= registerCodeMaxAttempts {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM als_register_email_codes WHERE email = ?;`, email)
		} else {
			_, _ = s.db.ExecContext(ctx, `UPDATE als_register_email_codes SET attempts = ? WHERE email = ?;`, newAttempts, email)
		}
		return ErrRegisterCodeInvalid
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM als_register_email_codes WHERE email = ?;`, email)
	if err != nil {
		return fmt.Errorf("consume register code: %w", err)
	}
	return nil
}
