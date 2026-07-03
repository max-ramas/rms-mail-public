package mail

import (
	"fmt"
	"os"
	"strings"
)

// DiscoveredAccount represents an email account found in local infrastructure.
type DiscoveredAccount struct {
	Email    string
	Password string // may be empty for skeleton accounts
}

// discoverFunc is a function that discovers local email accounts.
type discoverFunc func() ([]DiscoveredAccount, error)

var discoverFuncs []discoverFunc

// registerDiscoverFunc registers a discovery function (used by dev/test builds).
func registerDiscoverFunc(fn discoverFunc) {
	discoverFuncs = append(discoverFuncs, fn)
}

// DiscoverLocalAccounts scans local infrastructure for email accounts.
// Returns nil, nil if no discovery method is configured (graceful no-op).
func DiscoverLocalAccounts() ([]DiscoveredAccount, error) {
	// Try built-in discovery methods
	if key := os.Getenv("AAPANEL_API_KEY"); key != "" {
		return discoverAAPanel(key, os.Getenv("AAPANEL_URL"))
	}

	if _, err := os.Stat("/etc/postfix/virtual"); err == nil {
		return discoverPostfixVirtual()
	}

	// Try registered discovery functions (dev builds)
	for _, fn := range discoverFuncs {
		accounts, err := fn()
		if err != nil {
			continue
		}
		if len(accounts) > 0 {
			return accounts, nil
		}
	}

	// No discovery method configured — not an error
	return nil, nil
}

func discoverAAPanel(apiKey, url string) ([]DiscoveredAccount, error) {
	// aaPanel API: GET /mail?api_key=...
	// Placeholder — implement when aapanel integration is needed
	return nil, fmt.Errorf("aaPanel discovery not yet implemented")
}

func discoverPostfixVirtual() ([]DiscoveredAccount, error) {
	data, err := os.ReadFile("/etc/postfix/virtual")
	if err != nil {
		return nil, err
	}

	var accounts []DiscoveredAccount
	seen := make(map[string]bool)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Format: "user@domain.com target"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			email := parts[0]
			if !seen[email] && strings.Contains(email, "@") {
				accounts = append(accounts, DiscoveredAccount{Email: email})
				seen[email] = true
			}
		}
	}

	return accounts, nil
}
