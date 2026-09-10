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
