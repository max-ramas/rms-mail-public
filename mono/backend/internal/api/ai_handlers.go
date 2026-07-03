package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"rmsmail/internal/ai"
	"rmsmail/internal/chatbot"
	"rmsmail/internal/crypto"
	"rmsmail/internal/models"

	"github.com/google/uuid"
)

// --- AI request types ---

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIChatRequest struct {
	Messages []AIMessage `json:"messages"`
	Preset   string      `json:"preset"`
	Provider string      `json:"provider"`
	Model    string      `json:"model"`
	APIKey   string      `json:"api_key"`
}

type AICustomRequest struct {
	Preset   string `json:"preset"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	APIKey   string `json:"api_key"`
}

type AICategorizeRequest struct {
	Text     string `json:"text"`
	Preset   string `json:"preset"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

// debugLog prints a log line only when DEBUG=true env var is set.
func debugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		slog.Info(format, args...)
	}
}

// resolveAPIKey looks up the API key for a provider.
// Priority: env var (enterprise override) → account-specific ai_settings → global ai_settings.
func (h *Handler) resolveAPIKey(r *http.Request, providerName string) string {
	// 1. Enterprise override: environment variable (set by admin in Docker/k8s)
	if k := os.Getenv(ai.ProviderEnvKey(providerName)); k != "" {
		debugLog("[DIAG] resolveAPIKey: using env var for provider=%s", providerName)
		return k
	}

	// 2-3. ai_settings: account-specific → global (loop over candidate IDs).
	// Global fallback is ON by default; only disabled by ALLOW_GLOBAL_AI_KEYS=false.
	if h.Store == nil {
		return ""
	}
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			slog.Info("resolveAPIKey: access denied", "accountID", accountID, "error", err)
			return ""
		}
	}

	candidates := []string{accountID}
	if os.Getenv("ALLOW_GLOBAL_AI_KEYS") != "false" && accountID != "00000000-0000-0000-0000-000000000000" {
		candidates = append(candidates, "00000000-0000-0000-0000-000000000000")
	}

	for _, aid := range candidates {
		if aid == "" {
			continue
		}
		setting, err := h.Store.GetAISettings(r.Context(), aid)
		if err != nil || setting == nil || setting.APIKeysEncrypted == "" {
			continue
		}
		var dec string
		var decErr error
		decrypted := false
		for _, encKey := range crypto.GetAllEncryptionKeys() {
			if len(encKey) == 0 {
				continue
			}
			dec, decErr = crypto.Decrypt(setting.APIKeysEncrypted, encKey)
			if decErr == nil {
				decrypted = true
				break
			}
		}
		if !decrypted {
			debugLog("[DIAG] resolveAPIKey: decrypt FAILED for %s: %v", aid, decErr)
			continue
		}
		var keys map[string]string
		if json.Unmarshal([]byte(dec), &keys) != nil {
			debugLog("[DIAG] resolveAPIKey: JSON unmarshal FAILED for %s", aid)
			continue
		}
		k, ok := keys[providerName]
		if !ok || k == "" {
			continue
		}
		debugLog("[DIAG] resolveAPIKey: found key in ai_settings for provider=%s account=%s len=%d", providerName, aid, len(k))
		return k
	}
	debugLog("[DIAG] resolveAPIKey: no key for provider=%s", providerName)
	return ""
}

func (h *Handler) summarizeEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	if h.AI == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteJSONError(w, http.StatusNotFound, "email not found")
		} else {
			WriteAccessError(w, err)
		}
		return
	}
	bodyPath := safeBodyPath(email.BodyPath)
	if bodyPath == "" {
		WriteJSONError(w, http.StatusForbidden, "invalid body path")
		return
	}
	body, err := readEncryptedFile(bodyPath)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "failed to read email body")
		return
	}
	emailBody := string(body)
	if len(emailBody) > 32000 {
		emailBody = emailBody[:32000]
	}

	var customReq AICustomRequest
	if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&customReq)
	}

	// If a preset is provided, try to load its provider/model/key from settings.
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" || accountID == "unified" {
		accountID = email.AccountID
	}

	if customReq.Preset != "" && h.Store != nil {
		presetProvider, presetModel, err := h.Store.GetPresetSettings(r.Context(), accountID, customReq.Preset)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "internal error resolving preset")
			return
		}
		if presetProvider == "" || presetModel == "" {
			WriteJSONError(w, http.StatusUnprocessableEntity, "preset is missing or incomplete")
			return
		}
		// Apply preset values if not explicitly overridden by request
		if customReq.Provider == "" {
			customReq.Provider = presetProvider
		}
		if customReq.Model == "" {
			customReq.Model = presetModel
		}
	}

	providerName := customReq.Provider
	if providerName == "" {
		providerName = r.URL.Query().Get("provider")
	}
	if providerName == "" {
		providerName = "openrouter"
	}

	// Look up API key from ai_settings if not provided in request (mirrors AIChat)
	if customReq.APIKey == "" {
		customReq.APIKey = h.resolveAPIKey(r, providerName)
	}

	provider, ok := h.AI.GetProvider(providerName)
	if !ok {
		WriteJSONError(w, http.StatusBadRequest, "AI provider not found: "+providerName)
		return
	}

	var summary string
	if customReq.Model != "" || customReq.Prompt != "" || customReq.APIKey != "" {
		systemPrompt := customReq.Prompt
		if systemPrompt == "" {
			systemPrompt = "Summarize the following email in 2-3 sentences."
		}
		messages := []ai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: emailBody},
		}
		summary, err = h.callAIChat(r.Context(), providerName, provider, messages, customReq.Model, customReq.APIKey)
	} else {
		summary, err = provider.Summarize(r.Context(), emailBody)
	}

	if err != nil {
		h.logAI(r.Context(), "summarize", providerName, customReq.Model, 0, 0, 0, "error")
		slog.Info("summarizeEmail AI error", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	h.logAI(r.Context(), "summarize", providerName, customReq.Model, len(emailBody)/4, 50, 0, "success")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":      emailID,
		"summary": summary,
	})
}

func (h *Handler) AIChat(w http.ResponseWriter, r *http.Request) {
	if h.AI == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	AppMetrics.AIChatRequests.Add(1)

	var req AIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if len(req.Messages) > 50 {
		WriteJSONError(w, http.StatusBadRequest, "too many messages (max 50)")
		return
	}
	totalChars := 0
	for _, m := range req.Messages {
		totalChars += len(m.Content)
	}
	if totalChars > 100000 {
		WriteJSONError(w, http.StatusBadRequest, "total message content too large (max 100K chars)")
		return
	}

	if os.Getenv("LLM_ENVONLY") == "true" {
		req.Provider = os.Getenv("LLM_PROVIDER")
		req.Model = os.Getenv("LLM_MODEL")
		req.APIKey = os.Getenv(ai.ProviderEnvKey(req.Provider))
	}

	// Extract accountID from JWT claims early
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}

	if req.Preset != "" && h.Store != nil {
		presetProvider, presetModel, err := h.Store.GetPresetSettings(r.Context(), accountID, req.Preset)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "internal error resolving preset")
			return
		}
		if presetProvider == "" || presetModel == "" {
			WriteJSONError(w, http.StatusUnprocessableEntity, "preset is missing or incomplete")
			return
		}
		// Apply preset values if not explicitly overridden
		if req.Provider == "" {
			req.Provider = presetProvider
		}
		if req.Model == "" {
			req.Model = presetModel
		}
	}

	providerName := req.Provider
	if providerName == "" {
		providerName = "openrouter"
	}

	// If no API key from frontend, resolve via standard priority chain
	if req.APIKey == "" && providerName != "" {
		req.APIKey = h.resolveAPIKey(r, providerName)
	}

	debugLog("[DIAG] AIChat: after key resolution, req.APIKey empty=%v len=%d", req.APIKey == "", len(req.APIKey))

	messages := make([]ai.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ai.Message{Role: m.Role, Content: m.Content}
	}

	provider, ok := h.AI.GetProvider(providerName)
	if !ok {
		WriteJSONError(w, http.StatusBadRequest, "AI provider not found: "+providerName)
		return
	}

	// Tools loop
	const maxToolCalls = 5
	for i := 0; i < maxToolCalls; i++ {
		replyMsg, err := h.callAIChatWithTools(r.Context(), providerName, provider, messages, req.Model, req.APIKey, chatbot.AvailableTools)
		if err != nil {
			AppMetrics.AIErrors.Add(1)
			slog.Info("AIChat error", "provider", providerName, "model", req.Model, "error", err)
			h.logAI(r.Context(), "chat", providerName, req.Model, 0, 0, 0, "error")
			WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
			return
		}

		messages = append(messages, replyMsg)

		if len(replyMsg.ToolCalls) == 0 {
			h.logAI(r.Context(), "chat", providerName, "", len(messages)*50, 100, 0, "success")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"response": replyMsg.Content,
			})
			return
		}

		for _, tc := range replyMsg.ToolCalls {
			if tc.Function.Name == "fetch_emails" {
				var args struct {
					SearchQuery string `json:"search_query"`
					Limit       int    `json:"limit"`
					AccountID   string `json:"account_id"`
					FolderName  string `json:"folder_name"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				if args.Limit <= 0 || args.Limit > 50 {
					args.Limit = 10
				}
				if args.FolderName == "" {
					args.FolderName = "INBOX"
				}

				// GUARD: use user-resolved accountID, never "unified" from LLM
				queryAccountID := accountID

				filter := models.EmailFilterOpts{Search: args.SearchQuery}
				emails, err := h.Store.GetEmails(r.Context(), queryAccountID == "", queryAccountID, "", args.FolderName, 0, args.Limit, filter)

				var toolResult string
				if err != nil {
					toolResult = fmt.Sprintf("Error fetching emails: %v", err)
				} else {
					if len(emails) == 0 {
						toolResult = "No emails found matching the criteria."
					} else {
						var sb strings.Builder
						for idx, e := range emails {
							sb.WriteString(fmt.Sprintf("[%d] Subject: %s | From: %s <%s> | Snippet: %s\n", idx+1, e.Subject, e.SenderName, e.SenderAddress, e.Snippet))
						}
						toolResult = sb.String()
					}
				}

				messages = append(messages, ai.Message{
					Role:       "tool",
					Content:    toolResult,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
				})
			}
		}
	}

	h.logAI(r.Context(), "chat", providerName, "", len(messages)*50, 100, 0, "success")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"response": "I needed to make too many tool calls, please try to be more specific.",
	})
}

func (h *Handler) callAIChatWithTools(ctx context.Context, providerName string, defaultProvider ai.AIProvider, messages []ai.Message, model, apiKey string, tools []ai.Tool) (ai.Message, error) {
	// Use shallow copy via OverrideProviderSettings instead of mutating shared provider.
	// Prevents race condition when concurrent requests use different models/keys.
	if model != "" || apiKey != "" {
		if p := ai.OverrideProviderSettings(defaultProvider, model, apiKey); p != nil {
			return p.ChatWithTools(ctx, messages, tools)
		}
	}
	return defaultProvider.ChatWithTools(ctx, messages, tools)
}

func (h *Handler) callAIChat(ctx context.Context, providerName string, defaultProvider ai.AIProvider, messages []ai.Message, model, apiKey string) (string, error) {

	if model == "" && apiKey == "" {
		return defaultProvider.Chat(ctx, messages)
	}

	effectiveKey := apiKey
	if effectiveKey == "" {
		if ek := os.Getenv(ai.ProviderEnvKey(providerName)); ek != "" {
			effectiveKey = ek
		}
	}

	switch providerName {
	case "openrouter":
		if model == "" {
			model = os.Getenv("OPENROUTER_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://openrouter.ai/api/v1/chat/completions", model, effectiveKey, messages)
	case "openai":
		if model == "" {
			model = os.Getenv("OPENAI_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.openai.com/v1/chat/completions", model, effectiveKey, messages)
	case "deepseek":
		if model == "" {
			model = os.Getenv("DEEPSEEK_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.deepseek.com/chat/completions", model, effectiveKey, messages)
	case "groq":
		if model == "" {
			model = os.Getenv("GROQ_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.groq.com/openai/v1/chat/completions", model, effectiveKey, messages)
	case "xai":
		if model == "" {
			model = os.Getenv("XAI_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.x.ai/v1/chat/completions", model, effectiveKey, messages)
	case "qwen":
		if model == "" {
			model = os.Getenv("QWEN_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", model, effectiveKey, messages)
	case "ollama":
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			url = "http://localhost:11434"
		}
		if model == "" {
			model = os.Getenv("OLLAMA_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, url+"/v1/chat/completions", model, effectiveKey, messages)
	case "opencode":
		url := os.Getenv("OPENCODE_URL")
		if url == "" {
			url = "http://localhost:4312"
		}
		if model == "" {
			model = os.Getenv("OPENCODE_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, url+"/v1/chat/completions", model, effectiveKey, messages)
	case "anthropic":
		if model == "" {
			model = os.Getenv("ANTHROPIC_MODEL")
		}
		if p := ai.OverrideProviderSettings(defaultProvider, model, effectiveKey); p != nil {
			return p.Chat(ctx, messages)
		}
		return defaultProvider.Chat(ctx, messages)
	case "gemini":
		if model == "" {
			model = os.Getenv("GEMINI_MODEL")
		}
		if p := ai.OverrideProviderSettings(defaultProvider, model, effectiveKey); p != nil {
			return p.Chat(ctx, messages)
		}
		return defaultProvider.Chat(ctx, messages)
	default:
		return defaultProvider.Chat(ctx, messages)
	}
}

var ProviderModels = map[string][]string{
	"openrouter": {"llama-3.1-70b", "qwen-2.5", "llama-3.1-8b", "gemini-2.0-flash"},
	"openai":     {"gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo"},
	"anthropic":  {"claude-3-haiku", "claude-3.5-sonnet", "claude-3-opus"},
	"gemini":     {"gemini-2.0-flash", "gemini-2.0-flash-lite", "gemini-1.5-pro"},
	"deepseek":   {"deepseek-v4-pro", "deepseek-v4-flash"},
	"groq":       {"llama-3.1-70b", "llama-3.1-8b", "mixtral-8x7b-32768"},
	"ollama":     {"llama3.2", "qwen2", "mistral"},
	"xai":        {"grok-2-latest", "grok-2", "grok-beta"},
	"opencode":   {"codestral-latest", "deepseek-coder", "qwen2.5-coder"},
	"qwen":       {"qwen-plus", "qwen-max", "qwen-turbo", "qwen2.5-72b"},
}

func (h *Handler) AIModels(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	apiKey := r.URL.Query().Get("api_key")
	forceRefresh := r.URL.Query().Get("force_refresh") == "true"
	// Also accept API key from X-Api-Key header (doesn't conflict with JWT Authorization)
	if apiKey == "" {
		apiKey = r.Header.Get("X-Api-Key")
	}

	// Extract accountID from JWT for tenant-scoped ai_settings lookup
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}

	var models []string
	var fetchErr error

	if providerName != "" {
		models, fetchErr = h.fetchProviderModels(r.Context(), providerName, apiKey, accountID, forceRefresh)
	}

	// If there's an error and we are forcing refresh, don't fall back to hardcoded models.
	// This ensures the frontend gets the precise error from the provider.
	if len(models) == 0 && (!forceRefresh || fetchErr == nil) {
		if providerName != "" {
			models = ProviderModels[providerName]
			if models == nil {
				models = []string{}
			}
		} else {
			for _, v := range ProviderModels {
				models = append(models, v...)
			}
		}
	}

	resp := map[string]interface{}{
		"models":   models,
		"provider": providerName,
	}
	if fetchErr != nil {
		resp["error"] = "failed to fetch models"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) fetchProviderModels(ctx context.Context, providerName, apiKey, accountID string, forceRefresh bool) ([]string, error) {
	if h.AI == nil {
		return nil, nil
	}

	// Try Redis cache first (TTL 1 hour), unless forceRefresh is true
	if false {

	}

	effectiveKey := apiKey
	if effectiveKey == "" {
		if ek := os.Getenv(ai.ProviderEnvKey(providerName)); ek != "" {
			effectiveKey = ek
		}
	}

	// Fallback to ai_settings: account-specific first, then global
	if effectiveKey == "" && h.Store != nil {
		for _, aid := range []string{accountID, "00000000-0000-0000-0000-000000000000"} {
			if aid == "" {
				continue
			}
			setting, err := h.Store.GetAISettings(ctx, aid)
			if err != nil || setting == nil || setting.APIKeysEncrypted == "" {
				continue
			}
			encKey := []byte(crypto.GetPrimaryEncryptionKey())
			if len(encKey) == 0 {
				continue
			}
			dec, decErr := crypto.Decrypt(setting.APIKeysEncrypted, encKey)
			if decErr != nil {
				continue
			}
			var keys map[string]string
			if json.Unmarshal([]byte(dec), &keys) != nil {
				continue
			}
			if k, ok := keys[providerName]; ok && k != "" {
				effectiveKey = k
				break
			}
		}
	}

	var models []string
	var fetchErr error
	switch providerName {
	case "openrouter":
		models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://openrouter.ai/api/v1/models", effectiveKey)
	case "openai":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://api.openai.com/v1/models", effectiveKey)
		}
	case "deepseek":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://api.deepseek.com/models", effectiveKey)
		}
	case "groq":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://api.groq.com/openai/v1/models", effectiveKey)
		}
	case "xai":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://api.x.ai/v1/models", effectiveKey)
		}
	case "qwen":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchOpenAICompatModels(ctx, "https://dashscope.aliyuncs.com/compatible-mode/v1/models", effectiveKey)
		}
	case "ollama":
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			url = "http://localhost:11434"
		}
		models, fetchErr = ai.FetchOllamaModels(ctx, url)
	case "gemini":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchGeminiModels(ctx, effectiveKey)
		}
	case "anthropic":
		if effectiveKey != "" {
			models, fetchErr = ai.FetchAnthropicModels(ctx, effectiveKey)
		}
	case "opencode":
		url := os.Getenv("OPENCODE_URL")
		if url == "" {
			url = "http://localhost:4312"
		}
		models, fetchErr = ai.FetchOpenAICompatModels(ctx, url+"/v1/models", effectiveKey)
	}

	// Cache result in Redis for 1 hour
	if false {

	}

	return models, fetchErr
}

func (h *Handler) AICategorizeEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	if h.AI == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteJSONError(w, http.StatusNotFound, "email not found")
		} else {
			WriteAccessError(w, err)
		}
		return
	}

	bodyPath := safeBodyPath(email.BodyPath)
	if bodyPath == "" {
		WriteJSONError(w, http.StatusForbidden, "invalid body path")
		return
	}
	body, err := readEncryptedFile(bodyPath)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "failed to read email body")
		return
	}
	bodyStr := string(body)
	if len(bodyStr) > 32000 {
		bodyStr = bodyStr[:32000]
	}

	var customReq AICustomRequest
	if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&customReq)
	}

	if os.Getenv("LLM_ENVONLY") == "true" {
		customReq.Provider = os.Getenv("LLM_PROVIDER")
		customReq.Model = os.Getenv("LLM_MODEL")
		customReq.APIKey = os.Getenv(ai.ProviderEnvKey(customReq.Provider))
	}

	accountID := r.URL.Query().Get("account_id")
	if accountID == "" || accountID == "unified" {
		accountID = email.AccountID
	}

	if customReq.Preset != "" && h.Store != nil {
		presetProvider, presetModel, err := h.Store.GetPresetSettings(r.Context(), accountID, customReq.Preset)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "internal error resolving preset")
			return
		}
		if presetProvider == "" || presetModel == "" {
			WriteJSONError(w, http.StatusUnprocessableEntity, "preset is missing or incomplete")
			return
		}
		if customReq.Provider == "" {
			customReq.Provider = presetProvider
		}
		if customReq.Model == "" {
			customReq.Model = presetModel
		}
	}

	providerName := customReq.Provider
	if providerName == "" {
		providerName = r.URL.Query().Get("provider")
	}
	if providerName == "" {
		providerName = "openrouter"
	}

	// Look up API key from ai_settings if not provided in request (mirrors AIChat)
	if customReq.APIKey == "" {
		customReq.APIKey = h.resolveAPIKey(r, providerName)
	}

	provider, ok := h.AI.GetProvider(providerName)
	if !ok {
		WriteJSONError(w, http.StatusBadRequest, "AI provider not found: "+providerName)
		return
	}

	// Get categories from DB
	var customCats []string
	if setting, err := h.Store.GetSystemSetting(r.Context(), "ai_categories"); err == nil && setting != "" {
		var catList []models.AICategory
		if json.Unmarshal([]byte(setting), &catList) == nil {
			for _, c := range catList {
				customCats = append(customCats, c.Name)
			}
		}
	}

	var tags []string
	if customReq.Model != "" || customReq.Prompt != "" || customReq.APIKey != "" {
		systemPrompt := customReq.Prompt
		if systemPrompt == "" {
			catNames := "Invoice, Support, Urgent, Newsletter, Personal, Business, Official"
			if len(customCats) > 0 {
				catNames = strings.Join(customCats, ", ")
			}
			systemPrompt = "Categorize this email into one or more of these categories: " + catNames + ". Return only the category names separated by commas."
		}
		messages := []ai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: bodyStr},
		}
		resp, errAIChat := h.callAIChat(r.Context(), providerName, provider, messages, customReq.Model, customReq.APIKey)
		if errAIChat == nil {
			tags = ai.ParseCategories(resp)
		} else {
			err = errAIChat
		}
	} else {
		tags, err = provider.Categorize(r.Context(), bodyStr, customCats)
	}

	if err != nil {
		h.logAI(r.Context(), "categorize", providerName, customReq.Model, 0, 0, 0, "error")
		slog.Info("AICategorizeEmail error", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	h.logAI(r.Context(), "categorize", providerName, customReq.Model, len(bodyStr)/4, 10, 0, "success")

	if len(tags) > 0 {
		if err := h.Store.AddEmailTags(r.Context(), emailID, r.URL.Query().Get("account_id"), tags); err != nil {
			slog.Info("Failed to save tags", "emailID", emailID, "error", err)
		}
	}

	// Apply category rules: auto-read and auto-move based on ai_categories config.
	h.applyCategoryRules(r.Context(), email, tags)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{
		"tags": tags,
	})
}

func (h *Handler) AICategorize(w http.ResponseWriter, r *http.Request) {
	if h.AI == nil {
		WriteJSONError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	AppMetrics.AICategorizeReqs.Add(1)

	var req AICategorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if os.Getenv("LLM_ENVONLY") == "true" {
		req.Provider = os.Getenv("LLM_PROVIDER")
		req.Model = os.Getenv("LLM_MODEL")
		req.APIKey = os.Getenv(ai.ProviderEnvKey(req.Provider))
	}

	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}

	if req.Preset != "" && h.Store != nil {
		presetProvider, presetModel, err := h.Store.GetPresetSettings(r.Context(), accountID, req.Preset)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "internal error resolving preset")
			return
		}
		if presetProvider == "" || presetModel == "" {
			WriteJSONError(w, http.StatusUnprocessableEntity, "preset is missing or incomplete")
			return
		}
		if req.Provider == "" {
			req.Provider = presetProvider
		}
		if req.Model == "" {
			req.Model = presetModel
		}
	}

	providerName := req.Provider
	if providerName == "" {
		providerName = "openrouter"
	}

	// Look up API key from ai_settings if not provided (mirrors AIChat)
	if req.APIKey == "" {
		req.APIKey = h.resolveAPIKey(r, providerName)
	}

	provider, ok := h.AI.GetProvider(providerName)
	if !ok {
		WriteJSONError(w, http.StatusBadRequest, "AI provider not found: "+providerName)
		return
	}

	text := req.Text
	if len(text) > 32000 {
		text = text[:32000]
	}

	var tags []string
	var err error
	if req.Model != "" || req.APIKey != "" {
		systemPrompt := "Categorize this email into one or more of these categories: Invoice, Support, Urgent, Newsletter, Personal, Business, Official. Return only the category names separated by commas."
		messages := []ai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		}
		response, chatErr := h.callAIChat(r.Context(), providerName, provider, messages, req.Model, req.APIKey)
		if chatErr != nil {
			AppMetrics.AIErrors.Add(1)
			slog.Info("AICategorize chat error", "error", chatErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": chatErr.Error()})
			return
		}
		tags = ai.ParseCategories(response)
	} else {
		tags, err = provider.Categorize(r.Context(), text, nil)
	}
	if err != nil {
		AppMetrics.AIErrors.Add(1)
		h.logAI(r.Context(), "categorize", providerName, "", 0, 0, 0, "error")
		slog.Info("AICategorize error", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	h.logAI(r.Context(), "categorize", providerName, "", len(text)/4, 10, 0, "success")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{
		"tags": tags,
	})
}

// --- AI Log ---

func (h *Handler) logAI(ctx context.Context, action, provider, model string, promptTokens, completionTokens, durationMs int, status string) {
	if err := h.Store.LogAI(ctx, action, provider, model, promptTokens, completionTokens, durationMs, status); err != nil {
		slog.Info("Failed to log AI action", "error", err)
	}
}

func (h *Handler) ResetAIStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := h.Store.ResetAIStats(r.Context()); err != nil {
		slog.Error("internal error", "error", err)

		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetAIStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	stats, err := h.Store.GetAIStats(r.Context())
	if err != nil {
		slog.Error("internal error", "error", err)

		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) GetAILog(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	entries, err := h.Store.GetAILog(r.Context(), 100)
	if err != nil {
		slog.Error("internal error", "error", err)

		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	json.NewEncoder(w).Encode(entries)
}

// --- AI Settings ---

func (h *Handler) GetAISettings(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" && accountID != "00000000-0000-0000-0000-000000000000" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id required")
		return
	}
	setting, err := h.Store.GetAISettings(r.Context(), accountID)
	if err != nil {
		slog.Info("GetAISettings failed", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	if setting == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}
	// Never leak encrypted keys to the frontend.
	// The frontend uses localStorage as its key cache; the backend resolves
	// keys server-side from ai_settings on every AI request (Phase 22).
	setting.APIKeysEncrypted = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(setting)
}

func (h *Handler) UpsertAISettings(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("LLM_ENVONLY") == "true" {
		WriteJSONError(w, http.StatusForbidden, "AI settings are managed by administrator")
		return
	}

	var req struct {
		AccountID string `json:"account_id"`
		Preset    string `json:"preset"`
		Config    string `json:"config"`
		Prompts   string `json:"prompts"`
		APIKeys   string `json:"api_keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.AccountID != "" && req.AccountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if req.AccountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id required")
		return
	}

	encKey := []byte(crypto.GetPrimaryEncryptionKey())
	apiKeys := req.APIKeys
	debugLog("[DIAG] UpsertAISettings: account_id=%s encKey_len=%d api_keys_len=%d",
		req.AccountID, len(encKey), len(apiKeys))

	// Merge incoming keys with existing stored keys.
	// The frontend cannot restore keys from server (GetAISettings strips them for security),
	// so it only sends keys the user explicitly entered on this page load.
	// We MUST merge, not replace — otherwise entering one key wipes all others.
	var merged map[string]string

	// Load and decrypt existing keys from DB
	if existing, err := h.Store.GetAISettings(r.Context(), req.AccountID); err == nil && existing != nil && existing.APIKeysEncrypted != "" {
		decrypted, decErr := crypto.Decrypt(existing.APIKeysEncrypted, encKey)
		if decErr == nil {
			json.Unmarshal([]byte(decrypted), &merged)
		}
	}
	if merged == nil {
		merged = make(map[string]string)
	}

	// Merge incoming keys (only non-empty values overwrite existing)
	if apiKeys != "" {
		var incoming map[string]string
		if json.Unmarshal([]byte(apiKeys), &incoming) == nil {
			for k, v := range incoming {
				if v != "" {
					merged[k] = v
				}
			}
		}
	}

	// Serialize merged result
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	apiKeys = string(mergedJSON)
	debugLog("[DIAG] UpsertAISettings: merged %d keys total for account %s", len(merged), req.AccountID)

	if len(encKey) > 0 && apiKeys != "" && apiKeys != "{}" {
		encrypted, err := crypto.Encrypt(apiKeys, encKey)
		if err == nil {
			apiKeys = encrypted
			debugLog("[DIAG] UpsertAISettings: encrypted OK, len=%d", len(apiKeys))
		} else {
			debugLog("[DIAG] UpsertAISettings: encrypt FAILED: %v", err)
		}
	} else if len(encKey) == 0 && apiKeys != "" && apiKeys != "{}" {
		WriteJSONError(w, http.StatusInternalServerError, "encryption key not configured — cannot securely store API keys")
		return
	} else {
		debugLog("[DIAG] UpsertAISettings: skipping encrypt (encKey=%v hasKeys=%v)", len(encKey) > 0, apiKeys != "")
	}

	setting := models.AISetting{
		ID:               uuid.New().String(),
		AccountID:        req.AccountID,
		Preset:           req.Preset,
		Config:           req.Config,
		Prompts:          req.Prompts,
		APIKeysEncrypted: apiKeys,
	}
	if err := h.Store.UpsertAISetting(r.Context(), setting); err != nil {
		slog.Info("UpsertAISetting failed", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "AI request failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// applyCategoryRules applies auto-read and auto-move rules from ai_categories config.
func (h *Handler) applyCategoryRules(ctx context.Context, email *models.Email, tags []string) {
	raw, err := h.Store.GetSystemSetting(ctx, "ai_categories")
	if err != nil || raw == "" {
		return
	}
	var categories []models.AICategory
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return
	}

	for _, tag := range tags {
		for _, cat := range categories {
			if !strings.EqualFold(cat.Name, tag) {
				continue
			}
			// Auto-read
			if cat.AutoRead && !email.IsRead {
				if err := h.Store.MarkEmailRead(ctx, email.ID, email.AccountID); err != nil {
					slog.Info("applyCategoryRules: mark read failed", "emailID", email.ID, "error", err)
				}
			}
			// Auto-move to folder
			if cat.MoveTo != "" {
				if err := h.Store.MoveEmail(ctx, email.ID, email.AccountID, cat.MoveTo); err != nil {
					slog.Info("applyCategoryRules: move failed", "emailID", email.ID, "moveTo", cat.MoveTo, "error", err)
				}
			}
			break
		}
	}
}
