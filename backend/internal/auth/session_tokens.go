package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const sessionTokenPrefix = "st_"

func NewSessionToken() (plaintext string, tokenHash string, err error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", fmt.Errorf("generate random token bytes: %w", err)
	}

	plaintext = sessionTokenPrefix + hex.EncodeToString(random)
	return plaintext, HashSessionToken(plaintext), nil
}

func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
