"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import type { EmailComment } from "@/hooks/useEmails";

interface EmailCommentsSectionProps {
  comments: EmailComment[];
  onAddComment: (body: string, internal: boolean) => void;
  onDeleteComment: (id: string) => void;
}

export function EmailCommentsSection({
  comments,
  onAddComment,
  onDeleteComment,
}: EmailCommentsSectionProps) {
  const t = useTranslations("mail");
  const [commentText, setCommentText] = useState("");

  return (
    <div className="border-t border-border-muted pt-6 space-y-3">
      <h3 className="text-sm font-medium">
        {t("comments")} ({comments.length || 0})
      </h3>
      {(!comments || comments.length === 0) && (
        <p className="text-xs text-text-muted">{t("no_comments")}</p>
      )}
      <div className="space-y-2 max-h-40 overflow-y-auto">
        {(comments || []).map((c) => (
          <div
            key={c.id}
            className="flex items-start justify-between gap-2 text-xs text-text-muted bg-muted/30 rounded p-2"
          >
            <div className="flex-1 min-w-0">
              <span className="font-medium text-text-main/80">
                {c.author_id?.startsWith("00000000")
                  ? t("system_user")
                  : t("unknown_user", { id: c.author_id?.slice(0, 8) })}
              </span>
              : {c.body}
            </div>
            <button
              className="text-text-muted hover:text-red-400 shrink-0"
              onClick={() => onDeleteComment(c.id)}
              title={t("delete_comment")}
            >
              ✕
            </button>
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <Input
          className="flex-1 text-xs border-border-muted/50 focus:border-primary/50 transition-colors"
          placeholder={t("add_comment")}
          value={commentText}
          onChange={(e) => setCommentText(e.target.value)}
        />
        <button
          className="text-xs bg-primary text-primary-foreground px-3 py-1 rounded disabled:opacity-50"
          disabled={!commentText.trim()}
          onClick={() => {
            if (!commentText.trim()) return;
            onAddComment(commentText, true);
            setCommentText("");
          }}
        >
          {t("actions.send")}
        </button>
      </div>
    </div>
  );
}
