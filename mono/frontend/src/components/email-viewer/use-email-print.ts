"use client";

import { useCallback } from "react";
import { useCommandListener } from "@/lib/commandBus";
import { formatEmailDatetime } from "@/lib/date-format";
import { formatAddresses } from "@/lib/email-address-utils";
import type { Email } from "@/hooks/useEmails";

interface UseEmailPrintOptions {
  selectedEmail: Email | null;
  useThreads: boolean;
  threadEmails?: Email[];
  locale: string;
  emailHtmlRef: React.MutableRefObject<string | undefined>;
  emailBodyRef: React.MutableRefObject<string | undefined>;
}

export function useEmailPrint({
  selectedEmail,
  useThreads,
  threadEmails,
  locale,
  emailHtmlRef,
  emailBodyRef,
}: UseEmailPrintOptions) {
  const handlePrint = useCallback(() => {
    if (!selectedEmail) return;

    const printWindow = document.createElement("iframe");
    printWindow.style.position = "absolute";
    printWindow.style.top = "-9999px";
    printWindow.style.left = "-9999px";
    printWindow.style.width = "0";
    printWindow.style.height = "0";
    printWindow.style.border = "none";
    printWindow.setAttribute("sandbox", "allow-same-origin allow-modals");

    const escapeHtml = (unsafe: string) => {
      return (unsafe || "").replace(/[&<"'>]/g, (m) => {
        switch (m) {
          case "&":
            return "&amp;";
          case "<":
            return "&lt;";
          case ">":
            return "&gt;";
          case '"':
            return "&quot;";
          case "'":
            return "&#039;";
          default:
            return m;
        }
      });
    };

    const emailsToPrint =
      useThreads && threadEmails && threadEmails.length > 0
        ? [...threadEmails].sort(
            (a, b) =>
              new Date(a.date_sent).getTime() - new Date(b.date_sent).getTime(),
          )
        : [selectedEmail];

    const contentHtml = emailsToPrint
      .map(
        (email) => `
	      <div class="email-item">
	        <div class="header">
	          <div class="subject">${escapeHtml(email.subject || "No Subject")}</div>
	          <div class="meta">
	            <strong>From:</strong> ${escapeHtml(formatAddresses(email.sender_address))}<br/>
	            <strong>To:</strong> ${escapeHtml(formatAddresses(email.recipient_address))}<br/>
	            <strong>Date:</strong> ${escapeHtml(formatEmailDatetime(email.date_sent, locale))}
	          </div>
	        </div>
	        <div class="body">
	          ${email.id === selectedEmail?.id ? emailHtmlRef.current || emailBodyRef.current || email.snippet || "(no content)" : email.html || email.body || email.snippet || "(no content)"}
	        </div>
	      </div>
	    `,
      )
      .join("<hr/>");

    const fullHtml = `
      <!DOCTYPE html>
      <html>
        <head>
          <title>Print - ${escapeHtml(selectedEmail.subject)}</title>
          <style>
            @media print {
              * { background-color: transparent !important; color: black !important; }
              body { margin: 0; padding: 20px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
              .email-item { page-break-inside: auto; margin-bottom: 2rem; }
              .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; page-break-inside: avoid; }
              .subject { font-size: 1.5em; font-weight: bold; margin-bottom: 10px; }
              .meta { font-size: 0.9em; color: #555; line-height: 1.4; }
              .body { font-size: 1em; line-height: 1.5; color: black !important; }
              .body * { color: black !important; background-color: transparent !important; }
              .body img { max-width: 100%; height: auto; }
              hr { border: 0; border-top: 1px dashed #ccc; margin: 30px 0; }
            }
          </style>
        </head>
        <body>
          ${contentHtml}
        </body>
      </html>
    `;

    printWindow.srcdoc = fullHtml;
    document.body.appendChild(printWindow);

    setTimeout(() => {
      printWindow.contentWindow?.focus();
      printWindow.contentWindow?.print();
      setTimeout(() => {
        if (document.body.contains(printWindow)) {
          document.body.removeChild(printWindow);
        }
      }, 1000);
    }, 800);
  }, [
    selectedEmail,
    useThreads,
    threadEmails,
    locale,
    emailHtmlRef,
    emailBodyRef,
  ]);

  useCommandListener("mail:print", handlePrint);
}
