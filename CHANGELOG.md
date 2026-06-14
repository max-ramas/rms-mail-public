# Changelog

## [3.0.5] — 2026-06-14

### Stability & Concurrency Improvements
- **IMAP Streaming (OOM Fix)**: Removed massive memory allocations (`msg.Collect()`) during email synchronization. Implemented true `io.Reader` streaming that writes raw IMAP streams directly to a local `.eml` temporary file. This ensures `O(1)` memory consumption per folder regardless of email and attachment sizes.
- **Pgxpool Connection Limits**: Hardened PostgreSQL connection pooling by explicitly configuring `MinConns=5` and bounding `MaxConns` limits (defaulting to 100) to prevent connection drop timeouts under heavy load.

- **IMAP Sync Parallelism**: Massively parallelized IMAP fetch processing in `SyncWorker`. Batched multiple message UIDs into a single IMAP `FETCH` command to reduce latency, and implemented a bounded `errgroup` (max 4 concurrent CPU parsers per worker) to accelerate MIME parsing and prevent long CPU blocking loops.
- **Database Dual-Pool Isolation**: Segregated background sync jobs into a dedicated `syncPool` (5 connections) using `poolctx.WithSync(ctx)`, guaranteeing that heavy sync jobs never exhaust the primary API connection pool (20 connections).
- **HTTP Panic Recovery**: Resolved `panic: pattern "/api/health" conflicts` on startup by removing duplicate route registrations in the `ServeMux`.
- **Storage Cleanup**: Overhauled `ResetAccountSync` to properly purge orphaned physical files (`.eml` and attachments) from the disk rather than just clearing the database, preventing indefinite storage bloat.
- **SQLite Concurrency**: Explicitly enabled `PRAGMA busy_timeout=30000;` on SQLite connections to mitigate `database is locked` (SQLITE_BUSY) errors during heavy read/write contention in Mono edition.

### Bug Fixes
- **Email Read Status**: Fixed a critical bug where fetching new incoming emails automatically marked them as read on the server by enabling `Peek: true` in the IMAP `FetchOptions` body section specifier.
- **Frontend Filters**: Implemented empty group filtering in the frontend to automatically hide project groups that contain no accounts.
- **HTML Email Layout**: Restored rendering of legacy table alignments (`align`, `valign`, `bgcolor`) and body backgrounds by refactoring the HTML sanitizer policy and preserving the `<body>` tag as a surrogate `<div>`.
- **429 Rate Limit Storm**: Debounced React Query invalidations triggered by SSE `new-email` events, preventing initial IMAP syncs from spamming the backend and triggering global IP rate limiting lockouts.
- **Frontend API Guardrails**: Fixed infinite 404 retry loops in Mono edition by adding conditional checks to prevent React Query from fetching non-existent endpoints (e.g., `/api/users`, `/api/groups`).
- **Initial Sync Cycle**: Ensured `SyncAllFolders` is explicitly called during `runSyncCycle`, fixing an issue where newly connected or reset accounts failed to download historical emails until manually poked.
- **CI Build Fix**: Removed `*server*` from `.gitignore` to unblock `go vet` and remote builds that were failing due to missing `internal/async/server.go`.

### Database Optimizations
- **Keyset Pagination (PostgreSQL & SQLite)**: Completely eliminated slow `OFFSET` and nested `OR` conditions for email pagination. Implemented strict tuple comparison `(is_pinned, date_sent, id) < ($1, $2, $3)` for O(1) query latency at any depth, and added a dedicated covering index `idx_emails_pagination` to `schema.sql`.

- **PostgreSQL**: Added a composite index on `(folder_id, is_read, is_muted)` to eliminate sequential scans when the background worker recalculates folder unread counts every 30 seconds.
- **SQLite (Mono Edition)**: Added missing `idx_emails_folder_isread` index to `schema_mono.sql` to eliminate full table scans and database freeze scenarios during `RefreshUnreadCounts` background tasks.

### Fixed

#### IMAP Sync — New Emails Incorrectly Marked as Read
IMAP server (e.g. Gmail) was automatically marking newly pushed emails as `\Seen` on the server-side because our fetcher requested the email body without the `Peek` parameter, causing them to appear as "read" in the UI instantly.
**Fix (worker.go):**
- Added `Peek: true` to `FetchItemBodySection` in the sync worker to prevent IMAP fetches from altering the `\Seen` flag.

#### Disk Storage Bloat — Uncollected Files on Resync
Triggering "Reset Account Sync" deleted the database rows for emails and attachments, but left the physical `.html` and attachment files orphaned on the disk, causing infinite storage bloat.
**Fix (storage.go for Postgres & SQLite):**
- Added synchronous disk garbage collection to `ResetAccountSync`. The backend now explicitly queries and iterates over all `body_path` and attachment `path` strings and calls `os.Remove()` before wiping the database rows.

#### Database Performance — 5-10 Second Latency & Timeout Fixes
Opening an email thread or rendering the inbox list experienced massive delays (up to 10s) due to missing covering indexes and sub-optimal queue lookups.
**Fix (schema.sql, storage.go):**
- Added composite coverage indexes `idx_emails_thread`, `idx_emails_unread_fast`, and `idx_emails_folder_isread` to speed up the massive `COUNT(*)` and thread queries.
- Modified the sync worker queue index `idx_sync_queue_fetch_active` to prioritize `account_id` as the primary prefix, rescuing the sync worker from table scans.

#### HTML Emails — Layout and Formatting Stripped
Emails formatted with legacy HTML tables (`align`, `valign`, `bgcolor`) and body backgrounds lost all structural layout and colors because `bluemonday` aggressively stripped the `<body>` tag and deprecated presentation attributes.
**Fix (email_handlers.go, email_normalize.go):**
- Restored `align`, `valign`, and `bgcolor` attributes globally across `table`, `tr`, `td`, and `th` tags in the `bluemonday` policy.
- Modified `normalizeEmailHTML` to dynamically rename the top-level `<body>` tag into `<div id="rms-mail-body-surrogate">` before sanitization, bypassing `bluemonday`'s body-stripping behavior while safely preserving all inline body styles and backgrounds.

#### SQLite_BUSY — Database Locked During Sync (Mono)
Heavy background tasks, specifically IMAP syncs and folder unread counts, caused SQLite to throw `database is locked (5) (SQLITE_BUSY)` and block the API, Webhook poller, and other workers.
**Fix (schema_mono.sql, storage.go):**
- Added missing composite coverage index `idx_emails_folder_isread` to `schema_mono.sql` which instantly resolved 10s+ table scans during the `RefreshUnreadCounts` background task.
- Explicitly set `PRAGMA busy_timeout=30000;` on the LibSQL driver initialization, instructing SQLite to queue concurrent write locks instead of failing instantly.

#### Empty Groups Visible in UI
The frontend sidebar displayed Project Groups even when they contained no accounts (e.g. after removing accounts from a group).
**Fix (email-sidebar.tsx & models/email.go):**
- Added `accounts_count` to the `ProjectGroup` backend model.
- The frontend now explicitly filters out any group where `accounts_count == 0`.

#### 429 Rate Limit Storm on Initial Sync
When logging into a new account, the backend's initial IMAP sync emitted an SSE `new-email` event for **every single email downloaded**. The frontend's `useEmails.ts` reacted to each event instantly by invalidating queries, causing thousands of concurrent API requests that hit the backend's `InMemoryRateLimiter` limit (300 requests/minute), locking the user's IP globally across all `/api/*` endpoints.
**Fix (useEmails.ts):**
- Wrapped React Query invalidation and Desktop Notifications in a 2000ms `setTimeout` debounce.
- Incoming email bursts are now processed as a single UI update.
- Clustered desktop notifications into a single "You have N new emails" summary to prevent notification spam.

#### Initial Historical Sync Stall
Newly connected IMAP accounts (or accounts that were manually reset) were relying solely on the IMAP IDLE push mechanism or periodic jobs to fetch historical emails. They didn't proactively perform a full folder walk.
**Fix (worker.go):**
- Added an explicit `SyncAllFolders(ctx, c, acc)` call into `runSyncCycle()` so both `Mono` and `Unified` editions reliably execute a full top-down synchronization of all folders during their primary sync loop.

#### Missing Build Dependency (.gitignore)
The `go vet` and remote CI builds were failing with "undefined" reference errors for the background job server.
**Fix (.gitignore):**
- Removed `*server*` from the `.gitignore` exclusions to ensure `internal/async/server.go` is properly committed to the repository.

#### Frontend Infinite 404 Retry Loops (Mono Edition)
In the `Mono` (or whitelabel) edition, the backend deliberately doesn't mount the `/api/users` and `/api/groups` endpoints. React Query in the frontend received a `404 Not Found` for these endpoints and entered an aggressive exponential backoff retry loop, creating unnecessary network noise and contributing to rate limiting.
**Fix (useEmailQueries.ts):**
- Added strict `enabled` gating checks `typeof window !== "undefined" && localStorage.getItem("geomail_edition") !== "mono" && !window.location.host.startsWith("wm.")` to all administrative queries so they simply skip fetching when running in a standalone environment.


#### MIME Parsing — Gmail IMAP Body-Section Ordering & Boundary Corruption
Gmail IMAP returns `BODY[HEADER]` and `BODY[TEXT]` in arbitrary order. The old code took only the first section (`break` after one iteration), passing either bare headers or bare body text to `enmime.ReadEnvelope` — causing `malformed MIME header initial line` on **every** email synced via the consumer queue.

**Fix (fetcher.go — ProcessMessage + ProcessMessageToFolder):**
- Sections are now identified by `Specifier` and reordered: HEADER always first, then `\r\n` separator, then TEXT. Forms a complete RFC822 message.
- `repairMIMEBoundaries()` regex fixes Gmail's missing CRLF between MIME boundaries and the next header (`--abc123Content-Type:` → `--abc123\r\nContent-Type:`). Matches hex, alphanumeric, and `_/+=` boundaries.

#### XOAUTH2 — Infinite Authentication Loop (CheckWorker)
`CheckWorker.runSession()` created a temporary `SyncWorker` per attempt. When `refreshToken()` succeeded, it updated only the temporary worker's local Account copy — the CheckWorker's `Account` remained stale. Each retry re-created a new SyncWorker from the stale copy → infinite `AUTHENTICATIONFAILED` loop.

**Fix (checker.go, worker.go, manager.go):**
- CheckWorker re-fetches fresh credentials from DB after `"token refreshed, need reconnect"` error.
- `Manager.LockTokenRefresh(accountID)` — per-account mutex serializes OAuth token refreshes. Only one goroutine calls Google; others wait and proceed with fresh tokens.
- `refreshToken()` in SyncWorker acquires the per-account lock before refreshing.

#### ON CONFLICT — Missing Unique Constraints (SQLSTATE 42P10)
`EnqueueUIDs` (`ON CONFLICT (account_id, folder_name, uid)`) and `SaveEmailToFolder` (`ON CONFLICT (msg_id, account_id, folder_id)`) require unique indexes that may not exist in production databases after resync/rebuild.

**Fix (sync_queue.go, storage.go):**
- `EnqueueUIDs` catches 42P10 → falls back to `INSERT ... WHERE NOT EXISTS`.
- `SaveEmail` / `SaveEmailToFolder` catch 42P10 → fall back to check-then-insert-or-update.
- Schema: added `emails_msg_id_account_folder_key` unique index (3-column).

#### GetFolders — 3-Minute Freeze on Folder List
Correlated `COUNT(*)` subquery with nested smart-category `NOT IN` executed **per folder row**. On 50K emails × 10 folders = minutes.

**Fix (storage.go, schema.sql, queue_manager.go):**
- Added `folders.unread_count INT DEFAULT 0` column.
- `GetFolders` now reads `COALESCE(f.unread_count, 0)` directly (sub-millisecond).
- `RefreshUnreadCounts()` runs every 30s — single batch `UPDATE` for all folders, not per-folder.
- Partial index `idx_emails_folder_unread` for fast counting.

#### GetEmails — Correlated NOT IN on Every Row (Smart Categories)
`e.msg_id NOT IN (SELECT e2.msg_id ... WHERE e2.account_id = e.account_id ...)` — correlated subquery executed per email row.

**Fix (storage.go, schema.sql):**
- Added `emails.smart_category BOOLEAN DEFAULT false` column.
- `RefreshUnreadCounts` tags Promotions/Social/Updates emails in background.
- `GetEmails` filters with simple boolean: `e.smart_category = false` instead of correlated NOT IN.
- Partial index `idx_emails_smart_cat` for fast filtering.

#### SQLITE_BUSY — Database Locked Without Backoff (Mono Edition)
Webhook poller and queue manager hit `database is locked` and retried at fixed intervals, causing CPU thrashing.

**Fix (webhook_queue_mono.go, queue_manager.go):**
- Exponential backoff (up to 10s) on `SQLITE_BUSY` / `database is locked` errors.

#### GetStatsByAgent — N+1 Query Pattern (SQLite)
Separate `SELECT COUNT(*)` per agent row instead of aggregating in the main query.

**Fix (sqlite/storage.go):**
- `SUM(CASE WHEN e.is_read = 0 THEN 1 ELSE 0 END)` in GROUP BY — single query.

### Security

- **Secrets in Logs**: Removed MCP API key + request body logging (`misc_handlers.go:491-493`). OAuth error responses no longer include full HTTP body (may contain tokens). Removed DEBUG username log in `authenticate()`.
- **SSRF**: Block cloud metadata endpoints (`169.254.169.254`, `metadata.google.internal`) in all editions including Mono.
- **Graceful Shutdown**: `StopAll()` now waits for worker goroutines via `sync.WaitGroup` (15s timeout) instead of returning immediately.
- **Query Timeout**: PostgreSQL pools configured with `statement_timeout = 30s` (overridable via `PG_STATEMENT_TIMEOUT` env). Prevents hung queries from blocking workers indefinitely.
- **JSON Marshal Errors**: `GetEmails` no longer swallows `json.Marshal` errors — returns 500 instead of empty 200.
- **Content-Type**: `HandleLicense` and other handlers now set `Content-Type: application/json`.

### Added

- **Folder Management Module**: REST endpoints for IMAP folder CRUD with system folder protection and frontend UI.
- **Test Deployment Pipeline**: `build-and-push-test.sh` + `docker-compose-u-test.yml` / `docker-compose-m-test.yml` for isolated staging deployments with `:test` image tags.
- **Configurable Pool Sizes**: `SYNC_MAX_WORKERS`, `PG_SYNC_MAX_CONNS` environment variables.
- **Health & Metrics Endpoints**: `GET /api/health` returns `{"status":"ok"}` for Docker healthcheck. `GET /metrics` exposes 16 Prometheus counters (`rms_mail_emails_synced_total`, `http_requests_total`, etc.) via `api.MetricsHandler`.
- **Asynq Task Queue (Unified)**: Replaced fire-and-forget goroutines with Redis-backed persistent task queue. Telegram notifications, avatar resolution, webhook dispatch, and unread count refresh now use `asynq` with automatic retries, exponential backoff, and queue priorities (`critical:6, default:3, low:1`). `internal/async/` package with `TaskClient` + `TaskServer`.
- **LibSQL Driver (Mono)**: Migrated from `ncruces/go-sqlite3` to `tursodatabase/libsql-client-go`. Pure Go WASM driver, `CGO_ENABLED=0`, single-writer connection (`MaxOpenConns=1`), WAL mode, `busy_timeout=30000`. Zero external dependencies — local file only.
- **Audit Hardening (TOCTOU)**: Fallback save paths (`saveEmailFallback`, `saveEmailToFolderFallback`) now use `ON CONFLICT DO NOTHING` to prevent race conditions on concurrent saves. MIME boundary regex expanded to support `-` character in boundaries.
- **SyncAllFolders Auto-Discovery**: `runSyncCycle` now calls `SyncAllFolders` on every connection — new accounts get all folders discovered and synced immediately (previously INBOX-only until CheckWorker intervened).
- **All Folders Synced**: `ListFolders` no longer skips parent folders that have children — emails in nested folders are no longer silently lost.
- **SMTP via Asynq**: `SendEmail` handler now uses Asynq task queue (`EnqueueSendEmailDelayed`) when Redis is available, with Scheduler fallback. Full send pipeline via `HandleSendEmail` callback.
- **asynqmon Dashboard**: Asynq monitoring UI at `/mon/` (Unified only, Redis required) for task queue inspection.
- **Final Security Hardening**: Password hex-dumps removed from ALL locations (`worker.go`, `manager.go`). JWT leak in MCP logs fixed (`r.URL.Path` instead of `r.URL.String()`). Identity CREATE/DELETE now require `CheckAccountAccess`. OAuth error responses stripped of full HTTP body. Shared `http.Client` for webhook dispatches with proper body drain.

### Hardening Sprint — 2026-06-13

#### Security
- **JWT Revocation (Blacklist)**: Redis-backed token blacklist. Tokens revoked on password change; blacklist checked in `JWTAuthMiddleware`. Mono edition is a safe no-op (`nil` Redis).
- **Webhook HMAC Fix**: `webhook_queue.go` and `job_worker.go` were sending the RAW SECRET as `X-Signature-256` header — now compute `HMAC-SHA256(secret, payload)`. Secret never leaves the server.
- **Encryption Domain Separation**: `encryptPassword` now derives per-domain AES keys via `SHA256(raw || ":" || domain)` — IMAP passwords, OAuth tokens, MCP keys, and Telegram tokens each use independent key material (`imap_password`, `oauth_token`, `mcp_key`, `telegram_token`). Decryption falls back to raw key for legacy data.
- **MCP SSE Cross-Account Isolation**: MCP SSE event loop now filters events by `sessionAccountID` — an MCP client bound to account A cannot receive `new-email` events from account B. SSE (regular) already had `CheckAccountAccess`.
- **Token in Query String**: Deprecation warning logged when JWT is passed via `?token=` on non-SSE routes. Both `Authorization: Bearer` and `?token=` accepted; frontend already uses headers.

#### Concurrency & Performance
- **Mono Edition Multi-User Fix**: `MaxOpenConns` raised from 1 to 25, `MaxIdleConns` from 1 to 5. WAL mode now actually parallelizes: readers don't queue behind writers. DSN includes `_synchronous=NORMAL`, `_cache_size=-64000`, `_foreign_keys=ON` so every pooled connection gets them. Result: M edition handles 20-30 concurrent users without queueing.
- **Worker Round-Robin Rotation**: New `maybeRotateWorkers` evicts the oldest worker every 5 minutes when all `maxWorkers` slots are occupied. `bootstrapMissingWorkers` fills freed slots from the waiting queue. Prevents a single heavy account from starving others indefinitely.
- **`maxWorkers` Default**: Raised from 10 to 50. New accounts (`created_at DESC`) get priority for initial sync.
- **Goroutine Leak Mitigation**: `waitWithTimeout` goroutine now respects parent context cancellation — won't block on send when caller has abandoned the channel. IMAP-layer context-awareness remains a future improvement.
- **SendScheduler Lifecycle**: `recoverFromRedis`/`recoverFromDB` goroutines now start via explicit `Start()` call after `SetStore`/`SetContext`, eliminating the race window where they polled with nil dependencies.
- **Webhook `CloseIdleConnections`**: Added deferred cleanup to per-call `http.Client` in webhook dispatchers to prevent socket accumulation under load.

#### UX
- **GetFolders SQL Ordering**: `CASE WHEN UPPER(name) = 'INBOX' THEN 1 ... THEN 3 ELSE 2 END` — INBOX always first, custom labels alphabetical, Trash/Spam/Junk forced to bottom. Gmail `[Gmail]/Trash` and `[Gmail]/Spam` recognized. Applied to both Postgres and SQLite.
- **Smart Categories Instant Refresh**: Toggling `smart_categories` now calls `RefreshUnreadCounts` immediately instead of waiting for the 30-second periodic refresh.

#### Code Quality
- **`WakeUpCh` Dead Code Removed**: Channel never had a consumer. `WakeUpAccount` now calls `TriggerRefresh()`.
- **`OnNewEmail` De-globalized**: Moved from package-level `var` to `Manager.OnNewEmail` field. Wired through `Fetcher` struct.
- **`camoCache` LRU**: Replaced unbounded `sync.Map` with `container/list`-backed LRU (10K entries, periodic cleanup at 80% capacity).
- **`stripHTMLTagsFast` Entity Decoding**: Post-processes with `strings.NewReplacer` for `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`, `&nbsp;` — improves AI summary quality.
- **`slogWriter` Level**: IMAP debug trace now uses `slog.Debug` instead of `slog.Info` to avoid log spam at default level.
- **`notification.RateLimiter` Drop Message**: Now logs queue depth (`%d pending`) when dropping.
- **`ProcessAutoDraftJob` Robustness**: `context.Background()` replaced with proper `ctx`; ignored error on `GetAccount` now logged; `AppendToDraftsDeduplicated` is synchronous (error → job retry instead of silent loss).
- **`forceRestartWorkers` TOCTOU**: Added `LockChecker` call before starting each worker — an account locked between `GetAccounts` and worker start is now skipped.
- **Orphaned EML Cleanup**: `ProcessMessage` and `ProcessMessageToFolder` now `os.Remove(bodyPath)` when `SaveEmail` fails after the EML file was already written.
- **`generateAIDraft` Asynq Wiring**: `HandleGenerateAIDraft` now delegates to `OnGenerateAIDraft` callback → `ProcessAutoDraftJob` — Stage 3 is operational.
- **Webhook Dispatch via Asynq**: `sendWebhookWithRetry` now enqueues through `AsyncClient.EnqueueDispatchWebhook` when Redis is available. Webhooks get persistent queue, automatic retries (5x), and `asynqmon` dashboard visibility. Falls back to Redis ZSET (Unified without Asynq) → SQLite (Mono).
- **AI Draft via Asynq**: `handleAutoDraft` now prefers `EnqueueGenerateAIDraft` over DB job queue when AsyncClient is available. Falls back to DB queue for Mono.
- **`ResetAccountSync` Full Cleanup**: Clearing `email_sync_queue`, `imap_move_queue`, `scheduled_emails`, `email_comments`, and `emails_fts` (SQLite) on account reset. Previously, `email_sync_queue` retained `completed` tasks that blocked re-enqueue via `WHERE status != 'completed'` guard in `EnqueueUIDs` — causing 0 emails after resync. Now all sync state is fully purged so the worker performs a clean full re-sync.
- **Startup DB Flood Prevention**: `OnNewEmail` and `SetEventBroadcast` callbacks no longer call `store.GetEmail` on the main DB pool. Previously, every synced email triggered 2 `GetEmail` queries on the main pool (one in `OnNewEmail`, one in `InvalidateEmailCacheByEmailID`). With 200K+ Gmail inboxes, this generated 400K queries competing with HTTP handlers — causing 10-minute API paralysis after restart. Now `OnNewEmail` receives `subject`/`senderName`/`senderAddr` directly from the Fetcher (zero DB queries), and broadcast cache invalidation uses `account_id` from the payload instead of re-fetching the email.
- **Redis Caching — Full Coverage**: All read-heavy GET endpoints now use Redis cache with appropriate TTLs and automatic invalidation via `InvalidateMetaCache` on mutations:
  - `GetEmails` (5min), `GetFolders` (30s), `GetAccounts` (10s), `GetGroups` (30s)
  - `GetLabels` (60s), `GetRules` (30s), `GetTemplates` (60s), `GetContacts` (30s)
  - `GetIdentities` (60s), `GetWebhooks` (30s), `AIModels` (1h)
  - `InvalidateMetaCache` clears all per-account caches on CRUD operations. `InvalidateEmailCache` also clears folder caches. New `InvalidateMetaCache` helper for bulk invalidation on account/group mutations. Reduces DB load on every page load from 4+ queries to 0 (cache hit) or 1 (cache miss + write). Previously, every synced email triggered 2 `GetEmail` queries on the main pool (one in `OnNewEmail`, one in `InvalidateEmailCacheByEmailID`). With 200K+ Gmail inboxes, this generated 400K queries competing with HTTP handlers — causing 10-minute API paralysis after restart. Now `OnNewEmail` receives `subject`/`senderName`/`senderAddr` directly from the Fetcher (zero DB queries), and broadcast cache invalidation uses `account_id` from the payload instead of re-fetching the email.

### Bug Fix — Resync Produced Zero Emails
`ResetAccountSync` deleted emails from the DB but left 6148 `completed` rows in `email_sync_queue`. When `SyncAllFolders` re-enqueued UIDs, the `ON CONFLICT ... DO UPDATE ... WHERE status != 'completed'` clause skipped them all. The consumer loop dequeued nothing → 0 emails appeared after resync.
**Fix (postgres/storage.go, sqlite/storage.go):**
- `DELETE FROM email_sync_queue WHERE account_id = $1` added before folder/account UID reset.
- Also added cleanup for `imap_move_queue`, `scheduled_emails`, `email_comments` (belt-and-suspenders for tables with FK CASCADE).
- SQLite: `DELETE FROM emails_fts` (FTS5 virtual table has no FK support, would accumulate orphaned index entries).

### Performance Sprint — 2026-06-13

#### Keyset Pagination (O(1) page depth)
- **`GetEmailsCursor`** in both Postgres and SQLite stores: replaces `LIMIT 50 OFFSET 50000` with composite cursor `(date_sent, id)` — constant-time regardless of page depth.
- **`X-Next-Cursor`** response header with `Access-Control-Expose-Headers` in CORS config.
- **Frontend**: `useEmailsInfinite` uses `pageParam` as cursor string, reads `X-Next-Cursor` from axios response headers.
- **Backward compatible**: old clients sending `?offset=` continue to work via original `GetEmails`.

#### Instant Unread Counters
- **Live COUNT queries**: `GetFolders`, `GetUnreadCountByAccount`, `GetUnreadInboxCountByAccount`, `GetUnreadCountByFolder` now use direct `COUNT(*) FROM emails WHERE is_read=false` instead of cached `folders.unread_count` column. Partial index `idx_emails_folder_unread` makes these sub-millisecond.
- **Atomic read/unread**: `MarkEmailRead`, `BulkMarkEmailsRead`, `BulkMarkEmailsUnread` update `folders.unread_count` atomically via CTE — but the authoritative source is now live COUNT.
- **`RefreshUnreadCounts` removed from `queue_manager.go`** — no more 30-second background timer wasting CPU on idle counters.
- **`publishEvent` includes `account_id`** in bulk action messages — cache invalidation now fires correctly for read/unread toggles.

#### Cache Coverage
- **In-memory cache for Mono edition**: `MemoryCache` with TTL and periodic cleanup. When Redis is unavailable (Mono), 11 GET endpoints use in-memory `cacheGet`/`cacheSet`/`tryCache` helpers — transparently switching between Redis and local memory.
- **`CheckAccountAccess` caching**: Uses `cache:account:meta:{id}` (30s TTL) to avoid `GetAccount` DB query on every API call. Cuts auth overhead from N+1 to O(1).
- **`Get`/`Set`/`Ping` timeouts**: `redisOpTimeout=3s` applied to all Redis operations — prevents hung HTTP workers on slow Redis.
- **`InvalidateEmailCache` uses passed `ctx`** instead of `context.Background()` for proper lifecycle management.
- **`OnSendTelegram` granular cache**: `cache:account:meta:{id}` replaces full `accounts:list` JSON parse — O(1) lookup instead of O(n) scan.
- **`publishEvent` `context.Background()` → `ctx`** for Redis invalidation.

#### Database
- **`MaxConns=100`** default for PostgreSQL pool (`PG_MAX_CONNS` env overrides).
- **SQLite DSN** includes `_synchronous=NORMAL&_cache_size=-64000&_foreign_keys=ON` for all pool connections.

#### Graceful Shutdown & Stability
- **Shutdown order**: Asynq `Shutdown()` before `syncMgr.StopAll()` — lets in-flight SMTP/webhook tasks complete.
- **WebhookPoller only for Mono edition** — prevents double-delivery with Asynq in Unified.
- **`Asynq.Start(ctx)`** bound to application context instead of `context.Background()`.
- **Redis Publish with `WithTimeout(1s)`** — prevents goroutine leaks during shutdown.

#### Code Quality
- **Removed `case "webhook"`** from `StartJobWorker` (dead code — webhooks go through Asynq/Redis ZSET/SQLite poller).
- **Removed `EnqueueRefreshUnread`** (dead async type — unread counts now live).
- **`HandleSendEmail` SkipRetry** for cancelled jobs — stops 10x retry spam.
- **`SendScheduler.Start()`** explicit lifecycle — goroutines start after dependencies are set.

### Critical Fixes — Race Condition, Encryption Rotation, Telegram — 2026-06-14

- **Provider race condition eliminated**: `callAIChatWithTools` and `callAIChat` in `ai_handlers.go` replaced 80 lines of in-place shared-provider mutation with a 6-line shallow copy via `ai.OverrideProviderSettings()`. Concurrent AI requests with different models/keys no longer race.
- **`OverrideProviderSettings` exported**: `gateway.go` — renamed from `overrideProviderSettings` to exported `OverrideProviderSettings`. Added `*OpenCodeProvider` case.
- **Encryption key rotation fixed**: `resolveAPIKey` now iterates ALL keys from `ENCRYPTION_KEYS` via new `crypto.GetAllEncryptionKeys()` instead of only the first key. AI settings encrypted with old keys after rotation now decrypt correctly.
- **Telegram `MemSessionStore` thread-safe**: Added `sync.RWMutex` guarding all map operations — prevents `fatal error: concurrent map writes` crash under concurrent Telegram traffic.
- **Telegram bot token decryption**: `decryptTelegramToken` in `main.go` tries all domain-derived keys (`telegram_token`) and raw keys before falling back to ciphertext — fixes bot auth failure when token is loaded from DB.
- **Reject plaintext API key storage**: `UpsertAISettings` now returns HTTP 500 when `ENCRYPTION_KEY` is not configured but API keys are being saved — prevents silent plaintext storage.
- **`api_keys_encrypted` → `api_keys`**: Renamed JSON field on the wire (frontend ↔ backend). DB column name unchanged.
- **`AICategorizeEmail` parsing**: Replaced `strings.Split` with `ai.ParseCategories` — properly handles bulleted lists and multi-line AI responses.
- **Webhook secrets stripped from API response**: `GetWebhooks` no longer returns the `Secret` field to the frontend.
- **Telegram bot token masked in API response**: `GetTelegramSettings` returns `XXXX...XXXX` instead of the real token.
- **Postgres error check**: Fixed fragile `err.Error() == "no rows in result set"` → idiomatic `errors.Is(err, pgx.ErrNoRows)` in all 5 locations.
- **Groq model name**: `mixtral-8x7b` → `mixtral-8x7b-32768` in ProviderModels.
- **Ollama API key**: `callAIChat` for Ollama now passes `effectiveKey` instead of hardcoded `""`.
- **Webhook silent drop warning**: `EnqueueWebhook` now logs when Redis is unavailable.
- **AI settings flash fix**: Server data only overwrites localStorage when values actually differ — eliminates visible flash on page load.
- **ModelSelector double-refetch**: Removed redundant `refetch()` call during refresh — was causing triple re-render.
- **AI log card states**: Added loading spinner, error display, and "No AI usage yet" empty state.

### AI, UI & Cache Fixes — 2026-06-14

#### AI — Model Fetching & Key Management
- **Dynamic model fetching for all 10 providers**: Added `FetchAnthropicModels` (Anthropic API) and `FetchOpenAICompatModels` (OpenCode) to `fetchProviderModels`. All providers now fetch live models from their APIs instead of hardcoded lists. Hardcoded `ProviderModels` updated to current 2026 models for all 10 providers.
- **Gemini model filter**: `FetchGeminiModels` now filters to only `gemini-*` models, excluding legacy PaLM models (`chat-bison`, `text-bison`, `embedding`, `aqa`).
- **HTTP status checks**: Added missing HTTP status code validation to `FetchGeminiModels` and `FetchOllamaModels` — previously swallowed API errors silently.
- **OpenCode fixes**: `ChatWithTools` now uses `BaseURL` instead of hardcoded `api.opencode.com`. `ListModels` was `return nil, nil` — now fetches from provider. ProviderEnvKey added `"opencode"` case. Cloud URL detection (`/zen/go/v1` for opencode.ai, `/v1/models` for local).
- **Gemini chat format**: `callAPI` and `callAPIWithTools` rewritten to proper Gemini format: `system` → `systemInstruction` (separate field), `assistant` → `model` role mapping. Previously squashed all messages into one text blob.
- **API key merge (not replace)**: `UpsertAISettings` now merges incoming keys with existing stored keys. Entering one provider's key no longer wipes all other stored keys. Decrypts existing, merges non-empty values, re-encrypts.
- **Key resolution fixes**: `resolveAPIKey` global fallback no longer requires `ALLOW_GLOBAL_AI_KEYS=true` (now on by default, disabled by `=false`). Removed `setting.Preset == ""` blocker. Proper loop over candidate account IDs.
- **Multi-key decryption**: `fetchProviderModels` now tries all keys from `ENCRYPTION_KEYS` (comma-separated), not just the first — fixes model fetching after key rotation.
- **Real error responses**: All AI endpoints (`AIChat`, `summarizeEmail`, `AICategorize`, `AICategorizeEmail`) now return `{"error":"..."}` JSON instead of generic "internal error" — users can see actual API error messages.
- **Model guard in overrides**: `callAIChat` and `callAIChatWithTools` no longer overwrite provider model with empty string when `apiKey` is present but `model` is empty.
- **Frontend provider switch**: Changing provider in settings now resets model to empty — prevents sending incompatible models (e.g. `grok-2` to DeepSeek API).
- **ModelSelector auto-correct**: When loaded models don't include the current value (e.g. stale localStorage), auto-selects first available model. Force-refreshes on mount.

#### Cache & State Consistency
- **`InvalidateEmailCache` MemCache support**: Removed early `return` when Redis is nil (Mono edition). Now properly invalidates both Redis (Unified) and MemCache (Mono). Added `MemoryCache.Keys()` method for prefix-based invalidation.
- **Folder cache SCAN**: Changed exact `Del("folders:unified", "folders:")` to `scanAndDel("folders:*")` — correctly clears all account-specific folder caches.
- **All mutation handlers invalidate cache**: `markEmailRead`, `toggleFlagEmail`, `togglePinEmail`, `toggleMuteEmail`, `snoozeEmail`, `saveDraftReply`, `clearDraftReply`, `moveEmail`, `SetEmailLabels` now call `InvalidateEmailCache` + `publishEvent("email_updated")` with proper `account_id`.
- **BulkAction cache fix**: When `account_id` is not provided by frontend, explicitly calls `InvalidateEmailCache("")` — was previously silently skipped.
- **Frontend SSE listener**: Added `email_updated` event handler that invalidates `["email", id]`, `["emails-infinite"]`, and `["folders"]` React Query caches — previously only `new-email` was handled.

#### Email List & Counters
- **Auto-mark-read timer fix**: Timer was reset on every re-render because `selectedEmail` object reference changed on each refetch. Now uses `useRef` with email ID tracking — timer starts once and fires reliably after configured delay.
- **Timer `__pending__` guard**: Prevents re-timer after successful mutation while React Query cache hasn't updated yet. Clears when `is_read` actually flips.
- **Missing `["accounts"]` invalidation in `useMarkEmailRead`**: Auto-mark-read was invalidating `["folders"]` but not `["accounts"]` — sidebar header counter (`accounts.reduce(... a.unread_inbox)`) never updated. Manual button worked because `useBulkEmailAction` did invalidate accounts.
- **All mutation hooks now invalidate `["folders"]`**: `useFlagEmail`, `usePinEmail`, `useSnoozeEmail` previously only invalidated `["emails-infinite"]` — sidebar folder counters were stale after flag/pin/snooze actions. Added `refetchQueries` for folders to bypass `staleTime`.
- **Blue `is_dirty_locally` dot removed** from email cards — confusing UX, users don't care about IMAP sync status.
- **`useUsers` disabled for non-Teams editions**: Prevents 404 spam on `/api/users` in Unified and Mono editions.
- **Comments endpoint available on all editions**: Moved out of `IsTeams()` block — works on U, M, T.

#### Frontend Cleanup
- **Removed SSE debug logs** (`[SSE] connecting`, `[SSE] connected`, `[SSE] event source error`) from `useEmails.ts`.
- **Save toast fix**: Toast "Saved" was shown unconditionally even on server error. Now shows error toast on failure.
- **`hasSavedKey` preservation**: Flag no longer cleared when saving without re-entered keys after page reload.

### Sync Death Loop and Resync Fixes — 2026-06-14

- Inactivity restart killed consumer loop: heartbeat added
- WakeUpAccount force-restarted all workers: direct mgrWakeUp channel
- Per-host semaphore at dial not close: releaseSem func, perHostCap 10 to 5
- go-imap v2 type-assertion pointer vs value: removed * from FetchItemData types
- Resync race old worker wrote last_sync_uid after reset: atomic ResyncAccount + ctx.Done guard
- TriggerRefresh 30s cooldown dropped resyncs: removed cooldown
- ClaimsKey not set in SSE context: events silently dropped, fixed
- SSE 2s debounce blocked cache invalidation: immediate refetchQueries

## [3.0.4] — 2026-06-12

### Changed

#### IMAP `\Seen` Flag Implementation — Bidirectional Read-State Sync
The IMAP `\Seen` flag was completely ignored in both directions. Emails were always inserted as `is_read=false` (except Sent folders), and local read/unread changes were never pushed to the IMAP server.

**IMAP → RMS (reading `\Seen`)**:
- `fetcher.go`: `ProcessMessage` and `ProcessMessageToFolder` now parse `msg.Flags` and set `email.IsRead = true` when `\Seen` is present. Previously always `false`.

**Atomic ON CONFLICT (race condition protection)**:
- `SaveEmailToFolder` and `SaveEmail` in both PostgreSQL and SQLite: `ON CONFLICT ... DO UPDATE SET is_read = CASE WHEN emails.is_dirty_locally THEN emails.is_read ELSE EXCLUDED.is_read END`. If user changed read-state locally (`is_dirty_locally=true`), server value is not overwritten. Otherwise, sync updates from IMAP `\Seen`.

**RMS → IMAP (pushing `\Seen`)**:
- `worker.go`: `syncFlags()` — after main sync cycle, queries `GetDirtyEmails` (LIMIT 500), groups by read/unread status, sends batched IMAP `STORE` commands (200 UIDs per batch via `StoreFlagsAdd`/`StoreFlagsDel`).
- `ClearDirtyFlag()`: resets `is_dirty_locally=false` after successful IMAP flag sync.

### Added

- **SQLite-Backed Webhook Retry Queue (Mono Edition)**: Implemented an embedded, persistent retry queue (`webhook_retry_queue` table + background Go ticker) for webhooks in the Mono edition. This ensures webhook retries survive container restarts, bringing Mono's reliability on par with the Unified edition's Redis ZSET queue.
- **`GetDirtyEmails(ctx, accountID) ([]Email, error)`**: returns emails with `is_dirty_locally=true`, excluding drafts (separate `GetDirtyDrafts` path)
- **`ClearDirtyFlag(ctx, emailID) error`**: clears the dirty flag after successful IMAP STORE
- **`SyncStore` interface**: both methods added to sync package interface alongside existing `GetDirtyDrafts`

### Fixed

#### UI Freezes during License Validation & Ping
- **License Check (`isLicensed`)**: completely rewritten to be non-blocking. It now unconditionally returns the cached value immediately and fetches the live DB status asynchronously in the background. Fail-open on initial boot ensures zero block on the first request.
- **License Ping**: The HTTP API handler for saving the license key now triggers the background ping asynchronously (`goroutine`) and unconditionally returns `{"status": "ok"}` to the frontend immediately. This prevents the UI from freezing if the license server is unreachable.

#### Database Pool Exhaustion & Silent Account Lockouts
Concurrent background index builds (`CREATE INDEX CONCURRENTLY` across 64 partitions) saturated disk I/O and exhausted the PostgreSQL connection pool. This caused unrelated API queries—specifically `isLicensed` and `GetUnreadCount`—to timeout.
- **Fixed (License Manager)**: Transient `isLicensed` timeouts incorrectly triggered `is_locked: true` on the frontend, instantly kicking users out. Implemented a `sync.RWMutex` backed 30-second TTL cache, and a 5-minute graceful fallback if the database query fails.
- **Fixed (IO Breather)**: Added a `time.Sleep(2 * time.Second)` pause between FTS partition index creations in `RunBackgroundOptimizations` to allow the database to breathe and serve user requests.
- **Fixed (Build Error)**: Replaced orphaned `buildFTSPartitions` call in `main.go` with `store.RunBackgroundOptimizations(context.Background())`.

#### Mono Edition Build Failure
- **Fixed**: `invalid operation: store ... is not an interface`. The SQLite driver returns a concrete pointer type `*sqlite.Storage` instead of an interface, causing the `RunBackgroundOptimizations` type assertion to fail compilation. Wrapped with `any(store)` to satisfy the Go compiler for both editions.

#### Frontend — Group Color Missing for Long Names
Group names without `truncate` pushed the color dot outside the `overflow-hidden` container. Fixed: `shrink-0` on arrow/dot/count, `min-w-0 truncate` on name, `is_locked` badge moved outside name span.

#### GetGroups CTE Optimization
Correlated subquery replaced with `WITH inbox_unread AS (...)` CTE. 3 group subqueries × 250K emails → single hash join over ~500 filtered rows.

#### `\Seen` ON CONFLICT Previously Overwrote Server State
Original `ON CONFLICT DO UPDATE SET is_read = emails.is_read` always preserved DB value, even on first sync where `\Seen` should be the source of truth. New `CASE WHEN` logic correctly distinguishes first sync from local modifications.

## [3.0.3] — 2026-06-11

### Changed

#### IMAP Sync — Hybrid Sync Strategy (Descending + Ascending)
Background sync now uses a hybrid strategy to balance UI responsiveness with data safety. 
- **Initial Fast Fetch**: On a completely new inbox (`lastUID=0`), the worker performs a rapid descending fetch of the top 200 emails to populate the UI immediately without altering the database high-watermark.
- **Historical Ascending Sync**: The main sync loop fetches UIDs in ascending order (`1 → UIDNext`). Progress (`lastUID` checkpoint) is saved safely after every batch. If a crash occurs, sync resumes perfectly without data loss.
- **Sync throttling**: `SYNC_BATCH_DELAY_MS` env var (default 500ms) pauses between batches to prevent DB pool exhaustion

#### PostgreSQL — Asynchronous Partition FTS Indexing (Deadlock Fix)
- Removed `CREATE INDEX IF NOT EXISTS idx_emails_fts ON emails USING gin(fts);` from `schema.sql` to prevent synchronous `SHARE` locks on the entire table and all 64 partitions at startup, which previously exhausted connection pools and caused Docker healthcheck crashes on large databases.
- Added `BuildFTSPartitionsConcurrently` in the background, which iterates over the 64 hash partitions (`emails_p00` to `emails_p63`) and builds the GIN indices safely and independently using `CONCURRENTLY`.
- Unified backend now starts instantly, connection pools remain free, and FTS indexing occurs entirely asynchronously without blocking `INSERT`/`UPDATE` operations.


#### Dual-Pool PostgreSQL — Sync Worker Isolation
IMAP sync workers now use a dedicated 5-connection pool, separate from the 20-connection HTTP pool. Eliminates UI freezes during heavy sync on 200K+ inboxes.

- **`poolctx.WithSync(ctx)`**: context key shared via `rmsmail/internal/poolctx` package
- **`Storage.poolFrom(ctx)`**: selects syncPool for tagged contexts, mainPool otherwise
- **177 `s.pool.*` calls** replaced with `s.poolFrom(ctx).*` throughout postgres storage
- **`NewStorageWithSyncPool`**: creates dual pool on startup; factory (`store/store.go`) uses it for all non-Mono editions

#### Frontend — Build Optimization
- **Docker BuildKit cache mount**: `RUN --mount=type=cache,target=/root/.npm npm install` — npm cache persists across builds, `npm install` drops from 330s to 30-60s
- **`CI=true` moved before `npm install`**: suppresses progress-bar rendering overhead in Docker logs
- **Build tools for `sharp`**: `apk add vips-dev python3 make g++` → `npm install` → `apk del ...` — native C++ addon compiles cleanly on Alpine
- **Standalone runtime**: added `vips` library to runner stage for Next.js Image Optimization
- **Removed `@paypal/react-paypal-js`**: unused dependency (−12 packages, PayPal uses raw SDK via `window.paypal`)
- **Package audit**: 24 packages updated to wanted versions, 973 total (was 1031)

### Fixed

#### IMAP Sync — UIDValidity Data Wipe Prevention
`SyncWorker` early-returns on IMAP sync loops prevented the `UIDValidity` from being persisted into the database. Upon restart, the sync worker detected `UIDValidity = 0` vs the server's `UIDValidity`, forcing a full redownload of all emails.
- **Fixed**: Moved `UpdateAccountUIDValidity` to the top of the IMAP sync sequence immediately after evaluation, ensuring it is always saved to the database.

#### Full-Text Search — Async Indexing
Building FTS indexes on large databases paralyzes the connection pool during backend initialization.
- **Fixed (PostgreSQL - Unified)**: `CREATE INDEX IF NOT EXISTS` for `idx_emails_fts` remains synchronous in `schema.sql`, but the entire `InitSchema` migration runs inside a background goroutine in `main.go`. This prevents the exclusive lock on the `emails` table from blocking the HTTP server startup.
- **Fixed (SQLite - Mono)**: `ReindexFTS` operation inside `main.go` is now wrapped in a background goroutine, preventing backend API startup delays.

#### Mono — SQLite DSN Path Resolution
`modernc.org/sqlite` driver with `file:` prefix (single-slash URI) created the database at an incorrect path (WORKDIR) instead of the bind-mounted `/app/data/`. Container restarts silently lost all data.

- **Fixed**: removed `file:` prefix ambiguity, reverted to original DSN format with proper URI handling
- **Root execution**: `user: "0:0"` in `docker-compose-m.yml` for aaPanel/cPanel compatibility (userns-remap + bind-mount ownership mismatch)
- **Removed data path complexity**: rolled back `DATA_ROOT` env var, `checkDirWritable`, extra `MkdirAll` — returned to original simple `filepath.Join(wd, "data", "rms-mail-mono.db")`

#### Unread Filter — Inconsistent Count
Unread filter badge showed `emails.filter(e => !e.is_read).length` — count of loaded page only (e.g. 48). Actual unread: 455.

- **Fixed**: `GET /api/emails/count?unread=true` with server-side `is_read=false AND is_muted=false`
- **`GetEmailCount` extended**: accepts `unread bool` parameter, adds `is_muted = false` filter
- **Frontend**: `useEffect` fetches server-side count, re-fetches when email list data changes

#### Unread List — Missing `is_muted` Filter
Unread filter in email list (`is_read=false`) excluded muted emails from count but NOT from the list. 3 muted+unread emails appeared in list but were absent from badge.

- **Fixed**: added `is_muted=false` to all 5 Unread filter conditions in both PostgreSQL and SQLite stores

#### Groups — No Loading State
Create Group button had no disabled/loading state during async submission. Multiple rapid clicks created duplicate groups.

- **Fixed**: `disabled={createGroup.isPending || updateGroup.isPending}` + loading text

#### Bulk-by-Filter — IMAP Sync Skipped After Move (Unified)
After `BulkMoveByFilter` moved emails to trash/archive, `GetEmailIDsByFilter(sourceFolderID)` returned empty — no IMAP enqueue was performed. Emails were moved in DB but IMAP server was never notified.

- **Fixed**: fetch email IDs BEFORE the move, then enqueue IMAP after. Non-unified path was already correct.

- **Fixed**: intermediate marker pattern (`$10 → __PGH__11__ → $11`) — replacement-safe regardless of parameter count.

#### Bulk-by-Filter — Snoozed Emails Counted
Unified subquery in `buildFilterWhere` was missing the `snooze_until` filter present in `GetEmails`. Snoozed emails appeared in `GetEmailCount`/`GetEmailIDsByFilter` but were excluded from the email list — causing count/list mismatch.

- **Fixed**: `AND (e2.snooze_until IS NULL OR e2.snooze_until <= NOW())` added to unified subquery in both SQLite and PostgreSQL.

#### Sync Worker — Memory Accumulation
`messages = append(messages, batchMsgs...)` in the batching loop accumulated 250K `FetchMessageBuffer` objects during full sync — dead code (never read, `return nil` before access).

- **Fixed**: removed the append. Probe-success single-fetch path unaffected.

#### Frontend — Unread Counter Polling Churn
`useEffect` depended on `emails` array — every data refetch triggered a `/count` server request, creating positive feedback loop.

- **Fixed**: replaced `emails` dep with `unreadRefetchTrigger` — bumped in `clearSelected()` after every bulk action, triggers re-fetch only on actual state changes.

#### Groups Tab — Correlated Subquery (Production)
`GetGroups` used a correlated subquery to compute unread count per group. For each group, PostgreSQL executed `SELECT COUNT(*) FROM emails JOIN folders JOIN project_group_accounts WHERE group_id = pg.id`. With 250K emails and 3 groups: 750K row scans per call. Instant on local (50 emails), 30+ seconds on production.

- **Fixed**: replaced correlated subquery with `LEFT JOIN + COUNT(FILTER)` pattern. Single table scan instead of N subquery executions.

### Added

- **`GetEmailCount` endpoint**: `GET /api/emails/count?account_id=X&folder_id=Y&unread=true` — lightweight server-side count for filter badges and Select All
- **`GetAccountIDsByFilter`**: returns distinct account IDs matching unified folder filter (used for per-account grouping in bulk operations)
- **`poolctx` package**: shared context-key package for dual-pool selection, no circular dependencies

## [3.0.2] — 2026-06-10

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
- **FIX**: Fixed port mapping and network binding environment variables in production docker-compose configurations и Fixed positional parameter generation edge case in PostgreSQL query builder.

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
- Locked group click blocked in `group-manager.tsx` (was expandable despite `is_locked`)
- ESLint: 0 errors, 0 warnings (fixed `set-state-in-effect`, `static-components`, `immutability`, `no-explicit-any`)

### Docker
- Backend `Dockerfile`: `-tags mono` → `-tags ${EDITION}` (was hardcoded, broke Unified Docker build)

### Premium Upsell Modal **(Unified)**
- Context-based modal via `usePremiumUpsell` hook + `PremiumUpsellProvider` — no prop drilling, single render point
- Triggered from: Free badge click, locked group click, disabled Add Account/Add Group buttons
- Features promoted: Unlimited accounts, Unlimited groups, Priority support, Remove all limits, Commercial use license
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
