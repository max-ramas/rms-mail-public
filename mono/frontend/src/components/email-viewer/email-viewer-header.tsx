"use client";

import { useState } from "react";
import {
  Copy,
  Check,
  Pin,
  Star,
  Reply,
  ChevronDown,
  ShieldCheck,
  ShieldOff,
  UserPlus,
  UserCheck,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { formatEmailDatetime } from "@/lib/date-format";
import { formatAddresses } from "@/lib/email-address-utils";
import { Avatar } from "@/components/avatar";
import { Button } from "@/components/ui/button";
import { useContacts } from "@/hooks/useEmailQueries";
import { useCreateContact } from "@/hooks/useAdminQueries";
import { useToast } from "@/hooks/useToast";
import type { Email, Label } from "@/hooks/useEmails";
import { EmailLabelsDropdown } from "./email-labels-dropdown";

interface EmailViewerHeaderProps {
  selectedEmail: Email;
  locale: string;
  labelsData: Label[] | undefined;
  selectedLabelIds: Set<string>;
  onToggleLabel: (labelId: string) => void;
  onTogglePin: () => void;
  onToggleFlag: () => void;
}

export function EmailViewerHeader({
  selectedEmail,
  locale,
  labelsData,
  selectedLabelIds,
  onToggleLabel,
  onTogglePin,
  onToggleFlag,
}: EmailViewerHeaderProps) {
  const t = useTranslations("mail");
  const toast = useToast();
  const contactsQuery = useContacts();
  const createContact = useCreateContact();

  const [detailsOpen, setDetailsOpen] = useState(false);
  const [copiedSubject, setCopiedSubject] = useState(false);
  const [copiedAddress, setCopiedAddress] = useState(false);

  const isContactExist =
    !!selectedEmail?.sender_address &&
    !!contactsQuery.data?.some(
      (c) =>
        c.address.toLowerCase() ===
        selectedEmail.sender_address.toLowerCase(),
    );

  return (
    <div className="flex-none px-6 py-2 md:py-3 border-b border-border-muted/50 bg-card-bg/40 backdrop-blur-md space-y-3">
      <div className="flex items-center justify-between gap-2 md:gap-3 border-b border-border-muted/20 pb-2">
        <div className="flex items-center gap-2 group min-w-0 flex-1">
          <h1
            className="text-xl md:text-2xl font-bold tracking-tight text-text-main truncate"
            title={selectedEmail.subject}
          >
            {selectedEmail.subject}
          </h1>
          <button
            onClick={() => {
              navigator.clipboard.writeText(selectedEmail.subject);
              setCopiedSubject(true);
              toast.addToast(t("subject_copied"), "success");
              setTimeout(() => setCopiedSubject(false), 2000);
            }}
            className="p-1.5 rounded-lg hover:bg-muted/40 text-text-muted hover:text-text-main opacity-0 group-hover:opacity-100 transition-all focus:opacity-100 shrink-0"
            title={t("copy_subject")}
          >
            {copiedSubject ? (
              <Check className="w-4 h-4 text-emerald-400" />
            ) : (
              <Copy className="w-4 h-4" />
            )}
          </button>
        </div>

        <div className="flex items-center gap-1.5 shrink-0">
          <EmailLabelsDropdown
            labelsData={labelsData}
            selectedLabelIds={selectedLabelIds}
            onToggleLabel={onToggleLabel}
          />
          <button
            onClick={onTogglePin}
            className={`p-1.5 rounded-lg border transition-all ${
              selectedEmail.is_pinned
                ? "bg-amber-500/15 border-amber-500/35 text-amber-400 hover:bg-amber-500/25 shadow-[0_0_8px_rgba(245,158,11,0.1)]"
                : "bg-muted/20 border-border-muted/40 text-text-muted hover:text-text-main hover:bg-muted/30"
            }`}
            title={
              selectedEmail.is_pinned ? t("actions.unpin") : t("actions.pin")
            }
          >
            <Pin
              className={`w-4 h-4 ${selectedEmail.is_pinned ? "fill-amber-400" : ""}`}
            />
          </button>
          <button
            onClick={onToggleFlag}
            className={`p-1.5 rounded-lg border transition-all ${
              selectedEmail.is_flagged
                ? "bg-yellow-500/15 border-yellow-500/35 text-yellow-400 hover:bg-yellow-500/25 shadow-[0_0_8px_rgba(234,179,8,0.1)]"
                : "bg-muted/20 border-border-muted/40 text-text-muted hover:text-text-main hover:bg-muted/30"
            }`}
            title={
              selectedEmail.is_flagged
                ? t("actions.unflag")
                : t("actions.flag")
            }
          >
            <Star
              className={`w-4 h-4 ${selectedEmail.is_flagged ? "fill-yellow-400 text-yellow-400" : ""}`}
            />
          </button>
          {selectedEmail.is_answered && (
            <span
              className="p-1.5 rounded-lg border bg-sky-500/15 border-sky-500/35 text-sky-400"
              title={t("replied")}
            >
              <Reply className="w-4 h-4" />
            </span>
          )}
        </div>
      </div>

      <div className="flex flex-col md:flex-row md:items-center justify-between gap-2 md:gap-3">
        <div className="flex items-center gap-3">
          <Avatar
            src={selectedEmail.avatar_url || null}
            name={selectedEmail.sender_name}
            email={selectedEmail.sender_address}
            size={42}
          />
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h2 className="text-sm font-semibold text-text-main truncate">
                {selectedEmail.sender_name || t("unknown_sender")}
              </h2>
              <span className="text-xs text-text-muted truncate">
                &lt;{selectedEmail.sender_address}&gt;
              </span>
            </div>
            <div className="flex items-center gap-2 mt-0.5 text-xs text-text-muted">
              <span>
                {t("to_label")}{" "}
                {formatAddresses(selectedEmail.recipient_address)}
              </span>
              {selectedEmail.cc_address && (
                <>
                  <span>•</span>
                  <span>
                    {t("cc_label")}:{" "}
                    {formatAddresses(selectedEmail.cc_address)}
                  </span>
                </>
              )}
              <span>•</span>
              <span>
                {formatEmailDatetime(selectedEmail.date_sent, locale)}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2 self-end md:self-auto">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDetailsOpen(!detailsOpen)}
            className="h-8 text-xs font-semibold text-text-muted hover:text-text-main flex items-center gap-1.5 bg-muted/10 border border-border-muted/30 hover:bg-muted/20 px-3 rounded-lg transition-all"
          >
            <span>{t("details")}</span>
            <ChevronDown
              className={`w-3.5 h-3.5 transition-transform duration-200 ${
                detailsOpen ? "rotate-180" : ""
              }`}
            />
          </Button>
        </div>
      </div>

      {detailsOpen && (
        <div className="p-4 rounded-xl bg-zinc-900/30 border border-border-muted/40 backdrop-blur-md space-y-3.5 animate-in fade-in slide-in-from-top-2 duration-200">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
            <div className="space-y-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <span className="text-text-muted w-14 shrink-0 font-medium">
                  {t("from_label")}
                </span>
                <div className="flex items-center gap-1.5 min-w-0">
                  <span className="font-semibold text-text-main truncate">
                    {selectedEmail.sender_name || t("unknown_sender")}
                  </span>
                  <span className="text-text-muted truncate">
                    ({selectedEmail.sender_address})
                  </span>
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(
                        selectedEmail.sender_address,
                      );
                      setCopiedAddress(true);
                      toast.addToast(t("toast_address_copied"), "success");
                      setTimeout(() => setCopiedAddress(false), 2000);
                    }}
                    className="p-1 rounded-md hover:bg-muted/40 text-text-muted hover:text-text-main transition-colors shrink-0"
                    title={t("copy_address")}
                  >
                    {copiedAddress ? (
                      <Check className="w-3.5 h-3.5 text-emerald-400" />
                    ) : (
                      <Copy className="w-3.5 h-3.5" />
                    )}
                  </button>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <span className="text-text-muted w-14 shrink-0 font-medium">
                  {t("to_label")}
                </span>
                <span className="text-text-main truncate font-medium">
                  {selectedEmail.recipient_address}
                </span>
              </div>

              {selectedEmail.created_at && (
                <div className="flex items-center gap-2">
                  <span className="text-text-muted w-14 shrink-0 font-medium">
                    {t("created_label")}
                  </span>
                  <span className="text-text-main font-medium">
                    {formatEmailDatetime(selectedEmail.created_at, locale)}
                  </span>
                </div>
              )}
            </div>

            <div className="space-y-2.5">
              <div className="flex items-center gap-4">
                <span className="text-text-muted w-24 shrink-0 font-medium">
                  {t("security_label")}
                </span>
                <div className="flex items-center gap-2.5">
                  <div
                    className="flex items-center gap-1 cursor-help"
                    title={
                      selectedEmail.spf_pass
                        ? t("spf_pass_tooltip")
                        : t("spf_na_tooltip")
                    }
                  >
                    {selectedEmail.spf_pass ? (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold tracking-wider text-[10px]">
                        <ShieldCheck className="w-3.5 h-3.5" /> {t("spf_pass")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold tracking-wider text-[10px]">
                        <ShieldOff className="w-3.5 h-3.5" /> {t("spf_na")}
                      </span>
                    )}
                  </div>

                  <div
                    className="flex items-center gap-1 cursor-help"
                    title={
                      selectedEmail.dkim_pass
                        ? t("dkim_pass_tooltip")
                        : t("dkim_na_tooltip")
                    }
                  >
                    {selectedEmail.dkim_pass ? (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold tracking-wider text-[10px]">
                        <ShieldCheck className="w-3.5 h-3.5" /> {t("dkim_pass")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold tracking-wider text-[10px]">
                        <ShieldOff className="w-3.5 h-3.5" /> {t("dkim_na")}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-4">
                <span className="text-text-muted w-24 shrink-0 font-medium">
                  {t("crm_contact_label")}
                </span>
                {isContactExist ? (
                  <span className="inline-flex items-center gap-1.5 text-emerald-400 font-semibold">
                    <UserCheck className="w-3.5 h-3.5" /> {t("in_contacts_list")}
                  </span>
                ) : (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      createContact.mutate(
                        {
                          name:
                            selectedEmail.sender_name ||
                            selectedEmail.sender_address.split("@")[0],
                          address: selectedEmail.sender_address,
                        },
                        {
                          onSuccess: () => {
                            toast.addToast(t("toast_contact_added"), "success");
                          },
                          onError: () => {
                            toast.addToast(
                              t("toast_contact_add_error"),
                              "error",
                            );
                          },
                        },
                      );
                    }}
                    disabled={createContact.isPending}
                    className="h-6 text-xs bg-primary/15 hover:bg-primary/25 text-primary border border-primary/25 px-2.5 rounded-lg flex items-center gap-1 font-semibold transition-all"
                  >
                    <UserPlus className="w-3 h-3" />
                    {createContact.isPending
                      ? t("adding_contact")
                      : t("add_to_contacts")}
                  </Button>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
