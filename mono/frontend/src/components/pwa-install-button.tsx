"use client";

import React, { useEffect, useState, useRef } from "react";
import { Download } from "lucide-react";

interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[];
  readonly userChoice: Promise<{
    outcome: "accepted" | "dismissed";
    platform: string;
  }>;
  prompt(): Promise<void>;
}

export function PWAInstallButton() {
  const deferredPromptRef = useRef<BeforeInstallPromptEvent | null>(null);
  const [showInstall, setShowInstall] = useState(false);

  useEffect(() => {
    if (window.matchMedia("(display-mode: standalone)").matches) return;

    const handler = (e: Event) => {
      e.preventDefault();
      deferredPromptRef.current = e as BeforeInstallPromptEvent;
      setShowInstall(true);
    };
    window.addEventListener("beforeinstallprompt", handler);
    window.addEventListener("appinstalled", () => setShowInstall(false));
    return () => {
      window.removeEventListener("beforeinstallprompt", handler);
      window.removeEventListener("appinstalled", () => setShowInstall(false));
    };
  }, []);

  if (!showInstall) return null;

  const handleInstallClick = async () => {
    const ev = deferredPromptRef.current;
    if (!ev) return;
    try {
      await ev.prompt();
      const { outcome } = await ev.userChoice;
      if (outcome === "accepted") setShowInstall(false);
      deferredPromptRef.current = null;
    } catch {}
  };

  return (
    <button
      onClick={handleInstallClick}
      className="flex items-center justify-center gap-2 px-3 py-2 w-full text-sm font-medium text-amber-600 bg-amber-500/10 hover:bg-amber-500/20 rounded-md transition-colors"
    >
      <Download className="w-4 h-4" />
      <span>Install App</span>
    </button>
  );
}
