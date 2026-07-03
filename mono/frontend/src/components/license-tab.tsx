"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@radix-ui/react-label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Shield, CheckCircle2, XCircle, Loader2, Crown } from "lucide-react";
import { apiFetch } from "@/lib/api-client";

export function LicenseTab() {
  const t = useTranslations("settings");
  const [licenseKey, setLicenseKey] = useState("");
  const [status, setStatus] = useState<string>("loading");
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchLicenseInfo = useCallback(async () => {
    try {
      const res = await apiFetch("/api/license");
      if (res.ok) {
        const data = await res.json();
        if (data.error && data.status === "unlicensed") {
          setStatus(`unlicensed: ${data.error}`);
        } else {
          setStatus(data.status || "unlicensed");
        }
        if (data.expires_at) {
          setExpiresAt(data.expires_at);
        }
      }
    } catch {
      setStatus("error");
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchLicenseInfo();
  }, [fetchLicenseInfo]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await apiFetch("/api/license", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ license_key: licenseKey }),
      });

      const data = await res.json();
      if (data.status === "error") {
        setStatus(`error: ${data.error}`);
        setSaving(false);
        return;
      }

      setTimeout(() => {
        fetchLicenseInfo();
        setSaving(false);
      }, 500);
    } catch {
      setStatus("error");
      setSaving(false);
    }
  };

  const renderStatus = () => {
    if (status.startsWith("unlicensed:")) {
      return (
        <span className="text-yellow-600 flex items-center gap-2">
          <XCircle className="w-4 h-4 shrink-0" />{" "}
          <span>
            {t("license_unlicensed")} ({status.replace("unlicensed:", "")})
          </span>
        </span>
      );
    }
    if (status.startsWith("error:")) {
      return (
        <span className="text-red-600 flex items-center gap-2">
          <XCircle className="w-4 h-4 shrink-0" />{" "}
          <span>
            {t("license_error")} ({status.replace("error:", "")})
          </span>
        </span>
      );
    }

    switch (status) {
      case "active":
        return (
          <span className="text-green-600 flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />{" "}
            <span>{t("license_active")}</span>
          </span>
        );
      case "unlicensed":
        return (
          <span className="text-yellow-600 flex items-center gap-2">
            <XCircle className="w-4 h-4 shrink-0" />{" "}
            <span>{t("license_unlicensed")}</span>
          </span>
        );
      case "expired":
        return (
          <span className="text-red-600 flex items-center gap-2">
            <XCircle className="w-4 h-4 shrink-0" />{" "}
            <span>{t("license_expired")}</span>
          </span>
        );
      case "revoked":
        return (
          <span className="text-red-600 flex items-center gap-2">
            <XCircle className="w-4 h-4 shrink-0" />{" "}
            <span>{t("license_revoked")}</span>
          </span>
        );
      case "error":
        return (
          <span className="text-red-600 flex items-center gap-2">
            <XCircle className="w-4 h-4 shrink-0" />{" "}
            <span>{t("license_error")}</span>
          </span>
        );
      default:
        return (
          <span className="text-muted-foreground flex items-center gap-2">
            <div className="animate-spin h-4 w-4 border-2 border-primary border-t-transparent rounded-full shrink-0" />
            <span>{t("license_loading")}</span>
          </span>
        );
    }
  };

  return (
    <Card className="border">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Shield className="w-5 h-5 text-primary" />
          <CardTitle>{t("license_title")}</CardTitle>
        </div>
        <p className="text-sm text-muted-foreground">{t("license_desc")}</p>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <Label>{t("license_status")}</Label>
          <div className="flex items-center gap-2 text-sm font-medium">
            {renderStatus()}
          </div>
          {expiresAt && (
            <p className="text-xs text-muted-foreground mt-1">
              {t("license_expires")}{" "}
              {new Date(expiresAt * 1000).toLocaleString()}
            </p>
          )}
        </div>

        <div className="space-y-2">
          <Label>{t("license_key_label")}</Label>
          <Input
            value={licenseKey}
            onChange={(e) => setLicenseKey(e.target.value)}
            placeholder={t("license_key_placeholder")}
            className="font-mono"
          />
        </div>

        <Button onClick={handleSave} disabled={saving}>
          {saving ? <Loader2 className="w-4 h-4 animate-spin mr-1" /> : null}
          {saving ? t("saving", { defaultMessage: "Saving..." }) : t("save")}
        </Button>

        <Button variant="outline" className="w-full" onClick={() => window.open("https://license.rms-ds.com", "_blank")}>
          <Crown className="w-4 h-4 mr-1 text-amber-500" />
          {t("license_buy_button")}
        </Button>
      </CardContent>
    </Card>
  );
}
