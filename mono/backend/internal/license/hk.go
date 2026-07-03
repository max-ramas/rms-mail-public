package license

import (
	"crypto/sha256"
	"os"
)

// HK is the HMAC secret key derived from ENCRYPTION_KEY/ENCRYPTION_KEYS for system_settings integrity.
var HK []byte

// InitHK derives the HMAC secret from ENCRYPTION_KEY. Must be called before any DB operations that read license settings.
func InitHK() {
	encKey := os.Getenv("ENCRYPTION_KEYS")
	if encKey == "" {
		encKey = os.Getenv("ENCRYPTION_KEY")
	}
	if encKey != "" {
		h := sha256.Sum256([]byte(encKey + "_license_integrity_salt"))
		HK = h[:]
	}
}
