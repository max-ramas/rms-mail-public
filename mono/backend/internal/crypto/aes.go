package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// GetPrimaryEncryptionKey reads ENCRYPTION_KEYS (comma-separated list) first,
// and falls back to ENCRYPTION_KEY. The first key in the list encrypts new data;
// all keys are tried for decryption.
func GetPrimaryEncryptionKey() string {
	if k := os.Getenv("ENCRYPTION_KEYS"); k != "" {
		for _, key := range strings.Split(k, ",") {
			key = strings.TrimSpace(key)
			if key != "" {
				return key
			}
		}
	}
	return os.Getenv("ENCRYPTION_KEY")
}

// GetAllEncryptionKeys returns all encryption keys from ENCRYPTION_KEYS
// (comma-separated) with fallback to ENCRYPTION_KEY, as [][]byte.
func GetAllEncryptionKeys() [][]byte {
	var keys [][]byte
	raw := os.Getenv("ENCRYPTION_KEYS")
	if raw == "" {
		raw = os.Getenv("ENCRYPTION_KEY")
	}
	for _, keyStr := range strings.Split(raw, ",") {
		keyStr = strings.TrimSpace(keyStr)
		if keyStr != "" {
			keys = append(keys, []byte(keyStr))
		}
	}
	return keys
}

func deriveKey(raw []byte) []byte {
	h := sha256.Sum256(raw)
	return h[:]
}

// DeriveKeyWithDomain returns a domain-separated key via SHA-256(raw || ":" || domain).
// This prevents reuse of the same encryption key across different data types
// (IMAP passwords, OAuth tokens, MCP API keys, Telegram tokens).
func DeriveKeyWithDomain(raw []byte, domain string) []byte {
	input := append(raw, byte(':'))
	input = append(input, []byte(domain)...)
	h := sha256.Sum256(input)
	return h[:]
}

// Encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// The nonce is prepended to the ciphertext, and the result is base64-encoded.
// Panics if the nonce size is not 12 bytes (standard for AES-GCM).
func Encrypt(plaintext string, key []byte) (string, error) {
	derivedKey := deriveKey(key)
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if aesGCM.NonceSize() != 12 {
		panic("aes-gcm: unexpected nonce size; expected 12 bytes")
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext encrypted with Encrypt.
// It tries both the derived key and the raw key for backward compatibility
// with older-format passwords.
func Decrypt(cryptoText string, key []byte) (string, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	// Try derived key first, then raw key (for old-format passwords)
	for _, k := range [][]byte{deriveKey(key), key} {
		plain, err := tryDecrypt(ciphertext, k)
		if err == nil {
			return plain, nil
		}
	}
	return "", errors.New("decryption failed")
}

func tryDecrypt(ciphertext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptBytes encrypts a byte slice using AES-256-GCM with a random 12-byte nonce.
// The nonce is prepended to the ciphertext.
func EncryptBytes(plaintext []byte, key []byte) ([]byte, error) {
	derivedKey := deriveKey(key)
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptBytes decrypts a ciphertext byte slice using AES-256-GCM.
func DecryptBytes(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aesGCM.Open(nil, nonce, ct, nil)
}

// ReadEncryptedFile reads a file that may be AES-GCM encrypted and/or Zstd compressed.
// Returns the plaintext content. If the file is not encrypted, returns it as-is.
// Falls back gracefully for old-format or unencrypted files.
func ReadEncryptedFile(path, keyEnv string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key := []byte(keyEnv)
	var plain []byte
	if len(key) == 0 {
		plain = data
	} else {
		decrypted, err := DecryptBytes(data, key)
		if err != nil {
			plain = data
		} else {
			plain = decrypted
		}
	}
	if len(plain) >= 4 && plain[0] == 0x28 && plain[1] == 0xb5 && plain[2] == 0x2f && plain[3] == 0xfd {
		decoder, err := zstd.NewReader(nil)
		if err == nil {
			defer decoder.Close()
			decompressed, err := decoder.DecodeAll(plain, nil)
			if err == nil {
				return decompressed, nil
			}
		}
	}
	return plain, nil
}
