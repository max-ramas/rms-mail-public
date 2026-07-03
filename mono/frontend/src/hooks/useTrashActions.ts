"use client";

import { useCallback } from "react";
import type { Folder } from "@/hooks/useEmails";
import { apiFetch } from "@/lib/api-client";

interface UseTrashActionsOptions {
  selectedEmailId: string | null;
  setSelectedEmailId: (id: string | null) => void;
  refetchEmails: () => void;
  refetchFolders: () => void;
  onRestored?: () => void;
  onRestoreFailed?: () => void;
}

export function useTrashActions({
  selectedEmailId,
  setSelectedEmailId,
  refetchEmails,
  refetchFolders,
  onRestored,
  onRestoreFailed,
}: UseTrashActionsOptions) {
  const handleRestoreFromTrash = useCallback(
    (emailId: string) => {
      apiFetch(`/api/emails/restore/${emailId}`, { method: "POST" })
        .then(() => {
          refetchEmails();
          if (selectedEmailId === emailId) setSelectedEmailId(null);
          onRestored?.();
        })
        .catch(() => onRestoreFailed?.());
    },
    [
      selectedEmailId,
      setSelectedEmailId,
      refetchEmails,
      onRestored,
      onRestoreFailed,
    ],
  );

  const emptyTrash = useCallback(async () => {
    const res = await apiFetch(`/api/folders`);
    const allFolders: Folder[] = await res.json();

    const trashNames = [
      "Trash",
      "TRASH",
      "trash",
      "Корзина",
      "Deleted Items",
    ];
    const trashes = allFolders.filter((f: Folder) =>
      trashNames.includes(f.name),
    );
    if (trashes.length === 0) return false;
    if (!confirm(`Empty ${trashes.length} trash folder(s)?`)) return false;

    for (const trash of trashes) {
      await apiFetch(`/api/folders/${trash.id}`, { method: "DELETE" });
    }
    refetchFolders();
    refetchEmails();
    return true;
  }, [refetchEmails, refetchFolders]);

  return { handleRestoreFromTrash, emptyTrash };
}
