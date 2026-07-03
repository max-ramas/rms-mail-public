package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/klauspost/compress/gzhttp"

	"rmsmail/internal/ai"
	"rmsmail/internal/api"
	"rmsmail/internal/api/middleware"
	"rmsmail/internal/attachment"
	"rmsmail/internal/auth"
	"rmsmail/internal/chatbot"
	"rmsmail/internal/crypto"
	"rmsmail/internal/edition"
	"rmsmail/internal/gc"
	"rmsmail/internal/license"
	"rmsmail/internal/mail"
	"rmsmail/internal/models"
	"rmsmail/internal/sentry"
	"rmsmail/internal/store"
	"rmsmail/internal/sync"
	"rmsmail/internal/telemetry"
)

// AppVersion holds the version string injected via ldflags at build time.
var AppVersion string = "3.0.2"

type schemaStore interface {
	InitSchema(ctx context.Context, schema string) error
	RunMigrations(ctx context.Context) (int, error)
}

func bootstrapDatabase(store schemaStore, schema string) error {
	ctx := context.Background()
	if err := store.InitSchema(ctx, schema); err != nil {
		slog.Warn("schema initialization warning", "error", err)
	} else {
		slog.Info("database schema initialized")
	}

	migrated, err := store.RunMigrations(ctx)
	if err != nil {
		return err
	}
	if migrated > 0 {
		slog.Info("migrations applied", "count", migrated)
	}

	if pgStore, ok := any(store).(interface{ RunBackgroundOptimizations(context.Context) }); ok {
		go pgStore.RunBackgroundOptimizations(context.Background())
	}
	return nil
}

// decryptTelegramToken tries to decrypt a Telegram bot token using all configured
// encryption keys. Tokens are stored encrypted via DeriveKeyWithDomain("telegram_token").
// Falls back to raw keys for legacy data. Returns empty string if decryption fails.
func decryptTelegramToken(encrypted string) string {
	for _, key := range crypto.GetAllEncryptionKeys() {
		derived := crypto.DeriveKeyWithDomain(key, "telegram_token")
		if dec, err := crypto.Decrypt(encrypted, derived); err == nil {
			return dec
		}
	}
	for _, key := range crypto.GetAllEncryptionKeys() {
		if dec, err := crypto.Decrypt(encrypted, key); err == nil {
			return dec
		}
	}
	return ""
}


func main() {
	logLevel := slog.LevelInfo
	if envLvl := os.Getenv("LOG_LEVEL"); envLvl == "info" {
		logLevel = slog.LevelInfo
	} else if envLvl == "debug" {
		logLevel = slog.LevelDebug
	} else if envLvl == "error" {
		logLevel = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel, AddSource: true})))

	// Optional GlitchTip/Sentry error monitoring (zero overhead when SENTRY_DSN is unset).
	sentry.Init()
	defer sentry.Flush()

	// Initialize edition
	edition.Init()
	slog.Info("running edition", "edition", edition.Current)
	bgCtx := context.Background()

	if os.Getenv("ALLOW_INSECURE_TLS") == "true" {
		slog.Warn("ALLOW_INSECURE_TLS is enabled — TLS certificate verification is DISABLED for all IMAP/SMTP connections")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if edition.IsMono() {
		// Mono always uses SQLite, ignore DATABASE_URL
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		dbURL = filepath.Join(wd, "data", "rms-mail-mono.db")
		os.MkdirAll(filepath.Dir(dbURL), 0700)
	} else if dbURL == "" {
		slog.Error("configuration error", "detail", "DATABASE_URL environment variable is required")
		os.Exit(1)
	}
	// Read ENCRYPTION_KEYS (comma-separated), fall back to ENCRYPTION_KEY
	encKeyRaw := os.Getenv("ENCRYPTION_KEYS")
	if encKeyRaw == "" {
		encKeyRaw = os.Getenv("ENCRYPTION_KEY")
	}

	if encKeyRaw == "" {
		slog.Error("FATAL: ENCRYPTION_KEY or ENCRYPTION_KEYS is required")
		os.Exit(1)
	}

	// Derive each key via SHA-256 to 32 bytes (same as crypto.deriveKey)
	var encKeys [][]byte
	for _, keyStr := range strings.Split(encKeyRaw, ",") {
		keyStr = strings.TrimSpace(keyStr)
		if keyStr == "" {
			continue
		}
		encKeys = append(encKeys, []byte(keyStr))
	}

	store, err := store.NewStorage(dbURL, encKeys)
	if err != nil {
		slog.Error("failed to create storage", "error", err)
		os.Exit(1)
	}

	middleware.InitJWTAuth()

	// Ping with retry
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		if err := store.Ping(ctx); err == nil {
			break
		}
		if edition.IsMono() {
			slog.Error("failed to connect to database", "driver", "sqlite", "error", err)
			os.Exit(1)
		}
		slog.Info("waiting for database", "driver", "postgresql")
		time.Sleep(1 * time.Second)
	}
	if edition.IsMono() {
		slog.Info("database connected", "driver", "sqlite")
	} else {
		slog.Info("database connected", "driver", "postgresql")
	}

	api.InitCamoKey()

	// Initialize security module (JWT auth + encryption validation)
	api.InitializeSecurity()

	// Load schema
	schemaFile := "schema.sql"
	if edition.IsMono() {
		schemaFile = "schema_mono.sql"
	}
	schema, err := os.ReadFile(schemaFile)
	if err != nil {
		altPaths := []string{"backend/" + schemaFile, "app_build/backend/" + schemaFile, "../" + schemaFile, "../../" + schemaFile}
		for _, p := range altPaths {
			schema, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
	}
	if err == nil {
		if err := bootstrapDatabase(store, string(schema)); err != nil {
			slog.Error("database bootstrap failed", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("schema file not found", "detail", "will rely on existing DB schema")
		if migrated, err := store.RunMigrations(context.Background()); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		} else if migrated > 0 {
			slog.Info("migrations applied", "count", migrated)
		}
	}

	if edition.IsMonoPro() {
		allowed, _ := store.GetSystemSetting(context.Background(), "allowed_domains")
		mail.MonoProAllowedDomains.Store(allowed)
	}
	if edition.IsMono() || edition.IsMonoPro() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic", "area", "fts_reindex", "error", r)
				}
			}()
			if err := store.ReindexFTS(context.Background()); err != nil {
				slog.Warn("FTS reindex warning", "error", err)
			}
		}()
	}

	// Initialize License Manager
	appVersion := AppVersion
	if appVersion == "" {
		appVersion = "3.0.2" // Fallback
	}
	telemetry.TrackInstallation(appVersion)
	license.InitHK()
	productID := edition.Current.ProductID()
	pingMgr := license.NewManager(store, productID, appVersion)
	pingMgr.StartBackgroundPinger(bgCtx)

	if len(os.Args) > 1 && os.Args[1] == "-gc" {
		if err := gc.RunGarbageCollector(context.Background(), store, "storage"); err != nil {
			slog.Error("Garbage collection failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "-rekey" {
		count, err := store.RekeyAll(context.Background())
		if err != nil {
			slog.Error("Rekey failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Rekey complete", "re_encrypted", count)
		os.Exit(0)
	}

	// Storage directories (not needed in Mono)
	var cas *attachment.CASStorage
	if !edition.IsMono() {
		storageRoot := os.Getenv("STORAGE_ROOT")
		if storageRoot == "" {
			storageRoot = "storage"
		}
		emailsDir := filepath.Join(storageRoot, "emails")
		attachmentsDir := filepath.Join(storageRoot, "attachments")
		dirsOk := true
		for _, dir := range []string{emailsDir, attachmentsDir} {
			if err := os.MkdirAll(dir, 0750); err != nil {
				slog.Warn("failed to create directory", "directory", dir, "error", err)
				dirsOk = false
			}
		}

		if dirsOk {
			cas = attachment.NewCASStorage(attachmentsDir, store)
		} else {
			slog.Warn("CAS storage disabled — storage directories unavailable", "storage_root", storageRoot)
		}
	}

	aiDisabled := os.Getenv("AI_DISABLE") == "true"
	var aiGateway *ai.Gateway
	if !aiDisabled {
		aiGateway = ai.NewGateway()
		aiGateway.RegisterProvider("openrouter", &ai.OpenRouterProvider{
			APIKey: os.Getenv("OPENROUTER_API_KEY"),
			Model:  os.Getenv("OPENROUTER_MODEL"),
		})
		aiGateway.RegisterProvider("openai", &ai.OpenAIProvider{
			APIKey: os.Getenv("OPENAI_API_KEY"),
			Model:  os.Getenv("OPENAI_MODEL"),
		})
		aiGateway.RegisterProvider("anthropic", &ai.AnthropicProvider{
			APIKey: os.Getenv("ANTHROPIC_API_KEY"),
			Model:  os.Getenv("ANTHROPIC_MODEL"),
		})
		aiGateway.RegisterProvider("gemini", &ai.GeminiProvider{
			APIKey: os.Getenv("GEMINI_API_KEY"),
			Model:  os.Getenv("GEMINI_MODEL"),
		})
		aiGateway.RegisterProvider("ollama", ai.NewOllamaProvider(
			os.Getenv("OLLAMA_URL"),
			os.Getenv("OLLAMA_MODEL"),
		))
		aiGateway.RegisterProvider("deepseek", &ai.DeepSeekProvider{
			APIKey: os.Getenv("DEEPSEEK_API_KEY"),
			Model:  os.Getenv("DEEPSEEK_MODEL"),
		})
		aiGateway.RegisterProvider("groq", &ai.GroqProvider{
			APIKey: os.Getenv("GROQ_API_KEY"),
			Model:  os.Getenv("GROQ_MODEL"),
		})
		aiGateway.RegisterProvider("xai", &ai.XAIProvider{
			APIKey: os.Getenv("XAI_API_KEY"),
			Model:  os.Getenv("XAI_MODEL"),
		})
		aiGateway.RegisterProvider("opencode", ai.NewOpenCodeProvider(
			os.Getenv("OPENCODE_URL"),
			os.Getenv("OPENCODE_MODEL"),
		))
		aiGateway.RegisterProvider("qwen", &ai.QwenProvider{
			APIKey: os.Getenv("QWEN_API_KEY"),
			Model:  os.Getenv("QWEN_MODEL"),
		})
	}

	var syncMgr *sync.Manager
	// Avoid nil-concrete-pointer-in-non-nil-interface trap.
	// var cas *attachment.CASStorage = nil → casIf = nil interface (correct).
	// var cas *attachment.CASStorage = non-nil → casIf = non-nil interface (correct).
	var casIf sync.CASStore
	if cas != nil {
		casIf = cas
	}
	if !aiDisabled {
		syncMgr = sync.NewManager(store, casIf, sync.WithAIGateway(aiGateway))
	} else {
		syncMgr = sync.NewManager(store, casIf)
	}
	if edition.IsMono() || edition.IsMonoPro() {
		syncMgr.Timing = sync.MonoTiming()
		slog.Info("sync timing", "edition", edition.Current.String(), "idle_timeout", syncMgr.Timing.IDLETimeout)
	}
	// Live lock checking — computed by LicenseMgr, not from DB flag
	syncMgr.LockChecker = func(ctx context.Context, index int) bool { return false }
	go syncMgr.Start(bgCtx)

	var oauthManager *auth.OAuthManager
	if !edition.IsMono() {
		oauthManager = auth.NewOAuthManager(func(ctx context.Context, key string) (string, error) {
			return store.GetSystemSetting(ctx, key)
		})
		syncMgr.OAuth = oauthManager
	}

	// Initialize JWT token blacklist (no-op when Redis is nil, e.g. Mono)

	// Mono: webhook polling via SQLite trigger (no Redis dependency).
	// If Redis is available (e.g. optional external Redis), use Redis poller.
	if edition.IsMono() {
	}

	// Start job worker for DB-backed queues (auto_draft, etc.) — only if AI enabled
	if !aiDisabled {
	}

	eventBus := api.NewEventBus()

	// apiHandler is declared early so OnNewEmail can reference it for
	// cache invalidation. It is assigned later after full initialization.
	var apiHandler *api.Handler

	syncMgr.OnNewEmail = func(ctx context.Context, accountID, emailID, subject, senderName, senderAddr string) {
		// Invalidate email-list cache so the frontend picks up new emails.
		// SSE broadcast is handled by BroadcastEvent in the fetcher.
		if apiHandler != nil {
			apiHandler.InvalidateEmailCache(ctx, accountID)
		}
	}

	// Load token from DB if not in env
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		if dbToken, err := store.GetGlobalTelegramBotToken(context.Background()); err == nil && dbToken != "" {
			// Token from DB is stored encrypted (domain "telegram_token").
			// Try decryption with all keys; fall back to raw if already plaintext.
			if dec := decryptTelegramToken(dbToken); dec != "" {
				tgToken = dec
			} else {
				tgToken = dbToken
			}
			slog.Info("telegram bot token loaded from database")
		}
	}

	var tgAdapter chatbot.AIGateway
	if !aiDisabled {
		tgAdapter = &telegramAIAdapter{
			gateway: aiGateway,
			store:   store,
		}
	}
	tgBot := chatbot.NewTelegramBot(store, tgAdapter, chatbot.NewMemSessionStore())
	tgBot.Token = tgToken

	if tgToken != "" {
		if publicURL := os.Getenv("PUBLIC_URL"); publicURL != "" {
			// Validate webhook URL before registration (SSRF protection)
			if _, err := sync.ValidateWebhookURL(publicURL); err != nil {
				slog.Warn("invalid PUBLIC_URL, webhook not registered", "error", err, "url", publicURL)
			} else {
				go func() {
					// Wait a moment for server to start before setting webhook
					time.Sleep(2 * time.Second)
					if err := tgBot.SetWebhook(context.Background(), publicURL); err != nil {
						slog.Warn("failed to set telegram webhook", "error", err)
					} else {
						slog.Info("telegram webhook set", "url", publicURL+"/api/tg/webhook")
					}
				}()
			}
		} else {
			slog.Warn("public_url not set", "detail", "Telegram bot webhook will not be registered automatically")
		}

		// Fetch bot username in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if username, err := tgBot.FetchUsername(ctx); err == nil {
				slog.Info("telegram bot username", "username", username)
			} else {
				slog.Warn("failed to fetch telegram bot username", "error", err)
				// Fallback to TELEGRAM_BOT_NAME env
				if envName := os.Getenv("TELEGRAM_BOT_NAME"); envName != "" {
					tgBot.SetUsername(envName)
					slog.Info("using telegram bot name from env", "username", envName)
				}
			}
		}()

		slog.Info("telegram bot initialized")
	} else {
		slog.Info("telegram bot initialized", "token", "empty", "status", "pending UI configuration")
	}


	syncMgr.SetEventBroadcast(func(ctx context.Context, channel, message string) {
		// Extract accountID from message for cache invalidation.
		// Skip "new-email" — OnNewEmail already invalidates cache before this fires.
		if apiHandler != nil && (channel == "email_updated" || channel == "email_deleted") {
			var payload struct {
				AccountID string `json:"account_id"`
			}
			if err := json.Unmarshal([]byte(message), &payload); err == nil && payload.AccountID != "" {
				apiHandler.InvalidateEmailCache(ctx, payload.AccountID)
			}
		}

	})

	apiHandler = &api.Handler{
		Store:           store,
		Emails:          store,
		Writer:          store,
		Accounts:        store,
		Folders:         store,
		Entities:        store,
		Admin:           store,
		System:          store,
		TestConnLimiter: middleware.NewInMemoryRateLimiter(5),
		CAS:             cas,
		AI:              aiGateway,
		AIDisabled:      aiDisabled,
		OAuth:           oauthManager,
		EventBus:        eventBus,
		SyncManager:     syncMgr,
		PriorityChecker: sync.NewPriorityChecker(store, oauthManager),
	}
	if tgBot != nil {
		apiHandler.TGBot = tgBot
	}
	apiHandler.InitTicketStore()
	// In-memory cache fallback for Mono edition (no Redis).
	// When Redis is available, MemCache stays nil and all cache ops go through Redis.
	syncMgr.IsPaused = func(accountID string) bool {
		return apiHandler.IsAccountSyncPaused(context.Background(), accountID)
	}
	// Asynq task queue (Unified only — requires Redis)

	mux := http.NewServeMux()

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

	appURL := os.Getenv("NEXT_PUBLIC_APP_URL")
	corsWrapper := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// H2: Restrict CORS origins instead of returning *
			allowed := false
			if origin != "" {
				isDev := os.Getenv("APP_ENV") != "production"
				if (isDev && (origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" || origin == "http://localhost:3500" || origin == "http://127.0.0.1:3500" || origin == "http://localhost:8087")) || origin == appURL {
					allowed = true
				} else if allowedOrigins != "" {
					for _, o := range strings.Split(allowedOrigins, ",") {
						if strings.TrimSpace(o) == origin {
							allowed = true
							break
						}
					}
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, X-Token, X-Authorization")
			w.Header().Set("Access-Control-Expose-Headers", "X-Next-Cursor")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	securityHeaders := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if os.Getenv("APP_ENV") != "development" {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			isDev := os.Getenv("APP_ENV") != "production"
			// CSP connect-src: in dev, allow direct connections to backend/WS ports.
			// In production, 'self' covers same-origin API and SSE connections.
			connectSrc := "'self'"
			if isDev {
				connectSrc += " http://localhost:8087 ws://localhost:3500"
			}
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"connect-src "+connectSrc+"; "+
					"img-src 'self' data:; "+
					"style-src 'self' 'unsafe-inline'; "+
					"script-src 'self'; "+
					"frame-src 'none'; "+
					"object-src 'none'; "+
					"base-uri 'none'; "+
					"form-action 'self'",
			)
			next.ServeHTTP(w, r)
		})
	}

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	})

	mux.HandleFunc("/metrics", api.MetricsHandler)
	mux.HandleFunc("/api/stats", apiHandler.GetStats)

	// Login rate limiter: 5 requests per 60s per IP (brute-force protection)
	var loginLimiter func(http.Handler) http.Handler
	if loginLimiter == nil { loginLimiter = func(next http.Handler) http.Handler { return next } }
	loginHandler := loginLimiter(http.HandlerFunc(apiHandler.HandleLogin))
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler.ServeHTTP(w, r)
	})

	var aiRateLimit, searchRateLimit func(http.Handler) http.Handler
	if aiRateLimit == nil { noopMw := func(next http.Handler) http.Handler { return next }; aiRateLimit = noopMw; searchRateLimit = noopMw }
	mux.HandleFunc("/api/auth/status", apiHandler.HandleAuthStatus)
	mux.HandleFunc("/api/auth/edition", apiHandler.EditionInfo)
	mux.HandleFunc("/api/auth/verify", apiHandler.HandleVerifyToken)
	if !edition.IsMono() && !edition.IsMonoPro() {
		var setupLimiter func(http.Handler) http.Handler
		setupHandler := setupLimiter(http.HandlerFunc(apiHandler.HandleSetup))
		mux.HandleFunc("/api/auth/setup", func(w http.ResponseWriter, r *http.Request) {
			setupHandler.ServeHTTP(w, r)
		})
	}
	mux.HandleFunc("/api/auth/change-password", apiHandler.HandleChangePassword)
	mux.HandleFunc("/api/auth/ticket", apiHandler.HandleTicket)
	mux.HandleFunc("/api/auth/logout", apiHandler.HandleLogout)
	mux.HandleFunc("/api/auth/me", apiHandler.HandleGetMe)
	mux.HandleFunc("/api/auth/scan-local", apiHandler.HandleScanLocal)
	mux.HandleFunc("/api/auth/import-local", apiHandler.HandleImportLocal)
	mux.HandleFunc("/api/mail/resolve", apiHandler.ResolveMailServer)
	mux.HandleFunc("/api/user/telegram", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			apiHandler.GetTelegramSettings(w, r)
		} else if r.Method == http.MethodPost {
			apiHandler.UpdateTelegramSettings(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/events", apiHandler.SSE)
	mux.HandleFunc("/mcp/sse", apiHandler.MCPSSE)
	mux.HandleFunc("/mcp/messages", apiHandler.MCPMessages)
	mux.HandleFunc("/api/emails/restore/", apiHandler.RestoreFromTrash)
	mux.HandleFunc("/api/emails/", apiHandler.HandleEmail)
	mux.HandleFunc("/api/emails/bulk", apiHandler.BulkAction)
	mux.HandleFunc("/api/emails/ids", apiHandler.GetEmailIDs)
	mux.HandleFunc("/api/emails/count", apiHandler.GetEmailCount)
	mux.HandleFunc("/api/emails", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			apiHandler.HandleEmail(w, r)
		} else {
			apiHandler.GetEmails(w, r)
		}
	})
	mux.HandleFunc("/api/accounts/", apiHandler.HandleAccount)
	mux.HandleFunc("/api/accounts/test-connection", apiHandler.HandleTestAccountConnection)
	mux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			apiHandler.HandleAccount(w, r)
		} else {
			apiHandler.GetAccounts(w, r)
		}
	})
	mux.HandleFunc("/api/folders", apiHandler.GetFolders)
	mux.HandleFunc("/api/folders/", apiHandler.HandleFolder)
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		searchRateLimit(http.HandlerFunc(apiHandler.Search)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ai/chat", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(apiHandler.AIChat)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ai/models", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(apiHandler.AIModels)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ai/categorize", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(apiHandler.AICategorize)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/attachments/upload", apiHandler.UploadAttachment)
	mux.HandleFunc("/api/attachments/", apiHandler.GetAttachment)
	mux.HandleFunc("/internal/attachment/", apiHandler.GetAttachment)
	if !edition.IsMono() && !edition.IsMonoPro() {
		mux.HandleFunc("/api/oauth/url", apiHandler.GetOAuthURL)
		mux.HandleFunc("/api/oauth/callback", apiHandler.HandleOAuthCallback)
		mux.HandleFunc("/api/system/oauth", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				apiHandler.GetOAuthSettings(w, r)
			} else if r.Method == http.MethodPost {
				apiHandler.UpdateOAuthSettings(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
	}
	mux.HandleFunc("/api/system/ai-categories", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			apiHandler.GetAICategories(w, r)
		} else if r.Method == http.MethodPut {
			apiHandler.UpdateAICategories(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/emails/send", apiHandler.SendEmail)
	mux.HandleFunc("/api/emails/send/cancel/", apiHandler.CancelSend)
	mux.HandleFunc("/api/templates", apiHandler.GetTemplates)
	mux.HandleFunc("/api/templates/create", apiHandler.CreateTemplate)
	mux.HandleFunc("/api/templates/delete/", apiHandler.DeleteTemplate)
	mux.HandleFunc("/api/emails/draft", apiHandler.SaveStandaloneDraft)
	mux.HandleFunc("/api/contacts", apiHandler.HandleContacts)
	mux.HandleFunc("/api/notes", apiHandler.CreateNote)
	// Labels
	mux.HandleFunc("/api/labels", apiHandler.GetLabels)
	mux.HandleFunc("/api/labels/create", apiHandler.CreateLabel)
	mux.HandleFunc("/api/labels/update/", apiHandler.UpdateLabel)
	mux.HandleFunc("/api/labels/delete/", apiHandler.DeleteLabel)
	mux.HandleFunc("/api/emails/labels", apiHandler.SetEmailLabels)
	mux.HandleFunc("/api/email-labels/", apiHandler.GetEmailLabels)
	mux.HandleFunc("/api/email-labels/batch", apiHandler.GetBatchEmailLabels)
	mux.HandleFunc("/api/email-tags/batch", apiHandler.GetBatchEmailTags)
	// Rules
	mux.HandleFunc("/api/rules", apiHandler.GetRules)
	mux.HandleFunc("/api/rules/create", apiHandler.CreateRule)
	mux.HandleFunc("/api/rules/update/", apiHandler.UpdateRule)
	mux.HandleFunc("/api/rules/delete/", apiHandler.DeleteRule)
	// Webhooks
	mux.HandleFunc("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			apiHandler.CreateWebhook(w, r)
		} else {
			apiHandler.GetWebhooks(w, r)
		}
	})
	mux.HandleFunc("/api/webhooks/delete/", apiHandler.DeleteWebhook)
	// Groups (Unified/Teams only, not Mono)
	if !edition.IsMono() && !edition.IsMonoPro() {
		mux.HandleFunc("/api/groups", apiHandler.GetGroups)
		mux.HandleFunc("/api/groups/create", apiHandler.CreateGroup)
		mux.HandleFunc("/api/groups/update/", apiHandler.UpdateGroup)
		mux.HandleFunc("/api/groups/delete/", apiHandler.DeleteGroup)
		mux.HandleFunc("/api/groups/accounts", apiHandler.SetGroupAccounts)
		mux.HandleFunc("/api/groups/accounts/", apiHandler.GetGroupAccounts)
	}
	mux.HandleFunc("/api/media/proxy", apiHandler.MediaProxy)
	// Comments — available for all editions
	mux.HandleFunc("/api/comments/", apiHandler.GetComments)
	mux.HandleFunc("/api/comments/create", apiHandler.CreateComment)
	mux.HandleFunc("/api/comments/delete/", apiHandler.DeleteComment)
	// Teams-only features
	if edition.IsTeams() {
		mux.HandleFunc("/api/users", apiHandler.GetUsers)
		mux.HandleFunc("/api/users/create", apiHandler.CreateUser)
		mux.HandleFunc("/api/users/delete/", apiHandler.DeleteUser)
		mux.HandleFunc("/api/emails/assign", apiHandler.AssignEmail)
		mux.HandleFunc("/api/emails/unassign", apiHandler.UnassignEmail)
		mux.HandleFunc("/api/dashboard", apiHandler.GetSharedDashboard)
		mux.HandleFunc("/api/emails/status/", apiHandler.SetEmailStatus)
	}
	// Identities (Send-As) — available for all editions
	mux.HandleFunc("/api/identities", apiHandler.HandleIdentities)
	mux.HandleFunc("/api/identities/", apiHandler.HandleIdentities)
	// Telegram / WhatsApp webhooks
	mux.HandleFunc("/api/tg/webhook", apiHandler.TGWebhook)
	mux.HandleFunc("/api/wa/webhook", apiHandler.WAWebhook)
	mux.HandleFunc("/api/ai/stats", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				apiHandler.ResetAIStats(w, r)
			} else {
				apiHandler.GetAIStats(w, r)
			}
		})).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ai/log", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(apiHandler.GetAILog)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ai/settings", func(w http.ResponseWriter, r *http.Request) {
		aiRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				apiHandler.GetAISettings(w, r)
			} else {
				apiHandler.UpsertAISettings(w, r)
			}
		})).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/mcp/keys", apiHandler.ListMCPKeys)
	mux.HandleFunc("/api/mcp/keys/create", apiHandler.CreateMCPKey)
	mux.HandleFunc("/api/mcp/keys/delete/", apiHandler.DeleteMCPKey)
	mux.HandleFunc("/api/mcp/keys/toggle/", apiHandler.ToggleMCPKey)
	mux.HandleFunc("/api/mcp/keys/view/", apiHandler.ViewMCPKey)
	mux.HandleFunc("/api/mcp/connect", apiHandler.MCPConnectInfo)
	if edition.IsMonoPro() || edition.IsTeams() {
		mux.HandleFunc("/api/admin/settings", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				apiHandler.GetAdminSettings(w, r)
			} else if r.Method == http.MethodPost {
				apiHandler.UpdateAdminSettings(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				apiHandler.GetAdminUsers(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
	}

	// Asynq monitoring dashboard (Unified only, Redis required)

	i18nMiddleware := middleware.NewI18nMiddleware()

	// Limit request body size to 30MB for all API endpoints (covers Dropzone 25MB + multipart overhead)
	maxBodySize := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				r.Body = http.MaxBytesReader(w, r.Body, 30<<20)
			}
			next.ServeHTTP(w, r)
		})
	}

	// Apply JWT auth middleware to all /api/ routes.
	// Public endpoints (login, health, oauth, webhooks) are handled by PublicEndpoints map.
	var handler http.Handler = i18nMiddleware.Handler(mux)
	handler = middleware.AIDisableMiddleware(aiDisabled, handler)
	handler = middleware.JWTAuthMiddleware(handler)
	handler = middleware.SentryMiddleware(handler)

	panicRecovery := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("Panic recovered in HTTP handler", "error", err, "path", r.URL.Path)
					sentry.CaptureException(fmt.Errorf("panic: %v", err))
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
	handler = panicRecovery(handler)

	handler = maxBodySize(handler)
	handler = corsWrapper(handler)
	handler = securityHeaders(handler)

	// Apply Gzip compression — skip SSE endpoints (gzip wrapper breaks http.Flusher)
	preGzipHandler := handler
	gzipHandler := gzhttp.GzipHandler(preGzipHandler)
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/events" || r.URL.Path == "/mcp/sse" {
			// SSE requires http.Flusher; gzip wrapper doesn't preserve it.
			preGzipHandler.ServeHTTP(w, r)
			return
		}
		gzipHandler.ServeHTTP(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
	}

	server := &http.Server{
		Addr:           "0.0.0.0:" + port,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   0, // SSE connections live indefinitely
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	go func() {
		slog.Info("starting server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen error", "error", err)
			os.Exit(1)
		}
	}()

	// Background: resolve avatars for all existing senders (Unified only)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic", "area", "backfill_avatars", "error", r)
			}
		}()
	}()

	// Mono: periodic account discovery sync
	if edition.IsMono() || edition.IsMonoPro() {
		accountSync := sync.NewAccountSync(store)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic", "area", "account_sync", "error", r)
				}
			}()
			accountSync.Start(ctx)
		}()
	}

	// Background: unsnooze emails
	snoozeWorker := sync.NewSnoozeWorker(store, func(ctx context.Context, accountID, emailID string) {
		msg := fmt.Sprintf(`{"account_id":"%s","email_id":"%s"}`, accountID, emailID)
		eventBus.Publish("new-email", msg)
	})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic", "area", "snooze_worker", "error", r)
			}
		}()
		snoozeWorker.Start(ctx)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	slog.Info("shutting down server")

	// 1. Stop accepting new requests and drain in-flight connections
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}

	// 2. Stop Asynq task server first to let in-flight tasks (SMTP, webhooks) complete

	// 3. Stop all IMAP sync workers (with wait for goroutine drain)
	syncMgr.StopAll()

	// 3. Close Redis connection

	// 5. Cleanup unused attachments
	if cas != nil {
		if deleted, err := cas.DeleteUnused(context.Background()); err != nil {
			slog.Error("failed to cleanup unused attachments", "error", err)
		} else if deleted > 0 {
			slog.Info("cleaned up unused attachments", "count", deleted)
		}
	}

	slog.Info("server exiting")
}

type telegramAIAdapter struct {
	gateway *ai.Gateway
	store   api.Store
}

func (a *telegramAIAdapter) ChatWithTools(ctx context.Context, accountID string, messages []chatbot.AIMessage, tools []interface{}) (chatbot.AIMessage, error) {
	providerName := "openrouter" // default
	model := ""
	apiKey := ""

	// 1. Try account-specific ai_settings, then global
	var setting *models.AISetting
	var err error
	if a.store != nil {
		if accountID != "" {
			setting, err = a.store.GetAISettings(ctx, accountID)
		}
		if err != nil || setting == nil || setting.Preset == "" {
			setting, err = a.store.GetAISettings(ctx, "00000000-0000-0000-0000-000000000000")
		}

		if err == nil && setting != nil {
			// Parse Config JSON for chat provider/model
			if setting.Config != "" {
				var cfg map[string]map[string]interface{}
				if json.Unmarshal([]byte(setting.Config), &cfg) == nil {
					if chatCfg, ok := cfg["chat"]; ok {
						if p, ok := chatCfg["provider"].(string); ok && p != "" {
							providerName = p
						}
						if m, ok := chatCfg["model"].(string); ok && m != "" {
							model = m
						}
					}
				}
			}

			// Decrypt API keys (ai_settings) - try all encryption keys
			encKeyRaw := os.Getenv("ENCRYPTION_KEYS")
			if encKeyRaw == "" {
				encKeyRaw = os.Getenv("ENCRYPTION_KEY")
			}
			if encKeyRaw != "" && setting.APIKeysEncrypted != "" {
				for _, keyStr := range strings.Split(encKeyRaw, ",") {
					keyStr = strings.TrimSpace(keyStr)
					if keyStr == "" {
						continue
					}
					dec, err := crypto.Decrypt(setting.APIKeysEncrypted, []byte(keyStr))
					if err == nil {
						var keys map[string]string
						if json.Unmarshal([]byte(dec), &keys) == nil {
							if k, ok := keys[providerName]; ok && k != "" {
								apiKey = k
							}
						}
						break
					}
				}
			}
		}
	}

	// 2. Enterprise override: env var takes precedence over ai_settings
	if envKey := os.Getenv(ai.ProviderEnvKey(providerName)); envKey != "" {
		apiKey = envKey
	}

	aiMessages := make([]ai.Message, len(messages))
	for i, m := range messages {
		var toolCalls []ai.ToolCall
		if m.ToolCalls != nil {
			if tcBytes, err := json.Marshal(m.ToolCalls); err == nil {
				json.Unmarshal(tcBytes, &toolCalls)
			}
		}
		aiMessages[i] = ai.Message{
			Role:             m.Role,
			Content:          m.Content,
			Name:             m.Name,
			ToolCallID:       m.ToolCallID,
			ToolCalls:        toolCalls,
			ReasoningContent: m.ReasoningContent,
		}
	}

	aiTools := make([]ai.Tool, len(tools))
	for i, t := range tools {
		if tBytes, err := json.Marshal(t); err == nil {
			json.Unmarshal(tBytes, &aiTools[i])
		}
	}

	res, err := a.gateway.ChatWithTools(ctx, providerName, model, apiKey, aiMessages, aiTools)
	if err != nil {
		return chatbot.AIMessage{}, err
	}

	var resToolCalls interface{}
	if len(res.ToolCalls) > 0 {
		resToolCalls = res.ToolCalls
	}

	return chatbot.AIMessage{
		Role:             res.Role,
		Content:          res.Content,
		Name:             res.Name,
		ToolCallID:       res.ToolCallID,
		ToolCalls:        resToolCalls,
		ReasoningContent: res.ReasoningContent,
	}, nil
}

func (a *telegramAIAdapter) Chat(ctx context.Context, accountID string, messages []chatbot.AIMessage) (string, error) {
	providerName := "openrouter" // default
	model := ""
	apiKey := ""

	// 1. Try account-specific ai_settings, then global
	var setting *models.AISetting
	var err error
	if a.store != nil {
		if accountID != "" {
			setting, err = a.store.GetAISettings(ctx, accountID)
		}
		if err != nil || setting == nil || setting.Preset == "" {
			setting, err = a.store.GetAISettings(ctx, "00000000-0000-0000-0000-000000000000")
		}

		if err == nil && setting != nil {
			// Parse Config JSON for chat provider/model
			if setting.Config != "" {
				var cfg map[string]map[string]interface{}
				if json.Unmarshal([]byte(setting.Config), &cfg) == nil {
					if chatCfg, ok := cfg["chat"]; ok {
						if p, ok := chatCfg["provider"].(string); ok && p != "" {
							providerName = p
						}
						if m, ok := chatCfg["model"].(string); ok && m != "" {
							model = m
						}
					}
				}
			}

			// Decrypt API keys (ai_settings) - try all encryption keys
			encKeyRaw := os.Getenv("ENCRYPTION_KEYS")
			if encKeyRaw == "" {
				encKeyRaw = os.Getenv("ENCRYPTION_KEY")
			}
			if encKeyRaw != "" && setting.APIKeysEncrypted != "" {
				for _, keyStr := range strings.Split(encKeyRaw, ",") {
					keyStr = strings.TrimSpace(keyStr)
					if keyStr == "" {
						continue
					}
					dec, err := crypto.Decrypt(setting.APIKeysEncrypted, []byte(keyStr))
					if err == nil {
						var keys map[string]string
						if json.Unmarshal([]byte(dec), &keys) == nil {
							if k, ok := keys[providerName]; ok && k != "" {
								apiKey = k
							}
						}
						break
					}
				}
			}
		}
	}

	// 2. Enterprise override: env var takes precedence over ai_settings
	if envKey := os.Getenv(ai.ProviderEnvKey(providerName)); envKey != "" {
		apiKey = envKey
	}

	// 2. Fallback to global env configs
	if providerName == "" {
		providerName = os.Getenv("DEFAULT_AI_PROVIDER")
		if providerName == "" {
			providerName = "openai"
		}
	}

	effectiveKey := apiKey
	if effectiveKey == "" {
		switch providerName {
		case "openrouter":
			effectiveKey = os.Getenv("OPENROUTER_API_KEY")
		case "openai":
			effectiveKey = os.Getenv("OPENAI_API_KEY")
		case "anthropic":
			effectiveKey = os.Getenv("ANTHROPIC_API_KEY")
		case "gemini":
			effectiveKey = os.Getenv("GEMINI_API_KEY")
		case "deepseek":
			effectiveKey = os.Getenv("DEEPSEEK_API_KEY")
		case "groq":
			effectiveKey = os.Getenv("GROQ_API_KEY")
		case "xai":
			effectiveKey = os.Getenv("XAI_API_KEY")
		case "qwen":
			effectiveKey = os.Getenv("QWEN_API_KEY")
		}
	}

	aiMsgs := make([]ai.Message, len(messages))
	for i, m := range messages {
		aiMsgs[i] = ai.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	// 3. Dispatch to the right API using CallOpenAICompatChat or global provider
	switch providerName {
	case "openrouter":
		if model == "" {
			model = os.Getenv("OPENROUTER_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://openrouter.ai/api/v1/chat/completions", model, effectiveKey, aiMsgs)
	case "openai":
		if model == "" {
			model = os.Getenv("OPENAI_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.openai.com/v1/chat/completions", model, effectiveKey, aiMsgs)
	case "deepseek":
		if model == "" {
			model = os.Getenv("DEEPSEEK_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.deepseek.com/chat/completions", model, effectiveKey, aiMsgs)
	case "groq":
		if model == "" {
			model = os.Getenv("GROQ_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.groq.com/openai/v1/chat/completions", model, effectiveKey, aiMsgs)
	case "xai":
		if model == "" {
			model = os.Getenv("XAI_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://api.x.ai/v1/chat/completions", model, effectiveKey, aiMsgs)
	case "qwen":
		if model == "" {
			model = os.Getenv("QWEN_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", model, effectiveKey, aiMsgs)
	case "ollama":
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			url = "http://localhost:11434"
		}
		if model == "" {
			model = os.Getenv("OLLAMA_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, url+"/v1/chat/completions", model, "", aiMsgs)
	case "opencode":
		url := os.Getenv("OPENCODE_URL")
		if url == "" {
			url = "http://localhost:4312"
		}
		if model == "" {
			model = os.Getenv("OPENCODE_MODEL")
		}
		return ai.CallOpenAICompatChat(ctx, url+"/v1/chat/completions", model, effectiveKey, aiMsgs)
	default:
		// Fallback to globally registered provider, apply resolved key
		provider, ok := a.gateway.GetProvider(providerName)
		if !ok {
			for _, p := range a.gateway.Providers() {
				provider = p
				ok = true
				break
			}
		}
		if !ok || provider == nil {
			return "", fmt.Errorf("no AI providers configured")
		}
		// Apply resolved key to provider for gemini/anthropic etc.
		if effectiveKey != "" {
			switch p := provider.(type) {
			case *ai.GeminiProvider:
				p.APIKey = effectiveKey
			case *ai.AnthropicProvider:
				p.APIKey = effectiveKey
			case *ai.OllamaProvider:
				// no API key needed
			}
		}
		return provider.Chat(ctx, aiMsgs)
	}
}
