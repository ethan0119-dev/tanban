// Package securestore provides versioned authenticated encryption for
// tenant-scoped secrets. It deliberately stores no keys and performs no I/O;
// callers supply a dedicated 256-bit master key from an external secret file.
package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const version = "v1"

type Store struct {
	aead cipher.AEAD
}

func New(keyValue string) (*Store, error) {
	keyValue = strings.TrimSpace(keyValue)
	key := []byte(keyValue)
	if decoded, err := base64.StdEncoding.DecodeString(keyValue); err == nil && len(decoded) == 32 {
		key = decoded
	} else if decoded, err = base64.RawStdEncoding.DecodeString(keyValue); err == nil && len(decoded) == 32 {
		key = decoded
	}
	if len(key) != 32 {
		return nil, errors.New("data encryption key must be exactly 32 bytes or base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{aead: aead}, nil
}

// Encrypt binds a ciphertext to its tenant and purpose using AEAD additional
// data. A ciphertext copied to another tenant or field will not decrypt.
func (s *Store) Encrypt(plaintext []byte, tenantID int64, purpose string) (string, error) {
	if s == nil || s.aead == nil {
		return "", errors.New("secure store is not configured")
	}
	if tenantID <= 0 || strings.TrimSpace(purpose) == "" {
		return "", errors.New("tenant and purpose are required")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := s.aead.Seal(nil, nonce, plaintext, additionalData(tenantID, purpose))
	payload := append(nonce, sealed...)
	return version + ":" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (s *Store) Decrypt(ciphertext string, tenantID int64, purpose string) ([]byte, error) {
	if s == nil || s.aead == nil {
		return nil, errors.New("secure store is not configured")
	}
	prefix, encoded, ok := strings.Cut(strings.TrimSpace(ciphertext), ":")
	if !ok || prefix != version {
		return nil, errors.New("unsupported ciphertext version")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid ciphertext encoding")
	}
	if len(payload) < s.aead.NonceSize()+s.aead.Overhead() {
		return nil, errors.New("ciphertext is too short")
	}
	nonce, sealed := payload[:s.aead.NonceSize()], payload[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, sealed, additionalData(tenantID, purpose))
	if err != nil {
		return nil, errors.New("ciphertext authentication failed")
	}
	return plaintext, nil
}

func additionalData(tenantID int64, purpose string) []byte {
	return []byte(fmt.Sprintf("tanban|%s|tenant:%d|%s", version, tenantID, strings.TrimSpace(purpose)))
}
