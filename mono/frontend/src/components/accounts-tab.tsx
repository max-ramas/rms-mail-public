"use client";

import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Trash2,
  Edit3,
  Link,
  RefreshCw,
  Pause,
  Play,
} from "lucide-react";
import {
  useCreateAccount,
  useDeleteAccount,
  useUpdateAccount,
  useOAuthURL,
  useResetAccountSync,
  usePauseAccountSync,
  useResumeAccountSync,
  useCreateIdentity,
  useDeleteIdentity,
  useUpdateSmartCategories,
} from "@/hooks/useAdminQueries";
import { useIdentities, useLicenseInfo } from "@/hooks/useEmailQueries";
import { apiFetch } from "@/lib/api-client";
import { type Account } from "@/hooks/useEmailTypes";
import { useToast } from "@/hooks/useToast";
import { useTranslations } from "next-intl";
import { formatEmailDatetime } from "@/lib/date-format";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";

export function AccountsTab({
  accounts,
  selectedAccountId,
  setSelectedAccountId,
  isMono,
}: {
  accounts: Account[];
  selectedAccountId: string;
  setSelectedAccountId: (id: string) => void;
  isMono?: boolean;
}) {
  const t = useTranslations("settings");
  const toast = useToast();
  const createAcc = useCreateAccount();
  const deleteAcc = useDeleteAccount();
  const updateAcc = useUpdateAccount();
  const updateSmartCats = useUpdateSmartCategories();
  const resetSync = useResetAccountSync();
  const pauseSync = usePauseAccountSync();
  const resumeSync = useResumeAccountSync();
  const oauthURL = useOAuthURL();
  const queryClient = useQueryClient();
  const { data: licenseInfo } = useLicenseInfo();

  const isLimited =
    licenseInfo?.status === "unlicensed" ||
    licenseInfo?.status === "expired" ||
    licenseInfo?.status === "revoked";
  const limitReached = !isMono && isLimited && accounts.length >= 5;

  const def = {
    email: "",
    name: "",
    password: "",
    provider: "",
    imapHost: "imap.gmail.com",
    imapPort: 993,
    imapEnc: "ssl",
    smtpHost: "smtp.gmail.com",
    smtpPort: 465,
    smtpEnc: "ssl",
    signature: "",
  };
  const [form, setForm] = useState(def);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<string | null>(null);
  const [testError, setTestError] = useState("");
  const [resolving, setResolving] = useState(false);

  const handleEmailBlur = async (email: string) => {
    if (!email) return;
    setResolving(true);
    try {
      const r = await apiFetch(
        `/api/mail/resolve?email=${encodeURIComponent(email)}`,
      );
      if (r.ok) {
        const data = await r.json();
        setForm((prev) => ({
          ...prev,
          imapHost: data.imap_host || "",
          imapPort: data.imap_port || 993,
          imapEnc: data.imap_encryption || "ssl",
          smtpHost: data.smtp_host || "",
          smtpPort: data.smtp_port || 465,
          smtpEnc: data.smtp_encryption || "starttls",
        }));
      }
    } catch {
      // Ignore errors, user can fill manually
    } finally {
      setResolving(false);
    }
  };

  const handleTest = async () => {
    setTestLoading(true);
    setTestResult(null);
    try {
      interface TestConnectionBody {
        imap_host: string;
        imap_port: number;
        imap_ssl: boolean;
        username: string;
        password?: string;
        account_id?: string;
      }
      const body: TestConnectionBody = {
        imap_host: form.imapHost,
        imap_port: form.imapPort,
        imap_ssl: form.imapEnc !== "none",
        username: form.email,
        password: form.password,
      };
      if (editingId) body.account_id = editingId;
      const r = await apiFetch("/api/accounts/test-connection", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const j = await r.json();
      setTestResult(j.status);
      if (j.status !== "ok") setTestError(j.error?.slice(0, 50));
    } catch (e) {
      const err = e as Error;
      setTestResult("error");
      setTestError(err.message?.slice(0, 50) || t("unknown_error"));
    }
    setTestLoading(false);
  };

  const reset = () => {
    setForm(def);
    setShowAdvanced(false);
    setEditingId(null);
  };

  const handleSave = async () => {
    if (!form.email) {
      toast.addToast(t("toast_email_required"), "error");
      return;
    }
    try {
      const data = {
        email: form.email,
        name: form.name,
        provider: form.provider || "custom",
        imap_host: form.imapHost,
        imap_port: form.imapPort,
        imap_ssl: form.imapEnc !== "none",
        imap_encryption: form.imapEnc,
        smtp_host: form.smtpHost,
        smtp_port: form.smtpPort,
        smtp_ssl: form.smtpEnc !== "none",
        smtp_encryption: form.smtpEnc,
        username: form.email,
        password: form.password,
        ai_provider_config: "{}",
        signature: form.signature,
      };

      if (editingId) {
        await updateAcc.mutateAsync({ id: editingId, ...data });
        toast.addToast(t("toast_updated"), "success");
      } else {
        await createAcc.mutateAsync(data);
        toast.addToast(t("toast_added"), "success");
      }
      reset();
    } catch (e: unknown) {
      const err = e as { response?: { data?: { code?: string } } };
      if (err?.response?.data?.code === "ERROR_RESOLUTION_FAILED") {
        setShowAdvanced(true);
        toast.addToast(t("resolution_failed"), "error");
      } else {
        toast.addToast(t("toast_failed"), "error");
      }
    }
  };

  const handleOAuth = async (provider: string) => {
    try {
      const r = await oauthURL.mutateAsync(provider);

      const width = 500;
      const height = 600;
      const left = window.screenX + (window.outerWidth - width) / 2;
      const top = window.screenY + (window.outerHeight - height) / 2;

      window.open(
        r.url,
        "OAuthPopup",
        `width=${width},height=${height},left=${left},top=${top},status=no,menubar=no,toolbar=no`,
      );

      const handleResult = (data: {
        status: string;
        email?: string | null;
        error?: string | null;
      }) => {
        if (data.status === "success") {
          toast.addToast(
            t("toast_added") + (data.email ? ": " + data.email : ""),
            "success",
          );
          queryClient.invalidateQueries({ queryKey: ["accounts"] });
        } else {
          toast.addToast(data.error || t("toast_oauth_failed"), "error");
        }
      };

      const handleMessage = (event: MessageEvent) => {
        if (event.data?.type === "OAUTH_RESULT") {
          window.removeEventListener("message", handleMessage);
          window.removeEventListener("storage", handleStorage);
          handleResult(event.data);
        }
      };

      const handleStorage = (event: StorageEvent) => {
        if (event.key === "oauth_result" && event.newValue) {
          try {
            const data = JSON.parse(event.newValue);
            if (data.type === "OAUTH_RESULT") {
              window.removeEventListener("message", handleMessage);
              window.removeEventListener("storage", handleStorage);
              localStorage.removeItem("oauth_result");
              handleResult(data);
            }
          } catch {}
        }
      };

      window.addEventListener("message", handleMessage);
      window.addEventListener("storage", handleStorage);
    } catch {
      toast.addToast(t("toast_oauth_failed"), "error");
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t("delete_account_confirm"))) return;
    try {
      await deleteAcc.mutateAsync(id);
      toast.addToast(t("toast_deleted"), "success");
    } catch {
      toast.addToast(t("toast_failed"), "error");
    }
  };

  const handleEdit = (acc: Account) => {
    setForm({
      email: acc.email,
      name: acc.name || "",
      password: "",
      provider: acc.provider,
      imapHost: acc.imap_host,
      imapPort: acc.imap_port,
      imapEnc: acc.imap_encryption || (acc.imap_ssl ? "ssl" : "none"),
      smtpHost: acc.smtp_host,
      smtpPort: acc.smtp_port,
      smtpEnc: acc.smtp_encryption || (acc.smtp_ssl ? "ssl" : "none"),
      signature: acc.signature || "",
    });
    setEditingId(acc.id);
    setSelectedAccountId(acc.id);
  };

  const encOpts = (prefix: string) => [
    <option key="ssl" value="ssl">
      {t(prefix + "_ssl_tls")}
    </option>,
    <option key="starttls" value="starttls">
      {t(prefix + "_starttls")}
    </option>,
    <option key="none" value="none">
      {t(prefix + "_none")}
    </option>,
  ];

  return (
    <div className="space-y-6">
      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("connected")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {accounts.length === 0 && (
            <p className="text-muted-foreground text-sm">{t("no_accounts")}</p>
          )}
          {accounts.map((a: Account) => (
            <div
              key={a.id}
              className={`flex items-center justify-between px-3 py-2 rounded border ${selectedAccountId === a.id ? "border-primary/50 bg-primary/5" : "border"}`}
            >
              <div>
                <div className="text-sm font-medium flex items-center gap-2">
                  {a.name ? `${a.email} / ${a.name}` : a.email}
                  {a.is_locked && (
                    <span className="text-[10px] bg-red-100 text-red-600 px-1.5 rounded uppercase font-semibold">
                      {t("locked")}
                    </span>
                  )}
                </div>
                <div className="text-xs text-muted-foreground">
                  {a.provider} · {a.imap_host}:{a.imap_port}
                </div>

                {a.last_sync_error ? (
                  <div key="error" className="text-xs text-red-400 mt-0.5">
                    <span>{a.last_sync_error}</span>
                  </div>
                ) : a.last_sync_at ? (
                  <div key="synced" className="text-xs text-green-400 mt-0.5">
                    <span>
                      {t("synced_at", {
                        date: formatEmailDatetime(a.last_sync_at, "en"),
                      })}
                    </span>
                  </div>
                ) : (
                  <div
                    key="never"
                    className="text-xs text-muted-foreground mt-0.5"
                  >
                    <span>{t("never_synced")}</span>
                  </div>
                )}
              </div>
              <div className="flex items-center gap-1">
                {(a.provider === "google" ||
                  a.imap_host?.includes("gmail.com")) && (
                  <div className="flex items-center gap-2 border-r pr-3 mr-2">
                    <Label
                      htmlFor={`smart-categories-${a.id}`}
                      className="text-xs text-muted-foreground"
                    >
                      {t("smart_categories", {
                        defaultMessage: "Smart Categories",
                      })}
                    </Label>
                    <Switch
                      id={`smart-categories-${a.id}`}
                      checked={a.smart_categories !== false}
                      onCheckedChange={(checked) => {
                        updateSmartCats.mutate(
                          { id: a.id, smart_categories: checked },
                          {
                            onSuccess: () =>
                              toast.addToast(
                                t("smart_categories_updated", {
                                  defaultMessage: "Smart categories updated",
                                }),
                                "success",
                              ),
                            onError: () =>
                              toast.addToast(t("toast_failed"), "error"),
                          },
                        );
                      }}
                      disabled={updateSmartCats.isPending}
                    />
                  </div>
                )}
                <div className="flex gap-1">
                  <button
                    onClick={() => {
                      if (a.is_sync_paused) {
                        resumeSync.mutate(a.id, {
                          onSuccess: () =>
                            toast.addToast(t("resume_sync_done"), "success"),
                          onError: () =>
                            toast.addToast(t("toast_failed"), "error"),
                        });
                      } else {
                        pauseSync.mutate(a.id, {
                          onSuccess: () =>
                            toast.addToast(t("pause_sync_done"), "success"),
                          onError: () =>
                            toast.addToast(t("toast_failed"), "error"),
                        });
                      }
                    }}
                    disabled={pauseSync.isPending || resumeSync.isPending}
                    title={
                      a.is_sync_paused ? t("resume_sync") : t("pause_sync")
                    }
                    className={`${a.is_sync_paused ? "text-amber-400" : ""} inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-9 px-2`}
                  >
                    {pauseSync.isPending || resumeSync.isPending ? (
                      <RefreshCw className="w-3 h-3 animate-spin" />
                    ) : a.is_sync_paused ? (
                      <Play className="w-3 h-3" />
                    ) : (
                      <Pause className="w-3 h-3" />
                    )}
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (confirm(t("reset_sync_confirm"))) {
                        resetSync.mutate(a.id, {
                          onSuccess: () =>
                            toast.addToast(t("reset_sync_done"), "success"),
                          onError: () =>
                            toast.addToast(t("toast_failed"), "error"),
                        });
                      }
                    }}
                    disabled={resetSync.isPending}
                    title={t("reset_sync")}
                  >
                    <RefreshCw
                      className={`w-3 h-3 ${resetSync.isPending && resetSync.variables === a.id ? "animate-spin" : ""}`}
                    />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleEdit(a)}
                  >
                    <Edit3 className="w-3 h-3" />
                  </Button>
                  {!isMono && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDelete(a.id)}
                    >
                      <Trash2 className="w-3 h-3" />
                    </Button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      {(!isMono || editingId) && (
        <Card className="border">
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              {editingId ? t("edit_account") : t("add_account")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="text-[10px] text-muted-foreground uppercase h-4 flex items-center justify-between">
                  <span>{t("email")}</span>
                  {resolving && (
                    <RefreshCw className="w-3 h-3 animate-spin text-muted-foreground" />
                  )}
                </label>
                <Input
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  onBlur={(e) => handleEmailBlur(e.target.value)}
                  placeholder={t("email")}
                  type="email"
                  disabled={isMono}
                />
              </div>
              <div>
                <label className="text-[10px] text-muted-foreground uppercase h-4 flex items-center">
                  {t("name")}
                </label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder={t("placeholder_name")}
                  type="text"
                />
              </div>
            </div>

            {!isMono && (
              <>
                <div>
                  <label className="text-[10px] text-muted-foreground uppercase h-4 flex items-center">
                    {editingId
                      ? t("leave_blank_keep_current")
                      : t("password_app")}
                  </label>
                  <Input
                    value={form.password}
                    onChange={(e) =>
                      setForm({ ...form, password: e.target.value })
                    }
                    placeholder={t("password_app")}
                    type="password"
                  />
                </div>

                {!showAdvanced && (
                  <div className="text-right">
                    <button
                      type="button"
                      onClick={() => setShowAdvanced(true)}
                      className="text-xs text-primary hover:underline focus:outline-none"
                    >
                      {t("advanced_settings")}
                    </button>
                  </div>
                )}

                {showAdvanced && (
                  <div className="space-y-3 pt-3 border-t border-border mt-3">
                    <p className="text-xs font-semibold text-foreground uppercase">
                      {t("advanced_settings")}
                    </p>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="text-[10px] text-muted-foreground uppercase">
                          {t("imap_host")}
                        </label>
                        <Input
                          value={form.imapHost}
                          onChange={(e) =>
                            setForm({ ...form, imapHost: e.target.value })
                          }
                        />
                      </div>
                      <div>
                        <label className="text-[10px] text-muted-foreground uppercase">
                          {t("imap_port")}
                        </label>
                        <Input
                          value={form.imapPort}
                          onChange={(e) =>
                            setForm({
                              ...form,
                              imapPort: parseInt(e.target.value) || 993,
                            })
                          }
                          type="number"
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="text-[10px] text-muted-foreground uppercase">
                          {t("smtp_host")}
                        </label>
                        <Input
                          value={form.smtpHost}
                          onChange={(e) =>
                            setForm({ ...form, smtpHost: e.target.value })
                          }
                        />
                      </div>
                      <div>
                        <label className="text-[10px] text-muted-foreground uppercase">
                          {t("smtp_port")}
                        </label>
                        <Input
                          value={form.smtpPort}
                          onChange={(e) =>
                            setForm({
                              ...form,
                              smtpPort: parseInt(e.target.value) || 465,
                            })
                          }
                          type="number"
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <select
                        value={form.imapEnc}
                        onChange={(e) =>
                          setForm({ ...form, imapEnc: e.target.value })
                        }
                        className="h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm"
                      >
                        {encOpts("imap")}
                      </select>
                      <select
                        value={form.smtpEnc}
                        onChange={(e) =>
                          setForm({ ...form, smtpEnc: e.target.value })
                        }
                        className="h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm"
                      >
                        {encOpts("smtp")}
                      </select>
                    </div>
                  </div>
                )}
              </>
            )}

            <div>
              <label className="text-[10px] text-muted-foreground uppercase">
                {t("signature")}
              </label>
              <textarea
                className="w-full bg-muted border rounded px-2 py-1.5 text-sm resize-none h-16"
                value={form.signature}
                onChange={(e) =>
                  setForm({ ...form, signature: e.target.value })
                }
                placeholder={t("signature_placeholder")}
              />
            </div>
            <div className="flex gap-2 items-center flex-wrap">
              <Button
                size="sm"
                onClick={!editingId && limitReached ? () => {} : handleSave}
                className={!editingId && limitReached ? "opacity-50" : ""}
              >
                <Plus className="w-3 h-3 me-1" />{" "}
                {editingId ? t("save") : t("add_account")}
              </Button>
              {limitReached && !editingId && (
                <span className="text-xs text-red-500 font-medium ml-2">
                  {t("account_limit_reached")}
                </span>
              )}
              {editingId && (
                <Button size="sm" variant="ghost" onClick={reset}>
                  {t("cancel")}
                </Button>
              )}

              {!isMono && (
                <>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleTest}
                    disabled={testLoading}
                  >
                    {testLoading ? "..." : t("test_connection")}
                  </Button>
                  <div className="flex-1" />
                  {testResult && (
                    <span
                      className={`text-xs ${testResult === "ok" ? "text-green-400" : "text-red-400"}`}
                    >
                      {testResult === "ok"
                        ? t("connected_ok")
                        : t("connection_error", { error: testError })}
                    </span>
                  )}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleOAuth("google")}
                  >
                    <Link className="w-3 h-3 me-1" /> {t("oauth_google")}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleOAuth("microsoft")}
                  >
                    <Link className="w-3 h-3 me-1" /> {t("oauth_microsoft")}
                  </Button>
                </>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {editingId && <IdentitiesSection accountId={editingId} />}
    </div>
  );
}

interface Identity {
  id: string;
  account_id: string;
  email: string;
  name: string;
}

function IdentitiesSection({ accountId }: { accountId: string }) {
  const t = useTranslations("settings");
  const toast = useToast();
  const identitiesQuery = useIdentities(accountId);
  const createIdentity = useCreateIdentity();
  const deleteIdentity = useDeleteIdentity();
  const [newEmail, setNewEmail] = useState("");
  const [newName, setNewName] = useState("");

  const handleAdd = () => {
    if (!newEmail.trim()) {
      toast.addToast("Email required", "error");
      return;
    }
    createIdentity.mutate(
      {
        account_id: accountId,
        email: newEmail.trim(),
        name: newName.trim(),
      },
      {
        onSuccess: () => {
          setNewEmail("");
          setNewName("");
          toast.addToast(t("toast_identity_added"), "success");
        },
        onError: () => toast.addToast(t("toast_identity_add_error"), "error"),
      },
    );
  };

  const identities = (identitiesQuery.data || []) as Identity[];

  return (
    <Card className="border">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{t("add_identity")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {identities.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("no_identities")}</p>
        )}
        {identities.map((ident: Identity) => (
          <div
            key={ident.id}
            className="flex items-center justify-between px-3 py-2 rounded border"
          >
            <div className="text-sm">
              {ident.name ? (
                <>
                  <span className="font-medium">{ident.name}</span>
                  <span className="text-muted-foreground ms-2">
                    &lt;{ident.email}&gt;
                  </span>
                </>
              ) : (
                <span>{ident.email}</span>
              )}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => deleteIdentity.mutate(ident.id)}
            >
              <Trash2 className="w-3 h-3 text-red-400" />
            </Button>
          </div>
        ))}
        <div className="flex gap-2 items-end">
          <div className="flex-1 space-y-1">
            <label className="text-[10px] text-muted-foreground uppercase">
              {t("email")}
            </label>
            <Input
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
              placeholder={t("email_placeholder")}
              type="email"
            />
          </div>
          <div className="flex-1 space-y-1">
            <label className="text-[10px] text-muted-foreground uppercase">
              {t("name")}
            </label>
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={t("placeholder_name")}
            />
          </div>
          <Button
            size="sm"
            onClick={handleAdd}
            disabled={createIdentity.isPending || !newEmail.trim()}
          >
            <Plus className="w-3 h-3 me-1" /> {t("add")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
