import "@/lib/api-client";

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

export interface Email {
  id: string;
  account_id: string;
  folder_id: string;
  msg_id: string;
  uid: number;
  subject: string;
  sender_name: string;
  sender_address: string;
  recipient_address: string;
  cc_address: string;
  date_sent: string;
  is_read: boolean;
  has_attachments: boolean;
  is_dirty_locally?: boolean;
  avatar_url?: string;
  spf_pass?: boolean;
  dkim_pass?: boolean;
  is_pinned?: boolean;
  snooze_until?: string;
  is_muted?: boolean;
  is_flagged?: boolean;
  is_answered?: boolean;
  in_reply_to?: string;
  thread_id?: string;
  assigned_to?: string;
  status?: string;
  snippet: string;
  body?: string;
  html?: string;
  draft_reply?: string;
  draft_remote_uid?: number;
  created_at: string;
}

export interface Account {
  id: string;
  email: string;
  name?: string;
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
  ai_provider_config: string;
  signature?: string;
  last_sync_error?: string;
  last_sync_at?: string;
  unread_count?: number;
  unread_inbox?: number;
  system_discovered?: boolean;
  absent_since?: string;
  is_locked?: boolean;
  is_sync_paused?: boolean;
  smart_categories?: boolean;
}

export interface Contact {
  id?: string;
  address: string;
  name: string;
  phone?: string;
  notes?: string;
  company?: string;
  position?: string;
  tags?: string;
}

export interface Label {
  id: string;
  account_id: string;
  name: string;
  color: string;
}

export interface FilterRule {
  id?: string;
  account_id: string;
  name: string;
  enabled: boolean;
  condition_field: string;
  condition_operator: string;
  condition_value: string;
  action_type: string;
  action_value: string;
  priority: number;
  ai_provider?: string;
  ai_model?: string;
  webhook_secret?: string;
}

export interface ProjectGroup {
  id: string;
  name: string;
  color: string;
  sort_order: number;
  is_locked?: boolean;
  unread_count?: number;
  accounts_count?: number;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
}

export interface EmailComment {
  id: string;
  email_id: string;
  author_id: string;
  body: string;
  internal: boolean;
  created_at: string;
}

export interface Attachment {
  id: string;
  email_id: string;
  filename: string;
  size: number;
  hash: string;
  created_at: string;
}

export interface Folder {
  id: string;
  account_id: string;
  name: string;
  path: string;
  is_subscribed: boolean;
  last_sync_uid: number;
  unread_count?: number;
}

export interface AIMessage {
  role: "user" | "assistant" | "system";
  content: string;
}

export interface AILogEntry {
  id: string;
  action: string;
  provider: string;
  model: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  duration_ms: number;
  status: string;
  created_at: string;
}

export interface AILogStats {
  total_actions: number;
  total_tokens: number;
  total_cost_usd: number;
  by_action: Record<string, number>;
  by_provider: Record<string, number>;
}

export interface Identity {
  id: string;
  account_id: string;
  name: string;
  email: string;
}
