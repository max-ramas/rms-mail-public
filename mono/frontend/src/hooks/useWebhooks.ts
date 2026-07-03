import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";

export interface Webhook {
  id: string;
  account_id: string;
  name: string;
  url: string;
  has_secret?: boolean;
  created_at: string;
}

export function useWebhooks(accountId: string) {
  return useQuery({
    queryKey: ["webhooks", accountId],
    queryFn: async () => {
      const res = await axios.get(
        `${API_BASE}/api/webhooks?account_id=${accountId}`,
      );
      return res.data as Webhook[];
    },
    enabled: !!accountId,
  });
}

export function useCreateWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: {
      account_id: string;
      name: string;
      url: string;
      secret: string;
    }) => {
      const res = await axios.post(`${API_BASE}/api/webhooks`, data);
      return res.data as Webhook;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
}

export function useDeleteWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/webhooks/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
}
