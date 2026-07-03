"use client";

import React from "react";
import { apiFetch, clearToken } from "@/lib/api-client";
import { EmailViewer } from "@/components/email-viewer";
import { EmailList } from "@/components/email-list";
import { EmailSidebar } from "@/components/email-sidebar";
import { ComposePanel } from "@/components/compose-panel";
import { editionLetter } from "@/hooks/useEmails";
import type {
  Email,
  Account,
  Folder,
  Identity,
  Label,
  ProjectGroup,
} from "@/hooks/useEmailTypes";

interface MailInboxLayoutProps {
  locale: string;
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  onAttachmentUpload: (files: FileList) => Promise<void>;
  activeAccount: string;
  setActiveAccount: (v: string) => void;
  activeFolder: string;
  setActiveFolder: (v: string) => void;
  accounts: Account[];
  groups: ProjectGroup[];
  folders: Folder[];
  showAccountFolders: boolean;
  setShowAccountFolders: (v: boolean) => void;
  mobileView: "sidebar" | "list" | "viewer";
  setMobileView: (v: "sidebar" | "list" | "viewer") => void;
  onEmptyTrash: () => void;
  emails: Email[];
  useThreads: boolean;
  isLoading: boolean;
  isError?: boolean;
  selectedEmailId: string | null;
  onSelectEmail: (id: string) => void;
  onToggleFlagList: (id: string) => void;
  onTogglePinList: (id: string) => void;
  onSearchResult: (emails: Email[] | null) => void;
  onFilterChange: (filters: Record<string, string>) => void;
  accountLabels: Label[];
  emailLabelsMap: Record<string, Label[]>;
  hasNextPage: boolean;
  fetchNextPage: () => void;
  isReplying: boolean;
  replyTo: Email | null;
  composeTo: string[];
  composeCc: string[];
  composeSubject: string;
  composeBody: string;
  fromIdentity: string;
  identities: Identity[];
  onSendEmail: (options: {
    to: string[];
    cc: string[];
    subject: string;
    body: string;
    html: string;
    identity: string;
  }) => void;
  onSaveDraft: (
    syncRemote: boolean,
    data?: {
      to: string[];
      cc: string[];
      subject: string;
      body: string;
      identity: string;
    },
  ) => void;
  saveDraftPending: boolean;
  sendPending: boolean;
  attachments: Array<{ id: string; filename: string; size: number }>;
  onRemoveAttachment: (index: number) => void;
  setIsReplying: (v: boolean) => void;
  viewerProps: React.ComponentProps<typeof EmailViewer>;
  undoJob: string | null;
  undoToastLabel: string;
  undoButtonLabel: string;
  onUndoSend: (jobId: string) => void;
  isDesktop: boolean;
  commandPalette: React.ReactNode;
}

export function MailInboxLayout({
  locale,
  fileInputRef,
  onAttachmentUpload,
  activeAccount,
  setActiveAccount,
  activeFolder,
  setActiveFolder,
  accounts,
  groups,
  folders,
  showAccountFolders,
  setShowAccountFolders,
  mobileView,
  setMobileView,
  onEmptyTrash,
  emails,
  useThreads,
  isLoading,
  isError = false,
  selectedEmailId,
  onSelectEmail,
  onToggleFlagList,
  onTogglePinList,
  onSearchResult,
  onFilterChange,
  accountLabels,
  emailLabelsMap,
  hasNextPage,
  fetchNextPage,
  isReplying,
  replyTo,
  composeTo,
  composeCc,
  composeSubject,
  composeBody,
  fromIdentity,
  identities,
  onSendEmail,
  onSaveDraft,
  saveDraftPending,
  sendPending,
  attachments,
  onRemoveAttachment,
  setIsReplying,
  viewerProps,
  undoJob,
  undoToastLabel,
  undoButtonLabel,
  onUndoSend,
  isDesktop,
  commandPalette,
}: MailInboxLayoutProps) {
  return (
    <div
      className="flex h-screen bg-app-bg text-text-main font-sans"
      suppressHydrationWarning
    >
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={async (e) => {
          const files = e.target.files;
          if (!files) return;
          await onAttachmentUpload(files);
          e.target.value = "";
        }}
      />
      <EmailSidebar
        activeAccount={activeAccount}
        setActiveAccount={setActiveAccount}
        activeFolder={activeFolder}
        setActiveFolder={setActiveFolder}
        accounts={accounts}
        groups={groups}
        folders={folders}
        unreadCounts={{}}
        editionLetter={editionLetter}
        onOpenSettings={() => (window.location.href = `/${locale}/settings`)}
        onLogout={async () => {
          try {
            await apiFetch("/api/auth/logout", { method: "POST" });
          } catch {}
          clearToken();
          window.location.href = `/${locale}/login`;
        }}
        showAccountFolders={showAccountFolders}
        setShowAccountFolders={setShowAccountFolders}
        locale={locale}
        mobileView={mobileView}
        setMobileView={setMobileView}
        onEmptyTrash={onEmptyTrash}
      />

      <div className="flex flex-1 overflow-hidden">
        <div
          className={
            mobileView === "viewer" ? "hidden lg:contents" : "contents"
          }
        >
          <EmailList
            onMenuClick={() => setMobileView("sidebar")}
            emails={emails}
            useThreads={useThreads}
            isLoading={isLoading}
            isError={isError}
            selectedEmailId={selectedEmailId}
            onSelectEmail={onSelectEmail}
            onToggleFlag={onToggleFlagList}
            onTogglePin={onTogglePinList}
            onSearchResult={onSearchResult}
            onFilterChange={onFilterChange}
            activeFolder={activeFolder}
            activeAccount={activeAccount}
            labels={accountLabels}
            emailLabelsMap={emailLabelsMap}
            hasNextPage={hasNextPage}
            fetchNextPage={fetchNextPage}
            accounts={accounts}
          />
        </div>

        <div
          className={`flex-1 flex flex-col min-w-0 bg-app-bg ${mobileView !== "viewer" ? "hidden lg:flex" : "flex"}`}
        >
          {isReplying && !replyTo ? (
            <ComposePanel
              isReplying={isReplying}
              setIsReplying={setIsReplying}
              replyTo={replyTo}
              initialComposeTo={composeTo}
              initialComposeCc={composeCc}
              initialComposeSubject={composeSubject}
              initialComposeBody={composeBody}
              initialFromIdentity={fromIdentity}
              accounts={accounts}
              identities={identities}
              selectedAccountId={activeAccount}
              onSend={onSendEmail}
              onSaveDraft={onSaveDraft}
              saveDraftPending={saveDraftPending}
              sendPending={sendPending}
              attachments={attachments}
              onRemoveAttachment={onRemoveAttachment}
              onFileAttachClick={() => fileInputRef.current?.click()}
            />
          ) : (
            <EmailViewer {...viewerProps} />
          )}
        </div>
      </div>

      {undoJob && (
        <div className="fixed bottom-4 right-4 z-100 flex items-center gap-3 rounded-lg bg-card border shadow-lg px-4 py-3 text-sm text-text-main">
          <span>{undoToastLabel}</span>
          <button
            className="text-amber-400 font-medium hover:text-amber-300"
            onClick={() => onUndoSend(undoJob)}
          >
            {undoButtonLabel}
          </button>
        </div>
      )}
      {isDesktop && commandPalette}
    </div>
  );
}
