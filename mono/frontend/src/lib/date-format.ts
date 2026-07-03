import { format } from "date-fns";
import {
  ru,
  enUS,
  ka,
  zhCN,
  es,
  hi,
  ar,
  fr,
  pt,
  ja,
  ko,
  de,
  it,
  nl,
  sv,
  da,
  nb,
  fi,
  lt,
  lv,
  et,
  pl,
  cs,
  hu,
  ro,
  bg,
  el,
  sr,
  hr,
  uk,
  kk,
  hy,
  az,
  uz,
  tr,
  id,
  vi,
  th,
  he,
  bn,
  ca,
  ms,
  sl,
  sk,
} from "date-fns/locale";
import type { Locale } from "date-fns";

const locales: Record<string, Locale> = {
  ru,
  en: enUS,
  ka,
  zh: zhCN,
  es,
  hi,
  ar,
  fr,
  pt,
  ja,
  ko,
  de,
  it,
  nl,
  sv,
  da,
  nb,
  fi,
  lt,
  lv,
  et,
  pl,
  cs,
  hu,
  ro,
  bg,
  el,
  sr,
  hr,
  uk,
  kk,
  hy,
  az,
  uz,
  tr,
  id,
  vi,
  th,
  he,
  bn,
  ca,
  ms,
  sl,
  sk,
};

export type DateFormat = "auto" | "eu" | "us" | "iso" | "uk";

const FORMATS: Record<
  DateFormat,
  { date: string; time: string; datetime: string }
> = {
  auto: { date: "PP", time: "p", datetime: "PPp" },
  eu: { date: "dd.MM.yyyy", time: "HH:mm", datetime: "dd.MM.yyyy HH:mm" },
  us: { date: "MM/dd/yyyy", time: "hh:mm a", datetime: "MM/dd/yyyy hh:mm a" },
  iso: { date: "yyyy-MM-dd", time: "HH:mm", datetime: "yyyy-MM-dd HH:mm" },
  uk: { date: "dd/MM/yyyy", time: "HH:mm", datetime: "dd/MM/yyyy HH:mm" },
};

export function getSavedDateFormat(): DateFormat {
  if (typeof window === "undefined") return "auto";
  return (localStorage.getItem("rms-mail_date_format") as DateFormat) || "auto";
}

export function getSavedUndoDelay(): number {
  if (typeof window === "undefined") return 10000;
  const saved = localStorage.getItem("rms-mail_undo_delay_ms");
  if (saved === null) return 10000;
  const val = parseInt(saved, 10);
  return val === 0 ? 0 : val; // 0 = disabled
}

export function formatEmailDate(
  dateStr: string,
  localeCode: string,
  fmt?: DateFormat,
): string {
  try {
    const date = new Date(dateStr);
    const localeObj = locales[localeCode] || enUS;
    const dateFmt = fmt || getSavedDateFormat();

    if (dateFmt === "auto") {
      // Legacy behavior: smart formatting with relative time
      const now = new Date();
      const isToday = date.toDateString() === now.toDateString();
      const yesterday = new Date(now);
      yesterday.setDate(yesterday.getDate() - 1);
      const isYesterday = date.toDateString() === yesterday.toDateString();
      if (isToday) return format(date, "HH:mm", { locale: localeObj });
      if (isYesterday) return "Yesterday";
      if (date.getFullYear() === now.getFullYear())
        return format(date, "d MMM", { locale: localeObj });
      return format(date, "d MMM yyyy", { locale: localeObj });
    }

    const f = FORMATS[dateFmt];
    const now = new Date();
    const isToday = date.toDateString() === now.toDateString();
    if (isToday) return format(date, f.time, { locale: localeObj });
    if (date.getFullYear() === now.getFullYear())
      return format(date, f.date, { locale: localeObj });
    return format(date, f.date, { locale: localeObj });
  } catch {
    return dateStr;
  }
}

export function formatEmailDatetime(
  dateStr: string,
  localeCode: string,
  fmt?: DateFormat,
): string {
  try {
    const date = new Date(dateStr);
    const localeObj = locales[localeCode] || enUS;
    const dateFmt = fmt || getSavedDateFormat();

    if (dateFmt === "auto") {
      return format(date, "PPp", { locale: localeObj });
    }
    return format(date, FORMATS[dateFmt].datetime, { locale: localeObj });
  } catch {
    return dateStr;
  }
}
