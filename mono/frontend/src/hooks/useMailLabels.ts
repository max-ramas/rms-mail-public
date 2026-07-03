"use client";

import { useState, useRef, useEffect, useMemo } from "react";
import type { Label } from "@/hooks/useEmailTypes";

interface UseMailLabelsOptions {
  selectedEmailId?: string;
  batchLabels?: Record<string, Label[]>;
  setEmailLabels: {
    mutate: (args: {
      email_id: string;
      account_id: string;
      label_ids: string[];
    }) => void;
  };
  selectedEmailAccountId?: string;
}

export function useMailLabels({
  selectedEmailId,
  batchLabels,
  setEmailLabels,
  selectedEmailAccountId,
}: UseMailLabelsOptions) {
  const [selectedLabelIds, setSelectedLabelIds] = useState<Set<string>>(
    new Set(),
  );
  const selectedLabelIdsRef = useRef(selectedLabelIds);

  useEffect(() => {
    selectedLabelIdsRef.current = selectedLabelIds;
  }, [selectedLabelIds]);

  const selectedLabelIdsFromServer = useMemo(
    () =>
      selectedEmailId && batchLabels?.[selectedEmailId]
        ? new Set(batchLabels[selectedEmailId].map((l) => l.id))
        : null,
    [selectedEmailId, batchLabels],
  );

  const displayLabelIds = selectedLabelIdsFromServer ?? selectedLabelIds;

  const handleToggleLabel = (labelId: string) => {
    const currentIds = new Set(selectedLabelIdsRef.current);
    if (currentIds.has(labelId)) {
      currentIds.delete(labelId);
    } else {
      currentIds.add(labelId);
    }
    selectedLabelIdsRef.current = currentIds;
    setSelectedLabelIds(currentIds);
    setEmailLabels.mutate({
      email_id: selectedEmailId ?? "",
      account_id: selectedEmailAccountId ?? "",
      label_ids: Array.from(currentIds),
    });
  };

  const handleSetLabels = (
    emailId: string,
    accountId: string,
    labelIds: string[],
  ) => {
    setSelectedLabelIds(new Set(labelIds));
    setEmailLabels.mutate({
      email_id: emailId,
      account_id: accountId,
      label_ids: labelIds,
    });
  };

  return {
    displayLabelIds,
    handleToggleLabel,
    handleSetLabels,
  };
}
