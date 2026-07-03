"use client";

import { useState, useRef, useEffect } from "react";
import { Tags, Check } from "lucide-react";
import { useTranslations } from "next-intl";
import type { Label } from "@/hooks/useEmails";

interface EmailLabelsDropdownProps {
  labelsData: Label[] | undefined;
  selectedLabelIds: Set<string>;
  onToggleLabel: (labelId: string) => void;
}

export function EmailLabelsDropdown({
  labelsData,
  selectedLabelIds,
  onToggleLabel,
}: EmailLabelsDropdownProps) {
  const t = useTranslations("mail");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  if (!labelsData || labelsData.length === 0) return null;

  const activeLabels = labelsData.filter((l) => selectedLabelIds.has(l.id));
  const hasActive = activeLabels.length > 0;

  return (
    <div className="relative inline-flex items-center" ref={ref}>
      <button
        onClick={() => setOpen(!open)}
        className={`p-1.5 rounded-lg border transition-all ${
          hasActive
            ? "bg-emerald-500/15 border-emerald-500/35 text-emerald-400 hover:bg-emerald-500/25"
            : "bg-muted/20 border-border-muted/40 text-text-muted hover:text-text-main hover:bg-muted/30"
        }`}
        title={t("labels")}
      >
        <Tags className="w-4 h-4" />
      </button>
      {open && (
        <div className="absolute top-9.5 right-0 z-50 min-w-45 rounded-xl border border-zinc-800 bg-zinc-950/90 backdrop-blur-xl p-1.5 shadow-2xl animate-in fade-in slide-in-from-top-1 duration-150">
          {labelsData.map((l) => {
            const isActive = selectedLabelIds.has(l.id);
            return (
              <button
                key={l.id}
                onClick={() => {
                  onToggleLabel(l.id);
                  setOpen(false);
                }}
                className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer ${
                  isActive
                    ? "bg-zinc-800 text-text-main"
                    : "text-text-muted hover:bg-zinc-900 hover:text-text-main"
                }`}
              >
                <span
                  className="w-2.5 h-2.5 rounded-full shrink-0"
                  style={{ backgroundColor: l.color }}
                />
                <span>{l.name}</span>
                {isActive && (
                  <Check className="w-3 h-3 text-emerald-400 ms-auto" />
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
