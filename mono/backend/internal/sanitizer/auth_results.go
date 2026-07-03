package sanitizer

import (
	"strings"
)

// AuthResult holds parsed authentication results.
type AuthResult struct {
	SpfPass   bool
	DkimPass  bool
	DmarcPass bool
	Trusted   bool // true if the auth result is from a trusted MX
}

// ParseAuthenticationResults parses the Authentication-Results header.
// It validates that the authserv-id matches a trusted domain.
// Returns the FIRST valid result found (scanning top-to-bottom).
func ParseAuthenticationResults(headerValue string, trustedMXDomains []string) *AuthResult {
	lines := strings.Split(headerValue, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: mail.example.com; spf=pass smtp.mailfrom=user@example.com; dkim=pass header.i=@example.com
		// The authserv-id is before the first semicolon
		semiIdx := strings.Index(line, ";")
		if semiIdx < 0 {
			continue
		}

		authservID := strings.TrimSpace(line[:semiIdx])
		rest := line[semiIdx:]

		// Check if authserv-id is trusted
		trusted := false
		for _, domain := range trustedMXDomains {
			if strings.HasSuffix(authservID, domain) || authservID == domain {
				trusted = true
				break
			}
		}

		if !trusted {
			continue // Skip untrusted auth results (likely forged)
		}

		result := &AuthResult{Trusted: true}

		// Parse results like spf=pass, dkim=pass, dmarc=pass
		parts := strings.Fields(rest)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}

			key := strings.ToLower(strings.TrimSpace(kv[0]))
			value := strings.ToLower(strings.TrimSpace(kv[1]))

			switch key {
			case "spf":
				result.SpfPass = value == "pass"
			case "dkim":
				result.DkimPass = value == "pass"
			case "dmarc":
				result.DmarcPass = value == "pass"
			}
		}

		return result // Return first trusted result (top-to-bottom = nearest to MTA)
	}

	return nil
}
