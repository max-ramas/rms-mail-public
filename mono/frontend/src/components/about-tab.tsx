"use client";

import React, { useState, useSyncExternalStore } from "react";
import { Mail, Heart, RefreshCw } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { SupportModal } from "@/components/support-modal";
import { editionLetter } from "@/hooks/useEmails";
import { useTranslations } from "next-intl";
import packageJson from "../../package.json";
import { NotificationCenter } from "./notification-center";
import { useQueryClient } from "@tanstack/react-query";
import { API_BASE } from "@/hooks/useEmailTypes";
import { useLicenseInfo } from "@/hooks/useEmailQueries";
import { useGetMe } from "@/hooks/useAdminQueries";
import {
  updateChannelBadgeClass,
  updateChannelLabel,
} from "@/lib/update-channel";

function getEditionName(letter: string): string {
  if (letter === "M") return "Mono";
  if (letter === "T") return "Teams";
  return "Unified";
}

export function AboutTab() {
  const [isSupportModalOpen, setIsSupportModalOpen] = useState(false);
  const [isChecking, setIsChecking] = useState(false);
  const t = useTranslations("settings");
  const queryClient = useQueryClient();
  const { data: licenseInfo } = useLicenseInfo();
  const { data: user } = useGetMe();

  // SSR-safe mounting (same pattern as sidebar/login)
  const mounted = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  );

  const isMonoPro = mounted && editionLetter() === "MP";

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

  const letter = mounted ? editionLetter() : "U";
  const editionName = getEditionName(letter);
  const version =
    licenseInfo?.app_version ||
    process.env.NEXT_PUBLIC_APP_VERSION ||
    packageJson.version ||
    "3.0.1";
  const updateChannel = licenseInfo?.update_channel || "stable";

  return (
    <div className="flex flex-col min-h-[60vh] py-8 px-4 w-full">
      {/* Background gradient effect */}
      <div className="absolute inset-0 -z-10 bg-gradient-to-tr from-primary/10 via-transparent to-amber-500/10 rounded-3xl blur-3xl" />

      <Card className="w-full border-border/50 bg-card/80 backdrop-blur-xl rounded-xl">
        <CardContent className="p-8 md:p-12 flex flex-col items-center text-center space-y-8">
          {/* Logo */}
          <div className="flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary/20 to-primary/5 shadow-inner mb-2">
            <Mail className="w-10 h-10 text-primary drop-shadow-md" />
          </div>

          <div className="space-y-3">
            <h1 className="text-4xl md:text-5xl font-bold tracking-tight flex items-center justify-center gap-2">
              <span className="text-primary">RMS</span>
              <span className="text-foreground">Mail</span>
              <span className="text-primary/80 align-super text-2xl font-bold">
                {letter}
              </span>
            </h1>
            <div className="text-base font-medium text-muted-foreground flex flex-wrap items-center justify-center gap-2">
              {editionName} Edition{" "}
              <span className="px-2 py-0.5 rounded-full bg-primary/10 text-primary text-xs tracking-widest font-mono">
                v{version}
              </span>
              <span
                className={`px-2 py-0.5 rounded-full text-xs font-semibold tracking-wide ${updateChannelBadgeClass(updateChannel)}`}
                title={t("about_update_channel")}
              >
                {updateChannelLabel(updateChannel)}
              </span>
              {(!isMonoPro || user?.is_admin) && (
                <>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="w-6 h-6 rounded-full"
                    onClick={handleCheckUpdates}
                    disabled={isChecking}
                    title="Check for updates"
                  >
                    <RefreshCw
                      className={`w-3.5 h-3.5 ${isChecking ? "animate-spin text-primary" : ""}`}
                    />
                  </Button>
                  <NotificationCenter />
                </>
              )}
            </div>
          </div>

          {/* Copyright block */}
          <div className="text-sm text-muted-foreground space-y-1 mt-2">
            <p>
              {t("about_developed_by")}{" "}
              <a
                href="https://rms-ds.com"
                target="_blank"
                rel="noopener noreferrer"
                className="text-foreground hover:text-primary transition-colors font-medium hover:underline underline-offset-4"
              >
                RMS Digital Services
              </a>
            </p>
            <p>© 2026 {t("about_copyright")}</p>
          </div>

          {/* Support Author Button */}
          <div className="pt-6 flex justify-center w-full">
            <Button
              className="bg-primary hover:bg-primary/90 text-primary-foreground px-8 py-2 rounded-md font-medium transition-colors w-auto"
              onClick={() => setIsSupportModalOpen(true)}
            >
              <Heart className="w-4 h-4 me-2 fill-current" />{" "}
              {t("support_author")}
            </Button>
          </div>

          {/* Links grid */}
          <div className="w-full pt-8 mt-4 border-t border-border/50 grid grid-cols-1 md:grid-cols-2 gap-4 text-sm text-left">
            <a
              href="https://github.com/max-ramas/rms-mail-public"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50 transition-colors group"
            >
              <span className="text-muted-foreground group-hover:text-foreground transition-colors">
                {t("about_github")}
              </span>
              <span className="font-medium text-foreground truncate ml-4">
                GitHub
              </span>
            </a>
            <a
              href="mailto:rms-mail@rms-ds.com"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50 transition-colors group"
            >
              <span className="text-muted-foreground group-hover:text-foreground transition-colors">
                {t("about_support")}
              </span>
              <span className="font-medium text-foreground truncate ml-4">
                rms-mail@rms-ds.com
              </span>
            </a>
            <a
              href="mailto:info@rms-ds.com"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50 transition-colors group"
            >
              <span className="text-muted-foreground group-hover:text-foreground transition-colors">
                {t("about_partnership")}
              </span>
              <span className="font-medium text-foreground truncate ml-4">
                info@rms-ds.com
              </span>
            </a>
            <a
              href="mailto:m@ramzaeff.com"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50 transition-colors group"
            >
              <span className="text-muted-foreground group-hover:text-foreground transition-colors">
                {t("about_developer")}
              </span>
              <span className="font-medium text-foreground truncate ml-4">
                m@ramzaeff.com
              </span>
            </a>
          </div>
        </CardContent>
      </Card>

      <SupportModal
        isOpen={isSupportModalOpen}
        onClose={() => setIsSupportModalOpen(false)}
      />
    </div>
  );
}
