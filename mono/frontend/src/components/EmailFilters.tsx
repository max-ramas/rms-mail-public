"use client";

import React, { useState, useRef, useEffect } from "react";
import { SearchBar } from "@/components/search-bar";
import type { Label } from "@/hooks/useEmails";
import {
  Mail,
  Star,
  Paperclip,
  Tag,
  ChevronDown,
  CheckSquare,
  Square,
  Sparkles,
  Filter,
  ChevronLeft,
  Menu,
} from "lucide-react";

export interface EmailFiltersProps {
  searchQuery: string;
  onSearchChange: (q: string) => void;
  filterUnread: boolean;
  onFilterUnreadChange: (v: boolean) => void;
  filterAttachments: boolean;
  onFilterAttachmentsChange: (v: boolean) => void;
  filterFlagged: boolean;
  onFilterFlaggedChange: (v: boolean) => void;
  filterLabel: string;
  onFilterLabelChange: (v: string) => void;
  filterTag: string;
  onFilterTagChange: (v: string) => void;
  labels: Label[];
  aiCategories: { name: string; color: string }[];
  unreadCountBadge: number;
  flaggedCountBadge: number;
  attachmentsCountBadge: number;
  t: (key: string, values?: Record<string, string | number>) => string;
  selectedIds: Set<string>;
  selectAllActive: boolean;
  onClearSelected: () => void;
  onSelectAll: () => void;
  onMenuClick?: () => void;
}

const EmailFilters = React.memo(function EmailFilters({
  searchQuery,
  onSearchChange,
  filterUnread,
  onFilterUnreadChange,
  filterAttachments,
  onFilterAttachmentsChange,
  filterFlagged,
  onFilterFlaggedChange,
  filterLabel,
  onFilterLabelChange,
  filterTag,
  onFilterTagChange,
  labels,
  aiCategories,
  unreadCountBadge,
  flaggedCountBadge,
  attachmentsCountBadge,
  t,
  selectedIds,
  selectAllActive,
  onClearSelected,
  onSelectAll,
  onMenuClick,
}: EmailFiltersProps) {
  const [filtersExpanded, setFiltersExpanded] = useState(false);
  const [labelsDropdownOpen, setLabelsDropdownOpen] = useState(false);
  const [aiDropdownOpen, setAiDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const aiDropdownRef = useRef<HTMLDivElement>(null);

  // Close custom labels and AI dropdowns on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setLabelsDropdownOpen(false);
      }
      if (
        aiDropdownRef.current &&
        !aiDropdownRef.current.contains(event.target as Node)
      ) {
        setAiDropdownOpen(false);
      }
    }
    if (labelsDropdownOpen || aiDropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [labelsDropdownOpen, aiDropdownOpen]);

  return (
    <>
      <div className="h-16 px-4 flex items-center gap-2 border-b border-border-muted/50 shrink-0">
        <SearchBar value={searchQuery} onChange={onSearchChange} />
        {onMenuClick && (
          <button
            className="lg:hidden p-2 -mr-2 text-text-muted hover:text-text-main shrink-0"
            onClick={onMenuClick}
            aria-label={t("menu")}
          >
            <Menu className="w-5 h-5" />
          </button>
        )}
      </div>
      <div className="px-4 py-2.5 border-b border-border-muted/50 shrink-0">
        <div className="flex items-start gap-3">
          {/* Custom Select All checkbox button */}
          <button
            title={t("select_all")}
            onClick={() => {
              if (selectedIds.size > 0 || selectAllActive) {
                onClearSelected();
              } else {
                onSelectAll();
              }
            }}
            className="text-text-muted hover:text-primary transition-colors cursor-pointer shrink-0 self-center"
          >
            {selectedIds.size > 0 || selectAllActive ? (
              <CheckSquare className="w-3.5 h-3.5 text-primary animate-in zoom-in-75 duration-200" />
            ) : (
              <Square className="w-3.5 h-3.5" />
            )}
          </button>

          {/* Premium Filter Chips row */}
          <div className="flex-1 flex flex-wrap items-center gap-1.5 min-w-0">
            {/* Filter: Unread */}
            <button
              onClick={() => onFilterUnreadChange(!filterUnread)}
              className={`inline-flex items-center gap-1.5 px-2 h-[24px] rounded-full text-[11px] font-medium border transition-all duration-200 cursor-pointer select-none ${
                filterUnread
                  ? "bg-amber-500/10 text-amber-400 border-amber-500/30 shadow-[0_0_12px_rgba(245,158,11,0.05)]"
                  : "bg-card-bg/40 text-text-muted border-border-muted hover:text-text-main hover:bg-muted/30"
              }`}
            >
              <Mail className="w-3 h-3 shrink-0" />
              {filtersExpanded ? (
                <span>{t("filter_unread", { count: unreadCountBadge })}</span>
              ) : (
                unreadCountBadge > 0 && (
                  <span className="font-semibold">{unreadCountBadge}</span>
                )
              )}
            </button>

            {/* Filter: Flagged / Starred */}
            <button
              onClick={() => onFilterFlaggedChange(!filterFlagged)}
              className={`inline-flex items-center gap-1.5 px-2 h-[24px] rounded-full text-[11px] font-medium border transition-all duration-200 cursor-pointer select-none ${
                filterFlagged
                  ? "bg-yellow-500/10 text-yellow-400 border-yellow-500/30 shadow-[0_0_12px_rgba(234,179,8,0.05)]"
                  : "bg-card-bg/40 text-text-muted border-border-muted hover:text-text-main hover:bg-muted/30"
              }`}
            >
              <Star className="w-3 h-3 fill-current shrink-0" />
              {filtersExpanded ? (
                <span>
                  {t("filter_flagged", {
                    count: flaggedCountBadge,
                  })}
                </span>
              ) : (
                flaggedCountBadge > 0 && (
                  <span className="font-semibold">{flaggedCountBadge}</span>
                )
              )}
            </button>

            {/* Filter: Attachments */}
            <button
              onClick={() => onFilterAttachmentsChange(!filterAttachments)}
              className={`inline-flex items-center gap-1.5 px-2 h-[24px] rounded-full text-[11px] font-medium border transition-all duration-200 cursor-pointer select-none ${
                filterAttachments
                  ? "bg-blue-500/10 text-blue-400 border-blue-500/30 shadow-[0_0_12px_rgba(59,130,246,0.05)]"
                  : "bg-card-bg/40 text-text-muted border-border-muted hover:text-text-main hover:bg-muted/30"
              }`}
            >
              <Paperclip className="w-3 h-3 shrink-0" />
              {filtersExpanded ? (
                <span>
                  {t("filter_attachments", {
                    count: attachmentsCountBadge,
                  })}
                </span>
              ) : (
                attachmentsCountBadge > 0 && (
                  <span className="font-semibold">{attachmentsCountBadge}</span>
                )
              )}
            </button>

            {/* Filter: Labels Select Pill */}
            {labels.length > 0 && (
              <div
                className="relative inline-flex items-center"
                ref={dropdownRef}
              >
                <button
                  onClick={() => setLabelsDropdownOpen(!labelsDropdownOpen)}
                  className={`inline-flex items-center gap-1.5 px-2 h-[24px] rounded-full text-[11px] font-medium border transition-all duration-200 cursor-pointer select-none ${
                    filterLabel
                      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-400"
                      : "border-border-muted text-text-muted hover:text-text-main hover:bg-muted/30"
                  }`}
                >
                  <Tag className="w-3 h-3 shrink-0" />
                  {filtersExpanded ? (
                    filterLabel ? (
                      <>
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                          style={{
                            backgroundColor:
                              labels.find((l) => l.id === filterLabel)?.color ||
                              "#10B981",
                          }}
                        />
                        <span className="truncate max-w-[80px]">
                          {labels.find((l) => l.id === filterLabel)?.name}
                        </span>
                      </>
                    ) : (
                      <span>{t("all_labels")}</span>
                    )
                  ) : (
                    filterLabel && (
                      <span
                        className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                        style={{
                          backgroundColor:
                            labels.find((l) => l.id === filterLabel)?.color ||
                            "#10B981",
                        }}
                      />
                    )
                  )}
                  <ChevronDown className="w-3 h-3 shrink-0" />
                </button>

                {labelsDropdownOpen && (
                  <div className="absolute top-[30px] start-0 z-50 min-w-[160px] rounded-xl border border-zinc-800 bg-zinc-950/90 backdrop-blur-xl p-1.5 shadow-2xl animate-in fade-in slide-in-from-top-1 duration-150">
                    {/* All Labels Option */}
                    <button
                      onClick={() => {
                        onFilterLabelChange("");
                        setLabelsDropdownOpen(false);
                      }}
                      className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer ${
                        filterLabel === ""
                          ? "bg-zinc-800 text-text-main"
                          : "text-text-muted hover:bg-zinc-900 hover:text-text-main"
                      }`}
                    >
                      <span className="w-2.5 h-2.5 rounded-full border border-dashed border-zinc-600 inline-block shrink-0" />
                      <span>{t("all_labels")}</span>
                    </button>

                    <div className="h-px bg-zinc-800 my-1" />

                    {/* Individual Label Options */}
                    <div className="max-h-[200px] overflow-y-auto space-y-0.5">
                      {labels.map((l: Label) => (
                        <button
                          key={l.id}
                          onClick={() => {
                            onFilterLabelChange(l.id);
                            setLabelsDropdownOpen(false);
                          }}
                          className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer ${
                            filterLabel === l.id
                              ? "bg-zinc-800 text-text-main"
                              : "text-text-muted hover:bg-zinc-900 hover:text-text-main"
                          }`}
                        >
                          <span
                            className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                            style={{ backgroundColor: l.color }}
                          />
                          <span className="truncate">{l.name}</span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Filter: AI Categories Select Pill */}
            {aiCategories.length > 0 && (
              <div
                className="relative inline-flex items-center"
                ref={aiDropdownRef}
              >
                <button
                  onClick={() => setAiDropdownOpen(!aiDropdownOpen)}
                  className={`inline-flex items-center gap-1.5 px-2 h-[24px] rounded-full text-[11px] font-medium border transition-all duration-200 cursor-pointer select-none ${
                    filterTag
                      ? "border-current"
                      : "border-border-muted text-text-muted hover:text-text-main hover:bg-muted/30"
                  }`}
                  style={
                    filterTag
                      ? (() => {
                          const hex =
                            aiCategories.find((c) => c.name === filterTag)
                              ?.color || "#10B981";
                          return {
                            backgroundColor: hex + "18",
                            color: hex,
                            borderColor: hex + "40",
                          };
                        })()
                      : undefined
                  }
                >
                  <Sparkles className="w-3 h-3 shrink-0" />
                  {filtersExpanded ? (
                    filterTag ? (
                      <>
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                          style={{
                            backgroundColor:
                              aiCategories.find((c) => c.name === filterTag)
                                ?.color || "#10B981",
                          }}
                        />
                        <span className="truncate max-w-[80px]">
                          {filterTag}
                        </span>
                      </>
                    ) : (
                      <span>{t("all_categories")}</span>
                    )
                  ) : (
                    filterTag && (
                      <span
                        className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                        style={{
                          backgroundColor:
                            aiCategories.find((c) => c.name === filterTag)
                              ?.color || "#10B981",
                        }}
                      />
                    )
                  )}
                  <ChevronDown className="w-3 h-3 shrink-0" />
                </button>

                {aiDropdownOpen && (
                  <div className="absolute top-[30px] start-0 z-50 min-w-[160px] rounded-xl border border-zinc-800 bg-zinc-950/90 backdrop-blur-xl p-1.5 shadow-2xl animate-in fade-in slide-in-from-top-1 duration-150">
                    {/* All Categories Option */}
                    <button
                      onClick={() => {
                        onFilterTagChange("");
                        setAiDropdownOpen(false);
                      }}
                      className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer ${
                        filterTag === ""
                          ? "bg-zinc-800 text-text-main"
                          : "text-text-muted hover:bg-zinc-900 hover:text-text-main"
                      }`}
                    >
                      <span className="w-2.5 h-2.5 rounded-full border border-dashed border-zinc-600 inline-block shrink-0" />
                      <span>{t("all_categories")}</span>
                    </button>

                    <div className="h-px bg-zinc-800 my-1" />

                    {/* Individual AI Category Options */}
                    <div className="max-h-[200px] overflow-y-auto space-y-0.5">
                      {aiCategories.map((cat) => (
                        <button
                          key={cat.name}
                          onClick={() => {
                            onFilterTagChange(cat.name);
                            setAiDropdownOpen(false);
                          }}
                          className={`w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-[11px] font-medium transition-colors cursor-pointer ${
                            filterTag === cat.name
                              ? "bg-zinc-800 text-text-main"
                              : "text-text-muted hover:bg-zinc-900 hover:text-text-main"
                          }`}
                        >
                          <span
                            className="w-2.5 h-2.5 rounded-full shrink-0 inline-block"
                            style={{ backgroundColor: cat.color }}
                          />
                          <span className="truncate">{cat.name}</span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Filter Toggle Button (Moved outside flex-wrap so it never wraps) */}
          <button
            onClick={() => setFiltersExpanded(!filtersExpanded)}
            className="inline-flex items-center justify-center w-[24px] h-[24px] rounded-full text-text-muted hover:text-text-main hover:bg-muted/30 border border-transparent hover:border-border-muted transition-all duration-200 shrink-0 self-start mt-0.5"
          >
            {filtersExpanded ? (
              <ChevronLeft className="w-3.5 h-3.5 shrink-0" />
            ) : (
              <Filter className="w-3.5 h-3.5 shrink-0" />
            )}
          </button>
        </div>
      </div>
    </>
  );
});

export default EmailFilters;
