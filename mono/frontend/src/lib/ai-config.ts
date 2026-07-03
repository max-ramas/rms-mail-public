export type AITask = "summarize" | "categorize" | "autodraft" | "chat";

export function getAIConfig(task: AITask) {
  if (typeof window === "undefined") return undefined;
  const saved = localStorage.getItem("rms-mail_ai_config");
  if (!saved) return undefined;
  try {
    const s = JSON.parse(saved);
    const taskConfig = s.config?.[task] || {
      provider: "openrouter",
      model: "llama-3.1-70b",
    };
    const prompt = s.prompts?.[task] || "";
    return {
      provider: taskConfig.provider,
      model: taskConfig.model,
      prompt,
    };
  } catch {
    return undefined;
  }
}
