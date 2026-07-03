"use client";

import React, { useState, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import { EmailToolbar } from "@/components/email-toolbar";
import type { EmailBodyHandle } from "@/components/email-body";
import { editionLetter } from "@/hooks/useEmails";
import { useFolders } from "@/hooks/useEmailQueries";
import { useToast } from "@/hooks/useToast";
import type { Email } from "@/hooks/useEmails";
import { EmailViewerEmpty } from "./email-viewer/email-viewer-empty";
import { EmailViewerHeader } from "./email-viewer/email-viewer-header";
import { EmailReplyComposeForm, type EmailReplyComposeFormHandle } from "./email-viewer/email-reply-compose-form";
import { EmailSummaryBlock } from "./email-viewer/email-summary-block";
import { EmailTranslationBlock } from "./email-viewer/email-translation-block";
import { EmailDraftBanner } from "./email-viewer/email-draft-banner";
import { EmailBodySection } from "./email-viewer/email-body-section";
import { EmailThreadStack } from "./email-viewer/email-thread-stack";
import { EmailAiTags } from "./email-viewer/email-ai-tags";
import { EmailAssigneeSection } from "./email-viewer/email-assignee-section";
import { EmailCommentsSection } from "./email-viewer/email-comments-section";
import { EmailChatPanel } from "./email-viewer/email-chat-panel";
import { useEmailPrint } from "./email-viewer/use-email-print";
import type { EmailViewerProps } from "./email-viewer/types";

export type { EmailViewerProps } from "./email-viewer/types";

export const EmailViewer = React.memo(function EmailViewer({
  selectedEmail,
  locale,
  summary,
  translation,
  onBackClick,
  onReply,
  onForward,
  onReplyAll,
  onReplyWithQuote,
  onDismissTranslation,
  onArchive,
  onToggleRead,
  onDelete,
  onSummarize,
  onCategorize,
  onTranslate,
  onTogglePin,
  onToggleFlag,
  onToggleMute,
  onSnooze,
  onClearDraft,
  onCompose,
  summarizePending,
  categorizePending,
  translatePending,
  isReplying,
  isForwarding,
  replyTo,
  composeTo,
  composeCc,
  composeSubject,
  composeBody,
  forwardOriginalHtml,
  forwardMeta,
  fromIdentity,
  composeAttachments,
  accounts,
  activeAccount,
  identities,
  onRemoveAttachment,
  onCancelReply,
  onSendEmail,
  sendPending,
  onSaveDraft,
  saveDraftPending,
  onFileInputClick,
  getSignatureHtml,
  emailHtml,
  emailBody,
  emailSnippet,
  emailLoading,
  emailAttachments,
  draftReply,
  emailTagsData,
  labelsData,
  selectedLabelIds,
  onToggleLabel,
  usersData,
  onAssign,
  onUnassign,
  onSetStatus,
  comments,
  onAddComment,
  onDeleteComment,
  aiEnabled = true,
  chatOpen,
  onToggleChat,
  chatMessages,
  onChatSend,
  chatPending,
  formatFileSize,
  threadEmails,
  onRestoreFromTrash,
  onReplyToEmail,
  onDeleteEmail,
  onEmailAction,
}: EmailViewerProps) {
  const t = useTranslations("mail");
  const toast = useToast();
  const isTeams = editionLetter() === "T";
  const composeFormRef = useRef<EmailReplyComposeFormHandle>(null);
  const emailBodyRef = useRef<EmailBodyHandle>(null);
  const emailHtmlRef = useRef(emailHtml);
  const emailBodyRef2 = useRef(emailBody);
  emailHtmlRef.current = emailHtml;
  emailBodyRef2.current = emailBody;

  const [useThreads, setUseThreads] = useState(true);
  useEffect(() => {
    setUseThreads(localStorage.getItem("rms-mail_use_threads") !== "false");
  }, [selectedEmail?.id]);

  const [activeThreadEmail, setActiveThreadEmail] = useState<Email | null>(null);

  const foldersQuery = useFolders(selectedEmail?.account_id ?? "");
  const isTrash = React.useMemo(() => {
    const fid = selectedEmail?.folder_id;
    if (!fid || !foldersQuery.data) return false;
    const match = foldersQuery.data.find((f) => f.id === fid);
    if (!match) return false;
    const trashNames = ["Trash", "TRASH", "trash", "Корзина", "Deleted Items"];
    return trashNames.includes(match.name);
  }, [selectedEmail?.folder_id, foldersQuery.data]);

  const sortedThreadLength = threadEmails?.length ?? 0;

  useEmailPrint({
    selectedEmail,
    useThreads,
    threadEmails,
    locale,
    emailHtmlRef,
    emailBodyRef: emailBodyRef2,
  });

  const handleReplyWithQuote = React.useCallback(() => {
    const selectedText = emailBodyRef.current?.getSelectedText() || "";
    if (activeThreadEmail) {
      onReplyToEmail?.(activeThreadEmail);
      setActiveThreadEmail(null);
    }
    onReplyWithQuote?.(selectedText);
  }, [onReplyWithQuote, activeThreadEmail, onReplyToEmail]);

  if (!selectedEmail) {
    return (
      <EmailViewerEmpty
        aiEnabled={aiEnabled}
        onCompose={onCompose}
        onBackClick={onBackClick}
      />
    );
  }

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <EmailToolbar
        aiEnabled={aiEnabled}
        onBackClick={onBackClick}
        selectedEmail={selectedEmail}
        isComposing={false}
        isReplying={isReplying}
        isForwarding={isForwarding}
        onCompose={onCompose}
        onReply={() => {
          if (activeThreadEmail) {
            onReplyToEmail?.(activeThreadEmail);
            setActiveThreadEmail(null);
          } else if (isReplying) {
            onCancelReply();
          } else {
            onReply();
          }
        }}
        onReplyAll={onReplyAll}
        onReplyWithQuote={handleReplyWithQuote}
        onForward={() => {
          if (isForwarding) onCancelReply();
          else onForward();
        }}
        onSnooze={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "snooze")
            : onSnooze(180)
        }
        onSummarize={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "summarize")
            : onSummarize?.()
        }
        onCategorize={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "categorize")
            : onCategorize?.()
        }
        onChatToggle={onToggleChat ?? (() => {})}
        onPin={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "pin")
            : onTogglePin()
        }
        onMute={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "mute")
            : onToggleMute()
        }
        onTranslate={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "translate")
            : onTranslate?.()
        }
        onDownloadEml={async () => {
          const targetId = activeThreadEmail?.id || selectedEmail?.id;
          if (!targetId) return;
          try {
            const { default: axios } = await import("axios");
            const response = await axios.get(`/api/emails/${targetId}/raw`, {
              responseType: "blob",
            });
            const url = URL.createObjectURL(new Blob([response.data]));
            const a = document.createElement("a");
            a.href = url;
            a.download = `${targetId.slice(0, 8)}.eml`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            toast.addToast(t("toast_eml_downloaded"), "success");
          } catch (err) {
            if (process.env.NODE_ENV === "development")
              console.error("Download failed:", err);
            toast.addToast(t("toast_eml_download_error"), "error");
          }
        }}
        onArchive={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "archive")
            : onArchive()
        }
        onToggleRead={() =>
          activeThreadEmail
            ? onEmailAction?.(activeThreadEmail.id, "toggleRead")
            : onToggleRead()
        }
        onDelete={
          activeThreadEmail
            ? () => {
                onDeleteEmail?.(activeThreadEmail.id);
                setActiveThreadEmail(null);
              }
            : onDelete
        }
        onRestoreFromTrash={() => onRestoreFromTrash?.(selectedEmail?.id ?? "")}
        isTrash={isTrash}
        summarizePending={summarizePending}
        categorizePending={categorizePending}
        translatePending={translatePending}
      />

      <div className="flex-1 min-h-0 flex flex-col">
        <EmailViewerHeader
          selectedEmail={selectedEmail}
          locale={locale}
          labelsData={labelsData}
          selectedLabelIds={selectedLabelIds}
          onToggleLabel={onToggleLabel}
          onTogglePin={onTogglePin}
          onToggleFlag={onToggleFlag}
        />

        <div className="flex-1 overflow-y-auto overflow-x-auto">
          <div className="px-6 py-3 space-y-4">
            <EmailReplyComposeForm
              ref={composeFormRef}
              isReplying={isReplying}
              isForwarding={isForwarding}
              replyTo={replyTo}
              composeTo={composeTo}
              composeCc={composeCc}
              composeSubject={composeSubject}
              composeBody={composeBody}
              fromIdentity={fromIdentity}
              forwardOriginalHtml={forwardOriginalHtml}
              forwardMeta={forwardMeta}
              composeAttachments={composeAttachments}
              accounts={accounts}
              activeAccount={activeAccount}
              identities={identities}
              onRemoveAttachment={onRemoveAttachment}
              onCancelReply={onCancelReply}
              onSendEmail={onSendEmail}
              sendPending={sendPending}
              onSaveDraft={onSaveDraft}
              saveDraftPending={saveDraftPending}
              onFileInputClick={onFileInputClick}
            />
            <EmailSummaryBlock summary={summary} />
            <EmailTranslationBlock
              translation={translation}
              onDismiss={() => onDismissTranslation?.()}
            />
            <EmailDraftBanner
              draftReply={draftReply}
              selectedEmail={selectedEmail}
              onReply={onReply}
              onClearDraft={onClearDraft}
              getSignatureHtml={getSignatureHtml}
              onInsertDraft={(html) => composeFormRef.current?.insertBody(html)}
            />
            {useThreads && sortedThreadLength > 1 && threadEmails ? (
              <EmailThreadStack
                key={selectedEmail.id}
                selectedEmail={selectedEmail}
                threadEmails={threadEmails}
                locale={locale}
                emailHtml={emailHtml}
                emailBody={emailBody}
                emailAttachments={emailAttachments}
                emailBodyRef={emailBodyRef}
                formatFileSize={formatFileSize}
                onReplyToEmail={onReplyToEmail}
                onDeleteEmail={onDeleteEmail}
                activeThreadEmail={activeThreadEmail}
                onActiveThreadEmailChange={setActiveThreadEmail}
              />
            ) : (
              <EmailBodySection
                emailBodyRef={emailBodyRef}
                emailLoading={emailLoading}
                emailHtml={emailHtml}
                emailBody={emailBody}
                emailSnippet={emailSnippet}
                emailId={selectedEmail?.id}
                emailAttachments={emailAttachments}
                formatFileSize={formatFileSize}
              />
            )}
            <EmailAiTags tags={emailTagsData} />
            {isTeams && (
              <EmailAssigneeSection
                selectedEmail={selectedEmail}
                usersData={usersData}
                onAssign={onAssign}
                onUnassign={onUnassign}
                onSetStatus={onSetStatus}
              />
            )}
            <EmailCommentsSection
              comments={comments}
              onAddComment={onAddComment}
              onDeleteComment={onDeleteComment}
            />
          </div>
        </div>
      </div>

      <EmailChatPanel
        chatOpen={chatOpen}
        onToggleChat={onToggleChat}
        chatMessages={chatMessages}
        onChatSend={onChatSend}
        chatPending={chatPending}
      />
    </div>
  );
});
