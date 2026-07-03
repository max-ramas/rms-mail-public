//go:build dev || mono

package mail

import (
	"os"
)

// discoverEnvAccounts checks M_EMAIL/M_PASSWORD env vars and returns them as discovered accounts.
// This file is only compiled with the `dev` build tag (not included in production builds).
func discoverEnvAccounts() ([]DiscoveredAccount, error) {
	email := os.Getenv("M_EMAIL")
	password := os.Getenv("M_PASSWORD")
	if email == "" {
		return nil, nil
	}

	account := DiscoveredAccount{Email: email, Password: password}
	return []DiscoveredAccount{account}, nil
}

func init() {
	// Register env-based discovery as a fallback when no other method is configured
	registerDiscoverFunc(discoverEnvAccounts)
}
