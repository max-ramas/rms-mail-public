import type { Account, Email, Identity } from "@/hooks/useEmailTypes";
import { formatEmailDatetime } from "@/lib/date-format";

export function stripHtml(html: string): string {
  let text = html;
  text = text.replace(/<br\s*\/?>/gi, "\n");
  text = text.replace(/<\/p>/gi, "\n");
  text = text.replace(/<[^>]*>/g, "");
  return text;
}

export function formatRecipientAddress(
  email: Pick<Email, "sender_name" | "sender_address">,
): string[] {
  const addr = email.sender_name
    ? `${email.sender_name} <${email.sender_address}>`
    : email.sender_address || "";
  return addr ? [addr] : [];
}

export function parseCcAddresses(ccAddress?: string): string[] {
  if (!ccAddress) return [];
  return ccAddress
    .split(",")
    .map((e) => e.trim())
    .filter(Boolean);
}

export function buildForwardMeta(
  email: Email,
  locale: string,
): {
  from: string;
  subject: string;
  date: string;
  to: string;
} {
  return {
    from: email.sender_name
      ? `${email.sender_name} <${email.sender_address}>`
      : email.sender_address || "",
    subject: email.subject || "",
    date: formatEmailDatetime(email.date_sent, locale),
    to: email.recipient_address || "",
  };
}

export function buildTextCitation(quotedText: string): string {
  return quotedText
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .map((l) => `> ${l}`)
    .join("<br>");
}

export function resolveComposeAccountId(options: {
  activeAccount: string;
  accounts: Account[];
  identity: string;
  identities?: Identity[];
  isReplying: boolean;
  isForwarding: boolean;
  selectedEmail?: Email | null;
}): string | undefined {
  const {
    activeAccount,
    accounts,
    identity,
    identities,
    isReplying,
    isForwarding,
    selectedEmail,
  } = options;

  const finalAccountId =
    activeAccount !== "unified" ? activeAccount : accounts[0]?.id;

  if (identity.startsWith("account:")) {
    return identity.replace("account:", "");
  }
  if (identity.startsWith("identity:")) {
    const idId = identity.replace("identity:", "");
    const ident = identities?.find((i) => i.id === idId);
    if (ident) return ident.account_id;
  }
  if (
    activeAccount === "unified" &&
    (isReplying || isForwarding) &&
    selectedEmail
  ) {
    return selectedEmail.account_id;
  }
  return finalAccountId;
}

export function resolveFromIdentityHeader(
  identity: string,
  accounts: Account[],
  identities?: Identity[],
): { accountId?: string; fromHeader?: string } {
  if (identity.startsWith("account:")) {
    const accountId = identity.replace("account:", "");
    const acc = accounts.find((a) => a.id === accountId);
    return {
      accountId,
      fromHeader: acc
        ? acc.name
          ? `${acc.name} <${acc.email}>`
          : acc.email
        : undefined,
    };
  }
  if (identity.startsWith("identity:")) {
    const idId = identity.replace("identity:", "");
    const ident = identities?.find((i) => i.id === idId);
    if (!ident) return {};
    return {
      accountId: ident.account_id,
      fromHeader: ident.name
        ? `${ident.name} <${ident.email}>`
        : ident.email,
    };
  }
  return {};
}

export function appendQuotedForwardHtml(
  html: string,
  forwardOriginalHtml: string,
  forwardMeta: {
    from: string;
    subject: string;
    date: string;
    to: string;
  },
  labels: {
    begin: string;
    from: string;
    subject: string;
    date: string;
    to: string;
  },
): string {
  let cleanedOriginalHtml = forwardOriginalHtml;
  const bodyMatch = forwardOriginalHtml.match(
    /<body[^>]*>([\s\S]*?)<\/body>/i,
  );
  if (bodyMatch) {
    cleanedOriginalHtml = bodyMatch[1];
  }

  const quotedBlock = `
<br><br>
<div style="border-left:3px solid #aaaaaa;padding-left:12px;color:#555555;margin-top:16px;">
  <p style="font-style:italic;margin:0 0 8px 0;color:#888">${labels.begin}</p>
  <p style="margin:2px 0"><b>${labels.from}</b> ${forwardMeta.from}</p>
  <p style="margin:2px 0"><b>${labels.subject}</b> ${forwardMeta.subject}</p>
  <p style="margin:2px 0"><b>${labels.date}</b> ${forwardMeta.date}</p>
  <p style="margin:2px 0"><b>${labels.to}</b> ${forwardMeta.to}</p>
  <br>
  ${cleanedOriginalHtml}
</div>`;
  return html + quotedBlock;
}
