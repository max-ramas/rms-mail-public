"use client";

import { Mail } from "lucide-react";
import { useTranslations } from "next-intl";

interface EmailSummaryBlockProps {
  summary: string | null;
}

export function EmailSummaryBlock({ summary }: EmailSummaryBlockProps) {
  const t = useTranslations("mail");
  if (!summary) return null;

  return (
    <div className="bg-primary/10 border border-primary/30 rounded-xl p-4">
      <h3 className="text-xs font-bold text-amber-400 uppercase mb-1 flex items-center gap-2">
        <Mail className="w-3 h-3" /> {t("ai_summary")}
      </h3>
      <p className="text-sm text-amber-100">{summary}</p>
    </div>
  );
}
