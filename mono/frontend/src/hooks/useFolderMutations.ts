"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";

export function useCreateFolder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      accountId,
      name,
    }: {
      accountId: string;
      name: string;
    }) => {
      const response = await axios.post(
        `${API_BASE}/api/accounts/${accountId}/folders`,
        { name },
      );
      return response.data;
    },
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: ["folders", variables.accountId] });
      qc.invalidateQueries({ queryKey: ["folders"] });
    },
  });
}

export function useRenameFolder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      accountId,
      folderId,
      name,
    }: {
      accountId: string;
      folderId: string;
      name: string;
    }) => {
      const response = await axios.patch(
        `${API_BASE}/api/accounts/${accountId}/folders/${folderId}`,
        { name },
      );
      return response.data;
    },
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: ["folders", variables.accountId] });
      qc.invalidateQueries({ queryKey: ["folders"] });
    },
  });
}

export function useDeleteFolder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      accountId,
      folderId,
    }: {
      accountId: string;
      folderId: string;
    }) => {
      const response = await axios.delete(
        `${API_BASE}/api/accounts/${accountId}/folders/${folderId}`,
      );
      return response.data;
    },
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: ["folders", variables.accountId] });
      qc.invalidateQueries({ queryKey: ["folders"] });
      qc.invalidateQueries({ queryKey: ["emails"] });
      qc.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}
