"use client";

import React from "react";
import { useTranslations } from "next-intl";
import { Send, X } from "lucide-react";
import { Composer } from "@/components/composer";
import { RecipientInput } from "@/components/recipient-input";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Email, Identity, Account } from "@/hooks/useEmails";

interface ComposePanelProps {
  /** Whether the compose/reply panel is visible */
  isReplying: boolean;
  /** Toggle reply/compose state */
  setIsReplying: (v: boolean) => void;
  /** Original email being replied to (null for new compose) */
  replyTo: Email | null;
  /** Recipient addresses */
  initialComposeTo?: string[];
  /** CC addresses */
  initialComposeCc?: string[];
  /** Subject line */
  initialComposeSubject?: string;
  /** Email body (HTML) */
  initialComposeBody?: string;
  /** Accounts list */
  accounts: Account[];
  /** Available sending identities for this account */
  identities: Identity[];
  /** Currently selected account ID */
  selectedAccountId: string;
  /** Identity selection */
  initialFromIdentity?: string;
  /** Send handler */
  onSend: (options: {
    to: string[];
    cc: string[];
    subject: string;
    body: string;
    html: string;
    identity: string;
  }) => void;
  /** Whether send is in progress */
  sendPending?: boolean;
  /** Uploaded file attachments */
  attachments?: Array<{ id: string; filename: string; size: number }>;
  /** Remove attachment at index */
  onRemoveAttachment?: (index: number) => void;
  /** Trigger file picker */
  onFileAttachClick?: () => void;
  /** Save draft handler */
  onSaveDraft?: (
    syncRemote: boolean,
    data: {
      to: string[];
      cc: string[];
      subject: string;
      body: string;
      identity: string;
    },
  ) => void;
  /** Whether save draft is in progress */
  saveDraftPending?: boolean;
}

export function ComposePanel({
  isReplying,
  setIsReplying,
  replyTo,
  initialComposeTo = [],
  initialComposeCc = [],
  initialComposeSubject = "",
  initialComposeBody = "",
  initialFromIdentity = "",
  accounts,
  identities,
  selectedAccountId,
  onSend,
  sendPending = false,
  attachments = [],
  onRemoveAttachment,
  onFileAttachClick,
  onSaveDraft,
  saveDraftPending = false,
}: ComposePanelProps) {
  const t = useTranslations("mail");

  const [composeTo, setComposeTo] = React.useState(initialComposeTo);
  const [composeCc, setComposeCc] = React.useState(initialComposeCc);
  const [composeSubject, setComposeSubject] = React.useState(
    initialComposeSubject,
  );
  const [composeBody, setComposeBody] = React.useState(initialComposeBody);
  const [fromIdentity, setFromIdentity] = React.useState(initialFromIdentity);

  React.useEffect(() => {
    if (isReplying) {
      queueMicrotask(() => {
        setComposeTo(initialComposeTo);
        setComposeCc(initialComposeCc);
        setComposeSubject(initialComposeSubject);
        setComposeBody(initialComposeBody);
        setFromIdentity(initialFromIdentity);
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isReplying]);

  const handleSend = () => {
    onSend({
      to: composeTo,
      cc: composeCc,
      subject: composeSubject,
      body: composeBody,
      html: composeBody,
      identity: fromIdentity,
    });
  };

  // Local auto-save
  React.useEffect(() => {
    if (!onSaveDraft) return;
    const timer = setTimeout(() => {
      if (composeSubject || composeBody || composeTo.length > 0) {
        onSaveDraft(false, {
          to: composeTo,
          cc: composeCc,
          subject: composeSubject,
          body: composeBody,
          identity: fromIdentity,
        });
      }
    }, 2000); // 2 seconds debounce
    return () => clearTimeout(timer);
  }, [
    composeTo,
    composeCc,
    composeSubject,
    composeBody,
    fromIdentity,
    onSaveDraft,
  ]);

  if (!isReplying) return null;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="p-4 bg-app-bg border border-border-muted rounded-xl space-y-3">
        {/* To field */}
        <RecipientInput
          id="compose-to"
          value={composeTo}
          onChange={setComposeTo}
          placeholder={t("to_placeholder")}
          accountId={
            selectedAccountId !== "unified" ? selectedAccountId : undefined
          }
        />

        {/* Cc field */}
        <RecipientInput
          value={composeCc}
          onChange={setComposeCc}
          placeholder={t("cc_placeholder")}
          accountId={
            selectedAccountId !== "unified" ? selectedAccountId : undefined
          }
        />

        {/* Subject */}
        <Input
          placeholder={t("subject")}
          value={composeSubject}
          onChange={(e) => setComposeSubject(e.target.value)}
        />

        {/* Identity selector (From) */}
        <select
          className="h-9 rounded-md border bg-card-bg px-2 py-1 text-sm text-foreground shadow-sm w-full mb-3"
          value={fromIdentity}
          onChange={(e) => setFromIdentity(e.target.value)}
        >
          {selectedAccountId !== "unified" ? (
            <>
              <option value="">
                {t("actions.from")} —{" "}
                {accounts?.find((a: Account) => a.id === selectedAccountId)
                  ?.name ||
                  accounts?.find((a: Account) => a.id === selectedAccountId)
                    ?.email ||
                  ""}
              </option>
              {identities
                ?.filter((i: Identity) => i.account_id === selectedAccountId)
                .map((ident: Identity) => (
                  <option key={ident.id} value={`identity:${ident.id}`}>
                    {ident.name
                      ? `${ident.name} <${ident.email}>`
                      : ident.email}
                  </option>
                ))}
            </>
          ) : (
            <>
              <option value="">
                {t("actions.from")} — {t("default")}
              </option>
              {accounts?.map((a: Account) => (
                <React.Fragment key={a.id}>
                  <option value={`account:${a.id}`}>{a.name || a.email}</option>
                  {identities
                    ?.filter((i: Identity) => i.account_id === a.id)
                    .map((ident: Identity) => (
                      <option key={ident.id} value={`identity:${ident.id}`}>
                        &nbsp;&nbsp;↳{" "}
                        {ident.name
                          ? `${ident.name} <${ident.email}>`
                          : ident.email}
                      </option>
                    ))}
                </React.Fragment>
              ))}
            </>
          )}
        </select>

        {/* Body composer */}
        <Composer
          value={composeBody}
          onChange={setComposeBody}
          placeholder={
            replyTo && !composeTo.includes("")
              ? t("write_reply")
              : t("write_message")
          }
        />

        {/* Attachments */}
        {attachments.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {attachments.map((att, i) => (
              <div
                key={att.id}
                className="flex items-center gap-1.5 bg-muted px-2 py-1 rounded text-xs"
              >
                <span className="text-text-muted">📎</span>
                <span className="text-text-main/80">{att.filename}</span>
                {onRemoveAttachment && (
                  <button
                    className="text-red-400 hover:text-red-300"
                    onClick={() => onRemoveAttachment(i)}
                  >
                    <X className="w-3 h-3" />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Action bar */}
        <div className="flex justify-between items-center">
          {onFileAttachClick && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onFileAttachClick}
              title={t("attach")}
            >
              📎 {t("attach")}
            </Button>
          )}
          <div className="flex gap-2 ms-auto">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setIsReplying(false)}
            >
              {t("actions.cancel")}
            </Button>
            {onSaveDraft && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  onSaveDraft(true, {
                    to: composeTo,
                    cc: composeCc,
                    subject: composeSubject,
                    body: composeBody,
                    identity: fromIdentity,
                  })
                }
                disabled={saveDraftPending}
              >
                {saveDraftPending
                  ? t("actions.saving")
                  : t("actions.save_draft", { defaultMessage: "Save Draft" })}
              </Button>
            )}
            <Button
              size="sm"
              onClick={handleSend}
              disabled={sendPending || !composeBody.trim()}
            >
              {sendPending ? (
                t("actions.sending")
              ) : (
                <>
                  <Send className="w-4 h-4 me-1" />
                  {t("actions.send")}
                </>
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
