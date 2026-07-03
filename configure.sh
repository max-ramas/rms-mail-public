#!/bin/bash
set -e

echo "=== RMS Mail Configuration Script ==="

if ! command -v openssl &> /dev/null; then
    echo "[-] Error: openssl is required for secret generation." >&2
    exit 1
fi

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

safe_replace() {
    local pattern="$1"
    local file=".env"
    sed "$pattern" "$file" > "$file.tmp" && mv "$file.tmp" "$file"
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

            if [ -z "$admin_email" ]; then
                read -rp "Enter ADMIN_EMAIL (required for initial login): " admin_email
                while [ -z "$admin_email" ]; do
                    read -rp "ADMIN_EMAIL cannot be empty. Enter admin email: " admin_email
                done
            fi
            safe_replace "s|^ADMIN_EMAIL=['\"]\?.*|ADMIN_EMAIL=$admin_email|"
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
    safe_replace "s|^FRONTEND_URL=['\"]\?.*|FRONTEND_URL=$domain_url|"
    safe_replace "s|^ALLOWED_ORIGINS=['\"]\?.*|ALLOWED_ORIGINS=$domain_url|"
fi

echo -e "\n[✓] Environment configured successfully!"
echo "Now you can run: docker compose up -d"
