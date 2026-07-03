"use client";

import { useTranslations } from "next-intl";
import { EmailToolbar } from "@/components/email-toolbar";

interface EmailViewerEmptyProps {
  aiEnabled?: boolean;
  onCompose: () => void;
  onBackClick?: () => void;
}

export function EmailViewerEmpty({
  aiEnabled,
  onCompose,
  onBackClick,
}: EmailViewerEmptyProps) {
  const t = useTranslations("mail");

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <EmailToolbar
        aiEnabled={aiEnabled}
        selectedEmail={null}
        isComposing={false}
        isReplying={false}
        isForwarding={false}
        onCompose={onCompose}
        onReply={() => {}}
        onReplyAll={() => {}}
        onReplyWithQuote={() => {}}
        onForward={() => {}}
        onSnooze={() => {}}
        onSummarize={() => {}}
        onCategorize={() => {}}
        onChatToggle={() => {}}
        onPin={() => {}}
        onMute={() => {}}
        onTranslate={() => {}}
        onDownloadEml={() => {}}
        onArchive={() => {}}
        onDelete={() => {}}
        summarizePending={false}
        categorizePending={false}
        translatePending={false}
        onBackClick={onBackClick}
      />
      <div className="flex-1 flex items-center justify-center text-text-muted">
        {t("select_email")}
      </div>
    </div>
  );
}
