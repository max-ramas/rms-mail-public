"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

const LOCALES = [
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
  "fa",
  "bn",
  "ca",
  "ms",
  "sl",
  "sk",
];

function getCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

function detectLocale(): string {
  // 1. Cookie (explicit user choice)
  const cookie = getCookie("preferred_locale");
  if (cookie && LOCALES.includes(cookie)) return cookie;

  // 2. Browser language
  if (typeof navigator !== "undefined") {
    const lang = (navigator.language || "").toLowerCase();
    if (lang.startsWith("ge")) return "ka";
    if (lang.startsWith("no") || lang.startsWith("nn")) return "nb";
    const prefix = lang.split("-")[0];
    if (prefix && LOCALES.includes(prefix)) return prefix;
  }

  return "en";
}

export default function LoginRedirect() {
  const router = useRouter();

  useEffect(() => {
    const locale = detectLocale();
    router.replace(`/${locale}/login`);
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <p className="text-muted-foreground text-sm">Redirecting...</p>
    </div>
  );
}
