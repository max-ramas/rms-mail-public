FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

ARG CACHEBUST=0
COPY . .

ARG EDITION
ENV EDITION=${EDITION}

ARG APP_VERSION

ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Cross-compile for the target architecture
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags ${EDITION} -ldflags="-s -w -X main.AppVersion=${APP_VERSION} -X rmsmail/internal/license.PublicKeyHex=${PUBLIC_KEY}" -o main ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S appgroup && adduser -S -u 1000 appuser -G appgroup \
    && mkdir -p /app/storage/emails /app/storage/attachments /app/storage/camo /app/data \
    && chown -R appuser:appgroup /app \
    && rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app

ARG EDITION
ENV EDITION=${EDITION}
ARG UPDATE_CHANNEL=stable
ENV UPDATE_CHANNEL=${UPDATE_CHANNEL}
ENV PORT=8080
ENV STORAGE_ROOT=/app/storage

COPY --from=builder /app/main .
COPY --from=builder /app/schema_mono.sql ./schema_mono.sql

USER appuser
EXPOSE 8080
CMD ["./main"]
