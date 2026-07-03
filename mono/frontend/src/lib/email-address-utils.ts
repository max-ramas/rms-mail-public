/** Parse "Name <email>" and return friendly display names. */
export function formatAddresses(raw: string): string {
  if (!raw) return "";
  return raw
    .split(",")
    .map((part) => {
      const trimmed = part.trim();
      const match = trimmed.match(/^(.+?)\s*<(.+?)>$/);
      if (match) return match[1].trim();
      return trimmed;
    })
    .join(", ");
}
