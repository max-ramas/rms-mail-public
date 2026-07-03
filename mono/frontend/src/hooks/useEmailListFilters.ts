"use client";

import { useState, useEffect, useMemo } from "react";
import axios from "axios";
import { useQuery } from "@tanstack/react-query";
import type { Email, Label } from "@/hooks/useEmails";
import { API_BASE } from "@/hooks/useEmailTypes";

export interface UseEmailListFiltersOptions {
  emails: Email[];
  emailLabelsMap: Record<string, Label[]>;
  showUnassignedOnly?: boolean;
  useThreads: boolean;
  activeAccount: string;
  activeFolder: string;
  onFilterChange?: (filters: Record<string, string>) => void;
  onSearchResult: (emails: Email[]) => void;
}

async function fetchEmailCount(
  accountId: string,
  folderId: string,
  param: "unread" | "flagged" | "has_attachments",
): Promise<number> {
  const res = await axios.get(`${API_BASE}/api/emails/count`, {
    params: {
      account_id: accountId,
      folder_id: folderId,
      [param]: "true",
    },
  });
  return res.data?.count ?? 0;
}

export function useEmailListFilters({
  emails,
  emailLabelsMap,
  showUnassignedOnly,
  useThreads,
  activeAccount,
  activeFolder,
  onFilterChange,
  onSearchResult,
}: UseEmailListFiltersOptions) {
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [filterUnread, setFilterUnread] = useState(false);
  const [filterAttachments, setFilterAttachments] = useState(false);
  const [filterFlagged, setFilterFlagged] = useState(false);
  const [filterLabel, setFilterLabel] = useState("");
  const [filterTag, setFilterTag] = useState("");

  const { data: aiCategories = [] } = useQuery<
    { name: string; color: string }[]
  >({
    queryKey: ["ai-categories"],
    queryFn: async () => {
      try {
        const r = await axios.get(`${API_BASE}/api/system/ai-categories`);
        return (typeof r.data === "string" ? JSON.parse(r.data) : r.data) || [];
      } catch {
        return [];
      }
    },
    staleTime: 5 * 60 * 1000,
  });

  const countsEnabled = !!activeAccount;
  const { data: folderCounts } = useQuery({
    queryKey: ["email-folder-counts", activeAccount, activeFolder],
    queryFn: async () => {
      const [unread, flagged, attachments] = await Promise.all([
        fetchEmailCount(activeAccount, activeFolder, "unread"),
        fetchEmailCount(activeAccount, activeFolder, "flagged"),
        fetchEmailCount(activeAccount, activeFolder, "has_attachments"),
      ]);
      return { unread, flagged, attachments };
    },
    enabled: countsEnabled,
    staleTime: 0,
  });

  const unreadCountBadge = folderCounts?.unread ?? 0;
  const flaggedCountBadge = folderCounts?.flagged ?? 0;
  const attachmentsCountBadge = folderCounts?.attachments ?? 0;

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(searchQuery), 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const processedEmails = useMemo(() => {
    let result = emails;
    if (debouncedSearch) {
      const q = debouncedSearch.toLowerCase();
      result = result.filter(
        (e) =>
          e.subject?.toLowerCase().includes(q) ||
          e.sender_name?.toLowerCase().includes(q) ||
          e.sender_address?.toLowerCase().includes(q) ||
          e.snippet?.toLowerCase().includes(q),
      );
    }
    if (filterUnread) {
      result = result.filter((e) => !e.is_read);
    }
    if (filterAttachments) {
      result = result.filter((e) => e.has_attachments);
    }
    if (filterFlagged) {
      result = result.filter((e) => e.is_flagged);
    }
    if (filterLabel) {
      result = result.filter((e) =>
        emailLabelsMap[e.id]?.some((l) => l.id === filterLabel),
      );
    }
    if (showUnassignedOnly) {
      result = result.filter((e) => !e.assigned_to);
    }
    if (useThreads) {
      const seenThreads = new Set<string>();
      result = result.filter((e) => {
        if (!e.thread_id) return true;
        if (seenThreads.has(e.thread_id)) return false;
        seenThreads.add(e.thread_id);
        return true;
      });
    }
    return result;
  }, [
    emails,
    debouncedSearch,
    filterUnread,
    filterAttachments,
    filterFlagged,
    filterLabel,
    emailLabelsMap,
    showUnassignedOnly,
    useThreads,
  ]);

  useEffect(() => {
    const f: Record<string, string> = {};
    if (filterUnread) f.unread = "true";
    if (filterAttachments) f.has_attachments = "true";
    if (filterFlagged) f.flagged = "true";
    if (filterLabel) f.label_id = filterLabel;
    if (filterTag) f.tag = filterTag;
    if (debouncedSearch) f.search = debouncedSearch;
    onFilterChange?.(f);
  }, [
    filterUnread,
    filterAttachments,
    filterFlagged,
    filterLabel,
    filterTag,
    debouncedSearch,
    onFilterChange,
  ]);

  useEffect(() => {
    onSearchResult(processedEmails);
  }, [processedEmails, onSearchResult]);

  return {
    searchQuery,
    setSearchQuery,
    filterUnread,
    setFilterUnread,
    filterAttachments,
    setFilterAttachments,
    filterFlagged,
    setFilterFlagged,
    filterLabel,
    setFilterLabel,
    filterTag,
    setFilterTag,
    unreadCountBadge,
    flaggedCountBadge,
    attachmentsCountBadge,
    aiCategories,
    processedEmails,
  };
}
