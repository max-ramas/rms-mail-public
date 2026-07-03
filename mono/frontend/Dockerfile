FROM node:26-alpine AS builder

WORKDIR /app

# Disable Next.js telemetry and progress bar animations
ENV NEXT_TELEMETRY_DISABLED=1
ENV CI=true
# Transient registry drops (ECONNRESET) during docker build — retry longer.
ENV NPM_CONFIG_FETCH_RETRIES=5 \
    NPM_CONFIG_FETCH_RETRY_MINTIMEOUT=20000 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=120000 \
    NPM_CONFIG_FETCH_TIMEOUT=300000

COPY package*.json ./

# Lockfile install + BuildKit npm cache; outer shell retries whole ci on network blips.
RUN --mount=type=cache,target=/root/.npm \
    sh -ec 'for n in 1 2 3 4 5; do npm ci --no-audit --no-fund && exit 0; echo "npm ci attempt $n failed, retrying in ${n}0s..."; sleep $((n * 10)); done; exit 1'

ARG CACHEBUST=0
COPY . .

# Sentry: source map upload at build time (requires SENTRY_AUTH_TOKEN).
# Optional — build succeeds without it (source maps just won't be uploaded).
ARG SENTRY_ORG
ARG SENTRY_PROJECT
ARG SENTRY_AUTH_TOKEN
ARG SENTRY_DSN
ENV SENTRY_ORG=${SENTRY_ORG}
ENV SENTRY_PROJECT=${SENTRY_PROJECT}
ENV SENTRY_AUTH_TOKEN=${SENTRY_AUTH_TOKEN}
ENV NEXT_PUBLIC_SENTRY_DSN=${SENTRY_DSN}
ENV NEXT_PUBLIC_SENTRY_ENVIRONMENT=${SENTRY_ENVIRONMENT:-production}

ARG EDITION
ENV NEXT_PUBLIC_EDITION=${EDITION}

ARG NEXT_PUBLIC_APP_VERSION
ENV NEXT_PUBLIC_APP_VERSION=${NEXT_PUBLIC_APP_VERSION}
RUN sed -i "s/\"version\": \".*\"/\"version\": \"${NEXT_PUBLIC_APP_VERSION}\"/" package.json

ARG API_URL=http://backend:8080
ENV API_URL=${API_URL}
ARG INTERNAL_URL=http://backend:8080
ENV INTERNAL_URL=${INTERNAL_URL}

RUN echo "NEXT_PUBLIC_EDITION=${EDITION}" > .env.production

ENV NODE_OPTIONS="--max-old-space-size=4096"

RUN rm -rf .next node_modules/.cache && npm run build && rm -rf .next/cache

FROM node:26-alpine

WORKDIR /app

# Keep only standalone runtime
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
COPY --from=builder /app/src/i18n ./src/i18n
COPY --from=builder /app/src/locales ./src/locales

RUN chown -R node:node ./src

ENV HOSTNAME="0.0.0.0"
EXPOSE 3000

USER node

CMD ["node", "server.js"]
