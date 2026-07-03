"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { setEdition, editionLetter } from "@/hooks/useEmails";
import { setToken } from "@/lib/api-client";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { Mail } from "lucide-react";
import axios from "axios";

export default function SetupPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = React.use(params);
  const router = useRouter();
  const t = useTranslations("setup");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
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

    if (password !== confirm) {
      setError(t("error_mismatch"));
      return;
    }

    setLoading(true);

    try {
      const res = await axios.post("/api/auth/setup", {
        email,
        password,
      });

      const data = res.data as {
        token?: string;
        edition: string;
        user: { email: string };
      };
      if (data.token) setToken(data.token);
      setEdition(data.edition);
      router.replace("/");
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response) {
        if (err.response.status === 409) {
          setError(t("error_exists"));
        } else {
          setError(err.response.data?.message || t("error_generic"));
        }
      } else {
        setError(t("error_generic"));
      }
    } finally {
      setLoading(false);
    }
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
                htmlFor="setup-email"
                className="text-sm font-medium text-foreground"
              >
                {t("email")}
              </label>
              <Input
                id="setup-email"
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
                htmlFor="setup-password"
                className="text-sm font-medium text-foreground"
              >
                {t("password")}
              </label>
              <Input
                id="setup-password"
                type="password"
                placeholder={t("password_placeholder")}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={6}
              />
            </div>
            <div className="space-y-2">
              <label
                htmlFor="setup-confirm"
                className="text-sm font-medium text-foreground"
              >
                {t("confirm")}
              </label>
              <Input
                id="setup-confirm"
                type="password"
                placeholder={t("confirm_placeholder")}
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                required
                minLength={6}
              />
            </div>
            {error && (
              <p className="text-sm text-red-500" role="alert">
                {error}
              </p>
            )}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? t("creating") : t("create")}
            </Button>
          </form>

          {/* Theme + Language toggles */}
          <div className="flex items-center justify-center gap-2 mt-4 pt-4 border-t border-border">
            <LanguageToggle locale={locale} />
            <span className="text-muted-foreground text-xs">·</span>
            <ThemeToggle />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
