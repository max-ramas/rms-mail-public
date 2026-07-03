import { getRequestConfig } from "next-intl/server";
import { loadMessages } from "@/lib/load-messages";

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
  "bn",
  "ca",
  "ms",
  "sl",
  "sk",
];

export default getRequestConfig(async ({ requestLocale }) => {
  let locale = await requestLocale;

  if (!locale || !LOCALES.includes(locale)) {
    locale = "en";
  }

  const messages = await loadMessages(locale);

  return {
    locale,
    messages,
  };
});
