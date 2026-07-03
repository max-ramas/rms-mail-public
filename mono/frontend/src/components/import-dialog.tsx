"use client";

import React, { useState } from "react";
import axios from "axios";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Mail, X } from "lucide-react";

export interface ScanLocalAccount {
  email: string;
  username: string;
  imap_host: string;
  imap_port: number;
  smtp_host: string;
  smtp_port: number;
  use_ssl: boolean;
}

interface ImportDialogProps {
  accounts: ScanLocalAccount[];
  onClose: () => void;
  onImportComplete: () => void;
}

export function ImportDialog({
  accounts,
  onClose,
  onImportComplete,
}: ImportDialogProps) {
  const t = useTranslations("settings");
  const [selected, setSelected] = useState<Set<string>>(
    new Set(accounts.map((a) => a.email)),
  );
  const [password, setPassword] = useState("");
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState("");

  const toggle = (email: string) => {
    const next = new Set(selected);
    if (next.has(email)) {
      next.delete(email);
    } else {
      next.add(email);
    }
    setSelected(next);
  };

  const toggleAll = () => {
    if (selected.size === accounts.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(accounts.map((a) => a.email)));
    }
  };

  const handleImport = async () => {
    if (selected.size === 0) return;
    setError("");
    setImporting(true);

    try {
      const toImport = accounts.filter((a) => selected.has(a.email));
      await axios.post("/api/auth/import-local", {
        accounts: toImport,
        password,
      });
      onImportComplete();
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response) {
        setError(err.response.data?.message || t("import_failed"));
      } else {
        setError(t("import_connection_error"));
      }
    } finally {
      setImporting(false);
    }
  };

  const handleSkip = () => {
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <Card className="w-full max-w-md max-h-[80vh] overflow-y-auto">
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-lg flex items-center gap-2">
            <Mail className="w-5 h-5 text-primary" />
            {t("import_mailboxes_found")}
          </CardTitle>
          <button
            onClick={handleSkip}
            className="p-1 rounded-md hover:bg-muted transition-colors"
            title={t("close")}
          >
            <X className="w-5 h-5" />
          </button>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {t("import_mailboxes_desc")}
          </p>

          {/* Select All toggle */}
          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <Checkbox
              checked={selected.size === accounts.length}
              onCheckedChange={toggleAll}
            />
            <span className="font-medium">{t("select_all")}</span>
          </label>

          {/* Account list */}
          <div className="space-y-1 max-h-48 overflow-y-auto border rounded-md p-2">
            {accounts.map((acc) => (
              <label
                key={acc.email}
                className="flex items-center gap-2 py-1 text-sm cursor-pointer hover:bg-muted/50 rounded px-1"
              >
                <Checkbox
                  checked={selected.has(acc.email)}
                  onCheckedChange={() => toggle(acc.email)}
                />
                <span>{acc.email}</span>
              </label>
            ))}
          </div>

          {/* Password input */}
          <div className="space-y-2">
            <label className="text-sm font-medium">
              {t("import_password_label")}
            </label>
            <Input
              type="password"
              placeholder={t("import_password_placeholder")}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {t("import_password_hint")}
            </p>
          </div>

          {/* Error */}
          {error && (
            <p className="text-sm text-red-500" role="alert">
              {error}
            </p>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <Button variant="outline" onClick={handleSkip} className="flex-1">
              {t("skip")}
            </Button>
            <Button
              onClick={handleImport}
              disabled={importing || selected.size === 0 || !password}
              className="flex-1"
            >
              {importing
                ? t("importing")
                : t("import_count", { count: selected.size })}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
