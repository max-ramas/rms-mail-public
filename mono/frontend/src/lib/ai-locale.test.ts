import { describe, expect, it } from "vitest";
import { getTranslationTargetLanguage } from "./ai-locale";

describe("getTranslationTargetLanguage", () => {
  it("maps known locales", () => {
    expect(getTranslationTargetLanguage("ru")).toBe("Russian");
    expect(getTranslationTargetLanguage("ka")).toBe("Georgian");
    expect(getTranslationTargetLanguage("en")).toBe("English");
  });
});
