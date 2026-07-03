"use client";

import { useState, useCallback, useEffect } from "react";
import { useCommandListener } from "@/lib/commandBus";
import axios from "axios";
import { API_BASE } from "@/hooks/useEmailTypes";
import type { Email } from "@/hooks/useEmails";

export interface UseEmailSelectionOptions {
  activeAccountRef: React.MutableRefObject<string>;
  activeFolderRef: React.MutableRefObject<string>;
}

export interface UseEmailSelectionReturn {
  selectedIds: Set<string>;
  selectAllActive: boolean;
  selectAllCount: number;
  toggleSelected: (id: string) => void;
  clearSelected: () => void;
  handleSelectAll: (processedEmails: Email[]) => void;
}

export function useEmailSelection({
  activeAccountRef,
  activeFolderRef,
}: UseEmailSelectionOptions): UseEmailSelectionReturn {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [selectAllActive, setSelectAllActive] = useState(false);
  const [selectAllCount, setSelectAllCount] = useState(0);

  // Listen for global deselect command (u key)
  useCommandListener("mail:deselect", () => {
    setSelectedIds(new Set());
    setSelectAllActive(false);
  });

  const toggleSelected = useCallback((id: string) => {
    setSelectAllActive(false);
    setSelectAllCount(0);
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const clearSelected = useCallback(() => {
    setSelectedIds(new Set());
    setSelectAllActive(false);
    setSelectAllCount(0);
  }, []);

  const handleSelectAll = useCallback(
    (processedEmails: Email[]) => {
      axios
        .get(`${API_BASE}/api/emails/count`, {
          params: {
            account_id: activeAccountRef.current,
            folder_id: activeFolderRef.current,
          },
        })
        .then((res) => {
          const count = res.data?.count ?? 0;
          setSelectAllActive(true);
          setSelectAllCount(count);
          setSelectedIds(new Set());
        })
        .catch((err) => {
          if (process.env.NODE_ENV === "development")
            console.error("Failed to fetch email count", err);
          setSelectedIds(new Set(processedEmails.map((e) => e.id)));
        });
    },
    [activeAccountRef, activeFolderRef],
  );

  // Listen for select-all via command bus bridge (Cmd+A shortcut)
  useEffect(() => {
    const handler = () => {
      if (selectedIds.size > 0 || selectAllActive) {
        clearSelected();
      } else {
        axios
          .get(`${API_BASE}/api/emails/count`, {
            params: {
              account_id: activeAccountRef.current,
              folder_id: activeFolderRef.current,
            },
          })
          .then((res) => {
            const count = res.data?.count ?? 0;
            setSelectAllActive(true);
            setSelectAllCount(count);
            setSelectedIds(new Set());
          })
          .catch(() => {
            // Fallback handled inline
          });
      }
    };
    window.addEventListener("email-list:select-all", handler);
    return () => window.removeEventListener("email-list:select-all", handler);
  }, [selectedIds.size, selectAllActive, clearSelected, activeAccountRef, activeFolderRef]);

  return {
    selectedIds,
    selectAllActive,
    selectAllCount,
    toggleSelected,
    clearSelected,
    handleSelectAll,
  };
}
