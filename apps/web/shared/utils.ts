export function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

// Relative time with minute precision, e.g. "in 28m", "in 3h 5m",
// "in 2d 4h", "15m ago". Computed once at render — not live-updating.
export function formatRelativeTime(dateStr: string): string {
  const diffMs = new Date(dateStr).getTime() - Date.now();
  const future = diffMs > 0;
  const totalMinutes = Math.round(Math.abs(diffMs) / 60000);
  if (totalMinutes < 1) return future ? "in <1m" : "just now";

  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;

  const parts: string[] = [];
  if (days) parts.push(`${days}d`);
  if (hours) parts.push(`${hours}h`);
  if (minutes && !days) parts.push(`${minutes}m`);

  const text = parts.join(" ");
  return future ? `in ${text}` : `${text} ago`;
}

export type AbsoluteTimeStyle = "shortDate" | "shortDateTime" | "localeDate" | "localeDateTime";

const absoluteTimeFormats: Record<AbsoluteTimeStyle, Intl.DateTimeFormatOptions | undefined> = {
  shortDate: {
    month: "short",
    day: "numeric",
  },
  shortDateTime: {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  },
  localeDate: undefined,
  localeDateTime: undefined,
};

export function formatAbsoluteTime(
  dateStr: string | null | undefined,
  style: AbsoluteTimeStyle = "localeDateTime",
  fallback = "—",
): string {
  if (!dateStr) return fallback;
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;

  if (style === "localeDate") return date.toLocaleDateString();
  if (style === "localeDateTime") return date.toLocaleString();

  return date.toLocaleString(undefined, absoluteTimeFormats[style]);
}

export function formatAbsoluteTimeTooltip(dateStr: string | null | undefined, fallback = "—"): string {
  if (!dateStr) return fallback;
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;

  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const timestamp = date.toLocaleString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZoneName: "short",
  });

  return `${timestamp} (${timezone}, ${formatTimezoneOffset(date)})`;
}

function formatTimezoneOffset(date: Date): string {
  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absMinutes = Math.abs(offsetMinutes);
  const hours = String(Math.floor(absMinutes / 60)).padStart(2, "0");
  const minutes = String(absMinutes % 60).padStart(2, "0");
  return `UTC${sign}${hours}:${minutes}`;
}
