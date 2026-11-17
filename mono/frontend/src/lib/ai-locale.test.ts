import { describe, expect, it } from "vitest";
import { getTranslationTargetLanguage } from "./ai-locale";

describe("getTranslationTargetLanguage", () => {
  it("maps known locales", () => {
    expect(getTranslationTargetLanguage("ru")).toBe("Russian");
    expect(getTranslationTargetLanguage("ka")).toBe("Georgian");
    expect(getTranslationTargetLanguage("fa")).toBe("Persian");
    expect(getTranslationTargetLanguage("pt")).toBe("Portuguese");
    expect(getTranslationTargetLanguage("en")).toBe("English");
    expect(getTranslationTargetLanguage("unknown")).toBe("English");
  });
});
