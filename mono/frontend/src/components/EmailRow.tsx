"use client";

import React, { useRef, useState, useCallback } from "react";
import { formatEmailDate } from "@/lib/date-format";
import { Avatar } from "@/components/avatar";
import type { Label, Account } from "@/hooks/useEmails";
import type { GroupedEmail } from "@/components/email-list";
import {
  Mail,
  Paperclip,
  ChevronRight,
  CheckSquare,
  Square,
  MailOpen,
  Trash2,
} from "lucide-react";
import { format } from "date-fns";
import { setSemiTransparentDragImage } from "@/lib/drag-preview";

export const ROW_HEIGHT = 100;

const SWIPE_THRESHOLD = 80;
const SWIPE_VELOCITY = 0.5;

function formatDate(dateStr: string) {
  const locale =
    (typeof navigator !== "undefined" && navigator.language) || "en";
  return formatEmailDate(dateStr, locale);
}

export interface EmailRowProps {
  email: GroupedEmail;
  labels: Label[];
  selectedEmailId: string | null;
  selectedIds: Set<string>;
  selectAllActive: boolean;
  onSelectEmail: (id: string) => void;
  onToggleFlag?: (id: string) => void;
  t: (key: string, values?: Record<string, string | number>) => string;
  toggleSelected: (id: string) => void;
  accounts: Account[];
  swipeEnabled?: boolean;
  dragEnabled?: boolean;
  onDragStartEmail?: (e: React.DragEvent, emailId: string) => void;
  onDragEndEmail?: () => void;
  onSwipeAction?: (action: "delete" | "toggle_read", id: string) => void;
}

const EmailRow = React.memo(function EmailRow({
  email,
  labels,
  selectedEmailId,
  selectedIds,
  selectAllActive,
  onSelectEmail,
  onToggleFlag,
  t,
  toggleSelected,
  accounts,
  swipeEnabled = false,
  dragEnabled = false,
  onDragStartEmail,
  onDragEndEmail,
  onSwipeAction,
}: EmailRowProps) {
  const [offsetX, setOffsetX] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const startXRef = useRef(0);
  const startTimeRef = useRef(0);
  const pointerIdRef = useRef<number | null>(null);
  const draggedRef = useRef(false);

  const resetSwipe = useCallback(() => {
    setOffsetX(0);
    setIsDragging(false);
    pointerIdRef.current = null;
  }, []);

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!swipeEnabled || e.button !== 0 || e.pointerType !== "touch") return;
    pointerIdRef.current = e.pointerId;
    startXRef.current = e.clientX;
    startTimeRef.current = Date.now();
    setIsDragging(true);
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging || pointerIdRef.current !== e.pointerId) return;
    const delta = e.clientX - startXRef.current;
    setOffsetX(delta * 0.85);
  };

  const onPointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (pointerIdRef.current !== e.pointerId) return;
    const delta = e.clientX - startXRef.current;
    const elapsed = Math.max(Date.now() - startTimeRef.current, 1);
    const velocity = delta / elapsed;

    if (delta < -SWIPE_THRESHOLD || velocity < -SWIPE_VELOCITY) {
      onSwipeAction?.("toggle_read", email.id);
      draggedRef.current = true;
    } else if (delta > SWIPE_THRESHOLD || velocity > SWIPE_VELOCITY) {
      onSwipeAction?.("delete", email.id);
      draggedRef.current = true;
    } else if (Math.abs(delta) > 8) {
      draggedRef.current = true;
    }

    resetSwipe();
    e.currentTarget.releasePointerCapture(e.pointerId);
  };

  const onPointerCancel = () => {
    resetSwipe();
  };

  return (
    <div className="relative overflow-hidden border-b border-zinc-800 bg-list-bg">
      {swipeEnabled && (
        <>
          <div className="absolute inset-y-0 left-0 w-1/2 bg-red-500/20 flex items-center justify-start px-6">
            <Trash2 className="w-5 h-5 text-red-500" />
          </div>
          <div className="absolute inset-y-0 right-0 w-1/2 bg-green-500/20 flex items-center justify-end px-6">
            {email.is_read ? (
              <Mail className="w-5 h-5 text-green-500" />
            ) : (
              <MailOpen className="w-5 h-5 text-green-500" />
            )}
          </div>
        </>
      )}

      <div
        className={`${swipeEnabled ? "touch-pan-y" : ""} ${dragEnabled ? "cursor-grab active:cursor-grabbing" : "cursor-pointer"} px-4 py-2.5 flex items-start gap-2 relative group overflow-hidden ${
          selectedEmailId === email.id
            ? "bg-muted before:absolute before:start-0 before:top-0 before:bottom-0 before:w-1 before:bg-primary z-10"
            : "bg-list-bg hover:bg-muted"
        } ${email.is_pinned ? "border-s-2 border-orange-400" : ""}`}
        style={
          swipeEnabled
            ? {
                transform: `translateX(${offsetX}px)`,
                transition: isDragging ? "none" : "transform 0.2s ease-out",
              }
            : undefined
        }
        onPointerDown={swipeEnabled ? onPointerDown : undefined}
        onPointerMove={swipeEnabled ? onPointerMove : undefined}
        onPointerUp={swipeEnabled ? onPointerUp : undefined}
        onPointerCancel={swipeEnabled ? onPointerCancel : undefined}
        draggable={dragEnabled}
        onDragStart={
          dragEnabled
            ? (e) => {
                draggedRef.current = true;
                e.currentTarget.style.opacity = "0.35";
                setSemiTransparentDragImage(e);
                onDragStartEmail?.(e, email.id);
              }
            : undefined
        }
        onDragEnd={
          dragEnabled
            ? (e) => {
                e.currentTarget.style.opacity = "";
                onDragEndEmail?.();
                window.setTimeout(() => {
                  draggedRef.current = false;
                }, 0);
              }
            : undefined
        }
        data-selected={email.id}
        data-email-id={email.id}
        onClick={() => {
          if (draggedRef.current) {
            draggedRef.current = false;
            return;
          }
          onSelectEmail(email.id);
        }}
      >
        <button
          draggable={false}
          onClick={(e) => {
            e.stopPropagation();
            if (!draggedRef.current) toggleSelected(email.id);
          }}
          className="text-text-muted hover:text-primary transition-colors cursor-pointer shrink-0 self-center"
        >
          {selectedIds.has(email.id) || selectAllActive ? (
            <CheckSquare className="w-3.5 h-3.5 text-primary" />
          ) : (
            <Square className="w-3.5 h-3.5" />
          )}
        </button>
        <div className="flex-1 min-w-0">
          <div className="flex justify-between items-start mb-1">
            <span
              className={`text-sm flex items-center gap-1.5 ${
                !email.is_read ? "font-semibold" : "font-normal"
              }`}
            >
              <span
                className={`cursor-pointer text-sm select-none ${
                  email.is_flagged
                    ? "text-yellow-400"
                    : "text-zinc-700 hover:text-text-muted"
                }`}
                onClick={(e) => {
                  e.stopPropagation();
                  onToggleFlag?.(email.id);
                }}
              >
                {email.is_flagged ? "★" : "☆"}
              </span>
              {email.is_answered && (
                <span
                  className="text-sky-400 text-xs"
                  title={t("replied")}
                >
                  ↩
                </span>
              )}
              {!email.is_read && (
                <span
                  className="w-2 h-2 rounded-full bg-orange-500 inline-block shrink-0"
                  title={t("unread")}
                />
              )}
              {email.is_pinned && (
                <span
                  className="text-orange-400 text-xs"
                  title={t("actions.pin")}
                >
                  📌
                </span>
              )}
              {(() => {
                const isSentByMe = accounts?.some(
                  (a) =>
                    a.id === email.account_id &&
                    (a.username.toLowerCase() ===
                      email.sender_address.toLowerCase() ||
                      a.email.toLowerCase() ===
                        email.sender_address.toLowerCase()),
                );

                const displayName = isSentByMe
                  ? `${t("to_label")} ${email.recipient_address}`
                  : email.sender_name || email.sender_address;
                const avatarEmail = isSentByMe
                  ? email.recipient_address
                  : email.sender_address;

                return (
                  <>
                    <Avatar
                      src={email.avatar_url || null}
                      name={displayName}
                      email={avatarEmail}
                      size={28}
                    />
                    {email.spf_pass && email.dkim_pass && (
                      <span
                        className="text-green-400 text-[10px]"
                        title={t("verified")}
                      >
                        ✓
                      </span>
                    )}
                    <span className="truncate flex items-center gap-1.5 min-w-0">
                      <span className="truncate">{displayName}</span>
                    </span>
                  </>
                );
              })()}
            </span>
            <div className="flex flex-col items-end gap-1 shrink-0">
              <span className="text-[11px] text-text-muted flex items-center gap-1">
                {email.snooze_until &&
                  new Date(email.snooze_until) > new Date() && (
                    <span
                      className="text-purple-400 text-[10px]"
                      title={t("snooze_returns", {
                        time: format(new Date(email.snooze_until), "HH:mm"),
                      })}
                    >
                      💤
                    </span>
                  )}
                {formatDate(email.date_sent)}
              </span>
              {(email.has_attachments ||
                (email.thread_count && email.thread_count > 1)) && (
                <div className="flex items-center gap-1 text-[11px] font-medium text-text-muted bg-white/5 border border-white/10 rounded-md px-1.5 py-0.5">
                  {email.has_attachments && <Paperclip className="w-3 h-3" />}
                  {email.thread_count && email.thread_count > 1 && (
                    <>
                      <span>{email.thread_count}</span>
                      <ChevronRight className="w-3 h-3 text-primary" />
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
          <h4 className="text-sm font-medium text-text-main/80 truncate mb-1">
            {email.subject}
          </h4>
          <p className="text-xs text-text-muted line-clamp-2">
            {email.snippet}
          </p>
          {labels.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-2">
              {labels.map((lbl: Label) => (
                <span
                  key={lbl.id}
                  className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: lbl.color + "30",
                    color: lbl.color,
                    border: `1px solid ${lbl.color}60`,
                  }}
                >
                  {lbl.name}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
});

export default EmailRow;
