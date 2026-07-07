"use client";

import React from "react";
import {
  ALL_COMMANDS,
  getEffectiveKeys,
  eventToShortcut,
} from "@/lib/commands";
import { dispatchCommand } from "@/lib/commandBus";
import { isInsideModal } from "@/lib/query-cache";
import { isBulkSelectionActive } from "@/lib/bulk-selection-guard";
import { isAIDisabled } from "@/hooks/useEmails";
import { HotkeyManager } from "@/lib/HotkeyManager";

// ============================================================================
// Backward-compatible callback interface (deprecated: use Event Bus instead)
// ============================================================================

export interface ShortcutActions {
  onNextEmail?: () => void;
  onPrevEmail?: () => void;
  onReply?: () => void;
  onForward?: () => void;
  onArchive?: () => void;
  onDelete?: () => void;
  onSearchFocus?: () => void;
  onNewEmail?: () => void;
  onToggleHelp?: () => void;
  onEnterInbox?: () => void;
}

// ============================================================================
// Helpers
// ============================================================================

function isEditableTarget(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  if (tagName === "input" || tagName === "textarea") return true;
  if (target.isContentEditable) return true;
  return false;
}

/**
 * Keys that browsers hijack and must be prevented before the browser acts.
 * Format: "Ctrl+T", "Cmd+N", etc. Applied cross-platform.
 */
const PREVENT_DEFAULT_KEYS = new Set([
  "Cmd+N",
  "Cmd+T",
  "Cmd+W",
  "Cmd+K",
  "Ctrl+N",
  "Ctrl+T",
  "Ctrl+W",
  "Ctrl+K",
  "Cmd+S",
  "Ctrl+S",
  "Cmd+P",
  "Ctrl+P",
  "Cmd+A",
  "Ctrl+A",
  "Cmd+,",
  "Ctrl+,",
]);

// ============================================================================
// Main hook — registry-driven keyboard shortcuts
// ============================================================================

const commandKeyMap = new Map<string, string>();
const preventDefaultKeysLower = new Set(
  [...PREVENT_DEFAULT_KEYS].map((k) => k.replace(/^Ctrl/, "Cmd").toLowerCase()),
);

for (const cmd of ALL_COMMANDS) {
  const effectiveKeys = getEffectiveKeys(cmd);
  for (const keyDef of effectiveKeys) {
    const normalizedDef = keyDef.replace(/^Ctrl/, "Cmd").toLowerCase();
    // In case of conflict, first one wins (if you want the last to win, just keep assigning)
    if (!commandKeyMap.has(normalizedDef)) {
      commandKeyMap.set(normalizedDef, cmd.id);
    }
  }
}

/**
 * useKeyboardShortcuts manages global keyboard shortcuts using the Command Registry.
 *
 * Two modes:
 * 1. Event Bus (recommended): pass a `context` and handle commands via useCommandListener
 * 2. Callbacks (backward compat): pass ShortcutActions for legacy single-key shortcuts
 *
 * Escape Stack: Escape always dispatches 'ui:dismiss' with stopPropagation.
 * Each layer (palette, viewer, list) handles its own dismissal.
 */
export function useKeyboardShortcuts(
  contextOrActions: "inbox" | "compose" | "any" | ShortcutActions = "inbox",
): null {
  React.useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      // --- Escape Stack (always works, even in inputs) ---
      if (
        event.key === "Escape" &&
        !event.metaKey &&
        !event.ctrlKey &&
        !event.shiftKey
      ) {
        event.preventDefault();
        dispatchCommand("ui:dismiss");
        return;
      }

      if (isInsideModal(event.target)) return;

      // Ctrl+P / Cmd+P — custom email print (must run even in inputs)
      if (
        (event.key === "p" || event.key === "P") &&
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey
      ) {
        event.preventDefault();
        event.stopPropagation();
        dispatchCommand("mail:print");
        return;
      }

      if (isEditableTarget(event.target)) return;
      const pressed = eventToShortcut(event);
      if (!pressed) return; // bare modifier, skip

      // Normalize: treat Ctrl as Cmd equivalent for matching
      const normalized = pressed.replace(/^Ctrl/, "Cmd");
      const normLower = normalized.toLowerCase();
      const cmdId = commandKeyMap.get(normLower);

      // --- Legacy callbacks (single-key, no modifiers) ---
      if (typeof contextOrActions === "object") {
        const actions = contextOrActions as ShortcutActions;

        // Only trigger single-key fallbacks when NO modifiers are pressed
        if (
          !event.metaKey &&
          !event.ctrlKey &&
          !event.altKey &&
          !event.shiftKey
        ) {
          const bulkActive = isBulkSelectionActive();
          const code = event.code;
          switch (code) {
            case "KeyJ":
            case "ArrowDown":
              event.preventDefault();
              actions.onNextEmail?.();
              return;
            case "KeyK":
            case "ArrowUp":
              event.preventDefault();
              actions.onPrevEmail?.();
              return;
            case "KeyR":
              event.preventDefault();
              actions.onReply?.();
              return;
            case "KeyA":
            case "KeyE":
              if (bulkActive) return;
              event.preventDefault();
              actions.onArchive?.();
              return;
            case "KeyD":
            case "#":
            case "Backspace":
            case "Delete":
              if (bulkActive) return;
              event.preventDefault();
              actions.onDelete?.();
              return;
            case "/":
              event.preventDefault();
              dispatchCommand("navigation:focus-search");
              return;
            case "KeyU":
              event.preventDefault();
              dispatchCommand("mail:deselect");
              return;
            case "KeyN":
              event.preventDefault();
              actions.onNewEmail?.();
              return;
            case "?":
              event.preventDefault();
              actions.onToggleHelp?.();
              return;
          }
        }
      }

      // --- Alt-key shortcuts (Option on Mac) — handled via event.code for reliability ---
      if (event.altKey && !event.metaKey && !event.ctrlKey) {
        const code = event.code;
        if (code === "KeyT") {
          event.preventDefault();
          dispatchCommand("toggle:threads");
          return;
        }
        if (code === "KeyN") {
          event.preventDefault();
          dispatchCommand("toggle:account-names");
          return;
        }
        if (code === "KeyS") {
          event.preventDefault();
          if (!isAIDisabled()) dispatchCommand("ai:summarize");
          return;
        }
        if (code === "KeyC") {
          event.preventDefault();
          if (!isAIDisabled()) dispatchCommand("ai:categorize");
          return;
        }
        if (code === "KeyD") {
          event.preventDefault();
          if (!isAIDisabled()) dispatchCommand("ai:draft");
          return;
        }
        if (code === "KeyA") {
          event.preventDefault();
          dispatchCommand("compose:attach");
          return;
        }
      }

      // --- Registry-based lookup (Cmd/Shift combos only) ---
      if (cmdId) {
        if (
          preventDefaultKeysLower.has(normLower) ||
          PREVENT_DEFAULT_KEYS.has(pressed)
        ) {
          event.preventDefault();
        }
        dispatchCommand(cmdId);
        return;
      }
    };

    // Register as a wildcard handler via the global HotkeyManager.
    // The wildcard ["*"] receives every keydown; the handler does its own
    // internal filtering (editable-target skip, modifier-only skip, etc.).
    const unreg = HotkeyManager.register(
      ["*"],
      handler,
      typeof contextOrActions === "string" ? contextOrActions : "legacy",
    );
    return unreg;
  }, [contextOrActions]);

  return null;
}
