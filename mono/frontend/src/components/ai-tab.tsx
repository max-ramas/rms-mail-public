"use client";

import React, { useState } from "react";
import {
  Save,
  Play,
  Search,
  MessageCircle,
  Sparkles,
  Tags,
  Zap,
  Star,
  Settings,
} from "lucide-react";
import {
  useAIChat,
  useAICategorize,
  useAISettings,
  useSaveAISettings,
} from "@/hooks/useAIApi";
import { getEnvOnlyFlags } from "@/hooks/useEmails";
import { useToast } from "@/hooks/useToast";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ModelSelector } from "@/components/model-selector";

const PROVIDERS = [
  { id: "openrouter", name: "OpenRouter", needKey: true },
  { id: "openai", name: "OpenAI", needKey: true },
  { id: "anthropic", name: "Anthropic", needKey: true },
  { id: "gemini", name: "Gemini", needKey: true },
  { id: "deepseek", name: "DeepSeek", needKey: true },
  { id: "groq", name: "Groq", needKey: true },
  { id: "ollama", name: "Ollama", needKey: false },
  { id: "xai", name: "xAI (Grok)", needKey: true },
  { id: "opencode", name: "OpenCode", needKey: true },
  { id: "qwen", name: "Qwen", needKey: true },
];

const DEFAULTS: Record<string, { provider: string; model: string }> = {
  chat: { provider: "ollama", model: "llama3" },
  summarize: { provider: "ollama", model: "llama3" },
  categorize: { provider: "ollama", model: "llama3" },
  autodraft: { provider: "ollama", model: "llama3" },
};

const DEFAULT_PROMPTS: Record<string, string> = {
  chat: "You are a helpful email assistant.",
  summarize: "Summarize the following email in 2-3 sentences.",
  categorize: "Categorize this email into one of the following tags...",
  autodraft:
    "You are a helpful email assistant. Generate a professional reply to the following email thread. Consider the context of previous messages.",
};

export function AITab() {
  const envOnly = getEnvOnlyFlags().llm;
  const t = useTranslations("settings");
  const toast = useToast();
  const aiChat = useAIChat();
  const aiCat = useAICategorize();
  const aiSettingsQuery = useAISettings();
  const saveAISettings = useSaveAISettings();
  const [config, setConfig] = useState<
    Record<string, { provider: string; model: string }>
  >(() => {
    if (typeof window === "undefined") return DEFAULTS;
    const saved = localStorage.getItem("rms-mail_ai_config");
    if (saved) {
      try {
        const s = JSON.parse(saved);
        return s.config || DEFAULTS;
      } catch {
        return DEFAULTS;
      }
    }
    return DEFAULTS;
  });
  const [prompts, setPrompts] = useState<Record<string, string>>(() => {
    if (typeof window === "undefined") return DEFAULT_PROMPTS;
    const saved = localStorage.getItem("rms-mail_ai_config");
    if (saved) {
      try {
        const s = JSON.parse(saved);
        return { ...DEFAULT_PROMPTS, ...(s.prompts || {}) };
      } catch {
        return DEFAULT_PROMPTS;
      }
    }
    return DEFAULT_PROMPTS;
  });
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({});
  const [hasSavedKey, setHasSavedKey] = useState<Record<string, boolean>>(
    () => {
      // Track which providers have a key on the server (flag only, never the raw key)
      if (typeof window === "undefined") return {};
      try {
        return JSON.parse(localStorage.getItem("rms-mail_has_api_key") || "{}");
      } catch {
        return {};
      }
    },
  );
  const [preset, setPreset] = useState(() => {
    if (typeof window === "undefined") return "custom";
    const saved = localStorage.getItem("rms-mail_ai_config");
    if (saved) {
      try {
        const s = JSON.parse(saved);
        return s.preset || "custom";
      } catch {
        return "custom";
      }
    }
    return "custom";
  });

  const PRESETS = [
    {
      id: "fast",
      label: t("ai_preset_fast"),
      icon: Zap,
      config: { provider: "ollama", model: "llama3" },
    },
    {
      id: "quality",
      label: t("ai_preset_quality"),
      icon: Star,
      config: { provider: "anthropic", model: "claude-3.5-sonnet" },
    },
    { id: "custom", label: t("ai_preset_custom"), icon: Settings },
  ];

  const TASKS = [
    {
      id: "chat",
      label: t("ai_task_chat"),
      icon: MessageCircle,
      desc: t("ai_task_chat_desc"),
    },
    {
      id: "summarize",
      label: t("ai_task_summarize"),
      icon: Sparkles,
      desc: t("ai_task_summarize_desc"),
    },
    {
      id: "categorize",
      label: t("ai_task_categorize"),
      icon: Tags,
      desc: t("ai_task_categorize_desc"),
    },
    {
      id: "autodraft",
      label: t("ai_task_autodraft"),
      icon: MessageCircle,
      desc: t("ai_task_autodraft_desc"),
    },
  ];

  React.useEffect(() => {
    if (!aiSettingsQuery.data) return;
    try {
      const cfg = JSON.parse(aiSettingsQuery.data.config || "{}");
      if (Object.keys(cfg).length > 0) {
        React.startTransition(() => setConfig(cfg));
      }
    } catch {}
    try {
      const pr = JSON.parse(aiSettingsQuery.data.prompts || "{}");
      if (Object.keys(pr).length > 0) {
        React.startTransition(() => setPrompts({ ...DEFAULT_PROMPTS, ...pr }));
      }
    } catch {}
    if (aiSettingsQuery.data.preset) {
      React.startTransition(() => setPreset(aiSettingsQuery.data.preset));
    }
    try {
      const keys = JSON.parse(aiSettingsQuery.data.api_keys || "{}");
      if (Object.keys(keys).length > 0)
        React.startTransition(() => setApiKeys(keys));
    } catch {}
  }, [aiSettingsQuery.data]);

  const maskKey = (key: string) => {
    if (!key || key.length <= 10)
      return key ? key[0] + "\u2022\u2022\u2022" : "";
    return key.slice(0, 5) + "\u2022\u2022\u2022" + key.slice(-5);
  };

  const handlePreset = (p: (typeof PRESETS)[number]) => {
    setPreset(p.id);
    if (p.id !== "custom" && p.config) {
      const newConfig = { ...config };
      TASKS.forEach((t) => {
        newConfig[t.id] = { ...p.config };
      });
      setConfig(newConfig);
    }
  };

  const handleSave = async () => {
    // Persist config to localStorage for offline resilience
    localStorage.setItem(
      "rms-mail_ai_config",
      JSON.stringify({ config, prompts, preset }),
    );
    // Persist keys to server only (encrypted, source of truth)
    try {
      await saveAISettings.mutateAsync({
        account_id: "00000000-0000-0000-0000-000000000000",
        preset,
        config: JSON.stringify(config),
        prompts: JSON.stringify(prompts),
        api_keys: JSON.stringify(apiKeys),
      });
      // Remember which providers have keys on the server (flag only, never the key itself).
      // Only update flags for providers that have actual keys in the current save payload;
      // never clear flags for providers not in apiKeys (e.g. after page reload).
      const saved = { ...hasSavedKey };
      PROVIDERS.forEach((p) => {
        if (apiKeys[p.id]) {
          saved[p.id] = true;
        }
        // If apiKeys[p.id] is empty, keep the existing flag — don't clear it.
      });
      setHasSavedKey(saved);
      localStorage.setItem("rms-mail_has_api_key", JSON.stringify(saved));
      toast.addToast(t("saved"), "success");
    } catch (err) {
      if (process.env.NODE_ENV === "development")
        console.error("Failed to save AI settings to server:", err);
      toast.addToast(t("test_failed"), "error");
    }
  };

  const handleTest = async () => {
    try {
      const r = await aiChat.mutateAsync({
        messages: [{ role: "user", content: "Say hello" }],
        provider: config.chat?.provider,
        model: config.chat?.model,
      });
      toast.addToast(r.response, "info");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error ||
        (err as { message?: string })?.message ||
        t("test_failed");
      toast.addToast(String(msg), "error");
    }
  };

  const handleSummarizeTest = async () => {
    try {
      const r = await aiChat.mutateAsync({
        messages: [
          {
            role: "system",
            content: prompts.summarize || DEFAULT_PROMPTS.summarize,
          },
          {
            role: "user",
            content:
              "Meeting reminder: The quarterly review is scheduled for Friday at 3pm. Please prepare your department slides by Wednesday.",
          },
        ],
        provider: config.summarize?.provider,
        model: config.summarize?.model,
      });
      toast.addToast(r.response, "info");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error ||
        (err as { message?: string })?.message ||
        t("test_failed");
      toast.addToast(String(msg), "error");
    }
  };

  const handleCategorize = async () => {
    try {
      const r = await aiCat.mutateAsync({
        text: "Invoice #12345 for $500 is due.",
        provider: config.categorize?.provider,
        model: config.categorize?.model,
      });
      toast.addToast("Tags: " + r.tags.join(", "), "info");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error ||
        (err as { message?: string })?.message ||
        t("test_failed");
      toast.addToast(String(msg), "error");
    }
  };

  const handleAutoDraft = async () => {
    try {
      const r = await aiChat.mutateAsync({
        messages: [
          {
            role: "system",
            content: prompts.autodraft || DEFAULT_PROMPTS.autodraft,
          },
          {
            role: "user",
            content:
              "Subject: Meeting tomorrow at 10am\nFrom: john@example.com\n\nHi, can we reschedule to 11am? Let me know if that works for you.",
          },
        ],
        provider: config.autodraft?.provider,
        model: config.autodraft?.model,
      });
      toast.addToast("Draft: " + r.response.slice(0, 120) + "...", "info");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error ||
        (err as { message?: string })?.message ||
        t("test_failed");
      toast.addToast(String(msg), "error");
    }
  };

  return (
    <div className="space-y-6">
      {envOnly && (
        <div className="bg-primary/10 text-primary px-4 py-3 rounded-lg flex items-center gap-2 mb-4 border border-primary/20">
          <span className="text-xl">🔒</span>
          <div>
            <p className="font-medium text-sm">{t("managed_by_admin")}</p>
            <p className="text-xs opacity-80">{t("managed_by_admin_desc")}</p>
          </div>
        </div>
      )}

      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("ai_presets")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2 flex-wrap">
            {PRESETS.map((p) => (
              <button
                key={p.id}
                disabled={envOnly}
                onClick={() => handlePreset(p)}
                className={`px-3 py-1.5 rounded-full text-xs border transition-colors flex items-center gap-1.5 ${
                  preset === p.id
                    ? "border-primary bg-primary/10 text-primary"
                    : "border text-muted-foreground hover:text-foreground"
                } ${envOnly ? "opacity-50 cursor-not-allowed" : ""}`}
              >
                <p.icon className="w-3 h-3" /> {p.label}
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      {TASKS.map((task) => (
        <Card key={task.id} className="border">
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <task.icon className="w-4 h-4 text-primary" /> {task.label}
              <span className="text-xs text-muted-foreground font-normal ms-1">
                {task.desc}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex gap-2">
              <select
                value={config[task.id]?.provider || DEFAULTS[task.id].provider}
                onChange={(e) => {
                  const newProvider = e.target.value;
                  setConfig({
                    ...config,
                    [task.id]: {
                      provider: newProvider,
                      // Reset model when switching providers to avoid sending
                      // incompatible models (e.g. grok-2 to DeepSeek API).
                      model: "",
                    },
                  });
                }}
                className="w-[140px] shrink-0 h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm"
              >
                {PROVIDERS.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <ModelSelector
                provider={
                  config[task.id]?.provider || DEFAULTS[task.id].provider
                }
                apiKey={
                  apiKeys[
                    config[task.id]?.provider || DEFAULTS[task.id].provider
                  ]
                }
                value={config[task.id]?.model || DEFAULTS[task.id].model}
                onChange={(val) =>
                  setConfig({
                    ...config,
                    [task.id]: { ...config[task.id], model: val },
                  })
                }
                disabled={envOnly}
              />
            </div>
            <textarea
              value={prompts[task.id] || ""}
              onChange={(e) =>
                setPrompts({ ...prompts, [task.id]: e.target.value })
              }
              placeholder={t("ai_prompt_placeholder", { task: task.label })}
              className="w-full bg-muted border rounded px-3 py-2 text-xs resize-none h-16"
            />
          </CardContent>
        </Card>
      ))}

      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("api_keys")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {PROVIDERS.filter((p) => p.needKey).map((p) => (
            <div key={p.id} className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground w-24 shrink-0">
                {p.name}
              </span>
              <Input
                disabled={envOnly}
                type="password"
                value={apiKeys[p.id] || ""}
                onChange={(e) =>
                  setApiKeys({ ...apiKeys, [p.id]: e.target.value })
                }
                placeholder={
                  hasSavedKey[p.id]
                    ? t("api_key_saved_placeholder")
                    : t("api_key_placeholder")
                }
              />
              {apiKeys[p.id] && (
                <span className="text-xs text-muted-foreground font-mono ms-1">
                  {maskKey(apiKeys[p.id])}
                </span>
              )}
              {!apiKeys[p.id] && hasSavedKey[p.id] && (
                <span className="text-xs text-emerald-500 font-medium ms-1 shrink-0">
                  ● {t("api_key_saved_indicator")}
                </span>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      <div className="flex gap-2">
        <Button size="sm" disabled={envOnly} onClick={handleSave}>
          <Save className="w-3 h-3 me-1" /> {t("save_all")}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          onClick={handleTest}
          disabled={aiChat.isPending}
        >
          <Play className="w-3 h-3 me-1" /> {t("test_chat")}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          onClick={handleSummarizeTest}
          disabled={aiChat.isPending}
        >
          <Sparkles className="w-3 h-3 me-1" /> {t("test_summarize")}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          onClick={handleCategorize}
          disabled={aiCat.isPending}
        >
          <Search className="w-3 h-3 me-1" /> {t("test_categorize")}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          onClick={handleAutoDraft}
          disabled={aiChat.isPending}
        >
          <MessageCircle className="w-3 h-3 me-1" /> {t("ai_test_autodraft")}
        </Button>
      </div>
    </div>
  );
}
