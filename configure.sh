#!/bin/bash
set -e

echo "=== RMS Mail Configuration Script ==="

if ! command -v openssl &> /dev/null; then
    echo "[-] Error: openssl is required for secret generation." >&2
    exit 1
fi

# Clean up any leftover temp file if the script exits unexpectedly mid-sed
trap 'rm -f .env.tmp' EXIT

# CLI arguments (for CI/CD / non-interactive automation)
edition="$1"
domain_url="$2"
admin_email="$3"

if [ -z "$edition" ]; then
    echo "Select edition to configure:"
    echo "1) Mono (Lightweight / SQLite)"
    echo "2) Unified (Multi-account / Postgres + Redis)"
    echo "3) Mono Pro (Professional / Postgres + Redis)"
    read -rp "Enter choice [1-3]: " edition
fi

gen_secret() {
    openssl rand -hex "$1"
}

# Escapes &, |, and \ so a value can be safely used as a sed replacement
# with a | delimiter. Prevents broken substitutions or unintended
# behavior if a user-supplied value (email/domain) contains these chars.
sed_escape() {
    printf '%s' "$1" | sed -e 's/[\&|]/\\&/g'
}

safe_replace() {
    local pattern="$1"
    local file=".env"
    sed "$pattern" "$file" > "$file.tmp" && mv "$file.tmp" "$file"
}

# Basic sanity check for email input (not full RFC 5322 validation,
# just enough to catch empty/garbage input before it hits sed/.env)
validate_email() {
    local email="$1"
    if [[ ! "$email" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
        return 1
    fi
    return 0
}

case $edition in
    1)
        echo -e "\nConfiguring Mono edition..."
        cp .env-m.example .env
        cp docker-compose-m.yml docker-compose.yml

        KEY=$(gen_secret 32)
        JWT=$(gen_secret 32)

        safe_replace "s|^ENCRYPTION_KEY=['\"]\?.*|ENCRYPTION_KEY=$KEY|"
        safe_replace "s|^JWT_SECRET=['\"]\?.*|JWT_SECRET=$JWT|"
        ;;
    2|3)
        if [ "$edition" -eq 2 ]; then
            echo -e "\nConfiguring Unified edition..."
            cp .env-u.example .env
            cp docker-compose-u.yml docker-compose.yml
        else
            echo -e "\nConfiguring Mono Pro edition..."
            cp .env-mp.example .env
            cp docker-compose-mp.yml docker-compose.yml

            if [ -n "$admin_email" ] && ! validate_email "$admin_email"; then
                echo "[-] Warning: ADMIN_EMAIL provided via argument looks invalid. Ignoring it." >&2
                admin_email=""
            fi

            if [ -z "$admin_email" ]; then
                read -rp "Enter ADMIN_EMAIL (required for initial login): " admin_email
                while ! validate_email "$admin_email"; do
                    read -rp "That doesn't look like a valid email. Enter admin email: " admin_email
                done
            fi

            safe_admin_email=$(sed_escape "$admin_email")
            safe_replace "s|^ADMIN_EMAIL=['\"]\?.*|ADMIN_EMAIL=$safe_admin_email|"
        fi

        PG_PASS=$(gen_secret 16)
        RD_PASS=$(gen_secret 16)
        ENC_KEY=$(gen_secret 32)
        JWT_SEC=$(gen_secret 32)
        CAMO_KEY=$(gen_secret 32)

        safe_replace "s|^POSTGRES_PASSWORD=['\"]\?.*|POSTGRES_PASSWORD=$PG_PASS|"
        safe_replace "s|^REDIS_PASSWORD=['\"]\?.*|REDIS_PASSWORD=$RD_PASS|"
        safe_replace "s|^ENCRYPTION_KEY=['\"]\?.*|ENCRYPTION_KEY=$ENC_KEY|"
        safe_replace "s|^JWT_SECRET=['\"]\?.*|JWT_SECRET=$JWT_SEC|"
        safe_replace "s|^CAMO_HMAC_KEY=['\"]\?.*|CAMO_HMAC_KEY=$CAMO_KEY|"
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac

if [ -z "$domain_url" ]; then
    echo -e "\n--- Production Domain Setup ---"
    read -rp "Enter your domain (e.g., https://yourdomain.com) [leave blank for localhost]: " domain_url
fi

if [ -n "$domain_url" ]; then
    domain_url="${domain_url%/}"
    safe_domain_url=$(sed_escape "$domain_url")
    safe_replace "s|^FRONTEND_URL=['\"]\?.*|FRONTEND_URL=$safe_domain_url|"
    safe_replace "s|^ALLOWED_ORIGINS=['\"]\?.*|ALLOWED_ORIGINS=$safe_domain_url|"
fi

# .env now contains DB/JWT/encryption secrets — lock it down
chmod 600 .env

echo -e "\n[✓] Environment configured successfully!"
echo "Now you can run: docker compose up -d"
