"use client";

import React, { useState } from "react";
import { useLabels } from "@/hooks/useEmailQueries";
import { useCreateLabel, useUpdateLabel, useDeleteLabel } from "@/hooks/useAdminQueries";
import { editionLetter } from "@/hooks/useEmails";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const COLORS = [
  "#ef4444",
  "#f97316",
  "#f59e0b",
  "#eab308",
  "#84cc16",
  "#22c55e",
  "#10b981",
  "#14b8a6",
  "#06b6d4",
  "#0ea5e9",
  "#3b82f6",
  "#6366f1",
  "#8b5cf6",
  "#a855f7",
  "#d946ef",
  "#ec4899",
  "#f43f5e",
  "#78716c",
  "#6b7280",
  "#475569",
];

export function LabelManager({ accountId }: { accountId: string }) {
  const t = useTranslations("settings");
  const isMono = editionLetter().startsWith("M");
  // M: per-account labels; U/T: shared labels across all mailboxes ("unified").
  const labelsQueryAccountId = isMono ? accountId : undefined;
  const createAccountId = isMono ? accountId : "unified";
  const { data: labels, isLoading } = useLabels(labelsQueryAccountId);
  const createLabel = useCreateLabel();
  const updateLabel = useUpdateLabel();
  const deleteLabel = useDeleteLabel();
  const [name, setName] = useState("");
  const [color, setColor] = useState(COLORS[0]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);

  const handleSave = async () => {
    if (!name.trim()) return;
    try {
      if (editingId) {
        await updateLabel.mutateAsync({ id: editingId, name, color });
        setEditingId(null);
      } else {
        if (isMono && !createAccountId) return;
        await createLabel.mutateAsync({
          account_id: createAccountId,
          name,
          color,
        });
      }
      setName("");
      setColor(COLORS[0]);
    } catch {
      // mutation onError / toast handled by react-query defaults; avoid unhandledRejection
    }
  };

  if (isLoading)
    return <div className="text-muted-foreground text-sm">{t("loading")}</div>;

  return (
    <div className="space-y-3">
      <div className="flex gap-2 items-center">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("placeholder_name")}
          className="flex-1"
        />
        <div className="relative">
          <button
            onClick={() => setPickerOpen(!pickerOpen)}
            className="h-9 w-24 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm flex items-center gap-1.5"
            type="button"
          >
            <span
              className="w-4 h-4 rounded shrink-0 inline-block"
              style={{ backgroundColor: color }}
            />
            <span className="text-xs text-muted-foreground">{t("colors")}</span>
          </button>
          {pickerOpen && (
            <div className="absolute z-50 top-full mt-1 bg-card border rounded-lg shadow-lg p-2 grid grid-cols-5 gap-1 w-44">
              {COLORS.map((c) => (
                <button
                  type="button"
                  key={c}
                  onClick={() => {
                    setColor(c);
                    setPickerOpen(false);
                  }}
                  className="w-7 h-7 rounded-md border-2 transition-transform hover:scale-110"
                  style={{
                    backgroundColor: c,
                    borderColor: color === c ? "#fff" : "transparent",
                  }}
                />
              ))}
            </div>
          )}
        </div>
        <Button
          size="sm"
          onClick={handleSave}
          disabled={isMono && !createAccountId}
        >
          {editingId ? t("update") : t("add")}
        </Button>
        {editingId && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setEditingId(null);
              setName("");
            }}
          >
            {t("cancel")}
          </Button>
        )}
      </div>
      <div className="space-y-1">
        {(labels || []).map((l) => (
          <div
            key={l.id}
            className="flex items-center gap-2 px-2 py-1 rounded hover:bg-muted group"
          >
            <span
              className="w-3 h-3 rounded-full shrink-0"
              style={{ backgroundColor: l.color }}
            />
            <span className="flex-1 text-sm">{l.name}</span>
            <button
              className="text-xs text-muted-foreground hover:text-foreground hidden group-hover:inline"
              onClick={() => {
                setEditingId(l.id);
                setName(l.name);
                setColor(l.color);
              }}
            >
              {t("edit")}
            </button>
            <button
              className="text-xs text-muted-foreground hover:text-red-400 hidden group-hover:inline"
              onClick={() => deleteLabel.mutate(l.id)}
            >
              {t("del")}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
