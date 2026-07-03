package mail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rmsmail/internal/edition"
)

// ErrResolutionFailed is returned when mail server settings cannot be determined.
var ErrResolutionFailed = errors.New("resolution failed")

// MailSettings holds the resolved mail server configuration for a given email address.
type MailSettings struct {
	IMAPHost       string `json:"imap_host"`
	IMAPPort       int    `json:"imap_port"`
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	UseSSL         bool   `json:"use_ssl"`
	IMAPEncryption string `json:"imap_encryption"` // "ssl", "starttls"
	SMTPEncryption string `json:"smtp_encryption"` // "ssl", "starttls"
}

// knownProviders maps popular email domains to their known IMAP/SMTP settings.
var knownProviders = map[string]MailSettings{
	"gmail.com": {
		IMAPHost: "imap.gmail.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.gmail.com", SMTPPort: 587, UseSSL: false, SMTPEncryption: "starttls",
	},
	"outlook.com": {
		IMAPHost: "imap-mail.outlook.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp-mail.outlook.com", SMTPPort: 587, UseSSL: false, SMTPEncryption: "starttls",
	},
	"hotmail.com": {
		IMAPHost: "imap-mail.outlook.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp-mail.outlook.com", SMTPPort: 587, UseSSL: false, SMTPEncryption: "starttls",
	},
	"yahoo.com": {
		IMAPHost: "imap.mail.yahoo.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.mail.yahoo.com", SMTPPort: 587, UseSSL: false, SMTPEncryption: "starttls", // 465 SSL or 587 STARTTLS
	},
	"yandex.ru": {
		IMAPHost: "imap.yandex.ru", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.yandex.ru", SMTPPort: 465, UseSSL: true, SMTPEncryption: "ssl",
	},
	"yandex.com": {
		IMAPHost: "imap.yandex.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.yandex.com", SMTPPort: 465, UseSSL: true, SMTPEncryption: "ssl",
	},
	"mail.ru": {
		IMAPHost: "imap.mail.ru", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.mail.ru", SMTPPort: 465, UseSSL: true, SMTPEncryption: "ssl",
	},
	"icloud.com": {
		IMAPHost: "imap.mail.me.com", IMAPPort: 993, IMAPEncryption: "ssl",
		SMTPHost: "smtp.mail.me.com", SMTPPort: 587, UseSSL: false, SMTPEncryption: "starttls",
	},
}

// mxToProvider maps well-known MX host suffixes to provider settings.
var mxToProvider = map[string]string{
	"google.com":             "gmail.com",
	"googlemail.com":         "gmail.com",
	"outlook.com":            "outlook.com",
	"yandex.net":             "yandex.ru",
	"yandex.ru":              "yandex.ru",
	"mail.ru":                "mail.ru",
	"yahoodns.net":           "yahoo.com",
	"protection.outlook.com": "outlook.com",
}

// Resolve dynamically determines mail server settings for the given email address.
func Resolve(ctx context.Context, email string) (*MailSettings, error) {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return nil, ErrResolutionFailed
	}
	username := parts[0]
	domain := strings.ToLower(parts[1])

	// Env var override
	imapHostEnv := replacePlaceholders(os.Getenv("RMS_IMAP_HOST"), domain, username)
	smtpHostEnv := replacePlaceholders(os.Getenv("RMS_SMTP_HOST"), domain, username)
	if imapHostEnv != "" && smtpHostEnv != "" {
		if !isHostAllowed(imapHostEnv) || !isHostAllowed(smtpHostEnv) {
			return nil, ErrResolutionFailed
		}
		return &MailSettings{
			IMAPHost:       imapHostEnv,
			IMAPPort:       993,
			SMTPHost:       smtpHostEnv,
			SMTPPort:       587,
			UseSSL:         false,
			IMAPEncryption: "ssl",
			SMTPEncryption: "starttls",
		}, nil
	}

	if edition.IsMono() {
		// Priority 1: RMS_MAIL_HOST env var (Infrastructure as Code override)
		mailHostEnv := replacePlaceholders(os.Getenv("RMS_MAIL_HOST"), domain, username)
		if mailHostEnv != "" {
			if !isHostAllowed(mailHostEnv) {
				return nil, ErrResolutionFailed
			}
			return &MailSettings{
				IMAPHost:       mailHostEnv,
				IMAPPort:       993,
				SMTPHost:       mailHostEnv,
				SMTPPort:       587,
				UseSSL:         false,
				IMAPEncryption: "ssl",
				SMTPEncryption: "starttls",
			}, nil
		}
		// Priority 2: fall through to full MX/probing resolution below
	}

	// Level 1: Known providers (0ms)
	if settings, ok := knownProviders[domain]; ok {
		return &settings, nil
	}

	// Level 2: MX Records (50-200ms)
	mxHosts := lookupMX(ctx, domain)
	for _, mx := range mxHosts {
		// Check if MX belongs to a known provider
		for mxSuffix, providerDomain := range mxToProvider {
			if strings.HasSuffix(mx, mxSuffix) {
				settings := knownProviders[providerDomain]
				return &settings, nil
			}
		}
	}

	// Trusted MX Domains for Hosting Panels (e.g. 1Panel)
	trustedMX := os.Getenv("TRUSTED_MX_DOMAINS")
	if trustedMX != "" {
		for _, trusted := range strings.Split(trustedMX, ",") {
			trusted = strings.TrimSpace(trusted)
			if trusted != "" {
				for _, mx := range mxHosts {
					if strings.HasSuffix(mx, trusted) || mx == trusted {
						return &MailSettings{
							IMAPHost:       "mail." + domain,
							IMAPPort:       993,
							SMTPHost:       "mail." + domain,
							SMTPPort:       587,
							UseSSL:         false,
							IMAPEncryption: "ssl",
							SMTPEncryption: "starttls",
						}, nil
					}
				}
			}
		}
	}

	// Candidates for probing (Level 3)
	var candidates []string
	if len(mxHosts) > 0 {
		candidates = append(candidates, mxHosts[0]) // Try primary MX host first
	}
	candidates = append(candidates, "mail."+domain, "imap."+domain, domain)

	if edition.IsMono() {
		candidates = append(candidates, "host.docker.internal", "172.17.0.1", "172.18.0.1")
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipv4 := ipnet.IP.To4(); ipv4 != nil {
						gw := net.IPv4(ipv4[0], ipv4[1], ipv4[2], 1).String()
						candidates = append(candidates, gw)
					}
				}
			}
		}
	}

	// Deduplicate candidates
	seen := make(map[string]bool)
	var uniqueCandidates []string
	for _, c := range candidates {
		if !seen[c] && isHostAllowed(c) {
			seen[c] = true
			uniqueCandidates = append(uniqueCandidates, c)
		}
	}

	// Level 3: Port Probing Heuristics (~500-1500ms)
	// We probe IMAP (993, 143) and SMTP (465, 587).
	imapResult := probePorts(uniqueCandidates, []int{993, 143})

	smtpCandidates := uniqueCandidates
	smtpCandidates = append([]string{"smtp." + domain}, smtpCandidates...)

	// Deduplicate smtp candidates
	seenSmtp := make(map[string]bool)
	var uniqueSmtpCandidates []string
	for _, c := range smtpCandidates {
		if !seenSmtp[c] && isHostAllowed(c) {
			seenSmtp[c] = true
			uniqueSmtpCandidates = append(uniqueSmtpCandidates, c)
		}
	}

	smtpResult := probePorts(uniqueSmtpCandidates, []int{465, 587})

	if imapResult == nil || smtpResult == nil {
		return nil, ErrResolutionFailed
	}

	settings := &MailSettings{
		IMAPHost: imapResult.host,
		IMAPPort: imapResult.port,
		SMTPHost: smtpResult.host,
		SMTPPort: smtpResult.port,
	}

	if imapResult.port == 993 {
		settings.IMAPEncryption = "ssl"
	} else {
		settings.IMAPEncryption = "starttls"
	}

	if smtpResult.port == 465 {
		settings.UseSSL = true
		settings.SMTPEncryption = "ssl"
	} else {
		settings.UseSSL = false // STARTTLS for 587
		settings.SMTPEncryption = "starttls"
	}

	return settings, nil
}

func replacePlaceholders(s, domain, username string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "%d", domain)
	s = strings.ReplaceAll(s, "%n", username)
	return s
}

// lookupMX returns MX hosts for the domain, sorted by preference.
func lookupMX(ctx context.Context, domain string) []string {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, network, address)
		},
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	mxs, err := r.LookupMX(ctx, domain)
	if err != nil || len(mxs) == 0 {
		return nil
	}
	var hosts []string
	for _, mx := range mxs {
		hosts = append(hosts, strings.TrimSuffix(mx.Host, "."))
	}
	return hosts
}

type probeResult struct {
	host string
	port int
}

func probePorts(hosts []string, ports []int) *probeResult {
	var wg sync.WaitGroup
	resultCh := make(chan probeResult, len(hosts)*len(ports))
	doneCh := make(chan struct{})

	for _, host := range hosts {
		for _, port := range ports {
			wg.Add(1)
			go func(h string, p int) {
				defer wg.Done()
				addr := net.JoinHostPort(h, strconv.Itoa(p))
				conn, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
				if err == nil {
					conn.Close()
					resultCh <- probeResult{host: h, port: p}
				}
			}(host, port)
		}
	}

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	var results []probeResult
loop:
	for {
		select {
		case r := <-resultCh:
			results = append(results, r)
		case <-doneCh:
			break loop
		}
	}

	if len(results) == 0 {
		return nil
	}

	// Find the highest priority match.
	// Priority is determined by port index (e.g. 993 > 143), then host index.
	for _, port := range ports {
		for _, host := range hosts {
			for _, res := range results {
				if res.host == host && res.port == port {
					return &res
				}
			}
		}
	}

	return nil
}

var (
	myPublicIP     string
	myPublicIPOnce sync.Once
)

func getMyPublicIP() string {
	myPublicIPOnce.Do(func() {
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("https://api.ipify.org")
		if err == nil {
			defer resp.Body.Close()
			ipBytes, _ := io.ReadAll(resp.Body)
			myPublicIP = strings.TrimSpace(string(ipBytes))
		}
	})
	return myPublicIP
}

var MonoProAllowedDomains atomic.Value // expected to store a string

// isHostAllowed returns false if SSRF protection is enabled and the host resolves to a private IP,
// or if Mono edition is running in production and the host resolves to an external public IP.
func isHostAllowed(host string) bool {
	// Always block cloud metadata endpoints regardless of edition.
	if host == "169.254.169.254" || host == "metadata.google.internal" {
		return false
	}

	// Always allow trusted MX domains or explicit mail hosts
	trustedMX := os.Getenv("TRUSTED_MX_DOMAINS")
	if trustedMX != "" {
		for _, trusted := range strings.Split(trustedMX, ",") {
			trusted = strings.TrimSpace(trusted)
			if trusted != "" && (strings.HasSuffix(host, trusted) || host == trusted) {
				return true
			}
		}
	}

	if edition.IsMonoPro() {
		if val, ok := MonoProAllowedDomains.Load().(string); ok && val != "" {
			for _, allowed := range strings.Split(val, ",") {
				allowed = strings.TrimSpace(allowed)
				if allowed != "" && (strings.HasSuffix(host, allowed) || host == allowed) {
					return true
				}
			}
		}
	}

	if rmsHost := os.Getenv("RMS_MAIL_HOST"); rmsHost != "" && host == rmsHost {
		return true
	}

	if edition.IsMono() || edition.IsMonoPro() {
		// Allow any host in development mode or if explicitly allowed
		if os.Getenv("APP_ENV") == "development" || os.Getenv("ALLOW_PUBLIC_IPS") == "true" {
			return true
		}

		// In production, Mono edition MUST strictly use local/private IPs to prevent open-relay abuse
		ips, err := net.LookupIP(host)
		if err != nil {
			return false
		}

		serverIP := getMyPublicIP()

		for _, ip := range ips {
			// If it matches our own public IP, it's definitively local, so allow it!
			if serverIP != "" && ip.String() == serverIP {
				return true
			}

			// If it's NOT a local/private IP, block it!
			if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified() {
				return false
			}
		}
		return true
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS resolution fails, block it for robust SSRF protection
		return false
	}

	// Unified Edition (multi-tenant): block all private IPs to prevent SSRF
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return false
		}
	}
	return true
}

// ValidateManualConfig checks if user-provided hosts are allowed under current SSRF rules.
func ValidateManualConfig(imapHost, smtpHost string) error {
	if !isHostAllowed(imapHost) {
		return fmt.Errorf("IMAP host is not allowed (private IP blocked)")
	}
	if !isHostAllowed(smtpHost) {
		return fmt.Errorf("SMTP host is not allowed (private IP blocked)")
	}
	return nil
}
