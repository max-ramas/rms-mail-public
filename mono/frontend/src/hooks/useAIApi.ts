import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";
import type { AIMessage, AILogEntry, AILogStats } from "./useEmailTypes";

export function useAIChat() {
  return useMutation({
    mutationFn: async ({
      messages,
      preset,
      provider,
      model,
    }: {
      messages: AIMessage[];
      preset?: string;
      provider?: string;
      model?: string;
      api_key?: string; // deprecated: backend resolves via resolveAPIKey
    }) => {
      const response = await axios.post(`${API_BASE}/api/ai/chat`, {
        messages,
        preset,
        provider,
        model,
      });
      return response.data as { response: string };
    },
  });
}

export function useAICategorize() {
  return useMutation({
    mutationFn: async ({
      text,
      preset,
      provider,
      model,
    }: {
      text: string;
      preset?: string;
      provider?: string;
      model?: string;
      api_key?: string; // deprecated: backend resolves via resolveAPIKey
    }) => {
      const response = await axios.post(`${API_BASE}/api/ai/categorize`, {
        text,
        preset,
        provider,
        model,
      });
      return response.data as { tags: string[] };
    },
  });
}

export function useAIStats() {
  return useQuery({
    queryKey: ["ai-stats"],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/ai/stats`);
      return r.data as AILogStats;
    },
  });
}

export function useResetAIStats() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await axios.delete(`${API_BASE}/api/ai/stats`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-stats"] });
      queryClient.invalidateQueries({ queryKey: ["ai-log"] });
    },
  });
}

export function useAILog() {
  return useQuery({
    queryKey: ["ai-log"],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/ai/log`);
      return r.data as AILogEntry[];
    },
  });
}

export function useAISettings(accountId?: string) {
  return useQuery({
    queryKey: ["ai-settings", accountId],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/ai/settings`, {
        params: {
          account_id: accountId || "00000000-0000-0000-0000-000000000000",
        },
      });
      return r.data as {
        id?: string;
        preset?: string;
        config?: string;
        prompts?: string;
        api_keys?: string;
      };
    },
    enabled: typeof window !== "undefined",
  });
}

export function useSaveAISettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: {
      account_id: string;
      preset: string;
      config: string;
      prompts: string;
      api_keys: string;
    }) => {
      const r = await axios.post(`${API_BASE}/api/ai/settings`, payload);
      return r.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-settings"] });
    },
  });
}

export function useAIModels(
  provider?: string,
  _apiKey?: string,
  forceRefresh = false,
) {
  return useQuery({
    queryKey: ["ai-models", provider, forceRefresh],
    queryFn: async () => {
      const r = await axios.get(`${API_BASE}/api/ai/models`, {
        params: {
          provider,
          force_refresh: forceRefresh ? "true" : undefined,
        },
      });
      return r.data as { models: string[] };
    },
    enabled: !!provider,
    staleTime: 1000 * 60 * 5, // 5 minutes
    retry: false, // Don't retry automatically on error so we can show Toast
  });
}
