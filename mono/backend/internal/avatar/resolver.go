package avatar

import (
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 3 * time.Second}

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

// Resolve returns avatar URL or empty string for fallback to initials
// Cascading: Gravatar -> BIMI -> Google Favicon -> ""
func (r *Resolver) Resolve(ctx context.Context, email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	domain := parts[1]

	// 1. Gravatar (by full email hash)
	if url := checkGravatar(ctx, email); url != "" {
		return url
	}

	// 2. BIMI (by domain DNS TXT record)
	if url := checkBIMI(ctx, domain); url != "" {
		return url
	}

	// 3. Google Favicon (by domain)
	if url := checkGoogleFavicon(domain); url != "" {
		return url
	}

	return ""
}

func checkGravatar(ctx context.Context, email string) string {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(email)))
	url := fmt.Sprintf("https://www.gravatar.com/avatar/%s?d=404&s=80", hash)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return ""
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return url
	}
	return ""
}

var bimiRE = regexp.MustCompile(`l=([^;]+)`)

func checkBIMI(ctx context.Context, domain string) string {
	records, err := net.DefaultResolver.LookupTXT(ctx, "default._bimi."+domain)
	if err != nil {
		return ""
	}
	for _, rec := range records {
		m := bimiRE.FindStringSubmatch(rec)
		if len(m) >= 2 {
			url := strings.TrimSpace(m[1])
			if url == "" {
				continue
			}
			// Validate URL is reachable
			req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
			if err != nil {
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return url
			}
		}
	}
	return ""
}

func checkGoogleFavicon(domain string) string {
	url := fmt.Sprintf("https://www.google.com/s2/favicons?sz=64&domain=%s", domain)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return ""
	}
	// Short timeout for favicon
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return url
	}
	return ""
}
