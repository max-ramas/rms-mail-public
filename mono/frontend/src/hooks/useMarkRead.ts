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

export function useMarkEmailRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (emailId: string) => {
      await axios.post(`${API_BASE}/api/emails/${emailId}/read`);
    },
    onMutate: async (emailId: string) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      await queryClient.cancelQueries({ queryKey: ["email", emailId] });
      await queryClient.cancelQueries({ queryKey: ["folders"] });
      await queryClient.cancelQueries({ queryKey: ["accounts"] });

      const prevInfinite = snapshotInfiniteLists(queryClient);
      const prevEmail = snapshotEmailDetail(queryClient, emailId);
      const prevFolders = snapshotFolders(queryClient);
      const prevAccounts = queryClient.getQueryData(["accounts"]);

      let emailObj: Email | null = null;

      setInfiniteLists(queryClient, (oldData) => {
        if (!oldData?.pages) return oldData;
        return {
          ...oldData,
          pages: oldData.pages.map((page) => ({
            ...page,
            items: page.items.map((email: Email) => {
              if (email.id === emailId) {
                if (!emailObj) emailObj = email;
                return { ...email, is_read: true };
              }
              return email;
            }),
          })),
        };
      });

      if (!emailObj) {
        for (const [, data] of prevEmail) {
          if (data?.email) {
            emailObj = data.email;
            break;
          }
        }
      }

      setEmailDetail(queryClient, emailId, (oldData) => {
        if (!oldData?.email) return oldData;
        return { ...oldData, email: { ...oldData.email, is_read: true } };
      });

      if (emailObj && !emailObj.is_read) {
        setAllFolders(queryClient, (oldFolders) => {
          if (!oldFolders || !Array.isArray(oldFolders)) return oldFolders;
          return oldFolders.map((f: Folder) =>
            f.id === emailObj?.folder_id
              ? { ...f, unread_count: Math.max(0, (f.unread_count || 0) - 1) }
              : f,
          );
        });

        queryClient.setQueryData(
          ["accounts"],
          (oldAccs: Account[] | undefined) => {
            if (!oldAccs || !Array.isArray(oldAccs)) return oldAccs;
            return oldAccs.map((a: Account) =>
              a.id === emailObj?.account_id
                ? { ...a, unread_count: Math.max(0, (a.unread_count || 0) - 1) }
                : a,
            );
          },
        );
      }

      return { prevInfinite, prevEmail, prevFolders, prevAccounts };
    },
    onError: (
      err,
      emailId,
      context:
        | {
            prevInfinite?: ReturnType<typeof snapshotInfiniteLists>;
            prevEmail?: ReturnType<typeof snapshotEmailDetail>;
            prevFolders?: ReturnType<typeof snapshotFolders>;
            prevAccounts?: unknown;
          }
        | undefined,
    ) => {
      if (context?.prevInfinite) {
        restoreInfiniteLists(queryClient, context.prevInfinite);
      }
      if (context?.prevEmail) {
        restoreEmailDetail(queryClient, emailId, context.prevEmail);
      }
      if (context?.prevFolders) {
        restoreFolders(queryClient, context.prevFolders);
      }
      if (context?.prevAccounts) {
        queryClient.setQueryData(["accounts"], context.prevAccounts);
      }
    },
    onSettled: (data, error, emailId) => {
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
      queryClient.invalidateQueries({ queryKey: ["email", emailId] });
      queryClient.invalidateQueries({ queryKey: ["email-folder-counts"] });
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
      queryClient.invalidateQueries({ queryKey: ["groups"] });
    },
  });
}
