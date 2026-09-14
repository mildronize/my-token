// story-1/ticket-14: small display formatters for the usage console —
// no business logic, just number-to-string presentation.

/** `$1234.5` -> `"$1,234.50"` */
export function formatCost(cost: number): string {
  return `$${cost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

/** `1234` -> `"1,234"` */
export function formatInt(n: number): string {
  return Math.round(n).toLocaleString();
}

// story-3/ticket-3: the Machines page's "Last reported" column
// (machines.last_seen_at) answers "is this machine's collector still
// alive" (contract's Data model) — date alone (ApiKeySettings.tsx's own
// formatDate) would hide same-day staleness, so this includes the time.
/** An ISO timestamp -> `"Sep 13, 2026, 1:31 PM"` in the viewer's own locale/timezone. */
export function formatDateTime(value: string): string {
  return new Date(value).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * `1234567` -> `"1.23M"`, `8900` -> `"8.9K"`, `120` -> `"120"` — the
 * compact form used in tile subtexts and breakdown panels, where a raw
 * digit string would be too wide.
 */
export function formatTokensCompact(tokens: number): string {
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(tokens >= 10_000_000 ? 1 : 2)}M`;
  }
  if (tokens >= 1_000) {
    return `${(tokens / 1_000).toFixed(tokens >= 10_000 ? 0 : 1)}K`;
  }
  return formatInt(tokens);
}
