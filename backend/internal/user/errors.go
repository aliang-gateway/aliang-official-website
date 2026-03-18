package user

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailTaken          = errors.New("email already taken")
	ErrWrongPassword       = errors.New("wrong current password")
	ErrPasswordAlreadySet  = errors.New("password already set")
	ErrEmailNotVerified    = errors.New("email not verified")
	ErrInvalidEmailDomain  = errors.New("email domain is not allowed")
	ErrInvalidCode         = errors.New("invalid verification code")
	ErrCodeExpired         = errors.New("verification code expired")
	ErrCardNotFound        = errors.New("recharge card not found")
	ErrCardAlreadyRedeemed = errors.New("recharge card already redeemed")
	ErrCardExpired         = errors.New("recharge card expired")
	ErrProfileNotFound     = errors.New("profile not found")
	ErrInvalidProfileData  = errors.New("invalid profile data")
	ErrProfileNameTaken    = errors.New("profile name already taken")
)
