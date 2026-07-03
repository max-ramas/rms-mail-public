"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE, type EmailComment } from "./useEmailTypes";

export function useAssignEmail() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (d: { email_id: string; user_id: string }) => {
      await axios.post(`${API_BASE}/api/emails/assign`, d);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useUnassignEmail() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (email_id: string) => {
      await axios.post(`${API_BASE}/api/emails/unassign`, { email_id });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useCreateComment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (d: {
      email_id: string;
      author_id: string;
      body: string;
      internal: boolean;
    }) => {
      const r = await axios.post(`${API_BASE}/api/comments/create`, d);
      return r.data as EmailComment;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["comments"] }),
  });
}

export function useDeleteComment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/comments/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["comments"] }),
  });
}
