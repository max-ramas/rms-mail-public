"use client";

import { useEmailToggleMutation } from "./useEmailToggleMutation";
import { API_BASE } from "./useEmailTypes";

export function usePinEmail() {
  return useEmailToggleMutation({
    endpoint: (emailId) => `${API_BASE}/api/emails/${emailId}/pin`,
    field: "is_pinned",
  });
}
