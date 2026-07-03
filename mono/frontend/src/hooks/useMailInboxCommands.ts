"use client";

import React from "react";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { useCommandListener } from "@/lib/commandBus";
import { editionLetter } from "@/hooks/useEmails";
import type { Email, Account } from "@/hooks/useEmailTypes";

export interface MailInboxCommandsParams {
  locale: string;
  aiEnabled: boolean;
  accounts: Account[];
  emails: Email[];
  searchResultEmails: Email[] | null;
  selectedEmailId: string | null;
  isReplying: boolean;
  isForwarding: boolean;
  setSelectedEmailId: (id: string | null) => void;
  setSummary: (v: string | null) => void;
  setTranslation: (v: string | null) => void;
  setActiveAccount: (id: string) => void;
  setActiveFolder: (id: string) => void;
  setIsReplying: (v: boolean) => void;
  setIsForwarding: (v: boolean) => void;
  setComposeTo: (v: string[]) => void;
  setComposeCc: (v: string[]) => void;
  setComposeSubject: (v: string) => void;
  setComposeBody: (v: string) => void;
  setReplyTo: (v: Email | null) => void;
  setForwardOriginalHtml: (v: string) => void;
  setForwardMeta: (
    v: {
      from: string;
      subject: string;
      date: string;
      to: string;
    } | null,
  ) => void;
  setAttachments: React.Dispatch<
    React.SetStateAction<Array<{ id: string; filename: string; size: number }>>
  >;
  setDraftId: (v: string | null) => void;
  setFromIdentity: (v: string) => void;
  setChatOpen: React.Dispatch<React.SetStateAction<boolean>>;
  deselectedRef: React.MutableRefObject<boolean>;
  getSignatureHtml: () => string;
  handleArchive: () => void;
  handleReply: () => void;
  handleForward: () => void;
  handleDeleteEmail: () => void;
  handleSummarize: () => void;
  handleCategorize: () => void;
  handleSnooze: () => void;
  onOpenMoveDialog: () => void;
  resolveInboxFolderId?: (accountId: string) => string;
  flagEmailMutation: { mutate: (id: string) => void };
  pinEmailMutation: { mutate: (id: string) => void };
  markEmailRead: { mutate: (id: string) => void };
  bulkAction: { mutate: (args: { action: "unread" | "read"; ids: string[] }) => void };
}

export function useMailInboxCommands({
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
  onOpenMoveDialog,
  resolveInboxFolderId,
  flagEmailMutation,
  pinEmailMutation,
  markEmailRead,
  bulkAction,
}: MailInboxCommandsParams) {
  const navRef = React.useRef({
    displayedEmails: searchResultEmails ?? emails,
    selectedEmailId,
  });
  React.useEffect(() => {
    navRef.current = {
      displayedEmails: searchResultEmails ?? emails,
      selectedEmailId,
    };
  }, [searchResultEmails, emails, selectedEmailId]);

  const shortcuts = React.useMemo(
    () => ({
      onNextEmail: () => {
        const { displayedEmails: list, selectedEmailId: selected } =
          navRef.current;
        if (list.length === 0) return;
        const currentIdx = selected
          ? list.findIndex((e) => e.id === selected)
          : -1;
        const nextIdx = Math.min(currentIdx + 1, list.length - 1);
        if (nextIdx !== currentIdx && list[nextIdx]) {
          setSelectedEmailId(list[nextIdx].id);
          setSummary(null);
          setTranslation(null);
        }
      },
      onPrevEmail: () => {
        const { displayedEmails: list, selectedEmailId: selected } =
          navRef.current;
        if (list.length === 0) return;
        const currentIdx = selected
          ? list.findIndex((e) => e.id === selected)
          : -1;
        const nextIdx = Math.max(currentIdx - 1, 0);
        if (nextIdx !== currentIdx && list[nextIdx]) {
          setSelectedEmailId(list[nextIdx].id);
          setSummary(null);
          setTranslation(null);
        }
      },
      onDelete: () => {
        void handleDeleteEmail();
      },
      onArchive: () => {
        const { selectedEmailId: selected } = navRef.current;
        if (selected) handleArchive();
      },
      onReply: () => {
        const { selectedEmailId: selected } = navRef.current;
        if (selected) handleReply();
      },
      onForward: () => {
        const { selectedEmailId: selected } = navRef.current;
        if (selected) handleForward();
      },
      onNewEmail: () => {
        setComposeTo([]);
        setComposeCc([]);
        setReplyTo(null);
        setForwardOriginalHtml("");
        setForwardMeta(null);
        setAttachments([]);
        setIsForwarding(false);
        setIsReplying(true);
        setComposeSubject("");
        setComposeBody("");
      },
      onEnterInbox: () => {
        if (editionLetter() === "M" && accounts.length > 0) {
          const accountId = accounts[0].id;
          setActiveAccount(accountId);
          setActiveFolder(resolveInboxFolderId?.(accountId) ?? "");
        } else {
          setActiveAccount("unified");
          setActiveFolder("");
        }
      },
    }),
    [
      handleDeleteEmail,
      handleArchive,
      handleReply,
      handleForward,
      accounts,
      setSelectedEmailId,
      setSummary,
      setTranslation,
      setComposeTo,
      setComposeCc,
      setReplyTo,
      setForwardOriginalHtml,
      setForwardMeta,
      setAttachments,
      setIsForwarding,
      setIsReplying,
      setComposeSubject,
      setComposeBody,
      setActiveAccount,
      setActiveFolder,
      resolveInboxFolderId,
    ],
  );

  useKeyboardShortcuts(shortcuts);

  useCommandListener("toggle:threads", () => {
    const current = localStorage.getItem("rms-mail_use_threads") !== "false";
    localStorage.setItem("rms-mail_use_threads", String(!current));
    window.dispatchEvent(new Event("rms-mail_settings_changed"));
  });

  useCommandListener("toggle:account-names", () => {
    const current =
      localStorage.getItem("rms-mail_show_account_name") === "true";
    localStorage.setItem("rms-mail_show_account_name", String(!current));
    window.dispatchEvent(new Event("rms-mail_settings_changed"));
  });

  useCommandListener("ai:summarize", () => {
    if (!aiEnabled) return;
    if (selectedEmailId) handleSummarize();
  });
  useCommandListener("ai:categorize", () => {
    if (!aiEnabled) return;
    if (selectedEmailId) handleCategorize();
  });
  useCommandListener("ai:draft", () => {
    if (!aiEnabled) return;
    if (selectedEmailId) setChatOpen(true);
  });
  useCommandListener("ai:chat", () => {
    if (!aiEnabled) return;
    setChatOpen(true);
  });
  useCommandListener("mail:toggle-flag", () => {
    if (selectedEmailId) flagEmailMutation.mutate(selectedEmailId);
  });
  useCommandListener("mail:pin", () => {
    if (selectedEmailId) pinEmailMutation.mutate(selectedEmailId);
  });
  useCommandListener("mail:move", () => {
    if (!selectedEmailId) return;
    onOpenMoveDialog();
  });

  useCommandListener("navigation:go-inbox", () => {
    if (editionLetter() === "M" && accounts.length > 0) {
      const accountId = accounts[0].id;
      setActiveAccount(accountId);
      setActiveFolder(resolveInboxFolderId?.(accountId) ?? "");
    } else {
      setActiveAccount("unified");
      setActiveFolder("");
    }
  });
  useCommandListener("navigation:go-settings", () => {
    window.location.href = `/${locale}/settings`;
  });
  useCommandListener("navigation:go-drafts", () => {
    setActiveFolder("__drafts__");
  });
  useCommandListener("navigation:go-sent", () => {
    setActiveFolder("__sent__");
  });
  useCommandListener("navigation:scroll-up", () => {
    window.scrollBy({ top: -300, behavior: "smooth" });
  });
  useCommandListener("navigation:scroll-down", () => {
    window.scrollBy({ top: 300, behavior: "smooth" });
  });

  useCommandListener("compose:send", () => {});
  useCommandListener("compose:discard", () => {
    setIsReplying(false);
    setIsForwarding(false);
    setForwardOriginalHtml("");
    setForwardMeta(null);
    setAttachments([]);
    setReplyTo(null);
    setDraftId(null);
  });

  useCommandListener("ui:dismiss", () => {
    if (document.body.hasAttribute("data-lightbox-open")) {
      window.dispatchEvent(new CustomEvent("lightbox:close"));
      return;
    }
    const searchInput = document.getElementById("search-input");
    if (searchInput && document.activeElement === searchInput) {
      searchInput.blur();
      return;
    }
    if (document.querySelector('[role="dialog"][aria-modal="true"]')) {
      return;
    }
    if (isReplying || isForwarding) {
      setIsReplying(false);
      setIsForwarding(false);
      setForwardOriginalHtml("");
      setForwardMeta(null);
      setAttachments([]);
      setReplyTo(null);
      setDraftId(null);
    } else if (selectedEmailId) {
      setSelectedEmailId(null);
    }
  });

  useCommandListener("mail:archive", handleArchive);
  useCommandListener("mail:delete", handleDeleteEmail);
  useCommandListener("mail:reply", handleReply);
  useCommandListener("mail:forward", handleForward);
  useCommandListener("mail:new-email", () => {
    setReplyTo(null);
    setComposeTo([]);
    setComposeSubject("");
    setComposeCc([]);
    setComposeBody(getSignatureHtml());
    setFromIdentity("");
    setForwardOriginalHtml("");
    setForwardMeta(null);
    setAttachments([]);
    setDraftId(null);
    setIsReplying(true);
    setIsForwarding(false);
  });
  useCommandListener("mail:mark-read", () => {
    if (selectedEmailId) markEmailRead.mutate(selectedEmailId);
  });
  useCommandListener("mail:mark-unread", () => {
    if (selectedEmailId) {
      bulkAction.mutate({ action: "unread", ids: [selectedEmailId] });
    }
  });
  useCommandListener("mail:deselect", () => {
    deselectedRef.current = true;
  });
  useCommandListener("mail:snooze", handleSnooze);
  useCommandListener("mail:select-all", () => {
    window.dispatchEvent(new CustomEvent("email-list:select-all"));
  });
}
