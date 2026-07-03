"use client";

import React, { useRef, useCallback, useEffect } from "react";
import { useTranslations } from "next-intl";
import { useCommandListener } from "@/lib/commandBus";
import type { Email, Label, Account } from "@/hooks/useEmails";
import { useBulkEmailAction } from "@/hooks/useEmailMutations";
import EmailFilters from "@/components/EmailFilters";
import EmailToolbar from "@/components/EmailToolbar";
import VirtualEmailList from "@/components/VirtualEmailList";
import { useEmailSelection } from "@/components/useEmailSelection";
import { useEmailListFilters } from "@/hooks/useEmailListFilters";
import { useEmailListKeyboard } from "@/hooks/useEmailListKeyboard";
import { useEmailListVirtualizer } from "@/hooks/useEmailListVirtualizer";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { setBulkSelectionActive } from "@/lib/bulk-selection-guard";
import { resolveBulkSetFlagged } from "@/lib/bulk-flag";

export interface GroupedEmail extends Email {
  thread_count?: number;
}

interface EmailListProps {
  emails: Email[];
  isLoading: boolean;
  isError?: boolean;
  selectedEmailId: string | null;
  onSelectEmail: (id: string) => void;
  onToggleFlag: (id: string) => void;
  onTogglePin?: (id: string) => void;
  onSearchResult: (emails: Email[]) => void;
  onFilterChange?: (filters: Record<string, string>) => void;
  activeFolder: string;
  activeAccount: string;
  labels: Label[];
  emailLabelsMap: Record<string, Label[]>;
  hasNextPage?: boolean;
  fetchNextPage?: () => void;
  showUnassignedOnly?: boolean;
  accounts?: Account[];
  useThreads?: boolean;
  onMenuClick?: () => void;
}

export const EmailList = React.memo(function EmailList({
  emails,
  isLoading,
  isError = false,
  selectedEmailId,
  onSelectEmail,
  onToggleFlag,
  onTogglePin,
  onSearchResult,
  onFilterChange,
  activeFolder,
  activeAccount: _activeAccount,
  labels,
  emailLabelsMap,
  hasNextPage,
  fetchNextPage,
  showUnassignedOnly,
  accounts: _accounts = [],
  useThreads = true,
  onMenuClick,
}: EmailListProps) {
  const t = useTranslations("mail");
  const bulkAction = useBulkEmailAction();
  const isDesktop = useMediaQuery("(min-width: 1024px)");
  const swipeEnabled = !isDesktop;

  const activeAccountRef = useRef(_activeAccount);
  activeAccountRef.current = _activeAccount;
  const activeFolderRef = useRef(activeFolder);
  activeFolderRef.current = activeFolder;

  const {
    selectedIds,
    selectAllActive,
    selectAllCount,
    toggleSelected,
    clearSelected,
    handleSelectAll,
  } = useEmailSelection({ activeAccountRef, activeFolderRef });

  useEffect(() => {
    queueMicrotask(() => setBulkSelectionActive(selectedIds.size > 0 || selectAllActive));
    return () => setBulkSelectionActive(false);
  }, [selectedIds, selectAllActive]);

  const {
    searchQuery,
    setSearchQuery,
    filterUnread,
    setFilterUnread,
    filterAttachments,
    setFilterAttachments,
    filterFlagged,
    setFilterFlagged,
    filterLabel,
    setFilterLabel,
    filterTag,
    setFilterTag,
    unreadCountBadge,
    flaggedCountBadge,
    attachmentsCountBadge,
    aiCategories,
    processedEmails,
  } = useEmailListFilters({
    emails,
    emailLabelsMap,
    showUnassignedOnly,
    useThreads,
    activeAccount: _activeAccount,
    activeFolder,
    onFilterChange,
    onSearchResult,
  });

  const emailIds = React.useMemo(
    () => processedEmails.map((e) => e.id),
    [processedEmails],
  );

  const { listContainerRef, loadMoreRef, rowVirtualizer } =
    useEmailListVirtualizer(
      processedEmails.length,
      selectedEmailId,
      emailIds,
    );

  useCommandListener("navigation:focus-search", () => {
    setTimeout(() => {
      const input = document.getElementById(
        "search-input",
      ) as HTMLInputElement | null;
      if (input) {
        input.focus();
        input.value = "";
      }
    }, 0);
  });

  const handleBulkAction = useCallback(
    (action: "flag" | "read" | "unread" | "delete") => {
      const setFlagged =
        action === "flag"
          ? resolveBulkSetFlagged(emails, selectedIds, selectAllActive)
          : undefined;
      if (selectAllActive) {
        bulkAction.mutate({
          action,
          ids: [],
          setFlagged,
          filter: {
            account_id: activeAccountRef.current,
            filter_folder_id: activeFolderRef.current,
            unified: activeAccountRef.current === "unified",
          },
        });
      } else {
        bulkAction.mutate({
          action,
          ids: Array.from(selectedIds),
          setFlagged,
        });
      }
      clearSelected();
    },
    [selectAllActive, selectedIds, bulkAction, clearSelected, emails],
  );

  const handleSwipeAction = useCallback(
    (action: "delete" | "toggle_read", id: string) => {
      if (action === "delete") {
        bulkAction.mutate({ action: "delete", ids: [id] });
      } else {
        const email = emails.find((e) => e.id === id);
        bulkAction.mutate({
          action: email?.is_read ? "unread" : "read",
          ids: [id],
        });
      }
    },
    [bulkAction, emails],
  );

  useEmailListKeyboard({
    processedEmails,
    selectedIds,
    selectAllActive,
    selectedEmailId,
    activeAccountRef,
    activeFolderRef,
    clearSelected,
    toggleSelected,
    onTogglePin,
  });

  return (
    <div className="w-96 max-lg:w-full h-full min-h-0 shrink-0 flex-col flex border-e border-border-muted/50 bg-list-bg">
      <EmailFilters
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        filterUnread={filterUnread}
        onFilterUnreadChange={setFilterUnread}
        filterAttachments={filterAttachments}
        onFilterAttachmentsChange={setFilterAttachments}
        filterFlagged={filterFlagged}
        onFilterFlaggedChange={setFilterFlagged}
        filterLabel={filterLabel}
        onFilterLabelChange={setFilterLabel}
        filterTag={filterTag}
        onFilterTagChange={setFilterTag}
        labels={labels}
        aiCategories={aiCategories}
        unreadCountBadge={unreadCountBadge}
        flaggedCountBadge={flaggedCountBadge}
        attachmentsCountBadge={attachmentsCountBadge}
        t={t}
        selectedIds={selectedIds}
        selectAllActive={selectAllActive}
        onClearSelected={clearSelected}
        onSelectAll={() => handleSelectAll(processedEmails)}
        onMenuClick={onMenuClick}
      />

      <EmailToolbar
        selectedIds={selectedIds}
        selectAllActive={selectAllActive}
        selectAllCount={selectAllCount}
        t={t}
        onClearSelected={clearSelected}
        onBulkAction={handleBulkAction}
      />

      <VirtualEmailList
        emails={processedEmails}
        rowVirtualizer={rowVirtualizer}
        selectedEmailId={selectedEmailId}
        selectedIds={selectedIds}
        selectAllActive={selectAllActive}
        onSelectEmail={onSelectEmail}
        onToggleFlag={onToggleFlag}
        t={t}
        toggleSelected={toggleSelected}
        accounts={_accounts}
        emailLabelsMap={emailLabelsMap}
        hasNextPage={hasNextPage}
        fetchNextPage={fetchNextPage}
        isLoading={isLoading}
        isError={isError}
        swipeEnabled={swipeEnabled}
        onSwipeAction={handleSwipeAction}
        listContainerRef={listContainerRef}
        loadMoreRef={loadMoreRef}
      />
    </div>
  );
});
