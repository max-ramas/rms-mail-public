"use client";

import React from "react";
import { useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { useTranslations } from "next-intl";
import {
  useEmail,
  useUsers,
  useComments,
  useLabels,
  useGroups,
  useFolders,
} from "@/hooks/useEmailQueries";
import {
  useSummarizeEmail,
  useDeleteEmail,
  useCategorizeEmail,
  useSendEmail,
  useMoveEmailToFolder,
  useFlagEmail,
  usePinEmail,
  useMuteEmail,
  useSnoozeEmail,
  useAssignEmail,
  useUnassignEmail,
  useCreateComment,
  useDeleteComment,
  useClearDraftReply,
  useSetEmailLabels,
  useSaveDraft,
} from "@/hooks/useEmailMutations";
import { apiFetch, parseApiError } from "@/lib/api-client";
import { getAIConfig } from "@/lib/ai-config";
import { formatFileSize } from "@/lib/format-file-size";
import { useTrashActions } from "@/hooks/useTrashActions";
import { useMailInboxCommands } from "@/hooks/useMailInboxCommands";
import {
  useMailCompose,
  type UseMailComposeOptions,
} from "@/hooks/useMailCompose";
import { useMailInboxAI } from "@/hooks/useMailInboxAI";
import { useMailLabels } from "@/hooks/useMailLabels";
import { useAIChat } from "@/hooks/useAIApi";
import { type Email, type Folder } from "@/hooks/useEmailTypes";
import { useMailInboxState } from "@/hooks/useMailInboxState";
import { buildMailViewerProps } from "@/hooks/buildMailViewerProps";
import { getSavedMarkReadDelay } from "@/components/general-tab";
import { MailInboxLayout } from "@/components/mail-inbox-layout";
import { MoveToFolderDialog } from "@/components/move-to-folder-dialog";
import { useToast } from "@/hooks/useToast";
import dynamic from "next/dynamic";

const DynamicCommandPalette = dynamic(
  () =>
    import("@/components/command-palette").then((mod) => mod.CommandPalette),
  { ssr: false },
);

export function useMailInboxPage(locale: string) {
  const t = useTranslations("mail");
  const tc = useTranslations("commands");
  const queryClient = useQueryClient();
  const inbox = useMailInboxState();
  const {
    aiEnabled,
    activeAccount,
    setActiveAccount,
    activeFolder,
    setActiveFolder,
    showAccountFolders,
    setShowAccountFolders,
    selectedEmailId,
    setSelectedEmailId,
    searchResultEmails,
    setSearchResultEmails,
    mobileView,
    setMobileView,
    EMPTY_LABELS_MAP,
    setEmailFilters,
    useThreads,
    undoJob,
    setUndoJob,
    markEmailRead,
    bulkAction: bulkActionMutation,
    isDesktop,
    deselectedRef,
    foldersQuery,
    emailsQuery,
    emails,
    isLoading,
    isError,
    hasNextPage,
    fetchNextPage,
    accounts,
    displayedEmails,
    selectedEmail,
    batchLabels,
    identitiesQuery,
  } = inbox;

  const summarizeMutation = useSummarizeEmail();
  const categorizeMutation = useCategorizeEmail();
  const clearDraftMutation = useClearDraftReply();
  const saveDraftMutation = useSaveDraft();
  const sendEmailMutation = useSendEmail();
  const moveEmailMutation = useMoveEmailToFolder();
  const flagEmailMutation = useFlagEmail();
  const pinEmailMutation = usePinEmail();
  const muteEmailMutation = useMuteEmail();
  const snoozeEmailMutation = useSnoozeEmail();
  const deleteMutation = useDeleteEmail();
  const toast = useToast();

  const emailQuery = useEmail(selectedEmail?.id || null);

  const compose = useMailCompose({
    locale,
    activeAccount,
    accounts,
    identities: identitiesQuery.data,
    selectedEmail,
    saveDraftMutation,
    sendEmailMutation:
      sendEmailMutation as unknown as UseMailComposeOptions["sendEmailMutation"],
    toast,
    onUndoJobChange: setUndoJob,
  });

  const {
    fileInputRef,
    composeTo,
    setComposeTo,
    composeCc,
    setComposeCc,
    composeSubject,
    setComposeSubject,
    composeBody,
    setComposeBody,
    replyTo,
    setReplyTo,
    isReplying,
    setIsReplying,
    isForwarding,
    setIsForwarding,
    setDraftId,
    fromIdentity,
    setFromIdentity,
    attachments,
    setAttachments,
    forwardOriginalHtml,
    setForwardOriginalHtml,
    forwardMeta,
    setForwardMeta,
    getSignatureHtml,
    resetCompose,
    handleReply,
    handleReplyAll,
    handleReplyToEmail,
    handleSaveDraft,
    handleSendEmail,
  } = compose;

  const handleForward = React.useCallback(() => {
    compose.handleForward({
      html: emailQuery.data?.html,
      body: emailQuery.data?.body,
      attachments: emailQuery.data?.attachments,
    });
  }, [compose, emailQuery.data]);

  const handleReplyWithQuote = React.useCallback(
    (selectedText?: string) => {
      compose.handleReplyWithQuote(selectedText, {
        html: emailQuery.data?.html,
        body: emailQuery.data?.body,
      });
    },
    [compose, emailQuery.data],
  );

  const aiChatMutation = useAIChat();

  const ai = useMailInboxAI({
    locale,
    selectedEmail,
    emailBody: emailQuery.data?.body,
    emailSnippet: selectedEmail?.snippet,
    inboxPreview: emailsQuery.data?.pages?.[0]?.items,
    summarizeMutation,
    categorizeMutation,
    aiChatMutation,
    toast,
  });

  const {
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
  } = ai;

  const handleDeleteThreadEmail = React.useCallback(
    async (emailId: string) => {
      try {
        await deleteMutation.mutateAsync({
          emailId,
          accountId: (emails.find((e) => e.id === emailId) as Email)
            ?.account_id,
        });
        toast.addToast(t("toast_email_deleted"), "success");
      } catch {
        toast.addToast(t("toast_failed"), "error");
      }
    },
    [deleteMutation, toast, t, emails],
  );

  const handleSnooze = React.useCallback(() => {
    if (selectedEmail) {
      snoozeEmailMutation.mutate({ emailId: selectedEmail.id, minutes: 180 });
      toast.addToast(t("snoozed"), "success");
    }
  }, [selectedEmail, snoozeEmailMutation, toast, t]);

  const handleDeleteEmail = React.useCallback(async () => {
    const id = selectedEmailId;
    if (!id) return;

    const currentIndex = displayedEmails.findIndex((e) => e.id === id);
    let nextEmailId: string | null = null;
    if (currentIndex !== -1) {
      const nextEmail =
        displayedEmails[currentIndex + 1] || displayedEmails[currentIndex - 1];
      nextEmailId = nextEmail ? nextEmail.id : null;
    }

    try {
      await deleteMutation.mutateAsync({
        emailId: id,
        accountId: displayedEmails.find((e) => e.id === id)?.account_id,
      });
      setSelectedEmailId(nextEmailId);
      toast.addToast(t("toast_email_deleted"), "success");
    } catch (err) {
      console.error("handleDeleteEmail: mutation failed", err);
      const message = axios.isAxiosError(err)
        ? parseApiError(err.response?.data, err.message)
        : err instanceof Error
          ? err.message
          : "Failed to delete email";
      toast.addToast(message, "error");
    }
  }, [
    selectedEmailId,
    displayedEmails,
    deleteMutation,
    setSelectedEmailId,
    toast,
    t,
  ]);

  const { handleRestoreFromTrash, emptyTrash: onEmptyTrash } = useTrashActions({
    selectedEmailId,
    setSelectedEmailId,
    refetchEmails: () => {
      void emailsQuery.refetch();
    },
    refetchFolders: () => {
      void foldersQuery.refetch();
    },
    onRestored: () => toast.addToast(t("toast_restored"), "success"),
    onRestoreFailed: () => toast.addToast(t("toast_restore_failed"), "error"),
  });

  const emailTagsData = emailQuery.data?.tags || [];

  const groupsQuery = useGroups();
  const usersQuery = useUsers();
  const commentsQuery = useComments(selectedEmail?.id || null);
  const assignEmail = useAssignEmail();
  const unassignEmail = useUnassignEmail();
  const createComment = useCreateComment();
  const deleteComment = useDeleteComment();

  const handleEmailAction = React.useCallback(
    async (emailId: string, action: string) => {
      switch (action) {
        case "pin":
          pinEmailMutation.mutate(emailId);
          break;
        case "flag":
          flagEmailMutation.mutate(emailId);
          break;
        case "mute":
          muteEmailMutation.mutate(emailId);
          break;
        case "archive":
          moveEmailMutation.mutate({ emailId, folderId: "__archive__" });
          break;
        case "toggleRead": {
          const currentEmail = displayedEmails.find((e) => e.id === emailId);
          bulkActionMutation.mutate({
            action: currentEmail?.is_read ? "unread" : "read",
            ids: [emailId],
          });
          break;
        }
        case "snooze":
          snoozeEmailMutation.mutate({ emailId, minutes: 180 });
          break;
        case "summarize":
          summarizeMutation.mutate({
            emailId,
            aiConfig: getAIConfig("summarize"),
          });
          break;
        case "categorize":
          categorizeMutation.mutate({
            emailId,
            aiConfig: getAIConfig("categorize"),
          });
          break;
        case "translate":
          await handleTranslateEmail(emailId);
          break;
      }
    },
    [
      pinEmailMutation,
      flagEmailMutation,
      muteEmailMutation,
      moveEmailMutation,
      snoozeEmailMutation,
      summarizeMutation,
      categorizeMutation,
      handleTranslateEmail,
      bulkActionMutation,
      displayedEmails,
    ],
  );

  const labelsQuery = useLabels(selectedEmail?.account_id || "");
  const accountLabelsQuery = useLabels(
    activeAccount !== "unified" && !activeAccount?.startsWith("group:")
      ? activeAccount
      : undefined,
  );
  const setEmailLabels = useSetEmailLabels();
  const { displayLabelIds, handleToggleLabel, handleSetLabels } = useMailLabels(
    {
      selectedEmailId: selectedEmail?.id,
      batchLabels: batchLabels.data,
      setEmailLabels,
      selectedEmailAccountId: selectedEmail?.account_id,
    },
  );

  const markReadTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );
  const markReadIdRef = React.useRef<string | null>(null);
  React.useEffect(() => {
    const id = selectedEmail?.id;
    if (!id) {
      return;
    }
    if (selectedEmail.is_read !== false) {
      if (markReadIdRef.current === "__pending__") markReadIdRef.current = null;
      return;
    }
    if (
      markReadIdRef.current === id ||
      markReadIdRef.current === "__pending__"
    ) {
      return;
    }
    const delay = getSavedMarkReadDelay();
    markReadIdRef.current = id;
    if (markReadTimerRef.current) clearTimeout(markReadTimerRef.current);
    markReadTimerRef.current = setTimeout(() => {
      markReadIdRef.current = "__pending__";
      markEmailRead.mutate(id);
    }, delay);
    return () => {
      if (markReadTimerRef.current) clearTimeout(markReadTimerRef.current);
      if (markReadIdRef.current !== "__pending__") {
        markReadIdRef.current = null;
      }
    };
  }, [selectedEmail?.id, selectedEmail?.is_read, markEmailRead]);

  const handleArchive = React.useCallback(async () => {
    if (!selectedEmail?.id) return;

    const currentIndex = displayedEmails.findIndex(
      (e) => e.id === selectedEmail.id,
    );
    let nextEmailId: string | null = null;
    if (currentIndex !== -1) {
      const nextEmail =
        displayedEmails[currentIndex + 1] || displayedEmails[currentIndex - 1];
      nextEmailId = nextEmail ? nextEmail.id : null;
    }

    try {
      await moveEmailMutation.mutateAsync({
        emailId: selectedEmail.id,
        folderId: "__archive__",
      });
      setSelectedEmailId(nextEmailId);
      toast.addToast(t("toast_archived"), "success");
    } catch {
      toast.addToast(t("toast_failed"), "error");
    }
  }, [
    selectedEmail,
    displayedEmails,
    moveEmailMutation,
    toast,
    t,
    setSelectedEmailId,
  ]);

  const [moveDialogOpen, setMoveDialogOpen] = React.useState(false);

  const moveFoldersAccountId = React.useMemo(() => {
    if (selectedEmail?.account_id) return selectedEmail.account_id;
    if (activeAccount !== "unified" && !activeAccount.startsWith("group:")) {
      return activeAccount;
    }
    return "";
  }, [selectedEmail?.account_id, activeAccount]);

  const moveFoldersQuery = useFolders(moveFoldersAccountId);

  const resolveInboxFolderId = React.useCallback(
    (accountId: string) => {
      const folders = queryClient.getQueryData<Folder[]>([
        "folders",
        accountId,
      ]);
      return folders?.find((f) => f.name.toUpperCase() === "INBOX")?.id ?? "";
    },
    [queryClient],
  );

  const handleMoveToFolder = React.useCallback(
    async (folderId: string) => {
      if (!selectedEmailId) return;
      const currentIndex = displayedEmails.findIndex(
        (e) => e.id === selectedEmailId,
      );
      let nextEmailId: string | null = null;
      if (currentIndex !== -1) {
        const nextEmail =
          displayedEmails[currentIndex + 1] ||
          displayedEmails[currentIndex - 1];
        nextEmailId = nextEmail ? nextEmail.id : null;
      }
      try {
        await moveEmailMutation.mutateAsync({
          emailId: selectedEmailId,
          folderId,
        });
        setSelectedEmailId(nextEmailId);
        setMoveDialogOpen(false);
        toast.addToast(tc("toast_moved_success"), "success");
      } catch {
        toast.addToast(t("toast_failed"), "error");
      }
    },
    [
      selectedEmailId,
      displayedEmails,
      moveEmailMutation,
      toast,
      t,
      tc,
      setSelectedEmailId,
    ],
  );

  useMailInboxCommands({
    locale,
    aiEnabled,
    accounts,
    emails,
    searchResultEmails,
    selectedEmailId,
    isReplying,
    isForwarding,
    setSelectedEmailId,
    setSummary,
    setTranslation,
    setActiveAccount,
    setActiveFolder,
    setIsReplying,
    setIsForwarding,
    setComposeTo,
    setComposeCc,
    setComposeSubject,
    setComposeBody,
    setReplyTo,
    setForwardOriginalHtml,
    setForwardMeta,
    setAttachments,
    setDraftId,
    setFromIdentity,
    setChatOpen,
    deselectedRef,
    getSignatureHtml,
    handleArchive,
    handleReply,
    handleForward,
    handleDeleteEmail,
    handleSummarize,
    handleCategorize,
    handleSnooze,
    onOpenMoveDialog: () => setMoveDialogOpen(true),
    resolveInboxFolderId,
    flagEmailMutation,
    pinEmailMutation,
    markEmailRead,
    bulkAction: bulkActionMutation,
  });

  const handleAttachmentUpload = React.useCallback(
    async (files: FileList) => {
      const formData = new FormData();
      for (let i = 0; i < files.length; i++) {
        formData.append("files", files[i]);
      }
      try {
        const res = await apiFetch("/api/attachments/upload", {
          method: "POST",
          body: formData,
        });
        if (!res.ok) {
          let errMsg = `Upload failed with status ${res.status}`;
          try {
            const errJson = await res.json();
            if (errJson.error) errMsg = errJson.error;
          } catch {
            // response is not JSON
          }
          throw new Error(errMsg);
        }
        const uploaded = (await res.json()) || [];
        setAttachments((prev) => [...prev, ...uploaded]);
      } catch (err) {
        console.error("Upload failed", err);
        toast.addToast(
          err instanceof Error ? err.message : "File upload failed",
          "error",
        );
      }
    },
    [setAttachments, toast],
  );

  const handleSelectEmailList = React.useCallback(
    (id: string) => {
      setSelectedEmailId(id);
      setSummary(null);
      setTranslation(null);

      const email = (searchResultEmails ?? emails).find(
        (e: Email) => e.id === id,
      );

      if (email && email.status === "draft") {
        setIsReplying(true);
        setReplyTo(null);
        setDraftId(email.id);
        try {
          if (email.draft_reply) {
            const parsed = JSON.parse(email.draft_reply);
            setComposeTo(
              parsed.to
                ? parsed.to
                    .split(",")
                    .map((s: string) => s.trim())
                    .filter(Boolean)
                : [],
            );
            setComposeCc(
              parsed.cc
                ? parsed.cc
                    .split(",")
                    .map((s: string) => s.trim())
                    .filter(Boolean)
                : [],
            );
            setComposeSubject(parsed.subject || "");
            setComposeBody(parsed.html || parsed.body || "");
          } else {
            setComposeTo([]);
            setComposeCc([]);
            setComposeSubject(email.subject || "");
            setComposeBody(email.snippet || "");
          }
        } catch {
          setComposeTo([]);
          setComposeCc([]);
          setComposeSubject(email.subject || "");
          setComposeBody(email.snippet || "");
        }
      } else {
        setIsReplying(false);
        setDraftId(null);
      }

      // Only switch to viewer on mobile (<1024px) to avoid hiding the email list
      if (typeof window !== "undefined" && window.innerWidth < 1024) {
        setMobileView("viewer");
      }
    },
    // Setter refs from useState/useMailCompose are stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [searchResultEmails, emails],
  );

  const handleToggleFlagList = React.useCallback(
    (id: string) => flagEmailMutation.mutate(id),
    [flagEmailMutation],
  );

  const handleTogglePinList = React.useCallback(
    (id: string) => pinEmailMutation.mutate(id),
    [pinEmailMutation],
  );

  const handleUndoSend = React.useCallback(
    (jobId: string) => {
      apiFetch(`/api/emails/send/cancel/${jobId}`, { method: "POST" })
        .then(() => toast.addToast(t("actions.cancel"), "info"))
        .catch(() => toast.addToast("Error", "error"));
      setUndoJob(null);
    },
    [setUndoJob, toast, t],
  );

  const layoutElement = (
    <>
      <MailInboxLayout
        locale={locale}
        fileInputRef={fileInputRef}
        onAttachmentUpload={handleAttachmentUpload}
        activeAccount={activeAccount}
        setActiveAccount={setActiveAccount}
        activeFolder={activeFolder}
        setActiveFolder={setActiveFolder}
        accounts={accounts}
        groups={groupsQuery.data ?? []}
        folders={foldersQuery.data ?? []}
        showAccountFolders={showAccountFolders}
        setShowAccountFolders={setShowAccountFolders}
        mobileView={mobileView}
        setMobileView={setMobileView}
        onEmptyTrash={onEmptyTrash}
        emails={emails}
        useThreads={useThreads}
        isLoading={isLoading}
        isError={isError}
        selectedEmailId={selectedEmailId}
        onSelectEmail={handleSelectEmailList}
        onToggleFlagList={handleToggleFlagList}
        onTogglePinList={handleTogglePinList}
        onSearchResult={setSearchResultEmails}
        onFilterChange={setEmailFilters}
        accountLabels={accountLabelsQuery.data ?? []}
        emailLabelsMap={batchLabels.data ?? EMPTY_LABELS_MAP}
        hasNextPage={hasNextPage}
        fetchNextPage={fetchNextPage}
        isReplying={isReplying}
        replyTo={replyTo}
        composeTo={composeTo}
        composeCc={composeCc}
        composeSubject={composeSubject}
        composeBody={composeBody}
        fromIdentity={fromIdentity}
        identities={identitiesQuery.data ?? []}
        onSendEmail={handleSendEmail}
        onSaveDraft={handleSaveDraft}
        saveDraftPending={saveDraftMutation.isPending}
        sendPending={sendEmailMutation.isPending}
        attachments={attachments}
        onRemoveAttachment={(i) =>
          setAttachments((prev) => prev.filter((_, idx) => idx !== i))
        }
        setIsReplying={setIsReplying}
        undoJob={undoJob}
        undoToastLabel={t("undo.sending")}
        undoButtonLabel={t("undo.button")}
        onUndoSend={handleUndoSend}
        isDesktop={isDesktop}
        commandPalette={<DynamicCommandPalette context="inbox" />}
        viewerProps={buildMailViewerProps({
          aiEnabled,
          selectedEmailId,
          selectedEmail: selectedEmail ?? null,
          locale,
          summary,
          translation,
          onReply: handleReply,
          onForward: handleForward,
          onReplyAll: handleReplyAll,
          onReplyWithQuote: handleReplyWithQuote,
          onArchive: handleArchive,
          onDelete: handleDeleteEmail,
          onSummarize: handleSummarize,
          onCategorize: handleCategorize,
          onTranslate: handleTranslate,
          summarizePending: summarizeMutation.isPending,
          categorizePending: categorizeMutation.isPending,
          translatePending: aiChatMutation.isPending,
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
          composeAttachments: attachments,
          accounts,
          activeAccount,
          identities: identitiesQuery.data,
          onChangeComposeTo: setComposeTo,
          onChangeComposeCc: setComposeCc,
          onChangeComposeSubject: setComposeSubject,
          onChangeComposeBody: setComposeBody,
          onChangeFromIdentity: setFromIdentity,
          onCancelReply: resetCompose,
          onSendEmail: handleSendEmail,
          sendPending: sendEmailMutation.isPending,
          getSignatureHtml,
          emailHtml: emailQuery.data?.html,
          emailBody: emailQuery.data?.body,
          emailSnippet: selectedEmail?.snippet ?? "",
          emailLoading: emailQuery.isLoading,
          emailAttachments: emailQuery.data?.attachments,
          threadEmails: emailQuery.data?.thread_emails,
          draftReply: emailQuery.data?.email?.draft_reply,
          emailTagsData: emailTagsData,
          folders: foldersQuery.data ?? [],
          onSaveDraft: handleSaveDraft,
          saveDraftPending: saveDraftMutation.isPending,
          labelsData: labelsQuery.data,
          selectedLabelIds: displayLabelIds,
          onToggleLabel: handleToggleLabel,
          onSetLabels: handleSetLabels,
          usersData: usersQuery.data,
          comments: commentsQuery.data ?? [],
          chatOpen,
          onToggleChat: () => setChatOpen(!chatOpen),
          chatMessages,
          onChatSend: handleChatSend,
          chatPending: aiChatMutation.isPending,
          tags: emailTagsData,
          formatFileSize,
          onRestoreFromTrash: handleRestoreFromTrash,
          onReplyToEmail: handleReplyToEmail,
          onDeleteEmail: handleDeleteThreadEmail,
          onEmailAction: handleEmailAction,
          setMobileView,
          setTranslation,
          setIsReplying,
          setIsForwarding,
          setReplyTo,
          setDraftId,
          setComposeTo,
          setComposeCc,
          setComposeSubject,
          setComposeBody,
          setFromIdentity,
          setAttachments,
          moveEmailMutation,
          pinEmailMutation,
          flagEmailMutation,
          muteEmailMutation,
          clearDraftMutation,
          assignEmail,
          unassignEmail,
          createComment,
          deleteComment,
          bulkAction: {
            mutate: (args) =>
              bulkActionMutation.mutate(
                args as Parameters<typeof bulkActionMutation.mutate>[0],
              ),
          },
          usersQuery,
          displayedEmails,
          fileInputRef,
          handleSnooze,
        })}
      />
      <MoveToFolderDialog
        open={moveDialogOpen}
        onOpenChange={setMoveDialogOpen}
        folders={moveFoldersQuery.data ?? []}
        currentFolderId={selectedEmail?.folder_id}
        onConfirm={handleMoveToFolder}
      />
    </>
  );
  return { layout: layoutElement };
}
