// Package shared contains helpers used by both postgres and sqlite storage implementations.
package shared

import (
	"fmt"
	"log/slog"

	"rmsmail/internal/crypto"
)

// EncryptPassword encrypts a password using the primary key with domain derivation.
func EncryptPassword(encKey []byte, password, domain string) (string, error) {
	if password == "" {
		return "", nil
	}
	if encKey == nil {
		return "", fmt.Errorf("encryption key not set")
	}
	derived := crypto.DeriveKeyWithDomain(encKey, domain)
	enc, err := crypto.Encrypt(password, derived)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}
	return enc, nil
}

// DecryptPassword decrypts a password trying all keys (primary + fallbacks).
func DecryptPassword(encKeys [][]byte, encrypted, domain string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	if len(encKeys) == 0 || encKeys[0] == nil {
		slog.Warn("DecryptPassword called with nil encKey — password decryption unavailable")
		return "", fmt.Errorf("encryption key not configured")
	}
	for _, key := range encKeys {
		derived := crypto.DeriveKeyWithDomain(key, domain)
		dec, err := crypto.Decrypt(encrypted, derived)
		if err == nil {
			return dec, nil
		}
	}
	for _, key := range encKeys {
		dec, err := crypto.Decrypt(encrypted, key)
		if err == nil {
			return dec, nil
		}
	}
	return "", fmt.Errorf("decryption failed")
}
