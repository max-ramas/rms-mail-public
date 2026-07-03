"use client";

import React, { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "@/hooks/useEmails";
import { useTranslations } from "next-intl";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAccounts, useFolders } from "@/hooks/useEmailQueries";
import type { Account, Folder } from "@/hooks/useEmailTypes";

interface AICategory {
  name: string;
  color: string;
  icon: string;
  move_to: string;
  auto_read: boolean;
}

const PRESET_COLORS = [
  "#3b82f6",
  "#10b981",
  "#ef4444",
  "#8b5cf6",
  "#f59e0b",
  "#06b6d4",
  "#6b7280",
  "#ec4899",
  "#84cc16",
  "#f97316",
];

const DEFAULT_NAMES = [
  "Invoice",
  "Support",
  "Urgent",
  "Newsletter",
  "Personal",
  "Business",
  "Official",
  "Finance",
  "HR",
  "Marketing",
  "Legal",
  "Travel",
  "Receipt",
  "Order",
  "Shipping",
  "Meeting",
];

function useInitSelectedAccount(
  accounts: Account[],
): [string, (v: string) => void] {
  const [selected, setSelected] = useState("");
  const firstId = accounts[0]?.id ?? "";
  if (!selected && firstId) {
    return [firstId, setSelected];
  }
  return [selected, setSelected];
}

export function AICategoriesTab() {
  const t = useTranslations("settings");
  const qc = useQueryClient();
  const { data: accounts = [] } = useAccounts();

  const [categories, setCategories] = useState<AICategory[]>([]);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [selectedAccountId, setSelectedAccountId] =
    useInitSelectedAccount(accounts);

  const foldersQuery = useFolders(selectedAccountId);

  const { data: fetched, isLoading } = useQuery<AICategory[]>({
    queryKey: ["ai-categories"],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/system/ai-categories`);
      if (typeof r.data === "string") return [];
      return r.data as AICategory[];
    },
  });

  const displayedCategories = !dirty && fetched ? fetched : categories;

  const saveMutation = useMutation({
    mutationFn: async (cats: AICategory[]) => {
      await axios.put(`${API_BASE}/api/system/ai-categories`, cats);
    },
    onSuccess: () => {
      setDirty(false);
      setSaving(false);
      qc.invalidateQueries({ queryKey: ["ai-categories"] });
    },
    onError: (err: unknown) => {
      setSaving(false);
      const msg = err instanceof Error ? err.message : "Save failed";
      setError(msg);
    },
  });

  const addRow = () => {
    const currentList = dirty ? categories : (fetched ?? []);
    setCategories([
      ...currentList,
      {
        name: "",
        color: PRESET_COLORS[currentList.length % PRESET_COLORS.length],
        icon: "tag",
        move_to: "",
        auto_read: false,
      },
    ]);
    setDirty(true);
  };

  const removeRow = (idx: number) => {
    const currentList = dirty ? categories : (fetched ?? []);
    setCategories(currentList.filter((_, i) => i !== idx));
    setDirty(true);
  };

  const updateField = (
    idx: number,
    field: keyof AICategory,
    value: string | boolean,
  ) => {
    const currentList = dirty ? [...categories] : [...(fetched ?? [])];
    currentList[idx] = { ...currentList[idx], [field]: value };
    setCategories(currentList);
    setDirty(true);
  };

  if (isLoading)
    return (
      <div className="text-muted-foreground text-sm p-4">
        {t("ai_cat_loading")}
      </div>
    );

  return (
    <div className="space-y-4">
      {error && (
        <div className="bg-red-500/10 text-red-400 p-3 rounded text-sm">
          {error}
        </div>
      )}

      <div className="flex items-center gap-3">
        <label className="text-sm text-text-muted whitespace-nowrap">
          {t("ai_cat_account_label")}
        </label>
        <Select value={selectedAccountId} onValueChange={setSelectedAccountId}>
          <SelectTrigger className="w-[250px] bg-card border-border-muted h-8">
            <SelectValue placeholder={t("ai_cat_select_account")} />
          </SelectTrigger>
          <SelectContent className="bg-card border border-border-muted shadow-lg">
            {accounts.map((a: Account) => (
              <SelectItem key={a.id} value={a.id}>
                {a.email}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-4">
        {displayedCategories.length === 0 && !dirty && (
          <div className="text-sm text-muted-foreground p-4 border border-border-muted rounded-lg border-dashed text-center">
            {t("ai_cat_empty")}
          </div>
        )}
        {displayedCategories.map((cat, idx) => (
          <div key={idx} className="flex flex-col md:flex-row md:items-center gap-4 p-4 border border-border-muted rounded-lg bg-card shadow-sm transition-all hover:shadow-md">
            {/* Category Name */}
            <div className="flex-1 space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground md:hidden">{t("ai_cat_category")}</label>
              <div className="flex flex-col sm:flex-row gap-2">
                <Select
                  value={cat.name}
                  onValueChange={(v) => {
                    if (v === "__custom__") updateField(idx, "name", "");
                    else updateField(idx, "name", v);
                  }}
                >
                  <SelectTrigger className="h-9 w-full sm:w-[180px] bg-background border-input">
                    <SelectValue placeholder={t("ai_cat_pick")} />
                  </SelectTrigger>
                  <SelectContent className="bg-popover border-border shadow-lg">
                    {DEFAULT_NAMES.map((n) => (
                      <SelectItem key={n} value={n}>
                        {n}
                      </SelectItem>
                    ))}
                    <SelectItem value="__custom__">
                      {t("ai_cat_custom")}
                    </SelectItem>
                  </SelectContent>
                </Select>
                {!DEFAULT_NAMES.includes(cat.name) && (
                  <Input
                    value={cat.name}
                    onChange={(e) => updateField(idx, "name", e.target.value)}
                    placeholder={t("ai_cat_custom_placeholder")}
                    className="h-9 w-full sm:w-[180px] bg-background"
                  />
                )}
              </div>
            </div>

            {/* Category Color */}
            <div className="md:w-[220px] space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground md:hidden">{t("ai_cat_color")}</label>
              <div className="flex gap-2 flex-wrap">
                {PRESET_COLORS.map((c) => (
                  <button
                    key={c}
                    type="button"
                    className={`w-6 h-6 rounded-full transition-all ${cat.color === c ? "ring-2 ring-primary ring-offset-2 ring-offset-background scale-110" : "hover:scale-110 opacity-80 hover:opacity-100"}`}
                    style={{ backgroundColor: c }}
                    onClick={() => updateField(idx, "color", c)}
                  />
                ))}
              </div>
            </div>

            {/* Move To */}
            <div className="md:w-[200px] space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground md:hidden">{t("ai_cat_move_to")}</label>
              <Select
                value={cat.move_to || "__none__"}
                onValueChange={(v) =>
                  updateField(idx, "move_to", v === "__none__" ? "" : v)
                }
              >
                <SelectTrigger className="h-9 w-full bg-background border-input">
                  <SelectValue placeholder={t("ai_cat_no_move")} />
                </SelectTrigger>
                <SelectContent className="bg-popover border-border shadow-lg">
                  <SelectItem value="__none__">
                    {t("ai_cat_no_move")}
                  </SelectItem>
                  {(foldersQuery.data || []).map((f: Folder) => (
                    <SelectItem key={f.id} value={f.id}>
                      {f.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Auto Read & Delete */}
            <div className="flex items-center justify-between md:justify-end gap-6 md:w-[120px] pt-2 md:pt-0 border-t md:border-0 border-border-muted">
              <div className="flex items-center gap-2">
                <label className="text-xs font-medium text-muted-foreground md:hidden">{t("ai_cat_auto_read")}</label>
                <Switch
                  checked={cat.auto_read}
                  onCheckedChange={(v) => updateField(idx, "auto_read", v)}
                />
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive hover:bg-destructive/10"
                onClick={() => removeRow(idx)}
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          </div>
        ))}
      </div>

      <div className="flex items-center gap-3">
        <Button variant="outline" size="sm" onClick={addRow}>
          <Plus className="w-4 h-4 mr-1" /> {t("ai_cat_add")}
        </Button>
        <Button
          size="sm"
          onClick={() => {
            setSaving(true);
            saveMutation.mutate(dirty ? categories : (fetched ?? []));
          }}
          disabled={!dirty || saving}
        >
          {saving ? t("ai_cat_saving") : t("ai_cat_save")}
        </Button>
        {dirty && !saving && (
          <span className="text-xs font-medium text-amber-500 dark:text-yellow-400">{t("ai_cat_unsaved")}</span>
        )}
      </div>
    </div>
  );
}
