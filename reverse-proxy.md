# Reverse Proxy — RMS Mail

After [Quick Start](README.md#-quick-start) (`docker compose up -d`), the UI listens on **port 3000**.
For production, put **HTTPS** in front of that port (aaPanel, 1Panel, nginx, Caddy, etc.).

Proper proxy settings matter for **SSE** (`/api/events`, `/mcp/sse`) and long-lived MCP
connections. **IMAP IDLE** is outbound from the app to your mail server — it does **not** go through
your site proxy.

---

## Editions

| Edition | After install | Public port | API on host |
|---------|---------------|-------------|-------------|
| **Mono** | `cp docker-compose-m.yml docker-compose.yml`, `.env` from `.env-m.example` | **3000** (UI) | **No** — API/MCP only inside Docker |
| **Unified** | `cp docker-compose-u.yml docker-compose.yml`, `.env` from `.env-u.example` | **3000** (UI) | **8080** optional (backend also published) |

Both editions serve the UI at `http://localhost:3000` right after install. In production, terminate
TLS on your domain and proxy to `127.0.0.1:3000` (recommended) or use split routing (Unified only).

In `.env` (step 3 of Quick Start), set your public URL:

```env
FRONTEND_URL=https://yourdomain.com
ALLOWED_ORIGINS=https://yourdomain.com
# optional explicit public origin for MCP:
# MCP_API_URL=https://yourdomain.com
```

Restart after changes: `docker compose up -d`.

---

## Recommended: single public port

Works for **Mono and Unified**. One upstream — the UI container on **`:3000`**. The frontend proxies
`/api/*` and `/mcp/*` to the backend inside Docker; your edge proxy only needs to reach the UI.

```nginx
upstream rms_ui {
    server 127.0.0.1:3000;   # UI_PORT in .env, default 3000
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://rms_ui;
        proxy_http_version 1.1;

        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-Host  $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP         $remote_addr;

        # SSE (/api/events) and MCP SSE pass through the UI → backend
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header X-Accel-Buffering no;
        proxy_read_timeout 86400s;
    }
}

server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$host$request_uri;
}
```

**Public URLs (both editions, single port):**

| Consumer | URL |
|----------|-----|
| Browser / UI | `https://yourdomain.com` |
| Telegram webhook | `https://yourdomain.com/api/tg/webhook` |
| MCP SSE | `https://yourdomain.com/mcp/sse` |
| MCP messages | `https://yourdomain.com/mcp/messages` |
| Health (optional) | `https://yourdomain.com/api/health` |

Do **not** hardcode `X-Forwarded-Proto https` — always use `$scheme`.

> **Mono:** do not point the site proxy at `:8080` only — the backend is not meant to be the sole
> public entry. Use **`:3000`**.

---

## Optional: split routing (Unified)

Only if you intentionally expose the backend on the host (`PORT=8080` in `.env`). Nginx sends API and
MCP to the backend, everything else to the UI.

```nginx
upstream rms_api {
    server 127.0.0.1:8080;
}

upstream rms_ui {
    server 127.0.0.1:3000;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    proxy_set_header Host              $host;
    proxy_set_header X-Forwarded-Host  $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Real-IP         $remote_addr;

    location /api/ {
        proxy_pass http://rms_api;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header X-Accel-Buffering no;
        proxy_read_timeout 86400s;
    }

    location /mcp/ {
        proxy_pass http://rms_api;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header X-Accel-Buffering no;
        proxy_read_timeout 86400s;
    }

    location / {
        proxy_pass http://rms_ui;
        proxy_http_version 1.1;
    }
}
```

Do **not** expose `/metrics` to the internet.

---

## SSE requirements

The app uses **Server-Sent Events** on `/api/events` and `/mcp/sse` (not WebSockets).

```nginx
proxy_buffering off;
proxy_cache off;
proxy_set_header X-Accel-Buffering no;
proxy_read_timeout 86400s;    # 3600s minimum
```

---

## Control panel checklist (aaPanel / 1Panel)

1. Finish Quick Start for your edition (Mono or Unified).
2. Enable SSL; reverse proxy target = **127.0.0.1:3000** (recommended).
3. In the panel’s custom nginx snippet, set `X-Forwarded-Proto $scheme` and `X-Forwarded-Host $host`.
4. Set `FRONTEND_URL` and `ALLOWED_ORIGINS` in `.env` to `https://yourdomain.com`.
5. `docker compose up -d`
6. Verify: `curl -s https://yourdomain.com/api/health` → `{"status":"ok"}`

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| **Disconnected** in inbox | SSE buffered | `proxy_buffering off` on the path to `:3000` (or `/api/` in split mode) |
| **MCP 404** | `/mcp/*` not routed | Use single-port `:3000`, or add `location /mcp/` in split mode |
| **Mixed content** (`http://` MCP links) | Missing HTTPS hints | `FRONTEND_URL=https://…` in `.env`; `X-Forwarded-Proto $scheme` |
| **404 on /api/** | Wrong upstream | **Mono:** proxy `:3000`, not `:8080` alone |
| **502 Bad Gateway** | Container down / wrong port | `docker ps`; check `UI_PORT` / `PORT` in `.env` |
| **Slow SSE** | Short `proxy_read_timeout` | Raise to 3600s–86400s |
| **Telegram webhook fails** | Wrong public URL | `https://yourdomain.com/api/tg/webhook` |
