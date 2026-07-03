"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE, type Email } from "./useEmailTypes";
import {
  restoreFolders,
  restoreInfiniteLists,
  setInfiniteLists,
  snapshotFolders,
  snapshotInfiniteLists,
} from "@/lib/query-cache";

export function useMoveEmailToFolder() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      emailId,
      folderId,
    }: {
      emailId: string;
      folderId: string;
    }) => {
      await axios.post(`${API_BASE}/api/emails/${emailId}/move`, {
        folder_id: folderId,
      });
      return { emailId, folderId };
    },
    onMutate: async ({ emailId }: { emailId: string; folderId: string }) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      await queryClient.cancelQueries({ queryKey: ["folders"] });
      const prevInfinite = snapshotInfiniteLists(queryClient);
      const prevFolders = snapshotFolders(queryClient);
      setInfiniteLists(queryClient, (old) => {
        if (!old?.pages) return old;
        return {
          ...old,
          pages: old.pages.map((page) => ({
            ...page,
            items: page.items.filter((e: Email) => e.id !== emailId),
          })),
        };
      });
      return { prevInfinite, prevFolders };
    },
    onError: (
      _err: Error,
      _vars: { emailId: string; folderId: string },
      context:
        | {
            prevInfinite?: ReturnType<typeof snapshotInfiniteLists>;
            prevFolders?: ReturnType<typeof snapshotFolders>;
          }
        | undefined,
    ) => {
      if (context?.prevInfinite) {
        restoreInfiniteLists(queryClient, context.prevInfinite);
      }
      if (context?.prevFolders) {
        restoreFolders(queryClient, context.prevFolders);
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["emails"] });
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
      queryClient.refetchQueries({ queryKey: ["folders"], exact: false });
      queryClient.invalidateQueries({ queryKey: ["groups"] });
    },
  });
}
