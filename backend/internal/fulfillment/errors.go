package fulfillment

import "errors"

var (
	ErrJobNotFound         = errors.New("fulfillment job not found")
	ErrInvalidTransition   = errors.New("invalid fulfillment status transition")
	ErrIdempotencyConflict = errors.New("idempotency key already used with different fulfillment payload")
)
