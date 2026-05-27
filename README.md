# ✉️ RMS Mail

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
  <b>AI-native self-hosted email built for large-scale, multi-account workflows.</b><br>
  Built for developers, operators, and power users managing real-world workloads at scale.<br>
  <i>Because modern email workloads outgrew traditional webmail clones years ago.</i>
</p>

---

## 📑 Table of Contents

1. [🖥️ Why Browser-First?](#%EF%B8%8F-why-browser-first)
2. [💡 Why RMS Mail Exists](#-why-rms-mail-exists)
3. [🚀 What Makes RMS Mail Different?](#-what-makes-rms-mail-different)
4. [🧠 The Programmable Inbox](#-the-programmable-inbox)
5. [🛠️ Inbox Mastery at Scale](#-inbox-mastery-at-scale)
6. [👥 Who is this for?](#-who-is-this-for)
7. [🎯 Editions](#-editions)
8. [🏗️ Architecture & Tech Stack](#️-architecture--tech-stack)
9. [⚡ Vector Search & Performance Pipeline](#-vector-search--performance-pipeline)
10. [📧 Gmail-Style Email Processing](#-gmail-style-email-processing)
11. [🌍 Internationalization (45 Languages)](#-internationalization-45-languages)
12. [🚀 Quick Start](#-quick-start)
13. [📊 Feature Matrix](#-feature-matrix)
14. [💭 Philosophy](#-philosophy)
15. [🗺️ Roadmap](#-roadmap)

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
* millions of emails
* bulk operations at scale

Unlike traditional IMAP clients: search is locally indexed, metadata is normalized, UI rendering is virtualized, and operations run directly against DB/index pipelines.
**Result:** instant search, smooth scrolling, no IMAP `SEARCH` freezes, and bulk operations on thousands of emails simultaneously.

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
* `search_emails`
* `get_email`
* `email_agent` (Natural language email operations)

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

* **Unlimited Bulk Operations:** Works on ANY mailbox size. Select 10K+, 100K+ emails and apply rules instantly. No "visible rows only" limitations. No pagination hell.
* **Thread Chains (Conversations):** Full Gmail-style conversation threading. Smart grouping with a per-user toggle to switch between classic list and threaded views on the fly.
* **Configurable Send Delay (Undo Send):** Not just an "oops button". A robust, persistent backend queue manages outbound mails. Graceful system shutdowns preserve pending items. Can be toggled or configured per user.
* **Smart Notifications:** Browser push notifications, Telegram push alerts, AI-priority notifications, Rule-based notifications, and real-time IMAP IDLE events.

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
| **Mono** | **Release Candidate** | Multi-user deployment with strict 1:1 user-to-mailbox isolation (SQLite). |
| **Unified** | **Coming Soon** | Multi-account workspace with unified inboxes (PostgreSQL + Redis). |
| **Teams** | **Planned** | Shared mailbox collaboration & helpdesk workflows. |

### Mono
> **One mailbox. Zero infrastructure complexity.**

A multi-user deployment enforcing strict 1:1 mapping between a user profile and a single isolated mailbox. Mono intentionally avoids infrastructure complexity: no PostgreSQL, no Redis, no Kubernetes, no external dependencies.
Replaces Roundcube/SnappyMail and outdated self-hosted webmail stacks. Runs on SQLite and a single binary.

**Features:**
* modern Apple Mail-inspired UI
* instant vector search with Bluge
* IMAP IDLE push sync
* AI-native workflows
* Telegram & MCP integrations
* browser & Telegram notifications
* configurable email threading & Undo Send delay
* bulk operations for huge folders
* keyboard-first workflow
* rich HTML composer
* labels, rules, groups
* pin / snooze / mute
* SPF/DKIM verification
* 45 languages

### Unified
> **All your inboxes. One workspace.**

Designed for users managing many inboxes, client accounts, infrastructure mail, support-heavy workflows, and personal + business communication.
**Everything from Mono plus:**
* unified inbox
* unified project groups
* PostgreSQL + Redis
* real-time SSE updates
* OAuth2
* webhook automation
* AI action logs
* dashboard & sync monitoring
* cross-account workflows
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
┌─────────────────────────────────────────────────────────┐
│                     Frontend (Next.js 16)               │
│  React 19 · TipTap · Framer Motion · TanStack Virtual   │
│  45 languages (next-intl) · Tailwind CSS · shadcn/ui    │
└────────────────────────┬────────────────────────────────┘
                         │ REST + SSE
┌────────────────────────▼────────────────────────────────┐
│                  Backend (Go 1.26)                      │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐     │
│  │ IMAP/IDLE│  │ SMTP     │  │ MCP Server         │     │
│  │ Sync     │  │ Client   │  │ (JSON-RPC + SSE)   │     │
│  └──────────┘  └──────────┘  └────────────────────┘     │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐     │
│  │ AI Gateway│  │ Telegram │  │ JWT Auth           │     │
│  │ (10 LLMs) │  │ Bot      │  │ + MCP API Keys     │     │
│  └──────────┘  └──────────┘  └────────────────────┘     │
│                                                         │
│  ┌──────────┐  ┌──────────┐                             │
│  │ Bluge    │  │ AES-GCM  │                             │
│  │ FTS Index│  │ Crypto   │                             │
│  └──────────┘  └──────────┘                             │
└────────────────────────┬────────────────────────────────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
     ┌─────────┐   ┌──────────┐   ┌─────────┐
     │ SQLite  │   │PostgreSQL│   │  Redis  │
     │ (Mono)  │   │(Unified) │   │(Unified)│
     └─────────┘   └──────────┘   └─────────┘
````

### Tech Stack

**Frontend:**

- Next.js 16
    
- React 19
    
- Tailwind CSS
    
- TipTap
    
- TanStack Virtual
    
- next-intl
    
- Framer Motion
    

**Backend:**

- Go 1.26
    
- SQLite
    
- PostgreSQL
    
- Redis
    
- Bluge
    
- SSE
    
- MCP
    

## ⚡ Vector Search & Performance Pipeline

RMS Mail does not rely on slow IMAP search. Every email passes through a pipeline ensuring smooth virtualization even with huge folders.

Plaintext

```
┌─────────┐     ┌──────────────┐     ┌───────────────┐     ┌─────────┐
│  IMAP   │ ──▶ │  SQLite/PG   │ ──▶ │  Bluge Index  │ ──▶ │   UI    │
│ Server  │     │  (metadata)  │     │  (full-text)  │     │ (React) │
└─────────┘     └──────────────┘     └───────────────┘     └─────────┘
```

**Pipeline:**

1. IMAP synchronization
    
2. Metadata normalization
    
3. Local indexing with Bluge
    
4. Real-time UI rendering
    

**Result:** sub-100ms search, instant filtering, scalable inboxes, fast bulk operations.

## 📧 Gmail-Style Email Processing

Incoming emails are normalized before rendering to ensure privacy and safety.

Plaintext

```
Raw MIME ──▶ enmime parser ──▶ HTML sanitization ──▶ CSS normalization ──▶ Safe rendering
```

**Features:**

- MIME normalization
    
- HTML sanitization
    
- quote folding
    
- inline attachment support
    
- tracking protection
    
- XSS protection
    
- privacy-first rendering
    

## 🌍 Internationalization (45 Languages)

RMS Mail supports 45 languages out of the box. Includes LTR/RTL support, live language switching, localized dates, and full UI localization.

**Supported regions:** Europe, Middle East, East Asia, South Asia, Central Asia, Caucasus, Southeast Asia.

|**Code**|**Language**|**Code**|**Language**|**Code**|**Language**|
|---|---|---|---|---|---|
|`en`|🇬🇧 English|`ru`|🇷🇺 Русский|`zh`|🇨🇳 中文|
|`ja`|🇯🇵 日本語|`ko`|🇰🇷 한국어|`ar`|🇸🇦 العربية|
|`he`|🇮🇱 עברית|`hi`|🇮🇳 हिन्दी|`bn`|🇧🇩 Bengali|
|`ur`|🇵🇰 اردو|`fa`|🇮🇷 فارسی|`tr`|🇹🇷 Türkçe|
|`de`|🇩🇪 Deutsch|`fr`|🇫🇷 Français|`es`|🇪🇸 Español|
|`it`|🇮🇹 Italiano|`nl`|🇳🇱 Nederlands|`pl`|🇵🇱 Polski|
|`cs`|🇨🇿 Čeština|`sk`|🇸🇰 Slovenčina|`hu`|🇭🇺 Magyar|
|`ro`|🇷🇴 Română|`bg`|🇧🇬 Български|`el`|🇬🇷 Ελληνικά|
|`sr`|🇷🇸 Српски|`hr`|🇭🇷 Hrvatski|`sl`|🇸🇮 Slovenščina|
|`sv`|🇸🇪 Svenska|`da`|🇩🇰 Dansk|`nb`|🇳🇴 Norsk|
|`fi`|🇫🇮 Suomi|`et`|🇪🇪 Eesti|`lv`|🇱🇻 Latviešu|
|`lt`|🇱🇹 Lietuvių|`uk`|🇺🇦 Українська|`kk`|🇰🇿 Қазақша|
|`ka`|🇬🇪 ქართული|`hy`|🇦🇲 Հայերեն|`az`|🇦🇿 Azərbaycanca|
|`uz`|🇺🇿 Oʻzbekcha|`vi`|🇻🇳 Tiếng Việt|`th`|🇹🇭 ไทย|
|`id`|🇮🇩 Indonesia|`ms`|🇲🇾 Melayu|`ca`|🇪🇸 Català|

## 🚀 Quick Start

### Mono

Bash

```
git clone [https://github.com/your-org/rms-mail.git](https://github.com/your-org/rms-mail.git)
cd rms-mail

cp .env-m.example .env

docker compose -f docker-compose-m.yml up -d
```

Open: `http://localhost:8080`

### Unified

Bash

```
cp .env-u.example .env

# Configure:
# DATABASE_URL
# REDIS_URL
# ENCRYPTION_KEYS

docker compose -f docker-compose-u.yml up -d
```

Open: `http://localhost:8080`

## 📊 Feature Matrix

|**Feature**|**Mono**|**Unified**|**Teams**|
|---|---|---|---|
|IMAP Sync + IDLE|✅|✅|✅|
|SMTP Send|✅|✅|✅|
|AI Gateway (10 providers)|✅|✅|✅|
|AI Chat + Tool-calling|✅|✅|✅|
|Telegram Bot|✅|✅|✅|
|MCP Server|✅|✅|✅|
|Vector Search (Bluge)|✅|✅|✅|
|Unlimited Bulk Operations|✅|✅|✅|
|Labels, Rules, Groups|✅|✅|✅|
|Rich HTML Composer|✅|✅|✅|
|Keyboard Shortcuts|✅|✅|✅|
|45 Languages (i18n)|✅|✅|✅|
|Thread Chains (Toggleable)|✅|✅|✅|
|Configurable Send Delay|✅|✅|✅|
|Browser & TG Notifications|✅|✅|✅|
|IDE / Agent Integration|✅|✅|✅|
|Pin / Snooze / Mute|✅|✅|✅|
|Multi-Account Unified Inbox|—|✅|✅|
|Project Groups|—|✅|✅|
|PostgreSQL + Redis|—|✅|✅|
|OAuth 2.0 (Google, MS)|—|✅|✅|
|Shared Mailboxes|—|—|🚧|
|Assignments|—|—|🚧|
|Internal Comments|—|—|🚧|
|SLA Tracking|—|—|🚧|
|Role-based Access|—|—|🚧|

## 💭 Philosophy

RMS Mail is built around several core ideas:

- email should be fast
    
- email should scale
    
- email should be programmable
    
- email should integrate with AI naturally
    
- users should control their infrastructure
    
- self-hosted software should not feel outdated
    

This project is heavily shaped by support workflows, operational reality, multi-account overload, browser-first workflows, AI-assisted productivity, and real infrastructure constraints.

## 🗺️ Roadmap

Current priorities:

- Unified public release
    
- Teams edition
    
- onboarding simplification
    
- deeper IDE integrations
    
- more automation workflows
    
- expanded AI orchestration
    

## ⚖️ License

AGPLv3
