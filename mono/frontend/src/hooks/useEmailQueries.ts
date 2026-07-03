import {
  useQuery,
  useInfiniteQuery,
  keepPreviousData,
} from "@tanstack/react-query";
import { useMemo } from "react";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";
import type {
  Email,
  Account,
  Contact,
  Label,
  Folder,
  ProjectGroup,
  User,
  EmailComment,
  Attachment,
  FilterRule,
} from "./useEmailTypes";
import type { EmailListPage } from "./types";
import type { InfiniteData } from "@tanstack/react-query";

const EDITION_KEY = "rms_edition";

function countUniqueEmails(
  data: InfiniteData<EmailListPage> | undefined,
): number {
  const pages = data?.pages ?? [];
  const seen = new Set<string>();
  let count = 0;
  for (const page of pages) {
    for (const email of page.items ?? []) {
      if (seen.has(email.id)) continue;
      seen.add(email.id);
      count++;
    }
  }
  return count;
}

const MAIL_STATUS_POLL_MS = 30_000;

function syncWarmupRefetchInterval(
  data: InfiniteData<EmailListPage> | undefined,
  warmupExpired: boolean,
): number | false {
  const count = countUniqueEmails(data);
  const fast = count < 40 && (count > 0 || !warmupExpired);
  return fast ? 4_000 : false;
}

function listRefetchInterval(
  data: InfiniteData<EmailListPage> | undefined,
  warmupExpired: boolean,
): number | false {
  const fast = syncWarmupRefetchInterval(data, warmupExpired);
  if (fast !== false) return fast;
  return MAIL_STATUS_POLL_MS;
}

function clientEdition(): string {
  if (process.env.NEXT_PUBLIC_EDITION) {
    return process.env.NEXT_PUBLIC_EDITION;
  }
  if (typeof window === "undefined") return "unified";
  return localStorage.getItem(EDITION_KEY) || "unified";
}

function groupsQueryEnabled(): boolean {
  if (typeof window === "undefined") return false;
  if (window.location.host.startsWith("wm.")) return false;
  return !clientEdition().toLowerCase().startsWith("m");
}

// Keyset pagination: next cursor travels with each page (no shared mutable state)

export function useEmailsInfinite(
  unified: boolean = true,
  accountId?: string,
  folderId?: string,
  folderName?: string,
  groupId?: string,
  filters?: Record<string, string>,
  options?: { warmupExpired?: boolean },
) {
  const warmupExpired = options?.warmupExpired ?? false;
  return useInfiniteQuery({
    queryKey: [
      "emails-infinite",
      unified,
      accountId,
      folderId,
      folderName,
      groupId,
      filters ? JSON.stringify(filters) : undefined,
    ],
    queryFn: async ({ pageParam = "" }) => {
      const params = new URLSearchParams();
      if (pageParam) {
        params.append("cursor", pageParam);
      }
      params.append("limit", unified ? "250" : "50");
      if (groupId) {
        params.append("group_id", groupId);
        if (folderName) params.append("folder", folderName);
      } else if (unified) {
        params.append("unified", "true");
        if (folderName) params.append("folder", folderName);
      } else if (accountId) {
        params.append("account_id", accountId);
        if (folderId) params.append("folder_id", folderId);
      }
      if (filters) {
        for (const [k, v] of Object.entries(filters)) {
          if (v) params.append(k, v);
        }
      }
      const response = await axios.get(`${API_BASE}/api/emails?${params}`);
      const hdr =
        response.headers["x-next-cursor"] || response.headers["X-Next-Cursor"];
      const nextCursor = typeof hdr === "string" ? hdr : "";
      return {
        items: response.data as Email[],
        nextCursor,
      } satisfies EmailListPage;
    },
    initialPageParam: "",
    getNextPageParam: (lastPage) => {
      if (!lastPage?.items?.length) return undefined;
      const step = unified ? 250 : 50;
      if (lastPage.items.length < step) return undefined;
      if (!lastPage.nextCursor) return undefined;
      return lastPage.nextCursor;
    },
    placeholderData: keepPreviousData,
    staleTime: 15_000,
    refetchOnMount: true,
    refetchInterval: (query) =>
      listRefetchInterval(query.state.data, warmupExpired),
  });
}

export function useAccounts() {
  return useQuery({
    queryKey: ["accounts"],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/accounts`);
      return r.data as Account[];
    },
    staleTime: 10 * 1000,
  });
}

export function useEmail(id: string | null) {
  return useQuery({
    queryKey: ["email", id],
    queryFn: async () => {
      if (!id) return null;
      const r = await axios.get(`${API_BASE}/api/emails/${id}`);
      return r.data as {
        email: Email;
        body: string;
        html: string;
        attachments: Attachment[];
        thread_emails?: Email[];
        tags?: string[];
      };
    },
    enabled: !!id,
    staleTime: 60_000,
    refetchInterval: MAIL_STATUS_POLL_MS,
  });
}

export function useSearchEmails(
  query: string,
  accountId?: string,
  folderId?: string,
) {
  return useQuery({
    queryKey: ["search", query, accountId, folderId],
    queryFn: async () => {
      if (!query.trim()) return [];
      const params = new URLSearchParams();
      params.set("q", query);
      if (accountId) params.set("account_id", accountId);
      if (folderId) params.set("folder_id", folderId);
      const r = await axios.get(`${API_BASE}/api/search?${params}`);
      return r.data as Email[];
    },
    enabled: query.trim().length > 0,
  });
}

export function useFolders(accountId: string) {
  return useQuery({
    queryKey: ["folders", accountId],
    queryFn: async () => {
      const response = await axios.get(
        `${API_BASE}/api/folders?account_id=${accountId}`,
      );
      return response.data as Folder[];
    },
    enabled: !!accountId,
    staleTime: 10 * 1000,
  });
}

export function useLabels(accountId?: string) {
  return useQuery({
    queryKey: ["labels", accountId || ""],
    queryFn: async () => {
      const url =
        accountId && accountId !== "undefined"
          ? `${API_BASE}/api/labels?account_id=${accountId}`
          : `${API_BASE}/api/labels`;
      const res = await axios.get(url);
      return res.data as Label[];
    },
  });
}

export function useGroups() {
  return useQuery({
    queryKey: ["groups"],
    queryFn: async () => {
      const res = await axios.get(`${API_BASE}/api/groups`);
      return res.data as ProjectGroup[];
    },
    enabled: groupsQueryEnabled(),
  });
}

export function useGroupAccounts(groupId: string | null) {
  return useQuery({
    queryKey: ["group-accounts", groupId],
    queryFn: async () => {
      if (!groupId) return [];
      const r = await axios.get(`${API_BASE}/api/groups/accounts/${groupId}`);
      return r.data as string[];
    },
    enabled: !!groupId && groupsQueryEnabled(),
  });
}

export function useContacts(accountId?: string) {
  return useQuery({
    queryKey: ["contacts", accountId],
    queryFn: async () => {
      const params = accountId ? `?account_id=${accountId}` : "";
      const response = await axios.get(`${API_BASE}/api/contacts${params}`);
      return response.data as Contact[];
    },
    staleTime: 5 * 60 * 1000,
  });
}

export function useIdentities(accountId: string | undefined) {
  return useQuery({
    queryKey: ["identities", accountId],
    queryFn: async () => {
      if (!accountId) return [];
      const r = await axios.get(
        `${API_BASE}/api/identities?account_id=${accountId}`,
      );
      return r.data as Array<{
        id: string;
        account_id: string;
        email: string;
        name: string;
      }>;
    },
    enabled: !!accountId,
  });
}

export function useUsers() {
  return useQuery({
    queryKey: ["users"],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/users`);
      return r.data as User[];
    },
    enabled:
      typeof window !== "undefined" &&
      clientEdition().toLowerCase().startsWith("t"),
  });
}

export function useComments(emailId: string | null) {
  return useQuery({
    queryKey: ["comments", emailId],
    queryFn: async () => {
      if (!emailId) return [];
      const r = await axios.get(`${API_BASE}/api/comments/${emailId}`);
      return r.data as EmailComment[];
    },
    enabled: !!emailId,
  });
}

export function useEmailTags(id: string | null) {
  return useQuery({
    queryKey: ["email-tags", id],
    queryFn: async () => {
      if (!id) return [];
      const r = await axios.get(`${API_BASE}/api/emails/${id}/tags`);
      return r.data.tags as string[];
    },
    enabled: !!id,
  });
}

export function useBatchEmailLabels(emailIds: string[]) {
  const batchKey = useMemo(() => emailIds.join(","), [emailIds]);
  return useQuery({
    queryKey: ["email-labels", "batch", batchKey],
    queryFn: async () => {
      if (emailIds.length === 0) return {};
      const merged: Record<string, Label[]> = {};
      const chunkSize = 200;
      for (let i = 0; i < emailIds.length; i += chunkSize) {
        const chunk = emailIds.slice(i, i + chunkSize);
        const res = await axios.get(
          `${API_BASE}/api/email-labels/batch?ids=${chunk.join(",")}`,
        );
        Object.assign(merged, res.data as Record<string, Label[]>);
      }
      return merged;
    },
    enabled: emailIds.length > 0,
    staleTime: 60 * 1000,
  });
}

export function useBatchEmailTags(emailIds: string[]) {
  const batchKey = useMemo(() => emailIds.join(","), [emailIds]);
  return useQuery({
    queryKey: ["email-tags", "batch", batchKey],
    queryFn: async () => {
      if (emailIds.length === 0) return {};
      const merged: Record<string, string[]> = {};
      const chunkSize = 200;
      for (let i = 0; i < emailIds.length; i += chunkSize) {
        const chunk = emailIds.slice(i, i + chunkSize);
        const res = await axios.get(
          `${API_BASE}/api/email-tags/batch?ids=${chunk.join(",")}`,
        );
        Object.assign(merged, res.data as Record<string, string[]>);
      }
      return merged;
    },
    enabled: emailIds.length > 0,
    staleTime: 60 * 1000,
  });
}

export function useRules(accountId: string) {
  return useQuery({
    queryKey: ["rules", accountId],
    queryFn: async () => {
      const res = await axios.get(
        `${API_BASE}/api/rules?account_id=${accountId}`,
      );
      return res.data as FilterRule[];
    },
    enabled: !!accountId,
  });
}

export function useLicenseInfo() {
  return useQuery({
    queryKey: ["license"],
    queryFn: async () => {
      try {
        const res = await axios.get(`${API_BASE}/api/license`);
        return res.data as {
          status: string;
          instance_uid: string;
          expires_at?: number;
          latest_version?: string;
          release_notes?: string;
          app_version?: string;
          update_channel?: string;
        };
      } catch {
        return {
          status: "unlicensed",
          instance_uid: "",
          latest_version: "",
          release_notes: "",
        };
      }
    },
    staleTime: 60_000,
  });
}
