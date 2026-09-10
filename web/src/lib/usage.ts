// story-1/ticket-14: TanStack Query hooks for the console's read surface,
// GET /api/bff/usage/summary and GET /api/bff/usage/windows —
// bff-openapi.yaml's own `snake_case` wire shape (`group_by`,
// `reporting_installs`) is kept verbatim here too, same reasoning that
// file's own comment gives: the contract is this ticket's literal input,
// not something to "fix" into this SPA's usual camelCase convention.
import { useQuery } from "@tanstack/react-query";

import { bffFetch } from "~/lib/api/client";
import type { components } from "~/lib/api/bff-schema.gen";

export type UsageWindow = "5h" | "24h" | "today" | "week" | "month" | "lifetime";
export type UsageGroupBy = "actor" | "path" | "machine";

// FIXED_WINDOWS is exactly GET /usage/windows' own five rows, in the
// contract's own order — the console's time-window tabs render this set
// (a "lifetime" tab makes no sense as a *tab*, since every one of the
// other five already answers "since when" relative to now; lifetime only
// exists as the one summary tile that needs an unbounded range —
// ~/app/usage/UsagePage.tsx's own doc comment says more).
export const FIXED_WINDOWS: UsageWindow[] = ["5h", "24h", "today", "week", "month"];

export type UsageTotals = components["schemas"]["UsageTotals"];
export type UsageBreakdownRow = components["schemas"]["UsageBreakdownRow"];
export type UsageSummary = components["schemas"]["UsageSummary"];
export type UsageWindowRow = components["schemas"]["UsageWindowRow"];
export type UsageWindows = components["schemas"]["UsageWindows"];

export const usageSummaryQueryKey = (window: UsageWindow, groupBy: UsageGroupBy) =>
  ["bff", "usage", "summary", window, groupBy] as const;

export const usageWindowsQueryKey = ["bff", "usage", "windows"] as const;

/** GET /api/bff/usage/summary?window=...&group_by=... */
export function useUsageSummaryQuery(window: UsageWindow, groupBy: UsageGroupBy) {
  return useQuery({
    queryKey: usageSummaryQueryKey(window, groupBy),
    queryFn: () =>
      bffFetch<UsageSummary>(`/usage/summary?window=${window}&group_by=${groupBy}`),
  });
}

/** GET /api/bff/usage/windows — the fixed 5h/24h/today/week/month table. */
export function useUsageWindowsQuery() {
  return useQuery({
    queryKey: usageWindowsQueryKey,
    queryFn: () => bffFetch<UsageWindows>("/usage/windows"),
  });
}
