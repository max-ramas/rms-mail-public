# Changelog

## [3.1.5] — 2026-07-07

### Added

#### Gmail Labels Support — Multi-Label Deduplication
Gmail doesn't have real folders — only labels. One email appears in multiple "folders" with different UIDs. Previously each label was stored as a separate email row with independent read/delete state, causing inconsistent views across the UI.

**Detection**: Gmail accounts are detected via IMAP `X-GM-EXT-1` capability on connect. The `is_gmail` flag is persisted and used throughout the sync pipeline.

**Dedup**: `ProcessMessageStreamToFolder` checks `GetEmailByMsgIDAccount` before saving — same email in multiple labels becomes one row, with labels tracked via `email_labels_junction` table.

**Flag sync**: `syncFlagsGmail` applies flags globally via `UID STORE FLAGS` (no per-folder SELECT). Includes `\Seen`, `\Flagged`, and `\Answered`.

**Move sync**: `syncMoves` updates junction table after IMAP MOVE — removes source label, adds target.

**Skipped folders**: `[Gmail]/All Mail` and `[Gmail]/Trash` excluded from sync (redundant — all emails covered by other labels).

**Backfill**: `backfillGmailAccounts()` marks existing accounts with `imap.gmail.com` as Gmail on startup.

**Cleanup**: `CleanupGmailDuplicates` merges pre-migration duplicate rows (same `msg_id` across folders) once per account.

**API**: `GetEmails` routes Gmail folder queries through `GetEmailsByLabel` — JOINs `email_labels_junction` instead of filtering by `folder_id`.

**Schema**: Migration 025 adds `accounts.is_gmail`, `email_labels_junction` table. No FK constraints (PostgreSQL partitioned tables limitation — application-level cascade in `DeleteEmail`).

### Added

#### Command Palette Shortcuts
- **Cmd+K / Ctrl+K**: opens Command Palette
- **Cmd+, / Ctrl+,**: navigates to Settings
- Shortcuts displayed in Keyboard Shortcuts modal (Shift+?) header and navigation category
- Added to `PREVENT_DEFAULT_KEYS` to prevent browser focus-search/settings hijacking

### Fixed

#### Email Viewer — 3-Second Text Stub Before Body Loads (All Editions)
Clicking an email showed the plain-text snippet for 3–4 seconds before the full HTML body and thread interface loaded. Only the pulse skeleton should have been visible during loading.

**Root cause**: Commit `cb34fd97` introduced a `setQueryData` cache seed with partial list data (snippet, no body/html) combined with `refetchOnMount: false` — React Query believed it had data, skipped the fetch. Body arrived accidentally via mark-read `invalidateQueries` 3 seconds later.

**Fix**: Removed `setQueryData` seed (header renders from `selectedEmail` list data, not `emailQuery.data`). Removed `refetchOnMount: false`. Previously viewed emails still served from cache within `staleTime: 60_000`.

#### CAS Storage — Nil Pointer Panic in Production
`ProcessMessageStreamToFolder` panicked with `nil pointer dereference` on `f.CAS.Save()` for emails with attachments. Go interface nil trap.

**Fix**: `var casIf sync.CASStore; if cas != nil { casIf = cas }` — explicit nil check before interface assignment.

#### Login — DB Error vs Invalid Credentials
`HandleLogin` returned generic 401 for both "wrong password" and "database connection failure". AuthGuard 401 response caused logout on transient DB errors.

**Fix**: `GetAdminByEmail` uses `errors.Is(err, pgx.ErrNoRows)`. `HandleLogin` returns 503 on DB errors, 401 only for auth failures. `AuthGuard` retries verify once after 500ms on 401.

#### Gmail Label Pagination — Missing X-Next-Cursor
`GetEmails` Gmail routing sent responses without `X-Next-Cursor` header — frontend `useInfiniteQuery` stopped fetching after first page (35 emails with unread filter).

**Fix**: Gmail label path now returns offset-based cursor via standard `Cursor{DateSent: time.Unix(offset+limit), ID}` format. Frontend `ParseCursor` round-trips correctly. Provides correct pagination with zero frontend changes.

#### PostgreSQL — Schema Compatibility Fixes
- `email_labels_junction` FK removed — partitioned `emails` table doesn't support FK references. Application-level cascade in `DeleteEmail` instead.
- `emails_msg_id_account_key` unique index removed — can't create on partitioned table without partition key.
- `COALESCE(is_gmail, false)` → `COALESCE(is_gmail, 0)` — type mismatch (integer vs boolean).
- `UpdateAccountIsGmail` passes `0/1` instead of `bool` to INTEGER column.
- `UpsertEmailLabels` passes `0/1` instead of `bool` to INTEGER `system` column.
- `GetAccount`/`GetAccountCredentials` now scan `is_gmail` in both backends.

#### Gmail Flag State Reversion — Read/Unread Toggle Fails
When a Gmail email was marked as read, the flag appeared to change but later reverted. Root cause: dedup block applied flags from each label's IMAP fetch independently — if a second label's fetch showed `\Seen=false` (Gmail hadn't propagated the flag yet), `ApplyServerEmailFlags` overwrote `is_read=true` back to `false`.

**Fix**: Dedup block now merges flags — takes the most permissive state across existing row + current fetch. `\Seen` never reverts to unread. `\Flagged` and `\Answered` treated identically.

#### Redis Cache — Email Listing Stuck at First Page
Redis `tryCache` returned cached responses without `X-Next-Cursor` headers, breaking infinite scroll pagination for all Redis-backed editions (MP, U). The cache body was stored, but per-request computed headers were not.

**Fix**: Removed `tryCache` and `cacheSet` from `GET /api/emails` handler. Never cache paginated API responses that depend on per-request computed headers.

#### SQLite Migration — `DO $$` Block Incompatibility
Migration 025 used PostgreSQL `DO $$` syntax for column addition. SQLite rejected it with "near DO: syntax error", crashing backend bootstrap for Mono edition.

**Fix**: Created `025_gmail_labels_mono.sql` with plain `ALTER TABLE ADD COLUMN` (compatible with SQLite's `isBenignSQLiteMigrationError` handler).

#### Gmail Detection — CAPABILITY Miss
`detectGmail` relied solely on `X-GM-EXT-1` in `c.Caps()`, but some Gmail servers delay announcing this capability until after SELECT. Accounts were silently not detected as Gmail.

**Fix**: Added IMAP host fallback (`imap.gmail.com`) and persisted-flag re-check. Detection is now two-tier: capability first, host heuristic second.

#### Gmail — Inbound Flag Reversion
`syncInboundFlags` processed Gmail accounts with per-folder FETCH + `ApplyServerEmailFlags`. If one label's flags were stale (Gmail propagation delay), server-side state overwrote locally-synced flags — `is_read=true` reverted to `false`.

**Fix**: For Gmail accounts, `syncInboundFlags` now merges flags — `MAX(local, server)`. `\Seen`, `\Flagged`, `\Answered` never downgrade local state. Dirty emails (`is_dirty_locally=1`) excluded by existing query filter.

#### Gmail — Locale-Dependent All Mail/Trash Skip
`isGmailSkippedFolder` matched English folder names only (`"all mail"`, `"trash"`). On non-English Gmail locales (Russian `Вся почта`/`Корзина`, German `Alle Nachrichten`/`Papierkorb`, etc.) both folders were synced — creating guaranteed duplicate rows for every email.

**Fix**: Switched to IMAP attributes (`\All`, `\Trash`, `\Noselect`) — locale-independent per RFC 6154. Fallback to known localized names (English, Russian) for servers not returning attributes.

#### Gmail — Label Backfill
Existing Gmail emails synced before label support had no entries in `email_labels_junction`. `GetEmailsByLabel` returned empty → folders appeared empty despite having emails.

**Fix**: `BackfillGmailLabels` — one-time operation populates `email_labels_junction` from `emails.folder_id` → `folders.path` for all Gmail accounts. Runs once per account via `system_settings` flag.

#### Gmail — Silent Label Write Failures
`UpsertEmailLabels` errors were silently ignored in dedup block and new-email path. If the junction table didn't exist or a write failed, labels were lost with no diagnostic.

**Fix**: All `UpsertEmailLabels`/`GetGmailLabels` calls now log errors with context (emailID, folder, error).

#### Attachment Upload — Empty UUID in PostgreSQL
Uploading files for email compose failed silently on PostgreSQL (Unified/MonoPro). The handler set `EmailID: ""` which PostgreSQL rejected with `invalid input syntax for type uuid`. The user saw no error because the frontend `catch` block only logged in development mode.

**Fix**: `EmailID` and `AccountID` now use sentinel UUID `00000000-0000-0000-0000-000000000000` instead of empty string. Frontend `catch` always shows error toast. Backend added `h.CAS == nil` guard (503 instead of nil panic), and all per-file errors logged via `slog.Info`.

### Changed

- **Backend Dockerfile**: removed stale `ARG PUBLIC_KEY=""`, removed `schema.sql` copy (Mono-only).
- **License ping**: `LICENSE_SERVER_URL` env override removed — uses hardcoded `https://license.rms-ds.com`.
- **Mono edition**: `InitHK()`, periodic update pings with `latest_version`/`release_notes` restored.

## [3.1.4] — 2026-07-03

### Fixed

#### Email Viewer — 3-Second Text Stub Before Body Loads (All Editions)
Clicking an email showed the plain-text snippet for 3–4 seconds before the full HTML body and thread interface loaded. Only the pulse skeleton should have been visible during loading.

**Root cause**: Commit `cb34fd97` introduced a `setQueryData` call in `handleSelectEmailList` that seeded React Query's cache with partial list-item data (email metadata with `snippet` but without `body`, `html`, or `attachments`). Combined with `refetchOnMount: false` on `useEmail`, the query believed it already had data — `isLoading` stayed `false`, `EmailBody` rendered `{body || snippet}` showing the snippet. The real body arrived only accidentally via the mark-read `invalidateQueries` 3 seconds later (or never for already-read emails — they waited for the 30-second poll).

**Fix**:
- **Removed `setQueryData` cache seed** in `handleSelectEmailList` (`useMailInboxPage.tsx`). The email header (subject, sender, date) is rendered from `selectedEmail` (list data), not from `emailQuery.data`, so the seed provided zero visible benefit.
- **Removed `refetchOnMount: false`** from `useEmail` (`useEmailQueries.ts`). Now defaults to `true` — new emails always trigger an immediate `GET /api/emails/:id` fetch.
- Previously viewed emails within `staleTime: 60_000` are still served from cache instantly — no unnecessary refetches.

#### CAS Storage — Nil Pointer Panic in Production
`ProcessMessageStreamToFolder` panicked with `nil pointer dereference` on `f.CAS.Save()` when processing emails with attachments. Go's interface nil trap: a nil `*attachment.CASStorage` passed to an `interface{}` parameter wraps as non-nil.

**Fix**: `var casIf sync.CASStore; if cas != nil { casIf = cas }` in `cmd/server/main.go` — explicit nil check before assignment to interface-typed variable.

#### Login — DB Error vs Invalid Credentials
`HandleLogin` returned generic 401 for both "wrong password" and "database connection failure". AuthGuard responded to 401 with a redirect to `/login` — transient DB errors caused logout.

**Fix**:
- `GetAdminByEmail` uses `errors.Is(err, pgx.ErrNoRows)` for precise "not found" detection.
- `HandleLogin` returns 503 on DB errors, 401 only for genuine auth failures.
- `AuthGuard` retries `/api/auth/verify` once after 500ms on 401 (catch token sync race).

### Changed

- **Backend Dockerfile**: M2-specific compilation comment → generic "Cross-compile for the target architecture".
- **Frontend Dockerfile**: Russian comments → English.

## [3.1.3] — 2026-07-01

### Changed

#### IMAP `\Seen` Flag — Bidirectional Sync
The IMAP `\Seen` flag was completely ignored. Emails always inserted as `is_read=false`, and local read/unread changes never pushed to server.

**IMAP → RMS**: `ProcessMessage` and `ProcessMessageToFolder` now parse `msg.Flags` and set `email.IsRead = true` when `\Seen` is present.
**Atomic ON CONFLICT**: `CASE WHEN is_dirty_locally THEN emails.is_read ELSE EXCLUDED.is_read END` — local changes protected, server state respected.
**RMS → IMAP**: `syncFlags()` in worker — queries `GetDirtyEmails` (LIMIT 500), groups by read/unread, sends batched IMAP `STORE +FLAGS/-FLAGS \Seen` (200 UIDs/batch). `ClearDirtyFlag` resets after success.

#### Bulk-by-Filter — Folder Name → UUID Resolution
`perAccountFolder = "INBOX"` was passed as folder NAME to `buildFilterWhere` which compared it against `folder_id` (UUID column) for non-unified accounts. Result: 0 affected rows, operation appeared successful but did nothing.

**Fixed**: Multi-account and single-account paths in `bulkActionByFilter` + `runBulkFilterOp` now resolve folder names to UUIDs via `GetFolders` for each account before passing to filter-based operations.

### Fixed

#### Login — First Attempt Kicked Out
AuthGuard `GET /api/auth/verify` failed on first navigation after login — token was in localStorage but verify returned 401. Race condition between Next.js client-side navigation and axios interceptor picking up the token.

**Fixed**: `AuthGuard` now retries verify once after 500ms delay on 401 before redirecting to login.

#### Email Viewer + List Flicker (~3s after load)
SSE delivered queued events on initial connection, triggering `scheduleListRefresh()` and `invalidateQueries(["email", id])`. Both email list and viewer refetched — data unchanged but React re-rendered.

**Fixed**: 5-second warmup window via `warmupUntil` ref — `scheduleListRefresh` and `refreshListNow` skip SSE-triggered refreshes for first 5 seconds after mount. `useEmail` query: removed `placeholderData: keepPreviousData`, added `refetchOnMount: false`, raised `staleTime: 60_000`.

#### Group Color Missing for Long Names
Flex container `overflow-hidden` + long group name without `truncate` → color dot pushed outside visible area and clipped.

**Fixed**: `shrink-0` on arrow/dot/count, `min-w-0 truncate` on name.

#### GetGroups — Correlated Subquery on Production
Correlated subquery per group scanned `emails JOIN folders JOIN project_group_accounts WHERE group_id = pg.id` — O(N×M) where N=groups, M=250K emails. Instant on local (50 emails), 30+ seconds on production.

**Fixed**: Replaced with CTE `WITH inbox_unread AS (...)` filtering to ~500 rows first, then `LEFT JOIN` with groups. Single table scan.

#### `shiftPGPlaceholders` — Parameter Corruption
String replacement `$10 → $11` then `$1 → $2` in `$11` → `$21`. Intermediate marker pattern (`__PGH__N__`) eliminates double-replacement.

#### Bulk-by-Filter — Snoozed Emails Counted in Unified
`buildFilterWhere` unified subquery missing `snooze_until` filter — count included snoozed emails, list excluded them → mismatch.

**Fixed**: `AND (e2.snooze_until IS NULL OR e2.snooze_until <= NOW())` added to unified subquery in both SQLite and PostgreSQL.

#### Sync Worker — 250K Object Memory Accumulation
`messages = append(messages, batchMsgs...)` in batching loop — dead code (never read after loop, `return nil` before access). Held 250K `FetchMessageBuffer` objects during full sync.

**Fixed**: removed the append.

### Added

- **`GetDirtyEmails`** / **`ClearDirtyFlag`**: store methods for `\Seen` flag sync
- **`SyncStore`**: both methods added to sync package interface
- **AuthGuard retry**: visual feedback during token verification grace period

## [3.1.2] — 2026-06-29

### 🎉 New Product Launch: RMS Mail Mono Pro Edition
The **Mono Pro Edition** has been officially released as a standalone commercial product. It combines the single-tenant isolation model of Mono with the enterprise-grade infrastructure of Unified (PostgreSQL, Redis, Asynq).

- **Standalone Product Architecture**: Fully separated from the Unified Edition with dynamic database isolation (`geomail_mp` and Redis index `1`) to prevent cross-contamination.
- **Licensing & Administrator Control**: Enforced rigorous limits for unactivated instances. Unactivated Mono Pro instances are strictly locked to a single administrator account until a valid commercial license is provided.
- **Admin Panel Separation**: The Admin Panel is now a dedicated, independent interface (`/admin`) accessible only to users with the `is_admin` flag, providing a foundation for advanced team, security, and license management.
- **Dedicated Deployment Pipeline**: Released standalone build, run, and publish scripts (`run-mp.sh`, `beta-mp.sh`, `bp-mp.sh`) and docker-compose configurations specifically tailored for the Mono Pro lifecycle.

### 🔐 Admin Panel — Edition Isolation
- **`/api/admin/*` routes** now gated to Mono Pro and Teams only (`IsMonoPro() || IsTeams()`). Previously registered for all editions.
- **Frontend guards**: Admin page redirects Mono and Unified away. Sidebar «Admin Panel» button hidden in M/U.

### 👥 Admin Users Table — Role Column
- **`AdminUser` struct** now includes `role` field (`admin` / `user`).
- **`GetAdminUsers`** populates role from the `users` table via `GetUserByEmail`.
- **`UpsertUser` + `GetUserByEmail`** added to `EntityStore` (postgres + sqlite).
- **Login + `HandleGetMe`**: auto-create user record in `users` table with correct role on first activity.

### 🕐 LAST SEEN AT — Real Login Time
- **`GetAccounts()`** now includes `last_seen_at` in SELECT (was missing — always showed zero time).
- **`HandleGetMe`** updates `last_seen_at = NOW()` on account (Mono Pro / Teams only).
- **`UpdateAccountTimestamp`** extended with `"last_seen"` case (postgres).

### 🔔 Notifications — Admin Only in Mono Pro
- **`NotificationCenter`** in sidebar and About tab hidden for non-admin users in Mono Pro.
- **Check-updates button** in About tab similarly restricted.

### 🗑️ Bulk & Single Delete — Trash-Aware
- **Emails already in Trash** → hard delete (`BulkDeleteEmails` / `DeleteEmail`).
- **Emails not in Trash** → move to Trash (`BulkMoveAndEnqueue`).
- Previously: delete from Trash was a silent no-op (move to Trash again).

### 🐛 Bug Fixes
- **FTS Email Search Optimization (All Editions)**: Fixed a critical issue where searching by email address (e.g., `test@example.com`) failed or returned no results. 
  - In SQLite (Mono), special characters like `@` caused FTS5 syntax errors.
  - In PostgreSQL (Unified & Mono Pro), email addresses were parsed as monolith `email` tokens, preventing partial substring matches.
  - **Solution**: The FTS indexing and search queries now consistently sanitize and split email addresses by punctuation (`@`, `.`, `<`, `>`), ensuring robust, high-performance partial and exact matching across both database engines.
- **BulkMoveAndEnqueue — INSERT column mismatch (PostgreSQL)**: `INSERT INTO imap_move_queue` had 8 columns but only 6 values — caused `SQLSTATE 42601` on every bulk delete/archive/move. Fixed by adding missing `retry_count` and `created_at` parameters.
- **README Feature Matrix**: Removed OAuth 2.0 from Mono Pro column (OAuth is Unified/Teams only).
- **Dead code cleanup**: Removed `patch_auth.go` (orphaned test file at project root).
- **Edition guard hygiene**: `HandleGetMe` no longer calls `UpsertUser` / `GetUserByEmail` in Mono edition (unnecessary — all Mono users are admin). `last_seen` update restricted to MP/T. SQLite `UpdateAccountTimestamp` reverted to `default: return nil` for unused fields.
## [3.1.1] — 2026-06-27

### Backend Architecture Refactoring — SOLID / DRY / KISS / YAGNI

**Interface Segregation (ISP)**
- **`internal/api/store_interfaces.go`**: 7 segregated interfaces replacing the 100-method `Store` monolith.
  - `EmailReader` (31 methods), `EmailWriter` (41), `AccountStore` (17), `FolderStore` (8), `EntityStore` (templates/labels/rules/contacts/groups/users/comments), `AdminStore` (admin/AI/MCP/webhooks/Telegram), `SystemStore` (migrations/FTS/queue/jobs).
  - Composite `Store` interface preserved for backward compatibility.
- **`internal/api/handlers.go`**: `Handler` now exposes segregated fields (`Emails`, `Writer`, `Accounts`, `Folders`, `Entities`, `Admin`, `System`) alongside legacy `Store`.

**Single Responsibility (SRP) — Storage Monoliths Split**
- **`internal/store/postgres/`**: `storage.go` 3980 → 561 lines. New files: `emails.go` (2020), `accounts.go` (387), `org.go` (396), `integrations.go` (440), `folders.go` (137), `labels_tags.go` (82), `attachments.go` (55).
- **`internal/store/sqlite/`**: mirror split (4386 → 465 lines).
- **`internal/store/shared/crypto.go`**: extracted `EncryptPassword` / `DecryptPassword` shared between both implementations.
- **`WithTx()`** added to both `postgres.Storage` and `sqlite.Storage` for atomic cross-file transactions.

**DRY — Duplication Elimination**
- **`internal/mime/qp.go`**: unified `DecodeQuotedPrintable` — removed 3 duplicate implementations (`api`, `sync`, dead code in `handlers.go`).
- **`internal/api/email_action_handlers.go`**: `toggleEmail` helper — 3 toggle functions replaced with single-argument wrappers.
- **`internal/store/postgres/email_columns.go`**: `emailColumns` / `emailColumnsPrefixed` constants — 10 duplicated column lists → 2 constants.

**OCP — Registry Pattern**
- **`internal/api/email_action_registry.go`**: `emailActions` map (11 actions) registered in `init()`. `HandleEmail` switch-case replaced with O(1) map lookup. Thread-safe: write-once in `init()`, read-only at request time.

**Dependency Inversion (DIP)**
- **`internal/sync/interfaces.go`**: `CASStore` interface (1 method) + `AIProvider` interface (3 methods) defined in consuming package.
- `Fetcher` and `Manager` now depend on interfaces instead of concrete `*attachment.CASStorage` and `*ai.Gateway`. Testable without real file-system or AI.

**YAGNI Cleanup**
- Removed: `internalDecodeQP` + `unhexDigit` (dead code, −32 lines).
- Removed: `BulkToggleFlagEmails` from interface + both implementations + mock (−28 lines).
- Removed: `sqlite.Storage.Query/QueryRow/Exec` — raw SQL access not in contract (−15 lines).
- Removed: 5 backup files `storage.go.bak*` (−775KB).

### GlitchTip / Sentry Error Monitoring (Backend + Frontend)
- **`internal/sentry/`** (new package): `Init()`, `IsEnabled()`, `Flush()`, `CaptureException()`, `Recover()`, `Go()` — all no-ops when `SENTRY_DSN` is unset, zero overhead.
- **`cmd/server/main.go`**: `sentry.Init()` after log setup, `defer sentry.Flush()` on shutdown, `CaptureException` in HTTP panic recovery.
- **`internal/api/errors.go`**: `WriteInternalError` → `sentry.CaptureException(err)` — all 500 errors reported with request context.
- **`internal/sync/imap_connect.go`**: `reportProvisionalSyncError` → `sentry.CaptureException(err)` — IMAP/auth errors tracked per account.
- **Frontend Sentry SDK** (`@sentry/nextjs`): `sentry.client.config.ts` + `sentry.server.config.ts` + `sentry.edge.config.ts` — conditional init via `NEXT_PUBLIC_SENTRY_DSN`.
- **`next.config.ts`**: `withSentryConfig()` — conditional wrapping, source maps upload at build time (`SENTRY_AUTH_TOKEN`).
- **App Router error boundaries**: `src/app/[locale]/error.tsx` + `src/app/global-error.tsx` — `Sentry.captureException(error)` on SSR/CSR failures.
- **`sentry.client.config.ts`**: `tracePropagationTargets: [/^\/api/]` — auto-propagates `sentry-trace` header to backend, linking frontend errors with backend traces.
- **Docker**: docker-compose files + .env files updated with `SENTRY_DSN` / `SENTRY_ENVIRONMENT` / `NEXT_PUBLIC_SENTRY_DSN` variables. Frontend `Dockerfile` accepts `SENTRY_ORG` / `SENTRY_PROJECT` / `SENTRY_AUTH_TOKEN` as build ARGs for source map upload.
- **GlitchTip compatible**: uses standard Sentry SDK (DSN format identical). No proprietary features.


### Auth — Login Second-Attempt Fix
- **`internal/api/middleware/auth.go`**: `extractToken()` priority changed — `Authorization` header (localStorage) now checked **before** httpOnly cookie.
  - After login, the frontend stores the fresh token in `localStorage` synchronously and sends it via `Authorization` header.
  - Cookie was previously checked first, so a stale/expired cookie from a previous session could cause the first post-login request to fail with 401 — forcing a second login attempt.
  - Header-first ensures the freshest token is always used; cookie remains as fallback for page reload survival.


## [3.1.0] — 2026-06-26

### i18n — Docker Production Fix (MISSING_MESSAGE)
- **`src/lib/load-messages.ts`**: new isolated message-loading utility with strict error logging and runtime empty-object guard.
  - Reads JSON directly via `fs/promises` from `src/locales/{locale}/{ns}.json`.
  - `deepMerge` of namespaces: `common` → `mail` → `settings` → `auth` → `commands`.
  - Falls back to `en` first, then overlays target locale — missing keys show English, not raw keys.
  - `console.error` on empty messages object (diagnostic for Docker standalone).
- **`src/i18n/request.ts`**: updated to next-intl v4 API — `({ requestLocale })` with `await requestLocale` (Next.js 16 async params).
  - Delegates to `loadMessages(locale)`; removed inline file-reading.
- **`src/app/[locale]/layout.tsx`**: `getMessages()` called **without arguments** — reads from cached React context initialized by `request.ts`.
- **`next.config.ts`**: `outputFileTracingIncludes` fixed — key `"/[locale]/**/*"` (was invalid `"/*"`); paths `./src/locales/**/*.json` + `./src/i18n/**/*.ts`.
- **`Dockerfile`**: `src/i18n` + `src/locales` copied to runtime image; `chown -R node:node ./src` ensures read access for `USER node`.

### Priority On-Demand Mail Sync (Unified)
- **`internal/sync/priority_checker.go`**: new `PriorityChecker` — isolated, short-lived IMAP scanner for user-initiated sync.
  - Opens temporary connection (2 min timeout), authenticates, runs `SyncAllFolders` (INBOX + all folders), disconnects.
  - Never interferes with long-lived `CheckWorker` / `SyncWorker` goroutines.
  - Reuses existing `dialWithRateLimit`, `authenticate`, `SyncAllFolders`, `syncFolderByUID` — zero code duplication.
- **`internal/api/account_handlers.go`**: new endpoint `POST /api/accounts/{id}/check-now`.
  - `CheckAccountAccess` authorization.
  - Background goroutine with 2-minute context timeout — HTTP returns `{"status":"ok"}` immediately.
- **`cmd/server/main.go`**: `PriorityChecker` wired into `apiHandler` via `sync.NewPriorityChecker(store, oauthManager)`.
- **`frontend/src/components/email-sidebar.tsx`**: `handleAccountSelect` triggers `fetch POST /api/accounts/{id}/check-now` for concrete accounts (skip unified/groups).
  - `.catch(() => {})` — best-effort, never blocks UI.

### PostgreSQL Database Optimizations (Production)
- **Connection pool tuning** (`internal/store/postgres/storage.go`)
  - `MaxConns` default: `100` → `min(20, CPU*4)` (5–20 depending on CPU count).
  - Prevents OOM on Docker containers with ≤1 GB RAM (each PG backend process forks ~5–10 MB).
  - Override: `PG_MAX_CONNS` / `PG_SYNC_MAX_CONNS` env vars.
- **ANALYZE after bulk sync** (`internal/store/postgres/storage.go`, `internal/sync/worker.go`)
  - New `AnalyzeAfterBulk()` method runs `ANALYZE emails` asynchronously after `SyncAllFolders` completes.
  - Prevents query planner from choosing Seq Scan on recently-populated tables.
- **FTS subject update fix** (`internal/store/postgres/storage.go`)
  - `SaveEmail` / `SaveEmailToFolder` / both fallbacks: `tsvector` now updates when `subject` changes (was checking only `uid`, `snippet`, `body_path`, `folder_id`).
  - Fixes missing search results after email subject modification.
- **Covering index `idx_emails_folder_read_sent`** (`schema.sql`, `schema_mono.sql`)
  - `(folder_id, is_read, is_muted, is_pinned DESC, date_sent DESC, id DESC)` — eliminates sort step for `GetEmailsCursor` with unread filter.
- **BRIN index `idx_emails_date_brin`** (`schema.sql`)
  - `USING brin(date_sent)` — ~1000× smaller than B-tree, ideal for time-series aggregation queries.
- **Autovacuum insert tuning** (`schema.sql`)
  - `ALTER TABLE emails SET (autovacuum_vacuum_insert_scale_factor = 0.05)` — triggers vacuum on insert-heavy workloads before classic threshold.
- **Partial index `body_path` fix** (`schema.sql`)
  - `WHERE body_path IS NOT NULL AND body_path != ''` — empty-string default no longer breaks partial index.
- **`email_comments` FK index** (`schema.sql`, `schema_mono.sql`)
  - `idx_email_comments_email_account (email_id, account_id)` — fast cascade DELETE when email is removed.
- **`sender_profiles` cleanup index** (`schema.sql`)
  - `idx_sender_profiles_updated_at (updated_at)` — enables efficient TTL cleanup of stale avatar cache entries.

### Email HTML Normalizer — Block-Centering Fix (HTML4 → Standards Mode)
- **`internal/api/email_normalize.go`**: `normalizeNode` now emits `-webkit-center` / `-moz-center` alongside `text-align:center` for **all** block containers with `align="center"`:
  - `div`, `p`, `h1`–`h6` — these are always block containers; `text-align:center` only centers inline content in standards mode (`<!DOCTYPE html>`).
  - `td`, `th` — email clients sometimes put `display:block` on table cells for button-like styling (`border-radius` + background); without `table-cell` context `text-align:center` does not center block children.
- **`nowrap` → `white-space:nowrap`**: deprecated HTML4 attribute on `<td>` now converted to CSS (was silently ignored).
- **`internal/api/email_normalize_test.go`**: new `TestNormalizeAlignCenterOnBlockContainers` covers `div`, `td` (with `display:block`), `nowrap`, `valign`.
- **Affected emails**: marketing/newsletter emails using `<div align="center">` wrapper or `<td align="center" style="display:block">` button cells (Fix Price, GPB, Neon Buddha, Glassdoor — all verified against real `.eml` files).
- **Cascade safety**: `text-align:center` (always valid) → `-webkit-center` (WebKit/Blink override) → `-moz-center` (Gecko override). Each engine silently drops unknown values; the last valid one wins.

## [3.0.9] — 2026-06-25

### Email Body — Text Selection Fix
- **`pointer-events-none` removed** from iframe in `EmailBody` component — text in complex HTML emails was unselectable. Event handling (cursor, image lightbox, click forwarding) moved from the outer container into the iframe's `contentDocument` via `handleIframeLoad`.
- **`mousemove` listener** on iframe document restores `emailHoverCursor` (`zoom-in` for images, `pointer` for links).
- **`click` listener** on iframe document opens lightbox for standalone images; all other clicks (links, buttons) work natively.
- **`getSelectedText()`** (`useImperativeHandle`) already read from `contentWindow.getSelection()` — now works correctly without `pointer-events-none` interference.

### SSE email_updated — Real Status Fields in Payload
- **Backend**: all mutation handlers (`markEmailRead`, `toggleFlagEmail`, `togglePinEmail`, `toggleMuteEmail`, `snoozeEmail`, `moveEmail`, `saveDraftReply`, `clearDraftReply`) now include the **actual field value** in the SSE `email_updated` payload:
  - `is_read` (markEmailRead)
  - `is_flagged` (toggleFlagEmail)
  - `is_pinned` (togglePinEmail)
  - `is_muted` (toggleMuteEmail)
  - `folder_id` (moveEmail)
  - `snoozed` (snoozeEmail)
  - `draft_saved` / `draft_cleared` (saveDraftReply / clearDraftReply)
- **Frontend `EmailFlagPatch`**: extended with `is_pinned | is_muted` so SSE patching works for all status fields.
- **Frontend SSE handler**: now extracts `is_pinned` and `is_muted` from `email_updated` events; skips `scheduleListRefresh()` when only flags changed (cache already patched); falls through to full refresh when `folder_id` is present (move).

### useMuteEmail — Optimistic Detail Cache
- `updateEmailDetail: false` → `true` — mute status now optimistically updates the email detail cache (matching pin/flag behavior).

### Email Body — Link & Button Click Fix (All Modes)
- **Complex HTML (iframe)**: added button/submit interception — `<button>`, `<input type='submit'>`, `[role='button']` clicks now prevent default + open form `action` URL in new tab. Prevents form submission from navigating the iframe (white screen).
- **Simple HTML (div)**: same button interception added to `onClick` handler.
- **TS Fix**: `closest("a[href]")` → `closest<HTMLAnchorElement>("a[href]")` resolves TS2339.

## [3.0.8] — 2026-06-19

### Camo Image Proxy — Marketing Emails & Reliability
- **`sanitizeImageURL`**: some marketing platforms (Banana Republic, etc.) emit SOH (`0x01`) instead of `=` in image URL query strings — `net/url.Parse` failed → `/api/media/proxy` returned **502**. Repaired at HTML normalize and proxy handler time.
- **Pinned HTTPS fetch**: TLS `ServerName` when dialing resolved IP; prefer **IPv4** over IPv6 (Docker egress); follow up to **5** HTTP redirects with re-pin per hop.
- **Camo cache**: files under `STORAGE_ROOT/camo` (aligned with compose volume mount).
- **Graceful image failure**: upstream fetch errors return **1×1 transparent GIF (HTTP 200)** instead of 502 JSON — `<img>` tags no longer flood the browser console.
- **Timeouts / UA**: camo handler context **20 s**; browser-compatible `User-Agent` for picky CDNs.

### Email Iframe CSP — External Stylesheets
- **`style-src`**: expanded to `'unsafe-inline' https:` — external `<link rel="stylesheet">` loads (Google Fonts CSS, `cdn.tbank.ru`, etc.). Fixes CSP console errors on branded newsletters while `script-src` remains absent.

### About — Version & Update Channel
- **`GET /api/license`**: adds `app_version` (Go binary / ldflags) and `update_channel` (`stable` | `beta` | `alpha` from `UPDATE_CHANNEL` baked into image).
- **About tab**: **Stable / Beta / Alpha** badge beside version; displays backend `app_version` when available (matches license telemetry).

## [3.0.7] — 2026-06-19

### Inbox Live Updates — Atomic List & Counter Refresh
- **`refreshMailInbox()`**: single entry point — `reset`/`invalidate` `emails-infinite` plus simultaneous refetch of list, folders, accounts, and filter badge counts (`email-folder-counts`).
- **SSE `new-email`**: immediate `refreshListNow({ resetPages: true })` — new mail always on page 1, no 400 ms debounce.
- **`GlobalMailSSE`**: `useMailPeriodicRefresh` — shared 30 s fallback poll keeps list and counters in sync when SSE disconnects.
- **Polling desync fix**: removed independent `refetchInterval` on `useAccounts`/`useFolders`; dropped 120 s list poll (4 s only during `syncWarmup`).
- **SSE backend**: `emails_bulk_updated` added to `sse.go` subscription (Redis + EventBus) — frontend listened but server never forwarded the event.
- **SSE `folder_updated`**: frontend handler for atomic meta refresh.

### Post-Resync Inbox Recovery
- **`resetAccountSync`**: `InvalidateEmailCache` + `RefreshUnreadCounts` + SSE `emails_bulk_updated` (`action: reset_sync`).
- **`OnNewEmail`**: MemCache invalidation without Redis (Mono edition).
- **Frontend**: `removeQueries(["emails-infinite"])` on reset sync; `GlobalMailSSE` in `[locale]/layout`; `syncWarmup` fast-poll (4 s) while list has fewer than 40 emails.
- **SSE bursts during resync**: debounced `scheduleListRefresh` replaces blocking `fetchingRef` — list no longer stuck after the first refetch.

### Unread Counter Alignment
- **`RefreshUnreadCounts`**: tags `smart_category` first, then counts inbox unread excluding smart-category emails (same rule as `GetEmailsCursor`).
- **`buildFilterWhere` / `GetEmailCount`**: same exclusion for toolbar badges vs sidebar.
- **Mutations**: mark-read and bulk actions invalidate `email-folder-counts`.

### Email HTML Pipeline & UX
- **Removed bluemonday**: read path → `normalizeEmailHTML` → `wrapEmailForIframe`; XSS boundary is iframe srcdoc CSP (`script-src 'none'`).
- **Hardened `sanitizeNode`** in `email_normalize.go`; tests updated.
- **Email body cursor**: `emailHoverCursor` on iframe overlay (`pointer` for links/buttons, `zoom-in` for images); `coreStyles` + `.email-simple-content` CSS.

### Storage & GC
- **`GetActiveFilePaths`**: removed `LIMIT 10000` (SQLite + Postgres) — GC no longer marks valid `.eml` files as orphans on large mailboxes.

### IMAP Flag Sync — Multi-Client Parity
- **Inbound `syncInboundFlags`** (~30 s worker poll): `FETCH FLAGS` for up to 300 recent non-dirty messages per account; reconciles `\Seen`, `\Flagged`, and `\Answered` from server → DB (read/unread both directions, including `unread_count` adjustment).
- **`ApplyServerEmailFlags`**: atomic server-state apply without setting `is_dirty_locally`; replaces narrow `ApplyServerReadState` / unread-only candidate query.
- **Outbound `syncFlags`**: batched IMAP `STORE` for `\Seen`, `\Flagged`, and `\Answered` (200 UIDs/batch); `GetDirtyEmails` now includes `is_flagged` + `is_answered`.
- **Migration 023**: `emails.is_answered` (Postgres + Mono); `IsAnswered` on API model.
- **Reply → Answered**: `MarkEmailAnsweredByMsgID` after successful send when `in_reply_to` is set (sync + scheduler paths); outbound worker pushes `\Answered` to IMAP.
- **Fetch ingest**: `\Flagged` and `\Answered` parsed from IMAP flags on new/streaming messages.
- **SSE / cache**: `email_updated` payload includes `is_read`, `is_flagged`, `is_answered`; `InvalidateEmailCache` on `email_updated`/`email_deleted` without Redis gate (Mono MemCache).
- **Frontend status poll**: 30 s `refetchInterval` on inbox list and open email detail; `email_updated` triggers soft `scheduleListRefresh` (no page reset).

### IMAP Sync Pipeline — Reliability & Timeliness
- **IDLE push wired**: CheckWorker registers `UnilateralDataHandler` on dial — unilateral `EXISTS` signals `idleWake` and triggers `checkNewEmails` + `WakeUpAccount` (was dead channel on throwaway `SyncWorker`).
- **IDLE timing**: periodic refresh uses `Timing.IDLETimeout` (2 min Unified / 30 s Mono, cap 14 min); `IDLEWatchdog` reconnects hung IDLE sessions.
- **Non-INBOX folders**: `syncNonInboxFolders` in SyncWorker consumer loop (`FolderScanInterval` 5 min / 2 min Mono) — incremental UID scan for Sent/Archive/etc. without full reconnect.
- **`syncFlags` fix**: `ClearDirtyFlag` only when all `STORE` batches for a folder succeed — failed outbound flag sync is retried on next tick.
- **Filter rules on ingest**: `RunRules` from `ProcessMessageStreamToFolder` **only for new messages** (`isNewEmail`).
- **Missing server UIDs**: `GetEmailIDByFolderUID` + `DeleteEmail` when FETCH omits queued UID (message expunged on server).
- **CheckWorker cursor**: updates INBOX `last_sync_uid` after successful `EnqueueUIDs` — less duplicate SEARCH/enqueue.
- **Drafts folder**: `resolveDraftsFolder()` — IMAP `SPECIAL-USE \Drafts`, name heuristics (`drafts`, `черновики`), used in `syncDrafts` / `deleteOldDraft`.
- **`syncBatchDelay`**: pauses between folder sync and UID enqueue batches (`SYNC_BATCH_DELAY_MS`, default 500 ms).

### License & Docker
- **`UPDATE_CHANNEL`**: baked into Docker images at build time (`ARG`/`ENV` in `Dockerfile`; `stable`/`beta` via `bp-*.sh` / `beta-*.sh`).
- **`normalizeUpdateChannel()`**: license ping sends only `stable`/`beta`/`alpha` (default `stable`).

### Docker Hub — Single Repository, Edition in Tag
- **One repo** `maxramas/rms-mail` — edition and role encoded in **tag**: `{m|u|t}[-ui]-{channel|version}`.
- **Examples:** `m-latest`, `m-ui-latest`, `u-beta`, `m-ui-beta`, `m-test`, `u-3.0.7`, `t-ui-3.0.7`.
- **Build/push:** `bp-*.sh`, `beta-*.sh`, `build-and-push-test.sh`; all `docker-compose-*.yml` updated.
- **Deprecated:** split repos `rms-mail-m`, `rms-mail-m-ui`, `rms-mail-u-ui`, … and legacy `rms-mail:latest-m` + `rms-mail-ui:latest-m`.

### Sync Wake-Up & Inbound Flag Refresh
- **`WakeUpAccountNow`**: immediate consumer wake for IDLE unilateral push, opened email, and user bulk actions — no 0–30 s jitter (`WakeUpAccount` keeps jitter for bulk/restart paths).
- **`RequestFlagRefresh`**: opening an email queues it for the next inbound `FETCH FLAGS` pass + immediate `WakeUpAccountNow`.
- **Inbound flag batch**: `inboundFlagSyncLimit` raised to **2000** messages per account per tick (was 300).

### Expunged UID & Local File Cleanup
- **`PurgeEmailLocalFiles`**: removes encrypted `.eml` body from disk before `DeleteEmail` when server UID is expunged (sync worker + fetcher paths).

### Webhooks & MCP Integrations
- **Webhook payload**: structured `WebhookEventPayload` — `{ "event": "email.received", "email": { ... } }` (was flat email JSON).
- **`has_secret`**: webhook list/create responses expose signing status without returning the secret; UI shows `(signed)` badge.
- **`RunRules` on ingest**: filter rules run only for **new** messages (`isNewEmail` / `EmailExistsByMsgID`) — no rule re-fire on re-sync.
- **MCP tab**: API key dropdown filtered by current `accountId`; keys scoped per account on create.

### Security & API Hardening
- **JWT `?token=`**: rejected with **400** on all routes except legacy `/mcp/*` SSE paths (prefer `Authorization` header or `rms_token` cookie).
- **Rate limits**: `/api/ai/*` — 30 req/min; search endpoints — 60 req/min (Redis when available, in-memory fallback on Mono).

### Mono Schema Repair — `is_answered` (Production Hotfix)
- **Root cause**: migration `023_email_is_answered_mono.sql` with `ADD COLUMN IF NOT EXISTS` failed silently on some LibSQL/SQLite builds (`near "exists"`) while still recorded in `schema_migrations` → `/api/emails` 500 (`no such column: e.is_answered`).
- **Fix**: `addColumnIfMissing(..., "is_answered")` in `InitSchema`; `ensureMonoEmailColumns()` after `RunMigrations`; migration 023 rewritten to plain `ALTER TABLE ... ADD COLUMN` (duplicate column = benign).
- **Test**: `TestInitSchemaRepairsMissingIsAnswered` — legacy table without column repaired on startup.

### Frontend — Inbox & Email Viewer Refactor
- **`email-viewer`**: split into focused subcomponents (header, body, thread, compose, AI tags, comments, translation, etc.) + shared `types.ts`.
- **`mail-inbox-layout`**, hooks (`useMailCompose`, `useMailInboxAI`, `useMailInboxCommands`, `useMailLabels`, `useTrashActions`) — thinner page shell.
- **`api-client.ts`**: centralized fetch/error envelope; Vitest unit tests for `ai-config`, `compose-utils`, `email-address-utils`, `format-file-size`.
- **UI**: `is_answered` badge in list + viewer; SSE `email_updated` patches `is_read` / `is_flagged` / `is_answered` in React Query cache.

### Reverse Proxy & Mono Single-Port Deployment
- **Mono compose**: backend `expose: 8080` only — host `ports` disabled by default; all public traffic through **frontend `:3000`**.
- **Next.js rewrites**: `/api/*`, `/mcp/*`, `/internal/*` → Go backend inside Docker network.
- **`requestPublicBaseURL()`**: `MCP_API_URL` → `FRONTEND_URL` → `X-Forwarded-Host` / `X-Forwarded-Proto` (comma-separated proto supported).
- **MCP SSE**: `messages` URL built from public base URL — fixes mixed-content `http://` links behind HTTPS reverse proxy.
- **`reverse-proxy.md`**: production guide for **Mono** and **Unified** (aaPanel / nginx, single-port recommended, optional split routing for Unified).

### Email HTML & CI Polish
- **`sanitizeNode`**: strip all `<meta>` tags from email body fragments — invalid inside iframe srcdoc (viewport/charset console warnings).
- **Migration 023 comment**: no semicolons in SQL comments (runner splits on `;` — broke E2E/bootstrap on Mono).
- **`mcp-tab`**: derived `resolvedSelectedKeyId` at render — ESLint `set-state-in-effect` CI fix.

### IMAP Multi-Account Gmail — Dial Slots & Sync Error Surfacing
- **Dial-only semaphore**: `IMAP_PER_HOST_CONN` limits concurrent **TCP dials** to the same host only — slots release immediately after connect succeeds. IDLE and sync workers no longer hold slots for the connection lifetime (was ~2× account count against the cap → false `timed out waiting for IMAP connection slot` on `imap.gmail.com`).
- **Env cap fix**: `IMAP_PER_HOST_CONN` accepts **1–128** (was capped at 32 — values like `50` were rejected and fell back to default **10**, which matched misleading `cap 10` in errors).
- **Startup log**: `sync: IMAP per-host dial concurrency cap` — verify effective value after deploy (`docker logs … | grep IMAP`).
- **Compose / `.env`**: all prod and test `docker-compose-*.yml` pass `IMAP_PER_HOST_CONN`, `SYNC_MAX_WORKERS`, `SYNC_BATCH_DELAY_MS`, `PG_SYNC_MAX_CONNS`, `PUBLIC_URL`, Telegram, MCP vars via `env_file` + `environment`; `.env-m.example` / `.env-u.example` documented.
- **Sync error messages**: reconnect loop preserves `lastErr`; `last_sync_error` shows real dial/auth failure (not generic “failed after N attempts” only).
- **OAuth fatal detection**: `invalid_grant`, missing refresh token, and permanent refresh failures → `isFatalSyncAuthError` — account stops retry storm; user must re-authorize Google/Microsoft in UI.
- **Token refresh reconnect**: SyncWorker and CheckWorker reconnect immediately after successful token refresh (up to 3 tries); Google/Microsoft refresh errors include HTTP response body for diagnosis.

## [3.0.6] — 2026-06-18

### Performance & Correctness Sprint — 2026-06-16

**Sync Pipeline (S2-S3):** Initial sync now fetches full email bodies in a single streaming IMAP pass, eliminating the sync_queue round-trip. Emails ≤1 MiB parsed directly in RAM via `io.LimitReader` (~95% skip disk I/O). Checkpoint every 50 emails for crash recovery.

**Database (D1-D5):** Postgres `SearchFTS` un-stubbed — was no-op, now real GIN `ts_rank` search. `SaveEmail`/`SaveEmailToFolder` skip `to_tsvector()` when no FTS columns changed. OFFSET pagination killed — keyset cursor now default. SQLite `mmap_size=256MB`. `MarkEmailRead` atomically decrements `unread_count`.

**Frontend (F1-F6):** SSE `onopen` refetches on reconnect — no 2-min stale data after sleep. SSE reconnects after 30s instead of permanent close. `fetchingRef` 30s timeout guard against deadlock. Iframe `key={emailId}` for DOM reuse. Virtualizer `measureElement` for accurate scroll. Dead `useSyncExternalStore` removed.

**Hotkey Architecture:** Singleton `HotkeyManager` with capture phase — single `window.keydown` for app lifetime, no listeners lost on re-render. `Shift+U` fixed (was dispatching `deselect` instead of `mark-unread`). M edition `go-inbox` navigates to actual account. `Cmd+A`/`Ctrl+A` in `PREVENT_DEFAULT_KEYS`, bridged via `mail:select-all` command.

**Bulk Operations:** SQLite `BulkMarkEmailsRead`/`Unread` rewritten without `RETURNING`-in-CTE (PostgreSQL-only syntax was never working). Three-phase approach: count → update emails → update folder counters. `BulkAction` sends single `emails_bulk_updated` SSE event instead of N per-ID events.

**Sync & Rate Limiter:** Global rate limiter removed (kept only on `/api/auth/login`). Broken per-account UIDValidity check removed (was per-folder in IMAP). `folderID` fallback `CreateFolder` ensures `lastUID` always saved. `OnNewEmail`/`BroadcastEvent`/Telegram gated on `isNew` — no notification spam during re-sync. Double SSE event eliminated. Callbacks now accept `ctx context.Context`.

**Housekeeping:** `defer cmd.Close()` in `processTasks`. `time.After` → `time.NewTimer` + `defer Stop()` in retry loops. `defer cancel()` on `context.WithTimeout`. JSON marshal + rule action errors logged. Webhook semaphore gate. Avatar worker pool (10 concurrent). Jitter universal. `EmailFilterOpts` → `models`. Schema: `idx_emails_thread` + labels/contacts FK in SQLite, GIN `idx_emails_fts` uncommented. Optimistic updates for delete/move/mute/snooze. `invalidateQueries` scoped (no bare calls). `refetchInterval` 30s→120s. HTML composer code-view toggle. Debounce timer cleanup.

**Risk Zone Hardening:** `WakeUpAccount` now sleeps random 0-30s jitter to prevent thundering herd on multi-account restart. `RunBackgroundOptimizations` runs `VACUUM ANALYZE emails` after partition index builds to prevent GIN bloat. SSE handler audited — no large-object closure captures, no memory leak risk. CAS storage architecture verified ready for S3 backend swap.

### Extreme Performance Scaling & Architecture (Phase 2)
- **Inbox Indexing Reform**: Replaced expensive `LOWER(f.name) = 'inbox'` operations with an indexed `is_inbox` boolean flag in the `folders` table, completely eliminating full table scans across huge accounts for Unified Inbox.
- **Denormalized Unread Counts**: Moved `unread_count` computation away from on-the-fly `COUNT(*)` over the massive `emails` table into a denormalized counter directly on the `folders` table, reducing Dashboard latency to <1ms.
- **Massive Bulk SQL Operations**: Refactored `SetEmailLabels`, `SetGroupAccounts`, `enqueueUIDsFallback`, and `RekeyAll` to use batched operations (`INSERT ... VALUES` in SQLite, `UPDATE ... FROM unnest` and `pgx.Batch` in PostgreSQL). This completely eliminates N+1 query patterns, condensing hundreds of sequential transactions into single atomic operations.
- **OOM Memory Protection Limits**: Enforced hard safety bounds (`LIMIT 10000`) on previously unbounded memory-heavy queries like `GetEmailIDsByFilter`, `GetAllAttachments`, and `GetActiveFilePaths`. This prevents the server from crashing due to Out Of Memory exceptions on massive 100k+ IMAP accounts.
- **Unified Inbox Parameter Fix**: Resolved a critical SQLite & PostgreSQL positional argument mismatch bug where fetching emails from the unified "INBOX" caused database constraint errors or misaligned pagination.

### Performance & UI Polish
- **Account & Folder Loading Acceleration**: Eliminated the `O(N)` nested queries bottleneck in the `/api/emails` endpoint. Replaced the `GetFolders()` database fetch per email with an `O(1)` cached lookup inside `Fetcher.GetOrCreateFolder()`, and optimized the SQL unread/muted counts calculation via a direct `LEFT JOIN`. The accounts page and folder list now load instantly, even during massive background syncs.
- **Sync Pool Expansion**: Tripled the background `syncPool` capacity from 5 to 20 concurrent connections to aggressively process initial IMAP downloads without blocking. Also introduced `PG_SYNC_MAX_CONNS` as an environment variable override for custom environments.
- **Automated Query Planner Statistics**: Appended `ANALYZE emails;` directly into the database migrations (PostgreSQL and SQLite) right after the new indices (`is_read`, `is_muted`) are created. The database query planner instantly starts using the new indices without requiring manual DB administration.
- **Collapsible UI Filters**: Reworked the email list filter row to be collapsible by default, significantly saving vertical screen space. Filter chips smoothly expand to show text, while hiding into clean icon-only indicators with smart badges when collapsed.
- **Pixel-Perfect Alignment**: Reduced the filter chips' visual footprint (smaller height and padding) and fixed flexbox wrapping inconsistencies. Precision-aligned the "Select All" checkbox grid with individual email row checkboxes for a flawless, uniform look.
- **Global Localization Synecdoche**: Added missing `expand_filters` and `collapse_filters` translation strings for English and Russian, utilizing the custom `fill-translations.js` script to auto-fill these new keys into all 45 supported languages dynamically.

### Security
- **SSRF & Open Relay Protection (Mono Edition)**: Implemented strict DNS resolution and validation in `ValidateManualConfig` when `APP_ENV=production`. The Mono edition backend now actively rejects non-private IP addresses (must be RFC 1918, loopback, or link-local) for custom IMAP/SMTP configurations, closing a potential open-relay vulnerability vector.

### UX & Performance
- **Optimistic UI Updates**: Re-engineered React Query mutations for all email state changes (Mark as Read, Unread, Flag, Pin) and Bulk Actions to execute instant "Optimistic Updates". 
- **Timer Freeze Fix**: Addressed an issue where auto-read timers and click events appeared to "hang" or freeze the interface. By synchronously mutating the local cache (including nested account/folder unread counters) and gracefully rolling back on `onError`, the UI now reacts with zero latency, entirely hiding slow IMAP server network round-trips from the end user.
- **Enterprise Printing (B2B)**: Implemented isolated iframe-based printing triggered via `Cmd+P` (or `Ctrl+P`). It intercepts the browser's default print dialog, automatically cleans up dark mode styles and unnecessary UI elements, and renders the entire email thread into a clean, print-optimized document. Added a dedicated `actions` category to the Command Palette for print and other contextual actions.

### AI Categorization — Configurable Taxonomy & Rules Engine
- **Dynamic category taxonomy**: Replaced hardcoded category lists in all 10 AI providers with a dynamic `ai_categories` JSON config stored in `system_settings`. Admins can add, rename, or remove categories via Settings UI without touching code.
- **API**: `GET/PUT /api/system/ai-categories` — read/write the taxonomy (admin-only).
- **Provider integration**: Added `categories []string` parameter to `Categorize()` interface. `buildCategoriesPrompt()` generates the system prompt dynamically from the config.
- **Rules Engine — Auto-Read & Auto-Move**: After AI categorization saves tags, `applyCategoryRules` checks each tag against the taxonomy. Matching rules auto-mark as read (`auto_read`) and/or move to a configured folder (`move_to`). Uses existing `MoveEmail`/`MarkEmailRead` store methods — no new IMAP operations.
- **Race condition guard**: `applyCategoryRules` skips emails where `IsDirtyLocally` is true (user manually moved/tagged the email).
- **Tag filter API**: `GET /api/emails?tag=Invoice` — filters email list by AI tag via `WHERE EXISTS (SELECT 1 FROM email_tags WHERE email_id = e.id AND tag = $N)`. Works on both PostgreSQL and SQLite.
- **Database**: Migration `016_ai_categories.sql` — default taxonomy seed + covering indexes `idx_email_tags_email_tag` and `idx_email_tags_tag_email`.

### UI — AI Categories Settings & Tag Filter
- **Settings tab**: `AICategoriesTab` in Settings page — table with category name (16 presets + custom), color picker, auto-move folder selector (per-account), auto-read toggle. Edits saved via `PUT /api/system/ai-categories`.
- **Tag filter chips**: AI category chips in the email list filter bar. Click toggles `?tag=Invoice` filter. Color-coded per category from the taxonomy config.
- **i18n**: 16 new translation keys added to all 45 locales. Russian manually, 43 locales machine-translated via MyMemory API. 100% coverage.

### IMAP Edge Case Fixes — UIDVALIDITY & Expunge
- **UIDVALIDITY detection for ALL folders** (not just INBOX): `syncFolderByUID` now checks server-side UIDVALIDITY changes on every folder. Previously only INBOX was protected — `Sent`, `Trash`, and custom folders were vulnerable to silent data loss after server-side migrations or mailbox rebuilds.
- **`ClearFolderQueue` on UIDVALIDITY change**: When a folder's UIDVALIDITY mismatch is detected, old queue entries for that folder are purged. Prevents completed tasks from previous UIDVALIDITY universes from blocking new emails with numerically coincident UIDs.
- **Removed `estimatedStart` heuristic**: The `UIDNext - NumMessages` optimization could permanently miss emails with non-sequential UIDs (gappy mailboxes). Replaced with exhaustive `UID Fetch 1..UIDNext-1` in full sync mode — slightly slower but guaranteed complete.
- **Empty body → `FailSyncTask` with retry**: Messages where IMAP returns a valid UID but empty body are now marked as `failed` (retryable) instead of `completed` (permanently skipped). Prevents silent data loss from intermittent network/server issues during fetch.

### Performance — Database Query Optimization
- **Partition pruning**: Added `AND account_id = $N` to 8 single-email queries (`GetEmail`, `MarkEmailRead`, `MoveEmail`, `ToggleFlag`, `TogglePin`, `ToggleMute`, `DeleteEmail`, `UpdateEmailHasAttachments`). PostgreSQL hash-partitioned table now does 1 index probe instead of 64.
- **Attachments indexes**: `idx_attachments_hash` + `idx_attachments_email_id` — CAS dedup now uses index lookup instead of full table scan. Email detail views no longer sequential-scan the attachments table.
- **`GetFolders` — cached `unread_count`**: Replaced live `COUNT(*) FROM emails WHERE folder_id = f.id` correlated subquery with direct read of `folders.unread_count` column (updated by periodic `RefreshUnreadCounts` CTE).
- **`GetAccounts` — omit encrypted blobs**: Removed `password_encrypted`, `oauth_access_token`, `oauth_refresh_token` from the SELECT list (were zeroed in Go anyway). Saves megabytes of I/O per page load.
- **`name_lower` generated column + index**: Migration 020 — added `folders.name_lower TEXT` with index on `(account_id, name_lower)`. Replaced 20+ `LOWER(f.name)` occurrences across all query paths — every `GetEmails` variant, `GetEmailsCursor`, `GetFolderByName`, `buildPGWhere`. Eliminated sequential scans on the folders table.
- **Removed `accounts` JOIN in GetEmails/GetEmailsCursor**: The JOIN was solely for `a.smart_categories` — redundant since `e.smart_category` is already set per-email. All 4 query paths simplified.
- **`GetFolderByName`**: Lightweight single-folder lookup replacing heavy `GetFolders` calls in `deleteEmail`, `moveEmail`, `RestoreFromTrash`, `BulkAction` (6 call sites).
- **`GetUnreadCountByFolder` → cached**: Now reads `folders.unread_count` instead of scanning `emails`.
- **`RefreshUnreadCounts` SQLite**: Replaced correlated subquery UPDATE with efficient FROM-clause hash-join.
- **`IndexEmailFTS`**: Removed redundant subquery — `accountID` passed directly from caller.
- **Batch operations**: `CompleteSyncTasks` (single `WHERE id = ANY($1)` instead of loop), `AddEmailTags` (single `unnest` INSERT instead of per-tag loop), `BulkAction` fallbacks use `BulkDeleteEmails` instead of per-email `DeleteEmail`.
- **Safety LIMITs**: Added to `GetEmailIDsByFilter` (10000), `GetSnoozedEmails` (1000), `GetAllAttachments` (10000), `GetActiveFilePaths` (10000), `GetEmails`/`GetEmailsCursor` no-filter branches (100).

### Fixed — Bugs
- **`GetEmailsCursor` SQL syntax error**: Missing `$1` placeholder after `account_id =` in the cursor-based pagination path — would crash at runtime.
- **`FailSyncTask` wasteful RETURNING**: Removed `RETURNING account_id, folder_name, uid, attempts` from hot-path UPDATE (99.9% of calls ignore the returned values).

### Audit & Hardening Sprint — 2026-06-16

**Critical Fixes (P0):**
- **zstd pool panic**: Replaced unsafe type assertion with comma-ok pattern (`val := pool.Get(); encoder, ok := val.(*zstd.Encoder); if !ok || encoder == nil`) plus nil check — prevents panic when pool had not been initialized.
- **I/O error handling**: `io.ReadAll` errors in `ProcessMessage` and `ProcessMessageStreamToFolder` now properly returned instead of silently ignored — prevents silent data loss on disk/network errors.
- **Encrypted file permissions**: `os.WriteFile` mode changed from `0644` to `0600` for encrypted EML files (both code paths) — prevents world-readable email bodies on disk.
- **`os.MkdirAll` error checks**: Both calls now validate return value — prevents silent failure on disk-full or permission-denied errors.
- **SSE `EventBus` double-close protection**: Added `sync.Once` per subscriber channel — prevents panic when `Unsubscribe` is called twice or races with concurrent `Publish`. Covered by 5 new tests with `-race`.

**Backend Refactoring (P1):**
- **Unified Manager constructor**: Merged `NewManager` and `NewManagerWithAI` into single `NewManager(store, cas, ...ManagerOption)` with `WithAIGateway` functional option. `maxWorkers` extracted to `const defaultMaxWorkers = 50`.
- **`WakeUpAccount` non-blocking**: Replaced `time.Sleep(jitter)` with `time.AfterFunc` — caller goroutine no longer blocked for 0–30 seconds.
- **Structured logging**: Replaced all `slog.Info(fmt.Sprintf(...))` with proper key-value pairs across 10 backend files (`fetcher`, `manager`, `worker`, `checker`, `account_handlers`, `email_handlers`, `ai_handlers`, `sse`, `main`, `main_helpers`).
- **Git hygiene**: `backend.log` already excluded via `.gitignore` (`*.log`). Verified clean.

**Security (P1):**
- **SSE auth cleanup**: Removed legacy `api_key` query parameter check from browser SSE handler — MCP uses separate `MCPSSE` handler with its own auth.
- **Typed context keys**: Replaced `contextKey string` with `contextKey struct{name string}` — prevents key collisions between packages at compile time.
- **AI API Keys audit**: Verified already encrypted via `crypto.Encrypt` → `api_keys_encrypted` column before storage. No changes needed.
- **Telegram webhook URL validation**: Added `sync.ValidateWebhookURL(publicURL)` gate before `SetWebhook` — prevents SSRF via malicious `PUBLIC_URL` environment variable.

**Frontend Refactoring (P1):**
- **`email-list.tsx`** (1352 → 524 lines): Split into `EmailRow.tsx`, `EmailFilters.tsx`, `EmailToolbar.tsx`, `VirtualEmailList.tsx`, `useEmailSelection.ts`. All components receive props instead of closure-captured variables.
- **`useEmailMutations.ts`** (976 lines → barrel re-exports): Split into 13 focused hooks (`useMarkRead`, `useFlagEmail`, `usePinEmail`, `useMuteEmail`, `useSnoozeEmail`, `useMoveEmail`, `useDeleteEmail`, `useSendEmail`, `useSaveDraft`, `useBulkAction`, `useEmailAI`, `useAssignEmail`, `useFolderMutations`) + shared `types.ts`.
- **`HotkeyManager` lifecycle**: Added `dispose()` method for explicit cleanup. Reverted `WeakRef`/`cleanupStale()` experiment — `deref()` non-determinism caused intermittent hotkey failures. Back to strong references with proper `useEffect` cleanup.
- **`QueryClient`**: Verified correct singleton pattern via `useState(() => new QueryClient(...))` — no cleanup needed, no regression risk.
- **ChunkLoadError handling**: Added comment documenting graceful reload strategy.

**Performance (P2):**
- **Virtualizer `measureElement`**: Documented that it only measures ~15-20 visible rows — cost already bounded by virtual scrolling.
- **framer-motion drag**: Preserved for swipe gestures (core UX); virtual scrolling limits rendered instances to viewport.
- **Theme inline script**: Hardcoded — acceptable risk, documented.

**Database (P2):**
- **`name_lower` → GENERATED column**: Migration `021_folders_name_lower_generated.sql` — conditional `DO $$` block converts existing `name_lower TEXT` to `GENERATED ALWAYS AS (LOWER(name)) STORED`. Schema updated for fresh installs. Includes zero-downtime rename strategy for large tables.
- **ON DELETE CASCADE + 64 partitions**: Documented — PostgreSQL handles cascade across partitions. Async cleanup worker considered for future if performance degrades on mass deletes.

**Infrastructure (P3):**
- **Alpine**: `3.20` → `3.21`
- **Go**: `1.26.3` → `1.26.4`
- **Next.js**: `16.2.7` → `16.2.9`
- **Dependencies**: 11 Go packages upgraded (`pgx v5.9.2→v5.10.0`, `go-redis v9.19.0→v9.20.1`, `protobuf v1.36.10→v1.36.11`, `websocket v1.8.12→v1.8.15`, `gorilla/mux v1.8.0→v1.8.1`, etc.)
- **Docker HEALTHCHECK**: `wget --spider http://localhost:8080/api/health` every 30s with 5s timeout, 3 retries.
- **Nginx**: Extracted `upstream backend { server 127.0.0.1:8087; }` to deduplicate `/api/` and `/mcp/` `proxy_pass` blocks.

**Tests (P3):**
- **`EventBus` test suite** (`sse_test.go`): 5 tests — subscribe/publish/unsubscribe lifecycle, double-unsubscribe no-panic, publish to nonexistent channel, multiple subscribers broadcast, unsubscribe-during-publish race condition (100 subscribers × 50 publishers, `-race` clean).

### SSE Ticket System (PR-9) — 2026-06-16

- **Ticket Store rewrite**: Replaced `uuid.New()` with `crypto/rand` 32-byte hex tokens. `TicketData{UserID, AccountID, ExpiresAt}` with `sync.RWMutex`. Burn-after-reading: ticket deleted on first `ValidateTicket` call. 30s TTL, background cleanup every 30s.
- **Frontend `TicketManager`**: Singleton with `fetch()` deduplication via shared Promise. Falls back to cookie-only auth on network errors. Uses `credentials: "include"` for httpOnly cookie.
- **Frontend `useSSETicket` hook**: Manages full EventSource lifecycle — ticket prefetch → connect with `?ticket=` → exponential backoff reconnect (2s-30s, max 8 retries) → fresh ticket each reconnect. Uses `eventsKey` string instead of `...events` array in effect deps to prevent render loops.
- **`useEmails.ts`**: Replaced manual 200-line `EventSource` with `useSSETicket`. Consolidated 4 event handlers into single `handleSSE` callback. Mutable state moved to `useRef` for stable references across reconnects.

### Production Hardening — 2026-06-16

**SQLite TEXT to time.Time scan fixes (Mono edition):**
- **`GetEmailsCursor`**: `date_sent` scanned directly into `time.Time` — fixed to `sql.NullString` + `parseTime()`. Matched existing pattern from `GetEmails`, `GetEmail`, `GetEmailsByAccounts`, `GetEmailsByIDs`, `SearchEmails`.
- **`GetSnoozedEmails`**: `snooze_until` scanned directly into `*time.Time` — fixed to `sql.NullString` + `parseTime()`.
- **`GetAISettings`**: `created_at`/`updated_at` scanned directly into `time.Time` — fixed to `sql.NullString` + `parseTime()`.
- **`InitSchema` defensive columns**: Added `addColumnIfMissing` for `emails.cc_address`, `status`, `first_response_at`, `resolved_at`, `smart_category` — prevents "no such column" when ALTER TABLE in schema_mono.sql fails silently.
- **`smart_category` NULL fix**: SQLite ALTER TABLE ADD COLUMN sets NULL for existing rows. Added `UPDATE emails SET smart_category = 0 WHERE smart_category IS NULL` to both schema_mono.sql migration and InitSchema runtime fix. Query filter changed to `(smart_category = 0 OR smart_category IS NULL)`.

**CORS and CSP production hardening:**
- **CORS**: Added `http://localhost:3500` and `http://127.0.0.1:3500` to dev CORS allowlist (frontend dev server on port 3500).
- **CSP**: `connect-src` now uses `'self'` only in production; dev-only `localhost:8087` and `ws://localhost:3500` excluded from production CSP header.
- **Camo HMAC**: `InitCamoKey()` now called unconditionally (removed `!edition.IsMono()` guard) — fixes 403 Forbidden on avatar images in Mono edition.

**Docker compose fixes:**
- **All M and U compose files**: `user: "0:0"` (root) — fixes `SQLITE_READONLY (error code 8)` caused by host directory permissions mismatch (aaPanel/cPanel `www` owner vs container uid 1000). Safe for single-user Mono; U only uses bind-mount for attachment storage.

**Logging and observability:**
- **Default log level**: `slog.LevelWarn` changed to `slog.LevelInfo` — ensures all Info-level messages are visible by default.
- **`markEmailRead` error logging**: Added `slog.Error` on every failure step (tx begin, query, update, folder update). Handler now returns 404 when `GetEmail` fails to resolve `accountID`.
- **`GetEmails` handler**: 500 response body now includes the actual error message for client-side debugging via Network tab.

**Race condition tests:**
- All 10 test packages pass with `-race` flag — zero data races detected.

### Retry & Backoff Hardening — 2026-06-17

**Thundering herd prevention:**
- **Reconnect jitter**: `sync()` reconnect loop now uses `CalculateReconnectDelay` with ±15% jitter — prevents all workers from hitting the server simultaneously after an outage. Replaced fixed `reconnectDelay*2` with exponential backoff + random spread.
- **CheckWorker backoff**: Replaced fixed `time.Sleep(15s)` retry with exponential backoff (`15s × 2^(n-1)` + 30% jitter, capped at 5 min). Prevents CheckWorkers from DDoS-ing the IMAP server after connection failures.
- **Timing parameters**: `DefaultTiming`: `ReconnectRetries` 5→10, `ReconnectBase` 10s→15s, `ErrorBackoffBase` 30s→60s, `ReconnectMaxWait` 160s→300s. `MonoTiming`: `ReconnectRetries` 3→6, `ReconnectBase` 3s→10s, `ErrorBackoffBase` 10s→30s.
- **`CalculateReconnectDelay`**: New function in `timing.go` — `base × 2^attempt + random(-15%, +15%)`, capped at 5 min. Shared across `sync()` reconnect loop.

**Pause/Resume sync (Google rate-limit recovery):**
- **Backend**: `IsPaused` callback on Manager via cache layer (`account:sync_paused:{id}`, TTL 24h). Guards in `WakeUpAccount`, `refreshWorkers`, `bootstrapMissingWorkers`. `POST /api/accounts/{id}/pause-sync` stops both SyncWorker and CheckWorker. `POST /api/accounts/{id}/resume-sync` triggers `TriggerRefresh`. Auto-resume after 24h TTL expiry.
- **Frontend**: Single toggle button (Pause ⏸ / Play ▶) in Settings → Accounts, next to Resync. Amber highlight when paused. `is_sync_paused` injected into account response via cache lookup.
- **i18n**: 4 new keys (`pause_sync`, `pause_sync_done`, `resume_sync`, `resume_sync_done`) in all 45 locales. Machine-translated via MyMemory API for 43 locales.

**Production bug fixes:**
- **Unified inbox folders**: `getFolderName` returned undefined for real folder IDs — now looks up folder name from fetched data.
- **`name_lower` NULL fix**: Added `DO $$ BEGIN UPDATE folders SET name_lower = LOWER(name) WHERE name_lower IS NULL` to `schema.sql`. Existing installations had NULL `name_lower`, causing `name_lower = LOWER(...)` to return no results — empty group inboxes.
- **Docker compose**: `user: "0:0"` in all M/U compose files — fixes `SQLITE_READONLY (error 8)` on aaPanel/cPanel hosts.

**Frontend lint fixes:**
- **React Compiler**: Fixed 16 `react-hooks/preserve-manual-memoization` errors in `page.tsx` — added state setters to `useCallback`/`useMemo` dependency arrays.

### Production Hotfixes — 2026-06-17

**Redis connection fix (Unified edition):**
- **Root cause**: `strings.TrimPrefix(redisAddr, "redis://")` on `redis://:password@host:6379` left `:password@host:6379` — the leading colon from empty username caused `too many colons in address` error in `go-redis`/`asynq`. Additionally, password was never passed to Redis client — would fail with `NOAUTH` even after address fix.
- **Fix**: Added `parseRedisURL()` helper that correctly extracts `host:port` and `password` from `redis://` URLs. `cache.NewClient(addr, password)` now passes password via `redis.Options.Password`. `async.NewTaskClient` and `async.NewTaskServer` also receive password for `asynq.RedisClientOpt.Password`. All 3 constructors + test updated.

**Delete email 500 on PostgreSQL (pgx.ErrNoRows):**
- **Root cause**: `deleteEmail` handler checked `err != sql.ErrNoRows` but PostgreSQL via pgx returns `pgx.ErrNoRows` — a different error value. The guard never matched, so missing Trash folder always returned 500 instead of auto-creating it.
- **Fix**: Changed to `!errors.Is(err, sql.ErrNoRows)` — matches both `sql.ErrNoRows` (SQLite) and `pgx.ErrNoRows` (PostgreSQL).

**Partition indexes survived `DO $$` cleanup (PostgreSQL):**
- **Root cause**: `DO $$` block in `schema.sql` used POSIX regex `\d` to find orphaned partition indexes. PostgreSQL POSIX regex doesn't recognize `\d` as digit class — requires `[0-9]`. All 64 `emails_pXX_msg_id_account_id_idx` indexes survived silently.
- **Fix**: Changed regex to `'^emails_p[0-9]+_msg_id_account_id_idx$'`. Also added step to clean up any indexes still alive after parent constraint drop.

**Migrations embedded in binary (`go:embed`):**
- **Root cause**: Migration `.sql` files needed separate `COPY` in Dockerfile — repeatedly forgotten, leading to `RunMigrations` returning 0 migrations applied.
- **Fix**: Moved migrations to `internal/migrations/` with `//go:embed` directive. All 22 `.sql` files compiled into the Go binary. `RunMigrations` now reads from `embed.FS` — zero filesystem dependencies.

**Delete UI lag (optimistic update overwritten):**
- **Root cause**: `useDeleteEmail.onSuccess` invalidated 6 query keys (`emails`, `emails-infinite`, `search`, `accounts`, `folders`, `groups`) + `refetchQueries`. `onMutate` already removed the email from cache optimistically, but `onSuccess` triggered a full refetch of the email list, overwriting the instant optimistic update.
- **Fix**: `onSuccess` now only invalidates `["folders"]` to update sidebar counters. Email list updates instantly via `onMutate`; rollback via `onError` if server fails.

### Production Stabilization — 2026-06-17

**Auto-migration system:**
- **`RunMigrations`**: New method on Store interface. Executes all pending `.sql` migration files from `migrations/` directory in sorted order on every server restart. Uses `schema_migrations` tracking table for idempotency. `pg_advisory_lock` serializes migrations across Unified instances. Both PostgreSQL and SQLite implementations.
- Previously, migrations were manual-only — field operators had to run them by hand. Now applied automatically on container restart.

**Hotkey fixes:**
- **`navigation:go-inbox`**: Removed `"i"` from defaultKeys — was conflicting with `Shift+I` (`mail:mark-read`). `commandKeyMap` maps each key individually (no sequence parser), so plain `i` was triggering inbox navigation.
- **Debug logging**: Added and removed diagnostic logging in `useKeyboardShortcuts` to trace Shift combo resolution.

**API fixes:**
- **`group:` prefix**: Added `!strings.HasPrefix(accountID, "group:")` guard to `GetEmailCount` — group-prefixed account IDs were failing `CheckAccountAccess` with "account not found" (403).
- **`deleteEmail`/`RestoreFromTrash`**: `GetFolderByName` now treats `sql.ErrNoRows` as "folder not found" instead of 500. Handles missing `name_lower` population gracefully.

**Compose fixes:**
- **`cap_drop: ALL` removed from PostgreSQL services** in all compose files — blocked `chmod` during PostgreSQL initialization, causing container healthcheck failures on systems with userns-remap (aaPanel/1Panel).
- **All compose files**: Production and test variants cleaned of security_opt/cap_drop/cap_add for PostgreSQL.

**Frontend fixes:**
- **`getFolderName`**: Moved `foldersQuery` declaration before `getFolderName` to fix `ReferenceError: Cannot access 'foldersQuery' before initialization` (temporal dead zone).

### Delete Email — SQLite Transaction & Frontend Stability — 2026-06-18

**SQLite `MoveEmailAndEnqueueIMAP` atomic transaction:**
- Envelope UPDATE + IMAP queue INSERT now runs inside a single `retryBusy`-wrapped transaction. Previously two separate `retryBusy` calls could leave the email in Trash with no IMAP move queued on SQLITE_BUSY contention.
- `retryBusy` backoff increased from `50<<attempt` (max 1.55s) to `200<<attempt` (max 6.2s), giving more time for concurrent sync workers to release locks.

**Frontend `handleDeleteEmail` (toolbar / `mail:delete` command):**
- `selectedEmailId` is captured into local `id` at function entry. Selection advances to the next email only after `mutateAsync` resolves — prevents `setSelectedEmailId` from moving when the mutation fails (500 from SQLITE_BUSY).
- Added `try/catch` with error toast — unhandled promise rejection eliminated.

**Frontend `shortcuts.onDelete` (keyboard Delete / `d` / Backspace):**
- `deleteMutation.mutate()` called first (triggers sync optimistic update), then `setSelectedEmailId` advances to the next email. React 18 automatic batching renders both state changes together — `displayedEmails` and `selectedEmailId` are always consistent.

**Outcome:** If the backend returns 500 (SQLITE_BUSY), the email reappears in the list via `onError` rollback, selection stays on the original email, and a toast shows the error. If the backend succeeds, the email disappears instantly and selection moves to the next email with zero-lag UX.

### Security & API Sprint — 2026-06-18

**Handler modularization:**
- Split monolithic `email_handlers.go` (~3250 lines) into 9 focused files plus `email_helpers.go`: `email_action_handlers`, `email_attachment_handlers`, `email_bulk_handlers`, `email_folder_handlers`, `email_misc_handlers`, `email_org_handlers`, `email_read_handlers`, `email_send_handlers`, `email_team_handlers`.
- Standardized JSON error envelope: replaced `http.Error` with `WriteJSONError` / `WriteInternalError` across all API handlers. `WriteInternalError` deduplicates noisy internal-error logging.

**SSE auth — PR-9 complete:**
- `GET /api/events` returns **400** when `?token=` is present — JWT in query strings no longer accepted (prevents token leakage in access logs).
- Auth via `?ticket=` (burn-after-reading, 30s TTL from `GET /api/auth/ticket`) or `rms_token` httpOnly cookie.
- `TestSSE_RejectsTokenQueryParam` added. Non-SSE routes still accept `?token=` with deprecation warning.

**Auth hardening:**
- `HandleSetup` sets httpOnly `rms_token` cookie alongside JSON response — first-run flow no longer relies on localStorage token alone.

**E2E & CI:**
- Playwright mono smoke tests (`e2e/smoke.spec.ts`) in CI.
- Unified edition E2E (`e2e/unified-auth.spec.ts`) with ephemeral Postgres via `scripts/e2e-unified-backend.sh`. New CI job `e2e-unified`.

### Frontend Architecture Sprint — 2026-06-18

**Inbox page decomposition:**
- `page.tsx` inbox logic extracted to `useMailInboxPage` hook; further split into `useMailInboxState` (navigation, queries, folder sync, auto-select) and `buildMailViewerProps` (EmailViewer prop assembly).

**Email list hooks:**
- `email-list.tsx` decomposed into `useEmailListFilters`, `useEmailListKeyboard`, `useEmailListVirtualizer` — filters, keyboard shortcuts, and virtual scroll isolated from the list component.

**Performance:**
- **EmailRow**: Replaced `framer-motion` swipe gestures with CSS `transform` + `transition` + pointer events — removes per-row animation library overhead in the virtual list.
- **VirtualEmailList**: Hybrid virtualizer — fixed `ROW_HEIGHT` estimate for off-screen rows, `measureElement` on visible rows so labels/snippets are not clipped.
- **ChunkLoadRecovery**: Dedicated component auto-reloads on Next.js chunk load failures after deploy.

**Infrastructure:**
- Alpine `3.21` → `3.22` in Dockerfile.
- `.gitignore`: `backend_*.txt` debug log patterns excluded.

### Mono Inbox Stabilization — 2026-06-18

**SQLite write queue:** User mutations (delete, move, flag, pin, labels) go through a dedicated write queue prioritized over background IMAP sync. `enqueueWrite` uses `context.WithoutCancel` so writes complete even if the HTTP client disconnects.

**Auth & edition gates:** Restored session API auth for Mono. Edition-gated React Query hooks check `rms_edition` (was `geomail_edition`). Administrative queries (`/api/users`, `/api/groups`) skip fetching in Mono — no 404 retry storms.

**Labels & list UI:** Fixed label CRUD/display in M/U settings (`label-manager`, `settings.ai_cat_empty` i18n). `EmailRow` renders per-email labels and AI category chips with dynamic row height.

### Email Operations & Delete Reliability — 2026-06-18

**Backend:** `MoveEmail` UPDATE scoped by `account_id`. `BulkDeleteEmails` removes `emails_fts` rows and runs inside `withWriteRetry`. Bulk goroutines collect errors via mutex-protected `bulkErr` instead of fire-and-forget. Delete/move handlers use `context.WithoutCancel` for detached writes.

**Frontend:** `handleDeleteEmail` shows error toast; selection advances only after `mutateAsync` succeeds. Keyboard delete routes through `handleDeleteEmail` for consistent feedback.

**Tests:** `TestMoveEmailAndEnqueueIMAP_MovesToTrash` — SQLite move-to-trash + IMAP queue atomicity.

### Move Dialog & Hotkey UX — 2026-06-18

**Move to folder:** `MoveToFolderDialog` replaces `window.prompt` on `M` — in-dialog folder list with solid background, arrow-key navigation, Enter/double-click confirm.

**Hotkey stack:** Global shortcuts suppressed when focus is inside `[role="dialog"]` (`isInsideModal`) — arrow keys no longer switch emails while picking a folder. `Esc` blurs search input via `ui:dismiss` without deselecting the current email.

### E2E — Local Inbox Flow — 2026-06-18

- `e2e/helpers/credentials.ts`, `e2e/helpers/login.ts` — reusable login for local dev.
- `e2e/inbox-flow.spec.ts` — login, email viewing, bulk actions.
- `npm run test:e2e:local` — loads `E2E_EMAIL` / `E2E_PASSWORD` from `app_build/.env` or `.env.production` (or `E2E_ENV_FILE` override).

### Pre-Release Audit Remediation — 2026-06-18

**Backend — access control & data isolation:**
- `ensureEmailAccess` replaces legacy `checkEmailAccess` — email ownership always verified before mutations.
- Storage UPDATE/DELETE/toggle paths scoped by `account_id` in SQLite and Postgres.
- `verifyBulkEmailAccess` on bulk read/unread/flag and bulk delete/archive/move by ID.
- `CheckAccountAccess` uses `EqualFold` on DB path (matches cache path).
- Mono unified list/search/select-all: `scopedAccountIDs`, `filterEmailsByMonoAccess`, per-account `GetEmailIDs` / `GetEmailCount`.
- Project groups (`group:`): select-all count/IDs and bulk filter actions aggregate per group member account via `perAccountScopeIDs` / `runBulkFilterOp`.

**Backend — bulk & move reliability:**
- SQLite `Bulk*ByFilter` wrapped in `withWriteRetry` (no direct `db.Exec` bypass of write queue).
- Write worker `recover()` on panic.
- `resolveBulkMoveTarget` — unified/sidebar drag maps folder UUID to same-named folder per account.
- `folderNameMap` replaces N+1 `GetFolderByID` in bulk filter IMAP enqueue.
- `buildFilterWhere` / `buildPGWhere`: `INBOX` by name for single-account filter paths.

**Frontend — cache & UX:**
- `query-cache.ts` — shared optimistic helpers for `emails-infinite`, `folders`, `email` detail.
- Infinite list cursor stored per page (`{ items, nextCursor }`) — no global `_emailListCursor` race on account switch.
- `useBatchEmailLabels/Tags` — stable query key, batch capped at 200 IDs.
- `useBulkAction` — optimistic UI for select-all (filter mode).
- `HotkeyManager` logs gated behind `NODE_ENV === 'development'`.
- Unified move dialog: folders from selected email's `account_id`; `isInsideModal` hotkey guard.

**Tests:** `email_helpers_test.go` (`resolveBulkMoveTarget`, group aggregation). `go test ./...` + vitest 17/17 + `tsc --noEmit`.

**Known accepted risk:** `npm audit` reports 7 transitive issues in `next`/`@ducanh2912/next-pwa` (postcss, serialize-javascript). `npm audit fix --force` would downgrade Next.js — **not applied** (version rollback unacceptable).

### Post-Audit Hardening & Inbox UX — 2026-06-18

**Backend — security & correctness (Unified/Mono; Teams handlers untouched):**
- `CheckAccountAccess`: cache hit now runs `requireAdminForNonMono` — no admin bypass on warm `cache:account:meta:*`.
- `getEmailTags` → `ensureEmailAccess`; `GetBatchEmailTags` / `GetBatchEmailLabels` → `verifyBulkEmailAccess`.
- AI `summarizeEmail` / `AICategorizeEmail`: ownership check **before** `readEncryptedFile`.
- `GetEmails?group_id=`: account list filtered through `CheckAccountAccess` per member.
- Mono unified: empty `scopedAccountIDs` → empty list; `filterEmailsByMonoAccess` with no allowed accounts → `[]`.
- `verifyBulkEmailAccess` requires every ID to exist; bulk by ID capped at `maxBulkEmailIDs` (10 000).
- Filter bulk IMAP: email snapshot **before** `BulkMoveByFilter` — correct `source_folder_name` in queue (was target folder after move).
- Bulk flag: `BulkSetFlagEmails` / `BulkSetFlagByFilter` + optional `set_flagged` in bulk API (Gmail-style: all flagged → unflag all, else flag all).
- `/api/emails/count`: `flagged=true` and `has_attachments=true` via `EmailCountOpts` (same aggregation as unread).

**Frontend — filter badges & bulk flag:**
- Flagged / attachments chip counts from server (`email-folder-counts` React Query), not `emails.filter()` on loaded infinite-scroll slice.
- `resolveBulkSetFlagged` + `set_flagged` on bulk mutations; optimistic updates apply uniform flag state (not per-row toggle).
- Counts invalidated after bulk flag and single-email flag toggle.

**Frontend — swipe vs drag (desktop):**
- Swipes disabled on desktop (`swipeEnabled` at `<1024px`, `pointerType === 'touch'` only).
- Desktop drag-and-drop: `draggable` on `EmailRow`; virtual list rows positioned with `top: virtualRow.start` instead of `transform: translateY` (fixes DnD hit-test — only first row worked).
- Semi-transparent drag preview (`setDragImage` ~42% opacity); source row 35% opacity; list scroll locked during drag.

**Frontend — hotkeys & polish:**
- `bulk-selection-guard`: single-email `d` / `e` / `a` skipped when multi-select or select-all active.
- `useSnoozeEmail`: optimistic cache via `query-cache.ts` prefix (`emails-infinite` lists).
- `handleArchive`: selection advances only after successful `mutateAsync`.
- Batch labels/tags: chunked API requests (200 IDs per chunk, all loaded rows).
- Inbox `isError` UI instead of misleading empty state; bulk failure toast on rollback.

### Migration System & Mono Bootstrap — 2026-06-18

**Edition-aware migration filters:**
- `FilesForPostgres()` skips `*_mono.sql`; `FilesForSQLite()` prefers mono twins and excludes Postgres-only files (`005_partition_emails.sql`, `006_uid_validity_bigint.sql`, `010_partition_emails_64.sql`, `021_folders_name_lower_generated.sql`).
- `IsPostgresOnlyMigration()` exported for tests and future migration tooling.

**Synchronous bootstrap:**
- `bootstrapDatabase()` runs `InitSchema` + `RunMigrations` before the HTTP server starts; migration failure → `os.Exit(1)` (no half-initialized API).

**Postgres legacy DBs:**
- `backfillLegacyPostgresMigrations`: partitioned `emails` + empty `schema_migrations` → mark pre-022 migrations applied without re-running destructive partition SQL.
- `migrationAlreadySuperseded` skips `005` / `010_partition_emails_64` when `emails` is already partitioned (`relkind = p`).
- `isBenignPostgresMigrationError` tolerates idempotent DDL failures (`already exists`, `duplicate column`).
- `InitSchema` safety: `ALTER TABLE folders ADD COLUMN IF NOT EXISTS uid_validity BIGINT DEFAULT 0`.

**SQLite / Mono legacy DBs:**
- `backfillLegacySQLiteMigrations`: existing `emails` table + empty `schema_migrations` → mark pre-`022_folders_uid_validity_mono.sql` migrations applied.
- `isBenignSQLiteMigrationError` tolerates LibSQL `ADD COLUMN IF NOT EXISTS` syntax errors when column already exists via `addColumnIfMissing`.

**Fixes:**
- **Mono inbox "Error" state:** backend crashed on startup trying to run `005_partition_emails.sql` on SQLite — frontend showed `toast_failed` because `/api/emails` was unreachable.
- **Unified Postgres:** `column f.uid_validity does not exist` on legacy installs where async migrations raced with sync; `005` re-run on empty `schema_migrations` with already-partitioned `emails`.

### Per-Folder UIDValidity — 2026-06-18

- `folders.uid_validity` column — migration `022_folders_uid_validity.sql` (Postgres) / `022_folders_uid_validity_mono.sql` (SQLite).
- `models.Folder.UIDValidity`, `UpdateFolderUIDValidity`, `GetFolders` / `GetFolderByID` scans updated (postgres + sqlite).
- `multi_folder.go`: per-folder UIDVALIDITY mismatch → `ClearFolderQueue`, reset `last_sync_uid`, full folder resync; removed incorrect account-level `UpdateAccountUIDValidity`.
- `multi_folder_test.go`: `folderUIDValidityMismatch` regression tests.
- SQLite `InitSchema`: `addColumnIfMissing` for `folders.uid_validity`.

### ESLint CI & E2E Credentials — 2026-06-18

- **ESLint:** `setState-in-effect` fixes in `move-to-folder-dialog.tsx`, `useMailInboxState.ts` (render-time sync pattern); removed unused `onDeleteEmail` chain; `useEmailListVirtualizer` TanStack Virtual eslint-disable; CI lint 0 errors.
- **E2E local:** `scripts/e2e-local.sh` auto-maps `E2E_EMAIL` / `E2E_PASSWORD` from `M_EMAIL` / `M_PASSWORD` in `.env`; `e2e/helpers/login.ts` — API login + `localStorage` (React controlled inputs bypass).
- **Tests:** `filter_test.go` (`FilesForSQLite` postgres-only skip); `go test ./internal/migrations ./internal/store/sqlite` green.

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
- Added strict `enabled` gating checks `typeof window !== "undefined" && localStorage.getItem("rms_edition") !== "mono" && !window.location.host.startsWith("wm.")` to all administrative queries so they simply skip fetching when running in a standalone environment.


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
- **Token in Query String**: SSE rejects `?token=` with 400 (PR-9 complete). Non-SSE routes log deprecation warning when JWT is passed via `?token=`; `Authorization: Bearer` is preferred.

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
- SSE uses relative `/api/events` URL with `?ticket=` or httpOnly cookie — same-origin through Next.js proxy
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
- **FIX**: Fixed port mapping and network binding environment variables in production docker-compose configurations and fixed positional parameter generation edge case in PostgreSQL query builder.

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
