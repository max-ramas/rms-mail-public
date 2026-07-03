"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";

export function useSendEmail() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (email: {
      account_id: string;
      to: string[];
      cc?: string[];
      subject: string;
      body: string;
      html?: string;
      from_identity?: string;
      in_reply_to?: string;
      references?: string;
      attachment_hashes?: string[];
      draft_id?: string;
    }) => {
      const response = await axios.post(`${API_BASE}/api/emails/send`, email);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
    },
  });
}
