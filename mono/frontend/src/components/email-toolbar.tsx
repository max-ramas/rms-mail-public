"use client";

import React from "react";
import {
  SquarePen,
  Reply,
  ReplyAll,
  Forward,
  MessageSquareQuote,
  Sparkles,
  Tags,
  Globe,
  MessageCircle,
  Archive,
  Trash,
  Clock,
  Download,
  Pin,
  VolumeX,
  MoreHorizontal,
  MailOpen,
  ChevronLeft,
  Mail,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Email } from "@/hooks/useEmailTypes";

export interface EmailToolbarProps {
  aiEnabled?: boolean;
  selectedEmail: Email | null;
  isComposing: boolean;
  isReplying: boolean;
  isForwarding: boolean;
  onCompose: () => void;
  onReply: () => void;
  onReplyAll: () => void;
  onForward: () => void;
  onReplyWithQuote?: () => void;
  onSnooze: () => void;
  onSummarize: () => void;
  onCategorize: () => void;
  onChatToggle: () => void;
  onPin: () => void;
  onMute: () => void;
  onTranslate: () => void;
  onDownloadEml: () => void;
  onArchive: () => void;
  onDelete: () => void;
  onToggleRead?: () => void;
  onRestoreFromTrash?: () => void;
  isTrash?: boolean;
  summarizePending: boolean;
  categorizePending: boolean;
  translatePending: boolean;
  onBackClick?: () => void;
}

export function EmailToolbar({
  aiEnabled = true,
  selectedEmail,
  isReplying,
  isForwarding,
  onCompose,
  onReply,
  onReplyAll,
  onForward,
  onReplyWithQuote,
  onSnooze,
  onSummarize,
  onCategorize,
  onChatToggle,
  onPin,
  onMute,
  onTranslate,
  onDownloadEml,
  onArchive,
  onDelete,
  onToggleRead,
  onRestoreFromTrash,
  isTrash,
  summarizePending,
  categorizePending,
  translatePending,
  onBackClick,
}: EmailToolbarProps) {
  const t = useTranslations("mail");
  const tCommands = useTranslations("commands");

  return (
    <div className="relative z-20 flex-none flex w-full border-b border-border-muted/50 bg-card-bg/60 backdrop-blur-md shrink-0">
      <div className="flex-1 flex items-center justify-between gap-x-2 px-4 h-16 overflow-x-auto select-none no-scrollbar">
        {/* Left side: Compose and standard replies */}
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            onClick={onCompose}
            className="bg-primary hover:bg-primary/95 text-primary-foreground font-medium shadow-[0_2px_8px_rgba(251,191,36,0.15)] transition-all active:scale-[0.98]"
            title={t("actions.compose")}
          >
            <SquarePen className="w-4 h-4" />
            <span className="hidden sm:inline">{t("actions.compose")}</span>
          </Button>

          {selectedEmail && (
            <div className="flex items-center bg-muted/20 p-0.5 rounded-lg border border-border-muted/40">
              <Button
                size="sm"
                variant="ghost"
                onClick={onReply}
                className={`h-7 px-2.5 rounded-md transition-all ${
                  isReplying
                    ? "bg-primary/20 text-primary hover:bg-primary/30"
                    : "text-text-muted hover:text-text-main"
                }`}
                title={isReplying ? t("actions.cancel") : t("actions.reply")}
              >
                <Reply className="w-4 h-4 sm:me-1" />
                <span className="hidden md:inline">
                  {isReplying ? t("actions.cancel") : t("actions.reply")}
                </span>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={onReplyAll}
                className="h-7 px-2.5 rounded-md text-text-muted hover:text-text-main transition-all"
                title={t("actions.reply_all")}
              >
                <ReplyAll className="w-4 h-4 sm:me-1" />
                <span className="hidden md:inline">
                  {t("actions.reply_all")}
                </span>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={onReplyWithQuote}
                className={`h-7 px-2.5 rounded-md transition-all text-text-muted hover:text-text-main`}
                title={t("actions.reply_with_quote")}
              >
                <MessageSquareQuote className="w-4 h-4 sm:me-1" />
                <span className="hidden md:inline">
                  {t("actions.reply_with_quote")}
                </span>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={onForward}
                className={`h-7 px-2.5 rounded-md transition-all ${
                  isForwarding
                    ? "bg-primary/20 text-primary hover:bg-primary/30"
                    : "text-text-muted hover:text-text-main"
                }`}
                title={
                  isForwarding ? t("actions.cancel") : t("actions.forward")
                }
              >
                <Forward className="w-4 h-4 sm:me-1" />
                <span className="hidden md:inline">
                  {isForwarding ? t("actions.cancel") : t("actions.forward")}
                </span>
              </Button>
            </div>
          )}
        </div>

        {/* Right side: AI Toolkit, Sorting, and More Dropdown */}
        {selectedEmail && (
          <div className="flex items-center gap-2">
            {aiEnabled && (
              <div className="flex items-center gap-0.5 bg-transparent p-0.5 rounded-lg border border-amber-400/60 dark:border-amber-500/30 hover:border-amber-500/80 dark:hover:border-amber-500/50 shadow-[0_0_12px_rgba(245,158,11,0.05)] transition-all duration-300">
                {/* Summarize */}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onSummarize}
                  disabled={summarizePending}
                  className="h-7 px-2.5 text-amber-600 dark:text-amber-400 hover:text-amber-700 dark:hover:text-amber-300 hover:bg-amber-500/10 rounded-md transition-all disabled:opacity-50"
                  title={t("actions.summarize")}
                >
                  <Sparkles
                    className={`w-3.5 h-3.5 sm:me-1 ${summarizePending ? "animate-pulse text-yellow-300" : ""}`}
                  />
                  <span className="hidden lg:inline text-xs font-semibold">
                    {summarizePending
                      ? t("actions.thinking")
                      : t("actions.summarize")}
                  </span>
                </Button>

                {/* Categorize */}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onCategorize}
                  disabled={categorizePending}
                  className="h-7 px-2.5 text-amber-600 dark:text-amber-400 hover:text-amber-700 dark:hover:text-amber-300 hover:bg-amber-500/10 rounded-md transition-all disabled:opacity-50"
                  title={t("actions.categorize")}
                >
                  <Tags className="w-3.5 h-3.5 sm:me-1" />
                  <span className="hidden lg:inline text-xs font-semibold">
                    {categorizePending
                      ? t("actions.categorizing")
                      : t("actions.categorize")}
                  </span>
                </Button>

                {/* Translate */}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onTranslate}
                  disabled={translatePending}
                  className="h-7 px-2.5 text-amber-600 dark:text-amber-400 hover:text-amber-700 dark:hover:text-amber-300 hover:bg-amber-500/10 rounded-md transition-all disabled:opacity-50"
                  title={t("actions.translate")}
                >
                  <Globe className="w-3.5 h-3.5 sm:me-1" />
                  <span className="hidden lg:inline text-xs font-semibold">
                    {translatePending
                      ? t("actions.translating")
                      : t("actions.translate")}
                  </span>
                </Button>

                {/* AI Chat */}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onChatToggle}
                  className="h-7 px-2.5 text-amber-600 dark:text-amber-400 hover:text-amber-700 dark:hover:text-amber-300 hover:bg-amber-500/10 rounded-md transition-all"
                  title={t("actions.chat")}
                >
                  <MessageCircle className="w-3.5 h-3.5 sm:me-1" />
                  <span className="hidden lg:inline text-xs font-semibold">
                    {t("actions.chat")}
                  </span>
                </Button>
              </div>
            )}

            {/* Quick sorting: Archive & Delete */}
            <div className="flex items-center bg-muted/10 p-0.5 rounded-lg border border-border-muted/20">
              {onToggleRead && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onToggleRead}
                  className="h-7 w-7 p-0 text-text-muted hover:text-text-main transition-colors"
                  title={
                    selectedEmail.is_read
                      ? tCommands("mail_mark_unread")
                      : tCommands("mail_mark_read")
                  }
                >
                  {selectedEmail.is_read ? (
                    <Mail className="w-4 h-4" />
                  ) : (
                    <MailOpen className="w-4 h-4" />
                  )}
                </Button>
              )}
              <Button
                size="sm"
                variant="ghost"
                onClick={onArchive}
                className="h-7 w-7 p-0 text-text-muted hover:text-text-main transition-colors"
                title={t("actions.archive")}
              >
                <Archive className="w-4 h-4" />
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={onDelete}
                className="h-7 w-7 p-0 text-text-muted hover:text-red-400 hover:bg-red-500/10 transition-colors"
                title={t("actions.delete")}
                type="button"
              >
                <Trash className="w-4 h-4" />
              </Button>
              {isTrash && onRestoreFromTrash && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={onRestoreFromTrash}
                  className="h-7 px-2 text-xs bg-green-600/10 text-green-400 hover:bg-green-600/20 transition rounded-md"
                  title={t("actions.restore")}
                >
                  {t("actions.restore")}
                </Button>
              )}
            </div>

            {/* More options dropdown */}
            <DropdownMenu
              trigger={
                <Button
                  size="sm"
                  variant="secondary"
                  className="h-8 w-8 p-0 cursor-pointer bg-muted/30 border border-border-muted/30 hover:bg-muted/50 rounded-lg transition-colors flex items-center justify-center"
                >
                  <MoreHorizontal className="w-4 h-4 text-text-muted" />
                </Button>
              }
            >
              {/* Snooze */}
              <DropdownMenuItem
                onClick={onSnooze}
                className="flex items-center gap-2 hover:bg-muted/40 transition-colors"
              >
                <Clock className="w-4 h-4 text-purple-400" />
                <span>{t("actions.snooze")}</span>
              </DropdownMenuItem>

              {/* Download EML */}
              <DropdownMenuItem
                onClick={onDownloadEml}
                className="flex items-center gap-2 hover:bg-muted/40 transition-colors"
              >
                <Download className="w-4 h-4 text-blue-400" />
                <span>{t("actions.download_eml")}</span>
              </DropdownMenuItem>

              {/* Pin */}
              <DropdownMenuItem
                onClick={onPin}
                className="flex items-center gap-2 hover:bg-muted/40 transition-colors"
              >
                <Pin className="w-4 h-4 text-orange-400" />
                <span>{t("actions.pin")}</span>
              </DropdownMenuItem>

              {/* Mute */}
              <DropdownMenuItem
                onClick={onMute}
                className="flex items-center gap-2 hover:bg-muted/40 transition-colors"
              >
                <VolumeX className="w-4 h-4 text-rose-400" />
                <span>{t("actions.mute")}</span>
              </DropdownMenuItem>
            </DropdownMenu>
          </div>
        )}
      </div>

      {onBackClick && (
        <div className="lg:hidden flex-none flex items-center justify-center px-2 border-l border-border-muted/50 bg-card-bg/90 shadow-[-4px_0_12px_rgba(0,0,0,0.05)] z-30">
          <button
            className="p-2 text-text-muted hover:text-text-main"
            onClick={onBackClick}
            aria-label={t("actions.back")}
          >
            <ChevronLeft className="w-5 h-5" />
          </button>
        </div>
      )}
    </div>
  );
}
