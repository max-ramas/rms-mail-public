"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

const LOCALES = ["en", "ru", "ka"];

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
    const lang = navigator.language || "";
    if (lang.startsWith("ru")) return "ru";
    if (lang.startsWith("ka") || lang.startsWith("ge")) return "ka";
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
