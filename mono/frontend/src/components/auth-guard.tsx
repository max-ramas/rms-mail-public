"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { fetchEdition } from "@/hooks/useEmails";
import axios from "axios";
import "@/lib/api-client";

function isPublicPath(pathname: string): boolean {
  return pathname.includes("/login") || pathname.includes("/setup");
}

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const t = useTranslations();
  const pathname = usePathname();
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [authValid, setAuthValid] = useState<boolean | null>(null);
  const isPublic = isPublicPath(pathname);

  useEffect(() => {
    const id = setTimeout(() => setMounted(true), 0);
    fetchEdition();
    return () => clearTimeout(id);
  }, []);

  // Validate session cookie on protected routes
  useEffect(() => {
    if (!mounted || isPublic) return;

    let cancelled = false;
    const verify = async (attempt: number): Promise<void> => {
      try {
        await axios.get("/api/auth/verify");
        if (!cancelled) setAuthValid(true);
      } catch (err: unknown) {
        const ex = err as { response?: { status: number } };
        const status = ex.response?.status;
        if (status === 401 || status === 403) {
          if (attempt === 0) {
            await new Promise((r) => setTimeout(r, 500));
            if (!cancelled) return verify(1);
          }
          if (!cancelled) setAuthValid(false);
        } else {
          if (!cancelled) setAuthValid(true);
        }
      }
    };
    verify(0);
    return () => { cancelled = true; };
  }, [mounted, isPublic, pathname]);

  // Redirect to login when session is invalid
  useEffect(() => {
    if (authValid === false && !isPublic) {
      const match = pathname.match(/^\/([a-z]{2})\b/);
      const locale = match ? match[1] : "";
      router.replace(locale ? `/${locale}/login` : "/login");
    }
  }, [authValid, isPublic, router, pathname]);

  useEffect(() => {
    if (!mounted || authValid !== false) return;

    axios
      .get("/api/auth/status")
      .then((res) => {
        const data = res.data as { setup_needed: boolean };
        const match = pathname.match(/^\/([a-z]{2})\b/);
        const locale = match ? match[1] : "";

        if (data.setup_needed && !pathname.includes("/setup")) {
          router.replace(locale ? `/${locale}/setup` : "/setup");
        } else if (
          !data.setup_needed &&
          !pathname.includes("/login") &&
          isPublic
        ) {
          if (pathname.includes("/setup")) {
            router.replace(locale ? `/${locale}/login` : "/login");
          }
        } else if (!data.setup_needed && !isPublic) {
          router.replace(locale ? `/${locale}/login` : "/login");
        }
      })
      .catch(() => {
        if (!isPublic) {
          const match = pathname.match(/^\/([a-z]{2})\b/);
          const locale = match ? match[1] : "";
          router.replace(locale ? `/${locale}/login` : "/login");
        }
      });
  }, [mounted, pathname, router, isPublic, authValid]);

  if (isPublic) return <>{children}</>;

  if (!mounted || authValid === null) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-muted-foreground">{t("loading")}</div>
      </div>
    );
  }

  if (authValid) return <>{children}</>;

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="text-muted-foreground">{t("loading")}</div>
    </div>
  );
}
