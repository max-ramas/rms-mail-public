export function getTranslationTargetLanguage(locale: string): string {
  if (locale === "ru") return "Russian";
  if (locale === "ka") return "Georgian";
  return "English";
}
