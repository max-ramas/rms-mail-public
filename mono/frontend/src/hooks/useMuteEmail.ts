"use client";

import { useEmailToggleMutation } from "./useEmailToggleMutation";
import { API_BASE } from "./useEmailTypes";

export function useMuteEmail() {
  return useEmailToggleMutation({
    endpoint: (emailId) => `${API_BASE}/api/emails/${emailId}/mute`,
    field: "is_muted",
    updateEmailDetail: true,
  });
}
