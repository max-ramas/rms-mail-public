export type UpdateChannel = "stable" | "beta" | "alpha";

export function normalizeUpdateChannel(raw?: string): UpdateChannel {
  const c = (raw || "stable").toLowerCase();
  if (c === "beta" || c === "alpha") return c;
  return "stable";
}

export function updateChannelLabel(channel?: string): string {
  switch (normalizeUpdateChannel(channel)) {
    case "beta":
      return "Beta";
    case "alpha":
      return "Alpha";
    default:
      return "Stable";
  }
}

export function updateChannelBadgeClass(channel?: string): string {
  switch (normalizeUpdateChannel(channel)) {
    case "beta":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-400";
    case "alpha":
      return "bg-purple-500/15 text-purple-700 dark:text-purple-400";
    default:
      return "bg-muted/80 text-muted-foreground";
  }
}
