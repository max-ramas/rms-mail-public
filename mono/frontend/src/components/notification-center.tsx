"use client";

import React, { useState } from "react";
import { Mail, RefreshCw, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { useLicenseInfo } from "@/hooks/useEmailQueries";
import packageJson from "../../package.json";
import { useTranslations } from "next-intl";
import { useQueryClient } from "@tanstack/react-query";
import { API_BASE } from "@/hooks/useEmailTypes";

export function NotificationCenter() {
  const [isOpen, setIsOpen] = useState(false);
  const [isChecking, setIsChecking] = useState(false);
  const [dismissedVersion, setDismissedVersion] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem("rms_dismissed_update") || "";
    }
    return "";
  });
  const { data: licenseInfo } = useLicenseInfo();
  const queryClient = useQueryClient();
  const t = useTranslations("settings");

  const handleDismiss = () => {
    if (licenseInfo?.latest_version) {
      localStorage.setItem(
        "rms_dismissed_update",
        licenseInfo.latest_version,
      );
      setDismissedVersion(licenseInfo.latest_version);
    }
  };

  const handleCheckUpdates = async () => {
    try {
      setIsChecking(true);
      await fetch(`${API_BASE}/api/license/check`, { method: "POST" });
      await queryClient.invalidateQueries({ queryKey: ["license"] });
    } catch (err) {
      if (process.env.NODE_ENV === "development")
        console.error("Failed to check updates", err);
    } finally {
      setIsChecking(false);
    }
  };

  // Return null during SSR
  const currentVersion =
    process.env.NEXT_PUBLIC_APP_VERSION || packageJson.version || "3.0.1";
  const latestVersion = licenseInfo?.latest_version || currentVersion;
  const releaseNotes = licenseInfo?.release_notes || "";

  const isUpdateAvailable =
    latestVersion !== "0.0.0" &&
    latestVersion !== currentVersion &&
    latestVersion !== "";

  const isUnread = isUpdateAvailable && latestVersion !== dismissedVersion;

  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        className="relative w-8 h-8 rounded-full text-muted-foreground hover:text-foreground"
        title={t("notification_button") || "Notifications"}
        onClick={() => setIsOpen(true)}
      >
        <Mail className={`w-4 h-4 ${isUnread ? "text-amber-500" : ""}`} />
        {isUnread && (
          <div className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full border border-background" />
        )}
      </Button>

      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <div className="flex items-center justify-between">
              <DialogTitle>
                {isUpdateAvailable
                  ? t("notification_update_available")
                  : t("notification_center")}
              </DialogTitle>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleCheckUpdates}
                disabled={isChecking}
                className="w-8 h-8 rounded-full"
                title="Check for updates"
              >
                <RefreshCw
                  className={`w-4 h-4 text-muted-foreground ${isChecking ? "animate-spin text-primary" : ""}`}
                />
              </Button>
            </div>
          </DialogHeader>

          <div className="py-4 space-y-4">
            {isUpdateAvailable ? (
              <div className="space-y-4">
                <div className="flex items-center justify-between px-2">
                  <span className="font-medium text-muted-foreground text-sm">
                    {t("notification_current_version", {
                      version: currentVersion,
                    })}
                  </span>
                  <span className="font-bold text-amber-500 text-sm">
                    {t("notification_new_version", {
                      version: latestVersion,
                    })}
                  </span>
                </div>
                {releaseNotes ? (
                  <div className="p-4 bg-muted/50 rounded-lg text-sm border border-border">
                    <h4 className="font-semibold mb-2">
                      {t("notification_release_notes")}
                    </h4>
                    <div className="whitespace-pre-wrap text-foreground/80">
                      {releaseNotes}
                    </div>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground px-2">
                    {t("notification_no_release_notes")}
                  </p>
                )}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-8 text-center space-y-3">
                <Mail className="w-12 h-12 text-muted-foreground/30" />
                <p className="text-muted-foreground">
                  {t("notification_no_notifications")}
                </p>
              </div>
            )}
          </div>

          <DialogFooter className="flex items-center justify-between sm:justify-between w-full">
            <div>
              {isUnread && (
                <Button
                  variant="ghost"
                  className="text-muted-foreground"
                  onClick={handleDismiss}
                >
                  <Check className="w-4 h-4 mr-2" />
                  {t("notification_mark_read") || "Mark as read"}
                </Button>
              )}
            </div>
            <Button variant="outline" onClick={() => setIsOpen(false)}>
              {t("notification_close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
