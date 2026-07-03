"use client";

import React, { useState } from "react";
import { Reply, Trash2, ChevronDown } from "lucide-react";
import { useTranslations } from "next-intl";
import { formatEmailDatetime } from "@/lib/date-format";
import { Avatar } from "@/components/avatar";
import { EmailBody, type EmailBodyHandle } from "@/components/email-body";
import type { Email } from "@/hooks/useEmails";
import type { RefObject } from "react";

interface EmailThreadStackProps {
  selectedEmail: Email;
  threadEmails: Email[];
  locale: string;
  emailHtml: string | undefined;
  emailBody: string | undefined;
  emailAttachments:
    | Array<{
        hash: string;
        filename: string;
        size: number;
        content_id?: string;
      }>
    | undefined;
  emailBodyRef: RefObject<EmailBodyHandle | null>;
  formatFileSize: (bytes: number) => string;
  onReplyToEmail?: (email: Email) => void;
  onDeleteEmail?: (emailId: string) => void;
  activeThreadEmail: Email | null;
  onActiveThreadEmailChange: (email: Email | null) => void;
}

export function EmailThreadStack({
  selectedEmail,
  threadEmails,
  locale,
  emailHtml,
  emailBody,
  emailAttachments,
  emailBodyRef,
  formatFileSize,
  onReplyToEmail,
  onDeleteEmail,
  activeThreadEmail,
  onActiveThreadEmailChange,
}: EmailThreadStackProps) {
  const t = useTranslations("mail");

  const sortedThread = React.useMemo(() => {
    if (!threadEmails || threadEmails.length === 0) return [];
    return [...threadEmails].sort(
      (a, b) =>
        new Date(b.date_sent).getTime() - new Date(a.date_sent).getTime(),
    );
  }, [threadEmails]);

  const [expandedEmails, setExpandedEmails] = useState<Record<string, boolean>>(
    () => (selectedEmail?.id ? { [selectedEmail.id]: true } : {}),
  );
  const [lazyEmailData, setLazyEmailData] = useState<
    Record<
      string,
      {
        html?: string;
        body?: string;
        attachments?: Array<{
          hash: string;
          filename: string;
          size: number;
          content_id?: string;
        }>;
        loading: boolean;
      }
    >
  >({});

  const handleToggleExpand = async (emailId: string) => {
    const isCurrentlyExpanded = !!expandedEmails[emailId];
    setExpandedEmails((prev) => ({ ...prev, [emailId]: !isCurrentlyExpanded }));

    const found = sortedThread.find((e) => e.id === emailId);
    if (found) onActiveThreadEmailChange(found);

    if (
      !isCurrentlyExpanded &&
      emailId !== selectedEmail?.id &&
      !lazyEmailData[emailId]
    ) {
      setLazyEmailData((prev) => ({
        ...prev,
        [emailId]: { loading: true },
      }));
      try {
        const { default: axios } = await import("axios");
        const r = await axios.get(`/api/emails/${emailId}`);
        setLazyEmailData((prev) => ({
          ...prev,
          [emailId]: {
            html: r.data.html,
            body: r.data.body,
            attachments: r.data.attachments,
            loading: false,
          },
        }));
      } catch (err) {
        if (process.env.NODE_ENV === "development")
          console.error("Failed to load thread email content", err);
        setLazyEmailData((prev) => ({
          ...prev,
          [emailId]: { loading: false },
        }));
      }
    }
  };

  return (
    <div className="space-y-4">
      {sortedThread.map((email) => {
        const isCurrent = email.id === selectedEmail?.id;
        const isExpanded = !!expandedEmails[email.id];
        const isLazyLoading = !isCurrent && lazyEmailData[email.id]?.loading;
        const currentHtml = isCurrent
          ? emailHtml
          : lazyEmailData[email.id]?.html;
        const currentBody = isCurrent
          ? emailBody
          : lazyEmailData[email.id]?.body;
        const currentAttachments = isCurrent
          ? emailAttachments
          : lazyEmailData[email.id]?.attachments;

        return (
          <div
            key={email.id}
            className={`border rounded-xl transition-all duration-300 shadow-sm overflow-hidden ${
              isExpanded
                ? "bg-card-bg/60 border-border-muted/80"
                : "bg-card-bg/30 border-border-muted/30 hover:border-border-muted/60 p-4 hover:shadow-md"
            } ${activeThreadEmail?.id === email.id ? "border-primary/60 shadow-[0_0_0_2px_rgba(251,191,36,0.35)]" : ""}`}
          >
            <div
              onClick={() => handleToggleExpand(email.id)}
              className={`flex items-center justify-between gap-4 cursor-pointer select-none ${isExpanded ? "p-4" : ""}`}
            >
              <div className="flex items-center gap-3 min-w-0">
                <Avatar
                  src={email.avatar_url || null}
                  name={email.sender_name}
                  email={email.sender_address}
                  size={36}
                />
                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-semibold text-text-main truncate">
                      {email.sender_name || email.sender_address.split("@")[0]}
                    </span>
                    <span className="text-xs text-text-muted truncate">
                      &lt;{email.sender_address}&gt;
                    </span>
                  </div>
                  {!isExpanded && (
                    <p className="text-xs text-text-muted truncate max-w-xl mt-0.5">
                      {email.snippet}
                    </p>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-1 shrink-0">
                {onReplyToEmail && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onReplyToEmail(email);
                    }}
                    className="p-1 rounded-md text-text-muted hover:text-text-main hover:bg-muted/60 transition-colors"
                    title={t("thread_reply_to_message")}
                  >
                    <Reply className="w-3.5 h-3.5" />
                  </button>
                )}
                {onDeleteEmail && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onDeleteEmail(email.id);
                    }}
                    className="p-1 rounded-md text-text-muted hover:text-red-400 hover:bg-red-500/10 transition-colors"
                    title={t("thread_delete_message")}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                )}
                <span className="text-xs text-text-muted ms-1">
                  {formatEmailDatetime(email.date_sent, locale)}
                </span>
                <ChevronDown
                  className={`w-4 h-4 text-text-muted transition-transform duration-200 ${
                    isExpanded ? "rotate-180" : ""
                  }`}
                />
              </div>
            </div>

            {isExpanded && (
              <div className="border-t border-border-muted/30 p-4 pt-4 animate-in fade-in slide-in-from-top-1 duration-200 space-y-4">
                {isLazyLoading ? (
                  <div className="space-y-4 animate-pulse">
                    <div className="h-4 bg-muted rounded w-3/4" />
                    <div className="h-4 bg-muted rounded w-1/2" />
                    <div className="h-4 bg-muted rounded w-5/6" />
                  </div>
                ) : (
                  <EmailBody
                    ref={isCurrent ? emailBodyRef : undefined}
                    html={currentHtml}
                    body={currentBody}
                    snippet={email.snippet}
                    emailId={email.id}
                    attachments={currentAttachments}
                    onBodyClick={() => {
                      const found = sortedThread.find((e) => e.id === email.id);
                      if (found) onActiveThreadEmailChange(found);
                    }}
                  />
                )}

                {currentAttachments && currentAttachments.length > 0 && (
                  <div className="border-t border-border-muted/30 pt-4">
                    <h4 className="text-xs font-medium text-text-muted mb-2 flex items-center gap-1">
                      📎 {t("attachments")} ({currentAttachments.length})
                    </h4>
                    <div className="flex flex-wrap gap-2">
                      {currentAttachments.map((att) => (
                        <a
                          key={att.hash}
                          href={`/api/attachments/${att.hash}`}
                          download={att.filename}
                          className="inline-flex items-center gap-1.5 bg-muted px-3 py-1.5 rounded-lg text-xs text-text-main/80 hover:bg-muted/80 hover:text-text-main transition-colors border border-border-muted/30"
                        >
                          <span>📎</span>
                          <span className="font-medium">{att.filename}</span>
                          <span className="text-text-muted ms-1">
                            ({formatFileSize(att.size)})
                          </span>
                        </a>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
