"use client";

/**
 * Singleton that manages SSE ticket lifecycle.
 *
 * Deduplicates concurrent fetch requests.
 * On failure, returns null so the caller can fall back to cookie-based auth.
 */
class TicketManager {
  private pendingFetch: Promise<string | null> | null = null;

  async getTicket(): Promise<string | null> {
    if (this.pendingFetch) {
      return this.pendingFetch;
    }

    this.pendingFetch = this.fetchTicket().finally(() => {
      this.pendingFetch = null;
    });

    return this.pendingFetch;
  }

  private async fetchTicket(): Promise<string | null> {
    try {
      const res = await fetch("/api/auth/ticket", {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) {
        console.warn(
          "[TicketManager] Failed to fetch ticket, falling back to cookie auth",
        );
        return null;
      }
      const data = await res.json();
      return data.ticket || null;
    } catch (error) {
      console.warn(
        "[TicketManager] Network error, falling back to cookie auth:",
        (error as Error).message,
      );
      return null;
    }
  }
}

export const ticketManager = new TicketManager();
