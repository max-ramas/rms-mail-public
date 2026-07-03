"use client";

import { useEffect, type RefObject } from "react";
import type { Email } from "@/hooks/useEmails";
import { useBulkEmailAction } from "@/hooks/useEmailMutations";
import { isInsideModal } from "@/lib/query-cache";
import { resolveBulkSetFlagged } from "@/lib/bulk-flag";

export interface UseEmailListKeyboardOptions {
  processedEmails: Email[];
  selectedIds: Set<string>;
  selectAllActive: boolean;
  selectedEmailId: string | null;
  activeAccountRef: RefObject<string>;
  activeFolderRef: RefObject<string>;
  clearSelected: () => void;
  toggleSelected: (id: string) => void;
  onTogglePin?: (id: string) => void;
}

export function useEmailListKeyboard({
  processedEmails,
  selectedIds,
  selectAllActive,
  selectedEmailId,
  activeAccountRef,
  activeFolderRef,
  clearSelected,
  toggleSelected,
  onTogglePin,
}: UseEmailListKeyboardOptions) {
  const bulkAction = useBulkEmailAction();

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isInsideModal(e.target)) return;
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      const hasSelection = selectedIds.size > 0 || selectAllActive;
      const filterParams = selectAllActive
        ? {
            account_id: activeAccountRef.current,
            filter_folder_id: activeFolderRef.current,
            unified: activeAccountRef.current === "unified",
          }
        : undefined;

      if (
        e.code === "KeyE" ||
        e.key.toLowerCase() === "e" ||
        e.code === "KeyA" ||
        e.key.toLowerCase() === "a"
      ) {
        if (!e.metaKey && !e.ctrlKey && hasSelection) {
          e.stopImmediatePropagation();
          e.preventDefault();
          bulkAction.mutate({
            action: "archive",
            ids: filterParams ? [] : Array.from(selectedIds),
            filter: filterParams,
          });
          clearSelected();
        }
      }

      if (
        e.key === "Delete" ||
        e.key === "Backspace" ||
        e.code === "KeyD" ||
        e.key.toLowerCase() === "d" ||
        e.key === "#"
      ) {
        if (hasSelection) {
          e.stopImmediatePropagation();
          e.preventDefault();
          bulkAction.mutate({
            action: "delete",
            ids: filterParams ? [] : Array.from(selectedIds),
            filter: filterParams,
          });
          clearSelected();
        }
      }

      if (
        e.shiftKey &&
        (e.code === "KeyI" || e.key.toUpperCase() === "I") &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        hasSelection
      ) {
        e.stopImmediatePropagation();
        e.preventDefault();
        bulkAction.mutate({
          action: "read",
          ids: filterParams ? [] : Array.from(selectedIds),
          filter: filterParams,
        });
      }

      if (
        e.shiftKey &&
        (e.code === "KeyU" || e.key.toUpperCase() === "U") &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        hasSelection
      ) {
        e.stopImmediatePropagation();
        e.preventDefault();
        bulkAction.mutate({
          action: "unread",
          ids: filterParams ? [] : Array.from(selectedIds),
          filter: filterParams,
        });
      }

      if (
        e.shiftKey &&
        (e.code === "KeyF" || e.key.toUpperCase() === "F") &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        hasSelection
      ) {
        e.stopImmediatePropagation();
        e.preventDefault();
        bulkAction.mutate({
          action: "flag",
          ids: filterParams ? [] : Array.from(selectedIds),
          setFlagged: resolveBulkSetFlagged(
            processedEmails,
            selectedIds,
            selectAllActive,
          ),
          filter: filterParams,
        });
      }

      if (
        e.shiftKey &&
        (e.code === "KeyP" || e.key.toUpperCase() === "P") &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        selectedIds.size > 0 &&
        onTogglePin
      ) {
        e.stopImmediatePropagation();
        e.preventDefault();
        selectedIds.forEach((id) => onTogglePin(id));
      }

      if (
        e.key === " " &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        selectedEmailId
      ) {
        e.preventDefault();
        toggleSelected(selectedEmailId);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    processedEmails,
    selectedIds,
    selectAllActive,
    clearSelected,
    onTogglePin,
    selectedEmailId,
    toggleSelected,
    bulkAction,
    activeAccountRef,
    activeFolderRef,
  ]);
}
