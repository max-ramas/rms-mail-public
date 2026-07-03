import type { Email } from "@/hooks/useEmailTypes";

/** Gmail-style bulk flag: star all unless every selected message is already starred. */
export function resolveBulkSetFlagged(
  emails: Email[],
  selectedIds: Set<string>,
  selectAllActive: boolean,
): boolean | undefined {
  if (selectAllActive) return undefined;
  const selected = emails.filter((e) => selectedIds.has(e.id));
  if (selected.length === 0) return true;
  return !selected.every((e) => e.is_flagged);
}
