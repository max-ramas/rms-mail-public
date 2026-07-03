"use client";

import React from "react";
import { useTranslations } from "next-intl";
import type { Folder } from "@/hooks/useEmailTypes";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface MoveToFolderDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  folders: Folder[];
  currentFolderId?: string;
  onConfirm: (folderId: string) => void;
}

export function MoveToFolderDialog({
  open,
  onOpenChange,
  folders,
  currentFolderId,
  onConfirm,
}: MoveToFolderDialogProps) {
  const t = useTranslations("mail");
  const tc = useTranslations("commands");

  const availableFolders = React.useMemo(
    () => folders.filter((f) => f.id !== currentFolderId),
    [folders, currentFolderId],
  );

  const foldersKey = availableFolders.map((f) => f.id).join(",");
  const [selectedFolderId, setSelectedFolderId] = React.useState("");
  const [dialogSync, setDialogSync] = React.useState({
    open: false,
    foldersKey: "",
  });
  const listRef = React.useRef<HTMLDivElement>(null);

  if (
    open &&
    (dialogSync.open !== open || dialogSync.foldersKey !== foldersKey)
  ) {
    setDialogSync({ open: true, foldersKey });
    setSelectedFolderId(availableFolders[0]?.id ?? "");
  } else if (!open && dialogSync.open) {
    setDialogSync({ open: false, foldersKey: "" });
  }

  React.useEffect(() => {
    if (!open) return;
    const timeout = setTimeout(() => listRef.current?.focus(), 50);
    return () => clearTimeout(timeout);
  }, [open, foldersKey]);

  const handleConfirm = () => {
    if (!selectedFolderId) return;
    onConfirm(selectedFolderId);
  };

  const moveSelection = (delta: number) => {
    const idx = availableFolders.findIndex((f) => f.id === selectedFolderId);
    const nextIdx = Math.min(
      Math.max(idx + delta, 0),
      availableFolders.length - 1,
    );
    const next = availableFolders[nextIdx];
    if (next) setSelectedFolderId(next.id);
  };

  const handleListKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      e.stopPropagation();
      moveSelection(1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      e.stopPropagation();
      moveSelection(-1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      handleConfirm();
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{tc("move_prompt")}</DialogTitle>
        </DialogHeader>

        {availableFolders.length === 0 ? (
          <p className="text-sm text-muted-foreground">{tc("no_results")}</p>
        ) : (
          <div
            ref={listRef}
            role="listbox"
            tabIndex={0}
            aria-label={tc("move_prompt")}
            aria-activedescendant={
              selectedFolderId ? `move-folder-${selectedFolderId}` : undefined
            }
            className="max-h-56 overflow-y-auto rounded-md border border-input bg-card-bg shadow-sm outline-none focus-visible:ring-1 focus-visible:ring-ring"
            onKeyDown={handleListKeyDown}
          >
            {availableFolders.map((folder) => {
              const selected = folder.id === selectedFolderId;
              return (
                <button
                  key={folder.id}
                  id={`move-folder-${folder.id}`}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  className={cn(
                    "flex w-full cursor-default items-center px-3 py-2 text-left text-sm text-foreground outline-none hover:bg-accent hover:text-accent-foreground",
                    selected && "bg-accent text-accent-foreground",
                  )}
                  onClick={() => setSelectedFolderId(folder.id)}
                  onDoubleClick={handleConfirm}
                >
                  {folder.name}
                </button>
              );
            })}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("actions.cancel")}
          </Button>
          <Button onClick={handleConfirm} disabled={!selectedFolderId}>
            {tc("mail_move")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
