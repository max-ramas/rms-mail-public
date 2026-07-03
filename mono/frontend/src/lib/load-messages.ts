import { readFile } from "fs/promises";
import { join } from "path";

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

const NAMESPACES = ["common", "mail", "settings", "auth", "commands"];

function deepMerge(
  target: Record<string, unknown>,
  source: unknown,
): Record<string, unknown> {
  if (typeof source !== "object" || source === null) return target;

  const result: Record<string, unknown> = { ...target };
  for (const key in source as Record<string, unknown>) {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      const srcVal = (source as Record<string, unknown>)[key];
      if (
        typeof srcVal === "object" &&
        srcVal !== null &&
        !Array.isArray(srcVal)
      ) {
        const targetVal = result[key];
        result[key] = deepMerge(
          typeof targetVal === "object" && targetVal !== null
            ? (targetVal as Record<string, unknown>)
            : {},
          srcVal,
        );
      } else {
        result[key] = srcVal;
      }
    }
  }
  return result;
}

async function readNs(locale: string, ns: string): Promise<unknown> {
  const filePath = join(process.cwd(), "src", "locales", locale, `${ns}.json`);
  try {
    const raw = await readFile(filePath, "utf-8");
    return JSON.parse(raw);
  } catch (err) {
    // Логируем только реальные ошибки чтения, но не считаем их фатальными
    // (файл может просто отсутствовать для данной локали)
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") {
      console.error(
        `[i18n] Failed to read ${filePath}:`,
        (err as Error).message,
      );
    }
    return {};
  }
}

export async function loadMessages(
  locale: string,
): Promise<Record<string, unknown>> {
  const activeLocale = LOCALES.includes(locale) ? locale : "en";

  // Сначала загружаем базовую локаль en как fallback
  const baseMessages = await Promise.all(
    NAMESPACES.map((ns) => readNs("en", ns)),
  );

  // Затем загружаем целевую локаль (если это не en)
  let targetMessages: unknown[] = [];
  if (activeLocale !== "en") {
    targetMessages = await Promise.all(
      NAMESPACES.map((ns) => readNs(activeLocale, ns)),
    );
  }

  let messages: Record<string, unknown> = {};
  for (let i = 0; i < NAMESPACES.length; i++) {
    messages = deepMerge(messages, baseMessages[i]);
    if (targetMessages[i]) {
      messages = deepMerge(messages, targetMessages[i]);
    }
  }

  // Runtime-проверка: если объект пустой — критическая ошибка
  if (Object.keys(messages).length === 0) {
    console.error(
      `[i18n CRITICAL] No messages loaded for locale "${activeLocale}". ` +
        `Check that src/locales/**/*.json files exist and are readable.`,
    );
  }

  return messages;
}
