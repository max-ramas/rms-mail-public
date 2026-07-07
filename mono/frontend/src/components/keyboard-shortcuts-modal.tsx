"use client";

import React, { useState, useEffect } from "react";
import { useTranslations } from "next-intl";
import { X, HelpCircle } from "lucide-react";
import { ALL_COMMANDS, getEffectiveKeys, formatHotkey } from "@/lib/commands";
import { isAIDisabled } from "@/hooks/useEmails";
import { Button } from "@/components/ui/button";

const CATEGORY_LABELS: Record<string, string> = {
  inbox: "cat_inbox",
  actions: "cat_actions",
  navigation: "cat_navigation",
  compose: "cat_compose",
  toggle: "cat_toggle",
  ai: "cat_ai",
};

export function KeyboardShortcutsModal() {
  const t = useTranslations("commands");
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input
      if (
        document.activeElement?.tagName === "INPUT" ||
        document.activeElement?.tagName === "TEXTAREA" ||
        (document.activeElement as HTMLElement)?.isContentEditable
      ) {
        return;
      }
      if (e.key === "?" && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        setIsOpen((prev) => !prev);
      }
      if (e.key === "Escape" && isOpen) {
        setIsOpen(false);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen]);

  // Group commands by category — skip AI category when disabled
  const categories = ["inbox", "actions", "navigation", "compose", "toggle"];
  if (!isAIDisabled()) {
    categories.push("ai");
  }

  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        className="w-8 h-8 rounded-full text-muted-foreground hover:text-foreground"
        title={`${t("hotkeys_title")} (Cmd+K / Ctrl+K)`}
        onClick={() => setIsOpen(true)}
      >
        <HelpCircle className="w-4 h-4" />
      </Button>

      {isOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-background/80 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="bg-card w-full max-w-3xl max-h-[85vh] rounded-2xl border shadow-2xl flex flex-col overflow-hidden">
            <div className="p-4 border-b flex items-center justify-between bg-muted/30">
              <div>
                <h2 className="text-lg font-semibold text-foreground">
                  {t("hotkeys_title")}
                </h2>
                <p className="text-xs text-muted-foreground mt-1">
                  {t("hotkeys_hint")}
                </p>
                <div className="flex gap-1.5 mt-2">
                  <kbd className="px-1.5 py-0.5 text-[10px] font-mono font-medium text-foreground/70 bg-muted border border-border/50 rounded">Cmd+K</kbd>
                  <span className="text-[10px] text-muted-foreground self-center">or</span>
                  <kbd className="px-1.5 py-0.5 text-[10px] font-mono font-medium text-foreground/70 bg-muted border border-border/50 rounded">Cmd+Shift+P</kbd>
                </div>
              </div>
              <button
                onClick={() => setIsOpen(false)}
                className="p-2 text-muted-foreground hover:text-foreground hover:bg-muted rounded-full transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-4 overflow-y-auto flex-1">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-6">
                {categories.map((cat) => {
                  const cmds = ALL_COMMANDS.filter((c) => c.category === cat);
                  if (cmds.length === 0) return null;
                  return (
                    <div key={cat} className="space-y-3">
                      <h3 className="text-xs font-bold text-muted-foreground uppercase tracking-wider border-b pb-1">
                        {t(CATEGORY_LABELS[cat] as Parameters<typeof t>[0])}
                      </h3>
                      <div className="space-y-2">
                        {cmds.map((cmd) => {
                          const keys = getEffectiveKeys(cmd);
                          if (!keys || keys.length === 0) return null;
                          return (
                            <div
                              key={cmd.id}
                              className="flex items-center justify-between gap-4"
                            >
                              <span className="text-sm text-foreground/90">
                                {t(cmd.labelKey as Parameters<typeof t>[0])}
                              </span>
                              <div className="flex gap-1">
                                {keys.map((k) => (
                                  <kbd
                                    key={k}
                                    className="px-2 py-1 text-[11px] font-mono font-medium text-foreground bg-muted border border-border/50 rounded-md shadow-sm"
                                  >
                                    {formatHotkey(k)}
                                  </kbd>
                                ))}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
