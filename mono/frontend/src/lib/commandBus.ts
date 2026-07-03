"use client";

import { useEffect, useRef } from "react";

// ============================================================================
// Command Bus — decouples palette/keyboard from business logic via CustomEvent
// ============================================================================

/**
 * Dispatch a global command. Components subscribe via useCommandListener.
 * Palette and keyboard handler only know commandId, never call actions directly.
 *
 * Usage:
 *   dispatchCommand("mail:archive")
 *   dispatchCommand("navigation:next-email")
 */
export function dispatchCommand(commandId: string) {
  window.dispatchEvent(
    new CustomEvent("app:command", { detail: { commandId } })
  );
}

/**
 * Subscribe to a command. Handler is kept current via useRef to avoid
 * re-subscribing on every render (solves stale closure / double-fire bug).
 *
 * Usage:
 *   useCommandListener("mail:archive", () => archiveSelected());
 */
export function useCommandListener(commandId: string, handler: () => void) {
  // Keep handler current without re-registering the event listener
  const savedHandler = useRef(handler);
  useEffect(() => {
    savedHandler.current = handler;
  }, [handler]);

  // Stable subscription (only re-subscribes if commandId changes)
  useEffect(() => {
    const fn = (e: CustomEvent<{ commandId: string }>) => {
      if (e.detail.commandId === commandId) savedHandler.current();
    };
    window.addEventListener("app:command", fn as EventListener);
    return () => window.removeEventListener("app:command", fn as EventListener);
  }, [commandId]);
}
