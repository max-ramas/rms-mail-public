"use client";
// @todo: Edition T (Teams) — Standalone compose form component

import React, { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Composer } from "@/components/composer";

import { Email, Contact, Identity } from "@/hooks/useEmailTypes";

export const ComposeForm = React.memo(function ComposeForm({
  isForwarding,
  replyTo,
  composeTo,
  setComposeTo,
  composeSubject,
  setComposeSubject,
  composeBody,
  setComposeBody,
  contacts,
  identities,
  onSend,
  onCancel,
  sendPending,
}: {
  isForwarding: boolean;
  replyTo: Email | null;
  composeTo: string;
  setComposeTo: (v: string) => void;
  composeSubject: string;
  setComposeSubject: (v: string) => void;
  composeBody: string;
  setComposeBody: (v: string) => void;
  contacts: Contact[];
  identities: Identity[];
  onSend: () => void;
  onCancel: () => void;
  sendPending: boolean;
}) {
  const t = useTranslations("mail");
  const [showDropdown, setShowDropdown] = useState(false);

  return (
    <div className="p-4 bg-app-bg border border-border-muted rounded-xl space-y-3">
      {(!replyTo || isForwarding) && (
        <>
          {identities.length > 0 && (
            <select className="w-full h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm">
              {identities.map((id: Identity) => (
                <option key={id.id} value={id.email}>
                  {id.name ? `${id.name} <${id.email}>` : id.email}
                </option>
              ))}
            </select>
          )}
          <div className="relative">
            <Input
              placeholder={t("to_placeholder")}
              value={composeTo}
              onChange={(e) => setComposeTo(e.target.value)}
              onFocus={() => setShowDropdown(true)}
              onBlur={() => setTimeout(() => setShowDropdown(false), 200)}
            />
            {showDropdown && contacts && composeTo && (
              <div className="absolute z-10 top-full mt-1 w-full bg-card border-border-muted rounded-lg shadow-lg max-h-40 overflow-y-auto">
                {contacts
                  .filter((c: Contact) =>
                    c.address.toLowerCase().includes(composeTo.toLowerCase()),
                  )
                  .slice(0, 8)
                  .map((c: Contact) => (
                    <div
                      key={c.address}
                      className="px-3 py-2 cursor-pointer hover:bg-muted text-sm"
                      onMouseDown={() => {
                        setComposeTo(c.address);
                        setShowDropdown(false);
                      }}
                    >
                      <span className="text-text-main">
                        {c.name || c.address}
                      </span>
                      {c.name && (
                        <span className="text-text-muted ms-2">
                          {c.address}
                        </span>
                      )}
                    </div>
                  ))}
              </div>
            )}
          </div>
          <Input
            placeholder={t("subject")}
            value={composeSubject}
            onChange={(e) => setComposeSubject(e.target.value)}
          />
        </>
      )}
      <Composer
        value={composeBody}
        onChange={setComposeBody}
        placeholder={
          replyTo && !isForwarding ? t("write_reply") : t("write_message")
        }
      />
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <input
          type="file"
          multiple
          className="text-xs file:me-2 file:py-1 file:px-2 file:rounded file:border-0 file:text-xs file:bg-muted file:text-foreground hover:file:bg-muted/80"
          onChange={(e) => {
            const files = e.target.files;
            if (!files) return;
            for (let i = 0; i < files.length; i++) {
              if (files[i].size > 25 * 1024 * 1024) {
                alert(
                  t("file_size_exceeded", { name: files[i].name, limit: "25" }),
                );
                e.target.value = "";
                return;
              }
            }
          }}
        />
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="secondary" size="sm" onClick={onCancel}>
          {t("actions.cancel")}
        </Button>
        <Button
          size="sm"
          onClick={onSend}
          disabled={sendPending || !composeBody.trim()}
        >
          {sendPending ? t("actions.sending") : t("actions.send")}
        </Button>
      </div>
    </div>
  );
});
