"use client";

import React, { useState } from "react";
import { useRules, useLabels, useFolders } from "@/hooks/useEmailQueries";
import { useCreateRule, useUpdateRule, useDeleteRule } from "@/hooks/useAdminQueries";
import { type FilterRule } from "@/hooks/useEmailTypes";
import { useWebhooks } from "@/hooks/useWebhooks";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ModelSelector } from "@/components/model-selector";

const sel =
  "h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm";

export function RuleManager({ accountId }: { accountId: string }) {
  const t = useTranslations("settings");
  const { data: rules, isLoading } = useRules(accountId);
  const createRule = useCreateRule();
  const updateRule = useUpdateRule();
  const deleteRule = useDeleteRule();
  const { data: labels } = useLabels(accountId);
  const { data: folders } = useFolders(accountId);
  const { data: webhooks } = useWebhooks(accountId);

  const def = {
    account_id: accountId,
    name: "",
    enabled: true,
    condition_field: "from",
    condition_operator: "contains",
    condition_value: "",
    action_type: "apply_label",
    action_value: "",
    priority: 0,
  };
  const [form, setForm] = useState<FilterRule>(def);
  const [editingId, setEditingId] = useState<string | null>(null);

  const [labelsDropdownOpen, setLabelsDropdownOpen] = useState(false);
  const labelsDropdownRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (
        labelsDropdownRef.current &&
        !labelsDropdownRef.current.contains(e.target as Node)
      ) {
        setLabelsDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const handleSave = async () => {
    if (!form.condition_value.trim() || !form.name.trim()) return;
    if (editingId) {
      await updateRule.mutateAsync({ ...form, id: editingId });
      setEditingId(null);
    } else {
      await createRule.mutateAsync(form);
    }
    setForm(def);
  };

  const handleEdit = (r: FilterRule) => {
    setForm(r);
    setEditingId(r.id || null);
  };

  if (isLoading)
    return <div className="text-muted-foreground text-sm">{t("loading")}</div>;

  const FIELDS: [string, string][] = [
    ["from", t("rule_from")],
    ["subject", t("rule_subject")],
    ["to", t("rule_to")],
  ];
  const OPS: [string, string][] = [
    ["contains", t("rule_contains")],
    ["equals", t("rule_equals")],
  ];
  const ACTIONS: [string, string][] = [
    ["apply_label", t("rule_apply_label")],
    ["delete", t("rule_delete")],
    ["move", t("rule_move")],
    ["forward", t("rule_forward")],
    ["auto_draft", t("rule_auto_draft")],
    ["trigger_webhook", t("rule_webhook")],
    ["send_notification", t("rule_send_notification")],
  ];

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2">
        <Input
          placeholder={t("placeholder_rule_name")}
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
        <select
          className={sel}
          value={form.action_type}
          onChange={(e) => setForm({ ...form, action_type: e.target.value })}
        >
          {ACTIONS.map(([v, l]) => (
            <option key={v} value={v}>
              {l}
            </option>
          ))}
        </select>
        <select
          className={sel}
          value={form.condition_field}
          onChange={(e) =>
            setForm({ ...form, condition_field: e.target.value })
          }
        >
          {FIELDS.map(([v, l]) => (
            <option key={v} value={v}>
              {l}
            </option>
          ))}
        </select>
        <select
          className={sel}
          value={form.condition_operator}
          onChange={(e) =>
            setForm({ ...form, condition_operator: e.target.value })
          }
        >
          {OPS.map(([v, l]) => (
            <option key={v} value={v}>
              {l}
            </option>
          ))}
        </select>
        <div className="col-span-2 flex gap-2">
          <Input
            placeholder={t("placeholder_value")}
            value={form.condition_value}
            onChange={(e) =>
              setForm({ ...form, condition_value: e.target.value })
            }
            className="flex-1"
          />
          <Button size="sm" onClick={handleSave}>
            {editingId ? t("update") : t("add")}
          </Button>
          {editingId && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setEditingId(null);
                setForm(def);
              }}
            >
              {t("cancel")}
            </Button>
          )}
        </div>
        {form.action_type === "apply_label" && (
          <div className="col-span-1 relative" ref={labelsDropdownRef}>
            <button
              onClick={() => setLabelsDropdownOpen(!labelsDropdownOpen)}
              className={`${sel} w-full flex items-center justify-between gap-2`}
            >
              <div className="flex items-center gap-2 truncate">
                {form.action_value && labels?.find((l) => l.name === form.action_value) ? (
                  <>
                    <span
                      className="w-3 h-3 rounded-full shrink-0"
                      style={{ backgroundColor: labels.find((l) => l.name === form.action_value)?.color }}
                    />
                    <span className="truncate">{form.action_value}</span>
                  </>
                ) : (
                  <span className="text-muted-foreground">{t("select_label")}</span>
                )}
              </div>
              <span className="text-muted-foreground text-[10px]">▼</span>
            </button>
            {labelsDropdownOpen && (
              <div className="absolute top-full left-0 mt-1 z-50 w-full min-w-40 rounded-xl border border-border bg-card p-1.5 shadow-lg animate-in fade-in slide-in-from-top-1 duration-150">
                <button
                  onClick={() => {
                    setForm({ ...form, action_value: "" });
                    setLabelsDropdownOpen(false);
                  }}
                  className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-sm transition-colors cursor-pointer ${
                    !form.action_value
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                  }`}
                >
                  <span className="w-3 h-3 shrink-0" />
                  <span>{t("select_label")}</span>
                </button>
                {labels?.map((l) => (
                  <button
                    key={l.id}
                    onClick={() => {
                      setForm({ ...form, action_value: l.name });
                      setLabelsDropdownOpen(false);
                    }}
                    className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-sm transition-colors cursor-pointer ${
                      form.action_value === l.name
                        ? "bg-muted text-foreground"
                        : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                    }`}
                  >
                    <span
                      className="w-3 h-3 rounded-full shrink-0"
                      style={{ backgroundColor: l.color }}
                    />
                    <span className="truncate">{l.name}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
        {form.action_type === "move" && (
          <div className="col-span-1">
            <select
              className={`${sel} w-full`}
              value={form.action_value}
              onChange={(e) =>
                setForm({ ...form, action_value: e.target.value })
              }
            >
              <option value="">{t("select_folder", { defaultMessage: "Select folder" })}</option>
              {folders?.map((f) => (
                <option key={f.id} value={f.name}>
                  {f.name}
                </option>
              ))}
            </select>
          </div>
        )}
        {form.action_type === "forward" && (
          <div className="col-span-2">
            <Input
              placeholder={t("forward_target_placeholder")}
              value={form.action_value}
              onChange={(e) =>
                setForm({ ...form, action_value: e.target.value })
              }
              className="w-full"
            />
          </div>
        )}
        {form.action_type === "trigger_webhook" && (
          <div className="col-span-1">
            <select
              className={`${sel} w-full`}
              value={form.action_value}
              onChange={(e) =>
                setForm({ ...form, action_value: e.target.value })
              }
            >
              <option value="">{t("select_webhook")}</option>
              {webhooks?.map((w) => (
                <option key={w.id} value={w.id}>
                  {w.name} ({w.url})
                </option>
              ))}
            </select>
          </div>
        )}
        {form.action_type === "send_notification" && (
          <div className="col-span-1">
            <select
              className={`${sel} w-full`}
              value={form.action_value}
              onChange={(e) =>
                setForm({ ...form, action_value: e.target.value })
              }
            >
              <option value="">{t("select_channel")}</option>
              <option value="tg">{t("notification_telegram")}</option>
              <option value="smart_tg">
                {t("notification_smart_telegram")}
              </option>
              <option value="browser">{t("notification_browser")}</option>
            </select>
          </div>
        )}
        {form.action_type === "auto_draft" && (
          <div className="col-span-2 grid grid-cols-2 gap-2">
            <select
              className={sel}
              value={form.ai_provider || "openrouter"}
              onChange={(e) =>
                setForm({ ...form, ai_provider: e.target.value })
              }
            >
              <option value="openrouter">OpenRouter</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
              <option value="deepseek">DeepSeek</option>
              <option value="groq">Groq</option>
            </select>
            <ModelSelector
              provider={form.ai_provider || "openrouter"}
              value={form.ai_model || "llama-3.1-70b"}
              onChange={(val) => setForm({ ...form, ai_model: val })}
            />
          </div>
        )}
      </div>
      <div className="space-y-1">
        {(rules || []).map((r) => (
          <div
            key={r.id}
            className="flex items-center gap-2 px-2 py-1 rounded hover:bg-muted text-sm group"
          >
            <span
              className={`w-2 h-2 rounded-full shrink-0 ${r.enabled ? "bg-green-500" : "bg-muted-foreground"}`}
            />
            <span className="flex-1 text-xs">
              <span className="font-medium">{r.name}</span>
              <span className="text-muted-foreground flex items-center gap-1 mt-0.5 flex-wrap">
                {r.condition_field} {r.condition_operator} &quot;
                {r.condition_value}&quot; → {r.action_type}
                {r.action_type === "apply_label" && labels?.find((l) => l.name === r.action_value) && (
                  <span
                    className="inline-block w-2 h-2 rounded-full mx-1 border"
                    style={{ backgroundColor: labels.find((l) => l.name === r.action_value)?.color }}
                  />
                )}
                {r.action_value ? <span className="font-medium text-foreground">{r.action_value}</span> : null}
              </span>
            </span>
            <button
              className="text-xs text-muted-foreground hover:text-foreground hidden group-hover:inline"
              onClick={() => handleEdit(r)}
            >
              {t("edit")}
            </button>
            <button
              className="text-xs text-muted-foreground hover:text-red-400 hidden group-hover:inline"
              onClick={() => deleteRule.mutate(r.id!)}
            >
              {t("del")}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
