import { useMutation, useQueryClient, useQuery } from "@tanstack/react-query";
import axios from "axios";
import { API_BASE } from "./useEmailTypes";
import type {
  Account,
  Label,
  FilterRule,
  ProjectGroup,
  User,
  Contact,
} from "./useEmailTypes";

export function useGetMe() {
  return useQuery({
    queryKey: ["auth_me"],
    queryFn: async () => {
      const response = await axios.get(`${API_BASE}/api/auth/me`);
      return response.data as {
        email: string;
        is_admin: boolean;
        role: string;
      };
    },
    staleTime: 60 * 1000,
    retry: false, // Do not retry on 401
  });
}

// ── Accounts ───────────────────────────────────────────────────────

export function useCreateAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (account: {
      email: string;
      provider: string;
      imap_host: string;
      imap_port: number;
      imap_ssl: boolean;
      imap_encryption?: string;
      smtp_host: string;
      smtp_port: number;
      smtp_ssl: boolean;
      smtp_encryption?: string;
      username: string;
      password: string;
      ai_provider_config?: string;
      signature?: string;
    }) => {
      const response = await axios.post(`${API_BASE}/api/accounts`, account);
      return response.data as Account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["emails"] });
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useOAuthURL() {
  return useMutation({
    mutationFn: async (provider: string) => {
      const response = await axios.get(
        `${API_BASE}/api/oauth/url?provider=${provider}`,
      );
      return response.data as { url: string };
    },
  });
}

export function useOAuthCallback() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      provider,
      code,
    }: {
      provider: string;
      code: string;
    }) => {
      const response = await axios.get(
        `${API_BASE}/api/oauth/callback?provider=${encodeURIComponent(provider)}&code=${encodeURIComponent(code)}`,
      );
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["emails"] });
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useDeleteAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (accountId: string) => {
      await axios.delete(`${API_BASE}/api/accounts/${accountId}`);
      return accountId;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
    },
  });
}

export function useResetAccountSync() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (accountId: string) => {
      await axios.post(`${API_BASE}/api/accounts/${accountId}/reset-sync`);
      return accountId;
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["emails-infinite"] });
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["folders"] });
    },
  });
}

export function usePauseAccountSync() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (accountId: string) => {
      await axios.post(`${API_BASE}/api/accounts/${accountId}/pause-sync`);
      return accountId;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
    },
  });
}

export function useResumeAccountSync() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (accountId: string) => {
      await axios.post(`${API_BASE}/api/accounts/${accountId}/resume-sync`);
      return accountId;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
    },
  });
}

export function useUpdateAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (account: {
      id: string;
      email: string;
      provider: string;
      imap_host: string;
      imap_port: number;
      imap_ssl: boolean;
      imap_encryption?: string;
      smtp_host: string;
      smtp_port: number;
      smtp_ssl: boolean;
      smtp_encryption?: string;
      username: string;
      password: string;
      ai_provider_config?: string;
      signature?: string;
    }) => {
      const response = await axios.put(
        `${API_BASE}/api/accounts/${account.id}`,
        account,
      );
      return response.data as Account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["emails"] });
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

export function useUpdateSmartCategories() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      id,
      smart_categories,
    }: {
      id: string;
      smart_categories: boolean;
    }) => {
      const response = await axios.patch(
        `${API_BASE}/api/accounts/${id}/smart-categories`,
        {
          smart_categories,
        },
      );
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["emails"] });
      queryClient.invalidateQueries({ queryKey: ["emails-infinite"] });
    },
  });
}

// ── Identities ─────────────────────────────────────────────────────

export function useCreateIdentity() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (d: {
      account_id: string;
      email: string;
      name: string;
    }) => {
      const r = await axios.post(`${API_BASE}/api/identities`, d);
      return r.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["identities"] }),
  });
}

export function useDeleteIdentity() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/identities/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["identities"] }),
  });
}

// ── Labels ─────────────────────────────────────────────────────────

export function useCreateLabel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (label: {
      account_id: string;
      name: string;
      color: string;
    }) => {
      const res = await axios.post(`${API_BASE}/api/labels/create`, label);
      return res.data as Label;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["labels"] }),
  });
}

export function useUpdateLabel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (label: { id: string; name: string; color: string }) => {
      const res = await axios.put(
        `${API_BASE}/api/labels/update/${label.id}`,
        label,
      );
      return res.data as Label;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["labels"] }),
  });
}

export function useDeleteLabel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/labels/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["labels"] }),
  });
}

// ── Rules ──────────────────────────────────────────────────────────

export function useCreateRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (rule: FilterRule) => {
      const res = await axios.post(`${API_BASE}/api/rules/create`, rule);
      return res.data as FilterRule;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rules"] }),
  });
}

export function useUpdateRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (rule: FilterRule & { id: string }) => {
      const res = await axios.put(
        `${API_BASE}/api/rules/update/${rule.id}`,
        rule,
      );
      return res.data as FilterRule;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rules"] }),
  });
}

export function useDeleteRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/rules/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["rules"] }),
  });
}

// ── Groups ─────────────────────────────────────────────────────────

export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (g: {
      name: string;
      color: string;
      sort_order: number;
    }) => {
      const res = await axios.post(`${API_BASE}/api/groups/create`, g);
      return res.data as ProjectGroup;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["groups"] }),
  });
}

export function useUpdateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (g: {
      id: string;
      name: string;
      color: string;
      sort_order: number;
    }) => {
      const res = await axios.put(`${API_BASE}/api/groups/update/${g.id}`, g);
      return res.data as ProjectGroup;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["groups"] }),
  });
}

export function useDeleteGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/groups/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["groups"] }),
  });
}

export function useSetGroupAccounts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: { group_id: string; account_ids: string[] }) => {
      await axios.post(`${API_BASE}/api/groups/accounts`, data);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["groups"] });
      qc.invalidateQueries({ queryKey: ["group-accounts"] });
    },
  });
}

// ── Users ──────────────────────────────────────────────────────────

export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (u: { email: string; name: string; role: string }) => {
      const r = await axios.post(`${API_BASE}/api/users/create`, u);
      return r.data as User;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/users/delete/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

// ── Contacts ───────────────────────────────────────────────────────

export function useCreateContact() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (contact: {
      address: string;
      name: string;
      phone?: string;
      notes?: string;
      company?: string;
      position?: string;
      tags?: string;
    }) => {
      const response = await axios.post(`${API_BASE}/api/contacts`, contact);
      return response.data as Contact;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["contacts"] });
    },
  });
}

export function useUpdateContact() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      id,
      contact,
    }: {
      id: string;
      contact: {
        address: string;
        name: string;
        phone?: string;
        notes?: string;
        company?: string;
        position?: string;
        tags?: string;
      };
    }) => {
      const response = await axios.put(
        `${API_BASE}/api/contacts?id=${id}`,
        contact,
      );
      return response.data as Contact;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["contacts"] });
    },
  });
}

export function useDeleteContact() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await axios.delete(`${API_BASE}/api/contacts?id=${id}`);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["contacts"] });
    },
  });
}
export function useAdminUsers() {
  return useQuery({
    queryKey: ["admin_users"],
    queryFn: async () => {
      const response = await axios.get(`${API_BASE}/api/admin/users`);
      return response.data as {
        id: string;
        email: string;
        name: string;
        role: string;
        last_seen_at: string;
      }[];
    },
    staleTime: 60 * 1000,
  });
}

export function useGetAdminSettings() {
  return useQuery({
    queryKey: ["admin_settings"],
    queryFn: async () => {
      const response = await axios.get(`${API_BASE}/api/admin/settings`);
      return response.data as { allowed_domains: string };
    },
    staleTime: 60 * 1000,
  });
}

export function useUpdateAdminSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: { allowed_domains: string }) => {
      const response = await axios.post(`${API_BASE}/api/admin/settings`, data);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin_settings"] });
    },
  });
}
