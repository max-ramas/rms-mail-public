"use client";

import React, { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  Menu,
  X,
  Settings,
  Brain,
  User,
  Tags,
  Filter,
  FolderTree,
  Folder,
  Activity,
  FileText,
  Key,
  MessageCircle,
  Contact,
  Info,
  Webhook,
  Keyboard,
  ShieldAlert,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { useAccounts } from "@/hooks/useEmailQueries";
import { useGetMe } from "@/hooks/useAdminQueries";
import { editionLetter, isAIDisabled } from "@/hooks/useEmails";
import { type Account } from "@/hooks/useEmailTypes";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LabelManager } from "@/components/label-manager";
import { RuleManager } from "@/components/rule-manager";
import { WebhookManager } from "@/components/webhook-manager";
import { HotkeySettings } from "@/components/hotkey-settings";

// Lazy-loaded tab components — loading fallbacks use a simple text component
// (hooks can't be called at module scope from dynamic() imports)
function LoadingFallback() {
  return <div className="text-muted-foreground text-sm p-4">Loading...</div>;
}

const AITab = dynamic(
  () => import("@/components/ai-tab").then((m) => ({ default: m.AITab })),
  { loading: LoadingFallback },
);
const LicenseTab = dynamic(
  () =>
    import("@/components/license-tab").then((m) => ({ default: m.LicenseTab })),
  { loading: LoadingFallback },
);

const AccountsTab = dynamic(
  () =>
    import("@/components/accounts-tab").then((m) => ({
      default: m.AccountsTab,
    })),
  { loading: LoadingFallback },
);
const TemplatesTab = dynamic(
  () =>
    import("@/components/templates-tab").then((m) => ({
      default: m.TemplatesTab,
    })),
  { loading: LoadingFallback },
);
const MCPTab = dynamic(
  () => import("@/components/mcp-tab").then((m) => ({ default: m.MCPTab })),
  { loading: LoadingFallback },
);
const GeneralTab = dynamic(
  () =>
    import("@/components/general-tab").then((m) => ({
      default: m.GeneralTab,
    })),
  { loading: LoadingFallback },
);
const TelegramTab = dynamic(
  () =>
    import("@/components/telegram-tab").then((m) => ({
      default: m.TelegramTab,
    })),
  { loading: LoadingFallback },
);
const ContactManager = dynamic(
  () =>
    import("@/components/contact-manager").then((m) => ({
      default: m.ContactManager,
    })),
  { loading: LoadingFallback },
);
const AILogCard = dynamic(
  () =>
    import("@/components/ai-log-card").then((m) => ({ default: m.AILogCard })),
  { loading: LoadingFallback },
);
const AboutTab = dynamic(
  () => import("@/components/about-tab").then((m) => ({ default: m.AboutTab })),
  { loading: LoadingFallback },
);
const OAuthTab = dynamic(
  () => import("@/components/oauth-tab").then((m) => ({ default: m.OAuthTab })),
  { loading: LoadingFallback },
);
const FoldersManagementTab = dynamic(
  () =>
    import("@/components/folders-management-tab").then((m) => ({
      default: m.FoldersManagementTab,
    })),
  { loading: LoadingFallback },
);

import { AICategoriesTab } from "@/components/ai-categories-tab";


export default function SettingsPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = React.use(params);
  const t = useTranslations("settings");
  const [tab, setTab] = useState("general");
  const [localSelectedAccountId, setLocalSelectedAccountId] = useState("");
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const isDesktop = useMediaQuery("(min-width: 1024px)");
  const { data: accounts } = useAccounts();
  const isMono = editionLetter().startsWith("M");
  const activeAccountId = localSelectedAccountId || accounts?.[0]?.id || "";
  const { data: user, isLoading: isUserLoading } = useGetMe();

  useEffect(() => {
    if (typeof window !== "undefined") {
      const params = new URLSearchParams(window.location.search);
      
      const tabParam = params.get("tab");
      if (tabParam) {
        Promise.resolve().then(() => setTab(tabParam));
      }

      const oauth = params.get("oauth");
      if (oauth) {
        const result = {
          type: "OAUTH_RESULT",
          status: oauth,
          email: params.get("email"),
          error: params.get("error"),
        };

        // Try postMessage first
        if (window.opener) {
          window.opener.postMessage(result, "*");
        }

        // Fallback: localStorage
        localStorage.setItem(
          "oauth_result",
          JSON.stringify({ ...result, timestamp: Date.now() }),
        );

        window.close();
      }
    }
  }, []);

  useEffect(() => {
  }, [tab, user, isUserLoading]);

  const allTabs = [
    { id: "general", label: t("tab_general"), icon: Settings },
    { id: "ai", label: t("tab_ai"), icon: Brain },
    {
      id: "license",
      label: t("tab_license", { defaultMessage: "License" }),
      icon: Key,
    },
    { id: "accounts", label: t("tab_accounts"), icon: User },
    { id: "contacts", label: t("tab_contacts"), icon: Contact },
    { id: "labels", label: t("tab_labels"), icon: Tags },
    { id: "ai-categories", label: t("tab_ai_categories", { defaultMessage: "AI Categories" }), icon: Brain },

    { id: "rules", label: t("tab_rules"), icon: Filter },
    { id: "templates", label: t("tab_templates"), icon: FileText },
    {
      id: "folders",
      label: t("tab_folders", { defaultMessage: "Folders" }),
      icon: Folder,
    },
    { id: "groups", label: t("tab_groups"), icon: FolderTree },
    { id: "mcp", label: t("tab_mcp"), icon: Key },
    {
      id: "oauth",
      label: t("tab_oauth", { defaultMessage: "OAuth" }),
      icon: ShieldAlert,
    },
    { id: "telegram", label: t("tab_telegram"), icon: MessageCircle },
    {
      id: "webhooks",
      label: t("tab_webhooks", { defaultMessage: "Webhooks" }),
      icon: Webhook,
    },
    ...(isDesktop
      ? [{ id: "hotkeys", label: t("tab_hotkeys"), icon: Keyboard }]
      : []),
    { id: "stats", label: t("tab_stats"), icon: Activity },
    {
      id: "about",
      label: t("tab_about", { defaultMessage: "About" }),
      icon: Info,
    },
  ];

  const TABS = allTabs.filter((t) => {
    if (isMono && (t.id === "groups" || t.id === "oauth")) return false;
    // License tab is standalone ONLY in Unified edition. In Mono Pro and Teams, it's inside Admin Panel.
    if (t.id === "license" && editionLetter() !== "U") return false;
    return true;
  });
  const visibleTabs = isAIDisabled()
    ? TABS.filter((t) => t.id !== "ai" && t.id !== "mcp" && t.id !== "stats")
    : TABS;

  return (
    <div
      className="h-[100dvh] bg-background text-foreground flex flex-col md:flex-row overflow-hidden"
      suppressHydrationWarning
    >
      {/* Mobile Header */}
      <div className="md:hidden flex flex-none items-center justify-between p-4 border-b bg-background z-20">
        <div className="flex items-center gap-2">
          <Settings className="w-5 h-5 text-primary" />
          <span className="font-semibold">{t("title")}</span>
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
          <Settings className="w-5 h-5 text-primary" />
          <span className="font-semibold">{t("title")}</span>
        </div>
        <nav className="flex-1 py-2 overflow-y-auto">
          {visibleTabs.map((tItem) => (
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
            {t("back")}
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto p-4 md:p-8 relative z-0">
        <div className="space-y-6">
          {tab === "general" && <GeneralTab />}
          {tab === "ai" && <AITab />}
          {editionLetter() === "U" && tab === "license" && <LicenseTab />}
          {tab === "contacts" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">
                  {t("tab_contacts_title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <ContactManager />
              </CardContent>
            </Card>
          )}
          {tab === "accounts" && (
            <AccountsTab
              accounts={accounts || []}
              selectedAccountId={activeAccountId}
              setSelectedAccountId={setLocalSelectedAccountId}
              isMono={isMono}
            />
          )}
          {tab === "folders" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">
                  {t("tab_folders_title", {
                    defaultMessage: "Folders Management",
                  })}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <FoldersManagementTab />
              </CardContent>
            </Card>
          )}
          {tab === "ai-categories" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm font-medium">{t("tab_ai_categories", { defaultMessage: "AI Categories" })}</CardTitle>
              </CardHeader>
              <CardContent>
                <AICategoriesTab />
              </CardContent>
            </Card>
          )}

          {tab === "labels" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">
                  {t("tab_labels_title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <LabelManager
                  accountId={isMono ? activeAccountId : ""}
                />
              </CardContent>
            </Card>
          )}
          {tab === "rules" && activeAccountId && (
            <>
              {!isMono && (
                <div className="text-muted-foreground text-sm mb-2">
                  {t("account")}:{" "}
                  <select
                    value={activeAccountId}
                    onChange={(e) => setLocalSelectedAccountId(e.target.value)}
                    className="h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm ml-1"
                  >
                    {accounts?.map((a: Account) => (
                      <option key={a.id} value={a.id}>
                        {a.email}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <Card className="border">
                <CardHeader className="pb-3">
                  <CardTitle className="text-base">
                    {t("tab_rules_title")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <RuleManager accountId={activeAccountId} />
                </CardContent>
              </Card>
            </>
          )}
          {!isMono && tab === "groups" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">
                  {t("tab_groups_title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
              </CardContent>
            </Card>
          )}
          {tab === "webhooks" && activeAccountId && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t("webhooks")}</CardTitle>
              </CardHeader>
              <CardContent>
                <WebhookManager accountId={activeAccountId} />
              </CardContent>
            </Card>
          )}
          {tab === "hotkeys" && (
            <Card className="border">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t("tab_hotkeys")}</CardTitle>
              </CardHeader>
              <CardContent>
                <HotkeySettings />
              </CardContent>
            </Card>
          )}
          {tab === "templates" && <TemplatesTab />}
          {tab === "mcp" && <MCPTab accountId={activeAccountId} />}
          {!isMono && tab === "oauth" && <OAuthTab />}
          {tab === "telegram" && <TelegramTab />}
          {tab === "stats" && <AILogCard />}
          {tab === "about" && <AboutTab />}
        </div>
      </main>
    </div>
  );
}
