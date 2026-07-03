"use client";

import React, { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import { Menu, X, ShieldAlert, Users, Key } from "lucide-react";
import { useTranslations } from "next-intl";
import { useGetMe } from "@/hooks/useAdminQueries";
import { editionLetter } from "@/hooks/useEmails";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { Button } from "@/components/ui/button";

import { AdminSecurityTab } from "@/components/admin-security-tab";
import { AdminUsersTab } from "@/components/admin-users-tab";

function LoadingFallback() {
  return <div className="text-muted-foreground text-sm p-4">Loading...</div>;
}

const LicenseTab = dynamic(
  () =>
    import("@/components/license-tab").then((m) => ({ default: m.LicenseTab })),
  { loading: LoadingFallback },
);

export default function AdminPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = React.use(params);
  const t = useTranslations("settings");
  const [tab, setTab] = useState("license");
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { data: user, isLoading: isUserLoading } = useGetMe();

  useEffect(() => {
    if (typeof window !== "undefined") {
      const params = new URLSearchParams(window.location.search);
      const tabParam = params.get("tab");
      if (tabParam) {
        // Schedule state update to avoid synchronous cascading renders warning
        Promise.resolve().then(() => setTab(tabParam));
      }
    }
  }, []);

  const allTabs = [
    {
      id: "license",
      label: t("tab_license", { defaultMessage: "License" }),
      icon: Key,
    },
    {
      id: "users",
      label: t("admin_users", { defaultMessage: "Users" }),
      icon: Users,
    },
    {
      id: "security",
      label: t("admin_security", { defaultMessage: "Security" }),
      icon: ShieldAlert,
    },
  ];

  const TABS = allTabs.filter((t) => {
    // License tab is standalone ONLY in Unified edition. In Mono Pro and Teams, it's inside Admin Panel.
    if (
      t.id === "license" &&
      editionLetter() !== "MP" &&
      editionLetter() !== "T"
    )
      return false;
    return true;
  });

  if (isUserLoading) {
    return <LoadingFallback />;
  }

  // Mono edition has no admin panel
  if (editionLetter() === "M" || editionLetter() === "U") {
    if (typeof window !== "undefined") window.location.assign(`/${locale}`);
    return <LoadingFallback />;
  }

  if (!user?.is_admin) {
    return (
      <div className="flex h-[100dvh] items-center justify-center bg-background text-foreground">
        <div className="text-center space-y-4">
          <ShieldAlert className="w-12 h-12 text-destructive mx-auto" />
          <h1 className="text-2xl font-bold">
            {t("admin_access_required", {
              defaultMessage: "Admin Access Required",
            })}
          </h1>
          <p className="text-muted-foreground">
            {t("admin_access_desc", {
              defaultMessage:
                "You do not have permission to view or edit admin settings.",
            })}
          </p>
          <Button onClick={() => window.location.assign(`/${locale}`)}>
            {t("back", { defaultMessage: "← Back to Inbox" })}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div
      className="h-[100dvh] bg-background text-foreground flex flex-col md:flex-row overflow-hidden"
      suppressHydrationWarning
    >
      {/* Mobile Header */}
      <div className="md:hidden flex flex-none items-center justify-between p-4 border-b bg-background z-20">
        <div className="flex items-center gap-2">
          <ShieldAlert className="w-5 h-5 text-primary" />
          <span className="font-semibold">
            {t("tab_admin", { defaultMessage: "Admin Panel" })}
          </span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
        >
          {isMobileMenuOpen ? (
            <X className="w-5 h-5" />
          ) : (
            <Menu className="w-5 h-5" />
          )}
        </Button>
      </div>

      <aside
        className={`
        ${isMobileMenuOpen ? "flex" : "hidden"}
        md:flex
        w-full md:w-64
        border-r border flex-col shrink-0
        absolute md:relative z-10
        bg-background
        h-[calc(100dvh-65px)] md:h-[100dvh]
        top-[65px] md:top-0
      `}
      >
        <div className="hidden md:flex items-center gap-2 p-4 border-b border">
          <ShieldAlert className="w-5 h-5 text-primary" />
          <span className="font-semibold">
            {t("tab_admin", { defaultMessage: "Admin Panel" })}
          </span>
        </div>
        <nav className="flex-1 py-2 overflow-y-auto">
          {TABS.map((tItem) => (
            <button
              key={tItem.id}
              onClick={() => {
                setTab(tItem.id);
                setIsMobileMenuOpen(false);
              }}
              className={`w-full flex items-center gap-2 px-4 py-2.5 text-sm text-left transition-colors ${
                tab === tItem.id
                  ? "bg-muted text-foreground font-medium border-l-2 md:border-l-0 md:border-r-2 border-primary"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
              }`}
            >
              <tItem.icon className="w-4 h-4" /> {tItem.label}
            </button>
          ))}
        </nav>
        <div className="p-3 border-t border space-y-2 shrink-0">
          <div className="flex items-center justify-center gap-2">
            <LanguageToggle locale={locale} />
            <ThemeToggle />
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-center text-xs hover:bg-accent hover:text-accent-foreground"
            onClick={() => window.location.assign(`/${locale}`)}
          >
            {t("back", { defaultMessage: "← Back to Inbox" })}
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto p-4 md:p-8 relative z-0">
        <div className="space-y-6">
          {tab === "license" && <LicenseTab />}
          {tab === "users" && <AdminUsersTab />}
          {tab === "security" && <AdminSecurityTab />}
        </div>
      </main>
    </div>
  );
}
