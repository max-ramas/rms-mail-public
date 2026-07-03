"use client";

import React from "react";
import { Mail, MailOpen } from "lucide-react";

export interface EmailToolbarProps {
  selectedIds: Set<string>;
  selectAllActive: boolean;
  selectAllCount: number;
  t: (key: string, values?: Record<string, string | number>) => string;
  onClearSelected: () => void;
  onBulkAction: (action: "flag" | "read" | "unread" | "delete") => void;
}

const EmailToolbar = React.memo(function EmailToolbar({
  selectedIds,
  selectAllActive,
  selectAllCount,
  t,
  onClearSelected,
  onBulkAction,
}: EmailToolbarProps) {
  const hasSelection = selectedIds.size > 0 || selectAllActive;

  if (!hasSelection) return null;

  return (
    <div className="sticky top-0 z-10 bg-card/95 backdrop-blur border-b border-border-muted/50 px-4 py-2">
      <div className="flex items-center gap-3 mb-1.5">
        <span className="text-sm text-text-main/80">
          {selectAllActive
            ? `${selectAllCount} ${t("selected")}`
            : `${selectedIds.size} ${t("selected")}`}
        </span>
        <div className="flex-1" />
      </div>
      <div className="flex gap-1.5 flex-wrap">
        <button
          className="text-[11px] bg-muted hover:bg-muted/80 px-2.5 py-1 rounded text-text-main/80"
          onClick={onClearSelected}
        >
          {t("bulk_clear_selection")}
        </button>
        <button
          className="text-[11px] bg-muted hover:bg-muted/80 px-2.5 py-1 rounded text-text-main/80"
          onClick={() => onBulkAction("flag")}
        >
          {t("bulk_toggle_flag")}
        </button>
        <button
          className="text-[11px] bg-muted hover:bg-muted/80 px-2.5 py-1 rounded text-text-main/80 flex items-center gap-1"
          onClick={() => onBulkAction("read")}
          title={t("mark_read")}
        >
          <MailOpen size={14} />
          {t("mark_read")}
        </button>
        <button
          className="text-[11px] bg-muted hover:bg-muted/80 px-2.5 py-1 rounded text-text-main/80 flex items-center gap-1"
          onClick={() => onBulkAction("unread")}
          title={t("unread")}
        >
          <Mail size={14} />
          {t("unread")}
        </button>
        <button
          className="text-[11px] bg-muted hover:bg-muted/80 px-2.5 py-1 rounded text-text-main/80"
          onClick={() => onBulkAction("delete")}
        >
          {t("actions.delete")}
        </button>
      </div>
    </div>
  );
});

export default EmailToolbar;
