import { useCallback, useEffect, useRef } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import axios from "axios";
import "@/lib/api-client";
import {
  patchEmailDetailFlags,
  patchEmailInInfiniteLists,
} from "@/lib/query-cache";
import { useSSETicket } from "./useSSETicket";

export { API_BASE } from "./useEmailTypes";
export type {
  Email,
  Account,
  Contact,
  Label,
  FilterRule,
  ProjectGroup,
  User,
  EmailComment,
  Attachment,
  Folder,
  AIMessage,
  AILogEntry,
  AILogStats,
  Identity,
} from "./useEmailTypes";

// ── Edition ─────────────────────────────────────────────────────────
const EDITION_KEY = "rms_edition";
export function isAIDisabled(): boolean {
  if (typeof window === "undefined") return false;
  return localStorage.getItem("rms_ai_disabled") === "true";
}

export function getEdition(): string {
  if (process.env.NEXT_PUBLIC_EDITION) {
    return process.env.NEXT_PUBLIC_EDITION;
  }

  if (typeof window === "undefined") return "unified";
  return localStorage.getItem(EDITION_KEY) || "unified";
}

export function setEdition(edition: string): void {
  localStorage.setItem(EDITION_KEY, edition);
}

export async function fetchEdition(): Promise<string> {
  try {
    const r = await fetch("/api/auth/edition");
    const d = await r.json();
    const ed = d.edition || "unified";
    setEdition(ed);
    if (typeof window !== "undefined") {
      localStorage.setItem("LLM_ENVONLY", String(d.llm_envonly === true));
      localStorage.setItem("TG_ENVONLY", String(d.tg_envonly === true));
      localStorage.setItem("rms_ai_disabled", String(d.ai_disabled === true));
    }
    return ed;
  } catch {
    return getEdition();
  }
}

export function getEnvOnlyFlags(): { llm: boolean; tg: boolean } {
  if (typeof window === "undefined") return { llm: false, tg: false };
  return {
    llm: localStorage.getItem("LLM_ENVONLY") === "true",
    tg: localStorage.getItem("TG_ENVONLY") === "true",
  };
}

export function editionLetter(): string {
  const e = getEdition().toLowerCase();
  if (e === "mono_pro" || e === "monopro") return "MP";
  if (e.startsWith("m")) return "M";
  if (e.startsWith("t")) return "T";
  return "U";
}

axios.interceptors.response.use(
  (response) => response,
  (error) => {
    // AuthGuard handles 401 redirects per-route.
    // Global redirect removed to avoid race conditions with httpOnly cookie auth.
    return Promise.reject(error);
  },
);

const MAIL_LIST_REFRESH_MS = 400;
const MAIL_POLL_INTERVAL_MS = 30_000;

function refetchMailLists(queryClient: QueryClient) {
  return Promise.all([
    queryClient.refetchQueries({
      predicate: (query) => query.queryKey[0] === "emails-infinite",
      type: "active",
    }),
    queryClient.refetchQueries({ queryKey: ["folders"], type: "active" }),
    queryClient.refetchQueries({ queryKey: ["accounts"], type: "active" }),
    queryClient.refetchQueries({
      queryKey: ["email-folder-counts"],
      type: "active",
    }),
  ]);
}

async function refreshMailInbox(
  queryClient: QueryClient,
  opts?: { resetPages?: boolean },
) {
  if (opts?.resetPages) {
    await queryClient.resetQueries({ queryKey: ["emails-infinite"] });
  } else {
    await queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
  }
  await refetchMailLists(queryClient);
}

/** Periodic fallback when SSE is down — keeps list and counters in sync. */
export function useMailPeriodicRefresh(enabled = true) {
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!enabled) return;
    const id = setInterval(() => {
      void refreshMailInbox(queryClient);
    }, MAIL_POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [enabled, queryClient]);
}

// ── New Email Event (ticket-based SSE) ─────────────────
export function useNewEmailEvent(enabled = true) {
  const queryClient = useQueryClient();
  const listRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );
  const notificationTimerRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const notificationCount = useRef(0);
  const lastNotificationSender = useRef("");
  const lastNotificationSubject = useRef("");
  const warmupUntil = useRef(0);

  // skip SSE-triggered list refreshes for first 5s after mount
  useEffect(() => {
    warmupUntil.current = Date.now() + 5000;
  }, []);

  const refreshListNow = useCallback(
    (opts?: { resetPages?: boolean }) => {
      if (Date.now() < warmupUntil.current) return; // skip during warmup
      if (listRefreshTimerRef.current) {
        clearTimeout(listRefreshTimerRef.current);
        listRefreshTimerRef.current = null;
      }
      void refreshMailInbox(queryClient, opts);
    },
    [queryClient],
  );

  const scheduleListRefresh = useCallback(
    (opts?: { resetPages?: boolean }) => {
      if (Date.now() < warmupUntil.current) return; // skip during warmup
      if (listRefreshTimerRef.current) {
        clearTimeout(listRefreshTimerRef.current);
      }
      listRefreshTimerRef.current = setTimeout(() => {
        listRefreshTimerRef.current = null;
        void refreshMailInbox(queryClient, opts);
      }, MAIL_LIST_REFRESH_MS);
    },
    [queryClient],
  );

  useEffect(() => {
    return () => {
      if (listRefreshTimerRef.current)
        clearTimeout(listRefreshTimerRef.current);
      if (notificationTimerRef.current)
        clearTimeout(notificationTimerRef.current);
    };
  }, []);

  const handleSSE = useCallback(
    (type: string, event: MessageEvent) => {
      switch (type) {
        case "new-email": {
          notificationCount.current++;

          let isRead = true;
          try {
            const data = JSON.parse(event.data);
            lastNotificationSender.current =
              data.sender_name || data.sender_address || "";
            lastNotificationSubject.current = data.subject || "";
            isRead = data.is_read === true || data.is_read === "true";
          } catch {
            /* ignore parse errors */
          }

          // Reset to page 1 and refresh list + counters atomically.
          refreshListNow({ resetPages: true });

          if (isRead) {
            notificationCount.current = 0;
            return;
          }

          if (notificationTimerRef.current) {
            clearTimeout(notificationTimerRef.current);
          }
          notificationTimerRef.current = setTimeout(() => {
            notificationTimerRef.current = null;
            try {
              const notificationsEnabled =
                localStorage.getItem("rms-mail_notifications") === "true";
              if (
                notificationsEnabled &&
                typeof Notification !== "undefined" &&
                Notification.permission === "granted"
              ) {
                if (notificationCount.current === 1) {
                  new Notification(
                    "New email from: " + lastNotificationSender.current,
                    {
                      body: lastNotificationSubject.current || "New message",
                      icon: "/favicon.ico",
                    },
                  );
                } else if (notificationCount.current > 1) {
                  new Notification(
                    `You have ${notificationCount.current} new emails`,
                    {
                      body: "Click to view your inbox",
                      icon: "/favicon.ico",
                    },
                  );
                }
              }
            } catch {
              /* ignore */
            }
            notificationCount.current = 0;
          }, 2000);
          break;
        }

        case "email_updated": {
          try {
            const data = JSON.parse(event.data);
            if (data.email_id) {
              const flagPatch: {
                is_read?: boolean;
                is_flagged?: boolean;
                is_answered?: boolean;
                is_pinned?: boolean;
                is_muted?: boolean;
              } = {};
              if (typeof data.is_read === "boolean") {
                flagPatch.is_read = data.is_read;
              }
              if (typeof data.is_flagged === "boolean") {
                flagPatch.is_flagged = data.is_flagged;
              }
              if (typeof data.is_answered === "boolean") {
                flagPatch.is_answered = data.is_answered;
              }
              if (typeof data.is_pinned === "boolean") {
                flagPatch.is_pinned = data.is_pinned;
              }
              if (typeof data.is_muted === "boolean") {
                flagPatch.is_muted = data.is_muted;
              }
              if (Object.keys(flagPatch).length > 0) {
                patchEmailInInfiniteLists(
                  queryClient,
                  data.email_id,
                  flagPatch,
                );
                patchEmailDetailFlags(queryClient, data.email_id, flagPatch);
                // Cache patched — skip full list refresh for flag-only changes
                // When folder_id is present (move), full refresh is still needed
                if (typeof data.folder_id === "string") {
                  scheduleListRefresh();
                }
                return;
              } else {
                queryClient.invalidateQueries({
                  queryKey: ["email", data.email_id],
                });
              }
            }
          } catch (err) {
            if (process.env.NODE_ENV === "development")
              console.error("Failed to parse email_updated event", err);
          }
          scheduleListRefresh();
          break;
        }

        case "folder_updated": {
          scheduleListRefresh({ resetPages: true });
          break;
        }

        case "emails_bulk_updated": {
          let action = "";
          try {
            action = JSON.parse(event.data).action ?? "";
          } catch {
            /* ignore */
          }
          if (action === "reset_sync") {
            refreshListNow({ resetPages: true });
          } else {
            scheduleListRefresh();
          }
          break;
        }

        case "draft_ready": {
          try {
            const data = JSON.parse(event.data);
            if (data.email_id) {
              queryClient.invalidateQueries({
                queryKey: ["email", data.email_id],
              });
            }
          } catch (err) {
            if (process.env.NODE_ENV === "development")
              console.error("Failed to parse draft_ready event", err);
          }
          break;
        }
      }
    },
    [queryClient, refreshListNow, scheduleListRefresh],
  );

  useSSETicket({
    url: "/api/events",
    events: [
      "new-email",
      "email_updated",
      "emails_bulk_updated",
      "folder_updated",
      "draft_ready",
    ],
    onEvent: handleSSE,
    enabled,
    onOpen: useCallback(() => {
      refreshListNow({ resetPages: true });
    }, [refreshListNow]),
  });
}

// ── Re-exports from modular hooks ───────────────────────────────────

// Re-export queries
export {
  useEmailsInfinite,
  useAccounts,
  useEmail,
  useSearchEmails,
  useFolders,
  useLabels,
  useGroups,
  useGroupAccounts,
  useContacts,
  useIdentities,
  useUsers,
  useComments,
  useEmailTags,
  useBatchEmailLabels,
  useBatchEmailTags,
  useRules,
} from "./useEmailQueries";

// Re-export mutations
export {
  useMarkEmailRead,
  useFlagEmail,
  usePinEmail,
  useMuteEmail,
  useSnoozeEmail,
  useMoveEmailToFolder,
  useDeleteEmail,
  useSendEmail,
  useSummarizeEmail,
  useCategorizeEmail,
  useClearDraftReply,
  useSetEmailLabels,
  useAssignEmail,
  useUnassignEmail,
  useSaveDraft,
  useCreateComment,
  useDeleteComment,
} from "./useEmailMutations";

// Re-export AI hooks
export {
  useAIChat,
  useAICategorize,
  useAIStats,
  useAILog,
  useResetAIStats,
  useAISettings,
  useSaveAISettings,
} from "./useAIApi";

// Re-export admin hooks
export {
  useCreateAccount,
  useOAuthURL,
  useOAuthCallback,
  useDeleteAccount,
  useResetAccountSync,
  usePauseAccountSync,
  useResumeAccountSync,
  useUpdateAccount,
  useCreateIdentity,
  useDeleteIdentity,
  useCreateLabel,
  useUpdateLabel,
  useDeleteLabel,
  useCreateRule,
  useUpdateRule,
  useDeleteRule,
  useCreateGroup,
  useUpdateGroup,
  useDeleteGroup,
  useSetGroupAccounts,
  useCreateUser,
  useDeleteUser,
  useCreateContact,
  useUpdateContact,
  useDeleteContact,
} from "./useAdminQueries";
