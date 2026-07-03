"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import {
  API_BASE,
  type Email,
  type Folder,
  type Account,
} from "./useEmailTypes";
import {
  restoreEmailDetail,
  restoreFolders,
  restoreInfiniteLists,
  setAllFolders,
  setEmailDetail,
  setInfiniteLists,
  snapshotEmailDetail,
  snapshotFolders,
  snapshotInfiniteLists,
} from "@/lib/query-cache";
import { useToast } from "@/hooks/useToast";

export function useBulkEmailAction() {
  const queryClient = useQueryClient();
  const toast = useToast();

  return useMutation({
    mutationFn: async ({
      action,
      ids,
      folderId,
      setFlagged,
      filter,
    }: {
      action: "delete" | "read" | "unread" | "flag" | "archive" | "move";
      ids?: string[];
      folderId?: string;
      setFlagged?: boolean;
      filter?: {
        account_id: string;
        filter_folder_id?: string;
        unified?: boolean;
      };
    }) => {
      if (filter) {
        await axios.post(`${API_BASE}/api/emails/bulk`, {
          action,
          ids: [],
          folder_id: folderId,
          account_id: filter.account_id,
          filter_folder_id: filter.filter_folder_id || "",
          unified: filter.unified || false,
          ...(setFlagged !== undefined ? { set_flagged: setFlagged } : {}),
        });
        return;
      }
      if (!ids || ids.length === 0) return;
      await axios.post(`${API_BASE}/api/emails/bulk`, {
        action,
        ids,
        folder_id: folderId,
        ...(setFlagged !== undefined ? { set_flagged: setFlagged } : {}),
      });
    },
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      const prevInfinite = snapshotInfiniteLists(queryClient);

      if (vars.filter) {
        if (
          vars.action === "delete" ||
          vars.action === "archive" ||
          vars.action === "move"
        ) {
          setInfiniteLists(queryClient, (oldData) => {
            if (!oldData?.pages) return oldData;
            return {
              ...oldData,
              pages: oldData.pages.map((page) => ({ ...page, items: [] })),
            };
          });
        } else {
          setInfiniteLists(queryClient, (oldData) => {
            if (!oldData?.pages) return oldData;
            return {
              ...oldData,
              pages: oldData.pages.map((page) => ({
                ...page,
                items: page.items.map((email: Email) => {
                  const updated = { ...email };
                  if (vars.action === "read") updated.is_read = true;
                  if (vars.action === "unread") updated.is_read = false;
                  if (vars.action === "flag") {
                    updated.is_flagged =
                      vars.setFlagged !== undefined
                        ? vars.setFlagged
                        : !email.is_flagged;
                  }
                  return updated;
                }),
              })),
            };
          });
        }
        return { prevInfinite };
      }

      if (!vars.ids || vars.ids.length === 0) return { prevInfinite };

      await queryClient.cancelQueries({ queryKey: ["folders"] });
      await queryClient.cancelQueries({ queryKey: ["accounts"] });

      const prevFolders = snapshotFolders(queryClient);
      const prevAccounts = queryClient.getQueryData(["accounts"]);

      const emailCache: Record<
        string,
        {
          prev: Email;
          next: Email;
        }
      > = {};
      const prevEmails: Record<
        string,
        ReturnType<typeof snapshotEmailDetail>
      > = {};

      for (const id of vars.ids) {
        await queryClient.cancelQueries({ queryKey: ["email", id] });
        prevEmails[id] = snapshotEmailDetail(queryClient, id);
      }

      if (vars.action === "delete") {
        setInfiniteLists(queryClient, (oldData) => {
          if (!oldData?.pages) return oldData;
          return {
            ...oldData,
            pages: oldData.pages.map((page) => ({
              ...page,
              items: page.items.filter(
                (email: Email) => !vars.ids?.includes(email.id),
              ),
            })),
          };
        });

        const deletedFolders: Record<string, { unread_delta: number }> = {};
        for (const [, data] of prevInfinite) {
          if (!data?.pages) continue;
          for (const page of data.pages) {
            for (const email of page.items) {
              if (vars.ids?.includes(email.id) && !email.is_read) {
                deletedFolders[email.folder_id] ??= { unread_delta: 0 };
                deletedFolders[email.folder_id].unread_delta -= 1;
              }
            }
          }
        }

        setAllFolders(queryClient, (oldFolders) => {
          if (!oldFolders || !Array.isArray(oldFolders)) return oldFolders;
          return oldFolders.map((f: Folder) => {
            const delta = deletedFolders[f.id]?.unread_delta;
            if (!delta) return f;
            return {
              ...f,
              unread_count: Math.max(0, (f.unread_count || 0) + delta),
            };
          });
        });
      } else if (vars.action === "archive" || vars.action === "move") {
        setInfiniteLists(queryClient, (oldData) => {
          if (!oldData?.pages) return oldData;
          return {
            ...oldData,
            pages: oldData.pages.map((page) => ({
              ...page,
              items: page.items.filter(
                (email: Email) => !vars.ids?.includes(email.id),
              ),
            })),
          };
        });
      } else {
        // Read / Unread / Flag — обновляем поля писем в кэше
        setInfiniteLists(queryClient, (oldData) => {
          if (!oldData?.pages) return oldData;
          return {
            ...oldData,
            pages: oldData.pages.map((page) => ({
              ...page,
              items: page.items.map((email: Email) => {
                if (vars.ids?.includes(email.id)) {
                  const updatedEmail = { ...email };
                  if (vars.action === "read") updatedEmail.is_read = true;
                  if (vars.action === "unread") updatedEmail.is_read = false;
                  if (vars.action === "flag") {
                    updatedEmail.is_flagged =
                      vars.setFlagged !== undefined
                        ? vars.setFlagged
                        : !email.is_flagged;
                  }

                  emailCache[email.id] = {
                    prev: email,
                    next: updatedEmail,
                  };
                  return updatedEmail;
                }
                return email;
              }),
            })),
          };
        });
      }

      for (const id of vars.ids) {
        if (!emailCache[id] && prevEmails[id]) {
          for (const [, data] of prevEmails[id]) {
            if (data?.email) {
              const prevEmailAsEmail = data.email;
              emailCache[id] = {
                prev: prevEmailAsEmail,
                next: { ...prevEmailAsEmail },
              };
              if (vars.action === "read") emailCache[id].next.is_read = true;
              if (vars.action === "unread") emailCache[id].next.is_read = false;
              if (vars.action === "flag") {
                emailCache[id].next.is_flagged =
                  vars.setFlagged !== undefined
                    ? vars.setFlagged
                    : !prevEmailAsEmail.is_flagged;
              }
              break;
            }
          }
        }

        if (emailCache[id]) {
          setEmailDetail(queryClient, id, (old) => {
            if (!old?.email) return old;
            return { ...old, email: emailCache[id].next };
          });
        }
      }

      if (vars.action === "read" || vars.action === "unread") {
        setAllFolders(queryClient, (oldFolders) => {
          if (!oldFolders || !Array.isArray(oldFolders)) return oldFolders;
          return oldFolders.map((f: Folder) => {
            let diff = 0;
            for (const id of vars.ids!) {
              const ec = emailCache[id];
              if (ec && ec.prev.folder_id === f.id) {
                if (vars.action === "read" && !ec.prev.is_read) diff -= 1;
                if (vars.action === "unread" && ec.prev.is_read) diff += 1;
              }
            }
            if (diff === 0) return f;
            return {
              ...f,
              unread_count: Math.max(0, (f.unread_count || 0) + diff),
            };
          });
        });

        queryClient.setQueryData(
          ["accounts"],
          (oldAccs: Account[] | undefined) => {
            if (!oldAccs || !Array.isArray(oldAccs)) return oldAccs;
            return oldAccs.map((a: Account) => {
              let diff = 0;
              for (const id of vars.ids!) {
                const ec = emailCache[id];
                if (ec && ec.prev.account_id === a.id) {
                  if (vars.action === "read" && !ec.prev.is_read) diff -= 1;
                  if (vars.action === "unread" && ec.prev.is_read) diff += 1;
                }
              }
              if (diff === 0) return a;
              return {
                ...a,
                unread_count: Math.max(0, (a.unread_count || 0) + diff),
              };
            });
          },
        );
      }

      return {
        prevInfinite,
        prevFolders,
        prevAccounts,
        prevEmails,
        ids: vars.ids,
      };
    },
    onError: (
      err,
      vars,
      context:
        | {
            prevInfinite?: ReturnType<typeof snapshotInfiniteLists>;
            prevEmails?: Record<string, ReturnType<typeof snapshotEmailDetail>>;
            prevFolders?: ReturnType<typeof snapshotFolders>;
            prevAccounts?: unknown;
            ids?: string[];
          }
        | undefined,
    ) => {
      toast.addToast("Bulk action failed", "error");
      if (context?.prevInfinite) {
        restoreInfiniteLists(queryClient, context.prevInfinite);
      }
      if (context?.prevFolders) {
        restoreFolders(queryClient, context.prevFolders);
      }
      if (context?.prevAccounts) {
        queryClient.setQueryData(["accounts"], context.prevAccounts);
      }
      if (context?.prevEmails && context?.ids) {
        for (const id of context.ids) {
          if (context.prevEmails[id]) {
            restoreEmailDetail(queryClient, id, context.prevEmails[id]);
          }
        }
      }
    },
    onSettled: (_data, _error, vars) => {
      if (vars.action !== "delete" && vars.action !== "archive" && vars.action !== "move") {
        queryClient.invalidateQueries({
          queryKey: ["emails-infinite"],
        });
      }
      queryClient.invalidateQueries({ queryKey: ["email-folder-counts"] });
      if (vars.ids) {
        for (const id of vars.ids) {
          queryClient.invalidateQueries({ queryKey: ["email", id] });
        }
      }
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
      queryClient.invalidateQueries({ queryKey: ["groups"] });
    },
  });
}
