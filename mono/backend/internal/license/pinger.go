package license

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PublicKeyHex holds the Ed25519 public key used to verify signed update manifests.
var PublicKeyHex = "d9af1d7613798c86a5652b587305e2f902effca8de82d47a3e66bde8b41d324f"

// PingStore is the minimal store interface needed for license/update pings.
type PingStore interface {
	GetSystemSetting(ctx context.Context, key string) (string, error)
	SetSystemSetting(ctx context.Context, key, value string) error
}

// Manager handles periodic license server pings for update notifications and telemetry.
type Manager struct {
	store      PingStore
	serverURL  string
	publicKey  ed25519.PublicKey
	productID  string
	version    string
	httpClient *http.Client
}

// NewManager creates a ping Manager for Mono edition.
func NewManager(store PingStore, productID, version string) *Manager {
	pubKeyHex := os.Getenv("LICENSE_PUBLIC_KEY")
	if pubKeyHex == "" {
		pubKeyHex = PublicKeyHex
	}
	var pubKey ed25519.PublicKey
	if b, err := hex.DecodeString(pubKeyHex); err == nil && len(b) == ed25519.PublicKeySize {
		pubKey = b
	}
	return &Manager{
		store:      store,
		serverURL:  "https://license.rms-ds.com",
		publicKey:  pubKey,
		productID:  productID,
		version:    version,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetOrCreateInstanceUID retrieves or generates a persistent instance UUID.
func (m *Manager) GetOrCreateInstanceUID(ctx context.Context) (string, error) {
	uid, err := m.store.GetSystemSetting(ctx, "instance_uid")
	if err != nil {
		return "", err
	}
	if uid == "" {
		uid = uuid.New().String()
		if err := m.store.SetSystemSetting(ctx, "instance_uid", uid); err != nil {
			return "", err
		}
	}
	return uid, nil
}

// Ping sends a heartbeat to the license server and stores version/update info.
func (m *Manager) Ping(ctx context.Context) error {
	uid, err := m.GetOrCreateInstanceUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get instance uid: %w", err)
	}

	nonce := uuid.New().String()
	channel := normalizeUpdateChannel(os.Getenv("UPDATE_CHANNEL"))

	reqBody, _ := json.Marshal(map[string]string{
		"product_id":   m.productID,
		"instance_uid": uid,
		"version":      m.version,
		"nonce":        nonce,
		"channel":      channel,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", m.serverURL+"/api/v1/installations/ping", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping returned status %d", resp.StatusCode)
	}

	type pingResponse struct {
		LatestVersion string `json:"latest_version"`
		ReleaseNotes  string `json:"release_notes"`
		LicenseStatus string `json:"license_status"`
	}

	var pr pingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if pr.LatestVersion != "" && pr.LatestVersion != "0.0.0" {
		if err := m.store.SetSystemSetting(ctx, "latest_version", pr.LatestVersion); err != nil {
			slog.Info("license: failed to store latest_version", "error", err)
		}
		if pr.ReleaseNotes != "" {
			if err := m.store.SetSystemSetting(ctx, "release_notes", pr.ReleaseNotes); err != nil {
				slog.Info("license: failed to store release_notes", "error", err)
			}
		} else {
			m.store.SetSystemSetting(ctx, "release_notes", "")
		}
	} else {
		m.checkUpdateFromServer(ctx, channel)
	}

	if err := m.recordPingTelemetry(ctx, uid); err != nil {
		slog.Info("license: telemetry timestamp not saved", "error", err)
	}

	slog.Info("license: ping ok", "instance_uid", uid, "license_status", pr.LicenseStatus, "version", m.version)
	return nil
}

func (m *Manager) checkUpdateFromServer(ctx context.Context, channel string) {
	if m.productID == "" || m.serverURL == "" {
		return
	}
	u := fmt.Sprintf("%s/api/v1/software/update-check/%s?platform=linux&arch=amd64&format=tar.gz&channel=%s&current_version=%s",
		strings.TrimRight(m.serverURL, "/"),
		url.PathEscape(m.productID),
		url.QueryEscape(channel),
		url.QueryEscape(m.version),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	var ucResp struct {
		Version           string `json:"version"`
		ReleaseNotes      string `json:"release_notes"`
		Manifest          string `json:"manifest"`
		ManifestSignature string `json:"manifest_signature"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ucResp); err != nil {
		return
	}
	if ucResp.Version == "" || ucResp.Version == "0.0.0" {
		return
	}

	if len(m.publicKey) > 0 && ucResp.Manifest != "" && ucResp.ManifestSignature != "" {
		sigBytes, err := base64.RawURLEncoding.DecodeString(ucResp.ManifestSignature)
		if err != nil || len(sigBytes) != ed25519.SignatureSize || !ed25519.Verify(m.publicKey, []byte(ucResp.Manifest), sigBytes) {
			slog.Warn("license: update-check manifest signature verification failed")
			return
		}
		var manifest struct {
			ProductID string `json:"product_id"`
			Version   string `json:"version"`
		}
		if err := json.Unmarshal([]byte(ucResp.Manifest), &manifest); err != nil {
			return
		}
		if (manifest.ProductID != "" && manifest.ProductID != m.productID) || manifest.Version != ucResp.Version {
			slog.Warn("license: update-check manifest mismatch", "manifest_product", manifest.ProductID, "manifest_version", manifest.Version)
			return
		}
	}

	_ = m.store.SetSystemSetting(ctx, "latest_version", ucResp.Version)
	_ = m.store.SetSystemSetting(ctx, "release_notes", ucResp.ReleaseNotes)
}

func (m *Manager) recordPingTelemetry(ctx context.Context, uid string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return m.store.SetSystemSetting(ctx, "last_successful_ping", now)
}

// StartBackgroundPinger launches startup retry pings followed by periodic daily pings.
func (m *Manager) StartBackgroundPinger(ctx context.Context) {
	if backgroundPingDisabled() {
		slog.Info("license: background ping disabled")
		return
	}

	go func() {
		m.runStartupPings(ctx)
		m.runPeriodicPings(ctx)
	}()
}

func (m *Manager) runStartupPings(ctx context.Context) {
	delays := []time.Duration{0, 5 * time.Second, 15 * time.Second, 45 * time.Second, 2 * time.Minute}
	for i, d := range delays {
		if d > 0 {
			timer := time.NewTimer(d)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}
			timer.Stop()
		}
		if err := m.Ping(ctx); err == nil {
			slog.Info("license: startup ping ok", "attempt", i+1)
			return
		}
	}
	slog.Warn("license: startup ping exhausted retries; next attempt on periodic schedule")
}

func (m *Manager) runPeriodicPings(ctx context.Context) {
	for {
		jitter := time.Duration(rand.Intn(120)) * time.Minute
		sleep := 24*time.Hour + jitter

		timer := time.NewTimer(sleep)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
		timer.Stop()

		if err := m.Ping(ctx); err != nil {
			slog.Info("license: periodic ping failed", "error", err)
		}
	}
}

func backgroundPingDisabled() bool {
	if os.Getenv("DISABLE_LICENSE_PING") != "true" {
		return false
	}
	return os.Getenv("APP_ENV") == "development" || os.Getenv("NODE_ENV") == "development"
}

func normalizeUpdateChannel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "beta", "alpha":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "stable"
	}
}
