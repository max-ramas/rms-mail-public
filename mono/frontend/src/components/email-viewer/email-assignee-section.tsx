"use client";

import { useTranslations } from "next-intl";
import type { Email, User } from "@/hooks/useEmails";

interface EmailAssigneeSectionProps {
  selectedEmail: Email;
  usersData: User[] | undefined;
  onAssign: (emailId: string, userId: string) => void;
  onUnassign: (emailId: string) => void;
  onSetStatus?: (emailId: string, status: string) => void;
}

export function EmailAssigneeSection({
  selectedEmail,
  usersData,
  onAssign,
  onUnassign,
  onSetStatus,
}: EmailAssigneeSectionProps) {
  const t = useTranslations("mail");

  return (
    <div className="flex items-center gap-3">
      <span className="text-xs text-text-muted">{t("assigned")}:</span>
      <select
        className="rmsmail-select text-xs"
        value={selectedEmail.assigned_to || ""}
        onChange={(e) => {
          if (e.target.value) onAssign(selectedEmail.id, e.target.value);
          else onUnassign(selectedEmail.id);
        }}
      >
        <option value="">{t("unassigned")}</option>
        {(usersData || []).map((u) => (
          <option key={u.id} value={u.id}>
            {u.name}
          </option>
        ))}
      </select>
      {onSetStatus && selectedEmail.status && (
        <>
          <span className="text-xs text-text-muted">|</span>
          <select
            className="h-8 text-xs rounded border bg-background px-2"
            value={selectedEmail.status}
            onChange={(e) => onSetStatus(selectedEmail.id, e.target.value)}
          >
            <option value="new">{t("status_new")}</option>
            <option value="in_progress">{t("status_in_progress")}</option>
            <option value="resolved">{t("status_resolved")}</option>
            <option value="closed">{t("status_closed")}</option>
          </select>
        </>
      )}
    </div>
  );
}
