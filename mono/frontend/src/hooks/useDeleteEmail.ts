"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE, type Email, type Folder } from "./useEmailTypes";
import type { InfiniteCache } from "./types";
import {
  restoreFolders,
  setAllFolders,
  snapshotFolders,
} from "@/lib/query-cache";

type DeleteVars = { emailId: string; accountId?: string };
type DeleteContext = {
  prev: [queryKey: readonly unknown[], data: InfiniteCache | undefined][];
  prevFolders: ReturnType<typeof snapshotFolders>;
  deletedEmailInfo: { folder_id: string; was_unread: boolean } | null;
};

export function useDeleteEmail() {
  const queryClient = useQueryClient();

  return useMutation<string, Error, DeleteVars, DeleteContext>({
    mutationFn: async (p) => {
      const url = `${API_BASE}/api/emails/${p.emailId}`;
      await axios.delete(
        url,
        p.accountId ? { params: { account_id: p.accountId } } : undefined,
      );
      return p.emailId;
    },
    onMutate: async ({ emailId }) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      await queryClient.cancelQueries({ queryKey: ["folders"] });

      const prev = queryClient.getQueriesData<InfiniteCache>({
        queryKey: ["emails-infinite"],
      });

      // Find the email being deleted to know folder_id and unread status
      let deletedEmailInfo: {
        folder_id: string;
        was_unread: boolean;
      } | null = null;
      for (const [, data] of prev) {
        if (!data?.pages) continue;
        for (const page of data.pages) {
          const email = page.items.find((e: Email) => e.id === emailId);
          if (email) {
            deletedEmailInfo = {
              folder_id: email.folder_id,
              was_unread: !email.is_read,
            };
            break;
          }
        }
        if (deletedEmailInfo) break;
      }

      // Optimistically remove email from the list
      queryClient.setQueriesData<InfiniteCache>(
        { queryKey: ["emails-infinite"] },
        (old) => {
          if (!old?.pages) return old;
          return {
            ...old,
            pages: old.pages.map((page) => ({
              ...page,
              items: page.items.filter((e: Email) => e.id !== emailId),
            })),
          };
        },
      );

      const prevFolders = snapshotFolders(queryClient);

      if (deletedEmailInfo && deletedEmailInfo.was_unread) {
        setAllFolders(queryClient, (oldFolders) => {
          if (!oldFolders || !Array.isArray(oldFolders)) return oldFolders;
          return oldFolders.map((f: Folder) => {
            if (f.id === deletedEmailInfo!.folder_id) {
              return {
                ...f,
                unread_count: Math.max(0, (f.unread_count || 0) - 1),
              };
            }
            return f;
          });
        });
      }

      return { prev, prevFolders, deletedEmailInfo };
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) {
        for (const [key, data] of context.prev) {
          queryClient.setQueryData(key, data);
        }
      }
      if (context?.prevFolders) {
        restoreFolders(queryClient, context.prevFolders);
      }
    },
    onSuccess: () => {
      // Optimistic update уже удалил письмо из списка и обновил счётчики папок.
      // Повторный запрос к серверу не нужен — следующий SSE-ивент подсинхронизирует.
    },
  });
}
