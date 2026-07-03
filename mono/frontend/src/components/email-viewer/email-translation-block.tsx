"use client";

import { GlobeOff } from "lucide-react";
import { useTranslations } from "next-intl";

interface EmailTranslationBlockProps {
  translation: string | null;
  onDismiss: () => void;
}

export function EmailTranslationBlock({
  translation,
  onDismiss,
}: EmailTranslationBlockProps) {
  const t = useTranslations("mail");
  if (!translation) return null;

  return (
    <div className="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 relative">
      <h3 className="text-xs font-bold text-blue-400 uppercase mb-1 flex items-center gap-2">
        🌐 {t("actions.translation")}
        <button
          onClick={onDismiss}
          className="ms-auto p-1 rounded-md text-blue-400/60 hover:text-blue-300 hover:bg-blue-500/10 transition-colors"
          title={t("actions.hide_translation")}
        >
          <GlobeOff className="w-3.5 h-3.5" />
        </button>
      </h3>
      <p className="text-sm text-blue-100 whitespace-pre-wrap">{translation}</p>
    </div>
  );
}
