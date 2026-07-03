"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";
import type { AICustomParams } from "./types";

export function useSummarizeEmail() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      emailId,
      aiConfig,
    }: {
      emailId: string;
      aiConfig?: AICustomParams;
    }) => {
      const r = await axios.post(
        `${API_BASE}/api/emails/${emailId}/summarize`,
        aiConfig || {},
      );
      return r.data as { summary: string };
    },
    onSuccess: (_data, { emailId }) => {
      qc.invalidateQueries({ queryKey: ["email", emailId] });
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useCategorizeEmail() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      emailId,
      aiConfig,
    }: {
      emailId: string;
      aiConfig?: AICustomParams;
    }) => {
      const r = await axios.post(
        `${API_BASE}/api/emails/${emailId}/categorize`,
        aiConfig || {},
      );
      return r.data as { tags: string[] };
    },
    onSuccess: (_data, { emailId }) => {
      qc.invalidateQueries({ queryKey: ["email-tags", emailId] });
      qc.invalidateQueries({ queryKey: ["email", emailId] });
    },
  });
}

export function useClearDraftReply() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (emailId: string) => {
      await axios.post(`${API_BASE}/api/emails/${emailId}/clear-draft`);
    },
    onSuccess: (_data, emailId) => {
      qc.invalidateQueries({ queryKey: ["email", emailId] });
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useSetEmailLabels() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: {
      email_id: string;
      account_id: string;
      label_ids: string[];
    }) => {
      await axios.post(`${API_BASE}/api/emails/labels`, data);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
      qc.invalidateQueries({ queryKey: ["email-labels"] });
    },
  });
}
