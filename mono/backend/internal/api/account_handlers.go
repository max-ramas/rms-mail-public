package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"rmsmail/internal/api/middleware"
	"rmsmail/internal/auth"
	"rmsmail/internal/edition"
	"rmsmail/internal/mail"
	"rmsmail/internal/models"

	"github.com/emersion/go-imap/v2/imapclient"
)

func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	// Short Redis cache (10s) — per-user key for Mono/MonoPro.
	if edition.IsMono() || edition.IsMonoPro() {
	}
	if false {

	}

	accounts, err := h.Store.GetAccounts(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	if edition.IsMono() || edition.IsMonoPro() {
		userEmail := middleware.GetUserIDFromContext(r.Context())
		var filtered []models.Account
		for _, acc := range accounts {
			if strings.EqualFold(acc.Email, userEmail) {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}

	// Attach unread counts (all folders)
	unreadCounts, err := h.Store.GetUnreadCountByAccount(r.Context())
	if err == nil {
		for i := range accounts {
			if count, ok := unreadCounts[accounts[i].ID]; ok {
				accounts[i].UnreadCount = count
			}
		}
	}

	// Attach inbox-only unread counts (for unified "Все входящие" badge)
	inboxCounts, err := h.Store.GetUnreadInboxCountByAccount(r.Context())
	if err == nil {
		for i := range accounts {
			if count, ok := inboxCounts[accounts[i].ID]; ok {
				accounts[i].UnreadInbox = count
			}
		}
	}

	// Compute is_locked live (backend enforcement, not from DB flag)
	if false {

	}

	// Inject is_sync_paused from cache into each account response
	type accountResponse struct {
		models.Account
		IsSyncPaused bool `json:"is_sync_paused"`
	}
	result := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		result[i].Account = a
		result[i].IsSyncPaused = h.IsAccountSyncPaused(r.Context(), a.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(result)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)

	if false {

	}
}

func (h *Handler) HandleAccount(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if r.Method == http.MethodPost && len(pathParts) == 2 {
		h.createAccount(w, r)
		return
	}

	if len(pathParts) < 3 {
		WriteJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check for /api/accounts/{id}/reset-sync
	if len(pathParts) == 4 && pathParts[3] == "reset-sync" {
		accountID := pathParts[2]
		if r.Method == http.MethodPost {
			h.resetAccountSync(w, r, accountID)
		} else {
			WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Check for /api/accounts/{id}/pause-sync
	if len(pathParts) == 4 && pathParts[3] == "pause-sync" {
		accountID := pathParts[2]
		if r.Method == http.MethodPost {
			h.HandlePauseSync(w, r, accountID)
		} else {
			WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Check for /api/accounts/{id}/resume-sync
	if len(pathParts) == 4 && pathParts[3] == "resume-sync" {
		accountID := pathParts[2]
		if r.Method == http.MethodPost {
			h.HandleResumeSync(w, r, accountID)
		} else {
			WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Check for /api/accounts/{id}/check-now
	if len(pathParts) == 4 && pathParts[3] == "check-now" {
		accountID := pathParts[2]
		if r.Method == http.MethodPost {
			h.checkNow(w, r, accountID)
		} else {
			WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Check for /api/accounts/{id}/smart-categories
	if len(pathParts) == 4 && pathParts[3] == "smart-categories" {
		accountID := pathParts[2]
		if r.Method == http.MethodPatch {
			h.UpdateSmartCategories(w, r, accountID)
		} else {
			WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Check for /api/accounts/{id}/folders
	if len(pathParts) >= 4 && pathParts[3] == "folders" {
		accountID := pathParts[2]
		if len(pathParts) == 4 && r.Method == http.MethodPost {
			h.createFolder(w, r, accountID)
			return
		}
		if len(pathParts) == 5 {
			folderID := pathParts[4]
			if r.Method == http.MethodPatch {
				h.renameFolder(w, r, accountID, folderID)
				return
			} else if r.Method == http.MethodDelete {
				h.deleteFolder(w, r, accountID, folderID)
				return
			}
		}
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed or invalid path")
		return
	}

	accountID := pathParts[len(pathParts)-1]

	switch r.Method {
	case http.MethodGet:
		h.getAccount(w, r, accountID)
	case http.MethodPut:
		h.updateAccount(w, r, accountID)
	case http.MethodDelete:
		h.deleteAccount(w, r, accountID)
	default:
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) resetAccountSync(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	// Atomic: kill worker → reset DB → start fresh worker. No races.
	if h.SyncManager != nil {
		h.SyncManager.ResyncAccount(r.Context(), accountID)
	} else {
		if err := h.Store.ResetAccountSync(r.Context(), accountID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
	}
	h.InvalidateEmailCache(r.Context(), accountID)
	if err := h.Store.RefreshUnreadCounts(r.Context()); err != nil {
		slog.Info("resetAccountSync: RefreshUnreadCounts failed", "error", err)
	}
	h.publishEvent(r.Context(), "emails_bulk_updated", fmt.Sprintf(`{"affected":0,"action":"reset_sync","account_id":"%s"}`, accountID))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandlePauseSync pauses sync for an account.
func (h *Handler) HandlePauseSync(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	h.SyncManager.PauseAccount(accountID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// HandleResumeSync resumes sync for an account.
func (h *Handler) HandleResumeSync(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	h.SyncManager.ResumeAccount(accountID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (h *Handler) checkNow(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	if h.PriorityChecker == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "priority check not available")
		return
	}

	acc, err := h.Store.GetAccountCredentials(r.Context(), accountID)
	if err != nil || acc == nil {
		WriteJSONError(w, http.StatusNotFound, "account not found")
		return
	}

	// Run in background so the HTTP response returns immediately.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := h.PriorityChecker.CheckAccount(ctx, *acc); err != nil {
			slog.Info("PriorityCheck failed", "account", acc.Email, "error", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	account, err := h.Store.GetAccount(r.Context(), accountID)
	if err != nil || account == nil {
		WriteJSONError(w, http.StatusNotFound, "account not found")
		return
	}

	// Compute is_locked live (position-based, not DB flag)
	if false {

	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func (h *Handler) createAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email          string `json:"email"`
		Name           string `json:"name"`
		Provider       string `json:"provider"`
		IMAPHost       string `json:"imap_host"`
		IMAPPort       int    `json:"imap_port"`
		IMAPSSL        bool   `json:"imap_ssl"`
		IMAPEncryption string `json:"imap_encryption"`
		SMTPHost       string `json:"smtp_host"`
		SMTPPort       int    `json:"smtp_port"`
		SMTPSSL        bool   `json:"smtp_ssl"`
		SMTPEncryption string `json:"smtp_encryption"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		AIConfig       string `json:"ai_provider_config"`
		Signature      string `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	if req.Provider == "custom" && req.IMAPHost == "" && req.SMTPHost == "" {
		resolved, err := mail.Resolve(r.Context(), req.Email)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "resolution_failed",
				"code":  "ERROR_RESOLUTION_FAILED",
			})
			return
		}
		req.IMAPHost = resolved.IMAPHost
		req.IMAPPort = resolved.IMAPPort
		req.IMAPSSL = resolved.UseSSL
		req.IMAPEncryption = resolved.IMAPEncryption
		req.SMTPHost = resolved.SMTPHost
		req.SMTPPort = resolved.SMTPPort
		req.SMTPSSL = resolved.UseSSL
		if req.SMTPSSL {
			req.SMTPEncryption = "ssl"
		} else {
			req.SMTPEncryption = "starttls"
		}
	} else if req.Provider == "custom" {
		if err := mail.ValidateManualConfig(req.IMAPHost, req.SMTPHost); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "manual_resolution_failed",
				"code":  "ERROR_RESOLUTION_FAILED",
				"msg":   err.Error(),
			})
			return
		}
	}

	h.creationMu.Lock()
	defer h.creationMu.Unlock()

	if false {

	}

	account, err := h.Store.CreateAccount(r.Context(), req.Email, req.Name, req.Provider, req.IMAPHost, req.IMAPPort, req.IMAPSSL, req.IMAPEncryption, req.SMTPHost, req.SMTPPort, req.SMTPSSL, req.SMTPEncryption, req.Username, req.Password, req.AIConfig, req.Signature)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if h.SyncManager != nil {
		h.SyncManager.TriggerRefresh()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(account)
}

func (h *Handler) testAccountConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImapHost  string `json:"imap_host"`
		ImapPort  int    `json:"imap_port"`
		ImapSSL   bool   `json:"imap_ssl"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	password := req.Password
	if password == "" && req.AccountID != "" {
		// Use GetAccountCredentials (not GetAccount) — it returns decrypted passwords
		acc, err := h.Store.GetAccountCredentials(r.Context(), req.AccountID)
		if err == nil && acc != nil {
			password = acc.PasswordEncrypted
		}
	}
	if password == "" {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "No password provided"})
		return
	}

	addr := fmt.Sprintf("%s:%d", req.ImapHost, req.ImapPort)

	// Simple connect + login test
	var client *imapclient.Client
	var dialErr error

	opts := &imapclient.Options{
		Dialer:    &net.Dialer{Timeout: 5 * time.Second},
		TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
	}
	if req.ImapSSL {
		client, dialErr = imapclient.DialTLS(addr, opts)
	} else {
		client, dialErr = imapclient.DialInsecure(addr, opts)
	}
	if dialErr != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Connection failed: " + dialErr.Error()})
		return
	}
	defer client.Close()

	if err := client.Login(req.Username, password).Wait(); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Login failed: " + err.Error()})
		return
	}

	// Test folder access
	if _, selErr := client.Select("INBOX", nil).Wait(); selErr != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "INBOX access failed: " + selErr.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) HandleTestAccountConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Separate rate limiter for test-connection to prevent brute-force
	if h.TestConnLimiter != nil {
		h.TestConnLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.testAccountConnection(w, r)
		})).ServeHTTP(w, r)
		return
	}
	h.testAccountConnection(w, r)
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	// Live lock check (position-based, not DB flag)
	if false {

	}
	if err := h.Store.DeleteAccount(r.Context(), accountID); err != nil {
		WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

func (h *Handler) updateAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	// Live lock check (position-based, not DB flag)
	if false {

	}
	var req struct {
		Email          string `json:"email"`
		Name           string `json:"name"`
		Provider       string `json:"provider"`
		IMAPHost       string `json:"imap_host"`
		IMAPPort       int    `json:"imap_port"`
		IMAPSSL        bool   `json:"imap_ssl"`
		IMAPEncryption string `json:"imap_encryption"`
		SMTPHost       string `json:"smtp_host"`
		SMTPPort       int    `json:"smtp_port"`
		SMTPSSL        bool   `json:"smtp_ssl"`
		SMTPEncryption string `json:"smtp_encryption"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		AIConfig       string `json:"ai_provider_config"`
		Signature      string `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	account, err := h.Store.UpdateAccount(r.Context(), accountID, req.Email, req.Name, req.Provider, req.IMAPHost, req.IMAPPort, req.IMAPSSL, req.IMAPEncryption, req.SMTPHost, req.SMTPPort, req.SMTPSSL, req.SMTPEncryption, req.Username, req.Password, req.AIConfig, req.Signature)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if h.SyncManager != nil {
		h.SyncManager.TriggerRefresh()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func (h *Handler) UpdateSmartCategories(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	var req struct {
		SmartCategories bool `json:"smart_categories"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	if err := h.Store.UpdateSmartCategories(r.Context(), accountID, req.SmartCategories); err != nil {
		WriteInternalError(w, r, err)
		return
	}

	// Immediately recompute smart_category flags so the UI reflects the toggle
	// without waiting for the periodic RefreshUnreadCounts (every 30s).
	if err := h.Store.RefreshUnreadCounts(r.Context()); err != nil {
		slog.Info("UpdateSmartCategories: RefreshUnreadCounts failed", "error", err)
	}

	// Trigger sync to fetch missing emails if disabling, or just to reflect changes
	if h.SyncManager != nil {
		h.SyncManager.TriggerRefresh()
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetOAuthURL(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		WriteJSONError(w, http.StatusBadRequest, "provider required")
		return
	}

	if h.OAuth == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "OAuth not configured")
		return
	}

	// Build redirect URI dynamically from the request so it always matches
	// the actual server URL (fixes redirect_uri_mismatch)
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/callback", scheme, host)

	var authURL string
	switch provider {
	case "google":
		google, err := h.OAuth.GetGoogleProvider(r.Context())
		if err != nil || google == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "Google OAuth not configured")
			return
		}
		authURL = google.GetAuthURLWithRedirect(provider, redirectURI)
	case "microsoft":
		microsoft, err := h.OAuth.GetMicrosoftProvider(r.Context())
		if err != nil || microsoft == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "Microsoft OAuth not configured")
			return
		}
		authURL = microsoft.GetAuthURLWithRedirect(provider, redirectURI)
	default:
		WriteJSONError(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	slog.Info("OAuth: generated auth URL", "redirectURI", redirectURI)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": authURL,
	})
}

func (h *Handler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	slog.Debug("OAuth callback called")
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = r.URL.Query().Get("state")
	}
	code := r.URL.Query().Get("code")

	slog.Info("OAuth callback", "provider", provider, "code_len", len(code))

	if provider == "" || code == "" {
		WriteJSONError(w, http.StatusBadRequest, "missing parameters")
		return
	}

	if h.OAuth == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "OAuth not configured")
		return
	}

	// Build the same redirect URI used for the auth request
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/callback", scheme, host)

	var tokens *auth.Tokens
	var err error

	switch provider {
	case "google":
		google, errProv := h.OAuth.GetGoogleProvider(r.Context())
		if errProv != nil || google == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "Google OAuth not configured")
			return
		}
		stateParam := r.URL.Query().Get("state")
		if stateParam != "" && !google.VerifyState(stateParam) {
			WriteJSONError(w, http.StatusUnauthorized, "invalid state parameter")
			return
		}
		tokens, err = google.ExchangeCodeWithRedirect(r.Context(), code, redirectURI)
	case "microsoft":
		microsoft, errProv := h.OAuth.GetMicrosoftProvider(r.Context())
		if errProv != nil || microsoft == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "Microsoft OAuth not configured")
			return
		}
		stateParam := r.URL.Query().Get("state")
		if stateParam != "" && !microsoft.VerifyState(stateParam) {
			WriteJSONError(w, http.StatusUnauthorized, "invalid state parameter")
			return
		}
		tokens, err = microsoft.ExchangeCodeWithRedirect(r.Context(), code, redirectURI)
	default:
		WriteJSONError(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	var userEmail string
	switch provider {
	case "google":
		userEmail = h.getUserEmailWithRetry(r.Context(), provider, tokens.AccessToken, 3)
	case "microsoft":
		userEmail = h.getUserEmailWithRetry(r.Context(), provider, tokens.AccessToken, 3)
	}

	redirectURL := os.Getenv("FRONTEND_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:3500"
	}

	slog.Info(">>> userEmail after GetUserInfo", "userEmail", userEmail)

	if userEmail != "" {
		imapHost, imapPort := "imap.gmail.com", 993
		smtpHost, smtpPort := "smtp.gmail.com", 465
		if provider == "microsoft" {
			imapHost, smtpHost = "outlook.office365.com", "smtp.office365.com"
			smtpPort = 587
		}

		acc, err := h.Store.GetAccountCredentialsByEmail(r.Context(), userEmail)
		if err == nil && acc != nil {
			slog.Info("Existing OAuth account found, updating details", "userEmail", userEmail)
			err = h.Store.UpdateAccountOAuth(r.Context(), acc.ID, provider, imapHost, imapPort, true, "ssl", smtpHost, smtpPort, true, "ssl", userEmail)
			if err != nil {
				slog.Info("OAuth account update error", "error", err)
			}
			err = h.Store.UpdateAccountTokens(r.Context(), acc.ID, tokens.AccessToken, tokens.RefreshToken)
			if err != nil {
				slog.Info("OAuth tokens update error", "error", err)
			}
		} else {
			slog.Info("Creating new OAuth account", "userEmail", userEmail)

			h.creationMu.Lock()

			if false {

			}
			newAcc, err := h.Store.CreateAccount(r.Context(), userEmail, "", provider, imapHost, imapPort, true, "ssl", smtpHost, smtpPort, true, "ssl", userEmail, "", "{}", "")

			h.creationMu.Unlock()

			if err != nil {
				slog.Info("OAuth account save error", "error", err)
				http.Redirect(w, r, redirectURL+"/en/settings?oauth=error&error="+url.QueryEscape("Account creation failed: "+err.Error()), http.StatusTemporaryRedirect)
				return
			}
			if newAcc != nil {
				if err := h.Store.UpdateAccountTokens(r.Context(), newAcc.ID, tokens.AccessToken, tokens.RefreshToken); err != nil {
					slog.Info("OAuth tokens save error", "userEmail", userEmail, "error", err)
					http.Redirect(w, r, redirectURL+"/en/settings?oauth=error&error="+url.QueryEscape("Token save failed: "+err.Error()), http.StatusTemporaryRedirect)
					return
				}
			}
		}
	}

	if userEmail == "" {
		slog.Warn("OAuth failed: userEmail is empty")
		http.Redirect(w, r, redirectURL+"/en/settings?oauth=error&error="+url.QueryEscape("Failed to get user email from provider"), http.StatusTemporaryRedirect)
		return
	}

	slog.Info("OAuth account linked", "email", userEmail)
	http.Redirect(w, r, redirectURL+"/en/settings?oauth=success&email="+url.QueryEscape(userEmail), http.StatusTemporaryRedirect)
}

// getUserEmailWithRetry fetches user email from OAuth provider with exponential backoff retries.
func (h *Handler) getUserEmailWithRetry(ctx context.Context, provider, accessToken string, maxRetries int) string {
	var userEmail string
	var err error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(1<<i) * time.Second
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
			time.Sleep(backoff)
		}
		switch provider {
		case "google":
			google, errProv := h.OAuth.GetGoogleProvider(ctx)
			if errProv != nil || google == nil {
				slog.Info("google oauth not configured")
				return ""
			}
			userInfo, e := google.GetUserInfo(ctx, accessToken)
			err = e
			if userInfo != nil {
				userEmail = userInfo.Email
			}
		case "microsoft":
			microsoft, errProv := h.OAuth.GetMicrosoftProvider(ctx)
			if errProv != nil || microsoft == nil {
				slog.Info("microsoft oauth not configured")
				return ""
			}
			userInfo, e := microsoft.GetUserInfo(ctx, accessToken)
			err = e
			if userInfo != nil {
				userEmail = userInfo.Email
			}
		default:
			slog.Info("unsupported provider", "provider", provider)
			return ""
		}
		if err == nil && userEmail != "" {
			break
		}
		slog.Info("OAuth GetUserInfo attempt failed", "attempt", i+1, "maxRetries", maxRetries, "provider", provider, "error", err)
	}
	if err != nil {
		slog.Info("OAuth GetUserInfo failed after all retries", "maxRetries", maxRetries, "provider", provider, "error", err)
	}
	return userEmail
}

// RefreshAccountToken проверяет и обновляет OAuth токен для указанного аккаунта, возвращая обновленный аккаунт.
func (h *Handler) RefreshAccountToken(ctx context.Context, accountID string) (*models.Account, error) {
	if h.OAuth == nil {
		return nil, fmt.Errorf("OAuth providers not configured")
	}

	acc, err := h.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		return nil, err
	}


	slog.Info("Refreshing OAuth token for account", "email", acc.Email, "provider", acc.Provider)

	var newAccessToken, newRefreshToken string
	switch acc.Provider {
	case "google":
		google, errProv := h.OAuth.GetGoogleProvider(ctx)
		if errProv != nil || google == nil {
			return nil, fmt.Errorf("Google OAuth provider not configured")
		}
		tokens, err := google.RefreshTokens(ctx, acc.OAuthRefreshToken)
		if err != nil {
			return nil, fmt.Errorf("Google token refresh failed: %w", err)
		}
		newAccessToken = tokens.AccessToken
		if tokens.RefreshToken != "" {
			newRefreshToken = tokens.RefreshToken
		} else {
			newRefreshToken = acc.OAuthRefreshToken
		}
	case "microsoft":
		microsoft, errProv := h.OAuth.GetMicrosoftProvider(ctx)
		if errProv != nil || microsoft == nil {
			return nil, fmt.Errorf("Microsoft OAuth provider not configured")
		}
		tokens, err := microsoft.RefreshTokens(ctx, acc.OAuthRefreshToken)
		if err != nil {
			return nil, fmt.Errorf("Microsoft token refresh failed: %w", err)
		}
		newAccessToken = tokens.AccessToken
		if tokens.RefreshToken != "" {
			newRefreshToken = tokens.RefreshToken
		} else {
			newRefreshToken = acc.OAuthRefreshToken
		}
	default:
		return acc, nil
	}

	err = h.Store.UpdateAccountTokens(ctx, acc.ID, newAccessToken, newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to save refreshed tokens to DB: %w", err)
	}

	acc.OAuthAccessToken = newAccessToken
	acc.OAuthRefreshToken = newRefreshToken
	return acc, nil
}

type TelegramSettingsRequest struct {
	TelegramUserID   int64  `json:"telegram_user_id"`
	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramAINotif  bool   `json:"telegram_ai_notifications"`
	TelegramAIChat   bool   `json:"telegram_ai_chat"`
	TelegramBotToken string `json:"telegram_bot_token,omitempty"`
	BotConfigured    bool   `json:"bot_configured"`
	BotEnvConfigured bool   `json:"bot_env_configured"`
	BotUsername      string `json:"bot_username"`
}

func (h *Handler) GetTelegramSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	email := middleware.GetUserIDFromContext(r.Context())
	if email == "" {
		WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, enabled, aiNotif, aiChat, botToken, err := h.Store.GetTelegramSettings(r.Context(), email)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	envToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	botEnvConfigured := envToken != ""

	botConfigured := false
	botUsername := ""
	if h.TGBot != nil {
		botConfigured = h.TGBot.IsConfigured()
		botUsername = h.TGBot.GetUsername()
	}
	// Fallback: use TELEGRAM_BOT_NAME env if API username is empty
	if botUsername == "" {
		botUsername = os.Getenv("TELEGRAM_BOT_NAME")
	}

	// Mask token for frontend: show only first 4 + "..." + last 4 chars
	maskedToken := ""
	if len(botToken) >= 8 {
		maskedToken = botToken[:4] + "..." + botToken[len(botToken)-4:]
	} else if botToken != "" {
		maskedToken = botToken[:1] + "..."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TelegramSettingsRequest{
		TelegramUserID:   userID,
		TelegramEnabled:  enabled,
		TelegramAINotif:  aiNotif,
		TelegramAIChat:   aiChat,
		TelegramBotToken: maskedToken,
		BotConfigured:    botConfigured,
		BotEnvConfigured: botEnvConfigured,
		BotUsername:      botUsername,
	})
}

func (h *Handler) UpdateTelegramSettings(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("TG_ENVONLY") == "true" {
		WriteJSONError(w, http.StatusForbidden, "Telegram settings are managed by administrator")
		return
	}

	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	email := middleware.GetUserIDFromContext(r.Context())
	if email == "" {
		WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req TelegramSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.Store.UpdateTelegramSettings(r.Context(), email, req.TelegramUserID, req.TelegramEnabled, req.TelegramAINotif, req.TelegramAIChat, req.TelegramBotToken)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	if h.TGBot != nil && req.TelegramBotToken != "" && req.TelegramBotToken != h.TGBot.GetToken() {
		slog.Info("Updating Telegram Bot token in memory")
		h.TGBot.SetToken(req.TelegramBotToken)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if username, err := h.TGBot.FetchUsername(ctx); err == nil {
			slog.Info("telegram bot username fetched", "username", username)
		} else {
			slog.Info("failed to fetch telegram bot username", "error", err)
		}

		if publicURL := os.Getenv("PUBLIC_URL"); publicURL != "" {
			go func() {
				if err := h.TGBot.SetWebhook(context.Background(), publicURL); err != nil {
					slog.Info("failed to set telegram webhook", "error", err)
				}
			}()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) TGWebhook(w http.ResponseWriter, r *http.Request) {
	if h.TGBot != nil {
		h.TGBot.HandleWebhook(w, r)
		return
	}
	WriteJSONError(w, http.StatusServiceUnavailable, "Telegram bot not configured")
}

func (h *Handler) WAWebhook(w http.ResponseWriter, r *http.Request) {
	WriteJSONError(w, http.StatusNotImplemented, "WhatsApp not yet implemented")
}

func isSystemFolder(name string) bool {
	lowerName := strings.ToLower(name)
	systemNames := []string{"inbox", "sent", "trash", "junk", "spam", "drafts", "[gmail]"}
	for _, sys := range systemNames {
		if strings.HasPrefix(lowerName, sys) {
			return true
		}
	}
	return false
}

func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request, accountID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	f, err := h.Store.CreateFolder(r.Context(), accountID, req.Name, req.Name, true)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) renameFolder(w http.ResponseWriter, r *http.Request, accountID, folderID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	folder, err := h.Store.GetFolderByID(r.Context(), folderID)
	if err != nil || folder == nil {
		WriteJSONError(w, http.StatusNotFound, "folder not found")
		return
	}
	if isSystemFolder(folder.Name) || isSystemFolder(req.Name) {
		WriteJSONError(w, http.StatusBadRequest, "cannot modify system IMAP folder")
		return
	}
	if err := h.Store.RenameFolder(r.Context(), folderID, req.Name); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) deleteFolder(w http.ResponseWriter, r *http.Request, accountID, folderID string) {
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	folder, err := h.Store.GetFolderByID(r.Context(), folderID)
	if err != nil || folder == nil {
		WriteJSONError(w, http.StatusNotFound, "folder not found")
		return
	}
	if isSystemFolder(folder.Name) {
		WriteJSONError(w, http.StatusBadRequest, "cannot modify system IMAP folder")
		return
	}
	if err := h.Store.DeleteFolder(r.Context(), folderID); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
