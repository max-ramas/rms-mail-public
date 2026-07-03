"use client";

import React, { useEffect, useCallback } from "react";
import type { Virtualizer } from "@tanstack/react-virtual";
import EmailRow, { ROW_HEIGHT } from "@/components/EmailRow";
import type { Label, Account } from "@/hooks/useEmails";
import type { GroupedEmail } from "@/components/email-list";

const EMPTY_LABELS: Label[] = [];

export interface VirtualEmailListProps {
  emails: GroupedEmail[];
  rowVirtualizer: Virtualizer<HTMLDivElement, Element>;
  selectedEmailId: string | null;
  selectedIds: Set<string>;
  selectAllActive: boolean;
  onSelectEmail: (id: string) => void;
  onToggleFlag: (id: string) => void;
  t: (key: string, values?: Record<string, string | number>) => string;
  toggleSelected: (id: string) => void;
  accounts: Account[];
  emailLabelsMap: Record<string, Label[]>;
  hasNextPage?: boolean;
  fetchNextPage?: () => void;
  isLoading: boolean;
  isError?: boolean;
  swipeEnabled?: boolean;
  onSwipeAction: (action: "delete" | "toggle_read", id: string) => void;
  listContainerRef: React.RefObject<HTMLDivElement | null>;
  loadMoreRef: React.RefObject<HTMLDivElement | null>;
}

const VirtualEmailList = React.memo(function VirtualEmailList({
  emails,
  rowVirtualizer,
  selectedEmailId,
  selectedIds,
  selectAllActive,
  onSelectEmail,
  onToggleFlag,
  t,
  toggleSelected,
  accounts,
  emailLabelsMap,
  hasNextPage,
  fetchNextPage,
  isLoading,
  isError = false,
  swipeEnabled = false,
  onSwipeAction,
  listContainerRef,
  loadMoreRef,
}: VirtualEmailListProps) {
  const handleDragStart = useCallback(
    (e: React.DragEvent, emailId: string) => {
      e.dataTransfer.effectAllowed = "move";
      const payload = selectedIds.has(emailId)
        ? Array.from(selectedIds)
        : [emailId];
      e.dataTransfer.setData(
        "application/rms-email-ids",
        JSON.stringify(payload),
      );
      e.dataTransfer.setData("text/plain", emailId);
      if (listContainerRef.current) {
        listContainerRef.current.style.overflow = "hidden";
      }
    },
    [selectedIds, listContainerRef],
  );

  const handleDragEnd = useCallback(() => {
    if (listContainerRef.current) {
      listContainerRef.current.style.overflow = "";
    }
  }, [listContainerRef]);

  // Load more (infinite scroll) observer
  useEffect(() => {
    if (!fetchNextPage || !hasNextPage) return;
    if (!loadMoreRef.current) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          setTimeout(() => fetchNextPage(), 0);
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [hasNextPage, fetchNextPage, emails.length, loadMoreRef]);

  // Loading state
  if (isLoading) {
    return (
      <div className="p-8 text-center text-text-muted text-sm">
        {t("loading")}
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-8 text-center text-destructive text-sm">
        {t("toast_failed")}
      </div>
    );
  }

  // Empty state
  if (emails.length === 0) {
    return (
      <div className="p-8 text-center text-text-muted text-sm">
        {t("search.no_results")}
      </div>
    );
  }

  return (
    <div
      ref={listContainerRef}
      data-email-list-scroll
      className="flex-1 min-h-0 overflow-y-auto"
    >
      <div
        style={{
          height: `${rowVirtualizer.getTotalSize()}px`,
          width: "100%",
          position: "relative",
        }}
      >
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const email = emails[virtualRow.index];
          return (
            <div
              key={virtualRow.key}
              data-index={virtualRow.index}
              ref={rowVirtualizer.measureElement}
              style={{
                position: "absolute",
                top: virtualRow.start,
                left: 0,
                width: "100%",
                minHeight: virtualRow.size,
              }}
            >
              <EmailRow
                email={email}
                selectedEmailId={selectedEmailId}
                selectedIds={selectedIds}
                selectAllActive={selectAllActive}
                onSelectEmail={onSelectEmail}
                onToggleFlag={onToggleFlag}
                labels={emailLabelsMap[email.id] ?? EMPTY_LABELS}
                t={t}
                toggleSelected={toggleSelected}
                accounts={accounts}
                swipeEnabled={swipeEnabled}
                dragEnabled={!swipeEnabled}
                onDragStartEmail={handleDragStart}
                onDragEndEmail={handleDragEnd}
                onSwipeAction={onSwipeAction}
              />
            </div>
          );
        })}
      </div>
      {hasNextPage && (
        <div
          ref={loadMoreRef}
          className="h-16 flex items-center justify-center text-text-muted text-sm border-t border-border-muted/20"
        >
          <div className="w-4 h-4 border-2 border-primary border-t-transparent rounded-full animate-spin me-2" />
          {t("loading")}...
        </div>
      )}
    </div>
  );
});

export default VirtualEmailList;
export { ROW_HEIGHT };
