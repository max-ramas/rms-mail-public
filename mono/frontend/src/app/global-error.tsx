"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  return (
    <html>
      <body className="bg-background text-foreground font-sans">
        <div className="flex min-h-screen items-center justify-center p-4">
          <div className="text-center space-y-4 max-w-md">
            <h1 className="text-2xl font-bold">Application Error</h1>
            <p className="text-sm text-muted-foreground">
              A critical error occurred. Please refresh the page.
            </p>
            <button
              onClick={reset}
              className="px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium"
            >
              Refresh
            </button>
          </div>
        </div>
      </body>
    </html>
  );
}
