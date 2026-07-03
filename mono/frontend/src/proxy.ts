import { NextRequest } from "next/server";
import createMiddleware from "next-intl/middleware";

const LOCALE_LIST = [
  "en",
  "ru",
  "ka",
  "zh",
  "es",
  "hi",
  "ar",
  "fr",
  "pt",
  "ja",
  "ko",
  "de",
  "it",
  "nl",
  "sv",
  "da",
  "nb",
  "fi",
  "lt",
  "lv",
  "et",
  "pl",
  "cs",
  "hu",
  "ro",
  "bg",
  "el",
  "sr",
  "hr",
  "uk",
  "kk",
  "hy",
  "az",
  "uz",
  "tr",
  "id",
  "vi",
  "th",
  "he",
  "ur",
  "bn",
  "ca",
  "ms",
  "sl",
  "sk",
];

const middleware = createMiddleware({
  // A list of all locales that are supported
  locales: LOCALE_LIST,

  // Used when no locale matches
  defaultLocale: "en",
});

// Next.js 16 Proxy Convention: Named export 'proxy'
export function proxy(req: NextRequest) {
  return middleware(req);
}

export const config = {
  // Match only internationalized pathnames
  matcher: ["/", "/([a-z]{2})/:path*"],
};
