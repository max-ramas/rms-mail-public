"use client";

import { Tags } from "lucide-react";
import { useTranslations } from "next-intl";

interface EmailAiTagsProps {
  tags: string[] | undefined;
}

export function EmailAiTags({ tags }: EmailAiTagsProps) {
  const t = useTranslations("mail");
  if (!tags || tags.length === 0) return null;

  return (
    <div>
      <h3 className="text-xs font-medium text-text-muted mb-2 flex items-center gap-1">
        <Tags className="w-3 h-3" /> {t("ai_tags")}
      </h3>
      <div className="flex flex-wrap gap-2">
        {tags.map((tag) => (
          <span
            key={tag}
            className="inline-flex items-center rounded-full bg-muted px-3 py-1 text-xs font-medium text-text-main/80"
          >
            {tag}
          </span>
        ))}
      </div>
    </div>
  );
}
