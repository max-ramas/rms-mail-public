"use client";

import type { QueryClient } from "@tanstack/react-query";
import type { Email, Folder } from "@/hooks/useEmailTypes";
import type { InfiniteCache } from "@/hooks/types";

export type EmailFlagPatch = Partial<
  Pick<
    Email,
    "is_read" | "is_flagged" | "is_answered" | "is_pinned" | "is_muted"
  >
>;

export function patchEmailInInfiniteLists(
  qc: QueryClient,
  emailId: string,
  patch: EmailFlagPatch,
) {
  setInfiniteLists(qc, (old) => {
    if (!old?.pages) return old;
    let changed = false;
    const pages = old.pages.map((page) => ({
      ...page,
      items: page.items.map((email) => {
        if (email.id !== emailId) return email;
        changed = true;
        return { ...email, ...patch };
      }),
    }));
    return changed ? { ...old, pages } : old;
  });
}

export function patchEmailDetailFlags(
  qc: QueryClient,
  emailId: string,
  patch: EmailFlagPatch,
) {
  setEmailDetail(qc, emailId, (old) => {
    if (!old?.email) return old;
    return { ...old, email: { ...old.email, ...patch } };
  });
}

export function snapshotInfiniteLists(qc: QueryClient) {
  return qc.getQueriesData<InfiniteCache>({ queryKey: ["emails-infinite"] });
}

export function setInfiniteLists(
  qc: QueryClient,
  updater: (old: InfiniteCache | undefined) => InfiniteCache | undefined,
) {
  qc.setQueriesData<InfiniteCache>({ queryKey: ["emails-infinite"] }, updater);
}

export function snapshotFolders(qc: QueryClient) {
  return qc.getQueriesData<Folder[]>({ queryKey: ["folders"] });
}

export function setAllFolders(
  qc: QueryClient,
  updater: (old: Folder[] | undefined) => Folder[] | undefined,
) {
  qc.setQueriesData<Folder[]>({ queryKey: ["folders"] }, updater);
}

export type EmailDetailCache = {
  email: Email;
  body: string;
  html: string;
  attachments: unknown[];
  thread_emails?: Email[];
};

export function snapshotEmailDetail(qc: QueryClient, emailId: string) {
  return qc.getQueriesData<EmailDetailCache>({ queryKey: ["email", emailId] });
}

export function setEmailDetail(
  qc: QueryClient,
  emailId: string,
  updater: (old: EmailDetailCache | undefined) => EmailDetailCache | undefined,
) {
  qc.setQueriesData<EmailDetailCache>(
    { queryKey: ["email", emailId] },
    updater,
  );
}

export function restoreInfiniteLists(
  qc: QueryClient,
  snapshots: ReturnType<typeof snapshotInfiniteLists>,
) {
  for (const [key, data] of snapshots) {
    qc.setQueryData(key, data);
  }
}

export function restoreFolders(
  qc: QueryClient,
  snapshots: ReturnType<typeof snapshotFolders>,
) {
  for (const [key, data] of snapshots) {
    qc.setQueryData(key, data);
  }
}

export function restoreEmailDetail(
  qc: QueryClient,
  emailId: string,
  snapshots: ReturnType<typeof snapshotEmailDetail>,
) {
  for (const [key, data] of snapshots) {
    qc.setQueryData(key, data);
  }
}

export function isInsideModal(target: EventTarget | null): boolean {
  if (!target || !(target instanceof Element)) return false;
  return !!target.closest('[role="dialog"][aria-modal="true"]');
}
