"use client";

import { useTranslations } from "next-intl";
import { EmailBody, type EmailBodyHandle } from "@/components/email-body";
import type { RefObject } from "react";

interface EmailBodySectionProps {
  emailBodyRef: RefObject<EmailBodyHandle | null>;
  emailLoading: boolean;
  emailHtml: string | undefined;
  emailBody: string | undefined;
  emailSnippet: string;
  emailId: string | undefined;
  emailAttachments:
    | Array<{
        hash: string;
        filename: string;
        size: number;
        content_id?: string;
      }>
    | undefined;
  formatFileSize: (bytes: number) => string;
}

export function EmailBodySection({
  emailBodyRef,
  emailLoading,
  emailHtml,
  emailBody,
  emailSnippet,
  emailId,
  emailAttachments,
  formatFileSize,
}: EmailBodySectionProps) {
  const t = useTranslations("mail");

  return (
    <div className="border border-border-muted rounded-xl shadow-sm overflow-hidden">
      {emailLoading ? (
        <div className="space-y-4 animate-pulse">
          <div className="h-4 bg-muted rounded w-3/4" />
          <div className="h-4 bg-muted rounded w-1/2" />
          <div className="h-4 bg-muted rounded w-5/6" />
        </div>
      ) : (
        <EmailBody
          ref={emailBodyRef}
          html={emailHtml}
          body={emailBody}
          snippet={emailSnippet}
          emailId={emailId}
          attachments={emailAttachments}
        />
      )}
      {emailAttachments && emailAttachments.length > 0 && (
        <div className="border-t border-border-muted mt-4 pt-4 px-4 pb-4">
          <h3 className="text-xs font-medium text-text-muted mb-2 flex items-center gap-1">
            📎 {t("attachments")} ({emailAttachments.length})
          </h3>
          <div className="flex flex-wrap gap-2">
            {emailAttachments.map((att) => (
              <a
                key={att.hash}
                href={`/api/attachments/${att.hash}`}
                download={att.filename}
                className="inline-flex items-center gap-1.5 bg-muted px-3 py-1.5 rounded-lg text-xs text-text-main/80 hover:bg-muted/80 hover:text-text-main transition-colors border border-border-muted"
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
  );
}
