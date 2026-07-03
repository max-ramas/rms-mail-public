"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";

export function useSaveDraft() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (draft: {
      id?: string;
      account_id: string;
      to: string;
      cc: string;
      bcc: string;
      subject: string;
      body: string;
      html: string;
      in_reply_to?: string;
      sync_remote?: boolean;
    }) => {
      const response = await axios.post(`${API_BASE}/api/emails/draft`, draft, {
        headers: { "X-Account-Id": draft.account_id },
      });
      return response.data as { id: string };
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}
