"use client";

import React, { useState, useEffect, useRef, useImperativeHandle } from "react";
import { useTranslations } from "next-intl";
import { Composer } from "@/components/composer";
import { RecipientInput } from "@/components/recipient-input";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Account, Email, Identity } from "@/hooks/useEmails";

interface EmailReplyComposeFormProps {
  isReplying: boolean;
  isForwarding: boolean;
  replyTo: Email | null;
  composeTo: string[];
  composeCc: string[];
  composeSubject: string;
  composeBody: string;
  fromIdentity: string;
  forwardOriginalHtml?: string;
  forwardMeta?: {
    from: string;
    subject: string;
    date: string;
    to: string;
  } | null;
  composeAttachments: Array<{
    id: string;
    filename: string;
    size: number;
  }>;
  accounts: Account[];
  activeAccount: string;
  identities: Identity[] | undefined;
  onRemoveAttachment: (index: number) => void;
  onCancelReply: () => void;
  onSendEmail: (options: {
    to: string[];
    cc: string[];
    subject: string;
    body: string;
    html: string;
    identity: string;
  }) => void;
  sendPending: boolean;
  onSaveDraft?: (
    syncRemote: boolean,
    data?: {
      to: string[];
      cc: string[];
      subject: string;
      body: string;
      identity: string;
    },
  ) => void;
  saveDraftPending?: boolean;
  onFileInputClick: () => void;
}

export interface EmailReplyComposeFormHandle {
  insertBody: (html: string) => void;
}

export const EmailReplyComposeForm = React.forwardRef<
  EmailReplyComposeFormHandle,
  EmailReplyComposeFormProps
>(function EmailReplyComposeForm(
  {
  isReplying,
  isForwarding,
  replyTo,
  composeTo,
  composeCc,
  composeSubject,
  composeBody,
  fromIdentity,
  forwardOriginalHtml,
  forwardMeta,
  composeAttachments,
  accounts,
  activeAccount,
  identities,
  onRemoveAttachment,
  onCancelReply,
  onSendEmail,
  sendPending,
  onSaveDraft,
  saveDraftPending,
  onFileInputClick,
}: EmailReplyComposeFormProps,
  ref,
) {
  const t = useTranslations("mail");
  const forwardObserverRef = useRef<ResizeObserver | null>(null);

  const [localComposeTo, setLocalComposeTo] = useState(composeTo);
  const [localComposeCc, setLocalComposeCc] = useState(composeCc);
  const [localComposeSubject, setLocalComposeSubject] =
    useState(composeSubject);
  const [localComposeBody, setLocalComposeBody] = useState(composeBody);
  const [localFromIdentity, setLocalFromIdentity] = useState(fromIdentity);

  useImperativeHandle(ref, () => ({
    insertBody: (html: string) => setLocalComposeBody(html),
  }));

  useEffect(() => {
    return () => {
      forwardObserverRef.current?.disconnect();
    };
  }, []);

  useEffect(() => {
    if (isReplying || isForwarding) {
      setLocalComposeTo(composeTo);
      setLocalComposeCc(composeCc);
      setLocalComposeSubject(composeSubject);
      setLocalComposeBody(composeBody);
      setLocalFromIdentity(fromIdentity);
    }
  }, [
    isReplying,
    isForwarding,
    composeTo,
    composeCc,
    composeSubject,
    composeBody,
    fromIdentity,
  ]);

  useEffect(() => {
    if (!onSaveDraft || (!isReplying && !isForwarding)) return;
    const timer = setTimeout(() => {
      if (
        localComposeSubject ||
        localComposeBody ||
        localComposeTo.length > 0
      ) {
        onSaveDraft(false, {
          to: localComposeTo,
          cc: localComposeCc,
          subject: localComposeSubject,
          body: localComposeBody,
          identity: localFromIdentity,
        });
      }
    }, 2000);
    return () => clearTimeout(timer);
  }, [
    localComposeTo,
    localComposeCc,
    localComposeSubject,
    localComposeBody,
    localFromIdentity,
    isReplying,
    isForwarding,
    onSaveDraft,
  ]);

  if (!isReplying && !isForwarding) return null;

  return (
    <div className="p-4 bg-app-bg border border-border-muted rounded-xl space-y-3">
      <RecipientInput
        id="compose-to"
        value={localComposeTo}
        onChange={setLocalComposeTo}
        placeholder={t("to_placeholder")}
        accountId={activeAccount !== "unified" ? activeAccount : undefined}
      />
      <RecipientInput
        id="compose-cc"
        value={localComposeCc}
        onChange={setLocalComposeCc}
        placeholder={t("cc_placeholder")}
        accountId={activeAccount !== "unified" ? activeAccount : undefined}
      />
      <Input
        placeholder={t("subject")}
        value={localComposeSubject}
        onChange={(e) => setLocalComposeSubject(e.target.value)}
      />
      <select
        className="h-9 rounded-md border bg-card-bg px-2 py-1 text-sm text-foreground shadow-sm w-full mb-3"
        value={localFromIdentity}
        onChange={(e) => setLocalFromIdentity(e.target.value)}
      >
        {activeAccount !== "unified" ? (
          <>
            <option value="">
              {t("actions.from")} —{" "}
              {(() => {
                const acc = accounts.find((a) => a.id === activeAccount);
                return acc?.name
                  ? `${acc.name} <${acc.email}>`
                  : acc?.email || "";
              })()}
            </option>
            {identities
              ?.filter((i) => i.account_id === activeAccount)
              .map((ident) => (
                <option key={ident.id} value={ident.email}>
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
            {accounts.map((a) => (
              <React.Fragment key={a.id}>
                <option value={`account:${a.id}`}>
                  {a.name ? `${a.name} <${a.email}>` : a.email}
                </option>
                {identities
                  ?.filter((i) => i.account_id === a.id)
                  .map((ident) => (
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
      <Composer
        value={localComposeBody}
        onChange={setLocalComposeBody}
        placeholder={
          replyTo && !isForwarding ? t("write_reply") : t("write_message")
        }
      />
      {(isForwarding || isReplying) && forwardOriginalHtml && (
        <div className="mt-2 rounded-lg border border-border-muted overflow-hidden">
          <div className="bg-muted/50 px-3 py-2 text-xs text-text-muted border-b border-border-muted">
            <span className="italic">
              {isForwarding ? t("forward_begin") : t("reply_begin")}
            </span>
          </div>
          {forwardMeta && (
            <div className="px-3 py-2 text-xs text-text-muted space-y-0.5 border-b border-border-muted bg-muted/20">
              <div>
                <span className="font-semibold">{t("forward_from")}</span>{" "}
                {forwardMeta.from}
              </div>
              <div>
                <span className="font-semibold">{t("forward_subject")}</span>{" "}
                {forwardMeta.subject}
              </div>
              <div>
                <span className="font-semibold">{t("forward_date")}</span>{" "}
                {forwardMeta.date}
              </div>
              {forwardMeta.to && (
                <div>
                  <span className="font-semibold">{t("forward_to")}</span>{" "}
                  {forwardMeta.to}
                </div>
              )}
            </div>
          )}
          <iframe
            srcDoc={forwardOriginalHtml}
            sandbox="allow-same-origin"
            className="w-full border-0 bg-white"
            style={{ minHeight: 200 }}
            onLoad={(e) => {
              const iframe = e.currentTarget;
              const updateHeight = () => {
                try {
                  const doc = iframe.contentDocument;
                  if (doc && doc.documentElement) {
                    iframe.style.height = "0px";
                    const newHeight = Math.max(
                      doc.body.scrollHeight,
                      doc.documentElement.scrollHeight,
                      doc.body.offsetHeight,
                      doc.documentElement.offsetHeight,
                    );
                    iframe.style.height = newHeight + 24 + "px";
                  }
                } catch {}
              };

              try {
                const doc = iframe.contentDocument;
                if (doc) {
                  doc.documentElement.style.overflow = "hidden";
                  doc.body.style.overflow = "hidden";

                  if (forwardObserverRef.current) {
                    forwardObserverRef.current.disconnect();
                  }

                  const resizeObserver = new ResizeObserver(updateHeight);
                  resizeObserver.observe(doc.body);
                  resizeObserver.observe(doc.documentElement);
                  forwardObserverRef.current = resizeObserver;

                  const imgs = doc.querySelectorAll("img");
                  imgs.forEach((img) => {
                    if (!img.complete) {
                      img.addEventListener("load", updateHeight);
                    }
                  });

                  updateHeight();
                }
              } catch {}
            }}
          />
        </div>
      )}
      {composeAttachments.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {composeAttachments.map((att, i) => (
            <div
              key={att.id}
              className="flex items-center gap-1.5 bg-muted px-2 py-1 rounded text-xs"
            >
              <span className="text-text-muted">📎</span>
              <span className="text-text-main/80">{att.filename}</span>
              <button
                className="text-red-400 hover:text-red-300"
                onClick={() => onRemoveAttachment(i)}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="flex justify-between items-center">
        <Button
          variant="ghost"
          size="sm"
          onClick={onFileInputClick}
          title={t("attach")}
        >
          📎 {t("attach")}
        </Button>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={onCancelReply}>
            {t("actions.cancel")}
          </Button>
          {onSaveDraft && (
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                onSaveDraft(true, {
                  to: localComposeTo,
                  cc: localComposeCc,
                  subject: localComposeSubject,
                  body: localComposeBody,
                  identity: localFromIdentity,
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
            onClick={() => {
              onSendEmail({
                to: localComposeTo,
                cc: localComposeCc,
                subject: localComposeSubject,
                body: localComposeBody,
                html: localComposeBody,
                identity: localFromIdentity,
              });
            }}
            disabled={sendPending || !localComposeBody.trim()}
          >
            {sendPending ? t("actions.sending") : t("actions.send")}
          </Button>
        </div>
      </div>
    </div>
  );
});
