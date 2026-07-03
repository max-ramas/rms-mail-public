"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { ToastProvider } from "@/hooks/useToast";
import { ErrorBoundary } from "@/components/error-boundary";
import { HotkeyManager } from "@/lib/HotkeyManager";
import { ChunkLoadRecovery } from "@/components/chunk-load-recovery";

export function Providers({ children }: { children: React.ReactNode }) {
  // Start the global HotkeyManager singleton once on mount
  useEffect(() => {
    HotkeyManager.start();
  }, []);

  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 5 * 60 * 1000,
            gcTime: 30 * 60 * 1000,
            retry: (failureCount, error: unknown) => {
              if (error && typeof error === "object" && "response" in error) {
                const response = (error as { response?: { status?: number } })
                  .response;
                if (response?.status === 404) return false;
                if (response?.status === 429) return false;
              }
              return failureCount < 2;
            },
            refetchOnMount: false,
            refetchOnWindowFocus: false,
          },
        },
      }),
  );

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ChunkLoadRecovery />
        <ToastProvider>
          {children}
        </ToastProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
