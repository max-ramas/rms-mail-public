"use client";

import { usePathname } from "next/navigation";
import { useNewEmailEvent, useMailPeriodicRefresh } from "@/hooks/useEmails";

function isPublicPath(pathname: string): boolean {
  return pathname.includes("/login") || pathname.includes("/setup");
}

/** Keeps inbox queries fresh while on settings or other routes during sync. */
export function GlobalMailSSE() {
  const pathname = usePathname();
  const enabled = !isPublicPath(pathname);
  const { connected } = useNewEmailEvent(enabled);
  // Poll only when SSE is down — avoid double refresh storms.
  useMailPeriodicRefresh(enabled && !connected);
  return null;
}
