import type { Email } from "./useEmailTypes";

export type EmailListPage = { items: Email[]; nextCursor: string };
export type InfiniteCache = { pages: EmailListPage[] } | undefined;

export interface AICustomParams {
  provider?: string;
  model?: string;
  prompt?: string;
  api_key?: string;
}

export interface ForwardPayload {
  [key: string]: unknown;
}
