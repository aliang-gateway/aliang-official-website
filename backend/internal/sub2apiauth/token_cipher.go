package sub2apiauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedTokenPrefix = "enc:v1:"

type tokenCipher struct {
	aead cipher.AEAD
}

func ValidateTokenEncryptionKey(encodedKey string) error {
	_, err := decodeTokenEncryptionKey(strings.TrimSpace(encodedKey))
	return err
}

func newTokenCipher(encodedKey string) (*tokenCipher, error) {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		return nil, nil
	}
	key, err := decodeTokenEncryptionKey(encodedKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token AEAD: %w", err)
	}
	return &tokenCipher{aead: aead}, nil
}

func decodeTokenEncryptionKey(value string) ([]byte, error) {
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		hex.DecodeString,
	}
	for _, decode := range decoders {
		if key, err := decode(value); err == nil && len(key) == 32 {
			return key, nil
		}
	}
	return nil, errors.New("sub2api token encryption key must encode exactly 32 bytes (base64 or hex)")
}

func (c *tokenCipher) seal(plaintext string) (string, error) {
	if c == nil {
		return plaintext, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate token encryption nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, sealed...)
	return encryptedTokenPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *tokenCipher) open(stored string) (string, error) {
	if !strings.HasPrefix(stored, encryptedTokenPrefix) {
		return stored, nil // legacy plaintext row; rewritten on the next capture/rotation
	}
	if c == nil {
		return "", errors.New("encrypted sub2api token found but encryption key is not configured")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, encryptedTokenPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted sub2api token: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) <= nonceSize {
		return "", errors.New("encrypted sub2api token payload is truncated")
	}
	plaintext, err := c.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt sub2api token: %w", err)
	}
	return string(plaintext), nil
}
