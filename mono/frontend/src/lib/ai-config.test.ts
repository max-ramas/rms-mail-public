import { describe, expect, it } from "vitest";
import { getAIConfig } from "./ai-config";

describe("getAIConfig", () => {
  it("returns undefined when no config is saved", () => {
    localStorage.removeItem("rms-mail_ai_config");
    expect(getAIConfig("summarize")).toBeUndefined();
  });

  it("reads provider, model, and prompt for a task", () => {
    localStorage.setItem(
      "rms-mail_ai_config",
      JSON.stringify({
        config: {
          summarize: { provider: "openai", model: "gpt-4o" },
        },
        prompts: { summarize: "Summarize briefly" },
      }),
    );

    expect(getAIConfig("summarize")).toEqual({
      provider: "openai",
      model: "gpt-4o",
      prompt: "Summarize briefly",
    });
  });

  it("falls back to default provider and model", () => {
    localStorage.setItem("rms-mail_ai_config", JSON.stringify({}));
    expect(getAIConfig("chat")).toEqual({
      provider: "openrouter",
      model: "llama-3.1-70b",
      prompt: "",
    });
  });
});
