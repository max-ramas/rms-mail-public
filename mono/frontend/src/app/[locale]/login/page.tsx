"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Mail } from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { setEdition, editionLetter } from "@/hooks/useEmails";
import { setToken } from "@/lib/api-client";
import {
  ImportDialog,
  type ScanLocalAccount,
} from "@/components/import-dialog";
import axios from "axios";

export default function LoginPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = React.use(params);
  const router = useRouter();
  const t = useTranslations("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [scanAccounts, setScanAccounts] = useState<ScanLocalAccount[] | null>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    const id = setTimeout(() => {
      setMounted(true);
    }, 0);
    return () => clearTimeout(id);
  }, []);

  const logoEdition = mounted ? editionLetter() : "U";

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const payload: Record<string, string | number | boolean> = { email, password };

      const res = await axios.post("/api/auth/login", payload);

      const data = res.data as {
        token?: string;
        edition: string;
        user: { email: string };
      };
      if (data.token) setToken(data.token);
      setEdition(data.edition);

      try {
        const scanRes = await axios.post("/api/auth/scan-local", {
          email,
          password,
        });
        const scanData = scanRes.data as {
          found: boolean;
          accounts: ScanLocalAccount[];
        };
        if (scanData.found && scanData.accounts.length > 0) {
          setScanAccounts(scanData.accounts);
          setLoading(false);
          return;
        }
      } catch {
        // scan failed silently — just proceed to inbox
      }

      router.replace(`/${locale}`);
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response) {
        setError(err.response.data?.message || err.response.data?.msg || err.response.data?.error || t("errors.invalid"));
      } else {
        setError(t("errors.connection"));
      }
    } finally {
      if (!scanAccounts) {
        setLoading(false);
      }
    }
  };

  const handleImportComplete = () => {
    setScanAccounts(null);
    router.replace(`/${locale}`);
  };

  const handleImportClose = () => {
    setScanAccounts(null);
    router.replace(`/${locale}`);
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl font-bold flex items-center justify-center gap-2">
            <Mail className="w-7 h-7 text-primary" />
            <span>
              <span className="text-primary">RMS</span>{" "}
              <span className="text-foreground">Mail</span>{" "}
              <span className="text-primary align-super text-sm">
                {logoEdition}
              </span>
            </span>
          </CardTitle>
          <p className="text-sm text-muted-foreground mt-1">{t("subtitle")}</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label
                htmlFor="locale-email"
                className="text-sm font-medium text-foreground"
              >
                {t("email")}
              </label>
              <Input
                id="locale-email"
                type="email"
                placeholder={t("email_placeholder")}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <label
                htmlFor="locale-password"
                className="text-sm font-medium text-foreground"
              >
                {t("password")}
              </label>
              <Input
                id="locale-password"
                type="password"
                placeholder={t("password_placeholder")}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>

            {error && (
              <p className="text-sm text-red-500" role="alert">
                {error}
              </p>
            )}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? t("signing_in") : t("sign_in")}
            </Button>
          </form>

          {/* Theme + Language toggles — below the sign-in button, like in sidebar */}
          <div className="flex items-center justify-center gap-2 mt-4 pt-4 border-t border-border">
            <LanguageToggle locale={locale} />
            <span className="text-muted-foreground text-xs">·</span>
            <ThemeToggle />
          </div>
        </CardContent>
      </Card>
      {scanAccounts && (
        <ImportDialog
          accounts={scanAccounts}
          onClose={handleImportClose}
          onImportComplete={handleImportComplete}
        />
      )}
    </div>
  );
}
