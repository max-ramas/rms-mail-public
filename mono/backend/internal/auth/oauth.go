package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type GoogleOAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	mu           sync.Mutex
	state        map[string]string
	sf           singleflight.Group
}

func NewGoogleOAuthProvider(clientID, clientSecret, redirectURI string) *GoogleOAuthProvider {
	p := &GoogleOAuthProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://mail.google.com/",
		},
		state: make(map[string]string),
	}
	go p.cleanupState()
	return p
}

func (p *GoogleOAuthProvider) cleanupState() {
	for {
		time.Sleep(10 * time.Minute)
		p.mu.Lock()
		for k := range p.state {
			delete(p.state, k)
		}
		p.mu.Unlock()
	}
}

func (p *GoogleOAuthProvider) VerifyState(state string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.state[state]
	if ok {
		delete(p.state, state)
	}
	return ok
}

type GoogleTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       int64  `json:"expiry"`
	IDToken      string `json:"id_token"`
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *GoogleOAuthProvider) GetAuthURL(state string) string {
	return p.GetAuthURLWithRedirect(state, p.RedirectURI)
}

func (p *GoogleOAuthProvider) GetAuthURLWithRedirect(state, redirectURI string) string {
	p.mu.Lock()
	p.state[state] = state
	p.mu.Unlock()

	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(p.Scopes, " "))
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

func (p *GoogleOAuthProvider) ExchangeCode(ctx context.Context, code string) (*Tokens, error) {
	return p.ExchangeCodeWithRedirect(ctx, code, p.RedirectURI)
}

func (p *GoogleOAuthProvider) ExchangeCodeWithRedirect(ctx context.Context, code, redirectURI string) (*Tokens, error) {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("client_secret", p.ClientSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", resp.Status)
	}

	var googleTokens GoogleTokens
	if err := json.NewDecoder(resp.Body).Decode(&googleTokens); err != nil {
		return nil, err
	}

	return &Tokens{
		AccessToken:  googleTokens.AccessToken,
		RefreshToken: googleTokens.RefreshToken,
		ExpiresIn:    int(googleTokens.Expiry),
	}, nil
}

func (p *GoogleOAuthProvider) RefreshTokens(ctx context.Context, refreshToken string) (*Tokens, error) {
	result, err, _ := p.sf.Do("refresh:"+refreshToken, func() (interface{}, error) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return p.executeGoogleRefresh(refreshCtx, refreshToken)
	})
	if err != nil {
		return nil, err
	}
	return result.(*Tokens), nil
}

func (p *GoogleOAuthProvider) executeGoogleRefresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("client_secret", p.ClientSecret)
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return nil, fmt.Errorf("refresh failed: %s: %s", resp.Status, detail)
		}
		return nil, fmt.Errorf("refresh failed: %s", resp.Status)
	}

	var tokens Tokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

func (p *GoogleOAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info failed: %s", resp.Status)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (p *GoogleOAuthProvider) RevokeToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/revoke", strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

type OutlookOAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Tenant       string
	mu           sync.Mutex
	state        map[string]string
	sf           singleflight.Group
}

func NewOutlookOAuthProvider(clientID, clientSecret, redirectURI, tenant string) *OutlookOAuthProvider {
	if tenant == "" {
		tenant = "common"
	}
	p := &OutlookOAuthProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Tenant:       tenant,
		state:        make(map[string]string),
	}
	go p.cleanupState()
	return p
}

func (p *OutlookOAuthProvider) cleanupState() {
	for {
		time.Sleep(10 * time.Minute)
		p.mu.Lock()
		for k := range p.state {
			delete(p.state, k)
		}
		p.mu.Unlock()
	}
}

func (p *OutlookOAuthProvider) VerifyState(state string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.state[state]
	if ok {
		delete(p.state, state)
	}
	return ok
}

func (p *OutlookOAuthProvider) GetAuthURL(state string) string {
	return p.GetAuthURLWithRedirect(state, p.RedirectURI)
}

func (p *OutlookOAuthProvider) GetAuthURLWithRedirect(state, redirectURI string) string {
	p.mu.Lock()
	p.state[state] = state
	p.mu.Unlock()

	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "https://graph.microsoft.com/Mail.Read https://graph.microsoft.com/Mail.ReadWrite https://graph.microsoft.com/User.Read")
	params.Set("state", state)

	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s", p.Tenant, params.Encode())
}

func (p *OutlookOAuthProvider) ExchangeCode(ctx context.Context, code string) (*Tokens, error) {
	return p.ExchangeCodeWithRedirect(ctx, code, p.RedirectURI)
}

func (p *OutlookOAuthProvider) ExchangeCodeWithRedirect(ctx context.Context, code, redirectURI string) (*Tokens, error) {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("client_secret", p.ClientSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.Tenant), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", resp.Status)
	}

	var outTokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&outTokens); err != nil {
		return nil, err
	}

	return &Tokens{
		AccessToken:  outTokens.AccessToken,
		RefreshToken: outTokens.RefreshToken,
		ExpiresIn:    int(outTokens.ExpiresIn),
	}, nil
}

type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (p *OutlookOAuthProvider) RefreshTokens(ctx context.Context, refreshToken string) (*OAuthTokens, error) {
	result, err, _ := p.sf.Do("refresh:"+refreshToken, func() (interface{}, error) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return p.executeOutlookRefresh(refreshCtx, refreshToken)
	})
	if err != nil {
		return nil, err
	}
	return result.(*OAuthTokens), nil
}

func (p *OutlookOAuthProvider) executeOutlookRefresh(ctx context.Context, refreshToken string) (*OAuthTokens, error) {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("client_secret", p.ClientSecret)
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")
	params.Set("scope", "https://graph.microsoft.com/Mail.Read https://graph.microsoft.com/Mail.ReadWrite https://graph.microsoft.com/User.Read")

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.Tenant), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return nil, fmt.Errorf("refresh failed: %s: %s", resp.Status, detail)
		}
		return nil, fmt.Errorf("refresh failed: %s", resp.Status)
	}

	var tokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

func (p *OutlookOAuthProvider) RevokeToken(ctx context.Context, token string) error {
	return nil
}

type MicrosoftUserInfo struct {
	ID                string `json:"id"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
	DisplayName       string `json:"displayName"`
	GivenName         string `json:"givenName"`
	Surname           string `json:"surname"`
	Email             string `json:"-"`
}

func (p *OutlookOAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (*MicrosoftUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("microsoft graph failed: %s", resp.Status)
	}

	var info MicrosoftUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	if info.Mail != "" {
		info.Email = info.Mail
	} else {
		info.Email = info.UserPrincipalName
	}
	return &info, nil
}

type StoredCredentials struct {
	AccountID     string    `json:"account_id"`
	Provider      string    `json:"provider"` // gmail, outlook
	Email         string    `json:"email"`
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	TokenExpiry   time.Time `json:"token_expiry"`
	EncryptedKeys string    `json:"encrypted_keys"`
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type OAuthManager struct {
	fetchSetting func(ctx context.Context, key string) (string, error)
	mu           sync.RWMutex

	googleClientID     string
	googleClientSecret string
	google             *GoogleOAuthProvider

	microsoftClientID     string
	microsoftClientSecret string
	microsoft             *OutlookOAuthProvider
}

func NewOAuthManager(fetchSetting func(ctx context.Context, key string) (string, error)) *OAuthManager {
	return &OAuthManager{
		fetchSetting: fetchSetting,
	}
}

func (m *OAuthManager) getSettingWithFallback(ctx context.Context, key, envKey string) string {
	if m.fetchSetting != nil {
		if val, err := m.fetchSetting(ctx, key); err == nil && val != "" {
			return val
		}
	}
	return os.Getenv(envKey)
}

func (m *OAuthManager) GetGoogleProvider(ctx context.Context) (*GoogleOAuthProvider, error) {
	clientID := m.getSettingWithFallback(ctx, "oauth_google_client_id", "GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := m.getSettingWithFallback(ctx, "oauth_google_client_secret", "GOOGLE_OAUTH_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, nil // Not configured
	}

	redirectURI := m.getSettingWithFallback(ctx, "oauth_google_redirect_uri", "GOOGLE_OAUTH_REDIRECT_URI")

	m.mu.Lock()
	defer m.mu.Unlock()

	// If keys changed or it's the first time, recreate the provider
	if m.google == nil || m.googleClientID != clientID || m.googleClientSecret != clientSecret {
		m.google = NewGoogleOAuthProvider(clientID, clientSecret, redirectURI)
		m.googleClientID = clientID
		m.googleClientSecret = clientSecret
	}

	return m.google, nil
}

func (m *OAuthManager) GetMicrosoftProvider(ctx context.Context) (*OutlookOAuthProvider, error) {
	clientID := m.getSettingWithFallback(ctx, "oauth_microsoft_client_id", "MICROSOFT_OAUTH_CLIENT_ID")
	clientSecret := m.getSettingWithFallback(ctx, "oauth_microsoft_client_secret", "MICROSOFT_OAUTH_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, nil // Not configured
	}

	redirectURI := m.getSettingWithFallback(ctx, "oauth_microsoft_redirect_uri", "MICROSOFT_OAUTH_REDIRECT_URI")
	tenant := m.getSettingWithFallback(ctx, "oauth_microsoft_tenant", "MICROSOFT_TENANT")

	m.mu.Lock()
	defer m.mu.Unlock()

	// If keys changed or it's the first time, recreate the provider
	if m.microsoft == nil || m.microsoftClientID != clientID || m.microsoftClientSecret != clientSecret {
		m.microsoft = NewOutlookOAuthProvider(clientID, clientSecret, redirectURI, tenant)
		m.microsoftClientID = clientID
		m.microsoftClientSecret = clientSecret
	}

	return m.microsoft, nil
}
