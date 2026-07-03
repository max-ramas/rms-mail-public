"use client";

import React, { useEffect, useState, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import {
  type Command,
  getFrequentCommands,
  recordFrequentCommand,
  getCommandsByContext,
  formatHotkey,
} from "@/lib/commands";
import { dispatchCommand } from "@/lib/commandBus";

// ============================================================================
// Icon map — resolves icon name string to lucide component
// ============================================================================
import * as Icons from "lucide-react";

function Icon({ name }: { name: string }) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const IconComponent = (Icons as any)[name];
  if (!IconComponent) return <Search className="w-4 h-4" />;
  return <IconComponent className="w-4 h-4" />;
}

// ============================================================================
// Category labels and colors
// ============================================================================
const CATEGORY_META: Record<string, { labelKey: string; color: string }> = {
  inbox: { labelKey: "cat_inbox", color: "text-blue-400" },
  actions: { labelKey: "cat_actions", color: "text-orange-400" },
  navigation: { labelKey: "cat_navigation", color: "text-green-400" },
  compose: { labelKey: "cat_compose", color: "text-purple-400" },
  toggle: { labelKey: "cat_toggle", color: "text-yellow-400" },
  ai: { labelKey: "cat_ai", color: "text-pink-400" },
};

// ============================================================================
// Component
// ============================================================================

interface Props {
  context?: "inbox" | "compose" | "any";
}

export function CommandPalette({ context = "inbox" }: Props) {
  const t = useTranslations("commands");
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const savedFocus = useRef<Element | null>(null);

  // Filter commands: context-aware + fuzzy search + frequent
  const contextCommands = getCommandsByContext(context);
  const frequentIds = getFrequentCommands();
  const lowerQuery = query.toLowerCase();
  const filtered = query
    ? contextCommands.filter((c) => {
        const label = t(c.labelKey).toLowerCase();
        const id = c.id.toLowerCase();
        return label.includes(lowerQuery) || id.includes(lowerQuery);
      })
    : contextCommands;
  const contextFiltered = filtered.filter((c) =>
    contextCommands.some((cc) => cc.id === c.id),
  );

  // Frequent commands (top 5, only those in current context)
  const frequent = frequentIds
    .map((id) => contextCommands.find((c) => c.id === id))
    .filter(Boolean) as Command[];

  const closePalette = useCallback(() => {
    setOpen(false);
    setQuery("");
    // Restore focus
    requestAnimationFrame(() => {
      if (savedFocus.current instanceof HTMLElement) {
        savedFocus.current.focus();
      }
    });
  }, []);

  // Open/close handler
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.code === "KeyP") {
        e.preventDefault();
        savedFocus.current = document.activeElement;
        setOpen(true);
        setQuery("");
        setSelectedIndex(0);
      }
      if (e.key === "Escape" && open) {
        e.preventDefault();
        e.stopPropagation();
        closePalette();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Auto-focus input when opened
  useEffect(() => {
    if (open) {
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const executeCommand = useCallback(
    (command: Command) => {
      recordFrequentCommand(command.id);
      dispatchCommand(command.id);
      closePalette();
    },
    [closePalette],
  );

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      closePalette();
      return;
    }

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) =>
        Math.min(prev + 1, contextFiltered.length - 1),
      );
      return;
    }

    if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) => Math.max(prev - 1, 0));
      return;
    }

    if (e.key === "Enter") {
      e.preventDefault();
      const cmd = contextFiltered[selectedIndex];
      if (cmd) executeCommand(cmd);
      return;
    }
  };

  // Scroll selected item into view
  useEffect(() => {
    if (listRef.current) {
      const item = listRef.current.children[selectedIndex] as HTMLElement;
      if (item) item.scrollIntoView({ block: "nearest" });
    }
  }, [selectedIndex]);

  if (!open) return null;

  // Group commands by category for display
  const showFrequent = !query && frequent.length > 0;
  // Always show the full context-filtered list; frequent commands render as a separate section above
  const displayCommands = contextFiltered;
  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4"
      onClick={closePalette}
    >
      <div
        className="bg-card border rounded-xl shadow-2xl w-full max-w-lg mx-4 overflow-hidden flex flex-col"
        style={{ maxHeight: "calc(100vh - 15vh - 2rem)" }}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        {/* Search input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b">
          <Search className="w-5 h-5 text-muted-foreground shrink-0" />
          <input
            ref={inputRef}
            type="text"
            className="flex-1 bg-transparent text-base outline-none placeholder:text-muted-foreground"
            placeholder={t("placeholder")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>

        {/* Command list */}
        <div
          ref={listRef}
          className="flex-1 overflow-y-auto py-2"
          role="listbox"
        >
          {showFrequent && (
            <>
              <div className="px-4 py-1">
                <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  {t("frequent")}
                </span>
              </div>
              {frequent.map((cmd) => (
                <button
                  key={`freq-${cmd.id}`}
                  className="w-full flex items-center gap-3 px-4 py-2 text-sm text-left hover:bg-muted/50 transition-colors"
                  onClick={() => executeCommand(cmd)}
                >
                  <Icon name={cmd.icon} />
                  <span className="flex-1 truncate">{t(cmd.labelKey)}</span>
                  <span className="text-xs text-muted-foreground font-mono shrink-0">
                    {formatHotkey(cmd.defaultKeys)}
                  </span>
                </button>
              ))}
              <div className="border-b mx-4 my-1" />
            </>
          )}

          {displayCommands.map((cmd, idx) => {
            const prev = idx > 0 ? displayCommands[idx - 1] : null;
            const showCat = !prev || prev.category !== cmd.category;

            return (
              <React.Fragment key={cmd.id}>
                {showCat && (
                  <div className="px-4 pt-2 pb-1">
                    <span
                      className={`text-[10px] uppercase tracking-wider ${
                        CATEGORY_META[cmd.category]?.color ||
                        "text-muted-foreground"
                      }`}
                    >
                      {t(CATEGORY_META[cmd.category]?.labelKey)}
                    </span>
                  </div>
                )}
                <button
                  className={`w-full flex items-center gap-3 px-4 py-2 text-sm text-left transition-colors ${
                    idx === selectedIndex
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-muted/50"
                  }`}
                  onClick={() => executeCommand(cmd)}
                  onMouseEnter={() => setSelectedIndex(idx)}
                  role="option"
                  aria-selected={idx === selectedIndex}
                >
                  <Icon name={cmd.icon} />
                  <span className="flex-1 truncate">{t(cmd.labelKey)}</span>
                  <span className="text-xs text-muted-foreground font-mono shrink-0">
                    {formatHotkey(cmd.defaultKeys)}
                  </span>
                </button>
              </React.Fragment>
            );
          })}

          {displayCommands.length === 0 && (
            <div className="px-4 py-8 text-center text-muted-foreground text-sm">
              {t("no_results")}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// Hook for page-level integration
// ============================================================================

export function useCommandPalette(
  context: "inbox" | "compose" | "any" = "inbox",
) {
  return <CommandPalette context={context} />;
}
