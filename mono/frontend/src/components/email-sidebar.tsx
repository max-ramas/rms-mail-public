"use client";

import React, { useState, useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import {
  Mail,
  Settings,
  LogOut,
  Inbox,
  Send,
  Trash,
  Archive,
  Heart,
  Lock,
  Shield,
} from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import dynamic from "next/dynamic";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useLicenseInfo } from "@/hooks/useEmailQueries";
import { Crown } from "lucide-react";

const DynamicKeyboardShortcutsModal = dynamic(
  () =>
    import("@/components/keyboard-shortcuts-modal").then(
      (mod) => mod.KeyboardShortcutsModal,
    ),
  { ssr: false },
);
import { PWAInstallButton } from "@/components/pwa-install-button";
import { SupportModal } from "./support-modal";
import { NotificationCenter } from "./notification-center";
import { getShowAccountNameEnabled } from "@/components/general-tab";
import { useGroupAccounts } from "@/hooks/useEmailQueries";
import { useBulkEmailAction } from "@/hooks/useEmailMutations";
import { getEdition } from "@/hooks/useEmails";
import { useGetMe } from "@/hooks/useAdminQueries";
import {
  type Account,
  type Folder,
  type ProjectGroup,
} from "@/hooks/useEmailTypes";
import { Button } from "@/components/ui/button";

interface EmailSidebarProps {
  activeAccount: string;
  setActiveAccount: (id: string) => void;
  activeFolder: string;
  setActiveFolder: (id: string) => void;
  accounts: Account[];
  groups: ProjectGroup[];
  folders: Folder[];
  unreadCounts: { [key: string]: number };
  editionLetter: () => string;
  onOpenSettings: () => void;
  onLogout: () => void;
  showAccountFolders: boolean;
  setShowAccountFolders: (v: boolean) => void;
  locale: string;
  mobileView: "sidebar" | "list" | "viewer";
  setMobileView: (v: "sidebar" | "list" | "viewer") => void;
  onEmptyTrash: () => void;
}

const EmailSidebarComponent = ({
  activeAccount,
  setActiveAccount,
  activeFolder,
  setActiveFolder,
  accounts,
  groups,
  folders,
  unreadCounts,
  editionLetter,
  onOpenSettings,
  onLogout,
  showAccountFolders,
  setShowAccountFolders,
  locale,
  mobileView,
  setMobileView,
  onEmptyTrash,
}: EmailSidebarProps) => {
  const t = useTranslations("mail");
  const tSettings = useTranslations("settings");
  const { data: licenseInfo } = useLicenseInfo();
  const isPremium = licenseInfo?.status === "active";
  void unreadCounts;
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null);
  const [hoveredFolderId, setHoveredFolderId] = useState<string | null>(null);
  const [isSupportModalOpen, setIsSupportModalOpen] = useState(false);
  const mounted = React.useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  );
  const groupAccounts = useGroupAccounts(expandedGroup);
  const bulkAction = useBulkEmailAction();
  const isDesktop = useMediaQuery("(min-width: 1024px)");
  const { data: user, isLoading: isUserLoading } = useGetMe();

  const edition = getEdition();
  const isMono = edition === "mono" || edition === "mono_pro";
  const isMonoPro = edition === "mono_pro";
  const [showAccountName, setShowAccountName] = useState(() => {
    if (typeof window !== "undefined") return getShowAccountNameEnabled();
    return false;
  });
  useEffect(() => {
    const handler = () => setShowAccountName(getShowAccountNameEnabled());
    window.addEventListener("rms-mail_settings_changed", handler);
    return () =>
      window.removeEventListener("rms-mail_settings_changed", handler);
  }, [mounted]);

  useEffect(() => {
    if (isMono && accounts.length > 0 && activeAccount !== accounts[0].id) {
      setActiveAccount(accounts[0].id);
      setShowAccountFolders(true);
    }
  }, [
    isMono,
    accounts,
    activeAccount,
    setActiveAccount,
    setShowAccountFolders,
  ]);

  const logoEdition = mounted ? editionLetter() : "U";

  const totalUnread = useMemo(() => {
    if (accounts.length === 0) return 0;
    return accounts.reduce((acc, a) => acc + (a.unread_inbox ?? 0), 0);
  }, [accounts]);

  const handleAccountSelect = (accountId: string) => {
    setActiveAccount(accountId);
    setShowAccountFolders(true);
    setActiveFolder("");

    // Trigger priority sync for the selected account (Unified only, non-group)
    if (accountId !== "unified" && !accountId.startsWith("group:")) {
      fetch(`/api/accounts/${accountId}/check-now`, {
        method: "POST",
        credentials: "include",
      }).catch(() => {
        // Fail silently — this is a best-effort acceleration
      });
    }
  };

  const handleFolderSelect = (folderId: string) => {
    setActiveFolder(folderId);
  };

  const handleGroupClick = (g: ProjectGroup) => {
    if (activeAccount === `group:${g.id}`) {
      setActiveAccount("unified");
      setExpandedGroup(null);
    } else {
      setActiveAccount(`group:${g.id}`);
      setExpandedGroup(g.id);
      setActiveFolder("");
    }
    setMobileView("list");
  };

  const handleDrop = (e: React.DragEvent, folderId: string) => {
    e.preventDefault();
    setHoveredFolderId(null);
    try {
      const payload = e.dataTransfer.getData("application/rms-email-ids");
      if (payload) {
        const ids = JSON.parse(payload);
        if (ids.length > 0) {
          if (folderId === "__trash__") {
            bulkAction.mutate({ action: "delete", ids });
          } else if (folderId === "__archive__") {
            bulkAction.mutate({ action: "archive", ids });
          } else if (folderId !== "__sent__") {
            bulkAction.mutate({ action: "move", ids, folderId });
          }
        }
      } else {
        // Fallback for single drag if old format
        const id = e.dataTransfer.getData("application/rms-email-id");
        if (id) {
          if (folderId === "__trash__")
            bulkAction.mutate({ action: "delete", ids: [id] });
          else if (folderId === "__archive__")
            bulkAction.mutate({ action: "archive", ids: [id] });
          else if (folderId !== "__sent__")
            bulkAction.mutate({ action: "move", ids: [id], folderId });
        }
      }
    } catch {
      // ignore
    }
  };

  return (
    <>
      {/* Sidebar */}
      <aside
        className={`w-full lg:w-64 border-e border-border-muted/50 flex flex-col ${mobileView !== "sidebar" ? "hidden lg:flex" : "flex fixed lg:relative inset-0 z-40 bg-app-bg"}`}
      >
        <div className="h-16 px-4 flex items-center gap-2 font-bold text-xl border-b border-border-muted/50 shrink-0">
          <Mail className="w-6 h-6 text-primary" />
          <span className="flex-1">
            <span className="text-primary">RMS</span> Mail{" "}
            <span className="text-primary align-super text-sm">
              {logoEdition}
            </span>
          </span>
          {!isMono &&
            (isPremium ? (
              <span
                className="text-amber-500 cursor-default"
                title={t("premium")}
              >
                <Crown className="w-4 h-4" />
              </span>
            ) : (
              <button
                onClick={() => {}}
                className="text-[10px] bg-muted text-muted-foreground px-1.5 py-0.5 rounded-full font-semibold hover:bg-amber-500/10 hover:text-amber-600 transition-colors"
              >
                {t("free_edition")}
              </button>
            ))}
          <button
            className="lg:hidden text-text-muted"
            onClick={() => setMobileView("list")}
          >
            ✕
          </button>
        </div>
        <nav className="flex-1 overflow-y-auto p-4 space-y-4">
          <div>
            {!isMono && (
              <h3 className="text-xs font-semibold text-text-muted uppercase tracking-wider mb-2">
                {t("all_accounts")}
              </h3>
            )}
            {/* Project Groups */}
            {groups.filter((g: ProjectGroup) => (g.accounts_count ?? 0) > 0)
              .length > 0 && (
              <div className="mb-2 space-y-0.5">
                {groups
                  .filter((g: ProjectGroup) => (g.accounts_count ?? 0) > 0)
                  .map((g: ProjectGroup) => (
                    <div key={g.id}>
                      <Button
                        variant={
                          activeAccount === `group:${g.id}`
                            ? "secondary"
                            : "ghost"
                        }
                        className={`w-full justify-start text-sm ${g.is_locked ? "opacity-60" : ""}`}
                        onClick={() => {
                          if (g.is_locked) {
                            return;
                          }
                          handleGroupClick(g);
                        }}
                      >
                        <span className="text-xs me-1 shrink-0">
                          {expandedGroup === g.id ? "▼" : "▶"}
                        </span>
                        <span
                          className="w-2.5 h-2.5 rounded-full me-2 shrink-0"
                          style={{ backgroundColor: g.color }}
                        />
                        <span className="flex-1 text-left flex items-center gap-1.5 min-w-0">
                          {g.is_locked && (
                            <Lock className="w-3 h-3 text-red-400 shrink-0" />
                          )}
                          <span className="truncate">{g.name}</span>
                        </span>
                        {(g.unread_count ?? 0) > 0 && (
                          <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                            {g.unread_count}
                          </span>
                        )}
                      </Button>
                      {expandedGroup === g.id && groupAccounts.data && (
                        <div className="ms-4 space-y-0.5 mt-0.5">
                          {accounts
                            .filter((a: Account) =>
                              groupAccounts.data?.includes(a.id),
                            )
                            .map((a: Account) => (
                              <div key={a.id}>
                                <Button
                                  variant={
                                    activeAccount === a.id
                                      ? "secondary"
                                      : "ghost"
                                  }
                                  size="sm"
                                  className={`w-full justify-between text-xs ${a.is_locked ? "opacity-60" : ""}`}
                                  onClick={() => {
                                    if (a.is_locked) return;
                                    handleAccountSelect(a.id);
                                  }}
                                >
                                  <span className="flex items-center gap-1.5 flex-1 min-w-0">
                                    {a.is_locked ? (
                                      <Lock className="w-3 h-3 shrink-0 text-red-400" />
                                    ) : (
                                      <Mail className="w-3 h-3 shrink-0" />
                                    )}
                                    <span className="truncate">
                                      {showAccountName && a.name
                                        ? a.name
                                        : a.email}
                                    </span>
                                  </span>
                                  {(a.unread_count ?? 0) > 0 && (
                                    <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                                      {a.unread_count}
                                    </span>
                                  )}
                                </Button>
                                {showAccountFolders &&
                                  activeAccount === a.id &&
                                  folders.length > 0 && (
                                    <div className="space-y-0.5 mt-0.5">
                                      {folders.map((folder: Folder) => (
                                        <div
                                          key={folder.id}
                                          className="w-full"
                                          onDragOver={(e) => {
                                            e.preventDefault();
                                            e.dataTransfer.dropEffect = "move";
                                            setHoveredFolderId(folder.id);
                                          }}
                                          onDragLeave={() =>
                                            setHoveredFolderId(null)
                                          }
                                          onDrop={(e) =>
                                            handleDrop(e, folder.id)
                                          }
                                        >
                                          <Button
                                            variant={
                                              activeFolder === folder.id
                                                ? "secondary"
                                                : "ghost"
                                            }
                                            size="sm"
                                            className={`w-full justify-between text-[11px] pe-4 transition-colors duration-200 ${
                                              hoveredFolderId === folder.id
                                                ? "bg-accent text-accent-foreground ring-1 ring-primary/50"
                                                : ""
                                            }`}
                                            onClick={() => {
                                              handleFolderSelect(folder.id);
                                              setMobileView("list");
                                            }}
                                          >
                                            <span className="ps-4">
                                              {folder.name}
                                            </span>
                                            {(folder.unread_count ?? 0) > 0 && (
                                              <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                                                {folder.unread_count}
                                              </span>
                                            )}
                                          </Button>
                                        </div>
                                      ))}
                                    </div>
                                  )}
                              </div>
                            ))}
                        </div>
                      )}
                    </div>
                  ))}
                <div className="border-t border-border-muted/50 my-2" />
              </div>
            )}
            {!isMono && (
              <Button
                variant={activeAccount === "unified" ? "secondary" : "ghost"}
                className="w-full justify-between"
                onClick={() => {
                  setActiveAccount("unified");
                  setShowAccountFolders(false);
                  setMobileView("list");
                }}
              >
                <span className="flex items-center gap-3">
                  <Inbox className="w-4 h-4" />
                  {t("inbox")}
                </span>
                {totalUnread > 0 && (
                  <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                    {totalUnread}
                  </span>
                )}
              </Button>
            )}
            {accounts
              .filter((a: Account) => !groupAccounts.data?.includes(a.id))
              .map((account: Account) => (
                <div key={account.id}>
                  <Button
                    variant={
                      activeAccount === account.id ? "secondary" : "ghost"
                    }
                    className={`w-full justify-between ${account.is_locked ? "opacity-60" : ""}`}
                    onClick={() => {
                      if (account.is_locked) return;
                      handleAccountSelect(account.id);
                    }}
                  >
                    <span className="flex items-center gap-3 flex-1 min-w-0">
                      {account.is_locked ? (
                        <Lock className="w-4 h-4 shrink-0 text-red-400" />
                      ) : (
                        <Mail className="w-4 h-4 shrink-0" />
                      )}
                      <span className="truncate">
                        {showAccountName && account.name
                          ? account.name
                          : account.email}
                      </span>
                    </span>
                    {(account.unread_count ?? 0) > 0 && (
                      <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                        {account.unread_count}
                      </span>
                    )}
                  </Button>
                  {showAccountFolders &&
                    activeAccount === account.id &&
                    folders.length > 0 && (
                      <div className="space-y-1 mt-1">
                        {folders.map((folder: Folder) => (
                          <div
                            key={folder.id}
                            className="w-full"
                            onDragOver={(e) => {
                              e.preventDefault();
                              e.dataTransfer.dropEffect = "move";
                              setHoveredFolderId(folder.id);
                            }}
                            onDragLeave={() => setHoveredFolderId(null)}
                            onDrop={(e) => handleDrop(e, folder.id)}
                          >
                            <Button
                              variant={
                                activeFolder === folder.id
                                  ? "secondary"
                                  : "ghost"
                              }
                              size="sm"
                              className={`w-full justify-between text-[11px] pe-4 transition-colors duration-200 ${
                                hoveredFolderId === folder.id
                                  ? "bg-accent text-accent-foreground ring-1 ring-primary/50"
                                  : ""
                              }`}
                              onClick={() => {
                                handleFolderSelect(folder.id);
                                setMobileView("list");
                              }}
                            >
                              <span className="ps-6">{folder.name}</span>
                              {(folder.unread_count ?? 0) > 0 && (
                                <span className="text-[10px] bg-muted-foreground/20 text-muted-foreground px-1 rounded-full shrink-0">
                                  {folder.unread_count}
                                </span>
                              )}
                            </Button>
                          </div>
                        ))}
                      </div>
                    )}
                </div>
              ))}
          </div>

          <div>
            {!isMono && (
              <h3 className="text-xs font-semibold text-text-muted uppercase tracking-wider mb-2">
                {t("folders.title")}
              </h3>
            )}
            <div className="space-y-1">
              <Button
                variant="ghost"
                className="w-full justify-start"
                onClick={() => {
                  setActiveAccount("unified");
                  setActiveFolder("__sent__");
                  setShowAccountFolders(false);
                  setMobileView("list");
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                  // Cannot move to sent
                }}
                onDrop={(e) => {
                  e.preventDefault();
                }}
              >
                <Send className="w-4 h-4 me-3" /> {t("folders.sent")}
              </Button>
              <Button
                variant="ghost"
                className="w-full justify-start"
                onClick={() => {
                  setActiveAccount("unified");
                  setActiveFolder("__archive__");
                  setShowAccountFolders(false);
                  setMobileView("list");
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                  setHoveredFolderId("__archive__");
                }}
                onDragLeave={() => setHoveredFolderId(null)}
                onDrop={(e) => handleDrop(e, "__archive__")}
              >
                <Archive className="w-4 h-4 me-3" /> {t("folders.archive")}
              </Button>
              <Button
                variant="ghost"
                className="w-full justify-start"
                onClick={() => {
                  setActiveAccount("unified");
                  setActiveFolder("__trash__");
                  setShowAccountFolders(false);
                  setMobileView("list");
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                  setHoveredFolderId("__trash__");
                }}
                onDragLeave={() => setHoveredFolderId(null)}
                onDrop={(e) => handleDrop(e, "__trash__")}
              >
                <Trash className="w-4 h-4 me-3" /> {t("folders.trash")}
              </Button>
              <div className="flex gap-1 ms-7">
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-[10px] text-muted-foreground hover:text-red-400"
                  onClick={onEmptyTrash}
                >
                  {t("empty_trash")}
                </Button>
              </div>
            </div>
          </div>
        </nav>
        <div className="px-4 pb-2 flex items-center justify-center gap-2">
          {isDesktop && (
            <>
              {(!isMonoPro || user?.is_admin) && <NotificationCenter />}
              <Button
                variant="ghost"
                size="icon"
                className="w-8 h-8 rounded-full text-muted-foreground hover:text-foreground"
                title={tSettings("support_author")}
                onClick={() => setIsSupportModalOpen(true)}
              >
                <Heart className="w-4 h-4" />
              </Button>
              <DynamicKeyboardShortcutsModal />
            </>
          )}
        </div>
        <div className="p-3 border-t border-border-muted/50 space-y-2">
          <div className="flex items-center justify-center gap-2">
            <LanguageToggle locale={locale} />
            <ThemeToggle />
          </div>
          <PWAInstallButton />
          {!isUserLoading &&
            user?.is_admin &&
            (edition === "mono_pro" || edition === "teams") && (
              <Button
                variant="ghost"
                className="w-full justify-center text-xs text-blue-500 hover:text-blue-600 hover:bg-blue-100 dark:text-blue-400 dark:hover:text-blue-300 dark:hover:bg-blue-500/10"
                onClick={() => (window.location.href = `/${locale}/admin`)}
              >
                <Shield className="w-3.5 h-3.5 me-2" />{" "}
                {tSettings("tab_admin", { defaultMessage: "Admin Panel" })}
              </Button>
            )}
          <Button
            variant="ghost"
            className="w-full justify-center text-xs"
            onClick={onOpenSettings}
          >
            <Settings className="w-3.5 h-3.5 me-2" /> {t("actions.settings")}
          </Button>
          <Button
            variant="ghost"
            className="w-full justify-center text-xs text-red-500 hover:text-red-600 hover:bg-red-100 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-500/10"
            onClick={onLogout}
          >
            <LogOut className="w-3.5 h-3.5 me-2" /> {t("actions.logout")}
          </Button>
        </div>
      </aside>

      <SupportModal
        isOpen={isSupportModalOpen}
        onClose={() => setIsSupportModalOpen(false)}
      />
    </>
  );
};

export const EmailSidebar = React.memo(EmailSidebarComponent);
