"use client";

import { useState, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import type { Email, Account, Identity } from "@/hooks/useEmailTypes";
import { getSavedUndoDelay } from "@/lib/date-format";
import {
  appendQuotedForwardHtml,
  buildForwardMeta,
  buildTextCitation,
  formatRecipientAddress,
  parseCcAddresses,
  resolveComposeAccountId,
  resolveFromIdentityHeader,
  stripHtml,
} from "@/lib/compose-utils";

export interface ComposeAttachment {
  id: string;
  filename: string;
  size: number;
}

export interface ForwardMeta {
  from: string;
  subject: string;
  date: string;
  to: string;
}

interface EmailBodyData {
  html?: string;
  body?: string;
  attachments?: Array<{ hash: string; filename: string; size: number }>;
}

export interface UseMailComposeOptions {
  locale: string;
  activeAccount: string;
  accounts: Account[];
  identities?: Identity[];
  selectedEmail?: Email | null;
  saveDraftMutation: {
    mutate: (
      payload: {
        id?: string;
        account_id: string;
        to: string;
        cc: string;
        bcc: string;
        subject: string;
        body: string;
        html: string;
        in_reply_to?: string;
        sync_remote: boolean;
      },
      options?: { onSuccess?: (data: { id: string }) => void },
    ) => void;
  };
  sendEmailMutation: {
    mutateAsync: (
      payload: Record<string, unknown>,
    ) => Promise<{ job_id?: string }>;
  };
  toast: {
    addToast: (
      message: React.ReactNode,
      type?: "success" | "error" | "info",
    ) => void;
  };
  onUndoJobChange: (jobId: string | null) => void;
}

export function useMailCompose({
  locale,
  activeAccount,
  accounts,
  identities,
  selectedEmail,
  saveDraftMutation,
  sendEmailMutation,
  toast,
  onUndoJobChange,
}: UseMailComposeOptions) {
  const t = useTranslations("mail");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [composeTo, setComposeTo] = useState<string[]>([]);
  const [composeCc, setComposeCc] = useState<string[]>([]);
  const [composeSubject, setComposeSubject] = useState("");
  const [composeBody, setComposeBody] = useState("");
  const [replyTo, setReplyTo] = useState<Email | null>(null);
  const [isReplying, setIsReplying] = useState(false);
  const [isForwarding, setIsForwarding] = useState(false);
  const [draftId, setDraftId] = useState<string | null>(null);
  const [fromIdentity, setFromIdentity] = useState("");
  const [attachments, setAttachments] = useState<ComposeAttachment[]>([]);
  const [forwardOriginalHtml, setForwardOriginalHtml] = useState("");
  const [forwardMeta, setForwardMeta] = useState<ForwardMeta | null>(null);

  const getSignatureHtml = useCallback(() => {
    const accountId =
      activeAccount !== "unified" ? activeAccount : accounts[0]?.id;
    const selectedAccount = accounts.find((a) => a.id === accountId);
    const signature = selectedAccount?.signature || "";
    if (!signature.trim()) return "";
    return `<br/><br/>--<br/>${signature.replace(/\n/g, "<br/>")}`;
  }, [activeAccount, accounts]);

  const resetCompose = useCallback(() => {
    setIsReplying(false);
    setIsForwarding(false);
    setForwardOriginalHtml("");
    setForwardMeta(null);
    setAttachments([]);
    setReplyTo(null);
    setDraftId(null);
  }, []);

  const beginReplyTo = useCallback(
    (email: Email, subjectPrefix: "Re:" | "Fwd:" = "Re:") => {
      setReplyTo(email);
      setComposeTo(formatRecipientAddress(email));
      setComposeSubject(`${subjectPrefix} ${email.subject}`);
      setComposeBody(getSignatureHtml());
      setFromIdentity(email.account_id ? `account:${email.account_id}` : "");
      setIsReplying(subjectPrefix === "Re:");
      setIsForwarding(subjectPrefix === "Fwd:");
    },
    [getSignatureHtml],
  );

  const handleReply = useCallback(() => {
    if (!selectedEmail) return;
    setComposeCc([]);
    beginReplyTo(selectedEmail, "Re:");
  }, [selectedEmail, beginReplyTo]);

  const handleForward = useCallback(
    (emailBody?: EmailBodyData) => {
      if (!selectedEmail) return;
      setReplyTo(selectedEmail);
      setComposeTo([]);
      setComposeSubject(`Fwd: ${selectedEmail.subject}`);
      setComposeCc([]);

      const originalHtml = emailBody?.html || "";
      const originalText = emailBody?.body || selectedEmail.snippet || "";
      setForwardOriginalHtml(
        originalHtml || originalText.replace(/\n/g, "<br>"),
      );
      setForwardMeta(buildForwardMeta(selectedEmail, locale));
      setComposeBody(getSignatureHtml());
      setFromIdentity(
        selectedEmail.account_id ? `account:${selectedEmail.account_id}` : "",
      );
      setAttachments(
        (emailBody?.attachments ?? []).map((a) => ({
          id: a.hash,
          filename: a.filename,
          size: a.size,
        })),
      );
      setIsForwarding(true);
      setIsReplying(false);
    },
    [selectedEmail, getSignatureHtml, locale],
  );

  const handleReplyAll = useCallback(() => {
    if (!selectedEmail) return;
    setReplyTo(selectedEmail);
    setComposeTo(formatRecipientAddress(selectedEmail));
    setComposeCc(parseCcAddresses(selectedEmail.cc_address));
    setComposeSubject(`Re: ${selectedEmail.subject}`);
    setComposeBody(getSignatureHtml());
    setFromIdentity(
      selectedEmail.account_id ? `account:${selectedEmail.account_id}` : "",
    );
    setIsReplying(true);
    setIsForwarding(false);
  }, [selectedEmail, getSignatureHtml]);

  const handleReplyToEmail = useCallback(
    (email: Email) => {
      setComposeCc([]);
      beginReplyTo(email, "Re:");
    },
    [beginReplyTo],
  );

  const handleReplyWithQuote = useCallback(
    (selectedText?: string, emailBody?: EmailBodyData) => {
      if (!selectedEmail) return;
      setReplyTo(selectedEmail);
      setComposeTo(formatRecipientAddress(selectedEmail));
      setComposeSubject(`Re: ${selectedEmail.subject}`);

      let quotedText = (selectedText || "").trim();
      if (!quotedText) {
        try {
          const sel = window.getSelection();
          if (sel && sel.toString().trim()) {
            quotedText = sel.toString().trim();
          }
        } catch {}
      }

      if (quotedText) {
        const htmlCitation = buildTextCitation(quotedText);
        setComposeBody(`${getSignatureHtml()}<br><br>${htmlCitation}<br><br>`);
      } else {
        setComposeBody(getSignatureHtml());
        const originalHtml = emailBody?.html || "";
        const originalText = emailBody?.body || selectedEmail.snippet || "";
        setForwardOriginalHtml(
          originalHtml || originalText.replace(/\n/g, "<br>"),
        );
        setForwardMeta(buildForwardMeta(selectedEmail, locale));
      }

      setFromIdentity(
        selectedEmail.account_id ? `account:${selectedEmail.account_id}` : "",
      );
      setIsReplying(true);
      setIsForwarding(false);
    },
    [selectedEmail, getSignatureHtml, locale],
  );

  const handleSaveDraft = useCallback(
    (
      syncRemote: boolean,
      data?: {
        to: string[];
        cc: string[];
        subject: string;
        body: string;
        identity: string;
      },
    ) => {
      const currentIdentity = data ? data.identity : fromIdentity;
      const finalAccountId = resolveComposeAccountId({
        activeAccount,
        accounts,
        identity: currentIdentity,
        identities,
        isReplying,
        isForwarding,
        selectedEmail,
      });
      if (!finalAccountId) return;

      const currentTo = data ? data.to : composeTo;
      const currentCc = data ? data.cc : composeCc;
      const currentSubject = data ? data.subject : composeSubject;
      const currentBody = data ? data.body : composeBody;

      saveDraftMutation.mutate(
        {
          id: draftId ?? undefined,
          account_id: finalAccountId,
          to: currentTo.join(", "),
          cc: currentCc.join(", "),
          bcc: "",
          subject: currentSubject,
          body: stripHtml(currentBody),
          html: currentBody,
          in_reply_to: isReplying && replyTo ? replyTo.msg_id : undefined,
          sync_remote: syncRemote,
        },
        {
          onSuccess: (saved) => {
            setDraftId(saved.id);
            if (syncRemote) {
              toast.addToast("Draft saved", "success");
            }
          },
        },
      );
    },
    [
      activeAccount,
      accounts,
      fromIdentity,
      identities,
      isReplying,
      isForwarding,
      selectedEmail,
      draftId,
      composeTo,
      composeCc,
      composeSubject,
      composeBody,
      saveDraftMutation,
      toast,
      replyTo,
    ],
  );

  const handleSendEmail = useCallback(
    async (options: {
      to: string[];
      cc: string[];
      subject: string;
      body: string;
      html: string;
      identity: string;
    }) => {
      const finalTo = [...options.to];
      const toInput = document.getElementById(
        "compose-to-input",
      ) as HTMLInputElement | null;
      if (toInput && toInput.value.trim()) {
        const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        if (emailPattern.test(toInput.value.trim())) {
          finalTo.push(toInput.value.trim());
        }
      }

      if (finalTo.length === 0 || !options.subject) {
        toast.addToast(t("toast_to_subject_required"), "error");
        return;
      }

      const identityResolved = resolveFromIdentityHeader(
        options.identity,
        accounts,
        identities,
      );
      let finalAccountId = identityResolved.accountId;
      let finalFromIdentity = identityResolved.fromHeader;

      if (!finalAccountId) {
        finalAccountId = resolveComposeAccountId({
          activeAccount,
          accounts,
          identity: options.identity,
          identities,
          isReplying,
          isForwarding,
          selectedEmail,
        });
        if (finalAccountId && !finalFromIdentity) {
          const acc = accounts.find((a) => a.id === finalAccountId);
          if (acc) {
            finalFromIdentity = acc.name
              ? `${acc.name} <${acc.email}>`
              : acc.email;
          }
        }
      }

      if (!finalAccountId) {
        toast.addToast(t("toast_no_account"), "error");
        return;
      }

      try {
        let finalHtml = options.html;
        if (
          (isForwarding || isReplying) &&
          forwardOriginalHtml &&
          forwardMeta
        ) {
          finalHtml = appendQuotedForwardHtml(
            options.html,
            forwardOriginalHtml,
            forwardMeta,
            {
              begin: isForwarding
                ? t("forward_begin_colon")
                : t("reply_begin_colon"),
              from: t("forward_from"),
              subject: t("forward_subject"),
              date: t("forward_date"),
              to: t("forward_to"),
            },
          );
        }

        const result = await sendEmailMutation.mutateAsync({
          account_id: finalAccountId,
          to: finalTo,
          cc: options.cc,
          subject: options.subject,
          body: stripHtml(finalHtml),
          html: finalHtml,
          from_identity: finalFromIdentity,
          in_reply_to: isReplying && replyTo ? replyTo.msg_id : undefined,
          references:
            isReplying && replyTo
              ? replyTo.in_reply_to
                ? `${replyTo.in_reply_to} ${replyTo.msg_id}`
                : replyTo.msg_id
              : undefined,
          attachment_hashes: attachments.map((a) => a.id),
          draft_id: draftId ?? undefined,
        });

        resetCompose();
        if (result.job_id) {
          const jobId = result.job_id;
          onUndoJobChange(jobId);
          const undoMs = getSavedUndoDelay();
          if (undoMs > 0) {
            setTimeout(() => onUndoJobChange(null), undoMs);
          }
        } else {
          toast.addToast(t("toast_email_sent"), "success");
        }
      } catch {
        toast.addToast(t("toast_email_send_failed"), "error");
      }
    },
    [
      accounts,
      identities,
      sendEmailMutation,
      toast,
      isReplying,
      replyTo,
      isForwarding,
      forwardOriginalHtml,
      forwardMeta,
      attachments,
      t,
      activeAccount,
      selectedEmail,
      draftId,
      resetCompose,
      onUndoJobChange,
    ],
  );

  return {
    fileInputRef,
    composeTo,
    setComposeTo,
    composeCc,
    setComposeCc,
    composeSubject,
    setComposeSubject,
    composeBody,
    setComposeBody,
    replyTo,
    setReplyTo,
    isReplying,
    setIsReplying,
    isForwarding,
    setIsForwarding,
    draftId,
    setDraftId,
    fromIdentity,
    setFromIdentity,
    attachments,
    setAttachments,
    forwardOriginalHtml,
    setForwardOriginalHtml,
    forwardMeta,
    setForwardMeta,
    getSignatureHtml,
    resetCompose,
    handleReply,
    handleForward,
    handleReplyAll,
    handleReplyToEmail,
    handleReplyWithQuote,
    handleSaveDraft,
    handleSendEmail,
  };
}
