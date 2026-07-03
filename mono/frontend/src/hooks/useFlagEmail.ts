"use client";

import { useEmailToggleMutation } from "./useEmailToggleMutation";
import { API_BASE } from "./useEmailTypes";

export function useFlagEmail() {
  return useEmailToggleMutation({
    endpoint: (emailId) => `${API_BASE}/api/emails/${emailId}/flag`,
    field: "is_flagged",
  });
}
