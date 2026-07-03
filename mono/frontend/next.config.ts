import createNextIntlPlugin from "next-intl/plugin";
import withPWAInit from "@ducanh2912/next-pwa";
import { withSentryConfig } from "@sentry/nextjs";

import { readFileSync } from "fs";
import { join } from "path";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

const withPWA = withPWAInit({
  dest: "public",
  disable: process.env.NODE_ENV === "development",
  workboxOptions: {
    exclude: [/\/api\/events/],
  },
});

let appVersion = "3.1.2";
try {
  const bpUSh = readFileSync(join(process.cwd(), "..", "bp-u.sh"), "utf8");
  const match = bpUSh.match(/VERSION="\$\{1:-(.+?)\}"/);
  if (match && match[1]) {
    appVersion = match[1];
  }
} catch {
  // ignore
}

/** @type {import('next').NextConfig} */
const nextConfig = {
  env: {
    NEXT_PUBLIC_APP_VERSION: process.env.NEXT_PUBLIC_APP_VERSION || appVersion,
  },
  output: "standalone" as const,
  turbopack: {
    root: __dirname,
  },
  outputFileTracingIncludes: {
    "/[locale]/**/*": ["./src/locales/**/*.json", "./src/i18n/**/*.ts"],
  },
  async rewrites() {
    // Proxied to Go backend inside Docker / dev. Edge nginx (aaPanel) must pass
    // X-Forwarded-Proto and X-Forwarded-Host to this Next.js process so the backend
    // can build https:// public URLs (MCP, OAuth). Set FRONTEND_URL=https://… on backend too.
    const apiUrl = process.env.API_URL || "http://localhost:8087";
    const internalUrl = process.env.INTERNAL_URL || "http://localhost:8080";
    return [
      {
        source: "/api/:path*",
        destination: `${apiUrl}/api/:path*`,
      },
      {
        source: "/mcp/:path*",
        destination: `${apiUrl}/mcp/:path*`,
      },
      {
        source: "/internal/:path*",
        destination: `${internalUrl}/internal/:path*`,
      },
    ];
  },
};

const baseConfig = withPWA(withNextIntl(nextConfig));

// Conditionally wrap with Sentry — only when DSN is configured.
// Source map upload requires SENTRY_AUTH_TOKEN at build time.
const sentryDsn = process.env.NEXT_PUBLIC_SENTRY_DSN || process.env.SENTRY_DSN;
export default sentryDsn
  ? withSentryConfig(baseConfig, {
      silent: true,
      org: process.env.SENTRY_ORG,
      project: process.env.SENTRY_PROJECT,
      authToken: process.env.SENTRY_AUTH_TOKEN,
      sourcemaps: {
        disable: process.env.NODE_ENV !== "production",
      },
    })
  : baseConfig;
