"use client";

import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useTranslations } from "next-intl";

interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
  onFocus?: () => void;
}

export function SearchBar({ value, onChange, onFocus }: SearchBarProps) {
  const t = useTranslations("mail");

  return (
    <div className="relative w-full">
      <Search className="w-4 h-4 absolute start-3 top-1/2 -translate-y-1/2 text-text-muted" />
      <Input
        id="search-input"
        type="text"
        placeholder={t("search.placeholder")}
        className="ps-10"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={onFocus}
      />
    </div>
  );
}
