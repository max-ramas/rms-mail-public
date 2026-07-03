"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE, type Email } from "./useEmailTypes";
import type { InfiniteCache } from "./types";
import {
  restoreInfiniteLists,
  setInfiniteLists,
  snapshotInfiniteLists,
} from "@/lib/query-cache";

export function useSnoozeEmail() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      emailId,
      minutes,
    }: {
      emailId: string;
      minutes: number;
    }) => {
      await axios.post(`${API_BASE}/api/emails/${emailId}/snooze`, {
        minutes,
      });
    },
    onMutate: async ({ emailId }: { emailId: string; minutes: number }) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      const prev = snapshotInfiniteLists(queryClient);
      setInfiniteLists(queryClient, (old: InfiniteCache | undefined) => {
        if (!old?.pages) return old;
        return {
          ...old,
          pages: old.pages.map((page) => ({
            ...page,
            items: page.items.filter((e: Email) => e.id !== emailId),
          })),
        };
      });
      return { prev };
    },
    onError: (
      _err: Error,
      _vars: { emailId: string; minutes: number },
      context: { prev?: ReturnType<typeof snapshotInfiniteLists> } | undefined,
    ) => {
      if (context?.prev) restoreInfiniteLists(queryClient, context.prev);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
      queryClient.refetchQueries({ queryKey: ["folders"], exact: false });
      queryClient.invalidateQueries({ queryKey: ["groups"] });
    },
  });
}
