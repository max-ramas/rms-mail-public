"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

function isChunkLoadError(reason: unknown): boolean {
  if (!reason) return false;
  if (typeof reason === "string") {
    return reason.includes("ChunkLoadError");
  }
  if (typeof reason === "object") {
    const err = reason as { name?: string; message?: string };
    return (
      err.name === "ChunkLoadError" ||
      Boolean(err.message?.includes("ChunkLoadError"))
    );
  }
  return false;
}

/** Tries router.refresh() before a full page reload on stale chunk errors. */
export function ChunkLoadRecovery() {
  const router = useRouter();

  useEffect(() => {
    let attempts = 0;

    const recover = (reason: unknown) => {
      if (!isChunkLoadError(reason)) return false;
      if (attempts === 0) {
        attempts += 1;
        router.refresh();
        return true;
      }
      const url = new URL(window.location.href);
      url.searchParams.set("_cb", String(Date.now()));
      window.location.replace(url.toString());
      return true;
    };

    const onRejection = (event: PromiseRejectionEvent) => {
      if (recover(event.reason)) {
        event.preventDefault();
      }
    };

    const onError = (event: ErrorEvent) => {
      if (recover(event.error ?? event.message)) {
        event.preventDefault();
      }
    };

    window.addEventListener("unhandledrejection", onRejection);
    window.addEventListener("error", onError);
    return () => {
      window.removeEventListener("unhandledrejection", onRejection);
      window.removeEventListener("error", onError);
    };
  }, [router]);

  return null;
}
