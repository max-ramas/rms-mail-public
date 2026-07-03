"use client";

import React, { useState } from "react";
import {
  useWebhooks,
  useCreateWebhook,
  useDeleteWebhook,
  type Webhook,
} from "@/hooks/useWebhooks";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function WebhookManager({ accountId }: { accountId: string }) {
  const t = useTranslations("settings");
  const { data: webhooks, isLoading } = useWebhooks(accountId);
  const createWebhook = useCreateWebhook();
  const deleteWebhook = useDeleteWebhook();

  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");

  const handleCreate = async () => {
    if (!name.trim() || !url.trim()) return;
    await createWebhook.mutateAsync({
      account_id: accountId,
      name: name.trim(),
      url: url.trim(),
      secret: secret.trim(),
    });
    setName("");
    setUrl("");
    setSecret("");
  };

  if (isLoading)
    return <div className="text-muted-foreground text-sm">{t("loading")}</div>;

  return (
    <div className="space-y-4">
      <p className="text-muted-foreground text-xs">
        {t("webhooks_description")}
      </p>
      <div className="grid grid-cols-2 gap-2">
        <Input
          placeholder={t("webhook_name_placeholder")}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Input
          placeholder={t("webhook_url")}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
        <Input
          placeholder={t("webhook_secret_placeholder")}
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          className="col-span-1"
        />
        <Button size="sm" onClick={handleCreate} className="h-9">
          {t("add")}
        </Button>
      </div>
      <div className="space-y-1">
        {(webhooks || []).map((wh: Webhook) => (
          <div
            key={wh.id}
            className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted text-sm group"
          >
            <span className="w-2 h-2 rounded-full bg-green-500 shrink-0" />
            <span className="flex-1 text-xs">
              <span className="font-medium">{wh.name}</span>
              <span className="text-muted-foreground ml-1 truncate">
                {wh.url}
              </span>
              {wh.has_secret && (
                <span className="text-muted-foreground ml-1">(signed)</span>
              )}
            </span>
            <button
              className="text-xs text-muted-foreground hover:text-red-400 hidden group-hover:inline"
              onClick={() => deleteWebhook.mutate(wh.id)}
            >
              {t("del")}
            </button>
          </div>
        ))}
        {(!webhooks || webhooks.length === 0) && (
          <div className="text-muted-foreground text-xs text-center py-2">
            {t("no_webhooks")}
          </div>
        )}
      </div>
    </div>
  );
}
