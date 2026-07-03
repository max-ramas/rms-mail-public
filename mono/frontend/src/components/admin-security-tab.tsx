"use client";

import { useState, useEffect } from "react";
import { useTranslations } from "next-intl";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "./ui/card";
import { Input } from "./ui/input";
import { Button } from "./ui/button";
import { useToast } from "@/hooks/useToast";
import {
  useGetAdminSettings,
  useUpdateAdminSettings,
} from "@/hooks/useAdminQueries";

export function AdminSecurityTab() {
  const t = useTranslations("settings");
  const { addToast } = useToast();

  const { data: settings, isLoading: isLoadingSettings } = useGetAdminSettings();
  const updateSettings = useUpdateAdminSettings();

  const [allowedDomains, setAllowedDomains] = useState("");

  useEffect(() => {
    if (settings) {
      Promise.resolve().then(() => setAllowedDomains(settings.allowed_domains || ""));
    }
  }, [settings]);

  const handleSave = () => {
    updateSettings.mutate(
      { allowed_domains: allowedDomains },
      {
        onSuccess: () => {
          addToast("Admin settings updated successfully.", "success");
        },
        onError: (err: unknown) => {
          const error = err as { response?: { data?: { error?: string } } };
          addToast(error?.response?.data?.error || "Failed to update settings.", "error");
        },
      }
    );
  };

  return (
    <Card className="border">
      <CardHeader>
        <CardTitle>{t("admin_security", { defaultMessage: "Security" })}</CardTitle>
        <p className="text-sm text-muted-foreground">
          {t("admin_security_desc", { defaultMessage: "Configure system-wide security settings." })}
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoadingSettings ? (
          <div className="text-sm text-muted-foreground">{t("loading_settings", { defaultMessage: "Loading settings..." })}</div>
        ) : (
          <div className="space-y-2">
            <label className="text-sm font-medium">
              {t("admin_allowed_domains", { defaultMessage: "Allowed Email Domains" })}
            </label>
            <Input
              placeholder="e.g. example.com, company.org"
              value={allowedDomains}
              onChange={(e) => setAllowedDomains(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {t("admin_allowed_domains_desc", { defaultMessage: "Limit account registration to specific domains (e.g. example.com). Leave empty to allow any domain." })}
            </p>
          </div>
        )}
        <Button onClick={handleSave} disabled={isLoadingSettings || updateSettings.isPending}>
          {updateSettings.isPending ? t("saving", { defaultMessage: "Saving..." }) : t("save", { defaultMessage: "Save" })}
        </Button>
      </CardContent>
    </Card>
  );
}
