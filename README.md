<p align="center">
  <img src="screenshots/rms-mail-logo.png" alt="RMS Mail Logo">
</p>

---

<p align="center">
  <img src="https://img.shields.io/badge/go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/next.js-16-000000?style=for-the-badge&logo=nextdotjs&logoColor=white" alt="Next.js 16">
  <img src="https://img.shields.io/badge/react-19-61DAFB?style=for-the-badge&logo=react&logoColor=black" alt="React 19">
  <img src="https://img.shields.io/badge/sqlite-003B57?style=for-the-badge&logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/postgresql-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/redis-FF4438?style=for-the-badge&logo=redis&logoColor=white" alt="Redis">
  <img src="https://img.shields.io/badge/docker-ready-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker">
  <img src="https://img.shields.io/badge/self--hosted-privacy--first-6366f1?style=for-the-badge" alt="Privacy First">
  <img src="https://img.shields.io/badge/45_languages-i18n-6366f1?style=for-the-badge" alt="45 Languages">
  <img src="https://img.shields.io/badge/license-AGPLv3-blue?style=for-the-badge" alt="AGPLv3">
</p>

<p align="center">
  <b>High-performance self-hosted email built for large-scale, multi-account workflows.</b><br>
  Built for developers, operators, and power users managing real-world workloads at scale.<br>
  <i>Optional AI integrations available, because modern email outgrew traditional webmail clones years ago.</i><br>
  <b>Designed with a simple philosophy: predictable performance, minimal resource usage, and no unnecessary complexity.</b>
</p>

---

<div align="center">
  <a href="https://ko-fi.com/M7I020HKXX" target="_blank" rel="noopener noreferrer">
    <img src="https://ko-fi.com/img/githubbutton_sm.svg" alt="ko-fi">
  </a>
</div>

---

## 🚧 Current State

RMS Mail is actively developed and used in production environments.

Current status:
- Mono edition: Stable (v3.1.5)
- Unified edition: Stable (v3.1.5)
- Mono Pro edition: Stable (v3.1.5)
- Teams edition: Planned

Current development priorities:
- core stability & zero-overhead resource usage
- multi-gigabyte mailbox performance & chunk-based fetching
- fast search and low-latency mailbox operations
- infrastructure reliability & session state persistence
- workflow ergonomics

Documentation, walkthrough videos, and deployment guides are continuously expanding.

Production HTTPS / reverse-proxy setup:
**[reverse-proxy.md](./reverse-proxy.md)**

Full technical history:
**[CHANGELOG.md](./CHANGELOG.md)**

---

## License & Source

This repository is licensed under AGPLv3 and contains the full source code of the RMS Mail Mono edition.

The Mono edition is fully open-source and self-hostable.

Other editions (Unified, Mono Pro, Teams) are distributed as prebuilt Docker images and are not included in this repository.

---

## 📑 Table of Contents

1. [🖥️ Why Browser-First?](#%EF%B8%8F-why-browser-first)
2. [💡 Why RMS Mail Exists](#-why-rms-mail-exists)
3. [🚀 What Makes RMS Mail Different?](#-what-makes-rms-mail-different)
4. [🧠 The Programmable Inbox](#-the-programmable-inbox)
5. [🛠️ Inbox Mastery at Scale](#%EF%B8%8F-inbox-mastery-at-scale)
6. [👥 Who is this for?](#-who-is-this-for)
7. [🎯 Editions](#-editions)
8. [🏗️ Architecture & Tech Stack](#️-architecture--tech-stack)
9. [⚡ Native Database FTS & Performance Pipeline](#-native-database-fts--performance-pipeline)
10. [📧 Gmail-Style Email Processing](#-gmail-style-email-processing)
11. [🌍 Internationalization (45 Languages)](#-internationalization-45-languages)
12. [🚀 Quick Start](#-quick-start)
13. [🔒 Production (HTTPS / Reverse Proxy)](#-production-https--reverse-proxy)
14. [📊 Feature Matrix](#-feature-matrix)
15. [💭 Philosophy](#-philosophy)
16. [🗺️ Roadmap](#%EF%B8%8F-roadmap)
17. [🔑 Security: Database Encryption & Key Rotation](#-security-database-encryption--key-rotation)

---

## 🖥️ Why Browser-First?

Traditional desktop email clients break down at scale:
- tens of gigabytes of cached mail
- duplicated local databases
- RAM-heavy indexing
- slow synchronization
- poor remote/VPS workflows

RMS Mail moves indexing, synchronization, and storage to the server layer.

Result:
- lightweight clients
- instant access from anywhere
- centralized indexing
- lower hardware usage
- easier backups
- better multi-device workflows

---

## 💡 Why RMS Mail Exists

Modern self-hosted email is still broken. Most webmail clients:
* feel outdated (stuck in 2005)
* become painfully slow on large mailboxes
* have terrible search
* collapse under multi-account workflows
* ignore automation
* bolt AI on top as an afterthought
* force users into desktop apps that cache gigabytes locally

RMS Mail was built from real operational pain:
* many accounts
* millions of emails
* constant context switching
* support-heavy workflows
* browser-first work environments
* AI-assisted operations
* IDE-native automation

**The goal is simple: Make self-hosted email fast, programmable, scalable, and actually pleasant to use.**

---

## 🚀 What Makes RMS Mail Different?

### ⚡ Built for Huge Mailboxes
RMS Mail is designed for:
* tens of accounts
* hundreds of folders
* hundreds of thousands of emails per single mailbox (100K+ validated)
* bulk operations at scale

Unlike traditional IMAP clients: search is locally indexed using native DB engines, metadata is normalized, UI rendering is virtualized, and operations run directly against internal transactional DB pipelines.
**Result:** instant search, smooth scrolling, no IMAP `SEARCH` freezes, and bulk operations executed instantly on huge data sets.

**Multi-client parity:** read, starred, and answered states reconcile with Gmail, Apple Mail, and any other IMAP client — changes propagate in both directions without stale UI.

### Why Go + Next.js?

Go provides predictable memory usage, true concurrency, and low operational overhead.

Next.js delivers a modern browser UI while keeping deployment straightforward.

The combination allows RMS Mail to scale from a Raspberry Pi running Mono to enterprise PostgreSQL deployments without changing the application model.

### 🤖 AI Is Native — Not Bolted On
AI is integrated directly into the Web UI, Telegram, MCP, and IDE workflows. The AI can:
* search your inbox
* summarize threads
* draft replies
* categorize emails
* execute mailbox actions
* operate through tool-calling

**Supported providers:** OpenAI, Anthropic, Gemini, Groq, DeepSeek, Ollama, OpenRouter, Qwen, XAI, OpenCode.
*(Your providers. Your keys. Your infrastructure.)*

### 🧠 Unified Multi-Account Workflow
Unified edition solves one of the biggest missing features in self-hosted email: Real multi-account workflows.
**Features:** unified inboxes, unified project groups, cross-account search, cross-account bulk actions, unified notifications, centralized AI workflows.
**Designed for:** agencies, freelancers, operations teams, infrastructure engineers, support-heavy environments.

---

## 🧠 The Programmable Inbox

RMS Mail is an orchestration layer, not just a client. Control your mailbox from anywhere:

### 🔌 MCP Server & IDE Integration
RMS Mail ships with a native MCP server. Use your mailbox directly from **Cursor, Zed, Claude Desktop**, custom agents, orchestrators, and IDE-integrated workflows.
Available capabilities:
* `search_emails` (fully isolated by MCP API Key context — keys scoped per mailbox account)
* `get_email`
* `email_agent` (Natural language email operations with native tool-calling maps)

*MCP works behind HTTPS reverse proxies (Mono and Unified): SSE and JSON-RPC on `/mcp/*` through the UI entrypoint.*

*This is not an "AI wrapper" integration. Your mailbox becomes part of your agent ecosystem.*

### 💬 Deep Telegram Integration
RMS Mail includes a deeply integrated Telegram bot.
Capabilities:
* inbox summaries
* instant notifications
* AI-assisted chat
* mailbox search
* quick actions (`/archive`, `/delete`, `/reply`)
* workflow automation
* signed webhook payloads (`{ "event": "email.received", "email": { ... } }`) with HMAC-SHA256

*The same AI + mailbox system works consistently across browser UI, Telegram, MCP, and agents.*

---

## 🛠️ Inbox Mastery at Scale

We fixed the most annoying UX limitations of self-hosted email:

* **Smart Mail Auto-Discovery:** Dynamic Mail Server Resolver automatically discovers IMAP/SMTP hosts, ports, and encryption methods based purely on your email domain. No Thunderbird-style setup hell.
* **Resilient IMAP Sync Batching:** Synchronization queries data in strict 500-UID increments with per-batch checkpoints. Streaming fetch writes raw MIME to disk with bounded memory — safe on 200,000+ message folders and large attachments.
* **Multi-account Gmail on one host:** `IMAP_PER_HOST_CONN` limits concurrent dials to `imap.gmail.com` (not open IDLE sessions); sync status shows real OAuth errors (`invalid_grant` → re-authorize in Settings).
* **Camo image proxy:** external images in newsletters proxied with privacy; broken/marketing-mangled URLs degrade gracefully (no console 502 spam).
* **Newsletter fidelity:** email iframe allows HTTPS stylesheets (Google Fonts, bank CDNs) while keeping scripts blocked.
* **IMAP IDLE + watchdog:** Push sync via IDLE with configurable timeouts and reconnect watchdogs; non-INBOX folders scanned on a schedule (Sent, Archive, Drafts with localized mailbox name detection).
* **Full IMAP flag sync:** `\Seen`, `\Flagged`, and `\Answered` reconcile with the server in both directions; outbound changes batch `STORE` (200 UIDs); opening a message can trigger an immediate flag refresh.
* **Unlimited Bulk-by-Filter Actions:** Works on ANY folder density. Select all emails and apply read/unread/delete mutations instantly. No "visible rows only" limitations. No pagination or heavy JSON processing memory overhead.
* **Real-time inbox & counters:** Server-Sent Events drive the open list, folder badges, and filter counts together — with a 30 s fallback poll when SSE reconnects after sleep or proxy drops.
* **AI categories & rules:** Configurable taxonomy (Settings) with per-category auto-read and auto-move after AI tagging; filter chips in the inbox toolbar.
* **Auto-Draft:** AI can prepare reply drafts in the background; viewer shows a “draft ready” banner with one-click open.
* **Thread Chains (Conversations):** Full Gmail-style conversation threading. Smart grouping with a per-user toggle to switch between classic list and threaded views on the fly.
* **Configurable Send Delay (Undo Send):** Persistent outbound queue — Redis ZSET on Unified (and planned Mono Pro); SQLite `scheduled_emails` on Mono. Graceful shutdown preserves pending sends.
* **Folder management:** Create, rename, and delete IMAP folders from the UI with system-folder protection.
* **Smart Notifications:** Browser push notifications via SSE, Telegram push alerts, AI-priority notifications, Rule-based notifications, and real-time IMAP IDLE events.
* **Command Palette & Custom Hotkeys:** Fully rebindable physical position-based keyboard shortcuts (`event.code` layout independent) with a fuzzy-search command palette (`Cmd+Shift+P`) for lightning-fast, mouse-free navigation.
* **PWA (Installable App):** Install RMS Mail as a standalone, native desktop or mobile application with isolated windows and OS-level integration.

---

## 👥 Who is this for?

Ideal for:
* VPS owners
* developers
* homelabs
* self-hosters
* freelancers
* privacy-conscious users

Especially people who:
* hate outdated webmail
* manage email-heavy workflows
* want local AI integration
* use Telegram daily
* work inside IDEs

---

## 🎯 Editions

| Edition | Status | Purpose |
| :--- | :--- | :--- |
| **Mono** | **Stable** | Multi-user deployment with strict 1:1 user-to-mailbox isolation (SQLite). |
| **Mono Pro** | **Stable** | Mono product model on enterprise infrastructure (PostgreSQL + Redis + async workers). |
| **Unified** | **Stable** | Multi-account workspace with unified inboxes (PostgreSQL + Redis). |
| **Teams** | **Planned** | Unified workspace plus shared-mailbox collaboration & helpdesk workflows. |

### Mono (Completely Free)
> **One mailbox. Zero infrastructure complexity.**

A multi-user deployment enforcing strict 1:1 mapping between a user profile and a single isolated mailbox. Mono intentionally avoids infrastructure complexity: no PostgreSQL, no Redis, no Kubernetes, no external dependencies.
Replaces Roundcube/SnappyMail and outdated self-hosted webmail stacks. Runs on **LibSQL/SQLite** (WAL, `busy_timeout`) with a single backend binary plus Next.js UI container.

Designed for people who want modern email without operating enterprise infrastructure.

**Docker images:** `maxramas/rms-mail:m-latest` (API) + `maxramas/rms-mail:m-ui-latest` (UI) — see Quick Start for compose setup.

---

<p align="center">
  <img src="screenshots/mono-1.png" alt="RMS Mail Mono Interface" width="800">
  <br>
  <i>RMS Mail Mono Interface</i>
</p>

---

**Features:**
* modern Apple Mail-inspired UI
* zero-overhead native SQLite FTS5 search
* IMAP IDLE push sync with TCP keepalive watchdogs
* bidirectional IMAP flag sync (`\Seen` / `\Flagged` / `\Answered`)
* AI-native workflows + configurable AI categories & rules
* Auto-Draft replies in the viewer
* Telegram & MCP integrations (HTTPS-safe via UI `:3000` proxy)
* browser & Telegram notifications
* configurable email threading & Undo Send delay
* Bulk-by-Filter operations for huge folders
* webhook automation with signed payloads
* keyboard-first workflow (layout independent)
* rich HTML composer
* labels, rules, folder CRUD
* real-time SSE inbox + aligned unread counters
* pin / snooze / mute
* private per-email notes
* SPF/DKIM verification & anti-spoofing checks
* 45 languages

---

<video src="https://github.com/user-attachments/assets/70ce2ed9-e458-4f17-b601-6d25377cda13" autoplay loop muted playsinline width="100%"></video>

---

### Mono Pro (Paid)
> **Mono isolation. Enterprise infrastructure.**

Keeps the **Mono product model**: each user profile maps to **one mailbox**, with no unified multi-account inbox and no cross-account project groups. Swaps SQLite for the **Unified enterprise stack** — PostgreSQL, Redis (AOF persistence), and Asynq-backed async workers (Telegram, avatars, webhooks, scheduled send).

**Best for:** self-hosters and MSPs who want Mono-style 1:1 isolation and licensing, but need PostgreSQL scale, durable Redis queues, and production-grade session/rate-limit infrastructure — without adopting a multi-inbox agency workspace.

**Everything from Mono, on PostgreSQL + Redis, plus:**
* hash-partitioned email storage (PostgreSQL)
* persistent Undo Send & webhook retry queues (Redis ZSET / Asynq)
* strict unactivated limit (1 admin account) with license enforcement
* horizontal-ready job queues (same async foundation as Unified)
* dedicated build and local-testing environments (`run-mp.sh`, `beta-mp.sh`, `bp-mp.sh`)

---

### Unified (Freemium)
> **All your inboxes. One workspace.**

Designed for users managing many inboxes, client accounts, infrastructure mail, support-heavy workflows, and personal + business communication.

🎁 **Summer Promotion:** Use code **`SUMMER15`** before August 31 to get **15% off** on your Unified license.

---

<p align="center">
  <img src="screenshots/unified-1.png" alt="RMS Mail Unified Interface" width="800">
  <br>
  <i>RMS Mail Unified Interface</i>
</p>

---

**Everything from Mono plus:**
* unified inbox & native cross-account PostgreSQL `tsvector` FTS
* unified project groups with live aggregated count subqueries
* 64x Hash Partitioning on emails table for B-Tree safety
* persistent Redis backing (AOF mode) for sessions, jobs and rate limiters
* Asynq task queue — Telegram, avatars, webhooks, scheduled send with retries (`/mon/` dashboard)
* dual PostgreSQL pools — sync workers isolated from HTTP handlers
* OAuth2 Applications configuration layer (BYOA)
* dual unread counters (individual account vs unified inbox)
* centralized notifications
* license enforcement with live backend limits (Free vs Premium)

**Docker images:** `maxramas/rms-mail:u-latest` + `maxramas/rms-mail:u-ui-latest`.

### Teams
> **Email-native collaboration.**

Extends **Unified** (multi-account workspace **and** PostgreSQL + Redis + async workers) for support teams, agencies, and operations teams living inside shared inboxes.

**Mono Pro vs Teams (short):**

| | **Mono Pro** | **Teams** |
| :--- | :--- | :--- |
| **Product model** | Mono — 1 user ↔ 1 mailbox | Unified — many accounts, unified inbox & groups |
| **Infrastructure** | PostgreSQL + Redis + async | Same (inherits Unified) |
| **Collaboration** | — | Shared mailboxes, assignments, SLA, internal comments, RBAC |

**Everything from Unified plus:**
* shared mailboxes
* assignments
* SLA tracking
* internal comments
* role-based access
* team notifications
* shared automation

*(If your company needs Teams edition, please contact us or open an issue).*

---

## 🏗️ Architecture & Tech Stack

```text
┌──────────────────────────────────────────────────────────┐
│                  Frontend (Next.js 16)                   │
│   React 19 · TipTap · TanStack Virtual                    │
│   45 languages (next-intl) · Tailwind CSS · shadcn/ui    │
└────────────────────────┬─────────────────────────────────┘
                         │ REST + SSE
┌────────────────────────▼─────────────────────────────────┐
│                   Backend (Go 1.26)                      │
│                                                          │
│  ┌───────────┐  ┌──────────┐  ┌────────────────────┐     │
│  │ IMAP/IDLE │  │ SMTP     │  │ MCP Server         │     │
│  │ Sync      │  │ Client   │  │ (JSON-RPC + SSE)   │     │
│  └───────────┘  └──────────┘  └────────────────────┘     │
│                                                          │
│  ┌───────────┐  ┌──────────┐  ┌────────────────────┐     │
│  │ AI Gateway│  │ Telegram │  │ JWT Auth           │     │
│  │ (10 LLMs) │  │ Bot      │  │ + MCP API Keys     │     │
│  └───────────┘  └──────────┘  └────────────────────┘     │
│                                                          │
│  ┌───────────┐  ┌──────────┐                             │
│  │ Native    │  │ AES-GCM  │                             │
│  │ DB FTS    │  │ Crypto   │                             │
│  └───────────┘  └──────────┘                             │
└────────────────────────┬─────────────────────────────────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
     ┌─────────┐   ┌──────────┐   ┌──────────┐
     │ SQLite  │   │PostgreSQL│   │  Redis   │
     │ (Mono)  │   │(U, MP, T)│   │(U, MP, T)│
     └─────────┘   └─────┬────┘   └────┬─────┘
                         │             │
                         └────Asynq────┘

```

*PostgreSQL and Redis also power the planned **Mono Pro** and **Teams** editions.*

### Tech Stack

**Frontend:**

* Next.js 16
* React 19
* Tailwind CSS 4
* TipTap
* TanStack Virtual
* next-intl

**Backend:**

* Go 1.26
* SQLite (Mono FTS5 virtual tables)
* PostgreSQL (Unified / Mono Pro / Teams — GIN-indexed `tsvector`, hash partitions)
* Redis (Unified / Mono Pro / Teams — AOF persistence, queues, rate limits)
* Asynq task workers (Unified / Mono Pro / Teams)
* SSE
* MCP

---

## ⚡ Native Database FTS & Performance Pipeline

RMS Mail does not rely on slow IMAP search or memory-heavy external indexing sidecars. Every email passes through a zero-overhead pipeline ensuring instant access even inside massive directories.

```
┌─────────┐     ┌────────────────────────┐     ┌───────────────────────────────┐     ┌─────────┐
│  IMAP   │ ──▶ │   SQLite FTS5 (Mono)   │ ──▶ │  Go Memory Pre-parsing        │ ──▶ │   UI    │
│ Server  │     │ Postgres GIN (U, MP, T)│     │  zstd Raw Payload Compression │     │ (React) │
└─────────┘     └────────────────────────┘     └───────────────────────────────┘     └─────────┘

```

**Pipeline:**

1. Batch-based IMAP synchronization (500 UID chunks) with streaming MIME to disk.
2. Metadata normalization and cross-language UTF-8 strict sanitization.
3. Native full-text index generation (SQLite FTS5 virtual engine / PostgreSQL `tsvector` + GIN).
4. Keyset cursor pagination `(is_pinned, date_sent, id)` — O(1) depth at any inbox offset.
5. PostgreSQL production tuning (v3.1.0): covering index `(folder_id, is_read, is_muted, is_pinned DESC, date_sent DESC, id DESC)` eliminates sort on filtered reads; BRIN index on `date_sent` for compact time-series aggregations; connection pool `min(20, CPU*4)` prevents OOM in Docker; `ANALYZE emails` after bulk sync prevents Seq Scan; `autovacuum_vacuum_insert_scale_factor = 0.05` triggers vacuum promptly on insert-heavy workloads.
6. Real-time UI virtualization (TanStack Virtual + `measureElement`).

**Result:** typically sub-100 ms text search, instant filter counting, fast bulk operations on six-figure mailboxes.

---

## 📧 Gmail-Style Email Processing

Incoming emails are normalized before rendering to ensure privacy and safety.

```
Raw MIME ──▶ enmime parser ──▶ HTML normalization ──▶ iframe CSP sandbox ──▶ Safe rendering

```

**Features:**

* MIME normalization
* HTML normalization (`sanitizeNode`) with iframe `srcdoc` CSP boundary (`script-src 'none'`); HTML4→CSS3 attribute conversion including `align="center"` → `-webkit-center`/`-moz-center` for correct block centering in standards mode
* quote folding
* inline attachment support
* tracking protection
* XSS / XXE protection
* privacy-first rendering (Camo HMAC-signed image proxy)

---

## 🌍 Internationalization (45 Languages)

RMS Mail supports 45 languages out of the box. Includes LTR/RTL support, live language switching, localized dates, and full UI localization.

**Supported regions:** Europe, Middle East, East Asia, South Asia, Central Asia, Caucasus, Southeast Asia.

| **Code** | **Language** | **Code** | **Language** | **Code** | **Language** |
| --- | --- | --- | --- | --- | --- |
| `en` | 🇬🇧 English | `ru` | 🇷🇺 Русский | `zh` | 🇨🇳 中文 |
| `ja` | 🇯🇵 日本語 | `ko` | 🇰🇷 한국어 | `ar` | 🇸🇦 العربية |
| `he` | 🇮🇱 עברית | `hi` | 🇮🇳 हिन्दी | `bn` | 🇧🇩 Bengali |
| `ur` | 🇵🇰 اردو | `fa` | 🇮🇷 فارسی | `tr` | 🇹🇷 Türkçe |
| `de` | 🇩🇪 Deutsch | `fr` | 🇫🇷 Français | `es` | 🇪🇸 Español |
| `it` | 🇮🇹 Italiano | `nl` | 🇳🇱 Nederlands | `pl` | 🇵🇱 Polski |
| `cs` | 🇨🇿 Čeština | `sk` | 🇸🇰 Slovenčina | `hu` | 🇭🇺 Magyar |
| `ro` | 🇷🇴 Română | `bg` | 🇧🇬 Български | `el` | 🇬🇷 Ελληνικά |
| `sr` | 🇷🇸 Српски | `hr` | 🇭🇷 Hrvatski | `sl` | 🇸🇮 Slovenščina |
| `sv` | 🇸🇪 Svenska | `da` | 🇩🇰 Dansk | `nb` | 🇳🇴 Norsk |
| `fi` | 🇫🇮 Suomi | `et` | 🇪🇪 Eesti | `lv` | 🇱🇻 Latviešu |
| `lt` | 🇱🇹 Lietuvių | `uk` | 🇺🇦 Українська | `kk` | 🇰🇿 Қазақша |
| `ka` | 🇬🇪 ქართული | `hy` | 🇦🇲 Հայերեն | `az` | 🇦🇿 Azərbaycanca |
| `uz` | 🇺🇿 Oʻzbekcha | `vi` | 🇻🇳 Tiếng Việt | `th` | 🇹🇭 ไทย |
| `id` | 🇮🇩 Indonesia | `ms` | 🇲🇾 Melayu | `ca` | 🇪🇸 Català |

---

## 🚀 Quick Start

### All editions

```bash
# 1. Clone the repository and navigate to the project directory
git clone https://github.com/max-ramas/rms-mail-public.git
cd rms-mail-public

# 2. Run configuration script
./configure.sh

# 3. Fire it up!
docker compose up -d

# Check status
docker compose ps

```

For **HTTPS production** (aaPanel, nginx, Caddy), see **[reverse-proxy.md](./reverse-proxy.md)**

### Mono

```bash
# 1. Clone the repository and navigate to the project directory
git clone https://github.com/max-ramas/rms-mail-public.git
cd rms-mail-public

# 2. Set up your environment variables
cp .env-m.example .env

# 3. Configure your `ENCRYPTION_KEYS` or `ENCRYPTION_KEY` and `JWT_SECRET` inside the .env file
# (You only need to enter the `ENCRYPTION_KEYS` or `ENCRYPTION_KEY` and `JWT_SECRET`; that is all the app needs to function)
# To generate a secure random 32-byte hex key, run: openssl rand -hex 32
# Also add ALLOWED_ORIGINS and FRONTEND_URL (your domain name)

# 4. Copy the Mono-specific compose configuration
cp docker-compose-m.yml docker-compose.yml

# 5. Fire it up!
docker compose up -d

# Images are pulled from Docker Hub: maxramas/rms-mail:m-latest + m-ui-latest
```

Once started, open your browser and navigate to:
👉 `http://localhost:3000`

For **HTTPS production** (aaPanel, nginx, Caddy), see **[reverse-proxy.md](./reverse-proxy.md)** — proxy **`:3000` only** on Mono.

### Unified

```bash
# 1. Clone the repository and navigate to the project directory
git clone https://github.com/max-ramas/rms-mail-public.git
cd rms-mail-public

# 2. Set up your environment variables
cp .env-u.example .env

# 3. Configure required variables inside the .env file:
# - `POSTGRES_PASSWORD` (To generate a secure random 32-byte hex key, run: openssl rand -hex 16)
# - `REDIS_PASSWORD` (To generate a secure random 32-byte hex key, run: openssl rand -hex 16)
# - `ENCRYPTION_KEYS` or `ENCRYPTION_KEY`(To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `JWT_SECRET` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `CAMO_HMAC_KEY` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# Also add ALLOWED_ORIGINS and FRONTEND_URL (your domain name)

# 4. Copy the Unified-specific compose configuration
cp docker-compose-u.yml docker-compose.yml

# 5. Fire it up!
docker compose up -d

# Images: maxramas/rms-mail:u-latest + u-ui-latest
```

Once started, open your browser and navigate to:
👉 `http://localhost:3000`

### Mono Pro

```bash
# 1. Clone the repository and navigate to the project directory
git clone https://github.com/max-ramas/rms-mail-public.git
cd rms-mail-public

# 2. Set up your environment variables
cp .env-mp.example .env

# 3. Configure required variables inside the .env file:
# - `ADMIN_EMAIL` (Critical: The only account allowed to log in before license activation)
# - `POSTGRES_PASSWORD` (To generate a secure random 32-byte hex key, run: openssl rand -hex 16)
# - `REDIS_PASSWORD` (To generate a secure random 32-byte hex key, run: openssl rand -hex 16)
# - `ENCRYPTION_KEYS` or `ENCRYPTION_KEY`(To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `JWT_SECRET` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `CAMO_HMAC_KEY` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# Also add ALLOWED_ORIGINS and FRONTEND_URL (your domain name)

# 4. Copy the Mono Pro-specific compose configuration
cp docker-compose-mp.yml docker-compose.yml

# 5. Fire it up!
docker compose up -d

# Images: maxramas/rms-mail:mp-latest + mp-ui-latest
```
Once started, open your browser and navigate to:
👉 `http://localhost:3000`

---

## 🔒 Production (HTTPS / Reverse Proxy)

After Quick Start, terminate TLS on your domain and proxy to **port 3000** (recommended for both shipped editions). Full nginx / aaPanel examples: **[reverse-proxy.md](./reverse-proxy.md)**.

In `.env`:

```env
FRONTEND_URL=https://your-domain.com
ALLOWED_ORIGINS=https://your-domain.com
```

**Mono:** point the reverse proxy at **`:3000` only** — the backend stays on the Docker network. Next.js rewrites `/api/*`, `/mcp/*`, and `/internal/*` to Go. Forward `X-Forwarded-Host` and `X-Forwarded-Proto` (`$scheme`) for correct HTTPS MCP links.

For SSE (`/api/events`, `/mcp/sse`):

```nginx
proxy_buffering off;
proxy_read_timeout 86400s;
```

---

## 📊 Feature Matrix

| **Feature** | **Mono** | **Mono Pro** | **Unified** | **Teams** |
| --- | --- | --- | --- | --- |
| IMAP Sync + IDLE Push | ✅ | ✅ | ✅ | ✅ |
| Bidirectional IMAP Flags (`\Seen`/`\Flagged`/`\Answered`) | ✅ | ✅ | ✅ | ✅ |
| SMTP Send Engine | ✅ | ✅ | ✅ | ✅ |
| AI Gateway (10 providers) | ✅ | ✅ | ✅ | ✅ |
| AI Chat + Native Tool-calling | ✅ | ✅ | ✅ | ✅ |
| Telegram Bot Orchestration | ✅ | ✅ | ✅ | ✅ |
| MCP Server Protocol Engine | ✅ | ✅ | ✅ | ✅ |
| Native Database FTS Search | ✅ (FTS5) | ✅ (tsvector) | ✅ (tsvector) | ✅ (tsvector) |
| PWA (Installable Web App) | ✅ | ✅ | ✅ | ✅ |
| Command Palette & Hotkeys | ✅ | ✅ | ✅ | ✅ |
| Dynamic IMAP/SMTP Resolver | ✅ | ✅ | ✅ | ✅ |
| Auto-Draft (UIDPLUS + SSE) | ✅ | ✅ | ✅ | ✅ |
| AI Category Taxonomy + Auto Rules | ✅ | ✅ | ✅ | ✅ |
| Real-time SSE Inbox + Counter Sync | ✅ | ✅ | ✅ | ✅ |
| Webhook HMAC (`event` + `email` payload) | ✅ | ✅ | ✅ | ✅ |
| Zstd Compression & GC | ✅ | ✅ | ✅ | ✅ |
| Seamless Key Rotation CLI | ✅ | ✅ | ✅ | ✅ |
| Unlimited Bulk-by-Filter SQL | ✅ | ✅ | ✅ | ✅ |
| Full Mobile Responsiveness | ✅ | ✅ | ✅ | ✅ |
| Drafts with Autosave | ✅ | ✅ | ✅ | ✅ |
| Private Email Notes | ✅ | ✅ | ✅ | ✅ |
| IMAP Folder CRUD (UI) | ✅ | ✅ | ✅ | ✅ |
| Labels, Rules Architecture | ✅ | ✅ | ✅ | ✅ |
| Rich HTML TipTap Composer | ✅ | ✅ | ✅ | ✅ |
| 45 Languages (i18n) | ✅ | ✅ | ✅ | ✅ |
| Thread Chains (Toggleable) | ✅ | ✅ | ✅ | ✅ |
| Configurable Send Delay | ✅ | ✅ (Redis) | ✅ (Redis) | ✅ (Redis) |
| Browser & TG Notifications | ✅ | ✅ | ✅ | ✅ |
| IDE / Agent Integration | ✅ | ✅ | ✅ | ✅ |
| Pin / Snooze / Mute | ✅ | ✅ | ✅ | ✅ |
| 1:1 User ↔ Mailbox Model | ✅ | ✅ | — | — |
| Hash Partitioning (64x) | — | ✅ | ✅ | ✅ |
| Multi-Account Unified Inbox | — | — | ✅ | ✅ |
| Project Groups Isolation | — | — | ✅ | ✅ |
| PostgreSQL + Redis Infrastructure | — | ✅ | ✅ | ✅ |
| Asynq Async Workers | — | ✅ | ✅ | ✅ |
| OAuth 2.0 Applications (BYOA) | — | — | ✅ | ✅ |
| License Enforcement (live limits) | — | — | ✅ | ✅ |
| Shared Mailboxes | — | — | — | 🚧 |
| Assignments Workflow | — | — | — | 🚧 |
| Internal Comments Thread | — | — | — | 🚧 |
| SLA Tracking & Dashboards | — | — | — | 🚧 |
| Role-based Access Layers | — | — | — | 🚧 |

---

## 💭 Philosophy

RMS Mail is built around several core ideas:

* email should be fast
* email should scale to hundreds of thousands of entries natively
* email should be highly programmable
* email should integrate with AI naturally
* users should strictly control their data storage infrastructure
* self-hosted software should not feel outdated

This project is heavily shaped by support workflows, operational reality, multi-account overload, browser-first workflows, AI-assisted productivity, and real infrastructure constraints.

Good software should remain fast as data grows, not only on day one.

---

## Design Principles

RMS Mail intentionally avoids unnecessary abstractions.

Every subsystem is designed around a few engineering principles:

* predictable latency
* bounded memory usage
* streaming over buffering
* native database capabilities instead of external services
* minimal infrastructure
* backwards-compatible evolution

---

## 🗺️ Roadmap

**Current release: v3.1.2 (2026-06-29)** — see [CHANGELOG.md](./CHANGELOG.md) for the full history.

**Recently shipped:**
* **Docker production i18n fix (v3.1.0)** — standalone Next.js builds now correctly bundle and serve all 45 translation namespaces; resolved `MISSING_MESSAGE` errors in production containers.
* **Priority on-demand mail sync (v3.1.0)** — Unified edition: clicking an account in the sidebar triggers an immediate out-of-band IMAP scan (INBOX + all folders), independent from background workers.
* IMAP `\Seen` bidirectional sync (3.0.4) → full `\Flagged` / `\Answered` parity + reply→answered (3.0.7)
* Streaming sync, keyset pagination, denormalized unread counts, smart-category exclusion (3.0.5–3.0.6)
* AI categories, auto-rules, Auto-Draft, folder management UI (3.0.6)
* Atomic inbox SSE refresh, Docker Hub `maxramas/rms-mail`, Mono single-port HTTPS proxy (3.0.7)
* Webhook payload v2, MCP per-account keys, API hardening (3.0.7)
* IMAP multi-account Gmail: dial-only connection cap, OAuth `invalid_grant` surfacing (3.0.7)
* Camo proxy + newsletter CSS in email iframe; About update channel badge (3.0.8)
* Email body text selection fix; SSE `email_updated` payload with real status fields; `is_pinned`/`is_muted` SSE sync (3.0.9)
* **Mono Pro** edition — Mono isolation on PostgreSQL + Redis + async

Current priorities:

* **Teams** edition launch — collaboration layer on top of Unified
* **Mono Pro**/**Teams** edition ecosystem
* onboarding simplification
* deeper IDE integrations
* more automation workflows
* expanded AI orchestration

---

## 🔑 Security: Database Encryption & Key Rotation

RMS Mail securely encrypts sensitive data at rest using **AES-256-GCM**.

* **Key Derivation:** Raw keys provided via environment variables are hashed using SHA-256 to guarantee a 32-byte length. Per-domain key separation for IMAP passwords, OAuth tokens, MCP keys, and Telegram tokens.
* **Storage:** A secure, random 12-byte nonce is generated for every database entry. The result is stored as a base64-encoded string (`nonce + ciphertext`) and is fully supported on both PostgreSQL and SQLite.
* **API auth:** JWT via `Authorization` header or `rms_token` cookie — query-string `?token=` is rejected on API routes (legacy MCP SSE paths excepted).
* **Rate limits:** login, global API, AI (30/min), and search (60/min) tiers with Redis or in-memory fallback on Mono.

### Zero-Downtime Key Rotation

The system supports seamless key rotation. The `ENCRYPTION_KEYS` environment variable accepts a comma-separated list of keys (`encKeys` array).

* **Encryption:** The `encryptPassword` function always uses the primary key (`encKeys[0]`).
* **Decryption:** The `decryptPassword` function iterates through the entire `encKeys` array, returning the result from the first key that successfully decrypts the payload.

To rotate your encryption keys without downtime, follow these steps:

**1. Add the new key**
Update your environment variable. Place the new key at the beginning of the list, keeping the old key as a fallback.

```bash
export ENCRYPTION_KEYS="new-secret-key,old-secret-key"

```

**2. Re-encrypt existing data**
Run the application with the `-rekey` flag. This triggers the `store.RekeyAll()` method, which iterates through the `accounts.password_encrypted` and `mcp_keys.key_encrypted` fields. It decrypts each record using the available fallback keys and re-encrypts them using the new primary key (`encKeys[0]`).

```bash
./rms-mail -rekey
# Expected output: "Rekey complete" "re_encrypted"=5

```

**3. Remove the old key**
Once the rekey process is complete, the old key is no longer needed. You can safely remove it from your environment.

```bash
export ENCRYPTION_KEYS="new-secret-key"

```

---

## 💬 Support & Feedback

If you have any questions, architectural suggestions, or commercial inquiries, feel free to reach out directly:
📧 **rms-mail@rms-ds.com**

---

## ⚖️ License

AGPLv3

Our customers can request source code access under NDA for security review, compliance, or integration purposes.
