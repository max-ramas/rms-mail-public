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
  <i>Supercharged with an optional AI module, because modern email outgrew traditional webmail clones years ago.</i>
</p>

---

<div align="center">
  <a href="https://ko-fi.com/M7I020HKXX" target="_blank" rel="noopener noreferrer">
    <img src="https://ko-fi.com/img/githubbutton_sm.svg" alt="ko-fi">
  </a>
</div>

---

⚠️ Important Notice for Self-Hosters
Hey everyone, a quick note regarding update frequency: If I spot issues, hidden bugs, or performance bottlenecks under high load, I push fixes immediately. Because of this, you might see multiple patch releases within a single day.
Don't be alarmed by the frequent notifications. I use this app as my primary daily driver, and as a perfectionist, I refuse to tolerate even minor quirks in production.
My philosophy is simple: it is much better to deploy a hotfix instantly and maintain a clean production environment than to accumulate massive changelogs over months while leaving hidden landmines in the code.
I highly recommend always pulling and running the latest build. Keep your containers updated and enjoy the rock-solid stability! 🚀

---

## 🚧 Current State

RMS Mail is actively developed and used in intense real-world highload production workflows.

Current status:
- Mono edition: Stable / Production-Ready
- Unified edition: Stable / Production-Ready
- Teams edition: Planned

The project currently prioritizes:
- core stability & zero-overhead resource usage
- multi-gigabyte mailbox performance & chunk-based fetching
- native database indexing latency minimization
- infrastructure reliability & session state persistence
- workflow ergonomics

Documentation, walkthrough videos, and deployment guides are currently being expanded.

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
13. [📊 Feature Matrix](#-feature-matrix)
14. [💭 Philosophy](#-philosophy)
15. [🗺️ Roadmap](#%EF%B8%8F-roadmap)
16. [🔑 Security: Database Encryption & Key Rotation](#-security-database-encryption--key-rotation)

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
* hundreds of thousands of emails per single mailbox (250K+ validated)
* bulk operations at scale

Unlike traditional IMAP clients: search is locally indexed using native DB engines, metadata is normalized, UI rendering is virtualized, and operations run directly against internal transactional DB pipelines.
**Result:** instant search, smooth scrolling, no IMAP `SEARCH` freezes, and bulk operations executed instantly on huge data sets.

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
* `search_emails` (fully isolated by MCP API Key context)
* `get_email`
* `email_agent` (Natural language email operations with native tool-calling maps)

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

*The same AI + mailbox system works consistently across browser UI, Telegram, MCP, and agents.*

---

## 🛠️ Inbox Mastery at Scale

We fixed the most annoying UX limitations of self-hosted email:

* **Smart Mail Auto-Discovery:** Dynamic Mail Server Resolver automatically discovers IMAP/SMTP hosts, ports, and encryption methods based purely on your email domain. No Thunderbird-style setup hell.
* **Resilient IMAP Sync Batching:** Synchronization queries data in strict 500-UID increments, flushing checkpoints immediately to protect against server-side TCP timeouts on immense mailboxes (200,000+ messages).
* **Unlimited Bulk-by-Filter Actions:** Works on ANY folder density. Select all emails and apply read/unread/delete mutations instantly. No "visible rows only" limitations. No pagination or heavy JSON processing memory overhead.
* **Thread Chains (Conversations):** Full Gmail-style conversation threading. Smart grouping with a per-user toggle to switch between classic list and threaded views on the fly.
* **Configurable Send Delay (Undo Send):** Not just an "oops button". A robust, persistent backend Redis ZSET queue manages outbound mails. Graceful system shutdowns preserve pending items.
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
| **Unified** | **Released** | Multi-account workspace with unified inboxes (PostgreSQL + Redis). |
| **Teams** | **Planned** | Shared mailbox collaboration & helpdesk workflows. |

### Mono
> **One mailbox. Zero infrastructure complexity.**

A multi-user deployment enforcing strict 1:1 mapping between a user profile and a single isolated mailbox. Mono intentionally avoids infrastructure complexity: no PostgreSQL, no Redis, no Kubernetes, no external dependencies.
Replaces Roundcube/SnappyMail and outdated self-hosted webmail stacks. Runs on SQLite and a single binary.

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
* AI-native workflows
* Telegram & MCP integrations
* browser & Telegram notifications
* configurable email threading & Undo Send delay
* Bulk-by-Filter operations for huge folders
* webhook automation
* keyboard-first workflow (layout independent)
* rich HTML composer
* labels, rules
* real-time SSE updates
* pin / snooze / mute
* SPF/DKIM verification & anti-spoofing checks
* 45 languages

---

<video src="https://github.com/user-attachments/assets/70ce2ed9-e458-4f17-b601-6d25377cda13" autoplay loop muted playsinline width="100%"></video>

---

### Unified
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
* OAuth2 Applications configuration layer (BYOA)
* dual unread counters (individual account vs unified inbox)
* centralized notifications

### Teams
> **Email-native collaboration.**

Extends Unified for support teams, agencies, and operations teams living inside shared inboxes.
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
│   React 19 · TipTap · Framer Motion · TanStack Virtual   │
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
     ┌─────────┐   ┌──────────┐   ┌─────────┐
     │ SQLite  │   │PostgreSQL│   │  Redis  │
     │ (Mono)  │   │(Unified) │   │(Unified)│
     └─────────┘   └──────────┘   └─────────┘

```

### Tech Stack

**Frontend:**

* Next.js 16
* React 19
* Tailwind CSS 4
* TipTap
* TanStack Virtual
* next-intl
* Framer Motion

**Backend:**

* Go 1.26
* SQLite (Mono FTS5 virtual tables)
* PostgreSQL (Unified GIN-indexed tsvector partitions)
* Redis (Unified AOF Persistence)
* SSE
* MCP

---

## ⚡ Native Database FTS & Performance Pipeline

RMS Mail does not rely on slow IMAP search or memory-heavy external indexing sidecars. Every email passes through a zero-overhead pipeline ensuring instant access even inside massive directories.

```
┌─────────┐     ┌────────────────────────┐     ┌───────────────────────────────┐     ┌─────────┐
│  IMAP   │ ──▶ │  SQLite FTS5 (Mono)    │ ──▶ │  Go Memory Pre-parsing        │ ──▶ │   UI    │
│ Server  │     │  Postgres GIN (Unified)│     │  zstd Raw Payload Compression │     │ (React) │
└─────────┘     └────────────────────────┘     └───────────────────────────────┘     └─────────┘

```

**Pipeline:**

1. Batch-based IMAP synchronization (500 UID chunks).
2. Metadata normalization and cross-language UTF-8 strict sanitization.
3. Native full-text index generation (SQLite FTS5 virtual engine / PostgreSQL `tsvector`).
4. Real-time UI virtualization.

**Result:** sub-100ms text search, instant filter counting, fast bulk operations.

---

## 📧 Gmail-Style Email Processing

Incoming emails are normalized before rendering to ensure privacy and safety.

```
Raw MIME ──▶ enmime parser ──▶ HTML sanitization ──▶ CSS normalization ──▶ Safe rendering

```

**Features:**

* MIME normalization
* HTML sanitization (bluemonday backend whitelist wrapping)
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

# 4. Copy the Mono-specific compose configuration
cp docker-compose-m.yml docker-compose.yml

# 5. Fire it up!
docker compose up -d

```

Once started, open your browser and navigate to:
👉 `http://localhost:3000`

### Unified

```bash
# 1. Clone the repository and navigate to the project directory
git clone https://github.com/max-ramas/rms-mail-public.git
cd rms-mail-public

# 2. Set up your environment variables
cp .env-u.example .env

# 3. Configure required variables inside the .env file:
# - `POSTGRES_PASSWORD` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `ENCRYPTION_KEYS` or `ENCRYPTION_KEY`(To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `JWT_SECRET` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)
# - `CAMO_HMAC_KEY` (To generate a secure random 32-byte hex key, run: openssl rand -hex 32)

# 4. Copy the Unified-specific compose configuration
cp docker-compose-u.yml docker-compose.yml

# 5. Fire it up!
docker compose up -d

```

Once started, open your browser and navigate to:
👉 `http://localhost:3000`

---

## 📊 Feature Matrix

| **Feature** | **Mono** | **Unified** | **Teams** |
| --- | --- | --- | --- |
| IMAP Sync + IDLE Push | ✅ | ✅ | ✅ |
| SMTP Send Engine | ✅ | ✅ | ✅ |
| AI Gateway (10 providers) | ✅ | ✅ | ✅ |
| AI Chat + Native Tool-calling | ✅ | ✅ | ✅ |
| Telegram Bot Orchestration | ✅ | ✅ | ✅ |
| MCP Server Protocol Engine | ✅ | ✅ | ✅ |
| Native Database FTS Search | ✅ (FTS5) | ✅ (tsvector) | ✅ (tsvector) |
| PWA (Installable Web App) | ✅ | ✅ | ✅ |
| Command Palette & Hotkeys | ✅ | ✅ | ✅ |
| Dynamic IMAP/SMTP Resolver | ✅ | ✅ | ✅ |
| Auto-Draft (UIDPLUS + SSE) | ✅ | ✅ | ✅ |
| Zstd Compression & GC | ✅ | ✅ | ✅ |
| Seamless Key Rotation CLI | ✅ | ✅ | ✅ |
| Unlimited Bulk-by-Filter SQL | ✅ | ✅ | ✅ |
| Full Mobile Responsiveness | ✅ | ✅ | ✅ |
| Drafts with Autosave | ✅ | ✅ | ✅ |
| Private Email Notes | ✅ | ✅ | ✅ |
| Labels, Rules Architecture | ✅ | ✅ | ✅ |
| Rich HTML TipTap Composer | ✅ | ✅ | ✅ |
| 45 Languages (i18n) | ✅ | ✅ | ✅ |
| Thread Chains (Toggleable) | ✅ | ✅ | ✅ |
| Configurable Send Delay | ✅ | ✅ | ✅ |
| Browser & TG Notifications | ✅ | ✅ | ✅ |
| IDE / Agent Integration | ✅ | ✅ | ✅ |
| Pin / Snooze / Mute | ✅ | ✅ | ✅ |
| Hash Partitioning (64x) | — | ✅ | ✅ |
| Multi-Account Unified Inbox | — | ✅ | ✅ |
| Project Groups Isolation | — | ✅ | ✅ |
| PostgreSQL + Redis Infrastructure | — | ✅ | ✅ |
| OAuth 2.0 Applications (BYOA) | — | ✅ | ✅ |
| Shared Mailboxes | — | — | 🚧 |
| Assignments Workflow | — | — | 🚧 |
| Internal Comments Thread | — | — | 🚧 |
| SLA Tracking & Dashboards | — | — | 🚧 |
| Role-based Access Layers | — | — | 🚧 |

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

---

## 🗺️ Roadmap

Current priorities:

* Teams edition launch
* onboarding simplification
* deeper IDE integrations
* more automation workflows
* expanded AI orchestration

---

## 🔑 Security: Database Encryption & Key Rotation

RMS Mail securely encrypts sensitive data at rest using **AES-256-GCM**.

* **Key Derivation:** Raw keys provided via environment variables are hashed using SHA-256 to guarantee a 32-byte length.
* **Storage:** A secure, random 12-byte nonce is generated for every database entry. The result is stored as a base64-encoded string (`nonce + ciphertext`) and is fully supported on both PostgreSQL and SQLite.

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

## ⚖️ License

AGPLv3
