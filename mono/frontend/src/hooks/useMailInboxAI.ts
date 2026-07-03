"use client";

import { useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import type { AIMessage, Email } from "@/hooks/useEmailTypes";
import { getAIConfig } from "@/lib/ai-config";
import { getTranslationTargetLanguage } from "@/lib/ai-locale";

interface UseMailInboxAIOptions {
  locale: string;
  selectedEmail?: Email | null;
  emailBody?: string;
  emailSnippet?: string;
  inboxPreview?: Email[];
  summarizeMutation: {
    mutateAsync: (args: {
      emailId: string;
      aiConfig?: ReturnType<typeof getAIConfig>;
    }) => Promise<{ summary: string }>;
  };
  categorizeMutation: {
    mutateAsync: (args: {
      emailId: string;
      aiConfig?: ReturnType<typeof getAIConfig>;
    }) => Promise<unknown>;
  };
  aiChatMutation: {
    mutateAsync: (args: {
      messages: AIMessage[];
      provider?: string;
      model?: string;
    }) => Promise<{ response: string }>;
  };
  toast: {
    addToast: (
      message: React.ReactNode,
      type?: "success" | "error" | "info",
    ) => void;
  };
}

function readChatModelConfig(): { provider?: string; model?: string } {
  try {
    const saved = JSON.parse(
      localStorage.getItem("rms-mail_ai_config") || "{}",
    );
    const cfg = saved.config || saved;
    return {
      provider: cfg.chat?.provider,
      model: cfg.chat?.model,
    };
  } catch {
    return {};
  }
}

export function useMailInboxAI({
  locale,
  selectedEmail,
  emailBody,
  emailSnippet,
  inboxPreview,
  summarizeMutation,
  categorizeMutation,
  aiChatMutation,
  toast,
}: UseMailInboxAIOptions) {
  const t = useTranslations("mail");
  const [summary, setSummary] = useState<string | null>(null);
  const [translation, setTranslation] = useState<string | null>(null);
  const [chatOpen, setChatOpen] = useState(false);
  const [chatMessages, setChatMessages] = useState<AIMessage[]>([]);

  const handleSummarize = useCallback(async () => {
    if (!selectedEmail) return;
    setSummary(null);
    try {
      const result = await summarizeMutation.mutateAsync({
        emailId: selectedEmail.id,
        aiConfig: getAIConfig("summarize"),
      });
      setSummary(result.summary);
    } catch (err) {
      if (process.env.NODE_ENV === "development")
        console.error("Summarization failed:", err);
    }
  }, [selectedEmail, summarizeMutation]);

  const handleCategorize = useCallback(async () => {
    if (!selectedEmail) return;
    try {
      await categorizeMutation.mutateAsync({
        emailId: selectedEmail.id,
        aiConfig: getAIConfig("categorize"),
      });
      toast.addToast(t("toast_categorized"), "success");
    } catch {
      toast.addToast(t("toast_categorization_failed"), "error");
    }
  }, [selectedEmail, categorizeMutation, toast, t]);

  const translateBody = useCallback(
    async (bodyText: string) => {
      const { provider, model } = readChatModelConfig();
      const target = getTranslationTargetLanguage(locale);
      const result = await aiChatMutation.mutateAsync({
        messages: [
          {
            role: "user",
            content: `Translate this email to ${target}. Return only the translation, no explanations.\n\n${bodyText}`,
          },
        ],
        provider,
        model,
      });
      setTranslation(result.response);
    },
    [aiChatMutation, locale],
  );

  const handleTranslate = useCallback(async () => {
    if (!selectedEmail) return;
    setTranslation(null);
    const bodyText = emailBody || emailSnippet || "";
    if (!bodyText) {
      toast.addToast(t("toast_no_body_to_translate"), "error");
      return;
    }
    try {
      await translateBody(bodyText);
    } catch (err) {
      toast.addToast(t("toast_translation_failed"), "error");
      if (process.env.NODE_ENV === "development")
        console.error("Translation failed:", err);
    }
  }, [selectedEmail, emailBody, emailSnippet, translateBody, toast, t]);

  const handleTranslateEmail = useCallback(
    async (emailId: string) => {
      try {
        const { default: axios } = await import("axios");
        const r = await axios.get(`/api/emails/${emailId}`);
        const bodyText = r.data.body || "";
        if (!bodyText) {
          toast.addToast(t("toast_no_body_to_translate"), "error");
          return;
        }
        const cfg = getAIConfig("chat");
        const target = getTranslationTargetLanguage(locale);
        const result = await aiChatMutation.mutateAsync({
          messages: [
            {
              role: "user",
              content: `Translate this email to ${target}. Return only the translation, no explanations.\n\n${bodyText}`,
            },
          ],
          provider: cfg?.provider,
          model: cfg?.model,
        });
        setTranslation(result.response);
      } catch (err) {
        if (process.env.NODE_ENV === "development")
          console.error("Thread email translate failed:", err);
        toast.addToast(t("toast_translation_failed"), "error");
      }
    },
    [aiChatMutation, locale, toast, t],
  );

  const handleChatSend = useCallback(
    async (input: string) => {
      if (!input.trim()) return;
      const userMsg: AIMessage = { role: "user", content: input };
      setChatMessages((prev) => [...prev, userMsg]);
      try {
        const { provider, model } = readChatModelConfig();
        const contextMessages = [...chatMessages];
        if (chatMessages.length === 0) {
          let systemContent =
            "You are an AI assistant integrated into RMS Mail email client.";
          if (selectedEmail) {
            const emailCtx = `\n\n[Currently Selected Email]\nSubject: ${selectedEmail.subject || ""}\nFrom: ${selectedEmail.sender_name || selectedEmail.sender_address || ""}\nSnippet: ${selectedEmail.snippet || ""}`;
            systemContent +=
              " You have access to the currently selected email." + emailCtx;
          }
          if (inboxPreview?.length) {
            const top5 = inboxPreview.slice(0, 5);
            const listCtx =
              "\n\n[Latest 5 Emails in Current View]\n" +
              top5
                .map(
                  (e, i) =>
                    `[${i + 1}] Subject: ${e.subject} | From: ${e.sender_name || e.sender_address} | Snippet: ${e.snippet}`,
                )
                .join("\n");
            systemContent += listCtx;
          }
          contextMessages.unshift({
            role: "system",
            content: systemContent,
          });
        }
        const result = await aiChatMutation.mutateAsync({
          messages: [...contextMessages, userMsg],
          provider,
          model,
        });
        setChatMessages((prev) => [
          ...prev,
          { role: "assistant", content: result.response },
        ]);
      } catch (err) {
        toast.addToast(t("toast_chat_failed"), "error");
        if (process.env.NODE_ENV === "development")
          console.error("AI chat failed:", err);
      }
    },
    [chatMessages, selectedEmail, inboxPreview, aiChatMutation, toast, t],
  );

  return {
    summary,
    setSummary,
    translation,
    setTranslation,
    chatOpen,
    setChatOpen,
    chatMessages,
    handleSummarize,
    handleCategorize,
    handleTranslate,
    handleTranslateEmail,
    handleChatSend,
  };
}
