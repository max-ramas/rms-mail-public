"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { type Email } from "./useEmailTypes";
import {
  restoreEmailDetail,
  restoreInfiniteLists,
  setEmailDetail,
  setInfiniteLists,
  snapshotEmailDetail,
  snapshotInfiniteLists,
} from "@/lib/query-cache";

type ToggleField = "is_pinned" | "is_flagged" | "is_muted";

interface ToggleMutationOptions {
  endpoint: (emailId: string) => string;
  field: ToggleField;
  /** Also optimistically update the single-email cache (pin/flag). */
  updateEmailDetail?: boolean;
}

export function useEmailToggleMutation({
  endpoint,
  field,
  updateEmailDetail = true,
}: ToggleMutationOptions) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (emailId: string) => {
      const response = await axios.post(endpoint(emailId));
      return response.data;
    },
    onMutate: async (emailId: string) => {
      await queryClient.cancelQueries({ queryKey: ["emails-infinite"] });
      if (updateEmailDetail) {
        await queryClient.cancelQueries({ queryKey: ["email", emailId] });
      }

      const prevInfinite = snapshotInfiniteLists(queryClient);
      const prevEmail = updateEmailDetail
        ? snapshotEmailDetail(queryClient, emailId)
        : undefined;

      setInfiniteLists(queryClient, (old) => {
        if (!old?.pages) return old;
        return {
          ...old,
          pages: old.pages.map((page) => ({
            ...page,
            items: page.items.map((email: Email) =>
              email.id === emailId
                ? { ...email, [field]: !email[field] }
                : email,
            ),
          })),
        };
      });

      if (updateEmailDetail) {
        setEmailDetail(queryClient, emailId, (oldData) => {
          if (!oldData?.email) return oldData;
          return {
            ...oldData,
            email: { ...oldData.email, [field]: !oldData.email[field] },
          };
        });
      }

      return { prevInfinite, prevEmail };
    },
    onError: (
      _err,
      emailId,
      context:
        | {
            prevInfinite?: ReturnType<typeof snapshotInfiniteLists>;
            prevEmail?: ReturnType<typeof snapshotEmailDetail>;
          }
        | undefined,
    ) => {
      if (context?.prevInfinite) {
        restoreInfiniteLists(queryClient, context.prevInfinite);
      }
      if (updateEmailDetail && context?.prevEmail) {
        restoreEmailDetail(queryClient, emailId, context.prevEmail);
      }
    },
    onSettled: (_data, _error, emailId) => {
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
      if (updateEmailDetail) {
        queryClient.invalidateQueries({ queryKey: ["email", emailId] });
      }
      if (field === "is_flagged") {
        queryClient.invalidateQueries({ queryKey: ["email-folder-counts"] });
      }
      queryClient.invalidateQueries({ queryKey: ["folders"] });
      queryClient.invalidateQueries({ queryKey: ["groups"] });
      if (!updateEmailDetail) {
        queryClient.invalidateQueries({ queryKey: ["emails"] });
        queryClient.invalidateQueries({ queryKey: ["accounts"] });
        queryClient.refetchQueries({ queryKey: ["folders"], exact: false });
      }
    },
  });
}
