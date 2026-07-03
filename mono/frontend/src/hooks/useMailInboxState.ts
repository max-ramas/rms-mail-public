"use client";

import React, { useState, useMemo, useEffect, useRef } from "react";
import {
  useEmailsInfinite,
  useAccounts,
  useFolders,
  useIdentities,
  useBatchEmailLabels,
} from "@/hooks/useEmailQueries";
import {
  useMarkEmailRead,
  useBulkEmailAction,
} from "@/hooks/useEmailMutations";
import { isAIDisabled } from "@/hooks/useEmails";
import { type Email, type Folder } from "@/hooks/useEmailTypes";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useQueryClient } from "@tanstack/react-query";

export function useMailInboxState() {
  const queryClient = useQueryClient();
  const aiEnabled = !isAIDisabled();
  const [activeAccount, setActiveAccount] = useState("unified");
  const [activeFolder, setActiveFolder] = useState("");
  const [showAccountFolders, setShowAccountFolders] = useState(false);
  const [selectedEmailId, setSelectedEmailId] = useState<string | null>(null);
  const [searchResultEmails, setSearchResultEmails] = useState<Email[] | null>(
    null,
  );
  const [mobileView, setMobileView] = useState<"sidebar" | "list" | "viewer">(
    "list",
  );
  const EMPTY_LABELS_MAP = useMemo(() => ({}), []);
  const [emailFilters, setEmailFilters] = useState<Record<string, string>>({});
  const [useThreads, setUseThreads] = useState(true);
  const [undoJob, setUndoJob] = useState<string | null>(null);
  const [warmupExpired, setWarmupExpired] = useState(false);

  const accountsQuery = useAccounts();
  const markEmailRead = useMarkEmailRead();
  const bulkAction = useBulkEmailAction();
  const isDesktop = useMediaQuery("(min-width: 1024px)");
  const prevKeyRef = useRef("");
  const deselectedRef = useRef(false);

  const foldersQuery = useFolders(
    activeAccount !== "unified" && !activeAccount.startsWith("group:")
      ? activeAccount
      : "",
  );

  const getFolderName = (folder: string): string | undefined => {
    if (!folder) return undefined;
    if (folder.startsWith("__")) {
      return folder
        .replace(/^__(.*)__$/, "$1")
        .replace(/\b\w/g, (c) => c.toUpperCase());
    }
    const f = foldersQuery.data?.find((f: Folder) => f.id === folder);
    return f?.name;
  };

  const emailsQuery = useEmailsInfinite(
    activeAccount === "unified",
    activeAccount !== "unified" && !activeAccount.startsWith("group:")
      ? activeAccount
      : undefined,
    activeFolder && !activeFolder.startsWith("__") ? activeFolder : undefined,
    activeAccount === "unified" || activeAccount.startsWith("group:")
      ? getFolderName(activeFolder)
      : undefined,
    activeAccount.startsWith("group:")
      ? activeAccount.replace("group:", "")
      : undefined,
    emailFilters,
    { warmupExpired },
  );

  const emails = useMemo(() => {
    const allEmails =
      emailsQuery.data?.pages?.flatMap((page) => page.items ?? []) ?? [];
    const seen = new Set<string>();
    return allEmails.filter((email) => {
      if (seen.has(email.id)) return false;
      seen.add(email.id);
      return true;
    });
  }, [emailsQuery.data]);

  useEffect(() => {
    const timer = setTimeout(() => setWarmupExpired(true), 120_000);
    return () => clearTimeout(timer);
  }, []);

  const accounts = useMemo(
    () => accountsQuery?.data ?? [],
    [accountsQuery?.data],
  );

  useEffect(() => {
    const onPageShow = (event: PageTransitionEvent) => {
      if (!event.persisted) return;
      void queryClient.refetchQueries({ queryKey: ["emails-infinite"] });
      void queryClient.refetchQueries({ queryKey: ["folders"] });
    };
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, [queryClient]);

  useEffect(() => {
    if (
      typeof Notification !== "undefined" &&
      Notification.permission === "default"
    ) {
      Notification.requestPermission();
    }
  }, []);

  useEffect(() => {
    const handleStorage = () => {
      setUseThreads(localStorage.getItem("rms-mail_use_threads") !== "false");
    };
    handleStorage();
    window.addEventListener("storage", handleStorage);
    window.addEventListener("rms-mail_settings_changed", handleStorage);
    return () => {
      window.removeEventListener("storage", handleStorage);
      window.removeEventListener("rms-mail_settings_changed", handleStorage);
    };
  }, []);

  React.useEffect(() => {
    const key = `${activeFolder}-${activeAccount}`;
    if (key !== prevKeyRef.current) {
      prevKeyRef.current = key;
      deselectedRef.current = false;
      React.startTransition(() => {
        if (emails.length > 0) {
          setSelectedEmailId(emails[0].id);
        } else {
          setSelectedEmailId(null);
        }
      });
    } else if (
      !selectedEmailId &&
      emails.length > 0 &&
      !deselectedRef.current
    ) {
      React.startTransition(() => {
        setSelectedEmailId(emails[0].id);
      });
    }
  }, [emails, activeFolder, activeAccount, selectedEmailId]);

  const displayedEmails = searchResultEmails ?? emails;

  const foldersData = foldersQuery?.data;
  const foldersLength = foldersData?.length ?? 0;
  const folderSyncKey = `${activeAccount}-${foldersLength}-${activeFolder}`;
  const [syncedFolderKey, setSyncedFolderKey] = useState("");

  if (folderSyncKey !== syncedFolderKey) {
    setSyncedFolderKey(folderSyncKey);
    if (
      activeAccount !== "unified" &&
      foldersLength > 0 &&
      !activeFolder
    ) {
      const inboxFolder = foldersData!.find(
        (f: Folder) => f.name === "INBOX" || f.name.toUpperCase() === "INBOX",
      );
      if (inboxFolder) {
        setActiveFolder(inboxFolder.id);
      }
    }
  }

  const selectedEmail = selectedEmailId
    ? displayedEmails.find((e) => e.id === selectedEmailId)
    : displayedEmails?.[0];

  const visibleEmailIds = useMemo(() => emails.map((m) => m.id), [emails]);
  const batchLabels = useBatchEmailLabels(visibleEmailIds);

  const identitiesQuery = useIdentities(
    activeAccount !== "unified" && !activeAccount.startsWith("group:")
      ? activeAccount
      : undefined,
  );

  return {
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
    emailFilters,
    setEmailFilters,
    useThreads,
    undoJob,
    setUndoJob,
    accountsQuery,
    markEmailRead,
    bulkAction,
    isDesktop,
    deselectedRef,
    foldersQuery,
    emailsQuery,
    emails,
    isLoading: emailsQuery.isLoading,
    isError: emailsQuery.isError,
    hasNextPage: emailsQuery.hasNextPage,
    fetchNextPage: emailsQuery.fetchNextPage,
    accounts,
    displayedEmails,
    selectedEmail,
    batchLabels,
    identitiesQuery,
  };
}
