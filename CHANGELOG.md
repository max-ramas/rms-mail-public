# Changelog

All notable changes to RMS Mail will be documented in this file.

## [3.0.2] — 2026-06-10 (pre-release)

### Changed

#### Full-Text Search — Bluge → SQLite FTS5 + PostgreSQL tsvector
Major architectural refactoring: replaced the Bluge full-text search engine with native database solutions.

- **SQLite FTS5 (Mono edition)**: `emails_fts` virtual table with `account_id` isolation, `sanitizeFTSQuery()` injection prevention, auto-reindex on startup (`ReindexFTS()`), DELETE-before-INSERT dedup pattern, FTS cleanup on `DeleteEmail`/`DeleteEmailsInFolder`
- **PostgreSQL tsvector (Unified edition)**: GIN-indexed `fts tsvector` column computed inline in `SaveEmail`/`SaveEmailToFolder`, search via `plainto_tsquery('english', $1) ORDER BY ts_rank(...)`, replaces `ILIKE %word%`
- **SQL LIKE/ILIKE fallback**: preserved as secondary path when FTS returns no results

### Removed

- **Bluge**: `bluge_index.go` (293 lines), 4 dependencies (`bluge v0.2.2`, `bluge_segment_api`, `ice`, `ice/v2`), `detectLanguage()` function, `storage/index/` directory
- **Binary size**: −4MB (Bluge + ice dependencies)
- **Handler struct**: removed `Index *index.BlugeIndex` field
- **Fetcher/Manager**: removed `Index` field, removed `idx` parameter from constructors
- **Dead code**: `sanitizedHTML` variable (Bluge-only), `bluemondayPolicy` local declarations, `initialSync()` function
- **Dead components**: `mail-sidebar.tsx`, `shortcuts-modal.tsx`, `attachment-preview.tsx`, `quick-preview.tsx`
- **Console debug**: removed `console.log("MESSAGES KEYS:", ...)` debug leftover in `i18n/request.ts`
- **Login page**: removed "Advanced Settings" (IMAP/SMTP fields), `handleEmailBlur` auto-resolution
- **Edition T stubs**: `shared-dashboard.tsx`, `useDashboard.ts`, `compose-form.tsx` marked with `@todo`

### Security

#### Cross-Account Data Isolation (14 endpoints)
Access control gaps allowed any authenticated user to read/write data across accounts. All now enforce `CheckAccountAccess`.

| Endpoint | Fix |
|----------|-----|
| `POST /api/templates/create`, `/api/labels/create`, `/api/rules/create`, `/api/webhooks` | `CheckAccountAccess(req.AccountID)` |
| `PUT /api/rules/update/{id}` | `GetRule(id)` → verify `existing.AccountID` (blocks body-parameter-spoofing attack) |
| `PUT /api/labels/update/{id}` | `GetLabel(id)` → verify ownership before update |
| `DELETE /api/labels/delete/{id}`, `/api/templates/delete/{id}`, `/api/rules/delete/{id}`, `/api/comments/delete/{id}` | Fetch entity via new singular getters → `CheckAccountAccess(entity.AccountID)` |
| `GET /api/emails/{id}/tags`, `POST /api/emails/labels`, `GET /api/email-labels/{emailId}`, `POST /api/emails/assign`, `/unassign` | Email lookup → `CheckAccountAccess(email.AccountID)` |
| `markEmailRead`, `toggleFlagEmail`, `togglePinEmail`, `toggleMuteEmail`, `snoozeEmail`, `saveDraftReply`, `clearDraftReply`, `downloadRawEmail` | New `checkEmailAccess(r)` helper via query-param `account_id` |
| `DELETE /api/folders/{id}` | `GetFolderByID(id)` → `CheckAccountAccess(folder.AccountID)` |
| `POST/PUT/DELETE /api/contacts` | `AccountID` added to `models.Contact` + `ALTER TABLE contacts ADD COLUMN` migration |
| `GET /api/mcp/connect` | `GetMCPKey(id)` → `CheckAccountAccess(key.AccountID)` before revealing full key |

#### Admin-Only Endpoint Enforcement
Endpoints that operate on shared infrastructure now require admin privileges via `requireAdmin()`.

| Endpoint | Risk |
|----------|------|
| `GET/POST /api/system/oauth` | Any user could read/write OAuth client credentials |
| `GET /api/ai/stats`, `/api/ai/log`, `DELETE /api/ai/stats` | Global AI usage data leaked across accounts |
| `GET/POST /api/users`, `/api/users/create`, `/api/users/delete/{id}` | User CRUD unrestricted |
| `POST /api/groups/create`, `PUT /api/groups/update/{id}`, `DELETE /api/groups/delete/{id}`, `POST /api/groups/accounts` | Group management unrestricted |

#### New Store Interface Methods (6 singular getters)
`GetLabel`, `GetTemplate`, `GetRule`, `GetComment`, `GetContact`, `GetMCPKey` — implemented in both PostgreSQL and SQLite backends with mock coverage in `handlers_test.go`.

#### Authentication & Authorization (Round 4.2)
- `deleteOldDraft` now performs `SELECT Drafts` before STORE/UIDExpunge — old drafts properly deleted
- 6 AI handlers fixed: `CheckAccountAccess` errors no longer silently swallowed
- `JWT_SECRET` required and independent of `ENCRYPTION_KEY` — no key material reuse
- `CheckAccountAccess` added to 8 handlers: `SendEmail`, `UpsertAISettings`, `SaveStandaloneDraft`, `getAccount`, `updateAccount`, `deleteAccount`, `resetAccountSync`, `CancelSend`
- `WebhookDelete` checks ownership via `GetWebhook` + `CheckAccountAccess`
- `is_admin: true` claim embedded in JWT at login — `IsAdminFromContext()` reads from context (0 DB queries)

#### IMAP Sync & Data Integrity
- `SyncAllFolders` continues syncing remaining folders on error, returns first error
- `ON CONFLICT DO UPDATE` properly syncs `uid`, `folder_id`, `snippet`, `body_path` (Postgres + SQLite)
- Sync errors from `syncDrafts`/`syncMoves` propagated to `account.last_sync_error`
- `context.Background()` replaced with `ctx` in sync workers, fetcher, job_worker
- Empty `Message-ID` → synthesized `local-{UID}@{accountID}.synced` prevents silent drops
- `ON CONFLICT WHERE` clause fixed — adds `folder_id IS DISTINCT FROM` check
- Draft deletion moved out of scheduler path — undo-send preserves drafts

#### Encryption & Secrets
- **CRITICAL**: `crypto.GetPrimaryEncryptionKey()` reads `ENCRYPTION_KEYS` first, falls back to `ENCRYPTION_KEY`. All 11 call sites updated (was using `os.Getenv("ENCRYPTION_KEY")` directly, ignoring key rotation config)
- OAuth tokens encrypted in DB (AES-GCM)
- Telegram bot token encrypted at rest (Postgres + SQLite)
- `decryptPassword` with nil `encKey` returns error — no silent empty passwords
- `ENCRYPTION_KEY` hardcoded fallback removed from Camo proxy — `CAMO_HMAC_KEY` required via env
- `JWT_SECRET` and `CAMO_HMAC_KEY` documented in all 3 `.env` example files
- Database connections: `sslmode=disable` → `sslmode=prefer` in all configs

#### License Management
- **ZK-proof ping**: `license_id = SHA256(key)` + `signature = HMAC-SHA256(nonce, key)` — raw key never transmitted
- Ed25519 public key injected via ldflags at build time — mandatory for non-Mono in production (`os.Exit(1)`)
- HMAC-SHA256 integrity signature over critical `system_settings` fields (DB tamper protection)
- `expires_at` enforced on ping response — expired server responses rejected
- `HandleOAuthCallback` calls `CheckLimit("account", ...)` — prevents unlimited OAuth account creation
- `POST/DELETE /api/license` requires admin privileges
- **Synchronous Ping & Error Feedback**: `POST /api/license` now performs a synchronous ping, catching and returning detailed signature/network errors directly to the frontend instead of generic `Unlicensed` status.
- **HEX Public Key Enforced**: `LICENSE_PUBLIC_KEY` must be a 32-byte HEX string, resolving production activation failures caused by incorrect formats.

#### API Input Validation
- CRLF injection: `stripCRLF()` on Subject/InReplyTo/References in `SendEmail`, notes, job_worker
- Gemini API key moved from URL query to `x-goog-api-key` header
- API key debug logs stripped — only length logged
- SSRF check at webhook creation time (early rejection, not just at dispatch)
- `http.ServeFile` validates path prefix (`storage/`) — path traversal protection
- `AIChat` message limits: 50 messages, 100K chars total

#### Webhooks & Notifications
- HMAC signature preserved on webhook retry via `OriginalSecret` in `requeueWebhook`
- HTML injection in Telegram notifications: `htmlEscape()` on Subject/SenderName/Snippet
- `GetAnyTelegramSettings` cross-account notification leak removed
- TG webhook `secret_token` SHA-256 verification prevents spoofed updates
- Trash folder detection expanded: +4 locales

#### Rate Limiting — Redis (3 tiers, isolated prefixes)
- `ratelimit:global:{IP}` (60/min), `ratelimit:login:{IP}` (5/min), `ratelimit:ai:{IP}` (30/min)
- Login brute-force protection fully isolated from API/AI traffic
- Login + AI rate limiters Redis-backed when available — multi-instance safe
- In-memory fallback for all tiers when Redis is unavailable
- AI rate limiter now covers `/api/ai/models` in addition to chat/categorize/settings/stats/log

### Fixed

#### Redis Reliability
- **Goroutine leak prevention**: `Incr`, `Expire`, `Publish`, `ZAdd`, `ZRem` in `cache/redis.go` now accept `ctx context.Context` with internal 3s timeout — no unbounded hangs on Redis lag
- **At-Least-Once delivery**: `ZPop` replaced with `ZClaim` (Lua: ZRANGEBYSCORE + ZADD bump `now + 5min`, ZRem on success). Job survives process crash — re-picked after 5min visibility timeout
- **Redis persistence**: `--appendonly yes` + named volume `redisdata:/data` + healthcheck in both compose files

#### Search (FTS5/tsvector)
- Search handler nil-panic: `h.Index` was nil in Mono — Bluge removal eliminates the bug entirely
- FTS5 account isolation: added `account_id` column + WHERE filter in `SearchFTS`
- FTS5 query injection: `sanitizeFTSQuery()` strips all 8 FTS5 special characters (`*`, `"`, `'`, `-`, `(`, `)`, `:`, `NEAR/`)
- FTS5 duplicates: DELETE-before-INSERT in `IndexEmailFTS`
- FTS5 orphan cleanup: `DeleteEmail` + `DeleteEmailsInFolder` remove from `emails_fts`
- PostgreSQL `CREATE VIRTUAL TABLE` warning: FTS5 SQL moved to SQLite-only `InitSchema`

#### IMAP & Data
- `UploadAttachment` fd leak: `defer f.Close()` moved inside loop — 100 files no longer hold 100 open descriptors
- Undo-send race: re-check `GetScheduledEmail` before `SendMsg` — cancel between fetch and send aborts
- `AttachAvatars`, `GetEmailAttachments`, `GetEmailIDs`: added missing `rows.Err()` checks
- `move.RemoteUID == 0` now calls `FailIMAPMove` instead of `CompleteIMAPMove`

#### Performance
- `bluemonday.UGCPolicy()` → package-level `var ugcPolicy`: regex compiled once at startup
- `sanitizer.NewEmailSanitizer()` → package-level `var emailSanitizer`: single instance
- Redis `KEYS` → `SCAN` with 100-key cursor: no event loop blocking
- `time.After` → `time.NewTimer` + `defer timer.Stop()` in `startIdleMonitor`: no goroutine leaks
- `camoSign()` HMAC: `sync.Map` cache — same avatar URL = 1 HMAC computation
- `zstd.NewWriter(nil)` → `sync.Pool` with `Reset(nil)`: zero allocs during sync
- `ON CONFLICT DO UPDATE` with WHERE clause: skips 15-column rewrite on unchanged emails
- Connection pool lifecycle: `MinConns=2`, `MaxConnLifetime=1h`, `MaxConnIdleTime=30m`, `HealthCheckPeriod=1m`

#### Frontend
- API keys removed from `localStorage` — persisted to server only, `hasSavedKey` indicator
- `console.error`/`console.warn` guarded by `NODE_ENV === "development"` (17 locations)
- `setTimeout` cleanup added to 3 components (login, setup, auth-guard)
- Array index keys replaced with stable IDs in 5 components
- Mutation cache invalidation in `useSaveAISettings`, `useSetEmailStatus`, `useSendEmail`
- `prefers-reduced-motion` CSS media query (WCAG 2.3.3)
- SSE uses relative `/api/events` URL with optional `?token=` — same-origin through Next.js proxy
- `[SSE]` debug logging with `readyState` tracking in `onopen`/`onerror`

#### Reverse Proxy & Infrastructure
- `extractClientIP()`: `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`
- OpenResty nginx: SSE timeouts, `X-Forwarded-Proto/Host` headers, `proxy_buffering off`
- `Strict-Transport-Security` gated behind `APP_ENV != "development"`
- `docker-compose.yml`: hardcoded credentials replaced with `${PG_USER}`/`${PG_PASSWORD}`
- `run-m.sh` no longer starts PostgreSQL and Redis (Mono = SQLite only)
- Webhook delete route: frontend path fixed to match backend `/api/webhooks/delete/{id}`

#### Storage & Migrations
- PostgreSQL `ALTER TABLE emails ADD COLUMN IF NOT EXISTS fts tsvector` + GIN index
- `ALTER TABLE contacts ADD COLUMN IF NOT EXISTS account_id UUID`
- `ALTER TABLE mcp_keys ADD COLUMN IF NOT EXISTS account_id UUID`
- SQLite `ON CONFLICT DO UPDATE` now matches Postgres column set
- `GetStatsByAgent` N+1 → single `GROUP BY assigned_to` query
- CAS `GetStats` math fixed: `uniqueSize` instead of broken `deduped * 100 / raw`
- Camo cache: 500MB max, deletes oldest files when exceeded
- Postgres `MaxConns` configurable via `PG_MAX_CONNS`

### Added

- **FTS5 full-text search** (Mono): `emails_fts` virtual table, `IndexEmailFTS`, `SearchFTS`, `ReindexFTS`, `sanitizeFTSQuery`
- **PostgreSQL tsvector** (Unified): GIN-indexed `fts` column, `plainto_tsquery` search with `ts_rank` ordering
- **`crypto.GetPrimaryEncryptionKey()`**: shared utility for `ENCRYPTION_KEYS` key rotation
- **`checkEmailAccess(r)`**: helper for email action handler auth
- **`requireAdmin()`**: reusable admin privilege check in handler methods
- **Singular Store getters**: `GetLabel`, `GetTemplate`, `GetRule`, `GetComment`, `GetContact`, `GetMCPKey`
- **Redis**: `ZClaim` (visibility timeout pattern), `ZMember` struct, AOF persistence + healthcheck
- **Build script**: `bp-t.sh` for Teams edition Docker build+push
- **Node 26**: `.nvmrc` + `engines.node >= 26` + `--no-deprecation` flag
- **Dependencies**: Node 26.3.0, Redis 8-alpine, Alpine 3.22, Go 1.26.3
- **Bulk-by-Filter**: `POST /api/emails/bulk` filter-based mode — when `ids` is empty and `account_id` is present, operations use direct SQL `UPDATE` (no 250K-element JSON array). Removes the 10K `LIMIT` bottleneck on "Select All" + Delete/Archive/Read/Flag for 200K+ inboxes. Includes `/api/emails/count` lightweight endpoint, `GetAccountIDsByFilter` for unified per-account grouping, chunked IMAP enqueue (500/batch)

### Production Debugging — June 10, 2026

#### IMAP Sync — Batching (200K+ inboxes)
- **HIGH**: First sync fetched ALL UIDs in one IMAP command — 5000+ UIDs timed out at Gmail's 60s limit. No emails were saved.
- **FIX**: `syncFolderByUID` now fetches in 500-UID batches. Progress saved after each batch — survives connection drops and container restarts. Subsequent syncs are incremental (UID-based).

#### UTF-8 Encoding — Cross-Language Sanitization
- **HIGH**: Non-UTF-8 email content (Windows-1251 Cyrillic, ISO-2022-JP, etc.) caused `SQLSTATE 22021 — invalid byte sequence for encoding "UTF8"` on every INSERT. Emails silently dropped.
- **FIX**: `strings.ToValidUTF8()` applied to ALL text fields before PostgreSQL INSERT: `Subject`, `SenderName`, `SenderAddress`, `RecipientAddress`, `CcAddress`, `InReplyTo`, `Snippet`, `FromAddr`, `MsgID`. Invalid byte sequences stripped, valid UTF-8 passes through unchanged. Works for all 45 languages.

#### PostgreSQL ON CONFLICT — Missing Unique Constraint
- **HIGH**: `ON CONFLICT (msg_id, account_id)` had no matching unique constraint on partitioned `emails` table. Upgrade migrations dropped the index. Every `SaveEmail`/`SaveEmailToFolder` failed with `SQLSTATE 42P10`.
- **FIX**: Added `CREATE UNIQUE INDEX IF NOT EXISTS emails_msg_id_account_key ON emails (msg_id, account_id)` to `schema.sql`. Survives re-runs.

#### PostgreSQL — to_tsvector Parameter Type Ambiguity
- **HIGH**: `to_tsvector('english', $5 || ' ' || $6 || ...)` reused parameters in both VALUES and `||` context. pgx inferred conflicting types → `SQLSTATE 42P08 — inconsistent types deduced for parameter $7`.
- **FIX**: `ftsText` computed in Go (`email.Subject + " " + email.SenderName + ...`), passed as single `$20`/`$21` parameter to `to_tsvector('english', $20)`.

#### Gmail OAuth — Missing IMAP Scope
- **HIGH**: XOAUTH2 authentication looped indefinitely — token refreshed successfully but Gmail IMAP rejected it: `[AUTHENTICATIONFAILED] Invalid credentials`.
- **FIX**: Changed OAuth scope from `gmail.readonly` + `gmail.modify` to `https://mail.google.com/` (full Gmail IMAP access). Requires re-authorization of existing Gmail accounts.

#### OAuth Callback — Silent Token Save Failures
- **HIGH**: `UpdateAccountTokens` errors logged but never surfaced to user. Account created without tokens → worker looped with `no rows in result set`.
- **FIX**: Token save failure now redirects with `?oauth=error&error=Token save failed`. Worker detects fatal errors (`no refresh token`, `account not found`) and stops retrying.

#### docker-compose — Production Port Mapping
- **HIGH**: `PORT=${UI_PORT:-3000}` passed host port to container's Next.js. Container listened on 3751, Docker mapped 3751→3000. `Connection reset by peer`.
- **FIX**: `PORT=3000` (hardcoded internal), `HOST=0.0.0.0`. Added `LOG_LEVEL=info` for sync diagnostics. `DATABASE_URL`/`REDIS_URL` auto-constructed from POSTGRES_* vars.

#### Dockerfile — UID Mismatch
- **HIGH**: `adduser -S appuser` (no `-u` flag) → Alpine assigned UID 100. Host storage owned by UID 1000. `mkdir /app/storage/emails: permission denied`.
- **FIX**: `adduser -S -u 1000 appuser` + `STORAGE_ROOT=/app/storage` env var + graceful degradation (CAS disabled if dirs unwritable).

#### Schema Migrations — Missing Columns & Tables
- **CRITICAL**: `accounts.absent_since`, `accounts.last_seen_at`, `accounts.system_discovered`, `accounts.is_manual` referenced in Go but missing from `schema.sql`. `contacts.account_id` missing from `schema_mono.sql`. `attachments` FK incompatible with partitioned `emails`.
- **FIX**: Added `ALTER TABLE ADD COLUMN IF NOT EXISTS` for all missing columns. Removed broken FK from `attachments`. Added `account_id` to email_tags INSERT queries. Fixed SQLite `SaveStandaloneDraft` ON CONFLICT target. Encrypted OAuth tokens in SQLite `UpdateAccountTokens` + `GetAccountCredentials` decryption.

---

## [3.0.1] — 2026-06-05

### Security & Stability Audit
- **Password Encryption**: `encryptPassword`/`decryptPassword` strictly return errors instead of plaintext fallbacks, preventing insecure database states.
- **Connection Limits**: Postgres connection pool strictly limited (`MaxConns = 50`) to prevent database starvation.
- **Panic Recovery Middleware**: System-wide `panicRecovery` middleware implemented in HTTP server and background workers (`job_worker`, `sync_worker`) to gracefully catch crashes.
- **In-Memory Rate Limiting**: Added `InMemoryRateLimiter` with an automatic janitor routine as a fallback when Redis is unavailable, securing `/api/auth/login`.
- **SSRF/TOCTOU Prevention**: Strict IP pinning applied via `DialContext` for webhooks, effectively nullifying DNS rebinding attacks.
- **Directory Permissions Hardening**: `os.MkdirAll` now uses `0700` instead of `0755` for storage and databases.
- **TLS Configuration Security**: Stripped all insecure TLS fallbacks tied to the `mono` edition. Now exclusively governed by explicit `ALLOW_INSECURE_TLS` flag.
- **IMAP Debug Logs**: `imapclient` debug logs isolated strictly to when `IMAP_DEBUG="true"` is set.

### License Enforcement — Live Backend Computation
- **Position-based lock enforcement**: `is_locked` no longer read from DB — computed live in Go at request time
- `LicenseMgr.IsAccountLocked(index)` / `IsGroupLocked(index)` — position-based (first 5 accounts / 1 group unlocked on free tier)
- Removed `LockExcessResources`, `UnlockAllResources`, `EnforceLimitsGlobally`
- All API handlers enforce locks: create/update/delete account & group return `402 Payment Required` if locked
- Sync manager uses `LockChecker` callback for live lock computation
- M-version completely unaffected — all gates via `edition.IsMono()`

### IMAP Resolver — M-version Cascade **(Mono)**
- **Priority 1**: `RMS_MAIL_HOST` env var — instant return, no probing
- **Priority 2**: Full MX lookup + port probing (993/143/465/587) for cPanel/bare metal
- **Priority 3**: `host.docker.internal:993` with 500ms DialTimeout — Docker Desktop fallback
- New `RMS_MAIL_HOST` env var documented in all compose/env files

### License Tab
- Full translation to 44 languages (17 new keys)
- Status indicator: Active / Unlicensed / Expired / Revoked / Error / Loading
- Instance UID removed from UI (backend-only)

### i18n — Complete Translation Coverage
- 17 license-related keys added to all 44 languages
- 9 notification center keys added to all 44 languages
- Hardcoded strings removed from: `license-tab.tsx`, `group-manager.tsx`, `accounts-tab.tsx`, `notification-center.tsx`
- Russian hardcoded error message in `accounts-tab.tsx` replaced with `t("resolution_failed")`

### Notification Center
- New component with update availability check and release notes display
- Fully translated to 44 languages

### Sync Worker Improvements
- `last_sync_at` now updated immediately on successful IMAP login (not just after IDLE exit)
- Sync error cleared on successful reconnect

### Bug Fixes
- `support@geotax.biz` IMAP host fixed (`geotax.biz` → `mail.geotax.biz`, TLS certificate match)
- Locked group click blocked in `group-manager.tsx` (was expandable despite `is_locked`)
- ESLint: 0 errors, 0 warnings (fixed `set-state-in-effect`, `static-components`, `immutability`, `no-explicit-any`)

### Docker
- Backend `Dockerfile`: `-tags mono` → `-tags ${EDITION}` (was hardcoded, broke Unified Docker build)

### Premium Upsell Modal **(Unified)**
- Context-based modal via `usePremiumUpsell` hook + `PremiumUpsellProvider` — no prop drilling, single render point
- Triggered from: Free badge click, locked group click, disabled Add Account/Add Group buttons
- Features promoted: Unlimited accounts, Unlimited groups, Priority support, Remove all limits, Commercial use license
- Links to `https://license.rms-ds.com` for license purchase
- Only shown in U-version, gated on `!isLicensed`
- 44 languages translated

### Sidebar Badge **(Unified)**
- **Free**: gray clickable badge → opens upsell modal
- **Premium**: amber 👑 crown icon with tooltip "Premium"
- Hidden entirely in Mono version

### Button Behavior Fix **(Unified)**
- Disabled "Add Account" / "Add Group" buttons now use `opacity-50` styling instead of HTML `disabled` attribute, so `onClick` fires → upsell modal opens

---

## [3.0.1-pre3] — 2026-06-03

### AI & Licensing Foundation
- `AI_DISABLE` global runtime flag (env `AI_DISABLE=true`)
- ENCRYPTION_KEY rotation via `-rekey` CLI flag
- License manager: Ed25519 challenge-response, nonce verification
- License enforcement foundation (`LockExcessResources`, `UnlockAllResources`)
- `is_locked` columns in accounts and project_groups

### Infrastructure
- PostgreSQL email partitioning: `PARTITION BY HASH (account_id)` on 64 partitions
- MCP SSE and messages proxy in nginx config
- Auto-detect MCP base URL from request for reverse proxy support

### Frontend
- Version bump to 3.0.1-pre3
- Frontend dependencies updated (13 packages)
- MCP key viewing endpoint and key management UX
- Opt-in volume mounts for custom nginx configs

---

## [3.0.1-pre2] — 2026-05-30

### Security Hardening
- CORS origins restricted (no more `*` by default)
- 40+ API handlers now enforce `CheckAccountAccess` (IDOR prevention)
- SSE events scoped to authenticated user (no more broadcast to all clients)
- `TLSConfig: InsecureSkipVerify` limited to M-version only
- AI API keys moved from localStorage to backend (encrypted at rest)

### AI Enhancements
- Auto-Draft: LLM-generated reply drafts via filter rules
- Auto-Draft UI: "AI Draft Reply Ready" banner in email viewer
- AI Presets: Fast / Quality / Custom with per-task configuration (4 tasks)
- 10 AI providers: OpenRouter, OpenAI, Anthropic, Gemini, DeepSeek, Groq, Ollama, XAI, OpenCode, Qwen
- Multi-turn chat with `fetch_emails` tool-calling

### PWA
- Service worker with offline support and install prompt
- App icons and manifest

### UX Improvements
- Keyboard shortcuts: j/k navigation, bulk actions, AI commands (all on `event.code`)
- Command palette (Cmd+Shift+P)
- Mobile swipe gestures (delete, read toggle)
- Virtual scrolling (TanStack Virtual)
- Drag-and-drop folder moves
- Support modal (PayPal / Ko-fi)

### Infrastructure
- PostgreSQL partitioning: 64 hash partitions on `emails`
- IMAP IDLE push sync with timing profiles (Default + Mono)
- MCP Server: JSON-RPC + SSE, 3 tools
- Telegram Bot: notifications, AI chat, per-user isolation
- Redis pub/sub for real-time SSE
- Webhook retry queue with HMAC-SHA256 signing

### i18n
- 44 languages with locale-specific namespaces
- RTL support (Arabic, Hebrew, Urdu)

---

## [3.0.1-pre1] — 2026-05-28

### Core Platform
- Three editions: Mono (SQLite), Unified (PostgreSQL), Teams (PostgreSQL)
- JWT authentication + setup wizard
- AES-GCM encrypted passwords and OAuth tokens
- Content-Addressed Storage for attachments (SHA-256 dedup)

### Email
- IMAP sync with IDLE (push notifications)
- Multi-folder sync with UID mapping
- SMTP client with OAuth2 (XOAUTH2) + STARTTLS
- Dynamic mail server resolver (MX lookup + port probing)
- AI summarization, categorization, auto-tagging
- Undo-send with 10s delay
- HTML composer with TipTap (B/I/U/OL/Quote + CSS inlining)
- 10 AI providers for chat, summarization, categorization

### Frontend
- Next.js 16 with App Router
- Tailwind CSS with dark theme
- `next-intl` with 44 locales
- shadcn/ui components
- TanStack Query for data fetching

### DevOps
- Docker Compose: dev + production configs
- Build scripts for Linux/amd64
- nginx.conf with HTTP/2, X-Accel, SSE no-buffer

---

## Versioning

This project follows [Semantic Versioning](https://semver.org/). Given a version number `MAJOR.MINOR.PATCH`:

- **MAJOR**: Backend API breaking changes, storage format changes requiring migration
- **MINOR**: New features, new API endpoints, new CLI flags
- **PATCH**: Bug fixes, security patches, performance improvements
