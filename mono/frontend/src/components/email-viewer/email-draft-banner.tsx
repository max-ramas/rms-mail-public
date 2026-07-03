"use client";

import { useState, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import type { Email } from "@/hooks/useEmails";
import { useToast } from "@/hooks/useToast";

interface EmailDraftBannerProps {
  draftReply: string | undefined;
  selectedEmail: Email;
  onReply: () => void;
  onClearDraft: (emailId: string) => void;
  getSignatureHtml?: () => string;
  onInsertDraft: (html: string) => void;
}

export function EmailDraftBanner({
  draftReply,
  selectedEmail,
  onReply,
  onClearDraft,
  getSignatureHtml,
  onInsertDraft,
}: EmailDraftBannerProps) {
  const t = useTranslations("mail");
  const toast = useToast();
  const [hiddenDrafts, setHiddenDrafts] = useState<Set<string>>(new Set());
  const dismissTimers = useRef<Record<string, NodeJS.Timeout>>({});

  useEffect(() => {
    const timers = dismissTimers.current;
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  }, []);

  if (!draftReply || hiddenDrafts.has(selectedEmail.id)) return null;

  if (draftReply === "[AI draft pending]") {
    return (
      <div className="bg-primary/5 border border-primary/20 rounded-xl p-4 space-y-3 animate-pulse">
        <div className="h-4 bg-primary/20 rounded w-1/4"></div>
        <div className="space-y-2">
          <div className="h-3 bg-zinc-200 dark:bg-zinc-800 rounded w-full"></div>
          <div className="h-3 bg-zinc-200 dark:bg-zinc-800 rounded w-5/6"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-primary/10 border border-primary/30 rounded-xl p-4 space-y-2">
      <p className="text-xs font-medium text-primary">
        {t("ai_draft_reply_ready")}
      </p>
      <p className="text-sm text-text-main/80 line-clamp-3">{draftReply}</p>
      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={() => {
            const sigHtml = getSignatureHtml ? getSignatureHtml() : "";
            const formattedDraft = draftReply.replace(/\n/g, "<br/>");
            onInsertDraft(
              formattedDraft + (sigHtml ? "<br/>" + sigHtml : ""),
            );
            onReply();
            try {
              onClearDraft(selectedEmail.id);
            } catch (err) {
              if (process.env.NODE_ENV === "development")
                console.error("Failed to clear draft on backend:", err);
            }
          }}
        >
          {t("insert_into_reply")}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => {
            const emailId = selectedEmail.id;
            setHiddenDrafts((prev) => new Set(prev).add(emailId));

            toast.addToast(
              <div className="flex items-center justify-between w-full">
                <span>
                  {t("toast_draft_dismissed", {
                    defaultMessage: "Draft hidden",
                  })}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  className="ml-4 h-7 text-xs border-zinc-600 bg-zinc-800 text-zinc-200 hover:bg-zinc-700 hover:text-white"
                  onClick={(e) => {
                    e.stopPropagation();
                    clearTimeout(dismissTimers.current[emailId]);
                    delete dismissTimers.current[emailId];
                    setHiddenDrafts((prev) => {
                      const next = new Set(prev);
                      next.delete(emailId);
                      return next;
                    });
                  }}
                >
                  {t("toast_undo", { defaultMessage: "Undo" })}
                </Button>
              </div>,
              "info",
            );

            dismissTimers.current[emailId] = setTimeout(() => {
              try {
                onClearDraft(emailId);
              } catch (err) {
                if (process.env.NODE_ENV === "development")
                  console.error("Failed to clear draft on backend:", err);
              }
              delete dismissTimers.current[emailId];
            }, 5000);
          }}
        >
          {t("dismiss_draft")}
        </Button>
      </div>
    </div>
  );
}
