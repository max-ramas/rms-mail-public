// ============================================================================
// Command Registry — single source of truth for all keyboard/palette commands.
// Pure data: no functions, no React hooks. Only metadata consumed by:
//   - command-palette.tsx (rendering + filtering)
//   - useKeyboardShortcuts.ts (key binding)
//   - hotkey-settings.tsx (rebind UI)
// ============================================================================
export interface Command {
  id: string; // "mail:archive"
  labelKey: string; // "mail_archive"
  category: "inbox" | "actions" | "navigation" | "toggle" | "compose" | "ai";
  icon: string; // lucide icon name (passed to dynamic import)
  defaultKeys: string[]; // ["e"] or ["Cmd+Shift+A"]
  context: "inbox" | "compose" | "any";
}

// ============================================================================
// Modifier helpers
// ============================================================================

export const MODIFIER_KEYS = new Set([
  "Shift",
  "Control",
  "Alt",
  "Meta",
  "ShiftLeft",
  "ShiftRight",
  "ControlLeft",
  "ControlRight",
  "AltLeft",
  "AltRight",
  "MetaLeft",
  "MetaRight",
]);

export function isModifierKey(key: string): boolean {
  return MODIFIER_KEYS.has(key);
}

export function isMac(): boolean {
  if (typeof navigator === "undefined") return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

/**
 * Format a hotkey string for display.
 * "Cmd+S" → "⌘S" on Mac, "Ctrl+S" on Windows/Linux.
 */
export function formatHotkey(keys: string[] | string): string {
  if (!keys || keys.length === 0) return "";
  const mac = isMac();

  // If there are multiple alternative shortcuts, pick the best one for the current OS
  let keyStr = Array.isArray(keys) ? keys[0] : keys;
  if (Array.isArray(keys) && keys.length > 1) {
    if (mac) {
      keyStr =
        keys.find((k) => k.includes("Cmd") || k.includes("Meta")) || keys[0];
    } else {
      keyStr =
        keys.find((k) => !k.includes("Cmd") && !k.includes("Meta")) || keys[0];
    }
  }

  return keyStr
    .replace(/\bCmd\b/g, mac ? "⌘" : "Ctrl")
    .replace(/\bCtrl\b/g, mac ? "⌃" : "Ctrl")
    .replace(/\bShift\b/g, mac ? "⇧" : "Shift")
    .replace(/\bAlt\b/g, mac ? "⌥" : "Alt")
    .replace(/\bMeta\b/g, mac ? "⌘" : "Win")
    .replace(/\bArrowUp\b/g, "↑")
    .replace(/\bArrowDown\b/g, "↓")
    .replace(/\bArrowLeft\b/g, "←")
    .replace(/\bArrowRight\b/g, "→")
    .replace(/\bEnter\b/g, "↵")
    .replace(/\bEscape\b/g, "Esc")
    .replace(/\bBackspace\b/g, "⌫")
    .replace(/\bDelete\b/g, "⌦")
    .replace(/\bSpace\b/g, "␣")
    .replace(/\+/g, "");
}

/**
 * Parse a KeyboardEvent into a normalized shortcut string.
 * "Cmd+S" (with Meta+S pressed) or "Ctrl+S" (with Ctrl+S pressed).
 */
export function eventToShortcut(e: KeyboardEvent): string {
  const parts: string[] = [];
  if (e.metaKey) parts.push("Cmd");
  if (e.ctrlKey) parts.push("Ctrl");
  if (e.altKey) parts.push("Alt");
  if (e.shiftKey) parts.push("Shift");

  // Skip bare modifiers
  const key = e.key;
  if (
    isModifierKey(key) ||
    key === "Meta" ||
    key === "Control" ||
    key === "Alt"
  )
    return "";

  // Normalize key name — ALWAYS use e.code (physical key position)
  // for letter/digit keys. This works on ALL layouts (QWERTY, AZERTY,
  // Russian, etc.) because e.code is based on USB HID key position,
  // not the character produced by the current keyboard layout.
  let normalized: string;
  if (e.code && e.code.startsWith("Key")) {
    normalized = e.code.slice(3); // "KeyT" → "T"
  } else if (e.code && e.code.startsWith("Digit")) {
    normalized = e.code.slice(5); // "Digit1" → "1"
  } else if (e.key === " ") {
    normalized = "Space";
  } else if (e.key.length === 1) {
    normalized = parts.length > 0 ? e.key.toUpperCase() : e.key.toLowerCase();
  } else {
    normalized = e.key;
  }

  // Normalize case: modifiers → uppercase, bare key → lowercase
  // (matches the convention used in ALL_COMMANDS defaultKeys)
  if (parts.length > 0) {
    normalized = normalized.toUpperCase();
  } else {
    normalized = normalized.toLowerCase();
  }

  parts.push(normalized);
  return parts.join("+");
}

// ============================================================================
// All Commands
// ============================================================================

export const ALL_COMMANDS: Command[] = [
  // --- Actions ---
  {
    id: "mail:print",
    labelKey: "mail_print",
    category: "actions",
    icon: "Printer",
    defaultKeys: ["Cmd+P", "Ctrl+P"],
    context: "inbox",
  },
  {
    id: "mail:archive",
    labelKey: "mail_archive",
    category: "actions",
    icon: "Archive",
    defaultKeys: ["e"],
    context: "inbox",
  },
  {
    id: "mail:delete",
    labelKey: "mail_delete",
    category: "actions",
    icon: "Trash2",
    defaultKeys: ["#", "Delete", "Backspace"],
    context: "inbox",
  },
  {
    id: "mail:reply",
    labelKey: "mail_reply",
    category: "actions",
    icon: "Reply",
    defaultKeys: ["r"],
    context: "inbox",
  },
  {
    id: "mail:forward",
    labelKey: "mail_forward",
    category: "actions",
    icon: "Forward",
    defaultKeys: ["f"],
    context: "inbox",
  },
  {
    id: "mail:new-email",
    labelKey: "mail_new_email",
    category: "inbox",
    icon: "PenSquare",
    defaultKeys: ["n"],
    context: "any",
  },
  {
    id: "mail:mark-read",
    labelKey: "mail_mark_read",
    category: "inbox",
    icon: "CheckCheck",
    defaultKeys: ["Shift+I"],
    context: "inbox",
  },
  {
    id: "mail:mark-unread",
    labelKey: "mail_mark_unread",
    category: "inbox",
    icon: "Mail",
    defaultKeys: ["Shift+U"],
    context: "inbox",
  },
  {
    id: "mail:toggle-flag",
    labelKey: "mail_toggle_flag",
    category: "actions",
    icon: "Flag",
    defaultKeys: ["Shift+F"],
    context: "inbox",
  },
  {
    id: "mail:pin",
    labelKey: "mail_pin",
    category: "actions",
    icon: "Pin",
    defaultKeys: ["Shift+P"],
    context: "inbox",
  },
  {
    id: "mail:select-all",
    labelKey: "mail_select_all",
    category: "inbox",
    icon: "CheckSquare",
    defaultKeys: ["Cmd+A", "Ctrl+A"],
    context: "inbox",
  },
  {
    id: "mail:deselect",
    labelKey: "mail_deselect",
    category: "inbox",
    icon: "Square",
    defaultKeys: ["u"],
    context: "inbox",
  },
  {
    id: "mail:move",
    labelKey: "mail_move",
    category: "inbox",
    icon: "MoveRight",
    defaultKeys: ["m"],
    context: "inbox",
  },
  {
    id: "mail:snooze",
    labelKey: "mail_snooze",
    category: "inbox",
    icon: "Clock",
    defaultKeys: ["z"],
    context: "inbox",
  },

  // --- Navigation ---
  {
    id: "navigation:next-email",
    labelKey: "navigation_next_email",
    category: "navigation",
    icon: "ArrowDown",
    defaultKeys: ["j", "ArrowDown"],
    context: "inbox",
  },
  {
    id: "navigation:prev-email",
    labelKey: "navigation_prev_email",
    category: "navigation",
    icon: "ArrowUp",
    defaultKeys: ["k", "ArrowUp"],
    context: "inbox",
  },
  {
    id: "navigation:focus-search",
    labelKey: "navigation_focus_search",
    category: "navigation",
    icon: "Search",
    defaultKeys: ["/"],
    context: "inbox",
  },
  {
    id: "navigation:go-inbox",
    labelKey: "navigation_go_inbox",
    category: "navigation",
    icon: "Inbox",
    defaultKeys: ["g"],
    context: "any",
  },
  {
    id: "navigation:go-settings",
    labelKey: "navigation_go_settings",
    category: "navigation",
    icon: "Settings",
    defaultKeys: ["Cmd+,"],
    context: "any",
  },
  {
    id: "navigation:command-palette",
    labelKey: "navigation_command_palette",
    category: "navigation",
    icon: "Search",
    defaultKeys: ["Cmd+K", "Cmd+Shift+P"],
    context: "any",
  },
  {
    id: "navigation:go-drafts",
    labelKey: "navigation_go_drafts",
    category: "navigation",
    icon: "FileEdit",
    defaultKeys: ["g", "d"],
    context: "any",
  },
  {
    id: "navigation:go-sent",
    labelKey: "navigation_go_sent",
    category: "navigation",
    icon: "Send",
    defaultKeys: ["g", "s"],
    context: "any",
  },
  {
    id: "navigation:scroll-up",
    labelKey: "navigation_scroll_up",
    category: "navigation",
    icon: "ChevronUp",
    defaultKeys: ["Shift+Space"],
    context: "any",
  },
  {
    id: "navigation:scroll-down",
    labelKey: "navigation_scroll_down",
    category: "navigation",
    icon: "ChevronDown",
    defaultKeys: ["Space"],
    context: "any",
  },

  // --- Compose ---
  {
    id: "compose:send",
    labelKey: "compose_send",
    category: "compose",
    icon: "Send",
    defaultKeys: ["Cmd+Enter"],
    context: "compose",
  },
  {
    id: "compose:attach",
    labelKey: "compose_attach",
    category: "compose",
    icon: "Paperclip",
    defaultKeys: ["Alt+A"],
    context: "compose",
  },
  {
    id: "compose:save-draft",
    labelKey: "compose_save_draft",
    category: "compose",
    icon: "Save",
    defaultKeys: ["Cmd+S"],
    context: "compose",
  },
  {
    id: "compose:discard",
    labelKey: "compose_discard",
    category: "compose",
    icon: "X",
    defaultKeys: ["Escape"],
    context: "compose",
  },

  // --- Toggle ---
  {
    id: "toggle:threads",
    labelKey: "toggle_threads",
    category: "toggle",
    icon: "MessagesSquare",
    defaultKeys: ["Alt+T"],
    context: "any",
  },
  {
    id: "toggle:account-names",
    labelKey: "toggle_account_names",
    category: "toggle",
    icon: "User",
    defaultKeys: ["Alt+N"],
    context: "any",
  },

  // --- AI ---
  {
    id: "ai:summarize",
    labelKey: "ai_summarize",
    category: "ai",
    icon: "Sparkles",
    defaultKeys: ["Alt+S"],
    context: "inbox",
  },
  {
    id: "ai:categorize",
    labelKey: "ai_categorize",
    category: "ai",
    icon: "Tags",
    defaultKeys: ["Alt+C"],
    context: "inbox",
  },
  {
    id: "ai:draft",
    labelKey: "ai_draft",
    category: "ai",
    icon: "PenLine",
    defaultKeys: ["Alt+D"],
    context: "inbox",
  },
  {
    id: "ai:chat",
    labelKey: "ai_chat",
    category: "ai",
    icon: "MessageSquare",
    defaultKeys: ["Alt+G"],
    context: "any",
  },
];

// ============================================================================
// Helpers
// ============================================================================

export function getCommand(id: string): Command | undefined {
  return ALL_COMMANDS.find((c) => c.id === id);
}

export function getCommandsByContext(
  context: "inbox" | "compose" | "any",
): Command[] {
  const aiDisabled =
    typeof window !== "undefined" &&
    localStorage.getItem("rms_ai_disabled") === "true";
  return ALL_COMMANDS.filter(
    (c) =>
      (c.context === context || c.context === "any") &&
      !(aiDisabled && c.category === "ai"),
  );
}

export function searchCommands(query: string): Command[] {
  if (!query.trim()) return ALL_COMMANDS;
  const q = query.toLowerCase();
  return ALL_COMMANDS.filter((c) => {
    // Match by labelKey (the translation key itself for fuzzy matching)
    if (c.labelKey.toLowerCase().includes(q)) return true;
    if (c.id.toLowerCase().includes(q)) return true;
    if (c.category.toLowerCase().includes(q)) return true;
    return false;
  });
}

// ============================================================================
// localStorage: hotkey overrides + frequently used
// ============================================================================

const OVERRIDES_KEY = "rms_hotkey_overrides";
const FREQUENT_KEY = "rms_frequent_commands";

export function getHotkeyOverrides(): Record<string, string[]> {
  if (typeof window === "undefined") return {};
  try {
    return JSON.parse(localStorage.getItem(OVERRIDES_KEY) || "{}");
  } catch {
    return {};
  }
}

export function setHotkeyOverride(commandId: string, keys: string[]) {
  if (typeof window === "undefined") return;
  const overrides = getHotkeyOverrides();
  if (keys.length === 0) {
    delete overrides[commandId];
  } else {
    overrides[commandId] = keys;
  }
  localStorage.setItem(OVERRIDES_KEY, JSON.stringify(overrides));
}

export function resetAllHotkeyOverrides() {
  if (typeof window === "undefined") return;
  localStorage.removeItem(OVERRIDES_KEY);
}

export function getEffectiveKeys(command: Command): string[] {
  const overrides = getHotkeyOverrides();
  return overrides[command.id] || command.defaultKeys;
}

export function getFrequentCommands(): string[] {
  if (typeof window === "undefined") return [];
  try {
    return JSON.parse(localStorage.getItem(FREQUENT_KEY) || "[]");
  } catch {
    return [];
  }
}

export function recordFrequentCommand(commandId: string) {
  if (typeof window === "undefined") return;
  const frequent = getFrequentCommands();
  const filtered = frequent.filter((id) => id !== commandId);
  filtered.unshift(commandId);
  localStorage.setItem(FREQUENT_KEY, JSON.stringify(filtered.slice(0, 5)));
}
