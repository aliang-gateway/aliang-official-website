package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"ai-api-portal/backend/internal/auth"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db                       *sql.DB
	mailSender               MailSender
	allowedEmailDomains      map[string]struct{}
	requireEmailVerification bool
}

type MailSender interface {
	Send(ctx context.Context, toEmail, subject, body string) error
}

type noopMailSender struct{}

func (noopMailSender) Send(_ context.Context, _, _, _ string) error { return nil }

type ServiceOptions struct {
	MailSender               MailSender
	AllowedEmailDomains      []string
	RequireEmailVerification *bool
}

type AuthUser struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	SessionToken string `json:"session_token"`
}

type UserProfile struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionInfo struct {
	ID        int64
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RegisterResult struct {
	UserID                   int64  `json:"user_id"`
	Email                    string `json:"email"`
	Name                     string `json:"name"`
	SessionToken             string `json:"session_token"`
	EmailVerified            bool   `json:"email_verified"`
	RequireEmailVerification bool   `json:"require_email_verification"`
}

type Wallet struct {
	UserID        int64     `json:"user_id"`
	BalanceMicros int64     `json:"balance_micros"`
	Currency      string    `json:"currency"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WalletTransaction struct {
	ID                 int64     `json:"id"`
	Type               string    `json:"type"`
	AmountMicros       int64     `json:"amount_micros"`
	Currency           string    `json:"currency"`
	BalanceAfterMicros int64     `json:"balance_after_micros"`
	Description        string    `json:"description"`
	CreatedAt          time.Time `json:"created_at"`
}

type ProfileConfig struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	ProfileName   string    `json:"profile_name"`
	ProfileType   string    `json:"profile_type"`
	IsActive      bool      `json:"is_active"`
	ContentFormat string    `json:"content_format"`
	ContentText   string    `json:"content_text"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewService(database *sql.DB) *Service {
	return NewServiceWithOptions(database, ServiceOptions{})
}

func NewServiceWithMailSender(database *sql.DB, sender MailSender) *Service {
	return NewServiceWithOptions(database, ServiceOptions{MailSender: sender})
}

func NewServiceWithOptions(database *sql.DB, opts ServiceOptions) *Service {
	mailSender := opts.MailSender
	if mailSender == nil {
		mailSender = noopMailSender{}
	}

	allowedDomains := make(map[string]struct{})
	for _, domain := range opts.AllowedEmailDomains {
		d := strings.ToLower(strings.TrimSpace(domain))
		if d == "" {
			continue
		}
		allowedDomains[d] = struct{}{}
	}

	requireEmailVerification := true
	if opts.RequireEmailVerification != nil {
		requireEmailVerification = *opts.RequireEmailVerification
	}

	return &Service{
		db:                       database,
		mailSender:               mailSender,
		allowedEmailDomains:      allowedDomains,
		requireEmailVerification: requireEmailVerification,
	}
}

func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	if strings.TrimSpace(password) == "" || strings.TrimSpace(hash) == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthUser, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	var user AuthUser
	var passwordHash sql.NullString
	var emailVerified bool
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, role, password_hash, email_verified
		FROM users
		WHERE email = ?;
	`, email).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &passwordHash, &emailVerified)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if !passwordHash.Valid || passwordHash.String == "" {
		return nil, ErrInvalidCredentials
	}

	if !CheckPassword(password, passwordHash.String) {
		return nil, ErrInvalidCredentials
	}

	if s.requireEmailVerification && !emailVerified {
		return nil, ErrEmailNotVerified
	}

	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, user.ID, tokenHash, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	user.SessionToken = plaintext
	return &user, nil
}

func (s *Service) GetProfile(ctx context.Context, userID int64) (*UserProfile, error) {
	var profile UserProfile
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, role, created_at, updated_at
		FROM users
		WHERE id = ?;
	`, userID).Scan(&profile.ID, &profile.Email, &profile.Name, &profile.Role, &profile.CreatedAt, &profile.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user profile: %w", err)
	}

	return &profile, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, name, email string) error {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" || email == "" {
		return errors.New("name and email are required")
	}

	var existingID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM users WHERE email = ? AND id != ?;
	`, email, userID).Scan(&existingID)
	if err == nil {
		return ErrEmailTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check email uniqueness: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?, updated_at = ?
		WHERE id = ?;
	`, name, email, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("update user profile: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update profile rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s *Service) SetInitialPassword(ctx context.Context, userID int64, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" {
		return errors.New("new password is required")
	}

	var passwordHash sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash FROM users WHERE id = ?;
	`, userID).Scan(&passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("query password hash: %w", err)
	}

	if passwordHash.Valid && passwordHash.String != "" {
		return ErrPasswordAlreadySet
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;
	`, newHash, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	oldPassword = strings.TrimSpace(oldPassword)
	newPassword = strings.TrimSpace(newPassword)
	if oldPassword == "" || newPassword == "" {
		return errors.New("current and new password are required")
	}

	var passwordHash sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash FROM users WHERE id = ?;
	`, userID).Scan(&passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("query password hash: %w", err)
	}

	if !passwordHash.Valid || passwordHash.String == "" {
		return ErrWrongPassword
	}

	if !CheckPassword(oldPassword, passwordHash.String) {
		return ErrWrongPassword
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;
	`, newHash, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	return nil
}

func (s *Service) ListSessions(ctx context.Context, userID int64) ([]SessionInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at, expires_at, revoked_at
		FROM sessions
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC;
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var (
			s         SessionInfo
			revokedAt sql.NullTime
		)

		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			s.RevokedAt = &t
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

func (s *Service) Logout(ctx context.Context, userID int64, sessionToken string) error {
	tokenHash := auth.HashSessionToken(strings.TrimSpace(sessionToken))
	if tokenHash == auth.HashSessionToken("") {
		return ErrInvalidCredentials
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = ?
		WHERE user_id = ?
			AND token_hash = ?
			AND revoked_at IS NULL;
	`, time.Now().UTC(), userID, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("logout rows affected: %w", err)
	}
	if rows == 0 {
		return ErrInvalidCredentials
	}

	return nil
}

func (s *Service) Register(ctx context.Context, email, name, password string) (*RegisterResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	password = strings.TrimSpace(password)
	if email == "" || name == "" || password == "" {
		return nil, errors.New("email, name and password are required")
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("invalid email format")
	}

	if !s.isAllowedEmailDomain(email) {
		return nil, ErrInvalidEmailDomain
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	plaintext, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	verificationCode := ""
	if s.requireEmailVerification {
		verificationCode, err = generateVerificationCode()
		if err != nil {
			return nil, fmt.Errorf("generate verification code: %w", err)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin register tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	emailVerified := 0
	if !s.requireEmailVerification {
		emailVerified = 1
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO users(email, name, role, password_hash, email_verified)
		VALUES (?, ?, 'user', ?, ?);
	`, email, name, hash, emailVerified)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read user id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_wallets(user_id, balance_micros, currency)
		VALUES (?, 0, 'CNY');
	`, userID); err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES (?, ?, ?);
	`, userID, tokenHash, time.Now().UTC().Add(24*time.Hour)); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if s.requireEmailVerification {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_verification_tokens(user_id, code, expires_at)
			VALUES (?, ?, ?);
		`, userID, verificationCode, time.Now().UTC().Add(30*time.Minute)); err != nil {
			return nil, fmt.Errorf("create email verification token: %w", err)
		}

		subject := "Verify your email"
		body := "Your verification code is: " + verificationCode
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_outbox(to_email, subject, body)
			VALUES (?, ?, ?);
		`, email, subject, body); err != nil {
			return nil, fmt.Errorf("write email outbox: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit register tx: %w", err)
	}

	if s.requireEmailVerification {
		subject := "Verify your email"
		body := "Your verification code is: " + verificationCode
		if err := s.mailSender.Send(ctx, email, subject, body); err != nil {
			return nil, fmt.Errorf("send verification email: %w", err)
		}
	}

	return &RegisterResult{
		UserID:                   userID,
		Email:                    email,
		Name:                     name,
		SessionToken:             plaintext,
		EmailVerified:            !s.requireEmailVerification,
		RequireEmailVerification: s.requireEmailVerification,
	}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, email, code string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return ErrInvalidCode
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin verify email tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		tokenID   int64
		userID    int64
		expiresAt time.Time
	)
	err = tx.QueryRowContext(ctx, `
		SELECT evt.id, evt.user_id, evt.expires_at
		FROM email_verification_tokens evt
		JOIN users u ON u.id = evt.user_id
		WHERE u.email = ?
			AND evt.code = ?
			AND evt.used_at IS NULL
		ORDER BY evt.id DESC
		LIMIT 1;
	`, email, code).Scan(&tokenID, &userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCode
	}
	if err != nil {
		return fmt.Errorf("query verification token: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		return ErrCodeExpired
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE email_verification_tokens
		SET used_at = ?
		WHERE id = ?;
	`, time.Now().UTC(), tokenID); err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET email_verified = 1, updated_at = ?
		WHERE id = ?;
	`, time.Now().UTC(), userID); err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit verify email tx: %w", err)
	}

	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return errors.New("email is required")
	}

	var userID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?;`, email).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query user for password reset: %w", err)
	}

	code, err := generateVerificationCode()
	if err != nil {
		return fmt.Errorf("generate password reset code: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens(user_id, code, expires_at)
		VALUES (?, ?, ?);
	`, userID, code, time.Now().UTC().Add(30*time.Minute)); err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}

	subject := "Reset your password"
	body := "Your password reset code is: " + code
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO email_outbox(to_email, subject, body)
		VALUES (?, ?, ?);
	`, email, subject, body); err != nil {
		return fmt.Errorf("write reset email outbox: %w", err)
	}

	if err := s.mailSender.Send(ctx, email, subject, body); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

func (s *Service) ResetPasswordByCode(ctx context.Context, email, code, newPassword string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)
	newPassword = strings.TrimSpace(newPassword)
	if email == "" || code == "" || newPassword == "" {
		return ErrInvalidCode
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset password tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		tokenID   int64
		userID    int64
		expiresAt time.Time
	)
	err = tx.QueryRowContext(ctx, `
		SELECT prt.id, prt.user_id, prt.expires_at
		FROM password_reset_tokens prt
		JOIN users u ON u.id = prt.user_id
		WHERE u.email = ?
			AND prt.code = ?
			AND prt.used_at IS NULL
		ORDER BY prt.id DESC
		LIMIT 1;
	`, email, code).Scan(&tokenID, &userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCode
	}
	if err != nil {
		return fmt.Errorf("query password reset token: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		return ErrCodeExpired
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, updated_at = ?
		WHERE id = ?;
	`, hash, time.Now().UTC(), userID); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE password_reset_tokens
		SET used_at = ?
		WHERE id = ?;
	`, time.Now().UTC(), tokenID); err != nil {
		return fmt.Errorf("mark password reset token used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset password tx: %w", err)
	}

	return nil
}

func (s *Service) GetWallet(ctx context.Context, userID int64) (*Wallet, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO user_wallets(user_id, balance_micros, currency)
		VALUES (?, 0, 'CNY')
		ON CONFLICT(user_id) DO NOTHING;
	`, userID); err != nil {
		return nil, fmt.Errorf("ensure wallet: %w", err)
	}

	var wallet Wallet
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, balance_micros, currency, updated_at
		FROM user_wallets
		WHERE user_id = ?;
	`, userID).Scan(&wallet.UserID, &wallet.BalanceMicros, &wallet.Currency, &wallet.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query wallet: %w", err)
	}

	return &wallet, nil
}

func (s *Service) RedeemCard(ctx context.Context, userID int64, cardCode string) (*Wallet, error) {
	cardCode = strings.TrimSpace(strings.ToUpper(cardCode))
	if cardCode == "" {
		return nil, errors.New("card code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin redeem tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_wallets(user_id, balance_micros, currency)
		VALUES (?, 0, 'CNY')
		ON CONFLICT(user_id) DO NOTHING;
	`, userID); err != nil {
		return nil, fmt.Errorf("ensure wallet: %w", err)
	}

	var (
		cardID       int64
		amountMicros int64
		currency     string
		expiresAt    sql.NullTime
		redeemedBy   sql.NullInt64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id, amount_micros, currency, expires_at, redeemed_by_user_id
		FROM recharge_cards
		WHERE card_code = ?;
	`, cardCode).Scan(&cardID, &amountMicros, &currency, &expiresAt, &redeemedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query recharge card: %w", err)
	}

	if redeemedBy.Valid {
		return nil, ErrCardAlreadyRedeemed
	}
	if expiresAt.Valid && time.Now().UTC().After(expiresAt.Time) {
		return nil, ErrCardExpired
	}

	var balanceAfter int64
	err = tx.QueryRowContext(ctx, `
		SELECT balance_micros + ?
		FROM user_wallets
		WHERE user_id = ?;
	`, amountMicros, userID).Scan(&balanceAfter)
	if err != nil {
		return nil, fmt.Errorf("compute new wallet balance: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_wallets
		SET balance_micros = ?, updated_at = ?
		WHERE user_id = ?;
	`, balanceAfter, time.Now().UTC(), userID); err != nil {
		return nil, fmt.Errorf("update wallet: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE recharge_cards
		SET redeemed_by_user_id = ?, redeemed_at = ?
		WHERE id = ?;
	`, userID, time.Now().UTC(), cardID); err != nil {
		return nil, fmt.Errorf("mark card redeemed: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wallet_transactions(
			user_id, tx_type, amount_micros, currency, balance_after_micros, reference_type, reference_id, description
		)
		VALUES (?, 'recharge', ?, ?, ?, 'recharge_card', ?, ?);
	`, userID, amountMicros, currency, balanceAfter, cardID, "Recharge via card code"); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit redeem tx: %w", err)
	}

	return s.GetWallet(ctx, userID)
}

func (s *Service) ListWalletTransactions(ctx context.Context, userID int64, limit int) ([]WalletTransaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tx_type, amount_micros, currency, balance_after_micros, COALESCE(description, ''), created_at
		FROM wallet_transactions
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ?;
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query wallet transactions: %w", err)
	}
	defer rows.Close()

	txs := make([]WalletTransaction, 0)
	for rows.Next() {
		var tx WalletTransaction
		if err := rows.Scan(&tx.ID, &tx.Type, &tx.AmountMicros, &tx.Currency, &tx.BalanceAfterMicros, &tx.Description, &tx.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan wallet transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet transactions: %w", err)
	}

	return txs, nil
}

func (s *Service) CreateProfileConfig(ctx context.Context, userID int64, profileName, profileType, contentFormat, contentText string, isActive bool) (*ProfileConfig, error) {
	profileName = strings.TrimSpace(profileName)
	profileType = strings.TrimSpace(profileType)
	contentFormat = normalizeContentFormat(contentFormat)
	contentText = strings.TrimSpace(contentText)
	if profileName == "" || profileType == "" || contentText == "" {
		return nil, ErrInvalidProfileData
	}
	if err := validateProfileContent(contentFormat, contentText); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create profile tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if isActive {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_profiles
			SET is_active = 0, updated_at = ?
			WHERE user_id = ? AND profile_type = ?;
		`, time.Now().UTC(), userID, profileType); err != nil {
			return nil, fmt.Errorf("deactivate previous profiles: %w", err)
		}
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO user_profiles(user_id, profile_name, profile_type, is_active, content_format, content_text)
		VALUES (?, ?, ?, ?, ?, ?);
	`, userID, profileName, profileType, boolToInt(isActive), contentFormat, contentText)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrProfileNameTaken
		}
		return nil, fmt.Errorf("insert profile: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read profile id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create profile tx: %w", err)
	}

	return s.GetProfileConfig(ctx, userID, id)
}

func (s *Service) GetProfileConfig(ctx context.Context, userID, profileID int64) (*ProfileConfig, error) {
	var (
		cfg      ProfileConfig
		isActive int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, profile_name, profile_type, is_active, content_format, content_text, created_at, updated_at
		FROM user_profiles
		WHERE user_id = ? AND id = ?;
	`, userID, profileID).Scan(
		&cfg.ID, &cfg.UserID, &cfg.ProfileName, &cfg.ProfileType, &isActive,
		&cfg.ContentFormat, &cfg.ContentText, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query profile: %w", err)
	}
	cfg.IsActive = isActive == 1
	return &cfg, nil
}

func (s *Service) ListProfileConfigs(ctx context.Context, userID int64, profileType string) ([]ProfileConfig, error) {
	profileType = strings.TrimSpace(profileType)
	query := `
		SELECT id, user_id, profile_name, profile_type, is_active, content_format, content_text, created_at, updated_at
		FROM user_profiles
		WHERE user_id = ?
	`
	args := []any{userID}
	if profileType != "" {
		query += " AND profile_type = ?"
		args = append(args, profileType)
	}
	query += " ORDER BY id DESC;"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]ProfileConfig, 0)
	for rows.Next() {
		var (
			cfg      ProfileConfig
			isActive int64
		)
		if err := rows.Scan(
			&cfg.ID, &cfg.UserID, &cfg.ProfileName, &cfg.ProfileType, &isActive,
			&cfg.ContentFormat, &cfg.ContentText, &cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		cfg.IsActive = isActive == 1
		profiles = append(profiles, cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profiles: %w", err)
	}

	return profiles, nil
}

func (s *Service) UpdateProfileConfig(ctx context.Context, userID, profileID int64, profileName, profileType, contentFormat, contentText string, isActive bool) (*ProfileConfig, error) {
	profileName = strings.TrimSpace(profileName)
	profileType = strings.TrimSpace(profileType)
	contentFormat = normalizeContentFormat(contentFormat)
	contentText = strings.TrimSpace(contentText)
	if profileName == "" || profileType == "" || contentText == "" {
		return nil, ErrInvalidProfileData
	}
	if err := validateProfileContent(contentFormat, contentText); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update profile tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if isActive {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_profiles
			SET is_active = 0, updated_at = ?
			WHERE user_id = ? AND profile_type = ? AND id != ?;
		`, time.Now().UTC(), userID, profileType, profileID); err != nil {
			return nil, fmt.Errorf("deactivate other active profiles: %w", err)
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE user_profiles
		SET profile_name = ?, profile_type = ?, is_active = ?, content_format = ?, content_text = ?, updated_at = ?
		WHERE user_id = ? AND id = ?;
	`, profileName, profileType, boolToInt(isActive), contentFormat, contentText, time.Now().UTC(), userID, profileID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrProfileNameTaken
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read update profile rows: %w", err)
	}
	if affected == 0 {
		return nil, ErrProfileNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update profile tx: %w", err)
	}

	return s.GetProfileConfig(ctx, userID, profileID)
}

func (s *Service) DeleteProfileConfig(ctx context.Context, userID, profileID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM user_profiles
		WHERE user_id = ? AND id = ?;
	`, userID, profileID)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete profile rows: %w", err)
	}
	if affected == 0 {
		return ErrProfileNotFound
	}

	return nil
}

func normalizeContentFormat(contentFormat string) string {
	f := strings.ToLower(strings.TrimSpace(contentFormat))
	if f == "" {
		return "json"
	}
	return f
}

func validateProfileContent(contentFormat, contentText string) error {
	switch contentFormat {
	case "json":
		if !json.Valid([]byte(contentText)) {
			return ErrInvalidProfileData
		}
		return nil
	case "yaml", "yml":
		if !isLikelyValidYAML(contentText) {
			return ErrInvalidProfileData
		}
		return nil
	default:
		return ErrInvalidProfileData
	}
}

func generateVerificationCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

func (s *Service) isAllowedEmailDomain(email string) bool {
	idx := strings.LastIndex(email, "@")
	if idx <= 0 || idx+1 >= len(email) {
		return false
	}
	if len(s.allowedEmailDomains) == 0 {
		return true
	}
	domain := strings.ToLower(email[idx+1:])
	_, ok := s.allowedEmailDomains[domain]
	return ok
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isLikelyValidYAML(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return false
	}
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ":") || strings.HasPrefix(line, "-") {
			return true
		}
	}
	return false
}
