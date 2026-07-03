package api

import (
	"bytes"
	"container/list"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rmsmail/internal/crypto"
	"rmsmail/internal/edition"
)

var privateCIDRs = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"127.0.0.0/8", "169.254.0.0/16",
}

var hmacKey []byte

func InitCamoKey() { initCamoKey() }

// camoKeyFromEncryption derives a stable Camo HMAC key from ENCRYPTION_KEYS / ENCRYPTION_KEY.
// Mono does not require a separate CAMO_HMAC_KEY env var.
func camoKeyFromEncryption() []byte {
	primary := crypto.GetPrimaryEncryptionKey()
	if primary == "" {
		return nil
	}
	return crypto.DeriveKeyWithDomain([]byte(primary), "camo_hmac")
}

func initCamoKey() {
	if hmacKey != nil {
		return
	}
	key := os.Getenv("CAMO_HMAC_KEY")
	if key == "" {
		if edition.IsMono() {
			if derived := camoKeyFromEncryption(); len(derived) > 0 {
				hmacKey = derived
				slog.Info("CAMO_HMAC_KEY not set — using key derived from ENCRYPTION_KEYS (Mono)")
				camoCache.startCleanup(5 * time.Minute)
				return
			}
		}
		if os.Getenv("APP_ENV") != "development" {
			slog.Error("CAMO_HMAC_KEY environment variable is required in production. Generate: openssl rand -hex 32")
			os.Exit(1)
		}
		// Dev only: auto-generate random key (regenerates on restart).
		hmacKey = make([]byte, 32)
		_, _ = rand.Read(hmacKey)
		slog.Info("CAMO_HMAC_KEY not set — using auto-generated key (development only)")
		camoCache.startCleanup(5 * time.Minute)
		return
	}
	hmacKey = []byte(key)
	camoCache.startCleanup(5 * time.Minute)
}

const camoCacheMaxSize = 10000

// camoLRU is a simple LRU cache backed by container/list.
type camoLRU struct {
	mu      sync.Mutex
	ll      *list.List
	cache   map[string]*list.Element
	maxSize int
}

type camoEntry struct {
	key   string
	value string
}

var camoCache = &camoLRU{
	ll:      list.New(),
	cache:   make(map[string]*list.Element),
	maxSize: camoCacheMaxSize,
}

var startCleanupOnce sync.Once

func (c *camoLRU) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*camoEntry).value, true
	}
	return "", false
}

func (c *camoLRU) put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*camoEntry).value = value
		return
	}
	entry := &camoEntry{key: key, value: value}
	elem := c.ll.PushFront(entry)
	c.cache[key] = elem
	if c.ll.Len() > c.maxSize {
		c.evictOldestLocked()
	}
}

func (c *camoLRU) evictOldestLocked() {
	elem := c.ll.Back()
	if elem != nil {
		entry := elem.Value.(*camoEntry)
		c.ll.Remove(elem)
		delete(c.cache, entry.key)
	}
}

// startCleanup runs a periodic goroutine that trims the cache down to 80% of
// maxSize, providing headroom for bursts without growing unboundedly.
func (c *camoLRU) startCleanup(interval time.Duration) {
	startCleanupOnce.Do(func() {
		go func() {
			for {
				time.Sleep(interval)
				c.mu.Lock()
				for c.ll.Len() > c.maxSize*4/5 {
					c.evictOldestLocked()
				}
				c.mu.Unlock()
			}
		}()
	})
}

// sanitizeImageURL repairs URLs from HTML emails. Some marketing platforms emit
// SOH (0x01) instead of '=' in query strings; net/url rejects other C0 controls.
func sanitizeImageURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if strings.Contains(s, "\x01") {
		s = strings.ReplaceAll(s, "\x01", "=")
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 || c == '\t' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func camoSign(url string) string {
	url = sanitizeImageURL(url)
	if cached, ok := camoCache.get(url); ok {
		return cached
	}
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(url))
	result := hex.EncodeToString(mac.Sum(nil))
	camoCache.put(url, result)
	return result
}

func camoVerify(url, sig string) bool {
	if sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(url))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

func isPrivateIP(addr string) bool {
	host := addr
	if parsed, err := url.Parse(addr); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isPrivateIPAddr(ip)
}

func isPrivateIPAddr(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	for _, cidr := range privateCIDRs {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet != nil && subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func camoCacheDir() string {
	root := os.Getenv("STORAGE_ROOT")
	if root == "" {
		root = "storage"
	}
	return filepath.Join(root, "camo")
}

// resolvePublicIP prefers IPv4 (Docker hosts often lack working IPv6 egress).
func resolvePublicIP(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup %q: %w", host, err)
	}
	var v4, v6 net.IP
	for _, ip := range ips {
		if isPrivateIPAddr(ip) {
			continue
		}
		if ip.To4() != nil {
			if v4 == nil {
				v4 = ip
			}
			continue
		}
		if v6 == nil {
			v6 = ip
		}
	}
	if v4 != nil {
		return v4, nil
	}
	if v6 != nil {
		return v6, nil
	}
	return nil, fmt.Errorf("no public IP for %q", host)
}

func createCamoTransport(pinnedIP net.IP, hostname, scheme string) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	defaultPort := "443"
	if scheme == "http" {
		defaultPort = "80"
	}
	var tlsConfig *tls.Config
	if scheme == "https" {
		tlsConfig = &tls.Config{ServerName: hostname}
	}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(addr)
			if port == "" {
				port = defaultPort
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(pinnedIP.String(), port))
		},
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: 5 * time.Second,
	}
}

const camoUserAgent = "Mozilla/5.0 (compatible; RMS-Mail-Camo/1.0; +https://rms-ds.com)"

// 1×1 transparent GIF — returned when upstream image fetch fails (avoids 502 in <img> tags).
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00,
	0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00,
	0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02,
	0x44, 0x01, 0x00, 0x3b,
}

func isRedirectStatus(code int) bool {
	return code == http.StatusMovedPermanently ||
		code == http.StatusFound ||
		code == http.StatusSeeOther ||
		code == http.StatusTemporaryRedirect ||
		code == http.StatusPermanentRedirect
}

func resolveRedirect(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(location))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func fetchOnce(ctx context.Context, imageURL string) (data []byte, contentType string, redirectTo string, err error) {
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, "", "", fmt.Errorf("unsupported scheme %q", scheme)
	}
	host := parsedURL.Hostname()
	pinnedIP, err := resolvePublicIP(host)
	if err != nil {
		return nil, "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", camoUserAgent)

	transport := createCamoTransport(pinnedIP, host, scheme)
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("fetch %s: %w", host, err)
	}
	defer resp.Body.Close()

	if isRedirectStatus(resp.StatusCode) {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil, "", "", fmt.Errorf("fetch %s: redirect without Location", host)
		}
		next, err := resolveRedirect(imageURL, loc)
		if err != nil {
			return nil, "", "", err
		}
		return nil, "", next, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("fetch %s: %s", host, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, "", "", err
	}
	return body, detectContentType(body), "", nil
}

func fetchAndCache(ctx context.Context, imageURL string) ([]byte, string, error) {
	cacheDir := camoCacheDir()
	cacheKey := camoSign(imageURL)
	cachePath := filepath.Join(cacheDir, cacheKey[:2], cacheKey)

	if data, err := os.ReadFile(cachePath); err == nil {
		contentType := detectContentType(data)
		return data, contentType, nil
	}

	current := imageURL
	for hop := 0; hop < 5; hop++ {
		data, contentType, redirectTo, err := fetchOnce(ctx, current)
		if err != nil {
			return nil, "", err
		}
		if redirectTo != "" {
			current = redirectTo
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(cachePath, data, 0640); err != nil {
			return nil, "", err
		}
		return data, contentType, nil
	}
	return nil, "", fmt.Errorf("too many redirects for %s", hostFromURL(imageURL))
}

// detectContentType detects image content type from raw bytes.
// SVG is blocked (returns application/octet-stream) because SVGs can contain
// embedded scripts and XML external entities (XXE) that pose XSS and SSRF risks.
func detectContentType(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return "image/gif"
	}
	if data[0] == 'W' && data[1] == 'E' && data[2] == 'B' && data[3] == 'P' {
		return "image/webp"
	}
	if data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}
	// Block SVG: may contain scripts and XXE
	if (data[0] == '<' && data[1] == 's' && data[2] == 'v' && data[3] == 'g') ||
		bytes.Contains(data[:min(len(data), 512)], []byte("<svg")) ||
		bytes.Contains(data[:min(len(data), 512)], []byte("<SVG")) ||
		bytes.Contains(data[:min(len(data), 128)], []byte("<?xml")) {
		return "application/octet-stream"
	}
	return http.DetectContentType(data)
}

func (h *Handler) MediaProxy(w http.ResponseWriter, r *http.Request) {
	initCamoKey()

	imageURL := sanitizeImageURL(r.URL.Query().Get("url"))
	sig := r.URL.Query().Get("sig")

	if imageURL == "" {
		WriteJSONError(w, http.StatusBadRequest, "url required")
		return
	}

	if !camoVerify(imageURL, sig) {
		WriteJSONError(w, http.StatusForbidden, "invalid signature")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	data, contentType, err := fetchAndCache(ctx, imageURL)
	if err != nil {
		slog.Info("Camo fetch error", "host", hostFromURL(imageURL), "error", err)
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Camo-Error", "fetch-failed")
		w.Write(transparentGIF)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Camo-Origin", camoSign(imageURL))
	w.Write(data)
}

func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Hostname()
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
