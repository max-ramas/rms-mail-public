package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"rmsmail/internal/ai"
	"rmsmail/internal/api/middleware"
	"rmsmail/internal/chatbot"
	"rmsmail/internal/crypto"
	"rmsmail/internal/models"
)

// mcpSession stores per-session state for MCP email_agent.
type mcpSession struct {
	authenticated bool
	accountID     string       // bound MCP key's account — data isolation guard
	messages      []ai.Message // conversation history for email_agent
}

// mcpSessions maps sessionID → session state.
var mcpSessions = struct {
	mu   sync.Mutex
	data map[string]*mcpSession
}{data: make(map[string]*mcpSession)}

type contextKey string

const ctxMCPAccountKey contextKey = "mcp_account_id"

// mcpAccountID extracts the accountID bound to the current MCP session.
func mcpAccountID(r *http.Request) string {
	if aid, ok := r.Context().Value(ctxMCPAccountKey).(string); ok && aid != "" {
		return aid
	}
	return ""
}

// mcpRequireAccountID ensures MCP tools are scoped to a session-bound account.
func (h *Handler) mcpRequireAccountID(w http.ResponseWriter, r *http.Request, id *int64) (string, bool) {
	sessionAID := mcpAccountID(r)
	if sessionAID == "" {
		h.mcpWriteError(w, id, -32001, "MCP API key with account binding required")
		return "", false
	}
	return sessionAID, true
}

// mcpAccountIDFromCtx extracts accountID from a context (for use without *http.Request).
func mcpAccountIDFromCtx(ctx context.Context) string {
	if aid, ok := ctx.Value(ctxMCPAccountKey).(string); ok && aid != "" {
		return aid
	}
	return ""
}

func (h *Handler) CreateMCPKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		req.Name = "Default"
	}
	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	raw, hash, prefix := generateAPIKey()
	key, err := h.Store.CreateMCPKey(r.Context(), req.Name, req.AccountID, hash, prefix, raw)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         key.ID,
		"name":       key.Name,
		"account_id": key.AccountID,
		"api_key":    raw,
		"prefix":     prefix,
		"created":    key.CreatedAt,
	})
}

func (h *Handler) ListMCPKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.Store.ListMCPKeys(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	type listKey struct {
		ID        string     `json:"id"`
		Name      string     `json:"name"`
		AccountID string     `json:"account_id"`
		KeyPrefix string     `json:"key_prefix"`
		FullKey   string     `json:"full_key,omitempty"`
		IsActive  bool       `json:"is_active"`
		LastUsed  *time.Time `json:"last_used_at,omitempty"`
		CreatedAt time.Time  `json:"created_at"`
	}
	result := make([]listKey, 0, len(keys))
	for _, k := range keys {
		if err := h.CheckAccountAccess(r.Context(), k.AccountID); err != nil {
			continue // skip keys not belonging to this user
		}
		result = append(result, listKey{
			ID:        k.ID,
			Name:      k.Name,
			AccountID: k.AccountID,
			KeyPrefix: k.KeyPrefix,
			FullKey:   "",
			IsActive:  k.IsActive,
			LastUsed:  k.LastUsedAt,
			CreatedAt: k.CreatedAt,
		})
	}
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) ViewMCPKey(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	keys, err := h.Store.ListMCPKeys(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	for _, k := range keys {
		if k.ID == id {
			if err := h.CheckAccountAccess(r.Context(), k.AccountID); err != nil {
				WriteAccessError(w, err)
				return
			}
			fullKey, err := h.Store.GetMCPKeyFull(r.Context(), id)
			if err != nil {
				WriteJSONError(w, http.StatusNotFound, "key not found")
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"api_key": fullKey})
			return
		}
	}
	WriteJSONError(w, http.StatusNotFound, "key not found")
}

func (h *Handler) DeleteMCPKey(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	// Fetch key to verify ownership before deletion
	keys, err := h.Store.ListMCPKeys(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	var found bool
	for _, k := range keys {
		if k.ID == id {
			found = true
			if err := h.CheckAccountAccess(r.Context(), k.AccountID); err != nil {
				WriteAccessError(w, err)
				return
			}
			break
		}
	}
	if !found {
		WriteJSONError(w, http.StatusNotFound, "key not found")
		return
	}

	if err := h.Store.DeleteMCPKey(r.Context(), id); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

func (h *Handler) ToggleMCPKey(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	// Fetch key to verify ownership
	keys, err := h.Store.ListMCPKeys(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	var found bool
	for _, k := range keys {
		if k.ID == id {
			found = true
			if err := h.CheckAccountAccess(r.Context(), k.AccountID); err != nil {
				WriteAccessError(w, err)
				return
			}
			break
		}
	}
	if !found {
		WriteJSONError(w, http.StatusNotFound, "key not found")
		return
	}

	key, err := h.Store.ToggleMCPKey(r.Context(), id)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(key)
}

func (h *Handler) MCPConnectInfo(w http.ResponseWriter, r *http.Request) {
	apiURL := requestPublicBaseURL(r)

	keyID := r.URL.Query().Get("key_id")
	actualKey := "<your_key>"
	authHeader := ""

	if keyID != "" {
		mcp, err := h.Store.GetMCPKey(r.Context(), keyID)
		if err != nil || mcp == nil || mcp.AccountID == "" {
			// Fail closed — don't reveal keys for unknown/invalid IDs
		} else if err := h.CheckAccountAccess(r.Context(), mcp.AccountID); err != nil {
			// Key belongs to another account — don't reveal
		} else if fullKey, err := h.Store.GetMCPKeyFull(r.Context(), keyID); err == nil && fullKey != "" {
			actualKey = fullKey
			authHeader = "Bearer " + fullKey
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"mcp_url":     apiURL + "/api/mcp/connect",
		"mcp_sse_url": apiURL + "/mcp/sse",
		"sse_url":     apiURL + "/api/events",
		"api_url":     apiURL,
		"command":     "./rms-mail-mcp",
		"auth_header": authHeader,
		"env_vars": map[string]string{
			"RMS_MAIL_API_URL": apiURL,
			"RMS_MAIL_API_KEY": actualKey,
		},
		"tools": []string{"search_emails", "get_email", "get_thread", "send_reply", "list_accounts", "list_labels", "get_unread_count"},
		"config_json": map[string]interface{}{
			"context_servers": map[string]interface{}{
				"rms-mail": map[string]interface{}{
					"url": apiURL + "/mcp/sse",
					"headers": map[string]string{
						"Authorization": "Bearer " + actualKey,
					},
					"enabled": true,
				},
			},
		},
		"config_claude": map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"rms-mail": map[string]interface{}{
					"command": "npx",
					"args": []string{
						"-y",
						"@modelcontextprotocol/server-sse",
						apiURL + "/mcp/sse",
					},
					"env": map[string]string{
						"API-KEY": actualKey,
					},
				},
			},
		},
	})
}

// MCPSSE handles MCP protocol connections: POST for JSON-RPC methods, GET for SSE event stream.
// Authenticated via MCP API Key (preferred) or JWT token.
func (h *Handler) MCPSSE(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("[MCP] >>> %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr))

	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		middleware.SetCORSHeaders(w, r, "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusOK)
		return
	}

	// --- POST: JSON-RPC method call ---
	if r.Method == http.MethodPost {
		h.mcpHandleJSONRPC(w, r)
		return
	}

	// --- GET: SSE event stream ---
	slog.Info(fmt.Sprintf("[MCP] GET SSE from %s", r.RemoteAddr))
	AppMetrics.ActiveSSEConns.Add(1)
	defer AppMetrics.ActiveSSEConns.Add(-1)

	// Authenticate via MCP API Key — multiple sources (matching rms-mcp)
	apiKey := r.URL.Query().Get("token") // ?token= — primary for SSE transport
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}
	if apiKey == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if apiKey == "" {
		apiKey = r.Header.Get("X-API-Key")
	}

	// Try as MCP key first, resolve accountID, then fall back to JWT
	var mcpAuthOK bool
	var sessionAccountID string
	if apiKey != "" {
		keyRecord, err := h.Store.GetMCPKeyByAPIKey(r.Context(), apiKey)
		if err == nil && keyRecord != nil {
			mcpAuthOK = true
			sessionAccountID = keyRecord.AccountID
		}
	}

	if !mcpAuthOK {
		WriteJSONError(w, http.StatusUnauthorized, "invalid or expired MCP API key")
		return
	}
	if sessionAccountID == "" {
		WriteJSONError(w, http.StatusUnauthorized, "MCP API key must be bound to an account")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Retry-After", "5000")

	// Generate session ID and register it with accountID binding
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	mcpSessions.mu.Lock()
	mcpSessions.data[sessionID] = &mcpSession{authenticated: true, accountID: sessionAccountID}
	mcpSessions.mu.Unlock()
	slog.Info(fmt.Sprintf("[MCP] session created: %s account=%s", sessionID, sessionAccountID))

	// Send endpoint: client POSTs to /mcp/messages with session + token
	msgURL := fmt.Sprintf("%s/mcp/messages?sessionId=%s", requestPublicBaseURL(r), sessionID)
	if apiKey != "" {
		msgURL += "&token=" + apiKey
	}
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", msgURL)

	// Clean up session on disconnect
	defer func() {
		mcpSessions.mu.Lock()
		delete(mcpSessions.data, sessionID)
		mcpSessions.mu.Unlock()
	}()
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	msgCh := make(chan string, 10)

	if false {

	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-msgCh:
			if ok {
				// Per-account data isolation: skip events not bound to this session's account.
				// Mirrors the filter in SSE handler (sse.go:226-235).
				if sessionAccountID != "" && len(msg) > 0 && msg[0] == '{' {
					var payload struct {
						AccountID string `json:"account_id"`
					}
					if err := json.Unmarshal([]byte(msg), &payload); err == nil && payload.AccountID != "" {
						if payload.AccountID != sessionAccountID && sessionAccountID != "" {
							continue // skip — this event belongs to a different account
						}
					}
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				flusher.Flush()
			}
		default:
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-msgCh:
				if ok {
					if sessionAccountID != "" && len(msg) > 0 && msg[0] == '{' {
						var payload struct {
							AccountID string `json:"account_id"`
						}
						if err := json.Unmarshal([]byte(msg), &payload); err == nil && payload.AccountID != "" {
							if payload.AccountID != sessionAccountID && sessionAccountID != "" {
								continue
							}
						}
					}
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
					flusher.Flush()
				}
			case <-ticker.C:
				fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
				flusher.Flush()
			}
		}
	}
}

// mcpHandleJSONRPC processes MCP JSON-RPC requests over POST.

// MCPMessages handles POST JSON-RPC for established MCP SSE sessions.
// Auth is via sessionId (authenticated during SSE handshake).
func (h *Handler) MCPMessages(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("[MCP] MESSAGES from %s", r.RemoteAddr))
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	mcpSessions.mu.Lock()
	sess, valid := mcpSessions.data[sessionID]
	mcpSessions.mu.Unlock()
	if !valid || sess == nil || !sess.authenticated {
		WriteJSONError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	// Inject accountID from session into request context for tool handlers
	ctx := context.WithValue(r.Context(), ctxMCPAccountKey, sess.accountID)
	h.mcpHandleJSONRPC(w, r.WithContext(ctx))
}

func (h *Handler) mcpHandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Auth: api_key from query, token from query, Authorization Bearer, or X-API-Key
	apiKey := r.URL.Query().Get("api_key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("token")
	}
	if apiKey == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if apiKey == "" {
		apiKey = r.Header.Get("X-API-Key")
	}
	if apiKey == "" {
		slog.Info("[MCP] auth failed: no api_key/token/header")
		h.mcpWriteError(w, nil, -32001, "authentication required")
		return
	}
	keys, kerr := h.Store.ListMCPKeys(r.Context())
	if kerr != nil || !mcpValidateKey(keys, h.Store, r.Context(), apiKey) {
		pfx := apiKey
		if len(pfx) > 8 {
			pfx = pfx[:8]
		}
		slog.Info(fmt.Sprintf("[MCP] auth failed: invalid key prefix=%s...", pfx))
		h.mcpWriteError(w, nil, -32001, "invalid api_key")
		return
	}

	// Resolve accountID from MCP key and inject into request context
	keyRecord, keyErr := h.Store.GetMCPKeyByAPIKey(r.Context(), apiKey)
	if keyErr == nil && keyRecord != nil && keyRecord.AccountID != "" {
		ctx := context.WithValue(r.Context(), ctxMCPAccountKey, keyRecord.AccountID)
		r = r.WithContext(ctx)
		slog.Info(fmt.Sprintf("[MCP] POST auth: account=%s", keyRecord.AccountID))
	}

	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.mcpWriteError(w, nil, -32700, "parse error")
		return
	}

	slog.Info(fmt.Sprintf("[MCP] method=%s id=%v", req.Method, req.ID))

	switch req.Method {
	case "initialize":
		h.mcpInitialize(w, req.ID, req.Params)
	case "notifications/initialized":
		// MCP spec: 202 Accepted, no body for notifications
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		h.mcpListTools(w, req.ID)
	case "tools/call":
		h.mcpCallTool(w, r, req.ID, req.Params)
	case "prompts/list":
		h.mcpListPrompts(w, req.ID)
	case "prompts/get":
		h.mcpGetPrompt(w, r, req.ID, req.Params)
	default:
		slog.Info(fmt.Sprintf("[MCP] unknown method: %s", req.Method))
		h.mcpWriteError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

func mcpValidateKey(keys []models.MCPKey, store Store, ctx context.Context, apiKey string) bool {
	for _, k := range keys {
		if k.IsActive {
			fullKey, err := store.GetMCPKeyFull(ctx, k.ID)
			if err == nil && fullKey == apiKey {
				return true
			}
		}
	}
	return false
}

func (h *Handler) mcpWriteError(w http.ResponseWriter, id *int64, code int, msg string) {
	slog.Info(fmt.Sprintf("[MCP] RESP error code=%d msg=%s", code, msg))
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"error":   map[string]interface{}{"code": code, "message": msg},
	}
	if id != nil {
		resp["id"] = *id
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(resp)
}

func (h *Handler) mcpWriteResult(w http.ResponseWriter, id *int64, result interface{}) {
	slog.Info(fmt.Sprintf("[MCP] RESP ok id=%v", id))
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{"jsonrpc": "2.0", "result": result}
	if id != nil {
		resp["id"] = *id
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(resp)
}

func (h *Handler) mcpInitialize(w http.ResponseWriter, id *int64, params json.RawMessage) {
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	w.Header().Set("Mcp-Session-Id", sessionID)

	// Negotiate protocol version: use the highest version both sides support.
	// Server supports up to "2025-11-25". If client requests lower, use that.
	clientVersion := "2024-11-05" // default
	var initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(params, &initParams); err == nil && initParams.ProtocolVersion != "" {
		clientVersion = initParams.ProtocolVersion
	}
	// Our server implements spec 2025-11-25 features, but negotiate to common ground.
	protocolVersion := "2025-11-25"
	if clientVersion < protocolVersion {
		protocolVersion = clientVersion
	}

	slog.Info(fmt.Sprintf("[MCP] initialize: client=%s server=%s negotiated=%s session=%s", clientVersion, "2025-11-25", protocolVersion, sessionID))

	h.mcpWriteResult(w, id, map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools":   map[string]interface{}{"listChanged": true},
			"prompts": map[string]interface{}{"listChanged": true},
		},
		"serverInfo": map[string]interface{}{
			"name":    "rms-mail",
			"version": "1.0.0",
		},
	})
}

func (h *Handler) mcpListTools(w http.ResponseWriter, id *int64) {
	tools := []map[string]interface{}{
		{
			"name":        "search_emails",
			"description": "Search emails by query and folder. Results are scoped to your account.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":  map[string]interface{}{"type": "string", "description": "Search term"},
					"folder": map[string]interface{}{"type": "string", "description": "Folder name, default INBOX"},
					"limit":  map[string]interface{}{"type": "integer", "description": "Max results, default 10"},
				},
			},
		},
		{
			"name":        "get_email",
			"description": "Get full email body and metadata by ID.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "Email ID"},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "list_accounts",
			"description": "List connected email accounts with unread counts.",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
		{
			"name":        "get_unread_count",
			"description": "Get total unread email count for your account.",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
	}
	h.mcpWriteResult(w, id, map[string]interface{}{"tools": tools})
}

func (h *Handler) mcpCallTool(w http.ResponseWriter, r *http.Request, id *int64, params json.RawMessage) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		h.mcpWriteError(w, id, -32602, "invalid params")
		return
	}

	var args map[string]interface{}
	json.Unmarshal(call.Arguments, &args)

	switch call.Name {
	case "search_emails":
		sessionAID, ok := h.mcpRequireAccountID(w, r, id)
		if !ok {
			return
		}
		query, _ := args["query"].(string)
		folder, _ := args["folder"].(string)
		limit := 10
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		if folder == "" {
			folder = "INBOX"
		}
		// GUARD: use session-bound accountID, ignore LLM-provided account_id
		filter := models.EmailFilterOpts{Search: query}
		emails, err := h.Store.GetEmails(r.Context(), false, sessionAID, "", folder, 0, limit, filter)
		if err != nil {
			h.mcpWriteError(w, id, -32000, "internal error")
			return
		}
		type result struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
			Sender  string `json:"sender"`
			Date    string `json:"date"`
			Snippet string `json:"snippet"`
			Unread  bool   `json:"unread"`
		}
		var results []result
		for _, e := range emails {
			results = append(results, result{
				ID: e.ID, Subject: e.Subject,
				Sender: e.SenderName + " <" + e.SenderAddress + ">",
				Date:   e.DateSent.Format(time.RFC3339), Snippet: e.Snippet,
				Unread: !e.IsRead,
			})
		}
		h.mcpWriteResult(w, id, map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmtJSON(results)}},
		})

	case "get_email":
		sessionAID, ok := h.mcpRequireAccountID(w, r, id)
		if !ok {
			return
		}
		emailID, _ := args["id"].(string)
		if emailID == "" {
			h.mcpWriteError(w, id, -32602, "id required")
			return
		}
		email, err := h.Store.GetEmail(r.Context(), emailID, r.URL.Query().Get("account_id"))
		if err != nil || email == nil {
			h.mcpWriteError(w, id, -32000, "email not found")
			return
		}
		// GUARD: verify email belongs to session-bound account
		if email.AccountID != sessionAID {
			h.mcpWriteError(w, id, -32000, "email not found")
			return
		}
		body := ""
		if email.BodyPath != "" {
			if data, err := readEncryptedFile(email.BodyPath); err == nil {
				body = string(data)
			}
		}
		h.mcpWriteResult(w, id, map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmtJSON(map[string]interface{}{
				"id": email.ID, "subject": email.Subject,
				"from": email.SenderName + " <" + email.SenderAddress + ">",
				"to":   email.RecipientAddress, "date": email.DateSent.Format(time.RFC3339),
				"body": body, "snippet": email.Snippet,
			})}},
		})

	case "list_accounts":
		sessionAID, ok := h.mcpRequireAccountID(w, r, id)
		if !ok {
			return
		}
		var results []struct {
			ID     string `json:"id"`
			Email  string `json:"email"`
			Unread int    `json:"unread"`
			Active bool   `json:"active"`
		}
		if sessionAID != "" {
			account, err := h.Store.GetAccount(r.Context(), sessionAID)
			if err == nil && account != nil {
				results = append(results, struct {
					ID     string `json:"id"`
					Email  string `json:"email"`
					Unread int    `json:"unread"`
					Active bool   `json:"active"`
				}{ID: account.ID, Email: account.Email, Unread: account.UnreadCount, Active: account.IsActive})
			}
		}
		h.mcpWriteResult(w, id, map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmtJSON(results)}},
		})

	case "get_unread_count":
		sessionAID, ok := h.mcpRequireAccountID(w, r, id)
		if !ok {
			return
		}
		total := 0
		counts, err := h.Store.GetUnreadCountByFolder(r.Context(), sessionAID)
		if err == nil {
			for _, c := range counts {
				total += c
			}
		}
		h.mcpWriteResult(w, id, map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("%d unread", total)}},
		})

	default:
		h.mcpWriteError(w, id, -32601, "tool not found: "+call.Name)
	}
}

func fmtJSON(v interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
	// Trim trailing newline added by Encode
	s := buf.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

func (h *Handler) registeredProviderNames() []string {
	if h.AI == nil {
		return nil
	}
	names := make([]string, 0, len(h.AI.Providers()))
	for name := range h.AI.Providers() {
		names = append(names, name)
	}
	return names
}

// mcpListPrompts returns email tools as MCP prompts (visible in Zed / menu).
func (h *Handler) mcpListPrompts(w http.ResponseWriter, id *int64) {
	prompts := []map[string]interface{}{
		{
			"name":        "search_emails",
			"description": "Search emails by query and folder. Results are scoped to your account.",
			"arguments": []map[string]interface{}{
				{"name": "query", "description": "Search term", "required": false},
				{"name": "folder", "description": "Folder name, default INBOX", "required": false},
				{"name": "limit", "description": "Max results, default 10", "required": false},
			},
		},
		{
			"name":        "get_email",
			"description": "Get full email body and metadata by ID.",
			"arguments": []map[string]interface{}{
				{"name": "id", "description": "Email ID", "required": true},
			},
		},
		{
			"name":        "list_accounts",
			"description": "List connected email accounts with unread counts.",
			"arguments":   []map[string]interface{}{},
		},
		{
			"name":        "get_unread_count",
			"description": "Get total unread email count for your account.",
			"arguments":   []map[string]interface{}{},
		},
		{
			"name":        "email_agent",
			"description": "AI email assistant. Ask anything about your emails in natural language — e.g. 'show unread from Ivan', 'summarize the last 3 emails', 'find invoices from April'.",
			"arguments": []map[string]interface{}{
				{"name": "task", "description": "What do you need help with?", "required": true},
			},
		},
	}
	h.mcpWriteResult(w, id, map[string]interface{}{"prompts": prompts})
}

// mcpGetPrompt returns a resolved prompt message for the user.
func (h *Handler) mcpGetPrompt(w http.ResponseWriter, r *http.Request, id *int64, params json.RawMessage) {
	var call struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		h.mcpWriteError(w, id, -32602, "invalid params")
		return
	}

	if call.Name == "email_agent" {
		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID == "" {
			// Fallback: use remote addr for session continuity across streamable HTTP calls.
			sessionID = "http_" + r.RemoteAddr
		}
		h.mcpEmailAgent(w, r, id, call.Arguments["task"], sessionID)
		return
	}

	var msg string
	switch call.Name {
	case "search_emails":
		msg = "Please describe what emails you want me to find."
	case "get_email":
		msg = "Please provide the email ID to retrieve."
	case "list_accounts":
		msg = "Fetching your connected email accounts..."
	case "get_unread_count":
		msg = "Checking unread email count..."
	default:
		h.mcpWriteError(w, id, -32602, "prompt not found: "+call.Name)
		return
	}

	h.mcpWriteResult(w, id, map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": map[string]interface{}{
					"type": "text",
					"text": msg,
				},
			},
		},
	})
}

// mcpEmailAgent calls the AI with the user's natural language task.
// Uses global AI settings (ai_settings for account 00000000-...).
func (h *Handler) mcpEmailAgent(w http.ResponseWriter, r *http.Request, id *int64, task string, sessionID string) {
	if task == "" {
		h.mcpWriteError(w, id, -32602, "task is required")
		return
	}
	if h.AI == nil {
		h.mcpWriteError(w, id, -32000, "AI not configured on server")
		return
	}

	ctx := r.Context()

	// Load or create session history.
	mcpSessions.mu.Lock()
	sess, ok := mcpSessions.data[sessionID]
	if !ok || sess == nil {
		accountID := mcpAccountID(r)
		if accountID == "" {
			mcpSessions.mu.Unlock()
			h.mcpWriteError(w, id, -32001, "MCP API key with account binding required")
			return
		}
		sess = &mcpSession{authenticated: true, accountID: accountID}
		mcpSessions.data[sessionID] = sess
	}
	history := sess.messages
	mcpSessions.mu.Unlock()

	// Get global AI settings for provider + system prompt
	setting, err := h.Store.GetAISettings(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil || setting == nil || setting.Preset == "" {
		h.mcpWriteError(w, id, -32000, "no AI settings configured — set up AI provider in Settings → AI")
		return
	}

	providerName := setting.Preset

	// Config is `{"chat":{"provider":"...","model":"..."}, ...}` — use chat task config.
	// Preset ("custom"/"fast"/"quality") is a template, not a provider name.
	modelName := ""
	if setting.Config != "" {
		var cfg map[string]struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		if json.Unmarshal([]byte(setting.Config), &cfg) == nil {
			if chat, ok := cfg["chat"]; ok && chat.Provider != "" {
				providerName = chat.Provider
				modelName = chat.Model
			}
		}
	}

	// Resolve API key: env var → ai_settings encrypted keys
	apiKey := os.Getenv(ai.ProviderEnvKey(providerName))
	if apiKey == "" && setting.APIKeysEncrypted != "" {
		encKey := []byte(crypto.GetPrimaryEncryptionKey())
		if len(encKey) > 0 {
			if dec, decErr := crypto.Decrypt(setting.APIKeysEncrypted, encKey); decErr == nil {
				var keys map[string]string
				if json.Unmarshal([]byte(dec), &keys) == nil {
					apiKey = keys[providerName]
				}
			}
		}
	}
	// apiKey may be empty — local providers (Ollama, etc.) don't need one.

	// Get system prompt for email_agent
	systemPrompt := "You are an email assistant. Help the user with their emails. You can search, read, and manage emails. Respond concisely in the same language the user used."
	if setting.Prompts != "" {
		var prompts map[string]string
		if json.Unmarshal([]byte(setting.Prompts), &prompts) == nil {
			if p, ok := prompts["email_agent"]; ok && p != "" {
				systemPrompt = p
			}
		}
	}

	slog.Info(fmt.Sprintf("[MCP] email_agent: task=%q provider=%s model=%s apiKey=%v", task, providerName, modelName, apiKey != ""))

	// Same tools as TG bot and frontend chat.
	emailTools := make([]ai.Tool, len(chatbot.AvailableTools))
	copy(emailTools, chatbot.AvailableTools)

	// Build messages: system prompt + conversation history + new user message.
	// Keep only last 20 messages to bound token usage.
	maxHistory := 20
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	messages := make([]ai.Message, 0, len(history)+2)
	messages = append(messages, ai.Message{Role: "system", Content: systemPrompt})
	messages = append(messages, history...)
	messages = append(messages, ai.Message{Role: "user", Content: task})

	const maxToolRounds = 5
	for round := 0; round < maxToolRounds; round++ {
		replyMsg, err := h.AI.ChatWithTools(ctx, providerName, modelName, apiKey, messages, emailTools)
		if err != nil {
			slog.Info(fmt.Sprintf("[MCP] email_agent AI error (round %d): %v", round, err))
			h.mcpWriteError(w, id, -32000, "AI call failed: "+err.Error())
			return
		}

		messages = append(messages, replyMsg)

		// If no tool calls, this is the final text response.
		if len(replyMsg.ToolCalls) == 0 {
			if replyMsg.Content != "" {
				// Save conversation to session history.
				mcpSessions.mu.Lock()
				if s, ok := mcpSessions.data[sessionID]; ok && s != nil {
					s.messages = append(s.messages,
						ai.Message{Role: "user", Content: task},
						replyMsg,
					)
				}
				mcpSessions.mu.Unlock()

				h.mcpWriteResult(w, id, map[string]interface{}{
					"messages": []map[string]interface{}{
						{
							"role": "assistant",
							"content": map[string]interface{}{
								"type": "text",
								"text": replyMsg.Content,
							},
						},
					},
				})
				return
			}
			// Empty content with no tool calls — ask again.
			messages = append(messages, ai.Message{
				Role: "user", Content: "Please provide a response.",
			})
			continue
		}

		// Execute tool calls and append results.
		for _, tc := range replyMsg.ToolCalls {
			slog.Info(fmt.Sprintf("[MCP] email_agent tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments))
			result := h.mcpExecuteEmailTool(ctx, tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, ai.Message{
				Role:       "tool",
				Content:    result,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max rounds exceeded.
	h.mcpWriteError(w, id, -32000, "email agent exceeded maximum tool call rounds")
}

// mcpExecuteEmailTool runs the fetch_emails tool scoped to the session-bound account.
func (h *Handler) mcpExecuteEmailTool(ctx context.Context, name, argsJSON string) string {
	var args map[string]interface{}
	if argsJSON != "" {
		json.Unmarshal([]byte(argsJSON), &args)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	if name != "fetch_emails" {
		return fmtJSON(map[string]string{"error": "unknown tool: " + name})
	}

	searchQuery, _ := args["search_query"].(string)
	folderName, _ := args["folder_name"].(string)
	if folderName == "" {
		folderName = "INBOX"
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 && l <= 50 {
		limit = int(l)
	}

	// GUARD: use session-bound accountID from context, ignore LLM-provided account_id
	sessionAID := mcpAccountIDFromCtx(ctx)
	if sessionAID == "" {
		return fmtJSON(map[string]string{"error": "MCP API key with account binding required"})
	}

	filter := models.EmailFilterOpts{Search: searchQuery}
	emails, err := h.Store.GetEmails(ctx, false, sessionAID, "", folderName, 0, limit, filter)
	if err != nil {
		return fmtJSON(map[string]string{"error": err.Error()})
	}
	if len(emails) == 0 {
		return "No emails found matching the criteria."
	}

	var sb strings.Builder
	for i, e := range emails {
		sb.WriteString(fmt.Sprintf("[%d] Subject: %s | From: %s <%s> | Date: %s | Snippet: %s\n",
			i+1, e.Subject, e.SenderName, e.SenderAddress, e.DateSent.Format(time.RFC3339), e.Snippet))
	}
	return sb.String()
}
