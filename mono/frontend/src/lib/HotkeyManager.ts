"use client";

type HotkeyHandler = (e: KeyboardEvent) => void;

const devLog = (...args: unknown[]) => {
  if (process.env.NODE_ENV === "development") {
    console.log(...args);
  }
};

interface RegisteredHotkey {
  keys: string[]; // e.g. ["Cmd+A", "Ctrl+A"], or ["*"] for wildcard
  handler: HotkeyHandler;
  context?: string; // optional context filter (unused currently, future-proof)
}

class HotkeyManagerClass {
  private handlers: RegisteredHotkey[] = [];
  private boundHandler: ((e: KeyboardEvent) => void) | null = null;
  private registered = false;

  /** Start the global listener (called once in layout). */
  start() {
    if (this.registered) return;
    this.boundHandler = this.handleKeyDown.bind(this);
    // Capture phase: intercept before iframes/inputs consume the event
    window.addEventListener("keydown", this.boundHandler, true);
    this.registered = true;
    if (typeof window !== "undefined") {
      devLog("[HotkeyManager] Global listener started (capture phase)");
    }
  }

  /**
   * Dispose the global listener and clear all handlers.
   * Call during app-level cleanup (e.g. HMR dispose) or testing.
   */
  dispose() {
    if (this.boundHandler) {
      window.removeEventListener("keydown", this.boundHandler, true);
      this.boundHandler = null;
    }
    this.handlers = [];
    this.registered = false;
    if (typeof window !== "undefined") {
      devLog("[HotkeyManager] Disposed — all handlers cleared");
    }
  }

  /**
   * Register a hotkey handler. Returns an unregister function.
   *
   * IMPORTANT: Always call the returned unregister function in your
   * useEffect cleanup to prevent stale handlers from accumulating.
   *
   * @param keys — Shortcut strings like "Cmd+A" or "Ctrl+A". Use ["*"] as a
   *   wildcard to match ALL keydown events (the handler can then do its own
   *   internal dispatch).
   * @param handler — The callback that receives the original KeyboardEvent.
   * @param context — Optional context identifier (for debugging).
   */
  register(
    keys: string[],
    handler: HotkeyHandler,
    context?: string,
  ): () => void {
    const entry: RegisteredHotkey = { keys, handler, context };
    this.handlers.push(entry);
    if (typeof window !== "undefined") {
      devLog(`[HotkeyManager] Registered: ${keys.join(", ")}`);
    }
    return () => {
      this.handlers = this.handlers.filter((h) => h !== entry);
      if (typeof window !== "undefined") {
        devLog(`[HotkeyManager] Unregistered: ${keys.join(", ")}`);
      }
    };
  }

  /** Check if any handler is registered (for diagnostics). */
  getHandlerCount(): number {
    return this.handlers.length;
  }

  private handleKeyDown(e: KeyboardEvent) {
    // First pass: specific key handlers (take precedence over wildcards)
    for (const entry of this.handlers) {
      if (entry.keys.includes("*")) continue; // skip wildcards in first pass

      const pressed = eventToShortcut(e);
      if (!pressed) continue;

      // Normalize: treat Ctrl as Cmd equivalent for matching
      const normalized = pressed.replace(/^Ctrl/, "Cmd");
      for (const key of entry.keys) {
        const normKey = key.replace(/^Ctrl/, "Cmd");
        if (normalized.toLowerCase() === normKey.toLowerCase()) {
          entry.handler(e);
          return; // first specific match wins
        }
      }
    }

    // Second pass: wildcard handlers (catch-all, lower priority)
    for (const entry of this.handlers) {
      if (entry.keys.includes("*")) {
        entry.handler(e);
        return; // first wildcard wins
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Helper: convert KeyboardEvent to shortcut string like "Cmd+A" or "Shift+I"
// ---------------------------------------------------------------------------

export function eventToShortcut(e: KeyboardEvent): string {
  const parts: string[] = [];
  if (e.metaKey) parts.push("Cmd");
  if (e.ctrlKey) parts.push("Ctrl");
  if (e.altKey) parts.push("Alt");
  if (e.shiftKey) parts.push("Shift");

  const key = e.key;
  // Skip bare modifiers
  if (key === "Meta" || key === "Control" || key === "Alt" || key === "Shift")
    return "";

  // Normalize: use e.code for letter/digit keys (layout-independent)
  let normalized: string;
  if (e.code && e.code.startsWith("Key")) {
    normalized = e.code.slice(3);
  } else if (e.code && e.code.startsWith("Digit")) {
    normalized = e.code.slice(5);
  } else if (key === " ") {
    normalized = "Space";
  } else if (key.length === 1) {
    normalized = parts.length > 0 ? key.toUpperCase() : key.toLowerCase();
  } else {
    normalized = key;
  }

  if (parts.length > 0) {
    normalized = normalized.toUpperCase();
  } else {
    normalized = normalized.toLowerCase();
  }

  parts.push(normalized);
  return parts.join("+");
}

// Singleton
export const HotkeyManager = new HotkeyManagerClass();
