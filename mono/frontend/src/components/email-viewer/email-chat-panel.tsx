"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import type { AIMessage } from "@/hooks/useEmails";

interface EmailChatPanelProps {
  chatOpen?: boolean;
  onToggleChat?: () => void;
  chatMessages?: AIMessage[];
  onChatSend?: (input: string) => void;
  chatPending?: boolean;
}

export function EmailChatPanel({
  chatOpen,
  onToggleChat,
  chatMessages,
  onChatSend,
  chatPending,
}: EmailChatPanelProps) {
  const t = useTranslations("mail");
  const [localChatInput, setLocalChatInput] = useState("");

  if (!chatOpen) return null;

  return (
    <div className="flex-none h-96 border-t border-border-muted flex flex-col bg-card-bg">
      <div className="flex items-center justify-between px-3 py-2 border-b border-border-muted">
        <span className="text-xs font-medium">{t("actions.chat_title")}</span>
        <button
          onClick={onToggleChat}
          className="text-text-muted hover:text-text-main text-xs"
        >
          ✕
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {(chatMessages ?? []).map((m, i) => (
          <div
            key={`${m.role}-${i}`}
            className={`text-xs p-2 rounded-lg ${m.role === "user" ? "bg-primary/10 ms-8" : "bg-muted me-8"}`}
          >
            {m.content}
          </div>
        ))}
        {chatPending && (
          <div className="text-xs text-text-muted p-2">
            {t("actions.thinking")}
          </div>
        )}
      </div>
      <div className="p-2 border-t border-border-muted flex gap-2">
        <Input
          className="flex-1 text-xs"
          placeholder={t("ask_ai")}
          value={localChatInput}
          onChange={(e) => setLocalChatInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              onChatSend?.(localChatInput);
              setLocalChatInput("");
            }
          }}
        />
        <button
          className="text-xs bg-primary text-primary-foreground px-3 py-1 rounded disabled:opacity-50"
          disabled={!localChatInput.trim() || chatPending}
          onClick={() => {
            onChatSend?.(localChatInput);
            setLocalChatInput("");
          }}
        >
          {t("actions.send")}
        </button>
      </div>
    </div>
  );
}
