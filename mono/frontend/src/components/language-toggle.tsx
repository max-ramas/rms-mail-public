"use client";

import React, { useState, useRef, useEffect } from "react";

const FLAGS: Record<string, string> = {
  en: "🇬🇧",
  ru: "🇷🇺",
  ka: "🇬🇪",
  zh: "🇨🇳",
  es: "🇪🇸",
  hi: "🇮🇳",
  ar: "🇸🇦",
  fr: "🇫🇷",
  pt: "🇵🇹",
  ja: "🇯🇵",
  ko: "🇰🇷",
  de: "🇩🇪",
  it: "🇮🇹",
  nl: "🇳🇱",
  sv: "🇸🇪",
  da: "🇩🇰",
  nb: "🇳🇴",
  fi: "🇫🇮",
  lt: "🇱🇹",
  lv: "🇱🇻",
  et: "🇪🇪",
  pl: "🇵🇱",
  cs: "🇨🇿",
  hu: "🇭🇺",
  ro: "🇷🇴",
  bg: "🇧🇬",
  el: "🇬🇷",
  sr: "🇷🇸",
  hr: "🇭🇷",
  uk: "🇺🇦",
  kk: "🇰🇿",
  hy: "🇦🇲",
  az: "🇦🇿",
  uz: "🇺🇿",
  tr: "🇹🇷",
  id: "🇮🇩",
  vi: "🇻🇳",
  th: "🇹🇭",
  he: "🇮🇱",
  ur: "🇵🇰",
  bn: "🇧🇩",
  ca: "🇪🇸",
  ms: "🇲🇾",
  sl: "🇸🇮",
  sk: "🇸🇰",
};

const LABELS: Record<string, string> = {
  en: "English",
  ru: "Русский",
  ka: "ქართული",
  zh: "中文",
  es: "Español",
  hi: "हिन्दी",
  ar: "العربية",
  fr: "Français",
  pt: "Português",
  ja: "日本語",
  ko: "한국어",
  de: "Deutsch",
  it: "Italiano",
  nl: "Nederlands",
  sv: "Svenska",
  da: "Dansk",
  nb: "Norsk",
  fi: "Suomi",
  lt: "Lietuvių",
  lv: "Latviešu",
  et: "Eesti",
  pl: "Polski",
  cs: "Čeština",
  hu: "Magyar",
  ro: "Română",
  bg: "Български",
  el: "Ελληνικά",
  sr: "Српски",
  hr: "Hrvatski",
  uk: "Українська",
  kk: "Қазақша",
  hy: "Հայերեն",
  az: "Azərbaycan",
  uz: "Oʻzbekcha",
  tr: "Türkçe",
  id: "Bahasa Indonesia",
  vi: "Tiếng Việt",
  th: "ไทย",
  he: "עברית",
  ur: "اردو",
  bn: "বাংলা",
  ca: "Català",
  ms: "Bahasa Melayu",
  sl: "Slovenščina",
  sk: "Slovenčina",
};

export function LanguageToggle({ locale }: { locale: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const [pendingLang, setPendingLang] = useState<string | null>(null);

  useEffect(() => {
    if (!pendingLang) return;
    const d = new Date();
    d.setFullYear(d.getFullYear() + 1);
    document.cookie = `preferred_locale=${pendingLang}; path=/; expires=${d.toUTCString()}; SameSite=Lax`;
    const path = window.location.pathname.replace(
      /^\/[a-z]{2}/,
      `/${pendingLang}`,
    );
    window.location.assign(path);
  }, [pendingLang]);

  const switchTo = (l: string) => {
    setPendingLang(l);
  };

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="text-muted-foreground hover:text-foreground transition-colors px-2 py-1 rounded hover:bg-muted text-sm"
        title={LABELS[locale]}
      >
        {FLAGS[locale] || "🌐"}
      </button>
      {open && (
        <div className="absolute bottom-full mb-1 left-0 bg-card border rounded-lg shadow-lg py-1 z-50 min-w-[130px] max-h-60 overflow-y-auto">
          {Object.entries(LABELS).map(([code, label]) => (
            <button
              key={code}
              onClick={() => {
                switchTo(code);
                setOpen(false);
              }}
              className={`w-full text-left px-3 py-1.5 text-xs hover:bg-muted flex items-center gap-2 ${locale === code ? "bg-muted/50" : ""}`}
            >
              <span>{FLAGS[code]}</span>
              <span>{label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
