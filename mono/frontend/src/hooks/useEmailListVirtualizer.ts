"use client";

import { useCallback, useRef, useEffect } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ROW_HEIGHT } from "@/components/EmailRow";

export function useEmailListVirtualizer(
  count: number,
  selectedEmailId: string | null,
  emailIds: string[],
) {
  const listContainerRef = useRef<HTMLDivElement>(null);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const lastScrolledEmailIdRef = useRef<string | null>(null);

  const getScrollElement = useCallback(() => listContainerRef.current, []);
  // Fixed estimate for off-screen rows; visible rows measured via measureElement.
  const estimateSize = useCallback(() => ROW_HEIGHT, []);

  // TanStack Virtual returns unstable function refs; safe to use without Compiler memoization.
  // eslint-disable-next-line react-hooks/incompatible-library -- useVirtualizer is the intended API
  const rowVirtualizer = useVirtualizer({
    count,
    getScrollElement,
    estimateSize,
    measureElement: (el) => el.getBoundingClientRect().height,
    overscan: 5,
  });

  useEffect(() => {
    if (!selectedEmailId) return;
    if (lastScrolledEmailIdRef.current === selectedEmailId) return;

    const index = emailIds.indexOf(selectedEmailId);
    if (index !== -1) {
      try {
        rowVirtualizer.scrollToIndex(index, { align: "auto" });
        lastScrolledEmailIdRef.current = selectedEmailId;
      } catch {
        // virtualizer not ready yet
      }
    }
  }, [selectedEmailId, emailIds, rowVirtualizer]);

  return {
    listContainerRef,
    loadMoreRef,
    rowVirtualizer,
  };
}
