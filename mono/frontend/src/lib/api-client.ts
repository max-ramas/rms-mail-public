import axios from "axios";

const TOKEN_KEY = "rms_token";

axios.defaults.withCredentials = true;

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem(TOKEN_KEY);
}

axios.interceptors.request.use((config) => {
  const token = getToken();
  if (token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

/** fetch wrapper that always sends session cookies. */
export function apiFetch(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  const headers = new Headers(init?.headers);
  const token = getToken();
  if (token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return fetch(input, { credentials: "include", ...init, headers });
}

/** Extract a user-facing error message from an API response body. */
export function parseApiError(
  body: unknown,
  fallback = "Request failed",
): string {
  if (body && typeof body === "object" && "error" in body) {
    const msg = (body as { error?: unknown }).error;
    if (typeof msg === "string" && msg.length > 0) return msg;
  }
  if (typeof body === "string" && body.length > 0) return body;
  return fallback;
}
