"use client";

import React, { useState, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import { RotateCcw, AlertTriangle } from "lucide-react";
import {
  ALL_COMMANDS,
  isModifierKey,
  eventToShortcut,
  setHotkeyOverride,
  resetAllHotkeyOverrides,
  formatHotkey,
} from "@/lib/commands";
import { isAIDisabled } from "@/hooks/useEmails";
import { Button } from "@/components/ui/button";

// ============================================================================
// Detect conflicts: which command has the same shortcut?
// ============================================================================

function findConflicts(
  commandId: string,
  keys: string[] | undefined,
  allOverrides: Record<string, string[]>,
): string[] {
  if (!keys || keys.length === 0) return [];
  const keyStr = keys.join(",");
  const conflicts: string[] = [];
  for (const [id, overrideKeys] of Object.entries(allOverrides)) {
    if (id === commandId) continue;
    if (overrideKeys.length === 0) continue;
    if (overrideKeys.join(",") === keyStr) {
      conflicts.push(id);
    }
  }
  return conflicts;
}

// ============================================================================
// Hotkey capture input
// ============================================================================

function CaptureInput({
  value,
  onChange,
  hasConflict,
}: {
  value: string[];
  onChange: (keys: string[]) => void;
  hasConflict: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFocus = useCallback(() => {
    setEditing(true);
  }, []);

  const handleBlur = useCallback(() => {
    setEditing(false);
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      e.preventDefault();
      e.stopPropagation();

      // Backspace/Delete = clear shortcut
      if (e.key === "Backspace" || e.key === "Delete") {
        onChange([]);
        setEditing(false);
        return;
      }

      // Escape = cancel editing, keep current value
      if (e.key === "Escape") {
        setEditing(false);
        return;
      }

      // Skip bare modifiers
      if (isModifierKey(e.key)) return;

      // Parse the full combination
      const shortcut = eventToShortcut(e as unknown as KeyboardEvent);
      if (!shortcut) return;

      onChange([shortcut]);
      setEditing(false);
    },
    [onChange],
  );

  const displayText = editing
    ? "..."
    : value.length > 0
      ? formatHotkey(value)
      : "";

  return (
    <div className="relative">
      <input
        ref={inputRef}
        type="text"
        readOnly
        className={`w-24 text-center text-xs font-mono bg-muted border rounded px-2 py-1 outline-none cursor-pointer ${
          hasConflict
            ? "border-red-500 bg-red-500/10"
            : "border-transparent hover:border-primary/50 focus:border-primary"
        }`}
        value={displayText}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onKeyDown={handleKeyDown}
        placeholder="—"
      />
      {hasConflict && (
        <AlertTriangle className="absolute -right-4 top-1/2 -translate-y-1/2 w-3 h-3 text-red-500" />
      )}
    </div>
  );
}

// ============================================================================
// Category group
// ============================================================================

const CATEGORY_LABELS: Record<string, string> = {
  inbox: "cat_inbox",
  actions: "cat_actions",
  navigation: "cat_navigation",
  compose: "cat_compose",
  toggle: "cat_toggle",
  ai: "cat_ai",
};

// ============================================================================
// Main component
// ============================================================================

export function HotkeySettings() {
  const t = useTranslations("commands");
  const [overrides, setOverrides] = useState<Record<string, string[]>>(() => {
    // Read current overrides from localStorage
    if (typeof window === "undefined") return {};
    try {
      return JSON.parse(localStorage.getItem("rms_hotkey_overrides") || "{}");
    } catch {
      return {};
    }
  });
  const [justReset, setJustReset] = useState<string | null>(null);

  const handleChange = useCallback((commandId: string, keys: string[]) => {
    setHotkeyOverride(commandId, keys);
    setOverrides((prev) => {
      const next = { ...prev };
      if (keys.length === 0) {
        delete next[commandId];
      } else {
        next[commandId] = keys;
      }
      return next;
    });
  }, []);

  const handleResetRow = useCallback((commandId: string) => {
    setHotkeyOverride(commandId, []);
    setOverrides((prev) => {
      const next = { ...prev };
      delete next[commandId];
      return next;
    });
    setJustReset(commandId);
    setTimeout(() => setJustReset(null), 1000);
  }, []);

  const handleResetAll = useCallback(() => {
    resetAllHotkeyOverrides();
    setOverrides({});
    setJustReset("__all__");
    setTimeout(() => setJustReset(null), 1000);
  }, []);

  // Group commands by category — skip AI category when disabled
  const categories = ["inbox", "actions", "navigation", "compose", "toggle"];
  if (!isAIDisabled()) {
    categories.push("ai");
  }
  const hasAnyOverride = Object.keys(overrides).length > 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs">{t("hotkeys_desc")}</p>
        {hasAnyOverride && (
          <Button
            variant="outline"
            size="sm"
            onClick={handleResetAll}
            className="h-7 text-xs gap-1"
          >
            <RotateCcw className="w-3 h-3" />
            {t("reset_all")}
          </Button>
        )}
      </div>

      {categories.map((cat) => {
        const commands = ALL_COMMANDS.filter((c) => c.category === cat);
        if (commands.length === 0) return null;

        return (
          <div key={cat}>
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
              {t(CATEGORY_LABELS[cat])}
            </h3>
            <div className="border rounded-lg overflow-hidden">
              <table className="w-full text-sm">
                <tbody>
                  {commands.map((cmd) => {
                    const hasOverride = !!overrides[cmd.id];
                    const conflicts = findConflicts(
                      cmd.id,
                      overrides[cmd.id],
                      overrides,
                    );
                    const hasConflict = conflicts.length > 0;
                    const isReset = justReset === cmd.id;

                    return (
                      <tr
                        key={cmd.id}
                        className={`border-t first:border-t-0 transition-colors ${
                          isReset
                            ? "bg-green-500/10"
                            : hasConflict
                              ? "bg-red-500/5"
                              : "hover:bg-muted/30"
                        }`}
                      >
                        <td className="px-4 py-2.5 w-[50%]">
                          <span className="text-sm">{t(cmd.labelKey)}</span>
                          {hasConflict && conflicts.length > 0 && (
                            <span className="text-red-400 text-[10px] ml-2">
                              {t("conflict_with", {
                                cmd: conflicts
                                  .map((id) =>
                                    t(
                                      ALL_COMMANDS.find((c) => c.id === id)
                                        ?.labelKey || id,
                                    ),
                                  )
                                  .join(", "),
                              })}
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-center w-[25%]">
                          <CaptureInput
                            value={overrides[cmd.id] || []}
                            onChange={(keys) => handleChange(cmd.id, keys)}
                            hasConflict={hasConflict}
                          />
                        </td>
                        <td className="px-4 py-2.5 text-center w-[15%]">
                          <span className="text-xs text-muted-foreground font-mono">
                            {formatHotkey(cmd.defaultKeys)}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-center w-[10%]">
                          {hasOverride && (
                            <button
                              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                              onClick={() => handleResetRow(cmd.id)}
                              title={t("reset")}
                            >
                              <RotateCcw className="w-3 h-3" />
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        );
      })}
    </div>
  );
}
